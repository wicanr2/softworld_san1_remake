#!/usr/bin/env bash
# 正常出兵宣戰的高清左右對白；只在 Docker 中執行。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIG="${SAN1_ORIG:-$ROOT/org_game}"
target=workplace/hd-window/player/dialogue-v4
for dir in "$ROOT" "$ORIG" "$ROOT/workplace/hd-window" "$ROOT/workplace/hd-assets-portraits-v4" \
  "$ROOT/workplace/hd-inventory" "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$dir" || { echo "缺少目錄：$dir" >&2; exit 2; }
done
common=(--rm --network none --memory 3g --cpus 2 --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 -u "$(id -u):$(id -g)"
  -v "$ROOT:/src" -v "$ORIG:/src/org_game:ro" -v "$ORIG:/orig:ro"
  -v "$ROOT/workplace/hd-assets-portraits-v4:/src/workplace/hd-assets-portraits-v4:ro"
  -v "$ROOT/workplace/hd-inventory:/src/workplace/hd-inventory:ro" -w /src)
timeout 3m docker run "${common[@]}" --name san1-hd-dialogue-build \
  -e SAN1_HD_DIALOGUE_OUT="$target" \
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off -e GOWORK=off \
  --entrypoint /bin/bash rich2-go-ebiten:latest -c '
    set -euo pipefail
    target="$SAN1_HD_DIALOGUE_OUT"
    for path in workplace/hd-window/player "$target"; do
      if [[ -e "$path" && "$(stat -c %u:%g "$path")" != "$(id -u):$(id -g)" ]]; then
        echo "輸出擁有權不符：$path" >&2; exit 2
      fi
    done
    mkdir -p "$target"
    bash -n tools/verify-hd-dialogue.sh
    python3 -m py_compile tools/verify-hd-dialogue-inner.py
    go build -trimpath -o "$target/san1-window-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$target/title-reference.png" >/dev/null
  '
timeout 6m docker run "${common[@]}" --name san1-hd-dialogue-gui \
  --entrypoint python3 eob-audio-capture:20260922-r2 tools/verify-hd-dialogue-inner.py
