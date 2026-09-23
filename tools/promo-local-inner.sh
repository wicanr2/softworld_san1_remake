#!/usr/bin/env bash
# 僅供 tools/promo-local.sh 在 game-video Docker 容器內呼叫。
set -euo pipefail
ver="${1:-}"
[[ "$ver" =~ ^v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}$ ]] || exit 2
source="/src/workplace/promo-source/$ver"
release="/src/dist-all/$ver"
stage="/src/workplace/promo-build/$ver"
final="$release/promo"
[[ -d "$source" && -d "$release" && ! -e "$final" ]] || {
  echo '截圖來源或交付目錄不符，或影片已存在' >&2; exit 1;
}
for dir in /src/workplace /src/workplace/promo-build "$release"; do
  if [[ -e "$dir" && "$(stat -c '%u:%g' "$dir")" != "$(id -u):$(id -g)" ]]; then
    echo "輸出目錄擁有權不符：$dir" >&2; exit 1
  fi
done
for name in title lordpick main card battle ending; do
  [[ -f "$source/$name.png" ]] || { echo "缺少現行畫面：$name" >&2; exit 1; }
done
[[ -f /src/workplace/audio/風雲.wav ]] || exit 1
rm -rf -- "$stage"
mkdir -p "$stage"
font=/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc
labels=('智冠 1991《三國演義》' '選擇年代與君主' '治理州郡，決定天下局勢'
        '調度武將與城池' '親自指揮戰場' '原版與加強版，跨平台重製')
video="$stage/san1-$ver-promo-local.mp4"
names=(title lordpick main card battle ending)
: > "$stage/parts.txt"
for i in 0 1 2 3 4 5; do
  part="$stage/part-$i.mp4"
  filter="zoompan=z='1.0+on*0.0008':d=1:x='(iw-iw/zoom)/2':y='(ih-ih/zoom)/2':s=1280x816:fps=30,"
  filter+="pad=1280:880:0:0:black,drawtext=fontfile=$font:text='${labels[$i]}':fontsize=36:fontcolor=white:x=(w-text_w)/2:y=825,format=yuv420p"
  ffmpeg -hide_banner -loglevel error -y -loop 1 -framerate 30 -t 7 \
    -i "$source/${names[$i]}.png" -vf "$filter" \
    -c:v libx264 -preset veryfast -crf 20 -pix_fmt yuv420p -r 30 \
    -threads 1 -an "$part"
  printf "file '%s'\n" "$part" >> "$stage/parts.txt"
done
ffmpeg -hide_banner -loglevel error -y -f concat -safe 0 -i "$stage/parts.txt" \
  -i /src/workplace/audio/風雲.wav -map 0:v -map 1:a \
  -c:v copy -c:a aac -b:a 160k -af 'volume=14dB,afade=t=out:st=39.5:d=2.5' \
  -t 42 -movflags +faststart "$video"
rm -f -- "$stage"/part-*.mp4 "$stage/parts.txt"
ffprobe -v error -show_format -show_streams -of json "$video" > "$stage/ffprobe.json"
ffmpeg -hide_banner -i "$video" -vn -af volumedetect -f null - \
  > "$stage/volume.log" 2>&1
ffmpeg -hide_banner -i "$video" -vf 'blackdetect=d=0.5:pix_th=0.08' -an -f null - \
  > "$stage/blackdetect.log" 2>&1
ffmpeg -hide_banner -i "$video" -vf 'freezedetect=n=-60dB:d=2' -an -f null - \
  > "$stage/freezedetect.log" 2>&1
python3 - "$stage" <<'PY'
import json
from pathlib import Path
import re
import sys

root = Path(sys.argv[1])
probe = json.loads((root / 'ffprobe.json').read_text())
streams = probe['streams']
video = next((s for s in streams if s['codec_type'] == 'video'), None)
audio = next((s for s in streams if s['codec_type'] == 'audio'), None)
duration = float(probe['format']['duration'])
volume = (root / 'volume.log').read_text()
mean = re.search(r'mean_volume: (-?[0-9.]+) dB', volume)
peak = re.search(r'max_volume: (-?[0-9.]+) dB', volume)
if video is None or (video['width'], video['height']) != (1280, 880) or audio is None:
    raise SystemExit('影片缺少正確尺寸的視訊或音訊')
if not 41.9 <= duration <= 42.1:
    raise SystemExit(f'影片時長不符：{duration}')
if mean is None or peak is None or float(mean[1]) < -55 or float(peak[1]) < -30:
    raise SystemExit('影片音量太低或近乎靜音')
for name, marker in [('blackdetect.log', 'black_start:'),
                     ('freezedetect.log', 'freeze_start:')]:
    if marker in (root / name).read_text():
        raise SystemExit(f'影片有未預期的黑幀或凍結：{name}')
(root / 'QA.txt').write_text(
    f'視訊 1280x880、音訊存在、時長 {duration:.3f} 秒、平均音量 {mean[1]} dB、'
    '無連續 0.5 秒黑幀及 2 秒凍結：通過。\n'
    '字幕裁切：請檢視 contact-sheet.jpg 六格抽樣幀。\n', encoding='utf-8')
PY
for i in 0 1 2 3 4 5; do
  second=$((3 + i * 7))
  ffmpeg -hide_banner -loglevel error -y -ss "$second" -i "$video" \
    -frames:v 1 "$stage/frame-$i.png"
done
montage "$stage"/frame-*.png -tile 2x3 -geometry 640x440+8+8 -background black \
  "$stage/contact-sheet.jpg"
sha256sum "$video" "$source"/*.png /src/workplace/audio/風雲.wav \
  > "$stage/SHA256SUMS.txt"
cat > "$stage/RIGHTS.txt" <<EOF
版號：$ver
分類：僅供本機保存，不得公開上傳或再散布。
畫面：現行 remake 程式以玩家本機原版資料重新繪製，六張來源 PNG 由 tools/promo-local.sh 重生。
配樂：玩家本機原版《三國演義》「風雲」資料轉出之 WAV；著作權不屬本重製專案。
影片：六段各七秒，另含字幕；畫面不是 DOS 原版錄影，也不代表戰術全程已對拍。
EOF
mv "$stage" "$final"
python3 - "$release/SHA256SUMS.json" "$final/san1-$ver-promo-local.mp4" "$ver" <<'PY'
import hashlib
import json
from pathlib import Path
import sys

manifest_path, video, version = Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3]
manifest = json.loads(manifest_path.read_text(encoding='utf-8'))
if manifest['version'] != version:
    raise SystemExit('影片與版本清單的版號不符')
manifest['promo'] = {'file': 'promo/' + video.name, 'bytes': video.stat().st_size,
                     'sha256': hashlib.sha256(video.read_bytes()).hexdigest(),
                     'rights': 'local_only_original_art_and_music',
                     'duration_seconds': 42}
temporary = manifest_path.with_suffix('.tmp')
temporary.write_text(json.dumps(manifest, ensure_ascii=False, indent=2, sort_keys=True) + '\n',
                     encoding='utf-8')
temporary.replace(manifest_path)
PY
echo "本機推廣影片：$final/san1-$ver-promo-local.mp4"
