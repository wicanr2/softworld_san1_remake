#!/usr/bin/env python3
"""把統一年份分布的兩邊結果並排成 markdown（Issue #9）。

用法：

    python3 tools/unify_report.py workplace/dump/unify

讀 `unify-orig-<seed>.json`（`TestZZUnifyYearOriginal`，一顆種子一份）與
`unify-remake.json`（`TestUnifyYearDistribution`，八顆一份），印逐種子表與
最早／中位／最晚。**只報分布，不判 pass/fail。**
"""
import glob
import json
import os
import statistics
import sys


def load(dirpath):
    orig = {}
    for p in sorted(glob.glob(os.path.join(dirpath, "unify-orig-*.json"))):
        r = json.load(open(p, encoding="utf-8"))
        orig[r["seed"]] = r
    remake = {}
    p = os.path.join(dirpath, "unify-remake.json")
    if os.path.exists(p):
        for r in json.load(open(p, encoding="utf-8")):
            remake[r["seed"]] = r
    return orig, remake


def cell(r):
    if r is None:
        return "—"
    if not r.get("unified"):
        a = r.get("alive") or r.get("alive_per_60_months") or []
        last = a[-1] if a else "?"
        how = "40 分鐘逾時" if r.get("partial") else "上限"
        return f"沒統一（{how}，{r['months'] // 12} 年後剩 {last} 個勢力）"
    lord = r.get("winner_lord") or "（君主欄空）"
    return f"{r['year']} 年 {r['month']} 月，勢力 {r['winner']} {lord}"


def summary(rows):
    years = [r["year"] for r in rows if r.get("unified")]
    if not years:
        return "無"
    years.sort()
    return (f"{len(years)}/{len(rows)} 局統一；最早 {years[0]}、中位 "
            f"{statistics.median(years):g}、最晚 {years[-1]}")


def main():
    dirpath = sys.argv[1]
    orig, remake = load(dirpath)
    seeds = sorted(set(orig) | set(remake))
    print("| 種子 | 原版（示範模式） | remake（`ai.ModeBase`） |")
    print("|---|---|---|")
    for s in seeds:
        print(f"| `{s:#010x}` | {cell(orig.get(s))} | {cell(remake.get(s))} |")
    print()
    print(f"- 原版：{summary(list(orig.values()))}")
    print(f"- remake：{summary(list(remake.values()))}")
    print()
    print("逐年還有幾個勢力持郡（每十年取一格）：")
    print()
    print("| 種子 | 邊 | 開局 | +10 | +20 | +30 | +40 | +50 | +60 | +70 | +80 | +90 | +100 |")
    print("|---|---|---|---|---|---|---|---|---|---|---|---|---|")
    for s in seeds:
        for name, r in (("原版", orig.get(s)), ("remake", remake.get(s))):
            if r is None:
                continue
            if r.get("partial"):
                # 逾時的局只有每 60 個月一格：a[1] 是 +10 年、a[3] 是 +20 年……
                a = r.get("alive_per_60_months") or []
                cells = [""] + [str(a[i]) if i < len(a) else "" for i in range(1, 21, 2)]
            else:
                a = r.get("alive") or []
                cells = [str(a[i]) if i < len(a) else "" for i in range(0, 110, 10)]
            print(f"| `{s:#010x}` | {name} | " + " | ".join(cells) + " |")
    if orig:
        full = [r for r in orig.values() if not r.get("partial")]
        if full:
            print()
            print(f"原版跑完的局實跑 {min(r['seconds'] for r in full)}–{max(r['seconds'] for r in full)} 秒，"
                  f"指令數 {min(r['steps'] for r in full):,}–{max(r['steps'] for r in full):,}；"
                  f"逾時的局在 40 分鐘內走到 "
                  + "、".join(f"{r['months']} 個月" for r in orig.values() if r.get("partial")) + "。")


if __name__ == "__main__":
    main()
