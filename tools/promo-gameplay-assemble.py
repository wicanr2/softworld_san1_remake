#!/usr/bin/env python3
"""Assemble genuine gameplay/HD-switch footage with verified original DOS music in Docker."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
from PIL import Image, ImageDraw, ImageFont, ImageEnhance

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--source', type=Path, required=True)
p.add_argument('--audio', type=Path, required=True)
p.add_argument('--promo-out', type=Path, required=True)
p.add_argument('--font', type=Path, required=True)
p.add_argument('--version', required=True)
p.add_argument('--story', action='store_true', help='Story montage with genuine captured dialogue audio')
p.add_argument('--revision', type=int, default=1, help='Preserve earlier editorial drafts')
a = p.parse_args()
assert re.fullmatch(r'v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}', a.version)
capture = json.loads((a.source / 'receipt.json').read_text())
audio = json.loads((a.audio / 'receipt.json').read_text())
assert capture['passed'] and capture['version'] == a.version and audio['passed']
assert audio['method'].startswith('DOSBox-X') and audio['wav']['track'] == '風雲'
sha = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
wav = a.audio / audio['wav']['file']
assert sha(wav) == audio['wav']['sha256']
revision = f'-r{a.revision}' if a.revision > 1 else ''
final = a.promo_out / f'san1-{a.version}-{"story-gameplay" if a.story else "gameplay-hd"}{revision}-local.mp4'
qa = a.promo_out / (('story-gameplay-20261010' if a.story else 'gameplay-hd-20261010') + revision)
assert not final.exists() and not qa.exists() and final.parent.stat().st_uid == os.getuid()
qa.mkdir()


def run(cmd, timeout=300):
    return subprocess.run(cmd, check=True, capture_output=True, text=True, timeout=timeout)


def ff(*cmd):
    return run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y', *map(str, cmd)])


def video_args():
    return ['-c:v', 'libx264', '-preset', 'veryfast', '-crf', '20', '-threads', '2',
            '-profile:v', 'high', '-level:v', '4.2', '-pix_fmt', 'yuv420p',
            '-x264-params', 'keyint=60:min-keyint=60:scenecut=0', '-r', '30', '-an']


def card(background, title, subtitle, target):
    image = Image.open(background).convert('RGB').resize((1920, 1200), Image.Resampling.LANCZOS)
    image = ImageEnhance.Brightness(image).enhance(.35)
    draw = ImageDraw.Draw(image)
    draw.rectangle((76, 90, 1844, 1110), outline='#c5ac6d', width=5)
    for text, size, y in [(title, 88, 445), (subtitle, 42, 580)]:
        font = ImageFont.truetype(str(a.font), size)
        box = draw.textbbox((0, 0), text, font=font)
        assert box[2] - box[0] < 1700
        draw.text(((1920 - box[2] + box[0]) // 2, y), text, font=font, fill='#f4e8c8')
    image.save(target)


sequence = []
with tempfile.TemporaryDirectory(prefix='san1-live-promo-') as tmp:
    stage, parts, voice_parts = Path(tmp), [], []
    for name, title, subtitle, background in [
        ('intro-card', '三國演義', '智冠 1991　原貌 × B 高清　實際遊玩', 'card-switch-hd.png'),
        ('outro-card', '原版・加強版　完整版', 'Linux AppImage　Windows　macOS Intel / Apple Silicon', 'battle-switch-hd.png')]:
        card(a.source / background, title, subtitle, stage / (name + '.png'))
    scenes = [('intro-card', 4, 'title', ''),
              ('opening', None, 'live', '正式遊戲片頭　選項列實際切換原貌與高清'),
              ('new-game-card', None, 'live', '正常開新局與人物檢視　原貌 → B 高清 → 原貌'),
              ('battle-play', None, 'live', '董卓出兵・戰場紮寨　實際遊玩與 HD 切換'),
              ('outro-card', 4, 'title', '')]
    if a.story:
        card(a.source / 'card-switch-hd.png', '三國演義',
             '智冠 1991　重返群雄逐鹿的年代', stage / 'intro-card.png')
        card(a.source / 'battle-switch-hd.png', '東漢末年，群雄並起',
             '執掌一方諸侯，招攬將才，征戰天下', stage / 'story-card.png')
        card(a.source / 'battle-switch-hd.png', '運籌帷幄，逐鹿天下',
             '原貌 × 高清　Linux・Windows・macOS', stage / 'outro-card.png')
        scenes = [('intro-card', 4, 'title', ''),
                  ('story-card', 6, 'title', ''),
                  ('opening', 6, 'live', '滾滾長江東逝水，浪花淘盡英雄'),
                  ('realm-card', None, 'live', '招攬將才，經營一方　原貌與高清實際切換'),
                  ('war-advice', None, 'live', '兵者貴神速　聽將領獻策，揮軍出征'),
                  ('battle-switch', None, 'live', '馳戰沙場　原貌與高清實際切換'),
                  ('battle-march', None, 'live', '列陣交鋒，步步為營'),
                  ('battle-duel', None, 'live', '呂布迎戰陳宮　陣前單挑實錄'),
                  ('outro-card', 4, 'title', '')]
    clock = 0.0
    for name, seconds, kind, caption in scenes:
        part = stage / (name + '.mp4')
        if kind == 'title':
            ff('-loop', '1', '-framerate', '30', '-i', stage / (name + '.png'),
               '-t', seconds, '-vf', 'format=yuv420p,setsar=1', *video_args(), part)
        else:
            shot = json.loads((a.source / 'edit-plan.json').read_text())[name] if a.story else None
            source_name = shot.get('source', name) if shot else name
            info = next(x for x in capture['clips'] if x['file'] == source_name + '.mp4')
            src = a.source / info['file']
            assert sha(src) == info['sha256']
            caption_file = stage / (name + '-caption.txt')
            caption_file.write_text(caption)
            font = ImageFont.truetype(str(a.font), 34)
            assert font.getbbox(caption)[2] < 1800
            filters = (f'scale=1600:1100,pad=1920:1200:160:0:color=0x122426,setsar=1,fps=30,'
                       f'drawtext=fontfile={a.font}:textfile={caption_file}:fontsize=34:'
                       'fontcolor=0xf4e8c8:x=(w-text_w)/2:y=1134,format=yuv420p')
            trim = []
            if a.story:
                # The shot plan is generated after reviewing real source frames.
                trim = ['-ss', shot['start'], '-t', shot['duration']]
            ff('-i', src, *trim, '-vf', filters, *video_args(), part)
        probe = json.loads(run(['ffprobe', '-v', 'error', '-show_streams', '-of', 'json', str(part)]).stdout)
        length = int(probe['streams'][0]['nb_frames']) / 30
        sequence.append({'scene': name, 'begin': clock, 'duration': length, 'kind': kind,
                         'source': None if kind == 'title' else source_name + '.mp4',
                         'intentional_still': kind == 'title'})
        clock += length
        if a.story:
            voice_part = stage / (name + '-voice.wav')
            if kind == 'title':
                ff('-f', 'lavfi', '-i', 'anullsrc=r=48000:cl=stereo',
                   '-t', f'{length:.9f}', '-c:a', 'pcm_s16le', voice_part)
            else:
                first_sample = round(shot['start'] * 48000)
                sample_count = round(length * 48000)
                ff('-i', src, '-vn', '-af',
                   f'aresample=48000,atrim=start_sample={first_sample}:end_sample={first_sample+sample_count},'
                   f'asetpts=N/SR/TB,apad=whole_len={sample_count},atrim=end_sample={sample_count}',
                   '-ar', '48000', '-ac', '2', '-c:a', 'pcm_s16le', voice_part)
            voice_info = json.loads(run(['ffprobe', '-v', 'error', '-show_format', '-of', 'json', str(voice_part)]).stdout)
            assert abs(float(voice_info['format']['duration']) - length) < .001, (name, voice_info)
            voice_parts.append(voice_part)
        parts.append(part)
        print(json.dumps({'encoded': name, 'seconds': length}), flush=True)
    concat = stage / 'concat.txt'
    # MP4 format.duration rounds to milliseconds; explicit frame-derived durations
    # prevent each concat boundary from introducing a fractional-frame timestamp.
    concat.write_text(''.join("file '" + str(part) + "'\nduration " +
                              f"{scene['duration']:.12f}\n"
                              for part, scene in zip(parts, sequence)))
    if a.story:
        voice_concat = stage / 'voice-concat.txt'
        voice_concat.write_text(''.join("file '" + str(part) + "'\n" for part in voice_parts))
        gameplay_audio = qa / 'gameplay-track.wav'
        ff('-f', 'concat', '-safe', '0', '-i', voice_concat, '-c:a', 'copy', gameplay_audio)
        voice_info = json.loads(run(['ffprobe', '-v', 'error', '-show_format', '-of', 'json', str(gameplay_audio)]).stdout)
        assert abs(float(voice_info['format']['duration']) - clock) < .001
    sound = (f'[1:a][2:a]acrossfade=d=1.5:c1=tri:c2=tri,'
             f'afade=t=in:st=0:d=0.5,afade=t=out:st={clock - 3:.3f}:d=3[a]')
    if a.story:
        sound = (f'[1:a][2:a]acrossfade=d=1.5:c1=tri:c2=tri,atrim=0:{clock:.9f},'
                 f'volume=0.55,afade=t=in:st=0:d=1,afade=t=out:st={clock - 3:.3f}:d=3[music];'
                 '[3:a]asplit=2[voice][control];'
                 '[music][control]sidechaincompress=threshold=0.012:ratio=12:attack=5:release=350[duck];'
                 '[duck][voice]amix=inputs=2:duration=longest:normalize=0,alimiter=limit=0.90[a]')
    encoded = stage / final.name
    ff('-f', 'concat', '-safe', '0', '-i', concat, '-i', wav, '-i', wav,
       *(['-i', gameplay_audio] if a.story else []),
       '-filter_complex', sound, '-map', '0:v', '-map', '[a]', '-c:v', 'copy',
       '-c:a', 'aac', '-b:a', '192k', '-t', f'{clock:.9f}', '-movflags', '+faststart', encoded)
    shutil.copyfile(encoded, final)

probe = json.loads(run(['ffprobe', '-v', 'error', '-show_format', '-show_streams', '-of', 'json', str(final)]).stdout)
video = next(x for x in probe['streams'] if x['codec_type'] == 'video')
sound = next(x for x in probe['streams'] if x['codec_type'] == 'audio')
assert (video['width'], video['height'], video['avg_frame_rate']) == (1920, 1200, '30/1')
assert sound['codec_name'] == 'aac' and sound['channels'] == 2
assert abs(float(probe['format']['duration']) - clock) < .2
run(['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-threads', '2', '-i', str(final), '-f', 'null', '-'], timeout=180)
for name, filters in [('volume', ['-vn', '-af', 'volumedetect']),
                      ('blackdetect', ['-an', '-vf', 'blackdetect=d=0.5:pix_th=0.08']),
                      ('silencedetect', ['-vn', '-af', 'silencedetect=n=-60dB:d=3']),
                      ('freezedetect', ['-an', '-vf', 'freezedetect=n=-60dB:d=2'])]:
    check = run(['ffmpeg', '-nostdin', '-hide_banner', '-threads', '2', '-i', str(final),
                 *filters, '-f', 'null', '-'], timeout=180)
    (qa / (name + '.log')).write_text(check.stderr)
volume = (qa / 'volume.log').read_text()
mean = float(re.search(r'mean_volume: (-?[\d.]+) dB', volume)[1])
peak = float(re.search(r'max_volume: (-?[\d.]+) dB', volume)[1])
assert -50 < mean < -5 and -30 < peak < -.1
assert 'black_start:' not in (qa / 'blackdetect.log').read_text()
assert 'silence_start:' not in (qa / 'silencedetect.log').read_text()
for i, scene in enumerate(sequence):
    ff('-ss', scene['begin'] + min(scene['duration'] / 2, 3), '-i', final,
       '-frames:v', '1', '-threads', '1', qa / f'frame-{i}.png')
ff('-i', final, '-vf', f'fps=1/{clock / 12:.6f},scale=480:300,tile=4x3',
   '-frames:v', '1', '-threads', '2', qa / 'contact-sheet.jpg')
(qa / 'ffprobe.json').write_text(json.dumps(probe, indent=2) + '\n')
shutil.copyfile(a.source / 'receipt.json', qa / 'capture-receipt.json')
shutil.copyfile(a.audio / 'receipt.json', qa / 'original-audio-receipt.json')
(qa / 'RIGHTS.txt').write_text('僅供本機保存，含原版與衍生遊戲畫面及原版配樂，禁止公開上傳。\n')
result = {'version': a.version, 'technical_passed': True, 'visual_review': 'pending',
    'file': 'promo/' + final.name, 'sha256': sha(final), 'bytes': final.stat().st_size,
    'duration_seconds': float(probe['format']['duration']), 'mean_volume_db': mean,
    'peak_volume_db': peak, 'sequence': sequence, 'music': {'track': '風雲',
        'source_sha256': audio['wav']['sha256'], 'method': audio['method'],
        'editing': 'two verified original recordings with 1.5-second crossfade; start/end fade'},
    'footage': 'genuine packaged-binary GUI recording; reviewed montage, normal input' if a.story else
               'genuine packaged-binary GUI recording; complete source clips, normal input',
    'freeze_review': 'title cards and genuine player/toolbar pauses; review against event timeline',
    'rights': 'local_only_original_art_and_music'}
if a.story:
    result.update(dialogue_audio='synchronous PulseAudio capture from normal packaged-gameplay; music ducked under voices',
        story_sources=[{'source': 'docs/reference/01-manual-20-mechanics.md', 'pages': '8–14, 16–26, 28–35',
                        'supports': '六個時期、扮演諸侯、內政人事及行軍單挑'},
                       {'source': 'https://www.npm.gov.tw/NewChineseArtDownload.ashx?bid=3994',
                        'supports': '東漢末年群雄崛起的歷史背景；僅改寫背景，不使用館藏圖片'}],
        banned_promotional_copy=['18/18', '還原到', '其餘的電腦諸侯不會做'],
        filming_guide='knowledge-base/sources/claude/retro-cht/game-promo-video-ffmpeg.md',
        editorial_changes=['世界觀字卡與實機混剪', '遊戲木紋與青綠場景配色', '短鏡頭展示人物語音、行軍與單挑'])
    shutil.copyfile(a.source / 'edit-plan.json', qa / 'edit-plan.json')
    result['music']['editing'] = 'same verified original recording repeated with crossfade; start/end fade; sidechain ducking'
(qa / 'QA.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(result, ensure_ascii=False), flush=True)
