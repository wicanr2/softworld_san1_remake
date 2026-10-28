# 在另一台電腦重建

先安裝 Git、Docker 及 Bash。Windows 可在 WSL2 執行。工具鏈固定使用 Linux amd64 容器，
ARM64 主機須啟用 Docker 的 Linux amd64 模擬；本輪不宣稱 ARM64 主機實機驗證。
原版資料、macOS SDK 與研究工具授權由使用者另備。

## 下載與建置

```sh
git clone https://github.com/wicanr2/softworld_san1_remake.git
cd softworld_san1_remake
bash tools/build-desktop.sh linux-amd64
```

第一次會依 [Dockerfile.build](docker/Dockerfile.build) 建立
`san1-go:1.26.7-ebiten2.9.9-r1`。Go 1.26.7 基底以 digest 固定，Debian 套件取自
2026-10-10 快照，Go 模組依 `go.mod`、`go.sum` 下載並驗證。只有建立工具鏈需要網路；
快取恢復、編譯與測試均在無網路、非 root、有資源上限的容器內執行。
不需要本機 Go、其他專案的 image 或既有 `workplace/`。

執行檔放在 `workplace/rebuild/linux-amd64/san1`。其他平台：

```sh
bash tools/build-desktop.sh windows-amd64
bash tools/build-desktop.sh darwin-amd64
bash tools/build-desktop.sh darwin-arm64
# 一次建置四個目標
bash tools/build-desktop.sh all
```

macOS 目標第一次會依 [Dockerfile.macos](docker/Dockerfile.macos) 建立
`san1-macos:1.26.7-sdk15.5-r1`。SDK 來源以 digest 固定；SDK 與含 SDK 的 image
只供符合 Apple 條款的本機使用，不提交 repo 或 Release。
交叉編譯成功只證明產出對應架構，原生 Windows／macOS 試玩、簽章及公證另驗。

一般開發可用 `tools/go.sh build ./...`；`tools/go.sh test ./internal/font ./internal/assets`
會自動在 Xvfb 下執行，容器加上 `--init`，避免顯示器包裝程式作為 PID 1 時停在啟動等待。
原版對拍另需合法原版資料、指定的 dosgolem 工作副本與明示的 Go workspace。
正式版號與封包仍走 [發行入口](docs/release/01-packaging.md)，不將開發用執行檔當成新發行。

## 素材與原版資料

| 輸入 | GitHub 保存範圍 | 新電腦需要做什麼 |
|---|---|---|
| 現行 HD 美術與新增肖像 | `hd-assets/`，657 張 PNG、目錄與來源紀錄 | 隨 clone 取得 |
| 三段 AI 配音樣音 | `hd-assets/audio/`，OGG、來源 WAV、提示詞及收據 | 隨 clone 取得；樣音尚未接入正式遊戲 |
| 字型 | `fonts/`，點陣、TTF、來源與授權 | 隨 clone 取得 |
| 規則、劇本擴充與翻譯 | `internal/` 及受版控的資料檔 | 隨 clone 取得 |
| 原版 DOS 執行檔、GRP／IDX／NAM、原版音畫 | 不入 Git | 從自己合法持有的原版複製，使用 `-root` 指定 |
| San5 原圖、說明書掃描、母圖、未採用候選與本機影片 | 不屬於現行程式建置輸入，維持既有本機保存範圍 | 需要研究或重做這些產物時，另行搬移合法來源 |

現行素材共 681 個檔案，清單與 SHA-256 在
[rebuild-assets.json](docs/release/rebuild-assets.json)，可用
[核對工具](tools/verify-rebuild-assets.py)於 Docker 內驗證。
產出程式後，把 repo 的 `fonts/` 與 `hd-assets/` 複製到執行檔旁，並以 `-root`
指定合法原版目錄，例如：

```sh
./san1 -edition base -root /path/to/三國演義 -font fonts/unifont.hex.gz -hd-assets hd-assets
```

repo 足以重建引擎、字型與現行 HD 包；遊玩及原版對拍仍需自備原版資料。
正常啟動驗證工具 [verify-startup.py](tools/verify-startup.py)可用 `--title-reference`
明示提供正常選單參考圖；這是驗證輸入，不是引擎建置所需素材。

## 工具鏈與快取維護

```sh
# Dockerfile 或 go.mod／go.sum 改變時重建，沿用相同專案 tag
bash tools/build-toolchain.sh build
bash tools/build-toolchain.sh macos
# 快取清除後，由工具鏈內已驗證的模組離線恢復
bash tools/prepare-go-cache.sh
```

Docker build context 由 [.dockerignore](.dockerignore)限縮到鎖檔與 Dockerfile，
不包含原版資料、素材母圖或交付包。原版資料與正式交付勿因快取可重建而一起刪除。
本機驗證結果與後續清理候選見 [CONTEXT](CONTEXT.md)及 [WORKLOG](WORKLOG.md)。
四平台遠端重建與素材核對的實際範圍見 [驗證收據](docs/release/rebuild-verification.json)。
