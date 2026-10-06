#!/usr/bin/env bash
# 正常新局任命軍師，驗兩版三語及 Original／4×／恢復；完整證據只留本機。
# 重跑請指定尚不存在的 SAN1_HD_MAIN_BUBBLES_OUT，避免覆蓋舊收據。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${SAN1_HD_MAIN_BUBBLES_OUT:-workplace/hd-window/player/main-bubbles}"
for path in "$ROOT" "$ROOT/org_game" "$ROOT/workplace" \
  "$ROOT/workplace/hd-assets-mapcursor-v50-r1" \
  "$ROOT/workplace/hd-inventory/base/img/DATA3" \
  "$ROOT/workplace/hd-inventory/plus/img/DATA3"; do
  test -d "$path" || { echo "缺少輸入目錄：$path" >&2; exit 2; }
done
test ! -e "$ROOT/$OUT" || { echo "輸出已存在：$OUT" >&2; exit 2; }
COMMON=(--rm --network none --memory 4g --cpus 2 --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)"
  --mount "type=bind,src=$ROOT,dst=/src"
  --mount "type=bind,src=$ROOT/org_game,dst=/src/org_game,readonly"
  --mount "type=bind,src=$ROOT/org_game,dst=/orig,readonly" -w /src)
timeout 3m docker run "${COMMON[@]}" --name san1-hd-main-bubbles-build \
  -e GOWORK=off -e GOMAXPROCS=2 -e GOCACHE=/src/workplace/gocache \
  -e GOMODCACHE=/src/workplace/gomodcache \
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off \
  rich2-go-ebiten:latest bash -c '
    set -e
    path=workplace/hd-main-bubble-check-build
    if test -e "$path" && test "$(stat -c %u:%g "$path")" != "$(id -u):$(id -g)"; then
      echo "建置目錄擁有權不符" >&2; exit 2
    fi
    mkdir -p "$path"
    go build -trimpath -o "$path/san1-window-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$path/title-reference.png" >/dev/null
  '
timeout 12m docker run "${COMMON[@]}" --name san1-hd-main-bubbles-gui \
  --entrypoint python3 eob-audio-capture:20260922-r2 \
  tools/verify-hd-main-bubbles-inner.py --out "$OUT" \
  --binary workplace/hd-main-bubble-check-build/san1-window-check \
  --title-reference workplace/hd-main-bubble-check-build/title-reference.png "$@"
