#!/usr/bin/env python3
"""在 Docker/Xvfb 驗證零參數啟動、載入動畫、正常新局存檔與可見錯誤。"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

from PIL import Image

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--package', type=Path, required=True)
p.add_argument('--out', type=Path, required=True)
p.add_argument('--wine', action='store_true')
a = p.parse_args()
assert a.package.is_absolute() and a.out.is_absolute()
assert a.out.parent.stat().st_uid == os.getuid() and not a.out.exists()
a.out.mkdir()
env = os.environ | {'DISPLAY': ':96', 'LIBGL_ALWAYS_SOFTWARE': '1', 'LP_NUM_THREADS': '2',
                    'WINEDEBUG': '-all', 'WINEPREFIX': '/tmp/san1-startup-wine'}
receipt = {'passed': False, 'method': 'Wine/Xvfb' if a.wine else 'Linux/Xvfb',
           'cwd': '/tmp', 'state_injection': False, 'seed_injection': False, 'checks': [], 'captures': []}
procs, logs = [], []


def run(command, timeout=15):
    return subprocess.run(command, env=env, check=True, capture_output=True, text=True, timeout=timeout).stdout.strip()


def check(name, result):
    receipt['checks'].append({'name': name, 'passed': bool(result)})
    print(name, bool(result), flush=True)
    assert result, name


def window(proc, title=None, seconds=180):
    end = time.monotonic()+seconds
    while time.monotonic() < end:
        assert proc.poll() is None, '程式提前退出'
        result = subprocess.run(['xdotool', 'search', '--onlyvisible', '--name', title or '三國演義'],
                                env=env, capture_output=True, text=True)
        if result.returncode == 0 and result.stdout.strip():
            return result.stdout.splitlines()[-1]
        time.sleep(.04)
    raise RuntimeError('等待視窗逾時')


def shot(wid, name):
    path = a.out/(name+'.png')
    run(['import', '-window', wid, str(path)])
    with Image.open(path) as im:
        im.load()
        image = im.convert('RGB')
    receipt['captures'].append({'file': path.name, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
                                'width': image.width, 'height': image.height})
    return image


def key(wid, *keys):
    run(['xdotool', 'windowfocus', '--sync', wid])
    for k in keys:
        run(['xdotool', 'keydown', '--clearmodifiers', k])
        time.sleep(.09)
        run(['xdotool', 'keyup', k])
        time.sleep(.3)


def stop(proc):
    if proc.poll() is None:
        proc.terminate()
        try: proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)
    if a.wine:
        subprocess.run(['wineserver', '-k'], env=env, timeout=10, check=False)


def launch(package, tag, arguments):
    binary = package/('san1.exe' if a.wine else 'san1')
    log = (a.out/(tag+'.log')).open('w')
    logs.append(log)
    command = (['wine'] if a.wine else [])+[str(binary)]+arguments
    start = time.monotonic()
    proc = subprocess.Popen(command, cwd='/tmp', env=env, stdout=log, stderr=log)
    procs.append(proc)
    wid = window(proc)
    receipt.setdefault('launches', []).append({'tag': tag, 'arguments': arguments,
            'first_window_seconds': round(time.monotonic()-start, 3)})
    return proc, wid


try:
    log = (a.out/'xvfb.log').open('w')
    logs.append(log)
    xvfb = subprocess.Popen(['Xvfb', ':96', '-screen', '0', '1600x1100x24', '-nolisten', 'tcp', '-ac'], stdout=log, stderr=log)
    procs.append(xvfb)
    time.sleep(.5)
    binary = a.package/('san1.exe' if a.wine else 'san1')
    receipt['binary_sha256'] = hashlib.sha256(binary.read_bytes()).hexdigest()
    reference = Image.open('/src/workplace/promo-source/v.1.1.1-20261010/title-reference.png').convert('RGB').crop((40,27,608,187)).tobytes()
    for edition in ['base', 'plus']:
        arguments = [] if edition == 'base' else ['-edition', 'plus']
        proc, wid = launch(a.package, edition, arguments)
        name = run(['xdotool', 'getwindowname', wid])
        check(edition+'-loading-visible', '載入' in name)
        first = shot(wid, edition+'-loading-1')
        deadline = time.monotonic()+5
        while first.getpixel((10,10)) != (22,38,40) and time.monotonic() < deadline:
            time.sleep(.05)
            first = shot(wid, edition+'-loading-1')
        check(edition+'-loading-painted', first.getpixel((10,10)) == (22,38,40))
        time.sleep(.2)
        second = shot(wid, edition+'-loading-2')
        # 只比較指示條，避免將階段文字改變當成動畫證據。
        def indicator(im):
            return im.resize((640,400), Image.Resampling.NEAREST).crop((40,215,600,219)).tobytes()
        check(edition+'-loading-animated', indicator(first) != indicator(second))
        game_wid = window(proc, '三國演義 remake')
        check(edition+'-same-window', game_wid == wid)
        run(['xdotool', 'windowsize', wid, '640', '408'])
        run(['xdotool', 'windowmove', wid, '0', '0'])
        # 正常按鍵跳過片頭，以既有原貌主選單的靜態區域確認停點。
        deadline = time.monotonic()+45
        while time.monotonic() < deadline:
            key(wid, 'space')
            capture = shot(wid, edition+'-boot')
            pixels = capture.crop((40,27,608,187)).tobytes()
            if len(pixels) == len(reference) and sum(x != y for x,y in zip(pixels,reference)) < len(reference)//100:
                break
        else: raise RuntimeError('未到正常主選單')
        check(edition+'-normal-title', True)
        key(wid, '1', '1', '1', '2', '5')
        time.sleep(.6)
        shot(wid, edition+'-new-game')
        saves = a.package/'saves'
        before = {str(p): p.stat().st_mtime_ns for p in saves.glob('**/REMAKE.JSON')}
        key(wid, 'Return', '9', 'Return', '2', '1', 'Return')
        files = list(saves.glob('**/REMAKE.JSON'))
        check(edition+'-default-save-beside-exe', any(p.stat().st_mtime_ns != before.get(str(p)) for p in files))
        data = json.loads(max(files, key=lambda p:p.stat().st_mtime_ns).read_text())
        check(edition+'-saved-correct-edition', data['edition'] == edition)
        receipt.setdefault('saves', {})[edition] = {'file': str(max(files,key=lambda p:p.stat().st_mtime_ns)), 'edition': data.get('edition'), 'keys': list(data)}
        shot(wid, edition+'-saved')
        stop(proc)
    # 明確路徑及 -load 必須優先於預設值；使用實際正常流程剛寫出的加強版存檔。
    custom = a.out/'custom-saves'
    shutil.copytree(a.package/'saves', custom)
    def argument_path(path):
        return run(['winepath', '-w', str(path)]) if a.wine else str(path)
    proc, wid = launch(a.package, 'explicit-load', ['-edition', 'plus',
        '-root', argument_path(a.package/'game/plus'), '-saves', argument_path(custom),
        '-font', argument_path(a.package/'fonts/unifont.hex.gz'),
        '-hd-assets', argument_path(a.out/'missing-optional-hd'), '-load', '1'])
    wid = window(proc, '三國演義 remake')
    time.sleep(.3)
    shot(wid, 'explicit-load')
    check('explicit-paths-load-saved-game', proc.poll() is None)
    stop(proc)
    # 必要字型與必要原版資料分別缺失，皆須留下可關閉的錯誤視窗。
    for kind in ['font', 'game']:
        missing = a.out/('missing-'+kind)
        missing.mkdir()
        shutil.copy2(binary, missing/binary.name)
        if kind == 'game':
            (missing/'fonts').mkdir()
            shutil.copy2(a.package/'fonts/unifont.hex.gz', missing/'fonts/unifont.hex.gz')
        proc, wid = launch(missing, 'missing-'+kind, [])
        wid = window(proc, '無法啟動')
        time.sleep(.3)
        shot(wid, 'missing-'+kind)
        check('missing-'+kind+'-error-stays-visible', proc.poll() is None)
        key(wid, 'Escape')
        proc.wait(timeout=10)
        check('missing-'+kind+'-esc-exits', True)
        stop(proc)
    receipt['passed'] = True
finally:
    for proc in reversed(procs): stop(proc)
    for log in logs: log.close()
    (a.out/'receipt.json').write_text(json.dumps(receipt, ensure_ascii=False, indent=2)+'\n')
