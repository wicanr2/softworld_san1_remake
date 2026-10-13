#!/usr/bin/env python3
"""限 Docker：組裝本機 San5 重繪素材與總覽，保留既有版本封包。"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import zipfile

from PIL import Image


ROOT = Path('/src')


def sha(path):
    h = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def owned(path):
    assert not path.is_symlink(), path
    assert (path.stat().st_uid, path.stat().st_gid) == (os.getuid(), os.getgid()), path


def load_tool(name, filename):
    spec = importlib.util.spec_from_file_location(name, ROOT / 'tools' / filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def write_json(path, value):
    owned(path.parent)
    if path.exists():
        owned(path)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def prepare(work, baseline):
    art = work / 'hd-assets'
    state = json.loads((work / 'progress.json').read_text())
    index = json.loads((work / 'source-index.json').read_text())
    roster_path = ROOT / 'internal/game/expanded_roster.json'
    expanded = json.loads(roster_path.read_text())['scenarios'][0]['people']
    expected = {p['portrait']: p['name'] for p in expanded}
    assert len(expected) == 220 and state['completed'] == 220
    assert len(state['entries']) == 220 and state['rights'] == 'local-only San5-based HD redraw'
    assert {p['portrait']: p['name'] for p in state['entries']} == expected
    assert state['source_index_sha256'] == sha(work / 'source-index.json')
    assert {p['portrait']: p['name'] for p in index['entries']} == expected
    manifest = json.loads((art / 'expanded-portraits.json').read_text())
    assert manifest['schema'] == 1 and len(manifest['entries']) == 220
    assert {p['portrait'] for p in manifest['entries']} == set(expected)
    entries = {p['portrait']: p for p in manifest['entries']}
    for e in state['entries']:
        assert e['prompt'] and e['tool'] == 'built-in image_gen'
        assert e['file'] == f"X-SAN5-{e['portrait']:04d}.png"
        path = art / e['file']
        owned(path)
        assert sha(path) == e['sha256'] == entries[e['portrait']]['sha256']
        with Image.open(path) as im:
            im.verify()
        with Image.open(path) as im:
            im.load()
            assert im.mode == 'RGB' and im.size == (e['width'], e['height'])
    old = json.loads((baseline / 'manifest.json').read_text())
    assert len(old['entries']) == 904
    pack = work / 'assembled-hd-assets'
    if not pack.exists():
        shutil.copytree(baseline, pack)
    owned(pack)
    for e in old['entries']:
        assert sha(baseline / e['file']) == sha(pack / e['file']) == e['sha256']
    for path in art.iterdir():
        assert path.is_file() and not path.is_symlink()
        dest = pack / path.name
        if dest.exists():
            owned(dest)
        shutil.copy2(path, dest)
    assert len(list(pack.rglob('*.png'))) == 657
    return state, old, pack


def overview(work, state, old):
    data_path = ROOT / 'docs/reference/hd-generals.json'
    original = json.loads(data_path.read_text())['people']
    expanded_path = ROOT / 'internal/game/expanded_roster.json'
    expanded = json.loads(expanded_path.read_text())['scenarios'][0]['people']
    people = [dict(p, source='base', key=f"base:{p['index']:03}") for p in original]
    people += [dict(p, source='san5', key=f"san5:{p['portrait']}") for p in expanded]
    assert len(original) == 346 and len(people) == 566
    portraits = {('base', int(e['name'][1:4])): e for e in old['entries']
                 if e['edition'] == 'base' and e['name'].endswith('.FAC')}
    portraits.update({('san5', e['portrait']): e for e in state['entries']})
    renderer = load_tool('catalog', 'render-general-catalog.py')
    labels = load_tool('labels', 'render-asset-overviews.py').Labels(ROOT / 'fonts/unifont.hex.gz')
    canvas, tiles, count = renderer.render_sheet(ROOT, people, labels, portraits,
                                                expanded_assets=work / 'hd-assets')
    assert count == 471
    png = work / 'hd-generals-overview.png'
    if png.exists():
        owned(png)
    canvas.save(png, optimize=True)
    audit = dict(schema=2, rights='local-only San5-based HD redraw',
                 count=566, original_count=346, expanded_count=220, portraits=count,
                 width=canvas.width, height=canvas.height, image_sha256=sha(png),
                 data_sha256=sha(data_path), expanded_data_sha256=sha(expanded_path),
                 expanded_manifest_sha256=sha(work / 'hd-assets/expanded-portraits.json'),
                 progress_sha256=sha(work / 'progress.json'), items=tiles)
    old_overview = json.loads((ROOT / 'docs/images/hd-generals-overview.json').read_text())
    assert [(e['key'], e['name'], e['tile']) for e in old_overview['items']] == [
        (e['key'], e['name'], e['tile']) for e in tiles]
    assert old_overview['items'][:346] == tiles[:346]
    write_json(png.with_suffix('.json'), audit)
    return png, audit


def deliver(work, pack, png, version):
    assert version == 'v.1.2.0-20261012', '本次只核准既有版本的本機素材附包'
    dest = ROOT / 'dist-all' / version / 'full-local'
    owned(dest)
    stem = f'san1-{version}-san5-hd-redraw-20261013-r1-local'
    archive = dest / f'{stem}.zip'
    if archive.exists():
        owned(archive)
    text = ('San5 原圖 HD 重繪，本機素材附包\n\n'
            '含新增 220 張重繪及既有 437 張美術，適用原版與加強版。\n'
            '解壓縮後，用遊戲參數 -hd-assets 指向本包的 hd-assets 目錄。\n'
            'Windows 範例：san1.exe -hd-assets "C:\\San1-HD\\hd-assets"\n'
            'Linux／macOS 範例：./san1 -hd-assets "/完整路徑/hd-assets"\n'
            '在遊戲視窗選項列切換高清顯示。\n\n'
            '原有 346 人的肖像保持，新 HD 總覽與原圖總覽皆為 566 人。\n'
            '遊戲規則、能力及存檔格式沿用 v.1.2.0。此包不含執行檔。\n'
            'San5 原圖及衍生重繪僅供本機使用，未取得公開散布權。\n'
            'LICENSE 適用專案程式與文件，不授予 San5 素材散布權。\n')
    readme = work / 'README-san5-redraw.txt'
    if readme.exists():
        owned(readme)
    readme.write_text(text)
    files = {f'hd-assets/{p.relative_to(pack).as_posix()}': p
             for p in pack.rglob('*') if p.is_file()}
    files.update({f'sources/{p.name}': p for p in (work / 'sources').iterdir() if p.is_file()})
    for p in [work / 'source-index.json', work / 'progress.json', readme, ROOT / 'LICENSE', png, png.with_suffix('.json')]:
        files[p.name] = p
    progress_sha = sha(work / 'progress.json')
    expanded_sha = sha(pack / 'expanded-portraits.json')
    for name in ['final-verification.json', 'visual-review.json',
                 'loader-verification.json', 'gui-verification.json']:
        path = work / name
        audit = json.loads(path.read_text())
        assert audit['passed'] is True, name
        if name in ['final-verification.json', 'visual-review.json', 'gui-verification.json']:
            assert audit['progress_sha256'] == progress_sha, name
        if name in ['loader-verification.json', 'gui-verification.json']:
            assert audit['expanded_manifest_sha256'] == expanded_sha, name
        files[f'verification/{name}'] = path
    files['verification/loader-check.txt'] = work / 'loader-check.txt'
    for ext in ['png', 'json']:
        source = dest / f'generals-source-overview.{ext}'
        assert source.is_file()
        files[source.name] = source
    hashes = {name: sha(path) for name, path in sorted(files.items())}
    temporary = work / f'{stem}.zip.tmp'
    if temporary.exists():
        owned(temporary)
        temporary.unlink()
    with zipfile.ZipFile(temporary, 'x', compression=zipfile.ZIP_STORED) as z:
        for name, path in sorted(files.items()):
            assert not path.is_symlink()
            entry = zipfile.ZipInfo(name, date_time=(2026, 10, 13, 0, 0, 0))
            entry.external_attr = 0o100644 << 16
            z.writestr(entry, path.read_bytes())
    with zipfile.ZipFile(temporary) as z:
        assert len(z.namelist()) == len(files) and z.testzip() is None
        for name, digest in hashes.items():
            assert hashlib.sha256(z.read(name)).hexdigest() == digest, name
    if archive.exists():
        assert sha(archive) == sha(temporary), '既有附包內容不同，拒絕覆寫'
        temporary.unlink()
    else:
        temporary.replace(archive)
    for ext in ['png', 'json']:
        target = dest / f'hd-generals-overview-san5-redraw-20261013-r1.{ext}'
        source = png.with_suffix('.' + ext)
        if target.exists():
            owned(target)
            assert sha(target) == sha(source), '既有總覽內容不同，拒絕覆寫'
        else:
            shutil.copy2(source, target)
    receipt = dict(version=version, rights='local-only San5-based HD redraw',
                   file=archive.name, sha256=sha(archive), bytes=archive.stat().st_size,
                   member_count=len(hashes), members=hashes, crc_and_hashes_passed=True,
                   tool_sha256={name: sha(ROOT / 'tools' / name) for name in
                                ['package-san5-redraw.py', 'render-general-catalog.py',
                                 'verify-expanded-roster-inner.py']})
    write_json(dest / f'{stem}-SHA256.json', receipt)
    return {k: receipt[k] for k in ['file', 'sha256', 'bytes', 'member_count']}


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--workdir', type=Path, required=True)
    ap.add_argument('--baseline', type=Path, default=ROOT / 'workplace/hd-assets-mapcursor-v50-r1')
    ap.add_argument('--deliver', action='store_true')
    ap.add_argument('--version', default='v.1.2.0-20261012')
    args = ap.parse_args()
    work = args.workdir.resolve()
    assert work.is_relative_to(ROOT / 'workplace')
    owned(work)
    state, old, pack = prepare(work, args.baseline)
    png, audit = overview(work, state, old)
    result = dict(completed=220, overview_people=566, overview_sha256=sha(png),
                  pack=str(pack), original_entries_unchanged=904)
    if args.deliver:
        result['delivery'] = deliver(work, pack, png, args.version)
    write_json(work / 'assembly.json', result)
    print(json.dumps(result, ensure_ascii=False))


if __name__ == '__main__':
    main()
