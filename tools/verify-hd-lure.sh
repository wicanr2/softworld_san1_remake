#!/usr/bin/env bash
# 正常新局、合法紮寨與誘敵；素材、截圖及錄影只留本機。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIG="${SAN1_ORIG:-$ROOT/org_game}"
target="${SAN1_HD_LURE_OUT:-workplace/hd-window/player/lure-v21}"
pack="${SAN1_HD_LURE_PACK:-workplace/hd-assets-lure-v21}"
case "$target:$pack" in *..*|/*|*:/*) echo "輸出與素材須為專案內相對路徑" >&2; exit 2 ;; esac
for path in "$ROOT" "$ORIG" "$ROOT/workplace/hd-window/player" "$ROOT/$pack" "$ROOT/workplace/hd-inventory"; do
 test -d "$path" || { echo "缺少目錄：$path" >&2; exit 2; }
done
common=(--rm --network none --memory 4g --cpus 4 --pids-limit 256
 --log-opt max-size=10m --log-opt max-file=3 -u "$(id -u):$(id -g)"
 -e SAN1_HD_LURE_OUT="$target" -e SAN1_HD_LURE_PACK="$pack"
 -v "$ROOT:/src" -v "$ORIG:/src/org_game:ro" -v "$ORIG:/orig:ro"
 -v "$ROOT/$pack:/src/$pack:ro" -v "$ROOT/workplace/hd-inventory:/src/workplace/hd-inventory:ro" -w /src)
timeout 3m docker run "${common[@]}" --name san1-hd-lure-build \
 -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
 -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off -e GOWORK=off \
 --entrypoint /bin/bash rich2-go-ebiten:latest -c '
 set -euo pipefail
 for path in workplace/hd-window/player "$SAN1_HD_LURE_OUT"; do
  if [[ -e "$path" && "$(stat -c %u:%g "$path")" != "$(id -u):$(id -g)" ]]; then echo "輸出擁有權不符：$path" >&2; exit 2; fi
 done
 mkdir -p "$SAN1_HD_LURE_OUT"
 go build -trimpath -o "$SAN1_HD_LURE_OUT/san1-window-check" ./cmd/san1
 go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$SAN1_HD_LURE_OUT/title-reference.png" >/dev/null
 go run ./tools/hd-battle-branches-reference.go -out "$SAN1_HD_LURE_OUT"
 cp tools/verify-hd-lure.sh tools/verify-hd-lure-inner.py tools/verify-window-inner.py "$SAN1_HD_LURE_OUT/"
 '
timeout 12m docker run "${common[@]}" --name san1-hd-lure-gui \
 --entrypoint python3 eob-audio-capture:20260922-r2 tools/verify-hd-lure-inner.py "$@"
