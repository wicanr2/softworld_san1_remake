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
| F8 | 兩支執行檔都在 `int 21h AH=08`（無回顯字元輸入）上空轉 | `L0` | `[both]` | 同上，佔全部呼叫 99.99%。**成因是 dosgolem 沒有阻塞模型，不是遊戲卡住**——見 `docs/re/00` 第二輪 |
| F9 | 原版的 `DATA5.GRP` 與 `10.GRP` **逐位元組相同**；`20.GRP`（142,804）與 `D5.GRP`（145,377）是另外兩個變體 | `L0` | `[base]` | SHA-256 比對 |
| F10 | `10.BAT`／`20.BAT` 的內容是刪掉 `DATA5.GRP` 再從 `10.GRP`／`20.GRP` 複製一份回去 | `L0` | `[base]` | 檔案內容 |
| F11 | `README.DOC`（兩版相同）說明增強版的變更，含難度擴充到 1–20、密碼只需輸入一次、以 `DATA5.GRP` 標示版本 | `L0` | — | `README.DOC` |
| F12 | `ChineseSys` 是遊戲**自己的子系統名**，與 `FileSystem`／`AdLib`／`Music`／`Sound`／`Icon`／`Picture` 同在一張表（`AA.EXE` offset `0x47227` 附近）；不是外部中文系統偵測 | `L0` | `[both]` | 字串上下文 |
| F13 | 手上這本說明書是**原版**的：無版號、無印刷日期，全書無「加強版」字樣；可用時間標記只有代序落款民國八十年四月與版權年 1991 | `L0` | `[base]` | `docs/reference/01-manual-00` |
| F14 | 遊戲有密碼保護，密碼由三個座標決定（首圖 × 橫圖 × 地支），版面推算共 12×12×12 ＝ 1728 組四位數 | `L1`（結構 `L0`、總數 `L2`）| `[base]` | 手冊 p.15–16、密碼表 p.44–45 |
| F15 | 手冊**全書沒有郡名對照表**，42 個郡一律只用編號稱呼 | `L0` | `[base]` | `docs/reference/01-manual-40` |
| F16 | 附錄三收 342 位人物（姓名＋字），程式驗算條目數為 342 | `L0` | `[base]` | 同上 |
| F17 | `.NAM` 每項 16 byte（8.3 檔名）、`.IDX` 每項 4 byte 為結束位移、`.GRP` 為本體；DATA1/2/3 末值 ＝ 檔長，DATA1 round-trip 通過 | `L0` | `[both]` | `docs/formats/01` |
| F18 | `DATA0`／`DATA4`／`DATA5` 的 `.GRP` 開頭是 `MZ`；`DATA4.GRP` 帶未抹除的 `LZ91`（LZEXE 0.91）簽章，重定位數 0 | `L0` | `[both]` | 同上 |
| F19 | **42 個郡名**已取得：`DATA2.GRP` offset 818,235 起、每 176 byte 一筆、名稱在該筆 offset 0（4 byte Big5 ＋ NUL）。兩版該區逐位元組相同 | `L0` | `[both]` | `docs/formats/02` |
| F20 | `DATA1`／`DATA5` 裝字模與調色盤（`ZHONG.PAT`／`YING.PAT`／`EGAFILL.PAL`），`DATA3` 裝 422 筆臉譜（`F###.FAC`），`DATA2` 裝圖片（`SCG*`／`ENDO*.IMG`）| `L0` | `[both]` | `.NAM` 直接讀出 |

### 3.05 密碼表：唯一必須從執行檔取的東西

F14 的密碼表印在 p.44–45，**兩面都是防影印彩色網點，掃描完全不可判讀**。
版面結構讀得出來（12 個首圖 × 12 橫圖 × 12 地支），1728 組四位數本身讀不出來。

所以密碼表**只能從執行檔或資料檔取**——這不是繞過保護，是這份資料
在手上的載體已經失效，而 remake 要重現這個畫面就得有它。
解出來之後放 `docs/formats/`，不進版控（原版資料）。

順帶一提：手冊描述的輸入流程是打四位數 → `Enter` → `Y` 確認。
`docs/re/00` 那個 `AH=08` 迴圈**有可能**是這一關，但目前沒有任何輸出，
所以還是 `L3`——一個會畫密碼畫面的程式不會零繪圖。

### 3.06 郡名已取得

F15 說手冊只有郡編號沒有郡名。郡名已從 `DATA2.GRP` 取出（F19，`docs/formats/02`），
42 個全部合法 Big5、順序由北到南。**沒有一個字是憑印象補的**——
這正是 `CLAUDE.md` §7 第 5 條要防的事。

### 3.1 容器格式已解

`.NAM` 每項 16 byte（DOS 8.3 檔名）、`.IDX` 每項 4 byte 是**結束位移**、
`.GRP` 是資料本體首尾相接。DATA1／2／3 的 `.IDX` 末值精準等於 `.GRP` 長度，
DATA1 的 round-trip 通過（217 項零逆序、無縫覆蓋）。細節與注意事項在
`docs/formats/01-grp-idx-nam.md`。

`DATA0`／`DATA4`／`DATA5` 的 `.GRP` 是 MZ 執行檔，不是資料本體（見 F17–F19）。

---

## 4. 已被推翻的斷言

推翻任何斷言時，把原斷言、當初的證據、推翻的證據一起搬到這裡，
正文改寫成現況——`rulebook/63`。

### R1（2026-09-06）：「`.IDX` 不是單純的位元組位移表」

**原斷言**：`DATA0.GRP` 長度從 42,488 漲到 87,696，而 `DATA0.IDX` 一個位元組都沒變，
所以 `.IDX` 不會是位元組位移表——否則長度變了它必然跟著變。當時列了三個候選
（定長槽／`.GRP` 自帶目錄／`.IDX` 索引的是 `.NAM`），**三個都錯**。

**推翻的證據**：`.IDX` 就是位元組位移表，而且是**結束位移**。DATA1／2／3 三個
獨立檔案的 `.IDX` 末值精準等於各自 `.GRP` 的長度（770,330／1,156,221／1,213,106），
DATA1 的 round-trip 零逆序、無縫覆蓋。

**當初錯在哪**：推論本身沒問題，錯的是**沒驗前提**。那個推論預設了
「`DATA0.GRP` 與 `DATA1.GRP` 是同一種東西」，而 `DATA0.GRP` 開頭是 `MZ`——
它是執行檔，那個槽根本沒有容器可索引。矛盾不在 `.IDX` 的語意，在我把
六個槽當成同一種格式。

**教訓（寫成規則，已進 `docs/formats/01` §5）**：一組看起來同構的檔案，
**先逐檔驗魔數再談共同格式**。命名一致（`DATA<n>.GRP`）不是同構的證據。

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
| `docs/re/` | 程式碼在哪（位址、bytes、xref）| `00` DOS 服務普查 |
| `docs/formats/` | 資料長什麼樣 | `01` 容器、`02` 郡名 |
| `docs/spec/` | `DRAFT`／`READY`／`CONFORMED`／`SUPERSEDED` | 空 |
| `docs/mechanics/` | 遊戲怎麼運作 | 空 |
| `docs/playtest/` | 原版 vs remake 同狀態比較 | 空 |
| `docs/reference/` | 說明書整理、社群資料 | `01-manual-*`（5 份）、`02-web-*`（4 份）|
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
- [x] 在 `dosgolem-san` 實作 `int 21h AH=01/07/08/0B` ＋ `KeyWaits` 計數
- [ ] **dosgolem 要有阻塞模型**：佇列空時停機並回報「在等鍵盤」，而不是回一個值繼續跑。
      這是 dosgolem 的設計決定（走它的 `docs/spec/`），不是本遊戲專屬。**目前擋住一切的就是這一步。**
- [ ] 停機模型做好後重跑 probe，觀測第一個畫面與開檔順序
- [ ] `10.GRP`／`20.GRP`／`D5.GRP` 三個變體逐位元組 diff——同一個容器的三份不同內容，是解格式最便宜的槓桿
- [x] 從資料檔找 42 個郡名 → `docs/formats/02`
- [ ] 從 `DATA2.GRP` 取 342 位人物表（每筆 30 byte），對回手冊附錄三
- [ ] 用 `unlzexe` 驗 `DATA4`／`DATA5.GRP` 的 LZEXE 假說（解壓後長度應為 810,396／457,424）
- [ ] 從執行檔／資料檔取 1728 組密碼表（§3.05）
- [ ] 複核手冊 p.42 洪水提升率的四個兩位數值（掃描字級極小）
- [x] 說明書整理成 `docs/reference/01-manual-*`（5 份，1,758 行）
- [ ] 社群資料整理成 `docs/reference/02-web-*`
- [x] **解 `.IDX` 索引什麼** → 結束位移，`docs/formats/01`
- [ ] `DATA2.GRP` 兩版逐位元組 diff，差異落點分佈

### 之後要回頭確認的

- `LICENSE` 的「灰色地帶」與「第三方素材」兩段目前是**照計畫填的**，
  發行前（M8）要用 `git ls-files` 對一次實際內容。
