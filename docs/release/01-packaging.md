# 發行包

`tools/release.sh` 打三個平台的包，產物在 `workplace/release/`（gitignore）。

```sh
tools/release.sh            # 版本字串取自 git describe
tools/release.sh v0.1.0
```

## 1. 三個平台的條件不一樣

| 平台 | cgo | 工具鏈 | 狀態 |
|---|---|---|---|
| Windows amd64 | **不用** | 本機的 Go 容器 | 直接過 |
| Linux amd64 | 要（X11／GL）| 本機的 Go 容器 | 直接過 |
| macOS amd64／arm64 | 要（Objective-C）| osxcross 的 image | 過（`eob-remake-macos`）|

**Windows 不用 cgo** 是 Ebiten 的性質：它在 Windows 走 syscall 不走 cgo，
所以 `GOOS=windows CGO_ENABLED=0` 就交叉編得出來。Linux 與 macOS 都要 cgo，
差別在 macOS 的工具鏈不在手邊。

沒有 osxcross 的 image 時腳本會**跳過並說明**，不會假裝打包成功——
一個安靜地少一個平台的發行流程，看起來與「三個平台都好了」一樣。
備 image 的做法見 skill `osxcross-macos-cross-build`。

### macOS 那條要關掉 workspace

`go.work` 只給對拍測試用（它要接 `dosgolem`）。osxcross 的容器裡沒有
dosgolem，所以那一段要 `GOWORK=off`——而關掉之後就吃 `go.sum` 不吃
`go.work.sum`，`github.com/ebitengine/oto/v3` 要補進 `go.mod` 與 `go.sum`。

⚠ **那個模組只有 macOS 這條路徑用得到**，所以 Linux 與 Windows 建得起來
不代表 macOS 建得起來。三個平台都要真的建過一次才算數。

## 2. 包裡有什麼

```
san1[.exe]        引擎（**三個平台同名**，平台後綴只在建置目錄裡）
fonts/unifont.hex.gz  點陣字型（自由授權，不是原版字模）
LICENSE           RRSAL-1.0
README.md
如何開始.txt
```

## 3. `[HARD]` 包裡沒有原版檔案

**不散布原版執行檔、資料檔、美術、音樂、字型，也不散布說明書掃描**
（`CLAUDE.md` §1）。玩家要自備原版目錄：

```
san1 -root /path/to/三國演義
```

`tools/assets.sh` 轉出來的 PNG／JSON／OGG 也是玩家自己那一份的內容，
**同樣不進發行包**。

## 4. 實際建出來的

`8c0104a` 這一版四個包都建過、解開驗過型別：

| 包 | 大小 | 執行檔 |
|---|---|---|
| `windows-amd64.zip` | 4.4 MB | PE32+ x86-64 |
| `linux-amd64.tar.gz` | 4.6 MB | ELF 64-bit x86-64（動態連結）|
| `darwin-amd64.tar.gz` | 7.6 MB | Mach-O x86_64 |
| `darwin-arm64.tar.gz` | 7.1 MB | Mach-O arm64 |

## 5. 校驗碼

腳本最後產 `SHA256SUMS`。發布時一併附上。

## 6. 還沒做的

- **簽章**：Windows 的 code signing 與 macOS 的 notarization 都沒做。
  未簽章的 macOS 執行檔第一次開要右鍵「打開」。
- **AppImage**：`eob-remake-release` 的 image 裡有 appimage-tools，
  還沒接進來。
- **arm64 Linux**：沒有交叉工具鏈。
