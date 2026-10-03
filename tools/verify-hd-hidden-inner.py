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
gui.OUT = gui.ROOT / 'workplace/hd-window/player/hidden-v18'
pack = gui.ROOT / 'workplace/hd-assets-portraits-v18'
gui.receipt.update(method='Linux Xvfb 正常片頭、006 劉備新局、六郡休息後漢中尋訪姜維',
                   scenario='006', player=0, difficulty=5,
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
        if gui.rgb(capture, '160:16:424:300') == gui.rgb(gui.ROOT / 'workplace/hd-v18-continue-reference.png', '160:16:424:300'):
            gui.key(wid, 'y')
            continue
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


def search(edition):
    tag = edition + '-jiang-wei'
    proc, wid = gui.launch(edition, hd_assets=pack, tag=tag)
    gui.key(wid, '1', '6', '1', '1', '5')
    rests = [36, 32, 39, 37, 38, 30]
    for at in rests:
        gui.key(wid, *(['space']*12), '4', 'Return', '4')
        gui.key(wid, *(['space']*12), 'y')
        gui.shot(wid, tag + '-rest-' + str(at))
    gui.key(wid, *(['space']*12), '6', 'Return', '1', '1', 'Return')
    found = ('F019', 488, 88, False)
    await_face(wid, tag + '-found', found, advance=True)
    verify(wid, tag + '-found', [found], [(408, 36, 224, 256)])
    gui.receipt.setdefault('normal_search_plan', []).append({
        'edition': edition, 'scenario': '006', 'lord': '劉備', 'player': 0,
        'difficulty': 5, 'rests': rests, 'prefecture': 18, 'actor': '馬良',
        'actor_choice': 1, 'found': '姜維', 'portrait': 'F019',
        'state_injection': False, 'seed_override': False})
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
        Path(__file__), Path(__file__).with_name('verify-hd-hidden.sh'),
        Path(__file__).with_name('verify-window-inner.py')]}
    for edition in ['base', 'plus']:
        gui.receipt.setdefault('edition_key_start', {})[edition] = len(gui.receipt.get('keys', []))
        search(edition)
    gui.receipt['passed'] = True
finally:
    gui.receipt.setdefault('passed', False)
    for proc in reversed(gui.processes):
        gui.stop(proc)
    for log in gui.logs:
        log.close()
    (gui.OUT / 'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2) + '\n')
