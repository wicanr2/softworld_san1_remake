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


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path('/src'))
    args = parser.parse_args()
    root = args.root
    data_path = root / 'docs/reference/hd-generals.json'
    data = json.loads(data_path.read_text())
    people = data['people']
    assert len(people) == 346 and len({p['index'] for p in people}) == 346
    spec = importlib.util.spec_from_file_location('overviews', root / 'tools/render-asset-overviews.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    labels = module.Labels(root / 'fonts/unifont.hex.gz')
    manifest = json.loads((root / 'hd-assets/manifest.json').read_text())
    portraits = {}
    for e in manifest['entries']:
        if e['edition'] == 'base' and e['name'].endswith('.FAC'):
            portraits[int(e['name'][1:4])] = e
    cols, cell_w, cell_h, margin, header = 16, 160, 224, 24, 136
    canvas = Image.new('RGB', (cols * cell_w + margin * 2,
        header + math.ceil(len(people) / cols) * cell_h + margin), '#131a23')
    draw = ImageDraw.Draw(canvas)
    labels.heading(canvas, (margin, 22), '三國演義　HD 武將姓名總覽', scale=3)
    labels.heading(canvas, (margin, 80),
        f"{len(people)} 位人物　{len({p['portrait'] for p in people})} 張共用肖像　姓名與編號採遊戲原始資料　能力與表字見獨立圖鑑", scale=1)
    tiles = []
    cached = {}
    for i, p in enumerate(people):
        entry = portraits[p['portrait']]
        path = root / 'hd-assets' / entry['file']
        if p['portrait'] not in cached:
            assert sha(path) == entry['sha256']
            with Image.open(path) as image:
                image.load()
                assert image.size == (entry['width'], entry['height'])
                cached[p['portrait']] = image.convert('RGBA').resize((128, 160), Image.Resampling.LANCZOS)
        x, y = margin + i % cols * cell_w, header + i // cols * cell_h
        draw.rectangle((x + 2, y + 2, x + cell_w - 2, y + cell_h - 4), fill='#1c2632')
        canvas.paste(cached[p['portrait']], (x + 16, y + 5), cached[p['portrait']])
        labels.heading(canvas, (x + (cell_w - len(p['name']) * 32) // 2, y + 170), p['name'], scale=2)
        draw.text((x + 36, y + 208), f"{p['index']:03} / F{p['portrait']:03}", font=labels.small, fill='#a9bacb')
        tiles.append({'index': p['index'], 'name': p['name'], 'portrait': p['portrait'],
            'file': entry['file'], 'sha256': entry['sha256'], 'tile': [x, y, cell_w, cell_h]})
    png = root / 'docs/images/hd-generals-overview.png'
    md = root / 'docs/hd-generals.md'
    audit = root / 'docs/images/hd-generals-overview.json'
    for path in [png, md, audit]:
        assert path.parent.is_dir() and path.parent.stat().st_uid == os.getuid()
        assert not path.exists() or path.stat().st_uid == os.getuid()
    canvas.save(png, optimize=True)
    lines = ['# HD 武將圖鑑', '',
        '收錄遊戲的 346 位具名人物，使用正式 HD 素材。251 張肖像由不同人物共用；同一張臉不代表同一個人。點頭像可看原圖。', '',
        '姓名、人物槽、肖像與能力值直接取自兩版劇本 001，逐筆相同。未登場人物也列入；這些數值不代表所有年代或遊戲中的即時狀態。體能、戰力、謀略、魅力沿用遊戲用語。', '',
        '表字是外部查考資料，不是原版人物表欄位，也不寫入遊戲。各項附出處；「二手」表示採用百科資料，尚未由史書原文核實。史書、演義或姓名異文另註，未核得或同名難辨者保留「待考」。', '',
        '[武將姓名總覽原圖](images/hd-generals-overview.png) · [完整資料與來源](reference/hd-generals.json) · [HD 素材包](../hd-assets/README.md)', '',
        '| 頭像 | 姓名 | 表字與出處 | 體能 | 戰力 | 謀略 | 魅力 | 兵種 | 人物槽 | 肖像 |',
        '|---|---|---|---:|---:|---:|---:|---|---:|---|']
    for p in people:
        file = portraits[p['portrait']]['file']
        image = f'<a href="../hd-assets/{file}"><img src="../hd-assets/{file}" width="80" alt="{p["name"]} HD 肖像"></a>'
        courtesy = '待考'
        if p.get('courtesy'):
            courtesy = f'[{p["courtesy"]}]({p["courtesy_sources"][0]["url"]})'
            if p['courtesy_sources'][0]['kind'] == 'secondary':
                courtesy += '（二手）'
        if p.get('courtesy_note'):
            courtesy += '<br>' + p['courtesy_note']
        lines.append(f'| {image} | {p["name"]} | {courtesy} | {p["stamina"]} | {p["war"]} | {p["intel"]} | {p["charm"]} | {p["troop"]} | {p["index"]:03} | F{p["portrait"]:03} |')
    md.write_text('\n'.join(lines) + '\n')
    audit.write_text(json.dumps({'schema': 1, 'data_sha256': sha(data_path),
        'image_sha256': sha(png), 'markdown_sha256': sha(md), 'count': len(people),
        'portraits': len(cached), 'width': canvas.width, 'height': canvas.height, 'items': tiles}, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'people': len(people), 'portraits': len(cached), 'image_sha256': sha(png), 'markdown_sha256': sha(md)}))


if __name__ == '__main__':
    main()
