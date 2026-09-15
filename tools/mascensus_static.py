#!/usr/bin/env python3
"""靜態補掃：碼段裡還有哪些 `es:[reg+N]`（N 在諸侯記錄的未解位移）。

用法（在有 objdump 的容器裡）：

    python3 tools/mascensus_static.py <census.json> <code-00b000.bin> [--vma 0xb000]

動態普查（`TestZZMasterRecordAccessCensus`）只看得到跑過的碼。這裡拿
同一次執行倒出的碼段做線性反組譯，找所有帶 `es:` 段覆寫、基底暫存器加
一個小位移的存取，位移落在**普查沒碰過的位移**上的都列出來，並往回找
`ES` 是從哪個 DGROUP 變數載入的。

判準：ES 若來自動態命中點用過的同一批 DGROUP 變數（三張表的段值就放在
那幾格），這一筆就是「有讀寫端、語意未定」的候選；ES 來源不同或找不到，
列出來但標成「ES 來源不明」，不當成諸侯表的存取。

⚠ 線性反組譯在資料區會錯位、在跨段處會解出假指令。這是**補掃不是證明**：
輸出的候選要逐一回去看上下文才算數。
"""
import json
import re
import subprocess
import sys
from collections import defaultdict

MEM = re.compile(r"es:\[(bx|si|di|bp|bx\+si|bx\+di|bp\+si|bp\+di)(?:\+0x([0-9a-f]+))?\]")
ES_LOAD = re.compile(r"^(mov\s+es,WORD PTR (?:ds:)?\[?0x([0-9a-f]+)\]?|les\s+\w+,(?:WORD PTR )?(?:ds:)?\[?0x([0-9a-f]+)\]?)")


def disasm_all(code, vma):
    out = subprocess.run(
        ["objdump", "-b", "binary", "-m", "i8086", "-M", "intel", "-D",
         f"--adjust-vma={vma:#x}", code],
        capture_output=True, text=True, check=True).stdout
    rows = []
    for line in out.splitlines():
        m = re.match(r"\s*([0-9a-f]+):\s+((?:[0-9a-f]{2} )+)\s*(.*)$", line)
        if not m:
            continue
        rows.append((int(m.group(1), 16), m.group(2).strip(), m.group(3).strip()))
    return rows


def es_source(rows, i, back=14):
    """往回找最近一次載入 ES 的指令，回 (DGROUP 位移, 指令文字)。"""
    for j in range(i - 1, max(-1, i - back), -1):
        txt = rows[j][2]
        m = ES_LOAD.match(txt)
        if m:
            off = m.group(2) or m.group(3)
            return int(off, 16), txt
        if txt.startswith(("ret", "retf", "jmp", "call")):
            break
    return None, None


def main():
    args = sys.argv[1:]
    vma = 0xB000
    if "--vma" in args:
        i = args.index("--vma")
        vma = int(args[i + 1], 16)
        del args[i:i + 2]
    census_path, code = args[0], args[1]
    c = json.load(open(census_path, encoding="utf-8"))

    touched = {int(k) for k in c["by_offset"]}
    unknown = [o for o in range(72) if o not in touched]
    rows = disasm_all(code, vma)
    end_index = {a + len(b.split()): i for i, (a, b, _) in enumerate(rows)}

    # 動態命中點的 ES 來源：那幾個 DGROUP 變數放的是三張表的段值。
    table_seg_vars = defaultdict(int)
    for key, s in c["sites"].items():
        i = end_index.get(s["linear"])
        if i is None:
            continue
        off, _ = es_source(rows, i)
        if off is not None:
            table_seg_vars[off] += 1
    print(f"# 靜態補掃：{c['edition']}（{c['exe']}）")
    print()
    print("動態命中點往回找到的 ES 來源（DGROUP 位移 → 命中點數）：")
    for off, n in sorted(table_seg_vars.items()):
        print(f"- `0x{off:04x}`：{n}")
    print()

    cands = defaultdict(list)
    for i, (a, b, txt) in enumerate(rows):
        m = MEM.search(txt)
        if not m:
            continue
        disp = int(m.group(2), 16) if m.group(2) else 0
        if disp not in unknown:
            continue
        src, srctxt = es_source(rows, i)
        tag = "表段變數" if src in table_seg_vars else ("ES 來源不明" if src is None else f"ES 來自 0x{src:04x}")
        cands[disp].append((a, txt, tag, srctxt))

    print("| 位移 | 線性位址 | 指令 | ES 來源 |")
    print("|---:|---|---|---|")
    strong = defaultdict(int)
    for disp in sorted(cands):
        for a, txt, tag, srctxt in cands[disp]:
            if tag == "表段變數":
                strong[disp] += 1
            print(f"| {disp} | `0x{a:05x}` | `{txt}` | {tag}{'：`'+srctxt+'`' if srctxt else ''} |")
    print()
    print(f"未解位移 {len(unknown)} 個；有 `es:[reg+N]` 形狀的 {len(cands)} 個；"
          f"其中 ES 確定來自表段變數的 {len(strong)} 個：{sorted(strong)}")
    print(f"連形狀都沒有的位移：{[o for o in unknown if o not in cands]}")


if __name__ == "__main__":
    main()
