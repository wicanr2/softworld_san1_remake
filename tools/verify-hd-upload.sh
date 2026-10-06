#!/usr/bin/env bash
# 兩版正常玩家路徑，比較完整與局部畫面上傳；證據只留本機。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${SAN1_HD_UPLOAD_OUT:-workplace/hd-upload-current}"
case "$OUT" in workplace/hd-upload-*) ;; *) echo '輸出須在 workplace/hd-upload-*' >&2; exit 2 ;; esac
case "/$OUT/" in */../*|*/./*) echo '輸出不得跳層' >&2; exit 2 ;; esac
for path in "$ROOT" "$ROOT/org_game" "$ROOT/workplace" "$ROOT/workplace/gocache" \
  "$ROOT/workplace/gomodcache" "$ROOT/workplace/hd-assets-mapcursor-v50-r1" "$ROOT/workplace/hd-inventory"; do
  test -d "$path" || { echo "缺少目錄：$path" >&2; exit 2; }
done
test ! -e "$ROOT/$OUT" || { echo '輸出已存在' >&2; exit 2; }
COMMON=(--rm --network none --memory 4g --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)"
  -v "$ROOT:/src" -v "$ROOT/org_game:/src/org_game:ro" -v "$ROOT/org_game:/orig:ro"
  -v "$ROOT/workplace/hd-inventory:/src/workplace/hd-inventory:ro"
  -v "$ROOT/workplace/hd-assets-mapcursor-v50-r1:/src/workplace/hd-assets-mapcursor-v50-r1:ro" -w /src)
timeout 4m docker run "${COMMON[@]}" --cpus 2 --name san1-hd-upload-build \
  -e GOWORK=off -e GOMAXPROCS=2 -e GOCACHE=/src/workplace/gocache \
  -e GOMODCACHE=/src/workplace/gomodcache -e GOPROXY=file:///src/workplace/gomodcache/cache/download \
  -e GOSUMDB=off --entrypoint python3 rich2-go-ebiten:latest \
  tools/verify-hd-upload-inner.py prepare --out "$OUT"
timeout 12m docker run "${COMMON[@]}" --cpus 8 --name san1-hd-upload-gui \
  -e LP_NUM_THREADS=2 --entrypoint python3 eob-audio-capture:20260922-r2 \
  tools/verify-hd-upload-inner.py play --out "$OUT"
