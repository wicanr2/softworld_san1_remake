#!/usr/bin/env bash
# 正常玩家路徑驗證隱藏選項列及首批高清圖層。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIG="${SAN1_ORIG:-$ROOT/org_game}"
for dir in "$ROOT" "$ORIG" "$ROOT/workplace" "$ROOT/workplace/hd-preview" \
  "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$dir" || { echo "缺少目錄：$dir" >&2; exit 2; }
done
common=(--rm --network none --memory 3g --cpus 2 --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 -u "$(id -u):$(id -g)"
  -v "$ROOT:/src" -v "$ORIG:/src/org_game:ro" -v "$ORIG:/orig:ro" -w /src)
timeout 3m docker run "${common[@]}" --name san1-window-verify-build \
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off -e GOWORK=off \
  --entrypoint /bin/bash rich2-go-ebiten:latest -c '
    set -euo pipefail
    for target in workplace/hd-window workplace/hd-assets; do
      if [[ -e "$target" && "$(stat -c %u:%g "$target")" != "$(id -u):$(id -g)" ]]; then
        echo "輸出擁有權不符：$target" >&2; exit 2
      fi
    done
    mkdir -p workplace/hd-window
    go build -trimpath -o workplace/hd-window/san1-window-check ./cmd/san1
    go run ./cmd/san1hdpack -root /orig/三國演義 -peer-root /orig/三國演義1加強版
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png workplace/hd-window/title-reference.png >/dev/null
  '
timeout 5m docker run "${common[@]}" --name san1-window-verify-gui \
  --entrypoint python3 eob-audio-capture:20260922-r2 tools/verify-window-inner.py
