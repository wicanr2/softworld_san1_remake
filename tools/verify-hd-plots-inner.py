#!/usr/bin/env python3
"""兩版三種謀略的正常新局與命令；不注入人物、事件或 seed。"""
import argparse
import hashlib
import json
import os
from pathlib import Path

import importlib.util

spec = importlib.util.spec_from_file_location('event_check', Path(__file__).with_name('verify-hd-events-inner.py'))
events = importlib.util.module_from_spec(spec)
spec.loader.exec_module(events)
gui = events.gui
gui.OUT = gui.ROOT / 'workplace/hd-window/player/plots-v9'
pack = gui.ROOT / 'workplace/hd-assets-scenes-v9'
events.pack = events.branches.pack = pack
NAMES = ['SCG18', 'SCG23', 'SCG22']
KEYS = {'SCG18': ['8', 'Return', '2', '2', '7', 'Return', '2', '9', 'Return', '3', '1', 'Return', '1', 'Return'],
        'SCG23': ['8', 'Return', '1', '2', 'Return', '3', 'Return', '1', 'Return'],
        'SCG22': ['8', 'Return', '4', '2', 'Return', '1', 'Return']}


def await_main(wid, edition, tag, previous_date=None):
    prompt = gui.rgb(gui.OUT / f'plots-{edition}-main-reference.png', '176:16:0:0')
    portrait = events.branches.source_face(edition, 'F041', False)
    for step in range(64):
        shot = gui.shot(wid, tag + f'-main-await-{step}')
        date = gui.rgb(shot, '24:280:24:65')
        if (gui.rgb(shot, '176:16:424:300') == prompt and
            gui.rgb(shot, '64:80:536:116') == portrait and
            (previous_date is None or date != previous_date)):
            gui.key(wid, 'space')
            stable = gui.shot(wid, tag + f'-main-stable-{step}')
            if (gui.rgb(stable, '176:16:424:300') == prompt and
                gui.rgb(stable, '64:80:536:116') == portrait and
                gui.rgb(stable, '24:280:24:65') == date):
                gui.check(tag + '-main-ready', True)
                gui.receipt.setdefault('main_stops', []).append({'edition': edition, 'tag': tag,
                    'capture': stable.name, 'date_rgb_sha256': hashlib.sha256(date).hexdigest(),
                    'month_changed': previous_date is not None, 'portrait': 'F041', 'prefecture': 31})
                return date
        gui.key(wid, 'space')
    raise RuntimeError(tag + ' 尚未抵達長沙下令提示')


def plot(edition, name):
    tag = edition + '-' + name
    plan = next(p for p in json.loads((gui.OUT / 'plots-plan.json').read_text()) if p['edition'] == edition)
    proc, wid = gui.launch(edition, hd_assets=pack, tag=tag)
    gui.key(wid, '1', '1', '1', '3', '5')
    date = await_main(wid, edition, tag + '-initial')
    gui.key(wid, '7', 'Return', '1', '1', 'Return')
    await_main(wid, edition, tag + '-appointed', date)
    gui.receipt.setdefault('plot_routes', []).append({**plan, 'scene': name, 'plot_plan': plan[name],
        'keys': KEYS[name], 'method': '正常片頭、新局、任命軍師、月份交接與謀略選單'})
    events.branches.theme(wid, True)
    events.capture_scene(wid, edition, name, lambda: gui.key(wid, *KEYS[name]),
                         position=(432, 120) if name == 'SCG22' else (432, 80),
                         allow_confirmation=True)
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
        gui.receipt.update(method=__doc__, difficulty=5,
            randomness='正式新局預設亂數；沒有 seed／人物／金額／事件注入；不是原版 oracle',
            binary_sha256=hashlib.sha256((gui.OUT / 'san1-window-check').read_bytes()).hexdigest(),
            pack_sha256=hashlib.sha256((pack / 'manifest.json').read_bytes()).hexdigest())
        names = ['verify-hd-plots-inner.py', 'verify-hd-plots.sh', 'hd-plots-reference.go',
                 'verify-hd-events-inner.py', 'verify-hd-scenes-inner.py',
                 'verify-hd-battle-branches-inner.py', 'verify-window-inner.py',
                 'hd-battle-branches-reference.go', 'hd-events-reference.go']
        gui.receipt['tool_sha256'] = {name: hashlib.sha256(Path(__file__).with_name(name).read_bytes()).hexdigest()
                                     for name in names}
        gui.receipt['reference_sha256'] = {p.name: hashlib.sha256(p.read_bytes()).hexdigest()
                                          for p in gui.OUT.glob('*-reference.png')}
        gui.receipt['engine_source_sha256'] = {'cmd/san1/main.go': hashlib.sha256(
            (gui.ROOT / 'cmd/san1/main.go').read_bytes()).hexdigest()}
        gui.receipt['plan_sha256'] = hashlib.sha256((gui.OUT / 'plots-plan.json').read_bytes()).hexdigest()
        gui.receipt['input_sha256'] = {folder + '/' + name: hashlib.sha256(
            (Path('/orig') / folder / name).read_bytes()).hexdigest()
            for folder in ['三國演義', '三國演義1加強版']
            for name in ['DATA1.GRP', 'DATA2.GRP', 'DATA3.GRP']}
        for edition in [args.edition] if args.edition else ['base', 'plus']:
            for name in [args.only] if args.only else NAMES:
                plot(edition, name)
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
