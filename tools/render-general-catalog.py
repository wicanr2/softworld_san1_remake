#!/usr/bin/env python3
"""從已核對人物資料與正式 HD 素材產生姓名總覽及獨立 Markdown 圖鑑。"""
import argparse
import hashlib
import importlib.util
import json
import math
import os
from pathlib import Path

from PIL import Image, ImageDraw

SAN5_PALETTE = ['#202010', '#206510', '#BA3000', '#EFAA8A', '#104575', '#658A9A', '#BA7545', '#EFDFCF']
SAN5_FORMAT_REFERENCE = 'https://github.com/tzengyuxio/kaodata-re/blob/07595829805e88cb9057be02a036008451fa9c23/dekoei/sango.py#L60-L75'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def san5_face(data, portrait):
    """64×80，三個交錯位元平面。調色盤採固定版本的外部格式參考。"""
    start = portrait * 1920
    face = data[start:start + 1920]
    assert len(face) == 1920
    pixels = []
    for offset in range(0, len(face), 3):
        planes = face[offset:offset + 3]
        for bit in reversed(range(8)):
            pixels.append(sum(((value >> bit) & 1) << (2 - plane)
                              for plane, value in enumerate(planes)))
    # 用另一方向重建全部位元組，避免默默忽略尾端或位元平面。
    reconstructed = bytearray()
    for offset in range(0, len(pixels), 8):
        group = pixels[offset:offset + 8]
        for plane in reversed(range(3)):
            reconstructed.append(sum(((value >> plane) & 1) << (7 - bit)
                                     for bit, value in enumerate(group)))
    assert bytes(reconstructed) == face
    image = Image.new('P', (64, 80))
    image.putpalette([component for color in SAN5_PALETTE
                      for component in bytes.fromhex(color[1:])])
    image.putdata(pixels)
    return image.convert('RGB'), hashlib.sha256(face).hexdigest()


def render_sheet(root, people, labels, portraits, original_exports=None, san5_data=None):
    source = san5_data is not None
    cols, cell_w, cell_h, margin, header = 16, 160, 224, 24, 136
    canvas = Image.new('RGB', (cols * cell_w + margin * 2,
        header + math.ceil(len(people) / cols) * cell_h + margin), '#131a23')
    draw = ImageDraw.Draw(canvas)
    title = '原版＋San5 武將姓名總覽' if source else '三國演義　HD 武將姓名總覽'
    labels.heading(canvas, (margin, 22), title, scale=3)
    labels.heading(canvas, (margin, 80), '原版 346 位＋San5 新增 220 位＝566 位　同名人物依來源識別分列　未登場候選也收錄', scale=1)
    labels.heading(canvas, (margin, 106), '原圖供本機對照，San5 色盤採外部格式參考' if source else '471 張肖像　原版人物槽與 San5 來源編號分開標示　能力與來源見獨立圖鑑', scale=1)
    tiles, cached = [], {}
    for i, p in enumerate(people):
        key = p['key']
        art_key = (p['source'], p['portrait'])
        entry = portraits[art_key]
        if source and p['source'] == 'san5':
            path_label = 'KAODATA.S5'
            if art_key not in cached:
                image, digest = san5_face(san5_data, p['portrait'])
                cached[art_key] = image.resize((128, 160), Image.Resampling.NEAREST), digest
        else:
            path = (original_exports / 'img' / entry['container'] / f"F{p['portrait']:03}.png") if source else root / 'hd-assets' / entry['file']
            path_label = path.relative_to(root).as_posix()
            if art_key not in cached:
                digest = sha(path)
                if not source:
                    assert digest == entry['sha256'], path
                with Image.open(path) as image:
                    image.load()
                    assert image.size == ((64, 80) if source else (entry['width'], entry['height']))
                    cached[art_key] = image.convert('RGBA').resize((128, 160), Image.Resampling.NEAREST if source else Image.Resampling.LANCZOS), digest
        thumbnail, digest = cached[art_key]
        x, y = margin + i % cols * cell_w, header + i // cols * cell_h
        draw.rectangle((x + 2, y + 2, x + cell_w - 2, y + cell_h - 4), fill='#1c2632' if p['source'] == 'base' else '#1d302c')
        canvas.paste(thumbnail, (x + 16, y + 5), thumbnail if thumbnail.mode == 'RGBA' else None)
        assert 1 <= len(p['name']) <= 4
        labels.heading(canvas, (x + (cell_w - len(p['name']) * 32) // 2, y + 170), p['name'], scale=2)
        text = f"{p['index']:03} / F{p['portrait']:03}" if p['source'] == 'base' else f"San5 / {p['portrait']:03}"
        draw.text((x + 36, y + 208), text, font=labels.small, fill='#a9bacb')
        tiles.append({'key': key, 'source': p['source'], 'index': p.get('index'), 'name': p['name'], 'portrait': p['portrait'],
                      'file': path_label, 'sha256': digest, 'tile': [x, y, cell_w, cell_h]})
    return canvas, tiles, len(cached)


def expanded_section(root):
    data_path = root / 'internal/game/expanded_roster.json'
    manifest_path = root / 'hd-assets/expanded-portraits.json'
    data = json.loads(data_path.read_text())
    people = data['scenarios'][0]['people']
    art = {e['portrait']: e for e in json.loads(manifest_path.read_text())['entries']}
    assert len(people) == 220 and len({p['portrait'] for p in people}) == 220
    for e in art.values():
        assert Path(e['file']).name == e['file']
        assert sha(root / 'hd-assets' / e['file']) == e['sha256']
    lines = ['', '## 新增人物', '',
        '規格 022 的 remake 擴充共 220 位，以 San5 來源識別區分同名人物。原有 346 人的姓名與能力保持。六劇本依年代分別加入 220／207／198／180／175／168 位；本表列出第一劇本的完整候選，不表示所有人開局已登場。', '',
        '新增人物體能 80，戰力、謀略、魅力取自已核對來源。政治只供圖鑑查閱，不參與本作規則。220 張原創肖像已齊備；遊戲遇到缺圖或損毀時使用通用人物像。', '',
        '[擴充規格](spec/022-expanded-roster.md) · [來源核對](re/14-san5-roster.md) · [六劇本資料](../internal/game/expanded_roster.json) · [肖像目錄](../hd-assets/expanded-portraits.json) · [生成紀錄](../hd-assets/expanded-portrait-provenance.json)', '',
        '| 頭像 | 姓名 | 體能 | 戰力 | 謀略 | 政治 | 魅力 | 來源識別 |',
        '|---|---|---:|---:|---:|---:|---:|---|']
    for p in people:
        e = art.get(p['portrait'])
        picture = '待製作'
        if e:
            file = e['file']
            picture = f'<a href="../hd-assets/{file}"><img src="../hd-assets/{file}" width="80" alt="{p["name"]} HD 肖像"></a>'
        lines.append(f'| {picture} | {p["name"]} | 80 | {p["war"]} | {p["intel"]} | {p["politics"]} | {p["charm"]} | san5:{p["portrait"]} |')
    return lines


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path('/src'))
    parser.add_argument('--expanded-only', action='store_true', help='只更新新增人物表，保留既有總覽圖')
    parser.add_argument('--san5-kaodata', type=Path, help='本機唯讀 KAODATA.S5；另製原版＋San5 對照總覽')
    parser.add_argument('--original-exports', type=Path, help='san1assets 匯出的原版資料，預設 workplace/asset-overviews/base')
    args = parser.parse_args()
    root = args.root
    if args.expanded_only:
        md = root / 'docs/hd-generals.md'
        assert md.stat().st_uid == os.getuid()
        original = md.read_text().split('\n## 新增人物\n', 1)[0].rstrip()
        md.write_text(original + '\n' + '\n'.join(expanded_section(root)) + '\n')
        audit_path = root / 'docs/images/hd-generals-overview.json'
        assert audit_path.stat().st_uid == os.getuid()
        audit = json.loads(audit_path.read_text())
        audit['markdown_sha256'] = sha(md)
        audit['expanded_count'] = 220
        audit['expanded_data_sha256'] = sha(root / 'internal/game/expanded_roster.json')
        audit_path.write_text(json.dumps(audit, ensure_ascii=False, indent=2) + '\n')
        print(json.dumps({'expanded': 220, 'markdown_sha256': sha(md)}))
        return
    data_path = root / 'docs/reference/hd-generals.json'
    data = json.loads(data_path.read_text())
    people = data['people']
    assert len(people) == 346 and len({p['index'] for p in people}) == 346
    expanded_path = root / 'internal/game/expanded_roster.json'
    expanded = json.loads(expanded_path.read_text())['scenarios'][0]['people']
    assert len(expanded) == 220 and len({p['portrait'] for p in expanded}) == 220
    combined = [dict(p, source='base', key=f"base:{p['index']:03}") for p in people]
    combined += [dict(p, source='san5', key=f"san5:{p['portrait']}") for p in expanded]
    assert len(combined) == 566 and len({p['key'] for p in combined}) == 566
    spec = importlib.util.spec_from_file_location('overviews', root / 'tools/render-asset-overviews.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    labels = module.Labels(root / 'fonts/unifont.hex.gz')
    manifest = json.loads((root / 'hd-assets/manifest.json').read_text())
    portraits = {}
    for e in manifest['entries']:
        if e['edition'] == 'base' and e['name'].endswith('.FAC'):
            portraits['base', int(e['name'][1:4])] = e
    for e in json.loads((root / 'hd-assets/expanded-portraits.json').read_text())['entries']:
        portraits['san5', e['portrait']] = e
    canvas, tiles, portrait_count = render_sheet(root, combined, labels, portraits)
    assert portrait_count == 471
    png = root / 'docs/images/hd-generals-overview.png'
    md = root / 'docs/hd-generals.md'
    audit = root / 'docs/images/hd-generals-overview.json'
    for path in [png, md, audit]:
        assert path.parent.is_dir() and path.parent.stat().st_uid == os.getuid()
        assert not path.exists() or path.stat().st_uid == os.getuid()
    canvas.save(png, optimize=True)
    lines = ['# HD 武將圖鑑', '',
        '收錄原版 346 位與 San5 新增 220 位，共 566 位人物，使用 471 張正式 HD 肖像。共用肖像的人物各自列出，兩位馬忠依來源編號區分。點頭像可看原圖。', '',
        '原有 346 人的姓名、人物槽、肖像與能力值直接取自兩版劇本 001，逐筆相同；新增 220 人依規格 022 的已核對資料列出。未登場人物也列入；這些數值不代表所有年代或遊戲中的即時狀態。體能、戰力、謀略、魅力沿用遊戲用語。', '',
        '表字是外部查考資料，不是原版人物表欄位，也不寫入遊戲。各項附出處；「二手」表示採用百科資料，尚未由史書原文核實。史書、演義或姓名異文另註，未核得或同名難辨者保留「待考」。', '',
        '[武將姓名總覽原圖](images/hd-generals-overview.png) · [完整資料與來源](reference/hd-generals.json) · [HD 素材包](../hd-assets/README.md)', '',
        '| 頭像 | 姓名 | 表字與出處 | 體能 | 戰力 | 謀略 | 魅力 | 兵種 | 人物槽 | 肖像 |',
        '|---|---|---|---:|---:|---:|---:|---|---:|---|']
    for p in people:
        file = portraits['base', p['portrait']]['file']
        image = f'<a href="../hd-assets/{file}"><img src="../hd-assets/{file}" width="80" alt="{p["name"]} HD 肖像"></a>'
        courtesy = '待考'
        if p.get('courtesy'):
            courtesy = f'[{p["courtesy"]}]({p["courtesy_sources"][0]["url"]})'
            if p['courtesy_sources'][0]['kind'] == 'secondary':
                courtesy += '（二手）'
        if p.get('courtesy_note'):
            courtesy += '<br>' + p['courtesy_note']
        lines.append(f'| {image} | {p["name"]} | {courtesy} | {p["stamina"]} | {p["war"]} | {p["intel"]} | {p["charm"]} | {p["troop"]} | {p["index"]:03} | F{p["portrait"]:03} |')
    lines.extend(expanded_section(root))
    md.write_text('\n'.join(lines) + '\n')
    metadata = {'schema': 2, 'data_sha256': sha(data_path), 'expanded_data_sha256': sha(expanded_path),
        'image_sha256': sha(png), 'markdown_sha256': sha(md),
        'count': len(combined), 'original_count': len(people), 'expanded_count': len(expanded),
        'portraits': portrait_count, 'width': canvas.width, 'height': canvas.height, 'items': tiles}
    audit.write_text(json.dumps(metadata, ensure_ascii=False, indent=2) + '\n')
    if args.san5_kaodata:
        assert args.san5_kaodata.is_file()
        source_data = args.san5_kaodata.read_bytes()
        assert len(source_data) == 1407360 and hashlib.sha256(source_data).hexdigest() == 'f942631ced0bc891f796aa6f884461f28aae0769312f0eef90b22b7d7c98e2d6'
        exports = args.original_exports or root / 'workplace/asset-overviews/base'
        original_manifest = json.loads((exports / 'manifest.json').read_text())
        original_faces = {}
        for e in original_manifest['files']:
            if e['from'].endswith('.FAC'):
                assert sha(exports / e['path']) == e['output_sha256']
                original_faces[e['container'], e['from']] = e
        for p in people:
            entry = portraits['base', p['portrait']]
            assert original_faces[entry['container'], entry['name']]['source_sha256'] == entry['source_sha256']
        source_image, source_tiles, source_count = render_sheet(root, combined, labels, portraits, exports, source_data)
        source_png = root / 'workplace/asset-overviews/generals-source-overview.png'
        source_json = source_png.with_suffix('.json')
        for path in [source_png, source_json]:
            assert path.parent.is_dir() and path.parent.stat().st_uid == os.getuid()
            assert not path.exists() or path.stat().st_uid == os.getuid()
        source_image.save(source_png, optimize=True)
        source_json.write_text(json.dumps(dict(metadata, image_sha256=sha(source_png), portraits=source_count,
            items=source_tiles, rights='local_only_original_assets', san5_file_sha256=sha(args.san5_kaodata),
            san5_face_bytes=1920, san5_face_count=len(source_data) // 1920, san5_palette=SAN5_PALETTE,
            format_reference=SAN5_FORMAT_REFERENCE, palette_evidence='external_format_reference_not_runtime_oracle'), ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'people': len(combined), 'portraits': portrait_count, 'image_sha256': sha(png), 'markdown_sha256': sha(md), 'source_overview': bool(args.san5_kaodata)}))


if __name__ == '__main__':
    main()
