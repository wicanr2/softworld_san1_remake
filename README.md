# 三國演義 DOS remake

智冠科技（Softworld）1991 年 DOS 遊戲《三國演義》的逆向工程重寫，
以 Go / Ebiten 實作跨平台引擎。定位是文化資產保存——這是台灣自製 PC 遊戲的早期作品。

原版與《三國演義1加強版》兩個版本並列還原，規則層共用，版本差異以旗標切換。

## 現在做到哪裡

引擎已經能把原版的劇本資料讀出來並畫到畫面上（州郡一覽，42 個郡名、零缺字）。
遊戲玩法還沒實作。

```sh
tools/go.sh run ./cmd/san1     -root /path/to/三國演義   # Ebiten 視窗
tools/go.sh run ./cmd/san1dump -root /path/to/三國演義 -png out.png   # 無頭輸出
```

完成度的數字以 `VERIFICATION-MATRIX.md` 為準（尚未建立）；
目前的實際狀態在 [`CONTEXT.md`](CONTEXT.md)。

| 里程碑 | 狀態 |
|---|---|
| M0 環境 ＋ 素材 | 完成 |
| M1 dosgolem 跑得動 | 進行中：過了裝置選單，卡在 overlay 載入段 |
| M2 容器格式 | 實質完成（`docs/formats/01`–`03`）|
| M3 文字與字型 | 進行中：CJK 畫布與字型涵蓋率已通 |
| M4 靜態資料 | 未開始 |
| M5 規則層 | 未開始 |
| M6 引擎可玩 | 未開始 |
| M7 多語系 | 未開始 |
| M8 發行 | 未開始 |

## 需要原版

本儲存庫**不含**任何原版檔案——執行檔、資料檔、美術、音樂、字型、說明書都不散布。
要跑起來得自備原版。

## 驗證方式

規則還原不以「在 remake 裡看起來能玩」驗收，而是對原版對拍：
用 [dosgolem](https://github.com/wicanr2/dosgolem)（無頭、決定性的 DOS 執行器）
在程序內跑原版執行檔，直接讀原版自己的變數、攔它自己的呼叫，逐項比對。
DOSBox-X 作為交叉驗證。

## 授權

採 RRSAL-1.0（復古重製 source-available 授權條款），全文見 [`LICENSE`](LICENSE)。
非商業用途免費且寬鬆，包含修改與再散布、實況與影片；商業用途保留給著作權人另行洽談。

這不是 open source，是 source-available。

原版《三國演義》的著作權屬智冠科技，本專案與智冠科技無關。

## 文件

| 路徑 | 內容 |
|---|---|
| [`CLAUDE.md`](CLAUDE.md) | 專案規則與硬規則 |
| [`CONTEXT.md`](CONTEXT.md) | 現況、已知事實、決策紀錄、worklist |
| `docs/re/` | 反組譯筆記 |
| `docs/formats/` | 檔案格式 |
| `docs/spec/` | 規格（只有 `READY` 能授權實作）|
| `docs/mechanics/` | 遊戲機制 |
| `docs/playtest/` | 對拍紀錄 |
| `docs/reference/` | 說明書整理、社群資料 |
