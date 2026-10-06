#!/usr/bin/env python3
"""Docker/Xvfb 正常六槽存檔與主選單讀回，不注入遊戲狀態。"""
import argparse
import base64
import ctypes
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out', required=True)
args = parser.parse_args()
ROOT = Path('/src')
OUT = ROOT / args.out
assert OUT.resolve().is_relative_to((ROOT / 'workplace').resolve())
assert OUT.name.startswith('hd-save-')
PACK = ROOT / 'workplace/hd-assets-mapcursor-v50-r1'
spec = importlib.util.spec_from_file_location('gui', ROOT / 'tools/verify-window-inner.py')
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = OUT
save_dir = None
start_original = gui.start
sha = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()


def start(command, tag):
    if command[0] == str(OUT / 'san1-window-check'):
        command = list(command)
        command[command.index('-saves') + 1] = str(save_dir)
        gui.receipt.setdefault('launch_commands', []).append(command)
    return start_original(command, tag)


gui.start = start


def shot(wid, tag):
    path = OUT / f'{tag}-{len(gui.receipt["captures"])}.png'
    g = gui.geo(wid)
    gui.run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y',
             '-f', 'x11grab', '-draw_mouse', '0', '-video_size', f'{g["WIDTH"]}x{g["HEIGHT"]}',
             '-i', f':99+{g["X"]},{g["Y"]}', '-frames:v', '1', '-threads', '1', str(path)])
    gui.receipt['captures'].append(dict(file=path.name, width=g['WIDTH'], height=g['HEIGHT'], sha256=sha(path)))
    return path


gui.shot = shot


def key(wid, *names):
    gui.run(['xdotool', 'windowfocus', '--sync', wid])
    for name in names:
        gui.run(['xdotool', 'keydown', '--clearmodifiers', name])
        time.sleep(.6)
        gui.run(['xdotool', 'keyup', name])
        time.sleep(.35)
        gui.receipt.setdefault('keys', []).append(name)


def visible(path):
    return gui.rgb(path, '4:1:0:0') == bytes([24, 29, 38]) * 4


def reveal(wid):
    gui.move(wid, 100, 2)
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        if visible(shot(wid, 'bar-open')):
            return
    raise RuntimeError('選項列未展開')


def hide(wid):
    if visible(shot(wid, 'bar-before-close')):
        gui.move(wid, 616, 16)
        gui.run(['xdotool', 'mousedown', '1'])
        try:
            deadline = time.monotonic() + 10
            while time.monotonic() < deadline:
                if not visible(shot(wid, 'bar-close')):
                    break
            else:
                raise RuntimeError('選項列未收起')
        finally:
            gui.run(['xdotool', 'mouseup', '1'])
    gui.move(wid, 320, 200)


def option(wid, field, row):
    reveal(wid)
    gui.choose_ready(wid, field, row)
    hide(wid)
    if field == 1:
        gui.run(['xdotool', 'windowsize', wid, '2560' if row else '640', '1632' if row else '408'])
        gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    time.sleep(.5)


def selected(wid, field, expected, tag):
    reveal(wid)
    x = [100, 270, 470][field]
    probe = [10, 202, 378][field]
    gui.move(wid, x, 16)
    gui.run(['xdotool', 'mousedown', '1'])
    try:
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            path = shot(wid, tag + '-open')
            scale = gui.geo(wid)['WIDTH'] // 640
            if gui.rgb(path, f'1:1:{probe*scale}:{34*scale}') in [bytes([44,52,65]), bytes([44,92,112])]:
                break
        else:
            raise RuntimeError('選項清單未展開')
    finally:
        gui.run(['xdotool', 'mouseup', '1'])
    # 留在上方按鈕，避免滑鼠懸停另把其他列染成選取色。
    path = shot(wid, tag + '-selected')
    rows = 6 if field == 2 else (3 if field == 0 else 2)
    colors = [gui.rgb(path, f'1:1:{probe*scale}:{(34+24*i)*scale}') for i in range(rows)]
    gui.check(tag, colors == [bytes([44,92,112]) if i == expected else bytes([44,52,65]) for i in range(rows)])
    gui.receipt.setdefault('selections', []).append(dict(file=path.name, field=field, expected=expected, rows=rows, scale=scale))
    hide(wid)


def phase_restore(original, restored):
    palette = [(0,0,0),(0,0,170),(0,170,0),(0,170,170),(170,0,0),(170,0,170),(170,85,0),(170,170,170),
               (85,85,85),(85,85,255),(85,255,85),(85,255,255),(255,85,85),(255,85,255),(255,255,85),(255,255,255)]
    before, after = gui.rgb(original), gui.rgb(restored)
    if before == after:
        return dict(complete_pixels=640*408, excluded_pixels=0, identical=True)
    projections = []
    for n, frame in enumerate(json.loads((OUT / 'main-cursors.json').read_text())):
        assert (frame['Width'], frame['Height']) == (8,16)
        sprite, mask = map(base64.b64decode, (frame['Sprite'],frame['Mask']))
        for bg in range(16):
            projections.append((n,bg,b''.join(bytes(palette[(bg&m)|s]) for s,m in zip(sprite,mask))))
    def tile(raw,x,y):
        return b''.join(raw[((y+r)*640+x)*3:((y+r)*640+x+8)*3] for r in range(16))
    solutions = []
    for y in [300,316,332,348]:
        for x in range(424,617,8):
            old = tile(before,x,y)
            for old_phase,bg,old_rgb in projections:
                if old != old_rgb:
                    continue
                for new_phase,new_bg,new_rgb in projections:
                    if bg != new_bg or tile(after,x,y) != new_rgb:
                        continue
                    expected = bytearray(before)
                    for row in range(16):
                        pos = ((y+row)*640+x)*3
                        expected[pos:pos+24] = new_rgb[row*24:(row+1)*24]
                    if expected == after:
                        solutions.append(dict(rect=[x,y,8,16], background=bg, original_phase=old_phase,
                                              restored_phase=new_phase, complete_pixels=640*408, excluded_pixels=0))
    assert len(solutions) == 1, '完整畫布或原版游標相位不符'
    return solutions[0]


def pair(wid, tag):
    option(wid,1,0)
    original = shot(wid, tag+'-original')
    option(wid,1,1)
    high = shot(wid, tag+'-hd')
    option(wid,1,0)
    restored = shot(wid, tag+'-restored')
    try:
        phase = phase_restore(original,restored)
    except AssertionError as error:
        phase = dict(complete_pixels=640*408,excluded_pixels=0,error=str(error))
    gui.receipt.setdefault('diagnostics',[]).append(dict(name=tag+'-complete-restored',passed='error' not in phase))
    gui.receipt.setdefault('pairs', []).append(dict(tag=tag, original=original.name, high=high.name,
                                                   restored=restored.name, phase=phase))
    return high


def snapshot(directory):
    result = {}
    if directory.exists():
        for path in sorted(directory.rglob('*')):
            if path.is_file():
                name = str(path.relative_to(directory))
                if path.name == 'REMAKE.JSON':
                    meta = json.loads(path.read_text())
                    meta.pop('saved_at')
                    result[name] = dict(meta=meta, sha256=sha(path))
                else:
                    result[name] = dict(bytes=path.stat().st_size, sha256=sha(path))
    return result


def normal_close(proc,wid,tag):
    # 無視窗管理器的 Xvfb：送 WM_DELETE_WINDOW，不直接銷毀 drawable。
    class ClientMessage(ctypes.Structure):
        _fields_ = [('type',ctypes.c_int),('serial',ctypes.c_ulong),('send_event',ctypes.c_int),
                    ('display',ctypes.c_void_p),('window',ctypes.c_ulong),('message_type',ctypes.c_ulong),
                    ('format',ctypes.c_int),('data',ctypes.c_long*5)]
    class Event(ctypes.Union):
        _fields_ = [('client',ClientMessage),('pad',ctypes.c_long*24)]
    x = ctypes.CDLL('libX11.so.6')
    x.XOpenDisplay.argtypes = [ctypes.c_char_p]
    x.XOpenDisplay.restype = ctypes.c_void_p
    x.XInternAtom.argtypes = [ctypes.c_void_p,ctypes.c_char_p,ctypes.c_int]
    x.XInternAtom.restype = ctypes.c_ulong
    x.XSendEvent.argtypes = [ctypes.c_void_p,ctypes.c_ulong,ctypes.c_int,ctypes.c_long,ctypes.POINTER(Event)]
    x.XFlush.argtypes = x.XCloseDisplay.argtypes = [ctypes.c_void_p]
    display = x.XOpenDisplay(None)
    assert display, 'X11 display'
    try:
        event = Event()
        event.client.type = 33
        event.client.send_event = 1
        event.client.display = display
        event.client.window = int(wid)
        event.client.message_type = x.XInternAtom(display,b'WM_PROTOCOLS',0)
        event.client.format = 32
        event.client.data[0] = x.XInternAtom(display,b'WM_DELETE_WINDOW',0)
        assert x.XSendEvent(display,int(wid),0,0,ctypes.byref(event)), '正常關閉事件未送出'
        x.XFlush(display)
    finally:
        x.XCloseDisplay(display)
    code = proc.wait(timeout=15)
    gui.check(tag+'-window-close', code == 0)
    gui.receipt.setdefault('normal_closes', []).append(dict(tag=tag,returncode=code,method='X11 WM_DELETE_WINDOW'))


def save(wid, slot, memo, tag, numeric=True):
    # 新局／讀回先問數字；同一局存檔後的下一個命令直接收鍵。
    key(wid,'9',*(['Return'] if numeric else []),'2')
    shot(wid,tag+'-six-slots')
    key(wid,str(slot))
    # 字元經 X11 實際鍵盤輸入，不以程式直接寫入備註或存檔。
    gui.run(['xdotool','type','--clearmodifiers','--delay','180',memo])
    time.sleep(.6)
    path = shot(wid,tag+'-memo')
    meta_before = snapshot(save_dir)
    gui.receipt.setdefault('save_panels', []).append(dict(tag=tag,file=path.name,slot=slot,memo=memo,
        high=gui.geo(wid)['WIDTH']==2560, before=meta_before))
    target = save_dir/f'SV{slot}/REMAKE.JSON'
    previous_write = target.stat().st_mtime_ns if target.is_file() else None
    key(wid,'Return')
    deadline = time.monotonic()+10
    while time.monotonic()<deadline:
        if target.is_file() and target.stat().st_mtime_ns != previous_write and json.loads(target.read_text())['name'].endswith(memo):
            break
        time.sleep(.2)
    else:
        raise RuntimeError('正常儲存尚未完成')
    shot(wid,tag+'-saved')
    gui.receipt.setdefault('writes',[]).append(dict(tag=tag,slot=slot,previous_mtime_ns=previous_write,
        mtime_ns=target.stat().st_mtime_ns,sha256=sha(target)))


def main():
    global save_dir
    assert (OUT.stat().st_uid,OUT.stat().st_gid)==(os.getuid(),os.getgid())
    os.environ.update(DISPLAY=':99',LIBGL_ALWAYS_SOFTWARE='1',XDG_RUNTIME_DIR='/tmp')
    start_original(['Xvfb',':99','-screen','0','2800x1900x24','-nolisten','tcp'],'xvfb')
    gui.wait(['xdotool','getdisplaygeometry'])
    gui.receipt.update(method='正式片頭→001 劉備難度5→六槽儲存→OS 關閉→片頭→主選單讀回→正常同槽再存',
        binary_sha256=sha(OUT/'san1-window-check'),pack_sha256=sha(PACK/'manifest.json'),
        state_injection=False,seed_injection=False,audio_verified=False)
    for edition in ['base','plus']:
        save_dir = OUT/('saves-'+edition)
        proc,wid = gui.launch(edition,hd_assets=PACK,tag=edition+'-new')
        key(wid,'1','1','1','1','5')
        time.sleep(2)
        path = shot(wid,edition+'-new-main')
        gui.check(edition+'-normal-lord',gui.rgb(path,'64:80:536:116')==gui.rgb(ROOT/'workplace/hd-inventory/base/img/DATA3/F005.png'))
        for slot in range(1,7):
            locale = (slot-1)//2
            high = slot%2==0
            option(wid,0,locale)
            option(wid,2,slot-1)
            option(wid,1,int(high))
            memo = ('B' if edition=='base' else 'P')+f'61S{slot:02d}'
            tag = f'{edition}-save-{slot}'
            save(wid,slot,memo,tag,numeric=slot==1)
            meta = json.loads((save_dir/f'SV{slot}/REMAKE.JSON').read_text())
            gui.check(tag+'-ai-mode',meta['options']['ai_mode']==(edition if slot==1 else 'enhanced'))
            gui.check(tag+'-ai-orders',1<=meta['options']['ai_orders']<=5 and (slot==1 or meta['options']['ai_orders']==slot-1))
            gui.check(tag+'-no-window-options', 'theme' not in meta and 'language' not in meta and 'locale' not in meta)
        baseline = snapshot(save_dir)
        gui.receipt.setdefault('baselines',{})[edition] = baseline
        gui.check(edition+'-six-complete-slots',len(baseline)==37)
        normal_close(proc,wid,edition+'-new')
        # 六個 AI 選項已由正常儲存及資料層回歸全驗；重啟抽測三語各一槽。
        for slot in [1,4,6]:
            source = OUT/('saves-'+edition)
            save_dir = OUT/f'reload-{edition}-{slot}'
            shutil.copytree(source,save_dir)
            gui.check(f'{edition}-{slot}-exact-player-save-copy',snapshot(save_dir)==baseline)
            tag = f'{edition}-reload-{slot}'
            proc,wid = gui.launch(edition,hd_assets=PACK,tag=tag)
            selected(wid,1,0,tag+'-original-default')
            selected(wid,0,0,tag+'-language-default')
            option(wid,0,(slot-1)//2)
            conflict = 5 if slot<6 else 1
            option(wid,2,conflict)
            selected(wid,2,conflict,tag+'-conflicting-title-ai')
            key(wid,'2')
            shot(wid,tag+'-load-menu')
            key(wid,str(slot))
            time.sleep(1)
            selected(wid,2,slot-1,tag+'-saved-ai-restored')
            selected(wid,0,(slot-1)//2,tag+'-current-language-retained')
            selected(wid,1,0,tag+'-current-original-retained')
            high = pair(wid,tag)
            gui.check(tag+'-native-portrait',gui.rgb(high,'256:320:2144:464')==gui.rgb(PACK/'F005.png'))
            option(wid,1,int(slot%2==0))
            memo = ('B' if edition=='base' else 'P')+f'61S{slot:02d}'
            save(wid,slot,memo,tag+'-resave')
            after = snapshot(save_dir)
            functional = lambda table: {k:value['meta'] for k,value in table.items() if 'meta' in value}
            gui.check(tag+'-functional-resave-same',functional(after)==functional(baseline))
            canonical = lambda table: {k:({n:v for n,v in value.items() if n!='sha256'} if 'meta' in value else value) for k,value in table.items()}
            gui.receipt.setdefault('diagnostics',[]).append(dict(name=tag+'-complete-bytes-same',passed=canonical(after)==canonical(baseline)))
            gui.receipt.setdefault('roundtrips',[]).append(dict(tag=tag,edition=edition,slot=slot,locale=['zh-Hant','en','ja'][(slot-1)//2],
                high=slot%2==0,conflicting_ai=conflict,before=baseline,after=after,save_dir=save_dir.name))
            normal_close(proc,wid,tag)
        gui.check(edition+'-baseline-unchanged',snapshot(source)==baseline)
    gui.receipt['passed'] = True


try:
    main()
finally:
    gui.receipt.setdefault('passed',False)
    for proc in reversed(gui.processes):
        gui.stop(proc)
    for log in gui.logs:
        log.close()
    (OUT/'receipt.json').write_text(json.dumps(gui.receipt,ensure_ascii=False,indent=2)+'\n')
