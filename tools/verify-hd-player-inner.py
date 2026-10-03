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
commanders_only = sys.argv[1:] == ['--commanders']
commanders_next_only = sys.argv[1:] == ['--commanders-next']
officers_only = sys.argv[1:] == ['--officers']
officers_all_only = sys.argv[1:] == ['--officers-all']
if sys.argv[1:] and not (scene_only or custom_only or portraits_only or lords_only or lords_all_only or commanders_only or commanders_next_only or officers_only or officers_all_only):
    raise SystemExit('僅接受 --scene-plus、--custom、--portraits、--lords、--lords-all、--commanders、--commanders-next、--officers 或 --officers-all')
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
if commanders_only:
    gui.OUT /= 'commanders-v6'
if commanders_next_only:
    gui.OUT /= 'commanders-v10'
if officers_only:
    gui.OUT /= 'commanders-v11'
if officers_all_only:
    gui.OUT /= 'commanders-v12'
pack_dir = gui.ROOT / ('workplace/hd-assets-custom-v1' if custom_only else
                       'workplace/hd-assets-portraits-v12' if officers_all_only else
                       'workplace/hd-assets-portraits-v11' if officers_only else
                       'workplace/hd-assets-portraits-v10' if commanders_next_only else
                       'workplace/hd-assets-portraits-v6' if commanders_only else
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
    if commanders_next_only or officers_only or officers_all_only:
        source = gui.ROOT / 'workplace/hd-inventory' / edition / 'img/DATA3' / (name + '.png')
        gui.receipt.setdefault('source_reference_sha256', {})[edition + '/' + name] = \
            hashlib.sha256(source.read_bytes()).hexdigest()
        check(edition + '-' + tag + '-original-source-pixels',
              rgb(original, f'64:80:{x}:{y}') == rgb(source))
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
    if commanders_only:
        gui.receipt['method'] = 'Linux Xvfb 正常片頭、劇本 001–004、單人曹操、查看郡及檢視十位軍師／武將'
        gui.receipt['scenarios'] = ['001', '002', '003', '004']
        # 兩版正式新局第一次玩家停點的 PickAny／PickByStatus 清單相同。
        plans = [('001', '1', [('F111', '逢紀', 48, 4, 6, True),
                               ('F155', '夏侯淵', 30, 11, 3, False)]),
                 ('002', '2', [('F210', '王楷', 313, 11, 7, True),
                               ('F146', '典韋', 74, 15, 8, False),
                               ('F014', '周瑜', 96, 22, 2, True)]),
                 ('003', '3', [('F010', '司馬懿', 157, 13, 2, False),
                               ('F156', '許褚', 88, 13, 9, False),
                               ('F254', '陸遜', 152, 23, 1, True)]),
                 ('004', '4', [('F007', '龐統', 7, 28, 6, True),
                               ('F062', '黃忠', 5, 31, 2, True)])]
    if commanders_next_only:
        gui.receipt['method'] = 'Linux Xvfb 正常片頭、劇本 001、單人曹操、查看郡及檢視二十位武將；未注入狀態'
        gui.receipt['verification_mode'] = '--commanders-next'
        gui.receipt['state_injection'] = False
        gui.receipt['scenarios'] = ['001']
        # 兩版正式 session 第一次玩家停點的 PickAny／PickByStatus 清單。
        plans = [('001', '1', [
            ('F250', '曹仁', 343, 11, 4, False),
            ('F018', '曹洪', 344, 11, 5, False),
            ('F125', '樂進', 27, 11, 6, False),
            ('F190', '曹純', 112, 11, 7, False),
            ('F112', '陳宮', 26, 11, 8, False),
            ('F131', '張邈', 31, 11, 9, False),
            ('F249', '田豐', 51, 3, 2, True),
            ('F047', '顏良', 41, 3, 6, True),
            ('F102', '文醜', 42, 3, 5, True),
            ('F168', '許攸', 53, 3, 10, True),
            ('F129', '郭圖', 116, 3, 7, True),
            ('F178', '審配', 89, 4, 3, True),
            ('F113', '沮授', 52, 4, 4, True),
            ('F233', '張郃', 119, 4, 2, True),
            ('F179', '高覽', 118, 4, 10, True),
            ('F076', '李儒', 21, 14, 2, True),
            ('F160', '賈詡', 63, 15, 1, True),
            ('F052', '華雄', 36, 15, 3, True),
            ('F241', '程普', 37, 31, 2, True),
            ('F055', '黃蓋', 39, 31, 3, True)])]
    if officers_only:
        gui.receipt['method'] = 'Linux Xvfb 正常片頭、劇本 001、單人曹操、他國查看及檢視二十位武將；未注入狀態'
        gui.receipt['verification_mode'] = '--officers'
        gui.receipt['state_injection'] = False
        gui.receipt['scenarios'] = ['001']
        # 兩版正式 session 初次玩家停點的查看清單。
        plans = [('001', '1', [
            ('F186', '孫乾', 87, 9, 2, True),
            ('F181', '陳登', 76, 10, 2, True),
            ('F103', '麋芳', 120, 10, 3, True),
            ('F173', '麋竺', 75, 10, 4, True),
            ('F021', '太史慈', 77, 21, 2, True),
            ('F226', '陳武', 101, 21, 3, True),
            ('F213', '虞翻', 104, 23, 2, True),
            ('F094', '韓當', 38, 31, 4, True),
            ('F189', '朱治', 94, 31, 5, True),
            ('F244', '祖茂', 40, 31, 6, True),
            ('F100', '丁奉', 155, 31, 7, True),
            ('F246', '蔡瑁', 47, 28, 2, True),
            ('F092', '蒯越', 46, 28, 3, True),
            ('F130', '伊籍', 137, 28, 4, True),
            ('F169', '黃祖', 59, 29, 1, True),
            ('F091', '蒯良', 45, 30, 1, True),
            ('F167', '文聘', 143, 30, 2, True),
            ('F187', '吳懿', 210, 36, 4, True),
            ('F218', '張任', 203, 37, 2, True),
            ('F088', '黃權', 198, 37, 5, True)])]
    if officers_all_only:
        gui.receipt['method'] = 'Linux Xvfb 正常片頭、劇本 001、單人曹操、他國查看及檢視四十五位武將；未注入狀態'
        gui.receipt['verification_mode'] = '--officers-all'
        gui.receipt['state_injection'] = False
        gui.receipt['scenarios'] = ['001']
        # 兩版正式 session 初次玩家停點的查看清單。
        plans = [('001', '1', [
            ('F073', '嚴綱', 298, 2, 2, True),
            ('F199', '公孫越', 54, 2, 3, True),
            ('F022', '袁尚', 131, 3, 3, True),
            ('F126', '袁譚', 129, 3, 4, True),
            ('F050', '淳于瓊', 290, 3, 9, True),
            ('F080', '辛評', 50, 3, 11, True),
            ('F089', '袁熙', 130, 4, 1, True),
            ('F071', '張顗', 320, 4, 5, True),
            ('F175', '麴義', 297, 4, 7, True),
            ('F045', '馬延', 319, 4, 8, True),
            ('F137', '高幹', 317, 4, 9, True),
            ('F148', '陳琳', 117, 4, 11, True),
            ('F134', '董旻', 25, 6, 1, True),
            ('F096', '郭汜', 18, 6, 2, True),
            ('F032', '李肅', 24, 6, 4, True),
            ('F231', '楊彪', 291, 6, 5, True),
            ('F170', '董璜', 62, 6, 6, True),
            ('F211', '鮑信', 292, 7, 3, True),
            ('F066', '曹豹', 300, 9, 4, True),
            ('F205', '袁胤', 342, 13, 1, True),
            ('F188', '雷薄', 107, 13, 2, True),
            ('F185', '陳蘭', 108, 13, 3, True),
            ('F116', '徐榮', 43, 15, 4, True),
            ('F065', '王允', 345, 15, 5, True),
            ('F093', '胡軫', 294, 15, 6, True),
            ('F147', '張繡', 111, 16, 1, True),
            ('F223', '趙岑', 295, 16, 4, True),
            ('F191', '程銀', 186, 19, 4, True),
            ('F151', '楊秋', 192, 19, 5, True),
            ('F118', '華歆', 125, 24, 1, True),
            ('F165', '紀靈', 93, 27, 2, True),
            ('F067', '李豐', 308, 27, 3, True),
            ('F240', '呂公', 61, 29, 2, True),
            ('F243', '陳生', 60, 29, 3, True),
            ('F105', '張允', 335, 29, 4, True),
            ('F196', '劉磐', 175, 30, 3, True),
            ('F036', '孟達', 11, 36, 2, True),
            ('F225', '王累', 200, 36, 6, True),
            ('F079', '吳蘭', 211, 37, 1, True),
            ('F027', '冷苞', 202, 37, 3, True),
            ('F204', '雷同', 212, 37, 4, True),
            ('F183', '譙周', 222, 38, 1, True),
            ('F090', '楊懷', 205, 38, 3, True),
            ('F161', '高沛', 206, 38, 4, True),
            ('F133', '張肅', 208, 38, 5, True)])]
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
        if portraits_only or lords_only or lords_all_only or commanders_only or commanders_next_only or officers_only or officers_all_only:
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
