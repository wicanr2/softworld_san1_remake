#!/usr/bin/env python3
"""容器內正常董卓新局、呂布對陳宮單挑的 HD 使用端驗證。

輸出目錄須先放正式 san1-window-check、title-reference.png，並由
hd-battle-branches-reference.go 產生提示參考。來源 PNG 來自 hd-inventory。
預設核對美術接入及原貌恢復，--verify-names 另核對英文完整對白；
--verify-text 加驗日文完整叫陣與應戰。姓名牌與原版規則 parity 另驗。
"""
import argparse
import ast
import functools
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import time
import textwrap

ROOT = Path('/src')
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--edition', choices=['base', 'plus'], required=True)
parser.add_argument('--out', type=Path, required=True)
parser.add_argument('--hd-assets', type=Path, required=True)
parser.add_argument('--verify-names', action='store_true', help='以自由字模核對英文叫陣與應戰的來源姓名')
parser.add_argument('--verify-text', action='store_true', help='以自由字模核對英文與日文完整叫陣及應戰')
args = parser.parse_args()
edition, pack = args.edition, args.hd_assets
spec = importlib.util.spec_from_file_location('window_check', ROOT / 'tools/verify-window-inner.py')
gui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gui)
gui.OUT = args.out

# 沿用已驗證的新局、出兵及選項列操作；不執行舊批的美術比較器。
route = ROOT / 'tools/verify-hd-battle-branches-inner.py'
helpers = {'flip', 'source_face', 'theme', 'await_prompt', 'reach_attack_turn'}
nodes = [n for n in ast.parse(route.read_text()).body
         if isinstance(n, ast.FunctionDef) and n.name in helpers]
exec(compile(ast.Module(body=nodes, type_ignores=[]), str(route), 'exec'), globals())


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def crop(raw, width, x, y, w, h):
    return b''.join(raw[((y + r) * width + x) * 3:((y + r) * width + x + w) * 3]
                    for r in range(h))


def nearest(raw, width, height):
    return b''.join(b''.join(raw[(y * width + x) * 3:(y * width + x + 1) * 3] * 4
                            for x in range(width)) * 4 for y in range(height))


@functools.lru_cache(maxsize=512)
def rgb(path):
    return gui.rgb(Path(path))


def blit(dst, width, raw, w, h, x, y):
    for row in range(h):
        dst[((y + row) * width + x) * 3:((y + row) * width + x + w) * 3] = raw[row * w * 3:(row + 1) * w * 3]


def save():
    (gui.OUT / 'receipt.json').write_text(json.dumps(gui.receipt, ensure_ascii=False, indent=2) + '\n')


def shot(wid, name):
    path = gui.OUT / (name + '-' + str(len(gui.receipt['captures'])) + '.png')
    g = gui.geo(wid)
    gui.run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y',
             '-f', 'x11grab', '-draw_mouse', '0', '-video_size', f"{g['WIDTH']}x{g['HEIGHT']}",
             '-i', f":99+{g['X']},{g['Y']}", '-frames:v', '1', '-threads', '1', str(path)])
    gui.receipt['captures'].append(dict(file=path.name, width=g['WIDTH'], height=g['HEIGHT'], sha256=sha(path)))
    return path


gui.shot = shot
manifest = json.loads((pack / 'manifest.json').read_text())['entries']
source = ROOT / 'workplace/hd-inventory' / edition / 'img/DATA3'


def asset(name):
    entry = next(e for e in manifest if e['edition'] == edition and e['name'] == name)
    assert sha(pack / entry['file']) == entry['sha256'], name
    return entry, rgb(str(pack / entry['file']))


def language(wid, row):
    gui.key(wid, 'Escape')
    gui.choose_ready(wid, 0, row)
    gui.key(wid, 'Escape')
    gui.move(wid, 20, 380)


def restore_whole(before, after):
    """只接受完整原始標記的補色相位；不遮罩或忽略差異。"""
    if before == after:
        return None
    for col in range(8):
        for row in range(10):
            x, y = 56 + 48 * col, 36 + 32 * row + 16 * (col % 2)
            a, b = crop(before, 640, x, y, 48, 32), crop(after, 640, x, y, 48, 32)
            if b != bytes(v ^ 255 for v in a):
                continue
            expected = bytearray(before)
            blit(expected, 640, b, 48, 32, x, y)
            if expected == after:
                return dict(col=col, row=row, rect=[x, y, 48, 32], full_complement=True)
    raise RuntimeError('原貌恢復有完整來源標記相位以外的差異')


def face_name(original, left):
    x, y = (448, 44) if left else (560, 156)
    observed = crop(original, 640, x, y, 64, 80)
    # 呂布是本路線的挑戰者。其他說話者從完整原圖辨識，不推測肖像編號。
    paths = [source / 'F006.png'] if left else sorted(source.glob('F[0-9][0-9][0-9].png'))
    for path in paths:
        raw = rgb(str(path))
        if observed == (flip(raw, 64, 80) if left else raw):
            return path.stem, (x, y)
    raise RuntimeError('單挑說話者的完整原生肖像未匹配')


def nameplate_native(name, ink):
    """從自由字模重建 A 的整行姓名，不抄正式畫面的墨點。"""
    glyphs = {}
    with gzip.open(ROOT / 'fonts/ascii6x10.hex.gz', 'rt') as stream:
        for line in stream:
            line = line.strip()
            if not line or line.startswith('#'):
                continue
            key, value = line.split(':', 1)
            glyphs[chr(int(key, 16))] = bytes.fromhex(value)
    sw, sh = len(name)*24, 40
    dw, dh = min(192, sw), 40 if sw <= 192 else 40*192//sw
    ox, oy = (192-dw)//2, (64-dh)//2
    expected = bytearray(192*64*3)
    for y in range(dh):
        sy = ((2*y+1)*sh)//(2*dh)
        for x in range(dw):
            sx = ((2*x+1)*sw)//(2*dw)
            if glyphs[name[sx//24]][sy//4] & (1 << (7-(sx%24)//4)):
                at = ((oy+y)*192+ox+x)*3
                expected[at:at+3] = bytes(ink)
    return bytes(expected)


def speech_expected(original, left, locale):
    """從固定幾何、原字墨及原生 PNG 組出整塊可見面板。"""
    x1, y1, x2, y2 = (448, 44, 623, 139) if left else (448, 156, 623, 251)
    _, skin = asset('PANEL.BEVEL#1')
    expected = bytearray(skin)
    name, (px, py) = face_name(original, left)
    native = rgb(str(pack / (name + '.png')))
    blit(expected, 720, flip(native, 256, 320) if left else native, 256, 320,
         (px - (x1 - 2)) * 4, (py - (y1 - 2)) * 4)
    bx1, bx2 = (x1 + 70, x2 - 5) if left else (x1 + 4, x2 - 70)
    nx = x1 + 8 if left else x2 - 55
    rects = [(nx, y1 + 80, nx + 48, y1 + 96),
             (bx1, y1 + 5, bx2 + 1, y2 - 4),
             (bx1, y1 + 4, bx2 + 1, y1 + 5),
             (bx1, y2 - 4, bx2 + 1, y2 - 3),
             (bx1 - 1, y1 + 5, bx1, y2 - 4),
             (bx2 + 1, y1 + 5, bx2 + 2, y2 - 4)]
    for i in range(4):
        tx = x1 + 65 + i if left else x2 - 65 - i
        rects.append((tx, y1 + 47 - i, tx + 1, y1 + 49 + i))
    for ax, ay, bx, by in rects:
        raw = crop(original, 640, ax, ay, bx - ax, by - ay)
        blit(expected, 720, nearest(raw, bx - ax, by - ay), (bx - ax) * 4, (by - ay) * 4,
             (ax - (x1 - 2)) * 4, (ay - (y1 - 2)) * 4)
    if locale == 'en' and not left:
        native_name = nameplate_native('Chen Gong', (85, 255, 85))
        logical_name = b''.join(native_name[(y*4*192+x*4)*3:(y*4*192+x*4)*3+3]
                                for y in range(16) for x in range(48))
        gui.check('english-complete-nameplate-original',
                  crop(original, 640, nx, y1+80, 48, 16) == logical_name)
        blit(expected, 720, native_name, 192, 64, (nx-(x1-2))*4, 82*4)
    return bytes(expected), [x1 - 2, y1 - 2, 180, 100], name


def translated_speech_pixels(original, left, key, locale):
    """獨立讀自由字模與完整句；核對全字區，不從畫面抄字墨。"""
    names = ({'bub.duelChallenge': 'Chen Gong', 'bub.duelAccept': 'Lu Bu'} if locale == 'en'
             else {'bub.duelChallenge': '陳宮', 'bub.duelAccept': '呂布'})
    catalog = json.loads((ROOT / f'internal/i18n/lang/{locale}.json').read_text())
    text = catalog[key] % names[key]
    if locale == 'en':
        lines = textwrap.wrap(text, 12)
    else:
        # 這兩句日文沒有拉丁單字；逐全形字與空白獨立計寬。
        lines, line, width = [], '', 0
        for char in text:
            step = 1 if ord(char) < 128 else 2
            if width + step > 12:
                lines.append(line.strip())
                line, width = '', 0
                if char == ' ':
                    continue
            line += char
            width += step
        if line.strip():
            lines.append(line.strip())
    assert ''.join(''.join(lines).split()) == ''.join(text.split()), '完整句不得丟字'
    assert 2 < len(lines) <= 5, '此正常路徑應使用一般字級縮排'
    glyphs = {}
    with gzip.open(ROOT / 'fonts/unifont.hex.gz', 'rt') as font:
        for line in font:
            if ':' not in line:
                continue
            code, bits = line.strip().split(':')
            char = chr(int(code, 16))
            if char in text:
                glyphs[char] = bytes.fromhex(bits)
    assert all(len(glyphs[char]) in (16, 32) for char in set(''.join(lines))), '須有完整自由字模'
    x, y = (520, 52) if left else (456, 164)
    actual = crop(original, 640, x, y, 96, 83)
    palette = [(0, 0, 0), (0, 0, 170), (0, 170, 0), (0, 170, 170),
               (170, 0, 0), (170, 0, 170), (170, 85, 0), (170, 170, 170)]
    for index, color in enumerate(palette):
        expected = bytearray(b'\xff' * (96 * 83 * 3))
        for row, line in enumerate(lines):
            column = 0
            for char in line:
                glyph = glyphs[char]
                stride = len(glyph) // 16
                for gy in range(16):
                    bits = int.from_bytes(glyph[gy * stride:(gy + 1) * stride], 'big')
                    for gx in range(stride * 8):
                        if bits & (1 << (stride * 8 - gx - 1)):
                            at = ((row * 16 + gy) * 96 + column + gx) * 3
                            expected[at:at + 3] = bytes(color)
                column += stride * 8
            assert column <= 96, '整行字模須在原框內'
        if actual == expected:
            return dict(key=key, locale=locale, source_name=names[key], full_text=text, visible_lines=lines,
                        complete_text=True, logical_rect=[x, y, 96, 83], color=index,
                        expected_rgb_sha256=hashlib.sha256(expected).hexdigest()), bytes(expected)
    raise RuntimeError('叫陣／應戰的完整對白文字區與自由字模不符')


def sample(wid, stage, kind, value, text_key=None):
    for row, locale in enumerate(('zh-Hant', 'en', 'ja')):
        if row:
            language(wid, row)
        tag = edition + '-' + stage + '-' + locale
        original = shot(wid, tag + '-original')
        old = gui.rgb(original)
        projection, name_expected = None, None
        if text_key and ((args.verify_names and locale == 'en') or
                         (args.verify_text and locale in ('en', 'ja'))):
            projection, name_expected = translated_speech_pixels(old, value, text_key, locale)
            gui.check(tag + '-source-name-text', True)
        if kind == 'scene':
            expected_original = rgb(str(source / (f'SCG{value:02d}.png')))
            gui.check(tag + '-whole-source-scene', crop(old, 640, 448, 268, 176, 96) == expected_original)
            entry, expected = asset(f'SCG{value:02d}.IMG')
            rect = [448, 268, 176, 96]
            material = entry['name']
        else:
            expected, rect, material = speech_expected(old, value, locale)
        theme(wid, True)
        high = shot(wid, tag + '-hd')
        native = gui.rgb(high)
        gui.dimensions(wid, tag + '-native-dimensions', 2560, 1632)
        x, y, w, h = rect
        actual = crop(native, 2560, x * 4, y * 4, w * 4, h * 4)
        gui.check(tag + '-all-native-region-pixels', actual == expected)
        if projection:
            tx, ty, tw, th = projection['logical_rect']
            gui.check(tag + '-native-source-name-text',
                      crop(native, 2560, tx * 4, ty * 4, tw * 4, th * 4) == nearest(name_expected, tw, th))
        theme(wid, False)
        restored = shot(wid, tag + '-restored')
        phase = restore_whole(old, gui.rgb(restored))
        gui.check(tag + '-whole-original-restored', True)
        gui.receipt.setdefault('samples', []).append(dict(stage=stage, language=locale, kind=kind,
            material=material, logical_rect=rect, pixels=w * h * 16,
            original=original.name, high=high.name, restored=restored.name, marker_phase=phase,
            expected_rgb_sha256=hashlib.sha256(expected).hexdigest(), actual_rgb_sha256=hashlib.sha256(actual).hexdigest()))
        if projection:
            gui.receipt['samples'][-1]['speech_text'] = projection
        save()
    language(wid, 0)


def scene_at(raw):
    region = crop(raw, 640, 448, 268, 176, 96)
    for n in (29, 27, 28):
        if region == rgb(str(source / f'SCG{n:02d}.png')):
            return n
    return None


def await_scene(wid, tag):
    deadline = time.monotonic() + 40
    while time.monotonic() < deadline:
        p = shot(wid, tag)
        n = scene_at(gui.rgb(p))
        if n is not None:
            return n
        time.sleep(.2)
    raise RuntimeError('單挑完整場景未出現')


def await_stage(wid, menu):
    deadline = time.monotonic() + 40
    while time.monotonic() < deadline:
        p = shot(wid, edition + '-duel-step')
        raw = gui.rgb(p)
        n = scene_at(raw)
        if n is not None:
            return 'scene', n, p
        if crop(raw, 640, 448, 44, 64, 80) == source_face(edition, 'F006', True):
            return 'speech', True, p
        normal = (crop(raw, 640, 456, 52, 64, 80) == source_face(edition, 'F006', True)
                  and crop(raw, 640, 552, 164, 64, 80) == source_face(edition, 'F000', False))
        if normal and crop(raw, 640, 448, 268, 176, 32) == crop(menu, 176, 0, 0, 176, 32):
            return 'returned', None, p
        if not normal:
            try:
                face_name(raw, False)
            except RuntimeError:
                pass
            else:
                return 'speech', False, p
        time.sleep(.2)
    raise RuntimeError('單挑續頁未抵達可辨識完整畫面')


def play():
    proc, wid = gui.launch(edition, hd_assets=pack, tag=edition + '-duel')
    gui.key(wid, '1', '1', '1', '6', '5')
    waiting, rested = reach_attack_turn(wid, edition, edition)
    gui.key(wid, '2', 'Return', '2')
    if waiting == 14:
        gui.key(wid, '1', '5', 'Return')
    gui.key(wid, '1', '1', 'Return', '2', 'Return', '1', 'Return', 'Return', 'y',
            '0', 'Return', '1', '0', '0', '0', 'Return')
    await_prompt(wid, edition + '-camp', 'camp', 2)
    gui.key(wid, '3', '3', '6', '3', '0')
    await_prompt(wid, edition + '-command', 'command', 3)
    gui.key(wid, '2')
    await_prompt(wid, edition + '-direction', 'engage-direction', 1, False)
    gui.key(wid, '2')
    await_prompt(wid, edition + '-child', 'skirmish', 2)
    # 實際正常探查的合法行軍、休息與方向輸入。失敗時保留收據，不重擲。
    for keys in [('1', '5'), ('Return',), ('1', '5'), ('0', 'y'), ('1', '5')]:
        gui.key(wid, *keys)
        shot(wid, edition + '-approach')
    gui.key(wid, '2', '5')
    first = await_scene(wid, edition + '-first-scene')
    gui.check(edition + '-normal-duel-scene29', first == 29)
    gui.receipt['normal_route'] = dict(scenario='001', player='董卓', waiting=waiting, rested=rested,
        from_prefecture=15, target=11, leader='呂布', camp=['3', '3', '6', '3', '0'],
        approach=[['1', '5'], ['Return'], ['1', '5'], ['0', 'y'], ['1', '5']], duel=['2', '5'])
    sample(wid, 'challenge-scene', 'scene', first)
    speeches, scenes = [], [first]
    menu = rgb(str(gui.OUT / 'skirmish-reference.png'))
    for step in range(24):
        gui.key(wid, 'space')
        kind, value, p = await_stage(wid, menu)
        if kind == 'scene':
            scenes.append(value)
            sample(wid, 'result-scene-' + str(step), 'scene', value)
            continue
        # 泡泡肖像與普通軍力面板的錨點不同。完整來源肖像辨識，不從選單推定對白已完。
        if kind == 'speech':
            side = 'attacker' if value else 'defender'
            key = 'bub.duelChallenge' if step == 0 and value else ('bub.duelAccept' if step == 1 and not value else None)
            sample(wid, side + '-speech-' + str(step), 'speech', value, key)
            speeches.append(side)
            continue
        if kind == 'returned':
            gui.check(edition + '-normal-return-to-skirmish-menu', True)
            gui.receipt['returned'] = p.name
            break
        raise RuntimeError('單挑續頁抵達未辨識畫面；保留實圖供診斷')
    else:
        raise RuntimeError('單挑未在有界續頁內返回')
    gui.check(edition + '-both-speech-sides-observed', {'attacker', 'defender'}.issubset(speeches))
    gui.check(edition + '-actual-result-scene-observed', len(scenes) == 2 and scenes[-1] in (27, 28, 29))
    if args.verify_names or args.verify_text:
        wanted = {(locale, key) for locale in (('en', 'ja') if args.verify_text else ('en',))
                  for key in ('bub.duelChallenge', 'bub.duelAccept')}
        gui.check(edition + '-complete-translated-speeches-observed',
                  {(s['speech_text']['locale'], s['speech_text']['key']) for s in gui.receipt['samples']
                   if 'speech_text' in s} == wanted)
    gui.receipt.update(observed_scenes=scenes, observed_speeches=speeches)
    gui.stop(proc)


# 前置拒收放在 finally 外，才能保護已存在的收據。
assert gui.OUT.is_dir() and gui.OUT.stat().st_uid == os.getuid() and gui.OUT.stat().st_gid == os.getgid()
assert not (gui.OUT / 'receipt.json').exists(), '禁止覆寫先前收據'
try:
    os.environ.update(DISPLAY=':99', LIBGL_ALWAYS_SOFTWARE='1', XDG_RUNTIME_DIR='/tmp')
    gui.receipt.update(edition=edition,
        scope='兩版正常單挑場景、鏡像肖像、完整泡泡面板與三語 Theme 切換；只驗美術接入',
        state_injection=False, seed_injection=False, clock_injection=False,
        randomness='正式新局預設亂數；不重擲、不挑成功結果；不是 dosgolem oracle',
        audio='關閉音訊；未驗音畫', save_load='未驗戰役結束後存讀檔',
        localization=('自由字模核對英文與日文完整叫陣及應戰；姓名牌另驗' if args.verify_text else
                      '自由字模核對英文完整叫陣與應戰；日文及姓名牌另驗' if args.verify_names else
                      '只驗美術；未驗文字完整或插入姓名'),
        verify_names=args.verify_names, verify_text=args.verify_text,
        binary_sha256=sha(gui.OUT / 'san1-window-check'), manifest_sha256=sha(pack / 'manifest.json'),
        sources_sha256={str(p.relative_to(ROOT)): sha(p) for p in
                        [Path(__file__).resolve(), route, ROOT / 'tools/verify-window-inner.py']} )
    for path in gui.receipt['sources_sha256']:
        source_path = Path(path)
        with (gui.OUT / source_path.name).open('xb') as snapshot:
            snapshot.write(source_path.read_bytes())
    gui.start(['Xvfb', ':99', '-screen', '0', '2800x1900x24', '-nolisten', 'tcp', '-noreset', '-ac'], 'xvfb')
    gui.wait(['xdotool', 'getdisplaygeometry'])
    play()
    gui.receipt['passed'] = True
finally:
    gui.receipt.setdefault('passed', False)
    for proc in reversed(gui.processes):
        gui.stop(proc)
    for log in gui.logs:
        log.close()
    save()
