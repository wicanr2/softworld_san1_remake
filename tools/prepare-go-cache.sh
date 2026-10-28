#!/usr/bin/env bash
# 由鎖版工具鏈的模組來源離線恢復快取，依 go.mod／go.sum 核對。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test -d "$ROOT"
for file in go.mod go.sum; do test -f "$ROOT/$file"; done
IMAGE="${SAN1_GO_IMAGE:-san1-go:1.26.7-ebiten2.9.9-r1}"
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  bash "$ROOT/tools/build-toolchain.sh" build
fi
exec timeout 5m docker run --rm --platform linux/amd64 --network none --name "san1-cache-$$" --memory 2g --cpus 2 --pids-limit 128 \
  --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)" \
  --mount "type=bind,src=$ROOT,dst=/src" \
  -e HOME=/tmp -e GOWORK=off -e GOTOOLCHAIN=local -e GOCACHE=/tmp/gocache \
  -e GOMODCACHE=/src/workplace/gomodcache -e GOPROXY=file:///go/pkg/mod/cache/download \
  -e GOSUMDB=off -w /tmp "$IMAGE" bash -c '
    set -euo pipefail
    for path in /src/workplace /src/workplace/gomodcache /src/workplace/gocache; do
      if test -e "$path" && test "$(stat -c %u:%g "$path")" != "$(id -u):$(id -g)"; then
        echo "快取擁有權不符：$path" >&2; exit 2
      fi
    done
    mkdir -p /src/workplace/gomodcache /src/workplace/gocache
    input=$(mktemp -d /tmp/san1-go-dependencies.XXXXXX)
    trap '\''rm -rf "$input"'\'' EXIT
    cp /src/go.mod /src/go.sum "$input/"
    cd "$input"
    go mod download
    cmp /src/go.mod go.mod
    cmp /src/go.sum go.sum
    go mod verify
    test "$(stat -c %u:%g /src/workplace/gomodcache)" = "$(id -u):$(id -g)"
  '
