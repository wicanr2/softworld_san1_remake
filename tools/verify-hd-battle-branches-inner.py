#!/usr/bin/env python3
"""董卓正常新局、洛陽呂布攻陳留的子畫面與快戰；不注入戰鬥狀態。"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import time

spec = importlib.util.spec_from_file_location('window_check', Path(__file__).with_name('verify-window-inner.py'))
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = gui.ROOT / os.environ.get('SAN1_HD_BRANCHES_OUT', 'workplace/hd-window/player/battle-branches-v7')
pack = gui.ROOT / os.environ.get('SAN1_HD_BRANCHES_PACK', 'workplace/hd-assets-scenes-v7')
weather_enabled = any(e['container'] == 'DATA1' and e['name'].startswith('WEATHER')
                      for e in json.loads((pack / 'manifest.json').read_text())['entries'])
FACES = [('F006', 456, 52, True), ('F000', 552, 164, False)]
PANELS = [(448, 44, 176, 96), (448, 156, 176, 96)]


def flip(pixels, width, height):
    return b''.join(pixels[(y*width+x)*3:(y*width+x+1)*3]
                    for y in range(height) for x in range(width-1, -1, -1))


def source_face(edition, name, mirror):
    data = gui.rgb(gui.ROOT / f'workplace/hd-inventory/{edition}/img/DATA3/{name}.png')
    return flip(data, 64, 80) if mirror else data


def theme(wid, high):
    gui.key(wid, 'Escape')
    gui.choose_ready(wid, 1, int(high))
    gui.key(wid, 'Escape')
    gui.run(['xdotool', 'windowsize', wid, '2560' if high else '640', '1632' if high else '408'])
    gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    gui.move(wid, 320, 200)
    time.sleep(.5)


def await_prompt(wid, tag, reference, rows, advance=True, x=448, y=268):
    expected = gui.rgb(gui.OUT / (reference + '-reference.png'), f'176:{rows*16}:0:0')
    confirmation = gui.rgb(gui.OUT / 'continue-reference.png', '128:16:0:0')
    for step in range(24):
        capture = gui.shot(wid, tag + f'-await-{step}')
        if gui.rgb(capture, f'176:{rows*16}:{x}:{y}') == expected:
            gui.check(tag + '-prompt-ready', True)
            return capture
        if advance:
            gui.key(wid, 'y' if gui.rgb(capture, '128:16:424:300') == confirmation else 'space')
        else:
            time.sleep(.3)
    raise RuntimeError(tag + ' 尚未抵達正式提示')


def reach_attack_turn(wid, tag, edition):
    references = {at: gui.rgb(gui.OUT / f'waiting{at}-reference.png', '176:16:0:0')
                  for at in (14, 15, 16)}
    confirmation = gui.rgb(gui.OUT / 'continue-reference.png', '128:16:0:0')
    governors = {14: 'F077', 15: 'F160', 16: 'F147'}
    portraits = {at: source_face(edition, name, False) for at, name in governors.items()}
    rested = [6]
    gui.key(wid, '4', 'Return', '4')
    for step in range(64):
        capture = gui.shot(wid, tag + f'-turn-await-{step}')
        actual = gui.rgb(capture, '176:16:424:300')
        for at, expected in references.items():
            if actual == expected and gui.rgb(capture, '64:80:536:116') == portraits[at]:
                if at in (14, 15):
                    gui.check(tag + f'-waiting{at}-ready', True)
                    return at, rested
                rested.append(at)
                gui.key(wid, '4', 'Return', '4')
                break
        else:
            gui.key(wid, 'y' if gui.rgb(capture, '128:16:424:300') == confirmation else 'space')
    raise RuntimeError(tag + ' 尚未輪到可出兵的郡')


def await_resolution(wid, tag):
    references = [(name, rows, gui.rgb(gui.OUT / f'{name}-reference.png', f'176:{rows*16}:0:0'))
                  for name, rows in [('command', 3), ('captive', 2)]]
    for step in range(24):
        capture = gui.shot(wid, tag + f'-await-{step}')
        for name, rows, expected in references:
            if gui.rgb(capture, f'176:{rows*16}:448:268') == expected:
                gui.check(tag + '-' + name + '-ready', True)
                return capture, name
        gui.key(wid, 'space')
    raise RuntimeError(tag + ' 尚未抵達快戰後的玩家停點')


def outside_faces(before, after, rectangle, faces):
    x, y, width, height = rectangle
    original = gui.rgb(before, f'{width}:{height}:{x}:{y},scale={width*4}:{height*4}:flags=neighbor')
    high = gui.rgb(after, f'{width*4}:{height*4}:{x*4}:{y*4}')
    for row in range(height*4):
        spans = sorted((max(0, (fx-x)*4), min(width*4, (fx-x+64)*4))
                       for _, fx, fy, _ in faces
                       if (fy-y)*4 <= row < (fy-y+80)*4 and fx < x+width and fx+64 > x)
        at = 0
        for left, right in spans + [(width*4, width*4)]:
            if left > at:
                a, b = (row*width*4+at)*3, (row*width*4+left)*3
                if original[a:b] != high[a:b]:
                    return False
            at = max(at, right)
    return True


def verify(wid, edition, tag, faces, panels):
    original = gui.shot(wid, tag + '-original')
    weather_key = None
    if weather_enabled:
        actual = gui.rgb(original, '32:32:8:155')
        matches = [f'WEATHER{n}' for n in range(3) if actual == gui.rgb(
            gui.ROOT / f'workplace/hd-inventory/{edition}/img/DATA1/WEATHER{n}.png')]
        gui.check(tag + '-weather-source', len(matches) == 1)
        weather_key = matches[0]
    for name, x, y, mirror in faces:
        gui.check(tag + '-' + name + '-source',
                  gui.rgb(original, f'64:80:{x}:{y}') == source_face(edition, name, mirror))
    theme(wid, True)
    high = gui.shot(wid, tag + '-hd')
    if weather_key:
        gui.check(tag + '-weather-native', gui.rgb(high, '128:128:32:620') == gui.rgb(pack / (weather_key + '.png')))
        expected = gui.rgb(original, '56:70:0:140,scale=224:280:flags=neighbor')
        actual = gui.rgb(high, '224:280:0:560')
        gui.check(tag + '-weather-frame-text-unchanged', all(
            expected[(y*224+x)*3:(y*224+x+1)*3] == actual[(y*224+x)*3:(y*224+x+1)*3]
            for y in range(280) for x in range(224) if not (32 <= x < 160 and 60 <= y < 188)))
    for name, x, y, mirror in faces:
        expected = gui.rgb(pack / (name + '.png'))
        if mirror:
            expected = flip(expected, 256, 320)
        gui.check(tag + '-' + name + '-native',
                  gui.rgb(high, f'256:320:{x*4}:{y*4}') == expected)
    gui.check(tag + '-text-frame-unchanged', all(outside_faces(original, high, p, faces) for p in panels))
    theme(wid, False)
    restored = gui.shot(wid, tag + '-restored')
    if weather_key:
        gui.check(tag + '-weather-original-restored', gui.rgb(original, '56:70:0:140') == gui.rgb(restored, '56:70:0:140'))
        gui.receipt.setdefault('weather_samples', []).append({'edition': edition, 'stage': tag,
            'key': 'DATA1/' + weather_key + '.IMG', 'source_rect': [8,155,32,32],
            'high_rect': [32,620,128,128], 'original': str(original.relative_to(gui.ROOT)),
            'high': str(high.relative_to(gui.ROOT)), 'restored': str(restored.relative_to(gui.ROOT))})
    gui.check(tag + '-original-restored', all(
        gui.rgb(original, f'{w}:{h}:{x}:{y}') == gui.rgb(restored, f'{w}:{h}:{x}:{y}')
        for x, y, w, h in panels))
    gui.verify_hd_terrain(original, high, restored, pack, tag, edition)
    return original


def battle(edition, mode):
    tag = edition + '-' + mode
    proc, wid = gui.launch(edition, hd_assets=pack, tag=tag)
    gui.key(wid, '1', '1', '1', '6', '5')
    waiting, rested = reach_attack_turn(wid, tag, edition)
    gui.key(wid, '2', 'Return', '2')
    if waiting == 14:
        gui.key(wid, '1', '5', 'Return')
    gui.key(wid, '1', '1', 'Return',
            '2', 'Return', '1', 'Return', 'Return', 'y', '0', 'Return',
            '1', '0', '0', '0', 'Return')
    await_prompt(wid, tag + '-camp', 'camp', 2)
    gui.key(wid, '3', '3', '6', '3', '0')
    await_prompt(wid, tag + '-command', 'command', 3)
    before = verify(wid, edition, tag + '-before', FACES, PANELS)
    direction = 'engage-direction' if mode == 'skirmish' else 'quick-direction'
    gui.key(wid, '2' if mode == 'skirmish' else '3')
    await_prompt(wid, tag + '-direction', direction, 1, advance=False)
    gui.key(wid, '2')
    gui.receipt.setdefault('battle_plan', []).append({'edition': edition, 'mode': mode,
        'player': '董卓', 'scenario': '001', 'waiting': waiting, 'first_waiting': 6,
        'rested_prefectures': rested, 'from': 15, 'target': 11,
        'force': [23], 'leader': '呂布', 'group': 1, 'gold': 0, 'rice': 1000,
        'camp_keys': ['3', '3', '6', '3', '0'], 'attack_direction': '2',
        'state_injection': False})
    if mode == 'skirmish':
        await_prompt(wid, tag + '-child', 'skirmish', 2)
        verify(wid, edition, tag + '-child', FACES, PANELS + [(448, 268, 176, 48)])
        gui.key(wid, '7')
        inspector = [('F006', 552, 276, False)]
        verify(wid, edition, tag + '-inspect', inspector, PANELS + [(448, 268, 176, 96)])
        gui.key(wid, 'shift+Escape')
        await_prompt(wid, tag + '-returned', 'skirmish', 2, advance=False)
        returned = verify(wid, edition, tag + '-returned', FACES, PANELS)
        gui.key(wid, '0')
        await_prompt(wid, tag + '-rest-confirm', 'skirmish-rest', 1, advance=False, y=300)
        gui.key(wid, 'y')
        continued = await_prompt(wid, tag + '-continued', 'skirmish', 2)
        gui.check(tag + '-continued-clock-changed',
                  gui.rgb(returned, '32:96:8:228') != gui.rgb(continued, '32:96:8:228'))
        verify(wid, edition, tag + '-continued', FACES, PANELS)
    else:
        after, stop = await_resolution(wid, tag + '-resolved')
        gui.check(tag + '-resolution-panel-changed', any(
            gui.rgb(before, f'{w}:{h}:{x}:{y}') != gui.rgb(after, f'{w}:{h}:{x}:{y}')
            for x, y, w, h in PANELS))
        verify(wid, edition, tag + '-resolved', FACES, PANELS)
        captives = 0
        while stop == 'captive':
            if captives >= 10:
                raise RuntimeError(tag + ' 俘虜裁決超過上限')
            gui.key(wid, '3')
            captives += 1
            _, stop = await_resolution(wid, tag + f'-resume-{captives}')
        gui.check(tag + '-command-resumed', stop == 'command')
        gui.receipt.setdefault('quick_continuation', []).append({
            'edition': edition, 'released_captives': captives, 'resumed': stop})
    gui.stop(proc)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--edition', choices=['base', 'plus'])
    parser.add_argument('--mode', choices=['skirmish', 'quick'])
    args = parser.parse_args()
    try:
        if (gui.OUT.stat().st_uid, gui.OUT.stat().st_gid) != (os.getuid(), os.getgid()):
            raise RuntimeError('輸出擁有權不符')
        os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp')
        gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp'], 'xvfb')
        gui.wait(['xdotool', 'getdisplaygeometry'])
        gui.receipt.update(method='兩版正常董卓新局、洛陽呂布攻陳留、合法紮寨、對戰子畫面與快戰',
            scenario='001', difficulty=5, randomness='正式新局預設亂數；未注入 seed／人物／事件／戰鬥，不是原版 oracle')
        gui.receipt['binary_sha256'] = hashlib.sha256((gui.OUT / 'san1-window-check').read_bytes()).hexdigest()
        gui.receipt['pack_sha256'] = hashlib.sha256((pack / 'manifest.json').read_bytes()).hexdigest()
        gui.receipt['tool_sha256'] = {p.name: hashlib.sha256(p.read_bytes()).hexdigest()
            for p in [Path(__file__), Path(__file__).with_name('verify-hd-battle-branches.sh'),
                      Path(__file__).with_name('verify-window-inner.py'),
                      Path(__file__).with_name('hd-battle-branches-reference.go')]}
        gui.receipt['reference_sha256'] = {p.name: hashlib.sha256(p.read_bytes()).hexdigest()
            for p in gui.OUT.glob('*-reference.png')}
        gui.receipt['input_sha256'] = {folder + '/' + name: hashlib.sha256(
            (Path('/orig') / folder / name).read_bytes()).hexdigest()
            for folder in ['三國演義', '三國演義1加強版']
            for name in ['DATA1.GRP', 'DATA2.GRP', 'DATA3.GRP']}
        for edition in [args.edition] if args.edition else ['base', 'plus']:
            for mode in [args.mode] if args.mode else ['skirmish', 'quick']:
                battle(edition, mode)
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
