"""在已知**實際執行過**的位址種下程式碼，再讓 IDA 自動分析展開。

raw binary 沒有進入點，IDA 會把整段當資料——`func_count` 是 0，
那不是「沒有函式」，是「沒被分析成程式碼」。兩者長得一樣。

⚠ **位址換算要以 IDA 實際放的位置為準，不要憑基底相減。**
第一版寫成 `(seg - 基底) * 16 + off`，但 `-b0x110` 是把映像放在
線性 `0x1100`，正確的換算是 `seg * 16 + off`。錯的算法會讓一部分種子
落在映像外（看得出來），另一部分落在**錯的位置卻種成功**（看不出來）——
後者更糟，它會在資料上造出假的程式碼。

⚠ **16 位元實模式的段要設成 16-bit。** IDA 對 raw binary 預設 32 位元，
指令會整段解錯，而且解得出來、不會報錯。
"""
import json
import sys

import ida_auto
import ida_bytes
import ida_funcs
import ida_pro
import ida_segment
import ida_segregs
import idautils
import idc

ida_auto.auto_wait()

# 1. 把段設成 16 位元。
for s in idautils.Segments():
    seg = ida_segment.getseg(s)
    if seg.bitness != 0:
        ida_segment.set_segm_addressing(seg, 0)  # 0 = 16-bit

segs = [{
    "name": ida_segment.get_segm_name(ida_segment.getseg(s)),
    "start": hex(ida_segment.getseg(s).start_ea),
    "end": hex(ida_segment.getseg(s).end_ea),
    "bitness": ida_segment.getseg(s).bitness,
} for s in idautils.Segments()]

# 2. 種子：dosgolem 觀測到**實際執行過**的 seg:off。
#    線性位址 ＝ seg * 16 + off（見上面的警告）。
SEEDS = [
    (0x0110, 0x0000, "overlay 進入點"),
    (0x0110, 0x0E8F, "繪圖迴圈"),
    (0x0583, 0x04B6, "MSC exit()"),
    (0x0583, 0x0546, "null-pointer 檢查"),
    (0x0583, 0x3792, "開場插圖的等待迴圈"),
    (0x0AD0, 0x00FB, "被 0583 呼叫的常式"),
]

made = []
for seg, off, why in SEEDS:
    ea = seg * 16 + off
    rec = {"ea": hex(ea), "from": f"{seg:04X}:{off:04X}", "why": why}
    if not ida_bytes.is_loaded(ea):
        rec["result"] = "位址不在映像內"
    else:
        ida_bytes.del_items(ea, ida_bytes.DELIT_SIMPLE, 16)
        rec["create_insn"] = idc.create_insn(ea) != 0
        rec["add_func"] = bool(ida_funcs.add_func(ea))
    made.append(rec)

ida_auto.auto_wait()

funcs = list(idautils.Functions())
out = {
    "segments": segs,
    "seeds": made,
    "func_count_after": len(funcs),
    "funcs_sample": [hex(e) for e in funcs[:12]],
}
with open(sys.argv[1], "w", encoding="utf-8") as f:
    json.dump(out, f, ensure_ascii=False, indent=2)
ida_pro.qexit(0)
