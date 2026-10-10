#!/usr/bin/env python3
"""Create a clearly identified local redubbing audition using actual gameplay."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile

p = argparse.ArgumentParser(description=__doc__)
for key in ['river', 'gameplay', 'samples', 'music', 'out', 'font']:
    p.add_argument('--' + key, type=Path, required=True)
p.add_argument('--version', required=True)
a = p.parse_args()
sha = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
sample_receipt = json.loads((a.samples / 'receipt.json').read_text())
assert sample_receipt['passed'] and not sample_receipt['production_approved']
for row in sample_receipt['samples']:
    assert sha(a.samples / row['file']) == row['sha256']
for directory in [a.river, a.gameplay]:
    receipt = json.loads((directory / 'receipt.json').read_text())
    assert receipt['passed'] and receipt['version'] == a.version
    for clip in receipt['clips']:
        assert sha(directory / clip['file']) == clip['sha256']
assert not a.out.exists() and a.out.parent.stat().st_uid == os.getuid()
a.out.mkdir()
final = a.out / f'san1-{a.version}-hd-redub-audition-local.mp4'
def run(command):
    return subprocess.run(command, check=True, capture_output=True, text=True, timeout=180)
def ff(*args):
    return run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', *map(str,args)])

with tempfile.TemporaryDirectory(prefix='san1-redub-montage-') as temp:
    stage = Path(temp)
    parts = []
    sequence = []
    clock = 0.0
    for i, (source, start, length, caption) in enumerate([
        (a.river / 'river-opening.mp4', 29, 5, ''),
        (a.river / 'river-opening.mp4', 49, 7.5, '卷首詞重新配音試聽'),
        (a.gameplay / 'war-advice.mp4', 13, 3.5, '人物對白重新配音試聽')]):
        text = stage / f'caption-{i}.txt'
        text.write_text(caption)
        part = stage / f'part-{i}.mp4'
        filters = ('crop=1280:816:0:0,scale=1920:1200:force_original_aspect_ratio=decrease,'
                   'pad=1920:1200:(ow-iw)/2:(oh-ih)/2:black,setsar=1,fps=30,'
                   f'drawtext=fontfile={a.font}:textfile={text}:fontsize=34:'
                   'fontcolor=0xf4e8c8:x=(w-text_w)/2:y=1134,format=yuv420p')
        ff('-i', source, '-ss', start, '-t', length, '-vf', filters, '-an',
           '-c:v', 'libx264', '-preset', 'veryfast', '-crf', '20', '-threads', '2', part)
        sequence.append({'source': source.name, 'source_sha256': sha(source),
                         'start': start, 'begin': clock, 'duration': length})
        clock += length
        parts.append(part)
    concat = stage / 'concat.txt'
    concat.write_text(''.join(f"file '{part}'\nduration {row['duration']:.9f}\n"
                              for part, row in zip(parts, sequence)))
    filters = (f'[1:a]atrim=duration={clock},volume=0.5,afade=t=in:d=0.5,'
               f'afade=t=out:st={clock-1}:d=1[bg];'
               '[2:a]aresample=48000,adelay=5300:all=1,apad=whole_dur=16,atrim=duration=16[n];'
               '[3:a]aresample=48000,adelay=12800:all=1,apad=whole_dur=16,atrim=duration=16[d];'
               '[n][d]amix=inputs=2:normalize=0,asplit=2[voice][control];'
               '[bg][control]sidechaincompress=threshold=0.012:ratio=12:attack=5:release=350[duck];'
               '[duck][voice]amix=inputs=2:normalize=0,alimiter=limit=0.9[a]')
    ff('-f', 'concat', '-safe', '0', '-i', concat, '-i', a.music,
       '-i', a.samples / 'poem-narration.wav', '-i', a.samples / 'adviser-a.wav',
       '-filter_complex', filters, '-map', '0:v', '-map', '[a]', '-c:v', 'copy',
       '-c:a', 'aac', '-b:a', '192k', '-t', clock, '-movflags', '+faststart', final)
probe = json.loads(run(['ffprobe', '-v', 'error', '-show_format', '-show_streams', '-of', 'json', str(final)]).stdout)
assert abs(float(probe['format']['duration']) - 16) < .1
video = next(row for row in probe['streams'] if row['codec_type'] == 'video')
audio = next(row for row in probe['streams'] if row['codec_type'] == 'audio')
assert (video['width'], video['height'], video['avg_frame_rate'], int(video['nb_frames'])) == (1920,1200,'30/1',480)
assert audio['codec_name'] == 'aac' and audio['channels'] == 2
run(['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-i', str(final), '-f', 'null', '-'])
volume = run(['ffmpeg', '-nostdin', '-hide_banner', '-i', str(final), '-vn', '-af', 'volumedetect', '-f', 'null', '-'])
(a.out / 'volume.log').write_text(volume.stderr)
mean = float(re.search(r'mean_volume: (-?[\d.]+) dB', volume.stderr)[1])
peak = float(re.search(r'max_volume: (-?[\d.]+) dB', volume.stderr)[1])
assert -50 < mean < -5 and -30 < peak < -.1
ff('-i', final, '-vf', 'fps=1/2,scale=480:300,tile=4x2', '-frames:v', '1', '-threads', '2', a.out / 'contact-sheet.jpg')
result = {'technical_passed': True, 'human_listening': 'pending user audition',
          'production_approved': False, 'synthetic_ai_voice': True,
          'scope': '16-second video audition; poem narration and adviser sentence only; no game runtime change',
          'file': final.name, 'bytes': final.stat().st_size, 'sha256': sha(final),
          'duration_seconds': float(probe['format']['duration']), 'sequence': sequence,
          'video_frames': 480, 'mean_volume_db': mean, 'peak_volume_db': peak,
          'voice_model': sample_receipt['model'], 'voice_model_revision': sample_receipt['model_revision'],
          'voice_receipt_sha256': sha(a.samples / 'receipt.json'), 'music_sha256': sha(a.music),
          'rights': 'local_only_original_art_and_music_with_ai_auditions'}
(a.out / 'QA.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(result, ensure_ascii=False))
