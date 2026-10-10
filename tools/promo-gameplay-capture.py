#!/usr/bin/env python3
"""Record real opening, new-game input, battle setup and HD switches from a verified private package."""
import argparse
import ast
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import tarfile
import tempfile
import time
from PIL import Image, ImageChops

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--version', required=True)
parser.add_argument('--out', type=Path, required=True)
parser.add_argument('--story', action='store_true', help='Record player-facing story, voiced advice and actual combat footage')
args = parser.parse_args()
ROOT, OUT = Path('/src'), args.out
assert not OUT.exists() and OUT.parent.stat().st_uid == os.getuid()
OUT.mkdir()
sha = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
spec = importlib.util.spec_from_file_location('gui', ROOT / 'tools/verify-window-inner.py')
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = OUT
reference_dir = ROOT / 'workplace/promo-source' / args.version
for path in reference_dir.glob('*-reference.png'):
    shutil.copyfile(path, OUT / path.name)
assert (OUT / 'title-reference.png').is_file()
polls = tempfile.TemporaryDirectory(prefix='san1-capture-polls-')
stage = tempfile.TemporaryDirectory(prefix='san1-capture-package-')
archive = ROOT / 'dist-all' / args.version / f'full-local/san1-{args.version}-linux-amd64.tar.gz'
with tarfile.open(archive, 'r:gz') as t:
    for item in t.getmembers():
        assert '..' not in Path(item.name).parts and not item.name.startswith('/')
        assert item.isfile() or item.isdir()
    t.extractall(stage.name)
PACKAGE = Path(stage.name) / f'san1-{args.version}-linux-amd64'
receipt = gui.receipt
receipt.update(version=args.version, passed=False,
    method='正式完整版正常GUI：片頭、開新局、人物檢視、出兵及紮寨；實際Esc選項列切換HD',
    no_state_seed_clock_injection=True, binary_sha256=sha(PACKAGE / 'san1'),
    hd_manifest_sha256=sha(PACKAGE / 'hd-assets/manifest.json'),
    source_archive_sha256=sha(archive), clips=[], events=[], switches=[])
active_recording = None


def shot(wid, tag):
    path = Path(polls.name) / (tag + '.png')
    g = gui.geo(wid)
    gui.run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y', '-f', 'x11grab',
             '-video_size', f"{g['WIDTH']}x{g['HEIGHT']}", '-i', f":99+{g['X']},{g['Y']}",
             '-frames:v', '1', '-threads', '1', str(path)])
    return path


def rgb(path, crop=None):
    with Image.open(path) as source:
        image = source.convert('RGB')
        if crop:
            w, h, x, y = map(int, crop.split(':'))
            image = image.crop((x, y, x + w, y + h))
        return image.tobytes()


def logical_rgb(path, crop=None):
    with Image.open(path) as source:
        image = source.convert('RGB')
        if image.width == 1280 and image.height >= 816:
            image = image.resize((640, image.height // 2), Image.Resampling.NEAREST)
        if crop:
            w, h, x, y = map(int, crop.split(':'))
            image = image.crop((x, y, x + w, y + h))
        return image.tobytes()


gui.shot, gui.rgb = shot, rgb
route = ROOT / 'tools/verify-hd-battle-branches-inner.py'
nodes = [n for n in ast.parse(route.read_text()).body
         if isinstance(n, ast.FunctionDef) and n.name in {'flip', 'source_face', 'await_prompt', 'reach_attack_turn'}]


class LogicalRoute(ast.NodeTransformer):
    def visit_Call(self, node):
        self.generic_visit(node)
        if (isinstance(node.func, ast.Attribute) and node.func.attr == 'rgb' and
                isinstance(node.func.value, ast.Name) and node.func.value.id == 'gui'):
            node.func = ast.Name(id='logical_rgb', ctx=ast.Load())
        return node


exec(compile(ast.fix_missing_locations(LogicalRoute().visit(ast.Module(body=nodes, type_ignores=[]))),
             str(route), 'exec'), globals())


def keep(wid, tag):
    path = OUT / (tag + '.png')
    shutil.copyfile(shot(wid, tag), path)
    receipt['captures'].append({'file': path.name, 'sha256': sha(path), 'geometry': gui.geo(wid)})
    return path


def start_recording(name):
    global active_recording
    path = OUT / (name + '.mp4')
    command = ['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y',
        '-f', 'x11grab', '-framerate', '20', '-video_size', '1280x880', '-draw_mouse', '1',
        '-i', ':99']
    if args.story:
        command += ['-f', 'pulse', '-sample_rate', '48000', '-channels', '2',
                    '-fragment_size', '4096', '-i', 'san1.monitor', '-c:a', 'aac', '-b:a', '256k']
    else:
        command += ['-an']
    command += ['-c:v', 'libx264', '-threads', '2', '-preset', 'veryfast',
                '-crf', '18', '-pix_fmt', 'yuv420p', str(path)]
    proc = gui.start(command, name + '-record')
    active_recording = (proc, path, time.monotonic())
    return active_recording[2]


def event(action):
    receipt['events'].append({'clip': active_recording[1].name,
        'at': round(time.monotonic() - active_recording[2], 3), 'action': action})


def stop_recording():
    global active_recording
    proc, path, begin = active_recording
    assert proc.poll() is None
    proc.send_signal(signal.SIGINT)
    proc.wait(timeout=15)
    assert proc.returncode in (0, 255)
    probe = json.loads(gui.run(['ffprobe', '-v', 'error', '-show_format', '-show_streams', '-of', 'json', str(path)]))
    video = next(s for s in probe['streams'] if s['codec_type'] == 'video')
    assert (video['width'], video['height'], video['avg_frame_rate']) == (1280, 880, '20/1')
    gui.run(['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-threads', '2', '-i', str(path), '-an', '-f', 'null', '-'], timeout=90)
    receipt['clips'].append({'file': path.name, 'sha256': sha(path),
        'duration': float(probe['format']['duration']), 'normal_input': True})
    active_recording = None


def launch(tag):
    command = [str(PACKAGE / 'san1'), '-root', str(PACKAGE / 'game/base'), '-edition', 'base',
        '-ai', 'base', '-scale', '2', '-music=false', '-sound=' + str(args.story).lower(),
        '-saves', '/tmp/san1-promo-saves-' + tag, '-font', str(PACKAGE / 'fonts/unifont.hex.gz'),
        '-hd-assets', str(PACKAGE / 'hd-assets')]
    receipt.setdefault('launch_commands', []).append(command)
    proc = gui.start(command, tag)
    wid = gui.wait(['xdotool', 'search', '--onlyvisible', '--pid', str(proc.pid), '--name', '三國演義 remake']).splitlines()[0]
    gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    gui.move(wid, 320, 200)
    return proc, wid


def title(wid, tag):
    expected = logical_rgb(OUT / 'title-reference.png', '568:160:40:27')
    deadline = time.monotonic() + 70
    while time.monotonic() < deadline:
        actual = logical_rgb(shot(wid, tag + '-boot'), '568:160:40:27')
        if len(actual) == len(expected) and sum(a != b for a, b in zip(actual, expected)) < len(expected) // 100:
            gui.check(tag + '-normal-title', True)
            return
        gui.key(wid, 'space')
    raise RuntimeError('normal title not reached')


def switch(wid, tag):
    event(tag + ': Esc開啟選項列')
    gui.key(wid, 'Escape')
    original = keep(wid, tag + '-original')
    event(tag + ': 原貌切換B高清')
    gui.choose_ready(wid, 1, 1)
    time.sleep(1.5)
    hd = keep(wid, tag + '-hd')
    with Image.open(original) as a, Image.open(hd) as b:
        a, b = a.convert('RGB').crop((0, 64, 1280, 880)), b.convert('RGB').crop((0, 64, 1280, 880))
        different = sum(x != y for x, y in zip(a.tobytes(), b.tobytes()))
    gui.check(tag + '-actual-hd-content-changed', different > 50000)
    event(tag + ': B高清回原貌')
    gui.choose_ready(wid, 1, 0)
    time.sleep(.5)
    restored = keep(wid, tag + '-restored')
    receipt['switches'].append({'name': tag, 'original': original.name, 'hd': hd.name,
        'restored': restored.name, 'changed_rgb_bytes': different, 'real_toolbar_input': True})
    gui.key(wid, 'Escape')
    gui.move(wid, 320, 200)


def story_capture():
    """Normal inputs only. Retain voice timing and the actual toolbar switch."""
    proc, wid = launch('story-opening')
    start_recording('opening')
    event('遊戲片頭自然播放')
    time.sleep(6)
    stop_recording()
    title(wid, 'story-main')
    gui.key(wid, '1', '1', '1', '2', '5', '0', 'Return')
    gui.key(wid, '1', '1', '1', '1', 'Return', '1', '3', '1', 'Return')
    # Film the actual character inspection after setup; initial diagnostic logs
    # are outside the recorded shot, rather than hidden by an editorial overlay.
    start_recording('realm-card')
    event('曹操正常新局；人物資料檢視')
    time.sleep(3)
    switch(wid, 'card-switch')
    time.sleep(2)
    stop_recording()
    gui.stop(proc)

    proc, wid = launch('story-battle')
    title(wid, 'story-battle')
    gui.key(wid, '1', '1', '1', '6', '5')
    waiting, rested = reach_attack_turn(wid, 'story-battle', 'base')
    start_recording('war-advice')
    event('軍事出兵，正常觸發人物建議與原版語音')
    gui.key(wid, '2', 'Return')
    keep(wid, 'war-advice-shown')
    time.sleep(4)
    gui.key(wid, '2')
    if waiting == 14:
        gui.key(wid, '1', '5', 'Return')
    gui.key(wid, '1', '1', 'Return', '2', 'Return', '1', 'Return', 'Return', 'y',
            '0', 'Return', '1', '0', '0', '0', 'Return')
    event('軍師完成出兵前建議：兵者貴神速')
    time.sleep(1)
    keep(wid, 'attack-adviser-voice')
    time.sleep(3)
    await_prompt(wid, 'story-camp', 'camp', 2)
    stop_recording()
    gui.key(wid, '3', '3', '6', '3', '0')
    await_prompt(wid, 'story-command', 'command', 3)
    start_recording('battle-combat')
    event('主戰場實際選單切換高清')
    switch(wid, 'battle-switch')
    gui.key(wid, '2')
    await_prompt(wid, 'story-engage-direction', 'engage-direction', 1, advance=False)
    gui.key(wid, '2')
    await_prompt(wid, 'story-skirmish', 'skirmish', 2)
    event('對戰子畫面切換高清並續玩')
    gui.key(wid, 'Escape')
    gui.choose_ready(wid, 1, 1)
    gui.key(wid, 'Escape')
    time.sleep(1)
    event('對戰子畫面：正常行軍與休息')
    for keys in [('1', '5'), ('Return',), ('1', '5'), ('0', 'y'), ('1', '5')]:
        gui.key(wid, *keys)
        time.sleep(.5)
    event('呂布向陳宮叫陣，正常單挑與人物語音')
    gui.key(wid, '2', '5')
    time.sleep(3)
    keep(wid, 'duel-scene')
    for n in range(10):
        gui.key(wid, 'space')
        time.sleep(4)
        keep(wid, f'duel-stage-{n}')
    stop_recording()
    receipt['battle_route'] = {'waiting': waiting, 'rested': rested,
        'normal_attack': True, 'skirmish': True, 'duel': True}
    receipt['audio'] = 'PulseAudio san1.monitor; original in-game voices and effects; background music disabled'
    gui.stop(proc)


try:
    os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp', LP_NUM_THREADS='2')
    gui.start(['Xvfb', ':99', '-screen', '0', '1600x1000x24', '-nolisten', 'tcp', '-noreset', '-ac'], 'xvfb')
    gui.wait(['xdotool', 'getdisplaygeometry'])
    if args.story:
        runtime = Path('/tmp/san1-story-audio')
        runtime.mkdir(mode=0o700)
        os.environ.update(XDG_RUNTIME_DIR=str(runtime), PULSE_SERVER=f'unix:{runtime}/native')
        gui.start(['pulseaudio', '-n', '--daemonize=no', '--exit-idle-time=-1',
            '--load=module-null-sink sink_name=san1 rate=48000 channels=2',
            f'--load=module-native-protocol-unix socket={runtime}/native auth-anonymous=1'], 'pulse')
        gui.wait(['pactl', 'info'])
        gui.run(['pactl', 'set-default-sink', 'san1'])
    assert gui.run([str(PACKAGE / 'san1'), '-version']) == args.version
    if args.story:
        story_capture()
        receipt['passed'] = True
        raise SystemExit(0)
    proc, wid = launch('opening-game')
    start_recording('opening')
    event('正式遊戲片頭自然播放')
    time.sleep(4)
    switch(wid, 'opening-switch')
    gui.key(wid, 'space')
    time.sleep(4)
    keep(wid, 'opening-live')
    stop_recording()
    title(wid, 'main')
    start_recording('new-game-card')
    event('正常主選單開新局')
    gui.key(wid, '1', '1', '1', '2', '5', '0', 'Return')
    event('選擇君主並進入人物檢視')
    gui.key(wid, '1', '1', '1', '1', 'Return', '1', '3', '1', 'Return')
    time.sleep(1)
    switch(wid, 'card-switch')
    time.sleep(1)
    stop_recording()
    gui.stop(proc)
    proc, wid = launch('battle-game')
    title(wid, 'battle')
    start_recording('battle-play')
    event('正常新局與董卓出兵操作')
    gui.key(wid, '1', '1', '1', '6', '5')
    waiting, rested = reach_attack_turn(wid, 'promo-battle', 'base')
    receipt['battle_route'] = {'waiting': waiting, 'rested': rested}
    gui.key(wid, '2', 'Return', '2')
    if waiting == 14:
        gui.key(wid, '1', '5', 'Return')
    gui.key(wid, '1', '1', 'Return', '2', 'Return', '1', 'Return', 'Return', 'y',
            '0', 'Return', '1', '0', '0', '0', 'Return')
    await_prompt(wid, 'promo-camp', 'camp', 2)
    event('戰場紮寨與位置選擇')
    gui.key(wid, '3', '3', '6', '3', '0')
    await_prompt(wid, 'promo-command', 'command', 3)
    switch(wid, 'battle-switch')
    keep(wid, 'battle-live')
    time.sleep(2)
    stop_recording()
    receipt['passed'] = True
finally:
    if active_recording:
        try:
            stop_recording()
        except Exception as error:
            receipt['recording_cleanup_error'] = str(error)
    for proc in reversed(gui.processes):
        gui.stop(proc)
    for log in gui.logs:
        log.close()
    polls.cleanup()
    stage.cleanup()
    (OUT / 'receipt.json').write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n')
