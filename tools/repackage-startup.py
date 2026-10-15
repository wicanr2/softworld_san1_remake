#!/usr/bin/env python3
"""在 Docker 內以核對過的同版號舊包及新程式重建候選包，不直接覆寫交付。"""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import tempfile
import zipfile


def sha(path):
    h = hashlib.sha256()
    with path.open('rb') as f:
        for b in iter(lambda:f.read(1<<20), b''): h.update(b)
    return h.hexdigest()


p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--release', type=Path, required=True)
p.add_argument('--stage', type=Path, required=True)
p.add_argument('--source-commit', required=True)
p.add_argument('--resume-appimage', action='store_true', help='重新核對既有候選包，僅重建 AppImage')
a = p.parse_args()
root = Path('/src')
assert a.release.is_relative_to(root/'dist-all') and a.stage.is_relative_to(root/'workplace')
assert re.fullmatch(r'v\.[0-9]+\.[0-9]+\.[0-9]+-\d{8}', a.release.name)
assert a.stage.stat().st_uid == a.release.stat().st_uid == os.getuid()
version = a.release.name
baseline = a.stage/'previous-packages'
candidate = a.stage/'candidate'
if candidate.exists() and not a.resume_appimage:
    assert candidate.is_dir() and not candidate.is_symlink() and candidate.stat().st_uid == os.getuid()
    shutil.rmtree(candidate)  # 僅清理尚未交付的候選輸出，保留 baseline。
baseline.mkdir(exist_ok=True)
candidate.mkdir(exist_ok=a.resume_appimage)
m = json.loads((a.release/'SHA256SUMS.json').read_text())
assert m['version'] == version
old_manifest = baseline/'SHA256SUMS.json'
if old_manifest.exists(): assert json.loads(old_manifest.read_text()) == m
else: old_manifest.write_text(json.dumps(m, ensure_ascii=False, indent=2)+'\n')
rows, validation = [], []
new_intro = (
    '\n直接啟動：Windows 雙擊 san1.exe；Linux／macOS 執行 san1。\n'
    '預設原版，從程式旁的 game/base、fonts、hd-assets 讀取資料，存檔在旁邊的 saves。\n'
    '加強版使用 -edition plus，或沿用 play-plus 啟動器。\n'
    '載入期間顯示目前階段；必要資料缺失時會顯示錯誤。\n'
    'AppImage 沿用使用者資料目錄存檔。手動指定的參數優先。\n')

def update_notes(data):
    text = data.decode('utf-8-sig')
    text = text.replace('執行時工作目錄下的', '程式所在目錄下的').replace('執行時工作目錄的', '程式旁的')
    text = text.replace('-root /path/to/三國演義 -faction', '-root /path/to/三國演義 -font fonts/unifont.hex.gz -faction')
    return (text+new_intro).encode()

def archive_files(path):
    if path.suffix == '.zip':
        with zipfile.ZipFile(path) as z:
            assert z.testzip() is None
            for info in z.infolist():
                if not info.is_dir(): yield info.filename, z.read(info.filename)
    else:
        with tarfile.open(path, 'r:gz') as t:
            for info in t:
                if info.isfile(): yield info.name, t.extractfile(info).read()

for row in m['packages']:
    relative = Path(row['file'])
    source = a.release/relative
    assert source.stat().st_uid == os.getuid() and source.stat().st_size == row['bytes'] and sha(source) == row['sha256']
    saved = baseline/relative
    saved.parent.mkdir(parents=True, exist_ok=True)
    if saved.exists(): assert sha(saved) == row['sha256']
    else: os.link(source, saved)
    if relative.suffix == '.AppImage':
        # 沿用既有、已驗證的 runtime bytes 與 AppRun。只換新程式及說明。
        offset = int(subprocess.check_output([str(saved), '--appimage-offset'], text=True).strip())
        runtime = a.stage/'runtime-x86_64'
        with saved.open('rb') as f: runtime.write_bytes(f.read(offset))
        assert sha(runtime) == m['appimage']['runtime_sha256']
        with tempfile.TemporaryDirectory(prefix='san1-startup-appimage-') as tmp:
            tmp = Path(tmp)
            subprocess.run([str(saved), '--appimage-extract'], cwd=tmp, check=True, stdout=subprocess.DEVNULL)
            appdir = a.stage/'AppDir'
            if appdir.exists():
                assert a.resume_appimage and appdir.is_dir() and not appdir.is_symlink() and appdir.stat().st_uid == os.getuid()
                shutil.rmtree(appdir)
            shutil.move(str(tmp/'squashfs-root'), appdir)
        old_files = {str(p.relative_to(appdir)): sha(p) for p in appdir.rglob('*') if p.is_file()}
        shutil.copy2(a.stage/'bin/san1-linux-amd64', appdir/'san1')
        for name in ['README.md', '如何開始.txt']:
            if (appdir/name).exists():
                (appdir/name).write_bytes(update_notes((appdir/name).read_bytes()))
        for name in ['san1', 'README.md', '如何開始.txt']:
            if (appdir/name).exists(): os.utime(appdir/name, (m['source_date_epoch'], m['source_date_epoch']))
        assert all(sha(appdir/name) == digest for name,digest in old_files.items() if name not in ['san1','README.md','如何開始.txt'])
        output = candidate/relative
        output.parent.mkdir(parents=True, exist_ok=True)
        if output.exists():
            assert a.resume_appimage and output.stat().st_uid == os.getuid()
            output.unlink()
        with tempfile.TemporaryDirectory(prefix='san1-appimagetool-') as tmp:
            tmp = Path(tmp)
            subprocess.run(['/opt/appimage-tools/appimagetool.AppImage', '--appimage-extract'], cwd=tmp, check=True, stdout=subprocess.DEVNULL)
            tool_env = os.environ|{'ARCH':'x86_64'}
            tool_env.pop('SOURCE_DATE_EPOCH', None)  # appimagetool 自行傳入時間戳，避免 mksquashfs 重複指定。
            subprocess.run([str(tmp/'squashfs-root/AppRun'), '--no-appstream', '--runtime-file', str(runtime),
                            '--mksquashfs-opt', '-processors', '--mksquashfs-opt', '2',
                            str(appdir), str(output)], check=True, env=tool_env)
        output.chmod(0o755)
        rows.append(dict(row, bytes=output.stat().st_size, sha256=sha(output)))
        validation.append({'file': str(relative), 'platform': 'linux-x86_64', 'unchanged_files': len(old_files)-3,
                           'binary_sha256': sha(appdir/'san1'), 'runtime_sha256': sha(runtime), 'passed': True})
        print(str(relative), 'rebuilt', flush=True)
        continue
    platform = next(x for x in ['linux-amd64','windows-amd64','darwin-amd64','darwin-arm64'] if x in relative.name)
    binary = a.stage/'bin'/('san1-'+platform+('.exe' if platform.startswith('windows') else ''))
    assert binary.is_file() and version.encode() in binary.read_bytes()
    top = f'san1-{version}-{platform}'
    changed = {'san1.exe' if platform.startswith('windows') else 'san1': binary.read_bytes()}
    for name in ['README.md', '如何開始.txt']:
        changed[name] = None
    output = candidate/relative
    output.parent.mkdir(parents=True, exist_ok=True)
    if a.resume_appimage:
        expected = {}
        for name, data in archive_files(saved):
            short = Path(*Path(name).parts[1:]).as_posix()
            if short in changed:
                if changed[short] is not None: data = changed[short]
                elif short == '如何開始.txt' and source.suffix == '.zip':
                    data = b'\xef\xbb\xbf'+update_notes(data).decode().replace('\r\n','\n').replace('\n','\r\n').encode()
                else: data = update_notes(data)
            expected[name] = hashlib.sha256(data).hexdigest()
        actual = {name:hashlib.sha256(data).hexdigest() for name,data in archive_files(output)}
        assert actual == expected
        rows.append(dict(row, bytes=output.stat().st_size, sha256=sha(output)))
        validation.append({'file': str(relative), 'platform': platform, 'verified_files': len(actual),
                           'binary_sha256': sha(binary), 'passed': True})
        print(str(relative), 'reverified', len(actual), flush=True)
        continue
    unchanged = {}
    if source.suffix == '.zip':
        with zipfile.ZipFile(saved) as src, zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED, compresslevel=1) as dst:
            assert src.testzip() is None
            for info in src.infolist():
                parts = Path(info.filename).parts
                assert parts[0] == top and '..' not in parts
                if info.is_dir():
                    dst.writestr(info, b'')
                    continue
                name = Path(*parts[1:]).as_posix()
                data = src.read(info.filename)
                if name in changed:
                    if changed[name] is not None: data = changed[name]
                    elif name == 'README.md': data = update_notes(data)
                    else: data = b'\xef\xbb\xbf'+update_notes(data).decode().replace('\r\n','\n').replace('\n','\r\n').encode()
                else: unchanged[name] = hashlib.sha256(data).hexdigest()
                info.compress_type = zipfile.ZIP_DEFLATED
                dst.writestr(info, data)
        with zipfile.ZipFile(output) as verify:
            assert verify.testzip() is None
            for name,digest in unchanged.items(): assert hashlib.sha256(verify.read(top+'/'+name)).hexdigest() == digest
            assert verify.read(top+'/san1.exe') == binary.read_bytes()
    else:
        with tarfile.open(saved, 'r:gz') as src, tarfile.open(output, 'w:gz', compresslevel=1) as dst:
            for info in src:
                parts = Path(info.name).parts
                assert parts[0] == top and '..' not in parts and (info.isfile() or info.isdir())
                name = Path(*parts[1:]).as_posix()
                if info.isdir(): dst.addfile(info); continue
                data = src.extractfile(info).read()
                if name in changed:
                    data = changed[name] if changed[name] is not None else update_notes(data)
                else: unchanged[name] = hashlib.sha256(data).hexdigest()
                info.size = len(data)
                dst.addfile(info, io.BytesIO(data))
        with tarfile.open(output, 'r:gz') as verify:
            for info in verify:
                if not info.isfile(): continue
                name = Path(*Path(info.name).parts[1:]).as_posix()
                data = verify.extractfile(info).read()
                if name in unchanged: assert hashlib.sha256(data).hexdigest() == unchanged[name]
                if name == 'san1': assert data == binary.read_bytes()
    rows.append(dict(row, bytes=output.stat().st_size, sha256=sha(output)))
    validation.append({'file': str(relative), 'platform': platform, 'unchanged_files': len(unchanged),
                       'binary_sha256': sha(binary), 'passed': True})
    print(str(relative), 'verified', len(unchanged), flush=True)
m['source_commit'] = m['full_local_source_commit'] = a.source_commit
m['packages'] = rows
m['startup_repackage'] = {'date': '2026-10-15', 'same_version_authorized': True,
    'previous_source_commit': json.loads(old_manifest.read_text())['source_commit'],
    'tool_sha256': sha(Path(__file__)), 'scope': 'startup defaults and loading window',
    'public_release_modified': False}
for name in ['complete_delivery', 'full_local_smoke', 'native_smoke']:
    m.pop(name, None)
(candidate/'SHA256SUMS.json').write_text(json.dumps(m, ensure_ascii=False, indent=2)+'\n')
(a.stage/'package-verification.json').write_text(json.dumps({'version': version, 'passed': True,
    'source_commit': a.source_commit, 'packages': validation}, ensure_ascii=False, indent=2)+'\n')
