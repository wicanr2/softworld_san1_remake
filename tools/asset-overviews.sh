#!/usr/bin/env bash
# 重生 README 的兩張完整素材總覽。原版單張圖只寫到 gitignore 的 workplace。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIGINAL="${SAN1_ORIG:-$ROOT/org_game}"
GO_IMAGE="${SAN1_GO_IMAGE:-eob-remake-go:1.26.7-ebiten2.9.9-20261008-r2}"
ART_IMAGE="${SAN1_ART_IMAGE:-eob-audio-capture:20261009-r3}"
for p in "$ROOT/workplace" "$ROOT/hd-assets" "$ROOT/docs/images" \
    "$ORIGINAL/三國演義" "$ORIGINAL/三國演義1加強版"; do
    test -d "$p" || { echo "缺少目錄：$p" >&2; exit 1; }
done
docker image inspect "$GO_IMAGE" "$ART_IMAGE" >/dev/null
timeout 180s docker run --rm --name "san1-overviews-export-$$" --network none \
    --memory 3g --cpus 2 --pids-limit 256 \
    --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)" \
    --mount "type=bind,src=$ROOT,dst=/src,readonly" \
    --mount "type=bind,src=$ROOT/workplace,dst=/src/workplace" \
    --mount "type=bind,src=$ORIGINAL,dst=/orig,readonly" \
    -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
    -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off -e GOWORK=off \
    -w /src --entrypoint bash "$GO_IMAGE" -c '
        set -euo pipefail
        python3 -c "import os; from pathlib import Path; p=Path(\"/src/workplace\"); assert p.stat().st_uid==os.getuid(); assert all(f.stat().st_uid==os.getuid() for f in (p/\"asset-overviews\").rglob(\"*\"))"
        go run ./cmd/san1assets -root /orig/三國演義 -what img -out workplace/asset-overviews/base
        go run ./cmd/san1assets -root /orig/三國演義1加強版 -what img -out workplace/asset-overviews/plus
        go run tools/asset-overview-tiles.go /orig workplace/asset-overviews
    '
timeout 180s docker run --rm --name "san1-overviews-render-$$" --network none \
    --memory 2g --cpus 2 --pids-limit 128 \
    --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)" \
    --mount "type=bind,src=$ROOT,dst=/src,readonly" \
    --mount "type=bind,src=$ROOT/docs/images,dst=/src/docs/images" \
    --entrypoint python3 "$ART_IMAGE" /src/tools/render-asset-overviews.py
