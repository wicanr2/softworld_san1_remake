#!/usr/bin/env python3
"""限容器：正常開局、人物卡、宣戰對白及存讀檔的向量字體抽樣。"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import time

from PIL import Image, ImageChops

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out', required=True)
parser.add_argument('--binary', required=True)
parser.add_argument('--editions', nargs='+', default=['base', 'plus'], choices=['base', 'plus'])
args = parser.parse_args()
root = Path('/src')
out = root / args.out
if not out.resolve().is_relative_to((root / 'workplace/audio').resolve()) or out.exists():
    raise ValueError('輸出須為 workplace/audio/ 下的新目錄')
out.mkdir()
assert (out.stat().st_uid, out.stat().st_gid) == (os.getuid(), os.getgid())
spec = importlib.util.spec_from_file_location('gui', root / 'tools/verify-window-inner.py')
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = out
for source, name in [(root / args.binary, 'san1-window-check'),
                     (root / 'workplace/voice-check-build/title-reference.png', 'title-reference.png')]:
    shutil.copy2(source, out / name)
gui.receipt.update(method=__doc__, binary_sha256=hashlib.sha256((out/'san1-window-check').read_bytes()).hexdigest(),
                   state_injection=False, seed_injection=False, samples=[], passed=False)
gui.receipt['pack_sha256'] = hashlib.sha256((root/'workplace/hd-assets-mapcursor-v50-r1/manifest.json').read_bytes()).hexdigest()
os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp')


def resize(wid, high):
    gui.run(['xdotool', 'windowsize', wid, '2560' if high else '640', '1632' if high else '408'])
    gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    gui.move(wid, 638, 400)
    time.sleep(.4)


def theme(wid, high):
    resize(wid, False)
    gui.key(wid, 'Escape')
    gui.choose_ready(wid, 1, int(high))
    gui.key(wid, 'Escape')
    resize(wid, high)


def sample(wid, tag):
    gui.move(wid, 638, 400)
    original = gui.shot(wid, tag+'-original')
    theme(wid, True)
    high = gui.shot(wid, tag+'-hd')
    theme(wid, False)
    restored = gui.shot(wid, tag+'-restored')
    a, b = Image.open(original).convert('RGB'), Image.open(restored).convert('RGB')
    rect = (408, 36, 632, 292)
    gui.check(tag+'-original-text-restored', ImageChops.difference(a.crop(rect), b.crop(rect)).getbbox() is None)
    text_rect = (424, 52, 528, 84) if tag.endswith('-person') else (408, 36, 528, 252)
    hd_rect = tuple(v*4 for v in text_rect)
    old_text = a.crop(text_rect).resize((hd_rect[2]-hd_rect[0], hd_rect[3]-hd_rect[1]), Image.Resampling.NEAREST)
    gui.check(tag+'-vector-text-visible', ImageChops.difference(old_text, Image.open(high).convert('RGB').crop(hd_rect)).getbbox() is not None)
    if tag.endswith('-person'):
        portrait = root/'workplace/hd-inventory'/tag.split('-')[0]/'img/DATA3/F005.png'
        gui.check(tag+'-person-card-visible', gui.rgb(original, '64:80:536:68') == gui.rgb(portrait))
    gui.receipt['samples'].append(dict(tag=tag, original=original.name, high=high.name, restored=restored.name,
                                      original_comparison_rect=rect,
                                      excluded='只比較完整右上面板；排除地圖閃爍及下方面板的動畫輸入游標'))


def click(wid, x, y, button=1):
    gui.move(wid, x, y)
    gui.run(['xdotool', 'mousedown', str(button)])
    time.sleep(.12)
    gui.run(['xdotool', 'mouseup', str(button)])
    time.sleep(.4)


try:
    gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp'], 'xvfb')
    gui.wait(['xdotool', 'getdisplaygeometry'])
    for edition in args.editions:
        for font, key, value in [('kai', '3', 1), ('li', '4', 2)]:
            tag = edition+'-'+font
            proc, wid = gui.launch(edition, tag=tag, hd_assets=root/'workplace/hd-assets-mapcursor-v50-r1')
            gui.key(wid, key, '1', '1', '1', '1', '5')
            time.sleep(1)
            sample(wid, tag+'-main')
            gui.key(wid, '1', 'Return', '3')
            gui.shot(wid, tag+'-roster')
            gui.key(wid, '1', 'Return')
            sample(wid, tag+'-person')
            click(wid, 320, 200, 3)
            click(wid, 320, 200, 3)
            gui.key(wid, '9', '2', '1', 'Return')
            gui.shot(wid, tag+'-saved')
            files = list((out/('saves-'+edition)).glob('**/REMAKE.JSON'))
            gui.check(tag+'-font-saved', any(json.loads(p.read_text()).get('options', {}).get('font') == value for p in files))
            # 正常玩家命令：齊郡向北海宣戰，等拉幕完成再擷取對白。
            gui.key(wid, '2', '2', '8', 'Return', '7', 'Return',
                    '1', 'Return', '1', 'Return', 'Return', 'y', '0', 'Return', '1', '0', '0', 'Return')
            target = root/'workplace/hd-inventory'/edition/'img/DATA3/SCG06.png'
            expected = gui.rgb(target)
            deadline = time.monotonic()+45
            while time.monotonic() < deadline:
                path = gui.shot(wid, tag+'-war-await-'+str(len(gui.receipt['captures'])))
                if gui.rgb(path, '176:96:432:80') == expected:
                    break
                gui.key(wid, 'space')
            else:
                raise RuntimeError(tag+'未進入宣戰場景')
            time.sleep(.5)
            gui.key(wid, 'space')
            time.sleep(5)
            sample(wid, tag+'-dialogue')
            gui.stop(proc)
            # -load 是正式玩家入口，讀取剛才由 UI 寫出的存檔。
            folder = '三國演義' if edition == 'base' else '三國演義1加強版'
            command = [str(out/'san1-window-check'), '-root', '/orig/'+folder, '-edition', edition,
                       '-ai', edition, '-load', '1', '-scale', '1', '-music=false', '-sound=false',
                       '-saves', str(out/('saves-'+edition)), '-hd-assets', str(root/'workplace/hd-assets-mapcursor-v50-r1')]
            loaded = gui.start(command, tag+'-load')
            loaded_wid = gui.wait(['xdotool', 'search', '--onlyvisible', '--pid', str(loaded.pid), '--name', '三國演義 remake']).splitlines()[0]
            time.sleep(1)
            gui.move(loaded_wid, 638, 400)
            loaded_original = gui.shot(loaded_wid, tag+'-loaded-original')
            theme(loaded_wid, True)
            loaded_high = gui.shot(loaded_wid, tag+'-loaded-hd')
            for mode, image_path, scale in [('original', loaded_original, 1), ('hd', loaded_high, 4)]:
                before = out/(tag+'-main-'+mode+'.png')
                rect = tuple(v*scale for v in (408, 36, 528, 252))
                left = Image.open(before).convert('RGB').crop(rect)
                right = Image.open(image_path).convert('RGB').crop(rect)
                gui.check(tag+'-loaded-'+mode+'-font-restored', ImageChops.difference(left, right).getbbox() is None)
            gui.stop(loaded)
    gui.receipt['passed'] = True
finally:
    for proc in reversed(gui.processes):
        gui.stop(proc)
    for log in gui.logs:
        log.close()
    (out/'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2)+'\n')
