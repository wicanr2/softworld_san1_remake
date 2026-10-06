#!/usr/bin/env python3
"""正常新局宣戰的完整三段語音；只在限時 Docker 內執行。"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import signal
import time
import wave

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out', required=True)
parser.add_argument('--reference', required=True)
parser.add_argument('--binary', required=True)
parser.add_argument('--title-reference', required=True)
parser.add_argument('--editions', nargs='+', choices=['base','plus'], default=['base','plus'])
parser.add_argument('--locales', nargs='+', choices=['zh-Hant','en','ja'], default=['zh-Hant','en','ja'])
args = parser.parse_args()
ROOT = Path('/src')
OUT = ROOT / args.out
REFERENCE = ROOT / args.reference
PACK = ROOT / 'workplace/hd-assets-mapcursor-v50-r1'
assert OUT.resolve().is_relative_to((ROOT/'workplace/audio').resolve())
assert len(args.editions)==len(set(args.editions)) and len(args.locales)==len(set(args.locales))
spec=importlib.util.spec_from_file_location('gui',ROOT/'tools/verify-window-inner.py')
gui=importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT=OUT
original_start=gui.start

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def start(command, name):
    if command[0]==str(OUT/'san1-window-check'):
        command=['-sound=true' if x=='-sound=false' else x for x in command]
        gui.receipt.setdefault('launch_commands',[]).append(command)
    return original_start(command,name)

gui.start=start

def shot(wid,name):
    path=OUT/(name+'-'+str(len(gui.receipt['captures']))+'.png')
    g=gui.geo(wid)
    gui.run(['ffmpeg','-nostdin','-hide_banner','-loglevel','error','-y',
        '-f','x11grab','-draw_mouse','0','-video_size',f'{g["WIDTH"]}x{g["HEIGHT"]}',
        '-i',f':99+{g["X"]},{g["Y"]}','-frames:v','1','-threads','1',str(path)])
    gui.receipt['captures'].append(dict(file=path.name,width=g['WIDTH'],height=g['HEIGHT'],sha256=sha(path)))
    return path

gui.shot=shot

def hidden(wid):
    gui.move(wid,320,200)
    gui.key(wid,'Escape')
    gui.dimensions(wid,'選項列已收起',640,408)

def language(wid,row):
    gui.key(wid,'Escape')
    gui.choose_ready(wid,0,row)
    hidden(wid)

def theme(wid,high):
    gui.key(wid,'Escape')
    gui.choose_ready(wid,1,int(high))
    gui.move(wid,320,200)
    gui.key(wid,'Escape')
    gui.run(['xdotool','windowsize',wid,'2560' if high else '640','1632' if high else '408'])
    gui.run(['xdotool','windowmove',wid,'0','0'])
    gui.move(wid,320,200)
    time.sleep(.4)

def record(tag):
    path=OUT/(tag+'.wav')
    proc=original_start(['ffmpeg','-nostdin','-hide_banner','-loglevel','error','-y',
        '-f','pulse','-sample_rate','48000','-channels','2','-fragment_size','4096',
        '-i','san1.monitor','-c:a','pcm_s16le','-flush_packets','1',str(path)],tag+'-record')
    deadline=time.monotonic()+8
    while time.monotonic()<deadline:
        assert proc.poll() is None
        if path.exists() and path.stat().st_size>2000:return proc,path
        time.sleep(.05)
    raise RuntimeError('錄音尚未收到第一個封包')

def finish(recording,expected,tag):
    proc,path=recording
    proc.send_signal(signal.SIGINT)
    proc.wait(timeout=15)
    with wave.open(str(path)) as wav:
        assert (wav.getnchannels(),wav.getframerate(),wav.getsampwidth())==(2,48000,2)
        pcm=wav.readframes(wav.getnframes())
    start=pcm.find(expected)
    # 比較整段參考，並要求其前後只有靜音。不能以短片段或挑幀驗收。
    exact=start>=0 and start%4==0 and not any(pcm[:start]) and not any(pcm[start+len(expected):])
    entry=dict(file=path.name,sha256=sha(path),frames=len(pcm)//4,expected_frames=len(expected)//4,
               start_frame=start//4 if start>=0 else None,complete_exact=exact,excluded_frames=0)
    gui.receipt.setdefault('voice_wavs',[]).append(entry)
    gui.check(tag+'-complete-three-clips-once',exact)

def await_picture(wid,tag,picture,crop,advance=False):
    target=gui.rgb(picture)
    deadline=time.monotonic()+45
    while time.monotonic()<deadline:
        path=gui.shot(wid,tag+'-await')
        if gui.rgb(path,crop)==target:return path
        if advance:gui.key(wid,'space')
        else:time.sleep(.2)
    raise RuntimeError(tag+' 尚未顯示指定正常畫面')

def run():
    if OUT.exists():raise RuntimeError('輸出已存在，拒絕覆寫')
    OUT.mkdir()
    assert (OUT.stat().st_uid,OUT.stat().st_gid)==(os.getuid(),os.getgid())
    for src,name in [(ROOT/args.binary,'san1-window-check'),(ROOT/args.title_reference,'title-reference.png'),
                     (Path(__file__),Path(__file__).name),(ROOT/'tools/verify-window-inner.py','verify-window-inner.py')]:
        shutil.copy2(src,OUT/name)
    runtime=Path('/tmp/san1-voice')
    runtime.mkdir(mode=0o700)
    os.environ.update(DISPLAY=':99',LIBGL_ALWAYS_SOFTWARE='1',XDG_RUNTIME_DIR=str(runtime),PULSE_SERVER=f'unix:{runtime}/native')
    original_start(['Xvfb',':99','-screen','0','2800x1900x24','-nolisten','tcp'],'xvfb')
    gui.wait(['xdotool','getdisplaygeometry'])
    original_start(['pulseaudio','-n','--daemonize=no','--exit-idle-time=-1',
        '--load=module-null-sink sink_name=san1 rate=48000 channels=2',
        f'--load=module-native-protocol-unix socket={runtime}/native auth-anonymous=1'],'pulse')
    gui.wait(['pactl','info'])
    gui.run(['pactl','set-default-sink','san1'])
    refs=json.loads((REFERENCE/'reference.json').read_text())
    gui.receipt.update(method='正常片頭、001 劉備難度5、語音開關、齊郡出兵北海；完整三段 PCM 與 Theme 回切',
        binary_sha256=sha(OUT/'san1-window-check'),pack_sha256=sha(PACK/'manifest.json'),
        reference_sha256=sha(REFERENCE/'reference.json'),state_injection=False,seed_injection=False,
        human_listening=False,selection=dict(editions=args.editions,locales=args.locales),scope='兩則已證實對白；取樣率為 remake 近似')
    for edition in args.editions:
        for locale in args.locales:
            tag=edition+'-'+locale
            proc,wid=gui.launch(edition,hd_assets=PACK,tag=tag)
            gui.key(wid,'1','1','1','1','5')
            time.sleep(1)
            language(wid,['zh-Hant','en','ja'].index(locale))
            # 現行 remake 零值語音開啟；先關再開，明示正常開關路徑。
            gui.key(wid,'9','Return','8') # 主命令需要 Enter，子選單收單鍵。
            gui.shot(wid,tag+'-voice-disabled')
            gui.key(wid,'9','8')
            gui.shot(wid,tag+'-voice-enabled')
            # 開關提示後是直接命令；再送 Enter 會結束這個郡的回合。
            gui.key(wid,'2','2','8','Return','7','Return',
                '1','Return','1','Return','Return','y','0','Return','1','0','0','Return')
            scene=ROOT/'workplace/hd-inventory'/edition/'img/DATA3/SCG06.png'
            await_picture(wid,tag+'-war-scene',scene,'176:96:432:80',advance=True)
            time.sleep(.6) # 讓既有拉幕音效完成，語音仍尚未顯示。
            for number,crop in [(1,'64:80:552:80'),(2,'64:80:424:180')]:
                ref=refs[edition][str(number)]
                expected=(REFERENCE/ref['file']).read_bytes()
                rec=record(tag+'-'+str(number))
                gui.key(wid,'space')
                source=ROOT/'workplace/hd-inventory'/edition/'img/DATA3'/ref['portrait']
                if number==2:
                    # 左側肖像必須左右翻面。ffmpeg 僅用於讀取比較，不修改素材。
                    target=gui.rgb(source,'64:80:0:0,hflip')
                    path=gui.shot(wid,tag+'-reply-original')
                    gui.check(tag+'-reply-raw-portrait',gui.rgb(path,crop)==target)
                else:
                    await_picture(wid,tag+'-declare',source,crop)
                original=gui.shot(wid,tag+'-'+str(number)+'-original')
                theme(wid,True)
                high=gui.shot(wid,tag+'-'+str(number)+'-hd')
                theme(wid,False)
                restored=gui.shot(wid,tag+'-'+str(number)+'-restored')
                gui.check(tag+'-'+str(number)+'-whole-source-restored',gui.rgb(original)==gui.rgb(restored))
                gui.receipt.setdefault('samples',[]).append(dict(edition=edition,locale=locale,number=number,
                    original=original.name,high=high.name,restored=restored.name))
                time.sleep(len(expected)/192000+1)
                finish(rec,expected,tag+'-'+str(number))
            gui.stop(proc)
    gui.receipt['passed']=True

try:
    run()
finally:
    gui.receipt.setdefault('passed',False)
    for proc in reversed(gui.processes):gui.stop(proc)
    for log in gui.logs:log.close()
    if OUT.exists():(OUT/'receipt.json').write_text(json.dumps(gui.receipt,ensure_ascii=False,indent=2)+'\n')
