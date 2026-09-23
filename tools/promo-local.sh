#!/usr/bin/env bash
# 主機只啟動有界 Docker 工作；含原版美術與配樂的影片只留在本機。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ $# -ne 1 || ! "$1" =~ ^v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}$ ]]; then
  echo '用法：tools/promo-local.sh v.<主版>.<次版>.<修訂版>-YYYYMMDD' >&2
  exit 2
fi
VER="$1"
for d in "$ROOT" "$ROOT/org_game" "$ROOT/workplace" "$ROOT/dist-all/$VER"; do
  [[ -d "$d" ]] || { echo "缺少掛載目錄：$d" >&2; exit 1; }
done
[[ -f "$ROOT/workplace/audio/思古.wav" && -f "$ROOT/tools/promo-local-inner.sh" ]] || {
  echo '缺少本機原版配樂或影片腳本' >&2; exit 1;
}
source_dir="$ROOT/workplace/promo-source/$VER"
mkdir -p "$source_dir"
for spec in \
  'title -screen title' \
  'lordpick -screen lordpick -sel 0' \
  'main -screen art -sel 8 -months 0 -ai base -what none' \
  'card -screen art -sel 8 -card 3' \
  'battle -screen artbattle -sel 8 -months 2' \
  'ending -screen art -sel 8 -months 6 -ai base -what none'; do
  read -r name args <<<"$spec"
  # 參數清單完全由本腳本固定，不接受使用者輸入或 shell 展開。
  read -r -a options <<<"$args"
  SAN1_TIMEOUT=5m "$ROOT/tools/go.sh" run ./cmd/san1dump \
    -root /orig/三國演義 -slot 001 "${options[@]}" -png "workplace/promo-source/$VER/$name.png"
done
timeout 10m docker run --rm --network none --memory 2g --cpus 2 --pids-limit 128 \
  --log-opt max-size=10m --log-opt max-file=3 -u "$(id -u):$(id -g)" \
  -v "$ROOT:/src" -v "$ROOT/org_game:/src/org_game:ro" \
  -w /src --entrypoint bash game-video:latest tools/promo-local-inner.sh "$VER"
