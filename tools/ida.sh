#!/usr/bin/env bash
# IDA Pro 9.4 headless 包裝器。
#
#   tools/ida.sh analyze <輸入檔>              產 .i64
#   tools/ida.sh script  <.i64> <腳本> [參數…] 跑 IDAPython
#   tools/ida.sh raw     <idat 的參數…>        直接下命令
#
# ⚠ **exit code 不能當證據。** 同一種「沒有輸出」的失敗在不同 image 上
# 分別回 rc=0 與 rc=1。唯一可信的訊號是輸出檔本身——所以每支腳本都要
# 寫檔，而且輸出要帶 probe（函式數、輸入檔 SHA-256），
# 否則分不出「沒找到」與「沒跑到」。
#
# ⚠ **腳本崩掉會把 .i64 留在不可開啟的狀態**，之後任何指令都回
# 「Failed to initialize IDA as library (error code 4)」——那訊息讀起來
# 像授權失效或 image 壞掉。判斷法：拿另一個 .i64 跑同一支已知可用的腳本；
# 只有那一個壞就刪掉重跑 analyze。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${SAN1_IDA_IMAGE:-ida-pro-9.4-idapython:locked-v1}"
WORK="${SAN1_IDA_WORK:-$ROOT/workplace/ida}"
mkdir -p "$WORK"

run() {
  # idapyswitch 把選定的 interpreter 寫進 $HOME/.idapro，所以身分要一致；
  # locked-v1 已經以最終 UID 修好，這裡照樣傳 -u 保持一致。
  exec timeout "${SAN1_IDA_TIMEOUT:-30m}" docker run --rm --network none \
    --memory "${SAN1_IDA_MEM:-6g}" --cpus "${SAN1_IDA_CPUS:-2}" --pids-limit 256 \
    --log-opt max-size=10m --log-opt max-file=3 \
    -u "$(id -u):$(id -g)" \
    -v "$WORK:/work" \
    -v "$ROOT/tools/ida:/work/tools:ro" \
    -w /work "$IMAGE" "$@"
}

cmd="${1:-}"; shift || true
case "$cmd" in
  analyze) run idat -A -B "$@" ;;
  script)
    db="$1"; shift
    script="$1"; shift
    run idat -A "-S/work/tools/$script $*" "$db"
    ;;
  raw) run "$@" ;;
  *) echo "用法：tools/ida.sh {analyze|script|raw} …" >&2; exit 2 ;;
esac
