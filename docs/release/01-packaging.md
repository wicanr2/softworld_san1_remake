# 發行包與本機驗收

`tools/release.sh` 是建置入口。它在主機上只檢查 Git 狀態、輸入目錄及 Docker image，
並以有界、無網路的容器執行 `tools/release-inner.sh`；建置、封包、檢查及寫入都在容器內。
使用者需明確提供符合 `v.<主版>.<次版>.<修訂版>-YYYYMMDD` 的完整版號：

```sh
tools/release.sh v.1.0.0-20260923
```

腳本要求所有受 Git 追蹤的檔案無差異，且 `dist-all/<版本>/` 尚不存在；
已建立的交付不會覆寫。若建置失敗，`workplace/release-build/<版本>/`
只保留可重建的中間產物。

## 平台與工具鏈

| 封包 | 建置方式 | 本機驗收 |
|---|---|---|
| Linux amd64 | Go／cgo，`eob-remake-release:1.26.7-ebiten2.9.9-audio` | ELF 型別、封包內容；原版與加強版各執行 25 秒 |
| Windows amd64 | Go，`CGO_ENABLED=0`，同上 image | PE32+ 型別與封包內容；尚未在 Windows 原生啟動 |
| macOS amd64 | Go／osxcross，`eob-remake-macos:1.26.7-ebiten2.9.9-audio` | Mach-O 型別與封包內容；尚未在 macOS 原生啟動 |
| macOS arm64 | Go／osxcross，同上 image | Mach-O 型別與封包內容；尚未在 macOS 原生啟動 |

四個平台都必須成功編出；任何一個失敗就不建立現行交付。交叉編譯與檔案型別
不能替代目標平台的原生啟動、Windows 簽章或 macOS 公證。

## 交付內容與權利

正式本機產物放在 `dist-all/<版本>/`：

```text
dist-all/<版本>/
├── patch/             四個可公開的引擎與合法字型封包
├── full-local/        含玩家本機原版資料的私人完整版，禁止上傳
├── promo/             本機推廣片、抽樣幀、媒體探測與權利紀錄
├── smoke/             封包清單、執行檔型別與 Linux 啟動紀錄
└── SHA256SUMS.json    版號、來源 commit、image、輸入與封包 SHA-256
```

公開引擎包僅有 `san1[.exe]`、`LICENSE`、`README.md`、`如何開始.txt`、
現行 remake 截圖、四套點陣字型及其三份授權文件。`kai`／`li` 字型授權收在
`fonts/LICENSE-wangfonts.txt`。腳本解讀每一包的成員清單並逐項比對這份
固定清單，也檢查 ZIP 的 CRC、GZIP 完整性及執行檔格式。

**公開封包不含原版執行檔、資料、美術、音樂、語音、字模或說明書掃描。**
`tools/assets.sh` 從玩家素材轉出的檔案也不能加入公開包。玩家須自行提供
合法持有的原版目錄：

```sh
san1 -root /path/to/三國演義 -edition base
san1 -root /path/to/三國演義1加強版 -edition plus
```

預設存檔位置是執行時工作目錄的 `saves/`，可用 `-saves` 指定其他位置；
不會寫入 `-root`。兩版 Linux 冒煙測試都將原版素材唯讀掛載，並把 `-saves`
指向封包外各自的暫存目錄。測試使用 `xvfb-run`，`-version` 必須等於完整
版號；進入遊戲後滿 25 秒才由 `timeout` 終止，逾時碼 124 才算通過。
這只證明啟動與持續執行，不能替代正常玩家路徑驗收。

公開引擎包驗收後，`tools/full-local.sh <完整版號>` 從四個公開引擎包建立
`full-local/` 下四個本機專用封包。每包都含原版與加強版的實際遊戲檔，
附兩版啟動腳本；腳本把存檔寫入封包內的 `saves/`，不改動原版資料。
建置器逐檔比較原始輸入與四個封包內的 SHA-256，再從最終 Linux 封包
解開，以內附的兩版資料各啟動八秒。`full-local/ORIGINAL-SHA256.json`
保存原版輸入雜湊；最上層 `SHA256SUMS.json` 記錄四包大小與雜湊，
權利分類為 `local_only_original_assets`。這些封包只供本機保存，
不加入 Git，也不附上 GitHub Release。

`tools/promo-local.sh <完整版號>` 會從現行 remake 程式重生六張畫面，
以本機原版配樂「風雲」製作 42 秒的繁中推廣片，輸出到 `promo/`。
影片只供本機保存；它使用原版美術與音樂，不附上公開 Release。
`promo/` 同時保留六格接觸表、影片抽樣幀、FFprobe 資訊、音量、
黑幀與凍結檢測紀錄、來源及輸出 SHA-256、權利說明。這些收據用來
確認影片可播放、非靜音、畫面和字幕沒有明顯裁切。

## 版本、雜湊與未完成驗收

版號注入四支執行檔，並逐支檢查；檔名、目錄、`SHA256SUMS.json` 及
Linux `-version` 均使用相同完整版號。清單記錄原始碼 commit、兩個 Docker
image ID、主要輸入 SHA-256、每包長度與 SHA-256、權利分類及本機驗收結果。
封包固定檔案時間、順序和擁有者；要主張逐位元組可重現，仍須用相同輸入
與工具鏈重建兩次，再比對四包雜湊。

目前腳本產出**未簽章的本機封包**。Windows 與 macOS 的原生啟動、
相應平台的簽章／公證、玩家路徑驗收，以及公開 GitHub Release，
都需另附實際收據；不得把本機交叉編譯寫成這些項目已完成。

## `v.1.0.0-20260923` 本機收據

這次的來源提交是 `d74a7350ec3346152cb25904ed2ba256a93f8ae6`，
本機清單在 [`SHA256SUMS.json`](../../dist-all/v.1.0.0-20260923/SHA256SUMS.json)。
四個包各有 13 個清單項目；重新讀取後，SHA-256、壓縮完整性、檔案型別、
權利清單及目前使用者擁有權均通過。Linux 封包前目錄讓原版與加強版各
執行滿 25 秒，最終壓縮包解開後各再啟動 8 秒，`-version` 回傳相同完整版號。
本輪沒有雙次建置的雜湊比較，也沒有 Windows／macOS 原生啟動、簽章、公證、
Git tag 或公開 Release。

## `v.1.0.1-20260923` 現行本機收據

原版文字位置複驗發現戰術提示少了人物表兩字姓名欄前置空格；正式玩家路徑
已修正，既有 `v.1.0.0-20260923` 不覆寫。同日修正版從乾淨提交
`d1ec266e0d6b52526f844c600da49045831d8283` 建置，清單在
[`SHA256SUMS.json`](../../dist-all/v.1.0.1-20260923/SHA256SUMS.json)。

| 平台 | 封包 SHA-256 |
|---|---|
| Linux amd64 | `8750f1109b3e2f995b5c7a5f899a437eedefe4fe00a2ad257d52b538ee9fd015` |
| Windows amd64 | `5a6459d25b4d480293d92f9c04f924ee4ed2833535db90326bd2dd9f2cc33f52` |
| macOS amd64 | `7fe3fbbe9d20e4f0edda0d0322fb8a7d06f73b21e8adc6cd5adcea6e8d7b9782` |
| macOS arm64 | `e6791137b3139228001a6c7b4395852641dc0c2f11276ea1e481cea33b79a179` |

腳本逐包驗證成員清單、壓縮完整性、執行檔型別與來源權利分類；
本輪另從唯讀容器獨立重算四包長度及 SHA-256，全部相同且沒有 root
擁有的產物；包內 README 已改用清單作版號入口，沒有宣稱舊版為現行版。Linux `-version` 與完整版號相同，base／plus 以唯讀原版
素材在封包前各持續執行 25 秒；最終 Linux 壓縮包解開後，base／plus
另各持續執行 8 秒，`-version` 仍是同一版號。Windows／macOS 原生啟動、簽章、公證、雙次建置雜湊比較、
Git tag、遠端推送與公開 Release 均未做。
