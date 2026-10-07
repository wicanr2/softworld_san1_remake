#!/usr/bin/env python3
"""正式 GUI 切換錄影與原版 OPL 錄音的本機推廣片；只在 Docker 執行。"""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

version = sys.argv[1]
assert re.fullmatch(r'v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}', version)
root = Path('/src')
release = root/'dist-all'/version
source = root/'workplace/promo-source'/version
final = release/'promo'
assert release.stat().st_uid == os.getuid() and not final.exists()
capture = json.loads((source/'receipt.json').read_text())
original = json.loads(Path('/audio/receipt.json').read_text())
assert capture['passed'] and original['passed']
assert capture['version'] == version
assert original['wav']['track'] == '風雲' and original['method'].startswith('DOSBox-X')
wav = Path('/audio')/original['wav']['file']
sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
assert sha(wav) == original['wav']['sha256']
font = os.environ.get('SAN1_PROMO_FONT', '/promo-font.ttc')
assert Path(font).is_file()

def run(args):
    return subprocess.run(args, check=True, capture_output=True, text=True, timeout=300)

def ff(args):
    return run(['ffmpeg','-nostdin','-hide_banner','-loglevel','error','-y',*args])

def label(text):
    return f"drawtext=fontfile={font}:text='{text}':fontsize=36:fontcolor=0xeef2e2:x=(w-text_w)/2:y=1134"

sequence, duration = [], 0.0
with tempfile.TemporaryDirectory(prefix='san1-promo-encode-') as temp:
    stage, parts = Path(temp), []
    captions = {'title':'智冠 1991《三國演義》　原貌與 B 高清，實際切換',
                'main-card':'人物與操作面板高清化　繁中・English・日本語',
                'battle':'沿用原版戰場規則　即時切換 Theme'}
    for name, caption in captions.items():
        info = next(x for x in capture['clips'] if x['file']==name+'.mp4')
        path = source/info['file']
        assert sha(path)==info['sha256']
        part = stage/(name+'.mp4')
        ff(['-i',str(path),'-vf',f"scale=1600:1100,pad=1920:1200:160:0:color=0x122426,{label(caption)},format=yuv420p",
            '-r','30','-c:v','libx264','-preset','veryfast','-crf','20','-threads','2','-an',str(part)])
        parts.append(part)
        sequence.append({'scene':name,'begin':duration,'duration':info['duration'],'events':info['events'],
                         'intentional_pause':'玩家停點及選項列；操作回應另核對實際截圖'})
        duration += info['duration']
    compare = stage/'compare.mp4'
    fc = f"[0:v]scale=832:530[a];[1:v]crop=1280:816:0:64,scale=832:530[b];color=c=0x122426:s=1920x1200:r=30[bg];[bg][a]overlay=80:320[x];[x][b]overlay=1008:320,drawtext=fontfile={font}:text='同一玩家停點':fontsize=48:fontcolor=0xeef2e2:x=(w-text_w)/2:y=150,drawtext=fontfile={font}:text='原貌':fontsize=40:fontcolor=white:x=440:y=250,drawtext=fontfile={font}:text='B 高清':fontsize=40:fontcolor=white:x=1330:y=250,{label('保留 640×408 邏輯版面　高清素材採 4×')},format=yuv420p[out]"
    ff(['-loop','1','-i',str(source/'main-card-original.png'),'-loop','1','-i',str(source/'main-card-hd-options.png'),
        '-filter_complex',fc,'-map','[out]','-t','6','-r','30','-threads','2','-c:v','libx264','-preset','veryfast','-crf','20',str(compare)])
    parts.append(compare)
    sequence.append({'scene':'same-stop-comparison','begin':duration,'duration':6,'intentional_still':True}); duration+=6
    outro = stage/'outro.mp4'
    ff(['-loop','1','-i',str(source/'battle-hd-options.png'),'-vf',
        f"crop=1280:816:0:64,scale=1600:1020,pad=1920:1200:160:40:color=0x122426,{label('原版・加強版　跨平台引擎　自備合法原版資料')},format=yuv420p",
        '-t','4','-threads','2','-c:v','libx264','-preset','veryfast','-crf','20','-r','30',str(outro)])
    parts.append(outro)
    sequence.append({'scene':'outro','begin':duration,'duration':4,'intentional_still':True}); duration+=4
    concat = stage/'parts.txt'
    concat.write_text(''.join(f"file '{p}'\n" for p in parts))
    video = stage/f'san1-{version}-promo-local.mp4'
    ff(['-f','concat','-safe','0','-i',str(concat),'-stream_loop','-1','-i',str(wav),'-map','0:v','-map','1:a',
        '-vf','fps=30','-c:v','libx264','-preset','veryfast','-crf','20','-threads','2',
        '-c:a','aac','-b:a','192k','-af',
        f'afade=t=in:st=0:d=0.3,afade=t=out:st={duration-2.5:.3f}:d=2.5',
        '-t',f'{duration:.3f}','-movflags','+faststart',str(video)])
    final.mkdir()
    shutil.copy2(video,final/video.name)
video = final/video.name
probe = json.loads(run(['ffprobe','-v','error','-show_format','-show_streams','-of','json',str(video)]).stdout)
(final/'ffprobe.json').write_text(json.dumps(probe,indent=2)+'\n')
streams = {s['codec_type']:s for s in probe['streams']}
actual_duration = float(probe['format']['duration'])
assert (streams['video']['width'],streams['video']['height'])==(1920,1200) and 'audio' in streams
assert streams['video']['codec_name']=='h264' and streams['video']['avg_frame_rate']=='30/1'
assert streams['audio']['codec_name']=='aac' and streams['audio']['channels']==2
assert abs(actual_duration-duration)<.2
for name, args in [('volume',['-vn','-af','volumedetect']),('blackdetect',['-an','-vf','blackdetect=d=0.5:pix_th=0.08']),
                   ('freezedetect',['-an','-vf','freezedetect=n=-60dB:d=2']),
                   ('silencedetect',['-vn','-af','silencedetect=n=-60dB:d=3'])]:
    result=run(['ffmpeg','-nostdin','-hide_banner','-threads','2','-i',str(video),*args,'-f','null','-'])
    (final/(name+'.log')).write_text(result.stderr)
volume=(final/'volume.log').read_text()
mean=float(re.search(r'mean_volume: (-?[\d.]+) dB',volume)[1])
peak=float(re.search(r'max_volume: (-?[\d.]+) dB',volume)[1])
assert -55<mean<-5 and -30<peak<-.1
assert 'black_start:' not in (final/'blackdetect.log').read_text()
assert 'silence_start:' not in (final/'silencedetect.log').read_text()
for index,scene in enumerate(sequence):
    ff(['-ss',str(scene['begin']+min(3,scene['duration']/2)),'-i',str(video),'-frames:v','1','-threads','1',str(final/f'frame-{index}.png')])
ff(['-i',str(video),'-vf',f'fps=1/{duration/9},scale=640:400,tile=3x3','-frames:v','1','-threads','2',str(final/'contact-sheet.jpg')])
shutil.copy2(source/'receipt.json',final/'capture-receipt.json')
shutil.copy2(Path('/audio/receipt.json'),final/'original-audio-receipt.json')
(final/'RIGHTS.txt').write_text('本機保存，禁止公開散布。畫面為正式 remake 封包的正常 GUI 錄影。\n配樂是 DOSBox-X 執行原版 AA.EXE、音樂欣賞「風雲」的真實 OPL 錄音。\n公開 Release 只含引擎與合法字型，不附本片或高清素材。\n')
(final/'QA.json').write_text(json.dumps({'version':version,'technical_passed':True,'visual_review':'pending',
    'duration_seconds':actual_duration,'mean_volume_db':mean,'peak_volume_db':peak,'black_frames':False,
    'long_silence':False,'video_codec':'h264','audio_codec':'aac','fps':30,'audio_channels':2,
    'font_sha256':sha(Path(font)),
    'sequence':sequence,'freeze_review':'玩家停點、靜態比較與片尾依分鏡逐段審查','human_listening':'not_claimed',
    'video_sha256':sha(video),'audio_sha256':sha(wav)},ensure_ascii=False,indent=2)+'\n')
manifest_path=release/'SHA256SUMS.json'
manifest=json.loads(manifest_path.read_text())
manifest['promo']={'file':'promo/'+video.name,'bytes':video.stat().st_size,'sha256':sha(video),
    'rights':'local_only_original_art_and_music','duration_seconds':actual_duration,
    'source':'current_full_local_gui_and_original_dosbox_opl'}
tmp=manifest_path.with_suffix('.tmp')
tmp.write_text(json.dumps(manifest,ensure_ascii=False,indent=2,sort_keys=True)+'\n'); tmp.replace(manifest_path)
print(json.dumps(manifest['promo'],ensure_ascii=False))
