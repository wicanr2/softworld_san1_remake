# Goal：三國演義 remake 忠實度深化迴圈（2026-09-15）

以 GitHub repository `wicanr2/softworld_san1_remake` 的 open Issues 為唯一工作權威。
先依本文第 2 節的既定決策清掉現有五條 Issue，再進入第 3 節的「缺口普查 → 開 Issue → 做」迴圈，
直到普查連續兩輪找不到新的 agent-ready 缺口為止。

## 1. 基本規則

1. 工作分支 `main`。每輪開始先在主機執行三件事：
   `/home/anr2/.local/bin/gh auth status`、
   `git config user.email`（必須是 `wicanr2@gmail.com`，否則設 repo-local，不動全域）、
   `git -C /home/anr2/cht/dosgolem-san branch --show-current`（記下分支名，用到時以它為準）。
2. `gh issue list/view/comment/close/create` 與 `gh label` 一律在主機執行；不得在 Docker 或 sandbox 內執行 gh，不得搬運主機憑證。
3. 每條 Issue 開工前讀完整本文、留言與遠端狀態，並用目前程式、測試與產物核實缺口現在是否仍存在；已滿足的直接補收據關閉，不重做。
4. Issue 本文底部「worklist.json 是權威」與 `blob/master` 連結是舊產生器的過時內容。不得執行 `tools/worklist.py issues --apply`，不得用本機 worklist 覆蓋或推翻 Issue。
5. 分析、建置、測試、IDA、原版執行與 dosgolem 一律在有資源限制、`--rm`、預設 `--network none` 的 Docker 容器內完成；原版素材唯讀掛載；寫入用目前 UID/GID。專案的 Go 入口是 `tools/go.sh`。
6. 一條 Issue 一個獨立 commit；驗證、推送、Issue 收據回讀成功後才能以 `completed` 關閉。
7. 不得為了關閉 Issue 放寬測試、把 direct-entry 或座標注入當正常玩家路徑、把 skip 當通過、或把未知推測寫成正式規則或測試期望。
8. 不使用 subagent。
9. 任何預估超過十分鐘的執行（長對拍、多 seed 長局、整批 `-tags oracle`）開跑前先在 Issue 留言寫下「N 個單位 × 每單位耗時 ≈ 總時長」。`-tags oracle` 的對拍測試三支一批、帶 `-v`，不整包跑。
10. 只清理自己建立的 `san1-*` 容器；禁止任何 `docker prune`／`rmi`；不動 `~/.cache`、`~/.claude`、其他 repo。

## 2. 既定決策（使用者 2026-09-15 裁定，不重問）

| Issue | 決策 | 關閉方式 |
|---|---|---|
| #16 多人模式 | 本階段維持單人交付（`CONTEXT.md` 開頭已記）。不改 `session.Session`、不改存檔 schema。 | 留言指向 `CONTEXT.md` 該段後 `--reason "not planned"` |
| #4 Hercules B0000 | **授權**在 `/home/anr2/cht/dosgolem-san` 目前分支之上開新分支修改（不動 `/home/anr2/cht/dosgolem` 本體，不動上游 `main`）。dosgolem 側獨立 commit；**是否推送到 `wicanr2/dosgolem` 由使用者另行決定，預設不推**。 | 回本 repo 重生 probe 收據、更新 `docs/re/00`、`CONTEXT.md` §1 的 dosgolem 分支列後 `completed` |
| #6 諸侯 57 bytes | 停止線：對 offset 10–13、19–71 每一格，在兩支 `.i64`（`AA.EXE`／`ASV.EXE`）掃 72-byte stride 的讀寫端 xref。每格落入三種之一：有讀寫端且語意解出（`L0`／`L1`）、有讀寫端但語意未定（`L3`，列位址）、兩版都無讀寫端（記「無 consumer」）。覆蓋表填滿即達驗收，不要求全部解出。未證實欄位不進正式 schema。 | 覆蓋表寫進 `docs/spec/003`，留言附表後 `completed` |
| #9 統一年份分布 | 停止線：原版與 remake 各跑 8 個固定 seed，同一劇本、同一難度、全電腦（原版走示範模式 `es:0x5b02 = 0`）。先跑 1 個 seed 量成本並留言；單次超過 30 分鐘則縮到 4 個 seed。只報分布（最早／中位／最晚年份與終局勢力），**不設 pass/fail**。 | 記進 `docs/playtest/05-unify-year.md`，留言附分布後 `completed` |
| #2 三平台簽章 | 維持 blocked。不索取、搬運、輸出或提交任何私鑰／憑證。 | 不關閉 |

處理順序：#16 → #6 → #9 → #4。#2 不碰。

## 3. 缺口普查與開 Issue

現有 Issue 清完之後，每輪做一次普查，來源固定四處：
`CONTEXT.md` §1 與 §4、`VERIFICATION-MATRIX.md` §2–§4 標「未解／未接／還沒」的列、
`docs/mechanics/00-index.md` 的版本標記、`grep -rn Tune internal/game internal/battle`。

普查結果依下列優先序開 Issue。**每輪最多開 5 條 agent-ready**，開之前先
`gh issue list --state all --search "<關鍵字>"` 查重，已有的不重開。

### A. 加強版對拍（主線 3「兩版並列」的實際缺口）

`docs/mechanics` 的斷言 107 條 `[base]`、1 條 `[plus]`、2 條 `[both]`。
規則層目前只在難度那一塊量過兩版差異（`docs/mechanics/90` §6）。

1. 先確認 `ASV.EXE` 能用行為觸發開機（`docs/spec/015` 的做法）走到遊戲主畫面；不能就先補這一步，位址與停點放在 `internal/parity` 裡加強版自己的檔（現有 `plus_oracle_test.go`），不與原版共用；兩版位址不同，拿錯不會報錯只會一次都攔不到。
2. 依序把原版已有的對拍搬到加強版：開新遊戲盤面 → 一個月的狀態轉移 → 十四項玩家命令 → 戰役逐日 → 存讀檔兩向。每一項一條 Issue。
3. 每對完一項，把對應的機制斷言改標 `[both]`，有差異的寫進 `docs/mechanics/90` 並在 `docs/spec/004` 決定要不要加旗標——**沒量到差異就不造旗標**。

### B. `Tune*` 換成原版公式

`internal/game/tuning.go` 與 `internal/battle/tuning.go` 還有 21 個 `Tune` 常數，
對照表在 `docs/design/02`。每一個常數一條 Issue，流程固定：

1. 先 `grep -rn` 使用處。正常玩家路徑上已經沒人用的，刪掉常數與對照表列，不逆向。
2. 還在用的，從 `.i64` 找原版同一件事的算式（先 grep 自己的 `docs/re/`，落空再挖）。
3. 原版有算式：換掉、對拍、退役常數、更新 `docs/design/02` 與對應 `docs/mechanics`。
4. 原版沒有這件事（remake 自己加的行為）：常數留著，`docs/design/02` 標為 remake 差異，Issue 以 `completed` 關閉並註明「原版無此規則」。

### C. 還沒在實跑裡走過的規則

- 六種戰場計謀：門檻、費用、殺傷是 `L0`，但沒在實跑戰役裡對拍過。每種一條 Issue：玩家親征、盤面直寫記憶體、下計謀、讀原版的部隊記錄比結果。
- 語音 `R499`：宣戰訊息共用的第三段，語意未經聽辨；用 `docs/re/09` 的方法找出它在哪幾則訊息出現，給出語意或記「共用結尾」。

### D. 素材與資料格式的未解項

`.OKR` 465 項（已知 1bpp）、`.MSK`、版權畫面 `CMARKL`／`CMARKR` 未接、
主戰場三個面板的文字排法仍是 remake 自排、城門圖示 `WFLAG?5` 未用。
每一項一條 Issue，驗收一律是「與原版畫面逐像素比」或「round-trip」，不是「看起來像」。

### E. 文件過期斷言

普查途中看到與現況衝突的敘述直接修，同一個 commit 一起改，不另開 Issue。
已知一條：`VERIFICATION-MATRIX.md` §3「一個月的狀態轉移」那一列還寫著
「2026-09-10 起是紅的（差 145）」，§8 同一份已記 2026-09-11 到 0。

### Issue 範本

第一輪先確認標籤存在，沒有就建：`agent-ready`、`needs-human`。
每條 Issue 同時掛 `組:*` 與 `M*`（沿用既有標籤）。

```
標題：[組] 一句話說缺口

## 缺口
現況是什麼、證據在哪（檔案／位址／測試名）、目前等級與版本。

## 驗收
可機器判定的條件。對拍寫「比哪幾個欄位、逐位元組或逐像素」。

## 核實訊號
測試名，或 present／absent 的 pattern 與檔案。

## 文件
會動到的 docs/ 路徑。

<!-- goal:2026-09-15 -->
```

會改變玩家體驗、存檔格式、授權、發行、跨 repo 範圍、或需要主觀視覺取捨的，
開成 `needs-human`，本文寫清楚「已知事實、可行選項、建議、單一待答問題」，不動工。

## 4. 每條 Issue 的流程

1. 讀 Issue、留言、相關 spec／re／mechanics、目前程式與測試；先排除過期斷言。
2. 原版行為一律走 RE 證據 → `DRAFT` spec → `READY` → implementation → 同狀態驗證 → `CONFORMED`。只有 `READY` 能授權寫進 `internal/`。
3. 每條斷言帶 `L0`–`L3` 與 `[base]`／`[plus]`／`[both]`，附位址、bytes、SHA-256 或可重現的 seed＋輸入。
4. dosgolem 是正式 oracle；DOSBox-X 只作交叉驗證，截圖不冒充 dosgolem 收據。
5. 涉及 `RND()` 時兩邊都固定 seed 並記錄；不挑碰巧通過的重擲。
6. 解完機制 `docs/re/` 與 `docs/mechanics/` 兩份都要寫；剛解出的結論寫進既有文件時正文只寫現況，推翻紀錄集中到 `CONTEXT.md` §4。
7. hook 命中 0 次先當「攔錯地方」；位址印線性與 IDA 兩種並標明。

## 5. 收尾

每條 Issue：
- 跑相稱的測試與 `git diff --check`；確認沒有非預期 skip。
- 檢查 root-owned 殘留（`find . -user root`）與自己的 `san1-*` 容器。
- commit 並推送 `main`（commit message 不放 `Claude-Session:` 連結）。
- `gh issue comment --body-file` 留收據：commit SHA、做了什麼、原版檔名與 SHA-256、dosgolem 分支與 commit、seed／初始狀態／輸入、完整驗證命令、PASS／FAIL／SKIP 數、比對結果、證據路徑、已知限制。
- 回讀留言與 `stateReason`，再關閉。
- 重新列 open Issues 決定下一條。

## 6. 完成條件

- 第 2 節五條 Issue 依表處置完畢（#2 保持 open）。
- 普查連續兩輪開不出新的 agent-ready Issue。
- 剩餘 open Issues 全部是 `needs-human` 或 `blocked`，最終回覆列出：已關閉的 Issue 與主要證據、每條 needs-human 的單一待答問題、dosgolem-san 分支上未推送的 commit、Docker 與 git 工作樹狀態。
