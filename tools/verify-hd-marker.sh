#!/usr/bin/env bash
# 私人姓名驗證；所有建置、參考與正常 GUI 都在有界容器中執行。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIG="${SAN1_ORIG:-$ROOT/org_game}"
target="${SAN1_HD_MARKER_OUT:-workplace/hd-marker-check}"
pack="${SAN1_HD_MARKER_PACK:-workplace/hd-assets-mapcursor-v50-r1}"
case "$target:$pack" in *..*|/*|*:/*) echo '需要專案內相對路徑' >&2; exit 2 ;; esac
for path in "$ROOT" "$ORIG" "$ROOT/workplace" "$ROOT/$pack" \
  "$ROOT/workplace/hd-inventory" "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$path" || { echo "缺少目錄：$path" >&2; exit 2; }
done
test ! -e "$ROOT/$target" || { echo '輸出目錄已存在' >&2; exit 2; }
common=(--rm --network none --memory 4g --pids-limit 256 --user "$(id -u):$(id -g)"
  --log-opt max-size=10m --log-opt max-file=3
  -e SAN1_HD_MARKER_OUT="$target" -e SAN1_HD_MARKER_PACK="$pack"
  -v "$ROOT:/src" -v "$ORIG:/src/org_game:ro" -v "$ORIG:/orig:ro"
  -v "$ROOT/$pack:/src/$pack:ro" -v "$ROOT/workplace/hd-inventory:/src/workplace/hd-inventory:ro" -w /src)
timeout 3m docker run "${common[@]}" --cpus 2 --name san1-hd-marker-build \
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off -e GOWORK=off \
  --entrypoint /bin/bash rich2-go-ebiten:latest -c '
    set -euo pipefail
    test "$(stat -c %u:%g workplace)" = "$(id -u):$(id -g)"
    mkdir "$SAN1_HD_MARKER_OUT"
    Xvfb :99 -screen 0 800x600x24 -nolisten tcp >/tmp/marker-build-xvfb.log 2>&1 &
    xpid=$!
    trap '\''kill "$xpid" 2>/dev/null || true; wait "$xpid" 2>/dev/null || true'\'' EXIT
    export DISPLAY=:99
    sleep .3
    go build -trimpath -o "$SAN1_HD_MARKER_OUT/san1-window-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$SAN1_HD_MARKER_OUT/title-reference.png" >/dev/null
    go run ./tools/hd-battle-branches-reference.go -out "$SAN1_HD_MARKER_OUT"
    go run ./tools/hd-marker-reference.go -out "$SAN1_HD_MARKER_OUT"
    cp tools/verify-hd-marker.sh tools/verify-hd-marker-inner.py tools/hd-marker-reference.go \
      tools/verify-window-inner.py tools/verify-hd-battle-branches-inner.py \
      tools/hd-battle-branches-reference.go "$SAN1_HD_MARKER_OUT/"
    python3 -m py_compile tools/verify-hd-marker-inner.py
  '
timeout 12m docker run "${common[@]}" --cpus 8 --name san1-hd-marker-gui \
  --entrypoint python3 eob-audio-capture:20260922-r2 tools/verify-hd-marker-inner.py "$@"
