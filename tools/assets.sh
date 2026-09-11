#!/usr/bin/env bash
# 把原版素材轉成通用格式：圖 → PNG、資料表 → JSON、
# 配樂與 PC 喇叭的音效／語音 → OGG。
#
#   tools/assets.sh /path/to/三國演義 [輸出目錄]
#
# 兩步走：`cmd/san1assets` 在 Go 容器裡把配樂合成、把一位元取樣解成 WAV，
# 再交給另一個
# 容器的 ffmpeg 編成 Vorbis。編碼器不進 Go 執行檔——把 libvorbis 搬進去
# 要嘛帶 cgo 要嘛自己寫一個編碼器，兩件事都比這個腳本貴得多。
#
# 本儲存庫不含任何原版檔案；轉出來的東西是玩家自己那一份的內容，
# 輸出目錄預設在 workplace/（已經 gitignore），**不要散布**。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIG="${1:?用法：tools/assets.sh /path/to/三國演義 [輸出目錄]}"
OUT="${2:-$ROOT/workplace/assets}"
AUDIO_IMAGE="${SAN1_AUDIO_IMAGE:-eob-remake-audio:20260901}"
QUALITY="${SAN1_OGG_QUALITY:-5}"

SAN1_ORIG="$ORIG" "$ROOT/tools/go.sh" run ./cmd/san1assets -root /orig -out "${OUT#$ROOT/}"

# 配樂在 music/、PC 喇叭的音效與語音在 speech/，兩邊都是 WAV 轉 OGG。
# **取樣率各自保留**：配樂是 OPL2 的 49,716 Hz，語音是一位元取樣重取樣後
# 的 22,050 Hz——統一拉到 44,100 只會把語音放大成更大的檔。
shopt -s nullglob
total=0
for sub in music speech; do
  wavs=("$OUT/$sub"/*.wav)
  [[ ${#wavs[@]} -eq 0 ]] && continue
  total=$((total + ${#wavs[@]}))
  echo
  echo "轉 OGG：$sub ${#wavs[@]} 個檔（品質 $QUALITY）"
  docker run --rm --network none \
    --memory 2g --cpus 2 --pids-limit 128 \
    --log-opt max-size=10m --log-opt max-file=3 \
    -u "$(id -u):$(id -g)" \
    -v "$OUT/$sub:/audio" -w /audio \
    --entrypoint sh "$AUDIO_IMAGE" -c '
      set -e
      for f in *.wav; do
        ffmpeg -hide_banner -loglevel error -y -i "$f" \
          -c:a libvorbis -qscale:a '"$QUALITY"' "${f%.wav}.ogg"
      done
    '
  rm -f "$OUT/$sub"/*.wav
done
if [[ $total -eq 0 ]]; then
  echo "沒有 WAV 要轉，結束。"
  exit 0
fi
du -sh "$OUT"/music "$OUT"/speech 2>/dev/null
echo
echo "⚠ 這些是你自己那一份原版的內容，只給你自己用，不要散布。"
