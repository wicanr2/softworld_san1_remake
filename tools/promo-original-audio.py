#!/usr/bin/env python3
"""在 DOSBox-X 容器錄製原版音樂欣賞的風雲。收據只留本機。"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out', type=Path, required=True)
args = parser.parse_args()
out = args.out
assert out.is_dir() and out.stat().st_uid == os.getuid() and out.stat().st_gid == os.getgid()
assert not (out / 'receipt.json').exists()
processes, logs = [], []
receipt = {'method': 'DOSBox-X 原版 AA.EXE 正常開機／音樂欣賞／風雲，F12+W 內部 WAV 擷取',
           'hotkey_reference': 'https://github.com/joncampbell123/dosbox-x/wiki',
           'rights': 'local_only_original_music', 'oracle': False, 'keys': [], 'passed': False}
sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()

def run(command):
    return subprocess.run(command, check=True, capture_output=True, text=True, timeout=15).stdout.strip()

def start(command, name):
    log = (out / (name + '.log')).open('x')
    logs.append(log)
    p = subprocess.Popen(command, stdout=log, stderr=subprocess.STDOUT)
    processes.append(p)
    return p

def key(name, seconds=1):
    # SDL1 換顯示模式會重建視窗；每次查目前可見視窗並重新設焦點。
    wid = run(['xdotool', 'search', '--onlyvisible', '--name', 'DOSBox-X']).splitlines()[-1]
    run(['xdotool', 'windowfocus', '--sync', wid])
    run(['xdotool', 'keydown', '--clearmodifiers', name])
    time.sleep(.15)
    run(['xdotool', 'keyup', name])
    receipt['keys'].append(name)
    time.sleep(seconds)

def shot(name):
    p = out / (name + '.png')
    run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y', '-f', 'x11grab',
         '-video_size', '640x408', '-i', ':99', '-frames:v', '1', '-threads', '1', str(p)])
    receipt.setdefault('captures', []).append({'file': p.name, 'sha256': sha(p)})
    return p

try:
    os.environ.update(HOME='/tmp', DISPLAY=':99', SDL_AUDIODRIVER='dummy')
    source = Path('/orig/三國演義')
    receipt['original_sha256'] = {p.name: sha(p) for p in sorted(source.iterdir()) if p.is_file()}
    tool = subprocess.run(['dosbox-x', '-version'], capture_output=True, text=True)
    receipt['tool_version'] = (tool.stdout + tool.stderr).strip()
    shutil.copytree(source, '/tmp/game')
    config = f'''[sdl]
output=surface
autolock=false
[dosbox]
machine=svga_s3
memsize=16
captures={out}
[render]
aspect=false
scaler=none
[cpu]
core=normal
cputype=386
cycles=fixed 60000
[mixer]
nosound=false
rate=44100
[sblaster]
sbtype=sb16
oplmode=opl2
oplemu=nuked
oplrate=44100
[autoexec]
mount c /tmp/game
c:
AA.EXE
'''
    (out / 'dosbox.conf').write_text(config)
    start(['Xvfb', ':99', '-screen', '0', '1280x1024x24', '-nolisten', 'tcp'], 'xvfb')
    time.sleep(1)
    start(['dosbox-x', '-conf', str(out / 'dosbox.conf'), '-nomenu'], 'dosbox')
    time.sleep(8)
    wid = run(['xdotool', 'search', '--name', 'DOSBox-X']).splitlines()[-1]
    run(['xdotool', 'windowfocus', '--sync', wid])
    shot('device-options')
    key('2'); key('2'); key('2', 2)
    for _ in range(90):
        key('Return', 1)
    main_menu = shot('original-main-menu')
    key('5'); music_menu = shot('original-music-menu')
    assert sha(main_menu) != sha(music_menu), '音樂欣賞選單未開啟，拒絕把片頭音樂當風雲'
    key('3', .2)
    key('F12+w', .2)
    shot('original-track-fengyun')
    time.sleep(66)
    key('F12+w', 1)
    waves = list(out.glob('*.wav'))
    assert len(waves) == 1, '原版 WAV 擷取數量不符'
    wav = waves[0]
    probe = json.loads(run(['ffprobe', '-v', 'error', '-show_format', '-show_streams', '-of', 'json', str(wav)]))
    (out / 'ffprobe.json').write_text(json.dumps(probe, indent=2) + '\n')
    result = subprocess.run(['ffmpeg', '-nostdin', '-hide_banner', '-i', str(wav), '-af', 'volumedetect',
                             '-f', 'null', '-'], capture_output=True, text=True, timeout=30)
    (out / 'volume.log').write_text(result.stderr)
    duration = float(probe['format']['duration'])
    # 模擬器採內部 mixer frame 計數，與主機等待時間分別記錄。
    assert 45 <= duration <= 75 and 'mean_volume: -inf' not in result.stderr
    receipt.update(passed=True, wav={'file': wav.name, 'sha256': sha(wav), 'bytes': wav.stat().st_size,
                                     'duration_seconds': duration, 'track': '風雲'})
finally:
    for p in reversed(processes):
        if p.poll() is None:
            p.terminate()
            try: p.wait(timeout=5)
            except subprocess.TimeoutExpired: p.kill(); p.wait(timeout=5)
    for log in logs: log.close()
    (out / 'receipt.json').write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n')
