"""找「記錄大小」的常數：那是資料表存取的簽章。

`docs/formats/03` 量到劇本三表的記錄大小是 176（州郡）、30（人物）、
72（諸侯）。存取第 i 筆一定要算 `i * 記錄大小`，所以那幾個常數會出現在
索引計算裡——`mul`、`imul`，或是 shift/add 的組合。

⚠ **不要 grep .asm。** 那是攤平的文字、沒有交叉參考圖；
從呼叫端的參數順序反推是間接證據，會推錯。這裡直接查資料庫。
"""
import json
import sys
from collections import defaultdict

import ida_auto
import ida_bytes
import ida_funcs
import ida_pro
import ida_ua
import idautils
import idc

ida_auto.auto_wait()

# 目標常數與它們代表什麼（`docs/formats/03`）。
WANT = {
    176: "州郡記錄（BASESTA，43×176）",
    30: "人物記錄（BASEGEN，350×30）",
    72: "諸侯記錄（BASEMAS，16×72）",
    7568: "BASESTA 整段長度",
    10500: "BASEGEN 整段長度",
    1152: "BASEMAS 整段長度",
    42: "郡數",
    350: "人物槽數",
    16: "諸侯數",
}

hits = defaultdict(list)
for func_ea in idautils.Functions():
    for ea in idautils.FuncItems(func_ea):
        insn = ida_ua.insn_t()
        if ida_ua.decode_insn(insn, ea) == 0:
            continue
        mnem = insn.get_canon_mnem()
        for op in insn.ops:
            if op.type == ida_ua.o_void:
                break
            if op.type != ida_ua.o_imm:
                continue
            v = op.value & 0xFFFF
            if v in WANT:
                hits[v].append({
                    "ea": f"0x{ea:05X}",
                    "mnem": mnem,
                    "disasm": idc.GetDisasm(ea),
                    "func": ida_funcs.get_func_name(ea) or "",
                })

out = {}
for v, why in WANT.items():
    lst = hits.get(v, [])
    # mul/imul 的權重最高——那幾乎一定是索引計算。
    muls = [h for h in lst if h["mnem"] in ("mul", "imul")]
    out[str(v)] = {
        "meaning": why,
        "total": len(lst),
        "mul_count": len(muls),
        "mul_sites": muls[:12],
        "other_sites": [h for h in lst if h not in muls][:8],
    }

with open(sys.argv[1], "w", encoding="utf-8") as f:
    json.dump({
        "func_count": len(list(idautils.Functions())),
        "by_constant": out,
    }, f, ensure_ascii=False, indent=2)
ida_pro.qexit(0)
