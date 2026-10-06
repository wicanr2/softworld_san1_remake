#!/usr/bin/env bash
# 現行完整高清包的音訊 GUI 驗證，輸出不可覆寫。
set -euo pipefail
ARGS=("$@")
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${SAN1_HD_AUDIO_OUT:-workplace/audio/hd-current}"
case "$OUT" in workplace/audio/*) ;; *) echo '輸出須位於 workplace/audio/' >&2; exit 2 ;; esac
case "/$OUT/" in */../*|*/./*) echo '輸出不得含相對跳層' >&2; exit 2 ;; esac
REFERENCE="${OUT}-reference"
for path in "$ROOT" "$ROOT/org_game" "$ROOT/workplace/audio" \
  "$ROOT/workplace/hd-assets-mapcursor-v50-r1" "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$path" || { echo "缺少目錄：$path" >&2; exit 2; }
done
test ! -e "$ROOT/$OUT" || { echo "輸出已存在：$OUT" >&2; exit 2; }
test ! -e "$ROOT/$REFERENCE" || { echo "參考已存在：$REFERENCE" >&2; exit 2; }
COMMON=(--rm --network none --memory 4g --cpus 2 --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)"
  --mount "type=bind,src=$ROOT,dst=/src"
  --mount "type=bind,src=$ROOT/org_game,dst=/src/org_game,readonly"
  --mount "type=bind,src=$ROOT/org_game,dst=/orig,readonly"
  --mount "type=bind,src=$ROOT/workplace/hd-assets-mapcursor-v50-r1,dst=/src/workplace/hd-assets-mapcursor-v50-r1,readonly" -w /src)
timeout 3m docker run "${COMMON[@]}" --name san1-hd-audio-build \
  -e "SAN1_HD_AUDIO_REFERENCE=$REFERENCE" \
  -e GOWORK=off -e GOMAXPROCS=2 -e GOCACHE=/src/workplace/gocache \
  -e GOMODCACHE=/src/workplace/gomodcache -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off \
  rich2-go-ebiten:latest bash -c '
    set -euo pipefail
    path=workplace/hd-audio-check-build
    if test -e "$path" && test "$(stat -c %u:%g "$path")" != "$(id -u):$(id -g)"; then exit 2; fi
    mkdir -p "$path"
    go build -trimpath -o "$path/san1-window-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$path/title-reference.png" >/dev/null
    mkdir -p "$SAN1_HD_AUDIO_REFERENCE"
    go run ./tools/hd-audio-reference.go -root /orig/三國演義 -out "$SAN1_HD_AUDIO_REFERENCE"
  '
timeout 12m docker run "${COMMON[@]}" --name san1-hd-audio-gui \
  --entrypoint python3 eob-audio-capture:20260922-r2 tools/verify-hd-audio-inner.py \
  --out "$OUT" --binary workplace/hd-audio-check-build/san1-window-check \
  --title-reference workplace/hd-audio-check-build/title-reference.png \
  --reference "$REFERENCE" "${ARGS[@]}"
