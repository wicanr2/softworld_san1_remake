#!/usr/bin/env python3
"""限 Docker：逐批保存已確認的 San5 HD 重繪，不覆寫既有交付。"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil

from PIL import Image, ImageDraw, ImageFont


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def owned(path):
    assert not path.is_symlink(), path
    assert (path.stat().st_uid, path.stat().st_gid) == (os.getuid(), os.getgid()), path


def write_json(path, value):
    if path.exists():
        owned(path)
    tmp = path.with_suffix(path.suffix + '.tmp')
    if tmp.exists():
        owned(tmp)
    tmp.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')
    tmp.replace(path)


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--workdir', type=Path, required=True)
    ap.add_argument('--batch', type=Path)
    ap.add_argument('--generated-root', type=Path, default=Path('/generated'))
    ap.add_argument('--contact', type=Path)
    ap.add_argument('--replace', action='store_true', help='保存前版後，採用已檢查的修訂圖')
    ap.add_argument('--review-portraits', help='對照圖只收錄這些逗號分隔來源編號')
    args = ap.parse_args()
    work = args.workdir.resolve()
    assert work.is_relative_to(Path('/src/workplace').resolve())
    owned(work)
    source_path = work / 'source-index.json'
    sources = {e['portrait']: e for e in json.loads(source_path.read_text())['entries']}
    art = work / 'hd-assets'
    art.mkdir(exist_ok=True)
    owned(art)
    state_path = work / 'progress.json'
    state = json.loads(state_path.read_text()) if state_path.exists() else {
        'schema': 1, 'approved': '2026-10-13', 'rights': 'local-only San5-based HD redraw',
        'source_index_sha256': sha(source_path), 'entries': []}
    assert state['source_index_sha256'] == sha(source_path)
    known = {e['portrait'] for e in state['entries']}
    jobs = json.loads(args.batch.read_text()) if args.batch else []
    if not state['entries']:
        samples = json.loads((work / 'provenance.json').read_text())['entries']
        for entry in samples:
            jobs.insert(0, {'portrait': entry['portrait'], 'name': entry['name'],
                           'local_sample': entry['hd_file'], 'path': entry['built_in_source'],
                           'prompt': entry['prompt']})
    imported = []
    for job in jobs:
        portrait = job['portrait']
        assert type(portrait) is int and portrait in sources
        source = sources[portrait]
        assert job['name'] == source['name'] and job['prompt'].strip()
        original = Path(job['path'])
        assert original.is_absolute() and original.suffix == '.png'
        assert original.is_relative_to('/home/anr2/.codex/generated_images')
        src = work / job['local_sample'] if job.get('local_sample') else args.generated_root / original.relative_to('/home/anr2/.codex/generated_images')
        assert src.is_file() and not src.is_symlink()
        with Image.open(src) as im:
            im.verify()
        with Image.open(src) as im:
            im.load()
            assert im.mode == 'RGB' and 1000 <= im.width <= 4096 and 1250 <= im.height <= 4096
            assert abs(im.width / im.height - 0.8) < 0.03
            width, height = im.size
        digest = sha(src)
        previous = None
        if portrait in known:
            previous = next(e for e in state['entries'] if e['portrait'] == portrait)
            if previous['sha256'] == digest:
                continue
            assert args.replace, portrait
        dst = art / f'X-SAN5-{portrait:04}.png'
        revisions = []
        if previous:
            owned(dst)
            assert sha(dst) == previous['sha256']
            archive = work / 'revisions'
            archive.mkdir(exist_ok=True)
            owned(archive)
            old = archive / f"X-SAN5-{portrait:04}-{previous['sha256'][:12]}.png"
            if old.exists():
                owned(old)
                assert sha(old) == previous['sha256']
            else:
                shutil.copyfile(dst, old)
            revisions = previous.get('revisions', []) + [dict(
                {k: v for k, v in previous.items() if k != 'revisions'},
                archived_file=old.relative_to(work).as_posix())]
            tmp = dst.with_suffix('.new.png')
            assert not tmp.exists()
            shutil.copyfile(src, tmp)
            assert sha(tmp) == digest
            tmp.replace(dst)
            state['entries'] = [e for e in state['entries'] if e['portrait'] != portrait]
        if dst.exists():
            owned(dst)
            assert sha(dst) == digest
        else:
            shutil.copyfile(src, dst)
        owned(dst)
        assert sha(dst) == digest
        entry = {'portrait': portrait, 'name': source['name'], 'source_key': source['key'],
                 'file': dst.name, 'width': width, 'height': height, 'sha256': digest,
                 'session': original.parent.name, 'generation_id': original.stem,
                 'generated_path': str(original), 'prompt': job['prompt'],
                 'reference': source, 'tool': 'built-in image_gen',
                 'visual_basis': 'San5 original portrait, not an independent invented likeness',
                 'classification': state['rights']}
        if revisions:
            entry['revisions'] = revisions
        state['entries'].append(entry)
        state['entries'].sort(key=lambda e: e['portrait'])
        state['completed'] = len(state['entries'])
        write_json(state_path, state)
        known.add(portrait)
        imported.append(entry)
    manifest = {'schema': 1, 'entries': [{k: e[k] for k in ('portrait', 'file', 'width', 'height', 'sha256')} for e in state['entries']]}
    write_json(art / 'expanded-portraits.json', manifest)
    write_json(art / 'expanded-portrait-provenance.json', state)
    review_entries = imported if args.batch else state['entries']
    if args.review_portraits:
        selected = {int(n) for n in args.review_portraits.split(',')}
        review_entries = [e for e in review_entries if e['portrait'] in selected]
        assert {e['portrait'] for e in review_entries} == selected
    if args.contact and review_entries:
        contact = args.contact.resolve()
        assert contact.is_relative_to(work) and not contact.exists()
        cols = min(4, len(review_entries))
        sheet = Image.new('RGB', (cols * 272, ((len(review_entries) + cols - 1) // cols) * 208), '#17212b')
        draw = ImageDraw.Draw(sheet)
        font = ImageFont.truetype('/src/fonts/kai.ttf', 18)
        for i, entry in enumerate(review_entries):
            x, y = (i % cols) * 272, (i // cols) * 208
            draw.text((x + 6, y + 3), entry['name'] + f" #{entry['portrait']}", font=font, fill='#f3e9d5')
            for col, (path, mode) in enumerate([(work / entry['reference']['source']['file'], Image.Resampling.NEAREST),
                                               (art / entry['file'], Image.Resampling.LANCZOS)]):
                with Image.open(path) as im:
                    im.load()
                    sheet.paste(im.resize((128, 160), mode), (x + 4 + col * 136, y + 28))
            draw.text((x + 8, y + 188), 'San5 原圖       HD 重繪', font=font, fill='#b7c8d6')
        sheet.save(contact)
        write_json(contact.with_suffix('.json'), {'portraits': [e['portrait'] for e in review_entries], 'sha256': sha(contact)})
    print(json.dumps({'imported': len(imported), 'completed': len(state['entries']),
                      'remaining': 220 - len(state['entries']), 'manifest_sha256': sha(art / 'expanded-portraits.json')}, ensure_ascii=False))


if __name__ == '__main__':
    main()
