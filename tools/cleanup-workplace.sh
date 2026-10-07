#!/usr/bin/env bash
# 工作目錄清理只在隔離容器內執行，預設不刪除。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test -d "$ROOT/workplace"
MOUNTS=()
for protected in org_game dist-all workplace/hd-assets-mapcursor-v50-r1; do
  if test -d "$ROOT/$protected"; then
    MOUNTS+=(--mount "type=bind,src=$ROOT/$protected,dst=/src/$protected,readonly")
  fi
done
exec timeout 5m docker run --rm --network none --memory 1g --cpus 2 --pids-limit 128 \
  --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)" \
  --mount "type=bind,src=$ROOT,dst=/src" "${MOUNTS[@]}" -w /src \
  rich2-go-ebiten:latest python3 tools/cleanup-workplace.py "$@" --root /src
