#!/usr/bin/env python3
"""Download the pinned public model for local, original-voice redubbing prototypes."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import urllib.request

MODEL = 'Qwen/Qwen3-TTS-12Hz-1.7B-CustomVoice'
REVISION = '0c0e3051f131929182e2c023b9537f8b1c68adfe'
p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--out', type=Path, required=True)
a = p.parse_args()
assert not a.out.exists() and a.out.parent.stat().st_uid == os.getuid()
a.out.mkdir()
api = f'https://huggingface.co/api/models/{MODEL}/revision/{REVISION}?blobs=true'
with urllib.request.urlopen(api, timeout=45) as response:
    metadata = json.load(response)
assert metadata['sha'] == REVISION
result = {'model': MODEL, 'revision': REVISION, 'license': metadata['cardData']['license'],
          'purpose': 'local redubbing samples; no original actor cloning', 'files': [], 'passed': False}
try:
    for row in metadata['siblings']:
        name = row['rfilename']
        if name == '.gitattributes':
            continue
        assert '..' not in Path(name).parts and not Path(name).is_absolute()
        target = a.out / name
        target.parent.mkdir(parents=True, exist_ok=True)
        url = f'https://huggingface.co/{MODEL}/resolve/{REVISION}/{name}'
        digest, size = hashlib.sha256(), 0
        with urllib.request.urlopen(url, timeout=90) as response, target.open('xb') as output:
            while block := response.read(8 * 1024 * 1024):
                output.write(block)
                digest.update(block)
                size += len(block)
        expected = row.get('lfs', {}).get('sha256')
        if expected:
            assert digest.hexdigest() == expected and size == row['lfs']['size']
        result['files'].append({'file': name, 'bytes': size, 'sha256': digest.hexdigest(),
                                'url': url, 'lfs_verified': bool(expected)})
        print(json.dumps({'downloaded': name, 'bytes': size}), flush=True)
    result['passed'] = True
finally:
    (a.out / 'SOURCES.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
