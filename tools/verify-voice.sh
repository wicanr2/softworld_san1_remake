#!/usr/bin/env bash
# 已知宣戰語音的正常 GUI 驗證；原始資料及錄音保持本機。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_IMAGE="${SAN1_GO_IMAGE:-eob-remake-release:1.26.7-ebiten2.9.9-20261008-r2}"
AUDIO_IMAGE="${SAN1_AUDIO_IMAGE:-eob-audio-capture:20261009-r3}"
INNER=tools/verify-voice-inner.py
if [[ "${SAN1_VOICE_CATALOG:-0}" == 1 ]]; then INNER=tools/verify-voice-catalog-inner.py; fi
OUT="${SAN1_VOICE_OUT:-workplace/audio/voice-current}"
case "$OUT" in workplace/audio/*) ;; *) echo '輸出須位於 workplace/audio/' >&2; exit 2 ;; esac
case "/$OUT/" in */../*|*/./*) echo '輸出不得含相對跳層' >&2; exit 2 ;; esac
REFERENCE="${OUT}-reference"
for path in "$ROOT" "$ROOT/org_game" "$ROOT/workplace/audio" \
  "$ROOT/workplace/hd-assets-mapcursor-v50-r1" "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$path" || { echo "缺少目錄：$path" >&2; exit 2; }
done
test ! -e "$ROOT/$OUT" && test ! -e "$ROOT/$REFERENCE" || { echo '輸出已存在，拒絕覆寫' >&2; exit 2; }
COMMON=(--rm --network none --memory 4g --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)"
  --mount "type=bind,src=$ROOT,dst=/src,readonly"
  --mount "type=bind,src=$ROOT/workplace,dst=/src/workplace"
  --mount "type=bind,src=$ROOT/org_game,dst=/orig,readonly"
  --mount "type=bind,src=$ROOT/workplace/hd-assets-mapcursor-v50-r1,dst=/src/workplace/hd-assets-mapcursor-v50-r1,readonly" -w /src)
timeout 3m docker run "${COMMON[@]}" --cpus 2 --name san1-voice-build \
  -e GOWORK=off -e GOMAXPROCS=2 -e GOCACHE=/src/workplace/gocache \
  -e GOMODCACHE=/src/workplace/gomodcache -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off \
  -e "SAN1_VOICE_REFERENCE=$REFERENCE" --entrypoint bash "$BUILD_IMAGE" -c '
    set -euo pipefail
    path=workplace/voice-check-build
    for target in "$path" "$path/san1-window-check" "$path/title-reference.png"; do
      if test -e "$target" && test "$(stat -c %u:%g "$target")" != "$(id -u):$(id -g)"; then exit 2; fi
    done
    mkdir -p "$path"
    go build -trimpath -o "$path/san1-window-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$path/title-reference.png" >/dev/null
    go run tools/voice-reference.go -root /orig -out "$SAN1_VOICE_REFERENCE"
    go run tools/full-voice-reference.go -root /orig -out "$SAN1_VOICE_REFERENCE"
    go run tools/hd-battle-branches-reference.go -out "$SAN1_VOICE_REFERENCE"
  '
timeout 12m docker run "${COMMON[@]}" --cpus 8 --name san1-voice-gui \
  -e LP_NUM_THREADS=2 --entrypoint python3 "$AUDIO_IMAGE" "$INNER" \
  --out "$OUT" --reference "$REFERENCE" --binary workplace/voice-check-build/san1-window-check \
  --title-reference workplace/voice-check-build/title-reference.png "$@"
