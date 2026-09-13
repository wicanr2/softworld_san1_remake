"""從已證實的指令邊界做一次性 16-bit 線性解碼。

適用於 runtime raw dump 中已有動態入口或返回位址、但 IDA 尚未建立函式的
區段。輸出只作導航；分支與函式邊界仍須另以動態路標或 xref 證實。
"""

import hashlib
import json
import sys

import ida_auto
import ida_bytes
import ida_nalt
import ida_pro
import ida_segment
import ida_ua
import idautils
import idc


ida_auto.auto_wait()
start = int(sys.argv[2], 16)
end = int(sys.argv[3], 16)
input_path = ida_nalt.get_input_file_path()
with open(input_path, "rb") as fh:
    input_sha256 = hashlib.sha256(fh.read()).hexdigest()

for segment_ea in idautils.Segments():
    ida_segment.set_segm_addressing(ida_segment.getseg(segment_ea), 0)

lines = []
ea = start
while ea < end:
    ida_bytes.del_items(ea, ida_bytes.DELIT_SIMPLE, 16)
    size = idc.create_insn(ea)
    if not size:
        lines.append({
            "ea": f"0x{ea:05X}",
            "bytes": (ida_bytes.get_bytes(ea, 1) or b"").hex(" "),
            "error": "無法解碼；前進 1 byte",
        })
        ea += 1
        continue
    raw = ida_bytes.get_bytes(ea, size) or b""
    lines.append({
        "ea": f"0x{ea:05X}",
        "bytes": raw.hex(" "),
        "disasm": idc.GetDisasm(ea),
    })
    ea += size

out = {
    "tool": "IDA Pro 9.4 IDAPython",
    "address_space": "runtime linear address mapped directly into raw DB",
    "boundary_status": "start supplied from dynamic evidence; this script does not prove a function boundary",
    "input": input_path,
    "input_sha256": input_sha256,
    "start": f"0x{start:05X}",
    "end": f"0x{end:05X}",
    "lines": lines,
}
with open(sys.argv[1], "w", encoding="utf-8") as fh:
    json.dump(out, fh, ensure_ascii=False, indent=2)

ida_pro.qexit(0)
