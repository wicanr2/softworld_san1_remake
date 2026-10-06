#!/usr/bin/env bash
# 正常 GUI 六槽存檔、關閉、主選單讀回；收據只留本機。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${SAN1_HD_SAVE_OUT:-workplace/hd-save-current}"
case "$OUT" in workplace/hd-save-*) ;; *) echo '輸出須位於 workplace/hd-save-*' >&2; exit 2 ;; esac
case "/$OUT/" in */../*|*/./*) echo '輸出不得含相對跳層' >&2; exit 2 ;; esac
for path in "$ROOT" "$ROOT/org_game" "$ROOT/workplace" \
  "$ROOT/workplace/hd-assets-mapcursor-v50-r1" "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$path" || { echo "缺少目錄：$path" >&2; exit 2; }
done
test ! -e "$ROOT/$OUT" || { echo "輸出已存在：$OUT" >&2; exit 2; }
COMMON=(--rm --network none --memory 4g --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)"
  --mount "type=bind,src=$ROOT,dst=/src"
  --mount "type=bind,src=$ROOT/org_game,dst=/src/org_game,readonly"
  --mount "type=bind,src=$ROOT/org_game,dst=/orig,readonly"
  --mount "type=bind,src=$ROOT/workplace/hd-assets-mapcursor-v50-r1,dst=/src/workplace/hd-assets-mapcursor-v50-r1,readonly" -w /src)
BUILD_ENV=(-e GOWORK=off -e GOMAXPROCS=2 -e GOCACHE=/src/workplace/gocache
  -e GOMODCACHE=/src/workplace/gomodcache -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off)
timeout 3m docker run "${COMMON[@]}" --cpus 2 --name san1-hd-save-build \
  "${BUILD_ENV[@]}" -e "SAN1_HD_SAVE_OUT=$OUT" rich2-go-ebiten:latest bash -c '
    set -euo pipefail
    test "$(stat -c %u:%g workplace)" = "$(id -u):$(id -g)"
    test ! -e "$SAN1_HD_SAVE_OUT"
    mkdir "$SAN1_HD_SAVE_OUT"
    go build -trimpath -o "$SAN1_HD_SAVE_OUT/san1-window-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$SAN1_HD_SAVE_OUT/title-reference.png" >/dev/null
    go run ./tools/hd-audio-reference.go -root /orig/三國演義 -out "$SAN1_HD_SAVE_OUT" -cursors-only
  '
timeout 20m docker run "${COMMON[@]}" --cpus 8 --name san1-hd-save-gui \
  -e LP_NUM_THREADS=2 --entrypoint python3 eob-audio-capture:20260922-r2 \
  tools/verify-hd-save-inner.py --out "$OUT"
timeout 3m docker run "${COMMON[@]}" --cpus 2 --name san1-hd-save-readback \
  "${BUILD_ENV[@]}" rich2-go-ebiten:latest go run ./tools/verify-hd-save-readback.go -out "$OUT"
