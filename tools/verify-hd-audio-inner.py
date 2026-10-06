#!/usr/bin/env python3
"""正常 GUI 的配樂連續性與音效；PCM、錄音與完整截圖只留本機。"""
import argparse
import array
import base64
import hashlib
import importlib.util
import json
import math
import os
from pathlib import Path
import shutil
import signal
import time
import wave

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out', required=True)
parser.add_argument('--binary', required=True)
parser.add_argument('--title-reference', required=True)
parser.add_argument('--reference', default='workplace/hd-audio-check-build')
parser.add_argument('--scope', choices=['all','music','sound'], default='all')
parser.add_argument('--editions', nargs='+', choices=['base','plus'], default=['base','plus'])
parser.add_argument('--locales', nargs='+', choices=['zh-Hant','en','ja'], default=['zh-Hant','en','ja'])
args = parser.parse_args()
ROOT = Path('/src')
OUT = ROOT / args.out
assert OUT.resolve().is_relative_to((ROOT/'workplace/audio').resolve())
assert OUT.resolve() != (ROOT/'workplace/audio').resolve()
assert len(args.editions)==len(set(args.editions)) and len(args.locales)==len(set(args.locales))
REFERENCE=ROOT/args.reference
PACK = ROOT / 'workplace/hd-assets-mapcursor-v50-r1'
spec = importlib.util.spec_from_file_location('gui', ROOT/'tools/verify-window-inner.py')
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = OUT
mode = 'music'
start_original = gui.start

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def cpu_runtime():
    result = dict(affinity_cpus=len(os.sched_getaffinity(0)),
                  environment={name:os.environ.get(name) for name in
                               ['GOMAXPROCS','LP_NUM_THREADS','OMP_NUM_THREADS']})
    for name in ['cpu.max','cpu.stat','cpu.pressure']:
        path = Path('/sys/fs/cgroup')/name
        result[name] = path.read_text() if path.is_file() else None
    return result

def start(command, tag):
    if command[0] == str(OUT/'san1-window-check'):
        command = [('-music=' + str(mode == 'music').lower()) if s == '-music=false'
                   else ('-sound=' + str(mode == 'sound').lower()) if s == '-sound=false'
                   else s for s in command]
        gui.receipt.setdefault('launch_commands', []).append(command)
    return start_original(command, tag)

gui.start = start

def shot(wid, tag):
    path = OUT / f'{tag}-{len(gui.receipt["captures"])}.png'
    g = gui.geo(wid)
    focus = gui.run(['xdotool','getwindowfocus'])
    sink = gui.run(['pactl','list','sink-inputs'])
    gui.receipt.setdefault('device_samples',[]).append(dict(tag=tag,focus=focus,game_window=wid,
        corked=[line.strip() for line in sink.splitlines() if 'Corked:' in line]))
    gui.run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y',
             '-f', 'x11grab', '-draw_mouse', '0', '-video_size', f'{g["WIDTH"]}x{g["HEIGHT"]}',
             '-i', f':99+{g["X"]},{g["Y"]}', '-frames:v', '1', '-threads', '1', str(path)])
    gui.receipt['captures'].append(dict(file=path.name, width=g['WIDTH'], height=g['HEIGHT'], sha256=sha(path)))
    return path

gui.shot = shot

def phase_restore(original, restored):
    palette=[(0,0,0),(0,0,170),(0,170,0),(0,170,170),(170,0,0),(170,0,170),(170,85,0),(170,170,170),
             (85,85,85),(85,85,255),(85,255,85),(85,255,255),(255,85,85),(255,85,255),(255,255,85),(255,255,255)]
    frames=json.loads((REFERENCE/'main-cursors.json').read_text())
    before,after=gui.rgb(original),gui.rgb(restored)
    assert len(before)==len(after)==640*408*3
    projections=[]
    for n,frame in enumerate(frames):
        assert (frame['Width'],frame['Height'])==(8,16)
        sprite,mask=map(base64.b64decode,(frame['Sprite'],frame['Mask']))
        for bg in range(16):
            projections.append((n,bg,b''.join(bytes(palette[(bg&m)|s]) for s,m in zip(sprite,mask))))
    def tile(raw,x,y):
        return b''.join(raw[((y+r)*640+x)*3:((y+r)*640+x+8)*3] for r in range(16))
    solutions=[]
    for y in [300,316,332,348]:
        for x in range(424,617,8):
            old=tile(before,x,y)
            for old_phase,bg,old_rgb in projections:
                if old!=old_rgb:continue
                for new_phase,new_bg,new_rgb in projections:
                    if bg!=new_bg or tile(after,x,y)!=new_rgb:continue
                    expected=bytearray(before)
                    for row in range(16):
                        pos=((y+row)*640+x)*3
                        expected[pos:pos+24]=new_rgb[row*24:(row+1)*24]
                    if expected==after:
                        solutions.append(dict(rect=[x,y,8,16],background=bg,original_phase=old_phase,
                                              restored_phase=new_phase,complete_pixels=640*408,excluded_pixels=0))
    assert len(solutions)==1, '整張畫布或來源游標相位不符'
    return solutions[0]

def record(tag):
    path = OUT/(tag+'.wav')
    proc = start_original(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y',
                          '-f', 'pulse', '-sample_rate', '48000', '-channels', '2',
                          '-fragment_size', '4096', '-i', 'san1.monitor',
                          '-c:a', 'pcm_s16le', '-flush_packets', '1', str(path)], tag+'-record')
    deadline = time.monotonic()+8
    while time.monotonic()<deadline:
        assert proc.poll() is None, '錄音程式提早結束'
        if path.exists() and path.stat().st_size>2000:break
        time.sleep(.05)
    else: raise RuntimeError('錄音尚未收到第一個音訊封包')
    return proc, path

def finish(recording, kind, **extra):
    proc, path = recording
    proc.send_signal(signal.SIGINT)
    proc.wait(timeout=15)
    with wave.open(str(path)) as wav:
        assert (wav.getnchannels(), wav.getframerate(), wav.getsampwidth()) == (2,48000,2)
        pcm = wav.readframes(wav.getnframes())
    samples = array.array('h', pcm)
    assert samples, '錄音沒有音訊樣本'
    peak = max(abs(v) for v in samples)
    rms = math.sqrt(sum(v*v for v in samples)/len(samples))
    db = lambda v: 20*math.log10(v/32768) if v else -999
    entry = dict(file=path.name, kind=kind, seconds=len(pcm)/192000,
                 sha256=sha(path), rms_dbfs=db(rms), peak_dbfs=db(peak), **extra)
    gui.receipt.setdefault('audio', []).append(entry)
    if kind == 'silent':
        gui.check(path.stem+'-zero-output', peak <= 2)
    else:
        gui.check(path.stem+'-audible', db(rms)>-60 and db(peak)>-45)
    return entry

def bar_is_visible(path):
    return gui.rgb(path,'4:1:0:0') == bytes([24,29,38])*4

def reveal_bar(wid, tag):
    gui.move(wid,100,2)
    deadline=time.monotonic()+10
    while time.monotonic()<deadline:
        path=shot(wid,tag+'-bar-open')
        if bar_is_visible(path):return
    raise RuntimeError('選項列尚未展開')

def hide_bar(wid, tag):
    path=shot(wid,tag+'-before-close')
    if not bar_is_visible(path):
        gui.move(wid,320,200)
        return
    gui.move(wid,616,16)
    gui.run(['xdotool','mousedown','1'])
    try:
        deadline=time.monotonic()+10
        while time.monotonic()<deadline:
            path=shot(wid,tag+'-bar-close')
            if not bar_is_visible(path):
                gui.receipt.setdefault('toolbar_close',[]).append(path.name)
                break
        else:raise RuntimeError('選項列尚未收起，不送出遊戲指令')
    finally:
        gui.run(['xdotool','mouseup','1'])
    gui.move(wid,320,200)

def theme(wid, high):
    reveal_bar(wid,'theme')
    gui.choose_ready(wid,1,int(high))
    hide_bar(wid,'theme')
    gui.run(['xdotool','windowsize',wid,'2560' if high else '640','1632' if high else '408'])
    gui.run(['xdotool','windowmove',wid,'0','0'])
    gui.move(wid,320,200)
    time.sleep(.5)

def language(wid, row):
    reveal_bar(wid,'language')
    gui.choose_ready(wid,0,row)
    hide_bar(wid,'language')
    gui.move(wid,320,200)
    time.sleep(.4)

def action_key(wid, high, *names):
    if not high:
        gui.key(wid,*names)
        return
    # 保持實體按住／放開，沒有狀態注入；收列另外以畫面確認。
    gui.run(['xdotool','windowfocus','--sync',wid])
    for name in names:
        gui.run(['xdotool','keydown','--clearmodifiers',name])
        time.sleep(.6)
        gui.run(['xdotool','keyup',name])
        time.sleep(.35)
        gui.receipt.setdefault('keys',[]).append(name)
        gui.receipt.setdefault('held_keys',[]).append(dict(key=name,seconds=.6,window=wid))

def main(wid, tag):
    gui.key(wid,'1','1','1','1','5')
    time.sleep(2)
    path = shot(wid,tag+'-main')
    source = ROOT/'workplace/hd-inventory/base/img/DATA3/F005.png'
    gui.check(tag+'-normal-lord',gui.rgb(path,'64:80:536:116')==gui.rgb(source))

def pair(wid, tag):
    original = shot(wid,tag+'-original')
    theme(wid,True)
    high = shot(wid,tag+'-hd')
    gui.check(tag+'-native-portrait',gui.rgb(high,'256:320:2144:464')==gui.rgb(PACK/'F005.png'))
    theme(wid,False)
    restored = shot(wid,tag+'-restored')
    phase=phase_restore(original,restored)
    gui.check(tag+'-whole-source-phase-restored',True)
    gui.receipt.setdefault('samples',[]).append(dict(tag=tag,original=original.name,high=high.name,restored=restored.name))
    gui.receipt['samples'][-1]['source_phase_proof'] = phase

def run():
    global mode
    if OUT.exists(): raise RuntimeError('輸出已存在，拒絕覆蓋證據')
    OUT.mkdir(parents=True)
    assert (OUT.stat().st_uid,OUT.stat().st_gid)==(os.getuid(),os.getgid())
    shutil.copy2(ROOT/args.binary, OUT/'san1-window-check')
    shutil.copy2(ROOT/args.title_reference, OUT/'title-reference.png')
    shutil.copy2(Path(__file__), OUT/Path(__file__).name)
    shutil.copy2(ROOT/'tools/verify-window-inner.py',OUT/'verify-window-inner.py')
    runtime = Path('/tmp/san1-hd-audio')
    runtime.mkdir(mode=0o700)
    os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR=str(runtime),
                      PULSE_SERVER=f'unix:{runtime}/native')
    start_original(['Xvfb',':99','-screen','0','2800x1900x24','-nolisten','tcp'],'xvfb')
    gui.wait(['xdotool','getdisplaygeometry'])
    start_original(['pulseaudio','-n','--daemonize=no','--exit-idle-time=-1',
                    '--load=module-null-sink sink_name=san1 rate=48000 channels=2',
                    f'--load=module-native-protocol-unix socket={runtime}/native auth-anonymous=1'],'pulse')
    gui.wait(['pactl','info'])
    gui.run(['pactl','set-default-sink','san1'])
    gui.receipt.update(method='正常片頭、新局、選項列、音效開關與軍師任命；PulseAudio monitor',
                       binary_sha256=sha(OUT/'san1-window-check'),pack_sha256=sha(PACK/'manifest.json'),
                       state_injection=False,seed_injection=False,human_listening=False,
                       scope='所選配樂／音效的輸出；本工具不執行語音驗收')
    gui.receipt['reference_files']={name:sha(REFERENCE/name) for name in ['main-cursors.json','music-reference.pcm','sfx-reference.pcm']}
    gui.receipt['selection']=dict(scope=args.scope,editions=args.editions,locales=args.locales)
    gui.receipt['cpu_runtime']=dict(before=cpu_runtime())
    for edition in args.editions:
        if args.scope in ['all','music']:
            mode='music'
            proc,wid=gui.launch(edition,hd_assets=PACK,tag=edition+'-music')
            main(wid,edition+'-music')
            idle=record(edition+'-music-idle')
            time.sleep(3)
            finish(idle,'music',edition=edition,control='原貌閒置，沒有 Theme 或語言操作')
            rec=record(edition+'-music-continuous')
            for locale in args.locales:
                language(wid,['zh-Hant','en','ja'].index(locale))
                pair(wid,edition+'-music-'+locale)
            time.sleep(2)
            finish(rec,'music',edition=edition)
            gui.stop(proc)
        if args.scope=='music':continue
        for locale in args.locales:
            row=['zh-Hant','en','ja'].index(locale)
            mode='sound'
            tag=edition+'-sound-'+locale
            proc,wid=gui.launch(edition,hd_assets=PACK,tag=tag)
            main(wid,tag)
            language(wid,row)
            for high in [False,True]:
                if high: theme(wid,True)
                suffix=tag+('-hd' if high else '-original')
                # 主提示最初為數字輸入；開關提示之後為直接命令。
                action_key(wid,high,'9',*(['Return'] if not high else []),'4')
                off=record(suffix+'-off')
                time.sleep(1)
                finish(off,'silent',edition=edition,locale=locale,high=high)
                on=record(suffix+'-on')
                action_key(wid,high,'9','4')
                time.sleep(1)
                shot(wid,suffix+'-enabled')
                finish(on,'sfx',edition=edition,locale=locale,high=high,expected_clips=1)
            rec=record(tag+'-transition')
            action_key(wid,True,'7','1','1','Return')
            master=gui.rgb(PACK/'SCG05.png')
            deadline=time.monotonic()+45
            while time.monotonic()<deadline:
                path=shot(wid,tag+'-appoint')
                if gui.rgb(path,'704:384:1728:320')==master:break
            else: raise RuntimeError('正常任命場景未完成')
            time.sleep(.5)
            finish(rec,'sfx',edition=edition,locale=locale,high=True,expected_clip_counts=[22,24])
            gui.check(tag+'-normal-native-scene',True)
            gui.stop(proc)
    gui.receipt['passed']=True

try:
    run()
finally:
    gui.receipt.setdefault('passed',False)
    for proc in reversed(gui.processes):gui.stop(proc)
    for log in gui.logs:log.close()
    if 'cpu_runtime' in gui.receipt:gui.receipt['cpu_runtime']['after']=cpu_runtime()
    if OUT.exists():(OUT/'receipt.json').write_text(json.dumps(gui.receipt,ensure_ascii=False,indent=2)+'\n')
