# 三國演義（智冠 1991, DOS）remake

## ⚡ 動手前先讀

- `CONTEXT.md` 是單一入口：現況、文件索引、術語表、oracle 優先序、
  **已被推翻的斷言清單**、worklist。對話被壓縮或新 session 接手時先讀它。
- 這份 `CLAUDE.md` 只放「不論做什麼都要遵守」的目標、原則與硬規則。
  **已解出的事實不寫在這裡**——寫進 `docs/`，索引在 `docs/INDEX.md`。
  規則裡不准出現不存在的檔案、目錄或工具（寫進去之前先 `ls`）。
- 全域規則的觸發表在 `~/.claude/rules/00-rules-index.md`；本專案幾乎每個子任務都會命中
  `re-retro-cht-rulebook`（skill）、`retro-remake-spec-gated-workflow`、
  `retro-remake-source-selection-and-byte-signatures`、
  `compiler-runtime-helper-fingerprints`、`rulebook/85`（授權）。**不要憑記憶跳過。**

---

## 1. 專案目標

把智冠科技（Softworld）1991 年的 DOS 遊戲《三國演義》（陳則孝）完整逆向，
在 **Go / Ebiten** 上乾淨重寫成跨平台引擎。定位是文化資產保存——
這是台灣自製 PC 遊戲的早期作品，保存價值高於玩法翻新。

四條主線：

1. **資料格式全解**：`DATA0`–`DATA5` 的 `.GRP`／`.IDX`／`.NAM` 三件套容器，
   以及原版獨有的 `10.GRP`／`20.GRP`／`D5.GRP`／`PARTNSAV.FIL`、
   加強版獨有的 `NAME001`–`NAME006.SHA`。
2. **規則還原**：內政、外交、戰爭、武將、事件、電腦勢力 AI。
3. **兩版並列**（使用者裁定 2026-09-06）：原版與加強版共用一套 `internal/rules`，
   版本差異用旗標切換。**旗標要切什麼，得先解出兩版差在哪才知道**——
   在那之前不要預先造旗標（§3.4）。
4. **英日多語系**（使用者裁定 2026-09-06）：原文是 Big5 繁中，
   翻譯方向與前幾個 remake **相反**——是從繁中往外翻，不是翻成繁中（§3.3）。

### 定位：還原 ＋ 現代化外殼

核心規則一律對齊原版。外殼允許現代化——高解析畫布、視窗縮放、現代操作、存檔相容——
但**每一項改動都要在文件裡標記為 remake 差異**，不得默默改動遊戲規則。

### 不做的事

- 不散布原版執行檔、資料檔、美術、音樂、字型，也不散布說明書掃描。
  公開產出只有引擎程式碼、工具、文件與譯文。
  `org_game/`、`workplace/`、`manual/`、`.i64`、解包後 binary 一律 gitignore。
- 不做玩法設計改動。這是保存專案。
- 不把「在 remake 裡看起來能玩」當成驗收；驗收是對原版 oracle（§4）。

---

## 2. `[HARD]` 素材

### 2.1 手上有什麼

兩份 js-dos bundle，2026-09-06 取自 `dos.zczc.cz`（實際檔案在 `binary.dos.lol`）。
來源、雜湊、取得方式記在 `org_game/MANIFEST.md`；`bundles/` 保留未動過的原始 zip。

| 版本 | 進入點 | 目錄 | zip SHA-256 |
|---|---|---|---|
| 原版 | `AA.EXE`（354,960 B）| `org_game/三國演義/` | `8fb4cd1f…52ecd7fd` |
| 加強版 | `ASV.EXE`（300,494 B）| `org_game/三國演義1加強版/` | `3261b288…b0d3d42d` |

**兩版的檔案差異已量過（L0）**，這是目前最硬的起手線索：

| 相同 | 不同 | 只在原版 | 只在加強版 |
|---|---|---|---|
| **全部 6 個 `.IDX`**、**全部 6 個 `.NAM`**、`DATA1/3/4.GRP`、`COPYRIG.EXE`、`README.*` | `DATA0.GRP`（42,488 → 87,696）、`DATA2.GRP`（同長度、內容不同）、`DATA5.GRP`（145,378 → 287,590）| `10.GRP`／`20.GRP`／`D5.GRP`、`10.BAT`／`20.BAT`／`PLAY.BAT`、`PARTNSAV.FIL` | `NAME001`–`NAME006.SHA`、`SV.COM`、`CHKLIST.CPS`、`ASV.PIF`／`SV.PIF`、`弧.TXT`（0 bytes）|

**容器格式已解**（`docs/formats/01`）：`.NAM` 每項 16 byte 是 DOS 8.3 檔名，
`.IDX` 每項 4 byte 是該項的**結束位移**，`.GRP` 是資料本體首尾相接。
DATA1／2／3 的 `.IDX` 末值精準等於 `.GRP` 長度，round-trip 通過。

**但 `DATA0`／`DATA4`／`DATA5` 的 `.GRP` 是 MZ 執行檔**，`DATA4.GRP` 帶著
未被抹掉的 `LZ91`（LZEXE 0.91）簽章。這三個槽的 `.IDX` 末值遠大於 `.GRP` 長度，
假說是它們打包過、`.IDX` 索引的是解開後的內容（`L2`，還沒驗）。

所以**「三件套」不是同一種東西**。解碼器一律先檢查 `MZ`：
看到 `MZ` 就不是資料本體。直接當容器切會切出一堆看起來像資料的垃圾，
而且不會報錯（§7 第 18 條）。

`DATA2.GRP` 兩版等長不同容，是逐位元組 diff 最划算的一份。

### 2.2 說明書

`manual/珍098-三國演義.rar`（46 頁掃描，gitignore）→ 解到 `workplace/manual/`。
整理成 markdown 放 `docs/reference/01-manual-*`，含**術語與譯名對照表**。

說明書是**術語的權威來源**：指令名、能力值名、介面用語一律以它為準，
英日譯名從這份出發。

**但人名與地名以遊戲資料為準，不以說明書為準。** 說明書是掃描轉錄，
會受印刷品質與辨識影響（已知案例：p.51「韓玄」印得像「韓亦」）；
遊戲資料是 `L0`，直接讀原始 bytes。兩者在**字**上衝突時一律採資料。
證據與案例見 `docs/formats/03` §4。

它的**規則與數值只是提示**，位階低於執行檔（§4）——1991 年的說明書
與最終出貨版本不一定一致。這份是**原版**的說明書（`CONTEXT.md` F13）。

### 2.3 中文與社群資料

`docs/reference/02-web-*`。位階最低，只能當「往哪裡找」的提示。

**⚠ 這一款最大的資料污染源是同名混淆**：搜尋「三國演義」會大量撈到
光榮《三國志》、以及智冠後續的同系列作品。判別線索是
智冠／Softworld、1991、陳則孝、`AA.EXE`／`ASV.EXE`。
**分不清是哪一款、哪一版的資料一律標存疑，不得當成事實引用。**

---

## 3. 這一款的四個特性，決定了做法

### 3.1 x86 DOS ＋ Microsoft C 6.0

兩支執行檔都帶 `MS Run-Time Library - Copyright (c) 1990, Microsoft Corp`
字串（L0），即 **Microsoft C 6.0**。不是 Borland——
`borland-win16-rtl-helpers.md` 與 `borland-tpov-overlay-re.md` 的判準**不適用**，
不要套過來。

- **先把 runtime 與產品碼分開**（`compiler-runtime-helper-fingerprints`）。
  MSC 6.0 的 CRT、`printf`／`scanf` 家族、記憶體配置、長整數與浮點模擬常式
  會佔掉可觀的函式數。先辨認再讀遊戲碼，不要一支一支硬讀。
- MSC 的 `rand()` 是已知的 LCG。**公式通用，進入點與狀態變數位址不通用**——
  這正是 dosgolem `docs/spec/006` 拆分層時踩出來的判準（§4.1）。
- 反組譯用 IDA Pro 9.4，image `ida-pro-9.4-idapython:locked-v1`，
  坑見 `~/.claude/knowledge-base/retro/ida-pro-9.4.md`。
  **優先 IDAPython 不寫 IDC；headless 的 `print` 不進 stdout，腳本一律寫檔**，
  輸出帶 probe（函式數、輸入 SHA-256），否則分不出「沒找到」與「沒跑到」。
  **不 grep `.asm`，查 `.i64`**；**⛔ 不在 IDA 裡改名**，語意是附加註記。

### 3.2 文字是 Big5

- 解碼一律用 **`cp950`**，不要用 `big5`（後者缺常用擴充字與造字區）。
- **不能用 regex 找控制碼**：Big5 第二 byte 落在 `0x40–0x7E`／`0xA1–0xFE`，
  含 `0x5C`（反斜線）、`0x7C`，regex 會切半中文字。**逐字掃描。**
- 智冠這個年代的遊戲常有**造字**（Big5 使用者造字區 `0xFA40–0xFEFE`），
  字模可能在 `.GRP` 裡。碰到解不出的碼位先往這裡找，不要當成亂碼跳過。
- **掃描結果要看完整輸出或分區統計，不看前 N 行就下結論。**
- 武將名、城名、指令名的對照表**從資料檔取，不憑印象編**。
  三國題材尤其危險：腦中已有的三國知識會讓錯誤看起來很合理（§7 第 5 條）。

### 3.3 翻譯方向與原本相反

前幾個 remake 是把日文翻成繁中。**這一款原文就是繁中**，
要做的是**抽字串 → 對外翻成英日**。差別在：

- **繁中不是翻譯，是原文。** `translations/zh-Hant.json` 是母本不是譯文，
  內容一律從遊戲資料取，不得潤飾、不得改字。
- 英日譯文要能對回原版槽位；**槽位長度限制是原版版面決定的**，
  英文常比中文長，塞不下的地方要在 `docs/spec/` 記為 remake 差異。
- 字型策略：**remake 自建字庫，不內嵌任何原版字模**。
  繁中走 `~/cht/cht_fonts/`；日文與英文另備自由授權字型。
  原版造字對回 Unicode 後由同一字庫供應。
- 排版分兩層：`internal/cells`（格寬、裝不裝得下、折行）**不依賴 Ebiten**，
  無頭環境能測；`internal/ui` 才拉進 Ebiten。

### 3.4 兩版並列，旗標只切量到的差異

規則層共用是現況：`state.Edition`（`base`／`plus`）只切**兩件已經量到的
事**——難度上限（10／20）與電腦諸侯的出兵係數表。規格在
`docs/spec/004`，證據在 `docs/mechanics/90` §6。

**規則不變**：
- 旗標只切量到的差異。**沒量到就不造欄位**——造出來的旗標會固定住一個
  還沒驗證的假設，而且之後沒有人分得出哪些是量的、哪些是猜的。
- 兩版都當一等公民抽檔、記雜湊，`workplace/orig/{base,plus}/` 分開放。
- 每一條機制斷言標明**在哪一版驗的**。跨版外推一律禁止（§7 第 9 條）。

比兩版的執行期資料段時，**兩份 dump 一定要取自同一個時刻**。原版自己
在開機與戰役中之間就有 94 個位元組不同，混進去看起來會和版本差異一模
一樣。正對照是 `TestZZDumpBaseAtBoot`。

---

## 4. Oracle 優先序

1. **dosgolem 對拍**（`~/cht/dosgolem-san`，見 §4.1 的分支）——
   最高位階。程序內跑原版、讀原版自己的變數、攔它自己的呼叫，
   判準是原版的資料不是像素。詳見 §4.1。
2. **執行檔反組譯**（`AA.EXE`／`ASV.EXE`，IDA 9.4）。
3. **DOSBox-X 實跑**——交叉驗證用。dosgolem 畫出來的每一張都要拿
   DOSBox-X 的索引截圖驗過才算數，否則是拿自己驗自己。
   設定見 `~/.claude/knowledge-base/retro/dosbox-game-configs.md`；
   **`cycles=auto` 是可重現性的敵人**，對拍一律固定 cycles
   （兩份 bundle 附的 `.jsdos/dosbox.conf` 都是 `cycles=auto`，不要直接拿來用）。

   工具在 `tools/`：`dosboxx.sh` 抓單張畫面，`dosboxx-record.sh` 拿按鍵
   腳本驅動並錄影、每一步存兩張（隔一秒，判斷畫面靜止）。
   比對在 `internal/parity` 的 `TestZZDosgolemMatchesDosbox`，
   做法與坑寫在 `docs/playtest/03`。
4. **說明書**：繁中原文的權威來源；規則與數值僅供提示（§2.2）。
5. **社群 wiki、攻略、影片**——最低，只能當提示（§2.3）。

### 4.1 `[HARD]` dosgolem 是主要驗證器

工作副本 `~/cht/dosgolem-san`，**開獨立分支，不要動上游的整合分支**
（上游現在整合到 `main`）。**這裡不寫分支名**——名字會隨工作階段換，
寫死就會過期；現況記在 `CONTEXT.md` §1，用之前跑
`git -C ~/cht/dosgolem-san branch --show-current` 問一次。

換 base 時預期會撞到 API 漂移：上游動得快，`oracle` 的簽章與欄位會變。
判準不是「編得過」是「對拍的數字沒變」——同一份原版、同一串按鍵，
換 base 前後讀到的值要逐項相同（2026-09-10 那次的紀錄在 `CONTEXT.md` §1）。
不要在 `~/cht/dosgolem` 本體上動——那份與其他遊戲的 session 共用。

分層照 `dosgolem/docs/spec/006`，判準是一句話：
**「換一支 binary 之後，這段程式碼還成立嗎？」**

| 層 | 這個專案要不要動 |
|---|---|
| `internal/cpu`／`dos`／`machine` | **缺什麼補什麼**。補完誰都受惠 |
| `oracle/` | 不動 |
| `runtime/msc/` | **要新建**。MSC 6.0 的 `rand()` LCG、堆疊框、遠近指標、浮點模擬慣例 |
| `apps/san1/` | **一定要自己寫**。兩版的位址、流程、攔截點 |

- **接新程式的第一件事是跑 `cmd/probe`**，它會列出「用到而還沒實作的服務」
  （`o.Unimplemented()`）。那份清單就是待辦，不要靠猜。
- **加強版有 `SV.COM`**：`.COM` 載入在 `logh3-com-support` 分支（`c868cd6`）已經做了，
  需要時 cherry-pick，不要重寫。
- **位址 per-binary，而且兩版不同。** `runtime/msc` 放演算法，
  位址由 `apps/san1` 用 config 給，原版與加強版各一份。
  拿錯版本的位址套過去**不會報錯，只會一次都攔不到**。
- 接法照 `rich2`：Go workspace（`go.work`，gitignore，只在容器內成立），
  **不要在 `go.mod` 裡 `replace` 或 `require`**——建置容器是 `--network none`，
  指向不存在的版本會讓整包編不過，錯誤訊息還指向無關的檔案。
  對拍測試放 `-tags oracle` 之下，沒掛載的人 `go test ./...` 會 skip 不會紅。

### 4.2 執行環境 `[HARD]`

- 分析、抽檔、建置、測試、模擬器、IDA 一律 Docker：
  `docker run --rm --network none --memory … --cpus … --pids-limit …
  --log-opt max-size=10m --log-opt max-file=3 -u $(id -u):$(id -g)`，原始輸入唯讀掛載。
- Go／Ebiten 建置 image 從 `rich2-go-ebiten:latest` 或
  `eob-remake-go:1.26.7-ebiten2.9.9` 起，本專案自建 `san1-go:<tag>`，寫進 `docker/`。
- 主機只做 git、檔案編輯、docker 控制。**不在主機 pip install、不建 venv。**
- **禁止任何 docker prune／rmi；只清理自己建立的 `san1-*` container。**

---

## 5. 流程閘門：RE 證據 → `READY` spec → 實作 → 同狀態驗證

照 `retro-remake-spec-gated-workflow`：

- `docs/re/`：程式碼在哪（位址、bytes、xref、推論等級）。
- `docs/formats/`：資料長什麼樣（每個檔一份，含 round-trip 驗證）。
- `docs/spec/`：`DRAFT`／`READY`／`CONFORMED`／`SUPERSEDED`；
  **只有 `READY` 能授權寫進 `internal/` 的行為**。假說不進 production code、不進 golden test。
- `docs/mechanics/`：這個遊戲怎麼運作（§6）。
- `docs/playtest/`：原版與 remake 同狀態比較，含存檔雜湊、序列、dosgolem 對拍矩陣。
- `worklist.json`：**待辦與完成度的權威**（`rulebook/61`）。每一條掛一個跑得
  起來的 `verify`，回答「這一條的 status 還成立嗎」——`done` 驗證據還在、
  `open` 驗未完成的訊號還在。**不要在 markdown 打勾**；跑
  `tools/worklist.py verify`。
- `VERIFICATION-MATRIX.md`：完成度的唯一數字來源；§8 由
  `tools/worklist.py render` 產生，其餘各節是敘述層（方法、坑、取捨）。
  README 只連過去。

**⚠ 最常漏的一步：解完機制只寫了 `docs/re/`。** `docs/re/` 與 `docs/mechanics/` 兩份都要，
實作完、測試完、commit 完都不代表這一步做了。

---

## 6. 機制文件 `[HARD]`

| 檔案 | 屬性 |
|---|---|
| `00-index.md` | 索引與狀態總表 |
| `10-strategy.md` | 回合流程、指令清單、內政 |
| `20-personnel.md` | 武將能力值、登用、忠誠、離反 |
| `30-diplomacy.md` | 同盟、外交判定 |
| `40-military.md` | 出兵、戰場、地形、一騎討、糧草 |
| `50-events.md` | 天災、事件、反亂 |
| `60-economy.md` | 金、糧、人口、開發值 |
| **`70-ai.md`** | **電腦君主決策**——優先度最高 |
| `80-victory.md` | 統一判定、君主繼承、劇本差異 |
| `90-version-diff.md` | **原版 vs 加強版逐項差異**（本專案特有）|

每條機制標推論等級、**在哪一版驗的**、以及驗證位置（哪支函式、哪次對拍）。
**不准為了完整而編機制。**

---

## 7. 從前幾個 remake 帶過來的教訓

1. **動手挖之前先 grep 自己的 `docs/`。** 落空 ≠ 不存在（正對照先做）。
2. **掃常數，不掃結果**；老軟體的數字是公式算的。
3. **不用 `grep -v` 過濾組語**；濾掉的是索引計算。要短用 `sed -n`。
4. **寫下一個值前先 grep 它**，與既有常數衝突是最便宜的錯誤偵測。
5. **對照表從資料取，不憑印象編。** 三國題材尤其危險——
   腦中已有的三國知識會讓錯的東西看起來很合理。
6. **一條規則只留一份實作**（`internal/rules/`），新增前先 grep。
7. **訂正比新結論需要更硬的證據**；推翻前先找出當初的證據。
8. **先量熵再說「壓縮／加密」。** `.GRP` 是容器，壓不壓縮要量過才知道。
9. **不跨檔案、不跨版本外推。** 這個專案有兩版，這條是硬傷區。
10. **編碼猜三次沒中就停手，讀反組譯。**
11. **原版哨兵值 ≠ Go 零值**（`0xFF` 常是「沒有」），在唯一入口正規化。
12. **開新遊戲是一等驗收路徑**，從存檔載入看不到缺口。
13. **畫面 bug 測試看不到**：排版溢出只有實跑抓得到。
14. **「AI 沒動」可能是正確原版行為**，先確認前提滿足。
15. **連續兩輪同類失敗 → `rulebook/40`／`41`**：固定參數重試不是實驗。
16. **統計特徵不是語意**；「像圖形」要用 palette 實際渲染看過才算。
17. **剛解出結論正要寫進既有文件時 → `rulebook/63`**：
    正文只寫現況，推翻紀錄集中一處，教訓寫成規則不寫成事件敘述。
18. **測試 skip 不是綠。** 需要原版素材的測試沒掛素材時會 skip，
    `go test ./...` 照樣印一片 `ok`——而真正在跑的只有不碰原版的那些。
    `tools/go.sh` 現在預設把 `org_game/` 掛進去；要跑無素材的那一組
    設 `SAN1_ORIG=`（空字串）。
19. **解壓／解碼錯一個位元組不會報錯**，只會讓多數數值碰巧是對的。
    判準要是「兩條路徑對同一份資料逐格相同」，不是「開局現金對得上畫面」。

---

## 8. 推論等級（每條斷言必帶）

| 等級 | 意思 |
|---|---|
| `L0` | 原版 bytes／反組譯直接讀出，附位址與 SHA-256 |
| `L1` | dosgolem 或實跑可重現（固定輸入＋序列＋結果雜湊）|
| `L2` | 多處間接證據一致的推論 |
| `L3` | 假說，只允許出現在 `DRAFT` spec 與「未解」表 |

斷言另標**版本**：`[base]`／`[plus]`／`[both]`。沒標版本的機制斷言視同未驗證。

---

## 9. 儲存庫與交付

- GitHub **private** repo `wicanr2/softworld_san1_remake`。
- **git 身分 `wicanr2@gmail.com`**（repo-local，全域是公司信箱，**不要動全域**）。
  進 repo 工作時先 `git config user.email`，再跑
  `git log --format=%ae | sort -u` 看歷史有沒有混進公司信箱。
- commit message **不放 `Claude-Session:` 連結**；`Co-Authored-By` 可以留。
- 授權 RRSAL-1.0（`rulebook/85`，著作權人 Wang Chun-Yu）。
- 目錄骨架：`cmd/`、`internal/`（`rules`／`state`／`ui`／`assets`／`cells`／`session`）、
  `tools/`、`docker/`、`docs/{re,formats,spec,mechanics,playtest,reference,release,design}`、
  `translations/`。
- README 依 `rulebook/80` 骨架；完成度要有分母；不叫 open source，叫 source-available。
- 發行包每一份都帶 `LICENSE`。

---

## 10. 里程碑（順序即優先序）

| # | 里程碑 | 出口條件 |
|---|---|---|
| M0 | 環境 ＋ 素材 | 兩版抽檔完成、每檔 SHA-256；`tools/ida.sh` 能對兩支 EXE 產 `.i64`；`dosgolem cmd/probe` 跑過兩支 EXE 並產出未實作服務清單 |
| M1 | dosgolem 跑得動 | 兩支 EXE 在 dosgolem 內開機到標題畫面，索引畫面與 DOSBox-X 逐點相同；`runtime/msc` 能攔到 `rand()` |
| M2 | 容器格式 | `.IDX` 索引什麼解出來（§2.1 的矛盾要有答案）、`.NAM`／`.GRP` 格式 READY、round-trip 驗證過 |
| M3 | 文字與字型 | Big5 字串全抽出、造字碼位處理完、`translations/zh-Hant.json` round-trip；CJK 畫布 |
| M4 | 靜態資料 | 武將表、城池表、劇本能對回 dosgolem 讀出的執行期記憶體，**逐格相同** |
| M5 | 規則層 | `docs/mechanics/10–80` 至少 L1，`internal/rules` 有對應單測；`70-ai` 先於實作；`90-version-diff` 有答案 |
| M6 | 引擎可玩 | 開新遊戲 → 一回合內政 → 一場戰爭 → 存讀檔，對原版同狀態對拍 |
| M7 | 多語系 | 英日譯文全覆蓋、槽位溢出處理完、校訂流程 |
| M8 | 發行 | Linux／Windows／macOS 封包、LICENSE、README、VERIFICATION-MATRIX |

每個里程碑的實際狀態只寫在 `CONTEXT.md` 與 `VERIFICATION-MATRIX.md`，這裡不更新進度。
