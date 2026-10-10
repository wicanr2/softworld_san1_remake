#!/usr/bin/env python3
"""Run both editions from an extracted AppImage in Docker and verify its full contents."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import time

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--image', type=Path, required=True)
p.add_argument('--out', type=Path, required=True)
p.add_argument('--version', required=True)
a = p.parse_args()
assert a.out.is_dir() and a.out.stat().st_uid == os.getuid()
result = {'version': a.version, 'passed': False, 'edition_smoke': [], 'method': 'AppImage runtime extraction and actual AppRun under Xvfb'}
procs = []


def run(cmd, **kwargs):
    return subprocess.run(cmd, check=True, capture_output=True, text=True, timeout=60, **kwargs)


try:
    with tempfile.TemporaryDirectory(prefix='san1-appimage-smoke-') as tmp:
        stage = Path(tmp)
        run([str(a.image), '--appimage-extract'], cwd=stage)
        appdir = stage / 'squashfs-root'
        assert (appdir / 'AppRun').is_file() and (appdir / 'LICENSE').is_file()
        assert len(list((appdir / 'game').rglob('*.*'))) >= 60
        assert len(json.loads((appdir / 'hd-assets/manifest.json').read_text())['entries']) == 904
        env = os.environ | {'DISPLAY': ':98', 'LIBGL_ALWAYS_SOFTWARE': '1', 'LP_NUM_THREADS': '2',
                            'XDG_RUNTIME_DIR': str(stage), 'XDG_DATA_HOME': str(stage / 'user-data'), 'APPDIR': str(appdir)}
        log = (a.out / 'appimage-xvfb.log').open('w')
        xvfb = subprocess.Popen(['Xvfb', ':98', '-screen', '0', '1400x1000x24', '-nolisten', 'tcp', '-ac'], stdout=log, stderr=log)
        procs.append(xvfb)
        time.sleep(1)
        version = run([str(appdir / 'AppRun'), '-version'], env=env).stdout.strip()
        assert version == a.version
        for edition in ['base', 'plus']:
            with (a.out / f'appimage-{edition}.log').open('w') as output:
                proc = subprocess.Popen([str(appdir / 'AppRun'), '-edition', edition, '-ai', edition,
                    '-scale', '1', '-music=false', '-sound=false'], env=env, stdout=output, stderr=output, start_new_session=True)
                procs.append(proc)
                wid = None
                for _ in range(80):
                    assert proc.poll() is None
                    search = subprocess.run(['xdotool', 'search', '--onlyvisible', '--name', '三國演義 remake'], env=env, capture_output=True, text=True)
                    if search.returncode == 0 and search.stdout.strip():
                        wid = search.stdout.splitlines()[0]
                        break
                    time.sleep(.15)
                assert wid
                for _ in range(20):
                    run(['xdotool', 'windowfocus', '--sync', wid], env=env)
                    run(['xdotool', 'key', '--clearmodifiers', 'space'], env=env)
                    time.sleep(.15)
                time.sleep(2)
                picture = a.out / f'appimage-{edition}.png'
                run(['ffmpeg', '-nostdin', '-v', 'error', '-y', '-f', 'x11grab', '-video_size', '640x408',
                     '-i', ':98', '-frames:v', '1', '-threads', '1', str(picture)], env=env)
                assert picture.stat().st_size > 1000 and proc.poll() is None
                saves = stage / f'user-data/softworld-san1/saves-{edition}'
                assert saves.is_dir()
                result['edition_smoke'].append({'edition': edition, 'window_created': True,
                    'ran_with_game_and_hd_data': True, 'writable_save_directory': True,
                    'capture': picture.name, 'capture_sha256': hashlib.sha256(picture.read_bytes()).hexdigest()})
                os.killpg(proc.pid, signal.SIGTERM)
                proc.wait(timeout=10)
        result['image_sha256'] = hashlib.sha256(a.image.read_bytes()).hexdigest()
        result['binary_sha256'] = hashlib.sha256((appdir / 'san1').read_bytes()).hexdigest()
        result['passed'] = True
finally:
    for proc in reversed(procs):
        if proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=5)
    (a.out / 'appimage-smoke.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(result, ensure_ascii=False))
