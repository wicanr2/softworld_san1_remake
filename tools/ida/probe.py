"""三十秒驗證探針：確認這顆 image 的 IDAPython 真的跑得起來。

**不要靠 exit code。** 唯一可信的訊號是輸出檔存在、非空、欄位對。
輸出帶輸入檔的 SHA-256，這樣「沒找到」與「沒跑到」分得開。
"""
import hashlib
import json
import sys

import ida_auto
import ida_nalt
import ida_pro
import idautils

ida_auto.auto_wait()

path = ida_nalt.get_input_file_path()
try:
    with open(path, "rb") as f:
        sha = hashlib.sha256(f.read()).hexdigest()
except OSError:
    sha = "(讀不到輸入檔)"

funcs = list(idautils.Functions())
out = {
    "input": path,
    "sha256": sha,
    "func_count": len(funcs),
    "first_funcs": [hex(e) for e in funcs[:5]],
    "seg_count": len(list(idautils.Segments())),
}
with open(sys.argv[1], "w", encoding="utf-8") as f:
    json.dump(out, f, ensure_ascii=False, indent=2)

ida_pro.qexit(0)
