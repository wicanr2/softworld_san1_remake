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
├── smoke/             封包清單、執行檔型別與 Linux 啟動紀錄
└── SHA256SUMS.json    版號、來源 commit、image、輸入與封包 SHA-256
```

每包僅有 `san1[.exe]`、`LICENSE`、`README.md`、`如何開始.txt`、
四套點陣字型及其三份授權文件。`kai`／`li` 字型授權收在
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

## 版本、雜湊與未完成驗收

版號注入四支執行檔，並逐支檢查；檔名、目錄、`SHA256SUMS.json` 及
Linux `-version` 均使用相同完整版號。清單記錄原始碼 commit、兩個 Docker
image ID、主要輸入 SHA-256、每包長度與 SHA-256、權利分類及本機驗收結果。
封包固定檔案時間、順序和擁有者；要主張逐位元組可重現，仍須用相同輸入
與工具鏈重建兩次，再比對四包雜湊。

目前腳本產出**未簽章的本機封包**。Windows 與 macOS 的原生啟動、
相應平台的簽章／公證、玩家路徑驗收，以及公開 GitHub Release，
都需另附實際收據；不得把本機交叉編譯寫成這些項目已完成。
