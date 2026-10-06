#!/usr/bin/env bash
# 兩版正常單挑的高清使用端；素材與收據留在本機。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIG="${SAN1_ORIG:-$ROOT/org_game}"
pack="${SAN1_HD_DUEL_PACK:-workplace/hd-assets-mapcursor-v50-r1}"
target="${SAN1_HD_DUEL_OUT:-workplace/hd-window/player/duel}"
[[ $# -eq 0 ]] || { echo '用法：bash tools/verify-hd-duel.sh' >&2; exit 2; }
for rel in "$pack" "$target"; do
  [[ "$rel" == workplace/* && "$rel" != *'..'* ]] || { echo '輸出與素材須位於 workplace/ 內' >&2; exit 2; }
done
for dir in "$ROOT" "$ORIG" "$ROOT/$pack" "$ROOT/workplace/hd-inventory" \
  "$ROOT/workplace/hd-window/player" "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$dir" || { echo "缺少目錄：$dir" >&2; exit 2; }
done
test -f "$ROOT/$pack/manifest.json" || exit 2
test ! -e "$ROOT/$target" || { echo "拒絕覆寫既有收據：$target" >&2; exit 2; }
common=(--rm --network none --memory 2g --cpus 2 --pids-limit 128
  --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)"
  --mount "type=bind,src=$ROOT,dst=/src"
  --mount "type=bind,src=$ORIG,dst=/src/org_game,readonly"
  --mount "type=bind,src=$ORIG,dst=/orig,readonly"
  --mount "type=bind,src=$ROOT/$pack,dst=/src/$pack,readonly"
  --mount "type=bind,src=$ROOT/workplace/hd-inventory,dst=/src/workplace/hd-inventory,readonly"
  -w /src)
timeout 3m docker run "${common[@]}" --name san1-hd-duel-build \
  -e SAN1_HD_DUEL_OUT="$target" -e GOWORK=off -e GOMAXPROCS=2 \
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off \
  --entrypoint /bin/bash rich2-go-ebiten:latest -c '
    set -euo pipefail
    owner="$(id -u):$(id -g)"
    [[ "$(stat -c %u:%g workplace/hd-window/player)" == "$owner" ]] || exit 2
    [[ ! -e "$SAN1_HD_DUEL_OUT" ]] || exit 2
    mkdir "$SAN1_HD_DUEL_OUT"
    for edition in base plus; do
      out="$SAN1_HD_DUEL_OUT/$edition"
      mkdir "$out"
      go build -trimpath -o "$out/san1-window-check" ./cmd/san1
      go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$out/title-reference.png" >/dev/null
      go run tools/hd-battle-branches-reference.go -out "$out"
    done
    python3 -m py_compile tools/verify-hd-duel-inner.py
  '
for edition in base plus; do
  timeout 15m docker run "${common[@]}" --name "san1-hd-duel-$edition" \
    --entrypoint python3 eob-audio-capture:20260922-r2 \
    tools/verify-hd-duel-inner.py --edition "$edition" --out "/src/$target/$edition" \
    --hd-assets "/src/$pack"
done
