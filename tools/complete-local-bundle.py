#!/usr/bin/env python3
"""Finalize separate platform full packages and a separate local promotion video."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil


def sha(path):
    h = hashlib.sha256()
    with path.open('rb') as source:
        for block in iter(lambda: source.read(1048576), b''):
            h.update(block)
    return h.hexdigest()


p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--root', type=Path, required=True)
p.add_argument('--release-out', type=Path, required=True)
p.add_argument('--stage', type=Path, required=True)
p.add_argument('--version', required=True)
a = p.parse_args()
assert re.fullmatch(r'v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}', a.version)
release = a.release_out
manifest_file = release / 'SHA256SUMS.json'
for path in [release, release / 'full-local', release / 'smoke', manifest_file]:
    assert path.stat().st_uid == os.getuid() and path.stat().st_gid == os.getgid()
manifest = json.loads(manifest_file.read_text())
verified = json.loads((a.stage / 'package-verification.json').read_text())
appimage = json.loads((release / 'smoke/appimage-smoke.json').read_text())
wine = json.loads((release / 'smoke/windows-wine-smoke.json').read_text())
qa_files = list((release / 'promo').glob('gameplay-hd-*/QA.json'))
assert len(qa_files) == 1
qa = json.loads(qa_files[0].read_text())
assert manifest['version'] == a.version
assert all(x['version'] == a.version for x in [verified, appimage, wine, qa])
assert verified['passed'] and appimage['passed'] and wine['passed']
assert qa['technical_passed'] and qa['visual_review'] == 'passed'
assert verified['source_commit'] == manifest['source_commit']
image = release / 'full-local' / f'san1-{a.version}-linux-x86_64.AppImage'
if image.exists():
    assert sha(image) == appimage['image_sha256']
else:
    source = a.stage / image.name
    assert sha(source) == appimage['image_sha256']
    shutil.copyfile(source, image)
    image.chmod(0o755)
image_row = {'file': str(image.relative_to(release)), 'bytes': image.stat().st_size,
             'sha256': sha(image), 'rights': 'local_only_original_assets'}
existing = [x for x in manifest['packages'] if x['file'] == image_row['file']]
assert not existing or existing == [image_row]
if not existing:
    manifest['packages'].append(image_row)
platforms = [{'platform': 'linux-x86_64', **image_row}]
for platform in ['windows-amd64', 'darwin-amd64', 'darwin-arm64']:
    checked = next(x for x in verified['packages'] if x['platform'] == platform)
    row = next(x for x in manifest['packages'] if x['file'] == checked['file'])
    path = release / row['file']
    assert sha(path) == checked['sha256'] == row['sha256']
    assert path.stat().st_size == row['bytes']
    platforms.append({'platform': platform, **row})
movie = release / qa['file']
assert sha(movie) == qa['sha256'] and movie.stat().st_size == qa['bytes']
for row in manifest['packages']:
    assert sha(release / row['file']) == row['sha256']
delivery = {'version': a.version, 'passed': True, 'source_commit': manifest['source_commit'],
    'layout': 'separate_platform_packages', 'platforms': platforms,
    'promo': {key: qa[key] for key in ['file', 'bytes', 'sha256', 'duration_seconds', 'rights']},
    'voice_templates': len(json.loads((a.root / 'internal/speaker/voice_catalog.json').read_text())['cues']),
    'signing': 'unsigned', 'verification': {'package_contents': 'passed',
        'linux_appimage': 'base_plus_extracted_runtime_passed',
        'windows': 'base_plus_wine_passed_not_native', 'macos_native': 'not_run',
        'promo': 'technical_and_visual_passed'}}
assert not list((release / 'full-local').glob('*complete-all-platforms.zip'))
manifest['complete_delivery'] = delivery
manifest['promo_gameplay_hd'] = delivery['promo']
manifest['full_local_smoke']['windows-amd64'] = 'base_and_plus_passed_wine_8s_not_native'
manifest_file.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
receipt = release / 'smoke/platform-delivery.json'
assert not receipt.exists()
receipt.write_text(json.dumps(delivery, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(delivery, ensure_ascii=False), flush=True)
