#!/usr/bin/env python3
"""限 Docker：保存內建 image_gen 的原圖、提示詞、雜湊及驗證縮圖。"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil

from PIL import Image, ImageDraw


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--batch', required=True)
    ap.add_argument('--generated-root', required=True)
    ap.add_argument('--contact', required=True)
    args = ap.parse_args()
    root = Path('/src/hd-assets')
    mp, pp = root / 'expanded-portraits.json', root / 'expanded-portrait-provenance.json'
    contact = Path(args.contact)
    if contact.exists() or not contact.resolve().is_relative_to(Path('/src/workplace/audio').resolve()):
        raise ValueError('驗證縮圖須寫入 workplace/audio/ 的新檔案')
    for p in [root, mp, pp, contact.parent]:
        if (p.stat().st_uid, p.stat().st_gid) != (os.getuid(), os.getgid()):
            raise ValueError(f'擁有權不符：{p}')
    m, p = json.loads(mp.read_text()), json.loads(pp.read_text())
    jobs = json.loads(Path(args.batch).read_text())
    known = {e['portrait'] for e in m['entries']}
    staged = []
    for j in jobs:
        n = j['portrait']
        if type(n) is not int or not 0 <= n < 1000 or n in known or not j['prompt']:
            raise ValueError('重複或不合法的肖像識別')
        known.add(n)
        original = Path(j['path'])
        src = Path(args.generated_root) / original.name
        dst = root / f'X-SAN5-{n:04d}.png'
        if not src.is_file() or src.is_symlink() or dst.exists():
            raise ValueError(f'缺少來源或輸出已存在：{n}')
        with Image.open(src) as im:
            im.verify()
        with Image.open(src) as im:
            im.load()
            w, h = im.size
            if im.mode != 'RGB' or not 0 < w <= 4096 or not 0 < h <= 4096:
                raise ValueError('PNG 色彩或尺寸不符')
            thumb = im.copy()
            thumb.thumbnail((224, 280))
        digest = hashlib.sha256(src.read_bytes()).hexdigest()
        staged.append((j, src, dst, w, h, digest, thumb))
    sheet = Image.new('RGB', (5 * 224, ((len(staged) + 4) // 5) * 304), '#202020')
    draw = ImageDraw.Draw(sheet)
    for i, (j, src, dst, w, h, digest, thumb) in enumerate(staged):
        shutil.copyfile(src, dst)
        if hashlib.sha256(dst.read_bytes()).hexdigest() != digest or (dst.stat().st_uid, dst.stat().st_gid) != (os.getuid(), os.getgid()):
            raise ValueError('複製或擁有權驗證失敗')
        m['entries'].append(dict(portrait=j['portrait'], file=dst.name, width=w, height=h, sha256=digest))
        p['entries'].append(dict(portrait=j['portrait'], name=j['name'], session=Path(j['path']).parent.name,
                                 file=dst.name, sha256=digest, illustrative_design=True,
                                 requests=[dict(generation_id=src.stem, prompt=j['prompt'])]))
        x, y = (i % 5) * 224, (i // 5) * 304
        sheet.paste(thumb, (x, y))
        draw.text((x + 8, y + 284), f"san5:{j['portrait']} / {src.stem[5:13]}", fill='white')
    for path, data in [(mp, m), (pp, p)]:
        data['entries'].sort(key=lambda e: e['portrait'])
        path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n')
    sheet.save(contact)
    receipt = dict(imported=len(staged), total=len(m['entries']),
                   manifest_sha256=hashlib.sha256(mp.read_bytes()).hexdigest(),
                   contact_sha256=hashlib.sha256(contact.read_bytes()).hexdigest(),
                   entries=[dict(portrait=j['portrait'], name=j['name'], file=dst.name, sha256=digest)
                            for j, src, dst, w, h, digest, thumb in staged])
    contact.with_suffix('.json').write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({k: v for k, v in receipt.items() if k != 'entries'}, ensure_ascii=False))


if __name__ == '__main__':
    main()
