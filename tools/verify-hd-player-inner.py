#!/usr/bin/env python3
"""高清素材的正常新局、人物卡、地震與自創君主路徑。"""
import hashlib
import ctypes
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import time

spec = importlib.util.spec_from_file_location('window_check', Path(__file__).with_name('verify-window-inner.py'))
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = gui.ROOT / 'workplace/hd-window/player'
scene_only = sys.argv[1:] == ['--scene-plus']
custom_only = sys.argv[1:] == ['--custom']
portraits_only = sys.argv[1:] == ['--portraits']
lords_only = sys.argv[1:] == ['--lords']
lords_all_only = sys.argv[1:] == ['--lords-all']
if sys.argv[1:] and not (scene_only or custom_only or portraits_only or lords_only or lords_all_only):
    raise SystemExit('僅接受 --scene-plus、--custom、--portraits、--lords 或 --lords-all')
if scene_only:
    gui.OUT /= 'scene-plus'
if custom_only:
    gui.OUT /= 'custom-v1'
if portraits_only:
    gui.OUT /= 'portraits-v2'
if lords_only:
    gui.OUT /= 'lords-v3'
if lords_all_only:
    gui.OUT /= 'lords-v4'
pack_dir = gui.ROOT / ('workplace/hd-assets-custom-v1' if custom_only else
                       'workplace/hd-assets-portraits-v4' if lords_all_only else
                       'workplace/hd-assets-portraits-v3' if lords_only else
                       'workplace/hd-assets-portraits-v2' if portraits_only else 'workplace/hd-assets')
gui.receipt['method'] = 'Linux Xvfb 正常片頭、新局、查看武將與休息換月；未注入人物、日期或事件'
gui.receipt['scenario'] = '001'
gui.receipt['difficulty'] = 5
gui.receipt['randomness'] = '正式新局預設局面雜湊，無 LCG seed 注入；不是原版 oracle 收據'
run, key, shot, rgb, check = gui.run, gui.key, gui.shot, gui.rgb, gui.check


class XImage(ctypes.Structure):
    # Xlib.h 的 XImage 前段；不存取其後的函式表。
    _fields_ = [(name, ctypes.c_int) for name in ['width', 'height', 'xoffset', 'format']] + \
        [('data', ctypes.c_void_p)] + [(name, ctypes.c_int) for name in
        ['byte_order', 'bitmap_unit', 'bitmap_bit_order', 'bitmap_pad', 'depth', 'bytes_per_line', 'bits_per_pixel']] + \
        [(name, ctypes.c_ulong) for name in ['red_mask', 'green_mask', 'blue_mask']]


def scene_rgb():
    """直接讀 Xvfb，避免短動畫被 FFmpeg 的輸入分析等待吞掉。"""
    lib = ctypes.CDLL('libX11.so.6')
    lib.XOpenDisplay.argtypes, lib.XOpenDisplay.restype = [ctypes.c_char_p], ctypes.c_void_p
    lib.XDefaultRootWindow.argtypes, lib.XDefaultRootWindow.restype = [ctypes.c_void_p], ctypes.c_ulong
    lib.XGetImage.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_int, ctypes.c_int,
                             ctypes.c_uint, ctypes.c_uint, ctypes.c_ulong, ctypes.c_int]
    lib.XGetImage.restype = ctypes.POINTER(XImage)
    lib.XDestroyImage.argtypes = [ctypes.POINTER(XImage)]
    lib.XCloseDisplay.argtypes = [ctypes.c_void_p]
    display = lib.XOpenDisplay(b':99')
    if not display:
        raise RuntimeError('X11 顯示器尚未就緒')
    im = None
    try:
        im = lib.XGetImage(display, lib.XDefaultRootWindow(display), 1728, 320,
                          704, 384, ctypes.c_ulong(-1).value, 2)
        if not im:
            raise RuntimeError('X11 擷取失敗')
        info = im.contents
        if (info.width, info.height, info.bits_per_pixel, info.byte_order,
            info.red_mask, info.green_mask, info.blue_mask) != (704, 384, 32, 0, 0xff0000, 0xff00, 0xff):
            raise RuntimeError('Xvfb 像素格式與驗證契約不同')
        raw = ctypes.string_at(info.data, info.bytes_per_line*info.height)
        packed = b''.join(raw[y*info.bytes_per_line:y*info.bytes_per_line+704*4] for y in range(384))
        pixels = bytearray(704*384*3)
        pixels[0::3], pixels[1::3], pixels[2::3] = packed[2::4], packed[1::4], packed[0::4]
        return bytes(pixels)
    finally:
        if im:
            lib.XDestroyImage(im)
        lib.XCloseDisplay(display)


def theme(wid, high):
    key(wid, 'Escape')
    gui.choose_ready(wid, 1, int(high))
    key(wid, 'Escape')
    run(['xdotool', 'windowsize', wid, '2560' if high else '640', '1632' if high else '408'])
    run(['xdotool', 'windowmove', wid, '0', '0'])
    gui.move(wid, 320, 200)
    time.sleep(.5)


def portrait(wid, edition, tag, name, x, y):
    original = shot(wid, edition + '-' + tag + '-original')
    theme(wid, True)
    high = shot(wid, edition + '-' + tag + '-hd')
    master = pack_dir / (name + '.png')
    check(edition + '-' + tag + '-native-pixels', rgb(high, f'256:320:{x*4}:{y*4}') == rgb(master))
    # 右側資料面板的文字、外框及其他原圖，全部留在原座標。
    before = rgb(original, '224:256:408:36,scale=896:1024:flags=neighbor')
    after = rgb(high, '896:1024:1632:144')
    left, top = (x - 408)*4, (y - 36)*4
    outside = all(before[(row*896)*3:(row*896+left)*3] == after[(row*896)*3:(row*896+left)*3] and
                  before[(row*896+left+256)*3:((row+1)*896)*3] == after[(row*896+left+256)*3:((row+1)*896)*3]
                  if top <= row < top+320 else
                  before[(row*896)*3:((row+1)*896)*3] == after[(row*896)*3:((row+1)*896)*3]
                  for row in range(1024))
    check(edition + '-' + tag + '-text-frame-unchanged', outside)
    theme(wid, False)
    restored = shot(wid, edition + '-' + tag + '-restored')
    check(edition + '-' + tag + '-original-restored', rgb(original, '224:256:408:36') == rgb(restored, '224:256:408:36'))


def inspect(wid, pref, index, foreign):
    key(wid, '1', '1', *str(pref), 'Return')
    key(wid, '1', '3')
    if foreign:
        key(wid, 'y')
    key(wid, *str(index), 'Return')


def custom_rulers(edition):
    gui.receipt['method'] = 'Linux Xvfb 正常片頭、劇本 001／006、多人自創君主、設定、出現對白及主畫面'
    gui.receipt['scenarios'] = ['001', '006']
    plans = [('001', '1', '2', ['Right', 'Right', '3', '4'], ['F011', 'F001']),
             ('006', '6', '4', ['5', '6', 'Right', '1', '2'], ['F011', 'F001', 'F015', 'F009'])]
    for scenario, button, players, selections, faces in plans:
        gui.receipt.setdefault('scenario_key_start', {})[edition + '-' + scenario] = len(gui.receipt.get('keys', []))
        proc, wid = gui.launch(edition, hd_assets=pack_dir)
        key(wid, '1', button, players, *selections, '5')
        for name in faces:
            tag = scenario + '-custom-' + name
            portrait(wid, edition, tag + '-setting', name, 536, 64)
            key(wid, '6')
            portrait(wid, edition, tag + '-born', name, 552, 66)
            key(wid, 'space')
        key(wid, '0', 'Return')
        portrait(wid, edition, scenario + '-custom-main', 'F011', 536, 116)
        gui.stop(proc)


def portrait_cards(edition):
    gui.receipt['method'] = 'Linux Xvfb 正常片頭、劇本 001／004、單人曹操、查看郡及檢視將軍'
    gui.receipt['scenarios'] = ['001', '004']
    # 以兩版 DATA2 的 game.PickRoster(PickAny, PickByStatus) 分別核對。
    # 諸葛亮與趙雲在 001 未出場，使用 004 的實際駐軍，不注入人物。
    plans = [('001', '1', [
        ('F228', '鮑忠', 296, 7, 2, True),
        ('F005', '劉備', 0, 8, 1, True),
        ('F002', '關羽', 1, 8, 2, True),
        ('F012', '張飛', 2, 8, 3, True),
        ('F236', '陳珪', 110, 9, 1, True),
        ('F000', '曹操', 13, 11, 1, False),
        ('F184', '夏侯惇', 29, 11, 2, False),
        ('F006', '呂布', 23, 15, 2, True),
        ('F020', '嚴顏', 217, 38, 2, True)]),
        ('004', '4', [('F004', '諸葛亮', 6, 28, 2, True),
                       ('F013', '趙雲', 3, 28, 3, True)])]
    if lords_only:
        # 正式還原 AI 在玩家第一次停點前已移動原版孫權；加強版留在 23。
        sun_pref = 21 if edition == 'base' else 23
        plans = [('001', '1', [('F053', '袁紹', 16, 3, 1, True),
                               ('F077', '董卓', 14, 14, 1, True),
                               ('F041', '孫堅', 15, 31, 1, True)]),
                 ('004', '4', [('F008', '孫權', 56, sun_pref, 1, True)])]
    if lords_all_only:
        gui.receipt['method'] = 'Linux Xvfb 正常片頭、劇本 001–005、單人曹操、查看郡及檢視二十位君主'
        gui.receipt['scenarios'] = ['001', '002', '003', '004', '005']
        # 核對各版正式 session 第一次停點的清單，選實際君主本人。
        # F128／F166 也供其他武將共用；不以較早郡的共用肖像代替君主。
        early = [
            ('F063', '公孫瓚', 35, 2, 1, True),
            ('F117', '孔融', 32, 7, 1, True),
            ('F114', '陶謙', 33, 10, 1, True),
            ('F097', '李傕', 17, 16, 3, True),
            ('F192', '馬騰', 34, 19, 1, True),
            ('F166', '劉繇', 78, 21, 1, True),
            ('F209', '王朗', 103, 23, 1, True),
            ('F054', '袁術', 22, 27, 1, True),
            ('F142', '劉焉', 12, 36, 1, True),
            ('F237', '劉璋', 114, 36, 3, True)]
        second = [
            ('F128', '楊奉', 90, 5, 1, True),
            ('F227', '張魯', 115, 18, 1, True),
            ('F164', '孫策', 55, 22, 1, True),
            ('F078', '劉度', 167, 28, 6, True),
            ('F098', '趙範', 170, 33, 1, False)]
        liu_biao = ('F081', '劉表', 44, 28, 1, True)
        early.append(liu_biao)
        plans = [('001', '1', early), ('002', '2', second),
                 ('003', '3', [('F046', '韓玄', 160, 31, 5, True),
                               ('F024', '金旋', 173, 32, 3, True)]),
                 ('004', '4', [('F229', '曹丕', 135, 13, 3, False)]),
                 ('005', '5', [('F119', '孟獲', 253, 40, 1, True)])]
    for scenario, button, cards in plans:
        tag = edition + '-' + scenario
        gui.receipt.setdefault('scenario_key_start', {})[tag] = len(gui.receipt.get('keys', []))
        proc, wid = gui.launch(edition, hd_assets=pack_dir, tag=tag)
        key(wid, '1', button, '1', '2', '5', '0', 'Return')
        for name, label, person, pref, choice, foreign in cards:
            gui.receipt.setdefault('card_plan', []).append({
                'edition': edition, 'scenario': scenario, 'key': name,
                'name': label, 'general': person, 'prefecture': pref,
                'choice': choice, 'foreign': foreign})
            inspect(wid, pref, choice, foreign)
            portrait(wid, edition, scenario + '-card-' + name, name, 536, 68)
            key(wid, 'space', 'Return')
        gui.stop(proc)


def capture_quake(wid, edition):
    theme(wid, True)
    target = rgb(pack_dir / 'SCG01.png')
    deadline = time.monotonic() + 220
    steps = 0
    while time.monotonic() < deadline:
        # 僅讀真實 X11 遊戲區，不從程式內部得知事件或跳過動畫。
        sample = scene_rgb()
        if sample == target:
            shot(wid, edition + '-natural-quake-hd')
            gui.receipt.setdefault('quake_input_count', {})[edition] = steps
            check(edition + '-natural-quake-native-pixels', True)
            theme(wid, False)
            original = shot(wid, edition + '-natural-quake-original')
            theme(wid, True)
            restored = shot(wid, edition + '-natural-quake-hd-restored')
            check(edition + '-quake-switch-restored', rgb(restored, '704:384:1728:320') == target)
            # 君主肖像也會隨 Theme 改動，終點為 y=196；文字區從 200 比較。
            check(edition + '-quake-lower-panel-unchanged',
                  rgb(original, '224:120:408:200,scale=896:480:flags=neighbor') ==
                  rgb(restored, '896:480:1632:800'))
            return
        # 看見候選素材的部分像素時先等拉幕完成，避免續行鍵收掉完整場景。
        matching = sum(sample[i:i+3] == target[i:i+3] for i in range(0, len(target), 3))
        if matching > 704*384//100:
            gui.receipt.setdefault('quake_partial_frames', {})[edition] = \
                gui.receipt.get('quake_partial_frames', {}).get(edition, 0) + 1
            time.sleep(.6)
            continue
        key(wid, 'Return')
        steps += 1
        if steps % 20 == 0:
            print(edition, '正常換月操作', steps, flush=True)
            shot(wid, edition + '-month-progress')
    shot(wid, edition + '-quake-timeout')
    raise RuntimeError(edition + ' 正常換年地震尚未抵達')


try:
    assert gui.OUT.stat().st_uid == os.getuid() and gui.OUT.stat().st_gid == os.getgid()
    os.environ.update({'DISPLAY': ':99', 'LIBGL_ALWAYS_SOFTWARE': '1', 'XDG_RUNTIME_DIR': '/tmp'})
    gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp'], 'xvfb')
    gui.wait(['xdotool', 'getdisplaygeometry'])
    gui.receipt['binary_sha256'] = hashlib.sha256((gui.OUT / 'san1-window-check').read_bytes()).hexdigest()
    gui.receipt['pack_sha256'] = hashlib.sha256((pack_dir / 'manifest.json').read_bytes()).hexdigest()
    gui.receipt['tool_sha256'] = {path.name: hashlib.sha256(path.read_bytes()).hexdigest()
        for path in [Path(__file__), Path(__file__).with_name('verify-window-inner.py'),
                     Path(__file__).with_name('verify-hd-player.sh')]}
    gui.receipt['input_sha256'] = {folder + '/' + name: hashlib.sha256(
        (Path('/orig') / folder / name).read_bytes()).hexdigest()
        for folder in ['三國演義', '三國演義1加強版']
        for name in ['DATA1.GRP', 'DATA2.GRP', 'DATA3.GRP']}
    for edition in (['plus'] if scene_only else ['base', 'plus']):
        gui.receipt.setdefault('edition_key_start', {})[edition] = len(gui.receipt.get('keys', []))
        if custom_only:
            custom_rulers(edition)
            continue
        if portraits_only or lords_only or lords_all_only:
            portrait_cards(edition)
            continue
        proc, wid = gui.launch(edition)
        if scene_only:
            gui.receipt['method'] = 'Linux Xvfb 正常片頭、劇本 001、單人董卓、休息換月及地震'
            key(wid, '1', '1', '1', '6', '5', '0', 'Return')
            capture_quake(wid, edition)
            gui.stop(proc)
            continue
        key(wid, '1', '1', '1')
        lord_original = shot(wid, edition + '-lordpick-original')
        theme(wid, True)
        lord_high = shot(wid, edition + '-lordpick-hd')
        for name, x in [('F005', 420), ('F000', 488)]:
            check(edition + '-lordpick-' + name, rgb(lord_high, f'256:320:{x*4}:224') ==
                  rgb(gui.ROOT / 'workplace/hd-assets' / (name + '.png')))
        theme(wid, False)
        key(wid, '2', '5')
        key(wid, '0', 'Return')
        inspect(wid, 11, 1, False)
        portrait(wid, edition, 'card-cao', 'F000', 536, 68)
        key(wid, 'space', 'Return')
        inspect(wid, 8, 1, True)
        portrait(wid, edition, 'card-liu', 'F005', 536, 68)
        key(wid, 'space', 'Return')
        inspect(wid, 7, 2, True)
        portrait(wid, edition, 'card-bao', 'F228', 536, 68)
        key(wid, 'space', 'Return')
        capture_quake(wid, edition)
        gui.stop(proc)
    gui.receipt['passed'] = True
finally:
    gui.receipt.setdefault('passed', False)
    for proc in reversed(gui.processes):
        gui.stop(proc)
    for log in gui.logs:
        log.close()
    (gui.OUT / 'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2) + '\n')
