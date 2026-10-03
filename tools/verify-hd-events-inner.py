#!/usr/bin/env python3
"""兩版人事、築城、射箭與退兵場景；正常片頭與命令，不注入狀態。"""
import argparse
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import threading
import time


def module(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


scenes = module('scene_check', 'verify-hd-scenes-inner.py')
branches = module('branch_check', 'verify-hd-battle-branches-inner.py')
gui = scenes.gui
branches.gui = gui
gui.OUT = gui.ROOT / 'workplace/hd-window/player/events-v8'
pack = gui.ROOT / 'workplace/hd-assets-scenes-v8'
branches.pack = pack
NAMES = ['SCG04', 'SCG05', 'SCG07', 'SCG12', 'SCG13', 'SCG17', 'SCG21', 'SCG26']


def await_main(wid, edition, tag, name):
    expected = branches.source_face(edition, name, False)
    for step in range(32):
        capture = gui.shot(wid, tag + f'-main-await-{step}')
        if gui.rgb(capture, '64:80:536:116') == expected:
            gui.check(tag + '-main-ready', True)
            return
        gui.key(wid, 'space')
    raise RuntimeError(tag + ' 尚未抵達玩家主畫面')


def reach_fort(wid, edition, tag):
    plan = next(p for p in json.loads((gui.OUT / 'fort-plan.json').read_text()) if p['edition'] == edition)
    refs = {p['prefecture']: (gui.rgb(gui.OUT / f'fort-{edition}-waiting{p["prefecture"]}-reference.png', '176:16:0:0'),
            branches.source_face(edition, p['governor_portrait'], False)) for p in plan['waiting']}
    confirmation = gui.rgb(gui.OUT / 'continue-reference.png', '128:16:0:0')
    new_governor = gui.rgb(gui.OUT / 'new-governor-reference.png', '128:16:0:0')
    rested = []
    observed = []
    for step in range(128):
        capture = gui.shot(wid, tag + f'-fort-await-{step}')
        actual = gui.rgb(capture, '176:16:424:300')
        if gui.rgb(capture, '128:16:424:300') == new_governor:
            gui.receipt.setdefault('fort_governor_stops', []).append({
                'edition': edition, 'capture': capture.name, 'selection': 'first',
                'keys': ['BackSpace', '1', 'Return']})
            gui.key(wid, 'BackSpace', '1', 'Return')
            continue
        for at, (prompt, expected) in refs.items():
            portrait = gui.rgb(capture, '64:80:536:116')
            if actual == prompt and portrait == expected:
                # 清完最後一格對白，再核對主提示與主事者；補位停點不能當成下令。
                gui.key(wid, 'space')
                stable = gui.shot(wid, tag + f'-fort-stable-{step}')
                if gui.rgb(stable, '176:16:424:300') != prompt or gui.rgb(stable, '64:80:536:116') != portrait:
                    break
                observed.append({'prefecture': at, 'portrait': next(p['governor_portrait'] for p in plan['waiting'] if p['prefecture'] == at)})
                if at == 15:
                    gui.check(tag + '-fort-turn-ready', True)
                    gui.receipt.setdefault('fort_routes', []).append({**plan,
                        'rested_prefectures': rested, 'observed_governors': observed})
                    return plan
                rested.append(at)
                gui.key(wid, '4', 'Return', '4')
                break
        else:
            gui.key(wid, 'y' if gui.rgb(capture, '128:16:424:300') == confirmation else 'space')
    raise RuntimeError(tag + ' 尚未輪到洛陽築城')


def reach_battle(wid, edition, tag, archery=False):
    waiting, rested = branches.reach_attack_turn(wid, tag, edition)
    gui.key(wid, '2', 'Return', '2')
    if waiting == 14:
        gui.key(wid, '1', '5', 'Return')
    gui.key(wid, '1', '1', 'Return', '2', 'Return', '1', 'Return', 'Return', 'y',
            '0', 'Return', '1', '0', '0', '0', 'Return')
    branches.await_prompt(wid, tag + '-camp', 'camp', 2)
    camp = ['3', '0'] if archery else ['3', '3', '6', '3', '0']
    gui.key(wid, *camp)
    branches.await_prompt(wid, tag + '-command', 'command', 3)
    gui.receipt.setdefault('battle_routes', []).append({'edition': edition, 'scene': tag,
        'waiting': waiting, 'rested_prefectures': rested, 'from': 15, 'target': 11,
        'force': [23], 'gold': 0, 'rice': 1000, 'camp_keys': camp,
        'state_injection': False})


def capture_scene(wid, edition, name, action, lower=False):
    x, y = (448, 268) if lower else (432, 80)
    target = gui.rgb(pack / (name + '.png'))
    candidates = list(scenes.geometry())
    frames, errors = [], []
    done = threading.Event()
    animating = threading.Event()
    deadline = time.monotonic() + 60

    def collect():
        capture = scenes.Capture(x*4, y*4)
        try:
            previous = None
            while not done.is_set() and time.monotonic() < deadline:
                pixels = capture.read()
                if pixels != previous:
                    if len(frames) >= 320:
                        raise RuntimeError('動畫擷取超出有界容量')
                    frames.append((time.monotonic(), pixels))
                    previous = pixels
                    if not animating.is_set() and scenes.matches(pixels, target, candidates):
                        animating.set()
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
    try:
        action()
        escape_sent = False
        retreat = gui.rgb(gui.OUT / 'retreat-list-reference.png', '176:16:0:0,scale=704:64:flags=neighbor')
        confirmation = gui.rgb(gui.OUT / 'continue-reference.png', '128:16:0:0,scale=512:64:flags=neighbor')
        while not done.wait(.3) and time.monotonic() < deadline:
            # 首個有效拉幕幀後不再續頁，終點已到也不能多送空白鍵。
            if animating.is_set() or done.is_set():
                continue
            if name == 'SCG12' and not escape_sent:
                screen = gui.shot(wid, edition + '-' + name + '-retreat-await')
                # 退兵列表佔前三行，標題在第四行 y=316。
                if gui.rgb(screen, '704:64:1792:1264') == retreat:
                    gui.key(wid, '1')
                    escape_sent = True
                    continue
            if name == 'SCG21':
                screen = gui.shot(wid, edition + '-' + name + '-advice-await')
                if gui.rgb(screen, '512:64:1696:1200') == confirmation:
                    gui.receipt.setdefault('confirmations', []).append({
                        'edition': edition, 'key': name, 'kind': 'advice-continue',
                        'capture': screen.name})
                    gui.key(wid, 'y')
                    continue
            gui.key(wid, 'space')
    finally:
        done.set()
        worker.join(timeout=5)
    tag = edition + '-' + name
    if errors or not any(p == target for _, p in frames):
        gui.shot(wid, tag + '-capture-failed')
    gui.check(tag + '-capture-stopped', not worker.is_alive() and not errors)
    gui.check(tag + '-native-pixels', any(p == target for _, p in frames))
    partial = [(stamp, pixels, *match[0]) for stamp, pixels in frames
               if len(match := scenes.matches(pixels, target, candidates)) == 1]
    gui.check(tag + '-partial-frame', bool(partial))
    gui.check(tag + '-single-direction', len({p[2] for p in partial}) == 1)
    gui.check(tag + '-ordered-steps', all(a[3] < b[3] for a, b in zip(partial, partial[1:])))
    gui.check(tag + '-outside-unchanged', all(scenes.outside_equal(partial[0][1], p[1], p[4]) for p in partial))
    captured = []
    for stamp, pixels, kind, step, dst, src in partial:
        path = gui.OUT / f'{tag}-step-{step:02d}.rgb.gz'
        path.write_bytes(gzip.compress(pixels, mtime=0))
        captured.append({'file': path.name, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
            'rgb_sha256': hashlib.sha256(pixels).hexdigest(), 'kind': kind, 'step': step,
            'destination': dst, 'source': src, 'seconds_from_first_frame': round(stamp-frames[0][0], 6)})
    gui.receipt.setdefault('animations', []).append({'edition': edition, 'key': name,
        'position': [x, y], 'frames': captured, 'observed_direction': partial[0][2],
        'observed_steps': [p[3] for p in partial],
        'scope': '已捕獲揭露區全部像素及首幀後未覆蓋區；未捕獲步數不推定通過'})
    high = gui.shot(wid, tag + '-hd')
    gui.check(tag + '-screen-native', gui.rgb(high, f'704:384:{x*4}:{y*4}') == target)
    branches.theme(wid, False)
    original = gui.shot(wid, tag + '-original')
    source = gui.ROOT / f'workplace/hd-inventory/{edition}/img/DATA3/{name}.png'
    gui.check(tag + '-original-pixels', gui.rgb(original, f'176:96:{x}:{y}') == gui.rgb(source))
    branches.theme(wid, True)
    restored = gui.shot(wid, tag + '-hd-restored')
    gui.check(tag + '-hd-restored', gui.rgb(restored, f'704:384:{x*4}:{y*4}') == target)
    # 窄版攻方肖像在左側 (456,52)，文字在右側；守方相反。
    areas = [(528, 44, 96, 96), (448, 156, 96, 96)] if lower else [(408, 200, 224, 92)]
    gui.check(tag + '-text-frame-unchanged', all(
        gui.rgb(original, f'{w}:{h}:{ax}:{ay},scale={w*4}:{h*4}:flags=neighbor') ==
        gui.rgb(restored, f'{w*4}:{h*4}:{ax*4}:{ay*4}') for ax, ay, w, h in areas))


def event(edition, name):
    tag = edition + '-' + name
    proc, wid = gui.launch(edition, hd_assets=pack, tag=tag)
    lower = name in {'SCG12', 'SCG17'}
    if lower:
        gui.key(wid, '1', '1', '1', '6', '5')
        reach_battle(wid, edition, tag, name == 'SCG17')
        action = lambda: gui.key(wid, *(['8', 'y'] if name == 'SCG12' else ['5', '3']))
    elif name == 'SCG21':
        gui.key(wid, '1', '3', '1', '2', '5')
        plan = reach_fort(wid, edition, tag)
        def action():
            gui.key(wid, '4', 'Return', '3', '1', 'Return')
            gui.shot(wid, tag + '-fort-location')
            gui.key(wid, *plan['cursor_keys'], '0')
            gui.shot(wid, tag + '-fort-confirm')
            gui.key(wid, 'y')
    else:
        gui.key(wid, '1', '1', '1', '1' if name == 'SCG05' else '2', '5')
        await_main(wid, edition, tag, 'F005' if name == 'SCG05' else 'F000')
        keys = {'SCG04': ['6', 'Return', '2', '2', 'Return'],
                'SCG07': ['6', 'Return', '2', '2', 'Return'],
                'SCG05': ['7', 'Return', '1', '1', 'Return'],
                'SCG13': ['7', 'Return', '5', '1', '5', 'Return', '1', 'Return'],
                'SCG26': ['6', 'Return', '4', '1', 'Return']}[name]
        action = lambda: gui.key(wid, *keys)
    branches.theme(wid, True)
    capture_scene(wid, edition, name, action, lower)
    gui.stop(proc)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--only', choices=NAMES)
    parser.add_argument('--edition', choices=['base', 'plus'])
    args = parser.parse_args()
    try:
        if (gui.OUT.stat().st_uid, gui.OUT.stat().st_gid) != (os.getuid(), os.getgid()):
            raise RuntimeError('輸出擁有權不符')
        os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp')
        gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp'], 'xvfb')
        gui.wait(['xdotool', 'getdisplaygeometry'])
        gui.receipt.update(method='正常片頭、新局、人事、合法築城、洛陽呂布出兵與射箭／退兵',
            difficulty=5, randomness='正式新局預設亂數；沒有 seed／人物／金額／戰鬥注入；不是原版 oracle',
            binary_sha256=hashlib.sha256((gui.OUT / 'san1-window-check').read_bytes()).hexdigest(),
            pack_sha256=hashlib.sha256((pack / 'manifest.json').read_bytes()).hexdigest())
        gui.receipt['tool_sha256'] = {p.name: hashlib.sha256(p.read_bytes()).hexdigest()
            for p in Path(__file__).parent.glob('*') if p.name in {
                'verify-hd-events-inner.py', 'verify-hd-events.sh', 'hd-events-reference.go',
                'verify-hd-scenes-inner.py', 'verify-hd-battle-branches-inner.py',
                'verify-window-inner.py', 'hd-battle-branches-reference.go'}}
        gui.receipt['reference_sha256'] = {p.name: hashlib.sha256(p.read_bytes()).hexdigest()
            for p in gui.OUT.glob('*-reference.png')}
        gui.receipt['engine_source_sha256'] = {name: hashlib.sha256(
            (gui.ROOT / name).read_bytes()).hexdigest()
            for name in ['cmd/san1/main.go', 'cmd/san1/governor_test.go']}
        gui.receipt['input_sha256'] = {folder + '/' + name: hashlib.sha256(
            (Path('/orig') / folder / name).read_bytes()).hexdigest()
            for folder in ['三國演義', '三國演義1加強版']
            for name in ['DATA1.GRP', 'DATA2.GRP', 'DATA3.GRP']}
        for edition in [args.edition] if args.edition else ['base', 'plus']:
            for name in [args.only] if args.only else NAMES:
                event(edition, name)
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
