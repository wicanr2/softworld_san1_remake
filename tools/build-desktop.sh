#!/usr/bin/env bash
# 建立開發用執行檔；正式封包仍由 release.sh 管理。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_IMAGE="${SAN1_GO_IMAGE:-san1-go:1.26.7-ebiten2.9.9-r1}"
MAC_IMAGE="${SAN1_MAC_IMAGE:-san1-macos:1.26.7-sdk15.5-r1}"
case "${1:-linux-amd64}" in
  all) targets=(linux-amd64 windows-amd64 darwin-amd64 darwin-arm64) ;;
  linux-amd64|windows-amd64|darwin-amd64|darwin-arm64) targets=("${1:-linux-amd64}") ;;
  *) echo '用法：bash tools/build-desktop.sh [all|linux-amd64|windows-amd64|darwin-amd64|darwin-arm64]' >&2; exit 2 ;;
esac
if ! docker image inspect "$GO_IMAGE" >/dev/null 2>&1; then
  bash "$ROOT/tools/build-toolchain.sh" build
fi
bash "$ROOT/tools/prepare-go-cache.sh"
for target in "${targets[@]}"; do
  image="$GO_IMAGE"; cc=gcc; cgo=1; binary=san1
  case "$target" in
    windows-*) cgo=0; binary=san1.exe ;;
    darwin-*)
      image="$MAC_IMAGE"
      if ! docker image inspect "$image" >/dev/null 2>&1; then
        bash "$ROOT/tools/build-toolchain.sh" macos
      fi
      cc=o64-clang
      [[ "$target" != darwin-arm64 ]] || cc=oa64-clang
      ;;
  esac
  timeout "${SAN1_TIMEOUT:-15m}" docker run --rm --platform linux/amd64 --network none --name "san1-build-$target-$$" \
    --memory "${SAN1_MEM:-4g}" --cpus "${SAN1_CPUS:-2}" --pids-limit 256 \
    --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)" \
    --mount "type=bind,src=$ROOT,dst=/src,readonly" \
    --mount "type=bind,src=$ROOT/workplace,dst=/src/workplace" \
    -e HOME=/tmp -e GOWORK=off -e GOTOOLCHAIN=local \
    -e GOOS="${target%-*}" -e GOARCH="${target#*-}" -e CGO_ENABLED="$cgo" -e CC="$cc" \
    -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
    -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off \
    -e TARGET="$target" -e BINARY="$binary" -w /src "$image" bash -c '
      set -euo pipefail
      for path in /src/workplace /src/workplace/rebuild /src/workplace/rebuild/"$TARGET"; do
        if test -e "$path" && test "$(stat -c %u:%g "$path")" != "$(id -u):$(id -g)"; then
          echo "輸出擁有權不符：$path" >&2; exit 2
        fi
      done
      mkdir -p "/src/workplace/rebuild/$TARGET"
      go build -trimpath -buildvcs=false -o "/src/workplace/rebuild/$TARGET/$BINARY" ./cmd/san1
      file "/src/workplace/rebuild/$TARGET/$BINARY"
    '
done
