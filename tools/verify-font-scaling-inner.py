#!/usr/bin/env python3
"""容器內從正常主選單驗證兩版、兩個 theme 的楷書與隸書縮放。"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import time

from PIL import Image, ImageChops


def nearest_mismatches(source, actual):
    """逐像素核對原生取樣；恰落 texel 邊界時容許兩側任一原色，不容許混色。"""
    sw, sh = source.size
    aw, ah = actual.size

    def indices(pos, old, new):
        q, r = divmod((2 * pos + 1) * old, 2 * new)
        return {max(0, min(old - 1, v)) for v in ([q] if r else [q - 1, q])}

    src, dst = source.load(), actual.load()
    left, top, right, bottom = (v * aw // 640 for v in [176, 224, 312, 240])
    bad = 0
    for y in range(top, bottom):
        ys = indices(y, sh, ah)
        for x in range(left, right):
            choices = {src[sx, sy] for sx in indices(x, sw, aw) for sy in ys}
            bad += dst[x, y] not in choices
    return bad


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True)
    parser.add_argument('--binary', default='workplace/voice-check-build/san1-window-check')
    parser.add_argument('--editions', nargs='+', choices=['base', 'plus'], default=['base', 'plus'])
    args = parser.parse_args()
    root = Path('/src')
    out = root / args.out
    if not out.is_relative_to(root / 'workplace/audio') or '..' in out.parts:
        raise ValueError('輸出須位於 workplace/audio/')
    out.mkdir()
    if (out.stat().st_uid, out.stat().st_gid) != (os.getuid(), os.getgid()):
        raise ValueError('輸出擁有權不符')
    spec = importlib.util.spec_from_file_location('gui', root / 'tools/verify-window-inner.py')
    gui = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(gui)
    gui.OUT = out
    gui.receipt['passed'] = False
    gui.receipt['method'] = '正常片頭／主選單切換楷書隸書；原生文字區最近鄰比較'
    gui.receipt['samples'] = []
    shutil.copy2(root / args.binary, out / 'san1-window-check')
    shutil.copy2(root / 'workplace/voice-check-build/title-reference.png', out / 'title-reference.png')
    gui.receipt['binary_sha256'] = hashlib.sha256((out / 'san1-window-check').read_bytes()).hexdigest()
    os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp')

    def resize(wid, width):
        gui.run(['xdotool', 'windowsize', wid, str(width), str(width * 408 // 640)])
        gui.run(['xdotool', 'windowmove', wid, '0', '0'])
        gui.move(wid, 638, 400)
        time.sleep(.3)

    def text(image):
        width, _ = image.size
        return image.crop(tuple(v * width // 640 for v in [176, 224, 312, 240]))

    try:
        gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp'], 'xvfb')
        gui.wait(['xdotool', 'getdisplaygeometry'])
        for edition in args.editions:
            proc, wid = gui.launch(edition, hd_assets=root / 'workplace/hd-assets-mapcursor-v50-r1')
            for theme in ['original', 'hd']:
                if theme == 'hd':
                    resize(wid, 640)
                    gui.key(wid, 'Escape')
                    gui.choose_ready(wid, 1, 1)
                    gui.key(wid, 'Escape')
                    gui.move(wid, 638, 400)
                native = 640 if theme == 'original' else 2560
                for key, face in [('3', 'kai'), ('4', 'li')]:
                    resize(wid, native)
                    gui.key(wid, key)
                    tag = f'{edition}-{theme}-{face}'
                    path = gui.shot(wid, tag + '-native')
                    source = Image.open(path).convert('RGB')
                    for width in [640, 960, 1280, 2560]:
                        resize(wid, width)
                        actual_path = gui.shot(wid, tag + f'-{width}')
                        actual = Image.open(actual_path).convert('RGB')
                        bad = nearest_mismatches(source, actual)
                        equal = bad == 0
                        gui.check(tag + f'-{width}-nearest', equal)
                        gui.receipt['samples'].append({'edition': edition, 'theme': theme, 'face': face,
                            'width': width, 'native': path.name, 'actual': actual_path.name,
                            'text_rect_640': [176, 224, 312, 240], 'mismatches': bad, 'equal': equal})
                    if face == 'kai':
                        resize(wid, native)
                        gui.key(wid, '4', '3')
                        again = Image.open(gui.shot(wid, tag + '-restored')).convert('RGB')
                        gui.check(tag + '-font-restored',
                                  ImageChops.difference(text(source), text(again)).getbbox() is None)
            gui.stop(proc)
        gui.receipt['passed'] = True
    finally:
        for proc in reversed(gui.processes):
            gui.stop(proc)
        for log in gui.logs:
            log.close()
        (out / 'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
