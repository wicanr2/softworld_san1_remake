#!/usr/bin/env bash
# 快取被移除後，以專案鎖定的 go.mod／go.sum 在 Docker 補回依賴。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
for directory in "$ROOT" "$ROOT/workplace"; do test -d "$directory"; done
for file in go.mod go.sum; do test -f "$ROOT/$file"; done
exec timeout 5m docker run --rm --network bridge --memory 2g --cpus 2 --pids-limit 128 \
  --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)" \
  --mount "type=bind,src=$ROOT,dst=/src,readonly" \
  --mount "type=bind,src=$ROOT/workplace,dst=/src/workplace" \
  -e HOME=/tmp -e GOWORK=off -e GOTOOLCHAIN=local -e GOCACHE=/tmp/gocache \
  -e GOMODCACHE=/src/workplace/gomodcache -e GOPROXY=https://proxy.golang.org \
  -e GOSUMDB=sum.golang.org -w /tmp "${SAN1_GO_IMAGE:-rich2-go-ebiten:latest}" bash -c '
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
