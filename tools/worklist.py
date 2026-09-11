#!/usr/bin/env python3
"""worklist.json 的核實器與渲染器（`rulebook/61`）。

    tools/worklist.py verify              # 核實全部（oracle 那些很久）
    tools/worklist.py verify --fast       # 跳過 -tags oracle（每支要在 dosgolem 裡開機約 60 秒）
    tools/worklist.py verify --id screen-geometry-408
    tools/worklist.py verify --group 畫面
    tools/worklist.py lint                # 秒級：verify 綁得住不住（測試名還在嗎、pattern 中得到嗎）
    tools/worklist.py stats               # 不跑，只統計
    tools/worklist.py render              # 產生 VERIFICATION-MATRIX.md 的資料段
    tools/worklist.py selftest            # 正反對照：證明 verify 真的在看
    tools/worklist.py issues              # 列出要怎麼同步到 GitHub issue（不動 GitHub）
    tools/worklist.py issues --apply      # 真的同步：開、改、關

## issues：GitHub issue 是鏡像，權威是 worklist.json

未完成（`open`／`blocked`）的每一條對應一個 issue；條目改成 `done` 之後重跑，
那個 issue 會被關掉。對應靠 body 裡的 `<!-- worklist:ID -->` 標記，重跑只會
更新不會重複開。**在 GitHub 上直接改的內容，下次同步會被蓋掉**——要改就改
worklist.json（`rulebook/61`：待辦是資料，每條掛 verify；issue 沒有 verify）。

## verify 回答的是「這一條的 status 還成立嗎」

不是「這一項好不好」。清單會長出**過期斷言**——東西做好了而沒有人回頭改那一條，
或前提被推翻了而勾還在。症狀不是報錯，是清單上留著一句自信的話，然後有人拿它
當「還剩多少」的依據。

這個專案已經被咬過一次：`VERIFICATION-MATRIX.md` 記著「主選單 208,050 點逐點
相同」打勾很久，而那個數字建立在「畫面是 640×350」這個錯前提上（`CONTEXT.md`
R47）。勾沒有錯，**前提**錯了，而 markdown 分不出這兩件事。

所以兩個方向都要核實：

| status | verify 為真的意思 | 為假時報 |
|---|---|---|
| `done` | 證據還在（測試還過、數字還對）| **狀態過期**：說做完了，證據不見了 |
| `open`／`blocked` | 未完成的訊號還在（自承註解還在、東西還沒出現）| **可能已完成**：訊號不見了，回頭看這一條 |

## kind 綁什麼訊號

由穩到不穩（`rulebook/61`，本專案在最前面多一層）：

| kind | 為真＝ | 綁什麼 | 穩定度 |
|---|---|---|---|
| `test` | 測試通過 | `go test` 的個別結果 | 最穩：測試自己會紅 |
| `cmd` | 命令成功或輸出等於預期 | 可重數的數字 | 穩 |
| `json_len` | 某份 JSON 的欄位長度 ≤ max | 進度型清單 | 穩：註解會被順手改，項數不會 |
| `absent` | pattern 找不到 | 型別名、函式名還沒出現 | 中 |
| `present` | pattern 找得到 | 程式碼裡的**自承註解** | 弱：改動時可能被順手刪 |
| `manual` | 一律回「要人判」 | 沒有機器訊號的 | 沒有訊號 |

**[HARD] `manual` 一律回「仍待人判」並在報表上標出來。沉默不等於通過。**
**[HARD] 不硬湊 pattern。** 會誤判的 verify 比沒有 verify 更糟——它給人「有在看」
的假象。沒訊號就標 `manual`，在 `note` 寫明將來接上時該綁什麼。

## `test` 為什麼要看個別結果不看 exit code

`go test` 在三種情況都回 0 並印 `ok`：真的通過、整包被 skip、一支都沒選中。
後兩種都不是通過，而在 exit code 上與通過完全一樣（`CLAUDE.md` §7 第 18 條）。
所以判準是 `--- PASS/FAIL/SKIP` 那幾行：

- `skip` → **證據沒拿到**（多半是沒掛原版素材），不算通過。
- `nomatch` → **驗證失聯**：`-run` 一支都沒選中，測試改名或刪了。
"""

import argparse
import json
import os
import re
import subprocess
import sys
import time

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WORKLIST = os.path.join(ROOT, "worklist.json")
SCAN_SUFFIXES = {".go", ".py", ".sh", ".md", ".json"}

# **產物不能當訊號來源。** `VERIFICATION-MATRIX.md` 的資料段是 `render` 從
# worklist.json 產生的；綁它等於綁自己剛寫下去的東西，verify 會永遠為真而
# 看起來一切正常。所以 grep 結構上跳過這些檔——想綁的是**真正的來源**：
# 程式碼裡的自承註解、測試裡的 skip 條件、資料檔的項數。
RENDERED = {"VERIFICATION-MATRIX.md"}

# 核實結果。前四個是 verify 真的跑了；manual 是沒得跑。
HOLDS = "holds"        # status 還成立
STALE = "stale"        # status 過期了——**這才是要看的**
SKIP = "skip"          # 驗證跑了但被 skip：證據沒拿到，不算數
NOMATCH = "nomatch"    # 驗證失聯：測試改名或刪了
MANUAL = "manual"      # 沒有機器訊號，要人判
ORDER = [STALE, NOMATCH, SKIP, MANUAL, HOLDS]
MARK = {HOLDS: "✓", STALE: "✗", SKIP: "－", NOMATCH: "?", MANUAL: "人"}


def load():
    with open(WORKLIST, encoding="utf-8") as f:
        return json.load(f)


def run(cmd, timeout, cwd=None):
    """跑一條命令，回 (returncode, 合併輸出)。

    **逾時當失敗不當通過。** 接不住例外的話整支工具會炸在中間，
    看起來像工具壞了，不像那一項跑太久。
    """
    try:
        p = subprocess.run(cmd, shell=True, cwd=cwd or ROOT, timeout=timeout,
                           stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        return p.returncode, p.stdout
    except subprocess.TimeoutExpired as e:
        out = e.stdout or ""
        if isinstance(out, bytes):
            out = out.decode("utf-8", "replace")
        return 124, out + f"\n[逾時 {timeout} 秒]"


def is_test_file(name):
    """`*_test.go`／`test_*.py` 這一類。"""
    return "_test." in name or name.startswith("test_")


def grep(paths, pattern, root=None, include_tests=False):
    """在 paths 底下找 pattern，回第一個命中的檔案（相對路徑）或 None。

    **[HARD]（`rulebook/61`）預設不掃測試檔。** 測試本來就會提到還沒接上的
    東西——為了釘住將來的行為，或為了測那個資料結構本身。把 `*_test.*` 算進來，
    `absent` 會因為測試裡有一行呼叫就判成「已經做了」，真缺口就這樣被蓋掉。
    條目問的是「**產品程式碼**做了這件事沒有」。

    例外要**逐條明寫** `"include_tests": true`：本專案的「驗證缺口」
    （`layer: verification`——東西接上了但沒對拍過）訊號天然就在對拍測試裡，
    因為那裡才是證據的所在地。那是另一個問題，不是「功能做了沒」。
    """
    root = root or ROOT
    # **`re.M` 不能省。** 整個檔案當一個字串比對時，沒有 `re.M` 的 `^` 只匹配
    # 檔案開頭——`^func TestFoo\(` 因此永遠不中，而那看起來與「測試真的被刪了」
    # 一模一樣。（`lint` 第一次跑就這樣誤報了 61 條。）
    expr = re.compile(pattern, re.M)
    for target in paths:
        p = os.path.join(root, target)
        files = []
        if os.path.isdir(p):
            for dirpath, dirnames, filenames in os.walk(p):
                dirnames[:] = [d for d in dirnames
                               if d not in ("workplace", "org_game", ".git")]
                files += [os.path.join(dirpath, f) for f in filenames]
        elif os.path.isfile(p):
            files = [p]
        for f in files:
            if os.path.splitext(f)[1] not in SCAN_SUFFIXES:
                continue
            base = os.path.basename(f)
            if base in RENDERED:
                continue
            if not include_tests and is_test_file(base):
                continue
            try:
                with open(f, encoding="utf-8", errors="ignore") as fh:
                    if expr.search(fh.read()):
                        return os.path.relpath(f, root)
            except OSError:
                continue
    return None


def classify_go_test(out):
    """從 `go test -v` 的輸出判斷結果，回 (結論, (pass, fail, skip))。"""
    p = len(re.findall(r"^\s*--- PASS:", out, re.M))
    f = len(re.findall(r"^\s*--- FAIL:", out, re.M))
    s = len(re.findall(r"^\s*--- SKIP:", out, re.M))
    if re.search(r"\[build failed\]", out):
        return "build", (p, f, s)
    if f:
        return "fail", (p, f, s)
    if p:
        return "pass", (p, f, s)   # 有 pass 也有 skip 時算過，skip 數照樣印出來
    if s:
        return "skip", (p, f, s)
    return "nomatch", (p, f, s)


def check(item, fast=False, timeout=1800, root=None):
    """核實一項，回 (結果, 一行說明)。

    結果的語意一律是「**這一項的 status 還成立嗎**」，不是「這一項好不好」。
    """
    v = item.get("verify") or {}
    kind = v.get("kind", "manual")
    status = item.get("status", "open")
    done = status == "done"

    if kind == "manual":
        # [HARD] manual 一律標出來。沉默不等於通過。
        return MANUAL, v.get("why") or "（沒寫為什麼跑不動——補上，或換一個綁得住的訊號）"

    if kind == "test":
        if fast and v.get("tags") == "oracle":
            return SKIP, "--fast 跳過 oracle（每支要在 dosgolem 裡開機約 60 秒）"
        cmd = "tools/go.sh test -v"
        if v.get("tags"):
            cmd += f" -tags {v['tags']}"
        cmd += f" {v['pkg']}"
        if v.get("run"):
            cmd += f" -run '{v['run']}'"
        env = "".join(f"{k}={val} " for k, val in (v.get("env") or {}).items())
        _, out = run(env + cmd, timeout, cwd=root)
        verdict, (p, f, s) = classify_go_test(out)
        detail = f"PASS {p} / FAIL {f} / SKIP {s}"
        if verdict == "nomatch":
            return NOMATCH, f"`-run {v.get('run', '')}` 一支都沒選中——測試改名或刪了？"
        if verdict == "skip":
            return SKIP, detail + "　證據沒拿到（多半是沒掛原版素材）"
        if verdict == "build":
            return STALE, "編譯失敗"
        if verdict == "fail":
            first = next((ln.strip() for ln in out.splitlines()
                          if ln.strip().startswith("--- FAIL")), "")
            return (STALE if done else HOLDS), detail + (f"　{first}" if first else "")
        return (HOLDS if done else STALE), detail

    if kind == "cmd":
        rc, out = run(v["cmd"], timeout, cwd=root)
        expect = v.get("expect", "pass")
        if expect == "pass":
            ok = rc == 0
            detail = f"exit={rc}"
        else:
            got = out.strip()
            ok = got == str(expect)
            detail = f"＝ {expect}" if ok else f"預期 {expect}，實得 {got[:60]!r}"
        return (HOLDS if ok == done else STALE), detail

    if kind == "json_len":
        path = os.path.join(root or ROOT, v["path"])
        try:
            with open(path, encoding="utf-8") as fh:
                field = json.load(fh)
                for key in v["field"].split("."):
                    field = field[key]
        except (OSError, KeyError, ValueError) as e:
            return NOMATCH, f"讀不到 {v['path']} 的 {v['field']}：{e}"
        n, cap = len(field), v["max"]
        still_open = n <= cap
        detail = f"{v['path']} 的 {v['field']} 有 {n} 項（門檻 {cap}）"
        return (STALE if still_open == done else HOLDS), detail

    if kind in ("present", "absent"):
        where = grep(v["paths"], v["pattern"], root=root,
                     include_tests=bool(v.get("include_tests")))
        if kind == "present":
            still_open = bool(where)
            detail = f"自承還在 {where}" if where else f"找不到 /{v['pattern']}/ 了"
        else:
            still_open = not where
            detail = f"已經出現在 {where}" if where else f"還沒出現 /{v['pattern']}/"
        # still_open 為真代表「仍未完成」；status=done 時那就是矛盾。
        return (STALE if still_open == done else HOLDS), detail

    return NOMATCH, f"不認識的 verify.kind：{kind!r}"


def cmd_verify(args):
    data = load()
    items = data["items"]
    if args.id:
        items = [i for i in items if i["id"] in args.id]
        if not items:
            print(f"沒有這些 id：{args.id}", file=sys.stderr)
            return 2
    if args.group:
        items = [i for i in items if i.get("group") in args.group]
    if args.status:
        items = [i for i in items if i.get("status") in args.status]

    tally = {}
    bad = []
    t0 = time.time()
    for n, item in enumerate(items, 1):
        result, detail = check(item, fast=args.fast, timeout=args.timeout)
        tally[result] = tally.get(result, 0) + 1
        print(f"[{n}/{len(items)}] {MARK[result]} {item['id']}：{detail}", flush=True)
        if result in (STALE, NOMATCH, SKIP):
            bad.append((item, result, detail))

    print(f"\n核實 {len(items)} 項，{time.time() - t0:.0f} 秒")
    print("　".join(f"{k} {tally[k]}" for k in ORDER if tally.get(k)))
    if tally.get(MANUAL):
        print(f"⚠ {tally[MANUAL]} 項要人判——**沉默不等於通過**，收尾時逐一看過。")

    if bad:
        why = {
            STALE: None,   # 下面依 status 分開講
            NOMATCH: "驗證失聯：測試改名或刪了，這一條沒有人在看",
            SKIP: "證據沒拿到（多半是沒掛原版素材）——skip 不是綠",
        }
        print(f"\n⚠ 要處理的有 {len(bad)} 項：")
        for item, result, detail in bad:
            msg = why[result]
            if msg is None:
                msg = ("狀態過期：說做完了，證據不見了"
                       if item["status"] == "done"
                       else "可能已完成：未完成的訊號不見了，回頭看這一條")
            print(f"  {MARK[result]} {item['id']}（status={item['status']}）")
            print(f"      {msg}")
            print(f"      {detail}")
        return 1
    return 0


def cmd_lint(args):
    """靜態檢查每一條 verify 綁得住不住——**不跑測試**。

    抓的是 `verify` 三種會安靜失效的方式：

    1. **驗證失聯**：`test` 那條的測試名在原始碼裡找不到（改名或刪了）。
       這種最陰險——`go test -run` 選不中任何測試時回 0 並印 `ok`，
       要真的跑一輪（對拍那些一支一分鐘起跳）才會顯形。
    2. **pattern 一個都沒中**：`present` 綁到不存在的字串，或 `absent` 綁到
       **猜想中的識別字**。前者會誤判已完成，後者會永遠說「還沒做」而
       其實一次都沒真的看過（`rulebook/61`）。
    3. **路徑不存在**：`paths` 或 `json_len` 的 `path` 打錯字。

    跑起來是秒級的，收尾前掃一次比跑一小時對拍便宜太多。
    """
    data = load()
    bad = []
    for item in data["items"]:
        v = item.get("verify") or {}
        kind = v.get("kind", "manual")
        if kind == "manual":
            if not v.get("why"):
                bad.append((item["id"], "manual 沒寫 why——[HARD] 要說明為什麼跑不動"))
            if not v.get("note"):
                bad.append((item["id"], "manual 沒寫 note——[HARD] 要寫明將來接上時該綁什麼"))
            continue

        if kind == "test":
            # `-run` 是正則，把 `|` 拆開逐個找 `func <名字>(`。
            pkg = v["pkg"].lstrip("./")
            names = [n for n in v.get("run", "").split("|") if n]
            for n in names:
                found = grep([pkg], r"^func " + re.escape(n) + r"\(", include_tests=True)
                if not found:
                    bad.append((item["id"],
                                f"驗證失聯：`{pkg}` 裡找不到 `func {n}(`——改名或刪了？"))
            if not names:
                bad.append((item["id"], "test 沒寫 run——會跑整包，等於沒綁住這一條"))
            continue

        if kind in ("present", "absent"):
            for path in v["paths"]:
                if not os.path.exists(os.path.join(ROOT, path)):
                    bad.append((item["id"], f"路徑不存在：{path}"))
            where = grep(v["paths"], v["pattern"],
                         include_tests=bool(v.get("include_tests")))
            if kind == "present" and not where:
                bad.append((item["id"],
                            f"present 的 pattern 一個都沒中：/{v['pattern']}/"))
            # absent 沒中是正常的（東西還沒出現），但**那正是它最危險的時候**：
            # 綁到猜想中的識別字時看起來完全一樣。這裡只能提醒，不能判定。
            continue

        if kind == "json_len":
            if not os.path.exists(os.path.join(ROOT, v["path"])):
                bad.append((item["id"], f"路徑不存在：{v['path']}"))

    absents = [i["id"] for i in data["items"]
               if (i.get("verify") or {}).get("kind") == "absent"]
    print(f"檢查 {len(data['items'])} 項")
    if bad:
        print(f"\n✗ {len(bad)} 條綁不住：")
        for iid, why in bad:
            print(f"  {iid}：{why}")
    else:
        print("✓ 每一條都綁得住（測試名找得到、pattern 中得到、路徑存在）")
    if absents:
        print(f"\n⚠ `absent` 那 {len(absents)} 條沒辦法靜態檢查——"
              f"「還沒出現」與「pattern 寫錯」長得一模一樣。")
        print(f"   {'、'.join(absents)}")
        print("   收尾時逐條問一次：接上那件事時，這個 pattern **一定**會出現嗎？")
    return 1 if bad else 0


def cmd_stats(args):
    data = load()
    items = data["items"]
    by_status, by_kind, by_group = {}, {}, {}
    for i in items:
        by_status[i.get("status")] = by_status.get(i.get("status"), 0) + 1
        k = (i.get("verify") or {}).get("kind", "?")
        by_kind[k] = by_kind.get(k, 0) + 1
        g = i.get("group", "?")
        by_group.setdefault(g, {"n": 0, "done": 0, "auto": 0})
        by_group[g]["n"] += 1
        by_group[g]["done"] += i.get("status") == "done"
        by_group[g]["auto"] += k != "manual"

    print(f"共 {len(items)} 項\n")
    print("狀態：" + "　".join(f"{k} {v}" for k, v in sorted(by_status.items())))
    print("訊號：" + "　".join(f"{k} {v}" for k, v in sorted(by_kind.items())))
    auto = len(items) - by_kind.get("manual", 0)
    print(f"\n有機器訊號的 {auto} 項（{100 * auto // max(len(items), 1)}%）；"
          f"其餘 {by_kind.get('manual', 0)} 項要人判。\n")
    print(f"{'組':<10} {'項數':>4} {'done':>5} {'有訊號':>6}")
    for g, s in sorted(by_group.items(), key=lambda kv: -kv[1]["n"]):
        print(f"{g:<10} {s['n']:>4} {s['done']:>5} {s['auto']:>6}")
    return 0


def cmd_render(args):
    """產生 VERIFICATION-MATRIX.md 的資料段（**不要手改那一段**）。"""
    data = load()
    out = ["<!-- worklist:begin —— 這一段由 tools/worklist.py render 產生，不要手改。",
           "     要改內容改 worklist.json，然後重跑 render。 -->", ""]
    groups = {}
    for i in data["items"]:
        groups.setdefault(i.get("group", "其他"), []).append(i)
    for g, items in groups.items():
        out.append(f"### {g}")
        out.append("")
        out.append("| 項目 | 狀態 | 等級 | 版本 | 核實訊號 | 說明 |")
        out.append("|---|---|---|---|---|---|")
        for i in sorted(items, key=lambda x: (x.get("status") != "open", x["id"])):
            v = i.get("verify") or {}
            kind = v.get("kind", "manual")
            if kind == "test":
                # **測試名用頓號串，不用 `|`**：`-run` 的正則裡那個 `|` 會被
                # markdown 讀成欄位分隔，整列版面就散了。
                names = v.get("run", v["pkg"]).split("|")
                sig = "、".join(f"`{n}`" for n in names)
            elif kind == "cmd":
                sig = f"`{v['cmd'][:40]}`" if len(v["cmd"]) <= 40 else "指令重數"
            elif kind == "json_len":
                sig = f"`{v['path']}` 的項數 ≤ {v['max']}"
            elif kind in ("present", "absent"):
                # pattern 是正則，顯示時把跳脫拿掉並截短——這一欄是給人看
                # 「綁在哪個訊號上」，不是給人照抄的。
                pat = re.sub(r"\\(.)", r"\1", v["pattern"])[:36].replace("|", "、")
                sig = f"{kind}：{'／'.join(v['paths'][:1])} 的「{pat}」"
            else:
                sig = "**要人判**"
            st = {"done": "完成", "open": "未完成", "blocked": "卡住"}.get(i["status"], i["status"])
            # 說明裡的 `|` 同理要跳脫，否則一個算式就把表格切成兩半。
            note = (i.get("note") or "").replace("\n", " ").replace("|", "\\|")
            docs = "、".join(f"`{d}`" for d in i.get("docs", []))
            if docs:
                note = f"{note}（{docs}）" if note else docs
            out.append(f"| {i['title']} | {st} | {i.get('level') or '—'} | "
                       f"{i.get('edition') or '—'} | {sig} | {note} |")
        out.append("")
    out.append("<!-- worklist:end -->")
    print("\n".join(out))
    return 0


def cmd_selftest(args):
    """正反對照：證明 verify 真的在看，不是永遠印好消息。

    **[HARD]（`rulebook/61`）**：只驗「它印出仍未完成」證明不了機制有在看——
    空的檔案、寫錯的路徑、永遠為真的 pattern 都會印出一樣的好消息。所以每一種
    kind 都做兩次：**訊號在時**要報一種結論，**訊號拿掉之後**要報相反的那一種。
    """
    import shutil
    import tempfile

    fails = []

    def expect(name, got, want):
        ok = got == want
        print(f"  {'✓' if ok else '✗'} {name}：得 {got}，預期 {want}")
        if not ok:
            fails.append(name)

    tmp = tempfile.mkdtemp(prefix="worklist-selftest-")
    try:
        # --- present：綁自承註解 ---
        src = os.path.join(tmp, "a.go")
        with open(src, "w", encoding="utf-8") as f:
            f.write("// remake 還沒有雲團\npackage x\n")
        it = {"status": "open", "verify": {"kind": "present", "paths": ["a.go"],
                                           "pattern": "還沒有雲團"}}
        print("present（status=open）")
        expect("訊號在 → 這一條仍未完成，status 還成立", check(it, root=tmp)[0], HOLDS)
        with open(src, "w", encoding="utf-8") as f:
            f.write("package x\n")
        expect("訊號拿掉 → 開口說可能已完成", check(it, root=tmp)[0], STALE)

        # --- absent：綁型別名 ---
        it = {"status": "open", "verify": {"kind": "absent", "paths": ["a.go"],
                                           "pattern": "type CloudPuff"}}
        print("absent（status=open）")
        expect("東西還沒出現 → 仍未完成", check(it, root=tmp)[0], HOLDS)
        with open(src, "w", encoding="utf-8") as f:
            f.write("package x\n\ntype CloudPuff struct{}\n")
        expect("東西出現了 → 開口", check(it, root=tmp)[0], STALE)

        # --- json_len：綁數量 ---
        jp = os.path.join(tmp, "d.json")
        with open(jp, "w", encoding="utf-8") as f:
            json.dump({"a": {"b": [1, 2]}}, f)
        it = {"status": "open", "verify": {"kind": "json_len", "path": "d.json",
                                           "field": "a.b", "max": 3}}
        print("json_len（status=open）")
        expect("項數不足 → 仍未完成", check(it, root=tmp)[0], HOLDS)
        with open(jp, "w", encoding="utf-8") as f:
            json.dump({"a": {"b": [1, 2, 3, 4]}}, f)
        expect("項數過門檻 → 開口", check(it, root=tmp)[0], STALE)

        # --- cmd：綁可重數的數字 ---
        print("cmd（status=done）")
        it = {"status": "done", "verify": {"kind": "cmd", "cmd": "echo 7", "expect": "7"}}
        expect("數字對 → status 還成立", check(it, root=tmp)[0], HOLDS)
        it = {"status": "done", "verify": {"kind": "cmd", "cmd": "echo 8", "expect": "7"}}
        expect("數字變了 → 狀態過期", check(it, root=tmp)[0], STALE)

        # --- manual：一律開口 ---
        print("manual")
        it = {"status": "done", "verify": {"kind": "manual", "why": "要憑證"}}
        expect("永遠回要人判，不會靜靜地算通過", check(it, root=tmp)[0], MANUAL)

        # --- 誤判防線：路徑寫錯不能靜靜地變成好消息 ---
        print("路徑寫錯（present 綁到不存在的檔）")
        it = {"status": "done", "verify": {"kind": "present", "paths": ["沒這個檔.go"],
                                           "pattern": "x"}}
        expect("找不到 → done 項判 holds 是對的，但下一行要抓得到反例",
               check(it, root=tmp)[0], HOLDS)
        it = {"status": "open", "verify": {"kind": "present", "paths": ["沒這個檔.go"],
                                           "pattern": "x"}}
        expect("同一條路徑在 open 項上要開口（否則沒人會發現路徑寫錯）",
               check(it, root=tmp)[0], STALE)

        # --- 測試檔預設不算訊號 ---
        print("測試檔排除（[HARD]：條目問的是產品程式碼做了沒有）")
        with open(os.path.join(tmp, "b_test.go"), "w", encoding="utf-8") as f:
            f.write("func TestX(){ CloudPuff{} }\n")
        it = {"status": "open", "verify": {"kind": "absent", "paths": ["b_test.go"],
                                           "pattern": "CloudPuff"}}
        expect("測試裡出現不算做完，所以 open 項仍成立", check(it, root=tmp)[0], HOLDS)
        it["verify"]["include_tests"] = True
        expect("明寫 include_tests 才掃得到（驗證缺口用）", check(it, root=tmp)[0], STALE)

        # --- 產物不能當訊號來源 ---
        print("產物排除（綁 render 的產物等於綁自己寫的東西）")
        prod = os.path.join(tmp, "VERIFICATION-MATRIX.md")
        with open(prod, "w", encoding="utf-8") as f:
            f.write("這一段寫著「還沒接」\n")
        it = {"status": "open", "verify": {"kind": "present",
                                           "paths": ["VERIFICATION-MATRIX.md"],
                                           "pattern": "還沒接"}}
        expect("產物裡的字串不算訊號，所以 open 項要開口", check(it, root=tmp)[0], STALE)
        with open(os.path.join(tmp, "real.go"), "w", encoding="utf-8") as f:
            f.write("// 還沒接\n")
        it["verify"]["paths"] = ["real.go"]
        expect("同一個 pattern 綁在真正的來源上要成立", check(it, root=tmp)[0], HOLDS)

        # --- 行首錨點要在 re.M 之下 ---
        print("`^` 的行為（沒有 re.M 的話 lint 會誤報整批「測試被刪了」）")
        with open(os.path.join(tmp, "c.go"), "w", encoding="utf-8") as f:
            f.write("package x\n\nfunc TestFoo(t *testing.T) {}\n")
        it = {"status": "open", "verify": {"kind": "absent", "paths": ["c.go"],
                                           "pattern": r"^func TestFoo\("}}
        expect("`^func` 要匹配到行首，不是只匹配檔案開頭",
               check(it, root=tmp)[0], STALE)

        # --- go test 的三種假綠 ---
        print("go test 的輸出分類")
        expect("整包 skip 不是 pass",
               classify_go_test("=== RUN Test\n--- SKIP: Test (0.00s)\nPASS\nok\t x")[0], "skip")
        expect("一支都沒選中是 nomatch 不是 pass",
               classify_go_test("testing: warning: no tests to run\nPASS\nok\t x")[0], "nomatch")
        expect("真的通過才是 pass",
               classify_go_test("--- PASS: Test (0.01s)\nPASS\nok\t x")[0], "pass")
        expect("編譯失敗要單獨認出來",
               classify_go_test("# pkg [pkg.test]\nx.go:1:2: boom\nFAIL\t x [build failed]")[0],
               "build")
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    print()
    if fails:
        print(f"✗ selftest 有 {len(fails)} 條沒過：{fails}")
        return 1
    print("✓ selftest 全過——每一種 kind 都在訊號有無兩邊各報過一次相反的結論。")
    return 0


# ---- GitHub issue 同步 ----------------------------------------------------

ISSUE_LABEL = "worklist"
MARKER = "<!-- worklist:{} -->"


def gh(args, input_text=None):
    """跑 gh，回 (returncode, stdout, stderr)。"""
    p = subprocess.run(["gh"] + args, capture_output=True, text=True, input=input_text, cwd=ROOT)
    return p.returncode, p.stdout, p.stderr


def repo_slug():
    """從 origin 的 URL 取 owner/name。"""
    url = subprocess.run(["git", "remote", "get-url", "origin"], capture_output=True,
                         text=True, cwd=ROOT).stdout.strip()
    m = re.search(r"github\.com[:/](.+?)(?:\.git)?$", url)
    if not m:
        raise SystemExit(f"origin 不是 GitHub：{url!r}")
    return m.group(1)


def issue_labels(item):
    """一條要掛的標籤：worklist、里程碑、組、卡住。"""
    out = {ISSUE_LABEL}
    if item.get("milestone"):
        out.add(item["milestone"])
    if item.get("group"):
        out.add("組:" + item["group"])
    if item.get("status") == "blocked":
        out.add("blocked")
    return out


def issue_title(item):
    return f"[{item.get('group', '其他')}] {item['title']}"


def verify_text(v, slug):
    kind = v.get("kind", "manual")
    if kind == "test":
        return f"`test`：`go test {v['pkg']} -run '{v.get('run', '')}'`" + \
               ("（要掛原版素材）" if v.get("needs") else "")
    if kind == "cmd":
        return f"`cmd`：`{v['cmd']}`，預期 `{v.get('expect', '')}`"
    if kind in ("present", "absent"):
        where = "、".join(f"[`{p}`](https://github.com/{slug}/blob/master/{p})" for p in v["paths"])
        if kind == "present":
            return (f"`present`：{where} 裡找得到 `{v['pattern']}`——那是「還沒做完」的自承，"
                    "它不見了就表示這一條可能做完了")
        return (f"`absent`：{where} 裡找不到 `{v['pattern']}`——它出現了就表示這一條做完了")
    if kind == "json_len":
        return f"`json_len`：`{v['path']}` 的項數 ≤ {v['max']}"
    return "`manual`：沒有機器訊號，要人判" + (f"——{v['why']}" if v.get("why") else "")


def issue_body(item, slug):
    st = {"open": "未完成", "blocked": "卡住", "done": "完成"}.get(item["status"], item["status"])
    head = (f"**worklist**：`{item['id']}`　**組**：{item.get('group', '—')}　"
            f"**里程碑**：{item.get('milestone') or '—'}　**狀態**：{st}　"
            f"**等級**：{item.get('level') or '—'}　**版本**：{item.get('edition') or '—'}")
    parts = [head, "", item.get("note", "").strip() or "（沒有說明）"]
    if item.get("acceptance"):
        parts += ["", "### 驗收", "", item["acceptance"]]
    parts += ["", "### 核實訊號", "", verify_text(item.get("verify") or {}, slug)]
    if item.get("docs"):
        parts += ["", "### 文件", ""] + [f"- `{d}`" for d in item["docs"]]
    parts += ["", "---",
              "這個 issue 由 `tools/worklist.py issues` 從 "
              f"[`worklist.json`](https://github.com/{slug}/blob/master/worklist.json) 產生。"
              "**權威是 worklist.json**：要改內容或狀態請改 worklist 再重跑同步——"
              "在這裡直接改的內容，下次同步會被蓋掉。",
              "", MARKER.format(item["id"])]
    return "\n".join(parts)


def cmd_issues(args):
    """把未完成的條目同步成 GitHub issue；done 的關掉。預設只列計畫。"""
    data = load()
    slug = repo_slug()
    rc, out, err = gh(["issue", "list", "-R", slug, "--label", ISSUE_LABEL, "--state", "all",
                       "--limit", "1000", "--json", "number,title,state,body,labels"])
    if rc != 0:
        # 第一次跑時標籤還不存在，gh 會報錯——當成沒有既有的 issue。
        if "not found" not in err and "could not find" not in err.lower():
            print(err, file=sys.stderr)
            return 2
        out = "[]"
    existing = {}
    for iss in json.loads(out or "[]"):
        m = re.search(r"<!-- worklist:([^ ]+) -->", iss.get("body") or "")
        if m:
            existing[m.group(1)] = iss

    plan = []  # (動作, item, issue)
    for item in data["items"]:
        iss = existing.get(item["id"])
        want_open = item["status"] in ("open", "blocked")
        if want_open:
            if iss is None:
                plan.append(("開", item, None))
                continue
            title, body = issue_title(item), issue_body(item, slug)
            have_labels = {l["name"] for l in iss.get("labels", [])}
            if iss["state"] != "OPEN":
                plan.append(("重開", item, iss))
            elif iss["title"] != title or (iss.get("body") or "").strip() != body.strip() or \
                    not issue_labels(item) <= have_labels:
                plan.append(("改", item, iss))
        elif iss is not None and iss["state"] == "OPEN":
            plan.append(("關", item, iss))

    for act, item, iss in plan:
        num = f"#{iss['number']}" if iss else "（新）"
        print(f"{act} {num:>6} {item['id']}：{item['title']}")
    print(f"\n共 {len(plan)} 個動作（{sum(1 for p in plan if p[0] == '開')} 開、"
          f"{sum(1 for p in plan if p[0] in ('改', '重開'))} 改、"
          f"{sum(1 for p in plan if p[0] == '關')} 關）。")
    if not args.apply:
        if plan:
            print("這是計畫；加 --apply 才會動 GitHub。")
        return 0

    # 標籤先備齊：gh issue create 碰到不存在的標籤會整筆失敗。
    need = set()
    for _, item, _ in plan:
        need |= issue_labels(item)
    rc, out, _ = gh(["label", "list", "-R", slug, "--limit", "500", "--json", "name"])
    have = {l["name"] for l in json.loads(out or "[]")} if rc == 0 else set()
    for name in sorted(need - have):
        color = {"worklist": "5319e7", "blocked": "b60205"}.get(name, "c5def5")
        gh(["label", "create", name, "-R", slug, "--color", color,
            "--description", "由 tools/worklist.py issues 管理"])

    failed = 0
    for act, item, iss in plan:
        title, body = issue_title(item), issue_body(item, slug)
        if act == "開":
            cmd = ["issue", "create", "-R", slug, "--title", title, "--body-file", "-"]
            for l in sorted(issue_labels(item)):
                cmd += ["--label", l]
            rc, out, err = gh(cmd, body)
        elif act in ("改", "重開"):
            if act == "重開":
                gh(["issue", "reopen", str(iss["number"]), "-R", slug])
            cmd = ["issue", "edit", str(iss["number"]), "-R", slug, "--title", title,
                   "--body-file", "-"]
            for l in sorted(issue_labels(item)):
                cmd += ["--add-label", l]
            rc, out, err = gh(cmd, body)
        else:  # 關
            v = item.get("verify") or {}
            rc, out, err = gh(["issue", "close", str(iss["number"]), "-R", slug, "--comment",
                               f"worklist 標為 done。核實訊號：{verify_text(v, slug)}"])
        if rc != 0:
            failed += 1
            print(f"✗ {act} {item['id']}：{err.strip()}", file=sys.stderr)
        else:
            print(f"✓ {act} {item['id']} {out.strip()}")
    return 1 if failed else 0


def main():
    ap = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)

    v = sub.add_parser("verify", help="核實每一項的 status 還成不成立")
    v.add_argument("--id", nargs="*")
    v.add_argument("--group", nargs="*")
    v.add_argument("--status", nargs="*")
    v.add_argument("--fast", action="store_true",
                   help="跳過 -tags oracle 那些（每支要在 dosgolem 裡開機約 60 秒）")
    v.add_argument("--timeout", type=int, default=1800)
    v.set_defaults(func=cmd_verify)

    s = sub.add_parser("stats", help="不跑，只統計")
    s.set_defaults(func=cmd_stats)

    l = sub.add_parser("lint", help="靜態檢查 verify 綁得住不住（秒級，不跑測試）")
    l.set_defaults(func=cmd_lint)

    r = sub.add_parser("render", help="產生 VERIFICATION-MATRIX.md 的資料段")
    r.set_defaults(func=cmd_render)

    t = sub.add_parser("selftest", help="正反對照，證明 verify 真的在看")
    t.set_defaults(func=cmd_selftest)

    g = sub.add_parser("issues", help="同步未完成的條目到 GitHub issue（預設只列計畫）")
    g.add_argument("--apply", action="store_true", help="真的開、改、關 issue")
    g.set_defaults(func=cmd_issues)

    args = ap.parse_args()
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
