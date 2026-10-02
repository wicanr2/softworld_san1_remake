#!/usr/bin/env python3
"""正常新局 → 出兵 → 宣戰；不注入人物、戰役、事件或亂數。"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import time

spec = importlib.util.spec_from_file_location('window_check', Path(__file__).with_name('verify-window-inner.py'))
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = gui.ROOT / 'workplace/hd-window/player/dialogue-v4'
pack = gui.ROOT / 'workplace/hd-assets-portraits-v4'
gui.receipt.update(method='Linux Xvfb 正常片頭、新局、陳留出兵洛陽、整編、攜帶錢糧與宣戰',
                   scenario='001', player=1, difficulty=5,
                   formation={'general': 13, 'army_choice': 1}, gold=0, rice=1000,
                   randomness='正式新局預設亂數；不是原版 oracle 收據')


def flipped(pixels):
    return b''.join(pixels[(y*256+x)*3:(y*256+x+1)*3]
                    for y in range(320) for x in range(255, -1, -1))


def original_face(name, mirror=False):
    pixels = gui.rgb(gui.ROOT / f'workplace/hd-inventory/base/img/DATA3/{name}.png')
    if mirror:
        return b''.join(pixels[(y*64+x)*3:(y*64+x+1)*3]
                        for y in range(80) for x in range(63, -1, -1))
    return pixels


def theme(wid, high):
    gui.key(wid, 'Escape')
    gui.choose_ready(wid, 1, int(high))
    gui.key(wid, 'Escape')
    gui.run(['xdotool', 'windowsize', wid, '2560' if high else '640', '1632' if high else '408'])
    gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    gui.move(wid, 320, 200)
    time.sleep(.5)


def await_face(wid, edition, name, x, y, mirror=False, advance=False):
    target = original_face(name, mirror)
    for step in range(8):
        shot = gui.shot(wid, f'{edition}-{name}-await-{step}')
        if gui.rgb(shot, f'64:80:{x}:{y}') == target:
            return
        if advance:
            gui.key(wid, 'space')
        else:
            time.sleep(.4)
    raise RuntimeError(f'{edition} 正常路徑尚未看到 {name} 對白')


def portrait(wid, edition, name, x, y, mirror=False):
    tag = edition + '-' + name
    before = gui.shot(wid, tag + '-original')
    theme(wid, True)
    after = gui.shot(wid, tag + '-hd')
    expected = gui.rgb(pack / (name + '.png'))
    if mirror:
        expected = flipped(expected)
    gui.check(tag + '-native-mirrored-pixels' if mirror else tag + '-native-pixels',
              gui.rgb(after, f'256:320:{x*4}:{y*4}') == expected)
    # 只排除實際存在的主畫面及上下對白肖像；其餘文字、姓名與框線相同。
    original = gui.rgb(before, '224:256:408:36,scale=896:1024:flags=neighbor')
    high = gui.rgb(after, '896:1024:1632:144')
    rectangles = [(536, 116), (552, 80)] + ([(424, 180)] if mirror else [])
    outside = True
    for row in range(1024):
        intervals = sorted(((rx-408)*4, (rx-408+64)*4) for rx, ry in rectangles
                           if (ry-36)*4 <= row < (ry-36+80)*4)
        at = 0
        for left, right in intervals + [(896, 896)]:
            if left > at:
                a, b = (row*896+at)*3, (row*896+left)*3
                outside &= original[a:b] == high[a:b]
            at = max(at, right)
    gui.check(tag + '-text-frame-unchanged', outside)
    theme(wid, False)
    restored = gui.shot(wid, tag + '-restored')
    gui.check(tag + '-original-restored', gui.rgb(before, '224:256:408:36') ==
              gui.rgb(restored, '224:256:408:36'))


try:
    assert (gui.OUT.stat().st_uid, gui.OUT.stat().st_gid) == (os.getuid(), os.getgid())
    os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp')
    gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp'], 'xvfb')
    gui.wait(['xdotool', 'getdisplaygeometry'])
    gui.receipt['binary_sha256'] = hashlib.sha256((gui.OUT / 'san1-window-check').read_bytes()).hexdigest()
    gui.receipt['pack_sha256'] = hashlib.sha256((pack / 'manifest.json').read_bytes()).hexdigest()
    gui.receipt['tool_sha256'] = {p.name: hashlib.sha256(p.read_bytes()).hexdigest()
        for p in [Path(__file__), Path(__file__).with_name('verify-hd-dialogue.sh'),
                  Path(__file__).with_name('verify-window-inner.py')]}
    gui.receipt['input_sha256'] = {folder + '/' + name: hashlib.sha256(
        (Path('/orig') / folder / name).read_bytes()).hexdigest()
        for folder in ['三國演義', '三國演義1加強版']
        for name in ['DATA1.GRP', 'DATA2.GRP', 'DATA3.GRP']}
    for edition in ['base', 'plus']:
        gui.receipt.setdefault('edition_key_start', {})[edition] = len(gui.receipt['keys']) if 'keys' in gui.receipt else 0
        proc, wid = gui.launch(edition, hd_assets=pack)
        gui.key(wid, '1', '1', '1', '2', '5')
        gui.shot(wid, edition + '-main')
        # 軍事→發動戰役；來源陳留 11、目標洛陽 15；曹操分到第一軍。
        gui.key(wid, '2', 'Return', '2', '1', '1', 'Return', '1', '5', 'Return',
                '1', 'Return', '1', 'Return', 'Return', 'y', '0', 'Return',
                '1', '0', '0', '0', 'Return')
        await_face(wid, edition, 'F000', 552, 80, advance=True)
        portrait(wid, edition, 'F000', 552, 80)
        gui.key(wid, 'space')
        await_face(wid, edition, 'F077', 424, 180, mirror=True)
        portrait(wid, edition, 'F077', 424, 180, mirror=True)
        gui.stop(proc)
    gui.receipt['passed'] = True
finally:
    gui.receipt.setdefault('passed', False)
    for proc in reversed(gui.processes):
        gui.stop(proc)
    for log in gui.logs:
        log.close()
    (gui.OUT / 'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2) + '\n')
