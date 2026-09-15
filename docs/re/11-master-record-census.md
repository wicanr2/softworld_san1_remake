# 諸侯記錄（`BASEMAS`）的讀寫端普查

日期：2026-09-15
工具：`internal/parity/mascensus_oracle_test.go`（`TestZZMasterRecordAccessCensus`／
`…CensusPlus`）、`tools/mascensus_disasm.py`、`tools/mascensus_static.py`
版本：`[both]`。原版 `AA.EXE` SHA-256 `474780e5be697b3b4899da5e0dbadd2f327e0bbe7e56306ac3b732a15fc124ca`，
加強版 `ASV.EXE` SHA-256 `ad18a251fece9b7b…`（`docs/re/01` §3）。

回答的是一個問題：**72 個位元組裡，原版到底碰過哪幾個。**
答案：兩版都只碰 `0–9`、`14–18` 這 15 個；其餘 57 個在 54／59 個月的實跑裡
零存取，碼段裡也沒有以表段為 `ES` 的 `es:[reg+N]` 存取。

## 1. 為什麼要用執行期監看

三張表全部以結構基底加位移取用（`mov ax, es:[bx+8]`，`bx` ＝ 72 × 槽號），
靜態交叉參考一筆都對不到諸侯表（`docs/re/03` §1.1）。所以判準改成
**原版自己的存取**：在整張諸侯表（1,152 個位元組）掛 dosgolem 的
`WatchReadsAt`／`WatchWritesAt`，每一次讀寫記下「距表頭幾個位元組」與
那一刻的 `CS:IP`，依「記錄內位移 ＝ 距離 mod 72 × 指令位址」歸類。

`CS:IP` 是**存取發生那一刻**的值：CPU 取完 opcode／modrm／位移才碰記憶體，
所以記到的是存取指令的**結尾**（帶立即值的形式如 `imul ax, es:[bx+2], 0x1e`
則停在結尾前一個位元組）。對回指令時拿同一次執行倒出的碼段
（`0xb000`–`0x50000`）線性反組譯，挑該位址落在其範圍內、帶 `es:` 的那一道
（`tools/mascensus_disasm.py`）。

## 2. 怎麼跑

從進度 1 載入（原版走 `bootToGame`，加強版走 `bootLikePlus`），玩家每個月
「內政 → 休息 → Y」，電腦諸侯照常行動，60 輪：

| | 原版 | 加強版 |
|---|---|---|
| 三張表基底（線性）| `0x399b0` | `0x36200`（搜進度 1 的諸侯表得到，只命中一處）|
| 走過的月份 | 54（有幾個月被電腦間戰役佔掉）| 59 |
| 讀／寫次數 | 41,723／433 | 22,718／41 |
| 存取端 | 107 | 49 |
| 實跑時間 | 15 分 50 秒 | 16 分 41 秒 |

加強版跑兩次的讀寫次數逐次相同（22,718／41），執行是決定性的。

月份用**月底結算入口**的呼叫次數數，位址 per-binary：原版 `0x1581c`
（`docs/re/06` §9.5），加強版 `0x14878`。後者是拿原版的指令形狀序列
（把立即值抹成 `#` 之後連續十道）在加強版碼段裡比出來的，只命中一處；
同一個方法也找出「這個月處理到第幾格」的游標——原版
`DS:[0xa726]` 段的 `0x20f4`，加強版 `DS:[0xa8f8]` 段的 `0x20f6`。
⚠ 第一輪把原版的兩個位址直接套在加強版上：**不報錯，月底結算 0 次、
游標讀出 772**——`CLAUDE.md` §4.1 說的那種假零。

## 3. 碰過的位移（`L1`、`[both]`）

兩版碰到的位移集合相同：`0–9`、`14–18`，全部是 `docs/spec/003` §4 已解的欄位。
原版另有 offset 70 被讀 2 次，那是**越界不是欄位**（§5）。

逐欄位的讀寫端數（不同指令位址的個數）：

| 欄位 | 原版 讀／寫 | 加強版 讀／寫 | 寫入端（原版）|
|---|---|---|---|
| 0–1 操縱方 | 11／0 | 6／0 | — |
| 2–3 君主 | 19／1 | 12／0 | `0x14cc3`（繼承）|
| 4–5 AI 等級 | 1／0 | 1／0 | — |
| 6–7 軍師 | 10／1 | 8／0 | `0x0d8b0`（指定軍師）|
| 8–9 人望 | 19／4 | 10／1 | `0x204ba`／`0x204e6`（戰役勝負 ±2）、`0x14c89`（繼承折損）、**`0x16e5e`（秋季年度調整，見 §6）**；加強版 `0x15cc0` 同形狀 |
| 14 玉璽 | 2／0 | 1／0 | 春季事件的寫入這次沒發生 |
| 15 兵書 | 7／4 | 2／1 | `0x0dab7`（賞賜）、`0x1733a`（進貢）、`0x26d9b`／`0x26db5`（戰役）|
| 16 寶刀 | 8／5 | 3／2 | `0x0dc15`、`0x1733a`、`0x26c57`／`0x26e15`／`0x26e2f` |
| 17 美女 | 7／4 | 2／1 | `0x0dd8b`、`0x1733a`、`0x26e9c`／`0x26eb6` |
| 18 駿馬 | 7／4 | 3／2 | `0x0df3a`、`0x1733a`、`0x26f16`／`0x26f30` |

## 4. 沒碰過的 57 個位移：靜態補掃

動態只看得到跑過的碼。補掃拿同一份碼段做線性反組譯，列出所有帶 `es:`
段覆寫、基底暫存器加位移、位移落在未解集合（10–13、19–71）的存取，
並往回找 `ES` 最近一次從哪個 DGROUP 變數載入（`tools/mascensus_static.py`）。
表段變數是從動態命中點往回找出來的：原版 `0xa602`／`0xa61a`／`0xa628`／
`0xa734`／`0xa9dc`／`0xaa94`，加強版 `0xa794`／`0xa7be`／`0xa7d8`／`0xa7e8`／
`0xa906`／`0xabda`／`0xac94`。

兩版結果相同，只有三個位移有形狀，而且 `ES` 都不是表段：

| 位移 | 原版 | 加強版 | 是什麼 |
|---:|---|---|---|
| 10 | `0x3967c` `mov dx, es:[di+0xa]` | `0x35ecc` | 落在表格區前面的資料，不是碼 |
| 12 | `0x1067e`／`0x10795` `les bx, es:[bx+0xc]` | `0x0ff3e`／`0x10047` | 指標鏈，MSC 執行期 |
| 32 | `0x14209`／`0x1e554`／`0x1e8d2` `mov es:[bx+si+0x20], al` | `0x13518`／`0x1c812`／`0x1cb48` | `ES` 來自 `0xa6d6`／`0xa7dc`（原版），非表段 |

其餘 54 個位移連 `es:[reg+N]` 的形狀都沒有。

**這不是「證明是填充」。** 它證明的是：兩版在含戰役、四季事件、進貢、
繼承的實跑裡沒有讀寫，碼段裡也沒有靜態可見的存取。沒跑到的路徑
（自創君主、存讀檔的整筆搬運、結局）不在裡面；整筆搬運碰的是全部 72 個
位元組，本來就不是欄位語意。結論足以判定**不進 `state`／save schema**。

## 5. offset 70：州郡 −1 的越界（`L1`、`[base]`）

原版 `0x20344`／`0x2035e`（戰鬥子系統 `0x20200` 的開頭，`docs/re/05` §1）：

```
2033c:  mov ax, 0xb0            ; 176
2033f:  imul word [bp+0xc]      ; × 郡號（第三個參數：守方援郡）
20342:  mov bx, ax
20344:  mov al, es:[bx+0x49e]   ; 州郡 offset 30 ＝ 所屬勢力
```

援軍格填 `0xFFFF`（玩家的「發動戰役」兩個援軍格都是，`docs/re/03` §1.5），
`176 × (−1) + 0x49e` 在 16 位元裡繞回成 `0x03ee` ＝ 1006 ＝ 13 × 72 ＋ 70：
讀到的是**諸侯槽 13 的 offset 70**。原版沒有先檢查 `0xFFFF` 就讀了；
之後才用那個值。這次執行各命中 1 次——54 個月裡有一場兩邊都沒有援軍的
戰役走進了戰鬥子系統；加強版那次沒有戰役走到這裡（`0x1f000`–`0x2a000`
一帶零命中），但同一段碼形狀相同。

remake 不需要重現這個讀取——它讀到的是 0，而援軍格為空的分支後面另有判斷。
記在這裡是因為**任何拿諸侯表做記憶體對拍的人都會看到 offset 70 有讀取**，
不先知道這件事會把它當成欄位。

## 6. 順帶量到：人望每年秋季會調整（`L0`，未入 mechanics）

寫入端 `0x16e59`（迴圈 `0x16d4f`–`0x16e6a`，接在蝗害之後、冬季 `0x16e70` 之前）
是一條**沒有記在 `docs/mechanics` 的機制**：

```
兩張 16 格的表歸零（0x16d54）
for 郡 = 1..42：所屬 == 0xFF → 跳過
    領地數[所屬]++                                  ; es:0x24b2
    和[所屬] += 民眾忠誠(offset 26)÷2 + 土地價值(offset 27)÷2 ; es:0x2e36
for 勢力 = 0..15：領地數 <= 0 → 跳過
    d = 君主魅力(人物 offset 11) + 和 ÷ 領地數 − 100
    d = d ÷ 32（向零）                              ; 0x16e20–0x16e2c
    人望 = clamp(人望 + d, 0, 100)                  ; 0x16e59
```

與進貢用的那兩張表（`docs/re/06` §9，係數 ÷4、÷2）是**同一對緩衝區、
不同的係數**。這條的驗證與 remake 實作另開 Issue，不在本文範圍。

## 7. 對讀表：原版

存取那一刻的 `CS:IP` 是存取指令的結尾（§1）。`u16` 欄位只列一次。

| 位移 | 讀／寫 | 存取那一刻的 CS:IP（線性） | 指令 | 次數 | 槽數 |
|---:|---|---|---|---:|---:|
| 0 | r | `0x0d682` | `cmp    WORD PTR es:[bx+0x0],0x1` | 4228 | 12 |
| 0 | r | `0x0d7df` | `cmp    WORD PTR es:[bx+0x0],0x1` | 4228 | 12 |
| 0 | r | `0x14c2a` | `cmp    WORD PTR es:[bx+0x0],0x2` | 4 | 2 |
| 0 | r | `0x14e04` | `cmp    WORD PTR es:[bx+0x0],0xffff` | 2 | 1 |
| 0 | r | `0x15951` | `cmp    WORD PTR es:[bx+0x0],0x1` | 1620 | 15 |
| 0 | r | `0x17261` | `cmp    WORD PTR es:[bx+0x0],0x1` | 68 | 8 |
| 0 | r | `0x174ea` | `cmp    WORD PTR es:[bx+0x0],0x2` | 4336 | 13 |
| 0 | r | `0x1d6c6` | `cmp    WORD PTR es:[bx+0x0],0x1` | 62 | 6 |
| 0 | r | `0x203ca` | `cmp    WORD PTR es:[bx+0x0],0x1` | 2 | 1 |
| 0 | r | `0x203f0` | `cmp    WORD PTR es:[bx+0x0],0x1` | 2 | 1 |
| 0 | r | `0x20897` | `cmp    WORD PTR es:[bx+0x0],0x1` | 4 | 2 |
| 2 | r | `0x0e35c` | `imul   WORD PTR es:[bx+0x2]` | 536 | 4 |
| 2 | r | `0x0e404` | `imul   WORD PTR es:[bx+0x2]` | 3692 | 8 |
| 2 | w | `0x14cc3` | `mov    WORD PTR es:[bx+0x2],ax` | 2 | 2 |
| 2 | r | `0x14e57` | `imul   WORD PTR es:[bx+0x2]` | 2 | 1 |
| 2 | r | `0x15959` | `cmp    WORD PTR es:[bx+0x2],0xffff` | 108 | 1 |
| 2 | r | `0x15cda` | `cmp    WORD PTR es:[bx+0x2],0xffff` | 160 | 16 |
| 2 | r | `0x15e22` | `imul   WORD PTR es:[bx+0x2]` | 2 | 1 |
| 2 | r | `0x160ca` | `imul   WORD PTR es:[si+0x2]` | 12 | 4 |
| 2 | r | `0x161ed` | `imul   WORD PTR es:[si+0x2]` | 12 | 4 |
| 2 | r | `0x16dfa` | `imul   WORD PTR es:[si+0x2]` | 104 | 13 |
| 2 | r | `0x176b2` | `imul   WORD PTR es:[bx+0x2]` | 114 | 1 |
| 2 | r | `0x1dc8e` | `imul   WORD PTR es:[bx+0x2]` | 138 | 9 |
| 2 | r | `0x1decc` | `imul   WORD PTR es:[si+0x2]` | 60 | 9 |
| 2 | r | `0x1df11` | `imul   WORD PTR es:[bx+0x2]` | 60 | 10 |
| 2 | r | `0x26d14` | `imul   WORD PTR es:[bx+0x2]` | 2 | 1 |
| 2 | r | `0x2d2d7` | `imul   WORD PTR es:[bx+0x2]` | 80 | 11 |
| 2 | r | `0x2ddb0` | `imul   WORD PTR es:[bx+0x2]` | 144 | 8 |
| 2 | r | `0x2de2b` | `imul   WORD PTR es:[bx+0x2]` | 144 | 12 |
| 2 | r | `0x2de84` | `imul   WORD PTR es:[bx+0x2]` | 144 | 12 |
| 2 | r | `0x333ae` | `imul   WORD PTR es:[bx+0x2]` | 114 | 1 |
| 4 | r | `0x1752d` | `mov    ax,WORD PTR es:[bx+0x4]` | 4228 | 12 |
| 6 | r | `0x0d80c` | `mov    ax,WORD PTR es:[bx+0x6]` | 4228 | 12 |
| 6 | w | `0x0d8b0` | `mov    WORD PTR es:[bx+0x6],ax` | 4 | 4 |
| 6 | r | `0x0e563` | `cmp    WORD PTR es:[bx+0x6],0xffff` | 70 | 4 |
| 6 | r | `0x0e56d` | `imul   WORD PTR es:[bx+0x6]` | 40 | 2 |
| 6 | r | `0x0e5e3` | `cmp    WORD PTR es:[bx+0x6],0xffff` | 676 | 8 |
| 6 | r | `0x0e5ed` | `imul   WORD PTR es:[bx+0x6]` | 648 | 7 |
| 6 | r | `0x175b7` | `mov    ax,WORD PTR es:[bx+0x6]` | 108 | 1 |
| 6 | r | `0x2dd93` | `imul   WORD PTR es:[bx+0x6]` | 144 | 8 |
| 6 | r | `0x2de44` | `cmp    WORD PTR es:[bx+0x6],0xffff` | 144 | 12 |
| 6 | r | `0x2de50` | `imul   WORD PTR es:[bx+0x6]` | 114 | 9 |
| 6 | r | `0x33427` | `mov    ax,WORD PTR es:[bx+0x6]` | 114 | 1 |
| 8 | r | `0x0d006` | `mov    ax,WORD PTR es:[bx+0x8]` | 226 | 10 |
| 8 | w | `0x14c89` | `mov    WORD PTR es:[si+0x8],ax` | 2 | 2 |
| 8 | r | `0x15fc7` | `mov    ax,WORD PTR es:[bx+0x8]` | 2574 | 13 |
| 8 | r | `0x16e37` | `add    ax,WORD PTR es:[si+0x8]` | 104 | 13 |
| 8 | w | `0x16e5e` | `mov    WORD PTR es:[bx+0x8],ax` | 47 | 12 |
| 8 | r | `0x1dca9` | `mov    cx,WORD PTR es:[bx+0x8]` | 138 | 9 |
| 8 | r | `0x1dd03` | `cmp    WORD PTR es:[bx+0x8],0x32` | 138 | 11 |
| 8 | r | `0x1dd0a` | `mov    ax,WORD PTR es:[bx+0x8]` | 138 | 11 |
| 8 | r | `0x1dd47` | `cmp    WORD PTR es:[si+0x8],cx` | 138 | 11 |
| 8 | r | `0x1dd5f` | `mov    ax,WORD PTR es:[bx+0x8]` | 22 | 1 |
| 8 | r | `0x1de16` | `mov    ax,WORD PTR es:[bx+0x8]` | 60 | 9 |
| 8 | r | `0x1e0cc` | `mov    ax,WORD PTR es:[bx+0x8]` | 6 | 1 |
| 8 | r | `0x20039` | `cmp    WORD PTR es:[bx+0x8],ax` | 4 | 1 |
| 8 | r | `0x2004b` | `mov    ax,WORD PTR es:[bx+0x8]` | 4 | 1 |
| 8 | r | `0x204ba` | `add    WORD PTR es:[bx+0x8],0x2` | 2 | 1 |
| 8 | w | `0x204ba` | `add    WORD PTR es:[bx+0x8],0x2` | 1 | 1 |
| 8 | r | `0x204c0` | `cmp    WORD PTR es:[bx+0x8],0x64` | 2 | 1 |
| 8 | r | `0x204e6` | `sub    WORD PTR es:[bx+0x8],0x2` | 2 | 1 |
| 8 | w | `0x204e6` | `sub    WORD PTR es:[bx+0x8],0x2` | 1 | 1 |
| 8 | r | `0x2dde2` | `cmp    WORD PTR es:[bx+0x8],0x50` | 144 | 8 |
| 8 | r | `0x2dde9` | `mov    ax,WORD PTR es:[bx+0x8]` | 96 | 5 |
| 8 | r | `0x333dc` | `push   WORD PTR es:[bx+0x8]` | 114 | 1 |
| 8 | r | `0x38dc2` | `lods   ax,WORD PTR es:[si]` | 4 | 2 |
| 14 | r | `0x14ea5` | `cmp    BYTE PTR es:[si+0xe],0x0` | 1 | 1 |
| 14 | r | `0x1ddc5` | `cmp    BYTE PTR es:[bx+0xe],0x0` | 69 | 11 |
| 15 | r | `0x0d9b3` | `mov    al,BYTE PTR es:[bx+0xf]` | 1663 | 12 |
| 15 | r | `0x0dab7` | `dec    BYTE PTR es:[bx+0xf]` | 69 | 7 |
| 15 | w | `0x0dab7` | `dec    BYTE PTR es:[bx+0xf]` | 69 | 7 |
| 15 | r | `0x14f19` | `mov    al,BYTE PTR es:[bx+0xe]` | 4 | 1 |
| 15 | r | `0x17311` | `mov    al,BYTE PTR es:[bx+0xf]` | 136 | 8 |
| 15 | w | `0x1733a` | `mov    BYTE PTR es:[bx+0xf],al` | 105 | 8 |
| 15 | r | `0x26c5e` | `mov    al,BYTE PTR es:[bx+0xf]` | 1 | 1 |
| 15 | r | `0x26d7a` | `mov    al,BYTE PTR es:[si+0xf]` | 1 | 1 |
| 15 | w | `0x26d9b` | `mov    BYTE PTR es:[bx+0xf],al` | 1 | 1 |
| 15 | r | `0x26db5` | `sub    BYTE PTR es:[bx+0xf],al` | 1 | 1 |
| 15 | w | `0x26db5` | `sub    BYTE PTR es:[bx+0xf],al` | 1 | 1 |
| 16 | r | `0x0db11` | `mov    al,BYTE PTR es:[bx+0x10]` | 1660 | 12 |
| 16 | r | `0x0dc15` | `dec    BYTE PTR es:[bx+0x10]` | 69 | 8 |
| 16 | w | `0x0dc15` | `dec    BYTE PTR es:[bx+0x10]` | 69 | 8 |
| 16 | r | `0x26c57` | `add    BYTE PTR es:[bx+0xf],0x2` | 1 | 1 |
| 16 | w | `0x26c57` | `add    BYTE PTR es:[bx+0xf],0x2` | 1 | 1 |
| 16 | r | `0x26c8b` | `mov    al,BYTE PTR es:[bx+0x10]` | 1 | 1 |
| 16 | r | `0x26df4` | `mov    al,BYTE PTR es:[bx+0x10]` | 1 | 1 |
| 16 | w | `0x26e15` | `mov    BYTE PTR es:[bx+0x10],al` | 1 | 1 |
| 16 | r | `0x26e2f` | `sub    BYTE PTR es:[bx+0x10],al` | 1 | 1 |
| 16 | w | `0x26e2f` | `sub    BYTE PTR es:[bx+0x10],al` | 1 | 1 |
| 17 | r | `0x0dc6f` | `mov    al,BYTE PTR es:[bx+0x11]` | 1658 | 12 |
| 17 | r | `0x0dd8b` | `dec    BYTE PTR es:[bx+0x11]` | 58 | 7 |
| 17 | w | `0x0dd8b` | `dec    BYTE PTR es:[bx+0x11]` | 58 | 7 |
| 17 | r | `0x26cb8` | `mov    al,BYTE PTR es:[bx+0x11]` | 1 | 1 |
| 17 | r | `0x26e7b` | `mov    al,BYTE PTR es:[bx+0x11]` | 1 | 1 |
| 17 | w | `0x26e9c` | `mov    BYTE PTR es:[bx+0x11],al` | 1 | 1 |
| 17 | r | `0x26eb6` | `sub    BYTE PTR es:[bx+0x11],al` | 1 | 1 |
| 17 | w | `0x26eb6` | `sub    BYTE PTR es:[bx+0x11],al` | 1 | 1 |
| 18 | r | `0x0dde5` | `mov    al,BYTE PTR es:[bx+0x12]` | 1641 | 12 |
| 18 | r | `0x0df3a` | `dec    BYTE PTR es:[bx+0x12]` | 66 | 8 |
| 18 | w | `0x0df3a` | `dec    BYTE PTR es:[bx+0x12]` | 66 | 8 |
| 18 | r | `0x26ce5` | `mov    al,BYTE PTR es:[bx+0x12]` | 1 | 1 |
| 18 | r | `0x26ef5` | `mov    al,BYTE PTR es:[bx+0x12]` | 1 | 1 |
| 18 | w | `0x26f16` | `mov    BYTE PTR es:[bx+0x12],al` | 1 | 1 |
| 18 | r | `0x26f30` | `sub    BYTE PTR es:[bx+0x12],al` | 1 | 1 |
| 18 | w | `0x26f30` | `sub    BYTE PTR es:[bx+0x12],al` | 1 | 1 |
| 70 | r | `0x20349` | `mov    al,BYTE PTR es:[bx+0x49e]` | 1 | 1 |
| 70 | r | `0x20363` | `mov    al,BYTE PTR es:[bx+0x49e]` | 1 | 1 |

## 8. 對讀表：加強版

| 位移 | 讀／寫 | 存取那一刻的 CS:IP（線性） | 指令 | 次數 | 槽數 |
|---:|---|---|---|---:|---:|
| 0 | r | `0x0bf5d` | `cmp    WORD PTR es:[bx+0x0],0x2` | 2252 | 1 |
| 0 | r | `0x0d39f` | `cmp    WORD PTR es:[bx+0x0],0x1` | 4868 | 2 |
| 0 | r | `0x0d4d8` | `cmp    WORD PTR es:[bx+0x0],0x1` | 4868 | 2 |
| 0 | r | `0x14993` | `cmp    WORD PTR es:[bx+0x0],0x1` | 118 | 1 |
| 0 | r | `0x16079` | `cmp    WORD PTR es:[bx+0x0],0x1` | 16 | 2 |
| 0 | r | `0x162c6` | `cmp    WORD PTR es:[bx+0x0],0x2` | 4988 | 2 |
| 2 | r | `0x0dfca` | `imul   bx,WORD PTR es:[bx+0x2],0x1e` | 356 | 1 |
| 2 | r | `0x1499b` | `cmp    WORD PTR es:[bx+0x2],0xffff` | 118 | 1 |
| 2 | r | `0x14cbc` | `cmp    WORD PTR es:[bx+0x2],0xffff` | 160 | 16 |
| 2 | r | `0x14ded` | `imul   ax,WORD PTR es:[bx+0x2],0x1e` | 14 | 1 |
| 2 | r | `0x15c63` | `imul   di,WORD PTR es:[si+0x2],0x1e` | 20 | 2 |
| 2 | r | `0x160c7` | `imul   ax,WORD PTR es:[si+0x2],0x1e` | 8 | 1 |
| 2 | r | `0x16462` | `imul   ax,WORD PTR es:[bx+0x2],0x1e` | 120 | 1 |
| 2 | r | `0x1c028` | `imul   si,WORD PTR es:[bx+0x2],0x1e` | 12 | 1 |
| 2 | r | `0x2ad82` | `imul   bx,WORD PTR es:[bx+0x2],0x1e` | 22 | 1 |
| 2 | r | `0x2adf5` | `imul   si,WORD PTR es:[bx+0x2],0x1e` | 22 | 1 |
| 2 | r | `0x2ae43` | `imul   bx,WORD PTR es:[bx+0x2],0x1e` | 22 | 1 |
| 2 | r | `0x2fec6` | `imul   ax,WORD PTR es:[bx+0x2],0x1e` | 120 | 1 |
| 4 | r | `0x1630c` | `mov    ax,WORD PTR es:[bx+0x4]` | 356 | 1 |
| 6 | r | `0x0d501` | `mov    ax,WORD PTR es:[bx+0x6]` | 356 | 1 |
| 6 | r | `0x0e182` | `cmp    WORD PTR es:[bx+0x6],0xffff` | 68 | 1 |
| 6 | r | `0x0e189` | `imul   bx,WORD PTR es:[bx+0x6],0x1e` | 68 | 1 |
| 6 | r | `0x16388` | `mov    ax,WORD PTR es:[bx+0x6]` | 120 | 1 |
| 6 | r | `0x2ad6b` | `imul   si,WORD PTR es:[bx+0x6],0x1e` | 22 | 1 |
| 6 | r | `0x2ae0d` | `cmp    WORD PTR es:[bx+0x6],0xffff` | 22 | 1 |
| 6 | r | `0x2ae14` | `imul   bx,WORD PTR es:[bx+0x6],0x1e` | 22 | 1 |
| 6 | r | `0x2ff30` | `mov    ax,WORD PTR es:[bx+0x6]` | 120 | 1 |
| 8 | r | `0x0cd69` | `mov    ax,WORD PTR es:[bx+0x8]` | 20 | 1 |
| 8 | r | `0x14f64` | `mov    ax,WORD PTR es:[bx+0x8]` | 2602 | 2 |
| 8 | r | `0x15c9d` | `add    ax,WORD PTR es:[si+0x8]` | 20 | 2 |
| 8 | w | `0x15cc0` | `mov    WORD PTR es:[bx+0x8],ax` | 5 | 1 |
| 8 | r | `0x1c042` | `mov    cx,WORD PTR es:[bx+0x8]` | 12 | 1 |
| 8 | r | `0x1c097` | `cmp    WORD PTR es:[bx+0x8],0x32` | 12 | 1 |
| 8 | r | `0x1c09e` | `mov    ax,WORD PTR es:[bx+0x8]` | 12 | 1 |
| 8 | r | `0x1c0d5` | `cmp    WORD PTR es:[si+0x8],cx` | 12 | 1 |
| 8 | r | `0x1c0e8` | `mov    ax,WORD PTR es:[bx+0x8]` | 12 | 1 |
| 8 | r | `0x2adb2` | `cmp    WORD PTR es:[bx+0x8],0x50` | 22 | 1 |
| 8 | r | `0x2fef1` | `push   WORD PTR es:[bx+0x8]` | 120 | 1 |
| 14 | r | `0x1c142` | `cmp    BYTE PTR es:[bx+0xe],0x0` | 6 | 1 |
| 15 | r | `0x0d684` | `mov    al,BYTE PTR es:[bx+0xf]` | 146 | 1 |
| 15 | r | `0x1610e` | `mov    al,BYTE PTR es:[bx+0xf]` | 32 | 2 |
| 15 | w | `0x16133` | `mov    BYTE PTR es:[bx+0xf],al` | 26 | 2 |
| 16 | r | `0x0d7b6` | `mov    al,BYTE PTR es:[bx+0x10]` | 132 | 1 |
| 16 | r | `0x0d89b` | `dec    BYTE PTR es:[bx+0x10]` | 7 | 1 |
| 16 | w | `0x0d89b` | `dec    BYTE PTR es:[bx+0x10]` | 7 | 1 |
| 17 | r | `0x0d8e8` | `mov    al,BYTE PTR es:[bx+0x11]` | 153 | 1 |
| 18 | r | `0x0da2e` | `mov    al,BYTE PTR es:[bx+0x12]` | 137 | 1 |
| 18 | r | `0x0db5a` | `dec    BYTE PTR es:[bx+0x12]` | 3 | 1 |
| 18 | w | `0x0db5a` | `dec    BYTE PTR es:[bx+0x12]` | 3 | 1 |
