#!/usr/bin/env bash
# Go 走 docker，不裝到系統環境。
#
#   tools/go.sh build ./...
#   tools/go.sh test ./internal/assets -v
#
# 原版素材要給容器看時用 SAN1_ORIG 指到素材目錄，**唯讀**掛到 /orig：
#
#   SAN1_ORIG=$PWD/org_game tools/go.sh test ./internal/assets -tags orig
#
# 本儲存庫不含任何原版檔案；掛載一律 ro，容器也不會把它們帶出去。
# module cache 放 workplace/（gitignore），避免每次重抓。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# Ebiten 要連結 X11／GL，純 golang image 沒有那些標頭（會停在
# 「fatal error: X11/Xlib.h: No such file or directory」）。
# 用旁邊 remake 專案已經建好的 ebiten image。
IMAGE="${SAN1_GO_IMAGE:-rich2-go-ebiten:latest}"

mkdir -p "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"

PASS=()
# 對拍測試讀的環境變數要一起帶進容器。**漏一個不會報錯**——
# 測試看到空字串就當「沒有設」，該存的圖或該倒的表安靜地不存。
for v in GOOS GOARCH CGO_ENABLED SAN1_ORIG_DIR SAN1_SHOTS SAN1_DUMP \
         SAN1_TURNS SAN1_TURNKEY; do
  [[ -n "${!v:-}" ]] && PASS+=(-e "$v=${!v}")
done

# 素材在旁邊就預設掛進去。
#
# **沒掛的時候需要素材的測試會 skip 不會紅**，於是 `go test ./...` 印出
# 一片 ok，而真正在跑的只有不碰原版的那些——「全綠」因此是假的。
# 要刻意跑沒有素材的那一組，設 SAN1_ORIG= （空字串）。
if [[ -z "${SAN1_ORIG+x}" && -d "$ROOT/org_game" ]]; then
  SAN1_ORIG="$ROOT/org_game"
  echo "tools/go.sh：自動掛上 org_game（要跑無素材的那一組請設 SAN1_ORIG=）" >&2
fi

MOUNTS=()
if [[ -n "${SAN1_ORIG:-}" ]]; then
  MOUNTS+=(-v "$(cd "$SAN1_ORIG" && pwd):/orig:ro")
  PASS+=(-e "SAN1_ORIG=/orig")
fi

# dosgolem 在旁邊就唯讀掛進去，讓 -tags oracle 的對拍測試跑得起來。
# 沒有這份掛載的人 go test ./... 會 skip，不會紅。
if [[ -d "$ROOT/../dosgolem-san" ]]; then
  MOUNTS+=(-v "$(cd "$ROOT/../dosgolem-san" && pwd):/dosgolem:ro")
fi

# `go test` 自己有一個**十分鐘**的預設逾時，外層的 SAN1_TIMEOUT 管不到它。
# 兩個數字不一致時，長跑會在第十分鐘被 `panic: test timed out` 砍掉，
# 而外層看起來只是「測試失敗」——所以這裡把兩邊對齊。
ARGS=("$@")
if [[ "${1:-}" == "test" ]] && [[ ! " $* " == *" -timeout"* ]]; then
  ARGS=("test" "-timeout" "${SAN1_TIMEOUT:-30m}" "${@:2}")
fi

# GOPROXY 指向容器內的本地 module cache。GOPROXY=off 連本地 cache 都不查，
# 會在「zip 明明就在那裡」的情況下說找不到模組；file:// 讓解析走本地，
# 而且一樣零網路——容器本來就是 --network none。
exec timeout "${SAN1_TIMEOUT:-30m}" docker run --rm --network none \
  --memory "${SAN1_MEM:-4g}" --cpus "${SAN1_CPUS:-2}" --pids-limit 256 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" \
  -v "$ROOT:/src" \
  -v "$ROOT/workplace/gocache:/gocache" \
  -v "$ROOT/workplace/gomodcache:/gomodcache" \
  -e GOCACHE=/gocache -e GOMODCACHE=/gomodcache \
  -e GOPROXY=file:///gomodcache/cache/download \
  -e GOSUMDB=off -e GONOSUMCHECK=1 -e GOPRIVATE='*' \
  -e HOME=/tmp \
  "${PASS[@]}" "${MOUNTS[@]}" -w /src "$IMAGE" go "${ARGS[@]}"
