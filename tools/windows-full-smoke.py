#!/usr/bin/env python3
"""Smoke-test the existing Windows full package under Wine in a disposable Docker container."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import zipfile

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--package', type=Path, required=True)
p.add_argument('--out', type=Path, required=True)
p.add_argument('--version', required=True)
a = p.parse_args()
assert a.out.is_dir() and a.out.stat().st_uid == os.getuid()
result = {'version': a.version, 'passed': False, 'method': 'Wine under Xvfb; not native Windows', 'editions': []}
env = None
xvfb = None
processes = []


def run(cmd, **kwargs):
    return subprocess.run(cmd, check=True, capture_output=True, text=True, timeout=90, **kwargs)


try:
    with tempfile.TemporaryDirectory(prefix='san1-windows-smoke-') as tmp:
        stage = Path(tmp)
        with zipfile.ZipFile(a.package) as archive:
            assert archive.testzip() is None
            assert all('..' not in Path(n).parts and not Path(n).is_absolute() for n in archive.namelist())
            archive.extractall(stage)
        game = stage / f'san1-{a.version}-windows-amd64'
        binary = game / 'san1.exe'
        assert binary.is_file() and shutil.which('import')
        env = os.environ | {'DISPLAY': ':97', 'WINEPREFIX': str(stage / 'wine-prefix'),
                            'WINEDEBUG': '-all', 'LIBGL_ALWAYS_SOFTWARE': '1', 'LP_NUM_THREADS': '2'}
        with (a.out / 'windows-wine-xvfb.log').open('w') as log:
            xvfb = subprocess.Popen(['Xvfb', ':97', '-screen', '0', '1400x1000x24', '-nolisten', 'tcp', '-ac'], stdout=log, stderr=log)
        time.sleep(1)
        run(['wineboot', '-u'], env=env)
        run(['wineserver', '-w'], env=env)
        version = run(['wine', str(binary), '-version'], env=env).stdout.strip()
        assert version == a.version, version
        winpath = lambda path: 'Z:' + str(path).replace('/', '\\')
        for edition in ['base', 'plus']:
            saves = stage / f'saves-{edition}'
            saves.mkdir()
            with (a.out / f'windows-wine-{edition}.log').open('w') as log:
                proc = subprocess.Popen(['wine', str(binary), '-root', winpath(game / 'game' / edition),
                    '-edition', edition, '-ai', edition, '-saves', winpath(saves),
                    '-hd-assets', winpath(game / 'hd-assets'), '-font', winpath(game / 'fonts/unifont.hex.gz'),
                    '-scale', '1', '-music=false', '-sound=false'], env=env, cwd=game, stdout=log, stderr=log)
                processes.append(proc)
                window = None
                for _ in range(120):
                    assert proc.poll() is None, (edition, proc.returncode)
                    query = subprocess.run(['xdotool', 'search', '--onlyvisible', '--name', '三國演義 remake'],
                                           env=env, capture_output=True, text=True)
                    if query.returncode == 0 and query.stdout.strip():
                        window = query.stdout.splitlines()[0]
                        break
                    time.sleep(.2)
                assert window
                time.sleep(8)
                assert proc.poll() is None
                picture = a.out / f'windows-wine-{edition}.png'
                run(['import', '-window', window, str(picture)], env=env)
                assert picture.stat().st_size > 1000
                result['editions'].append({'edition': edition, 'window_created': True, 'sustained_seconds': 8,
                    'capture': picture.name, 'capture_sha256': hashlib.sha256(picture.read_bytes()).hexdigest()})
                run(['wineserver', '-k'], env=env)
                proc.wait(timeout=15)
                run(['wineserver', '-w'], env=env)
        result['package_sha256'] = hashlib.sha256(a.package.read_bytes()).hexdigest()
        result['binary_sha256'] = hashlib.sha256(binary.read_bytes()).hexdigest()
        result['passed'] = True
finally:
    if env:
        subprocess.run(['wineserver', '-k'], env=env, timeout=15, capture_output=True)
    for proc in processes:
        if proc.poll() is None:
            proc.kill()
            proc.wait(timeout=5)
    if xvfb and xvfb.poll() is None:
        xvfb.terminate()
        xvfb.wait(timeout=5)
    (a.out / 'windows-wine-smoke.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(result, ensure_ascii=False), flush=True)
