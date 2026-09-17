#!/usr/bin/env bash
# 用 DOSBox-X 把原版跑到主選單，抓一張 640×408 的畫面。
#
# 這一張的用途是**驗 dosgolem**：`workplace/shots/open/` 那幾張是 dosgolem
# 畫的，拿 dosgolem 自己畫的圖去驗 dosgolem 讀出來的版面等於自己驗自己
# （`CLAUDE.md` §4）。DOSBox-X 是獨立的第二個實作，兩邊對得起來才算數。
#
#   tools/dosboxx.sh                     # 存到 workplace/shots/dosboxx/
#   SAN1_DOSBOX_IMAGE=... tools/dosboxx.sh
#   SAN1_DOSBOX_MODE=trademark tools/dosboxx.sh
#       # 答完三個裝置問題就每半秒抓一張、抓十秒，存 trademark-NN.png
#       #（開機第一幕的智冠商標畫面，Issue #34）
#   SAN1_DOSBOX_MODE=opening tools/dosboxx.sh
#       # 片頭連拍：不按鍵拍到〈臨江仙〉等鍵、Enter、拍到三英圖、Enter、
#       # 拍到主選單，存 opening-{a,b,c}-NNN.png（Issue #61）
#
# 容器要有 dosbox-x、Xvfb、xdotool 與 ImageMagick。
# ⚠ 本儲存庫不含任何原版檔案；讀的是 `org_game/` 底下玩家自己那一份，
#    唯讀掛載，產出留在 gitignore 的 `workplace/`。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${SAN1_DOSBOX_IMAGE:-civ1-dosboxx-input:20260830}"
GAME="${SAN1_DOSBOX_GAME:-$ROOT/org_game/三國演義}"
OUT="${SAN1_DOSBOX_OUT:-$ROOT/workplace/shots/dosboxx}"

if [[ ! -d "$GAME" ]]; then
  echo "tools/dosboxx.sh：找不到原版目錄 $GAME" >&2
  exit 2
fi
mkdir -p "$OUT"

# 容器內的腳本。**cycles 固定**：`cycles=auto` 是可重現性的敵人，
# 兩份 bundle 附的 dosbox.conf 都是 auto，不要直接拿來用。
cat > "$OUT/.run.sh" <<'EOF'
set -euo pipefail
export HOME=/tmp DISPLAY=:99
Xvfb :99 -screen 0 1280x1024x24 >/tmp/xvfb.log 2>&1 &
sleep 2
# 遊戲會寫自己的目錄，所以複製一份出來跑，原版目錄保持唯讀。
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
# 而畫面照樣在動，看起來像「遊戲卡住」。
xdotool windowfocus --sync "$WIN"
sleep 1
key() { xdotool key --clearmodifiers "$1"; sleep "${2:-1}"; }
key 1; key 2; key 2 0.2        # 音效／繪圖／磁碟三題（int 21h AH=08）
if [[ "${MODE:-}" == opening ]]; then
  # 片頭（Issue #61）：不按鍵連拍到等鍵，按一次 Enter 再連拍到三英圖，
  # 再按一次連拍到主選單。每張檔名帶段號與序號。
  shoot() { for i in $(seq -w 0 "$2"); do import -window "$WIN" "/out/opening-$1-$i.png"; sleep "${3:-0.3}"; done; }
  shoot a 119; key Return 0.1
  shoot b 99; key Return 0.1
  shoot c 19
  kill $DBX 2>/dev/null || true
  exit 0
fi
if [[ "${MODE:-}" == trademark ]]; then
  for i in $(seq -w 0 19); do import -window "$WIN" /out/trademark-$i.png; sleep 0.5; done
  kill $DBX 2>/dev/null || true
  exit 0
fi
sleep 1.8
# 開場動畫一路按 Enter 續行。主選單只收 1–6，Enter 在那裡沒有作用，
# 所以按過頭是安全的；方向鍵不是（會走進「載入進度」那一層）。
for i in $(seq 1 30); do xdotool key --clearmodifiers Return; sleep 2; done
import -window "$WIN" /tmp/raw.png
# **640×408 就是畫面本體，不要裁。** 這裡原本寫著「上下各一條黑邊，
# 畫面本體在左上角 640×350」，而那句話自己就矛盾——上下都有黑邊的話
# 不會從左上角裁。原版寫進 CRTC 的 `[12] = 97h` 是 408 列
#（`docs/spec/006`），DOSBox-X 是完整的 VGA 實作，它給的高度就是答案。
cp /tmp/raw.png /out/menu.png
cp /tmp/raw.png /out/menu-raw.png
kill $DBX 2>/dev/null || true
EOF

echo "tools/dosboxx.sh：跑 DOSBox-X（約 80 秒，1 個容器、2 核）" >&2
timeout "${SAN1_DOSBOX_TIMEOUT:-300}" docker run --rm --network none \
  --memory 2g --cpus 2 --pids-limit 128 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" \
  -v "$GAME:/orig:ro" -v "$OUT:/out" -e MODE="${SAN1_DOSBOX_MODE:-}" \
  "$IMAGE" bash /out/.run.sh 2>&1 | grep -v XGetInputFocus || true

rm -f "$OUT/.run.sh"
ls -l "$OUT"
