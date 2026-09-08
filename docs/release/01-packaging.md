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

- **簽章**：管線寫好了（`tools/release.sh` 的「簽章」那一段），
  缺的是憑證本身——那是要花錢申請的東西，不在這個 repo 裡。見下一節。
- **AppImage**：`eob-remake-release` 的 image 裡有 appimage-tools，
  還沒接進來。
- **arm64 Linux**：沒有交叉工具鏈。

## 簽章

**憑證不進 repo 也不進容器**，一律走環境變數。沒設就跳過並在畫面上
說明「這不是簽過」——與 macOS 交叉編譯那一段同一個原則：**不假裝成功**。

### Windows

```
SAN1_WIN_PFX=/path/to/cert.pfx SAN1_WIN_PFX_PASS=… tools/release.sh
```

用 `osslsigncode`（Linux 上簽 PE 的標準做法）帶時戳簽 `san1.exe`。
要的東西是一張 **code signing 憑證**（OV 或 EV）。沒有簽章的話
SmartScreen 會擋，玩家要點「其他資訊 → 仍要執行」。

### macOS

**codesign 與 notarytool 不能交叉執行**——osxcross 只負責編譯。
所以 `release.sh` 只把待簽的執行檔列出來，實際三道指令要在真的
macOS 上跑：

```
codesign --force --options runtime --timestamp \
  --sign "Developer ID Application: <名字> (<TeamID>)" san1
ditto -c -k --keepParent san1 san1.zip
xcrun notarytool submit san1.zip --apple-id <帳號> \
  --team-id <TeamID> --password <app-specific 密碼> --wait
```

要的東西是 **Apple Developer Program 會籍**（年費）加上一張
Developer ID Application 憑證。沒有 notarize 的話 Gatekeeper 會擋，
玩家第一次開要在 Finder 裡右鍵「打開」。

⚠ **單檔執行檔不能 `xcrun stapler staple`**——stapler 只認
`.app`／`.dmg`／`.pkg`。單檔要嘛接受「第一次開要連線驗證」，
要嘛包成 `.app` 再 staple。

### Linux

沒有平台級的簽章慣例。發行走 `SHA256SUMS` ＋ GPG 分離簽章：

```
SAN1_GPG_KEY=<金鑰 ID> tools/release.sh
```

產出 `SHA256SUMS.asc`。公鑰要另外公布（README 或 release 頁）。

## 可重現建置

**四個包逐位元組可重現**：同一份原始碼建兩次，`SHA256SUMS` 的四行完全
相同（2026-09-09 實測三次）。

沒有憑證的時候，這是唯一能讓別人獨立驗證「這個包確實是這份原始碼建出
來的」的東西——而它只有在「同一份內容每次壓出同一個位元組串」時才有
意義。預設的 `tar`／`gzip`／`zip` 會把**修改時間、擁有者、檔案順序**
寫進檔頭，於是同一份內容每次的雜湊都不一樣，而**看起來完全正常**。

做法：

| 環節 | 措施 |
|---|---|
| Go 建置 | `-trimpath`（拿掉建置路徑）|
| 檔案時間 | 壓縮前一律 `touch -h -d @0`；`SOURCE_DATE_EPOCH` 可覆蓋 |
| tar | `--sort=name --mtime --owner=0 --group=0 --numeric-owner` |
| gzip | `-n`（不寫檔名與時戳）|
| zip | `-X`（不寫額外屬性）＋ 檔案清單先 `LC_ALL=C sort` |

驗證方式：建兩次、比 `SHA256SUMS`。**不能只看旗標加對了**——
旗標加對但漏掉某一環的話，雜湊照樣每次不同。

```
tools/release.sh 0.0.0-check && cp workplace/release/SHA256SUMS /tmp/a
tools/release.sh 0.0.0-check && diff /tmp/a workplace/release/SHA256SUMS
```

## 冒煙測試

**建得出來不等於跑得起來。** Ebiten 在 package init 就開 GLFW，
所以少一個共享函式庫、字型路徑寫錯、資產讀法改過——這些都不會讓
`go build` 失敗，只會讓玩家一按下去就閃退。

`tools/release.sh` 因此把 Linux 那個包解開來真的跑一次：先 `-h`
確認起得來，再帶原版素材跑 25 秒。**跑滿 25 秒被 `timeout` 砍掉
（退出碼 124）才是通過——自己結束反而是壞消息。**

只驗 Linux：另外三個平台在這台機器上執行不了，硬要驗會變成假綠。
Windows 與 macOS 的包目前只有型別檢查（`file` 認得出 PE／Mach-O）
與可重現雜湊。

⚠ **無顯示環境連 `san1 -h` 都會 panic**（`glfw: The GLFW library is
not initialized`）。那是 Ebiten 的 package init 行為，不是 remake 的
問題；容器裡要 `xvfb-run`。
