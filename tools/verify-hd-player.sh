#!/usr/bin/env bash
# 首批高清素材的正常玩家路徑，保留先前選項列收據。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIG="${SAN1_ORIG:-$ROOT/org_game}"
case "${1:-}" in
  '') target=workplace/hd-window/player ;;
  --scene-plus) target=workplace/hd-window/player/scene-plus ;;
  *) echo "用法：bash tools/verify-hd-player.sh [--scene-plus]" >&2; exit 2 ;;
esac
[[ $# -le 1 ]] || exit 2
for dir in "$ROOT" "$ORIG" "$ROOT/workplace/hd-window" "$ROOT/workplace/hd-assets" \
  "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$dir" || { echo "缺少目錄：$dir" >&2; exit 2; }
done
common=(--rm --network none --memory 3g --cpus 2 --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 -u "$(id -u):$(id -g)"
  -v "$ROOT:/src" -v "$ORIG:/src/org_game:ro" -v "$ORIG:/orig:ro" -w /src)
timeout 3m docker run "${common[@]}" --name san1-hd-player-build \
  -e SAN1_HD_PLAYER_OUT="$target" \
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off -e GOWORK=off \
  --entrypoint /bin/bash rich2-go-ebiten:latest -c '
    set -euo pipefail
    target="$SAN1_HD_PLAYER_OUT"
    for path in workplace/hd-window "$target"; do
      if [[ -e "$path" && "$(stat -c %u:%g "$path")" != "$(id -u):$(id -g)" ]]; then
        echo "輸出擁有權不符：$path" >&2; exit 2
      fi
    done
    mkdir -p "$target"
    go build -trimpath -o "$target/san1-window-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$target/title-reference.png" >/dev/null
    python3 -m py_compile tools/verify-window-inner.py tools/verify-hd-player-inner.py
  '
timeout 12m docker run "${common[@]}" --name san1-hd-player-gui \
  --entrypoint python3 eob-audio-capture:20260922-r2 tools/verify-hd-player-inner.py "$@"
