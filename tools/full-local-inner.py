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
    hd_source = Path('/hdpack')
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
        explanation = (
            f'三國演義 remake {version}\n此封包含原版與加強版遊戲資料，僅供本機保存，禁止公開上傳或再散布。\n'
            'Linux／macOS：執行 ./play-base.sh 或 ./play-plus.sh；Windows：執行 play-base.cmd 或 play-plus.cmd。\n'
            '每次預設原貌。按 Esc 或把滑鼠移到視窗上緣，從選項列切換 B 高清、語言及 AI。\n'
            '存檔寫在封包目錄的 saves/，不會改動 game/ 內的原版檔案。\n')
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
                for relative, digest in hd_files.items():
                    assert hashlib.sha256(z.read(f'{top}/hd-assets/{relative}')).hexdigest() == digest
                for edition, files in originals.items():
                    for relative, digest in files.items():
                        member = f'{top}/game/{edition}/{relative}'
                        if hashlib.sha256(z.read(member)).hexdigest() != digest:
                            raise RuntimeError(f'封包原版資料不符：{member}')
        else:
            with tarfile.open(output, 'r:gz') as t:
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
        {'version': version, 'entries': 904, 'per_edition': 452, 'style': 'b', 'scale': 4,
         'rights': 'local_only_original_derived_art', 'files': hd_files},
        ensure_ascii=False, indent=2, sort_keys=True) + '\n', encoding='utf-8')
    smoke_dir = release / 'smoke'
    for edition in SOURCES:
        shutil.copy2(smoke / f'full-local-{edition}.log', smoke_dir)
    manifest['packages'].extend(new_items)
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
