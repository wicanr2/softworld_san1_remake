#!/usr/bin/env python3
"""兩版正常曹操新局，陳留攻鄴郡，選誘敵並回讀連續原貌／高清幀。"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import time

spec = importlib.util.spec_from_file_location('window_check', Path(__file__).with_name('verify-window-inner.py'))
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = gui.ROOT / os.environ.get('SAN1_HD_LURE_OUT', 'workplace/hd-window/player/lure-v21')
pack = gui.ROOT / os.environ.get('SAN1_HD_LURE_PACK', 'workplace/hd-assets-lure-v21')
gui.receipt.update(method='正常片頭、新局、出兵、合法紮寨、策略方向與誘敵選單；連續 X11 錄影',
                   scenario='001', player=1, difficulty=5, gold=400, rice=1000,
                   randomness='正式新局預設亂數；不注入狀態、不重擲；不是原版 oracle',
                   audio='本工具關閉音訊；不宣稱音效或人耳驗證')


def crop(data, width, x, y, w, h):
    return b''.join(data[((y+r)*width+x)*3:((y+r)*width+x+w)*3] for r in range(h))


def theme(wid, high):
    gui.key(wid, 'Escape')
    gui.choose_ready(wid, 1, int(high))
    gui.key(wid, 'Escape')
    gui.run(['xdotool', 'windowsize', wid, '2560' if high else '640', '1632' if high else '408'])
    gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    gui.move(wid, 20, 380)
    time.sleep(.5)


def await_face(wid, tag):
    source = gui.rgb(gui.ROOT / 'workplace/hd-inventory/base/img/DATA3/F000.png')
    source = b''.join(source[(y*64+x)*3:(y*64+x+1)*3] for y in range(80) for x in range(63, -1, -1))
    for step in range(12):
        capture = gui.shot(wid, tag + f'-camp-await-{step}')
        if gui.rgb(capture, '64:80:456:52') == source:
            return
        gui.key(wid, 'space')
    raise RuntimeError(tag + ' 未到達正常戰場')


def command_ready(wid, tag, high):
    s = 4 if high else 1
    expected = gui.rgb(gui.OUT / 'command-reference.png', '176:48:0:0')
    for step in range(40):
        capture = gui.shot(wid, tag + f'-command-{step}')
        actual = gui.rgb(capture, f'{176*s}:{48*s}:{448*s}:{268*s},scale=176:48:flags=neighbor')
        if actual == expected:
            gui.check(tag + '-normal-command', True)
            return capture
        gui.key(wid, 'space')
    gui.check(tag + '-normal-command', False)


def verify_movie(tag, movie, high, x, y):
    s = 4 if high else 1
    w, h = 80*s, 64*s
    size = w*h*3
    targets = {n: gui.rgb(pack / f'EICON{n:02d}.png') if high else
               gui.rgb(gui.ROOT / f'workplace/hd-preview/v21-source-EICON{n:02d}.png') for n in range(32, 36)}
    decoder = subprocess.Popen(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-threads', '1',
                                '-i', str(movie), '-pix_fmt', 'rgb24', '-threads', '1', '-f', 'rawvideo', 'pipe:1'], stdout=subprocess.PIPE)
    frames, runs, saved, outside, restoration = 0, [], {}, None, None
    outside_ok, matched_frames = True, 0
    try:
        while True:
            frame = decoder.stdout.read(size)
            if not frame:
                break
            if len(frame) != size:
                raise RuntimeError('錄影幀不完整')
            sample = crop(frame, w, 16*s, 16*s, 48*s, 32*s)
            found = next((n for n, t in targets.items() if sample == t), None)
            if found is not None:
                border = b''.join(frame[(row*w)*3:(row*w+16*s)*3] + frame[(row*w+64*s)*3:((row+1)*w)*3]
                                  if 16*s <= row < 48*s else frame[(row*w)*3:((row+1)*w)*3] for row in range(h))
                if outside is None:
                    outside = border
                outside_ok = outside_ok and border == outside
                matched_frames += 1
                if not runs or runs[-1]['tile'] != found:
                    runs.append({'tile': found, 'first_frame': frames, 'frames': 0})
                runs[-1]['frames'] += 1
                if found not in saved:
                    out = gui.OUT / f'{tag}-EICON{found}.png'
                    subprocess.run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-f', 'rawvideo',
                                    '-pix_fmt', 'rgb24', '-s', f'{w}x{h}', '-i', 'pipe:0', '-frames:v', '1',
                                    '-threads', '1', str(out)], input=frame, check=True, timeout=15)
                    saved[found] = {'file': out.name, 'sha256': hashlib.sha256(out.read_bytes()).hexdigest()}
            elif len(runs) == 22 and restoration is None:
                out = gui.OUT / f'{tag}-restoration.png'
                subprocess.run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-f', 'rawvideo',
                                '-pix_fmt', 'rgb24', '-s', f'{w}x{h}', '-i', 'pipe:0', '-frames:v', '1',
                                '-threads', '1', str(out)], input=frame, check=True, timeout=15)
                restoration = {'frame': frames, 'file': out.name,
                               'sha256': hashlib.sha256(out.read_bytes()).hexdigest()}
                palette = [(0,0,0),(0,0,170),(0,170,0),(0,170,170),(170,0,0),(170,0,170),
                           (170,85,0),(170,170,170),(85,85,85),(85,85,255),(85,255,85),
                           (85,255,255),(255,85,85),(255,85,255),(255,255,85),(255,255,255)]
                restoration['flag_redrawn'] = False
                for name in ['WFLAGA00', 'WFLAGD00']:
                    flag = gui.rgb(gui.ROOT / f'workplace/hd-inventory/base/img/DATA1/{name}.png')
                    for complement in [False, True]:
                        pixels = (bytes(v for n in range(0, len(flag), 3)
                                        for v in palette[palette.index(tuple(flag[n:n+3])) ^ 15])
                                  if complement else flag)
                        expected = b''.join(pixels[(yy*24+xx)*3:(yy*24+xx+1)*3]
                                            for yy in range(15) for sy in range(s)
                                            for xx in range(24) for sx in range(s))
                        if crop(frame, w, 24*s, 16*s, 24*s, 15*s) == expected:
                            restoration.update(flag_redrawn=True, flag_source=name + '.IMG',
                                               flag_complement=complement)
            frames += 1
        if decoder.wait(timeout=15) != 0:
            raise RuntimeError('錄影解碼失敗')
    finally:
        if decoder.poll() is None:
            decoder.kill()
            decoder.wait(timeout=5)
    expected = [32, 33] + [34, 35]*10
    gui.receipt.setdefault('lure_movies', []).append({'stage': tag, 'high': high,
        'file': movie.name, 'sha256': hashlib.sha256(movie.read_bytes()).hexdigest(), 'frames': frames,
        'runs': runs, 'source_rect': [x, y, 48, 32], 'record_rect': [x-16, y-16, 80, 64],
        'saved_frames': saved, 'native_equal': set(saved) == {32, 33, 34, 35},
        'sequence_equal': [r['tile'] for r in runs] == expected,
        'outside_equal': outside is not None and outside_ok, 'matched_frames': matched_frames,
        'restoration_frame': restoration})
    gui.check(tag + '-outside-all-animation-frames', outside is not None and outside_ok)
    gui.check(tag + '-native-all-four', set(saved) == {32, 33, 34, 35})
    gui.check(tag + '-complete-22-steps', [r['tile'] for r in runs] == expected)
    gui.check(tag + '-flag-redrawn', restoration is not None and restoration['flag_redrawn'])


def play(edition, high):
    tag = edition + ('-hd' if high else '-original')
    proc, wid = gui.launch(edition, hd_assets=pack, tag=tag)
    gui.key(wid, '1', '1', '1', '2', '5')
    gui.key(wid, '2', 'Return', '2', '1', '1', 'Return', '4', 'Return',
            '1', 'Return', '1', 'Return', 'Return', 'y', '4', '0', '0', 'Return',
            '1', '0', '0', '0', 'Return')
    await_face(wid, tag)
    gui.key(wid, *(['space']*9))
    # 入口 (5,8) 至合法 (5,4)，再朝左下城池 (4,5) 用計。
    gui.key(wid, '5', '5', '5', '5', '0')
    original = command_ready(wid, tag, False)
    if high:
        theme(wid, True)
        command_ready(wid, tag, True)
    x, y = 56+48*5, 36+32*4+16
    gui.receipt.setdefault('player_plans', []).append({'edition': edition, 'high': high,
        'camp_keys': ['5', '5', '5', '5', '0'], 'caster_cell': [5, 4], 'target_cell': [4, 5],
        'cast_keys': ['6', '1', '4'], 'gold': 400, 'rice': 1000})
    gui.key(wid, '6', '1', '4')
    # 誘敵對白仍由正常按鍵結束；先錄影，再收這一句。
    gui.shot(wid, tag + '-speech')
    s = 4 if high else 1
    movie = gui.OUT / (tag + '-lure.mkv')
    g = gui.geo(wid)
    rate, duration = (30, 90) if high else (60, 6)
    recorder = gui.start(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-threads', '1',
        '-f', 'x11grab', '-draw_mouse', '0', '-framerate', str(rate), '-video_size', f'{80*s}x{64*s}',
        '-i', f":99+{g['X']+(x-16)*s},{g['Y']+(y-16)*s}", '-t', str(duration), '-r', str(rate), '-c:v', 'ffv1',
        '-level', '3', '-pix_fmt', 'bgr0', '-threads', '1', str(movie)], tag + '-movie')
    time.sleep(.5)
    gui.key(wid, 'space')
    recorder.wait(timeout=duration+20)
    after = gui.shot(wid, tag + '-after')
    verify_movie(tag, movie, high, x, y)
    # 第一個恢復幀已核對原始旗圖；長錄影後的戰況可能已繼續推進。
    if high:
        theme(wid, False)
    restored = gui.shot(wid, tag + '-restored')
    gui.check(tag + '-original-theme-restored', (gui.geo(wid)['WIDTH'], gui.geo(wid)['HEIGHT']) == (640, 408))
    gui.receipt.setdefault('lure_completion', []).append({'stage': tag, 'original_before': original.name,
        'after': after.name, 'restored': restored.name, 'flag_redrawn': True})
    gui.stop(proc)


try:
    if (gui.OUT.stat().st_uid, gui.OUT.stat().st_gid) != (os.getuid(), os.getgid()):
        raise RuntimeError('輸出擁有權不符')
    os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp')
    socket_dir = Path('/tmp/.X11-unix')
    socket_dir.mkdir(exist_ok=True)
    socket_dir.chmod(0o1777)
    display = gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp',
                         '-noreset', '-ac'], 'xvfb')
    geometry = gui.wait(['xdotool', 'getdisplaygeometry'])
    gui.check('display-ready', display.poll() is None and geometry == '2800 1900' and
              (socket_dir / 'X99').is_socket())
    gui.receipt.update(binary_sha256=hashlib.sha256((gui.OUT / 'san1-window-check').read_bytes()).hexdigest(),
        pack_sha256=hashlib.sha256((pack / 'manifest.json').read_bytes()).hexdigest(),
        tool_sha256={p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in [Path(__file__),
            Path(__file__).with_name('verify-hd-lure.sh'), Path(__file__).with_name('verify-window-inner.py')]})
    for edition in ['base', 'plus']:
        for high in [False, True]:
            play(edition, high)
    gui.receipt['passed'] = True
finally:
    gui.receipt.setdefault('passed', False)
    for proc in reversed(gui.processes):
        gui.stop(proc)
    for log in gui.logs:
        log.close()
    (gui.OUT / 'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2)+'\n')
