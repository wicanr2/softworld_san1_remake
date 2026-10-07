#!/usr/bin/env bash
# 現行程式的正常晚期讀檔、高清製作群與主選單返回。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${SAN1_HD_CREDITS_OUT:-workplace/hd-credits-current}"
case "$OUT" in workplace/hd-credits-*) ;; *) echo '輸出須位於 workplace/hd-credits-*' >&2; exit 2 ;; esac
case "/$OUT/" in */../*|*/./*) exit 2 ;; esac
test ! -e "$ROOT/$OUT"
for path in "$ROOT" "$ROOT/org_game" "$ROOT/workplace" "$ROOT/workplace/gocache" \
  "$ROOT/workplace/gomodcache" "$ROOT/workplace/hd-assets-mapcursor-v50-r1" \
  "$ROOT/workplace/credits-v44-fixture-r1" "$ROOT/workplace/hd-credits-v44-native-formal-r1"; do
  test -d "$path" || { echo "缺少目錄：$path" >&2; exit 2; }
done
COMMON=(--rm --network none --memory 4g --pids-limit 256 --user "$(id -u):$(id -g)"
  --log-opt max-size=10m --log-opt max-file=3 --mount "type=bind,src=$ROOT,dst=/src"
  --mount "type=bind,src=$ROOT/org_game,dst=/src/org_game,readonly"
  --mount "type=bind,src=$ROOT/org_game,dst=/orig,readonly"
  --mount "type=bind,src=$ROOT/workplace/hd-assets-mapcursor-v50-r1,dst=/src/workplace/hd-assets-mapcursor-v50-r1,readonly"
  --mount "type=bind,src=$ROOT/workplace/credits-v44-fixture-r1,dst=/src/workplace/credits-v44-fixture-r1,readonly"
  --mount "type=bind,src=$ROOT/workplace/hd-credits-v44-native-formal-r1,dst=/src/workplace/hd-credits-v44-native-formal-r1,readonly" -w /src)
timeout 4m docker run "${COMMON[@]}" --cpus 2 --name san1-hd-credits-build \
  -e SAN1_HD_CREDITS_OUT="$OUT" -e GOWORK=off -e GOMAXPROCS=2 \
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off \
  rich2-go-ebiten:latest bash -c '
    set -euo pipefail
    test "$(stat -c %u:%g workplace)" = "$(id -u):$(id -g)"
    mkdir "$SAN1_HD_CREDITS_OUT"
    go build -trimpath -o "$SAN1_HD_CREDITS_OUT/san1-window-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$SAN1_HD_CREDITS_OUT/title-reference.png" >/dev/null
    Xvfb :99 -screen 0 640x480x24 -nolisten tcp >/tmp/san1-credits-xvfb.log 2>&1 &
    display_pid=$!
    trap '\''kill "$display_pid" 2>/dev/null || true; wait "$display_pid" 2>/dev/null || true'\'' EXIT
    DISPLAY=:99 go run ./tools/hd-credits-ending-reference.go
    cp tools/verify-hd-credits.sh tools/verify-hd-credits-inner.py tools/verify-window-inner.py tools/hd-credits-ending-reference.go "$SAN1_HD_CREDITS_OUT/"
  '
timeout 8m docker run "${COMMON[@]}" --cpus 8 --name san1-hd-credits-gui \
  -e LP_NUM_THREADS=2 -e PYTHONDONTWRITEBYTECODE=1 --entrypoint python3 \
  eob-audio-capture:20260922-r2 tools/verify-hd-credits-inner.py --out "$OUT" "$@"
