#!/usr/bin/env bash
# 兩版正常對戰子畫面與快戰；只在 Docker 執行，素材與收據留本機。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIG="${SAN1_ORIG:-$ROOT/org_game}"
target="${SAN1_HD_BRANCHES_OUT:-workplace/hd-window/player/battle-branches-v7}"
pack="${SAN1_HD_BRANCHES_PACK:-workplace/hd-assets-scenes-v7}"
case "$target:$pack" in *..*|/*|*:/*) echo "輸出與素材須使用專案內相對路徑" >&2; exit 2 ;; esac
for path in "$ROOT" "$ORIG" "$ROOT/workplace/hd-window/player" "$ROOT/$pack" \
  "$ROOT/workplace/hd-inventory" "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$path" || { echo "缺少目錄：$path" >&2; exit 2; }
done
common=(--rm --network none --memory 3g --cpus 2 --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 -u "$(id -u):$(id -g)"
  -e SAN1_HD_BRANCHES_OUT="$target" -e SAN1_HD_BRANCHES_PACK="$pack"
  -v "$ROOT:/src" -v "$ORIG:/src/org_game:ro" -v "$ORIG:/orig:ro"
  -v "$ROOT/$pack:/src/$pack:ro" -v "$ROOT/workplace/hd-inventory:/src/workplace/hd-inventory:ro" -w /src)
timeout 3m docker run "${common[@]}" --name san1-hd-battle-branches-build \
  -e SAN1_HD_BRANCHES_OUT="$target" \
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off -e GOWORK=off \
  --entrypoint /bin/bash rich2-go-ebiten:latest -c '
    set -euo pipefail
    for path in workplace/hd-window/player "$SAN1_HD_BRANCHES_OUT"; do
      if [[ -e "$path" && "$(stat -c %u:%g "$path")" != "$(id -u):$(id -g)" ]]; then
        echo "輸出擁有權不符：$path" >&2; exit 2
      fi
    done
    mkdir -p "$SAN1_HD_BRANCHES_OUT"
    go build -trimpath -o "$SAN1_HD_BRANCHES_OUT/san1-window-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$SAN1_HD_BRANCHES_OUT/title-reference.png" >/dev/null
    go run ./tools/hd-battle-branches-reference.go -out "$SAN1_HD_BRANCHES_OUT"
    python3 -m py_compile tools/verify-hd-battle-branches-inner.py
    cp tools/verify-hd-battle-branches.sh tools/verify-hd-battle-branches-inner.py tools/verify-window-inner.py tools/hd-battle-branches-reference.go "$SAN1_HD_BRANCHES_OUT/"
  '
timeout 18m docker run "${common[@]}" --name san1-hd-battle-branches-gui \
  --entrypoint python3 eob-audio-capture:20260922-r2 tools/verify-hd-battle-branches-inner.py "$@"
