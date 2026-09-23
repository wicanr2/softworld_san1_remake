#!/usr/bin/env bash
# 從已驗收的公開引擎包建立含玩家本機原版資料的私人封包。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ $# -ne 1 || ! "$1" =~ ^v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}$ ]]; then
  echo '用法：tools/full-local.sh v.<主版>.<次版>.<修訂版>-YYYYMMDD' >&2
  exit 2
fi
VER="$1"
IMAGE="eob-remake-release:1.26.7-ebiten2.9.9-audio"
for d in "$ROOT" "$ROOT/org_game" "$ROOT/dist-all/$VER" "$ROOT/dist-all/$VER/patch" "$ROOT/workplace"; do
  [[ -d "$d" ]] || { echo "缺少掛載目錄：$d" >&2; exit 1; }
done
for d in "$ROOT/org_game/三國演義" "$ROOT/org_game/三國演義1加強版"; do
  [[ -d "$d" ]] || { echo "缺少原版目錄：$d" >&2; exit 1; }
done
[[ -f "$ROOT/tools/full-local-inner.py" ]] || exit 1
docker image inspect "$IMAGE" >/dev/null
timeout 10m docker run --rm --network none --memory 2g --cpus 2 --pids-limit 128 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" \
  -v "$ROOT:/src" -v "$ROOT/org_game:/src/org_game:ro" \
  -e HOME=/tmp -w /src "$IMAGE" python3 tools/full-local-inner.py "$VER"
