#!/usr/bin/env python3
"""正常新局與命令的高清場景；以 X11 擷取實際動畫，不改遊戲狀態。"""
import argparse
import ctypes
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import threading
import time

spec = importlib.util.spec_from_file_location('window_check', Path(__file__).with_name('verify-window-inner.py'))
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = gui.ROOT / 'workplace/hd-window/player/scenes-v7'
pack = gui.ROOT / 'workplace/hd-assets-scenes-v7'
W, H = 704, 384


class XImage(ctypes.Structure):
    _fields_ = [(n, ctypes.c_int) for n in ['width', 'height', 'xoffset', 'format']] + \
        [('data', ctypes.c_void_p)] + [(n, ctypes.c_int) for n in
        ['byte_order', 'bitmap_unit', 'bitmap_bit_order', 'bitmap_pad', 'depth', 'bytes_per_line', 'bits_per_pixel']] + \
        [(n, ctypes.c_ulong) for n in ['red_mask', 'green_mask', 'blue_mask']]


class Capture:
    def __init__(self):
        self.lib = ctypes.CDLL('libX11.so.6')
        self.lib.XOpenDisplay.argtypes, self.lib.XOpenDisplay.restype = [ctypes.c_char_p], ctypes.c_void_p
        self.lib.XDefaultRootWindow.argtypes, self.lib.XDefaultRootWindow.restype = [ctypes.c_void_p], ctypes.c_ulong
        self.lib.XGetImage.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_int, ctypes.c_int,
                                      ctypes.c_uint, ctypes.c_uint, ctypes.c_ulong, ctypes.c_int]
        self.lib.XGetImage.restype = ctypes.POINTER(XImage)
        self.lib.XDestroyImage.argtypes = [ctypes.POINTER(XImage)]
        self.lib.XCloseDisplay.argtypes = [ctypes.c_void_p]
        self.display = self.lib.XOpenDisplay(b':99')
        if not self.display:
            raise RuntimeError('X11 未就緒')
        self.root = self.lib.XDefaultRootWindow(self.display)

    def read(self):
        im = self.lib.XGetImage(self.display, self.root, 1728, 320, W, H, ctypes.c_ulong(-1).value, 2)
        if not im:
            raise RuntimeError('X11 擷取失敗')
        try:
            v = im.contents
            if (v.width, v.height, v.bits_per_pixel, v.byte_order, v.red_mask, v.green_mask,
                v.blue_mask) != (W, H, 32, 0, 0xff0000, 0xff00, 0xff):
                raise RuntimeError('X11 像素格式不符')
            raw = ctypes.string_at(v.data, v.bytes_per_line*H)
            raw = b''.join(raw[y*v.bytes_per_line:y*v.bytes_per_line+W*4] for y in range(H))
            rgb = bytearray(W*H*3)
            rgb[0::3], rgb[1::3], rgb[2::3] = raw[2::4], raw[1::4], raw[0::4]
            return bytes(rgb)
        finally:
            self.lib.XDestroyImage(im)

    def close(self):
        self.lib.XCloseDisplay(self.display)


def theme(wid, high):
    gui.key(wid, 'Escape')
    gui.choose_ready(wid, 1, int(high))
    gui.key(wid, 'Escape')
    gui.run(['xdotool', 'windowsize', wid, '2560' if high else '640', '1632' if high else '408'])
    gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    gui.move(wid, 320, 200)
    time.sleep(.5)


def part(pixels, rect):
    x, y, w, h = rect
    return b''.join(pixels[((y+r)*W+x)*3:((y+r)*W+x+w)*3] for r in range(h))


def geometry():
    # 獨立採用 spec/010 §3：整塊從對邊滑入，不呼叫引擎 Reveal／Source。
    for kind in range(4):
        for step in range(1, 24 if kind < 2 else 22):
            if kind == 0:
                h = step*16
                dst, src = (0, 0, W, h), (0, H-h, W, h)
            elif kind == 1:
                h = step*16
                dst, src = (0, H-h, W, h), (0, 0, W, h)
            elif kind == 2:
                w = step*32
                dst, src = (0, 0, w, H), (W-w, 0, w, H)
            else:
                w = step*32
                dst, src = (W-w, 0, w, H), (0, 0, w, H)
            yield kind, step, dst, src


def matches(frame, target, candidates):
    found = []
    for kind, step, dst, src in candidates:
        x, y, w, h = dst
        sx, sy, _, _ = src
        anchors = [(0, 0), (w-1, h-1), (w//2, h//2)]
        if all(frame[((y+b)*W+x+a)*3:((y+b)*W+x+a)*3+3] ==
               target[((sy+b)*W+sx+a)*3:((sy+b)*W+sx+a)*3+3] for a, b in anchors):
            if part(frame, dst) == part(target, src):
                found.append((kind, step, dst, src))
    return found


def outside_equal(a, b, rect):
    x, y, w, h = rect
    return all(a[r*W*3:(r*W+x)*3] == b[r*W*3:(r*W+x)*3] and
               a[(r*W+x+w)*3:(r+1)*W*3] == b[(r*W+x+w)*3:(r+1)*W*3]
               if y <= r < y+h else a[r*W*3:(r+1)*W*3] == b[r*W*3:(r+1)*W*3]
               for r in range(H))


PLANS = {
    'SCG06': ['2', 'Return', '2', '1', '1', 'Return', '1', '5', 'Return',
              '1', 'Return', '1', 'Return', 'Return', 'y', '0', 'Return', '1', '0', '0', '0', 'Return'],
    'SCG09': ['3', 'Return', '2', '1', 'Return', '1', 'Return'],
    'SCG15': ['3', 'Return', '1'],
    'SCG20': ['2', 'Return', '1', '1', '1', 'Return', '1', '2', 'Return',
              '1', 'Return', 'Return', '0', 'Return', '0', 'Return'],
    'SCG24': ['6', 'Return', '3'],
    'SCG30': ['5', 'Return', '1', '1', 'Return'],
    'SCG31': ['4', 'Return', '1', '1', 'Return'],
}


def scene(wid, edition, name):
    target = gui.rgb(pack / (name + '.png'))
    candidates = list(geometry())
    frames, errors = [], []
    done = threading.Event()
    deadline = time.monotonic() + 45

    def collect():
        capture = Capture()
        try:
            previous = None
            while not done.is_set() and time.monotonic() < deadline:
                pixels = capture.read()
                if pixels != previous:
                    if len(frames) >= 160:
                        raise RuntimeError('動畫擷取超出有界容量')
                    frames.append((time.monotonic(), pixels))
                    previous = pixels
                if pixels == target:
                    done.set()
                time.sleep(.005)
        except Exception as err:
            errors.append(str(err))
            done.set()
        finally:
            capture.close()

    worker = threading.Thread(target=collect, daemon=True)
    worker.start()
    gui.key(wid, *PLANS[name])
    if name == 'SCG24':
        # 勸諫在賞賜名單之前；只續行正式對白與「是否繼續」詢問。
        gui.key(wid, 'space', 'y', '1', 'Return', '1', 'Return')
    continuation = 0
    while not done.wait(.5) and time.monotonic() < deadline:
        # 確認勸諫，不改變規則、日期或亂數。已有中間幀時等動畫完成。
        if frames and matches(frames[-1][1], target, candidates):
            continue
        gui.key(wid, 'space' if continuation % 2 == 0 else 'y')
        continuation += 1
    done.set()
    worker.join(timeout=5)
    gui.check(edition + '-' + name + '-capture-stopped', not worker.is_alive() and not errors)
    gui.check(edition + '-' + name + '-native-pixels', any(p == target for _, p in frames))
    partial = []
    for stamp, pixels in frames:
        match = matches(pixels, target, candidates)
        if len(match) == 1:
            kind, step, dst, src = match[0]
            partial.append((stamp, pixels, kind, step, dst, src))
    gui.check(edition + '-' + name + '-partial-frame', bool(partial))
    gui.check(edition + '-' + name + '-single-direction', len({p[2] for p in partial}) == 1)
    gui.check(edition + '-' + name + '-ordered-steps', all(a[3] < b[3] for a, b in zip(partial, partial[1:])))
    baseline = partial[0][1]
    gui.check(edition + '-' + name + '-partial-outside-unchanged',
              all(outside_equal(baseline, p[1], p[4]) for p in partial))
    captured = []
    for i, (stamp, pixels, kind, step, dst, src) in enumerate(partial):
        path = gui.OUT / f'{edition}-{name}-step-{step:02d}.rgb.gz'
        with path.open('wb') as f:
            f.write(gzip.compress(pixels, mtime=0))
        captured.append({'file': path.name, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
                         'rgb_sha256': hashlib.sha256(pixels).hexdigest(), 'kind': kind,
                         'step': step, 'destination': dst, 'source': src,
                         'seconds_from_first_frame': round(stamp-frames[0][0], 6)})
    gui.receipt.setdefault('animations', []).append({'edition': edition, 'key': name,
        'frames': captured, 'observed_direction': partial[0][2],
        'observed_steps': [p[3] for p in partial],
        'scope': '已捕獲幀的揭露區全部像素及首個捕獲幀之後的未覆蓋區；未捕獲步數不推定通過'})
    high = gui.shot(wid, edition + '-' + name + '-hd')
    theme(wid, False)
    original = gui.shot(wid, edition + '-' + name + '-original')
    container = 'DATA2' if name in {'SCG30', 'SCG31'} else 'DATA3'
    source = gui.ROOT / f'workplace/hd-inventory/{edition}/img/{container}/{name}.png'
    gui.check(edition + '-' + name + '-original-pixels', gui.rgb(original, '176:96:432:80') == gui.rgb(source))
    theme(wid, True)
    restored = gui.shot(wid, edition + '-' + name + '-hd-restored')
    gui.check(edition + '-' + name + '-hd-restored', gui.rgb(restored, '704:384:1728:320') == target)
    # 君主／對白肖像另有高清素材；文字與框線核對下方固定區。
    gui.check(edition + '-' + name + '-lower-text-frame-unchanged',
              gui.rgb(original, '224:92:408:200,scale=896:368:flags=neighbor') ==
              gui.rgb(restored, '896:368:1632:800'))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--only', choices=list(PLANS))
    parser.add_argument('--edition', choices=['base', 'plus'])
    args = parser.parse_args()
    try:
        assert (gui.OUT.stat().st_uid, gui.OUT.stat().st_gid) == (os.getuid(), os.getgid())
        os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp')
        gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp'], 'xvfb')
        gui.wait(['xdotool', 'getdisplaygeometry'])
        gui.receipt.update(method='兩版正常片頭、001 曹操、原有選單與命令，X11 實際高清中間幀',
                           scenario='001', difficulty=5,
                           randomness='正式新局預設亂數，沒有 seed／事件／人物注入；不是原版 oracle 收據')
        gui.receipt['binary_sha256'] = hashlib.sha256((gui.OUT / 'san1-window-check').read_bytes()).hexdigest()
        gui.receipt['pack_sha256'] = hashlib.sha256((pack / 'manifest.json').read_bytes()).hexdigest()
        gui.receipt['tool_sha256'] = {p.name: hashlib.sha256(p.read_bytes()).hexdigest()
            for p in [Path(__file__), Path(__file__).with_name('verify-hd-scenes.sh'),
                      Path(__file__).with_name('verify-window-inner.py')]}
        gui.receipt['input_sha256'] = {folder + '/' + name: hashlib.sha256(
            (Path('/orig') / folder / name).read_bytes()).hexdigest()
            for folder in ['三國演義', '三國演義1加強版']
            for name in ['DATA1.GRP', 'DATA2.GRP', 'DATA3.GRP']}
        for edition in [args.edition] if args.edition else ['base', 'plus']:
            for name in [args.only] if args.only else PLANS:
                proc, wid = gui.launch(edition, hd_assets=pack, tag=edition + '-' + name)
                gui.key(wid, '1', '1', '1', '2', '5')
                theme(wid, True)
                scene(wid, edition, name)
                gui.stop(proc)
        gui.receipt['passed'] = True
    finally:
        gui.receipt.setdefault('passed', False)
        for proc in reversed(gui.processes):
            gui.stop(proc)
        for log in gui.logs:
            log.close()
        (gui.OUT / 'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2) + '\n')


if __name__ == '__main__':
    main()
