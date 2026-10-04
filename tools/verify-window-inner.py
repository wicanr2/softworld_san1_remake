#!/usr/bin/env python3
"""Xvfb 中走正常新局，收據及圖片只留本機。"""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import threading
import time

ROOT = Path('/src')
OUT = ROOT / 'workplace/hd-window'
processes, logs = [], []
receipt = {'method': 'Linux Xvfb 正常片頭／主選單／新局／存檔', 'checks': [], 'captures': []}


def run(args, timeout=15):
    return subprocess.run(args, check=True, timeout=timeout, capture_output=True, text=True).stdout.strip()


def start(args, name):
    log = open(OUT / (name + '.log'), 'w')
    logs.append(log)
    proc = subprocess.Popen(args, stdout=log, stderr=subprocess.STDOUT)
    processes.append(proc)
    return proc


def stop(proc):
    if proc.poll() is None:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)


def wait(args, seconds=20):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        try:
            value = run(args, 2)
            if value:
                return value
        except (subprocess.SubprocessError, OSError):
            pass
        time.sleep(.2)
    raise RuntimeError(f'等待逾時：{args}')


def key(wid, *keys):
    run(['xdotool', 'windowfocus', '--sync', wid])
    for k in keys:
        run(['xdotool', 'keydown', '--clearmodifiers', k])
        time.sleep(.08)
        run(['xdotool', 'keyup', k])
        time.sleep(.35)
        receipt.setdefault('keys', []).append(k)


def geo(wid):
    return {k: int(v) for k, v in (line.split('=', 1) for line in run(
        ['xdotool', 'getwindowgeometry', '--shell', wid]).splitlines()) if k != 'WINDOW'}


def check(name, result):
    receipt['checks'].append({'name': name, 'passed': bool(result)})
    print(name, bool(result), flush=True)
    if not result:
        raise RuntimeError(name)


def dimensions(wid, name, width, height):
    time.sleep(.3)
    g = geo(wid)
    check(name, (g['WIDTH'], g['HEIGHT']) == (width, height))


def move(wid, x, y):
    scale = geo(wid)['WIDTH'] / 640
    run(['xdotool', 'mousemove', '--window', wid, str(int(x * scale)), str(int(y * scale))])
    time.sleep(.25)


def choose(wid, field, row):
    x = [100, 270, 470][field]
    move(wid, x, 16)
    run(['xdotool', 'click', '1'])
    time.sleep(.2)
    move(wid, x, 32 + row * 24 + 12)
    run(['xdotool', 'click', '1'])
    time.sleep(.4)


def choose_ready(wid, field, row):
    """按住滑鼠直到下拉畫面回應；避免短按被軟體渲染的幀間隔漏掉。"""
    x = [100, 270, 470][field]
    probe_x = [10, 202, 378][field]
    colors = [bytes([44, 52, 65]), bytes([44, 92, 112])]
    action = len(receipt.setdefault('toolbar_actions', []))
    receipt['toolbar_actions'].append({'field': field, 'row': row})

    def click_until(opened, phase):
        run(['xdotool', 'mousedown', '1'])
        try:
            deadline = time.monotonic() + 8
            step = 0
            while time.monotonic() < deadline:
                scale = geo(wid)['WIDTH'] / 640
                capture = shot(wid, f'toolbar-{action}-{phase}-{step}')
                pixel = rgb(capture, f'1:1:{int(probe_x*scale)}:{int(34*scale)}')
                if (pixel in colors) == opened:
                    return
                step += 1
            raise RuntimeError(f'選項列 {field} 的 {phase} 畫面未回應')
        finally:
            run(['xdotool', 'mouseup', '1'])

    move(wid, x, 16)
    click_until(True, 'open')
    move(wid, x, 32 + row * 24 + 12)
    click_until(False, 'selected')


def shot(wid, name):
    path = OUT / (name + '.png')
    g = geo(wid)
    run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y',
         '-f', 'x11grab', '-video_size', f"{g['WIDTH']}x{g['HEIGHT']}",
         '-i', f":99+{g['X']},{g['Y']}", '-frames:v', '1', str(path)])
    receipt['captures'].append({'file': path.name, 'width': g['WIDTH'], 'height': g['HEIGHT'],
                                'sha256': hashlib.sha256(path.read_bytes()).hexdigest()})
    return path


def rgb(path, crop=None):
    args = ['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-threads', '1', '-i', str(path)]
    if crop:
        args += ['-vf', 'crop=' + crop]
    return subprocess.run(args + ['-frames:v', '1', '-pix_fmt', 'rgb24', '-threads', '1', '-f', 'rawvideo', 'pipe:1'],
                          check=True, capture_output=True, timeout=15).stdout


def verify_hd_terrain(original, high, restored, pack, tag, edition):
    """從正常截圖辨識完整未遮擋原圖格，再逐格核對原生高清與恢復。"""
    manifest = json.loads((pack / 'manifest.json').read_text())['entries']
    entries = [e for e in manifest if e['edition'] == edition and
               e['container'] == 'DATA1' and e['name'].startswith('EICON.GRP#') and
               0 <= int(e['name'].split('#')[1]) <= 14]
    if not entries:
        return

    def crop(data, width, x, y, w, h):
        return b''.join(data[((y+r)*width+x)*3:((y+r)*width+x+w)*3] for r in range(h))

    before, after, again = rgb(original), rgb(high), rgb(restored)
    check(tag + '-terrain-canvas-size', len(before) == len(again) == 640*408*3 and len(after) == 2560*1632*3)
    sources = {}
    for entry in entries:
        n = int(entry['name'].split('#')[1])
        source = ROOT / f'workplace/hd-preview/v20-source-EICON{n:02d}.png'
        sources[rgb(source)] = (entry, rgb(pack / entry['file']))
    samples = []
    for col in range(12):
        for row in range(10):
            x, y = 56+48*col, 36+32*row+16*(col%2)
            raw = crop(before, 640, x, y, 48, 32)
            match = sources.get(raw)
            if match is None:
                continue
            entry, expected = match
            samples.append({'key': 'DATA1/'+entry['name'], 'source_rect': [x,y,48,32],
                            'high_rect': [x*4,y*4,192,128],
                            'native_equal': crop(after,2560,x*4,y*4,192,128) == expected,
                            'original_restored': crop(again,640,x,y,48,32) == raw})
    check(tag + '-terrain-source-present', bool(samples))
    check(tag + '-terrain-native', all(s['native_equal'] for s in samples))
    check(tag + '-terrain-original-restored', all(s['original_restored'] for s in samples))
    receipt.setdefault('terrain_samples', []).append({'edition': edition, 'stage': tag,
        'original': str(original.relative_to(ROOT)), 'high': str(high.relative_to(ROOT)),
        'restored': str(restored.relative_to(ROOT)), 'tiles': samples,
        'scope': '正常截圖中逐像素符合完整原圖的未遮擋格；不注入地圖或 seed'})


def launch(edition, missing=False, hd_assets=None, tag=None):
    folder = '三國演義' if edition == 'base' else '三國演義1加強版'
    tag = tag or edition
    t = time.monotonic()
    proc = start([str(OUT / 'san1-window-check'), '-root', '/orig/' + folder, '-edition', edition,
                  '-ai', edition, '-scale', '1', '-music=false', '-sound=false',
                  '-saves', str(OUT / ('saves-' + edition)),
                  '-hd-assets', '/tmp/missing-hd' if missing else str(hd_assets or ROOT / 'workplace/hd-assets')], tag)
    wid = wait(['xdotool', 'search', '--onlyvisible', '--pid', str(proc.pid), '--name', '三國演義 remake']).splitlines()[0]
    move(wid, 320, 200)
    reference = rgb(OUT / 'title-reference.png', '568:160:40:27')
    deadline = time.monotonic() + 45
    stop_input = threading.Event()
    input_errors = []
    receipt.setdefault('boot_input', {})[tag] = {'method': 'Space 按下／放開各 80 ms，與抓圖分開；主選單辨識後停止', 'count': 0}
    run(['xdotool', 'windowfocus', '--sync', wid])

    def input_stream():
        try:
            while not stop_input.is_set() and time.monotonic() < deadline:
                run(['xdotool', 'keydown', '--clearmodifiers', 'space'])
                time.sleep(.08)
                run(['xdotool', 'keyup', 'space'])
                receipt.setdefault('keys', []).append('space')
                receipt['boot_input'][tag]['count'] += 1
                stop_input.wait(.08)
        except Exception as error:
            input_errors.append(str(error))
        finally:
            run(['xdotool', 'keyup', 'space'])

    worker = threading.Thread(target=input_stream, daemon=True)
    worker.start()
    try:
        while time.monotonic() < deadline:
            if input_errors:
                raise RuntimeError('片頭輸入失敗：' + input_errors[0])
            path = shot(wid, tag + '-boot')
            sample = rgb(path, '568:160:40:27')
            if len(sample) == len(reference) and sum(a != b for a, b in zip(sample, reference)) < len(reference) // 100:
                break
        else:
            raise RuntimeError('片頭未進入正常主選單')
    finally:
        stop_input.set()
        worker.join(timeout=5)
        if worker.is_alive():
            raise RuntimeError('片頭輸入程序未停止')
    receipt.setdefault('launch_seconds', {})[tag] = round(time.monotonic() - t, 3)
    dimensions(wid, tag + '-hidden-default', 640, 408)
    return proc, wid



if __name__ == "__main__":
    try:
        assert OUT.stat().st_uid == os.getuid() and OUT.stat().st_gid == os.getgid()
        os.environ.update({'DISPLAY': ':99', 'LIBGL_ALWAYS_SOFTWARE': '1', 'XDG_RUNTIME_DIR': '/tmp'})
        start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp'], 'xvfb')
        wait(['xdotool', 'getdisplaygeometry'])
        receipt['binary_sha256'] = hashlib.sha256((OUT / 'san1-window-check').read_bytes()).hexdigest()
        receipt['pack_sha256'] = hashlib.sha256((ROOT / 'workplace/hd-assets/manifest.json').read_bytes()).hexdigest()
        for edition in ['base', 'plus']:
            proc, wid = launch(edition)
            move(wid, 100, 2)
            dimensions(wid, edition + '-hover-show', 640, 440)
            shot(wid, edition + '-hover-toolbar')
            move(wid, 320, 200)
            dimensions(wid, edition + '-hover-hide', 640, 408)
            key(wid, 'Escape')
            dimensions(wid, edition + '-escape-show', 640, 440)
            # 固定在列外，鍵盤展開不被自動收起。
            move(wid, 320, 200)
            dimensions(wid, edition + '-escape-pinned', 640, 440)
            choose(wid, 2, 5)
            for row, label in [(1, 'en'), (2, 'ja'), (0, 'zh-Hant')]:
                choose(wid, 0, row)
                shot(wid, edition + '-language-' + label)
            key(wid, 'Escape')
            dimensions(wid, edition + '-escape-hide', 640, 408)
            # 非原比例的視窗也依實際遊戲倍率加高，避免展開後放大遊戲。
            move(wid, 320, 200)
            run(['xdotool', 'windowsize', wid, '1280', '600'])
            dimensions(wid, edition + '-wide-hidden', 1280, 600)
            key(wid, 'Escape')
            dimensions(wid, edition + '-wide-toolbar', 1280, 647)
            key(wid, 'Escape')
            dimensions(wid, edition + '-wide-restored', 1280, 600)
            run(['xdotool', 'windowsize', wid, '640', '408'])
            time.sleep(.3)
            key(wid, '1', '1', '1', '2', '5')
            time.sleep(.6)
            key(wid, '0', 'Return')
            original = shot(wid, edition + '-original-main')
            key(wid, 'Escape')
            key(wid, '9')  # 選項列的數字鍵不能滲入遊戲數字輸入。
            choose(wid, 1, 1)
            key(wid, 'Escape')
            run(['xdotool', 'windowsize', wid, '2560', '1632'])
            run(['xdotool', 'windowmove', wid, '0', '0'])
            time.sleep(1)
            native = shot(wid, edition + '-hd-native-main')
            master = ROOT / 'workplace/hd-assets/F000.png'
            check(edition + '-native-portrait-detail', rgb(native, '256:320:2144:464') == rgb(master))
            receipt.setdefault('rss_kib_hd', {})[edition] = next(int(line.split()[1]) for line in
                Path(f'/proc/{proc.pid}/status').read_text().splitlines() if line.startswith('VmRSS:'))
            key(wid, 'Escape')
            dimensions(wid, edition + '-native-toolbar', 2560, 1760)
            shot(wid, edition + '-hd-toolbar')
            choose(wid, 1, 0)
            key(wid, 'Escape')
            run(['xdotool', 'windowsize', wid, '640', '408'])
            time.sleep(.5)
            restored = shot(wid, edition + '-restored-main')
            check(edition + '-original-restored', rgb(original, '224:200:408:36') == rgb(restored, '224:200:408:36'))
            # 狀態命令已退出數字輸入，9 直接開其他選單，不再按 Enter 休息。
            save_started_ns = time.time_ns()
            key(wid, '9')
            shot(wid, edition + '-other')
            key(wid, '2')
            shot(wid, edition + '-save-picker')
            key(wid, '1', 'Return')
            time.sleep(.4)
            shot(wid, edition + '-saved')
            files = [p for p in (OUT / ('saves-' + edition)).glob('**/REMAKE.JSON')
                     if p.stat().st_mtime_ns >= save_started_ns]
            saved = [json.loads(p.read_text()) for p in files]
            check(edition + '-ai-strength-saved', any(x.get('options', {}).get('ai_orders') == 5 and
                x.get('options', {}).get('ai_mode') == 'enhanced' for x in saved))
            stop(proc)
        proc, wid = launch('base', missing=True)
        key(wid, 'Escape')
        choose(wid, 1, 1)
        key(wid, 'Escape')
        dimensions(wid, 'missing-pack-original', 640, 408)
        shot(wid, 'missing-pack-original')
        receipt['passed'] = True
    finally:
        receipt.setdefault('passed', False)
        for proc in reversed(processes):
            stop(proc)
        for log in logs:
            log.close()
        (OUT / 'receipt.json').write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n')
