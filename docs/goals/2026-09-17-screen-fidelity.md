# Goal：三國演義 remake 畫面一致性收尾（2026-09-17）

以 `wicanr2/softworld_san1_remake` 的 open Issues 為唯一工作權威。承接 `2026-09-15-fidelity-loop.md`
的 §1 基本規則、§3 開 Issue 範本、§4 收尾與 §2.5「規則層對拍到此為止、改做畫面一致性」，這裡不重抄。
repo 的 `CLAUDE.md`／`CONTEXT.md` 全部適用。

## 1. 起點（2026-09-17 盤點）

- open：#58、#56（agent-ready）；#35、#34、#33（needs-human）；#2（blocked）。
- 工作樹有 #58 的未提交修改（10 檔修改、3 檔新增），**`tools/go.sh build ./...` 失敗**：
  `internal/game/plot.go:182` 缺 `assets` import。先修到編得過，**不要 stash、不要 reset**。

## 2. 既定決策（2026-09-17 裁定，不重問）

| Issue | 決策 | 關閉 |
|---|---|---|
| #35 | 照原版當場問：戰鬥中插入「被擒處置」與「中途紮寨」兩個提示 | `completed` |
| #34 | 接：remake 開場放 `CMARKL`／`CMARKR` | `completed` |
| #33 | 延到 M7 再決定；這一輪不動 | 不關 |
| #2 | 維持 blocked，不碰憑證 | 不關 |

每條決策先在 Issue 留言記下裁定、改標 `agent-ready`（#33 不改），再動工。

## 3. 順序與各條要點

**#58 場景圖拉幕（先收尾現有工作）**
- 上一輪讀出的事實：`0x32e40` 是四個方向的拉幕，把 `SCG01`–`SCG31` 拉進 (432,80)／(432,120)／(448,268)，
  共 48 個呼叫端。這**推翻 Issue 本文「四種動畫」的前提**：留言更正，並把核實訊號改成
  `ui.NewSceneWipe` 存在、`TestZZSceneEffectMatchesTheOriginal` PASS。
- 還缺：`docs/spec/010` §1 的 48 呼叫端表（程式註解已引用，文件裡還沒有）；主畫面命令約 20 處呼叫端
  （徵兵、訓練、武裝、調整兵力、調動、運送、開墾、治水、築關、買賣米、登用、挖角、賞賜、賜物、撤職、
  釋放、軍師、太守、發動戰役、新君主、絕嗣）；尋訪 `0x1b912` 的圖號（`SCG11` 用途未知）；
  刪掉已無人用的 `PendingWipe`／`TakeWipe` 與其測試。
- 呼叫端每接一處都要保住骰序：`TestZZMonthParity`、`TestZZUnitAIDayParity` 兩版仍逐位元組／逐鏈相同。
- 這一條可以分兩個 commit（戰場＋事件、主畫面命令），兩個都推了才關。

**#35 被擒處置與中途紮寨**
- `cmd/san1` 的 `battleKey` 加兩個 waiting 狀態，選單文字取原版（`DS:0x7f9e`、`0x2731a`）。
- 拿掉 `Unit.Unplaced` 的自動擺放，以及「玩家捕獲留到戰後處置」的延後路徑；`docs/spec` 刪除對應的 remake 差異。
- 驗收：玩家捕獲的那一條路在 `SAN1_NORESYNC=1` 下逐鏈相同。

**#56 對戰子畫面**：照 Issue 驗收做，核實訊號是 `DrawSkirmish`，以及 `PlayerSkirmish` 不是 nil。

**#34 版權畫面**
- 先驗證 `CMARKL`／`CMARKR` 真的由 `COPYRIG.EXE` 載入（目前是 L2）。原版 `PLAY.BAT` 只跑 `AA`。
- 若確認，dosgolem 驅動 `COPYRIG.EXE` 存基準，remake 開場逐像素對拍，DOSBox-X 交叉驗一張。
- 缺服務就依 `CLAUDE.md` §4.1 補在 dosgolem-san 的分支上，**不推送**。
- 若 `COPYRIG.EXE` 不載入這兩張，停工並留言回報證據，不自行決定落點。

## 4. 普查

四條做完後，依前一份 §3「F. 畫面一致性」再普查一次，開前先查重；A／B／C 三軌仍然停開。

## 5. 完成條件

#58、#35、#56、#34 關閉並附收據；普查連續兩輪開不出新的 agent-ready；剩下的 open 只有 #33、#2 與 needs-human。
最終回覆列：已關 Issue 與證據、新的 needs-human 待答問題、dosgolem-san 未推 commit、Docker 與工作樹狀態。
