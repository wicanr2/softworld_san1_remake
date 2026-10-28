#!/usr/bin/env python3
"""核對 GitHub 重建所需的 HD、配音、字型與授權檔；在 Docker 內執行。"""
import argparse
import hashlib
import json
from pathlib import Path

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
p.add_argument('--write-manifest', action='store_true')
a = p.parse_args()
root = a.root.resolve()
manifest = root / 'docs/release/rebuild-assets.json'
paths = sorted(path for directory in ['hd-assets', 'fonts']
               for path in (root / directory).rglob('*') if path.is_file())
actual = []
for path in paths:
    assert not path.is_symlink(), path
    with path.open('rb') as source:
        digest = hashlib.file_digest(source, 'sha256').hexdigest()
    actual.append({'file': str(path.relative_to(root)), 'bytes': path.stat().st_size, 'sha256': digest})
if a.write_manifest:
    assert manifest.parent.is_dir()
    manifest.write_text(json.dumps({'schema': 1, 'files': actual}, ensure_ascii=False, indent=2) + '\n')
expected = json.loads(manifest.read_text())
assert expected['schema'] == 1
assert expected['files'] == actual, '素材檔案、大小或 SHA-256 不符重建清單'
print(json.dumps({'passed': True, 'files': len(actual),
                  'png': sum(row['file'].endswith('.png') for row in actual),
                  'bytes': sum(row['bytes'] for row in actual)}, ensure_ascii=False))
