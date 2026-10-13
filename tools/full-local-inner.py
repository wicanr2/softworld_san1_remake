#!/usr/bin/env python3
"""只在 Docker 內從公開引擎包建立含原版資料的本機私人封包。"""

import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import zipfile


ROOT = Path('/src')
ORIG = ROOT / 'org_game'
SOURCES = {'base': ORIG / '三國演義', 'plus': ORIG / '三國演義1加強版'}
PLATFORMS = ('linux-amd64', 'windows-amd64', 'darwin-amd64', 'darwin-arm64')


def sha(path):
    h = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def owned(path):
    if path.exists():
        st = path.stat()
        if (st.st_uid, st.st_gid) != (os.getuid(), os.getgid()):
            raise RuntimeError(f'輸出路徑擁有權不符：{path}')


def stage_hd(source, target):
    """沿用已驗收 B 包，附上目前的獨立新增肖像及生成紀錄。"""
    entries = sorted(source.rglob('*'))
    if any(p.is_symlink() or not (p.is_file() or p.is_dir()) for p in entries):
        raise RuntimeError('高清包包含非一般檔案')
    shutil.copytree(source, target)
    expanded = source / 'expanded-portraits.json'
    if not expanded.is_file():
        expanded = ROOT / 'hd-assets/expanded-portraits.json'
    if not expanded.is_file():
        return 0
    m = json.loads(expanded.read_text())
    if m.get('schema') != 1 or len(m.get('entries', [])) > 220:
        raise RuntimeError('新增肖像目錄規格不符')
    seen = set()
    for e in m['entries']:
        n, name = e['portrait'], e['file']
        if type(n) is not int or not 0 <= n < 1000 or n in seen or name != f'X-SAN5-{n:04d}.png':
            raise RuntimeError('新增肖像識別或檔名不符')
        seen.add(n)
        p = expanded.parent / name
        if not p.is_file() or p.is_symlink() or sha(p) != e['sha256']:
            raise RuntimeError(f'新增肖像雜湊不符：{name}')
        shutil.copy2(p, target / name)
    for name in ['expanded-portraits.json', 'expanded-portrait-provenance.json']:
        p = expanded.parent / name
        if not p.is_file() or p.is_symlink():
            raise RuntimeError(f'缺少新增肖像紀錄：{name}')
        shutil.copy2(p, target / name)
    shutil.copy2(ROOT / 'hd-assets/README.md', target / 'README.md')
    return len(seen)


def stage_overviews(target, hd_source):
    """原版及 San5 原圖只進本機包；兩張總覽的名單及位置必須一致。"""
    redraw = Path(os.environ['SAN1_REDRAW_WORK'])
    if not redraw.is_relative_to(ROOT / 'workplace'):
        raise RuntimeError('重繪輸入須在 workplace 內')
    hd = redraw / 'hd-generals-overview.json'
    source = ROOT / 'workplace/asset-overviews/generals-source-overview.json'
    audits = [json.loads(p.read_text()) for p in (hd, source)]
    for audit, path in zip(audits, (hd, source)):
        if (audit.get('schema'), audit.get('count'), audit.get('original_count'), audit.get('expanded_count'), audit.get('portraits')) != (2, 566, 346, 220, 471):
            raise RuntimeError('武將總覽分母不符')
        if sha(path.with_suffix('.png')) != audit['image_sha256']:
            raise RuntimeError('武將總覽圖片雜湊不符')
        if audit['expanded_data_sha256'] != sha(ROOT / 'internal/game/expanded_roster.json'):
            raise RuntimeError('武將總覽使用過期名單')
    if [(p['key'], p['name'], p['tile']) for p in audits[0]['items']] != [(p['key'], p['name'], p['tile']) for p in audits[1]['items']]:
        raise RuntimeError('兩張總覽的人物或順序不符')
    if audits[1].get('rights') != 'local_only_original_assets':
        raise RuntimeError('原圖總覽缺少本機分類')
    if audits[0]['expanded_manifest_sha256'] != sha(hd_source / 'expanded-portraits.json'):
        raise RuntimeError('HD 總覽與實際肖像目錄不符')
    files = {}
    for path in (hd, hd.with_suffix('.png'), source, source.with_suffix('.png')):
        dest = target / 'docs/images' / path.name
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(path, dest)
        files[str(dest.relative_to(target))] = sha(dest)
    for relative in ['docs/hd-generals.md', 'docs/reference/hd-generals.json', 'docs/spec/022-expanded-roster.md', 'docs/re/14-san5-roster.md']:
        dest = target / relative
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / relative, dest)
        files[relative] = sha(dest)
    catalog = target / 'docs/hd-generals.md'
    text = catalog.read_text()
    start = text.index('本頁圖片保留原 v.1.2.0 套件的肖像版本。')
    end = text.index('\n\n', start)
    text = text[:start] + '本包新增 220 人採已確認的 San5 原圖 HD 重繪，原有 346 人肖像保持。' + text[end:]
    catalog.write_text(text)
    files['docs/hd-generals.md'] = sha(catalog)
    readme = target / 'README.md'
    shutil.copy2(ROOT / 'README.md', readme)
    text = readme.read_text()
    begin, end = '<!-- LOCAL_SAN5_PREVIEW_BEGIN -->', '<!-- LOCAL_SAN5_PREVIEW_END -->'
    if text.count(begin) != 1 or text.count(end) != 1:
        raise RuntimeError('README 武將總覽入口不符')
    gui = json.loads((redraw / 'gui-verification.json').read_text())
    if not gui['passed'] or gui['expanded_manifest_sha256'] != sha(hd_source / 'expanded-portraits.json'):
        raise RuntimeError('人物卡截圖未通過本版素材驗證')
    shots = ROOT / gui['raw_receipt']
    if sha(shots) != gui['raw_receipt_sha256']:
        raise RuntimeError('人物卡來源收據不符')
    captures = {p['file']: p for p in gui['captures']}
    for name in ['base-001-person.png', 'base-001-hd-person.png']:
        source_shot = shots.parent / name
        if sha(source_shot) != captures[name]['sha256']:
            raise RuntimeError('人物卡截圖雜湊不符')
        dest = target / 'docs/images' / name
        shutil.copy2(source_shot, dest)
        files[str(dest.relative_to(target))] = sha(dest)
    preview = ('| 張角人物卡，原貌 | 張角人物卡，San5 原圖 HD 重繪 |\n|---|---|\n'
        '| <a href="docs/images/base-001-person.png"><img src="docs/images/base-001-person.png" width="420" alt="張角人物卡原貌"></a> '
        '| <a href="docs/images/base-001-hd-person.png"><img src="docs/images/base-001-hd-person.png" width="420" alt="張角人物卡高清重繪"></a> |\n\n'
        '| 原版＋San5 原圖 | HD 肖像 |\n|---|---|\n'
        '| <a href="docs/images/generals-source-overview.png"><img src="docs/images/generals-source-overview.png" width="420" alt="566 位人物的原版與 San5 原圖對照總覽"></a> '
        '| <a href="docs/images/hd-generals-overview.png"><img src="docs/images/hd-generals-overview.png" width="420" alt="相同順序的 566 位 HD 武將總覽"></a> |')
    text = text[:text.index(begin)] + preview + text[text.index(end) + len(end):]
    for relative in set(re.findall(r'<img\s+[^>]*src="([^"]+)"', text)):
        if not relative.startswith('docs/images/') or '..' in Path(relative).parts:
            raise RuntimeError(f'README 圖片路徑不符：{relative}')
        dest = target / relative
        if not dest.exists():
            shutil.copy2(ROOT / relative, dest)
        files[relative] = sha(dest)
    readme.write_text(text)
    files['README.md'] = sha(readme)
    return files


def safe_extract(archive, target, top):
    if archive.suffix == '.zip':
        with zipfile.ZipFile(archive) as z:
            bad = z.testzip()
            if bad:
                raise RuntimeError(f'ZIP CRC 失敗：{bad}')
            for entry in z.infolist():
                parts = Path(entry.filename).parts
                if not parts or parts[0] != top or '..' in parts:
                    raise RuntimeError(f'不安全的 ZIP 成員：{entry.filename}')
            z.extractall(target)
    else:
        with tarfile.open(archive, 'r:gz') as t:
            for entry in t.getmembers():
                parts = Path(entry.name).parts
                if not parts or parts[0] != top or '..' in parts or not (entry.isfile() or entry.isdir()):
                    raise RuntimeError(f'不安全的 TAR 成員：{entry.name}')
            t.extractall(target)


def main():
    if len(sys.argv) != 2 or not re.fullmatch(r'v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}', sys.argv[1]):
        raise SystemExit('版號格式錯誤')
    version = sys.argv[1]
    release = ROOT / 'dist-all' / version
    dest = release / 'full-local'
    temporary_stage = tempfile.TemporaryDirectory(prefix='san1-full-local-')
    stage = Path(temporary_stage.name)
    manifest_path = release / 'SHA256SUMS.json'
    for directory in (ROOT / 'workplace', stage, release):
        owned(directory)
    if dest.exists():
        raise RuntimeError(f'拒絕覆寫已建立的本機完整版：{dest}')
    if not manifest_path.is_file():
        raise RuntimeError('缺少公開引擎包清單')
    manifest = json.loads(manifest_path.read_text(encoding='utf-8'))
    if manifest['version'] != version:
        raise RuntimeError('引擎包版號不符')
    for item in manifest['packages']:
        package = release / item['file']
        if not package.is_file() or sha(package) != item['sha256']:
            raise RuntimeError(f'引擎包雜湊不符：{package}')
    originals = {}
    for edition, source in SOURCES.items():
        if not source.is_dir():
            raise RuntimeError(f'缺少原版目錄：{source}')
        entries = sorted(source.rglob('*'))
        files = [p for p in entries if p.is_file()]
        if not files or any(p.is_symlink() or not (p.is_file() or p.is_dir()) for p in entries):
            raise RuntimeError(f'原版目錄包含非一般檔案：{source}')
        originals[edition] = {str(p.relative_to(source)): sha(p) for p in files}
    hd_source = stage / 'hd-assets'
    expanded_count = stage_hd(Path('/hdpack'), hd_source)
    subprocess.run(['go', 'run', 'tools/hd-pack-check.go', str(hd_source), str(ORIG)], cwd=ROOT, check=True)
    hd_entries = sorted(hd_source.rglob('*'))
    if any(p.is_symlink() or not (p.is_file() or p.is_dir()) for p in hd_entries):
        raise RuntimeError('高清包包含非一般檔案')
    hd_files = {p.relative_to(hd_source).as_posix(): sha(p) for p in hd_entries if p.is_file()}
    hd_manifest = json.loads((hd_source / 'manifest.json').read_text())
    if len(hd_manifest['entries']) != 904 or not hd_files:
        raise RuntimeError('高清包分母不符')
    (stage / 'packages').mkdir(parents=True)
    (stage / 'output').mkdir()
    epoch = int(manifest['source_date_epoch'])
    overview_files = {}
    for platform in PLATFORMS:
        top = f'san1-{version}-{platform}'
        suffix = '.zip' if platform.startswith('windows-') else '.tar.gz'
        patch = release / 'patch' / (top + suffix)
        if not patch.is_file():
            raise RuntimeError(f'缺少引擎包：{patch}')
        safe_extract(patch, stage / 'packages', top)
        package_dir = stage / 'packages' / top
        if not package_dir.is_dir():
            raise RuntimeError(f'引擎包根目錄不存在：{top}')
        for edition, source in SOURCES.items():
            shutil.copytree(source, package_dir / 'game' / edition)
        shutil.copytree(hd_source, package_dir / 'hd-assets')
        overview_files[platform] = stage_overviews(package_dir, hd_source)
        explanation = (
            f'三國演義 remake {version}\n此封包含原版與加強版遊戲資料，僅供本機保存，禁止公開上傳或再散布。\n'
            'Linux／macOS：執行 ./play-base.sh 或 ./play-plus.sh；Windows：執行 play-base.cmd 或 play-plus.cmd。\n'
            '每次預設原貌。按 Esc 或把滑鼠移到視窗上緣，從選項列切換 B 高清、語言及 AI。\n'
            '左鍵點選選單、人物、郡與戰場；右鍵返回；金額與兵數使用畫面數字鍵盤，鍵盤操作仍可使用。\n'
            '存檔寫在封包目錄的 saves/，不會改動 game/ 內的原版檔案。\n')
        explanation += 'README.md 並列原版＋San5 與 HD 的 566 人武將總覽；完整圖片及來源索引在 docs/images/。\n'
        (package_dir / '本機完整版說明.txt').write_bytes(
            (b'\xef\xbb\xbf' + explanation.replace('\n', '\r\n').encode('utf-8'))
            if platform.startswith('windows-') else explanation.encode('utf-8'))
        for edition in SOURCES:
            if platform.startswith('windows-'):
                launcher = package_dir / f'play-{edition}.cmd'
                launcher.write_bytes(
                    ('@echo off\r\ncd /d "%~dp0"\r\n'
                     f'san1.exe -root "game\\{edition}" -edition {edition} -saves "saves" -hd-assets "hd-assets" %*\r\n').encode('ascii'))
            else:
                launcher = package_dir / f'play-{edition}.sh'
                launcher.write_text(
                    '#!/bin/sh\ncd "$(dirname "$0")" || exit 1\n'
                    f'exec ./san1 -root ./game/{edition} -edition {edition} -saves ./saves -hd-assets ./hd-assets "$@"\n',
                    encoding='ascii')
                launcher.chmod(0o755)
        output = stage / 'output' / (top + suffix)
        if suffix == '.zip':
            with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as z:
                for path in sorted(package_dir.rglob('*')):
                    if not path.is_file():
                        continue
                    info = zipfile.ZipInfo(str(path.relative_to(stage / 'packages')), (1980, 1, 1, 0, 0, 0))
                    info.compress_type = zipfile.ZIP_DEFLATED
                    info.external_attr = (0o100755 if path.stat().st_mode & 0o111 else 0o100644) << 16
                    z.writestr(info, path.read_bytes(), compress_type=zipfile.ZIP_DEFLATED, compresslevel=9)
        else:
            with output.open('wb') as stream:
                subprocess.run(['bash', '-c',
                                'set -o pipefail; tar -C "$1" --sort=name --mtime="@$2" --owner=0 --group=0 '
                                '--numeric-owner -cf - "$3" | gzip -n -9',
                                'sh', str(stage / 'packages'), str(epoch), top],
                               stdout=stream, check=True)
        # 實際重讀封包中的兩版原版資料，逐檔比對輸入雜湊。
        if suffix == '.zip':
            with zipfile.ZipFile(output) as z:
                if z.testzip():
                    raise RuntimeError(f'ZIP CRC 失敗：{output}')
                if any(not i.flag_bits & 0x800 for i in z.infolist() if not i.filename.isascii()):
                    raise RuntimeError('Windows ZIP 非 ASCII 成員缺少 UTF-8 旗標')
                assert z.read(top + '/本機完整版說明.txt').startswith(b'\xef\xbb\xbf')
                for relative, digest in overview_files[platform].items():
                    assert hashlib.sha256(z.read(f'{top}/{relative}')).hexdigest() == digest
                for relative, digest in hd_files.items():
                    assert hashlib.sha256(z.read(f'{top}/hd-assets/{relative}')).hexdigest() == digest
                for edition, files in originals.items():
                    for relative, digest in files.items():
                        member = f'{top}/game/{edition}/{relative}'
                        if hashlib.sha256(z.read(member)).hexdigest() != digest:
                            raise RuntimeError(f'封包原版資料不符：{member}')
        else:
            with tarfile.open(output, 'r:gz') as t:
                for relative, digest in overview_files[platform].items():
                    assert hashlib.sha256(t.extractfile(f'{top}/{relative}').read()).hexdigest() == digest
                for relative, digest in hd_files.items():
                    assert hashlib.sha256(t.extractfile(f'{top}/hd-assets/{relative}').read()).hexdigest() == digest
                for edition, files in originals.items():
                    for relative, digest in files.items():
                        member = f'{top}/game/{edition}/{relative}'
                        if hashlib.sha256(t.extractfile(member).read()).hexdigest() != digest:
                            raise RuntimeError(f'封包原版資料不符：{member}')
    # 從真正的 Linux 封包解開，再用封包內兩版資料各啟動一次。
    linux_top = f'san1-{version}-linux-amd64'
    smoke = stage / 'smoke'
    smoke.mkdir()
    safe_extract(stage / 'output' / f'{linux_top}.tar.gz', smoke, linux_top)
    linux_dir = smoke / linux_top
    version_out = subprocess.check_output(['xvfb-run', '-a', './san1', '-version'], cwd=linux_dir, text=True).strip()
    if version_out != version:
        raise RuntimeError(f'Linux 完整版程式版本不符：{version_out}')
    for edition in SOURCES:
        result = subprocess.run(['timeout', '-s', 'INT', '8', 'xvfb-run', '-a',
                                 f'./play-{edition}.sh', '-music=false'], cwd=linux_dir,
                                capture_output=True, text=True, timeout=30)
        (smoke / f'full-local-{edition}.log').write_text(result.stdout + result.stderr, encoding='utf-8')
        if result.returncode != 124:
            raise RuntimeError(f'Linux 完整版 {edition} 啟動未持續 8 秒：{result.returncode}')
    dest.mkdir()
    new_items = []
    for path in sorted((stage / 'output').iterdir()):
        shutil.move(str(path), dest / path.name)
        new_items.append({'file': 'full-local/' + path.name, 'bytes': (dest / path.name).stat().st_size,
                          'sha256': sha(dest / path.name), 'rights': 'local_only_original_assets'})
    (dest / 'ORIGINAL-SHA256.json').write_text(
        json.dumps({'version': version, 'original_assets_sha256': originals}, ensure_ascii=False,
                   indent=2, sort_keys=True) + '\n', encoding='utf-8')
    (dest / 'HD-SHA256.json').write_text(json.dumps(
        {'version': version, 'entries': 904, 'per_edition': 452, 'expanded_portraits': expanded_count, 'style': 'b', 'scale': 4,
         'rights': 'local_only_original_derived_art', 'files': hd_files},
        ensure_ascii=False, indent=2, sort_keys=True) + '\n', encoding='utf-8')
    (dest / 'GENERALS-SHA256.json').write_text(json.dumps(
        {'version': version, 'count': 566, 'original_count': 346, 'expanded_count': 220,
         'rights': 'local_only_original_assets', 'platform_files': overview_files},
        ensure_ascii=False, indent=2, sort_keys=True) + '\n', encoding='utf-8')
    smoke_dir = release / 'smoke'
    for edition in SOURCES:
        shutil.copy2(smoke / f'full-local-{edition}.log', smoke_dir)
    manifest['packages'].extend(new_items)
    manifest['full_local_source_commit'] = os.environ.get('SAN1_FULL_LOCAL_SOURCE_COMMIT', '')
    manifest['full_local_tool_sha256'] = {name: sha(ROOT / 'tools' / name) for name in ['full-local.sh', 'full-local-inner.py']}
    manifest['full_local_smoke'] = {'linux-amd64': 'base_and_plus_passed_8s',
                                    'windows-amd64': 'not_run', 'darwin-amd64': 'not_run',
                                    'darwin-arm64': 'not_run'}
    temporary = manifest_path.with_suffix('.tmp')
    temporary.write_text(json.dumps(manifest, ensure_ascii=False, indent=2, sort_keys=True) + '\n',
                         encoding='utf-8')
    temporary.replace(manifest_path)
    temporary_stage.cleanup()
    print(f'本機完整版：{dest}；原版檔案 {sum(map(len, originals.values()))} 筆；四平台封包；Linux 兩版已啟動')


if __name__ == '__main__':
    main()
