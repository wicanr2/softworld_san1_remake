#!/usr/bin/env python3
"""Verify existing private packages and prepare a game-containing AppImage AppDir in Docker."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import struct
import subprocess
import tarfile
import zipfile


def sha(path):
    h = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path('/src'))
    parser.add_argument('--stage', type=Path, required=True)
    parser.add_argument('--version', required=True)
    args = parser.parse_args()
    assert re.fullmatch(r'v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}', args.version)
    root, stage = args.root, args.stage
    assert stage.is_dir() and stage.stat().st_uid == os.getuid()
    release = root / 'dist-all' / args.version
    manifest = json.loads((release / 'SHA256SUMS.json').read_text())
    originals = {edition: {f.relative_to(root / 'org_game' / folder).as_posix(): sha(f)
                          for f in (root / 'org_game' / folder).rglob('*') if f.is_file()}
                 for edition, folder in [('base', '三國演義'), ('plus', '三國演義1加強版')]}
    hd = json.loads((release / 'full-local/HD-SHA256.json').read_text())
    # Existing manifest is the authority for the HD file hashes.
    if 'files' in hd:
        hd = hd['files']
    assert isinstance(hd, dict)
    rows = []
    for platform in ['linux-amd64', 'windows-amd64', 'darwin-amd64', 'darwin-arm64']:
        top = f'san1-{args.version}-{platform}'
        suffix = '.zip' if platform.startswith('windows') else '.tar.gz'
        relative = 'full-local/' + top + suffix
        path = release / relative
        expected = next(x for x in manifest['packages'] if x['file'] == relative)
        assert sha(path) == expected['sha256'] and path.stat().st_size == expected['bytes']
        with (zipfile.ZipFile(path) if suffix == '.zip' else tarfile.open(path, 'r:gz')) as archive:
            if suffix == '.zip':
                assert archive.testzip() is None
                members = {x.filename: x for x in archive.infolist() if not x.is_dir()}
                read = archive.read
                assert all(x.flag_bits & 0x800 for x in members.values() if not x.filename.isascii())
            else:
                members = {x.name: x for x in archive.getmembers() if x.isfile()}
                read = lambda name: archive.extractfile(members[name]).read()
            for name in members:
                assert Path(name).parts[0] == top and '..' not in Path(name).parts
            for edition, files in originals.items():
                for name, digest in files.items():
                    assert hashlib.sha256(read(f'{top}/game/{edition}/{name}')).hexdigest() == digest
            for name, digest in hd.items():
                assert hashlib.sha256(read(f'{top}/hd-assets/{name}')).hexdigest() == digest
            assert f'{top}/LICENSE' in members
            binary = read(f'{top}/san1' + ('.exe' if platform.startswith('windows') else ''))
            assert args.version.encode() in binary
            if platform == 'linux-amd64':
                assert binary[:4] == b'\x7fELF' and struct.unpack_from('<H', binary, 18)[0] == 62
            elif platform.startswith('windows'):
                pe = struct.unpack_from('<I', binary, 0x3c)[0]
                assert binary[pe:pe + 4] == b'PE\0\0' and struct.unpack_from('<H', binary, pe + 4)[0] == 0x8664
            else:
                assert binary[:4] == b'\xcf\xfa\xed\xfe'
                assert struct.unpack_from('<I', binary, 4)[0] == (0x01000007 if platform.endswith('amd64') else 0x0100000c)
            rows.append({'platform': platform, 'file': relative, 'sha256': expected['sha256'],
                         'original_files': sum(len(x) for x in originals.values()), 'hd_files': len(hd),
                         'binary_sha256': hashlib.sha256(binary).hexdigest(), 'passed': True})
    appdir = stage / 'AppDir'
    assert not appdir.exists()
    archive = release / f'full-local/san1-{args.version}-linux-amd64.tar.gz'
    with tarfile.open(archive, 'r:gz') as t:
        top = f'san1-{args.version}-linux-amd64'
        for member in t.getmembers():
            parts = Path(member.name).parts
            assert parts[0] == top and '..' not in parts and (member.isfile() or member.isdir())
            relative = Path(*parts[1:])
            target = appdir / relative
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
            else:
                target.parent.mkdir(parents=True, exist_ok=True)
                with t.extractfile(member) as source, target.open('wb') as dest:
                    shutil.copyfileobj(source, dest)
                target.chmod(member.mode & 0o777)
    licenses = appdir / 'THIRD-PARTY-LICENSES'
    shutil.copytree(stage / 'runtime-licenses', licenses)
    library_dir = appdir / 'usr/lib'
    library_dir.mkdir(parents=True)
    dependencies = subprocess.run(['ldd', str(appdir / 'san1')], check=True, text=True, capture_output=True).stdout
    bundled = []
    for name, target in re.findall(r'^\s*(\S+) => (/\S+)', dependencies, re.M):
        if name.startswith(('libc.so', 'libm.so', 'libdl.so', 'libpthread.so', 'librt.so', 'ld-linux')):
            continue
        shutil.copyfile(target, library_dir / name)
        owner = subprocess.run(['dpkg-query', '-S', target], text=True, capture_output=True)
        if owner.returncode:
            owner = subprocess.run(['dpkg-query', '-S', str(Path(target).resolve())], text=True, capture_output=True)
        assert owner.returncode == 0, target
        package = owner.stdout.split(': ')[0].split(':')[0]
        copyright_file = Path('/usr/share/doc') / package / 'copyright'
        assert copyright_file.is_file(), package
        shutil.copyfile(copyright_file, licenses / (package + '-copyright.txt'))
        bundled.append({'soname': name, 'package': package, 'sha256': sha(library_dir / name)})
    (appdir / 'AppRun').write_text('''#!/bin/sh
set -eu
appdir=${APPDIR:-$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)}
edition=${SAN1_EDITION:-base}
previous=
for argument in "$@"; do
    if [ "$previous" = edition ]; then edition=$argument; previous=; continue; fi
    case "$argument" in
        -edition|--edition) previous=edition ;;
        -edition=*|--edition=*) edition=${argument#*=} ;;
    esac
done
case "$edition" in base|plus) ;; *) echo 'edition must be base or plus' >&2; exit 2 ;; esac
data_home=${XDG_DATA_HOME:-${HOME}/.local/share}/softworld-san1
mkdir -p "$data_home/saves-$edition"
export LD_LIBRARY_PATH="$appdir/usr/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
cd "$appdir"
exec "$appdir/san1" -root "$appdir/game/$edition" -edition "$edition" -saves "$data_home/saves-$edition" -font "$appdir/fonts/unifont.hex.gz" -hd-assets "$appdir/hd-assets" "$@"
''')
    (appdir / 'AppRun').chmod(0o755)
    (appdir / 'san1.desktop').write_text('''[Desktop Entry]
Type=Application
Name=三國演義 remake
Exec=AppRun
Icon=san1
Categories=Game;StrategyGame;
Terminal=false
''')
    from PIL import Image, ImageDraw
    icon = Image.new('RGBA', (256, 256), '#163337')
    d = ImageDraw.Draw(icon)
    d.rectangle((15, 15, 240, 240), outline='#c5ac6d', width=8)
    d.line((64, 88, 192, 88), fill='#e8d6a0', width=12)
    d.line((78, 130, 178, 130), fill='#e8d6a0', width=12)
    d.line((54, 174, 202, 174), fill='#e8d6a0', width=12)
    icon.save(appdir / 'san1.png')
    shutil.copyfile(appdir / 'san1.png', appdir / '.DirIcon')
    (appdir / 'AppImage使用說明.txt').write_text(
        f'三國演義 remake {args.version}\n含原版、加強版及HD素材，僅供本機保存。\n'
        'chmod +x *.AppImage 後執行；預設原版，加強版使用 -edition plus。\n'
        '每次預設原貌，按Esc開啟選項列切換高清。\n'
        '存檔在 ${XDG_DATA_HOME:-$HOME/.local/share}/softworld-san1/saves-base 或 saves-plus。\n'
        '未提供FUSE的環境可用 APPIMAGE_EXTRACT_AND_RUN=1 執行。\n'
        '需要x86_64 Linux、glibc及可用的OpenGL顯示環境。\n')
    (stage / 'package-verification.json').write_text(json.dumps(
        {'version': args.version, 'passed': True, 'packages': rows,
         'appdir_binary_sha256': sha(appdir / 'san1'), 'bundled_libraries': bundled,
         'source_commit': manifest['source_commit'], 'rights': 'local_only_original_assets'},
        ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'verified_platform_packages': len(rows), 'original_files_per_package': rows[0]['original_files'],
                      'hd_files_per_package': rows[0]['hd_files'], 'appdir_ready': True}), flush=True)


if __name__ == '__main__':
    main()
