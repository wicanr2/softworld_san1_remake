# Goal：三國演義 remake 忠實度深化迴圈（2026-09-15）

以 `wicanr2/softworld_san1_remake` 的 open Issues 為唯一工作權威。先依 §2 清掉現有五條，
再進 §3「缺口普查 → 開 Issue → 做」迴圈，直到普查連續兩輪開不出新的 agent-ready Issue。
repo 的 `CLAUDE.md`／`CONTEXT.md` 全部適用（Docker、RE→spec 閘門、推論等級、oracle 位階、素材禁令），這裡不重抄。

## 1. 基本規則

1. 分支 `main`。每輪先在主機跑 `/home/anr2/.local/bin/gh auth status`、`git config user.email`（須為 `wicanr2@gmail.com`，否則設 repo-local）、`git -C /home/anr2/cht/dosgolem-san branch --show-current`。
2. `gh` 只在主機執行，不進 Docker、不搬憑證。不得執行 `tools/worklist.py issues --apply`；Issue 底部「worklist.json 是權威」是過時內容。
3. 開工前核實缺口仍存在；已滿足的補收據關閉，不重做。
4. 一條 Issue 一個 commit；驗證、推送、收據回讀後才以 `completed` 關閉。
5. 不放寬測試、不把 direct-entry 當玩家路徑、不把 skip 當通過、不把推測寫成規則。
6. 不用 subagent。預估超過十分鐘的執行先留言「N × 每單位耗時 ≈ 總時長」；`-tags oracle` 三支一批帶 `-v`。
7. 只清自己的 `san1-*` 容器；禁止 prune／rmi；不動 `~/.cache`、`~/.claude`、其他 repo。

## 2. 既定決策（2026-09-15 裁定，不重問）

| Issue | 決策與停止線 | 關閉 |
|---|---|---|
| #16 多人 | 維持單人（`CONTEXT.md` 開頭）。不改 session／存檔 | `not planned` |
| #4 Hercules | 授權在 `/home/anr2/cht/dosgolem-san` 目前分支上開新分支改；不動 `~/cht/dosgolem` 本體與上游 `main`；**不推送 `wicanr2/dosgolem`**。dosgolem 側獨立 commit，回本 repo 重生 probe 收據、更新 `docs/re/00` 與 `CONTEXT.md` §1 分支列 | `completed` |
| #6 諸侯 57 bytes | offset 10–13、19–71 每格在兩支 `.i64` 掃 72-byte stride 的讀寫端，分三類：語意解出（L0/L1）／有讀寫端語意未定（L3，列位址）／兩版皆無（記「無 consumer」）。表填滿即驗收，寫進 `docs/spec/003`；未證實欄位不進 schema | `completed` |
| #9 統一年份 | 兩邊各 8 個固定 seed，同劇本同難度全電腦（原版示範模式 `es:0x5b02=0`）。先跑 1 個量成本並留言，單次逾 30 分鐘縮到 4 個。只報分布（最早／中位／最晚、終局勢力），不設 pass/fail，寫進 `docs/playtest/05-unify-year.md` | `completed` |
| #2 簽章 | 維持 blocked，不碰憑證 | 不關 |

順序 #16 → #6 → #9 → #4。

## 3. 缺口普查與開 Issue

來源四處：`CONTEXT.md` §1／§4、`VERIFICATION-MATRIX.md` §2–§4 標「未解／未接／還沒」的列、`docs/mechanics/00-index.md` 版本標記、`grep -rn Tune internal/game internal/battle`。
每輪最多開 5 條 agent-ready；開前 `gh issue list --state all --search` 查重。優先序：

**A. 加強版對拍**（`docs/mechanics` 107 條 `[base]`、1 條 `[plus]`）。先確認 `ASV.EXE` 能照 `docs/spec/015` 行為觸發開到主畫面，位址放 `internal/parity` 加強版自己的檔（如 `plus_oracle_test.go`）。再依序搬：開新遊戲盤面 → 月度轉移 → 十四項玩家命令 → 戰役逐日 → 存讀檔，一項一條。對完改標 `[both]`，差異進 `docs/mechanics/90`；**沒量到差異不造旗標**。

**B. 21 個 `Tune*` 換原版公式**（`docs/design/02`）。一個常數一條：先 grep 使用處，死碼直接刪；活碼從 `.i64` 找原版算式（先 grep `docs/re/`），有就換掉對拍退役；原版無此規則則常數留著、`design/02` 標 remake 差異後關閉。

**C. 沒實跑過的規則**：六種戰場計謀各一條（玩家親征、盤面直寫記憶體、下計謀、比原版部隊記錄）；語音 `R499` 語意（`docs/re/09` 的方法）。

**D. 素材與格式未解**：`.OKR` 465 項、`.MSK`、版權畫面 `CMARKL`／`CMARKR`、主戰場面板文字排法、城門圖示 `WFLAG?5`。驗收一律逐像素或 round-trip。

**E. 過期斷言**：看到就同 commit 修，不開 Issue。已知：`VERIFICATION-MATRIX.md` §3 月度轉移列還寫「差 145 紅的」，§8 已記到 0。

第一輪確認標籤 `agent-ready`、`needs-human` 存在，沒有就建；每條同掛 `組:*` 與 `M*`。範本：
`標題：[組] 一句話缺口` ＋ 四段 `## 缺口`（現況、證據位置、等級與版本）／`## 驗收`（可機器判定）／`## 核實訊號`（測試名或 present／absent pattern）／`## 文件`，末尾 `<!-- goal:2026-09-15 -->`。
涉及玩家體驗、存檔格式、授權、發行、跨 repo、主觀視覺取捨的開 `needs-human`，寫「已知事實、可行選項、建議、單一待答問題」，不動工。

## 4. 每條 Issue 的收尾

跑相稱測試與 `git diff --check`，確認無非預期 skip；`find . -user root`；commit 並推 `main`（不放 `Claude-Session:` 連結）；`gh issue comment --body-file` 留收據（commit SHA、做了什麼、原版檔名與 SHA-256、dosgolem 分支與 commit、seed／初始狀態／輸入、驗證命令、PASS／FAIL／SKIP、比對結果、證據路徑、已知限制）；回讀後關閉；重列 open Issues 選下一條。
解完機制 `docs/re/` 與 `docs/mechanics/` 兩份都寫；推翻紀錄集中 `CONTEXT.md` §4。

## 5. 完成條件

§2 五條處置完畢（#2 保持 open）；普查連續兩輪開不出 agent-ready；剩餘 open 全是 `needs-human`／`blocked`。最終回覆列：已關閉 Issue 與主要證據、每條 needs-human 的單一待答問題、dosgolem-san 未推送的 commit、Docker 與 git 工作樹狀態。
