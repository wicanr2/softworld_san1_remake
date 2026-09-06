"""把指定位址所屬的函式反組譯出來（含每行的位址）。

⚠ 查資料庫不 grep `.asm`：`.asm` 沒有交叉參考圖，
從呼叫端參數順序反推是間接證據。
"""
import json
import sys

import ida_auto
import ida_funcs
import ida_pro
import idautils
import idc

ida_auto.auto_wait()
out = {}
for spec in sys.argv[2:]:
    ea = int(spec, 16)
    f = ida_funcs.get_func(ea)
    if f is None:
        out[spec] = {"error": "不在任何函式內"}
        continue
    lines = []
    for item in idautils.FuncItems(f.start_ea):
        lines.append(f"{item:05X}  {idc.GetDisasm(item)}")
    out[spec] = {
        "func_start": f"0x{f.start_ea:05X}",
        "func_end": f"0x{f.end_ea:05X}",
        "name": ida_funcs.get_func_name(f.start_ea),
        "callers": [f"0x{x.frm:05X}" for x in idautils.XrefsTo(f.start_ea) if x.iscode][:10],
        "lines": lines,
    }
with open(sys.argv[1], "w", encoding="utf-8") as fh:
    json.dump(out, fh, ensure_ascii=False, indent=2)
ida_pro.qexit(0)
