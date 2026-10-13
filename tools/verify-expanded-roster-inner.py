#!/usr/bin/env python3
"""限容器：正常選單選新君主、查看人物及格式 2 存讀檔；不注入局面。"""
import argparse
import base64
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
parser.add_argument('--editions', nargs='+', choices=['base', 'plus'], default=['base', 'plus'])
parser.add_argument('--scenarios', nargs='+', type=int, choices=[1, 2, 6], default=[1, 2, 6])
parser.add_argument('--hd-proof', help='正式載入器輸出的新增肖像驗證圖目錄')
parser.add_argument('--difficulty', type=int, choices=range(1, 10), default=5)
parser.add_argument('--package-root', type=Path, help='正式封包的解包根目錄；使用其中的兩版資料、HD 素材與字型')
parser.add_argument('--hd-assets', type=Path, help='驗證本機替代 HD 素材包，須與 package-root 分開使用')
args = parser.parse_args()
if args.package_root and args.hd_assets:
    parser.error('--package-root 與 --hd-assets 不能同時指定')
root = Path('/src')
out = root / args.out
if not out.resolve().is_relative_to((root / 'workplace/audio').resolve()) or out.exists():
    raise ValueError('輸出須為 workplace/audio/ 下的新目錄')
assert out.parent.is_dir() and out.parent.stat().st_uid == os.getuid()
out.mkdir()
spec = importlib.util.spec_from_file_location('gui', root / 'tools/verify-window-inner.py')
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = out
original_wait = gui.wait


def wait_for_window(args, seconds=20):
    # 全部新肖像載入後才建立視窗；保留有界等待，勿把 20 秒工具上限當成崩潰。
    if args[:2] == ['xdotool', 'search']:
        seconds = max(seconds, 90)
    return original_wait(args, seconds)


gui.wait = wait_for_window
for source, name in [(root / args.binary, 'san1-window-check'),
                     (root / 'workplace/voice-check-build/title-reference.png', 'title-reference.png')]:
    assert source.is_file()
    shutil.copy2(source, out / name)
gui.receipt.update(method=__doc__, state_injection=False, seed_injection=False, passed=False,
                   binary_sha256=hashlib.sha256((out / 'san1-window-check').read_bytes()).hexdigest(), samples=[])
gui.receipt['window_wait_seconds'] = 90
if args.hd_assets:
    art = args.hd_assets.resolve()
    assert all((art / name).is_file() for name in ['manifest.json', 'expanded-portraits.json'])
    original_art_start = gui.start

    def art_start(command, name):
        if command[0] == str(out / 'san1-window-check'):
            command = list(command)
            command[command.index('-hd-assets') + 1] = str(art)
        return original_art_start(command, name)

    gui.start = art_start
    gui.receipt['hd_assets'] = str(art)
    gui.receipt['expanded_hd_manifest_sha256'] = hashlib.sha256((art / 'expanded-portraits.json').read_bytes()).hexdigest()
if args.package_root:
    package = args.package_root.resolve()
    assert all((package / path).is_file() for path in ['san1', 'fonts/unifont.hex.gz', 'fonts/kai.ttf', 'fonts/li.ttf', 'hd-assets/expanded-portraits.json'])
    assert all((package / 'game' / edition).is_dir() for edition in args.editions)
    assert hashlib.sha256((package / 'san1').read_bytes()).hexdigest() == gui.receipt['binary_sha256']
    original_start = gui.start

    def package_start(command, name):
        if command[0] == str(out / 'san1-window-check'):
            command = list(command)
            edition = command[command.index('-edition') + 1]
            command[command.index('-root') + 1] = str(package / 'game' / edition)
            command[command.index('-hd-assets') + 1] = str(package / 'hd-assets')
            command += ['-font', str(package / 'fonts/unifont.hex.gz')]
        return original_start(command, name)

    gui.start = package_start
    gui.receipt['package_root'] = str(package)
    gui.receipt['package_hd_manifest_sha256'] = hashlib.sha256((package / 'hd-assets/expanded-portraits.json').read_bytes()).hexdigest()
os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp')


def key(wid, *keys):
    gui.run(['xdotool', 'windowfocus', '--sync', wid])
    for k in keys:
        gui.run(['xdotool', 'keydown', '--clearmodifiers', k])
        time.sleep(.25)
        gui.run(['xdotool', 'keyup', k])
        time.sleep(.5)
        gui.receipt.setdefault('keys', []).append(k)


try:
    gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp'], 'xvfb')
    gui.wait(['xdotool', 'getdisplaygeometry'])
    for edition in args.editions:
        for scenario, pages, pick, name, city, count in [(1, 2, '3', '張角', 5, 570),
                                                       (2, 2, '4', '王朗', 23, 557),
                                                       (6, 0, '5', '徹里吉', 20, 518)]:
            if scenario not in args.scenarios:
                continue
            tag = f'{edition}-{scenario:03d}'
            saves = out / ('saves-' + edition)
            assert not saves.exists(), '每個正常新局須有獨立存檔目錄'
            proc, wid = gui.launch(edition, tag=tag, hd_assets=root / 'hd-assets')
            key(wid, '1')
            gui.shot(wid, tag + '-scenario-picker')
            key(wid, str(scenario))
            gui.shot(wid, tag + '-player-count')
            key(wid, '1')
            for _ in range(pages):
                key(wid, 'Right')
            gui.shot(wid, tag + '-new-lord-page')
            key(wid, pick)
            gui.shot(wid, tag + '-difficulty-picker')
            key(wid, str(args.difficulty))
            # 電腦先行可能留下死亡／戰役對白。Space 只關對白，不輸入主命令。
            deadline = time.monotonic() + 60
            while True:
                current = gui.shot(wid, tag + '-main')
                with Image.open(current) as im:
                    ready = im.getpixel((420, 250))[:3] == (0, 170, 170)
                if ready:
                    break
                if time.monotonic() >= deadline:
                    raise RuntimeError(tag + ' 未停在玩家主提示，保留畫面供判讀')
                key(wid, 'space')
            # 正常回合已停在玩家首府。主提示收命令 0–9，並非郡號。
            key(wid, '1', 'Return', '3')
            gui.shot(wid, tag + '-roster')
            key(wid, '1', 'Return')
            gui.shot(wid, tag + '-person')
            if args.hd_proof and scenario in [1, 6]:
                key(wid, 'Escape')
                gui.choose_ready(wid, 1, 1)
                key(wid, 'Escape')
                gui.run(['xdotool', 'windowsize', wid, '2560', '1632'])
                gui.run(['xdotool', 'windowmove', wid, '0', '0'])
                time.sleep(1)
                high = gui.shot(wid, tag + '-hd-person')
                portrait = 250 if scenario == 1 else 139
                proof = root / args.hd_proof / f'{edition}-{portrait:04d}.png'
                gui.check(tag + '-hd-native-portrait', gui.rgb(high, '256:320:2144:272') == gui.rgb(proof))
                key(wid, 'Escape')
                gui.choose_ready(wid, 1, 0)
                key(wid, 'Escape')
                gui.run(['xdotool', 'windowsize', wid, '640', '408'])
                time.sleep(.5)
            for _ in range(2):
                gui.move(wid, 320, 200)
                gui.run(['xdotool', 'mousedown', '3'])
                time.sleep(.12)
                gui.run(['xdotool', 'mouseup', '3'])
                time.sleep(.4)
            gui.shot(wid, tag + '-after-person-close')
            key(wid, '9', '2', '1', 'Return')
            time.sleep(.6)
            gui.shot(wid, tag + '-saved')
            path = out / ('saves-' + edition) / 'SV1/REMAKE.JSON'
            gui.check(tag + '-saved-from-normal-ui', path.is_file())
            saved = json.loads(path.read_text())
            snap = saved.get('snapshot', {})
            gui.check(tag + '-format2-full-roster', saved.get('version') == 2 and len(snap.get('generals', [])) == count)
            faction = next(f for f in snap['factions'] if f['ID'] == 16)
            lord = snap['generals'][faction['Lord']]
            gui.check(tag + '-correct-new-lord', lord['Name'] == name and lord['Location'] == city)
            players = list(base64.b64decode(saved.get('players', '')))
            gui.check(tag + '-new-faction-is-human', saved.get('player') == 16 and players == [16])
            if scenario == 2:
                gui.check(tag + '-wang-original-identity', lord['Index'] == 103 and 'source' not in lord)
            gui.receipt['samples'].append(dict(edition=edition, scenario=scenario, name=name, city=city,
                                               count=count, save_sha256=hashlib.sha256(path.read_bytes()).hexdigest()))
            gui.stop(proc)
            folder = '三國演義' if edition == 'base' else '三國演義1加強版'
            loaded = gui.start([str(out / 'san1-window-check'), '-root', '/orig/' + folder,
                                '-edition', edition, '-ai', edition, '-load', '1', '-scale', '1',
                                '-music=false', '-sound=false', '-saves', str(out / ('saves-' + edition)),
                                '-hd-assets', str(root / 'hd-assets')], tag + '-load')
            loaded_wid = gui.wait(['xdotool', 'search', '--onlyvisible', '--pid', str(loaded.pid),
                                   '--name', '三國演義 remake']).splitlines()[0]
            time.sleep(1)
            gui.move(loaded_wid, 638, 400)
            gui.shot(loaded_wid, tag + '-loaded')
            gui.check(tag + '-normal-load-alive', loaded.poll() is None)
            gui.stop(loaded)
            archived = out / ('saves-' + tag)
            shutil.move(saves, archived)
            gui.receipt['samples'][-1]['save_path'] = str((archived / 'SV1/REMAKE.JSON').relative_to(out))
    captures = {c['file']: c for c in gui.receipt['captures']}
    for capture in captures.values():
        path = out / capture['file']
        with Image.open(path) as im:
            im.verify()
        with Image.open(path) as im:
            im.load()
            gui.check(capture['file'] + '-independent-png', im.size == (capture['width'], capture['height'])
                      and hashlib.sha256(path.read_bytes()).hexdigest() == capture['sha256'])
    gui.receipt['passed'] = True
finally:
    for proc in reversed(gui.processes):
        gui.stop(proc)
    for log in gui.logs:
        log.close()
    (out / 'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2) + '\n')
