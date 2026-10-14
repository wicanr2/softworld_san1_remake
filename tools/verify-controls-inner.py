#!/usr/bin/env python3
"""Docker/Xvfb：正常新局的操作模式、Enter、換月場景與行軍動畫。"""
import argparse
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import time

from PIL import Image

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out', required=True)
parser.add_argument('--binary', required=True)
parser.add_argument('--package-root', type=Path, help='正式封包的解包根目錄，使用包內資料、HD 素材及字型')
parser.add_argument('--editions', nargs='+', default=['base', 'plus'], choices=['base', 'plus'])
parser.add_argument('--months', type=int, default=18)
parser.add_argument('--hd-march-only', action='store_true', help='只驗正常高清換月的行軍與路線')
parser.add_argument('--command-gate-only', action='store_true', help='只驗明確指令前後的換月條件')
args = parser.parse_args()
ROOT = Path('/src')
OUT = ROOT / args.out
assert OUT.is_relative_to(ROOT / 'workplace') and not OUT.exists()
assert OUT.parent.stat().st_uid == os.getuid()
OUT.mkdir()
spec = importlib.util.spec_from_file_location('gui', ROOT / 'tools/verify-window-inner.py')
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = OUT
gui.receipt.update(method='正式片頭、新局、設定列、鍵盤與滑鼠、正常休息換月；無局面注入',
                   source_binary_sha256=hashlib.sha256((ROOT / args.binary).read_bytes()).hexdigest())
shutil.copy2(ROOT / args.binary, OUT / 'san1-window-check')
shutil.copy2(ROOT / 'workplace/promo-source/v.1.1.1-20261010/title-reference.png', OUT / 'title-reference.png')
if args.package_root:
    package = args.package_root.resolve()
    assert all((package / name).is_file() for name in
               ['san1', 'fonts/unifont.hex.gz', 'fonts/kai.ttf', 'fonts/li.ttf', 'hd-assets/expanded-portraits.json'])
    assert all((package / 'game' / edition).is_dir() for edition in args.editions)
    assert hashlib.sha256((package / 'san1').read_bytes()).hexdigest() == gui.receipt['source_binary_sha256']
    original_start = gui.start

    def package_start(command, name):
        if command[0] == str(OUT / 'san1-window-check'):
            command = list(command)
            edition = command[command.index('-edition') + 1]
            command[command.index('-root') + 1] = str(package / 'game' / edition)
            command[command.index('-hd-assets') + 1] = str(package / 'hd-assets')
            command += ['-font', str(package / 'fonts/unifont.hex.gz')]
        return original_start(command, name)

    gui.start = package_start
    gui.receipt.update(package_root=str(package), state_injection=False, seed_injection=False,
                       expanded_hd_manifest_sha256=hashlib.sha256((package / 'hd-assets/expanded-portraits.json').read_bytes()).hexdigest())


def image(path):
    with Image.open(path) as im:
        return im.convert('RGB')


def region(path, box):
    return image(path).crop(box).tobytes()


def click(wid, x, y):
    gui.move(wid, x, y)
    gui.run(['xdotool', 'mousedown', '1'])
    time.sleep(.25)
    gui.run(['xdotool', 'mouseup', '1'])
    time.sleep(.3)
    # 截圖會收進 X11 游標，移出比較的右側面板再擷取。
    gui.move(wid, 320, 200)


def toolbar(wid, field, row):
    gui.key(wid, 'Escape')
    gui.choose_ready(wid, field, row)
    gui.key(wid, 'Escape')
    gui.move(wid, 320, 200)


def save(wid, edition):
    # Enter 先回到同一個主提示輸入 owner，再選其他／儲存。
    folder = OUT / ('saves-' + edition)
    previous = {p: p.stat().st_mtime_ns for p in folder.glob('**/REMAKE.JSON')}
    gui.key(wid, 'Return', '9', 'Return', '2', '1', 'Return')
    files = list(folder.glob('**/REMAKE.JSON'))
    gui.check(edition + '-save-created', bool(files) and any(p.stat().st_mtime_ns != previous.get(p) for p in files))
    data = json.loads(max(files, key=lambda p: p.stat().st_mtime_ns).read_text())
    data.pop('saved_at', None)
    return data


def command_page(wid):
    # 明確選 0 狀態，再以 Enter 切回命令表。
    gui.key(wid, 'Return', '0', 'Return', 'Return')


patterns = {}
for word in ['請對', '出兵中', '休息', '是否']:
    patterns[word] = []
    for name in ['unifont.hex.gz', 'kai.hex.gz', 'li.hex.gz']:
        glyphs = {}
        with gzip.open(ROOT / 'fonts' / name, 'rt') as f:
            for line in f:
                if ':' not in line:
                    continue
                code, bitmap = line.strip().split(':', 1)
                if int(code, 16) in {ord(c) for c in word}:
                    glyphs[chr(int(code, 16))] = bytes.fromhex(bitmap)
        if len(glyphs) != len(set(word)):
            continue
        rows = []
        for y in range(16):
            rows.append(bytes((glyphs[c][y*2+x//8] >> (7-x%8)) & 1
                              for c in word for x in range(16)))
        patterns[word].append(rows)


def find_word(im, word):
    area = im.crop((424, 300, 624, 380))
    bits = bytes(int(rgb == (255, 255, 85)) for rgb in area.getdata())
    width, height = area.size
    for rows in patterns[word]:
        anchor = max(range(16), key=lambda y: sum(rows[y]))
        offset = 0
        while (at := bits.find(rows[anchor], offset)) >= 0:
            offset = at + 1
            x, top = at % width, at // width - anchor
            if x + len(rows[0]) > width or not 0 <= top <= height-16:
                continue
            if all(bits[(top+y)*width+x:(top+y)*width+x+len(row)] == row
                   for y, row in enumerate(rows)):
                return (424+x+len(rows[0])//2, 300+top+8)
    return None


def has_word(im, word):
    return find_word(im, word) is not None


def command_gate(wid, edition):
    def capture(label):
        path = gui.shot(wid, label)
        return path, image(path).resize((640, 408), Image.Resampling.NEAREST)

    def date(im):
        return im.crop((8, 36, 40, 384)).tobytes()

    for theme, label in [(0, 'original'), (1, 'hd')]:
        tag = edition + '-gate-' + label
        toolbar(wid, 1, theme)
        scale = 4 if theme else 1
        gui.run(['xdotool', 'windowsize', wid, str(640*scale), str(408*scale)])
        gui.run(['xdotool', 'windowmove', wid, '0', '0'])
        before = save(wid, edition)
        command_page(wid)
        _, first = capture(tag + '-before')
        gui.check(tag + '-main-prompt-ready', has_word(first, '請對'))
        for _ in range(3):
            gui.key(wid, 'Return', 'KP_Enter')
        time.sleep(1.5)
        click(wid, 352, 390)  # 舊通用確定位置；本輪主提示沒有此按鈕。
        click(wid, 300, 380)  # 遊戲區空白處。
        click(wid, 448, 68)   # 狀態。
        click(wid, 112, 390)  # 指令，只顯示命令表。
        _, untouched = capture(tag + '-after-navigation')
        gui.check(tag + '-navigation-keeps-date', date(first) == date(untouched))
        command_page(wid)
        click(wid, 448, 260)  # 內政。
        _, child = capture(tag + '-civil-menu')
        gui.check(tag + '-opening-menu-keeps-date', date(first) == date(child) and has_word(child, '休息'))
        gui.run(['xdotool', 'click', '3'])  # 取消內政。
        time.sleep(.4)
        gui.check(tag + '-navigation-keeps-full-save', before == save(wid, edition))
        command_page(wid)
        click(wid, 448, 260)
        _, child = capture(tag + '-before-rest')
        position = find_word(child, '休息')
        gui.check(tag + '-named-rest-visible', position is not None)
        click(wid, *position)
        deadline = time.monotonic() + 120
        step = 0
        while time.monotonic() < deadline:
            path, current = capture(tag + '-rest-' + str(step))
            step += 1
            # 對白仍可能保留下方舊主提示；須等上面板也回復主畫面。
            if has_word(current, '請對') and current.getpixel((420, 56)) == first.getpixel((420, 56)) and date(current) != date(first):
                gui.check(tag + '-named-rest-advances-month', True)
                gui.receipt.setdefault('command_gates', []).append({'edition': edition, 'theme': label,
                    'file': path.name, 'method': '明確點選內政、休息及必要 Y 確認後，回到下一月玩家主提示'})
                break
            if has_word(current, '是否'):
                click(wid, 128, 358)  # 明確按「是」。
            else:
                gui.key(wid, 'space')  # 只續行已提交指令後的對白。
        else:
            raise RuntimeError(tag + ' 明確休息後未回到下一月')
        after = save(wid, edition)
        expected = (before['year'] + (before['month'] == 12), before['month'] % 12 + 1)
        gui.check(tag + '-saved-next-month', (after['year'], after['month']) == expected)


scene_refs = {}
for edition in args.editions:
    folder = ROOT / 'workplace/asset-overviews' / edition / 'img/DATA3'
    if not folder.is_dir() and edition == 'plus':
        base = Path('/orig/三國演義/DATA3.GRP')
        plus = Path('/orig/三國演義1加強版/DATA3.GRP')
        digest = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
        assert digest(base) == digest(plus), '兩版 DATA3 不同，不能共用場景參考'
        folder = ROOT / 'workplace/asset-overviews/base/img/DATA3'
        gui.receipt['plus_scene_reference'] = {'method': '兩版 DATA3 原始 bytes 完全相同', 'sha256': digest(base)}
    assert folder.is_dir()
    scene_refs[edition] = {image(p).tobytes(): p.stem for p in folder.glob('SCG*.png')}


def monthly(wid, edition):
    observed = {'scene': False, 'march': False}
    captures = 0
    toolbar(wid, 2, 1)
    for month in range(args.months):
        gui.key(wid, 'Return', '4', 'Return', '4', 'y')
        deadline = time.monotonic() + 100
        while time.monotonic() < deadline:
            path = gui.shot(wid, f'{edition}-month-{month:02d}-{captures:03d}')
            captures += 1
            im = image(path)
            scene = scene_refs[edition].get(im.crop((432, 80, 608, 176)).tobytes())
            if scene and not observed['scene']:
                blue = [(420, 56), (620, 200), (480, 260)]
                gui.check(edition + '-normal-scene-cleared', all(im.getpixel(p) == (0, 0, 170) for p in blue))
                observed['scene'] = True
                gui.receipt.setdefault('normal_scenes', []).append({'edition': edition, 'scene': scene, 'file': path.name})
                toolbar(wid, 1, 1)
                gui.run(['xdotool', 'windowsize', wid, '2560', '1632'])
                gui.run(['xdotool', 'windowmove', wid, '0', '0'])
                gui.shot(wid, edition + '-normal-scene-hd')
                toolbar(wid, 1, 0)
                gui.run(['xdotool', 'windowsize', wid, '640', '408'])
            if has_word(im, '出兵中') and not observed['march']:
                first = im.crop((56, 36, 400, 364)).tobytes()
                samples = [path.name]
                for step in range(8):
                    time.sleep(.2)
                    other = gui.shot(wid, f'{edition}-march-{step}')
                    samples.append(other.name)
                    after = image(other)
                    if has_word(after, '出兵中') and after.crop((56, 36, 400, 364)).tobytes() != first:
                        observed['march'] = True
                        break
                gui.check(edition + '-normal-march-moves', observed['march'])
                gui.receipt.setdefault('normal_marches', []).append({'edition': edition, 'files': samples})
            if has_word(im, '請對') and im.getpixel((420, 56)) != (0, 0, 170):
                break
            gui.key(wid, 'space')
        else:
            raise RuntimeError(edition + ' 換月未回到玩家主提示')
        print(edition, 'month', month+1, observed, flush=True)
        if all(observed.values()):
            break
    gui.check(edition + '-normal-month-scene-observed', observed['scene'])
    gui.check(edition + '-normal-auto-war-observed', observed['march'])


def hd_march(wid, edition):
    toolbar(wid, 1, 1)
    gui.run(['xdotool', 'windowsize', wid, '2560', '1632'])
    gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    toolbar(wid, 2, 1)
    for month in range(args.months):
        gui.key(wid, 'Return', '4', 'Return', '4', 'y')
        deadline = time.monotonic() + 100
        capture = 0
        while time.monotonic() < deadline:
            path = gui.shot(wid, f'{edition}-hd-month-{month:02d}-{capture:03d}')
            capture += 1
            im = image(path)
            logical = im.resize((640, 408), Image.Resampling.NEAREST)
            if has_word(logical, '出兵中'):
                first = im.crop((224, 144, 1600, 1456)).tobytes()
                files = [path.name]
                for step in range(8):
                    time.sleep(.2)
                    other = gui.shot(wid, f'{edition}-hd-march-{step}')
                    files.append(other.name)
                    after = image(other)
                    if has_word(after.resize((640, 408), Image.Resampling.NEAREST), '出兵中') and after.crop((224, 144, 1600, 1456)).tobytes() != first:
                        gui.check(edition + '-normal-hd-march-moves', True)
                        gui.receipt.setdefault('normal_hd_marches', []).append({'edition': edition, 'files': files})
                        return
                raise RuntimeError(edition + ' 高清行軍未出現連續動作')
            if has_word(logical, '請對'):
                break
            gui.key(wid, 'space')
        else:
            raise RuntimeError(edition + ' 高清換月未回到玩家主提示')
    raise RuntimeError(edition + ' 未觀測到正常高清行軍')


try:
    os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp')
    gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp', '-noreset', '-ac'], 'xvfb')
    gui.wait(['xdotool', 'getdisplaygeometry'])
    for edition in args.editions:
        proc, wid = gui.launch(edition, hd_assets=ROOT / 'hd-assets', startup_seconds=90 if args.package_root else 60)
        if args.command_gate_only:
            toolbar(wid, 2, 1)
            gui.key(wid, '1', '1', '1', '2', '5')
            command_gate(wid, edition)
            gui.stop(proc)
            continue
        if args.hd_march_only:
            gui.key(wid, '1', '1', '1', '2', '5')
            hd_march(wid, edition)
            gui.stop(proc)
            continue
        gui.key(wid, 'Escape')
        for row, lang in enumerate(['zh-Hant', 'en', 'ja']):
            gui.choose_ready(wid, 0, row)
            # 以鍵盤開 AI 欄位，再移到說明區查看。
            gui.key(wid, 'Tab', 'Tab', 'Return')
            gui.move(wid, 20, 210)
            path = gui.shot(wid, edition + '-ai-help-' + lang)
            gui.check(edition + '-ai-help-visible-' + lang, image(path).getpixel((10, 260)) == (24, 29, 38))
            gui.key(wid, 'Return')
        gui.choose_ready(wid, 0, 0)
        gui.choose_ready(wid, 2, 5)
        gui.key(wid, 'Escape')
        gui.key(wid, '1', '1', '1', '2', '5')
        time.sleep(.5)
        before_save = save(wid, edition)
        before = gui.shot(wid, edition + '-before-enter')
        gui.key(wid, 'Return')
        command = gui.shot(wid, edition + '-after-enter')
        gui.key(wid, 'KP_Enter')
        restored = gui.shot(wid, edition + '-enter-restored')
        upper = (408, 36, 632, 292)
        gui.check(edition + '-enter-changes-panel', region(before, upper) != region(command, upper))
        gui.check(edition + '-enter-restores-panel', region(before, upper) == region(restored, upper))
        date = (8, 36, 40, 384)
        gui.check(edition + '-enter-keeps-date', region(before, date) == region(command, date) == region(restored, date))
        toolbar(wid, 3, 0)
        command_page(wid)
        before = gui.shot(wid, edition + '-keyboard-only-before-click')
        click(wid, 448, 68)
        blocked = gui.shot(wid, edition + '-keyboard-only-after-click')
        gui.check(edition + '-keyboard-only-blocks-mouse', region(before, upper) == region(blocked, upper))
        gui.key(wid, '0', 'Return')
        physical = gui.shot(wid, edition + '-keyboard-only-keyboard-works')
        gui.check(edition + '-keyboard-still-works', region(before, upper) != region(physical, upper))
        toolbar(wid, 3, 1)
        command_page(wid)
        before = gui.shot(wid, edition + '-mixed-before-click')
        click(wid, 448, 68)
        after = gui.shot(wid, edition + '-mixed-after-click')
        gui.check(edition + '-mixed-restores-mouse', region(before, upper) != region(after, upper))
        gui.check(edition + '-controls-keep-full-save', before_save == save(wid, edition))
        monthly(wid, edition)
        gui.stop(proc)
    gui.receipt['passed'] = True
finally:
    gui.receipt.setdefault('passed', False)
    for proc in reversed(gui.processes):
        gui.stop(proc)
    for log in gui.logs:
        log.close()
    (OUT / 'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2)+'\n')
