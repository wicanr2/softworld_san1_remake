#!/usr/bin/env bash
# 用 DOSBox-X 跑原版、照腳本送鍵、逐格抓圖，最後合成影片。
#
# 用途是**對拍的參照**：dosgolem 那一側跑同一串按鍵，兩邊逐格比。
#
#   tools/dosboxx-record.sh keys.txt out/
#
# keys.txt 一行一步，格式 `<等待秒數> <xdotool 鍵名…>`，`#` 開頭是註解：
#
#   8   1              # 音樂
#   1   2              # 繪圖
#   1   2              # 磁碟
#   40  Return         # 開場動畫一路續行
#
# ⚠ 本儲存庫不含任何原版檔案；讀的是 `org_game/` 底下玩家自己那一份，
#    唯讀掛載，產出留在 gitignore 的 `workplace/`。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
KEYS="${1:?要給一個按鍵腳本}"
OUT="${2:-$ROOT/workplace/rec}"
case "$OUT" in /*) ;; *) OUT="$PWD/$OUT" ;; esac
IMAGE="${SAN1_DOSBOX_IMAGE:-civ1-dosboxx-input:20260830}"
GAME="${SAN1_DOSBOX_GAME:-$ROOT/org_game/三國演義}"
FPS="${SAN1_REC_FPS:-4}"

mkdir -p "$OUT/frames"
cp "$KEYS" "$OUT/keys.txt"

cat > "$OUT/.run.sh" <<'EOF'
set -euo pipefail
export HOME=/tmp DISPLAY=:99
Xvfb :99 -screen 0 1280x1024x24 >/tmp/xvfb.log 2>&1 &
sleep 2
mkdir -p /tmp/game; cp -r /orig/. /tmp/game/; chmod -R u+w /tmp/game
cat > /tmp/dosbox.conf <<CONF
[sdl]
output=surface
autolock=false
windowresolution=original
[dosbox]
machine=svga_s3
memsize=16
[render]
aspect=false
scaler=none
[cpu]
core=normal
cputype=386
cycles=fixed 60000
[autoexec]
mount c /tmp/game
c:
AA.EXE
CONF
dosbox-x -conf /tmp/dosbox.conf -nomenu >/tmp/dbx.log 2>&1 &
DBX=$!
sleep 8
WIN=$(xdotool search --name 'DOSBox-X' | tail -1)
# 沒有視窗管理員，焦點要自己設；不設的話按鍵送不進去，
# 而畫面照樣在動，看起來像遊戲卡住。
xdotool windowfocus --sync "$WIN"
sleep 1
FPSV=${FPS:-4}

# **用 x11grab 連續錄，不要逐格 import。**
# 一格 import ＋ convert 要 0.4 秒，按鍵腳本裡的「秒」會變成假的
# ——等 60 秒實際等了 144 秒，時序全部對不上。
ffmpeg -f x11grab -framerate $FPSV -video_size 640x408 -i :99 \
  -vf "crop=640:350:0:0,scale=1280:700:flags=neighbor" \
  -c:v libx264 -preset veryfast -pix_fmt yuv420p -y /out/parity.mp4 \
  >/tmp/ff.log 2>&1 &
FF=$!
sleep 1

N=0
snap() {  # 在關鍵時刻存 640×350 的原尺寸圖，給逐格比對用
  # **存兩張，隔一秒。** 兩張一樣才表示畫面靜止；不一樣就是還在動，
  # 那一步不能拿來當對拍的判準——動畫在兩個實作上不會停在同一格，
  # 而那不是誰做錯了。
  N=$((N+1))
  import -window "$WIN" /tmp/f.png 2>/dev/null || return 0
  convert /tmp/f.png -crop 640x350+0+0 +repage \
    "$(printf '/out/frames/%03d-%s.png' $N "$1")"
  sleep 1
  import -window "$WIN" /tmp/f2.png 2>/dev/null || return 0
  convert /tmp/f2.png -crop 640x350+0+0 +repage \
    "$(printf '/out/frames/%03d-%s.b.png' $N "$1")"
}

while IFS= read -r line; do
  line="${line%%#*}"
  [ -z "${line// /}" ] && continue
  secs=$(echo "$line" | awk '{print $1}')
  keys=$(echo "$line" | cut -d' ' -f2-)
  for k in $keys; do
    xdotool key --clearmodifiers "$k"
    sleep 0.15
  done
  sleep "$secs"
  snap "$(echo "$keys" | tr -d ' ')"
done < /out/keys.txt

sleep 1
kill -INT $FF 2>/dev/null || true
wait $FF 2>/dev/null || true
kill $DBX 2>/dev/null || true
sleep 1
echo "關鍵畫面 $N 張，影片 parity.mp4"
ls -l /out/parity.mp4 2>/dev/null || tail -5 /tmp/ff.log
EOF

echo "tools/dosboxx-record.sh：跑 DOSBox-X 錄製（一個容器、2 核）" >&2
timeout "${SAN1_REC_TIMEOUT:-1200}" docker run --rm --network none \
  --memory 2g --cpus 2 --pids-limit 256 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" \
  -e "FPS=$FPS" \
  -v "$GAME:/orig:ro" -v "$OUT:/out" \
  "$IMAGE" bash /out/.run.sh 2>&1 | grep -v XGetInputFocus || true

rm -f "$OUT/.run.sh"
ls -l "$OUT" | head
