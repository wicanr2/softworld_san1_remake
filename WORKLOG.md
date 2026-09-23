# 工作歷程

目前狀態以 [`CONTEXT.md`](CONTEXT.md) 為準；本檔按日期記錄已做的工作與驗證。

## 2026-09-23：玩家戰役結算與玉璽狀態收斂

- 目標：釐清 GitHub Issues #102、#103 指出的玩家戰後安置、錢糧、援軍與寶物缺口，核對兩版原版程式碼，將已證實規則接入 remake。
- 原版證據：IDA Pro 9.4 在一次性資料庫以執行期線性位址讀 `AA.EXE`、`ASV.EXE`。四支玩家收尾常式、強制退兵、戰場郡安置與四類寶物分贓記在 [`docs/re/05`](docs/re/05-battle-layer.md) §14；輸入 SHA-256、工具與位址基準也在該節。固定原版 seed `0x13579bdf` 的 `TestBattleFinishesWithPlayer` 走正常玩家戰役，量到三支收尾呼叫各一次、戰場郡金米與四項受損值，以及繼承後四類寶物 38→40、玉璽兩側不變。
- 程式：依 [`docs/spec/020`](docs/spec/020-player-battle-settlement.md) 的 `READY` 規則接入敗軍強制退兵、勝方主軍／援軍安置、隨軍錢糧、玩家戰場受損與退場君主分贓；移除沒有原版位址的 `takePrefecture`／整份搬走五件寶物規則。修正出征整編對四支軍團人物所在郡的清空與來源郡重整。
- 勘誤：舊說「退兵去處不寫戰略層」漏掉 `0x23dd4` 下層 `0x24318` 的人物 offset 19 寫入，現已修正 `docs/mechanics/40`、歷史 `worklist.json` 與由其產生的驗證矩陣。舊說「滅國時玉璽全數轉手」只憑說明書；原版四類分贓不碰玉璽，絕嗣可清寶庫但獨立的已現世旗標不倒退。已讓 `BASEPRO` offset `0xC4`、`game.State`、存讀檔維持該狀態，並移除未使用且內容錯誤的三語戰報字串。
- 驗證：`tools/go.sh test ./... -count=1` 通過；`TestZZMonthParity`、`TestZZMonthParityPlus` 在 dosgolem 的既有兩版月度視窗均通過，三張表差 0 位元組。玩家原版正常路徑的 `TestBattleFinishesWithPlayer` 固定種子收據通過。原本全庫失敗的 `TestEnhancedRespectsOnePerMonth` 是測試將不耗令的指定太守也算成重複耗令；按 `ActPrefecture` 現有契約訂正測試後，全庫乾淨重跑通過。
- 待驗：玩家戰後整張人物／州郡表的同狀態逐格對拍與加強版正常玩家戰後收據尚未建立；`020` 維持 `READY`，不宣稱 `CONFORMED`。GitHub Issues #102、#103、#2 的遠端狀態未修改；現行 `dist-all/` 尚無正式交付版。
- 版控與清理：本輪變更僅作本地提交，未推送遠端。一次性 Docker 容器均使用 `--rm`，收尾檢查無專案相關容器；清除本輪 `/tmp/san1-ida-session`，並移除逐項確認為空的既有 root-owned `workplace/ida/tools` 誤建目錄，未對儲存庫遞迴改擁有權。
