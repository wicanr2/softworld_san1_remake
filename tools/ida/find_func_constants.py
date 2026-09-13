"""找同一函式內同時出現指定立即數的候選。

用法：find_func_constants.py <輸出.json> <常數>...。常數接受 Python 的
`0x` 表示法或十進位；也可寫 `add=0x50`，要求該立即數出現在特定
mnemonic。這只是候選縮小器；輸出保留原始位址、bytes 與 operand，常數
同現本身不作語意證據。
"""

import hashlib
import json
import sys

import ida_auto
import ida_bytes
import ida_funcs
import ida_nalt
import ida_pro
import ida_ua
import idautils
import idc


ida_auto.auto_wait()
requirements = []
for token in sys.argv[2:]:
    if "=" in token:
        mnemonic, value = token.split("=", 1)
    else:
        mnemonic, value = None, token
    number = int(value, 0) & 0xFFFF
    label = f"{mnemonic or '*'}=0x{number:X}"
    requirements.append((label, mnemonic, number))
wanted_labels = {item[0] for item in requirements}
input_path = ida_nalt.get_input_file_path()
with open(input_path, "rb") as fh:
    input_sha256 = hashlib.sha256(fh.read()).hexdigest()

matches = []
for func_ea in idautils.Functions():
    seen = set()
    lines = []
    for ea in idautils.FuncItems(func_ea):
        insn = ida_ua.insn_t()
        if ida_ua.decode_insn(insn, ea) == 0:
            continue
        values = set()
        for op in insn.ops:
            if op.type == ida_ua.o_void:
                break
            if op.type == ida_ua.o_imm:
                values.add(op.value & 0xFFFF)
        hit = {
            label for label, mnemonic, number in requirements
            if number in values and (mnemonic is None or mnemonic == insn.get_canon_mnem())
        }
        if not hit:
            continue
        seen.update(hit)
        raw = ida_bytes.get_bytes(ea, ida_bytes.get_item_size(ea)) or b""
        lines.append({
            "ea": f"0x{ea:05X}",
            "bytes": raw.hex(" "),
            "requirements": sorted(hit),
            "disasm": idc.GetDisasm(ea),
        })
    if seen == wanted_labels:
        func = ida_funcs.get_func(func_ea)
        matches.append({
            "start": f"0x{func.start_ea:05X}",
            "end": f"0x{func.end_ea:05X}",
            "original_name": ida_funcs.get_func_name(func.start_ea),
            "matching_lines": lines,
        })

out = {
    "tool": "IDA Pro 9.4 IDAPython",
    "address_space": "IDA linear effective address",
    "input": input_path,
    "input_sha256": input_sha256,
    "wanted": sorted(wanted_labels),
    "match_count": len(matches),
    "matches": matches,
}
with open(sys.argv[1], "w", encoding="utf-8") as fh:
    json.dump(out, fh, ensure_ascii=False, indent=2)

ida_pro.qexit(0)
