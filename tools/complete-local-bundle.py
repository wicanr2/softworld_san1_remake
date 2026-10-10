#!/usr/bin/env python3
"""Bundle verified full platform packages and genuine gameplay video for local delivery."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import zipfile


def sha(path):
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(block)
    return digest.hexdigest()


def write_member(archive, name, source=None, data=None, executable=False):
    info = zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0))
    info.create_system = 3
    info.external_attr = (0o100755 if executable else 0o100644) << 16
    info.compress_type = zipfile.ZIP_STORED
    if source:
        with source.open('rb') as src, archive.open(info, 'w', force_zip64=True) as dest:
            shutil.copyfileobj(src, dest)
    else:
        archive.writestr(info, data)


p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--root', type=Path, required=True)
p.add_argument('--release-out', type=Path, required=True)
p.add_argument('--stage', type=Path, required=True)
p.add_argument('--version', required=True)
a = p.parse_args()
assert re.fullmatch(r'v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}', a.version)
release = a.release_out
assert release.is_dir() and release.stat().st_uid == os.getuid()
manifest_file = release / 'SHA256SUMS.json'
assert manifest_file.stat().st_uid == os.getuid()
assert (release / 'full-local').stat().st_uid == os.getuid()
assert (release / 'smoke').stat().st_uid == os.getuid()
manifest = json.loads(manifest_file.read_text())
assert manifest['version'] == a.version
verified = json.loads((a.stage / 'package-verification.json').read_text())
appimage = json.loads((release / 'smoke/appimage-smoke.json').read_text())
wine = json.loads((release / 'smoke/windows-wine-smoke.json').read_text())
qa = json.loads((release / 'promo/gameplay-hd-20261010/QA.json').read_text())
assert verified['passed'] and appimage['passed'] and wine['passed']
assert qa['technical_passed'] and qa['visual_review'] == 'passed'
assert all(x['version'] == a.version for x in [verified, appimage, wine, qa])
assert verified['source_commit'] == manifest['source_commit']
inputs = []
for platform in ['windows-amd64', 'darwin-amd64', 'darwin-arm64']:
    row = next(x for x in verified['packages'] if x['platform'] == platform)
    path = release / row['file']
    assert sha(path) == row['sha256']
    inputs.append((path, platform, row['file']))
image_name = f'san1-{a.version}-linux-x86_64.AppImage'
image_source = a.stage / image_name
assert sha(image_source) == appimage['image_sha256']
image = release / 'full-local' / image_name
if image.exists():
    assert sha(image) == appimage['image_sha256']
else:
    shutil.copyfile(image_source, image)
    image.chmod(0o755)
inputs.insert(0, (image, 'linux-x86_64', 'full-local/' + image.name))
movie = release / qa['file']
assert sha(movie) == qa['sha256']
inputs.append((movie, 'promo', qa['file']))
top = f'san1-{a.version}-complete'
members = []
for path, platform, relative in inputs:
    members.append({'file': f'{platform}/{path.name}', 'sha256': sha(path), 'bytes': path.stat().st_size,
                    'release_file': relative, 'rights': 'local_only_original_assets_and_music'})
internal_manifest = {'version': a.version, 'source_commit': manifest['source_commit'],
    'engine_rebuilt': False, 'delivery_date': '2026-10-10', 'files': members,
    'rights': 'local_only_original_assets_and_music', 'signing': 'unsigned',
    'verification': {'four_platform_package_contents': 'passed', 'appimage_base_plus': 'passed_extracted_runtime',
        'windows_base_plus': 'passed_wine_not_native', 'macos_native': 'not_run',
        'promo_technical_and_visual': 'passed', 'promo_human_listening': 'not_run'}}
readme = f'''三國演義 remake {a.version} 完整版

包含原版、加強版、高清素材，以及實際遊玩推廣影片。
此包含原版素材與音樂，僅供本機保存，不得公開上傳。

Linux x86_64
  進入 linux-x86_64/，將 AppImage 設為可執行後啟動：
  chmod +x {image.name}
  ./{image.name}
  加強版：./{image.name} -edition plus
  沒有 FUSE 時，可在上述執行命令前加 APPIMAGE_EXTRACT_AND_RUN=1。
  存檔位置：${{XDG_DATA_HOME:-$HOME/.local/share}}/softworld-san1/saves-base 或 saves-plus。
  需要 glibc 與可用的 OpenGL 顯示環境。

Windows x64
  解壓 windows-amd64/ 的 ZIP，執行 play-base.cmd 或 play-plus.cmd。
  存檔在解壓後遊戲目錄的 saves/。

macOS
  Intel 選 darwin-amd64/；Apple Silicon 選 darwin-arm64/。
  解壓 tar.gz，進入遊戲目錄，執行 ./play-base.sh 或 ./play-plus.sh。
  存檔在解壓後遊戲目錄的 saves/。

操作
  每次啟動預設原貌。按 Esc 或將滑鼠移到上緣開啟選項列。
  在 Theme 選原貌或 B 高清，也可切換繁中、英文與日文。
  Shift+Esc 用於遊戲內返回或取消。

推廣影片
  promo/ 內為 1920×1200、30 fps 的有聲 MP4。
  包含片頭、正常開局、人物卡、董卓出兵與戰場紮寨。
  三個場景均錄下實際選項列操作及原貌／高清切換。
  配樂為 DOSBox-X 執行原版 AA.EXE 實際錄下的〈風雲〉。

驗收
  四平台封包內容與雜湊已核對；Linux AppImage 以解開後的 AppRun 啟動兩版。
  Windows 兩版在 Wine 啟動通過。Windows 與 macOS 原生啟動、簽章、公證尚未驗收。
  影片完整影音解碼、音量、黑幀、長靜音與代表畫面檢查通過，人耳尚未確認。
  SHA256SUMS.json 列出各包與影片的雜湊。
'''
readme_data = b'\xef\xbb\xbf' + readme.replace('\n', '\r\n').encode('utf-8')
license_data = (a.root / 'LICENSE').read_bytes()
manifest_data = (json.dumps(internal_manifest, ensure_ascii=False, indent=2) + '\n').encode()
target = release / 'full-local' / f'san1-{a.version}-complete-all-platforms.zip'
assert not target.exists()
with zipfile.ZipFile(target, 'w', allowZip64=True) as archive:
    for (path, _, _), row in zip(inputs, members):
        write_member(archive, top + '/' + row['file'], source=path, executable=path.suffix == '.AppImage')
    write_member(archive, top + '/使用說明.txt', data=readme_data)
    write_member(archive, top + '/LICENSE', data=license_data)
    write_member(archive, top + '/SHA256SUMS.json', data=manifest_data)
with zipfile.ZipFile(target) as archive:
    assert archive.testzip() is None
    assert len(archive.infolist()) == len(inputs) + 3
    assert archive.read(top + '/LICENSE') == license_data
    assert archive.read(top + '/使用說明.txt') == readme_data
    assert archive.getinfo(top + '/使用說明.txt').flag_bits & 0x800
    for row in members:
        digest = hashlib.sha256()
        with archive.open(top + '/' + row['file']) as src:
            for block in iter(lambda: src.read(1024 * 1024), b''):
                digest.update(block)
        assert digest.hexdigest() == row['sha256']
summary = {'version': a.version, 'passed': True, 'source_commit': manifest['source_commit'],
    'file': str(target.relative_to(release)), 'sha256': sha(target), 'bytes': target.stat().st_size,
    'members': members, 'zip_crc': 'passed', 'member_sha256': 'passed', 'utf8_readme': 'BOM_CRLF_flagged',
    'rights': 'local_only_original_assets_and_music'}
for path, rights in [(image, 'local_only_original_assets'), (target, summary['rights'])]:
    row = {'file': str(path.relative_to(release)), 'bytes': path.stat().st_size, 'sha256': sha(path), 'rights': rights}
    existing = [x for x in manifest['packages'] if x['file'] == row['file']]
    assert not existing or existing == [row]
    if not existing:
        manifest['packages'].append(row)
manifest['promo_gameplay_hd'] = {key: qa[key] for key in ['file', 'sha256', 'bytes', 'duration_seconds', 'rights']}
manifest['complete_delivery'] = internal_manifest | {'bundle': summary['file'], 'bundle_sha256': summary['sha256']}
manifest['full_local_smoke']['windows-amd64'] = 'base_and_plus_passed_wine_8s_not_native'
assert sha(release / manifest['promo']['file']) == manifest['promo']['sha256']
for row in manifest['packages']:
    assert sha(release / row['file']) == row['sha256']
manifest_file.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
(release / 'smoke/complete-delivery.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2) + '\n')
assert all(path.stat().st_uid == os.getuid() for path in [target, image, manifest_file])
print(json.dumps(summary, ensure_ascii=False), flush=True)
