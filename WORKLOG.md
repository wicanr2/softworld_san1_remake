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

## 2026-09-23：玩家戰後兩版人物／州郡全表對拍

- 決定：使用者選擇「先完成全表對拍再發行」，排除先發未簽預覽；發行閘門已記入 `CONTEXT.md`。
- 方法：原版／加強版各用正常玩家戰役與固定起始 seed `0x13579bdf`，在結算入口讀原版三表、四軍與當時種子。remake 用 `game.Continue` 從同一份執行期表格接續，透過僅在 `oracle` 建置啟用的 `ReplayPlayerSettlement` 呼叫正式結算。兩側入口三表逐位元組相同；從這個切點比較戰後整張州郡表與人物表。
- 收據：`SAN1_TIMEOUT=25m tools/go.sh test ./internal/parity -tags oracle -count=1 -run '^TestZZPlayerSettlementTables(Plus)?$' -v` 通過。兩版各自的州郡表 7,568 B、人物表 10,500 B，戰後皆 0 差。原版入口／返回三表 SHA-256 是 `4919cd29…bc151`／`599d4be7…b7cd`，加強版是 `82efca6d…bc78`／`0ba23416…24a0`；完整雜湊、結算入口種子、限制見 `docs/spec/020` §5。
- 訂正：第一次加強版對拍把天數的 `+2` 位移外推到未移位的勝負欄位 `0x20dc`，導致結算入口 0 次；查 `docs/re/05` §8.2 後固定原位重跑。第二次把執行期表格交給 `game.New`，令 12 個 AI 等級重套新局難度；換成既有的 `game.Continue` 後，兩版入口三表相同。
- 限制：這是**結算切點**的對拍，戰術結果取自原版；並非 remake 從整編到戰後的全程獨立驗證。諸侯人望已在原版切點之前變動，退場君主也已離開戰場部隊；這個重播介面不重建兩段前置事件，戰後諸侯表原版／加強版各差 12／2 B。`020` 維持 `READY`，不把諸侯表或戰術全程宣稱 `CONFORMED`。

## 2026-09-23：本機交付工具鏈準備

- 決定：使用者選 `v.1.0.0-20260923` 作本機交付版號，排除 `v.0.1.0-20260923`；諸侯表與戰術全程的已知限制照實寫入交付紀錄。
- 將舊 `tools/release.sh` 的主機建置、`workplace/release/` 路徑及非正式版號，改為主機僅控制 Docker 的入口；實際建置、封包與驗收由 `tools/release-inner.sh` 在無網路容器執行。四包須齊全，並逐包核對格式、檔案清單及壓縮完整性；Linux 用唯讀原版素材抽測兩版啟動。
- 驗證：`tools/go.sh test ./... -count=1` 通過；`-tags oracle -run '^$'` 編譯全庫通過；發行腳本在容器內通過 `bash -n`。這些檢查尚不是正式封包或 Windows／macOS 原生啟動收據。
- 首次四平台編譯完成後，封包清單驗收把 GNU tar 對中文檔名的跳脫顯示誤判為不同檔案；未建立 `dist-all/`。將 tar 清單改為原樣列名，並以已產生的 Linux 包確認成員名稱後，從乾淨輸入重建。
- 第二次封包清單通過後，Linux 原版啟動在片頭 `iter.Pull` 崩潰：初始化建立協程時 Ebiten 鎖著主執行緒，第一幀更新卻在另一個未鎖執行緒呼叫 `next`。把片頭協程延到第一幀 `Update` 建立；全庫測試重跑通過，實際封包啟動仍須從乾淨輸入重驗。第二次也未建立 `dist-all/`。
- 第三次本機預交付成功產出四包，base／plus 各跑滿 25 秒；解開 Linux 包另各啟動 8 秒，四包 SHA-256 與擁有權回讀通過。交付前發現包內 README 的 M8 欄仍寫「尚無 dist-all」，因此把該欄改成以 manifest 為準的穩定入口，將這份未交付的本機預產物撤離現行根目錄，再從乾淨輸入重建相同首版。此時尚無 Git tag 或公開 Release。
