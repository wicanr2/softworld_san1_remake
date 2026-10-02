#!/usr/bin/env python3
"""容器內的正常視窗配樂錄音；素材及錄音只留在 workplace/audio。"""
import array
import hashlib
import json
import math
import os
from pathlib import Path
import subprocess
import time
import wave

ROOT = Path('/src')
OUT = ROOT / 'workplace/audio'
PREFIX = 'music-check'
processes = []
logs = []
receipt = {'schema': 'san1-music-playback/1', 'method': '正式 GUI → Ebiten/oto → PulseAudio monitor',
           'audio_image': os.environ.get('SAN1_AUDIO_IMAGE_ID'), 'checks': [],
           'human_listening': False, 'platform': 'Linux amd64'}


def run(args, **kwargs):
    return subprocess.run(args, check=True, timeout=kwargs.pop('timeout', 15),
                          text=True, capture_output=True, **kwargs).stdout.strip()


def start(args, name):
    log = open(OUT / f'{PREFIX}-{name}.log', 'w')
    logs.append(log)
    proc = subprocess.Popen(args, stdout=log, stderr=subprocess.STDOUT)
    processes.append(proc)
    return proc


def stop(proc):
    if proc.poll() is None:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)


def wait_for(args, seconds=15):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        try:
            value = run(args, timeout=2)
            if value:
                return value
        except (subprocess.SubprocessError, OSError):
            pass
        time.sleep(.2)
    raise RuntimeError(f'等待工具就緒逾時：{args}')


def keys(wid, *names):
    run(['xdotool', 'windowfocus', '--sync', wid])
    for name in names:
        run(['xdotool', 'keydown', '--clearmodifiers', name])
        time.sleep(.08)
        run(['xdotool', 'keyup', name])
        receipt.setdefault('keys', []).append(name)
        time.sleep(.4)


def screenshot(wid, name):
    geometry = dict(line.split('=', 1) for line in
                    run(['xdotool', 'getwindowgeometry', '--shell', wid]).splitlines())
    run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y',
         '-f', 'x11grab', '-video_size', f"{geometry['WIDTH']}x{geometry['HEIGHT']}",
         '-i', f":99+{geometry['X']},{geometry['Y']}", '-frames:v', '1',
         str(OUT / f'{PREFIX}-{name}.png')])


def wait_menu(wid):
    # 用正式渲染產生的主選單固定上圖確認狀態；排除下方動畫與選取反白。
    # Layout 是 640×400，640×408 視窗的上下各有 4 像素留白。
    base = ['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error']
    tail = ['-frames:v', '1', '-pix_fmt', 'rgb24',
            '-f', 'rawvideo', 'pipe:1']
    reference = subprocess.run(base + ['-i', str(OUT / f'{PREFIX}-title-reference.png'),
                               '-vf', 'crop=568:160:40:27'] + tail,
                               check=True, capture_output=True, timeout=15).stdout
    deadline = time.monotonic() + 45
    while time.monotonic() < deadline:
        geo = dict(line.split('=', 1) for line in
                   run(['xdotool', 'getwindowgeometry', '--shell', wid]).splitlines())
        assert (int(geo['WIDTH']), int(geo['HEIGHT'])) == (640, 408), geo
        shot = subprocess.run(base + ['-f', 'x11grab', '-video_size',
                              f"{geo['WIDTH']}x{geo['HEIGHT']}",
                              '-i', f":99+{geo['X']},{geo['Y']}",
                              '-vf', 'crop=568:160:40:31'] + tail,
                              check=True, capture_output=True, timeout=15).stdout
        if len(shot) == len(reference) and sum(a != b for a, b in zip(shot, reference)) < len(reference) // 100:
            return
        keys(wid, 'space')
    screenshot(wid, 'menu-timeout')
    raise RuntimeError('片頭續行後仍未看到正式主選單')


def capture(name, audible=True, seconds=6):
    path = OUT / f'{PREFIX}-{name}.wav'
    run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y',
         '-f', 'pulse', '-sample_rate', '48000', '-channels', '2',
         '-i', 'san1.monitor', '-t', str(seconds), '-c:a', 'pcm_s16le', str(path)],
        timeout=seconds + 10)
    with wave.open(str(path)) as wav:
        assert (wav.getnchannels(), wav.getframerate(), wav.getsampwidth()) == (2, 48000, 2)
        samples = array.array('h', wav.readframes(wav.getnframes()))
    peak = max(abs(v) for v in samples)
    rms = math.sqrt(sum(v * v for v in samples) / len(samples))
    db = lambda value: 20 * math.log10(value / 32768) if value else -999
    result = {'name': name, 'seconds': len(samples) / 96000, 'expected_audible': audible,
              'rms_dbfs': round(db(rms), 2), 'peak_dbfs': round(db(peak), 2),
              'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}
    sound_ok = db(rms) > -60 and db(peak) > -45 if audible else peak <= 2
    result['passed'] = sound_ok and seconds - 1 <= result['seconds'] <= seconds + 1
    receipt['checks'].append(result)
    print(json.dumps(result, ensure_ascii=False), flush=True)
    if not result['passed']:
        raise RuntimeError(f'音訊輸出不符：{name}')


def launch(edition, dirname):
    root = Path('/orig') / dirname
    receipt.setdefault('inputs', {})[edition] = {
        'DATA1.GRP': hashlib.sha256((root / 'DATA1.GRP').read_bytes()).hexdigest()}
    proc = start([str(OUT / 'san1-music-check'), '-root', str(root), '-edition', edition,
                  '-ai', edition, '-scale', '1', '-sound=false',
                  '-saves', '/tmp/san1-music-saves'], edition)
    wid = wait_for(['xdotool', 'search', '--onlyvisible', '--pid', str(proc.pid),
                    '--name', '三國演義 remake']).splitlines()[0]
    # Space 只續行／略過片頭；主選單不以 Space 選擇任何項目。
    wait_menu(wid)
    screenshot(wid, f'{edition}-menu')
    if proc.poll() is not None:
        raise RuntimeError(f'{edition} 遊戲提早結束')
    return proc, wid


def new_game(wid):
    # 主選單 → 劇本一 → 單人 → 君主第二格 → 難度五。
    keys(wid, '1', '1', '1', '2', '5')
    time.sleep(1)


try:
    assert OUT.stat().st_uid == os.getuid() and OUT.stat().st_gid == os.getgid()
    runtime = Path('/tmp/san1-pulse')
    runtime.mkdir(mode=0o700)
    os.environ.update({'DISPLAY': ':99', 'LIBGL_ALWAYS_SOFTWARE': '1',
                       'XDG_RUNTIME_DIR': str(runtime), 'PULSE_SERVER': f'unix:{runtime}/native'})
    start(['Xvfb', ':99', '-screen', '0', '1280x900x24', '-nolisten', 'tcp'], 'xvfb')
    start(['pulseaudio', '-n', '--daemonize=no', '--exit-idle-time=-1',
           '--load=module-null-sink sink_name=san1 rate=48000 channels=2',
           f'--load=module-native-protocol-unix socket={runtime}/native auth-anonymous=1'], 'pulse')
    wait_for(['pactl', 'info'])
    run(['pactl', 'set-default-sink', 'san1'])
    receipt['binary_sha256'] = hashlib.sha256((OUT / 'san1-music-check').read_bytes()).hexdigest()
    receipt['revision'] = run(['git', 'rev-parse', 'HEAD'], cwd=ROOT)
    receipt['modified_sources'] = {
        name: hashlib.sha256((ROOT / name).read_bytes()).hexdigest()
        for name in ['internal/music/stream.go', 'tools/verify-music-inner.py', 'tools/verify-music.sh']}
    receipt['tools'] = {'pulseaudio': run(['pulseaudio', '--version']),
                        'ffmpeg': run(['ffmpeg', '-version']).splitlines()[0]}
    proc, wid = launch('base', '三國演義')
    capture('base-default')
    for i, name in enumerate(['思古', '小徑', '風雲', '戰鼓', '末路'], 1):
        # 正式選曲後會回主選單，每首都重新進「音樂欣賞」。
        keys(wid, '5')
        screenshot(wid, f'base-track-{i}-pick')
        keys(wid, str(i))
        time.sleep(1)
        screenshot(wid, f'base-track-{i}')
        capture(f'base-track-{i}-{name}')
    keys(wid, 'Escape')
    new_game(wid)
    screenshot(wid, 'base-main')
    capture('base-main')
    # 新局主提示以數字輸入收命令，Enter 確定後才進其他選單。
    keys(wid, '9', 'Return')
    screenshot(wid, 'base-other')
    keys(wid, '3')
    time.sleep(2)
    screenshot(wid, 'base-muted')
    capture('base-muted', audible=False, seconds=4)
    # 靜音提示後已退出數字輸入，數字鍵直接開其他選單。
    keys(wid, '9')
    screenshot(wid, 'base-other-resume')
    keys(wid, '3')
    time.sleep(1)
    screenshot(wid, 'base-resumed')
    capture('base-resumed')
    stop(proc)
    time.sleep(1)
    proc, wid = launch('plus', '三國演義1加強版')
    new_game(wid)
    screenshot(wid, 'plus-main')
    capture('plus-main')
    receipt['passed'] = True
finally:
    receipt.setdefault('passed', False)
    for proc in reversed(processes):
        stop(proc)
    for log in logs:
        log.close()
    (OUT / f'{PREFIX}-receipt.json').write_text(
        json.dumps(receipt, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
