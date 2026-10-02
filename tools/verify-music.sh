#!/usr/bin/env bash
# 在隔離容器內驗證正式視窗的配樂輸出，不使用主機音效卡。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIG="${SAN1_ORIG:-$ROOT/org_game}"
case "${1:-}" in
  '') target=workplace/audio ;;
  --hd) target=workplace/audio/hd-v4 ;;
  *) echo '用法：bash tools/verify-music.sh [--hd]' >&2; exit 2 ;;
esac
[[ $# -le 1 ]] || exit 2
for dir in "$ROOT" "$ORIG" "$ROOT/workplace" "$ROOT/workplace/audio" \
  "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$dir" || { echo "缺少目錄：$dir" >&2; exit 2; }
done
if [[ "${1:-}" == --hd ]]; then
  test -d "$ROOT/workplace/hd-assets-portraits-v4" || { echo '缺少本機高清包 v4' >&2; exit 2; }
fi
uid="$(id -u):$(id -g)"
build_image="${SAN1_GO_IMAGE:-rich2-go-ebiten:latest}"
audio_image="${SAN1_AUDIO_IMAGE:-eob-audio-capture:20260922-r2}"
docker image inspect "$build_image" "$audio_image" >/dev/null
common=(--rm --network none --memory 3g --cpus 2 --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 -u "$uid"
  -v "$ROOT:/src" -v "$ORIG:/src/org_game:ro" -v "$ORIG:/orig:ro" -w /src)
if [[ "${1:-}" == --hd ]]; then
  common+=(-v "$ROOT/workplace/hd-assets-portraits-v4:/src/workplace/hd-assets-portraits-v4:ro")
fi
timeout 3m docker run "${common[@]}" --name san1-music-verify-build \
  -e SAN1_MUSIC_VERIFY_OUT="$target" \
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off -e GOWORK=off \
  --entrypoint /bin/bash "$build_image" -c '
    set -euo pipefail
    target="$SAN1_MUSIC_VERIFY_OUT"
    for path in workplace/audio "$target" "$target/san1-music-check"; do
      if [[ -e "$path" && "$(stat -c %u:%g "$path")" != "$(id -u):$(id -g)" ]]; then
        echo "輸出擁有權不符：$path" >&2; exit 2
      fi
    done
    mkdir -p "$target"
    bash -n tools/verify-music.sh
    python3 -m py_compile tools/verify-music-inner.py
    go build -trimpath -o "$target/san1-music-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title \
      -png "$target/music-check-title-reference.png" >/dev/null
  '
image_id="$(docker image inspect "$audio_image" --format '{{.Id}}')"
timeout 5m docker run "${common[@]}" --name san1-music-verify-capture \
  -e SAN1_AUDIO_IMAGE_ID="$image_id" --entrypoint python3 "$audio_image" \
  tools/verify-music-inner.py "$@"
