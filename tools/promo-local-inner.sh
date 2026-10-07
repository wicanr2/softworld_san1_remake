#!/usr/bin/env bash
# 合成與媒體驗收在容器執行，暫存段落只存在 /tmp。
set -euo pipefail
exec python3 tools/promo-assemble-inner.py "$@"
