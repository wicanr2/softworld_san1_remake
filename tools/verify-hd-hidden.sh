#!/usr/bin/env bash
# 兩版 006 劉備正常休息後尋訪姜維；只在 Docker 中執行。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIG="${SAN1_ORIG:-$ROOT/org_game}"
target=workplace/hd-window/player/hidden-v18
pack=workplace/hd-assets-portraits-v18
for dir in "$ROOT" "$ORIG" "$ROOT/workplace/hd-window" "$ROOT/$pack" \
  "$ROOT/workplace/hd-inventory" "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$dir" || { echo "缺少目錄：$dir" >&2; exit 2; }
done
common=(--rm --network none --memory 3g --cpus 2 --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 -u "$(id -u):$(id -g)"
  -v "$ROOT:/src" -v "$ORIG:/src/org_game:ro" -v "$ORIG:/orig:ro"
  -v "$ROOT/$pack:/src/$pack:ro"
  -v "$ROOT/workplace/hd-inventory:/src/workplace/hd-inventory:ro" -w /src)
timeout 3m docker run "${common[@]}" --name san1-hd-hidden-build \
  -e SAN1_HD_CONTEXTS_OUT="$target" \
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off -e GOWORK=off \
  --entrypoint /bin/bash rich2-go-ebiten:latest -c '
    set -euo pipefail
    target="$SAN1_HD_CONTEXTS_OUT"
    for path in workplace/hd-window/player "$target"; do
      if [[ -e "$path" && "$(stat -c %u:%g "$path")" != "$(id -u):$(id -g)" ]]; then
        echo "輸出擁有權不符：$path" >&2; exit 2
      fi
    done
    mkdir -p "$target"
    bash -n tools/verify-hd-hidden.sh
    python3 -m py_compile tools/verify-hd-hidden-inner.py
    go build -trimpath -o "$target/san1-window-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$target/title-reference.png" >/dev/null
    cp tools/verify-hd-hidden.sh tools/verify-hd-hidden-inner.py tools/verify-window-inner.py "$target/"
  '
timeout 14m docker run "${common[@]}" --name san1-hd-hidden-gui \
  --entrypoint python3 eob-audio-capture:20260922-r2 tools/verify-hd-hidden-inner.py
