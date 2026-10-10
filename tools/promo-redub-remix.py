#!/usr/bin/env python3
"""Mix the three existing AI redubs into a verified full trailer, preserving its video stream."""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--source-qa', type=Path, required=True)
p.add_argument('--samples', type=Path, required=True)
p.add_argument('--music', type=Path, required=True)
p.add_argument('--promo-out', type=Path, required=True)
p.add_argument('--revision', type=int, required=True)
a = p.parse_args()
sha = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()

def run(command, timeout=180):
    return subprocess.run(command, check=True, capture_output=True, text=True, timeout=timeout)

def ff(*args):
    return run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', *map(str, args)])

def probe(path):
    return json.loads(run(['ffprobe', '-v', 'error', '-show_data_hash', 'sha256',
                          '-show_format', '-show_streams', '-of', 'json', str(path)]).stdout)

source = json.loads(a.source_qa.read_text())
assert source['technical_passed'] and source['visual_review'] == 'passed'
movie = a.promo_out.parent / source['file']
assert movie.is_file() and sha(movie) == source['sha256']
track = a.source_qa.parent / 'gameplay-track.wav'
assert track.is_file()
if 'gameplay_track_sha256' in source:
    assert sha(track) == source['gameplay_track_sha256']
samples = json.loads((a.samples / 'receipt.json').read_text())
assert samples['passed']
for row in samples['samples']:
    assert sha(a.samples / row['file']) == row['sha256']
assert sha(a.music) == source['music']['source_sha256']
original = json.loads((a.source_qa.parent / 'voice-review.json').read_text())
assert original['passed'] and original['mixed_trailer']['sha256'] == source['sha256']
original_cue = original['mixed_trailer']
scenes = {row['scene']: row for row in source['sequence']}
duration = source['duration_seconds']
assert scenes['river-poem']['duration'] > 6.46 and scenes['battle-march']['duration'] > 2.6
plan = [
    {'file': 'poem-narration.wav', 'role': 'poem_narration', 'begin': scenes['river-poem']['begin'] + .3},
    {'file': 'adviser-a.wav', 'role': 'adviser_dialogue_replacement',
     'begin': round(original_cue['start_seconds'], 3)},
    {'file': 'commander-b.wav', 'role': 'battle_voiceover',
     'begin': scenes['battle-march']['begin'] + 1, 'gain': 1.4},
]
for row in plan:
    item = next(s for s in samples['samples'] if s['file'] == row['file'])
    row.update(text=item['text'], duration_seconds=item['duration_seconds'], source_sha256=item['sha256'])
    row.setdefault('gain', 1.0)
    assert row['begin'] + row['duration_seconds'] < duration

version = source['version']
final = a.promo_out / f'san1-{version}-story-gameplay-r{a.revision}-local.mp4'
qa = a.promo_out / f'story-gameplay-20261010-r{a.revision}'
assert not final.exists() and not qa.exists() and a.promo_out.stat().st_uid == os.getuid()
qa.mkdir()
erased = [original_cue['start_seconds'] - .05,
          original_cue['start_seconds'] + original_cue['reference_seconds'] + .05]
voiceover = plan[2]
softened = [voiceover['begin'] - .05,
            voiceover['begin'] + voiceover['duration_seconds'] + .05]
filters = [f"[0:a]volume=0:enable='between(t,{erased[0]:.6f},{erased[1]:.6f})',"
           f"volume=0.25:enable='between(t,{softened[0]:.6f},{softened[1]:.6f})'[original]"]
for i, row in enumerate(plan, 1):
    delay = round(row['begin'] * 1000)
    filters.append(f'[{i}:a]aresample=48000,aformat=channel_layouts=stereo,volume={row["gain"]},'
                   f'adelay={delay}:all=1,apad=whole_dur={duration},atrim=duration={duration}[v{i}]')
filters.append('[original][v1][v2][v3]amix=inputs=4:duration=first:normalize=0[a]')
remixed = qa / 'gameplay-redubbed.wav'
inputs = ['-i', track]
for row in plan:
    inputs += ['-i', a.samples / row['file']]
ff(*inputs, '-filter_complex', ';'.join(filters), '-map', '[a]',
   '-ar', '48000', '-ac', '2', '-c:a', 'pcm_s16le', remixed)
assert abs(float(probe(remixed)['format']['duration']) - duration) < .001
mix = (f'[1:a][2:a]acrossfade=d=1.5:c1=tri:c2=tri,atrim=0:{duration},'
       f'volume=0.55,afade=t=in:st=0:d=1,afade=t=out:st={duration-3}:d=3[music];'
       '[3:a]asplit=2[voice][control];'
       '[music][control]sidechaincompress=threshold=0.012:ratio=12:attack=5:release=350[duck];'
       '[duck][voice]amix=inputs=2:duration=longest:normalize=0,alimiter=limit=0.85:level=false[a]')
ff('-i', movie, '-i', a.music, '-i', a.music, '-i', remixed, '-filter_complex', mix,
   '-map', '0:v:0', '-map', '[a]', '-c:v', 'copy', '-c:a', 'aac', '-b:a', '192k',
   '-t', duration, '-movflags', '+faststart', final)
info, old_info = probe(final), probe(movie)
video = next(row for row in info['streams'] if row['codec_type'] == 'video')
old_video = next(row for row in old_info['streams'] if row['codec_type'] == 'video')
audio = next(row for row in info['streams'] if row['codec_type'] == 'audio')
assert (video['width'], video['height'], video['avg_frame_rate'], int(video['nb_frames'])) == (1920,1200,'30/1',2637)
assert video['extradata_hash'] == old_video['extradata_hash']
assert audio['channels'] == 2 and audio['codec_name'] == 'aac'
assert abs(float(info['format']['duration']) - duration) < .05
def video_hash(path):
    return ff('-i', path, '-map', '0:v:0', '-c:v', 'copy', '-f', 'hash', '-hash', 'sha256', '-').stdout.strip()
old_hash, new_hash = video_hash(movie), video_hash(final)
assert old_hash == new_hash
run(['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-threads', '2', '-i', str(final), '-f', 'null', '-'])
for name, options in [
    ('volume', ['-vn', '-af', 'volumedetect']),
    ('blackdetect', ['-an', '-vf', 'blackdetect=d=0.5:pix_th=0.08']),
    ('silencedetect', ['-vn', '-af', 'silencedetect=n=-60dB:d=3']),
]:
    check = run(['ffmpeg', '-nostdin', '-hide_banner', '-threads', '2', '-i', str(final),
                 *options, '-f', 'null', '-'])
    (qa / (name + '.log')).write_text(check.stderr)
volume = (qa / 'volume.log').read_text()
mean = float(re.search(r'mean_volume: (-?[\d.]+) dB', volume)[1])
peak = float(re.search(r'max_volume: (-?[\d.]+) dB', volume)[1])
assert -50 < mean < -5 and -30 < peak < -.1
assert 'black_start:' not in (qa / 'blackdetect.log').read_text()
assert 'silence_start:' not in (qa / 'silencedetect.log').read_text()
for i, at in enumerate([3, 16, plan[1]['begin'] + .5, plan[2]['begin'] + .5, 72]):
    ff('-ss', at, '-i', final, '-frames:v', '1', '-threads', '1', qa / f'frame-{i}.png')
ff('-i', final, '-vf', f'fps=1/{duration/12:.6f},scale=480:300,tile=4x3',
   '-frames:v', '1', '-threads', '2', qa / 'contact-sheet.jpg')
result = copy.deepcopy(source)
result.update(file='promo/' + final.name, sha256=sha(final), bytes=final.stat().st_size,
              duration_seconds=float(info['format']['duration']), mean_volume_db=mean,
              peak_volume_db=peak, visual_review='passed', technical_passed=True,
              video_frames=int(video['nb_frames']), human_listening=False,
              redubbing_status='three AI redubs authorized for the full trailer; game runtime unchanged',
              dialogue_audio='AI poem narration, adviser replacement and battle voiceover; remaining genuine captured dialogue',
              redubbing={'movie_use_authorized': True, 'full_game_voice_direction_approved': False,
                         'samples': plan, 'removed_original_cue_seconds': erased,
                         'softened_game_effects_seconds': softened,
                         'source_receipt_sha256': sha(a.samples / 'receipt.json')},
              gameplay_track_sha256=sha(remixed), gameplay_track_file=remixed.name,
              source_gameplay_track_sha256=sha(track),
              audio_limiter={'limit': 0.85, 'automatic_output_gain': False},
              source_video_sha256=source['sha256'], video_payload_sha256=new_hash,
              visual_proof='identical compressed H.264 payload and codec configuration; representative output frames checked')
for field in ['voice_alignment_delta_seconds', 'audio_review']:
    result.pop(field, None)
(qa / 'QA.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
(qa / 'ffprobe.json').write_text(json.dumps(info, indent=2) + '\n')
shutil.copyfile(a.source_qa, qa / 'source-qa.json')
for name in ['edit-plan.json', 'river-capture-receipt.json']:
    shutil.copyfile(a.source_qa.parent / name, qa / name)
shutil.copyfile(a.samples / 'receipt.json', qa / 'redub-generation-receipt.json')
(qa / 'RIGHTS.txt').write_text('僅供本機保存。含原版與衍生遊戲畫面、原版配樂及AI重新配音。\n')
print(json.dumps(result, ensure_ascii=False), flush=True)
