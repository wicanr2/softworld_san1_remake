#!/usr/bin/env python3
"""Docker/Xvfb：從本機正式完整包正常操作，擷取 README 畫面與來源收據。"""
import argparse
import ast
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import tarfile
import tempfile
import time

from PIL import Image

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--version', required=True)
parser.add_argument('--out', required=True, type=Path)
args = parser.parse_args()
ROOT, OUT = Path('/src'), args.out
assert OUT.is_relative_to(ROOT / 'workplace') and not OUT.exists()
assert OUT.parent.stat().st_uid == os.getuid()
release = ROOT / 'dist-all' / args.version
manifest = json.loads((release / 'SHA256SUMS.json').read_text())
assert manifest['version'] == args.version
archive = release / f'full-local/san1-{args.version}-linux-amd64.tar.gz'

def sha(path):
    digest = hashlib.sha256()
    with path.open('rb') as source:
        for block in iter(lambda: source.read(1048576), b''):
            digest.update(block)
    return digest.hexdigest()

assert sha(archive) == next(p['sha256'] for p in manifest['packages'] if p['file'] == str(archive.relative_to(release)))
OUT.mkdir()
spec = importlib.util.spec_from_file_location('gui', ROOT / 'tools/verify-window-inner.py')
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = OUT
package_stage = tempfile.TemporaryDirectory(prefix='san1-readme-package-')
with tarfile.open(archive, 'r:gz') as tar:
    for member in tar.getmembers():
        assert Path(member.name).parts[0] == f'san1-{args.version}-linux-amd64'
        assert '..' not in Path(member.name).parts and (member.isfile() or member.isdir())
    tar.extractall(package_stage.name)
PACKAGE = Path(package_stage.name) / f'san1-{args.version}-linux-amd64'
shutil.copy2(PACKAGE / 'san1', OUT / 'san1-window-check')
shutil.copy2(ROOT / 'workplace/voice-check-build/title-reference.png', OUT / 'title-reference.png')
refs = ROOT / 'workplace/hd-window/player/panels-v38-wide-r3'
for name in ['camp', 'command', 'continue']:
    shutil.copy2(refs / (name + '-reference.png'), OUT / (name + '-reference.png'))
original_start = gui.start

def package_start(command, name):
    if command[0] == str(OUT / 'san1-window-check'):
        command = list(command)
        command[command.index('-root') + 1] = str(PACKAGE / 'game/base')
        command[command.index('-hd-assets') + 1] = str(PACKAGE / 'hd-assets')
        command += ['-font', str(PACKAGE / 'fonts/unifont.hex.gz')]
    return original_start(command, name)

gui.start = package_start
route = ROOT / 'tools/verify-hd-battle-branches-inner.py'
nodes = [n for n in ast.parse(route.read_text()).body if isinstance(n, ast.FunctionDef) and n.name == 'await_prompt']
exec(compile(ast.Module(body=nodes, type_ignores=[]), str(route), 'exec'), globals())
receipt = gui.receipt
receipt.update(version=args.version, source_commit=manifest['source_commit'], passed=False,
    method='正式 Linux 完整包：正常片頭、曹操新局、人物卡、出兵紮寨、選項列與字體切換',
    state_injection=False, seed_injection=False, clock_injection=False,
    binary_sha256=sha(PACKAGE / 'san1'), source_archive_sha256=sha(archive),
    hd_manifest_sha256=sha(PACKAGE / 'hd-assets/manifest.json'),
    script_sha256=sha(Path(__file__)), outputs=[])

def resize(wid, width, height):
    gui.run(['xdotool', 'windowsize', wid, str(width), str(height)])
    gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    gui.run(['xdotool', 'mousemove', '2790', '1890'])
    time.sleep(.5)

def publish_capture(wid, name, theme, scene):
    path = gui.shot(wid, name)
    with Image.open(path) as im:
        im.load()
        width, height = im.size
    expected = (2560, 1760 if scene.startswith('toolbar') else 1632) if theme == 'hd' else (640, 408)
    gui.check(name + '-native-size', (width, height) == expected)
    receipt['outputs'].append(dict(source=path.name, target='docs/images/' + path.name,
        sha256=sha(path), width=width, height=height, theme=theme, scene=scene, edition='base'))
    return path

def theme(wid, high):
    gui.key(wid, 'Escape')
    gui.choose_ready(wid, 1, int(high))
    gui.key(wid, 'Escape')
    resize(wid, 2560 if high else 640, 1632 if high else 408)

def pair(wid, name, scene):
    original = publish_capture(wid, name, 'original', scene)
    theme(wid, True)
    high = publish_capture(wid, name + '-hd', 'hd', scene)
    x, y = (72, 276) if scene == 'battle' else (536, 68)
    with Image.open(high) as im, Image.open(PACKAGE / 'hd-assets/F000.png') as face:
        expected = face.convert('RGB')
        if scene == 'battle':
            expected = expected.transpose(Image.Transpose.FLIP_LEFT_RIGHT)
        gui.check(name + '-native-Cao-Cao-portrait', im.convert('RGB').crop((x*4, y*4, (x+64)*4, (y+80)*4)).tobytes() == expected.tobytes())
    theme(wid, False)
    receipt.setdefault('pairs', []).append(dict(original=original.name, hd=high.name,
        same_player_pause=True, scenario=1, player='曹操', difficulty=5,
        scope='remake 原貌／HD 的顯示比較，不是原版 oracle 對拍'))

def open_field(wid, field):
    x, probe = {2: (348, 282), 3: (508, 426)}[field]
    gui.move(wid, x, 16)
    gui.run(['xdotool', 'mousedown', '1'])
    try:
        deadline = time.monotonic() + 8
        while time.monotonic() < deadline:
            path = gui.shot(wid, 'toolbar-open-' + str(field))
            with Image.open(path) as im:
                if im.convert('RGB').getpixel((probe*4, 34*4)) in [(44, 52, 65), (44, 92, 112)]:
                    return
        raise RuntimeError('選項列未展開')
    finally:
        gui.run(['xdotool', 'mouseup', '1'])

try:
    os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp', LP_NUM_THREADS='2')
    gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp', '-noreset', '-ac'], 'xvfb')
    gui.wait(['xdotool', 'getdisplaygeometry'])
    gui.check('formal-version', gui.run([str(PACKAGE / 'san1'), '-version']) == args.version)
    proc, wid = gui.launch('base', startup_seconds=90, tag='main')
    gui.key(wid, '1', '1', '1', '2', '5', '1', 'Return', '3', '1', 'Return')
    pair(wid, 'remake-main', 'main-card')
    theme(wid, True)
    gui.key(wid, 'Escape')
    resize(wid, 2560, 1760)
    open_field(wid, 2)
    gui.move(wid, 348, 164)
    publish_capture(wid, 'remake-ai-options', 'hd', 'toolbar-ai')
    open_field(wid, 3)
    gui.move(wid, 508, 76)
    publish_capture(wid, 'remake-input-options', 'hd', 'toolbar-input')
    gui.stop(proc)
    proc, wid = gui.launch('base', startup_seconds=90, tag='battle')
    gui.key(wid, '1', '1', '1', '2', '5')
    gui.key(wid, '2', 'Return', '2', '1', '1', 'Return', '1', '3', 'Return', '1', 'Return', '1', 'Return', 'Return', 'y', '0', 'Return', '1', '0', '0', '0', 'Return')
    await_prompt(wid, 'battle-camp', 'camp', 2)
    gui.key(wid, '3', '3', '2', '2', '0')
    await_prompt(wid, 'battle-command', 'command', 3)
    pair(wid, 'remake-battle', 'battle')
    gui.stop(proc)
    proc, wid = gui.launch('base', startup_seconds=90, tag='fonts')
    for key, face in [('3', 'kai'), ('4', 'li')]:
        gui.key(wid, key)
        resize(wid, 640, 408)
        publish_capture(wid, 'remake-font-' + face, 'original', 'font-' + face)
        theme(wid, True)
        publish_capture(wid, 'remake-font-' + face + '-hd', 'hd', 'font-' + face)
        theme(wid, False)
    gui.stop(proc)
    assert len(receipt['outputs']) == 10
    receipt['passed'] = True
finally:
    for proc in reversed(gui.processes):
        gui.stop(proc)
    for log in gui.logs:
        log.close()
    package_stage.cleanup()
    (OUT / 'receipt.json').write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n')
