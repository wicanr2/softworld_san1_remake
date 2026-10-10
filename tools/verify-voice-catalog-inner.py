#!/usr/bin/env python3
"""Normal chief appointment, atlas, battle rest and duel voice/AI checks in Docker."""
import argparse
import ast
import functools
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import time

parser = argparse.ArgumentParser(description=__doc__)
for name in ('out', 'reference', 'binary', 'title-reference'):
    parser.add_argument('--' + name, required=True)
parser.add_argument('--editions', nargs='+', choices=['base', 'plus'], default=['base', 'plus'])
parser.add_argument('--samples', nargs='+', choices=['main', 'atlas', 'battle', 'duel'],
                    default=['main', 'atlas', 'battle', 'duel'])
args = parser.parse_args()
ROOT = Path('/src')
OUT, REFERENCE = ROOT / args.out, ROOT / args.reference
PACK = ROOT / 'workplace/hd-assets-mapcursor-v50-r1'
assert OUT.is_relative_to(ROOT / 'workplace/audio')
spec = importlib.util.spec_from_file_location('gui', ROOT / 'tools/verify-window-inner.py')
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = OUT
original_start, original_key = gui.start, gui.key

# Reuse the existing complete-PCM recorder and normal visual/input helpers.
def load_functions(path, names):
    nodes = [n for n in ast.parse(path.read_text()).body
             if isinstance(n, (ast.Import, ast.ImportFrom)) or isinstance(n, ast.FunctionDef) and n.name in names]
    assert {n.name for n in nodes if isinstance(n, ast.FunctionDef)} == names
    exec(compile(ast.Module(body=nodes, type_ignores=[]), str(path), 'exec'), globals())


load_functions(ROOT / 'tools/verify-voice-inner.py',
               {'sha', 'start', 'shot', 'hidden', 'theme', 'record', 'finish', 'await_picture'})
gui.start, gui.shot = start, shot
load_functions(ROOT / 'tools/verify-hd-battle-branches-inner.py',
               {'flip', 'source_face', 'await_prompt', 'reach_attack_turn'})


def check_cue(wid, tag, ref, trigger, crop, mirror=False):
    assert sha(REFERENCE / ref['file']) == ref['sha256']
    expected = (REFERENCE / ref['file']).read_bytes()
    rec = record(tag)
    trigger()
    source = ROOT / 'workplace/hd-inventory' / edition / 'img/DATA3' / ref['portrait']
    target = gui.rgb(source, '64:80:0:0,hflip' if mirror else None)
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline:
        picture = gui.shot(wid, tag + '-shown')
        if gui.rgb(picture, crop) == target:
            break
        time.sleep(.2)
    else:
        raise RuntimeError(tag + ': expected normal dialogue portrait not shown')
    theme(wid, True)
    gui.shot(wid, tag + '-hd')
    theme(wid, False)
    gui.shot(wid, tag + '-restored')
    time.sleep(len(expected) / 192000 + 1)
    finish(rec, expected, tag)
    gui.receipt.setdefault('samples', []).append({'tag': tag, 'key': ref['key'],
        'indices': ref['indices'], 'speaker_index': ref['speaker_index'], 'normal_gui': True})


def main_samples():
    proc, wid = gui.launch(edition, hd_assets=PACK, tag=edition + '-main')
    row = refs[edition]
    gui.key(wid, '1', '1', '1', str(row['lord_menu']), '5')
    time.sleep(1)
    gui.key(wid, 'Escape')
    previous = None
    for level in range(1, 6):
        gui.choose_ready(wid, 2, level)
        gui.move(wid, 320, 200)
        path = gui.shot(wid, edition + f'-ai-{level}')
        current = gui.rgb(path, '640:32:0:0')
        if previous is not None:
            gui.check(edition + f'-ai-{level}-visible-change', current != previous)
        previous = current
    gui.choose_ready(wid, 2, 0)
    hidden(wid)
    gui.key(wid, '7', 'Return', '1', str(row['chief_position']), 'Return')
    scene = ROOT / 'workplace/hd-inventory' / edition / 'img/DATA3' / row['scene']
    await_picture(wid, edition + '-appointment-scene', scene, '176:96:432:80', advance=True)
    time.sleep(.6)
    check_cue(wid, edition + '-chief-order', row['samples']['chief-order'],
              lambda: gui.key(wid, 'space'), '64:80:552:80')
    check_cue(wid, edition + '-chief-reply', row['samples']['chief-reply'],
              lambda: gui.key(wid, 'space'), '64:80:424:180', True)
    gui.stop(proc)


def atlas_samples():
    proc, wid = gui.launch(edition, hd_assets=PACK, tag=edition + '-atlas-new-game')
    row = refs[edition]
    gui.key(wid, '1', '1', '1', str(row['lord_menu']), '5')
    time.sleep(1)
    check_cue(wid, edition + '-atlas', row['samples']['atlas'],
              lambda: gui.key(wid, '1', 'Return', '5'), '64:80:560:268')
    gui.stop(proc)


def enter_battle(tag):
    proc, wid = gui.launch(edition, hd_assets=PACK, tag=tag)
    gui.key(wid, '1', '1', '1', '6', '5')
    waiting, rested = reach_attack_turn(wid, tag, edition)
    gui.key(wid, '2', 'Return', '2')
    if waiting == 14:
        gui.key(wid, '1', '5', 'Return')
    gui.key(wid, '1', '1', 'Return', '2', 'Return', '1', 'Return', 'Return', 'y',
            '0', 'Return', '1', '0', '0', '0', 'Return')
    await_prompt(wid, tag + '-camp', 'camp', 2)
    gui.key(wid, '3', '3', '6', '3', '0')
    await_prompt(wid, tag + '-command', 'command', 3)
    return proc, wid


def battle_samples():
    proc, wid = enter_battle(edition + '-rest')
    time.sleep(5)
    check_cue(wid, edition + '-camp', refs[edition]['samples']['camp'],
              lambda: gui.key(wid, '0', 'y'), '64:80:560:268')
    gui.stop(proc)


def duel_samples():
    global source, args, duel_recording, duel_number
    source = ROOT / 'workplace/hd-inventory' / edition / 'img/DATA3'
    load_functions(ROOT / 'tools/verify-hd-duel-inner.py',
                   {'crop', 'rgb', 'face_name', 'scene_at', 'await_scene', 'await_stage', 'play'})
    # The old route refers to pack and optional text-audit switches.
    globals()['pack'] = PACK
    args.verify_names = args.verify_text = False
    duel_recording, duel_number = None, -1

    def key(wid, *keys):
        global duel_recording
        if keys == ('space',) and duel_number in (0, 1):
            duel_recording = record(edition + '-duel-' + str(duel_number))
        return original_key(wid, *keys)

    def sample(wid, stage, kind, value, text_key=None):
        global duel_recording, duel_number
        gui.shot(wid, edition + '-' + stage)
        if stage == 'challenge-scene':
            time.sleep(.6)
            duel_number = 0
        elif text_key in ('bub.duelChallenge', 'bub.duelAccept'):
            tag = 'duel-challenge' if text_key == 'bub.duelChallenge' else 'duel-accept'
            ref = refs[edition]['samples'][tag]
            assert sha(REFERENCE / ref['file']) == ref['sha256']
            expected = (REFERENCE / ref['file']).read_bytes()
            assert duel_recording is not None
            theme(wid, True)
            gui.shot(wid, edition + '-' + tag + '-hd')
            theme(wid, False)
            time.sleep(len(expected) / 192000 + 1)
            finish(duel_recording, expected, edition + '-' + tag)
            duel_recording = None
            duel_number += 1
            gui.receipt.setdefault('samples', []).append({'tag': edition + '-' + tag,
                'key': ref['key'], 'indices': ref['indices'], 'normal_gui': True})
    globals()['sample'] = sample
    gui.key = key
    try:
        play()
    finally:
        gui.key = original_key


assert not OUT.exists(), 'refuse to overwrite a previous receipt'
try:
    OUT.mkdir()
    assert OUT.stat().st_uid == os.getuid()
    for path, name in [(ROOT / args.binary, 'san1-window-check'),
                       (ROOT / args.title_reference, 'title-reference.png')]:
        shutil.copy2(path, OUT / name)
    for path in REFERENCE.glob('*-reference.png'):
        shutil.copy2(path, OUT / path.name)
    runtime = Path('/tmp/san1-full-voice')
    runtime.mkdir(mode=0o700)
    os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR=str(runtime),
                      PULSE_SERVER=f'unix:{runtime}/native')
    original_start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp', '-ac'], 'xvfb')
    gui.wait(['xdotool', 'getdisplaygeometry'])
    original_start(['pulseaudio', '-n', '--daemonize=no', '--exit-idle-time=-1',
        '--load=module-null-sink sink_name=san1 rate=48000 channels=2',
        f'--load=module-native-protocol-unix socket={runtime}/native auth-anonymous=1'], 'pulse')
    gui.wait(['pactl', 'info'])
    gui.run(['pactl', 'set-default-sink', 'san1'])
    refs = json.loads((REFERENCE / 'full-reference.json').read_text())
    gui.receipt.update(method='正常新局、人事、地理誌、董卓出兵、休息與單挑；完整 PCM 與 HD 切換',
        binary_sha256=sha(OUT / 'san1-window-check'), state_injection=False, seed_injection=False,
        clock_injection=False, human_listening=False, scope='兩版繁中六類新增對白與 AI 1–5；宣戰三語另驗',
        reference_sha256=sha(REFERENCE / 'full-reference.json'), selection_editions=args.editions,
        selection_samples=args.samples,
        sources_sha256={p.name: sha(p) for p in [Path(__file__), ROOT / 'tools/full-voice-reference.go',
            ROOT / 'tools/verify-voice-inner.py', ROOT / 'tools/verify-hd-duel-inner.py',
            ROOT / 'tools/verify-hd-battle-branches-inner.py', ROOT / 'tools/verify-window-inner.py']})
    for edition in args.editions:
        for sample_name in args.samples:
            {'main': main_samples, 'atlas': atlas_samples, 'battle': battle_samples,
             'duel': duel_samples}[sample_name]()
        gui.receipt.setdefault('edition_passed', {})[edition] = True
    gui.receipt['passed'] = True
finally:
    gui.receipt.setdefault('passed', False)
    for proc in reversed(gui.processes): gui.stop(proc)
    for log in gui.logs: log.close()
    if OUT.exists(): (OUT / 'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2) + '\n')
