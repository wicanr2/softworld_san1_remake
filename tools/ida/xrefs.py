"""匯出指定位址的 IDA 交叉參照與呼叫端脈絡。

輸出保留原始位址、bytes、operand、函式邊界與輸入 SHA-256；語意判讀由
受版控研究文件另行分級，不用推測性改名覆蓋資料庫定位資訊。
"""

import hashlib
import json
import sys

import ida_auto
import ida_bytes
import ida_funcs
import ida_loader
import ida_nalt
import ida_pro
import idautils
import idc


ida_auto.auto_wait()

input_path = ida_nalt.get_input_file_path()
with open(input_path, "rb") as fh:
    input_sha256 = hashlib.sha256(fh.read()).hexdigest()


def context(ea, radius=3):
    """回傳 ea 前後固定筆數的原始反組譯行。"""
    first = ea
    for _ in range(radius):
        prev = idc.prev_head(first)
        if prev == idc.BADADDR:
            break
        first = prev
    lines = []
    cur = first
    for _ in range(radius * 2 + 1):
        if cur == idc.BADADDR:
            break
        raw = ida_bytes.get_bytes(cur, ida_bytes.get_item_size(cur)) or b""
        lines.append({
            "ea": f"0x{cur:05X}",
            "bytes": raw.hex(" "),
            "disasm": idc.GetDisasm(cur),
        })
        cur = idc.next_head(cur)
    return lines


targets = {}
for spec in sys.argv[2:]:
    if spec.startswith("file:"):
        file_offset = int(spec.removeprefix("file:"), 16)
        ea = ida_loader.get_fileregion_ea(file_offset)
        if ea == idc.BADADDR:
            targets[spec] = {
                "error": f"檔案偏移 0x{file_offset:X} 沒有映射到 IDA EA",
            }
            continue
    else:
        ea = int(spec, 16)
        file_offset = ida_loader.get_fileregion_offset(ea)
    refs = []
    for xref in idautils.XrefsTo(ea):
        func = ida_funcs.get_func(xref.frm)
        refs.append({
            "from": f"0x{xref.frm:05X}",
            "to": f"0x{xref.to:05X}",
            "type": int(xref.type),
            "iscode": bool(xref.iscode),
            "function": None if func is None else {
                "start": f"0x{func.start_ea:05X}",
                "end": f"0x{func.end_ea:05X}",
                "original_name": ida_funcs.get_func_name(func.start_ea),
            },
            "context": context(xref.frm),
        })
    raw = ida_bytes.get_bytes(ea, 16) or b""
    targets[spec] = {
        "ea": f"0x{ea:05X}",
        "file_offset": None if file_offset < 0 else f"0x{file_offset:X}",
        "original_name": idc.get_name(ea),
        "bytes_16": raw.hex(" "),
        "xrefs": refs,
    }

out = {
    "tool": "IDA Pro 9.4 IDAPython",
    "address_space": "IDA linear effective address",
    "input": input_path,
    "input_sha256": input_sha256,
    "targets": targets,
}
with open(sys.argv[1], "w", encoding="utf-8") as fh:
    json.dump(out, fh, ensure_ascii=False, indent=2)

ida_pro.qexit(0)
