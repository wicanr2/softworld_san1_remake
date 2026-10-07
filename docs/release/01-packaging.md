# 發行包與本機驗收

`tools/release.sh` 是建置入口。它在主機上只檢查 Git 狀態、輸入目錄及 Docker image，
並以有界、無網路的容器執行 `tools/release-inner.sh`；建置、封包、檢查及寫入都在容器內。
入口明確接受符合 `v.<主版>.<次版>.<修訂版>-YYYYMMDD` 的完整版號：

```sh
tools/release.sh v.1.0.0-20260923
```

腳本要求所有受 Git 追蹤的檔案無差異，且 `dist-all/<版本>/` 尚不存在；
已建立的交付不會覆寫。若建置失敗，`workplace/release-build/<版本>/`
只保留可重建的中間產物。

## 高清完整版與推廣片契約

**狀態：READY。** 使用者於 2026-10-07 授權完成高清版、打包及 Release，並選定公開引擎包，完整版與有聲影片留本機。新版沿用既有四平台及 RRSAL-1.0。高清功能增加次版號，採 `v.1.1.0-20261007`，不覆寫既有 `v.1.0.3-20260924`。

本機完整版另收經正式載入器驗證的 B／4× `hd-assets/`。兩版各 452 筆，合計 904 筆；PNG、manifest 與來源逐項核對，封包重讀後的檔案雜湊須相同。兩版啟動器明確指向此包，每次仍預設原貌，以 Esc 或滑鼠上緣切換 Theme。公開引擎包不包含這些素材。兩種包的 Windows 中文檔名須有 ZIP UTF-8 旗標，使用說明採 UTF-8 BOM 與 CRLF。

推廣片須錄製當前正式封包中的正常玩家操作，清楚顯示實際選項列與原貌／高清切換，涵蓋主選單、人物卡與戰場。配樂從 DOSBox-X 執行本機原版的實際 OPL 輸出錄製，保存原版輸入、工具、設定、選曲畫面與 WAV 雜湊；不得用 remake 合成器轉出的 WAV 替代。影片、抽樣幀、字幕、音量、黑幀、預期停留區間、來源與權利收據收在 `promo/`。合成中間片段及解包目錄放容器 `/tmp`，用完即移除。真正的原生平台、人耳及簽章驗收限制須明示。

公開 Release 只附四個 `patch/` 封包及由公開封包產生的公開 SHA-256 清單。完整版、高清素材、原版錄音、有聲影片與私人總清單不上傳。

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

`SAN1_PROMO_AUDIO=workplace/<原版錄音> tools/promo-local.sh <完整版號>` 從最終 Linux 完整版錄製主選單、人物卡、戰場的正常 Theme 切換及三語操作，再加上同停點比較與片尾。原版錄音入口是 [promo-original-audio.py](../../tools/promo-original-audio.py)，須在既有 `civ1-dosboxx-input:20260830` 內執行，原版唯讀掛載至 `/orig`，輸出掛載至 `/out`，命令為 `python3 /src/tools/promo-original-audio.py --out /out`。用 DOSBox-X 的 F12＋W 擷取內部 WAV；按 5 開音樂欣賞，再按 3 播風雲並回主選單，原版行為證據見 [005 §6.6](../spec/005-main-screen.md#66-音樂欣賞0x145cal0l1baseissue-70)。快捷鍵依 [DOSBox-X 官方說明](https://github.com/joncampbell123/dosbox-x/wiki)。

影片只留本機。`promo/` 保留接觸表、抽樣幀、FFprobe、音量、黑幀與凍結檢測、操作時間線、來源與 SHA-256、權利說明。靜態比較、片尾與玩家停點須依分鏡審查，不為避開凍結檢測而假造畫面移動。解包、輪詢及合成中間段落使用容器 `/tmp`，退出即清理。

影片合成改用本專案 [Dockerfile.promo](../../docker/Dockerfile.promo)，映像檔為
`san1-promo:ffmpeg5.1.9-r1`。Python base 固定 digest、FFmpeg 固定套件版號，
完整 Debian 套件版本與下載的 DEB 雜湊保留在映像檔 `/opt/san1-promo/`。
此映像檔只供合成及媒體驗收，不取代 Go、DOSBox-X 或正常 GUI 錄影工具鏈。
建置入口為 `DOCKER_BUILDKIT=0 timeout 6m docker build --pull=false --force-rm --rm --memory 1g --cpu-period 100000 --cpu-quota 200000 --ulimit nproc=128:128 --network default -t san1-promo:ffmpeg5.1.9-r1 -f docker/Dockerfile.promo docker`。
字幕使用唯讀掛載的 NotoSansCJK-Regular.ttc，`SAN1_PROMO_FONT` 可指定來源檔。
已有通過收據的正式錄影時，以 `SAN1_PROMO_ASSEMBLE_ONLY=1` 接續合成；合成器仍
核對版號、錄影通過狀態、片段雜湊與原版錄音，不用此選項跳過正常 GUI 驗收。

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

## `v.1.0.3-20260924` 公開發行與本機完整版

正式來源提交為 `8d89946357580a6a650b1e9a4bc0fea12dadcf3b`，同名 Git tag
與[公開 Release](https://github.com/wicanr2/softworld_san1_remake/releases/tag/v.1.0.3-20260924)
已回讀確認；儲存庫可見度為公開。Release 僅附 `patch/` 的 Linux amd64、
Windows amd64、macOS amd64／arm64 四個引擎封包，以及
`patch/SHA256SUMS-public.json`。遠端五個附件的檔名、大小與 SHA-256 已回讀；
本機總清單 `dist-all/v.1.0.3-20260924/SHA256SUMS.json` 不公開，因為它也
記錄私人完整版和推廣片。

`full-local/` 的四個封包逐包包含原版與加強版共 66 個實際遊戲檔案，
封包內的每個原版檔均與本機來源 SHA-256 相同。最終 Linux 完整版解開後，
兩版各啟動八秒；Windows／macOS 完整版仍只完成封包、格式與內容核對。
這四個封包只留在本機，沒有放進 GitHub Release。

`promo/san1-v.1.0.3-20260924-promo-local.mp4` 是以現行 remake 重生六張畫面、
搭配本機原版「風雲」資料製作的 42 秒影片。視訊為 1280×880 H.264，
音訊為 AAC；平均音量 −38.5 dB，沒有連續 0.5 秒黑幀或 2 秒凍結，
六格抽樣幀已目視核對字幕位置。影片 SHA-256 為
`f5637cee38484cf156bc2a05dbd594f7579ac35d36cd1562d4ea273a3a3546c7`。
原版美術與配樂的公開再散布授權尚未提供，因此影片只留在本機。

四個公開引擎包都未簽章。Linux 在容器中的兩版啟動，不等於 Windows、
macOS 原生啟動；後兩者以及簽章、公證仍待實測，見
[GitHub Issue #2](https://github.com/wicanr2/softworld_san1_remake/issues/2)。

## `v.1.1.0-20261007` 本機交付

沿用上一輪定案版號，正式程式來源為
`cecfd31d94adf6133813a3776c1e43eaf994dd17`。本機交付在
[`dist-all/v.1.1.0-20261007/`](../../dist-all/v.1.1.0-20261007/)，
[公開 Release](https://github.com/wicanr2/softworld_san1_remake/releases/tag/v.1.1.0-20261007)
已發布並設為 latest。四個引擎包與四個私人完整版均已從壓縮檔獨立回讀，
封包 SHA-256、LICENSE、啟動器、Windows UTF-8 旗標與說明編碼相符。
每個私人包有兩版共 66 個原版檔、904 筆高清項目及 444 個高清包檔案，
全部與本機來源相符。正式載入器兩版各 452/452、零警告。

Linux 引擎包兩版各持續啟動 25 秒，真正完整版解包後兩版各啟動八秒，
程式回傳相同完整版號。Windows amd64、macOS amd64／arm64 僅完成建置、
格式及封包內容檢查，原生啟動、簽章與公證未驗。
公開清單為 `patch/SHA256SUMS-public.json`，只列四個公開引擎包；
包含私人產物的總清單 `SHA256SUMS.json` 不上傳。
發行說明母本為 `workplace/v66-release-notes.txt`，遠端完整說明已核對相符。
五個附件的大小及 GitHub 提供的 SHA-256 全部符合本機清單，公開收據為
`workplace/v66-release-published.json`。annotated tag 的實際提交已回讀為
`cecfd31d94adf6133813a3776c1e43eaf994dd17`，既有 tag／Release 未改動。

| 公開平台 | SHA-256 |
|---|---|
| Linux amd64 | `8b91b5e709a69cfe58ba13f29514a633c16b3f6c059de119f40fb14a544e8a8b` |
| Windows amd64 | `5e13bc8f115cbc524e23b1cfca270c259c9e6524409004cd5570f3e30d7e32a5` |
| macOS amd64 | `52ca245d7f25e6c6aaff4b0ddf753109f91cdb7236ec2e3925e7ca463fe3968c` |
| macOS arm64 | `4938e309aa7bdfcc70b0b995d906d56b53a94b65bce10fc8d45d2308f04ecaa3` |

推廣片從真正 Linux 完整版正常開機，錄製主選單、曹操人物卡、董卓出兵與
合法紮寨後的戰場，實際切換 Theme 及三語，沒有狀態、seed 或時鐘注入。
三段錄影、九張來源畫面及正式執行檔／高清 manifest 與封包相符。
配樂為先前 DOSBox-X 執行原版 AA.EXE 的「風雲」實際 OPL 錄音。

影片為 1920×1200 H.264、固定 30 fps、44.401 秒容器時長，AAC 雙聲道
44,100 Hz；1,329 個視訊影格及整段影音獨立解碼通過。視訊時長 44.3 秒，
與容器時長差 0.101 秒，在既定 0.2 秒界線內。平均音量 −19.5 dB、峰值
−6.1 dB，沒有長靜音或黑幀；六個靜止區間符合四個玩家停點、六秒靜態比較
與四秒片尾。五幕完整代表幀、英日兩張完整幀與九格接觸表已查看，字幕完整。
影片 SHA-256 為 `793b3b8276728d9e2fb60aca52d1e8748822489f36821d2a689a5f8b61b50cfe`。
人耳尚未驗，不擴大原版 oracle 或跨平台音訊聲明，影片只留本機。

獨立收據為 `workplace/v66-packages-independent.json` 及
`workplace/v66-promo-independent.json`；影片分鏡、原版音源、音量、黑幀、
靜音、凍結、代表幀與權利紀錄位於該版本 `promo/`。
錄影 R1 的主動 SIGINT 退出碼斷言、缺少字幕字型及時間戳候選均保留失敗證據。
合成工具只修復這些工具問題，正式 Go 程式與已驗封包未重建或變更。
使用過的共用工具映像檔目前已無法取得，合成改用本專案固定 revision；
完整 Go／macOS 重建仍須先恢復原工具鏈。正式建置暫存及影片合成暫存已清理。
