"""在已執行位址之前尋找 16-bit 函式 prologue，作一次性 raw DB 種子。

輸入位址必須來自動態執行證據。腳本只在前 0x400 bytes 內的 `55 8B EC`
候選建立函式，並輸出距離、bytes 與結果；prologue 命中只是函式邊界候選，
語意仍須由 caller、控制流與資料流證實。
"""

import hashlib
import json
import sys

import ida_auto
import ida_bytes
import ida_funcs
import ida_nalt
import ida_pro
import ida_segment
import idautils
import idc


ida_auto.auto_wait()
input_path = ida_nalt.get_input_file_path()
with open(input_path, "rb") as fh:
    input_sha256 = hashlib.sha256(fh.read()).hexdigest()

for segment_ea in idautils.Segments():
    segment = ida_segment.getseg(segment_ea)
    ida_segment.set_segm_addressing(segment, 0)

records = []
for spec in sys.argv[2:]:
    target = int(spec, 16)
    candidates = []
    start = max(0, target - 0x400)
    for ea in range(start, target + 1):
        if ida_bytes.get_bytes(ea, 3) != b"\x55\x8b\xec":
            continue
        raw = ida_bytes.get_bytes(ea, 12) or b""
        ida_bytes.del_items(ea, ida_bytes.DELIT_SIMPLE, 16)
        made_insn = idc.create_insn(ea) != 0
        made_func = bool(ida_funcs.add_func(ea))
        candidates.append({
            "ea": f"0x{ea:05X}",
            "distance_to_target": target - ea,
            "bytes_12": raw.hex(" "),
            "create_insn": made_insn,
            "add_func": made_func,
        })
    records.append({
        "target": f"0x{target:05X}",
        "candidates": candidates,
    })

ida_auto.auto_wait()
out = {
    "tool": "IDA Pro 9.4 IDAPython",
    "address_space": "runtime linear address mapped directly into raw DB",
    "input": input_path,
    "input_sha256": input_sha256,
    "records": records,
    "func_count_after": len(list(idautils.Functions())),
}
with open(sys.argv[1], "w", encoding="utf-8") as fh:
    json.dump(out, fh, ensure_ascii=False, indent=2)

ida_pro.qexit(0)
