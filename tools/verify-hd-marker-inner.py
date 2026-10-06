#!/usr/bin/env python3
"""兩版正常新局與對戰，回讀三語完整姓名及 Theme 切換。"""
import argparse
import ast
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import time

ROOT = Path('/src')
OUT = ROOT / os.environ['SAN1_HD_MARKER_OUT']
PACK = ROOT / os.environ['SAN1_HD_MARKER_PACK']
spec = importlib.util.spec_from_file_location('window_check', ROOT/'tools/verify-window-inner.py')
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = OUT
shot_counts = {}
original_shot = gui.shot


def unique_shot(wid, name):
    count = shot_counts.get(name, 0) + 1
    shot_counts[name] = count
    chosen = name if count == 1 else f'{name}-{count:04d}'
    if (OUT/(chosen+'.png')).exists():
        raise RuntimeError('截圖名稱已存在：'+chosen)
    return original_shot(wid, chosen)


gui.shot = unique_shot
namespace = {'gui': gui, 'time': time}
tree = ast.parse((ROOT/'tools/verify-hd-battle-branches-inner.py').read_text())
selected = [n for n in tree.body if isinstance(n, ast.FunctionDef) and
            n.name in ['flip', 'source_face', 'reach_attack_turn', 'await_prompt']]
exec(compile(ast.Module(body=selected, type_ignores=[]), 'normal-battle-input', 'exec'), namespace)
PALETTE = [(0,0,0), (0,0,170), (0,170,0), (0,170,170), (170,0,0), (170,0,170),
           (170,85,0), (170,170,170), (85,85,85), (85,85,255), (85,255,85),
           (85,255,255), (255,85,85), (255,85,255), (255,255,85), (255,255,255)]


def crop(data, width, x, y, w, h):
    return b''.join(data[((y+r)*width+x)*3:((y+r)*width+x+w)*3] for r in range(h))


def matches(actual, mask):
    for bg, ink in [(0, c) for c in range(10,14)] + [(15, c) for c in range(2,6)]:
        expected = b''.join(bytes(PALETTE[ink if mask[i] else bg])
                            for i in range(0, len(mask), 3))
        if actual == expected:
            return {'background': bg, 'ink': ink}
    return None


def marker_checks(capture, locale, positions, scale):
    data = gui.rgb(capture)
    width = 640*scale
    results = []
    for raw, col, row in positions:
        entry = next(r for r in references if r['raw'] == raw and r['locale'] == locale)
        x, y = 56+col*48, 36+row*32+16*(col%2)
        # 選項列開啟時遊戲從第 32 列開始；停點仍由正常玩家命令產生。
        actual = crop(data, width, x*scale, (y+32)*scale, 47*scale, 15*scale)
        if scale == 4:
            actual = b''.join(actual[((dy*4)*(47*4)+dx*4)*3:
                                    ((dy*4)*(47*4)+dx*4)*3+3]
                              for dy in range(15) for dx in range(47))
        result = matches(actual, masks[(raw, locale)])
        gui.check(capture.stem+'-'+raw+'-full-name', result is not None)
        results.append({'raw': raw, 'display': entry['name'], 'col': col, 'row': row,
                        'colors': result})
    return results


def resize(wid, high):
    gui.run(['xdotool', 'windowsize', wid, '2560' if high else '640',
             '1760' if high else '440'])
    gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    gui.move(wid, 320, 220)
    time.sleep(.4)


def battle(edition):
    proc, wid = gui.launch(edition, hd_assets=PACK, tag=edition)
    gui.key(wid, '1', '1', '1', '6', '5')
    waiting, rested = namespace['reach_attack_turn'](wid, edition, edition)
    gui.key(wid, '2', 'Return', '2')
    if waiting == 14:
        gui.key(wid, '1', '5', 'Return')
    gui.key(wid, '1', '1', 'Return', '2', 'Return', '1', 'Return', 'Return',
            'y', '0', 'Return', '1', '0', '0', '0', 'Return')
    namespace['await_prompt'](wid, edition+'-camp', 'camp', 2)
    gui.key(wid, '3', '3', '6', '3', '0')
    namespace['await_prompt'](wid, edition+'-command', 'command', 3)
    gui.key(wid, '2')
    namespace['await_prompt'](wid, edition+'-direction', 'engage-direction', 1, advance=False)
    gui.key(wid, '2')
    before = namespace['await_prompt'](wid, edition+'-child', 'skirmish', 2)
    data = gui.rgb(before)
    positions = []
    for raw in ['呂布', '陳宮', '曹操', '夏侯惇', '夏侯淵']:
        found = [(col, row) for col in range(12) for row in range(10)
                 if matches(crop(data, 640, 56+col*48, 36+row*32+16*(col%2), 47, 15),
                            masks[(raw, 'zh-Hant')])]
        if len(found) == 1:
            positions.append((raw, *found[0]))
    gui.check(edition+'-normal-markers', any(p[0]=='呂布' for p in positions) and
              any(len(next(r['name'] for r in references if r['raw']==p[0] and r['locale']=='en')) > 7
                  for p in positions))
    gui.key(wid, 'Escape')
    resize(wid, False)
    for index, locale in enumerate(['zh-Hant', 'en', 'ja']):
        gui.choose_ready(wid, 0, index)
        gui.choose_ready(wid, 1, 0)
        gui.move(wid, 320, 220)
        original = gui.shot(wid, edition+'-'+locale+'-original')
        markers = marker_checks(original, locale, positions, 1)
        gui.choose_ready(wid, 1, 1)
        resize(wid, True)
        high = gui.shot(wid, edition+'-'+locale+'-hd')
        marker_checks(high, locale, positions, 4)
        gui.choose_ready(wid, 1, 0)
        resize(wid, False)
        restored = gui.shot(wid, edition+'-'+locale+'-restored')
        gui.check(edition+'-'+locale+'-whole-original-restored', gui.rgb(original) == gui.rgb(restored))
        gui.receipt.setdefault('samples', []).append({'edition': edition, 'locale': locale,
            'original': original.name, 'high': high.name, 'restored': restored.name, 'markers': markers})
    gui.choose_ready(wid, 0, 0)
    gui.key(wid, 'Escape')
    gui.run(['xdotool', 'windowsize', wid, '640', '408'])
    gui.move(wid, 320, 200)
    gui.key(wid, '7')
    inspect = gui.shot(wid, edition+'-inspect')
    gui.key(wid, 'shift+Escape')
    returned = namespace['await_prompt'](wid, edition+'-returned', 'skirmish', 2, advance=False)
    gui.check(edition+'-inspect-return', gui.rgb(inspect, '176:48:448:268') !=
              gui.rgb(returned, '176:48:448:268'))
    gui.key(wid, '0')
    namespace['await_prompt'](wid, edition+'-rest', 'skirmish-rest', 1, advance=False, y=300)
    gui.key(wid, 'y')
    continued = namespace['await_prompt'](wid, edition+'-continued', 'skirmish', 2)
    gui.check(edition+'-rest-advances-clock', gui.rgb(returned, '32:96:8:228') !=
              gui.rgb(continued, '32:96:8:228'))
    gui.receipt.setdefault('plans', []).append({'edition': edition, 'player': '董卓',
        'scenario': '001', 'from': 15, 'target': 11, 'general': '呂布', 'waiting': waiting,
        'rested': rested, 'state_injection': False, 'seed_injection': False})
    gui.stop(proc)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--edition', choices=['base', 'plus'])
    args = parser.parse_args()
    if (OUT.stat().st_uid, OUT.stat().st_gid) != (os.getuid(), os.getgid()):
        raise RuntimeError('輸出擁有權不符')
    gui.receipt.update(method=__doc__, audio=False, state_injection=False,
                       binary_sha256=hashlib.sha256((OUT/'san1-window-check').read_bytes()).hexdigest(),
                       pack_sha256=hashlib.sha256((PACK/'manifest.json').read_bytes()).hexdigest())
    try:
        os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp', LP_NUM_THREADS='2')
        gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp'], 'xvfb')
        gui.wait(['xdotool', 'getdisplaygeometry'])
        for edition in [args.edition] if args.edition else ['base', 'plus']:
            battle(edition)
        gui.receipt['passed'] = True
    finally:
        gui.receipt.setdefault('passed', False)
        for proc in reversed(gui.processes):
            gui.stop(proc)
        for log in gui.logs:
            log.close()
        (OUT/'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2)+'\n')


references = json.loads((OUT/'marker-reference.json').read_text())
masks = {(r['raw'], r['locale']): gui.rgb(OUT/r['file'], '47:15:0:0') for r in references}
if __name__ == '__main__':
    main()
