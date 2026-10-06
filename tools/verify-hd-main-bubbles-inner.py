#!/usr/bin/env python3
"""正常片頭與合法任命軍師；畫面、輸入及字模證據留本機。"""
import argparse
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import struct
import textwrap
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out', required=True)
parser.add_argument('--binary', required=True)
parser.add_argument('--title-reference', required=True)
parser.add_argument('--baseline', action='store_true')
args = parser.parse_args()
ROOT = Path('/src')
OUT = ROOT / args.out
PACK = ROOT / 'workplace/hd-assets-mapcursor-v50-r1'
spec = importlib.util.spec_from_file_location('gui', ROOT/'tools/verify-window-inner.py')
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = OUT
palette = [(0,0,0),(0,0,170),(0,170,0),(0,170,170),(170,0,0),(170,0,170),(170,85,0),(170,170,170)]

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def shot(wid, tag):
    path = OUT / f'{tag}-{len(gui.receipt["captures"])}.png'
    geo = gui.geo(wid)
    gui.run(['ffmpeg','-nostdin','-hide_banner','-loglevel','error','-y','-f','x11grab',
             '-draw_mouse','0','-video_size',f'{geo["WIDTH"]}x{geo["HEIGHT"]}',
             '-i',f':99+{geo["X"]},{geo["Y"]}','-frames:v','1','-threads','1',str(path)])
    gui.receipt['captures'].append(dict(file=path.name,width=geo['WIDTH'],height=geo['HEIGHT'],sha256=sha(path)))
    return path

gui.shot = shot

def crop(raw,width,x,y,w,h):
    return b''.join(raw[((y+r)*width+x)*3:((y+r)*width+x+w)*3] for r in range(h))

def nearest(raw,w,h):
    return b''.join(b''.join(raw[(y*w+x)*3:(y*w+x+1)*3]*4 for x in range(w))*4 for y in range(h))

def theme(wid,high):
    gui.key(wid,'Escape')
    gui.choose_ready(wid,1,int(high))
    gui.key(wid,'Escape')
    gui.run(['xdotool','windowsize',wid,'2560' if high else '640','1632' if high else '408'])
    gui.run(['xdotool','windowmove',wid,'0','0'])
    gui.move(wid,320,200)
    time.sleep(.5)

def language(wid,row):
    gui.key(wid,'Escape')
    gui.choose_ready(wid,0,row)
    gui.key(wid,'Escape')
    gui.move(wid,320,200)
    time.sleep(.5)

def glyph_projection(text,locale):
    if locale=='en':
        lines=textwrap.wrap(text,14)
    else:
        lines,line,width=[],'',0
        for char in text:
            size=1 if ord(char)<128 else 2
            if width+size>14:
                lines.append(line.rstrip())
                line,width='',0
                if char==' ': continue
            line+=char
            width+=size
        if line.strip(): lines.append(line.rstrip())
    # 可放下的兩行保持原版雙倍高度；長句使用一般字級。
    sy,top,gap=(2,4,40) if len(lines)<=2 else (1,0,16)
    assert top+(len(lines)-1)*gap+16*sy<=83
    assert ''.join(''.join(lines).split())==''.join(text.split())
    glyphs={}
    with gzip.open(ROOT/'fonts/unifont.hex.gz','rt') as font:
        for line in font:
            if ':' in line:
                code,bits=line.strip().split(':')
                char=chr(int(code,16))
                if char in text: glyphs[char]=bytes.fromhex(bits)
    projections=[]
    for color in palette:
        result=bytearray(b'\xff'*(112*83*3))
        for row,line in enumerate(lines):
            column=0
            for char in line:
                glyph=glyphs[char]
                stride=len(glyph)//16
                assert stride in (1,2)
                for gy in range(16):
                    bits=int.from_bytes(glyph[gy*stride:(gy+1)*stride],'big')
                    for gx in range(stride*8):
                        if bits & (1<<(stride*8-gx-1)):
                            for repeat in range(sy):
                                pos=((top+row*gap+gy*sy+repeat)*112+column+gx)*3
                                result[pos:pos+3]=bytes(color)
                column+=stride*8
            assert column<=112
        projections.append(bytes(result))
    return lines,projections

def source_portrait(edition,name):
    folder='三國演義' if edition=='base' else '三國演義1加強版'
    stem=Path('/orig')/folder/'DATA2'
    names=stem.with_suffix('.NAM').read_bytes()
    idx=stem.with_suffix('.IDX').read_bytes()
    grp=stem.with_suffix('.GRP').read_bytes()
    ends=struct.unpack('<'+'I'*(len(idx)//4),idx)
    entries=[(names[i:i+8].decode('ascii').rstrip()+
              names[i+8:i+16].split(b'\0',1)[0].decode('ascii').strip()).upper()
             for i in range(0,len(names),16)]
    slot=entries.index('BASEGEN.001')
    start=ends[slot-1] if slot else 0
    table=grp[start:ends[slot]]
    assert len(table)==350*30
    for i in range(350):
        rec=table[i*30:(i+1)*30]
        if rec[:6].strip(b' \0')==name.encode('cp950'):
            portrait=f'F{rec[27]:03d}'
            gui.receipt.setdefault('person_sources',{}).setdefault(edition+'/'+name,dict(
                source_file=folder+'/DATA2.GRP',source_sha256=sha(stem.with_suffix('.GRP')),
                container_entry='BASEGEN.001',general_index=i,record_offset=i*30,
                name_field_hex=rec[:6].hex(),portrait_offset=27,portrait=portrait))
            return portrait
    raise RuntimeError('原始人物表找不到 '+name)

def flip(raw,w,h):
    return b''.join(b''.join(raw[(r*w+x)*3:(r*w+x+1)*3] for x in reversed(range(w))) for r in range(h))

def sample(wid,edition,stage,key,left):
    x,y=(496,188) if left else (432,88)
    portrait_name=source_portrait(edition,'關羽' if left else '劉備')
    px,py=(424,180) if left else (552,80)
    source=ROOT/f'workplace/hd-inventory/{edition}/img/DATA3/{portrait_name}.png'
    master=PACK/(portrait_name+'.png')
    source_rgb=gui.rgb(source)
    native_rgb=gui.rgb(master)
    if left:
        source_rgb=flip(source_rgb,64,80)
        native_rgb=flip(native_rgb,256,320)
    for row,locale in enumerate(['zh-Hant','en','ja']):
        language(wid,row)
        tag=f'{edition}-{stage}-{locale}'
        original=shot(wid,tag+'-original')
        raw=gui.rgb(original)
        gui.check(tag+'-whole-source-portrait',crop(raw,640,px,py,64,80)==source_rgb)
        proof=None
        if locale!='zh-Hant':
            catalog=json.loads((ROOT/f'internal/i18n/lang/{locale}.json').read_text())
            name='Guan Yu' if locale=='en' else '関羽'
            text=catalog[key] % name if key=='bub.chiefOrder' else catalog[key]
            lines,projections=glyph_projection(text,locale)
            observed=crop(raw,640,x,y,112,83)
            matches=[i for i,p in enumerate(projections) if p==observed]
            proof=dict(key=key,locale=locale,full_text=text,visible_lines=lines,rect=[x,y,112,83],
                       color=matches[0] if matches else None,matched=bool(matches),complete_text=True)
            if args.baseline:
                gui.check(tag+'-old-defect-rejected',not matches)
            else:
                gui.check(tag+'-full-free-glyphs',bool(matches))
        theme(wid,True)
        high=shot(wid,tag+'-hd')
        native=gui.rgb(high)
        gui.check(tag+'-native-hd-portrait',crop(native,2560,px*4,py*4,256,320)==native_rgb)
        if proof and not args.baseline:
            expected=projections[proof['color']]
            gui.check(tag+'-hd-full-free-glyphs',crop(native,2560,x*4,y*4,448,332)==nearest(expected,112,83))
            proof['expected_sha256']=hashlib.sha256(expected).hexdigest()
        theme(wid,False)
        restored=shot(wid,tag+'-restored')
        gui.check(tag+'-whole-original-restored',raw==gui.rgb(restored))
        gui.receipt.setdefault('samples',[]).append(dict(edition=edition,stage=stage,locale=locale,
            original=original.name,high=high.name,restored=restored.name,text=proof,
            portrait=portrait_name,portrait_rect=[px,py,64,80],source_sha256=sha(source),master_sha256=sha(master)))
    language(wid,0)

def run():
    if OUT.exists():
        raise RuntimeError('輸出已存在，拒絕覆蓋既有證據')
    OUT.mkdir(parents=True)
    for path in [OUT,ROOT/'workplace/hd-window/player']:
        assert (path.stat().st_uid,path.stat().st_gid)==(os.getuid(),os.getgid())
    shutil.copy2(ROOT/args.binary,OUT/'san1-window-check')
    shutil.copy2(ROOT/args.title_reference,OUT/'title-reference.png')
    shutil.copy2(Path(__file__),OUT/Path(__file__).name)
    shutil.copy2(ROOT/'tools/verify-window-inner.py',OUT/'verify-window-inner.py')
    os.environ.update(DISPLAY=':99',LIBGL_ALWAYS_SOFTWARE='1',XDG_RUNTIME_DIR='/tmp')
    gui.start(['Xvfb',':99','-screen','0','2800x1900x24','-nolisten','tcp'],'xvfb')
    gui.wait(['xdotool','getdisplaygeometry'])
    gui.receipt.update(method='正常片頭、新局、合法軍師任命、選項列換語言與 Theme、續頁',
        scenario='001',lord='劉備',difficulty=5,state_injection=False,seed_injection=False,
        binary_sha256=sha(OUT/'san1-window-check'),pack_sha256=sha(PACK/'manifest.json'),
        tool_sha256={Path(__file__).name:sha(Path(__file__)),'verify-window-inner.py':sha(ROOT/'tools/verify-window-inner.py')},
        baseline=args.baseline,scope='主畫面兩句的完整譯文、姓名與 Theme 顯示；不是原版 oracle 或音畫驗收')
    for edition in ['base','plus']:
        proc,wid=gui.launch(edition,hd_assets=PACK,tag=edition)
        gui.key(wid,'1','1','1','1','5')
        # 任命配方沿用既有 SCG05 正常路線，不注入人物或事件。
        time.sleep(2)
        main=shot(wid,edition+'-main')
        face=ROOT/f'workplace/hd-inventory/{edition}/img/DATA3/F005.png'
        gui.check(edition+'-normal-main-lord',gui.rgb(main,'64:80:536:116')==gui.rgb(face))
        gui.key(wid,'7','Return','1','1','Return')
        time.sleep(2)
        scene=shot(wid,edition+'-appoint-scene')
        scene_ref=ROOT/f'workplace/hd-inventory/{edition}/img/DATA3/SCG05.png'
        gui.check(edition+'-normal-scene',gui.rgb(scene,'176:96:432:80')==gui.rgb(scene_ref))
        gui.key(wid,'space')
        sample(wid,edition,'order','bub.chiefOrder',False)
        gui.key(wid,'space')
        sample(wid,edition,'reply','bub.chiefReply',True)
        gui.key(wid,'space')
        for step in range(32):
            end=shot(wid,edition+f'-normal-return-{step}')
            if gui.rgb(end,'64:80:536:116')==gui.rgb(face) and gui.rgb(end,'176:16:424:300')==gui.rgb(main,'176:16:424:300'):
                gui.check(edition+'-normal-command-return',True)
                break
            gui.key(wid,'space')
        else:
            raise RuntimeError('任命後未正常返回下令停點')
        gui.receipt.setdefault('endings',[]).append(dict(edition=edition,capture=end.name,input='space'))
        gui.stop(proc)
    gui.receipt['passed']=True

try:
    run()
finally:
    gui.receipt.setdefault('passed',False)
    for proc in reversed(gui.processes): gui.stop(proc)
    for log in gui.logs: log.close()
    if OUT.exists(): (OUT/'receipt.json').write_text(json.dumps(gui.receipt,ensure_ascii=False,indent=2)+'\n')
