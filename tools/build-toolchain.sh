#!/usr/bin/env bash
# 主機只控制 Docker；不依賴其他專案或主機 Go。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_IMAGE="${SAN1_GO_IMAGE:-san1-go:1.26.7-ebiten2.9.9-r1}"
MAC_IMAGE="${SAN1_MAC_IMAGE:-san1-macos:1.26.7-sdk15.5-r1}"
case "${1:-build}" in
  build)
    test -f "$ROOT/docker/Dockerfile.build"
    test -f "$ROOT/go.mod" && test -f "$ROOT/go.sum"
    exec timeout 20m docker build --platform linux/amd64 --network default --memory 4g \
      --cpu-period 100000 --cpu-quota 200000 \
      -f "$ROOT/docker/Dockerfile.build" -t "$GO_IMAGE" "$ROOT"
    ;;
  macos)
    if ! docker image inspect "$GO_IMAGE" >/dev/null 2>&1; then
      bash "$ROOT/tools/build-toolchain.sh" build
    fi
    exec timeout 20m docker build --platform linux/amd64 --network default --memory 4g \
      --cpu-period 100000 --cpu-quota 200000 \
      --build-arg "SAN1_GO_BASE=$GO_IMAGE" \
      -f "$ROOT/docker/Dockerfile.macos" -t "$MAC_IMAGE" "$ROOT"
    ;;
  *) echo '用法：bash tools/build-toolchain.sh [build|macos]' >&2; exit 2 ;;
esac
