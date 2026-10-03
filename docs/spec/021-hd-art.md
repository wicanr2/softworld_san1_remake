# 021：B 寫實手繪 HD 素材與渲染

狀態：`READY`，授權視窗選項列、首批四張素材及 §6.7–6.11、§6.15 已審查的肖像批次、§6.17 場景批次；其餘美術依 #108、#109 分批驗收。

使用者於 2026-10-02 選定 B「寫實手繪」，並授權開始 HD 計畫。排除 A 原貌高清及 C 現代英武立繪。沿用 640×408 邏輯版面、人物資料、規則及存檔。

工作權威為 [#104](https://github.com/wicanr2/softworld_san1_remake/issues/104) 及 #105–#110。使用者接續選定預設原貌，要求視窗選項列切換語言、Theme 及 AI 強度；選項列在遊戲上方，預設隱藏，由 Esc 或滑鼠上緣展開。全批 HD 依下列契約及 #108、#109 分批接入，不更換已定案的 B 畫風或玩法。

## 1. 已定案與待決事項

| 項目 | 狀態 | 契約 |
|---|---|---|
| 畫風 | 使用者定案 | B 寫實手繪，保留智冠原圖的冠帽、鬚髮、服色、視線及頭部構圖；補足體積與筆觸 |
| 版面 | 使用者定案 | 保留原版 640×408 邏輯座標；尺寸證據見 [006](006-screen-geometry.md) |
| 規則與存檔 | 使用者定案 | 不改遊戲規則、人物肖像索引或存檔 schema；HD 是 remake 美術差異 |
| 高清主規格 | 使用者定案 | 4× 2560×1632；肖像 256×320、事件場景 704×384。2× 僅保留作比較，視窗可縮小 |
| 顯示模式與預設值 | 使用者定案 | 每次啟動預設原貌；選項列切換原貌／B 高清。缺圖逐項回退原圖 |
| 首批正式素材 | 正常玩家路徑已驗 | F000、F005、F228 與 SCG01 的首批兩版收據見 §6.6；全批仍按 #108、#109 驗收 |
| 正式資產格式及圖層介面 | READY | 首批契約見 §6；原始資源鍵、版本、來源／輸出雜湊及 4× 尺寸須全部吻合 |

## 2. 來源與盤點

盤點見 [資產目錄 §6](../formats/04-asset-inventory.md#6-hd-兩版盤點)。使用既有 `assets.OpenContainer`、`DecodeImage`、`DecodeMask`、`BattleTiles` 與 `state.LoadScenario`。完整原始檔、資源區間、SHA-256、尺寸、索引像素雜湊、使用端及人物引用在本機 `workplace/hd-inventory/inventory.json`。

| 來源 | 原版 SHA-256 | 加強版 SHA-256 |
|---|---|---|
| 進入點 | `AA.EXE`：`474780e5be697b3b4899da5e0dbadd2f327e0bbe7e56306ac3b732a15fc124ca` | `ASV.EXE`：`ad18a251fece9b7b8b0c5f2fd42565f4981883af4b55fd68ca2df00f1bc88be5` |
| DATA1.GRP | `958f44fe45e38624401af55f033ffb037bd3211a037eadbce90f827637d977a5` | 同原版 |
| DATA2.GRP | `98a2a7139bb4ad796121b7ede6ea854c964c8588a0424ca67fc7555d382428a7` | `97d5f9e5ab5cec3fb3aa4360844569ab210e19d69fc4f43beaf2013929755ec6` |
| DATA3.GRP | `24e642cc8c3df7614909c054b6a92334fe6a0f3d1eefa544f571591648e42e5e` | 同原版 |

這一輪只用檔案位移，不引用 IDA 或執行期線性位址。來源尺寸、數量及雜湊為 `L0`、`[both]`；目前 remake 使用端的核對不升格為原版對拍。

## 3. 定版參考與試作

定版樣式原圖為本機 `workplace/hd-sample-b.png`，1536×1024，SHA-256：
`48a3d1ce797edf117279129a9c409ff17bed11bc12de5d8e3ffec6e33289e8a0`。

樣式板含標題與四個分區，不能直接當遊戲素材。以樣式板及原版單張圖共同參考，使用內建 `image_gen` 分別生成以下獨立圖，保存完整畫面。生成工具未回報模型版本或固定 seed；精確重生依靠保存的原圖與雜湊，提示詞只能重做候選圖。

| 資源鍵 | 劇本 001 的資料引用 | 試作原圖 | 尺寸 | SHA-256 |
|---|---|---|---|---|
| DATA3/F000.FAC | 人物 13 曹操，35 歲 | `workplace/hd-b-F000-v1.png` | 1122×1402 | `506ea2dcab16019de4a4aac76f38837fd3da3926a189433486d30b4f5596ce50` |
| DATA3/F005.FAC | 人物 0 劉備，29 歲 | `workplace/hd-b-F005-v1.png` | 1122×1402 | `904dc3233000d92075b14b1cdcc2935e43f3b6ccb8d5e794416c2d22aa9cd145` |
| DATA3/F228.FAC | 人物 84 魏續、296 鮑忠、331 陳就共用 | `workplace/hd-b-F228-v1.png` | 1122×1402 | `526460e5d81b0271060def1798a8adbb096fd2136021b2efa4b0975c9905d8d3` |
| DATA3/SCG01.IMG | 地震；事件使用端見 [010](010-screen-transitions.md) | `workplace/hd-b-SCG01-v1.png` | 1698×926 | `060a91b463e3f3614a680962818cc7f2818e608a9332098ca34aa6e1cc92ef89` |

肖像槽為 64×80、4:5；場景槽為 176×96、11:6。工具產出的整數尺寸略有四捨五入，試作只容許一個來源像素內的比例誤差，使用完整圖縮放，不裁去冠帽、肩部或背景。三張肖像目前集中於 29–35 歲，不能宣稱已覆蓋老年人物；正式全批前仍要抽驗白髮、特殊冠帽及女性肖像。

原版參考、定版板與衍生圖的公開再散布權未知，依專案素材規則保存於本機。Git 只保存工具、契約與雜湊，不加入圖片。

## 4. 同狀態版面 prototype

入口為 [`tools/hd-preview.go`](../../tools/hd-preview.go)。以 `game.New` 建立劇本 001、曹操、難度 5 的狀態，呼叫既有正式 UI 畫主畫面、三張人物卡與地震場景。從 640×408 底圖保留文字、數值、疆界及外框，再直接把獨立高清圖畫到 2×／4× 對應素材區，高清圖沒有先降為 64×80。

| 抽樣 | 原座標及尺寸 |
|---|---|
| 主畫面曹操肖像 | (536,116)，64×80 |
| 三張人物卡 | (536,68)，64×80；人物選取由資料的 Portrait 欄決定 |
| 地震場景 | (432,80)，176×96；先由正式 `DrawScene` 完成原場景 |

兩版各有三語系 × 五種畫面 × 兩個倍率，共 30 張高清樣圖。每張驗證素材矩形以外與原 UI 最近鄰放大結果逐像素相同，並核對畫圖前後三張遊戲表的 SHA-256。這是靜態版面驗證，未經正常玩家輸入、動畫、存讀檔或音樂驗收，不是 dosgolem 收據。

本機輸出與重跑：

```sh
tools/go.sh run ./tools/hd-preview.go -root /orig/三國演義 -edition base
tools/go.sh run ./tools/hd-preview.go -root /orig/三國演義1加強版 -edition plus
```

預設輸出在 `workplace/hd-preview/`，以版本、語系、畫面與倍率命名。`base-receipt.json`、`plus-receipt.json` 記錄每張圖的尺寸、素材區、輸入／輸出雜湊與越界像素數，另輸出四張選定 4× 尺寸的首批素材及 `prepared_4x_assets` 收據。原版狀態三表雜湊為 `d6bc3e64d92838c09fc0c483a77352ff7f6f27dd81586203e3df36556b406962`，加強版為 `bfac209e97716ebb3639a041ba3ee7de610e5870320d868bd97ce5cdf151d7f2`。本輪 60 張樣圖皆為素材區外 0 差，畫圖前後三表不變。

## 5. 正式接入的查證條件

- 維持 640×408 邏輯幾何。`app.Layout` 已依實際畫布回傳完整高度；原貌 640×408、高清 2560×1632，選項列另外加高，不裁掉最下方 8 列。
- 直接在高解析輸出上畫高清美術。文字、數值、游標、邊框與點擊座標分開處理；不得以高清圖覆蓋後畫的文字或提示。
- 同一 F### 用於主畫面、卡片、對白、選君主、自創君主、尋訪及兩種戰場；左側對白及攻方的鏡像保持。缺圖、錯尺寸、錯版本、來源雜湊不符時的回退須明定。
- SCG30／31 實際在 DATA2。正式 `NewArtScreen` 已接收 DATA2，`ArtScreen.Scene` 依實際來源取圖；兩版素材測試核對 30／31 的尺寸與解碼。AI 圖不補作原版證據。
- 地圖拓樸、旗幟陣型、遮罩、字幕、動畫揭露區及疊層先從既有契約導出。CP 與 MVM／MVO 未解用途不進首批。
- B、4× 及原貌預設均由使用者定案。§6 依盤點、現行 UI 及既有 AI 選項形成首批 READY 契約；完成靜態樣圖不將整份規格標成 `CONFORMED`。

後續驗收順序沿用 #107–#110：小批接入、正式玩家路徑、缺圖及錯資料回退、兩版／三語系、戰場分支、存讀檔及音樂；Windows／macOS 原生驗收獨立記錄。

## 6. 首批渲染及視窗選項列契約

### 6.1 素材包

入口為 `-hd-assets <目錄>`，省略時查執行檔旁的 `hd-assets/`。未提供有效包時原貌可正常啟動，高清選項顯示尚無素材。包內 `manifest.json` 的 schema 為 1，style 為 `b`，scale 為 4；這是顯示素材包，不改原版資料或存檔 schema。

每項記 `edition`、`container`、`name`、`source_sha256`、`file`、`sha256`、`width`、`height`。固定資源鍵僅接受 DATA3 的 F000–F255.FAC、SCG01–SCG29.IMG，及 DATA2 的 SCG30／31.IMG。首批只有 F000／F005／F228／SCG01，第二批另含 §6.7 的四個自創君主槽。尺寸必須等於原版寬高四倍，來源雜湊比對玩家自己的容器，PNG 雜湊與表頭尺寸相符；檔名限定包內相對路徑。重複鍵、越界鍵、錯來源、錯尺寸或損壞圖逐項停用，不阻塞其他有效圖或原貌。

載入後以實際索引像素及尺寸建立對應，另登錄肖像水平鏡像。只替換整張已辨識的素材，不用相似度辨識。跨版共用仍須各自記版本及來源雜湊。工具保存生成原圖與提示詞收據，公開包的權利仍未確認。

### 6.2 畫面及疊層

原貌持續畫入既有 640×408 CPU 畫布。高清輸出直接合成 2560×1632；原貌底圖、文字、數值、游標及圖框用最近鄰四倍，候選 PNG 保留自身高清像素。原版畫布不改寫，仍能獨立對拍。

高清素材在既有繪製順序中登錄矩形。其後在該矩形內繪製的文字、清底、框及游標保留覆蓋權，不能在最後一步讓高清圖蓋住它們。整頁底圖清掉前頁登錄。主畫面及戰場預合成肖像須在文字之前獨立登錄；人物卡、選君主、自創君主及對白沿用整張貼圖入口。尋訪的合成場景保留肖像定位，四種拉幕沿用 §010 的 Reveal／Source 與步數，僅把相同區間映到 4×。

`app.Layout` 回傳實際畫布尺寸及選項列高度。原貌完整保留 408 列，文字 fallback 保留自身 400 列。Theme 切換保持視窗中的遊戲大小及正在輸入的資料，轉場中不重新擲骰或重播。選項列以獨立畫布畫在遊戲上方，不改遊戲內部座標。

### 6.3 選項列與輸入

- 原貌及隱藏選項列為每次啟動預設，不把本次選擇寫入遊戲存檔；`-lang` 保留既有作用。
- 顯示時選項列高 32 個邏輯像素，下面是完整遊戲。展開／收起調整視窗高度，遊戲大小不變。
- Esc 展開或收起並鎖定目前顯示；滑鼠移至上緣 6 像素暫時展開，離開選項列及展開中的選單後自動收起。鍵盤展開不因滑鼠離開而收起。
- 語言為繁體中文、English、日本語；選擇即重畫標籤及可回譯的目前提示，不重開局、不取消數字輸入。既有已成句歷史紀錄無法重新取得參數時保留。
- Theme 為原貌與 B 高清 4×；有效首批包可切換，沒有有效圖時禁用高清並顯示原因。
- AI 強度沿用已實作的同版本還原 AI，以及強化 AI 1–5。強化數字對應 `Options.SetAIOrders`；模式沿用 `Session.SetBrain`／`Options.SetAIMode`。原版難度與開局 AI 等級不重算，既有 AI 存檔欄位照常保存。尚未開局的選擇套到新局，讀檔採存檔本身的 AI 設定。
- 選項列開啟時暫停遊戲更新，音訊繼續；鍵盤及滑鼠事件不送到原遊戲輸入。Esc 原有返回／取消動作保留在 Shift＋Esc。下拉選單選取後不重複送出按鍵。

### 6.4 首批驗收

在 Docker 的 Xvfb 視窗以正常片頭、主選單及新局操作驗證：隱藏預設、Esc／滑鼠上緣展開、移開收起、選項列在上、408 列不裁切；三語言、原貌／B 高清、兩版 AI 選擇與存讀檔。以原貌及高清截圖核對主畫面、人物卡、選君主、鏡像對白與場景四方向中間態。加入缺包、錯來源／版本、錯尺寸、重複鍵及前景遮擋反例；記錄時間、記憶體及輸出雜湊。正式音樂腳本依完整 408 列幾何更新後重跑；原貌既有 UI 對拍仍獨立執行。

### 6.5 首批接入與驗收入口

首批包由 [`cmd/san1hdpack`](../../cmd/san1hdpack/main.go) 準備。它只讀本機已保存的四張 4× 素材，輸出 PNG 及兩版各四筆 manifest，再以正式載入器逐項驗證。沒有這些本機圖片時不能重生 AI 美術；公開儲存庫不含圖片或原版資料。

```sh
tools/go.sh run ./cmd/san1hdpack -root /orig/三國演義 -peer-root /orig/三國演義1加強版
```

預設讀 `workplace/hd-preview/`，輸出至 `workplace/hd-assets/`；可用 `-input`、`-out` 指定已有目錄。正式遊戲使用 `san1 -root <原版目錄> -hd-assets <素材包目錄>`。

[`tools/verify-window.sh`](../../tools/verify-window.sh) 沿用既有 Go 建置與音訊擷取映像，以無網路容器建立引擎及首批包，再走兩版正常片頭、主選單、新局及存檔。重跑入口為 `bash tools/verify-window.sh`；前置資料與快取同 §009 的音樂驗證，另需 `workplace/hd-preview/` 的四張已保存素材。圖片與收據在 `workplace/hd-window/`，每次重跑覆寫同名驗證產物。

測試項目包含預設隱藏、上緣展開及離開收起、Esc 固定展開、三語系、AI 選擇不滲入遊戲輸入、2560×1632 主畫面與 2560×1760 選項列、高清肖像與 PNG 全部像素相同、切回原貌恢復面板、缺包回退，以及本次新寫入存檔的強化 AI 5。載入器與圖層反例、肖像鏡像及四方向拉幕各步由 `internal/ui/hd_test.go` 核對；它們是 remake 圖層驗證，不是高清美術與原版的像素對拍。

2026-10-02 的 Linux 正常視窗抽樣共 28/28 通過，含兩版 1280×600 寬視窗展開與恢復。原版／加強版在原生 4× 主畫面時的常駐記憶體為 393,664／375,180 KiB；測量包含軟體 OpenGL、字型、原版及高清素材，不是單張畫布用量。主選單等待約 33–34 秒，含正常片頭、續行按鍵與截圖辨識，不能視為素材載入時間。

| 本機驗證產物 | SHA-256 |
|---|---|
| `workplace/hd-assets/manifest.json` | `ebd9d31270f103a4fc0b792ab4eb8d31b24d4a3b4bb2c02343172d42107f1474` |
| `workplace/hd-window/san1-window-check` | `0be79f0c1455232c9b418edc91a5a2a0f8ee06f8eea96b5c17977ed6e607fd86` |
| `workplace/hd-window/receipt.json` | `d59b8b911345c43ebbbbba1364672592e5bf7f4f6b10d0a75fd2aaf729e605e0` |

選項列 §6.3 的首批正常操作已驗收；整份規格仍為 READY。AI 存讀檔的選項契約另由 `internal/menu/windowoptions_test.go` 驗證，GUI 本輪只走存檔。正式音樂五曲、兩版新局、靜音與恢復共 10/10 通過，收據見 [009 §6.1](009-music.md#61-正式音訊串流契約)。

目視三語系主選單後，日文按鈕採用相同意思的短標籤，保留六個原有位置與編號；`TestTitleLocalesStayInsideButtonFrames` 核對三語字型墨點不碰框線，繁中另由既有原版落點測試核對。此文字修正晚於上表 GUI 收據，另存 `workplace/hd-window/title-ja-fitted.png`；沒有把 GUI 收據的執行檔雜湊換成後續建置。

這輪接入只涵蓋三張肖像與一張地震圖。全部人物、場景、地圖及戰場高清化仍依 #108／#109 推進；正常玩家的各種人物卡、對白、尋訪、兩條戰場分支及跨平台驗收依 #107／#110 完成後，才可把整份規格標成 `CONFORMED`。

### 6.6 首批四圖的正常玩家路徑

入口為 [`tools/verify-hd-player.sh`](../../tools/verify-hd-player.sh)，操作與擷取由 [`verify-hd-player-inner.py`](../../tools/verify-hd-player-inner.py) 執行。沿用 §6.5 的兩個 Docker 映像及已準備的四張素材包，以非 root、無網路容器重新建置正式引擎。重跑使用 `bash tools/verify-hd-player.sh`；輸出在本機 `workplace/hd-window/player/`，保留 §6.5 既有收據，重跑本工具會覆寫自己的同名輸出。

兩版都走正常片頭、主選單、劇本 001、單人曹操及難度 5。先在選君主畫面核對 F005／F000，再透過「查看」選郡及「檢視將軍」打開曹操、劉備與鮑忠的資料卡。依人物資料，三人分別位於郡 11、8、7，清單序號為 1、1、2；F228 由鮑忠引用，沒有把尚未出場的魏續注入遊戲。

每張資料卡核對 256×320 原生高清肖像、素材矩形外的資料文字及外框，再切回原貌核對整個右側資料面板。地震使用正常休息、換月及對白續行，沒有直入畫面、修改日期、植入存檔或強制排入事件。正式新局沿用目前局面雜湊，沒有注入 LCG seed；此項驗證是 remake 玩家路徑，不是原版亂數或規則對拍。

地震核對 SCG01 的 704×384 原生像素及原貌／高清來回切換。右側文字的比較區從 y=200 起，避開同時顯示至 y=196 的高清君主肖像。四方向的每步揭露仍由 §6.5 的 CPU 圖層測試及獨立原貌 oracle 抽驗負責；本項不把單一自然事件外推成所有高清動畫或戰場分支完成。

兩版曹操路徑的 30/30 檢查通過；目視確認原版地震圖擷取於 190/1，加強版那張則是在 200/3 終局後顯示待播佇列，不能證明進行中的地震顯示時機。因此另以 `bash tools/verify-hd-player.sh --scene-plus` 從加強版片頭開單人董卓、難度 5 新局，4/4 通過；截圖顯示 190/1，仍有正常玩家主提示。首批 SCG01 的進行中收據以原版曹操及加強版董卓為準。

加強版窄驗證輸出在 `workplace/hd-window/player/scene-plus/`。場景輪詢直接用 X11 `XGetImage` 讀取 Xvfb，檢查尺寸、色彩遮罩及像素格式；完整 PNG 仍由 FFmpeg 保存。縮短 FFmpeg 分析範圍後單次取圖仍約 0.711 秒，直接讀圖的一次抽樣為 0.0091 秒，後者已在相同 220 秒場景期限內完成董卓正常路徑。這是本機單次量測，不代表引擎效能基準。

| 本機驗證產物 | SHA-256 |
|---|---|
| 兩份收據使用的正式引擎 | `4bb987ed8531d4a48f55f412cb238e29c25f3f701f6738b2f1984115f68f853b` |
| `workplace/hd-window/player/receipt.json`，30/30 | `4d2ea30eaf4c9b56b56666393db4e3130f22be2016c406abe8952c8b65259709` |
| `workplace/hd-window/player/scene-plus/receipt.json`，4/4 | `78fd06afa8f6a2ed66ec40b0c72114da7a7fb68290a6e3fee995efedfd137026` |
| 原版 190/1 的 `base-natural-quake-hd.png` | `cd2e07a1a55c99da7142a4c27c9db94d6290392b90b4c9d6cf128e5ed621c9f4` |
| 加強版董卓 190/1 的 `scene-plus/plus-natural-quake-hd.png` | `84126e9981f1b98746ced813b8c66fca3b83888445adc6201bda3bced11ef0a1` |

收據記錄來源、素材包、引擎、工具雜湊與按鍵序列。30/30 使用當時的 FFmpeg 場景輪詢；原文另存本機 `player/verify-hd-player-tested.py` 及 `.sh`，沒有把其工具雜湊換成後來的 X11 版本。最新工具已跑加強版董卓的窄驗證，未把兩份收據合稱一次完整重跑。場景圖片及原始資料仍只留本機。

### 6.7 自創君主肖像批次

本批依 #108 的自創君主優先順序，使用原版 DATA3 的 F011、F001、F015、F009。四個範本的年齡皆為 20；肖像槽順序由 `state.CustomLordPortrait` 及 [013](013-custom-lord.md) 的原始資料契約決定，不改人物索引或開局資料。來源及槽位是 `L0`、`[both]`；B 畫風仍屬 remake 美術差異。

| 資源鍵 | 原圖識別 | 生成原圖 SHA-256 |
|---|---|---|
| DATA3/F011.FAC | 藍盔、紅寶石、細鬚與短鬍 | `3aa22a8c7c9e42335bda66fc2a4543bb8aa0047cb7d1fdaca7d0200c87c6c26b` |
| DATA3/F001.FAC | 紫紅盔、灰藍飾、細鬚與短鬍 | `fb50f34e29091e7be6a74fe7eff9904963a6304ed254452ae59fdf0a1f7c832d` |
| DATA3/F015.FAC | 橘紅盔、淺藍飾、無鬚 | `48d8dde27842a4dc7c0abccec1a20a243041cc962fd530c936ecd6f042bb3fae` |
| DATA3/F009.FAC | 藍盔、紅寶石、較濃黑鬍 | `949a4978f16aa98eb95c05bbb3975ead88df4c23b3b0c980354b455fc903606f` |

四張原圖保存為本機 `workplace/hd-b-F###-v1.png`，皆為 1122×1402。完整提示詞、工具來源與逐張目視紀錄在 `workplace/hd-custom-batch.json`。內建 `image_gen` 未回報模型版本或 seed；本輪審查由 Codex 執行，尚無使用者逐張簽核，不稱為人工驗收。

封包工具可用 `-assets` 明列本批鍵，省略時保留首批四張。`-master-dir` 讀取已保存的 B 原圖，按來源槽尺寸四倍縮放整張圖，不裁切；比例誤差沿用 §3 的一個來源像素上限。每版來源實際尺寸及雜湊分別核對，來源 bytes 不同時不得共用同一候選圖。預先縮好的 PNG 路徑仍使用 `-input`。本批包另存 `workplace/hd-assets-custom-v1/`，不覆寫首批收據。

正式驗收從片頭、選劇本、選新君主進入能力設定、出現對白及開局主畫面，核對原生 256×320 圖、素材外文字與外框及切回原貌。劇本 001 只有兩個名額，另從實際含四名額的劇本抽驗其餘兩槽；不得注入肖像、日期或存檔代替正常輸入。本批通過不表示全部 256 個肖像、鏡像、尋訪或戰場完成。

重跑入口為 `bash tools/verify-hd-player.sh --custom`，輸出在本機 `workplace/hd-window/player/custom-v1/`。兩版各從片頭開劇本 001 的兩位及 006 的四位新君主，走設定、出現對白與正式主畫面，88/88 通過。每個肖像核對原生高清像素、素材外文字／框線及切回原貌；每次開局均預設隱藏選項列。中途沒有注入人物或存檔。

正常路徑找出的新君主勢力啟用及名額綁定缺口，已依 [013 §3.1](013-custom-lord.md#31-名額綁定與啟用) 修正；兩版六劇本的正常選單與存讀後三表相同通過 12 組，全庫回歸通過。原貌 dosgolem 複驗 `TestZZNewLordBornMatchesTheOriginal` 及 `TestZZTwoPlayersMatchTheOriginal` 通過，原版畫面由 dosgolem 自行重生至 `custom-v1/oracle-shots/`；維持既有字模排除與 43 格物價差異範圍，不宣稱加強版原版多人新局或整個回合的新 parity。

| 本機驗證產物 | SHA-256 |
|---|---|
| `workplace/hd-assets-custom-v1/manifest.json` | `58939e4bde20e9910cc73a3d033b28ab386ba6be2cbe1c5f2f4fdad72f3a7235` |
| `custom-v1/san1-window-check` | `a53b2dded3d22fc2c05c0f9a56590c7e2a0d9fc1d777c5d54c9a9804980787fe` |
| `custom-v1/receipt.json`，88/88 | `4522ec38f3f34731f8c59ec887219de9adaa11cc81ce5e5210b7a10eee44e57c` |
| `custom-v1/after-fix-test.log`，12 組 | `8018bf4d129954564e0b8534b3ca67e00aefed7c6bc313ac432d823892e1408d` |
| `custom-v1/all-tests.log` | `5f8ccb21560406aa0852627320258d4926c7642695f414f4ff0595a5d7a07106` |
| `custom-v1/original-parity.log` | `9e0c633c61219ed9051fb497cd067d00744667bdd1ac3a4906b34f2a9239a762` |

### 6.8 白鬚、獨眼與側面肖像批次

狀態：`READY`，依 #108 授權接入 F020、F184、F236、F006。原版單張圖逐張核對，保留白鬚、冠帽顏色、眼罩在畫面右眼、單眼可見的側面及原服色。肖像是原版固定槽，不依劇本年份重新生成不同年齡。

| 資源鍵 | 劇本 001 引用 | 採用原圖 | SHA-256 |
|---|---|---|---|
| DATA3/F020.FAC | 人物 217 嚴顏，資料年齡 40，原圖已為白鬚 | `workplace/hd-b-F020-v2.png` | `5b086d5aefe5be6d5a2cc7ca7f4da96166cbfdd9706fb40c02b4ff12663bc9c4` |
| DATA3/F184.FAC | 人物 29 夏侯惇 | `workplace/hd-b-F184-v1.png` | `15ac7980895e7c3388dddef68c6364d1a22091f652a2d72b479274e314736718` |
| DATA3/F236.FAC | 人物 110 陳珪，60 歲 | `workplace/hd-b-F236-v2.png` | `159adaff63c01bc447700c5bf8121b797fc87cbe215aad517e07eb4533d9601f` |
| DATA3/F006.FAC | 人物 23 呂布 | `workplace/hd-b-F006-v1.png` | `a6b98047380f5629a4968077c7afd4357e356fe98d66d68bd5ddc0a0d302dc94` |

完整提示詞、來源與候選雜湊在本機 `workplace/hd-portraits-identity-batch.json`。F020 第一版把紅色冠飾改為金色；F236 第一版增加額飾、轉三分面及服色偏藍，兩者均退件並保留原檔，採修正後第二版。由 Codex 逐張目視，尚無使用者逐張簽核；工具未回報模型版本及 seed。

封包工具用 `-master-revisions F020=2,F236=2` 明列採用版次，其餘維持 v1；版次只接受正整數，資源須在 `-assets` 中。它不覆寫生成原圖。十二張的累積本機包存於 `workplace/hd-assets-portraits-v1/`，含十一張肖像與 SCG01，兩版各十二筆；前兩批包及驗證收據保留。接入後仍須從正常「查看」路徑核對人物卡，再列入正常玩家完成範圍。

### 6.9 主要武將肖像批次

狀態：`READY`，依 #108 接入 F002、F004、F012、F013。沿用原版固定肖像，不因劇本 001 的諸葛亮尚未出場而把成人原圖改為兒童。

| 資源鍵 | 資料引用 | 生成原圖 SHA-256 |
|---|---|---|
| DATA3/F002.FAC | 關羽，保留藍巾、淡色及紅色額邊、長黑鬚 | `c6d1f21e79bbc139eada776c61d9a8ee591ccf0ed8fb2c7fee2d20488aae91bc` |
| DATA3/F004.FAC | 諸葛亮，保留高藍冠、細鬚及紅袍 | `b1992bd1f4bf9ef8e3e395453a0b9f5e088efe76c526b96eafcdb94bfc5eb8ee` |
| DATA3/F012.FAC | 張飛，保留藍巾、紅額邊、濃黑鬚及露齒表情 | `c3b29607cc8346e001bf9caa5b1d7abfcfe0df76776a21991824c6363969e662` |
| DATA3/F013.FAC | 趙雲，保留藍盔、藍冠飾、淡色額帶及無鬚面孔 | `1d2429c533ff39c749faf3b1f0df980de2ba07785c132ce348895481090c443e` |

原圖皆為 `workplace/hd-b-F###-v1.png`，1122×1402；完整提示詞、來源與逐張 Codex 審查紀錄在本機 `workplace/hd-portraits-heroes-batch.json`。工具未回報模型版本或 seed，尚無使用者逐張簽核。累積十六張本機包存於 `workplace/hd-assets-portraits-v2/`，含十五張肖像及 SCG01；採用版次維持 F020／F236 為 v2，其餘 v1，前批包保留。正常人物卡驗收仍須包含已出場的實際劇本。

§6.8／§6.9 的八張肖像，連同先前 F000／F005／F228，已從兩版正常新局「查看→檢視將軍」重跑。劇本 001 抽驗九張；諸葛亮及趙雲使用已出場的劇本 004 駐軍。每張核對原生 256×320 像素、素材外文字／框線與切回原貌；70/70 通過。沒有注入人物、存檔或事件，這是 remake 正常 GUI 收據，不升格為原版 oracle。

重跑入口為 `bash tools/verify-hd-player.sh --portraits`。完整資料及按鍵序列留在本機 `workplace/hd-window/player/portraits-v2/`。收據 SHA-256 為 `eba97716eb7a042398c8e27ece9860f2597a053a7a2a91e22366c3bb8413b7d2`；正式引擎與 §6.7 相同。素材包 manifest 為 `8cc78e42d7784b055c9832ea1212b6337fd3b53eec745aa2da0774271f152bcc`，準備紀錄為 `96870a9f2377ca4490fd73d30049772aed04fc7d5085973407001ec2fd944d9b`。十個非法版次 CLI 案例均拒絕且未建立輸出目錄，收據為 `862680209802e4fbf09369d6d6a8940489d6a2b20169bc1957b3d35832364d9e`。

### 6.10 孫堅、袁紹、董卓與孫權批次

狀態：`READY`。依原版六劇本的 typed 君主資料選出實際肖像槽，不依人物的通俗形象重畫冠帽或改服色。

| 資源鍵 | 資料引用與原圖識別 | 採用原圖 | SHA-256 |
|---|---|---|---|
| DATA3/F041.FAC | 人物 15 孫堅，灰盔紅飾、黑鬚、綠巾、朝左 | `workplace/hd-b-F041-v2.png` | `69fced14ccf7cd11041af57503ce4c170f39ef2ec75ff3571dc008a38700fe12` |
| DATA3/F053.FAC | 人物 16 袁紹，青盔紅飾、長黑鬚、正面 | `workplace/hd-b-F053-v1.png` | `de8ea70696841e08afaf13a856a07494e3534f7af4a9d43d949e928aae0d026b` |
| DATA3/F077.FAC | 人物 14 董卓，粉金高冠、藍紫飾、濃黑鬚、朝左 | `workplace/hd-b-F077-v2.png` | `1e5b49eb78f03d7c3fa14e57e29a674a8fa3571a2a94c2f0bed268111298830b` |
| DATA3/F008.FAC | 人物 56 孫權，黑髮無冠、黑鬚、灰衣紅領、朝左 | `workplace/hd-b-F008-v2.png` | `6ea4f88129b769aa3f51cc8b54e2b8fd947a3b3b4253168e4ed99fddcf324f9c` |

來源槽與引用為 `L0`、`[both]`；B 圖為 remake 美術差異。F041／F077／F008 第一版為 1072×1467，比例不符槽位，保留退件。第二版由 `image_gen` 擴展背景與肩部，採用圖皆為 1122×1402，不以程式拉寬人物。提示詞、來源、候選圖與採用紀錄在本機 `workplace/hd-portraits-lords-generation.json` 及 `workplace/hd-portraits-lords-ratio.json`。Codex 已逐張目視，尚無使用者逐張簽核；工具未回報模型版本及 seed。

累積包存於 `workplace/hd-assets-portraits-v3/`，含十九張肖像及 SCG01，兩版各二十筆。`-master-revisions` 明列 F020／F236／F041／F077／F008 為 v2。正常人物卡驗證另存新收據，不覆寫前批包。

重跑入口為 `bash tools/verify-hd-player.sh --lords`。兩版劇本 001 的袁紹／董卓／孫堅及劇本 004 的孫權，經正常查看、原生像素、文字／框線、切回原貌核對，28/28 通過。原版還原 AI 在第一次玩家停點前把孫權由第 23 郡移到第 21 郡，加強版仍在第 23 郡；先以正式 `session.AdvanceToHuman` 診斷，再於實際 GUI 驗證各自駐軍。失敗時選到張昭的收據留在 `lords-v3/failed-before-roster-fix/`，沒有改人物或遊戲規則。

| 本機驗證產物 | SHA-256 |
|---|---|
| `workplace/hd-assets-portraits-v3/manifest.json` | `af5404abc5a05b431d8d6e607f35ef35bb44d6e2be0c8f0e787206c50f003424` |
| `workplace/hd-assets-portraits-v3/preparation.json` | `f975102702da594927ec3d0e7ba93ee06088a1165968ab3047612810fe666309` |
| `workplace/hd-window/player/lords-v3/receipt.json`，28/28 | `abcc554af55213ecd0dd0685e41e925dcaf01a5ad9b7c4df66ec1453e78aeb89` |
| `workplace/hd-lord-player-stop.json`，remake 清單診斷 | `d23ba8c9b1c279eda8bcd149d5d83dfd4fd16903aeb83bced494c5f608e34ee5` |

正式引擎與 §6.7 相同。上述十九張肖像的正常 GUI 及清單診斷都不是原版 oracle 收據；模型／seed、使用者簽核、全批素材及跨平台限制照舊。

### 6.11 六劇本君主肖像批次

狀態：`READY`。由兩版六劇本的 typed 君主資料取剩餘二十個獨立 F###，各自以原版單張圖及定版 B 筆觸生成。DATA1 的相同肖像沿用對應槽，不另生成。

| 槽 | 君主 | 採用版次 | 保留的原圖特徵 |
|---|---|---|---|
| F054 | 袁術 | v3 | 綠冠、短黑鬚、紅衣 |
| F142 | 劉焉 | v2 | 藍冠、長白鬚、紫衣、左向 |
| F192 | 馬騰 | v1 | 紫盔、黑鬚、正面 |
| F081 | 劉表 | v2 | 黑髮、小金冠飾、黑鬚、藍衣 |
| F114 | 陶謙 | v3 | 藍冠紅飾、長白鬚 |
| F063 | 公孫瓚 | v1 | 金冠、短黑鬚、粉衣 |
| F166 | 劉繇 | v1 | 藍巾、長黑鬚、單眼可見的左側面 |
| F209 | 王朗 | v1 | 紅冠、白鬚、青衣 |
| F117 | 孔融 | v1 | 灰冠、小粉紅飾、黑鬚、左向 |
| F164 | 孫策 | v1 | 藍盔、無鬚、粉領、左向 |
| F097 | 李傕 | v1 | 青盔紅飾、短黑鬚、藍甲 |
| F237 | 劉璋 | v1 | 藍冠、黑鬚、青衣、左向 |
| F227 | 張魯 | v1 | 暗冠與金粉額帶、長黑鬚 |
| F128 | 楊奉 | v1 | 藍紫盔、紫色額飾、黑鬚、粉衣 |
| F024 | 金旋 | v1 | 藍盔黑冠飾、短黑鬚、紅衣 |
| F046 | 韓玄 | v1 | 藍花巾、白鬚、紫衣 |
| F098 | 趙範 | v1 | 綠盔紅飾、短黑鬚、灰甲 |
| F078 | 劉度 | v1 | 高藍冠粉飾、長黑鬚、紅衣、左向 |
| F119 | 孟獲 | v1 | 青盔紅飾、黑鬚、藍衣 |
| F229 | 曹丕 | v1 | 高灰冠青飾、粉紅額飾、無鬚、黑衣黃綠邊 |

來源引用為 `L0`、`[both]`；B 圖為 remake 美術差異。年齡欄不改固定肖像，例如王朗白鬚及金旋、劉度黑鬚。二十張採用原圖皆為 1122×1402，保存於本機 `workplace/hd-b-F###-vN.png`。F054／F142／F081／F114 第一版及 F054／F114 第二版比例偏窄，保留退件與完整提示詞，不用程式拉寬人物。各候選的圖檔、工具、參考、尺寸、SHA-256 與 Codex 審查在 `workplace/hd-lords-v4-generation.json`；未回報的模型版本與 seed 明記未知，使用者逐張簽核仍未完成。

累積包使用 `workplace/hd-assets-portraits-v4/`，含三十九張肖像及 SCG01，兩版各四十筆。`-master-revisions` 除既有五槽 v2，增加 F142／F081 的 v2 及 F054／F114 的 v3。前批包與收據保留。正常新局驗證須使用各版第一次玩家停點後的真實人物清單，不以生成圖或原始劇本表取代實際 GUI。

| 本機產物 | SHA-256 |
|---|---|
| `workplace/hd-lords-v4-generation.json`，含採用及退件 | `2967919ccc693e81a3ed05499c584407ad44ef74eccfc0ecfaf04c889cac737e` |
| `workplace/hd-assets-portraits-v4/manifest.json` | `5a702eacdce1b2757338671669f1883758594406ca0db896294a2ade5f46d212` |
| `workplace/hd-assets-portraits-v4/preparation.json` | `1e5fa16bbe4d6a80c0e8dafee6c6643908c07c2d1c65570b9a1f90a79e6db6b0` |
| `workplace/hd-lord-player-stop-v4.json`，第一次玩家停點診斷 | `a07f334e134866bfc78808f0d5b1d15fefefd139b767829efeeb4b6005f129e1` |
| `workplace/hd-preview/lords-v4-contact.png`，原圖與高清圖比較頁 | `8f29d8556f8fe32300acb3870309398c7e157001f3e64364ea4912a4ac69b985` |

重跑使用 `bash tools/verify-hd-player.sh --lords-all`。兩版均經正常片頭及單人曹操新局，劇本 001 抽驗十一張、002 五張、003 兩張、004 曹丕、005 孟獲；共四十張人物卡與十次啟動，130/130 通過。F128／F166 共用槽使用實際君主楊奉／劉繇，不用其他同槽武將代替。每張核對原生像素、素材外文字／框線及切回原貌。

收據為 `workplace/hd-window/player/lords-v4/receipt.json`，SHA-256 `ddecb7771bdc7fec57670d6e696648e355a45cae06be1aeaa42907940a2c5aa9`，引擎與 §6.13 相同。當時工具另存 `lords-v4/tested-verify-hd-player.sh`、`tested-verify-hd-player-inner.py` 及 `tested-verify-window-inner.py`；其後 wrapper 只增加素材包唯讀掛載，未以新 wrapper 的雜湊替換原收據。

初次批次外層期限不足及一次誤選張濟的收據分別保留於 `lords-v4/partial-before-batch-limit/` 與 `failed-before-path-fix/`。正式 GUI 的 `0` 為不耗回合的狀態命令；診斷先前多做一次休息，改以第一次玩家停點的實際清單後重跑。這些是驗證條件問題，不改規則或固定正式亂數，收據不升格為原版 oracle。

### 6.12 全 256 槽的肖像稽核

入口為 [`tools/hd-portrait-audit.py`](../../tools/hd-portrait-audit.py)，在 Docker 內執行。以兩版完整盤點為分母，逐槽核對包內來源與輸出雜湊、4× 尺寸、包內檔案路徑、準備紀錄、生成原圖比例、提示詞、工具及模型／seed 紀錄。重複鍵、未知槽、缺設定、損壞圖與越界檔案均列為技術問題。拒絕或缺少審查的圖不計入 Codex 審查，使用者簽核另列，不自動推定。

`--pack` 指定累積包，重複 `--records` 指定各生成紀錄，`--reviews` 可補記逐張審查，`--out` 指定既有私人目錄下的收據。普通稽核可回報未完成的批次；`--require-complete` 在任何槽缺圖或未經 Codex 審查時回傳 3，技術錯誤回傳 1，輸入契約錯誤回傳 2。這項 gate 不代替正常 GUI、使用者簽核或權利驗收。

實際 v3 包的 CLI 正反案例包含全量盤點缺項、重複登錄、錯來源／尺寸、PNG 損壞、絕對路徑、越界及符號連結、缺生成紀錄、拒絕的審查、原圖比例不符與不完整盤點。使用 Python `-O` 重跑，檢查不依賴可停用的 assert。收據留本機 `workplace/hd-audit-checks/receipt.json`，每例保存工具版本、命令、返回碼及報告雜湊。

十六例均通過，收據 SHA-256 為 `9c35ec86f3ea760274bc739b17caf47bf27f9caf3296f09f066cbec02fe8694b`。最新 v4 稽核為 39/256 槽兩版均備妥、Codex 審查 39、使用者簽核 0、缺圖 217、技術問題 0；`--require-complete` 正確回傳 3。完整逐槽報告在 `workplace/hd-portrait-audit-v4.json`，SHA-256 為 `306279c54cd38a7459e723148f7692fba14873cfd2bab0daf890d9fb147bc8c1`。

### 6.13 高清模式配樂驗證

入口為 [`tools/verify-music.sh`](../../tools/verify-music.sh)，重跑使用 `bash tools/verify-music.sh --hd`。沿用 [009 §6.1](009-music.md#61-正式音訊串流契約) 的工具鏈及量測判準，另需本機 `workplace/hd-assets-portraits-v4/`。先從原貌正常片頭啟動，再以選項列切為 B 高清，走音樂欣賞、兩版曹操新局、靜音及恢復。五首曲子與主選單共六段、新局兩段、靜音與恢復各一段，共 10/10 通過；靜音的 16 位元樣本全零。兩版新局同時核對原生 256×320 曹操肖像。

錄音及截圖另存 `workplace/audio/hd-v4/`，保留原貌收據。此項只核對 Linux 正式播放器的配樂輸出；未驗人耳聽辨、音效、語音或 Windows／macOS 原生音訊，不能代替 #110 的全部音畫驗收。原版配樂與含原版內容的產物只留本機。

| 本機驗證產物 | SHA-256 |
|---|---|
| `workplace/audio/hd-v4/music-check-receipt.json`，10/10 | `8ebcba462f9b164e0cf2aedc347ed7a9b7ed56dc1780f865597aa3a88850b666` |
| 配樂驗證使用的正式引擎 | `a42b688037b3fbe46153437d8cd7ed8aa02ad13f70c7306f2d0bb52d7f206340` |
| `workplace/hd-assets-portraits-v4/manifest.json` | `5a702eacdce1b2757338671669f1883758594406ca0db896294a2ade5f46d212` |

### 6.14 正常宣戰對白與鏡像

入口為 [`tools/verify-hd-dialogue.sh`](../../tools/verify-hd-dialogue.sh)，重跑使用 `bash tools/verify-hd-dialogue.sh`，前置環境沿用 §6.5，另需 v4 素材包及完整本機盤點。兩版均從正常片頭開劇本 001、單人曹操、難度 5，由陳留出兵洛陽，將曹操分配到第一軍、攜帶金 0 與米 1000。沒有植入人物、部隊、事件或亂數。

正常宣戰的曹操對白在 (552,80)，董卓對白在 (424,180)，後者水平鏡像。原生 4× 像素與包內肖像或其水平鏡像逐像素相同；其餘姓名、文字及框線不變，切回原貌恢復原面板。兩版共 14/14 通過，收據為 `workplace/hd-window/player/dialogue-v4/receipt.json`，SHA-256 `912cfa865d8a3e786952cea6113b3fe4740c494d099e1c0c94427a05224dadda`；引擎及素材包與 §6.13 相同。此項只補齊這條對白路徑，不涵蓋尚缺素材的 SCG06 高清動畫、尋訪或戰場 HUD。

滑鼠測試按住至下拉畫面實際展開／收起，保存畫面後才放開。先前瞬間點擊漏收，造成縮回 640×408 的畫面仍是高清肖像；失敗收據及工具留在 `dialogue-v4/failed-before-path-fix/`。此為測試輸入同步問題，沒有修改正式遊戲或重試亂數。

### 6.15 軍師與武將肖像批次

本批延續使用者定案的 B、4× 及原貌預設，依原版 DATA3 的實際肖像槽製作。十張均由內建 `image_gen` 各自生成，F053 的採用原圖只提供畫風與 4:5 畫布，人物識別依該槽原圖；不依劇本年齡改成另一張臉。王楷及逢紀分別由正常曹操新局的尋訪及鄴郡戰場補入。

| 槽 | 資料人物 | 保留的原圖特徵 | 採用版次 |
|---|---|---|---|
| F007 | 龐統 | 金色帽、粉紫帽緣、無鬚寬臉、藍衣 | 1 |
| F010 | 司馬懿 | 綠冠、紅色冠飾、正面嚴肅面容、灰白長鬚 | 2 |
| F014 | 周瑜 | 朝左面容、青冠紅飾、無鬚、藍衣 | 1 |
| F062 | 黃忠 | 朝左單眼側臉、黑盔紫邊、黑長鬚、綠色衣領 | 2 |
| F146 | 典韋 | 紫盔綠飾、厚黑鬚、寬臉、深紅衣領 | 1 |
| F155 | 夏侯淵 | 藍盔藍飾、雙眼、黑鬚、青藍衣領 | 1 |
| F156 | 許褚 | 粉紫布帽、厚黑鬚、壯碩面容、紫衣 | 1 |
| F254 | 陸遜 | 紫色文官帽、無鬚、朝左成人面容、淺色衣領 | 1 |
| F210 | 王楷 | 朝左低頭、黑髮髻、淺青髮飾、無鬚、綠衣 | 1 |
| F111 | 逢紀 | 淺金卷帽、向上帽飾、黑長鬚、藍衣 | 1 |

提示詞、來源、生成原圖、工具資訊及 Codex 審查保存於本機 `workplace/hd-commanders-v6-generation.json`。黃忠第一版因多露出另一眼及衣領服色偏差被拒絕，司馬懿第一版因黑鬚偏離灰白鬚髮被拒絕，候選仍保留。十張原圖均為 1122×1402，完整縮放至 256×320，不裁切、不拉伸。尚無使用者逐張簽核。

累積包另存 `workplace/hd-assets-portraits-v6/`，保留 v4、v5 包與原有驗證收據。準備及稽核仍走 §6.5、§6.12 的正式入口，正常玩家路徑另記驗證結果。

人物卡入口為 [`tools/verify-hd-player.sh`](../../tools/verify-hd-player.sh)，重跑使用 `bash tools/verify-hd-player.sh --commanders`。從兩版片頭開劇本 001–004、單人曹操、難度 5，依正式新局第一次玩家停點的清單查看本批十位人物。清單、人物索引與肖像鍵另存 `hd-commanders-card-plan-v5.json`；包升為 v6 時人物清單未變。

兩版二十張正常人物卡及八次新局共 68/68 檢查通過，原生高清像素、素材外文字／框線及切回原貌均相符。一百六十九個最新擷取檔的尺寸與雜湊回讀一致，工具快照與目前版本也符合收據。司馬懿及黃忠的正式人物卡已目視。

| 本機產物 | SHA-256 |
|---|---|
| `hd-assets-portraits-v6/manifest.json`，兩版各五十筆 | `eab57806cfc050dc5dfb086fec5569f5f7460ef0f6bb08e4ad691626ed4f57f7` |
| `hd-assets-portraits-v6/preparation.json` | `6ff92293ec5b6bcf0dc2f48bb25f843c6b3c969d416c0438373bf36a584ad2f4` |
| `hd-commanders-v6-generation.json`，含兩張拒絕候選 | `39569c7c200fdaa5ce1abfa4bcecd734a05c3ab578003e63374a97a565717e11` |
| `hd-portrait-audit-v6.json`，49 槽兩版備妥／Codex 審查、缺 207、技術問題 0 | `a6d46d6344d2cab2fab574e6fa47c10b13761e5f116cb9c7eba46ad401715e28` |
| `hd-commanders-card-plan-v5.json`，兩版第一次玩家停點的實際清單 | `d7df9b8458247d254519760a7821b89c443df7442e940a001aecdcd2f92b9a92` |
| `hd-preview/commanders-v6-contact.png`，原圖與高清圖比較頁 | `1efcb7de6e01e17aa5c0a7dde201da16d01330a74847a2d704dc1ee99eea424f` |
| `hd-window/player/commanders-v6/receipt.json`，68/68 | `852a4887d25059aac57d066e8b66506036a591b5b58e469e558572181a4d9af5` |
| `hd-window/player/commanders-v6/san1-window-check` | `0d3a08a977d1c3948d7ce17898f1a7c646b0b62f4146d27006e14fe7dbd1f472` |
| `hd-window/player/commanders-v6/verify-hd-player-inner.py` | `02efbafbe7bf994577f13befe6eb156cc0b384eddd629a007c5405c20048dd4d` |
| `hd-window/player/commanders-v6/verify-hd-player.sh` | `2ddb3d8a1a8dc751ec28c8abdd2feea72e4becd17217af707b4262249d787301` |
| `hd-window/player/commanders-v6/verify-window-inner.py` | `d7f0f648e4d50ce76cd6ffa1fc8b345d8e4d5662f4e2515c40c5b244b39d38a0` |
| `hd-v6-delivery-verification.json`，本批兩份 GUI 收據及 301 個最新擷取檔核對 | `a2c7f64e880c35fc1d461f281f2def421346b0749aff913ef4ce77d6527330c5` |

以上路徑均位於本機 `workplace/`。全批 gate 仍回傳 3，使用者簽核為 0，沒有把技術稽核當成全批美術完成。

初次片頭同步與抓圖耦合的失敗資料保留，改法見 §6.16。十二分鐘批次雖有 68 項成功及完整收據，外層在收尾時返回 124，整批仍判未完成；截圖、收據及當時工具存於 `commanders-v6/failed-before-18m-limit/`。依實際耗時改為十八分鐘有界批次，乾淨重跑返回 0，沒有放寬像素判準或修改正式遊戲程式。

### 6.16 正常尋訪、寬窄主戰場與查看

入口為 [`tools/verify-hd-contexts.sh`](../../tools/verify-hd-contexts.sh)，重跑使用 `bash tools/verify-hd-contexts.sh`。前置環境沿用 §6.5，另需本機 v6 包及完整盤點。兩版均從片頭開劇本 001、單人曹操、難度 5，不植入人物、事件、部隊或亂數。

尋訪由曹操執行，正常找到王楷，肖像在 (488,88)，結果回報的曹操肖像在 (552,180)。主戰場由陳留出兵鄴郡與洛陽，曹操分配第一軍、金 0、米 1000，收完進場對白後用正式 `9` 鍵完成預設紮寨，再正常 `7` 鍵查看部隊。

| 版面 | 攻方肖像，水平鏡像 | 守方肖像 | 查看肖像 |
|---|---|---|---|
| 鄴郡，窄版 | 曹操 F000，(456,52) | 逢紀 F111，(552,164) | 曹操 F000，(552,276)，不鏡像 |
| 洛陽，寬版 | 曹操 F000，(72,276) | 呂布 F006，(360,276) | 曹操 F000，(552,276)，不鏡像 |

每處先核對原貌來源，再核對原生 256×320 高清或其水平鏡像；人物資料、姓名與外框核對素材矩形以外的像素，切回原貌核對原面板。窄版查看頁遮住上方兩個軍力面板，只剩下方查看肖像可見；上方遮蔽區仍須完整符合原貌。寬版查看時三張肖像均可見。

兩版六次正常新局、十二處畫面，共 70/70 檢查通過。回讀一百三十二個最新擷取檔，尺寸、雜湊及擁有權相符；工具原文也存於收據目錄，與收據及目前版本的 SHA-256 一致。

| 本機 `workplace/hd-window/player/contexts-v6/` 產物 | SHA-256 |
|---|---|
| `receipt.json` | `6cbfc56b09608ea5e907e6748d7f3139be3f5c83ced34ccbfcdd9324c9254e2c` |
| `san1-window-check` | `0d3a08a977d1c3948d7ce17898f1a7c646b0b62f4146d27006e14fe7dbd1f472` |
| `verify-hd-contexts-inner.py` | `a5b60aca492f05470bcc0f8ac688854a6d559762e93d313b9ebaf73369591390` |
| `verify-hd-contexts.sh` | `b6ae00e859c120ccba1e9bd32ac309055110540a6fca457c142c686060bf1916` |
| `verify-window-inner.py` | `d7f0f648e4d50ce76cd6ffa1fc8b345d8e4d5662f4e2515c40c5b244b39d38a0` |

片頭輸入與抓圖分開，每次 Space 按下／放開各 80 ms，主選單辨識後停止並確認輸入執行緒已結束；單次片頭期限仍為 45 秒。舊工具在抓圖後才送鍵，經常只結束當下等待，錯過動畫中斷時點。修正的是驗證工具同步，正式 Go 程式未改。

窄版查看的初次驗證誤把被頁面遮住的肖像當成可見，依實際原貌畫面修正預期後重跑。失敗收據與當時工具保留於 `failed-before-inspect-occlusion/`；先前片頭同步失敗資料保留於 `failed-before-input-stream/`。

此範圍不涵蓋對戰子畫面、快速戰鬥、其他戰場肖像或尚未製作的地形／旗陣高清素材。

### 6.17 主畫面命令的七張場景批次

狀態：`READY`。依 #109 及使用者定案的 B、4×、原貌預設製作七張場景。來源槽、尺寸及既有命令用途以兩版盤點、正式 `ArtScreen.Scene` 與 [010 §1.1、§8](010-screen-transitions.md) 為準；原版呼叫點為 `L0`、`[base]`，§8 已抽驗命令的骰序為 `L1`、`[both]`。高清美術是 remake 差異，不宣稱與原版像素相同。

接入前的證據審查已核對七張採用圖、三張退件、兩版來源雜湊相同、原尺寸 176×96、原圖比例及完整提示詞；依上述輸入、邊界與驗收契約轉為 READY。

| 槽 | 容器 | 正式用途 | 採用版次 |
|---|---|---|---|
| SCG06 | DATA3 | 發動戰役 | 1 |
| SCG09 | DATA3 | 徵兵、購買武器 | 1 |
| SCG15 | DATA3 | 訓練、調整兵力 | 2 |
| SCG20 | DATA3 | 調動軍隊、運送錢糧 | 1 |
| SCG24 | DATA3 | 賞賜、賜物 | 1 |
| SCG30 | DATA2 | 買米、賣米 | 2 |
| SCG31 | DATA2 | 開墾、治水 | 2 |

輸入為 `workplace/hd-b-SCG##-vN.png`，原圖 11:6、完整縮放為 704×384。不得裁切、拉伸人物或把文字烘焙進圖片。保留原構圖、衣著、旗幟配色與人物方向；不得因命令名稱在 SCG20 加入原圖沒有的車馬。四方向切入仍沿用 010 的 22／24 步、對邊整塊滑入、原座標及不收輸入契約，不改規則、亂數或存檔。

提示詞、參考圖、工具、原圖尺寸、來源與候選雜湊及 Codex 逐張審查保存在 `workplace/hd-scenes-v1-generation.json`。SCG15／30／31 第一版比例不符，退件保留；採用第二版由 `image_gen` 重構畫布。工具未回報模型版本與 seed，使用者逐張簽核為 0。全部圖與含原版資料的產物只留本機私人驗收。

累積包另存 `workplace/hd-assets-scenes-v7/`，包含既有 49 個肖像槽與 8 張場景，兩版各 57 筆；前批包與收據保留。接入前核對兩版來源 bytes、原圖比例、圖檔形態與擁有權。失敗時依既有 §6.2 回退原貌，不猜測新鍵、命令用途或場景內容。

驗收須從兩版正常片頭、新局、原有選單與命令進入場景。核對原生 704×384 像素、原貌切回、素材外文字與框線，並直接擷取實際 X11 動畫中間幀。獨立幾何判準按 010 §3 核對四方向的實際高清素材全部步數；正常 GUI 未捕獲的方向或步數分開記錄，不用靜態終點代替動畫完成。SCG 全族分母為 31；已準備、Codex 審查、使用者簽核與缺圖分別列數字，不將 SCG11 的素材存在推定為正式用途。

兩版正式載入器均接受 57 筆，無警告。SCG15／30／31 採用圖為 1698×926，SCG20 為 1699×926，依既有一個來源像素內的比例誤差契約接受；其他採用圖為 1698×926。來源盤點中的使用端文字是歷史快照，例如 SCG30／31 的舊未接入敘述不代表目前狀態，正式路由以 §5、010 及目前程式為準。

[`tools/hd-portrait-audit.py`](../../tools/hd-portrait-audit.py) 新增 `--family scenes`，沿用來源、PNG、生成設定及審查契約，SCG01–29 必須來自 DATA3、SCG30／31 必須來自 DATA2。SCG01 的 Codex 補記在 `workplace/hd-scenes-pilot-review.json`，不推定使用者簽核。最新稽核為 8/31 兩版備妥、Codex 審查 8、使用者簽核 0、缺 23、技術問題 0；`--require-complete` 返回 3。原有 16 個肖像及新增 8 個場景 CLI 正反例均在 Python `-O` 下通過，保存於 `workplace/hd-audit-checks-v7/`。累積包的肖像重驗為 49/256、缺 207、技術問題 0。

完整圖層入口為 [`tools/hd-wipe-check.go`](../../tools/hd-wipe-check.go)，Docker 內使用以下命令。它透過正式容器及高清載入器讀取實際素材，獨立按 010 §3 計算預期矩形及來源位置，核對整張 800×440 圖層、素材外像素、原貌畫布與結束閘門；不拿實作的 `Reveal`／`Source` 作預期。

```sh
tools/go.sh run ./tools/hd-wipe-check.go -root /orig/三國演義 -edition base -pack workplace/hd-assets-scenes-v7 -out workplace/hd-wipes-v7-base.json
tools/go.sh run ./tools/hd-wipe-check.go -root /orig/三國演義1加強版 -edition plus -pack workplace/hd-assets-scenes-v7 -out workplace/hd-wipes-v7-plus.json
```

兩版各 8 張場景、四方向的 24／24／22／22 步，共 736 個完整幀通過，合計 1,472 幀。此為實際素材的 remake 圖層驗證，不能代替正常玩家或原版 oracle。

| 本機 `workplace/` 產物 | SHA-256 |
|---|---|
| `hd-assets-scenes-v7/manifest.json` | `b40ca7121dc1b78ad52530e081055364e6b2074dda2da4631743fa560d57ac9b` |
| `hd-assets-scenes-v7/preparation.json` | `45569b93738c0a9bdc883df7c599a859088f6153ff1268567b6d18a156b9b176` |
| `hd-scenes-v1-generation.json`，7 張採用與 3 張退件 | `0aa05e51724f7b6a54c2327c143958dbba0940b76c6033fb0541de0c01933cd9` |
| `hd-scene-audit-v7.json` | `391e0dc1011f19d563441a87e92dc731e362e2e8c89c2ce2e0bbb35761f74663` |
| `hd-portrait-audit-v7.json` | `034eb2a2ef58fb257edba6b9b6a434f182054cedbc6d7bdc283f5d822045a3aa` |
| `hd-wipes-v7-base.json`，736 幀 | `4718c3511c6a72c862d07769d01b07ae1d459003b47606449993293a797f0cab` |
| `hd-wipes-v7-plus.json`，736 幀 | `eee869bdb6cca44e69cf599b96da79bf9dcf37b7864f8c08a33cc02a8d3fbf2f` |
| `hd-preview/scenes-v7-contact.png`，8 張原圖與高清比較頁 | `fbe57efd41c731dfda8434731f2a4e3aced3d89039f432ef11fe14bd9050c906` |

正常視窗入口為 [`tools/verify-hd-scenes.sh`](../../tools/verify-hd-scenes.sh)，重跑使用 `bash tools/verify-hd-scenes.sh`；前置映像及原版資料沿用 §6.5，另需 v7 包與完整盤點。從兩版正常片頭開 001、單人曹操、難度 5，各場景重新開局，以原有選單與命令進入。X11 擷取從最後命令輸入前開始，保存已捕獲中間幀的 RGB、方向、步數、來源／目的矩形、時間及雜湊。幾何按 010 §3 獨立計算，核對揭露區全部像素與首個捕獲幀之後的未覆蓋區；不同驗證範圍分開記錄。

宣戰以陳留出兵洛陽，曹操編入第一軍、金 0、米 1000；調動由陳留至相鄰無主的譙郡 12，兩版正式第一次玩家停點已查證為 11、郡 12 的主人為 255。徵兵選第一位、增加 1；賞賜第一位將軍 1 金，交易買 1 金的米；訓練與開墾均經原有命令，不植入人物、事件、日期或亂數。這些是 remake 正常操作，未作新的規則或原版 seed 對拍聲明。

兩版各七次片頭／新局、十四次正常命令，共 140/140 檢查通過。704×384 終點與切回後的高清圖逐像素符合包內素材，176×96 原貌符合來源；下方 (408,200) 的 224×92 文字與框線符合原貌的四倍最近鄰縮放。其他肖像另有高清圖，未把整個視窗都宣稱為原貌像素相同。

| 場景 | 正常命令捕獲的方向，兩版相同 | 每版非終點步數 |
|---|---|---|
| SCG06 | 3，向左 | 1–21 |
| SCG09 | 1，向上 | 1–23 |
| SCG15 | 1，向上 | 1–23 |
| SCG20 | 3，向左 | 1–21 |
| SCG24 | 2，向右 | 1–21 |
| SCG30 | 0，向下 | 1–23 |
| SCG31 | 2，向右 | 1–21 |

合計 306 個實際 X11 中間幀，兩版均涵蓋四方向。每段從第一步開始、步數完整且遞增，揭露區全部像素符合對邊滑入的來源位置，首幀後未覆蓋區保持相同。正常命令每張只涵蓋表中實際方向；其他方向由前述完整圖層驗證另證。沒有反覆重擲或挑選方向作通過證據。首個擷取幀之前的遮蔽區不在此 GUI 比較範圍。

收據目錄保存三份工具原文、執行檔、157 個最新 PNG 與 306 份壓縮 RGB；片頭及選項列同名圖片依最後一次擷取核對。方向比較頁由實際中間幀轉成 PNG，只加標籤，沒有重新生成動畫圖片。完整批次外層返回 0。

| 本機 `workplace/` 產物 | SHA-256 |
|---|---|
| `hd-window/player/scenes-v7/receipt.json`，140/140 | `a723e712152453be87fa4ffb74a56a1f3a9a43a22de1234c8c9502ec7af66d8e` |
| `hd-window/player/scenes-v7/san1-window-check` | `b7fc59b7cbe365c7b7912805184321a592f3a5c3ad109184e3ae83647f893ee6` |
| `hd-window/player/scenes-v7/verify-hd-scenes-inner.py` | `0fa8da115ec6d1ccc0da8e684525b05b3ef3e727e7e205086173b6ab689b660a` |
| `hd-window/player/scenes-v7/verify-hd-scenes.sh` | `1aa98a0a71d06e348fed3a23189ba448028c6a84ca649702a0f1306115f6480a` |
| `hd-window/player/scenes-v7/verify-window-inner.py` | `d7f0f648e4d50ce76cd6ffa1fc8b345d8e4d5662f4e2515c40c5b244b39d38a0` |
| `hd-audit-checks-v7/receipt.json`，24/24 | `8f8c3c90bf23d62b04aa397e37bcf42036713d6f14699ddbf721bc90354f2195` |
| `hd-preview/scenes-v7-directions.png`，四方向實際中間幀 | `54f93baf71e6171ba133817562a7378ad4f65783abd7bf95aafe64a85ae02d51` |
| `hd-v7-delivery-verification.json`，157 張最新 PNG、306 個中間幀及工具／素材回讀 | `aa1c3a1851f95983350b7b77ee6e0d7897291d4d6078098ee9c6c8c238c5a7ef` |

本批沒有修改正式 Go 程式、規則、seed 或存檔，未新增原版 oracle 收據。使用者逐張簽核、其餘 23 張 SCG、其他素材家族、對戰子畫面、快速戰鬥及其他高清動畫仍待完成。Linux 配樂的先前 10/10 收據維持 §6.13 的範圍，未在此批重跑或擴張平台聲明。

### 6.18 正常對戰子畫面與快戰

沿用 READY 的肖像、圖層及輸入契約，補 #107 的既有正式使用端驗證，不新增美術、規則或玩家路徑。來源與位置依 [005 §8](005-main-screen.md)、[014](014-art-main-overlays.md)、目前 `cmd/san1/battle.go` 與 `internal/ui/artbattle.go`，原版證據範圍沿用原規格，不由 remake 診斷升格。

兩版從片頭正常開劇本 001、單人董卓、難度 5，第一次停點為上黨 6；依序讓上黨 6、京兆 16 休息，經軍師勸諫及確認後輪到洛陽 15，再以原有指令出兵陳留 11，只派呂布並分到第一軍，金 0、米 1000。正式新局／出兵診斷確認兩版均可在 (Q=4,R=3) 合法紮寨，方向 `2` 為相鄰曹操部隊；UI 以 `3,3,6,3,0` 移游標並確認，不修改部隊座標。診斷位於本機 `workplace/hd-battle-branches-{skirmish,quick}-plan.json`，不當成正常 GUI 或原版 oracle。

正常入口為 [`tools/verify-hd-battle-branches.sh`](../../tools/verify-hd-battle-branches.sh)，使用 `bash tools/verify-hd-battle-branches.sh`，可加 `--edition base|plus` 或 `--mode skirmish|quick` 縮小診斷。工具鏈及原版素材沿用 §6.5，需本機 v7 包與完整盤點。就緒辨識由 [`tools/hd-battle-branches-reference.go`](../../tools/hd-battle-branches-reference.go) 用正式翻譯、字型及文字視窗重生，僅作輸入同步參考，不替代實際玩家抓圖。

驗收核對窄版攻方呂布 (456,52) 的鏡像、守方曹操 (552,164)、對戰子畫面的同位部隊肖像，以及正常 `7` 查看呂布 (552,276)。每處比較原生 256×320 高清像素、文字／框線與原貌恢復；查看頁遮住的上方面板仍符合原貌。快戰另核對正式 `3` 與方向提示、合法交戰後軍力資料變化及持續可操作，不能只靠同一張戰場終點圖推定有執行快戰。

兩版各走對戰與快戰，共四次正常片頭／新局，117/117 檢查通過，完整批次返回 0。對戰依序驗證進入子畫面、`7` 查看、Shift＋Esc 返回、`0` 與 `Y` 休息，再核對左欄 (8,228) 的 32×96 時刻框推進；部隊主面板不在每次子命令後更新，不能拿它當作子戰鬥未動的證據。快戰核對戰前／戰後軍力面板變化及下令畫面恢復；原版釋放一名俘虜後恢復，加強版沒有此裁決停點。

輸入同步同時核對玩家提示與原貌主事者肖像。郡 14／15／16 對應董卓 F077、賈詡 F160、張繡 F147；提示先出現而電腦回合對白尚未清空時，先用正式按鍵清完對白。軍師繼續確認及俘虜裁決都由畫面辨識後再送鍵，不用固定等待秒數推定已進入下一階段。

獨立回讀 194 個最新 PNG、四份工具快照、執行檔、原始輸入及包內 114 筆素材，雜湊與尺寸相符。十四組戰前、子畫面、查看、返回、續玩及快戰戰後圖，另逐像素核對原貌來源、原生高清、肖像外文字／框線與原貌恢復，全部通過。查看遮蔽及時刻／軍力更新也獨立回讀；未把地圖標記的反白閃爍當成操作成功。

| 本機 `workplace/` 產物 | SHA-256 |
|---|---|
| `hd-window/player/battle-branches-v7/receipt.json`，117/117 | `7a4029184811dcb0497ec4c1bf08ffeaa988a8db5f84372d15c877be38059d9a` |
| `hd-window/player/battle-branches-v7/san1-window-check` | `4e12af406fcf9bf2fa474c41d8d51d23da701e90a9d5ebd65845f78d263c24d9` |
| `hd-window/player/battle-branches-v7/verify-hd-battle-branches-inner.py` | `fee87a97c49731851ec0fda4476757cfb7f272bf3d5d9a2c073c070769918257` |
| `hd-window/player/battle-branches-v7/verify-hd-battle-branches.sh` | `e83432dc99eb999f6636b00b60280337c8e392b05215a0e28ff6c34790c7e44e` |
| `hd-window/player/battle-branches-v7/verify-window-inner.py` | `d7f0f648e4d50ce76cd6ffa1fc8b345d8e4d5662f4e2515c40c5b244b39d38a0` |
| `hd-window/player/battle-branches-v7/hd-battle-branches-reference.go` | `909f4031a9dc9f72e7df992d3d633c4349bb3a2ecbb5bfdd077c1a3448333aab` |
| `hd-battle-branches-v7-verification.json`，獨立回讀 | `3640daff65318319cac4932f0fd5c7ed20d13abd3fead604c6933dfc2ec139d6` |
| `verify-hd-battle-branches-delivery.py`，私人回讀工具 | `8dbaf0ba01589ac5492eaf7c4c3a1bb93088c15bb8f25a524440837ab873f496` |

圖片、原版資料、失敗資料與完整收據只留本機。此批僅新增驗證工具與文件，素材仍為 49/256 肖像及 8/31 SCG 場景，正式 Go 程式、規則、seed 及存檔未改。原版 oracle、配樂、三語系、其他動畫／遮罩及跨平台的既有限制不因此擴張，021 維持 READY。

### 6.19 人事、築城與戰場事件插圖批次

本節狀態：READY。八張候選的兩版來源、比例與 Codex 逐張審查已通過，正式載入器兩版各接受 65 筆且沒有警告。正常玩家驗收另列，不由素材載入推定完成。

| 場景 | 既有使用端 | 邏輯位置 |
|---|---|---|
| SCG04 | 登用成功、登用他國人才成功 | (432,80) |
| SCG05 | 任命軍師、主事者 | (432,80) |
| SCG07 | 登用前對白 | (432,80) |
| SCG12 | 戰場退兵 | (448,268) |
| SCG13 | 登用他國人才前對白 | (432,80) |
| SCG17 | 戰場射箭 | (448,268) |
| SCG21 | 築城 | (432,80) |
| SCG26 | 撤職、釋放俘虜 | (432,80)、(448,268) |

使用端依 [010](010-screen-transitions.md) 與目前程式，不推定新的原版行為。盤點已確認八鍵均來自 DATA3、176×96，兩版來源 bytes 相同。以每鍵原圖為構圖參考，SCG06 的 B 候選僅作筆觸參考；輸出不透明 11:6 原圖，完整縮放至 704×384。保留人物相對位置、衣著、動作、旗色及原場景主體，不加入文字或新物件。每鍵獨立生成，比例或構圖不符即保留退件並修正。

提示詞、參考路徑、來源及候選雜湊、採用理由與未回報的模型／seed 分別記錄在私人 `workplace/`。Codex 審查與使用者簽核分列，不因 B 畫風已定案推定逐張簽核。原版規則、日期、亂數、存檔及命令路由不改。正常玩家驗收必須經片頭、新局與原有命令，診斷及靜態圖層驗證不能取代正常 GUI；尚未驗證的結果保留未完成狀態。

生成紀錄為 `workplace/hd-scenes-v2-generation.json`，八張均採第一版。SCG21 採用圖為 1699×926，其他七張 1698×926，依既有一個來源像素內的比例誤差契約接受，沒有裁切。新包為 `workplace/hd-assets-scenes-v8/`，含 49/256 肖像與 16/31 SCG，兩版各 65 筆。場景缺 15、肖像缺 207；兩族技術問題均為 0，使用者逐張簽核均為 0。

沿用 §6.17 的全族稽核及完整圖層工具，v8 兩版各 16 張場景、四方向的全部 24／24／22／22 步，共 1,472 幀通過，合計 2,944 幀。這是實際素材的 remake 圖層驗證，不能代替正常玩家或原版 oracle。

| 本機 `workplace/` 產物 | SHA-256 |
|---|---|
| `hd-assets-scenes-v8/manifest.json` | `2c5f7cacb5f044194087f35b5ca1d7605432e0a4e79360913fbc04eccd94ea25` |
| `hd-assets-scenes-v8/preparation.json` | `9aa5153e5424adb0fd43c9f8ebf614d2eecd3a88d2beed94367017188f06ac14` |
| `hd-scenes-v2-generation.json`，八張採用 | `5f7205eca291fdd4fa33bf63edd2b3a7d39b47e5e5831bdc82585431256e0e44` |
| `hd-scene-audit-v8.json` | `c58db1060bf78c23b5eef349b55678cf12a29aa475bf10599a4b3d185af493a9` |
| `hd-portrait-audit-v8.json` | `f87ab62472428f81f5ac56645f24aa84ebbfc487df09844d50c59852b986539b` |
| `hd-wipes-v8-base.json`，1,472 幀 | `81630027d2403ebaf44562934b854da5952886032a463bc8d7044f333ed83e86` |
| `hd-wipes-v8-plus.json`，1,472 幀 | `bb4643c39e6eb66466f308b70c8d2a98e3e9955975f933aa8a855d332ba7bfbd` |
| `hd-preview/events-v8-contact.png`，原圖與高清比較頁 | `86d6ac7887e47b223a5832e44f5e003028ad82c9df340774f93ab4ca9444f874` |

正常視窗入口為 [`tools/verify-hd-events.sh`](../../tools/verify-hd-events.sh)，使用 `bash tools/verify-hd-events.sh`，可加 `--edition base|plus` 或 `--only SCG##` 縮小診斷。前置工具鏈及原版素材沿用 §6.5，需私人 v8 包與完整盤點。實際 X11 擷取沿用 [場景工具](../../tools/verify-hd-scenes-inner.py) 的獨立幾何判準，依場景位置讀取主畫面或戰場下方的 704×384 矩形；原貌及素材外文字另核對。

人事路徑以 001 曹操登用名單第二位張邈、撤職名單第一位夏侯惇，以及登用洛陽名單第一位他國將領；任命場景以 001 劉備指定關羽為軍師。築城另開 003 曹操，弘農 14、潁川 13、上黨 6 依序休息，輪到洛陽 15 後，選楊修並按 `2,3,3,0,Y` 在第 27 格築城。正式新局與命令診斷確認原有金 3,118 足以付 3,000，關寨 3→4、金剩 118。路徑與提示由 [參考工具](../../tools/hd-events-reference.go) 重生，診斷不替代正常 GUI。

戰場路徑沿用 §6.18 的董卓新局與洛陽呂布攻陳留。退兵在既有 (Q=4,R=3) 紮寨點選第一個合法逃郡譙郡 12；弓箭另在 (Q=1,R=4) 合法紮寨，游標鍵為 `3,0`，方向 `3` 射向 (Q=3,R=4) 曹洪。正式戰術 API 已確認中間格為淺水、箭數為 4，兩版均可射擊。舊紮寨點的方向 `2` 中間格為城池，依法不能射箭；保留診斷失敗，修正測試路徑，不改正式規則或部隊座標。

退兵列表佔下方面板前三行，「退兵」標題位於第四行 y=316。築城確認位置後，正式流程另有軍師勸諫及「主公是否繼續」停點；工具依實際提示按 Y，並保存確認圖。窄版戰場的文字比較取攻方右側 (528,44) 與守方左側 (448,156) 的 96×96 區域，避開另有高清素材的肖像。初次驗證誤認退兵標題行、將高清肖像納入文字區，以及漏答築城勸諫，失敗資料分別保存於 `events-v8-before-retreat-sync/`、`events-v8-before-hud-region/`、`events-v8-before-fort-confirm/`，均在 `workplace/hd-window/player/` 下。修正的是驗證流程，正式 Go 程式未改。

加強版任命軍師的初次完整擷取已包含 1–23 步與終點，截圖卻被額外續頁鍵推到劉備對白。失敗收據及實圖保存於同層 `events-v8-before-wipe-continuation/`。擷取器改為首個有效拉幕幀後鎖住續頁，直到目標原生像素完成；動畫開始後不再按空白鍵。這是輸入同步修正，不改動畫幀數、速度或玩家流程。

兩版各八次正常片頭／新局，共十六次，200/200 檢查通過，完整批次返回 0。每張核對 704×384 原生高清、176×96 原貌來源、高清恢復與素材外文字框線。SCG04 只驗本郡登用成功，SCG05 只驗軍師任命，SCG13 只驗他國登用開場，SCG26 只驗主畫面撤職；同鍵其他使用端仍待抽驗。

| 場景 | 原版捕獲方向／非終點步數 | 加強版捕獲方向／非終點步數 |
|---|---|---|
| SCG04 | 3，向左／1–21 | 3，向左／1–21 |
| SCG05 | 1，向上／1–23 | 1，向上／1–23 |
| SCG07 | 2，向右／1–21 | 2，向右／1–21 |
| SCG12 | 1，向上／1–23 | 0，向下／1–23 |
| SCG13 | 1，向上／1–23 | 1，向上／1–23 |
| SCG17 | 2，向右／1–21 | 0，向下／1–23 |
| SCG21 | 1，向上／1–23 | 1，向上／1–23 |
| SCG26 | 3，向左／1–21 | 3，向左／1–21 |

合計 354 個實際 X11 中間幀，十六段均捕獲該方向全部非終點步數；步數遞增、揭露區全部像素與首幀後未覆蓋區符合獨立幾何。原版本批捕獲三方向，加強版捕獲四方向，未重擲或挑選方向；各圖其他方向由前述完整圖層另驗。首個捕獲幀之前的遮蔽區不在 GUI 比較範圍。

獨立回讀 290 個最新 PNG、354 份壓縮 RGB、七份工具、兩份控制器原文、執行檔、六份原始 GRP、包內 130 筆及八份生成紀錄，雜湊、尺寸與擁有權相符。另以獨立像素幾何重驗揭露區、未覆蓋區、原貌來源、文字框線及高清恢復，全部通過；v7 的 114 筆素材保留且欄位相同。原版與加強版都捕獲築城勸諫確認；加強版另捕獲正常太守補位與返回。收據的 `rested_prefectures` 記錄送出休息序列的嘗試，加強版潁川第一次被補位停點打斷，完成補位後再送休息，不將重複記錄視為兩次已執行命令。

| 本機 `workplace/` 產物 | SHA-256 |
|---|---|
| `hd-window/player/events-v8/receipt.json`，200/200 | `53e359ee089bbe07cd77d050fd9bb2acb0ac611b2756df79750038ecfb75d7f1` |
| `hd-window/player/events-v8/san1-window-check` | `aeba6c86449f2f38fb6f3a46400580edf4ed82ddb2ebb2379c725d11ca6f7b6b` |
| `hd-window/player/events-v8/verify-hd-events-inner.py` | `2fcb05d066a036b33dd46370b9d6e73d4df2d364e89b63c7169850e9ea8b86f0` |
| `hd-window/player/events-v8/verify-hd-events.sh` | `2351d2d94ee48882c4687bca8c6c3312f378b2ebef60943ecf9b03b83b95acb1` |
| `hd-window/player/events-v8/hd-events-reference.go` | `68e1e58a09b43bbf8469db4e449e0c6a9266965562baa910863f6413d636ec22` |
| `hd-window/player/events-v8/verify-hd-scenes-inner.py` | `b7dce8db9c193db0657b9b919fb90e4ce5f5ae57f2d1075de1b9385cc3b3d3c1` |
| `hd-window/player/events-v8/san1-main.go` | `5b8c585bc065ba67ad086b9ba18be37a34b17544f5b6b0b06f95afd3c92b2004` |
| `hd-window/player/events-v8/governor_test.go` | `a2915593a9e127ac22b49f2cacb399a73b7730e08bbd881b8b3e9d4fa8e27300` |
| `hd-window/player/governor-prompt-tests.log`，四項 | `ff131040b19bf87fd78266351fa448cc5b846c52f599bb194e52c039fd927af8` |
| `hd-window/player/governor-all-tests-summary.json`，653 通過／27 跳過 | `8b37f4a4de2e63a29216a15f7edbae51291c1a4c7701ed3c57dc4b6ed4078b79` |
| `hd-window/player/governor-all-tests.jsonl` | `9524b78108196d917dc82ab2037955546ef1dbe0d430bd6b03a390721ddfca3a` |
| `hd-v8-delivery-verification.json`，獨立回讀 | `7709795018886bf7899da6f539dabd836def5cd4fdbd3af2d363932303d9daeb` |
| `verify-hd-v8-delivery.py`，私人回讀器 | `4ba404866afd9da50b7be516315aa72135d9f94dbfa818bf4a5c603a513b4e4b` |

圖片、原版資料、生成紀錄、失敗資料與完整收據只留本機。規則、seed 與存檔未改；正式 UI 的補位返回修復依下一節。沒有新增原版 oracle 或配樂重跑聲明，使用者逐張簽核、其餘素材家族與使用端、動畫／遮罩、三語及跨平台仍待完成，021 維持 READY。

#### 6.19.1 太守補位後恢復下令提示

狀態：READY。證據審查沿用 [014 §2.1](014-art-main-overlays.md#21-主提示時上面板是十項指令表) 的主迴圈重印提示，以及同規格的 `0x1d638` → `0x1d6ed` 太守補位契約；原版證據維持該規格的 `[base]` L0／L1 範圍。

加強版正常築城路徑出現太守補位，選完候選後，`closeRoster` 清掉提示與輸入，既有 `askNewGovernor` 回呼沒有重建目前玩家郡的下令提示。修復限定在合法補位完成後：若沒有其他待補位郡，且 `Session.Waiting()` 仍有玩家郡，恢復該郡的面板與 `mainAsk` 數字輸入。仍有待補位郡時交回既有補位流程，沒有玩家停點時交回既有電腦流程。不得結束回合、推進年月或改變規則、亂數與存檔。

驗收須同時核對兩版的補位回呼、無玩家停點時不插入下令提示，以及加強版正常新局、補位、續行至洛陽與築城。控制器測試屬回歸，正常 GUI 收據另列，不升格為新的原版 oracle。

[控制器回歸](../../cmd/san1/governor_test.go) 的兩版有／無玩家停點共四項通過；玩家停點與年月保持，無玩家停點時不插入下令提示。加強版正常補位、續行至洛陽與築城窄驗 12/12 通過，收據保留於 `workplace/hd-window/player/events-v8-fort-plus-return-pass/`。

含原版素材的正式 Go 套件回歸為 653 項、17 個有測試套件通過；27 項跳過，完整列表在 `workplace/hd-window/player/governor-all-tests-summary.json`，不計入通過或原版 parity。`oracle` 標籤未啟用。初次 `go test ./...` 被私人 `workplace/` 的多個診斷 `main` 阻擋；重跑以 tmpfs 隔離該目錄，原版資料唯讀掛 `/orig`，快取與輸出另外掛載，Xvfb 有界且收尾關閉。失敗 log 保留為 `governor-all-tests-private-workplace-failure.log`，完整 JSON log 為 `governor-all-tests.jsonl`，兩者均在同一玩家驗證目錄。

### 6.20 其餘災害、謀略與單挑場景

本節狀態：READY。依 v8 全族稽核，本批為 DATA3 的 SCG02／03／08／10／11／14／16／18／19／22／23／25／27／28／29，兩版來源 bytes 均相同，尺寸皆為 176×96。十五個唯一採用鍵的來源、比例與 Codex 逐張審查已通過；素材包及正常玩家驗收另列。使用端與原版定位沿用 [010 §1.1](010-screen-transitions.md#11-呼叫端l0base)，不因素材存在推定新用途。

| 場景 | 已有用途 | 邏輯位置 |
|---|---|---|
| SCG02／10／14 | 水災、蝗害、瘟疫 | (432,80) |
| SCG16／25 | 元月老死／無繼承人、元月出頭 | (432,80) |
| SCG18／23 | 遠交近攻、驅虎吞狼 | (432,80) |
| SCG22 | 策反人民得手 | (432,120) |
| SCG03／19 | 水淹、火攻／燒糧 | (448,268) |
| SCG08 | 被擒囚禁 | (448,268) |
| SCG27／28／29 | 單挑戰死、被擒、叫陣／平手 | (448,268) |
| SCG11 | 010 的原版場景呼叫表未使用 | 無新使用端 |

沿用 B 寫實手繪、4×、原貌預設。每鍵獨立參考原圖與已採用 B 場景，保留構圖、姿態、服色、主體數量與景物；完整圖縮放至 704×384，不裁切、不烘焙文字。比例或構圖不符時保存退件，回到該張修正。來源、提示詞、候選、版次、工具未回報的模型／seed、Codex 審查與使用者逐張簽核分開記錄，原版及衍生圖片留本機。

候選逐張審查、來源及比例通過後，本節才轉 READY 並準備新版私人包。必須保留 v8 已有 130 筆素材與欄位，兩版正式載入器及全族稽核須通過。完整圖層四方向各步另驗；正常 GUI 從片頭與新局、原有合法命令或自然事件進入，不注入人物、事件、日期或 seed。SCG11 只計素材準備，不宣稱正常使用或新增原版行為。尚未跑到的使用端、單挑分支及自然事件維持未驗證。

十七份候選採用十五張。SCG22／27 第一版比例不符，保留退件並由 `image_gen` 重新構圖，採用第二版；其餘採第一版。SCG16／27 採用圖為 1699×926，其餘 1698×926，符合既有一個來源像素的比例誤差契約。來源、提示詞、參考、候選、採用版次與 Codex 審查在本機 `workplace/hd-scenes-v3-generation.json`，SHA-256 `a5095d55359a9e405709fcae9aec2c80e4bd87891becb1f6d38b080a5e16015b`；模型／seed 未回報，使用者逐張簽核為 0。來源比較頁為 `workplace/hd-preview/scenes-v9-source-contact.png`，SHA-256 `d4538b160db841f196a702c64bfd2b1beb2f726d8320cc776ea28a6e731fb347`。

正常謀略驗證沿既有 014 的命令流程：兩版各從片頭開 001 孫堅新局、難度 5，在長沙任命清單第一位程普為軍師，清完對白並核對主提示、孫堅肖像與月份變化後下令。驅虎吞狼選出使郡 2、攻打郡 3；遠交近攻選出使郡 27、攻打郡 29、我方郡 31；策反人民選目標郡 2。使者均選正式魅力排序清單第一位孫堅。診斷只讀正式劇本與合法命令，確認選單順序、條件與策反得手，不代替正常 GUI。

三張各跑兩版新局，按正式選單輸入；軍師勸諫後以正式「主公是否繼續呢(Y/N)」畫面核對，再送 Y。核對 704×384 原生圖、實際中間幀的完整揭露區、未覆蓋區、原貌切回、高清恢復及場景外文字框線。SCG22 使用原版 (432,120)，文字比較自 y=216 起，避開仍屬圖內的 y=200–215。成功聲明限實際捕獲的場景與方向，不外推後續戰役、自然災害、單挑或原版規則 parity。

私人 v9 包為 `workplace/hd-assets-scenes-v9/`，兩版各 80 筆，共 160 筆；49/256 肖像與 31/31 SCG，正式載入器警告為 0。v8 已有 130 筆的全部欄位與 PNG bytes 保持。兩族全槽稽核的技術問題均為 0，場景缺圖為 0、Codex 審查 31、使用者逐張簽核 0；肖像仍缺 207。場景 `--require-complete` 返回 0，只代表完整準備與 Codex 審查，不包含使用者簽核或全部正常使用端；肖像同閘門仍返回 3。

實際場景的四方向完整圖層兩版各 2,852 幀，共 5,704/5,704 通過；每張 24／24／22／22 步，整幀、塊外與原貌 CPU 畫布差異為 0。沿用 [完整圖層工具](../../tools/hd-wipe-check.go)，不以此代替正常 GUI。包的精確重建命令、採用版次、來源 GRP 雜湊與 Go 1.24.13 工具鏈在本機建置收據。

| 本機產物 | SHA-256 |
|---|---|
| `workplace/hd-scenes-v9-build-receipt.json` | `2ee41eb0fca927a28fda7037cba79feedc81e4a24e6c52d0d68a9f580d7864cd` |
| `workplace/hd-assets-scenes-v9/manifest.json` | `e30b356a40226f3466d1def6e6a0eed752b262742cdcfca0ae2b3380f910d7f5` |
| `workplace/hd-assets-scenes-v9/preparation.json` | `3cd3d3b8a587c58129323fba8fc1f00ab4d399b756df0a22adfadb3b79cbe8b1` |
| `workplace/hd-scene-audit-v9.json` | `3386d3a026ce3ea9602f6d7b00df97bfbf0495f2a26b9b0bf00c435729e7400f` |
| `workplace/hd-portrait-audit-v9.json` | `df8fc87a46b6eff1edfb54e664e5972e2f1ce939312bee5cee87b6782bd59b85` |
| `workplace/hd-wipes-v9-base.json` | `6682d1f2bc66e6936103a361e591e225520725de6c2ca178363a71aad023811b` |
| `workplace/hd-wipes-v9-plus.json` | `c8cb531ee399f179b2f2ae56adf31010aeb3989ef59285de3972be95e4cb8435` |
| `workplace/hd-preview/scenes-v9-contact.png` | `5a3d3f283ccdc9be637e4b9b2ddd861184532ff847def3544be7d86e04c60e12` |

正常謀略重跑入口為 `bash tools/verify-hd-plots.sh`，只控制既有 Docker 工具鏈。前置資料、快取與私人 v9 包必須存在；建置用 `rich2-go-ebiten:latest`，GUI 用 `eob-audio-capture:20260922-r2`。三張與兩版選擇可用 `--only SCG18|SCG23|SCG22`、`--edition base|plus`。 [合法輸入與提示](../../tools/hd-plots-reference.go) 只提供同步參考，不代替正常操作或原版 oracle。收據及實際中間幀在本機 `workplace/hd-window/player/plots-v9/`。

兩版各三次正常片頭／孫堅新局，共 78/78 檢查通過，完整批次返回 0；三張合計核對六次正式軍師勸諫確認。任命前後主提示、原貌 F041 與月份變化均由實際截圖核對。實際 X11 中間幀共 134，六段完整捕獲該方向全部非終點步數，沒有缺步；每版 SCG18 為方向 2 的 1–21，SCG23／22 為方向 1 的 1–23。其餘方向由上述完整圖層另驗，不將它們稱為已由本批正常 GUI 跑過。

獨立回讀 114 張最新 PNG、134 份壓縮 RGB、九份工具、正式控制器原文、執行檔、六份原始 GRP、160 筆包內素材、十七份候選與兩族稽核。來源、原生像素、場景外文字框線、月份推進、尺寸、雜湊及擁有權相符；v8 舊素材 bytes 保持。首次策反腳本漏處理勸諫確認，失敗收據與舊工具保留於本機；修正後的原版策反窄驗證 13/13 通過，再乾淨重跑完整兩版。正式 Go 程式、規則、seed 與存檔未改，未重跑或擴大原版 oracle、既有回歸與音訊聲明。

| 正常玩家驗證產物 | SHA-256 |
|---|---|
| `workplace/hd-window/player/plots-v9/receipt.json` | `e562bd0aa14113c0abc95ab73a355e6270a9ef5997442154255d6aa2d3d06cfc` |
| `workplace/hd-v9-delivery-verification.json` | `e3e054426f81597de7ea1cf593da20fe9634830278dc60cc1996517b1594e480` |
| `workplace/verify-hd-v9-delivery.py` | `d93679e1d87c8f684f7184c77feb953a7b48876c649d2b7bd5b5eef94781314d` |
| `workplace/hd-preview/plots-v9-contact.png` | `ccf7153aa33ebfa226fb861226e98966da40c627359ae998585ed61241e3b420` |

兩版正常謀略與全批來源／高清比較頁已目視。SCG 素材準備完成不等於全部使用端驗收：自然事件、戰場謀略、被擒處置及單挑分支仍待驗，SCG11 不新增使用端。其餘素材家族、207 肖像、使用者逐張簽核、三語系與跨平台未完成，整份 021 保持 READY。原圖、AI 圖、候選、完整收據及包只留本機，不加入公開 Git 或 Release。

### 6.21 二十張魏、袁、董、孫勢力肖像

本節狀態：READY。二十個唯一採用鍵的來源、比例與 Codex 逐張審查已通過；素材包與正常玩家驗收另列。沿用 §6.15 的固定肖像與 B 寫實手繪契約。二十個 DATA3 肖像槽皆為 64×80，兩版來源 bytes 相同。姓名、人物索引與查看清單位置由兩版正式 `session` 新局第一次玩家停點讀出，不改寫人物、日期、勢力、亂數或存檔。

| 肖像槽 | 人物 | 人物索引 | 查看郡 | 清單選項 |
|---|---|---|---|---|
| F250 | 曹仁 | 343 | 11 | 4 |
| F018 | 曹洪 | 344 | 11 | 5 |
| F125 | 樂進 | 27 | 11 | 6 |
| F190 | 曹純 | 112 | 11 | 7 |
| F112 | 陳宮 | 26 | 11 | 8 |
| F131 | 張邈 | 31 | 11 | 9 |
| F249 | 田豐 | 51 | 3 | 2 |
| F047 | 顏良 | 41 | 3 | 6 |
| F102 | 文醜 | 42 | 3 | 5 |
| F168 | 許攸 | 53 | 3 | 10 |
| F129 | 郭圖 | 116 | 3 | 7 |
| F178 | 審配 | 89 | 4 | 3 |
| F113 | 沮授 | 52 | 4 | 4 |
| F233 | 張郃 | 119 | 4 | 2 |
| F179 | 高覽 | 118 | 4 | 10 |
| F076 | 李儒 | 21 | 14 | 2 |
| F160 | 賈詡 | 63 | 15 | 1 |
| F052 | 華雄 | 36 | 15 | 3 |
| F241 | 程普 | 37 | 31 | 2 |
| F055 | 黃蓋 | 39 | 31 | 3 |

正常驗證統一從片頭開劇本 001、單人曹操、難度 5，189 年元月停在郡 11。前六位查看本國，其餘使用正式他國查看確認。逐張在原貌人物卡核對原版肖像，切 B 高清核對 256×320 原生圖及圖外文字框線，再切回原貌核對還原。人物卡落點為 (536,68)，沿用既有布局，不調整字區。

每張獨立以 `image_gen` 參考該原圖與已採用的 B 筆觸；後者只作風格參考。保留原圖冠帽、服色、鬚髮、朝向、可見眼睛與剪影，不以通俗三國形象或劇本數字年齡補造外觀。完整 4:5 圖縮放至 256×320，不裁切、不拉伸、不添加文字。候選有誤時保留退件並另生版次。逐張 Codex 審查與使用者逐張簽核分開記錄。

來源與正式清單的四十列計畫在本機 `workplace/hd-commanders-v10-plan.json`，SHA-256 `0939c7ca7bc4d7c78b303f1677ebe161b090765f9fe315e3294c4144dea21e87`。來源比較頁為 `workplace/hd-preview/commanders-v10-sources.png`，SHA-256 `76f6da73fcdb227619158f23fb4e541c1d1f8cc10dff1302c843091dc7509a9e`。採用前須核對來源雜湊、比例與外觀，再轉 READY、重建私人包、跑兩版正式載入器及全族稽核。新版須保留 v9 全部 160 筆欄位及 PNG bytes；場景族不變。

本節只擴充已定案的美術，規則與存檔不變。正常人物卡驗證不宣稱原版 oracle parity，也不代替三語系、其他肖像使用端、跨平台或人耳音訊驗收。原圖、候選、生成紀錄與私人包留本機。

二十九份候選採用二十張，採用圖皆為 1122×1402，符合既有一個來源像素的比例誤差契約。F190／F112／F131／F179／F052／F241 第一版比例不符，採第二版。F076 第一版高冠偏離低帽，採第二版。F047 第一版多露出遠眼並添額前紅寶石，第二版修正外觀但比例不符，採第三版。全部退件保留，不以裁切或拉伸通過。來源、完整提示詞、參考圖、生成路徑、候選版次與 Codex 審查在本機 `workplace/hd-commanders-v10-generation.json`，SHA-256 `7fb613cddac23e37f16ca386c8dd92f55cfaef509a28b3aa7a6b1fdf601f9b80`；工具未回報模型／seed，使用者逐張簽核為 0。

正常人物卡重跑入口為 `bash tools/verify-hd-player.sh --commanders-next`。原版資料、快取、私人 `workplace/hd-assets-portraits-v10/` 及兩版來源肖像 PNG 必須存在；建置與 GUI 沿用既有 Docker 工具鏈。新增原貌來源像素檢查，兩版各一次劇本 001 新局；收據與工具快照保存於本機 `workplace/hd-window/player/commanders-v10/`。

私人 v10 包為 `workplace/hd-assets-portraits-v10/`，兩版各 100 筆，共 200 筆；69/256 肖像及 31/31 SCG。兩版正式載入器警告為 0，v9 全部 160 筆欄位及 PNG bytes 保持。兩族稽核的技術問題均為 0，肖像缺 187、Codex 審查 69，場景缺 0、Codex 審查 31；使用者逐張簽核均為 0。場景完整準備閘門返回 0，肖像仍返回 3。場景圖未改，§6.20 的四方向與正常謀略收據保持原範圍。

| 本機產物 | SHA-256 |
|---|---|
| `workplace/hd-commanders-v10-build-receipt.json` | `e9269f2aced15903d77c342623eeeef16095d548e551f08a6b2eea15932ba70e` |
| `workplace/hd-assets-portraits-v10/manifest.json` | `17010a4fe8cbc621cf13ef2c68413df26b08ebc42b31874c0e48d7edeb51e1fa` |
| `workplace/hd-assets-portraits-v10/preparation.json` | `f7c20ecca0b5c2b292866f52bab54f7ebe40ea923e4cfb36d60b5a346bec19bb` |
| `workplace/hd-portrait-audit-v10.json` | `666dd5ff4840678802751aedfd7b4e5c227bb7fc8d3b78cc9d082fcd6f2f4875` |
| `workplace/hd-scene-audit-v10.json` | `fe5a0b5f5c2accaf272bcdb1f92896b5b9311d17e95077f4fb2e4d321d94684b` |
| `workplace/hd-preview/commanders-v10-contact.png` | `096e862208d17e0443dc80f8d3b1cf0e4ed409abc7cd3ff20a30f8862abde776` |

兩版正常人物卡完整批次返回 0，162/162 檢查通過。兩次正常片頭／001 曹操新局，四十張卡逐張核對原貌來源、256×320 原生高清像素、肖像外文字框線與原貌恢復；未注入人物、日期、勢力或 seed。原貌、高清及切回的圖外區域皆相符，姓名與冠帽等識別依來源及正式人物卡目視確認。

獨立回讀 304 張最新 PNG、三份工具快照、執行檔、六份原始 GRP、200 筆包內素材及二十九份候選，雜湊、尺寸、像素與擁有權相符。v9 全部 160 筆欄位、PNG bytes 與準備紀錄保持。四十張卡的來源、原生高清、圖外文字框線與原貌恢復再以獨立 RGB 解碼核對；兩版完整人物卡比較頁已目視。獨立工具為本機 `workplace/verify-hd-v10-delivery.py`，在既有 `eob-audio-capture:20260922-r2` 容器以唯讀原始資料執行，不再啟動遊戲。

| 本機產物 | SHA-256 |
|---|---|
| `workplace/hd-window/player/commanders-v10/receipt.json` | `9d96e37518581d8ef33642d98515562afe5f5c7f7f2fb6a34e9b6365c14d9ced` |
| `workplace/hd-v10-delivery-verification.json` | `879e1b3e808b614212eaff3ff5630321f448e7f222d082dfb03efbeb65bc45dc` |
| `workplace/verify-hd-v10-delivery.py` | `8e2e380f6f75cdff104aba9dce2d82dd807a223723961afc5fdf517b2c7d06f3` |
| `workplace/hd-preview/commanders-v10-gui-base-contact.png` | `3f38eb9129437b6f500d1f1b6a9e5bbd5a948e6c5bee002fadfbbab634246a00` |
| `workplace/hd-preview/commanders-v10-gui-plus-contact.png` | `2a518f684281c0983333b6c384d1418eac5b46343e65a85bc3f3b146b9049d08` |

### 6.22 二十張徐州、江東、荊州與益州肖像

本節狀態：READY。沿用 §6.15／6.21 的固定肖像、B 寫實手繪與原貌預設契約。來源、比例與二十張採用圖的 Codex 逐張審查已通過，私人包與正常 GUI 驗收另列。二十個 DATA3 肖像槽皆為 64×80，兩版來源 bytes 與 PNG 相同。來源槽、尺寸及跨版原始 bytes 比較為 L0、[both]；B 圖與正常 GUI 收據屬 remake 美術差異及使用端驗證。

| 肖像槽 | 人物 | 人物索引 | 查看郡 | 清單選項 |
|---|---|---|---|---|
| F186 | 孫乾 | 87 | 9 | 2 |
| F181 | 陳登 | 76 | 10 | 2 |
| F103 | 麋芳 | 120 | 10 | 3 |
| F173 | 麋竺 | 75 | 10 | 4 |
| F021 | 太史慈 | 77 | 21 | 2 |
| F226 | 陳武 | 101 | 21 | 3 |
| F213 | 虞翻 | 104 | 23 | 2 |
| F094 | 韓當 | 38 | 31 | 4 |
| F189 | 朱治 | 94 | 31 | 5 |
| F244 | 祖茂 | 40 | 31 | 6 |
| F100 | 丁奉 | 155 | 31 | 7 |
| F246 | 蔡瑁 | 47 | 28 | 2 |
| F092 | 蒯越 | 46 | 28 | 3 |
| F130 | 伊籍 | 137 | 28 | 4 |
| F169 | 黃祖 | 59 | 29 | 1 |
| F091 | 蒯良 | 45 | 30 | 1 |
| F167 | 文聘 | 143 | 30 | 2 |
| F187 | 吳懿 | 210 | 36 | 4 |
| F218 | 張任 | 203 | 37 | 2 |
| F088 | 黃權 | 198 | 37 | 5 |

姓名、人物索引與選項由兩版正式 `session` 新局讀出。驗證從正常片頭開劇本 001、單人曹操、難度 5，189 年元月停在郡 11；二十位皆走正式他國查看確認。人物卡位置為 (536,68)，高清為 256×320；逐張核對原貌來源、原生高清、肖像外文字框線及原貌恢復，不注入人物、日期、勢力或 seed。

來源與正式清單的四十列計畫為本機 `workplace/hd-commanders-v11-plan.json`，SHA-256 `833da6a90273fc94722853c691301ac5cba0adfbb19a01c3830d59a0fdaa2f11`。二十張來源比較頁為 `workplace/hd-preview/commanders-v11-sources.png`，SHA-256 `647003d04b3fb06341720382f1a09547106fc97c3779738b2bf5733a57ad7637`。

每張以 `image_gen` 獨立生成，原圖是身份、冠帽、服色、鬚髮、姿態與可見眼睛的依據，已採用 B 圖只作筆觸與畫布參考。完整 4:5 圖縮放至 256×320，不裁切、不拉伸、不烘焙文字，不依劇本年齡或通俗人物形象改造。退件與版次保留；工具未回報模型／seed 時據實標示。逐張 Codex 審查與使用者逐張簽核分開記錄。

二十三份候選採用二十張，全部為不透明 1122×1402，比例符合既有契約。F189／F088 採第二版，分別修掉原圖沒有的下巴鬍鬚及遠側眼；其餘採第一版。F094 原圖放大後確認眼線左高右低，第一版符合來源，先前相反判讀及第二版提示保留於生成紀錄，不據錯誤判讀改正式圖。三份退件保留，使用者逐張簽核為 0。完整紀錄在本機 `workplace/hd-commanders-v11-generation.json`。

私人 v11 包須跑兩版正式載入器及全族稽核，v10 全部 200 筆欄位、PNG bytes 與準備紀錄須保持。正常人物卡入口為 [`tools/verify-hd-player.sh`](../../tools/verify-hd-player.sh)，使用 `bash tools/verify-hd-player.sh --officers`，另以 GUI 與獨立回讀驗證，不替代其他使用端、三語系、音訊、跨平台或原版 oracle。來源、候選、完整生成紀錄與包只留本機；規則與存檔不變。

本機包為 `workplace/hd-assets-portraits-v11/`，89/256 肖像與 31/31 SCG，兩版各 120 筆、共 240 筆，正式載入器警告 0。v10 全部 200 筆欄位、PNG bytes 與準備紀錄由獨立回讀核對保持。兩族技術問題均為 0；肖像 Codex 審查 89、缺 167，場景 Codex 審查 31、缺 0。使用者逐張簽核均為 0。場景完整準備閘門返回 0，肖像返回 3。

| 本機產物 | SHA-256 |
|---|---|
| `workplace/hd-commanders-v11-generation.json` | `bf0d06b704866e39aae89556a201a187765958eca16096130c45574f21d71b4c` |
| `workplace/hd-commanders-v11-build-receipt.json` | `7133de56e155bb6b23979f536a2c747a120213e8a6de40efbe36993a25f7cbf8` |
| `workplace/hd-assets-portraits-v11/manifest.json` | `408d24875a49ad268af268509fb7cb73c2a6dae09b401bc3ab4a49e20843bf19` |
| `workplace/hd-assets-portraits-v11/preparation.json` | `76b24af73e79afaaed432c0d2014bebba2651d63f579284fe9f7c5ce96d63833` |
| `workplace/hd-portrait-audit-v11.json` | `257639adde340ebecd36ed9f96c367014635dfd16b8ea1253da432e8affe48bd` |
| `workplace/hd-scene-audit-v11.json` | `9fe4800d32662beaf8533f00c38d505228bb11700588c0635ae1a9ea2a93df07` |
| `workplace/hd-preview/commanders-v11-contact.png` | `c1af0ef555fc90e6632c6f513b5536584fc00d41c7ef2c346ecd65145b4d574f` |

兩版正常人物卡完整批次返回 0，162/162 檢查通過。兩次正常片頭／001 曹操新局、四十張卡，全數經原有他國查看確認，逐張核對原貌來源、256×320 原生高清像素、肖像外文字框線及原貌恢復；未注入人物、日期、勢力或 seed。二十張正式高清圖、來源比較頁與兩版全部人物卡比較頁已目視。

獨立回讀 298 張最新 PNG、三份工具快照、執行檔、六份原始 GRP、240 筆素材及二十三份候選，雜湊、尺寸、像素與擁有權相符。v10 全部 200 筆欄位、PNG bytes 與準備紀錄保持；四十張卡的來源、原生高清、圖外文字框線及原貌恢復再以獨立 RGB 解碼核對。本機 `workplace/verify-hd-v11-delivery.py` 在既有 `eob-audio-capture:20260922-r2` 容器以唯讀輸入執行，不再啟動遊戲。

| 本機產物 | SHA-256 |
|---|---|
| `workplace/hd-window/player/commanders-v11/receipt.json` | `2d4c3424be008976d90fe5f39909cffd4e5a764004dddd90226952a48a477e82` |
| `workplace/hd-v11-delivery-verification.json` | `58b18abec5aea5858ad57d652572d3474e38862d30247a4992cb5d72e5675eab` |
| `workplace/verify-hd-v11-delivery.py` | `0c58ccfdd698931c5eac23e54fc62b2168a03ed4eb5154e46ed06f40e2c9d808` |
| `workplace/hd-preview/commanders-v11-gui-base-contact.png` | `e73fc9e5af040f563c8ad4102a39c80da927e027b06043a8a3d4a459b15d7f68` |
| `workplace/hd-preview/commanders-v11-gui-plus-contact.png` | `0414cb9116d4bb3a0b8b5056575a38735cda4bf80181106c0870e809590b39fb` |
