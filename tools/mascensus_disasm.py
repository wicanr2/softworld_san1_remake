#!/usr/bin/env python3
"""把諸侯表存取普查（`TestZZMasterRecordAccessCensus`）的每個存取端對回指令。

用法（在有 objdump 的容器裡）：

    python3 tools/mascensus_disasm.py <census.json> <code-00b000.bin> [--vma 0xb000]

普查記的 `linear` 是**讀寫發生那一刻的 CS:IP**，而 CPU 在存取記憶體時
IP 已經跨過該指令的 opcode／modrm／位移——所以它通常是「存取指令的下一道」
的位址；帶立即值的形式（`imul ax, es:[bx+2], 0x1e`）讀記憶體時立即值還沒
取，IP 停在指令結尾前一個位元組。這裡對每個存取端反組譯前後一小段，
挑出**該位址落在其範圍內（不含起點）**、帶 `es:` 的那一道指令。

輸出是 markdown 表：位移、讀寫、線性位址、指令、命中次數、槽數。
"""
import json
import re
import subprocess
import sys
from collections import defaultdict


def disasm(code, vma, lo, hi):
    """objdump 一段，回 [(addr, bytes, text)]。"""
    out = subprocess.run(
        ["objdump", "-b", "binary", "-m", "i8086", "-M", "intel", "-D",
         f"--adjust-vma={vma:#x}", f"--start-address={lo:#x}",
         f"--stop-address={hi:#x}", code],
        capture_output=True, text=True, check=True).stdout
    rows = []
    for line in out.splitlines():
        m = re.match(r"\s*([0-9a-f]+):\s+((?:[0-9a-f]{2} )+)\s*(.*)$", line)
        if not m:
            continue
        rows.append((int(m.group(1), 16), m.group(2).strip(), m.group(3).strip()))
    return rows


def instruction_ending_at(code, vma, end):
    """從幾個不同的起點反組譯，找結尾＝end 的那一道指令。

    線性掃描從錯的起點開始會錯位；換幾個起點，只要有一個起點讓某道指令
    剛好在 end 結束，而且該指令帶 es: 段覆寫，就採用。
    """
    best = None
    for back in range(16, 1, -1):
        rows = disasm(code, vma, end - back, end + 2)
        for i, (a, b, txt) in enumerate(rows):
            n = len(b.split())
            if a < end <= a + n and txt and not txt.startswith("(bad)"):
                if "es:" in txt:
                    return a, b, txt
                if best is None and a + n == end:
                    best = (a, b, txt)
    return best


def main():
    args = sys.argv[1:]
    vma = 0xB000
    if "--vma" in args:
        i = args.index("--vma")
        vma = int(args[i + 1], 16)
        del args[i:i + 2]
    census_path, code = args[0], args[1]
    c = json.load(open(census_path, encoding="utf-8"))
    sites = c["sites"]

    by_site = {}
    for key, s in sites.items():
        r = instruction_ending_at(code, vma, s["linear"])
        by_site[key] = r

    print(f"# {c['edition']}：{c['exe']}（SHA-256 `{c['exe_sha256'][:16]}…`）"
          f"，{c['month_ends']} 個月，讀 {c['reads']} 次、寫 {c['writes']} 次")
    print()
    print("| 位移 | 讀／寫 | 指令結尾（線性） | 指令 | 次數 | 槽數 |")
    print("|---:|---|---|---|---:|---:|")
    by_off = defaultdict(list)
    for key, s in sites.items():
        for off in s["offs"]:
            by_off[off].append(key)
    for off in sorted(by_off):
        for key in sorted(by_off[off], key=lambda k: sites[k]["linear"]):
            s = sites[key]
            r = by_site[key]
            txt = f"`{r[2]}`" if r else "（找不到結尾對得上的指令）"
            print(f"| {off} | {s['kind']} | `0x{s['linear']:05x}` | {txt} "
                  f"| {s['count']} | {len(s['slots'])} |")
    untouched = [o for o in range(72) if o not in by_off]
    print()
    print(f"沒碰過的位移（{len(untouched)} 個）：{untouched}")


if __name__ == "__main__":
    main()
