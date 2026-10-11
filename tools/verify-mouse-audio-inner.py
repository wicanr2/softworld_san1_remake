#!/usr/bin/env python3
"""Docker/Xvfb 正常滑鼠新局與片頭、標題配樂；原版素材與收據只留本機。"""
import argparse
import array
import hashlib
import importlib.util
import json
import math
import os
from pathlib import Path
import time
import wave

from PIL import Image

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out', required=True)
parser.add_argument('--binary', default='workplace/san1-mouse-audio-check')
parser.add_argument('--ui-only', action='store_true')
parser.add_argument('--hd', action='store_true')
parser.add_argument('--editions', nargs='+', choices=['base', 'plus'], default=['base', 'plus'])
parser.add_argument('--battle', action='store_true')
parser.add_argument('--affair', action='store_true')
args = parser.parse_args()
ROOT = Path('/src')
OUT = ROOT / args.out
assert OUT.resolve().is_relative_to((ROOT / 'workplace').resolve())
assert OUT.is_dir() and OUT.stat().st_uid == os.getuid()
assert not (OUT / 'receipt.json').exists(), '拒絕覆寫既有收據'
spec = importlib.util.spec_from_file_location('gui', ROOT / 'tools/verify-window-inner.py')
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = OUT
gui.receipt.update(method='正式片頭 → 滑鼠選單 → 新局 → 儲存，無狀態注入',
                   human_listening=False, audio_image=os.environ.get('SAN1_AUDIO_IMAGE_ID'), hd=args.hd)
sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()


def click(wid, x, y, button=1):
    scale = gui.geo(wid)['WIDTH'] / 640
    gui.run(['xdotool', 'windowfocus', '--sync', wid])
    gui.run(['xdotool', 'mousemove', '--window', wid, str(round(x*scale)), str(round(y*scale))])
    # 與既有 gui.move 相同，先讓設定列的 Reveal 處理移出事件。
    time.sleep(.25)
    gui.run(['xdotool', 'mousedown', str(button)])
    # 沿用既有高清 GUI 驗證的 600 ms 按住時間。
    time.sleep(.6)
    gui.run(['xdotool', 'mouseup', str(button)])
    time.sleep(.4)
    gui.receipt.setdefault('clicks', []).append([x, y, button, scale])


def shot(wid, tag):
    path = OUT / (tag + '-' + str(len(gui.receipt['captures'])) + '.png')
    g = gui.geo(wid)
    gui.run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y',
             '-f', 'x11grab', '-draw_mouse', '0', '-video_size', f"{g['WIDTH']}x{g['HEIGHT']}",
             '-i', f":99+{g['X']},{g['Y']}", '-frames:v', '1', '-threads', '1', str(path)])
    gui.receipt['captures'].append(dict(file=path.name, width=g['WIDTH'], height=g['HEIGHT'], sha256=sha(path)))
    return path


def menu_visible(path):
    with Image.open(path) as im, Image.open(ROOT / 'workplace/mouse-audio-title-reference.png') as ref:
        a, b = im.crop((40, 27, 608, 187)).convert('RGB').tobytes(), ref.crop((40, 27, 608, 187)).convert('RGB').tobytes()
    return len(a) == len(b) and sum(x != y for x, y in zip(a, b)) < len(b)//100


def region(path, box):
    with Image.open(path) as im:
        return im.convert('RGB').crop(tuple(round(n*im.width/640) for n in box)).tobytes()


def hd_theme(wid):
    # 整段經滑鼠操作選項列，收起後再驗證 4× 的遊戲座標。
    gui.move(wid, 270, 2)
    time.sleep(.6)
    click(wid, 270, 16)
    click(wid, 270, 68)
    click(wid, 616, 16)
    gui.run(['xdotool', 'windowsize', wid, '2560', '1632'])
    gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    gui.move(wid, 320, 200)
    time.sleep(.5)


def record(tag, reference):
    path = OUT / (tag + '.wav')
    gui.run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y',
             '-f', 'pulse', '-sample_rate', '48000', '-channels', '2',
             '-fragment_size', '4096', '-i', 'san1.monitor', '-t', '4', '-c:a', 'pcm_s16le', str(path)], timeout=15)
    with wave.open(str(path)) as wav:
        assert (wav.getframerate(), wav.getnchannels(), wav.getsampwidth()) == (48000, 2, 2)
        pcm = wav.readframes(wav.getnframes())
    samples = array.array('h', pcm)
    rms = 20*math.log10(max(1e-12, math.sqrt(sum(x*x for x in samples)/len(samples))/32768))
    expected = (OUT / reference).read_bytes()
    # 用錄音中央的非靜音連續區定位，再回讀整段，不挑選最像的一秒。
    probe = len(pcm)//2//4*4
    at = expected.find(pcm[probe:probe+4096])
    start = at-probe if at >= 0 else -1
    exact = start >= 0 and expected[start:start+len(pcm)] == pcm
    gui.receipt.setdefault('recordings', []).append(dict(file=path.name, sha256=sha(path),
        seconds=len(pcm)/192000, rms_dbfs=rms, reference=reference,
        reference_start_frame=start//4 if start >= 0 else None, complete_exact=exact))
    gui.check(tag+'-audible', rms > -60)
    gui.check(tag+'-complete-reference', exact)


def battle(wid, edition):
    assert not args.hd, '戰場樣本用原貌，高清座標另由 --hd 驗證'
    refs = ROOT/'workplace/promo-source/v.1.1.1-20261010'
    def prompt(path, name, rows):
        with Image.open(refs/(name+'-reference.png')) as im:
            if name == 'camp':
                # 郡名、軍力及將軍名不同，錯誤提示也可能推下一列。
                expected = im.convert('RGB').crop((0,32,112,48)).tobytes()
                return any(region(path,(448,268+16*r,560,284+16*r)) == expected for r in range(6))
            expected = im.convert('RGB').crop((0,0,176,rows*16)).tobytes()
        return region(path, (448,268,624,268+rows*16)) == expected
    def number(digits):
        points = {'0': (110,322), '1': (110,290)}
        for digit in digits:
            click(wid, *points[digit])
        click(wid, 302, 290)
    click(wid, 470, 164)  # 軍事
    click(wid, 460, 322)  # 發動戰役
    shot(wid, edition+'-attack-source')
    click(wid, 460, 228)  # 陳留
    target_image = shot(wid, edition+'-attack-target')
    # 挑郡清單的黃色是正式規則給出的有效目標；無主郡不能用發動戰役攻打。
    with Image.open(target_image) as im:
        valid = [n for n in range(1,43) if (255,255,85) in
                 im.convert('RGB').crop((432+(n-1)//14*64,60+(n-1)%14*16,
                                        496+(n-1)//14*64,76+(n-1)%14*16)).getdata()]
    assert valid, '正常挑郡清單沒有合法攻擊目標'
    target = valid[0]
    click(wid, 460+(target-1)//14*64, 68+(target-1)%14*16)
    shot(wid, edition+'-formation')
    number('1')
    number('1')
    number('')
    click(wid, 128, 358)  # 分配完畢
    number('0')
    number('1000')
    for _ in range(30):
        path = shot(wid, edition+'-await-camp')
        if prompt(path, 'camp', 2):
            break
        click(wid, 128, 358)  # 續行對白或確認勸諫
    else:
        raise RuntimeError('正常出兵尚未到達紮寨')
    gui.check(edition+'-mouse-only-reaches-camp', True)
    chosen = None
    for col, row in [(5,8),(4,8),(6,8),(5,7),(4,7),(6,7)]:
        click(wid, 56+48*col+24, 36+32*row+16*(col%2)+16)
        path = shot(wid, edition+'-camp-field-click')
        if prompt(path, 'command', 3):
            chosen = col, row
            break
    gui.check(edition+'-mouse-camp-submitted', chosen is not None)
    click(wid, 460, 276)  # 行軍
    shot(wid, edition+'-move-direction')
    col, row = chosen
    col += 1
    click(wid, 56+48*col+24, 36+32*row+16*(col%2)+16)
    shot(wid, edition+'-move-field-click')
    click(wid, 358, 390)  # 原有的 0 結束行軍
    shot(wid, edition+'-move-finished')
    gui.receipt.setdefault('battle_samples', []).append(dict(edition=edition,source=11,target=target,camp=chosen,normal_route=True))


def run_edition(edition, directory):
    saves = OUT / (edition+'-saves')
    saves.mkdir()
    command = [str(ROOT/args.binary), '-root', '/orig/'+directory, '-edition', edition,
               '-ai', edition, '-scale', '1', '-sound=false', '-saves', str(saves)]
    if args.hd:
        command += ['-hd-assets', str(ROOT/'workplace/hd-assets-mapcursor-v50-r1')]
    gui.receipt.setdefault('launch_commands', []).append(command)
    proc = gui.start(command, edition)
    wid = gui.wait(['xdotool', 'search', '--onlyvisible', '--pid', str(proc.pid), '--name', '三國演義 remake']).splitlines()[0]
    gui.run(['xdotool', 'windowmove', wid, '0', '0'])
    time.sleep(1)
    shot(wid, edition+'-logo')
    for _ in range(4):
        click(wid, 320, 200)
    shot(wid, edition+'-intro')
    if not args.ui_only:
        record(edition+'-intro', 'opening-reference.pcm')
    deadline = time.monotonic()+90
    while not menu_visible(shot(wid, edition+'-await-title')):
        assert time.monotonic() < deadline, '滑鼠略過片頭逾時'
        click(wid, 320, 200)
    if not args.ui_only:
        record(edition+'-title', 'music-reference.pcm')
    if args.hd:
        hd_theme(wid)
    click(wid, 250, 230)  # 新局
    shot(wid, edition+'-scenario')
    click(wid, 250, 230)  # 第一劇本
    shot(wid, edition+'-players')
    click(wid, 302, 290)  # 預設單人，數字鍵盤確認
    shot(wid, edition+'-lords')
    click(wid, 520, 100)  # 曹操
    shot(wid, edition+'-difficulty')
    click(wid, 302, 290)  # 預設難度五
    time.sleep(1)
    shot(wid, edition+'-main')
    click(wid, 118, 390)  # 郡資料旁的「指令」按鈕
    click(wid, 470, 116)  # 查看
    shot(wid, edition+'-view')
    click(wid, 320, 200, 3)
    shot(wid, edition+'-cancel')
    if args.ui_only:
        click(wid, 470, 116)
        click(wid, 460, 306)  # 查看那一郡
        shot(wid, edition+'-prefectures')
        click(wid, 460, 228)  # 陳留，編號 11
        shot(wid, edition+'-prefecture-picked')
        click(wid, 118, 390)
        click(wid, 470, 116)
        click(wid, 460, 322)  # 將軍冊
        shot(wid, edition+'-roster')
        click(wid, 500, 90)
        shot(wid, edition+'-general-card')
        for _ in range(2):
            click(wid, 320, 200, 3)
        click(wid, 580, 68)  # 商業
        click(wid, 460, 306)  # 買入米糧
        pad = shot(wid, edition+'-number-pad')
        for _ in range(6):
            click(wid, 238, 226)  # 9
        click(wid, 302, 290)
        retry = shot(wid, edition+'-number-range-retry')
        gui.check(edition+'-invalid-amount-stays-in-pad', region(pad, (80,212,332,336)) == region(retry, (80,212,332,336)))
        click(wid, 320, 200, 3)
        shot(wid, edition+'-number-cancel')
    click(wid, 580, 260)  # 其他
    shot(wid, edition+'-other')
    click(wid, 520, 300)  # 儲存
    shot(wid, edition+'-save-pick')
    click(wid, 500, 60)   # 第一槽
    shot(wid, edition+'-save-memo')
    click(wid, 358, 390)  # 空備註也可確認
    shot(wid, edition+'-saved')
    metadata = list(saves.rglob('REMAKE.JSON'))
    gui.check(edition+'-mouse-only-save', len(metadata) == 1)
    gui.receipt.setdefault('save_metadata', {})[edition] = json.loads(metadata[0].read_text())
    if args.battle:
        battle(wid, edition)
    if args.affair:
        click(wid, 470, 260)  # 內政
        click(wid, 460, 306)  # 開墾
        shot(wid, edition+'-reclaim-roster')
        click(wid, 500, 90)   # 第一位可用將軍
        shot(wid, edition+'-reclaim-result')
        gui.receipt.setdefault('affair_samples', []).append(dict(edition=edition,order='reclaim',normal_route=True))
    gui.stop(proc)


try:
    runtime = Path('/tmp/san1-pulse')
    runtime.mkdir(mode=0o700)
    os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', LP_NUM_THREADS='2',
                      XDG_RUNTIME_DIR=str(runtime), PULSE_SERVER=f'unix:{runtime}/native')
    gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp'], 'xvfb')
    gui.start(['pulseaudio', '-n', '--daemonize=no', '--exit-idle-time=-1',
               '--load=module-null-sink sink_name=san1 rate=48000 channels=2',
               f'--load=module-native-protocol-unix socket={runtime}/native auth-anonymous=1'], 'pulse')
    gui.wait(['pactl', 'info'])
    gui.run(['pactl', 'set-default-sink', 'san1'])
    gui.receipt['binary_sha256'] = sha(ROOT/args.binary)
    for edition in args.editions:
        run_edition(edition, '三國演義' if edition == 'base' else '三國演義1加強版')
    gui.receipt['passed'] = True
finally:
    gui.receipt.setdefault('passed', False)
    gui.receipt['process_status_before_cleanup'] = [p.poll() for p in gui.processes]
    for proc in reversed(gui.processes):
        gui.stop(proc)
    for log in gui.logs:
        log.close()
    (OUT/'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2)+'\n')
