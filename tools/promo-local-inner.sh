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
[[ -f /src/workplace/audio/思古.wav ]] || exit 1
rm -rf -- "$stage"
mkdir -p "$stage"
font=/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc
labels=('智冠 1991《三國演義》' '選擇年代與君主' '治理州郡，決定天下局勢'
        '調度武將與城池' '親自指揮戰場' '原版與加強版，跨平台重製')
inputs=()
for name in title lordpick main card battle ending; do
  inputs+=(-loop 1 -framerate 30 -t 7 -i "$source/$name.png")
done
inputs+=(-i /src/workplace/audio/思古.wav)
filters=''
for i in 0 1 2 3 4 5; do
  filters+="[$i:v]zoompan=z='min(zoom+0.0005,1.14)':d=1:x='(iw-iw/zoom)/2':y='(ih-ih/zoom)/2':s=1280x816:fps=30,"
  filters+="pad=1280:880:0:0:black,drawtext=fontfile=$font:text='${labels[$i]}':fontsize=36:fontcolor=white:x=(w-text_w)/2:y=825,format=yuv420p[v$i];"
done
filters+='[v0][v1][v2][v3][v4][v5]concat=n=6:v=1:a=0[v];[6:a]atrim=duration=42,afade=t=out:st=39.5:d=2.5[a]'
video="$stage/san1-$ver-promo-local.mp4"
ffmpeg -hide_banner -loglevel error -y "${inputs[@]}" \
  -filter_complex "$filters" -map '[v]' -map '[a]' \
  -c:v libx264 -preset veryfast -crf 20 -pix_fmt yuv420p -r 30 \
  -c:a aac -b:a 160k -movflags +faststart "$video"
ffprobe -v error -show_format -show_streams -of json "$video" > "$stage/ffprobe.json"
ffmpeg -hide_banner -i "$video" -vn -af volumedetect -f null - \
  > "$stage/volume.log" 2>&1
ffmpeg -hide_banner -i "$video" -vf 'blackdetect=d=0.5:pix_th=0.08' -an -f null - \
  > "$stage/blackdetect.log" 2>&1
ffmpeg -hide_banner -i "$video" -vf 'freezedetect=n=-60dB:d=2' -an -f null - \
  > "$stage/freezedetect.log" 2>&1
for i in 0 1 2 3 4 5; do
  second=$((3 + i * 7))
  ffmpeg -hide_banner -loglevel error -y -ss "$second" -i "$video" \
    -frames:v 1 "$stage/frame-$i.png"
done
montage "$stage"/frame-*.png -tile 2x3 -geometry 640x440+8+8 -background black \
  "$stage/contact-sheet.jpg"
sha256sum "$video" "$source"/*.png /src/workplace/audio/思古.wav \
  > "$stage/SHA256SUMS.txt"
cat > "$stage/RIGHTS.txt" <<EOF
版號：$ver
分類：僅供本機保存，不得公開上傳或再散布。
畫面：現行 remake 程式以玩家本機原版資料重新繪製，六張來源 PNG 由 tools/promo-local.sh 重生。
配樂：玩家本機原版《三國演義》「思古」資料轉出之 WAV；著作權不屬本重製專案。
影片：六段各七秒，另含字幕；畫面不是 DOS 原版錄影，也不代表戰術全程已對拍。
EOF
mv "$stage" "$final"
echo "本機推廣影片：$final/san1-$ver-promo-local.mp4"
