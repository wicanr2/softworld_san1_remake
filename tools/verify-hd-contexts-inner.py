#!/usr/bin/env python3
"""正常新局的尋訪、寬／窄主戰場及查看。"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import time

spec = importlib.util.spec_from_file_location('window_check', Path(__file__).with_name('verify-window-inner.py'))
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = gui.ROOT / 'workplace/hd-window/player/contexts-v6'
pack = gui.ROOT / 'workplace/hd-assets-portraits-v6'
gui.receipt.update(method='Linux Xvfb 正常片頭、新局、尋訪及出兵；正常預設紮寨及查看',
                   scenario='001', player=1, difficulty=5,
                   randomness='正式新局預設亂數；不是原版 oracle 收據')


def flip(pixels, width, height):
    return b''.join(pixels[(y*width+x)*3:(y*width+x+1)*3]
                    for y in range(height) for x in range(width-1, -1, -1))


def source_face(name, mirror):
    pixels = gui.rgb(gui.ROOT / f'workplace/hd-inventory/base/img/DATA3/{name}.png')
    return flip(pixels, 64, 80) if mirror else pixels


def theme(wid, high):
    gui.key(wid, 'Escape')
    gui.choose_ready(wid, 1, int(high))
    gui.key(wid, 'Escape')
    gui.run(['xdotool', 'windowsize', wid, '2560' if high else '640', '1632' if high else '408'])
    gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    gui.move(wid, 320, 200)
    time.sleep(.5)


def await_face(wid, tag, face, advance=False):
    name, x, y, mirror = face
    target = source_face(name, mirror)
    for step in range(12):
        capture = gui.shot(wid, tag + f'-await-{step}')
        if gui.rgb(capture, f'64:80:{x}:{y}') == target:
            return
        if advance:
            gui.key(wid, 'space')
        else:
            time.sleep(.4)
    raise RuntimeError(tag + ' 正常路徑尚未抵達指定肖像')


def outside_faces(before, after, rectangle, faces):
    x, y, width, height = rectangle
    original = gui.rgb(before, f'{width}:{height}:{x}:{y},scale={width*4}:{height*4}:flags=neighbor')
    high = gui.rgb(after, f'{width*4}:{height*4}:{x*4}:{y*4}')
    for row in range(height*4):
        intervals = sorted((max(0, (fx-x)*4), min(width*4, (fx-x+64)*4))
                           for _, fx, fy, _ in faces
                           if (fy-y)*4 <= row < (fy-y+80)*4 and fx < x+width and fx+64 > x)
        at = 0
        for left, right in intervals + [(width*4, width*4)]:
            if left > at:
                a, b = (row*width*4+at)*3, (row*width*4+left)*3
                if original[a:b] != high[a:b]:
                    return False
            at = max(at, right)
    return True


def verify(wid, tag, faces, panels):
    original = gui.shot(wid, tag + '-original')
    for face in faces:
        name, x, y, mirror = face
        gui.check(tag + '-' + name + f'-source-{x}-{y}',
                  gui.rgb(original, f'64:80:{x}:{y}') == source_face(name, mirror))
    theme(wid, True)
    high = gui.shot(wid, tag + '-hd')
    for name, x, y, mirror in faces:
        expected = gui.rgb(pack / (name + '.png'))
        if mirror:
            expected = flip(expected, 256, 320)
        gui.check(tag + '-' + name + f'-native-{x}-{y}',
                  gui.rgb(high, f'256:320:{x*4}:{y*4}') == expected)
    gui.check(tag + '-text-frame-unchanged', all(outside_faces(original, high, p, faces) for p in panels))
    theme(wid, False)
    restored = gui.shot(wid, tag + '-restored')
    gui.check(tag + '-original-restored', all(
        gui.rgb(original, f'{w}:{h}:{x}:{y}') == gui.rgb(restored, f'{w}:{h}:{x}:{y}')
        for x, y, w, h in panels))


def new_game(edition, tag):
    proc, wid = gui.launch(edition, hd_assets=pack, tag=tag)
    gui.key(wid, '1', '1', '1', '2', '5')
    return proc, wid


def search(edition):
    tag = edition + '-search'
    proc, wid = new_game(edition, tag)
    gui.key(wid, '6', 'Return', '1', '1', 'Return')
    found = ('F210', 488, 88, False)
    await_face(wid, tag + '-found', found)
    verify(wid, tag + '-found', [found], [(408, 36, 224, 256)])
    gui.key(wid, 'space')
    report = ('F000', 552, 180, False)
    await_face(wid, tag + '-report', report)
    verify(wid, tag + '-report', [report], [(408, 36, 224, 256)])
    gui.stop(proc)


def battle(edition, target):
    narrow = target == 4
    layout = 'narrow' if narrow else 'wide'
    tag = edition + '-' + layout
    proc, wid = new_game(edition, tag)
    gui.key(wid, '2', 'Return', '2', '1', '1', 'Return', *str(target), 'Return',
            '1', 'Return', '1', 'Return', 'Return', 'y', '0', 'Return',
            '1', '0', '0', '0', 'Return')
    faces = [('F000', 456, 52, True), ('F111', 552, 164, False)] if narrow else [
             ('F000', 72, 276, True), ('F006', 360, 276, False)]
    panels = [(448, 44, 176, 96), (448, 156, 176, 96)] if narrow else [
              (64, 268, 176, 96), (256, 268, 176, 96)]
    await_face(wid, tag + '-camp', faces[0], advance=True)
    # 戰場肖像也會出現在進場對白。先收完對白，再用正式 9 鍵完成預設紮寨。
    gui.key(wid, *(['space']*9), '9')
    gui.receipt.setdefault('battle_plan', []).append({
        'edition': edition, 'target': target, 'layout': layout,
        'camp_keys': ['9'],
        'attacker': 'F000', 'defender': faces[1][0]})
    verify(wid, tag + '-command', faces, panels)
    gui.key(wid, '7')
    inspector = ('F000', 552, 276, False)
    await_face(wid, tag + '-inspect', inspector)
    # 窄版查看頁會遮住上方軍力面板；遮蔽區也須完整保留原貌。
    visible = [inspector] if narrow else faces + [inspector]
    verify(wid, tag + '-inspect', visible, panels + [(448, 268, 176, 96)])
    gui.stop(proc)


try:
    if (gui.OUT.stat().st_uid, gui.OUT.stat().st_gid) != (os.getuid(), os.getgid()):
        raise RuntimeError('輸出擁有權不符')
    os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp')
    gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp'], 'xvfb')
    gui.wait(['xdotool', 'getdisplaygeometry'])
    gui.receipt['binary_sha256'] = hashlib.sha256((gui.OUT / 'san1-window-check').read_bytes()).hexdigest()
    gui.receipt['pack_sha256'] = hashlib.sha256((pack / 'manifest.json').read_bytes()).hexdigest()
    gui.receipt['tool_sha256'] = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in [
        Path(__file__), Path(__file__).with_name('verify-hd-contexts.sh'),
        Path(__file__).with_name('verify-window-inner.py')]}
    gui.receipt['input_sha256'] = {folder + '/' + name: hashlib.sha256(
        (Path('/orig') / folder / name).read_bytes()).hexdigest()
        for folder in ['三國演義', '三國演義1加強版']
        for name in ['DATA1.GRP', 'DATA2.GRP', 'DATA3.GRP']}
    for edition in ['base', 'plus']:
        gui.receipt.setdefault('edition_key_start', {})[edition] = len(gui.receipt.get('keys', []))
        search(edition)
        for target in [4, 15]:
            battle(edition, target)
    gui.receipt['passed'] = True
finally:
    gui.receipt.setdefault('passed', False)
    for proc in reversed(gui.processes):
        gui.stop(proc)
    for log in gui.logs:
        log.close()
    (gui.OUT / 'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2) + '\n')
