#!/usr/bin/env bash
# 正常任命軍師及三種謀略；私人素材與收據。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIG="${SAN1_ORIG:-$ROOT/org_game}"
target=workplace/hd-window/player/plots-v9
pack=workplace/hd-assets-scenes-v9
for path in "$ROOT" "$ORIG" "$ROOT/workplace/hd-window/player" "$ROOT/$pack" \
  "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$path" || { echo "缺少目錄：$path" >&2; exit 2; }
done
common=(--rm --network none --memory 3g --cpus 2 --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 -u "$(id -u):$(id -g)"
  -v "$ROOT:/src" -v "$ORIG:/src/org_game:ro" -v "$ORIG:/orig:ro"
  -v "$ROOT/$pack:/src/$pack:ro" -w /src)
timeout 3m docker run "${common[@]}" --name san1-hd-plots-build \
  -e SAN1_HD_PLOTS_OUT="$target" \
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off -e GOWORK=off \
  --entrypoint /bin/bash rich2-go-ebiten:latest -c '
    set -euo pipefail
    for path in workplace/hd-window/player "$SAN1_HD_PLOTS_OUT"; do
      if [[ -e "$path" && "$(stat -c %u:%g "$path")" != "$(id -u):$(id -g)" ]]; then
        echo "輸出擁有權不符：$path" >&2; exit 2
      fi
    done
    mkdir -p "$SAN1_HD_PLOTS_OUT"
    Xvfb :98 -screen 0 640x408x24 -nolisten tcp >/tmp/san1-plots-reference-xvfb.log 2>&1 &
    reference_pid=$!
    trap '\''kill "$reference_pid" 2>/dev/null || true; wait "$reference_pid" 2>/dev/null || true'\'' EXIT
    export DISPLAY=:98 LIBGL_ALWAYS_SOFTWARE=1 XDG_RUNTIME_DIR=/tmp
    go build -trimpath -o "$SAN1_HD_PLOTS_OUT/san1-window-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$SAN1_HD_PLOTS_OUT/title-reference.png" >/dev/null
    go run ./tools/hd-battle-branches-reference.go -out "$SAN1_HD_PLOTS_OUT"
    go run ./tools/hd-events-reference.go -out "$SAN1_HD_PLOTS_OUT"
    go run ./tools/hd-plots-reference.go -out "$SAN1_HD_PLOTS_OUT"
    python3 -m py_compile tools/verify-hd-plots-inner.py tools/verify-hd-events-inner.py \
      tools/verify-hd-scenes-inner.py tools/verify-hd-battle-branches-inner.py tools/verify-window-inner.py
    cp tools/verify-hd-plots.sh tools/verify-hd-plots-inner.py tools/hd-plots-reference.go \
      tools/verify-hd-events-inner.py tools/verify-hd-scenes-inner.py tools/verify-hd-battle-branches-inner.py \
      tools/hd-battle-branches-reference.go tools/hd-events-reference.go tools/verify-window-inner.py "$SAN1_HD_PLOTS_OUT/"
    cp cmd/san1/main.go "$SAN1_HD_PLOTS_OUT/san1-main.go"
  '
timeout 12m docker run "${common[@]}" --name san1-hd-plots-gui \
  --entrypoint python3 eob-audio-capture:20260922-r2 tools/verify-hd-plots-inner.py "$@"
