#!/usr/bin/env bash
# 人事、築城、射箭與退兵的正常 GUI；私人素材與收據。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIG="${SAN1_ORIG:-$ROOT/org_game}"
target=workplace/hd-window/player/events-v8
pack=workplace/hd-assets-scenes-v8
for path in "$ROOT" "$ORIG" "$ROOT/workplace/hd-window/player" "$ROOT/$pack" \
  "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$path" || { echo "缺少目錄：$path" >&2; exit 2; }
done
common=(--rm --network none --memory 3g --cpus 2 --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 -u "$(id -u):$(id -g)"
  -v "$ROOT:/src" -v "$ORIG:/src/org_game:ro" -v "$ORIG:/orig:ro"
  -v "$ROOT/$pack:/src/$pack:ro" -w /src)
timeout 3m docker run "${common[@]}" --name san1-hd-events-build \
  -e SAN1_HD_EVENTS_OUT="$target" \
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off -e GOWORK=off \
  --entrypoint /bin/bash rich2-go-ebiten:latest -c '
    set -euo pipefail
    for path in workplace/hd-window/player "$SAN1_HD_EVENTS_OUT"; do
      if [[ -e "$path" && "$(stat -c %u:%g "$path")" != "$(id -u):$(id -g)" ]]; then
        echo "輸出擁有權不符：$path" >&2; exit 2
      fi
    done
    mkdir -p "$SAN1_HD_EVENTS_OUT"
    go build -trimpath -o "$SAN1_HD_EVENTS_OUT/san1-window-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$SAN1_HD_EVENTS_OUT/title-reference.png" >/dev/null
    go run ./tools/hd-battle-branches-reference.go -out "$SAN1_HD_EVENTS_OUT"
    go run ./tools/hd-events-reference.go -out "$SAN1_HD_EVENTS_OUT"
    python3 -m py_compile tools/verify-hd-events-inner.py tools/verify-hd-scenes-inner.py tools/verify-hd-battle-branches-inner.py tools/verify-window-inner.py
    cp tools/verify-hd-events.sh tools/verify-hd-events-inner.py tools/hd-events-reference.go \
      tools/verify-hd-scenes-inner.py tools/verify-hd-battle-branches-inner.py \
      tools/hd-battle-branches-reference.go tools/verify-window-inner.py "$SAN1_HD_EVENTS_OUT/"
    cp cmd/san1/main.go "$SAN1_HD_EVENTS_OUT/san1-main.go"
    cp cmd/san1/governor_test.go "$SAN1_HD_EVENTS_OUT/governor_test.go"
  '
timeout 25m docker run "${common[@]}" --name san1-hd-events-gui \
  --entrypoint python3 eob-audio-capture:20260922-r2 tools/verify-hd-events-inner.py "$@"
