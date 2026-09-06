"""拿 dosgolem 的執行覆蓋率當種子，把 raw dump 展開成程式碼。

種子是**實際執行過的指令起點**，不是從結構猜的——所以不會種到資料上。
這是打包過的執行檔唯一可靠的入口：靜態反組譯是亂碼，
而 IDA 對 raw dump 不知道從哪裡開始。

⚠ 位址是線性的 `seg*16 + off`，與 `-b0x110` 放的位置一致。
⚠ 段要設 16 位元；IDA 對 raw binary 預設 32 位元，指令會整段解錯
   而且解得出來、不會報錯。
"""
import json
import sys

import ida_auto
import ida_bytes
import ida_funcs
import ida_pro
import ida_segment
import idautils
import idc

cov_path, out_path = sys.argv[1], sys.argv[2]

ida_auto.auto_wait()

for s in idautils.Segments():
    seg = ida_segment.getseg(s)
    if seg.bitness != 0:
        ida_segment.set_segm_addressing(seg, 0)

seg0 = ida_segment.getseg(next(iter(idautils.Segments())))
lo, hi = seg0.start_ea, seg0.end_ea

with open(cov_path, encoding="utf-8") as f:
    cov = json.load(f)

addrs = []
for sp in cov["spans"]:
    a, b = int(sp["start"], 16), int(sp["end"], 16)
    for ea in range(a, b):
        if lo <= ea < hi:
            addrs.append(ea)

made = 0
for ea in addrs:
    if ida_bytes.is_code(ida_bytes.get_flags(ea)):
        continue
    ida_bytes.del_items(ea, ida_bytes.DELIT_SIMPLE, 1)
    if idc.create_insn(ea):
        made += 1

ida_auto.auto_wait()

# 再讓每個種子成為函式起點的候選——只在它是某個 call 的目標時才成立，
# 所以這裡不強制建函式，交給自動分析。
funcs = list(idautils.Functions())

code_bytes = 0
ea = lo
while ea < hi:
    if ida_bytes.is_code(ida_bytes.get_flags(ea)):
        code_bytes += 1
    ea += 1

with open(out_path, "w", encoding="utf-8") as f:
    json.dump({
        "seg": {"start": hex(lo), "end": hex(hi), "bitness": seg0.bitness},
        "seeds_in_range": len(addrs),
        "insns_created": made,
        "func_count": len(funcs),
        "code_bytes": code_bytes,
        "code_ratio": round(code_bytes / (hi - lo), 3),
    }, f, ensure_ascii=False, indent=2)
ida_pro.qexit(0)
