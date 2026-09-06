# CONTEXT — 三國演義（智冠 1991, DOS）remake

新 session 或對話被壓縮後，先讀這一份。規則在 `CLAUDE.md`，這裡是**現況**。

---

## 1. 現在做到哪裡

| | 狀態 | 日期 |
|---|---|---|
| 素材取得 | 兩版 js-dos bundle 下載完成、SHA-256 驗過 | 2026-09-06 |
| 專案骨架 | 建立；`CLAUDE.md`／`LICENSE`／`.gitignore` 就位 | 2026-09-06 |
| dosgolem 工作副本 | `~/cht/dosgolem-san`，分支 `san1-msc-oracle`（基於 `origin/master`）| 2026-09-06 |
| 說明書 | 46 頁解到 `workplace/manual/`，整理中 | 2026-09-06 |
| dosgolem probe | 兩版跑過，服務清單產出（`docs/re/00`）| 2026-09-06 |
| 反組譯 | **未開始** | |
| 格式解析 | **未開始** | |
| Go 程式 | **未開始**（`internal/` 目前是空目錄）| |

里程碑定義在 `CLAUDE.md` §10。目前在 **M0**。

---

## 2. 素材

`org_game/MANIFEST.md` 有來源、雜湊與取得方式。摘要：

| 版本 | 進入點 | 編譯器 |
|---|---|---|
| 原版 | `AA.EXE`（354,960 B）| Microsoft C 6.0 |
| 加強版 | `ASV.EXE`（300,494 B）| Microsoft C 6.0 |

`workplace/manual/珍098-三國演義/` 有 46 頁掃描（`000.jpg`–`045.jpg`，約 1080×1380）。

---

## 3. 已知事實

每條標推論等級（`CLAUDE.md` §8）與版本（`[base]`／`[plus]`／`[both]`）。

| # | 斷言 | 等級 | 版本 | 證據 |
|---|---|---|---|---|
| F1 | 兩支主程式都由 Microsoft C 6.0 編譯 | `L0` | `[both]` | 兩檔皆含字串 `MS Run-Time Library - Copyright (c) 1990, Microsoft Corp` |
| F2 | 六個 `.IDX` 與六個 `.NAM` 兩版**逐位元組相同** | `L0` | `[both]` | `org_game/` 下 SHA-256 比對 |
| F3 | `DATA1.GRP`／`DATA3.GRP`／`DATA4.GRP` 兩版相同 | `L0` | `[both]` | 同上 |
| F4 | `DATA0.GRP` 42,488 → 87,696；`DATA5.GRP` 145,378 → 287,590；`DATA2.GRP` 長度相同但內容不同 | `L0` | — | 同上 |
| F5 | 加強版獨有 `NAME001`–`NAME006.SHA`、`SV.COM`、`CHKLIST.CPS`；原版獨有 `10/20/D5.GRP`、`PARTNSAV.FIL`、三個 `.BAT` | `L0` | — | 同上 |
| F6 | 兩份 bundle 附的 `dosbox.conf` 皆 `machine=svga_s3`、`memsize=16`、`core=auto`、`cycles=auto` | `L0` | `[both]` | `.jsdos/dosbox.conf`，兩版只差 autoexec 末行的執行檔名 |
| F7 | 兩版啟動時的 DOS 服務輪廓**逐項相同**（`AH=35`×12、`25`×11、`44`×5、`30`×2、`4A`×2、`48`×1），停止位址只差 `0x21` | `L0` | `[both]` | dosgolem probe，`docs/re/00` |
| F8 | 兩支執行檔都在 `int 21h AH=08`（無回顯字元輸入）上空轉；dosgolem 未實作該服務 | `L0` | `[both]` | 同上，佔全部呼叫 99.99% |

### 3.1 待解的矛盾（最高優先）

**`DATA0.GRP` 長度翻倍，而 `DATA0.IDX` 一個位元組都沒變**（F2 + F4）。

`.IDX` 因此**不是單純的位元組位移表**。三個候選解釋，都還沒驗：

- `L3` 定長槽，索引記的是槽號不是位移
- `L3` `.GRP` 自帶內部檔頭／目錄，`.IDX` 索引的是更上層的東西
- `L3` `.IDX` 索引的根本不是 `.GRP`，是 `.NAM`

**先解這個再碰 `.GRP` 解碼器。** 反過來做會得到自洽但錯的結果，
而且不會報錯（`CLAUDE.md` §7 第 18 條）。

`DATA2.GRP` 兩版等長不同容，是逐位元組 diff 最划算的一份。

另一條路是 dosgolem：probe 的「開過的檔」會直接說出原版用什麼順序、讀哪幾段取三件套，比靜態猜格式硬。前提是先補完 `AH=08`（F8）。

---

## 4. 已被推翻的斷言

（目前無。推翻任何上表斷言時，把原斷言、當初的證據、推翻的證據一起搬到這裡，
正文改寫成現況——`rulebook/63`。）

---

## 5. 決策紀錄

| 日期 | 決策 | 誰 |
|---|---|---|
| 2026-09-06 | 兩版並列，`internal/rules` 共用，版本差異用旗標；**差異解出來之前不預先造旗標** | 使用者 |
| 2026-09-06 | 定位是 remake ＋ 英日多語系；繁中是原文不是譯文 | 使用者 |
| 2026-09-06 | repo 名 `softworld_san1_remake`，private | 使用者 |
| 2026-09-06 | dosgolem 上游加 `apps/san1` ＋ `runtime/msc`；本機另 clone `dosgolem-san`，獨立分支 | 使用者 |

---

## 6. 文件索引

| 目錄 | 內容 | 現況 |
|---|---|---|
| `docs/re/` | 程式碼在哪（位址、bytes、xref）| 空 |
| `docs/formats/` | 資料長什麼樣 | 空 |
| `docs/spec/` | `DRAFT`／`READY`／`CONFORMED`／`SUPERSEDED` | 空 |
| `docs/mechanics/` | 遊戲怎麼運作 | 空 |
| `docs/playtest/` | 原版 vs remake 同狀態比較 | 空 |
| `docs/reference/` | 說明書整理、社群資料 | 整理中 |
| `docs/design/` | remake 自己的設計決策 | 空 |
| `docs/release/` | 發行 | 空 |

---

## 7. 術語表

遊戲內術語一律以說明書繁中原文為準（`docs/reference/01-manual-40-glossary.md`）。
下面只放**專案內部**的詞。

| 詞 | 意思 |
|---|---|
| **base** | 原版，`AA.EXE` |
| **plus** | 加強版，`ASV.EXE` |
| **三件套** | `DATA<n>.GRP` ＋ `DATA<n>.IDX` ＋ `DATA<n>.NAM` 這組容器 |
| **對拍** | 用 dosgolem 在程序內跑原版，逐項比對 remake 的結果 |
| **槽位** | 原版版面上一段固定寬度的文字位置；多語系溢出問題出在這裡 |

---

## 8. Worklist

按 `CLAUDE.md` §10 的 M0 出口條件排：

- [ ] 兩版抽檔，每檔記 SHA-256，分開放 `workplace/orig/{base,plus}/`
- [ ] `tools/ida.sh` 包裝器（照 sangokushi 的形狀），對兩支 EXE 產 `.i64`
- [ ] `tools/go.sh` 包裝器 ＋ `docker/go/Dockerfile`（從 `rich2-go-ebiten` 起）
- [x] `dosgolem cmd/probe` 跑兩支 EXE，產未實作 DOS 服務清單 → `docs/re/00`
- [ ] **在 `dosgolem-san` 實作 `int 21h AH=08`**（阻塞式無回顯輸入）——目前擋住一切的那一步
- [ ] 補完 `AH=08` 後用 `-keys` 重跑 probe，觀測開檔順序
- [ ] 說明書整理成 `docs/reference/01-manual-*`
- [ ] 社群資料整理成 `docs/reference/02-web-*`
- [ ] **解 `.IDX` 索引什麼**（§3.1）
- [ ] `DATA2.GRP` 兩版逐位元組 diff，差異落點分佈

### 之後要回頭確認的

- `LICENSE` 的「灰色地帶」與「第三方素材」兩段目前是**照計畫填的**，
  發行前（M8）要用 `git ls-files` 對一次實際內容。
