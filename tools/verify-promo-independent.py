#!/usr/bin/env python3
"""在 Docker 從正式封包與影片重讀來源、全部影格及影音解碼。"""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tarfile

version = sys.argv[1]
assert re.fullmatch(r'v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}', version)
root = Path('/src/dist-all') / version
promo = root / 'promo'
video = promo / f'san1-{version}-promo-local.mp4'
qa = json.loads((promo / 'QA.json').read_text())
capture = json.loads((promo / 'capture-receipt.json').read_text())
sha = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
assert sha(video) == qa['video_sha256'] and capture['passed']
assert capture['version'] == version

with tarfile.open(root / f'full-local/san1-{version}-linux-amd64.tar.gz') as archive:
    top = f'san1-{version}-linux-amd64'
    assert hashlib.sha256(archive.extractfile(top + '/san1').read()).hexdigest() == capture['binary_sha256']
    assert hashlib.sha256(archive.extractfile(top + '/hd-assets/manifest.json').read()).hexdigest() == capture['manifest_sha256']

source = Path('/src/workplace/promo-source') / version
for item in capture['captures'] + capture['clips']:
    assert sha(source / item['file']) == item['sha256']

probe = json.loads(subprocess.check_output([
    'ffprobe', '-v', 'error', '-count_frames', '-show_streams', '-show_format', '-of', 'json', str(video)
], text=True))
vs = next(s for s in probe['streams'] if s['codec_type'] == 'video')
assert vs['avg_frame_rate'] == '30/1' and int(vs['nb_read_frames']) == int(vs['nb_frames'])
result = subprocess.run([
    'ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-threads', '2', '-i', str(video), '-f', 'null', '-'
], capture_output=True, text=True, timeout=120)
assert result.returncode == 0 and not result.stderr

for language, at in [('en', 15.1), ('ja', 18.1)]:
    out = promo / f'frame-language-{language}.png'
    assert not out.exists()
    subprocess.run([
        'ffmpeg', '-nostdin', '-v', 'error', '-y', '-ss', str(at), '-i', str(video),
        '-frames:v', '1', '-threads', '1', str(out)
    ], check=True, capture_output=True, timeout=20)

proof = {
    'passed': True, 'source_binary_and_hd_manifest_match': True,
    'source_capture_files': len(capture['captures']), 'source_clips': len(capture['clips']),
    'decoded_video_frames': int(vs['nb_read_frames']),
    'video_duration_seconds': float(vs['duration']),
    'container_duration_seconds': float(probe['format']['duration']),
    'whole_audio_video_decode': True, 'video_sha256': sha(video)
}
out = Path('/src/workplace/v66-promo-independent.json')
assert not out.exists() and out.parent.stat().st_uid == os.getuid()
out.write_text(json.dumps(proof, indent=2) + '\n')
print(json.dumps(proof))
