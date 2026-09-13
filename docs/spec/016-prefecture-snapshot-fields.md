# 016：州郡的兵士與現役將快照

狀態：`CONFORMED`

## 範圍

本規格只處理 `BASESTA` 州郡記錄的兩個執行期快照：offset 16 的兵士
（`u16`，實際兵數除以 100）與 offset 22 的現役武將數（`u8`）。不改人物
記錄的兵力、在野將數、所屬、主事者或存檔格式。

## 輸入與工具

- 原版 `AA.EXE` SHA-256：
  `474780e5be697b3b4899da5e0dbadd2f327e0bbe7e56306ac3b732a15fc124ca`。
- 版本：base。
- dosgolem-san commit：`d351681ba86d97aab571d00b979c36e2336486f3`。
- 原版位址均為目前文件採用的線性位址；州郡 offset 則是磁碟與執行期
  共用的記錄內位移。

## 證據審查

1. `0x1949e` 的「重整守將清單」先建立該郡現役名單，再把兵力合計除以
   100 寫到州郡 offset 16；每郡回合入口 `0x17471`、移防與調整兵力會
   呼叫它。等級 `L0`／`[base]`。
2. 登用常式先把當前名單人數寫到州郡 offset 22：
   `cec9: 26 88 8f 96 04  mov es:[bx+0x496],cl`；成功後再於 `d087`
   執行 `incb es:[si+0x496]`，同時於 `d0a4` 把新人的兵力除以 100 加到
   offset 16。等級 `L0`／`[base]`。
3. 第 1 格存檔有六個郡的兩欄快照與人物表即時計數不同。原版載入後只把
   月游標所指的玩家郡 41 刷新為 `5／1`，其餘仍保留存檔快照；這正是
   「存值＋特定邊界刷新」而非導出值。原先對拍把差異欄位覆寫後才比較，
   不能滿足完整 19,220-byte 載入驗收。等級 `L1`／`[base]`。

證據入口：`docs/re/03-main-program-code-map.md` §1.4–1.5、
`docs/mechanics/40-military.md` §3、`internal/parity/loadsave_oracle_test.go`。

## 型別與狀態轉移

- `game.Prefecture` 分別保存兵士快照與現役將快照。
- 從劇本或存檔建立局面時，兩欄直接取自原始 `BASESTA`，不得從人物表重算。
- `RefreshGarrison(郡)` 只在原版會重整守將清單的邊界呼叫，同時刷新兩欄。
- 登用常式按原版分開維護：檢查上限前刷新 offset 22；成功後 offset 22
  加一，offset 16 加上新人的兵力除以 100，不把整郡兵士提前重算。
- 原版載入 `BASEPRO` 後會由其中的月游標接回迴圈；游標若正停在玩家郡，
  先執行 `0x17471` 的重整再進主命令。remake 讀原版進度時只重現這個
  可證實的玩家停點，不把只刷新兩欄誤當成完整的電腦郡分派器。
- 即時規則若要知道人物表目前有幾名現役將，仍使用動態計數；寫回
  `BASESTA` offset 22 時只能使用快照。
- `Tables()` 不得因序列化而刷新快照；讀取或存檔不能偷偷改變局面。

## 失敗模式與驗收

- 快照未初始化，或漏掉讀檔後游標所指玩家郡的重整：剛載入即與原版不同。
- 在 `Tables()` 重算：出征後到下次重整前的原版可見舊值被消失。
- 只刷新兵士：回合入口後兩欄代表不同時間點。

驗收：`TestOriginalSaveLoadsIdentically` 不遮蔽任何欄位，原版載入第 1 格與
remake `save.ReadOriginal` 產生的三張表共 19,220 bytes 全部相同；月度對拍
仍須保持 0-byte 差異。

## 驗證收據（2026-09-13）

- `go test ./internal/game ./internal/save`：PASS。
- 使用既有 `rich2-go-ebiten:latest` 執行 `go test ./...`：PASS；所有套件
  完成，無非預期 skip。
- `go test -tags oracle ./internal/parity -run '^TestOriginalSaveLoadsIdentically$' -count=1 -v`：
  PASS；三張表 19,220 bytes，沒有遮蔽欄位。
- 第一輪無遮蔽測試曾精確顯示郡 41 的 offset 16／22 差 2 bytes：原版
  `5／1`、檔案與 remake `0／0`。依 `BASEPRO` 游標接回 `0x17471` 的順序補上
  玩家停點重整後，同一命令通過；這是失敗對照，不是挑樣本略過。
- `SAN1_SEED=0x13579BDF go test -tags oracle ./internal/parity -run '^TestZZMonthParity$' -count=1 -v`：
  PASS；對拍視窗 38 郡，原版與 remake 三張表差 0 bytes，人物差 0 位。

## 排除與停止線

- 不由此 Issue 推導 offset 23 在野將數的所有刷新時機。
- 不外推加強版；加強版需獨立同狀態證據。
- 不追逐鍵盤、PIT 或其他與兩欄狀態無關的硬體時序。
- 原版素材只作唯讀 oracle，不進版控與發行包。
