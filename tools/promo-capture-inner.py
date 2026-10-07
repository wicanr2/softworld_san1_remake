#!/usr/bin/env python3
"""Docker 內從正式 Linux 完整版錄製正常新局、人物卡及戰場 Theme 切換。"""
import argparse
import ast
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out', required=True, type=Path)
parser.add_argument('--package', required=True, type=Path)
args = parser.parse_args()
ROOT, OUT, PACKAGE = Path('/src'), args.out, args.package
spec = importlib.util.spec_from_file_location('gui', ROOT / 'tools/verify-window-inner.py')
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = OUT
pack = PACKAGE / 'hd-assets'
sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
start_original = gui.start

def start(command, tag):
    if command[0] == str(OUT / 'san1-window-check'):
        command = list(command)
        command[0] = str(PACKAGE / 'san1')
        edition = command[command.index('-edition')+1]
        command[command.index('-root')+1] = str(PACKAGE / 'game' / edition)
        command[command.index('-saves')+1] = '/tmp/promo-saves-' + edition
        gui.receipt.setdefault('launch_commands', []).append(command)
    return start_original(command, tag)

gui.start = start
poll_dir = tempfile.TemporaryDirectory(prefix='san1-promo-polls-')

def shot(wid, tag):
    # 輪詢和下拉選單影格不留重複工作樹檔案。
    p = Path(poll_dir.name) / (tag + '.png')
    g = gui.geo(wid)
    gui.run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y', '-f', 'x11grab',
             '-video_size', f"{g['WIDTH']}x{g['HEIGHT']}", '-i', f":99+{g['X']},{g['Y']}",
             '-frames:v', '1', '-threads', '1', str(p)])
    return p

gui.shot = shot
route = ROOT / 'tools/verify-hd-battle-branches-inner.py'
helpers = {'flip', 'source_face', 'await_prompt', 'reach_attack_turn'}
nodes = [n for n in ast.parse(route.read_text()).body if isinstance(n, ast.FunctionDef) and n.name in helpers]
exec(compile(ast.Module(body=nodes, type_ignores=[]), str(route), 'exec'), globals())

def keep(wid, tag):
    import shutil
    p = OUT / (tag + '.png')
    shutil.copyfile(shot(wid, tag), p)
    gui.receipt['captures'].append({'file': p.name, 'sha256': sha(p), 'geometry': gui.geo(wid)})
    return p

def demo(wid, name, languages=False):
    gui.run(['xdotool', 'windowsize', wid, '1280', '816'])
    gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    gui.move(wid, 320, 200)
    time.sleep(.5)
    keep(wid, name + '-original')
    clip = OUT / (name + '.mp4')
    rec = gui.start(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y',
                     '-f', 'x11grab', '-framerate', '15', '-video_size', '1280x880', '-draw_mouse', '1',
                     '-i', ':99', '-an', '-c:v', 'libx264', '-threads', '2', '-preset', 'veryfast',
                     '-crf', '18', '-pix_fmt', 'yuv420p', str(clip)], name + '-capture')
    begin = time.monotonic()
    events = []
    try:
        time.sleep(1)
        gui.key(wid, 'Escape')
        gui.dimensions(wid, name + '-options-visible', 1280, 880)
        events.append({'at': round(time.monotonic()-begin,3), 'action': 'Theme：B 高清'})
        gui.choose_ready(wid, 1, 1)
        time.sleep(1.2)
        keep(wid, name + '-hd-options')
        if languages:
            for row, locale in [(1,'en'), (2,'ja'), (0,'zh-Hant')]:
                events.append({'at': round(time.monotonic()-begin,3), 'action': '語言：' + locale})
                gui.choose_ready(wid, 0, row)
                time.sleep(.8)
        events.append({'at': round(time.monotonic()-begin,3), 'action': 'Theme：原貌'})
        gui.choose_ready(wid, 1, 0)
        time.sleep(.8)
        gui.key(wid, 'Escape')
        gui.move(wid, 320, 200)
        keep(wid, name + '-restored')
        time.sleep(.8)
    finally:
        rec.send_signal(2)
        rec.wait(timeout=10)
    assert rec.returncode == 0
    probe = json.loads(gui.run(['ffprobe','-v','error','-show_format','-of','json',str(clip)]))
    gui.receipt.setdefault('clips',[]).append({'file':clip.name,'sha256':sha(clip),
        'duration':float(probe['format']['duration']),'events':events,'normal_input':True})

assert OUT.is_dir() and OUT.stat().st_uid == os.getuid()
assert not (OUT/'receipt.json').exists()
try:
    os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp', LP_NUM_THREADS='2')
    gui.receipt.update(method='正式 Linux 完整版的正常玩家 GUI 錄影；沒有直入、seed、clock 或狀態注入',
                       binary_sha256=sha(PACKAGE/'san1'), manifest_sha256=sha(pack/'manifest.json'),
                       sources_sha256={str(p.relative_to(ROOT)):sha(p) for p in
                           [Path(__file__).resolve(), route, ROOT/'tools/verify-window-inner.py']},
                       rights='local_only_original_derived_art', passed=False)
    gui.start(['Xvfb',':99','-screen','0','2800x1900x24','-nolisten','tcp','-noreset','-ac'],'xvfb')
    gui.wait(['xdotool','getdisplaygeometry'])
    actual_version = gui.run([str(PACKAGE/'san1'), '-version'])
    assert actual_version == os.environ['SAN1_PROMO_VERSION']
    gui.receipt['version'] = actual_version
    proc,wid = gui.launch('base',hd_assets=pack,tag='promo-cards')
    demo(wid,'title')
    gui.key(wid,'1','1','1','2','5','0','Return')
    gui.key(wid,'1','1','1','1','Return','1','3','1','Return')
    demo(wid,'main-card',languages=True)
    gui.stop(proc)
    proc,wid = gui.launch('base',hd_assets=pack,tag='promo-battle')
    gui.key(wid,'1','1','1','6','5')
    waiting,rested = reach_attack_turn(wid,'promo-battle','base')
    gui.key(wid,'2','Return','2')
    if waiting == 14: gui.key(wid,'1','5','Return')
    gui.key(wid,'1','1','Return','2','Return','1','Return','Return','y',
            '0','Return','1','0','0','0','Return')
    await_prompt(wid,'promo-camp','camp',2)
    gui.key(wid,'3','3','6','3','0')
    await_prompt(wid,'promo-command','command',3)
    demo(wid,'battle')
    gui.receipt['passed'] = True
finally:
    for p in reversed(gui.processes): gui.stop(p)
    for log in gui.logs: log.close()
    poll_dir.cleanup()
    (OUT/'receipt.json').write_text(json.dumps(gui.receipt,ensure_ascii=False,indent=2)+'\n')
