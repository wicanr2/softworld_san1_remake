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

### 6.23 劇本 001 其餘四十五張肖像

本節狀態：READY。沿用 §6.15／6.22 的固定肖像、B 寫實手繪、4× 與原貌預設契約。這批選取劇本 001 正常開局可查看、尚未製作的四十五個 DATA3 槽。兩版 64×80 來源 PNG 及原始槽 bytes 已逐項核對相同，原始資料為 L0、[both]。查看選項來自 remake 正式新局清單，不當作原版 oracle。來源、比例與四十五張採用圖的 Codex 審查已通過，私人包與正常 GUI 驗收另列。完整九十列在本機 `workplace/hd-commanders-v12-plan.json`，SHA-256 `a7d006509a77b371bfb4ea917d871af7e27fc403e617d904c054796dab2005c1`。

| 肖像槽 | 人物 | 人物索引 | 查看郡 | 清單選項 |
|---|---|---|---|---|
| F073 | 嚴綱 | 298 | 2 | 2 |
| F199 | 公孫越 | 54 | 2 | 3 |
| F022 | 袁尚 | 131 | 3 | 3 |
| F126 | 袁譚 | 129 | 3 | 4 |
| F050 | 淳于瓊 | 290 | 3 | 9 |
| F080 | 辛評 | 50 | 3 | 11 |
| F089 | 袁熙 | 130 | 4 | 1 |
| F071 | 張顗 | 320 | 4 | 5 |
| F175 | 麴義 | 297 | 4 | 7 |
| F045 | 馬延 | 319 | 4 | 8 |
| F137 | 高幹 | 317 | 4 | 9 |
| F148 | 陳琳 | 117 | 4 | 11 |
| F134 | 董旻 | 25 | 6 | 1 |
| F096 | 郭汜 | 18 | 6 | 2 |
| F032 | 李肅 | 24 | 6 | 4 |
| F231 | 楊彪 | 291 | 6 | 5 |
| F170 | 董璜 | 62 | 6 | 6 |
| F211 | 鮑信 | 292 | 7 | 3 |
| F066 | 曹豹 | 300 | 9 | 4 |
| F205 | 袁胤 | 342 | 13 | 1 |
| F188 | 雷薄 | 107 | 13 | 2 |
| F185 | 陳蘭 | 108 | 13 | 3 |
| F116 | 徐榮 | 43 | 15 | 4 |
| F065 | 王允 | 345 | 15 | 5 |
| F093 | 胡軫 | 294 | 15 | 6 |
| F147 | 張繡 | 111 | 16 | 1 |
| F223 | 趙岑 | 295 | 16 | 4 |
| F191 | 程銀 | 186 | 19 | 4 |
| F151 | 楊秋 | 192 | 19 | 5 |
| F118 | 華歆 | 125 | 24 | 1 |
| F165 | 紀靈 | 93 | 27 | 2 |
| F067 | 李豐 | 308 | 27 | 3 |
| F240 | 呂公 | 61 | 29 | 2 |
| F243 | 陳生 | 60 | 29 | 3 |
| F105 | 張允 | 335 | 29 | 4 |
| F196 | 劉磐 | 175 | 30 | 3 |
| F036 | 孟達 | 11 | 36 | 2 |
| F225 | 王累 | 200 | 36 | 6 |
| F079 | 吳蘭 | 211 | 37 | 1 |
| F027 | 冷苞 | 202 | 37 | 3 |
| F204 | 雷同 | 212 | 37 | 4 |
| F183 | 譙周 | 222 | 38 | 1 |
| F090 | 楊懷 | 205 | 38 | 3 |
| F161 | 高沛 | 206 | 38 | 4 |
| F133 | 張肅 | 208 | 38 | 5 |

生成時逐張沿用原圖身份、冠帽、服色、鬚髮、姿態與可見眼睛；不依劇本年齡或通俗人物形象改造。完整 4:5 圖縮放至 256×320，不裁切、不拉伸、不烘焙文字。來源、提示詞、候選與退件保留本機，Codex 審查與使用者逐張簽核分列。四十五張全部通過來源與比例審查後，本節由 DRAFT 轉 READY，再建立私人 v12 包。

四十七份候選採用四十五張，全部不透明 1122×1402，比例符合既有契約。F151／F036 採第二版，分別修掉額外遠側眼與恢復原有微張嘴表情，其餘採第一版。兩份退件保留，使用者逐張簽核 0。工具為 image_gen，模型／seed 未回報。完整紀錄為本機 `workplace/hd-commanders-v12-generation.json`，SHA-256 `c6f4b0380842258114aff0cc4473447c42d97966026805d56dd3c3f6fd98b311`。

私人包保留 v11 全部 240 筆欄位、PNG bytes 與準備紀錄。兩版正式載入器及全族稽核另列。正常玩家驗證從片頭開劇本 001、單人曹操、難度 5，189 年元月停在郡 11，四十五位皆走原有他國查看確認。人物卡位置為 (536,68)，高清為 256×320；逐張核對原貌來源、原生高清、肖像外文字框線及原貌恢復。不注入人物、日期、勢力或 seed。入口為 [`tools/verify-hd-player.sh`](../../tools/verify-hd-player.sh) 加 `--officers-all`，兩版共九十張人物卡。

此批屬 remake 美術差異及使用端驗證，不替代原版 oracle、其他使用端、三語系、音訊或跨平台驗收。規則與存檔維持既有契約。


本機包為 `workplace/hd-assets-portraits-v12/`，134/256 肖像與 31/31 SCG，兩版各 165 筆、共 330 筆，正式載入器警告 0。v11 全部 240 筆欄位與 PNG bytes 保持；準備紀錄亦經獨立回讀確認保持。兩族技術問題均為 0；肖像 Codex 審查 134、缺 122，場景審查 31、缺 0。使用者逐張簽核均為 0。場景完整準備閘門返回 0，肖像返回 3。

| 本機產物 | SHA-256 |
|---|---|
| `workplace/hd-commanders-v12-generation.json` | `c6f4b0380842258114aff0cc4473447c42d97966026805d56dd3c3f6fd98b311` |
| `workplace/hd-commanders-v12-build-receipt.json` | `60bf1c94e19cad42e8cc6892fcc533a8fdee94d778347734417b1018fe1ede48` |
| `workplace/hd-assets-portraits-v12/manifest.json` | `581f2bd5beb3f6be421e3a346cdfac7361a6a9cfd47068faacd3baedf5b62bea` |
| `workplace/hd-assets-portraits-v12/preparation.json` | `b5d157775aec1cc1e2b1a6b9a9272788a41ab9c9e277582699ea289b98c398b5` |
| `workplace/hd-portrait-audit-v12.json` | `8d510e80322ce3410b046e7d001bc28a734edc5436b7c243c0d6456461ab67ba` |
| `workplace/hd-scene-audit-v12.json` | `0e39c5cbbebefccaf79a14d06570fafb9b7d64ad03c027c5236d389bd104f027` |
| `workplace/hd-preview/commanders-v12-contact-1.png` | `08de2f51f7cc395e4691ce4e6d5f5c9ba1a4770f610ac0a4ac304abb4efa4f18` |
| `workplace/hd-preview/commanders-v12-contact-2.png` | `830e543c5205ee62e3aa9067a83968feccb3bd12357b34091b3a99aabf74ac28` |
| `workplace/hd-preview/commanders-v12-contact-3.png` | `f8dbfc4bb4fec618a56fca1b4c7b87ba31c9038101cba1acda9ae0f3a8a59f1e` |
兩版正常人物卡 362/362 通過，完整批次返回 0。兩次正常片頭與 001 曹操新局，各四十五張卡，全部走他國查看確認；原貌來源、原生高清、肖像外文字框線及原貌恢復均相符。每次新局仍以原貌及隱藏選項列啟動。

獨立回讀 692 張最新 PNG、三份工具快照、執行檔、六份原始 GRP、330 筆素材及四十七份候選，雜湊、尺寸、像素與擁有權相符。v11 全部 240 筆欄位、PNG bytes 與準備紀錄保持，九十張卡的四類像素比較由獨立 RGB 解碼再驗。三張來源／高清比較頁與兩版各三張正式人物卡頁已目視。完整收據只留本機。

| 本機驗證產物 | SHA-256 |
|---|---|
| `workplace/hd-commanders-v12-plan.json` | `a7d006509a77b371bfb4ea917d871af7e27fc403e617d904c054796dab2005c1` |
| `workplace/hd-window/player/commanders-v12/receipt.json` | `fc906cdfc20aa75603f5ec242e1609f3f8b83dee999ac712e29d1777bd0e3338` |
| `workplace/hd-v12-delivery-verification.json` | `9b9ee1a112596345d854e39f32a11ff305918b6bc25d590e381e81da3254b62e` |
| `workplace/hd-v12-audit-commands.json` | `6f9573f1dc8a8fa9c2f345895f91e1d37a07909a5404d887667edbfc58a0ec1d` |
| `workplace/hd-preview/commanders-v12-gui-base-contact-1.png` | `46a401bd8c72d3012d02f3ef2a0c9555b3dc679320920c53499c0779019b5599` |
| `workplace/hd-preview/commanders-v12-gui-base-contact-2.png` | `4856b8ca108d45004ac46f2af954cc29039badf31e26fdefeb382822288ea766` |
| `workplace/hd-preview/commanders-v12-gui-base-contact-3.png` | `116e62305bee29983d3b691ad39be04dbf0f0a8ee0504aa133ec6ee265a09fd6` |
| `workplace/hd-preview/commanders-v12-gui-plus-contact-1.png` | `b7e3eee5bf639aad185be10580ec6e7a0253cdb0b9979b4c78d3d7c5f2516d1e` |
| `workplace/hd-preview/commanders-v12-gui-plus-contact-2.png` | `aa77f0a6cd2ff433c539088ae64506af5c98efd669f815e574b9a9f976ac370a` |
| `workplace/hd-preview/commanders-v12-gui-plus-contact-3.png` | `3df89086f296378225acc4c4db6fbab93efa0bacea41d4955b7de6908efa7491` |

### 6.24 劇本 002 新增四十六張肖像

本節狀態：READY。沿用 §6.15／6.23 的固定肖像、B 寫實手繪、4× 與原貌預設契約。這批選取劇本 002 初次玩家停點可查看、尚未製作的四十六個 DATA3 槽。兩版 64×80 來源 PNG 及原始槽 bytes 逐項相同，原始資料為 L0、[both]。查看清單由 remake 正式劇本 002、單人曹操、難度 5 新局產生，195 年元月停在郡 16；不當作原版 oracle。完整九十二列保存本機 `workplace/hd-commanders-v13-plan.json`，SHA-256 `4002d2ab4e7f1a65f917ac45202743df81626bfc5bec1d5b77be8d9eab051abe`。

| 肖像槽 | 人物 | 人物索引 | 查看郡 | 清單選項 | 他國確認 |
|---|---|---|---|---|---|
| F224 | 王修 | 318 | 3 | 10 | 是 |
| F060 | 張南 | 323 | 3 | 11 | 是 |
| F033 | 梁剛 | 307 | 4 | 9 | 是 |
| F029 | 徐晃 | 91 | 5 | 2 | 是 |
| F176 | 程昱 | 67 | 6 | 1 | 否 |
| F049 | 于禁 | 72 | 6 | 3 | 否 |
| F162 | 滿寵 | 70 | 6 | 5 | 否 |
| F177 | 鮑龍 | 172 | 7 | 3 | 是 |
| F222 | 傅幹 | 336 | 8 | 5 | 是 |
| F154 | 簡雍 | 113 | 9 | 3 | 是 |
| F101 | 張遼 | 79 | 11 | 3 | 是 |
| F085 | 侯成 | 86 | 11 | 4 | 是 |
| F099 | 宋憲 | 85 | 11 | 6 | 是 |
| F238 | 郝萌 | 81 | 11 | 8 | 是 |
| F206 | 許氾 | 312 | 11 | 9 | 是 |
| F171 | 宋謙 | 177 | 12 | 1 | 是 |
| F069 | 橋蕤 | 109 | 13 | 3 | 是 |
| F120 | 徐庶 | 146 | 13 | 6 | 是 |
| F201 | 鍾繇 | 180 | 14 | 1 | 否 |
| F172 | 荀彧 | 65 | 15 | 2 | 否 |
| F143 | 郭嘉 | 68 | 15 | 3 | 否 |
| F180 | 劉曄 | 69 | 15 | 5 | 否 |
| F248 | 李典 | 28 | 15 | 6 | 否 |
| F215 | 荀攸 | 66 | 15 | 7 | 否 |
| F202 | 陳矯 | 164 | 16 | 3 | 否 |
| F104 | 閻圃 | 230 | 18 | 3 | 是 |
| F145 | 楊松 | 221 | 18 | 5 | 是 |
| F030 | 馬岱 | 9 | 19 | 2 | 是 |
| F255 | 馬超 | 4 | 19 | 3 | 是 |
| F150 | 馬玩 | 191 | 19 | 8 | 是 |
| F194 | 馬鐵 | 181 | 20 | 4 | 是 |
| F195 | 馬休 | 182 | 20 | 5 | 是 |
| F197 | 梁興 | 189 | 20 | 6 | 是 |
| F031 | 周泰 | 100 | 22 | 6 | 是 |
| F157 | 闞澤 | 144 | 24 | 1 | 是 |
| F028 | 呂蒙 | 151 | 24 | 4 | 是 |
| F043 | 賈華 | 178 | 24 | 5 | 是 |
| F174 | 潘璋 | 154 | 24 | 6 | 是 |
| F153 | 顧雍 | 128 | 24 | 7 | 是 |
| F061 | 魏延 | 8 | 27 | 6 | 是 |
| F074 | 王粲 | 159 | 29 | 4 | 是 |
| F251 | 甘寧 | 156 | 30 | 4 | 是 |
| F051 | 王威 | 327 | 30 | 5 | 是 |
| F163 | 張松 | 196 | 36 | 2 | 是 |
| F083 | 許靖 | 224 | 36 | 6 | 是 |
| F044 | 費詩 | 229 | 37 | 7 | 是 |

每張生成圖須保留原圖冠帽、服色、鬚髮、朝向、可見眼睛與表情，不依劇本年齡或通俗人物形象改造。採用既定 B 樣圖作筆觸參考，完整 4:5 圖縮放至 256×320，不裁切、不拉伸、不烘焙文字。來源、提示詞、候選與退件保留本機，Codex 審查與使用者逐張簽核分列。四十六張原圖已逐張查看，六十六份候選均為不透明的 1122×1402 圖，比例檢查全部通過。Codex 採用四十六張、退件二十份，使用者逐張簽核為零；三頁最終對照已查看，來源與採用圖審查完成，授權建立私人 v13 包。完整提示詞及每次實際參考圖雜湊保存在 `workplace/hd-commanders-v13-generation.json`；`image_gen` 未回報模型版本與 seed，不自行補值。

私人包須保留 v12 全部 330 筆欄位、PNG bytes 與準備紀錄。兩版正式載入器與全族稽核另驗。正常玩家驗證從片頭開劇本 002、單人曹操、難度 5，照上表走本國或原有他國查看確認；九十二張人物卡逐項核對原貌來源、原生高清、圖外文字框線及原貌恢復。不注入人物、日期、勢力或 seed。規則與存檔沿用既有契約，此批不替代其他使用端、三語系、音訊、跨平台或原版 oracle 驗收。

準備結果：私人 v13 包為 180/256 肖像及 31/31 SCG，兩版各 211 筆、共 422 筆，正式載入器警告零。v12 全部 330 筆欄位、PNG bytes 及準備紀錄逐項保持。兩族技術問題均為零，Codex 肖像審查 180、缺 76，場景審查 31、缺零；使用者逐張簽核零。場景完整準備閘門返回 0，肖像返回 3。

正常 GUI 結果為 [both]、L1，限於 remake 的實際玩家路徑。重跑入口為 `bash tools/verify-hd-player.sh --officers-002`，容器內從片頭開兩次正式新局，兩版各查看四十六張人物卡，其中三十六位經他國查看、十位經本國查看。370/370 檢查通過，九十二張卡的原貌來源、256×320 原生高清、圖外文字框線與切回原貌各 92/92 相符；沒有注入人物、日期、勢力或 seed。六頁正常人物卡對照已查看，肖像框、姓名及能力欄位未見裁切或遮擋。

獨立回讀再次核對全部 678 張最新 PNG 的解碼、尺寸、擁有權與雜湊，以及原版六份 GRP、實際工具與程式快照、六十六份候選的原請求及參考圖雜湊、四十六份採用紀錄、完整私人包與 v12 的 330 筆保留紀錄。九十二張卡的四項圖面契約逐像素相符，收據 `workplace/hd-v13-delivery-verification.json` 的 SHA-256 為 `04559216184deaccf537c7a5ace8d89e729d62853f520ae7e70943726df88080`。這些結果涵蓋本批人物卡；其他使用端、三語系、音訊、跨平台與原版 oracle 仍依各自驗收範圍。

剩餘來源診斷沿用已驗的 v12 正式六劇本初次停點清單，依目前 v13 稽核排除已準備槽，再回讀目前來源 PNG 及槽 bytes。剩餘七十六槽中，六十三槽有清單，兩版共 126 列；003／004／005／006 分別為 34／14／9／6。F159 查看選項仍為原版 1、加強版 3，F230 為 11／10，依版保存。另十三槽未在初次停點找到，F003 沒有劇本人物引用；不據此猜用途或注入狀態。這是既有診斷的篩選與來源回讀，沒有重跑玩家流程，不算生成或正常 GUI 完成。

本機準備與驗證產物如下。圖片、來源及含原版內容的包不加入版控或發行包。

| 路徑 | SHA-256 |
|---|---|
| `workplace/hd-commanders-v13-plan.json` | `4002d2ab4e7f1a65f917ac45202743df81626bfc5bec1d5b77be8d9eab051abe` |
| `workplace/hd-commanders-v13-generation.json` | `d0dbfa49b69fd7e302f933a2cd6ee8cd9b63174f88dc8e7c1bb0bf11e1a4f754` |
| `workplace/hd-commanders-v13-requests.json` | `ea20ad5f15a7e71f7a7307b352d1abfb05c5fe0fd5ad686d5d2ae3c25449287a` |
| `workplace/hd-commanders-v13-reviews.json` | `a0675150ad49d71e989912765fd63229a6db06540c2beb6ca4af30e002f935b5` |
| `workplace/hd-commanders-v13-build-receipt.json` | `52f0e1e04c57168ca7a6f9d90d75648622c1f3dca81a7423c58ed81218e9c1d2` |
| `workplace/hd-assets-portraits-v13/manifest.json` | `40e9d06f6e3e511c7691405ad1ff26274a8c7e5b1044d852f98d294bdab7f265` |
| `workplace/hd-assets-portraits-v13/preparation.json` | `0fae92eaed4cbbf2fa61a10221e3cc403c8473e195124798af26c5274f202861` |
| `workplace/hd-portrait-audit-v13.json` | `11dc0ee4ef035a2855e5dfe17422e0d22533e620089c0c1020fc25be00a98a3e` |
| `workplace/hd-scene-audit-v13.json` | `f6feef798512639b6e21dc49de34adfe2c5ad463b8c9c2dd038d61fc08f74481` |
| `workplace/hd-v13-contact-receipt.json` | `eb3cae4e038ea247f324deb4d95ba53908bf9e0f9d0c399808d68f4099fbd455` |
| `workplace/hd-v13-next-plan-verification.json` | `d2cb0f14ee4b81eb558c9fe34e1d2326422d0b369806bc823f6fdcd0e352b808` |
| `workplace/hd-preview/commanders-v13-contact-1.png` | `b3591ebc74407acfcbc43af35e634f378c54909584f2587a57f852dc66ff93f7` |
| `workplace/hd-preview/commanders-v13-contact-2.png` | `f24bb6080bc8a2c3737e785c4fb0b2c0087b717b94b425cca3e5ee3eb1cc87de` |
| `workplace/hd-preview/commanders-v13-contact-3.png` | `a88925440827ee882ff604022a91014bffeac8c913a41ba1b86e3bcce9fe6672` |
| `workplace/hd-window/player/commanders-v13/receipt.json` | `910a55d8e752eb38d2208fae2679823c78dc3ffed4efeea31b1768f5af2e866f` |
| `workplace/hd-v13-delivery-verification.json` | `04559216184deaccf537c7a5ace8d89e729d62853f520ae7e70943726df88080` |
| `workplace/hd-v13-gui-contact-receipt.json` | `919547948ad93d79c539d9295f08d013da02d66f7bd993155f9f509765823bbf` |
| `workplace/hd-v13-audit-commands.json` | `efb62d6601823c140cbc7662b722659b81c0f3959e0dbf9231aa94e423253e7a` |
| `workplace/verify-hd-v13-delivery.py` | `00ba2b797b2d1a6fcef3bac76dedfe60522abc5aa9b934f50fa3ac581792f77a` |
| `workplace/hd-preview/commanders-v13-gui-base-contact-1.png` | `cc61ffea46f4bf678c5dc6ef444b12d04867a8afe4350e907be678fd17770966` |
| `workplace/hd-preview/commanders-v13-gui-base-contact-2.png` | `acd428ec0030090f49a80692a68f497a8ad34dfb603c08301e7f2e9677ffea7c` |
| `workplace/hd-preview/commanders-v13-gui-base-contact-3.png` | `eb86d5e9bbdfe7f99fbcd69a88936e3956cf2dc0355955e681b51f59b48e95a7` |
| `workplace/hd-preview/commanders-v13-gui-plus-contact-1.png` | `d51a7595d272bacad841124387bc5a7e423b5e3cf7a9e0990e171fd9e86feb19` |
| `workplace/hd-preview/commanders-v13-gui-plus-contact-2.png` | `ce93da45f6d603ee97e5d4a70fad22bfa4f6f1fd548a332a5dc2702266db8646` |
| `workplace/hd-preview/commanders-v13-gui-plus-contact-3.png` | `d5d5191698f37a84b212000aa2d02a1ea9d3796435717c1e17447a72b14901a8` |

### 6.25 劇本 003 新增三十四張肖像

**狀態：READY**。沿用 §6.15／6.24 的固定肖像、B 寫實手繪、4× 與原貌預設契約。這批選取正式劇本 003 初次玩家停點可查看、尚未製作的三十四個 DATA3 槽。兩版 64×80 來源 PNG 與實際原始槽 bytes 逐項相同，原始資料為 L0、[both]。清單沿用已驗的 v12 正式六劇本診斷，由 v13 剩餘清單篩選，再回讀目前 DATA3.GRP／IDX／NAM 與 PNG；此步沒有重跑玩家流程，不當作原版 oracle。

兩版均為 003、單人曹操、難度 5，201 年元月停在郡 14；每版二十七位經他國查看、七位經本國查看。F159 的清單選項分版保留，原版 1、加強版 3。完整六十八列本機計畫 `workplace/hd-commanders-v14-plan.json`，SHA-256 `5769200cdcc98c5f915ece16f79a28ab2285143137bef3149ec300760665a826`；來源回讀收據 `workplace/hd-v14-plan-verification.json`，SHA-256 `9c46be7c8a1571d5f6c88e72c07cc218c729d8cb4f3452a0d092b8dbf56fd2d6`。

| 肖像槽 | 人物 | 人物索引 | 查看郡 | 清單選項，原版／加強版 | 他國確認 |
|---|---|---|---|---|---|
| F025 | 夏侯霸 | 334 | 6 | 2 | 否 |
| F057 | 臧霸 | 80 | 6 | 7 | 否 |
| F159 | 諸葛瑾 | 127 | 9 | 1／3 | 是 |
| F242 | 郝昭 | 275 | 13 | 3 | 否 |
| F058 | 毛玠 | 73 | 13 | 13 | 否 |
| F056 | 楊修 | 197 | 15 | 1 | 否 |
| F048 | 曹真 | 254 | 15 | 3 | 否 |
| F082 | 賈逵 | 244 | 15 | 9 | 否 |
| F059 | 楊昂 | 337 | 18 | 4 | 是 |
| F138 | 龐德 | 184 | 19 | 2 | 是 |
| F216 | 張紘 | 98 | 21 | 5 | 是 |
| F144 | 張昭 | 97 | 21 | 7 | 是 |
| F110 | 孫翊 | 57 | 21 | 12 | 是 |
| F219 | 嚴峻 | 329 | 21 | 14 | 是 |
| F152 | 程秉 | 147 | 21 | 15 | 是 |
| F135 | 朱桓 | 148 | 23 | 2 | 是 |
| F132 | 魯肅 | 126 | 24 | 1 | 是 |
| F217 | 陸績 | 328 | 24 | 4 | 是 |
| F234 | 徐盛 | 153 | 25 | 1 | 是 |
| F247 | 趙統 | 277 | 27 | 5 | 是 |
| F252 | 關平 | 124 | 27 | 6 | 是 |
| F039 | 周倉 | 123 | 27 | 8 | 是 |
| F207 | 蔡勳 | 142 | 28 | 8 | 是 |
| F141 | 劉琦 | 138 | 30 | 2 | 是 |
| F123 | 韓浩 | 333 | 30 | 3 | 是 |
| F070 | 劉賢 | 168 | 30 | 5 | 是 |
| F122 | 劉琮 | 139 | 31 | 1 | 是 |
| F108 | 楊齡 | 176 | 31 | 4 | 是 |
| F072 | 鞏志 | 174 | 32 | 2 | 是 |
| F182 | 霍峻 | 213 | 35 | 1 | 是 |
| F198 | 法正 | 10 | 36 | 9 | 是 |
| F106 | 李嚴 | 219 | 37 | 9 | 是 |
| F109 | 向朗 | 215 | 37 | 10 | 是 |
| F158 | 王伉 | 259 | 38 | 4 | 是 |

每張保留原圖冠帽、服色、鬚髮、朝向、可見眼睛與表情，不依劇本年齡或通俗形象換臉。原版 64×80 圖的最近鄰 16× 放大圖作編輯目標。早期兩張人物請求另附 B 人物圖，發現帽形受參考人物影響後，未執行的請求改以單張來源與文字描述既定 B 筆觸；已執行的原請求保留。郝昭與趙統的修正版以退稿作目標、原圖作外觀依據，分別修短圓捲邊帽與單眼側臉。完整 4:5 圖縮放至 256×320，不裁切、不拉伸、不烘焙文字。

原圖三十四張、三十八張候選與三頁對照總覽均已逐張查看。三十四張由 Codex 採用，四張因冠帽或可見眼睛偏差退件並保留；使用者逐張簽核為零。各候選為 1122×1402、不透明，符合 §6.15 比例誤差契約。來源、原請求、實際參考圖清單與雜湊、候選及退件保留本機。單來源請求的 B 參考只記為畫風沿革，不宣稱曾作圖像輸入。模型與 seed 為工具未回報。生成記錄 `workplace/hd-commanders-v14-generation.json`，SHA-256 `4f62343d2b770c68ebce5266ecfc1a5c93ad8ef47200f1acacc6122440e4e396`；原請求 `workplace/hd-commanders-v14-requests.json`，SHA-256 `9b4f221bba761977605c3a0b0e632a775771ee79718001855675ba18334c4ea9`；對照頁收據 `workplace/hd-v14-contact-receipt.json`，SHA-256 `c88b193463da574aa7846ec9d00d4cc0544c512c6ef27f87ca07f0ee252387eb`。本機彙整、總覽與建包入口為 `workplace/collect-hd-commanders-v14.py`、`workplace/make-hd-v14-contact.py`、`workplace/build-hd-commanders-v14.py`。

私人包須保留 v13 全部 422 筆欄位、PNG bytes 與準備紀錄，兩版正式載入器與全族稽核另驗。正常 GUI 從片頭開上述新局，依分版清單查看六十八張人物卡，逐項核對原貌來源、原生高清、圖外文字框線及原貌恢復。不注入人物、日期、勢力或 seed。此批不改規則、存檔或音訊，也不替代其他使用端、三語系、跨平台或原版 oracle 驗收。


清單換頁沿用 [014 §4.2](014-art-main-overlays.md#42-挑一位將軍0x18024l0l1baseissue-77) 的十二列與空白換頁契約。驗證工具以當局實際清單首列的 1／13 字模判斷頁面，需要時按空白，不讀取或注入遊戲狀態。首次批次因未換頁而在毛玠第十三位停下，保留 `workplace/hd-window/player/commanders-v14-attempt-1/` 的全部收據與畫面；此為驗證腳本問題，正式遊戲行為未改。

私人 v14 包備妥 214/256 肖像與 31/31 SCG，兩版各 245 筆、共 490 筆，245 張獨立 PNG；正式載入器警告零。v13 全部 422 筆欄位、PNG bytes 與準備紀錄完整保持。肖像缺 42、Codex 審查 214，場景缺 0、審查 31；兩族技術問題及使用者逐張簽核均為零。場景完整準備閘門返回 0，肖像返回 3。

`bash tools/verify-hd-player.sh --officers-003` 從片頭開兩版正式 003 曹操新局，六十八張人物卡共 274/274 通過，外層返回 0。原貌來源、256×320 原生高清、圖外文字框線與原貌恢復各 68/68 相符，啟動仍為原貌及隱藏選項列。587 張最新 PNG、六十八份清單頁碼、三份工具快照、執行檔、原版六份 GRP、完整包與候選獨立回讀相符；四類人物卡像素另以獨立 RGB 解碼核對。三頁來源／高清對照與兩版各三頁正常人物卡均已查看。獨立工具首次誤寫遮罩右側界線，修正後在同一容器環境完整重跑通過；失敗工具快照及分類紀錄保留本機。

本機獨立回讀、正常人物卡總覽及文件核對入口為 `workplace/verify-hd-v14-delivery.py`、`workplace/make-hd-v14-gui-contact.py`、`workplace/verify-hd-v14-docs.py`。來源準備入口為 `workplace/prepare-hd-commanders-v14-plan.py`。兩族稽核的完整命令列保存於 `workplace/hd-v14-audit-commands.json`。上述工具依專案 Docker-only 契約執行，原始資料、生成原檔、包與 GUI 截圖唯讀掛載；獨立工具使用 `eob-audio-capture:20260922-r2`，生成原檔目錄掛到 `/generated`，原版資料掛到 `/orig`，工作樹掛到 `/src`。私人資料未加入 Git。

剩餘 42 槽中，29 槽有已驗的正式初次停點清單，004／005／006 各 14／9／6；其餘十三槽路徑未明。`workplace/verify-hd-v14-next-plan.py` 從固定 v12 診斷排除已準備槽，另核對當前來源與全槽稽核，收據為 `workplace/hd-v14-next-plan-verification.json`；此篩選不算下一批生成或新 GUI 驗收。場景素材、正式 Go 程式、規則、存檔、音訊及既有原版 oracle 保持原範圍，其他使用端、素材家族、三語系及跨平台仍待完成。圖片、候選、完整收據與含原版內容的包只留本機，未建立 Release，整份 021 保持 READY。

| 本機產物 | SHA-256 |
|---|---|
| `workplace/hd-commanders-v14-plan.json` | `5769200cdcc98c5f915ece16f79a28ab2285143137bef3149ec300760665a826` |
| `workplace/hd-v14-plan-verification.json` | `9c46be7c8a1571d5f6c88e72c07cc218c729d8cb4f3452a0d092b8dbf56fd2d6` |
| `workplace/hd-commanders-v14-source-observations.json` | `6e4b3b319a3c0ff6982f4c798711765f605e538e96bbe84f831dda9c838dfaf6` |
| `workplace/hd-commanders-v14-requests.json` | `9b4f221bba761977605c3a0b0e632a775771ee79718001855675ba18334c4ea9` |
| `workplace/hd-commanders-v14-generated-paths.json` | `2f7932da506f99d26391d5a1a0c676561c317010936f4b29751dacadd0438493` |
| `workplace/hd-commanders-v14-generation.json` | `4f62343d2b770c68ebce5266ecfc1a5c93ad8ef47200f1acacc6122440e4e396` |
| `workplace/hd-commanders-v14-reviews.json` | `6c9c5fbe1154fa9a0a2b5cee6d191b044a96845ce758c49306ab92360aa14d4c` |
| `workplace/hd-v14-contact-receipt.json` | `c88b193463da574aa7846ec9d00d4cc0544c512c6ef27f87ca07f0ee252387eb` |
| `workplace/hd-assets-portraits-v14/manifest.json` | `3d008a020bf4ad34ab43647580bf095b6f1d75b0df1cf75ac95139fe2fe05dc5` |
| `workplace/hd-assets-portraits-v14/preparation.json` | `59434c0fef8db8a3c29ef8f7fa89ad8546f61f356acf6a31a225bbeed1c1a933` |
| `workplace/hd-commanders-v14-build-receipt.json` | `3db361be592b64ad66d1830ea5b7dd62ba23be51c8dd0e37005fa977dad10651` |
| `workplace/hd-v14-audit-commands.json` | `3cd1c16b205e31d59c9ad3296c5e852120875ef9f042449322899384b1b0e143` |
| `workplace/hd-portrait-audit-v14.json` | `e11e3c3ba804e26181d177bf1d0718e260edc3766dd48eacce3ad51e85b586f0` |
| `workplace/hd-scene-audit-v14.json` | `4b720709bfce8dd03bd1e61f0dc9365b45228b6d0a1237960aba3324cfa8c8f8` |
| `workplace/hd-window/player/commanders-v14/receipt.json` | `dcf0559f922cea26af1150657d940a3068a791156c3320fa32fd9123383f707b` |
| `workplace/hd-v14-gui-contact-receipt.json` | `c567909bf647bdbcba5a6730d2f59d7448a3510df759d16dd843f37f308441e1` |
| `workplace/hd-v14-next-plan-verification.json` | `5c39d6e87c32bb3a7c9f9f0984be7f424241ca0635f5d50c7181e256d6030143` |
| `workplace/verify-hd-v14-delivery.py` | `cdf2fcafe869df62c8d83f38cc6bb89e6f18f25fdfcec9c687ff0a2c52fc5702` |
| `workplace/make-hd-v14-gui-contact.py` | `47f0efa104a14ff9da56dcf95e837e48e4c6c3ff8c18ee54e4d84083826eb7a4` |
| `workplace/verify-hd-v14-docs.py` | `19ad51328bbc6cbadfd53882671ab177029a0ee0201ef2e166f146b16799be95` |
| `workplace/verify-hd-v14-next-plan.py` | `251e582b93779b0a22d168ab103bbc87dfd02f912182f41404ae8ed0fe7b90c4` |
| `workplace/prepare-hd-commanders-v14-plan.py` | `a908f50e41054f8c9b7277d57ebdfbec55d6420dd52866e19087c3cec0b7d617` |
| `workplace/hd-v14-delivery-verification.json` | `8d25c4195cb73253487c48f29228209de4b1b7be575fd6166936f5f543ded25a` |
| `workplace/hd-v14-delivery-attempt-1.json` | `beea48cabc85a388372253c8d85b08f38ef0ca6a2c48401901f82b083a94c0bc` |
| `workplace/verify-hd-v14-delivery-attempt-1.py` | `9b57d47673af09ee33966b38fe286100ee2e811393757f4edccd9cd215c54b62` |
| `workplace/hd-window/player/commanders-v14-attempt-1/receipt.json` | `d20f90bd51aaaa14ce8c43961acdf081b8520dd4313da14cfdf762c99ccd4203` |


### 6.26 劇本 004 新增十四張肖像

**狀態：READY**。沿用 §6.15／6.25 的固定肖像、B 寫實手繪、4× 與原貌預設契約。這批選取正式劇本 004 初次玩家停點可查看、尚未製作的十四個 DATA3 槽。兩版 64×80 來源 PNG 與原始槽 bytes 逐項相同，原始資料為 L0、[both]。清單由已驗的 v12 正式六劇本診斷與 v14 剩餘清單篩選，再回讀當前 DATA3.GRP／IDX／NAM 與 PNG；這一步未重跑玩家流程，不當作原版 oracle。

兩版均為 004、單人曹操、難度 5，208 年元月停在郡 13；每版本國三位、他國十一位。F230 保留原版清單選項 11、加強版 10。完整二十八列本機計畫為 `workplace/hd-commanders-v15-plan.json`，SHA-256 `40916419ec188905f420383f80b39a0caab54ca949d9ce50e782686ee4dfebde`；來源回讀收據為 `workplace/hd-v15-plan-verification.json`，SHA-256 `75cf6ef37272922c8163d68f62e1a9b8724456fa19de807b8c1f96c8d9233caf`。

| 肖像槽 | 人物 | 人物索引 | 查看郡 | 清單選項，原版／加強版 | 他國確認 | 原圖特徵 |
|---|---|---|---|---|---|---|
| F068 | 陳群 | 183 | 12 | 1 | 否 | 朝左的成年面容、黑髮與頂端金色髮飾、濃黑長鬚、綠衣、雙眼 |
| F115 | 夏侯德 | 235 | 13 | 8 | 否 | 近正面略朝左、黑髮與上端灰白飾、黑色短鬚、略張口露齒、白領與紅紫衣、雙眼 |
| F127 | 曹植 | 136 | 13 | 9 | 否 | 朝左面容、黑髮與灰白後側飾、黑色髭與尖長下巴鬚、灰白領及紅紫衣、雙眼 |
| F230 | 周魴 | 276 | 21 | 11／10 | 是 | 朝左低頭、斜向金色冠帽與正面綠飾、黑色鬚髮、灰白領與紅衣、雙眼 |
| F087 | 張溫 | 149 | 25 | 5 | 是 | 朝左面容、黑髮與頂端金飾、細黑鬚紋、灰白領與深青衣、遠側眼僅小部分 |
| F042 | 關索 | 261 | 27 | 2 | 是 | 朝左面容、黑髮與青色髮飾、黑鬚、略張口露齒、青衣、雙眼 |
| F017 | 關興 | 241 | 27 | 4 | 是 | 朝左的無鬚面容、藍色獸紋盔與黃飾、青衣 |
| F214 | 馬良 | 165 | 28 | 9 | 是 | 朝左垂視、淺白眉紋、金橙冠、黑色髭與尖下巴鬚、白領與綠衣 |
| F235 | 張苞 | 250 | 29 | 3 | 是 | 朝左側臉、紫色布帽、濃黑鬚、紫衣、單眼 |
| F124 | 吳班 | 249 | 30 | 5 | 是 | 略朝左、藍灰圓盔與紅飾、短黑鬚、粉紅衣與白領、雙眼 |
| F064 | 刑道榮 | 169 | 34 | 2 | 是 | 朝左側臉、綠色紋飾盔與黃邊、濃黑鬚、露齒張口、紫紅衣、單眼 |
| F140 | 蔣琬 | 216 | 37 | 1 | 是 | 略朝左、灰銀紋飾盔與紫飾、黑鬚、紅紫衣與白領、雙眼 |
| F121 | 劉巴 | 207 | 38 | 2 | 是 | 朝左、灰白邊黑冠、細黑唇鬚及小下頷暗紋、青衣與白領、雙眼 |
| F023 | 向寵 | 266 | 39 | 1 | 是 | 朝左、粉紅金色紋飾盔、黑鬚、綠衣及白領、雙眼 |

來源十四張及其最近鄰 16× 編輯目標均已逐張查看。以單張原圖編輯目標與文字描述既定 B 筆觸生成，人物識別只依該槽來源，不依劇本年齡或通俗形象換臉。冠帽、服色、鬚髮、朝向、可見眼睛與表情須保持，完整 4:5 圖縮放至 256×320，不裁切、不拉伸、不烘焙文字。原請求、實際參考圖與雜湊、採用版次及退件保留本機；模型與 seed 以工具實際回報記錄，尚無使用者逐張簽核。

本機來源準備入口為 `workplace/prepare-hd-commanders-v15-plan.py`；來源放大圖為 `workplace/hd-preview/v15-source-F###-8x.png` 與 `v15-source-F###-16x.png`。候選須逐張審查，來源／高清對照頁全部查看後才能轉 READY 並建私人 v15 包。包須保留 v14 全部 490 筆欄位、PNG bytes 與準備紀錄，正式載入器及全族稽核另驗。正常 GUI 必須從片頭開上述新局、依分版清單查看二十八張人物卡，核對原貌來源、原生高清、圖外文字框線及原貌恢復，不注入人物、日期、勢力或 seed。這批不改規則、存檔或音訊，也不替代其他使用端、三語系、跨平台或原版 oracle 驗收。

原請求、逐項來源觀察、實際生成路徑與審查登錄分別存於 `workplace/hd-commanders-v15-requests.json`、`workplace/hd-commanders-v15-source-observations.json`、`workplace/hd-commanders-v15-generated-paths.json`、`workplace/hd-commanders-v15-reviews.json`。已定案 B 沿革參考為 `workplace/hd-b-F172-v2.png`，SHA-256 `3e0e123211e7002af31c5de14625eba9b6e93d46ed9e061e200f37fd7446835b`；本批初版不把它作實際圖像輸入。

已逐張查看十九份候選及最終來源／高清總覽，十四張採用、五張退件保留。夏侯德、曹植、關索、馬良及張苞採第二版，其餘採第一版；使用者逐張簽核零。全部候選為 1122×1402、不透明且符合既有 4:5 誤差契約，縮放完整圖至 256×320。生成彙整 `workplace/hd-commanders-v15-generation.json` 的 SHA-256 為 `23ae74f734d11a12c3dca52881702120131dd41a48f676b8912fce103b889b26`；最終總覽 `workplace/hd-preview/commanders-v15-contact-1.png` 為 `1773a0c7f6a377ad23cf07436c18d04f0dbf958b68c20385d790f7dfbb06b902`。本批 READY 已解鎖私人 v15 包，正常人物卡與獨立回讀均已通過；整份 HD 仍維持 READY。

私人 v15 包為 228/256 肖像、31/31 SCG，兩版各 259 筆、共 518 筆。正式載入器零警告，v14 全部 490 筆欄位、PNG bytes 與準備紀錄逐項保持。全族稽核的技術問題及使用者逐張簽核均為零；場景完整準備閘門返回 0，肖像因缺二十八槽返回 3。

重跑入口為 `bash tools/verify-hd-player.sh --officers-004`。工具在 Docker 內從兩版片頭開 004 曹操、難度 5 正常新局，以各版正式清單查看十四人；每版他國十一人、本國三人，全部選項在第一頁。二十八張卡的原貌來源、256×320 原生高清、圖外文字框線及原貌恢復均 28/28，另兩次啟動的原貌及隱藏列共 114/114 通過，203 張最新 PNG。未注入人物、日期、勢力或亂數，此收據屬 remake 正常 GUI，不是新美術與原版的像素 parity。

獨立回讀入口為 `workplace/verify-hd-v15-delivery.py`，在 `eob-audio-capture:20260922-r2` Docker 內執行；回讀全部 203 張最新 PNG 的尺寸、雜湊、可解碼性與擁有權，二十八張人物卡的來源、原生像素、圖外文字框線及原貌恢復，以及十九份候選的實際生成輸出、完整原請求、實際參考雜湊、不透明像素、工具未回報的模型／seed 分類、原始 DATA3 容器與全族稽核。v14 全部 490 筆準備紀錄逐項保持。兩版人物卡總覽均已查看。

從已驗 v12 正式六劇本診斷排除 v15 已備槽，剩二十八槽中十五槽有初次清單入口，005 九槽、006 六槽；另十三槽尚未在初次清單找到。分版清單差異零。來源雜湊回讀通過；此步未重跑玩家流程，不增加生成、GUI 或原版 oracle 完成聲明。

| 本機產物 | SHA-256 |
|---|---|
| `workplace/hd-commanders-v15-plan.json` | `40916419ec188905f420383f80b39a0caab54ca949d9ce50e782686ee4dfebde` |
| `workplace/hd-v15-plan-verification.json` | `75cf6ef37272922c8163d68f62e1a9b8724456fa19de807b8c1f96c8d9233caf` |
| `workplace/prepare-hd-commanders-v15-plan.py` | `4ad58fff7b5bedaa9a34c1563976a30eb096ae1443594274d904471d426912d6` |
| `workplace/hd-commanders-v15-requests.json` | `388e7475359993e5f9c1ffd5be8ee6fd873c2f443f7dba3a57820b2b62494d15` |
| `workplace/hd-commanders-v15-source-observations.json` | `686068c8718c985ae8e5cb568a27e94cfca59e73d9e149931d07a93ad87ee0bb` |
| `workplace/hd-commanders-v15-generated-paths.json` | `20d9e44316abd2c29f6ff7179f09a162651ffa5594a6cc1bbae46d3848e5a212` |
| `workplace/hd-commanders-v15-reviews.json` | `9ab152439c18bf9bc326ca9763468300c668310d5aa3147b585e74941f974f84` |
| `workplace/hd-commanders-v15-generation.json` | `23ae74f734d11a12c3dca52881702120131dd41a48f676b8912fce103b889b26` |
| `workplace/collect-hd-commanders-v15.py` | `abc8f1ed171ce788ab2d21ca3b6294785fcc9f8b95d96bb8bc559cd0bb4439c8` |
| `workplace/build-hd-commanders-v15.py` | `8e2fc4eaf06bcc5498d8acbc24c92400a3add9a67f3fc66bd942cd08762e3726` |
| `workplace/hd-commanders-v15-build-receipt.json` | `c4429a36d167089cb9b44ece30b5a63305c46b9a26e2cf69d8ab85596b158704` |
| `workplace/hd-assets-portraits-v15/manifest.json` | `4382c1769dae9572333d64a020e03fbaef59747269464c518d9788173e9551e7` |
| `workplace/hd-assets-portraits-v15/preparation.json` | `0199fe7d980521e9d4cf038b1e6dc0379ad45225ac00030e4c368ceb80152696` |
| `workplace/hd-portrait-audit-v15.json` | `8220ab1c653c5cff220d0acc5570c0435ad1a974d51fa3df58414905cd1be7fd` |
| `workplace/hd-scene-audit-v15.json` | `d80540a9dc24c72102f45e805d477c229ff36f573d038e348bf119ec85c4b20e` |
| `workplace/hd-v15-audit-commands.json` | `4fb3d6a58c41c2348de76091b5c72534221615ca148a53ace6fd7d29052d0452` |
| `workplace/hd-v15-contact-receipt.json` | `43d1e1ae7537a350e3f6a4912c4a28d08b9c66078c0d5db19f1a8a3f28d14e2b` |
| `workplace/hd-preview/commanders-v15-contact-1.png` | `1773a0c7f6a377ad23cf07436c18d04f0dbf958b68c20385d790f7dfbb06b902` |
| `workplace/hd-v15-next-plan-verification.json` | `7d4c2151cefe15210d8e023a99964b40924007267f9a4c0444f44fcbb488abe1` |
| `workplace/verify-hd-v15-next-plan.py` | `959433590503db053c2123e3398b8cb17bae1e5e5aaa605e37002d118e35bbfb` |
| `workplace/verify-hd-v15-delivery.py` | `6657f1ec3c504dcc9eb65b5605e32e75e9c84921791da61b5025939583e593ff` |
| `workplace/hd-window/player/commanders-v15/receipt.json` | `d440efa6d250ea6f24e3d856789a4917498b2597b48f20b539df6aa3cff0f782` |
| `workplace/hd-v15-delivery-verification.json` | `d14abee96c11df3a3f374145deb8dabd0aa17d019f977ab72d85f57dc5abb349` |
| `workplace/hd-v15-gui-contact-receipt.json` | `718979433fd3944d42801880407a053681f713618dfe2e693388350adcda4e5a` |
| `workplace/make-hd-v15-gui-contact.py` | `c7dfc193f6d8960ffd81bb32b7c1dfab2f78b571dbc3516ea2a2d1574f1c77f0` |
| `workplace/hd-preview/commanders-v15-gui-base-contact-1.png` | `dc22456ea1c8ef0019de7e9b24318997b1f475365a1fb926584db6d98de3d379` |
| `workplace/hd-preview/commanders-v15-gui-plus-contact-1.png` | `9346b3a93edc7406de19aa53a0cec3493cb7215875e73a1d1c15f1a8629c6b3e` |

### 6.27 劇本 005 新增九張肖像

**狀態：READY**。沿用 §6.15／6.26 的固定肖像、B 寫實手繪、4× 與原貌預設契約。從已驗 v12 正式六劇本診斷及 v15 剩餘清單選取九槽，再直接回讀兩版 DATA3.GRP／IDX／NAM、原始槽 bytes 與 64×80 PNG，逐項相同，原始資料為 L0、[both]。來源準備未重跑玩家流程，不當作原版 oracle。

兩版均為 005、單人曹操、難度 5，215 年元月停在郡 19；每版本國二位、他國七位，分版選項相同且均在第一頁。姓名沿用遊戲資料，包含「費褘」。完整十八列本機計畫 `workplace/hd-commanders-v16-plan.json`，SHA-256 `1a54d15c6ad7bdf78cc7878e9d3a3bf41bd395f4989ee4f41be38cffbc858bec`；來源回讀收據 `workplace/hd-v16-plan-verification.json`，SHA-256 `814ff96c1a79292861ce3deaf6a2d884ff84ccd370ee389c6b65a2366f701fb4`。

| 肖像槽 | 人物 | 人物索引 | 查看郡 | 清單選項 | 他國確認 | 原圖特徵 |
|---|---|---|---|---|---|---|
| F232 | 崔琰 | 321 | 4 | 1 | 否 | 朝左的三分之二側臉、雙眼、低平深藍帽與青色斜帽帶及淺色小飾、下垂黑髭及尖黑鬚、青領與紫藍衣、閉口 |
| F139 | 郭淮 | 233 | 16 | 3 | 否 | 略朝左的近正面、雙眼、紅色圓盔與藍色方飾及金邊、粗眉與下垂黑髭、短而濃的黑鬚、紫衣、略張口露齒 |
| F193 | 傅士仁 | 240 | 28 | 5 | 是 | 朝左仰頭、雙眼向上、青黑盔與紅色前飾及淺色邊線、黑髭與尖黑鬚、略張口露出白色上齒、藍青衣 |
| F221 | 董允 | 340 | 28 | 7 | 是 | 朝左側臉、雙眼、露額黑髮及高髻後側灰青飾、黑髭與長尖黑鬚、灰領與青藍衣、閉口 |
| F239 | 鄧芝 | 226 | 35 | 4 | 是 | 朝左側臉、單眼、黑灰低冠與直條紋、黑髭及長尖黑鬚、灰黑領衣與下緣紫紅色區、閉口 |
| F107 | 費褘 | 228 | 36 | 10 | 是 | 朝左側臉、雙眼、露額黑髮及右上方灰黑高髮飾、黑髭與尖黑鬚、灰白領與黑衣、閉口 |
| F220 | 郭攸之 | 339 | 36 | 12 | 是 | 略朝左、雙眼微垂、露額黑髮與右上藍青條紋飾、黑髭與短尖黑鬚、紫衣與青紫領、閉口 |
| F136 | 楊儀 | 267 | 38 | 3 | 是 | 強烈朝左的側臉、依原圖保留眼區、灰黑折冠及條紋、露額、細黑髭與細長尖黑鬚、青衣白領、閉口 |
| F038 | 呂凱 | 260 | 38 | 6 | 是 | 朝左側臉、單眼、青色直條冠與橫帽帶及側繫帶、黑髭與細長尖黑鬚、綠衣白領、閉口 |

九張原圖與最近鄰 16× 編輯目標已逐張查看。人物識別以各槽原圖為準，冠帽、服色、鬚髮、朝向、可見眼睛與表情保持；完整 4:5 圖縮放至 256×320，不裁切、不拉伸、不烘焙文字。初版只用該人物原圖作圖像輸入，以文字描述既定 B 筆觸。已定案 B 沿革參考 `workplace/hd-b-F172-v2.png`，SHA-256 `3e0e123211e7002af31c5de14625eba9b6e93d46ed9e061e200f37fd7446835b`，不作本批初版圖像輸入。

來源準備入口 `workplace/prepare-hd-commanders-v16-plan.py`；來源放大圖在 `workplace/hd-preview/v16-source-F###-8x.png` 與 `v16-source-F###-16x.png`。完整原請求、來源觀察、實際生成路徑及審查分別記於 `workplace/hd-commanders-v16-requests.json`、`workplace/hd-commanders-v16-source-observations.json`、`workplace/hd-commanders-v16-generated-paths.json`、`workplace/hd-commanders-v16-reviews.json`。候選及最終來源／高清總覽全部查看後才能轉 READY 並建私人 v16 包。包須保留 v15 全部 518 筆欄位、PNG bytes 與準備紀錄，另驗正式載入器及全族稽核。

正常 GUI 須從兩版片頭開上述新局、以正式清單查看十八張人物卡，核對原貌來源、原生高清、圖外文字框線及原貌恢復，不注入人物、日期、勢力或 seed。本批不改規則、存檔或音訊，也不替代其他使用端、三語系、跨平台、使用者逐張簽核或原版 oracle 驗收。

十二份實際候選及最終來源／高清總覽均已逐張查看，九張採用、三張退件保留。郭淮、鄧芝與呂凱採第二版，其餘採第一版；使用者逐張簽核零。全部候選為 1122×1402，不透明且符合既有 4:5 誤差契約，縮放完整圖至 256×320。生成彙整 `workplace/hd-commanders-v16-generation.json` 的 SHA-256 為 `7264bcd4fe843b059ad71402ef09e2c76c0c75bd1db26b42cd3c1f35309b5a30`；最終總覽 `workplace/hd-preview/commanders-v16-contact-1.png` 為 `c86bc0e311e9df467bcd6d9e99f941676bff81529c1edba3c60f076f81cb17c6`。本批 READY 解鎖私人 v16 包；建包入口 `workplace/build-hd-commanders-v16.py`，生成彙整及總覽入口 `workplace/collect-hd-commanders-v16.py`、`workplace/make-hd-v16-contact.py`。整份 HD 仍保持 READY。

私人 v16 包為 237/256 肖像、31/31 SCG，兩版各 268 筆、共 536 筆。正式載入器零警告，v15 全部 518 筆欄位、PNG bytes 與準備紀錄逐項保持。全族稽核的技術問題及使用者逐張簽核均為零；場景完整準備閘門返回 0，肖像因缺十九槽返回 3。

重跑入口 `bash tools/verify-hd-player.sh --officers-005`，在 Docker 內從兩版片頭各開 005 曹操、難度 5 正常新局，以正式清單查看九人；每版他國七人、本國二人，全部選項在第一頁。十八張卡的原貌來源、256×320 原生高清、圖外文字框線及原貌恢復均 18/18，另兩次啟動的原貌及隱藏列共 74/74 通過，133 張最新 PNG。未注入人物、日期、勢力或亂數；這是 remake 正常 GUI 收據，不是新美術與原版的像素 parity。

獨立回讀入口 `workplace/verify-hd-v16-delivery.py`，於 `eob-audio-capture:20260922-r2` Docker 內執行。回讀全部 133 張最新 PNG 的尺寸、雜湊、可解碼性與擁有權，十八張人物卡的來源、原生像素、圖外文字框線及原貌恢復，以及十二份候選的實際生成輸出、完整原請求、實際參考雜湊、不透明像素、工具未回報的模型／seed 分類、原始 DATA3 容器、536 筆素材與全族稽核。v15 全部 518 筆準備紀錄保持；兩版人物卡總覽均已查看，製作入口 `workplace/make-hd-v16-gui-contact.py`。

從已驗 v12 正式六劇本診斷排除 v16 已備槽，剩十九槽中六槽有初次清單入口，均在 006；另十三槽尚未在初次清單找到。分版清單差異零，來源雜湊回讀通過。入口 `workplace/verify-hd-v16-next-plan.py`；此步未重跑玩家流程，不增加生成、GUI 或原版 oracle 完成聲明。文件及公開檢查入口 `workplace/verify-hd-v16-docs.py`、`workplace/verify-hd-v16-publish.py`，公開檢查範圍限於實際暫存文字的 PNG 簽章、完整提示詞與 MZ 前綴。

| 本機產物 | SHA-256 |
|---|---|
| `workplace/hd-commanders-v16-plan.json` | `1a54d15c6ad7bdf78cc7878e9d3a3bf41bd395f4989ee4f41be38cffbc858bec` |
| `workplace/hd-v16-plan-verification.json` | `814ff96c1a79292861ce3deaf6a2d884ff84ccd370ee389c6b65a2366f701fb4` |
| `workplace/prepare-hd-commanders-v16-plan.py` | `4e4c91856c29ca640cd086a499326625d2eff603e64aef451d43a4f701e069ca` |
| `workplace/hd-commanders-v16-requests.json` | `ee6ef5d3900c19833ca3f0a0871f1ecdfde4b457cc4a96161ffdb2ea7bb170dc` |
| `workplace/hd-commanders-v16-source-observations.json` | `2550c2690e2418ed2fbf3d57a41823fd17cb11929799819a0b7fd71a42c030db` |
| `workplace/hd-commanders-v16-generated-paths.json` | `1d5a46b679e58642b95cfbe6aca4046ba551d44dd852bb36396025ba0b717f1d` |
| `workplace/hd-commanders-v16-reviews.json` | `342b819d7f0a38c8aca891078e73a707d6b918385a6cb39b4d315b013ddc28c4` |
| `workplace/hd-commanders-v16-generation.json` | `7264bcd4fe843b059ad71402ef09e2c76c0c75bd1db26b42cd3c1f35309b5a30` |
| `workplace/collect-hd-commanders-v16.py` | `ca9c4b2b80a73aae2ecc4cc20fafd93d4544628a82e624a5179fd14f68d272ab` |
| `workplace/make-hd-v16-contact.py` | `ec2c690bac6646bed33ba78450d0c94197ed208dbf01417342c28725580b860e` |
| `workplace/build-hd-commanders-v16.py` | `cac3fe632af57dc7ccadf12a9dc7ff9ddb312c98cc743c798a4cb20e3b8c0ac1` |
| `workplace/hd-commanders-v16-build-receipt.json` | `322731b498cacb37055be397ab067d9881431fde943cb24ff5c42c96225aa256` |
| `workplace/hd-assets-portraits-v16/manifest.json` | `5ee3ef63de9276a132ebe816d25afc62cf407d36bc7e21d6025016c20dce5742` |
| `workplace/hd-assets-portraits-v16/preparation.json` | `66ebd2f3c719d0eb2f5bc857cfc44db13873894c974a6ca85f63875875cc2561` |
| `workplace/hd-portrait-audit-v16.json` | `bae8006bad81b33ae2cf29fc7ed53d5cc26ed159ab855459429bb7cb1c7877ff` |
| `workplace/hd-scene-audit-v16.json` | `884478b8ab902bc67538a3d139ae4d7b6484ca2d4d2b6a8c244a0f0f678f7a69` |
| `workplace/hd-v16-audit-commands.json` | `f448653fb0e88aa6bc3d521e81f2c4ae757eb4d9c2cae123975f1ee3bb67c137` |
| `workplace/hd-v16-contact-receipt.json` | `f14fcc2dab43316fffd2739f134a8f356665801eb4687c42095d27ec3a49946f` |
| `workplace/hd-preview/commanders-v16-contact-1.png` | `c86bc0e311e9df467bcd6d9e99f941676bff81529c1edba3c60f076f81cb17c6` |
| `workplace/hd-v16-next-plan-verification.json` | `d9728ff0a23257c6492cc5ade5dc429ebdcd9abaecad936d3b99816d496c0d3f` |
| `workplace/verify-hd-v16-next-plan.py` | `695bc760f197f167234feb591cf1aebbf639134f9fe74ac01b61b9cc0482fbbd` |
| `workplace/verify-hd-v16-delivery.py` | `407ca9976dfd3f6af1a480aae2b2598d0ff1cb0f049e4b68a4eb4b8d40bb2097` |
| `workplace/hd-window/player/commanders-v16/receipt.json` | `5b8f2bc751ac8f6c2076d2700f6eb15edfdf92e6da2606e9fd679bf79d202829` |
| `workplace/hd-v16-delivery-verification.json` | `13a3359b8b22616b970d2f3a85cdd2a291823a4b3655cb3b06de0f51244901b6` |
| `workplace/hd-v16-gui-contact-receipt.json` | `c4dc3f20bb71357189fdc481037c8977208e60bfd83205b1b1c7dffadd64178f` |
| `workplace/make-hd-v16-gui-contact.py` | `2adbe6ab3936b449875bb28049fc4ca1296c25d1d687e7d4c5dfc265a4f1771c` |
| `workplace/hd-preview/commanders-v16-gui-base-contact-1.png` | `684b2b7548c8fb047c814d19d937476849aa01f571ab5fede4146ef64494ab05` |
| `workplace/hd-preview/commanders-v16-gui-plus-contact-1.png` | `be53d7d357a6d669868714505273aaa46bcd7a5ea6e18eba8d704cb99d3e4b4f` |
| `workplace/verify-hd-v16-docs.py` | `3a559f93169dcc724e410334c734b46672ca94c477c1224542a18c09412d3e8d` |
| `workplace/verify-hd-v16-publish.py` | `bc343ab646f57dd60cdd1a7aac5ea374d2e65a0cd23d58b186dd4d3f38918457` |

### 6.28 劇本 006 新增六張肖像

**狀態：READY**。沿用 §6.15／6.27 的 B 寫實手繪、4×、原貌預設與固定肖像契約。從 v16 剩餘清單選取六槽，已直接回讀兩版 DATA3 原始槽及 64×80 PNG。原始資料為 L0、[both]；本次來源準備未重跑玩家流程。

兩版均為 006、單人曹操、難度 5，220 年元月停在郡 5；每版本國三位、他國三位。十二列清單的分版選項相同，均在第一頁。姓名採遊戲資料的「曹叡」「傅彤」。

| 肖像槽 | 人物 | 人物索引 | 查看郡 | 清單選項 | 他國確認 | 原圖特徵 |
|---|---|---|---|---|---|---|
| F040 | 曹叡 | 265 | 16 | 1 | 否 | 朝左雙眼、高黑冠、無鬚、閉口、藍衣灰黑領 |
| F149 | 張翼 | 225 | 18 | 4 | 是 | 雙眼怒眉、藍青飾盔、無鬚、開口上齒、紅衣 |
| F035 | 諸葛恪 | 278 | 21 | 1 | 是 | 朝左雙眼、高黑冠青紋帽帶、黑髭尖黑鬚、閉口、青衣淺黃領 |
| F016 | 鄧艾 | 283 | 27 | 2 | 否 | 雙眼、紅盔綠方飾及紅側邊、無鬚、開口上齒、綠衣黃綠領 |
| F200 | 程武 | 270 | 28 | 3 | 否 | 朝左雙眼與小幅遠側眼、藍青折冠、細髭短尖鬚、閉口、藍青衣淺領 |
| F034 | 傅彤 | 246 | 39 | 2 | 是 | 朝左單眼側臉、藍青條紋盔與側邊、無鬚、閉口、藍青衣綠背景 |

來源入口 `workplace/prepare-hd-commanders-v17-plan.py`；計畫 `workplace/hd-commanders-v17-plan.json` 的 SHA-256 為 `10b4ea550a0ccd9c23559692869c5321adace8261aa10a84ff43706f4bf1f1fe`，回讀收據 `workplace/hd-v17-plan-verification.json` 為 `0dd0d8c0d5f34b7da25b3b298549ae6e239f10bf7c1d8dbaa687464070794b22`。六張最近鄰 16× 目標已查看，人物識別以各槽原圖為準。生成請求、來源觀察、實際路徑與審查依序記於 `workplace/hd-commanders-v17-requests.json`、`workplace/hd-commanders-v17-source-observations.json`、`workplace/hd-commanders-v17-generated-paths.json`、`workplace/hd-commanders-v17-reviews.json`。初版僅輸入各人物原圖，文字沿用 B 畫風。完整 4:5 圖縮放至 256×320，不裁切、不烘焙文字。

所有候選與最終來源／高清總覽查看後才轉 READY。私人 v17 包須保留 v16 全部 536 筆欄位、PNG bytes 與準備紀錄。正常 GUI 須從兩版片頭開上述新局，以正式清單查看十二張人物卡，核對原貌來源、原生高清、圖外文字框線及原貌恢復。不注入人物、日期、勢力或 seed。本批不改規則、存檔或音訊。整份 HD 仍為 READY；跨平台與使用者逐張簽核尚未完成。

七份實際候選及最終來源／高清總覽均已查看，六張採用、一張退件保留。傅彤採第二版，其餘採第一版；首版鼻樑前的遠側眉尖已局部移除。使用者逐張簽核零。全部候選為 1122×1402，不透明且符合既有 4:5 誤差契約；內建 image_gen 未回報模型與 seed。生成彙整 `workplace/hd-commanders-v17-generation.json` 的 SHA-256 為 `b2cd0f6ab917f1c3d83056903839d0a53eb9082baedc76dfb246d8c3ff8effdf`，最終總覽 `workplace/hd-preview/commanders-v17-contact-1.png` 為 `39f40e1655869c00bff6130ae7aa84d6551e5fb638d8d43552fa46d7148fee0e`。來源與完整請求不公開。

本批 READY 解鎖私人 v17 包。生成彙整、比較頁與建包入口依序為 `workplace/collect-hd-commanders-v17.py`、`workplace/make-hd-v17-contact.py`、`workplace/build-hd-commanders-v17.py`；正常 GUI 入口為 `bash tools/verify-hd-player.sh --officers-006`。獨立回讀、兩版人物卡總覽、餘槽診斷、文件及公開文字檢查入口依序為 `workplace/verify-hd-v17-delivery.py`、`workplace/make-hd-v17-gui-contact.py`、`workplace/verify-hd-v17-next-plan.py`、`workplace/verify-hd-v17-docs.py`、`workplace/verify-hd-v17-publish.py`。建包與 GUI 結果見下列收據，整份 HD 保持 READY。

私人 v17 包備妥 243/256 肖像與 31/31 SCG，兩版各 274 筆、共 548 筆；正式載入器無警告，v16 全部 536 筆欄位及 PNG bytes 保持。全族稽核技術問題與使用者逐張簽核均為零；場景完整準備閘門返回 0，肖像因缺十三槽返回 3。準備紀錄保持亦經獨立回讀驗證。

從已驗 v12 正式六劇本診斷排除 v17 已備槽，十三個剩餘槽均未在初次清單找到，分版選項差異零；這步未重跑玩家流程。下一步核對正式出現與尋訪入口，不為取得畫面修改規則或注入人物。其他素材家族、使用端、三語系、平台與使用者逐張簽核仍待完成。

| 本機產物 | SHA-256 |
|---|---|
| `workplace/build-hd-commanders-v17.py` | `63f6218b616135411962e55625c6efcc162e093a0089b3eae3516036041c5e05` |
| `workplace/collect-hd-commanders-v17.py` | `b717e296a598122cbb3b47e36d9bcb59737eb1751ec849b8b7a3103859c1de21` |
| `workplace/hd-assets-portraits-v17/manifest.json` | `2a1b7b0ab0755dc5418766826e7367d9f3599ea010cae74f04ba04b0e5f80f02` |
| `workplace/hd-assets-portraits-v17/preparation.json` | `17588a11d1e8e011bf9d8883ce7640d720c94837e2a027ef8b986e3a8ead3f87` |
| `workplace/hd-commanders-v17-build-receipt.json` | `fdb3c238c03c7f9887af6892f23444e0b23dded7ef019ef364b80ee7f2e8ab6d` |
| `workplace/hd-commanders-v17-generated-paths.json` | `c15f22e0b1d37b5100e1275592fcfd4a05edfb79bed205ef96703c94647a621d` |
| `workplace/hd-commanders-v17-generation.json` | `b2cd0f6ab917f1c3d83056903839d0a53eb9082baedc76dfb246d8c3ff8effdf` |
| `workplace/hd-commanders-v17-plan.json` | `10b4ea550a0ccd9c23559692869c5321adace8261aa10a84ff43706f4bf1f1fe` |
| `workplace/hd-commanders-v17-requests.json` | `bd41544157181e60dff5d09860482048efced7bdc709f3a1defa99fd62ab2b46` |
| `workplace/hd-commanders-v17-reviews.json` | `a646aaddd565ead4a1f5fe765f5e18bd3196623a18916c8b19ce285d9c700d0f` |
| `workplace/hd-commanders-v17-source-observations.json` | `27e6a6d0ac1417343d65c985af1836cbc4cb9cc3fa43a3d9fa2c1992a8c88d5f` |
| `workplace/hd-portrait-audit-v17.json` | `29edad8d4134a107359a00f2ab7fafc7a6dff0a989783f93d725ca2414354ece` |
| `workplace/hd-preview/commanders-v17-contact-1.png` | `39f40e1655869c00bff6130ae7aa84d6551e5fb638d8d43552fa46d7148fee0e` |
| `workplace/hd-scene-audit-v17.json` | `bee46a5af90c59661d77e3d3452c79bf3e9de6c68bf0b93762edd7119d6991a4` |
| `workplace/hd-v17-audit-commands.json` | `37c4a67f9aa4720be83b2cbce80134321d95fa28a1f02b46785f180bc2c79872` |
| `workplace/hd-v17-contact-receipt.json` | `25b1a39eff133da90098df5bd6b8e6f404aa4cd7b153e9da0585bb2cfb948dbc` |
| `workplace/hd-v17-next-plan-verification.json` | `38d46b74dc4b95a660fae3ab91d18cd877dc1f14cbe92d2fc3826a99c8ad5cc3` |
| `workplace/hd-v17-plan-verification.json` | `0dd0d8c0d5f34b7da25b3b298549ae6e239f10bf7c1d8dbaa687464070794b22` |
| `workplace/make-hd-v17-contact.py` | `9cc2c67c54ba2e4e3c4977969a2bf16a8281d0fddf1caeed1912c123832c87fd` |
| `workplace/make-hd-v17-gui-contact.py` | `470fe3f90638420fe711ab40deb611401fc4c1bd7babb8675dd0f5425b4a87d9` |
| `workplace/prepare-hd-commanders-v17-plan.py` | `f595ca4023aa7081ed382b76d29faecef28128fb92cef654365161ead61f7b29` |
| `workplace/verify-hd-v17-delivery.py` | `9532f82d7170187ec42854acedfb9e29234ed5c3b112086eafa71561e6afcf2c` |
| `workplace/verify-hd-v17-docs.py` | `974dc0e983d81f0d0fcb78978ccebc85de21145315a67b97d31119df108b4880` |
| `workplace/verify-hd-v17-next-plan.py` | `dd9d98d2497c5dc53715f83352b261cdf033bad33ab2c75aec41934cc38cc62e` |
| `workplace/verify-hd-v17-publish.py` | `442d7e67ebd94b20dc3d3c319a6f5271a693b1e0faac7bbf5affcb69b7687058` |
| `workplace/hd-v17-delivery-verification.json` | `ab0370b460b8d0efdf7138e9366af0923a9f23437f4854468b0c9c5d29d8c4a0` |
| `workplace/hd-v17-gui-contact-receipt.json` | `0643d2faa8d5394c4a6cf959759d62f1b19aaa4512765873b515487bb55428a3` |
| `workplace/hd-window/player/commanders-v17/receipt.json` | `8d56eec27ff74d4352ee4797e8e8ad9780ac25fe1463f30652395f96319db233` |
| `workplace/hd-preview/commanders-v17-gui-base-contact-1.png` | `b2bfe79ed1bc2170d6d0a53a1a9e325f0befdf20fd81be896e0e1ab3fc7fa596` |
| `workplace/hd-preview/commanders-v17-gui-plus-contact-1.png` | `a469f0add3b9198231e2142afda319c7892c215ba15b3a6bc605b7dcf75efdfa` |

`bash tools/verify-hd-player.sh --officers-006` 在 Docker 內從兩版片頭各開 006 曹操、難度 5 正常新局。十二張人物卡的原貌來源、256×320 原生高清、圖外文字框線及原貌恢復各 12/12，另兩次啟動的原貌及隱藏列共 50/50 通過，89 張最新 PNG。未注入人物、日期、勢力或 seed；屬 remake 正常 GUI 驗證，不是新美術與原版的像素 parity。

獨立回讀全部 89 張最新 PNG、十二張人物卡、七份候選的完整請求與實際輸出、實際參考雜湊、不透明像素、兩版原始 DATA3 容器與 548 筆素材；v16 全部 536 筆欄位、PNG bytes 與準備紀錄保持。兩版人物卡總覽均已查看，使用者逐張簽核仍為零。

### 6.29 最後十三張固定肖像

郡名勘誤：本批初次工具與文件曾把郡 18 寫成漢中。兩版原始 DATA2.GRP 檔案位移 821,227 的四個 bytes `a4 d1 a4 f4`，依 cp950 均為天水，與正常尋訪畫面一致，L0、[both]；見 [州郡表](../formats/02-data2-prefecture-table.md#3-42-個郡名)。來源回讀收據為 `workplace/hd-v18-location-correction.json`；先前 GUI 收據另保留為 `workplace/hd-v18-gui-location-first.json`。修正地名描述後依相同正常操作重新生收據，不改郡編號、人物、遊戲狀態或規則。

初次獨立回讀與公開文字收據另存 `workplace/hd-v18-delivery-location-first.json`、`workplace/hd-v18-publish-first-check.json`，保留地名勘誤前的驗證範圍。

**狀態：READY**。沿用 §6.28 的 B 寫實手繪、4×、每次啟動原貌及固定肖像契約。本批補齊全 256 槽的最後十三槽，不以初次人物清單是否可見改變素材分母。直接回讀兩版 DATA3.GRP／IDX／NAM、原槽 bytes 與 64×80 PNG，分版來源相同，L0、[both]。來源回讀收據 `workplace/hd-v18-plan-verification.json` 的 SHA-256 為 `d73f102e22a05260870385ac2efecdac77cbdf33ca881a079e2966b2c6f54a83`，二十六列計畫 `workplace/hd-commanders-v18-plan.json` 為 `e74a6deb2a0248029b93de9e872359e281e363341c91066a5b42f99c2f6307d6`。

| 肖像槽 | 遊戲資料姓名 | 原圖識別重點 |
|---|---|---|
| F003 | 未對應人物槽 | 近正面雙眼、紅色包頭黃邊、濃黑髭與濃黑鬚、青藍衣 |
| F019 | 姜維 | 朝左雙眼及窄遠眼、青藍紋盔、短黑髭與無下巴鬚、閉口、青藍衣 |
| F026 | 鍾會 | 朝左雙眼、紫紅褶帽與紅黃前飾、黑髭尖黑鬚、青紫衣 |
| F037 | 司馬師 | 朝左單眼側臉、紅盔青方飾金邊、黑髭長尖黑鬚、青衣 |
| F075 | 傅巽 | 朝左雙眼、高黑帽與紅帽帶及側垂帶、無髭鬚、綠衣白領 |
| F084 | 蔣欽／文虎 | 朝左單眼側臉、青藍盔與前飾及後紋、黑髭長黑鬚、微張口小淺色齒區 |
| F086 | 彭羕 | 朝左雙眼、露額黑髮及右上青飾、黑髭長尖黑鬚、青衣白領 |
| F095 | 秦宓 | 近正面雙眼、黑帽紅髻、黑髭尖黑鬚、微張口上齒、紅衣 |
| F203 | 諸葛誕 | 朝左雙眼及窄遠眼、灰紋折帽、黑髭尖黑鬚、黑衣淺領 |
| F208 | 司馬昭／陳孫 | 略朝左雙眼微垂、黑髮及青紋後飾、細黑髭與無下巴鬚、青衣白領 |
| F212 | 鍾毓 | 朝左單眼側臉、紅盔綠方飾、黑髭尖黑鬚、微張口上齒、紅金衣 |
| F245 | 文鴛 | 強烈朝左雙眼及窄遠眼、青藍紋盔、黑髭尖黑鬚、紅黑衣黃領 |
| F253 | 劉禪 | 朝左單眼側臉、露額黑髮及右上白紅髮飾、無鬚、紅衣白領 |

F003 在六劇本人物表沒有引用，不猜人物姓名。F084 與 F208 各由兩人共用；保持原固定肖像，不按劇本年齡另畫。正式初始狀態診斷 `workplace/hd-portrait-hidden-state-v18.json` 的兩版六劇本共 168 列，姜維在 006 為天水隱藏人物，其餘有引用槽均為未登場狀態。此為 remake 初始停點診斷，不是正常 GUI 或新增原版 oracle。

來源準備入口 `workplace/prepare-hd-commanders-v18-plan.py`；狀態診斷入口 `workplace/hd-portrait-hidden-plan-v18.go`。原圖及最近鄰 16× 目標全部查看，完整請求、原圖觀察、實際生成路徑與審查保存於 `workplace/hd-commanders-v18-requests.json`、`workplace/hd-commanders-v18-source-observations.json`、`workplace/hd-commanders-v18-generated-paths.json`、`workplace/hd-commanders-v18-reviews.json`。使用內建 image_gen，初版只輸入該人物原圖；B 畫風沿革參考 §6.28，不另輸入他人肖像。完整 4:5 圖縮放至 256×320，不裁切或烘焙文字。候選與最終來源／高清比較頁全部查看後才轉 READY 並建立私人 v18 包。

私人包須保留 v17 全部 548 筆欄位、PNG bytes 與準備紀錄，正式載入器及兩族完整準備稽核須通過。正常 GUI 抽驗循 006 劉備的正式新局與尋訪入口確認姜維，不注入人物、日期、勢力或亂數，不宣稱十三槽均有正常 GUI 收據。本批不改規則、存檔或音訊。其他素材家族、使用端、三語系、平台與使用者逐張簽核仍待完成。

候選彙整、來源比較頁與私人包重建入口依序為 `workplace/collect-hd-commanders-v18.py`、`workplace/make-hd-v18-contact.py`、`workplace/build-hd-commanders-v18.py`。建包程式只允許本節 READY 後執行。

原圖辨識勘誤保存於 `workplace/hd-commanders-v18-source-corrections.json`；五張口部複驗圖為 `workplace/hd-preview/v18-beard-source-review.png`。初始觀察、已執行請求及退件不重寫。

二十三份實際候選及最終原圖比較頁均已查看，十三張採用、十張退件保留。姜維、司馬師、傅巽、蔣欽共用槽與劉禪採第二版，司馬昭共用槽採第三版，鍾毓採第四版，其餘採第一版。全部候選為 1122×1402、不透明且符合既有 4:5 誤差契約，使用者逐張簽核零。生成彙整 `workplace/hd-commanders-v18-generation.json` 的 SHA-256 為 `69012f42152e2378b8380aa3128ed4a8b4dbb04d5f59ac4fe80234c631fbaf43`；最終原圖比較頁 `workplace/hd-preview/commanders-v18-contact-final-1.png` 為 `78cf1cdb707c1d22bd817f32884febf62ae869c2f6f6c949c1d54e202018fd19`。READY 解鎖私人 v18 包；下列正式載入、正常尋訪與獨立回讀均已完成。

正常尋訪輸入的診斷入口為 `workplace/hd-v18-search-plan.go`，輸出 `workplace/hd-v18-search-plan.json`。沿正式選單開 006 劉備、難度 5，依正常休息流程輪到天水，再選本地謀略最高者尋訪。天水屬勢力 0 劉備，空郡哨兵為 255，不能作自創君主起點。正式新局的預設亂數不覆寫；此診斷先確認 GUI 等價輸入，不增加原版 parity 聲明。

正常 GUI 重跑入口 `bash tools/verify-hd-hidden.sh`，容器內控制器為 `tools/verify-hd-hidden-inner.py`，收據位於 `workplace/hd-window/player/hidden-v18/receipt.json`。兩版各休息郡 36、32、39、37、38、30，再於天水以謀略排序第一位馬良尋訪；核對姜維來源、原生高清、圖外文字框線及原貌恢復。

獨立回讀入口為 `workplace/verify-hd-v18-delivery.py`，在 `eob-audio-capture:20260922-r2` Docker 執行，收據為 `workplace/hd-v18-delivery-verification.json`。回讀完整素材包、前版保留紀錄、實際候選與參考雜湊、原始容器、兩版正常尋訪及全部最新截圖；與建包程式分開檢查。

私人 v18 包為 256/256 肖像、31/31 SCG，兩版各 287 筆、共 574 筆及 287 張獨立 PNG。正式載入器兩版無警告，兩族完整準備閘門均返回 0。v17 全部 548 筆欄位、PNG bytes 與準備紀錄保持；場景未改。兩族技術問題零，使用者逐張簽核零。

兩版正常尋訪共 10/10 通過，全部 36 張最新截圖及兩張原生高清畫面已獨立回讀，高清畫面亦已查看。各版姜維的原貌來源、256×320 原生高清、圖外文字框線及原貌恢復均通過；不注入狀態或覆寫 seed。軍師勸諫後依實際「主公是否繼續」送 Y，辨識用原貌參考為 `workplace/hd-v18-continue-reference.png`；第一次未送確認的失敗收據保留。本批未重跑原版 oracle 或音訊，既有範圍保持。

獨立回讀核對二十三份完整執行請求、實際原始輸出、參考雜湊、不透明像素、兩版 DATA3 原始容器、完整素材包、執行工具快照與正常尋訪全部最新 PNG，結果相符。完整產物索引為 `workplace/hd-v18-artifact-index.json`。文件檢查入口與收據為 `workplace/verify-hd-v18-docs.py`、`workplace/hd-v18-docs-verification.json`；公開文字檢查入口與收據為 `workplace/verify-hd-v18-publish.py`、`workplace/hd-v18-publish-check.json`，Issue 檢查另存 `workplace/verify-hd-v18-issue-publish.py`、`workplace/hd-v18-issue-publish-check.json`。各工具只在本節記錄的隔離 Docker 執行；範圍與結果另記 WORKLOG。

| 本機產物或工具 | SHA-256 |
|---|---|
| `workplace/hd-commanders-v18-plan.json` | `e74a6deb2a0248029b93de9e872359e281e363341c91066a5b42f99c2f6307d6` |
| `workplace/hd-v18-plan-verification.json` | `d73f102e22a05260870385ac2efecdac77cbdf33ca881a079e2966b2c6f54a83` |
| `workplace/prepare-hd-commanders-v18-plan.py` | `e88dafbc8b0994131d54920ee5bd4a50f121ead4dcc3ac36404a7daae4860203` |
| `workplace/hd-portrait-hidden-plan-v18.go` | `5e9f3cbad87f0583a957e8505728929d823f8e47bc3cc23eb285be94c2e56b01` |
| `workplace/hd-portrait-hidden-state-v18.json` | `d9bb6abdd4e9cca4b69d3fdbfcaee32efaa941fc45db37cf89c4436e2b185d38` |
| `workplace/hd-commanders-v18-requests.json` | `64002bd40c7c9ade9d0df6cb9f51354c628a99f43821bd3d9b34efdf7da5c7b5` |
| `workplace/hd-commanders-v18-source-observations.json` | `aa277a402589d80004b3979ea30a419d22941f6553c55238b8e5b17913ff26ad` |
| `workplace/hd-commanders-v18-source-corrections.json` | `28c3f354ef93a744636a8aa0be50de7ef38ae699b5727495746221a432c8c363` |
| `workplace/hd-preview/v18-beard-source-review.png` | `56f0fc8b074cfa01f95c4e2b19ba2e194904257ca1914ab183e3302ef8bd2634` |
| `workplace/hd-commanders-v18-generated-paths.json` | `79bc6a3a2508ff80f38d67031ee92ffd13c00a00266b12c4a737de879e5b6179` |
| `workplace/hd-commanders-v18-reviews.json` | `608adcfffdc40101403edf1e72981327fbe877280443f5b6094b8153ef9a7877` |
| `workplace/hd-commanders-v18-generation.json` | `69012f42152e2378b8380aa3128ed4a8b4dbb04d5f59ac4fe80234c631fbaf43` |
| `workplace/collect-hd-commanders-v18.py` | `ec022caf417b59c03cd1f7a5ffeb8e1f96c14f95e1f66411ff5d5bf4a6ea5626` |
| `workplace/make-hd-v18-contact.py` | `6322fb95dac787cc2702f740fa13d42e334ca70eae72506c7aeb2fc2b35e5ac2` |
| `workplace/build-hd-commanders-v18.py` | `9aaf360648043c69b3334920e6719911737bbe8923084e245e3ded20c415f799` |
| `workplace/hd-assets-portraits-v18/manifest.json` | `5fdc94365df56068f583a89df00f4fc114d9feaff3631b6ae4c52f597c74d241` |
| `workplace/hd-assets-portraits-v18/preparation.json` | `653f7c390041b21b7dac3ce22550d257d8fb6d679febad960c07b8fe4b5d554f` |
| `workplace/hd-commanders-v18-build-receipt.json` | `a3628ac6564b3c349a75a2895e4cad363c65867b47320573958425c9adc2f2b3` |
| `workplace/hd-portrait-audit-v18.json` | `e29adcac3d200ac959d04031bba5c2de258fa23d911ed76eebc7388e912e8095` |
| `workplace/hd-scene-audit-v18.json` | `f3890579485cf8655393b93126306aece801ac2c66cb415bad23ff68a79d8a1f` |
| `workplace/hd-v18-audit-commands.json` | `76da692d754b6e8516bf879787a7cdd3fafce62746af9451bfdaa673407af01e` |
| `workplace/hd-preview/commanders-v18-contact-final-1.png` | `78cf1cdb707c1d22bd817f32884febf62ae869c2f6f6c949c1d54e202018fd19` |
| `workplace/hd-v18-contact-receipt.json` | `692effb8ee3f470e3ec3b29723cedf06345cce088bf03c1ae32c47c30043045e` |
| `workplace/hd-v18-search-plan.go` | `f17e1c545ebeac3e5ba21af93ce5cb5cc4897e1718d6a51016309414299c980c` |
| `workplace/hd-v18-search-plan.json` | `a377c1c42926f578ed6844dae1eb63bb313872b49d6a1a017be60192a82c2b0a` |
| `workplace/hd-v18-continue-reference.png` | `6621bdec13742de2acbc1b677536c4f3227a1f8b58155cc4fad587069cab3d34` |
| `workplace/hd-v18-gui-first-attempt.json` | `bdc590cfff0037e21ae830e058b65b7e339a1db86f0fb1def4bf3f08c4e47bc5` |
| `workplace/hd-window/player/hidden-v18/receipt.json` | `ccb370f8937f96f480861ca4c8ec4dab8ee9675881af413a5311d73610cb6170` |
| `workplace/verify-hd-v18-delivery.py` | `4dd34f1db24372803be62b14bb8346a088aabd83eba48bd526c1a2a028a0d168` |
| `tools/verify-hd-hidden.sh` | `548c3ed3664e41ad9f3eabaf845fb9220ae886a5b2556ce69e3d2dd6e10b8601` |
| `tools/verify-hd-hidden-inner.py` | `5a6efe73b3b5b816f2049433017817435d730eacc188d93e495b1c2a5a09c8bb` |
| `workplace/hd-v18-delivery-verification.json` | `41caceb286670dfdf1470c4bc98bc5249fa51cfb48a3bbc2d7db4a8c304e3735` |
| `workplace/hd-v18-location-correction.json` | `c6a287522e622c627e819d59d113f932aea8e725ba254a891de1151ff39d0c19` |
| `workplace/hd-v18-gui-location-first.json` | `19c5f51e8137ad4c56fd77585c87f905ce55b1a6402cf37f0050f5012943c56f` |
| `workplace/hd-v18-delivery-location-first.json` | `8e2feecad58d093cf8323c4efdf85fea568fa27a23e30a02ca1c2537c8b09fb7` |
| `workplace/hd-v18-artifact-index.json` | `1411b2e1f0dc8107020692d09e06658df116d9ce7707cc604782ee218cf12a30` |

### 6.30 三種天候圖示

**狀態：READY**。沿用 B 寫實手繪、4× 及每次啟動原貌，補齊戰場左欄的晴、雨、風圖示。本批不改天候判定、戰術條件、索引、日期或存檔。原版與加強版的三個 DATA1 槽來源 bytes 相同，32×32 不透明，L0、[both]；完整來源見 [盤點](../formats/04-asset-inventory.md#6-hd-兩版盤點)。

| 資源鍵 | 圖示語意 | 原圖／高清尺寸 | 原始槽 SHA-256 |
|---|---|---|---|
| DATA1/WEATHER0.IMG | 晴 | 32×32／128×128 | `48dec119d31511402fc579212773823dc2c884a51d64a5ea3b99f15ce792b01b` |
| DATA1/WEATHER1.IMG | 雨 | 32×32／128×128 | `22aebb469db88b9106e0e8ef6df3319075a705b32b7701df01e6eed9c5c79652` |
| DATA1/WEATHER2.IMG | 風 | 32×32／128×128 | `8c5a28d81bf05316857b0016e8d9256ff9af403c5f66b07be10f83b6f101e63d` |

直接回讀 DATA1.NAM／IDX／GRP 與既有 PNG，來源準備入口為 `workplace/prepare-hd-weather-v19.py`，計畫及收據為 `workplace/hd-weather-v19-plan.json`、`workplace/hd-weather-v19-source-verification.json`；最近鄰預覽為 `workplace/hd-preview/v19-source-WEATHER0-16x.png`、`workplace/hd-preview/v19-source-WEATHER1-16x.png`、`workplace/hd-preview/v19-source-WEATHER2-16x.png`。原圖與預覽均須目視後才生成，候選保存於 `workplace/hd-weather-v19-requests.json`、`workplace/hd-weather-v19-generated-paths.json`、`workplace/hd-weather-v19-reviews.json`、`workplace/hd-weather-v19-generation.json`。工具未回報模型或 seed 時明示，不推定使用者逐張簽核。

圖像解碼使用既有 `assets.DecodeImage` 的四個 I／R／G／B 位元平面，不當成雙像素打包格式。正式 parser 回讀入口為 `workplace/verify-hd-weather-v19-source.go`，收據為 `workplace/hd-weather-v19-decoder-verification.json`；逐像素核對已匯出 PNG，避免只憑檔案大小判定。

候選彙整與來源比較入口為 `workplace/collect-hd-weather-v19.py`、`workplace/make-hd-weather-v19-contact.py`，比較頁為 `workplace/hd-preview/weather-v19-contact.png`。通過後才建立 `workplace/hd-assets-weather-v19/` 私人包，重建入口與收據為 `workplace/build-hd-weather-v19.py`、`workplace/hd-weather-v19-build-receipt.json`。

比較頁收據為 `workplace/hd-weather-v19-contact-receipt.json`；三組均為左原圖、右採用候選的 128×128 全圖縮放，順序為晴、雨、風。候選本體採 `workplace/hd-b-WEATHER0-v1.png`、`workplace/hd-b-WEATHER1-v1.png`、`workplace/hd-b-WEATHER2-v1.png` 等版次保存，完整執行請求與實際路徑另列於生成紀錄。

現行天候索引為 `battle.Weather.OriginalIndex()` 的晴 0、雨 1、風 2；不直接用 remake 列舉值。主戰場、窄戰場與對戰子畫面均由 `DrawArtBattle` 在 (8,155) 畫 32×32 圖示。高清只於相同矩形疊 128×128 完整圖，文字、時刻、地形、兵力牌與其他框線保留覆蓋權；原貌 CPU 畫布不改。索引與原版位置依 [主戰場規格](005-main-screen.md)，實際入口為 `internal/ui/artbattle.go`、`internal/assets/battlescreen.go` 與 `internal/battle/battle.go`。

候選及原圖比較通過後才轉 READY，允許正式載入器接受 DATA1 的 WEATHER0–2，核對實際來源存在、32×32、4× 尺寸與來源／輸出雜湊。沿用 schema 1，雙版上限增加六筆至 580，不開放其他未審查鍵；缺 DATA1、缺圖、錯槽、錯尺寸或錯來源逐項回退。私人 v19 包須保留 v18 全部 574 筆欄位、PNG bytes 與準備紀錄。

來源的 6 個槽及 6,144 像素核對通過。三張 1254×1254 候選與原圖、128×128 全圖縮放比較頁均已查看，Codex 採用，使用者逐張簽核為 0。生成紀錄 SHA-256 為 `31b0807474f5255b46c154d197888f55f3c6b8586767c471c9ce92f443784d49`，比較頁為 `457ec8ee1bb2437078a03d77ca97df4ccf473536b71a1839812bf6df32aab1fd`。來源、構圖與尺寸契約已足以授權正式接入。

驗收分列來源／候選、載入器正反例、三種天候的正式圖層，以及兩版正常玩家戰場。正常 GUI 不注入天候、人物、戰場或 seed；只宣稱實際抵達的天候與分支，不以圖層檢查替代正常流程。正式接入、三家族完整準備稽核及兩版正常晴天 GUI 已通過；正常雨、風尚未由本批驗證，不新增原版天候規則 parity。

正式驗證沿用 `internal/ui/hd_test.go`、`tools/hd-portrait-audit.py --family weather` 與 `tools/verify-hd-battle-branches.sh`。GUI 使用 `SAN1_HD_BRANCHES_PACK=workplace/hd-assets-weather-v19`、`SAN1_HD_BRANCHES_OUT=workplace/hd-window/player/weather-v19`，收據為該目錄的 `receipt.json`。載入器回讀、三家族完整稽核、CLI 正反例與獨立交付回讀分別由 `workplace/verify-hd-weather-v19-pack.go`、`workplace/audit-hd-weather-v19.py`、`workplace/test-hd-weather-v19-audit.py`、`workplace/verify-hd-weather-v19-delivery.py` 執行；同名 JSON 保存命令、結果與雜湊。

UI 測試結果另存 `workplace/hd-weather-v19-tests.json`。CLI 正反例只在 `workplace/` 建立短期 fixture，完成後自動移除。三家族報告為 `workplace/hd-portraits-audit-v19.json`、`workplace/hd-scenes-audit-v19.json`、`workplace/hd-weather-audit-v19.json`。

文件檢查入口與收據為 `workplace/verify-hd-weather-v19-docs.py`、`workplace/hd-weather-v19-docs-verification.json`。公開檢查沿用完整 PNG、完整提示詞、MZ 前綴與正反對照，只掃本輪實際暫存文字或五份 Issue 本文，入口為 `workplace/verify-hd-weather-v19-publish.py`，收據依序為 `workplace/hd-weather-v19-publish-check.json`、`workplace/hd-weather-v19-issue-publish-check.json`、`workplace/hd-weather-v19-checkpoint-publish-check.json`。Issue 計畫、本文及回讀為 `workplace/hd-weather-v19-issue-plan.json`、`workplace/hd-weather-v19-issue-{104,107,108,109,110}-body.md`、`workplace/hd-weather-v19-issue-sync.json`。

正式載入器兩版各 290 筆、無警告；三家族備妥數為 256/256、31/31、3/3，Codex 已審查，使用者逐張簽核均為 0。天候 CLI 正反例 14/14、正式圖層 12/12 組與完整 UI 套件通過。正常兩版 001 董卓、難度 5，依序休息郡 6、16 後，洛陽呂布攻陳留，合法紮寨、對戰、查看、續行及快戰 173/173，14 個停點皆為晴。子畫面時刻推進，快戰結算後返回，原版釋放一名俘虜、加強版無俘虜；未注入狀態或改寫 seed。兩版高清畫面已查看。189 張最新 GUI PNG、全部 580 筆登錄、290 張 PNG、三張實際生成檔與 v18 保留紀錄獨立回讀通過。

獨立交付收據 SHA-256 為 `e4d9d91eb13e0f2cf004b0d55f11e25bc6d614c0859ee7b46f7287f7fb8544af`。本批不外推正常雨、風、其他素材家族或平台，不新增原版 oracle、音畫或人耳驗收。

| 產物或入口 | SHA-256 |
|---|---|
| `workplace/hd-weather-v19-plan.json` | `6c525e649082f5e31542fe1c4e7d2963c89affc9a97cc217533215ca7573bec7` |
| `workplace/hd-weather-v19-source-verification.json` | `230210121a21b83e79c200082924d2d6a552755903b99334b5f934f95fd8ad04` |
| `workplace/hd-weather-v19-decoder-verification.json` | `27c1c55b7a183027a84e0087e8ecd2ec000a03056c30798b4ab451c59409d77e` |
| `workplace/hd-weather-v19-requests.json` | `c751ac4d1bbce8e417a8dcc993a3026171ab238cdb35d0628fe7ce98f4b6a70f` |
| `workplace/hd-weather-v19-generated-paths.json` | `47b362e255d3f8f9bad964331da992140672bcc9256a8272ec7b7a34e7c23be6` |
| `workplace/hd-weather-v19-reviews.json` | `ec1a3efc3e6590c761ff2cfd1653aba268108664a669a67174df7289a8516159` |
| `workplace/hd-weather-v19-generation.json` | `31b0807474f5255b46c154d197888f55f3c6b8586767c471c9ce92f443784d49` |
| `workplace/hd-weather-v19-contact-receipt.json` | `a376415ef58b4c57001cc8fca97933b2dcb51093d636240a6a30c97ee06aa43e` |
| `workplace/hd-preview/weather-v19-contact.png` | `457ec8ee1bb2437078a03d77ca97df4ccf473536b71a1839812bf6df32aab1fd` |
| `workplace/hd-assets-weather-v19/manifest.json` | `ed2a7ccad8457606c894fe93b5f475d3bacc365edd081896ecff75d7ce0994d9` |
| `workplace/hd-assets-weather-v19/preparation.json` | `dca0a61314c2bdb1993a7ca8ff3c78f3ee874b07546fcec82134034b437d8c93` |
| `workplace/hd-weather-v19-build-receipt.json` | `d1716ce147974c6d1dd9e61e639bbe96bfc4a8bf335e7cea148f7757f03bae24` |
| `workplace/verify-hd-weather-v19-pack.json` | `b55749fb508d85f560f4e1a419839a14cf22966c5880e656f7247dad19f9b117` |
| `workplace/audit-hd-weather-v19.json` | `01e2afcb3e7062cc0244965b1591a1e6f2472dcb40a91228e8eefc572f9a0c4c` |
| `workplace/hd-portraits-audit-v19.json` | `cee3f8ac7988b300169fa91f47be859ed7db8ae1dc421ae79ad1b4785a22a01b` |
| `workplace/hd-scenes-audit-v19.json` | `415e6810bab238826a33a9a0269e8365c362b4a1110b271b4bff0b18063f5a6e` |
| `workplace/hd-weather-audit-v19.json` | `b9f72a04084f6493b84967ce099ee16a2db93b94858ae03bbea0637bf28d5eda` |
| `workplace/test-hd-weather-v19-audit.json` | `84f0035a743d72bb3bcaeaf42acf4fcb0d199ce10642b74e9fc1df0d8b05070c` |
| `workplace/hd-weather-v19-tests.json` | `ec5d7196d2de14695c86094d7f514c55c122490e3d7d71d51f3c0b845965111c` |
| `workplace/hd-window/player/weather-v19/receipt.json` | `70a71264e45dc3832521bdf424e28fbecf80c2c830f756d99900dfa0d46e3228` |
| `workplace/verify-hd-weather-v19-delivery.json` | `e4d9d91eb13e0f2cf004b0d55f11e25bc6d614c0859ee7b46f7287f7fb8544af` |
| `workplace/prepare-hd-weather-v19.py` | `d04d7f5360f42d22175414ad42d7c5b10dceed7371f82bb8c718cf93527669bc` |
| `workplace/verify-hd-weather-v19-source.go` | `535d999127549857daed43627a0eea0016b9ef8864b488499a9ace368c7b1882` |
| `workplace/collect-hd-weather-v19.py` | `8d5350b8b6057ce58c3f1520eac61ab6d9398a60f85a5059cd9aa7c94eadb1e3` |
| `workplace/make-hd-weather-v19-contact.py` | `1f4807ebdd07e08b8b06dc63206e3c6578d9ba00f98af9abff1cbb40bebed29d` |
| `workplace/build-hd-weather-v19.py` | `3762b954e8ed1f1d8939fb98672ef5a6fa64f5c14f47563b6ee2e3245128b93d` |
| `workplace/verify-hd-weather-v19-pack.go` | `9b85db7ab9c67e2a0b75410faf68ab6a394201255e311e07c704d25a987ede58` |
| `workplace/audit-hd-weather-v19.py` | `4b805a630d483a348e373c8243057522aa905b8ddeaed61b51f42a8c58128266` |
| `workplace/test-hd-weather-v19-audit.py` | `f3c0eaa638c4d5241546679012a7ee84fb5fdbcf8d0dc5d207c223802ee4b6b7` |
| `workplace/verify-hd-weather-v19-delivery.py` | `6b93e32f872df32287a5503b2c87108dffbb653d64276e20f840d978915de34a` |
| `workplace/verify-hd-weather-v19-publish.py` | `64d3a878a74ca66e9a128b0c92d49107b7b3aa38bd1cc284c756e5ee35359c1a` |
| `tools/hd-portrait-audit.py` | `c16d3fb10da2f3b560c087af63149a5daf9801056ab306caec95c5dfd34c847f` |
| `tools/verify-hd-battle-branches.sh` | `e11421c01e7bfd619840e61ba1ea583c0c363058bbf110100eb2c4d93e0d0139` |
| `tools/verify-hd-battle-branches-inner.py` | `ffe87236739ce50bd4e7270c05181dc2cccc178d195c1226a439e8b21689bbd6` |
| `tools/verify-window-inner.py` | `d7f0f648e4d50ce76cd6ffa1fc8b345d8e4d5662f4e2515c40c5b244b39d38a0` |
| `tools/hd-battle-branches-reference.go` | `909f4031a9dc9f72e7df992d3d633c4349bb3a2ecbb5bfdd077c1a3448333aab` |

### 6.31 主戰場與對戰子畫面的地形圖塊

**狀態：READY**。沿用已定案 B 寫實手繪、4×、每次啟動原貌與固定 640×408 邏輯版面，補 EICON.GRP 的 00–14。原版與加強版來源相同，各張 48×32、四平面、772 bytes，高清 192×128；地形、州郡資料、移動與戰術規則、存檔不改。正式主戰場只畫低四位 0–10，對戰子畫面 0–14，15 跳過；來源與繪製證據見 [005 §8](005-main-screen.md#8-主戰場的地形圖塊) 及 `internal/assets/battlefield.go`。本批不替 15–35 自訂用途。

保持 `assets.FieldCell` 的 x=56+48 欄、y=36+32 列與奇數欄 +16，逐欄順序及矩形範圍不變。高清地形在旗幟、兵力牌、子地圖將領標記、左右線、軍力／命令面板與文字之前；後續前景與選取效果沿原像素覆蓋。原貌 CPU 畫布逐像素保持，缺圖逐格回退。不得在已合成旗幟的整張畫面上直接覆蓋高清地形。

來源回讀與原圖輸出入口為 `workplace/prepare-hd-terrain-v20.go`，來源收據及 15 槽計畫為 `workplace/hd-terrain-v20-source.json`、`workplace/hd-terrain-v20-plan.json`。原圖輸出至既有 `workplace/hd-preview/`，檔名 `v20-source-EICON00.png` 至 `v20-source-EICON14.png`；比較頁由 `workplace/make-hd-terrain-v20-source-contact.py` 產生 `workplace/hd-preview/terrain-v20-source-contact.png`，另存 `workplace/hd-terrain-v20-source-contact.json`。先查看來源，再逐張記錄可見構圖、地形辨識與配色，不憑印象替未具名圖塊補用途。

候選及圖層 prototype 通過後才轉 READY。資源鍵沿盤點的 `DATA1/EICON.GRP#00`–`#14`；載入器必須驗實際 36 記錄與 27,792 bytes、來源子記錄 SHA-256、48×32 與 4× 尺寸，沿用 schema 1。雙版上限為 610，不開放未審查的圖塊。原始素材、提示詞、候選及含原版衍生美術的包只留本機。

最近鄰 16× 生成目標為 `workplace/hd-preview/v20-source-EICON00-16x.png` 至 `v20-source-EICON14-16x.png`，在既有預覽目錄製作。來源比較頁依序五欄、三列排列 00–14，來源回讀包含每槽 bytes、正式 parser 的索引像素及 PNG 像素，不只核對表頭。外部工具未回報模型與 seed 時明示；不推定使用者逐張簽核。

候選生成只以各自 16× 原圖作編輯目標，記錄在 `workplace/hd-terrain-v20-requests.json`。實際預設生成路徑保存於 `workplace/hd-terrain-v20-generated-paths.json`，候選副本採 `workplace/hd-b-EICON00-v1.png` 至 `workplace/hd-b-EICON14-v1.png` 等版次；不覆寫原始生成檔。候選收集、技術回讀及 Codex 審查分別為 `workplace/collect-hd-terrain-v20.py`、`workplace/hd-terrain-v20-generation.json`、`workplace/hd-terrain-v20-reviews.json`。完整提示詞與原圖只留本機。

圖層試作由 `workplace/prepare-hd-terrain-v20-prototype.py` 產生 `workplace/hd-terrain-v20-prototype_test.go`，收據為 `workplace/hd-terrain-v20-prototype.json`。隔離容器短暫複製測試至 `internal/ui/zz_hd_terrain_prototype_test.go`，以 trap 移除；試作不接正式玩家路徑。抽測寬／窄主戰場、子地圖、選取閃爍、前景遮擋與 CPU 畫布。前景記錄以未畫像素 255 為哨兵，執行同一套索引繪圖操作；不依原畫面與地形的顏色差判斷。選取矩形保留原本的索引 XOR 15 像素。EGA 色盤的棕色例外使 XOR 15 不完全等於 RGB 取補數，因此不以通道反相近似選取效果。

正常玩家地理誌及築城畫面共用 `DrawArtAtlas`，同樣需要高清地形，鄰郡標籤、提示與游標仍保有覆蓋權。原版紮寨的網點 `assets.ApplyMask` 目前未接到正式 `cmd/san1` 玩家路徑，本批不新增紮寨行為，也不宣稱已還原該遮罩；該差異不以高清素材補猜。

候選比較頁由 `workplace/make-hd-terrain-v20-contact.py` 製作 `workplace/hd-preview/terrain-v20-contact.png`，收據為 `workplace/hd-terrain-v20-contact.json`；每組左原圖、右 192×128 全圖縮放，順序 00–14。通過後以 `workplace/build-hd-terrain-v20.py` 建立私人 `workplace/hd-assets-terrain-v20/`，保存 v19 全部 580 筆欄位、PNG bytes 與準備紀錄，只增 30 筆兩版地形，收據為 `workplace/hd-terrain-v20-build.json`。

正式驗證入口為 `internal/ui/hd_test.go` 與 `tools/verify-hd-battle-branches.sh`；`tools/hd-portrait-audit.py` 已新增 `--family terrain`。本機正式載入與獨立回讀入口採 `workplace/verify-hd-terrain-v20-pack.go`、`workplace/verify-hd-terrain-v20-delivery.py`，同名 JSON 保存結果。UI 測試收據為 `workplace/hd-terrain-v20-tests.json`；四家族稽核收據為 `workplace/hd-terrain-v20-audits.json`。驗收仍分來源／候選、正式圖層與正常玩家路徑，不以試作或直接入口替代玩家流程。

兩版 30 個來源記錄、46,080 像素與 PNG 回讀通過；15 張 1536×1024 不透明候選及實際 192×128 比較頁均已查看，Codex 採用，使用者逐張簽核為 0。生成紀錄 SHA-256 為 `93c65c61f3d77a36a10cbefa5c236a1e960cd4c898c9040811a161a1c4d5bc77`；比較頁為 `0f12796bc668e18c22c1a78913021cdbe574656179c8634a46825cf744f9debc`。可丟棄圖層試作 40 項檢查通過，涵蓋兩種版面、子地圖與閃爍，足以授權正式接入；正式 GUI 與完整素材包驗收另行記錄。

正常玩家驗收採 `SAN1_HD_BRANCHES_PACK=workplace/hd-assets-terrain-v20`、`SAN1_HD_BRANCHES_OUT=workplace/hd-window/player/terrain-v20`。寬／窄主戰場另用 `tools/verify-hd-contexts.sh --battle-only`，設定 `SAN1_HD_CONTEXTS_PACK=workplace/hd-assets-terrain-v20`、`SAN1_HD_CONTEXTS_OUT=workplace/hd-window/player/terrain-contexts-v20`；兩目錄的 `receipt.json` 分列分支及範圍。兩項工具維持舊預設。共用 `tools/verify-window-inner.py` 從正常原貌截圖辨識完整未遮擋格，再核對原生 192×128 及原貌恢復；只辨識已回讀的 00–14 原圖，不注入狀態。

四家族完整稽核由 `workplace/audit-hd-terrain-v20.py` 執行，分族報告保存於 `workplace/hd-{portraits,scenes,weather,terrain}-audit-v20.json`。CLI 正反例入口與收據為 `workplace/test-hd-terrain-v20-audit.py`、`workplace/test-hd-terrain-v20-audit.json`；只建立自動清除的私人 fixture。獨立回讀另核對候選原檔、固定原圖、全部登錄與 PNG、前版保留紀錄及正常 GUI 最新截圖。

完整 UI／assets 套件的原始測試事件保存於 `workplace/hd-terrain-v20-tests-log.jsonl`，摘要為上述 `workplace/hd-terrain-v20-tests.json`，分列通過與 skip，不以 skip 宣稱完成。

寬版查看頁的正式 `drawOverlay` 填滿 (64,4) 至 (624,264)，沒有完整地形格可見；該停點改驗整張查看頁的原貌覆蓋與恢復，不計入可見地形分母。第一輪錯誤要求可見地形的收據及完整截圖保留於 `workplace/hd-window/player/terrain-contexts-v20-first/`，屬驗證腳本問題；正式 renderer 未因此修改。

第二輪收據及截圖保留於 `workplace/hd-window/player/terrain-contexts-v20-second/`。查看頁僅 2,700 像素不同，差異框為高清座標 (1252,772) 至 (1315,835)，原貌恢復完全相同；原因是 `x11grab` 同時擷取固定螢幕尺寸的系統游標。比對前將滑鼠移至邏輯 (20,380)，避開所有驗證矩形，再以相同命令乾淨重跑；不遮罩差異，也不修改遊戲繪製。

本批第五輪在遊戲啟動前遇到 X11 顯示器連線失敗，保存於 `workplace/hd-window/player/lure-v21-fifth/`。非 root Xvfb 缺少 Unix socket 目錄，原啟動檢查也未保證探測後的顯示器連線可用；驗證工具在容器內建立 socket 目錄，停用最後客戶端退出時的重設，並核對實際 socket 與顯示器幾何後重跑。此項屬驗證環境，不列為遊戲缺陷。

第六輪完整高清 22 步通過，長錄影結束時已進入後續電腦回合，施放前截圖另含閃爍游標，收據保留於 `workplace/hd-window/player/lure-v21-sixth/`。改核對錄影內動畫結束的第一個恢復幀：施法者 WFLAGA00 依正式索引 XOR 15 取補數後，整塊 24×15 旗區與原貌及高清均逐像素相同。兵力及後續戰況不拿來要求與施放前相同；原規則明定動畫後接交戰結算。

第七輪兩版原貌及原版高清的完整動畫通過。加強版在誘敵後的交戰中退出主攻軍，第一個恢復幀已改畫主守軍 WFLAGD00，完整 24×15 像素與來源相同；原收據保存於 `workplace/hd-window/player/lure-v21-seventh/`。驗證器核對本路徑兩支中軍的 A00／D00 原圖及正式反白，記錄實際來源與反白狀態，不預設施法者存活。

完整私人產物索引採 `workplace/hd-terrain-v20-artifact-index.json`，由 `workplace/index-hd-terrain-v20.py` 收錄準備、生成、包、驗證工具與收據雜湊。文件閘門入口及結果為 `workplace/verify-hd-terrain-v20-docs.py`、`workplace/hd-terrain-v20-docs-verification.json`。公開閘門為 `workplace/verify-hd-terrain-v20-publish.py`，分列 `workplace/hd-terrain-v20-publish-check.json`、`workplace/hd-terrain-v20-issue-publish-check.json` 及 `workplace/hd-terrain-v20-checkpoint-publish-check.json`，檢查實際暫存文字或 Issue 本文，保存 PNG 簽章、完整提示詞及 MZ 前綴的正反對照；不宣稱通用片段或編碼外洩偵測。

五項 Issue 的讀取、待寫全文與比對結果分別保存於 `workplace/hd-terrain-v20-issue-{104,107,108,109,110}-{before.json,body.md,after.json}`；更新計畫及同步收據為 `workplace/hd-terrain-v20-issues-plan.json`、`workplace/hd-terrain-v20-issues-sync.json`。只更新目前狀態與 #104 工作表，保留歷史、標題與 OPEN 狀態。

正式載入器雙版各 305 筆、警告 0，四家族完整準備稽核均返回 0，技術問題與使用者簽核為 0。地形 CLI 正反例 14/14；完整 UI／assets 187 項通過、skip 0。正式圖層涵蓋寬窄、主場／子場、閃爍的 8/8 組，以及地理誌／築城共用圖層、鄰郡標籤及游標顯隱。地理誌與築城正常 GUI 不由圖層測試代替。

兩版正常董卓新局、呂布攻陳留的對戰／快戰 229/229，14 個地形停點共 933 格、205 張最新 PNG；兩版曹操新局、陳留攻鄴郡／洛陽的寬窄主戰場與查看 80/80，6 個可見地形停點共 332 格、兩個完整遮擋停點、87 張最新 PNG。兩次 GUI 的正式執行檔雜湊相同。獨立回讀全部原始來源、610 登錄、305 PNG、15 張實際生成檔、v19 保留紀錄、完整格及查看頁通過，收據 `b1149ae28bd7093fd2cec0bb7407867f1aa0846dd74921de66339145f23c538a`。兩版實際高清主戰場、子畫面與查看頁已查看。

正常 GUI 保持正式預設亂數，沒有注入人物、戰場或 seed。可見地形集合為 0、1、2、3、6、7、8、10、11、13，其他五張由正式載入與圖層驗證，不推定本批正常 GUI 已出現。分支驗證的 14 個天候停點仍皆晴；寬版流程可出現雨，但本批未新增完整天候像素驗收。原貌預設、隱藏選項列、規則與存檔維持既有契約。整份 021 保持 READY，其他素材家族、誘敵動畫、自然事件與單挑等使用端、三語及跨平台仍待完成；配樂沿用既有收據。

| 本機產物或工具 | SHA-256 |
|---|---|
| `workplace/hd-terrain-v20-source.json` | `577840f2bfd79e92ea9dac413bfe6f51737539ee8ed613853dfb031285849f5c` |
| `workplace/hd-terrain-v20-plan.json` | `363cdc04c13274201429ed07e4b608a9f07f616312115c5af456cd3d43ff56cf` |
| `workplace/hd-terrain-v20-source-contact.json` | `0a5f0d46f1bb1fba1986bc50248e4e79b9839e1313051056914f17d285c70525` |
| `workplace/hd-terrain-v20-requests.json` | `89b9ccc2ae87678d01add6a2cbcbbafd9e9080b1649101ab1838315049a0bbe9` |
| `workplace/hd-terrain-v20-generated-paths.json` | `3f64bd930c5dd74ce843896b95b2b18db0f51a4ae0aec9fa6b8f1c97adec2cf2` |
| `workplace/hd-terrain-v20-generation.json` | `93c65c61f3d77a36a10cbefa5c236a1e960cd4c898c9040811a161a1c4d5bc77` |
| `workplace/hd-terrain-v20-reviews.json` | `756f558711c9eda2b02e7a79b3376f46a4d188cdd94bad2d87e1dc515e9a2405` |
| `workplace/hd-terrain-v20-contact.json` | `8e895badfd78dccbedebc524c281f41e32111e43bed0a13506198783152f6b28` |
| `workplace/hd-terrain-v20-prototype.json` | `10a9bf153b90b46b3e463b2e6fda6e5b91eb4bf7b823d06a81400764e4a5f2eb` |
| `workplace/hd-terrain-v20-build.json` | `b242a0413dd4d1832718555ae4896f1b43050009784646c8b998bfbfa2d83000` |
| `workplace/hd-assets-terrain-v20/manifest.json` | `64ae8f8745ee67cf2da28948d3e36ea43157570e9e56649e83b2d931ddae5b2c` |
| `workplace/hd-assets-terrain-v20/preparation.json` | `06f89dc4afbb839b15e19f9f4c2ee6711e6cce9189d7ef68d2d77d39372475f7` |
| `workplace/verify-hd-terrain-v20-pack.json` | `839f720c36c36c61ee3772a5335e76c7e71de1bbb8b46b583bb488e8d5fed01c` |
| `workplace/hd-terrain-v20-audits.json` | `da379850cf3af7283d3d54b1dcc64b80b6d19cefd7a2013fa52cc88b4bfa8a84` |
| `workplace/hd-portraits-audit-v20.json` | `435ee87adb421e53b465ec718f1b4f6cbc465fa5132ebec9c8906f92d4981fcd` |
| `workplace/hd-scenes-audit-v20.json` | `02aa7bc6f95d92408eed678ee0cb474e8c6b111ce157cafa1af5a553e50b1c97` |
| `workplace/hd-weather-audit-v20.json` | `752a94e420835c1fe98f99a1694a2b88e45be58826a36050852059d6b49b8d8a` |
| `workplace/hd-terrain-audit-v20.json` | `1a6c6104f720374f4ed421c39cf0faee0dded9160b38f0acc69807e75278017a` |
| `workplace/test-hd-terrain-v20-audit.json` | `a8b6f3b06a65ce77043df97e542471912fd57ce5a7dd4b55c90430a8310036e9` |
| `workplace/hd-terrain-v20-tests.json` | `03ddeaf9abfafdd6612a9eeefb21d1628ee94fb66f539ccdc4487c9c2e594bdd` |
| `workplace/hd-terrain-v20-tests-log.jsonl` | `fd980801e7cdf11d41dd557147e5bdf6490391adbba7f26fff5d47ede10c2208` |
| `workplace/hd-window/player/terrain-v20/receipt.json` | `98abe6e55aab60c54debcd4bcf873d1a127195f98123f2fe146f079631e13ea2` |
| `workplace/hd-window/player/terrain-contexts-v20/receipt.json` | `c08fef776a120e9dbb51401e4def116bfb031230ee5a467960a6965042cae6f4` |
| `workplace/verify-hd-terrain-v20-delivery.json` | `b1149ae28bd7093fd2cec0bb7407867f1aa0846dd74921de66339145f23c538a` |
| `workplace/hd-terrain-v20-artifact-index.json` | `cc89ed257009b58fe6afce77a5f5ef0d2649155984823af1f3ff276d0bd93ca0` |
| `tools/hd-portrait-audit.py` | `af017c26cc11478fcc81cf9237c710ec1b3d5b8a667db1794c63640a21a315e1` |
| `tools/verify-hd-battle-branches-inner.py` | `d7728ea00803973e9da33e218bd71c8f46a9365563ccdfc02acee0e61b729a30` |
| `tools/verify-hd-battle-branches.sh` | `e11421c01e7bfd619840e61ba1ea583c0c363058bbf110100eb2c4d93e0d0139` |
| `tools/verify-hd-contexts-inner.py` | `d0c930a73e53dcea0bc1ee384272d8ec9fcf35d8fb97d0f071bab1ffbfc10ec4` |
| `tools/verify-hd-contexts.sh` | `1d069b19dd7ae9f1f78eeb8ed6c966497b23d4926b3c527e3214cbc0c7168a7f` |
| `tools/verify-window-inner.py` | `8374fa135ad677af19c583b4bf6213b81f933f3de411f38b574adc48bafdab59` |
| `workplace/verify-hd-terrain-v20-delivery.py` | `5e9f3dac484d483e226f14438e470dbcaedda240e374dd9a1b543c61e8f04898` |

## 6.32 誘敵的四張特效圖

**狀態：READY**。沿用 B 寫實手繪、4×、原貌預設及既定玩家路徑，處理 DATA1/EICON.GRP#32–35。兩版來源容器相同，36 張 48×32 圖；本批只開放已確認的四張誘敵圖，不開放 15–31。原版證據與原始位址見 [005 §8 誘敵特效](005-main-screen.md#誘敵的特效0x2b7830x2b7f4l0l1bothissue-63)。原版的 22 步為 32、33，接 34／35 交替二十次；速度為 440、440，接 300／252 交替。位置仍由 `assets.FieldCell` 算施法者那一格，48×32 整塊不透明覆蓋，完成後由既有路徑重畫地形與旗幟。不改戰術、命中判定、亂數、聲音、等待或存檔。

先回讀及查看四張來源，按實際構圖生成候選，保留角色、特效方向與各幀差異。完整圖縮放至 192×128，不以任意 alpha 改變原整塊覆蓋。圖層試作須涵蓋 22 步、原貌畫布、原生高清、矩形外像素、前幀覆蓋、缺圖回退與完成後恢復；通過後才轉 READY 並接正式渲染。正常玩家施放及播放另驗，不以直接入口取代。

來源準備入口與收據為 `workplace/prepare-hd-lure-v21.go`、`workplace/hd-lure-v21-source.json`、`workplace/hd-lure-v21-plan.json`；原圖及 16× 編輯目標保存於既有 `workplace/hd-preview/` 的 `v21-source-EICON32.png` 至 `v21-source-EICON35.png` 與各自 `-16x.png`。來源比較頁與獨立回讀由 `workplace/make-hd-lure-v21-source-contact.py` 保存 `workplace/hd-preview/lure-v21-source-contact.png` 及 `workplace/hd-lure-v21-source-contact.json`，另驗兩版八個記錄的 12,288 像素。

執行請求、實際生成路徑、候選審查及生成彙整分別為 `workplace/hd-lure-v21-requests.json`、`workplace/hd-lure-v21-generated-paths.json`、`workplace/hd-lure-v21-reviews.json`、`workplace/hd-lure-v21-generation.json`。候選採 `workplace/hd-b-EICON32-v1.png` 至 `workplace/hd-b-EICON35-v1.png` 等版次，不覆寫實際生成檔；收集入口為 `workplace/collect-hd-lure-v21.py`。最終比較入口與產物為 `workplace/make-hd-lure-v21-contact.py`、`workplace/hd-preview/lure-v21-contact.png`、`workplace/hd-lure-v21-contact.json`。

可丟棄圖層入口及收據採 `workplace/hd-lure-v21-prototype_test.go`、`workplace/hd-lure-v21-prototype.json`，隔離容器短暫複製為 `internal/ui/zz_hd_lure_prototype_test.go`，以 trap 移除。READY 後才由 `workplace/build-hd-lure-v21.py` 建立私人 `workplace/hd-assets-lure-v21/`，保存 v20 全部 610 筆欄位、PNG bytes 與準備紀錄，只增兩版八筆，schema 1 上限為 618，收據為 `workplace/hd-lure-v21-build.json`。正式載入與整包回讀入口採 `workplace/verify-hd-lure-v21-pack.go`、`workplace/verify-hd-lure-v21-delivery.py`，各自同名 JSON 保存結果。所有來源、提示詞、候選及含原版衍生美術的包只留本機。

來源八個記錄的 12,288 像素回讀通過。四張 1536×1024 不透明候選及 192×128 比較頁已查看，Codex 採用，使用者逐張簽核為 0；生成工具採內建 image_gen，模型與 seed 未回報。人物姿態與紅黃格帶按原圖改編，高清外觀不宣稱原像素一致。生成紀錄雜湊為 `bf9f8527f8d58b6b6e8bb2d617f3b2ec67d7663c91dce480da1355c7ffbad7d4`，比較頁為 `7879395c162acc5cd2d369da6aa486fe6a07af9c279b17024e1577de1632ac5f`。圖層試作 108 項通過，含 22 步、四個位置、原貌、矩形外像素、前幀、裁切、缺圖及重畫恢復，足以授權正式接入；正常玩家驗證另行記錄。

正式準備稽核由 `workplace/audit-hd-lure-v21.py` 保存 `workplace/hd-lure-v21-audits.json` 與 `workplace/hd-{portraits,scenes,weather,terrain,lure}-audit-v21.json`；CLI 正反例為 `workplace/test-hd-lure-v21-audit.py` 與同名 JSON。完整 UI／assets 的原始事件與摘要採 `workplace/hd-lure-v21-tests-log.jsonl`、`workplace/hd-lure-v21-tests.json`。正常 GUI 入口預定為 `tools/verify-hd-lure.sh`、`tools/verify-hd-lure-inner.py`，資料與連續錄影保存於 `workplace/hd-window/player/lure-v21/`；素材只讀並使用正常新局、出兵、合法紮寨與策略選單。私人靜態玩家計畫入口及收據採 `workplace/hd-lure-v21-player-plan.go`、`workplace/hd-lure-v21-player-plan.json`，只讀 typed data，不注入執行中的遊戲。

首輪正常 GUI 在紮寨後仍有戰場對白，驗證器過早要求命令選單。原始收據與截圖保存於 `workplace/hd-window/player/lure-v21-first/`。第二輪已抵達正常選單，但整塊參考不含動態部隊資料，保留於 `workplace/hd-window/player/lure-v21-second/`。依失敗診斷路由核對實際畫面後，改以正常空白鍵收對白，只辨識固定三行選單，不改遊戲程式、能力、資金或亂數。

第三輪原貌正常施放完整 22 步與四幀原像素通過，旗幟恢復檢查卻錯用未合成的單張旗圖。收據、錄影及截圖保留於 `workplace/hd-window/player/lure-v21-third/`；逐像素查明動畫前後的實際旗區完全相同，改比較正常畫面上的同區，不抹除原有選取或疊層。

第四輪原貌完整 22 步與恢復通過。高清六秒錄影只到 32／33，已出現的幀均與原生素材相符，保留於 `workplace/hd-window/player/lure-v21-fourth/`；高清另採 30 fps、90 秒有界錄影並指定輸出幀率，核對完整步序及實際時間。較長錄影不算效能合格，也不改遊戲的速度或等待常數。

第五輪在遊戲啟動前遇到 X11 顯示器連線失敗，保存於 `workplace/hd-window/player/lure-v21-fifth/`。非 root Xvfb 缺少 Unix socket 目錄，原啟動檢查也未保證探測後的顯示器連線可用；驗證工具在容器內建立 socket 目錄，停用最後客戶端退出時的重設，並核對實際 socket 與顯示器幾何後重跑。此項屬驗證環境，不列為遊戲缺陷。

第六輪完整高清 22 步通過，長錄影結束時已進入後續電腦回合，施放前截圖另含閃爍游標，收據保留於 `workplace/hd-window/player/lure-v21-sixth/`。改核對錄影內動畫結束的第一個恢復幀：施法者 WFLAGA00 依正式索引 XOR 15 取補數後，整塊 24×15 旗區與原貌及高清均逐像素相同。兵力及後續戰況不拿來要求與施放前相同；原規則明定動畫後接交戰結算。

第七輪兩版原貌及原版高清的完整動畫通過。加強版在誘敵後的交戰中退出主攻軍，第一個恢復幀已改畫主守軍 WFLAGD00，完整 24×15 像素與來源相同；原收據保存於 `workplace/hd-window/player/lure-v21-seventh/`。驗證器核對本路徑兩支中軍的 A00／D00 原圖及正式反白，記錄實際來源與反白狀態，不預設施法者存活。

完整私人產物索引採 `workplace/index-hd-lure-v21.py` 與 `workplace/hd-lure-v21-artifact-index.json`。文件與公開文字閘門入口採 `workplace/verify-hd-lure-v21-docs.py`、`workplace/verify-hd-lure-v21-publish.py`；收據分別為 `workplace/hd-lure-v21-docs-verification.json` 與 `workplace/hd-lure-v21-{publish,issue-publish,checkpoint-publish}-check.json`。公開閘門核對實際暫存文字與五份 Issue 全文，附私人 PNG、完整提示詞及 MZ 前綴的正反對照。

五項 Issue 的全文讀取、待寫本文與遠端核對採 `workplace/hd-lure-v21-issue-{104,107,108,109,110}-{before.json,body.md,after.json}`，計畫及同步收據為 `workplace/hd-lure-v21-issues-plan.json`、`workplace/hd-lure-v21-issues-sync.json`；保留標題、歷史與 OPEN 狀態。工作歷程只追加 `WORKLOG.md`，現況回填 `CONTEXT.md` 與 `VERIFICATION-MATRIX.md`。

本批已驗結果由 `workplace/update-hd-lure-v21-docs.py` 回填上述現況文件，必須先通過獨立整包及錄影回讀，再執行文件與公開閘門。

### 6.32.1 本批驗證結果

私人 v21 包為 618 登錄、309 PNG，兩版各 309，正式載入器警告 0，v20 全部 610 筆欄位、圖檔及準備紀錄保持。五家族完整稽核技術問題均為 0，CLI 正反例 14/14，正式 UI／assets 195 項通過、skip 0；108 項圖層檢查保持原貌 CPU 畫布、22 步及速度、矩形外像素、前幀覆蓋、裁切、缺圖回退與重畫恢復。

`bash tools/verify-hd-lure.sh` 在 `rich2-go-ebiten:latest` 建置後，以 `eob-audio-capture:20260922-r2` 進行兩版各原貌及高清的正常片頭、新局、陳留出兵鄴郡、400 金／1000 米、合法紮寨及誘敵。容器為 UID/GID 1000、4 CPU、4 GiB、256 pids、network none，原始資料與素材包只讀。四段共 31/31 檢查，每段完整 22 步、四張原生圖、外框保持，動畫結束的第一個恢復幀與實際 A00／D00 原圖及反白完全相同。戰損與後續回合照原路徑推進，不要求施法者存活。正常新局不注入人物、戰場或 seed，不稱為原版 oracle。

獨立回讀確認來源八筆／12,288 像素、四份實際生成檔、完整 192×128 縮放、不透明、618 登錄與前版保留；四段錄影、54 張最新完整視窗截圖、16 張動畫及 4 張恢復截圖全部回讀通過，兩版高清畫面已查看。

錄影採原貌 60 fps／6 秒及高清 30 fps／90 秒，指定實際輸出幀率並逐幀核對。觀察到的動畫時長為 base-original 4.617 秒、base-hd 32.233 秒、plus-original 4.617 秒、plus-hd 5.667 秒。這是軟體 OpenGL 與錄影並行的環境，未建立獨立效能基準；高清明顯較慢，#110 的播放效能仍未完成。不改音訊等待、TPS 或速度常數，也不外推硬體時序、人耳或其他平台。整份 HD 仍為 READY，#107–#110 保持 OPEN。

| 私人收據或素材 | SHA-256 |
|---|---|
| `workplace/hd-assets-lure-v21/manifest.json` | `1310aa2ba0363298ec753fed67d2e62f9327be4bea1b009a5186d2def16b7600` |
| `workplace/hd-assets-lure-v21/preparation.json` | `c71b4d7b5b2cc61298f67ea86f548c1af607bd06a80a9d7aeaa24b57da035a7e` |
| `workplace/hd-lure-v21-generation.json` | `bf9f8527f8d58b6b6e8bb2d617f3b2ec67d7663c91dce480da1355c7ffbad7d4` |
| `workplace/hd-lure-v21-contact.json` | `230d84402684956d179282c5037645a50647586d4c54ff44fec847a13d03198e` |
| `workplace/hd-lure-v21-build.json` | `e3a939787daa6e2f43605183a9a82cc6dedbe591d8740f5ee94f6521f0f6f3c5` |
| `workplace/hd-lure-v21-source-contact.json` | `cdf413e9ec5c07b69739bf914f99874947b6c174ddf7d2807025b17aa053fc5c` |
| `workplace/hd-lure-v21-prototype.json` | `f0af4c1185ae25a41c99456a0ff58ad4bd4484f8db51e36b68e8ff4a7fdb5175` |
| `workplace/verify-hd-lure-v21-pack.json` | `b628a7dd93e2a69deaba8cdfa83d3dcf1103da4ac5f5aba44b51b842ed7ad499` |
| `workplace/hd-lure-v21-audits.json` | `b9d8ee21aa11a6e387eb6100133a2a6af835dc4ec8c42bafa301d936a76b2b52` |
| `workplace/test-hd-lure-v21-audit.json` | `d6bfcba41fa7bf2f1c76ec6ce247e7fdc8fba8fc1612a1fc2e73fee26a645d8e` |
| `workplace/hd-lure-v21-tests.json` | `584fc61577be800f18215cdb0d22e52d06824a928779c58fda2f1aa215039723` |
| `workplace/hd-window/player/lure-v21/receipt.json` | `8d6e5cc8586efdcb3cc3d65167c88b9e0d871a8048b28369be1f013f13d64498` |
| `workplace/verify-hd-lure-v21-delivery.json` | `c469723d947427c999536fac35310b38687ef5f400f055ab2c15b609e64792dc` |
| `workplace/hd-lure-v21-artifact-index.json` | `231e33e86b32407b0dedecc3556b5855da6e9e16bff2f568a31990d263b44f22` |

## 6.33 高清播放效能量測

**狀態：CONFORMED，限固定 4× 像素等價維護**。#110 的整體播放效能尚未完成，整份 HD 仍為 READY。§6.32 四段完整誘敵錄影證實高清顯示正確；在 4 CPU／4 GiB 的軟體 OpenGL 與錄影並行環境，原版高清 32.233 秒、加強版高清 5.667 秒，兩段原貌皆 4.617 秒。此證據未隔離 CPU 合成、GPU／驅動與錄影負載，不能據此指定產品修法。

本輪保持 B、4×、完整 2560×1632、每次啟動原貌及所有原像素、疊層與缺圖回退。先量測既有 `DrawArtBattle`、`DrawLureFlash`、`Canvas.Output`、`uploadGame` 與視窗繪製的時間及配置量，再依實際瓶頸做可丟棄的等價實作。不得降低解析度、跳過動畫相位、調整 TPS、延遲或音訊等待來縮短時間。Go、Ebiten 與顯示器使用目前 Docker 工具鏈，不退回主機執行。

控制組只讀目前兩版原始容器及私人 v21 包；寬窄主戰場、場景與肖像的 CPU 合成量測不能取代正常玩家路徑。量測入口及收據採 `workplace/hd-perf-v22-probe_test.go`、`workplace/hd-perf-v22-cpu-probe.json`、`workplace/hd-perf-v22-cpu-profile.pprof`、`workplace/hd-perf-v22-cpu-profile.txt`，隔離容器暫時複製為 `internal/ui/zz_hd_perf_v22_probe_test.go`，以 trap 移除。

首份 CPU 控制量測為 Go 1.24.13、兩版 × 寬窄 × 原貌／高清，共八組 960 幀。原貌 paint 平均 1.93–2.94 ms；高清 paint 3.24–3.62 ms、Output 24.16–27.46 ms。通用最近鄰縮放占 CPU 採樣 53.79%，含載入採樣；此項為 remake 控制量測已證實，不證明原版硬體時序或正常視窗根因已全部解決。

固定 4× 的複製試作採 `workplace/hd-perf-v22-scale-prototype_test.go`、`workplace/hd-perf-v22-scale-prototype.json`，同樣暫存於 `internal/ui/zz_hd_perf_v22_scale_test.go` 並以 trap 移除。以目前 `x/image` 的最近鄰 `draw.Src` 作獨立預期值，核對完整 RGBA、alpha、非零原點、來源／目的 stride、子圖與矩形外像素；原版寬窄兩種版面的 22 相位逐幀回讀。試作不可改變原畫布、覆蓋權、解析度或素材。等價檢查與速度量測通過後，才授權將純像素等價的維護接入正式 `Canvas.Output`。

試作已通過 12 組幾何與 44 個原版正式疊層幀的完整 RGBA 比較。寬窄兩組各 120 次純放大量測，通用縮放平均 21.17／23.78 ms，固定複製平均 2.51／2.56 ms，分別快 8.45／9.29 倍。來源與目的原點、stride、子圖、alpha 及矩形外位元組皆相同。此證據足以授權純維護接入；正式回歸由 [hd_scale_test.go](../../internal/ui/hd_scale_test.go) 保存同一個獨立最近鄰預期值，不在 CI 設不穩定的耗時門檻。

基線程式與建置來源保存於 `workplace/hd-perf-v22-baseline-hd.go`、`workplace/hd-perf-v22-baseline-build.json`；後續純維護實作不得覆寫這份控制組。正式接入後的 CPU 與正常玩家複驗採 `workplace/hd-perf-v22-{optimized-cpu.json,optimized-profile.pprof,optimized-profile.txt,optimized-normal.py,optimized-normal.json,optimized-build.json}` 及 `workplace/hd-window/player/perf-v22-optimized/`，以相同素材、鍵序、資源限制與計時 overlay 比較。完整 UI／assets 事件與摘要採 `workplace/hd-perf-v22-tests.jsonl`、`workplace/hd-perf-v22-tests.json`。

正常玩家量測以 Go overlay 的可丟棄副本接入計時與 CPU 採樣，不改正式程式。入口、工具副本、原始與摘要收據存於 `workplace/hd-perf-v22-normal.py`、`workplace/hd-perf-v22-overlay.json`、`workplace/hd-perf-v22-{main.go,windowbar.go,trace.go}`、`workplace/hd-perf-v22-normal.json` 與既有 `workplace/hd-window/player/perf-v22/`。各次正常原貌／高清新局、出兵、合法紮寨與誘敵採相同既有鍵序，不注入人物、seed 或戰場；先停用錄影，量測完整 22 步與實際更新／繪製，再決定是否需要錄影交叉驗證。收據記錄程式、來源、包、工具與容器版本及雜湊，不稱為原版硬體時序對拍。

無錄影基線四組皆畫出完整 22 相位。兩版原貌為 4.636／4.634 秒，高清為 5.299／13.671 秒；高清 Output 平均為 29.96／40.12 ms。移除錄影後仍有波動，先前 32.233 秒不能只歸因於這個縮放函式。後續比較保持同一 overlay、軟體 OpenGL、4 CPU／4 GiB 與正常操作；純 CPU 量測和整段正常動畫分別報告。

正式維護將通用最近鄰改為逐行複製 4×4 的 RGBA bytes，保留 `highOps` 的疊圖及原 UI 覆蓋權。來源限定為現有 RGBA 畫布，目的矩形尺寸是來源四倍，兩者不重疊；不引入 unsafe、近似插值或低解析輸出。回歸十二組尺寸／子圖幾何、完整 UI／assets 208 項均通過，skip 與 fail 為 0。八組 CPU 各量測 120 幀、合計 960 幀；八組末幀的原貌與高清輸出雜湊相同，完整逐幀等價另由前述 44 幀試作與幾何回歸驗證。

| CPU 高清 Output | 修改前平均 ms | 修改後平均 ms | 速度比 |
|---|---:|---:|---:|
| 原版窄圖 | 27.46 | 4.68 | 5.86× |
| 原版寬圖 | 24.16 | 3.67 | 6.59× |
| 加強版窄圖 | 26.06 | 3.76 | 6.93× |
| 加強版寬圖 | 24.80 | 3.52 | 7.04× |

控制組與修改組均為 Go 1.24.13、GOMAXPROCS 14、4 CPU／4 GiB，18 份原始來源雜湊與私人 v21 包完全相同。含載入的 CPU 採樣中，固定複製占 10.46% flat、13.03% cumulative；此百分比的分母含 PNG 與字型載入，不能外推正常視窗占比。

| 無錄影正常動畫 | 修改前秒 | 修改後秒 | 修改前／後 Output 平均 ms |
|---|---:|---:|---:|
| 原版原貌 | 4.636 | 4.633 | <0.001／<0.001 |
| 原版高清 | 5.299 | 5.587 | 29.96／6.18 |
| 加強版原貌 | 4.634 | 4.634 | <0.001／<0.001 |
| 加強版高清 | 13.671 | 10.506 | 40.12／7.17 |

兩組皆透過相同 Go overlay 從正常片頭、新局、出兵、合法紮寨及策略選單施放，每次 22 個相位都有實際 Draw；每段 Update 次數均為 279，來源、正式等待、速度與 TPS 保持。CPU 合成改善已證實，整段高清仍較慢；尚未分離 GPU 呈現、驅動及排程，#110 保持未完成，不以單次時長認定跨平台效能或原版硬體時序一致。

不帶 overlay 的正式正常路徑另以 [verify-hd-lure.sh](../../tools/verify-hd-lure.sh) 重跑，`SAN1_HD_LURE_OUT=workplace/hd-window/player/lure-perf-v22`，核對四段原生圖、完整相位、塊外像素及恢復旗圖。私人交付回讀、索引、文件及公開閘門入口採 `workplace/verify-hd-perf-v22-{delivery.py,delivery.json,docs.py,docs.json,publish.py}`、`workplace/index-hd-perf-v22.py`、`workplace/hd-perf-v22-artifact-index.json` 與 `workplace/hd-perf-v22-{publish-check.json,issue-publish-check.json,checkpoint-publish-check.json}`。五份 Issue 的讀寫前後快照及本文採 `workplace/hd-perf-v22-issue-{104,107,108,109,110}-{before.json,body.md,after.json}`，同步計畫與收據採 `workplace/hd-perf-v22-issue-{plan.json,sync-receipt.json}`。不公開原圖、高清圖、完整提示詞或私人收據。

正式四次正常玩家操作共 31/31 檢查通過。獨立回讀核對實際程式、18 份原始來源、私人包的全部 309 PNG、原始測試事件、兩組共八次計時及四段錄影；54 張最新完整視窗 PNG、16 張動畫與 4 張第一個恢復幀 PNG 皆核對雜湊。四段各完整 22 步、四張原生圖、動畫外框及恢復旗圖相符，兩版高清畫面已查看。原貌動畫各約 4.633 秒，錄影並行的高清約 6.833／20.267 秒；此時長含錄影負載，與無錄影量測分開，不作效能合格聲明。音訊未重測，保留 §6.13 的 Linux 配樂範圍；不新增音效、人耳、原版 oracle 或跨平台聲明。

本輪主要私人證據：

| 路徑 | SHA-256 |
|---|---|
| `workplace/hd-perf-v22-cpu-probe.json` | `1880db38a733dcd0b274c938ca2bcfee2febec8ebc29cfbe9732cd56456cf198` |
| `workplace/hd-perf-v22-optimized-cpu.json` | `cc11e6ff286386ec1b71b8ab3adc158b2cae969ada068c876b11cf3e1c97c29a` |
| `workplace/hd-perf-v22-scale-prototype.json` | `e533dae8bf64a8bf4972f0db6f553770f150cba321220960c9bcd19a9cb2b1cc` |
| `workplace/hd-perf-v22-normal.json` | `fa8f5477c3a5838447318be46ad07c2deed90bd2f7c9a09ca8cbfde5f5553cd9` |
| `workplace/hd-perf-v22-optimized-normal.json` | `5e4fa13e3186dd19673f28eebade2a13f62958097dc0d509c93225c12192b16a` |
| `workplace/hd-perf-v22-baseline-build.json` | `82f6533a39a6db805c729bc3261c26b5994efeefb29aaf16914cef7f6ddf98f0` |
| `workplace/hd-perf-v22-optimized-build.json` | `39de485ca48717006c988f0b5c209dc787dcc4f3423cb58d4bca9a6376753657` |
| `workplace/hd-perf-v22-tests.jsonl` | `43326e35f0c1172d9d8e881354e65f23164565e4f197838f993b733db0bb3e72` |
| `workplace/hd-perf-v22-tests.json` | `d839159ca920480c50b4339ea3f640695fa9dec674e13411e0628f9e664ec348` |
| `workplace/verify-hd-perf-v22-delivery.py` | `9821a6679a304166132f626c2890afa391776d13a3348e72da5bd7f720b973d2` |
| `workplace/verify-hd-perf-v22-delivery.json` | `35073ff1826980587069b585f2c486ab3337407b226efb8cce6519da794c1efb` |

## 6.34 軟體顯示驅動與 CPU 配額

**狀態：DRAFT**。§6.33 已完成像素等價的 CPU 放大維護，整段高清仍較慢。本節先區分顯示環境、容器排程與產品合成，不授權修改正式等待、速度、TPS、動畫相位或解析度。

工具容器為既有 `eob-audio-capture:20260922-r2`，實際 Mesa 為 22.3.6、LLVM 15.0.6；可見 CPU 與 affinity 均為 14。Mesa 官方 [LLVMpipe 文件](https://docs.mesa3d.org/drivers/llvmpipe.html) 說明軟體光柵器採多執行緒；[環境變數契約](https://docs.mesa3d.org/envvars.html#llvmpipe-driver-environment-variables) 指定 `LP_NUM_THREADS` 控制渲染執行緒數，預設依可見核心數。診斷與八組正常比較實際 `cpu.max` 為 `400000 100000`，維持 4 CPU／4 GiB。程序的實際 Mesa 執行緒名稱確認預設 14 個、指定組 2 個；兩組 Go GOMAXPROCS 均為 14。

以同一正式來源、同一可丟棄計時程式及正常鍵序，預先指定兩組：`LP_NUM_THREADS` 未設、以及 `LP_NUM_THREADS=2`。Go 的 GOMAXPROCS 維持 14。每組均跑兩版原貌／高清，完整 22 相位與等待保持；記錄實際 Mesa renderer、執行緒名稱、容器 `cpu.stat` 開始與結束值及逐階段時間。不在試驗中關閉 rasterization、移除畫面、縮小輸出或挑選較快 seed。結果無論快慢均保存；若出現產品缺口，另回到規格審查。

`cpu.max` 與 `cpu.stat` 的單位及語意依 [Linux cgroup v2 文件](https://docs.kernel.org/admin-guide/cgroup-v2.html#cpu-interface-files)。`cpu.stat` 包含整個容器的程序，限流欄位記錄此 cgroup 自身的頻寬限制，不含祖先限制；不把累積的 `throttled_usec` 直接當成遊戲的單一牆鐘停頓。開始／結束讀取位於相同動畫邊界，結束值在停止 CPU 採樣前擷取。

本機入口與收據採 `workplace/hd-perf-v23-{platform.py,platform.json,normal.py,trace.go,overlay.json,build.json,normal-default.json,normal-two.json}`，正常輸出沿用 `workplace/hd-window/player/` 下的 `perf-v23-default/` 與 `perf-v23-two/`。原始來源、私人 v21 包及先前 v22 收據唯讀；計時及 cgroup 蒐證只由 Go overlay 接入，不進正式程式。這是 remake 顯示環境診斷，不是原版 oracle、硬體時序、音效或人耳驗收。

八組皆完整繪製 22 相位，Tile／Speed 次序及 279 次 Update 保持，兩組 binary bytes 相同。無錄影動畫時間如下；這是固定試驗的結果，不作平台效能保證。

| 版本與外觀 | 預設 14 執行緒，秒 | 2 執行緒，秒 |
|---|---:|---:|
| 原版原貌 | 4.640 | 4.639 |
| 原版高清 | 4.638 | 9.988 |
| 加強版原貌 | 4.678 | 4.615 |
| 加強版高清 | 4.893 | 8.941 |

預設高清的容器限流增加 35／44 期，累積 1,795,674／3,715,040 微秒；指定 2 執行緒的四段動畫均無限流，高清反而變慢。因此不採用 `LP_NUM_THREADS=2`，也不能把「渲染執行緒超過配額」當成既有慢速的已證實真因。本試驗未重現先前最慢的時長，未排除其他呈現與排程因素。正式遊戲、驗證入口及等待均不改，本節保留 DRAFT，不再靠增加同類參數試驗求過關。

獨立回讀入口與收據為 `workplace/verify-hd-perf-v23-delivery.py`、`workplace/verify-hd-perf-v23-delivery.json`。逐項核對正式來源、overlay、相同 binary、八組原始 trace、實際執行緒及完整相位；圖片與原始紀錄只留本機。後續呈現路徑的變更須另做像素等價試作，通過後才授權正式維護。

## 6.35 最終呈現的像素等價試作

**狀態：DRAFT**。§6.34 的兩執行緒設定不採用。本節只試作減少原生尺寸、隱藏選項列時的一次全畫面 GPU 繪製；保留 4×、B 畫風、完整遊戲畫面、透明度、動畫 22 相位、速度、TPS、等待及音訊。正式來源、資料與存檔保持，試作僅由私人 Go overlay 接入。

目前 `cmd/san1` 在 `Draw` 將已上傳的遊戲圖畫至 Ebiten 中間畫布，再由引擎畫到最終視窗。鎖定 Ebiten 2.9.9 的 [FinalScreenDrawer API](https://github.com/hajimehoshi/ebiten/blob/v2.9.9/run.go) 與[預設呈現實作](https://github.com/hajimehoshi/ebiten/blob/v2.9.9/gameforui.go)：最終回呼在 `Draw` 之後，縮放矩陣含等比縮放及置中；整數倍率採最近鄰，縮小採線性，非整數放大採 Pixelated。不能直接合併任意倍率、選項列及透明圖層。

試作保留 `Draw` 原有拉幕起點、CPU 繪圖及上傳，將視窗合成放到最終回呼。只有選項列隱藏、最終矩陣六元素完全為單位矩陣、遊戲圖／中間畫布／最終視窗的 bounds 相同時，直接畫已上傳遊戲圖。其餘情況依原有 `drawWindow` 合成中間畫布，再呼叫 `DefaultDrawFinalScreen`。不依視窗大小、DPI 或前一幀推測矩陣，也不變更插值模式。

先以真實原貌／高清人物卡及戰場全圖，另加透明像素反例，在 Docker／Xvfb 的實際 GPU 路徑回讀整張 RGBA，比較原合成與試作。涵蓋原生兩倍率、選項列及下拉、縮小、非整數放大及置中。控制與試作兩組均以最終回呼記錄正常誘敵相位，完整 22 步及 279 次 Update，兩版各原貌／高清；維持預設 Mesa 執行緒、同一來源與相同鍵序，不錄影、不注入狀態或 seed。幀數與時間實測保存，像素不等價或播放未改善時不進正式程式。

入口與證據沿用私人 `workplace/`，前綴為 `hd-perf-v24-`：`prepare.py`、`main-control.go`、`main-prototype.go`、`main-fixture.go`、`windowbar.go`、`trace.go`、`final-control.go`、`final-prototype.go`、`fixture.go`、`overlay-control.json`、`overlay-prototype.json`、`overlay-fixture.json`、`normal.py`、`build.json`、`gpu.json`。輸出在既有 `workplace/hd-window/player/` 下的 `perf-v24-control/`、`perf-v24-prototype/`、`perf-v24-gpu/`。公開維護須在上述證據審查後轉 READY，另跑正常視窗的 Esc／hover、語言、Theme、AI 及縮放，再驗正式無 overlay 的原生圖、外框與動畫恢復。這是 remake 呈現維護，原版 oracle 與既有音訊驗證範圍保持。

GPU 14 組共 22,712,320 像素回讀相同，原生直接呈現及非原生回退皆命中預期。帶 CPU 採樣的八段正常量測仍不一致：兩版高清控制為 47.499／4.747 秒，試作為 4.645／11.897 秒。全部完整 22 相位及 279 次 Update；試作的最終回呼均走直接呈現。這些結果不足以宣稱一致改善，本節保持 DRAFT。

後續只隔離 CPU 採樣這一因素，不調正式參數：同一程式、資料、矩陣、配額及鍵序，控制／試作各四段，關閉 pprof，保留原有逐階段計時及 cgroup 起訖。原組保留不覆寫。入口與收據仍用 `hd-perf-v24-` 前綴：`prepare-plain.py`、`trace-plain.go`、`normal-plain.py`、`overlay-plain-control.json`、`overlay-plain-prototype.json`、`build-plain.json`、`normal-plain-control.json`、`normal-plain-prototype.json`；輸出在 `workplace/hd-window/player/perf-v24-plain-control/` 與 `perf-v24-plain-prototype/`。兩組都不採用前輪的兩執行緒設定；若仍無一致證據，不修改正式呈現，改推進其他未完成驗收。

獨立回讀入口及收據為 `workplace/verify-hd-perf-v24-delivery.py`、`workplace/verify-hd-perf-v24-delivery.json`，逐項核對正式來源、固定試作／控制程式、十四組 GPU 分母、四組建置、十六段原始 trace、全部最新完整 PNG 與兩版全部相位。計時結果分開 CPU 採樣與無採樣，不將最終回呼頻率稱為實體螢幕更新率。

本次試作已結束，**不採用**。關閉 CPU 採樣後，原版高清控制／試作為 4.655／4.652 秒，加強版為 5.029／32.276 秒，仍無一致改善。十六段均完整保留 22 相位、原速度及 279 次 Update；獨立回讀通過 124 份正式來源、184 張最新完整 PNG、四組建置與十四組 GPU 全圖比較。正式呈現保持原有路徑，本節保留 DRAFT 及完整負結果，不再以調參數重跑此候選。這些數字不證明特定驅動、CPU 採樣或主機排程是差異真因。

## 6.36 三語系的正常玩家畫面與即時切換

**狀態：DRAFT**。接續 #110 的三語驗收，範圍為兩版正常片頭、新局、主畫面、人物卡、對白、選單與主戰場。沿用 B、4×、原貌預設與 §6.3 的即時切換契約，不改玩法、資料或存檔格式。

先用目前正式程式重現每個玩家停點的繁中／英文／日文切換，分別擷取原貌與原生高清。核對語系確實更新、文字安全區、日期十槽、肖像身份與圖框、高清外的文字／框線及切回原貌。已驗圖片不推定全部長姓名、所有事件、子畫面或三語系全文完成；結果逐項記入驗證矩陣。

靜態檢查發現 `cmd/san1/windowbar.go` 的 `relocalizeWindow` 目前更新主畫面與主選單，未處理 `fight.view` 的已生成文字。`ui.BattleCommandWindow` 在建立玩家命令提示時依當下語系生成多行字串；是否在即時切換後保留舊語言，先以正式 GUI 查證，不先修改程式。

私人入口沿用 `workplace/`，前綴為 `hd-locale-v25-`：`normal.py`、`prepare.py`、`build-before.json`；正常重現輸出為 `workplace/hd-window/player/locale-v25-before/`。先使用已核對 124 份正式來源的現有無 overlay 執行檔，保留控制器、鍵序、PNG、來源及素材包雜湊。若發現缺陷，補齊顯示狀態的輸入／輸出、語系回復、存檔隔離及實際失敗證據後，才轉 READY 修正，再以同一正常路徑重跑。

首輪原版正常戰場的英文／日文標題更新，命令前三行仍與繁中完全相同，原貌／高清皆重現。加強版第一組全 96 列比較只差輸入游標的 576 個高清像素，範圍為面板內 (576,272)–(607,295)，前三行相同。此差異是非同步擷取的游標，不列為產品缺陷；原失敗保存，改用 `normal-r2.py` 比較不含游標的前三行，輸出 `locale-v25-before-r2/`、`build-before-r2.json`，游標差異收據為 `cursor-diff.json`，前綴同上。

### 6.36.1 戰場已生成提示的即時更新

**狀態：CONFORMED，限戰場命令即時切換及顯示字串回歸**。原 READY 契約只處理已生成的戰場顯示文字，不涵蓋整份三語系或所有字區。兩版正常證據為 `locale-v25-before-r2/receipt.json`，四次英文／日文切換皆保留繁中命令；共同正式來源的 `BattleView.Window` 在 `nextActor` 以當下語系產生，`relocalizeWindow` 未更新 `fight.view`。這是 remake 選項列缺陷，無原版語言切換 oracle，亦不修改原版規則。

輸入為舊語系、目前戰場顯示快照及同一隊伍依現有 `commandWindow` 產生的新語系提示。已知舊命令選單保留前方訊息，替換為完整新命令與姓名；其他已知提示、選單、分頁逐項依既有字串表更新。未知文字原樣保存，不猜譯或用改寫文字決定遊戲行為。同步更新戰術協程的已保存顯示快照，避免答完後恢復舊語言；已排入控制器的對白只更新文字。

隊伍、等待種類、已選命令、位置、移動力、亂數、游標、反白、分頁位置及輸入回呼保持；沒有存檔欄位變更。回歸驗證兩種目標語系及回切，包含未知前綴、數值與非文字欄位。實作入口為 `ui.BattleView.Relocalize` 及 `cmd/san1/windowbar.go`，回歸在 `internal/ui/battleview_locale_test.go`；正式兩版正常 GUI 依相同 `normal-r2.py` 重跑，後續其他畫面仍由 §6.36 追蹤。

修改後的無 overlay 建置入口為 `workplace/hd-locale-v25-build-after.sh`，輸出在 `workplace/hd-window/player/locale-v25-after/`，來源、工具及建置記錄為 `workplace/hd-locale-v25-build-after.json`，原始測試事件為 `workplace/hd-locale-v25-tests.jsonl`。使用同一 `normal-r2.py`，僅以 `SAN1_HD_LOCALE_OUT` 指向新目錄；兩版控制及修改後的完整原生圖與文字比較分開保存。

首輪回切測試揭露既有 `i18n.Relocalize` 把只有空白的 `%s` 模板當成已知提示，將未知紀錄回切成籍貫；原始事件、來源及目錄分別保存為 `workplace/hd-locale-v25-tests-first.jsonl`、`workplace/hd-locale-v25-first-source.json`、`locale-v25-after-first/`。修正契約只允許含文字或數字常量的格式模板參與辨識，並接受數字欄的填寬空白，保留解析後的數值。回歸入口包含 `internal/i18n/relocalize_test.go` 的未知文字雙向回切及 0／6／17／−6 填寬數值。

獨立回讀入口及收據為 `workplace/verify-hd-locale-v25-delivery.py`、`workplace/verify-hd-locale-v25-delivery.json`，核對兩組正式來源、控制組的四項真實失敗、修改後全部正常停點、測試原始事件、最新完整 PNG 及原生肖像／日期／命令文字。公開檢查入口與收據為 `workplace/verify-hd-locale-v25-publish.py`、`workplace/hd-locale-v25-publish-check.json`；Issue 同步依 `workplace/hd-locale-v25-issue-sync-plan.json` 及 `workplace/hd-locale-v25-issue-sync-receipt.json` 核對全文與 OPEN 狀態。這些私人收據不加入 Git。

修改後 R2 的 28/28 正常檢查通過。獨立下框比較發現八組均只差系統滑鼠游標的 2,322 像素，完整差異框為下框內 (52,4)–(115,67)；擷取入口的 x11grab 未關閉 `draw_mouse`。R2 保留，不遮掉差異；`normal-r3.py` 只將擷取改為 `-draw_mouse 0`，同鍵序、同 binary 重跑，輸出在 `locale-v25-after-r3/`，建置對照為 `build-after-r3.json`，前綴同上。原獨立讀取器保存為 `workplace/verify-hd-locale-v25-delivery-first.py`。這是擷取工具修正，不變更遊戲。

R3 兩版繁中、英文、日文及回切繁中的 28/28 正常檢查通過。UI、翻譯及控制器三個套件共 186 項測試通過，零 skip、零 fail。獨立回讀控制／修改後共 166 張最新完整 PNG，核對 124 份正式來源、固定素材包、原生高清曹操肖像、命令前三行及完整日期下框；全部通過。兩版英文與日文的四張完整高清畫面已查看。正式來源中 `Relocalize` 的使用端僅更新顯示文字，未接到規則、識別鍵或存檔；語言與行為分離契約沿用 `local/localization-display-semantic-isolation.md`，既有 JSON 母本維持專案契約。

§6.36 的三語系驗收仍為 DRAFT。完整畫面可見英語軍力面板的標籤／數值裁切、姓名與地名仍使用繁中字形，以及日文時刻欄的文字重疊；這些在控制組已存在，不是本次命令更新造成。下一步先追查 `battleInfo`、`DrawArtBattle` 的翻譯與字區，再依已量版面建立窄修正契約。主畫面、人物卡、對白、選單及其他戰場分支尚未完成此輪三語驗收；戰術保存快照與非命令提示目前只有回歸測試，不外推正常 GUI 完成。

### 6.36.2 戰場名稱與資料的完整顯示

**狀態：CONFORMED**，限下列正式顯示修正、回歸及兩版正常窄版主戰場停點。沿用 [014 §3.2、§7](014-art-main-overlays.md#32-下面板) 已定案的字級與原版面板。`battleInfo` 提供原始姓名及郡／州名，先前 `DrawArtBattle` 未套用既有 `PersonName`、`PlaceName`；英文面板仍保留繁中大字姓名欄，資料只剩 64 像素，且混入中文字使 ASCII 小字回退失效。v25 的兩版正常完整圖可重現軍力名稱及數值裁切。

先以私人 overlay 試作顯示層翻譯，資料與識別鍵維持原文。繁中座標、字型、全部像素保持；日文只依既有新字體對照。拉丁姓名使用既有面板的 96 像素資料區，完整保留統帥、君主、軍別、將數、兵、金及米；必要時使用 6×10 小字，最多九行，最後一行仍在 96 像素面板內。部隊與查看面板也必須從原始姓名翻譯，不要求呼叫者預先翻譯。空首將維持原有空面板。

英文郡名在原有 32×64 槽內用完整拼音折行，不取前兩個字母。州名沿用既有「放不下才拿掉州字再轉」契約；天候在原有 32×16 槽內量寬後選字級。英文下框日期須完整顯示既有 `Date.FormatWithSeason` 的年號或西曆、年、月及季節，留在原有日期範圍，保留雙色網點；繁中十槽保持。

前輪對日文時刻欄的重疊判斷須訂正：v25 兩版完整 32×96 日數／時辰框與繁中逐像素相同，雜湊均為 `f944aad0036ed62baf83fbfbb3648c4000a3e5ec0a387eef9d04b3703ca78c13`。原排法的時辰／數字在 x 8–23，「時」在 x 24–39；同列的兩塊字區沒有重疊，不改原排法。此項是 remake 截圖及來源幾何回讀，未新增原版 oracle。

私人入口沿用 `workplace/hd-locale-v26-` 前綴：`prepare.py`、`prototype-artbattle.go`、`prototype-artscreen.go`、`probe_test.go`、`overlay.json`、`control-overlay.json`、`run-probe.sh`、`before.json`。控制與試作輸出沿用 `workplace/hd-window/player/locale-v26-control/`、`locale-v26-prototype/`，各有 `probe.json`。以兩版六劇本的實際姓名與郡／州資料、四軍及上界數值核對完整文字、原貌字區與繁中不變；高清及正常玩家路徑另驗，不以直入畫面代替。證據足以描述輸入／輸出及邊界後再轉 READY；不改人物、規則、亂數、等待、肖像或存檔。

首輪兩版六劇本共 347 種原始姓名、42 郡名，拼音姓名最長 12 格，郡名最長 9 格。九行試作將數值上界截字由 128 組降至 4 組，剩餘皆為原版模板名「新君主」，既有 `PersonName` 原樣回退。R2 只在戰場顯示層將此模板名對到現有 `title.newLord`，不更改會寫入新局的 `title.newLordName` 或全域姓名轉換。原版姓名之外的未知自訂姓名仍按既有整串原文回退，不猜譯；混合文字的完整排版另列限制。首輪全部試作來源保存於 `workplace/hd-locale-v26-first-source.json`，R2 控制／試作輸出為 `locale-v26-control-r2/`、`locale-v26-prototype-r2/`，另加入實際模板名的完整畫面。

R2 包裝腳本先誤改來源／overlay 檔名，檢查全部輸入路徑後修復；兩組 Go 原始測試均 PASS，彙整器卻未接受 Go 空列表的 JSON `null`。R2 來源與失敗保留於 `workplace/hd-locale-v26-r2-source.json` 及原目錄；修復摘要後在同一映像、同一命令乾淨重跑，R3 輸出為 `locale-v26-control-r3/`、`locale-v26-prototype-r3/`。兩次檔名失敗後已讀取規則 40／41，不以固定重試取代全部路徑核對；這些是驗證腳本問題，沒有新增產品失敗。

R3 的 Go 原始測試均 PASS，但候選輸出環境仍指向 R2，彙整讀錯 R3 路徑。停止替換檔名前綴，改用 `workplace/hd-locale-v26-run-probe-final.sh` 的兩個固定輸出環境變數，Go 與彙整器讀同一變數；逐項檢查輸入及新目錄後乾淨重跑。最終控制／試作輸出為 `locale-v26-control-final/`、`locale-v26-prototype-final/`，建置來源與工具版本為 `workplace/hd-locale-v26-probe.json`。先前目錄與失敗保留；不把包裝腳本錯誤稱為產品失敗。

試作獨立回讀入口與收據為 `workplace/verify-hd-locale-v26-prototype.py`、`workplace/verify-hd-locale-v26-prototype.json`，逐張核對完整控制／候選 RGBA、繁中全圖、文字矩形外、肖像及圖框、日文時辰、來源三表、尺寸、雜湊與擁有權。

最終兩組 Go 試作確實執行且 PASS，未 skip；排版判定另讀完整收據。347 種原始姓名乘四軍共 1,388 組，控制 124 組裁切，試作 0 組；最多九行。兩版六劇本的十二組來源三表相同，42 郡名保持來源。寬窄版、主場／部隊／查看／新君主模板、三語共 48 張控制與 48 張候選完整 640×408 圖獨立回讀，繁中 16 張全圖相同，日文日數／時辰框保持。

首輪獨立回讀在天候框外發現六個差分，原檢查器與失敗保存於 `workplace/hd-locale-v26-independent-first.json`。查明是控制組的 Clear 以 8 像素字寬畫到 x 47，超出 x 8–39 字區；候選改用 6 像素字寬後，原白色墨點消失。最終檢查不放寬字區，只逐點核對這些舊溢字已恢復為同 fixture 的繁中背景。十六張英文圖共恢復 96 像素，其餘字區外、肖像及框線保持。讀取腳本曾使用不合法的 Python comprehension 語法，修正後同環境重跑，屬驗證腳本問題。

獨立收據 SHA-256 為 `c580a7aee28d204326ec9084b09e9e37883b7be91c9213b4183cef4b7ebbf3f6`，實際六份私人來源、兩組原始測試日誌、96 張 PNG 雜湊及擁有權均符合建置收據；117 份正式來源保持。此回讀階段為 DRAFT，當時試作尚未進入正式程式；全日期／州名邊界、高清疊層與正常玩家路徑待補，不宣稱原版 parity、存讀檔或整份三語畫面驗收。

本輪文件與公開檢查入口為 `workplace/verify-hd-locale-v26-docs.py`、`workplace/verify-hd-locale-v26-publish.py`。Issue 候選與變更計畫保存於 `workplace/hd-locale-v26-issue-sync-plan.json`、`workplace/hd-locale-v26-issue-candidates.json`；只修改 #104、#107、#110 的目前狀態，#108、#109 保持全文。私人來源、候選圖及完整提示詞不入 Git。

正式接入前的日期／州名邊界入口為 `workplace/hd-locale-v27-boundary_test.go`、`workplace/hd-locale-v27-boundary-overlay.json`、`workplace/hd-locale-v27-run-boundaries.sh`；沿用已回讀的 v26 候選，不覆寫原試作。日期以完整單行字模另做整行高度縮放，再核對候選雙色網點的每個像素；涵蓋十七年號的起訖、表外、西曆、整數上下界與十二月份。十四州、四十二郡與三種天候按現有名稱與原字區核對。收據為 `workplace/hd-locale-v27-boundary.json`、`workplace/hd-locale-v27-boundary-proof.json`。

首次邊界驗證在 240 秒外層逾時終止，來源及原始事件保存於 `workplace/hd-locale-v27-boundary-first.json`。檢查器每個日期反覆解析字型，改為共用已載入的兩張畫布並逐組清空；日期集合、字模比較及容器配額保持。修正後原始事件另存 `workplace/hd-locale-v27-boundary-r2-tests.jsonl`，不覆蓋首輪日誌。

行高防護另以 `workplace/hd-locale-v27-prototype-artbattle.go`、`workplace/hd-locale-v27-fallback_test.go`、`workplace/hd-locale-v27-overlay.json`、`workplace/hd-locale-v27-run-candidate.sh` 試作。資料能完整使用 ASCII 小字時才增加統帥首行；混合姓名維持既有回退。原尺寸判斷同時量行數，七行不能使用 16 像素行高。輸出為 `workplace/hd-window/player/locale-v27-prototype/`，收據為 `workplace/hd-locale-v27-candidate-proof.json`；須與 v26 的 48 張完整候選圖一致。接入前正式來源、十八個原版容器檔、字型、語系與素材包雜湊保存於 `workplace/hd-locale-v27-before.json`。

READY 最小契約於 2026-10-04 經上述證據審查成立。960 組日期包括四十個代表年份、十二月份及兩種曆法，完整字模與雙色網點相同，最寬 304 像素，小於原 (128,378) 的 384×23 字區。十四州最寬 30 像素；四十二郡完整折行留在 32×64，三種天候留在 32×16。邊界收據 SHA-256 為 `11aef339e5109a4028201ec7eecf668b7cba440e7401cfd70c3cbe1e8b85b7a1`。行高防護的 1,388 組排版零裁切、兩個試作測試零 skip，48 張完整圖與 v26 的 PNG bytes 相同；收據 SHA-256 為 `9e1c46ec23b6004f197713d1c9c662254915b413220e7eb1fc9c94af9f74656f`。

- 證據等級為 `[both] L1 remake 顯示驗證`。來源是兩版 DATA1／DATA2／DATA3 的 NAM、IDX、GRP 共十八檔，雜湊與 124 份接入前正式來源、四套字型及三份語系目錄均在 `hd-locale-v27-before.json`。工具為既有 `rich2-go-ebiten:latest` 的 Go 1.24.13、Xvfb；原字區定位回到 `assets.BattleLayoutFor`、`BattleLeftBox`、`BattleDate*` 與 [005](005-main-screen.md)。英文排法為已授權的 remake 差異，未新增原版 oracle 聲明。
- typed input 為 `ArtBattleInfo`、`UnitPanel`、`InspectPanel`、`game.Date` 與現有戰役資料。繪圖時套用既有姓名／地名譯名及 `title.newLord`；識別、比較、原始人物名、`title.newLordName`、存檔、規則、亂數、等待與美術來源均保持。`battleInfo` 仍傳原文，呼叫者不預先翻譯。
- 本節授權修改 `internal/ui/artbattle.go` 及 `internal/ui/artscreen.go` 的顯示。原始姓名、主軍／援軍、統帥與君主不同、米列、部隊／查看與模板名須由正式 renderer 回歸。空首將維持空面板；未知姓名整串原文回退，混合文字的完整排版仍是未完成限制，不猜譯。
- 驗收依序為正式來源回歸、兩版從片頭開 001 曹操新局、正常出兵及紮寨、繁中／英／日與回切、原貌／高清切換。完整命令、軍力、左欄及日期字區須在 4× 下與原貌最近鄰相同，原貌恢復保持；原生高清肖像、圖框與原始資料雜湊另核對。尚未通過前不稱 CONFORMED；其他畫面、音訊、平台與整份 HD 仍依 #104、#110 推進。

正式原始姓名、資料／肖像區、模板名及完整日期回歸在 [artbattle_locale_test.go](../../internal/ui/artbattle_locale_test.go)。無 overlay 的正式測試與建置入口為 `workplace/hd-locale-v27-build.sh`，收據為 `workplace/hd-locale-v27-build.json`，輸出沿用 `workplace/hd-window/player/locale-v27-after/`；只允許兩份 UI 來源改變，原始容器、字型、語系及素材包保持。

正常玩家驗證入口為 `workplace/hd-locale-v27-normal.py`，必須指定 `SAN1_HD_LOCALE_OUT=/src/workplace/hd-window/player/locale-v27-after`，使用該目錄的正式建置。保存七個完整字區的原貌、原生高清與恢復圖，不裁掉命令末行或隱藏差異；系統滑鼠不入擷取。完整 PNG 與收據留在該目錄。

完整命令末行仍有原有六格輸入游標。首輪 132 項中 12 項相位差、其餘六個完整字區全數通過，保存 `workplace/hd-locale-v27-normal-first.json`。相位同步重跑入口為 `workplace/hd-locale-v27-normal-r2.py`，輸出改為 `workplace/hd-window/player/locale-v27-after-r2/`；所有候選截圖保留，只有等到相同完整命令與游標相位才比較，未遮罩或改變遊戲時間。

混合統帥與已知君主的回退另用 `workplace/hd-locale-v27-mixed-commander_test.go`、`workplace/hd-locale-v27-mixed-control-overlay.json`、`workplace/hd-locale-v27-run-mixed-control-r2.sh`，實際 renderer 核對面板下完整空隙。新增統帥首行前必須連同統帥確認 ASCII 小字可完整顯示，維持原本六行回退，不能新增第七行溢出。這項防護未解決未知混合姓名的完整顯示。

控制組英文混合統帥「Bob龘」與已知君主「劉備」確實在 (540,143) 溢出，繁中及日文通過；負例、完整候選來源及原始事件保存於 `workplace/hd-locale-v27-mixed-negative.json`。修正前述防護並將同一負例加入正式 renderer 回歸。控制腳本首次誤設 `/opt/gopath`，沒有執行產品測試，已改用既有專案快取；首輪空日誌保留。相位分析曾將首墨列當成槽起點，依既有 8×16 槽界線訂正，入口及收據為 `workplace/verify-hd-locale-v27-first-diff.py`、`workplace/hd-locale-v27-first-diff.json`，初次判斷保存在 `workplace/hd-locale-v27-first-diff-first.json`；十二個完整差分均只落在原有游標槽。

最終建置入口為 `workplace/hd-locale-v27-build-r2.sh`，收據為 `workplace/hd-locale-v27-build-r2.json`，原始回歸日誌為 `workplace/hd-locale-v27-r2-tests.jsonl`。三套件 212 項通過，零 skip、零 fail，含混合姓名的三語實際繪圖回歸。正常操作沿用 `hd-locale-v27-normal-r2.py`，指定 `SAN1_HD_LOCALE_OUT=/src/workplace/hd-window/player/locale-v27-after-r3`，兩版各從片頭開 001 曹操、難度 5 新局、出兵鄴郡與合法紮寨；繁中／英／日與回切八個停點共 132/132。

獨立回讀入口及收據為 `workplace/verify-hd-locale-v27-delivery.py`、`workplace/verify-hd-locale-v27-delivery.json`，SHA-256 為 `a766b050aea91caa171699a1a8cb2bc06355fcb7e8fc9981d20754b32476d9fb`。143 張最新完整 PNG、24 組有界相位擷取、七個完整字區的原貌／4× 最近鄰／原貌恢復、攻方原生曹操肖像及朝向、兩側完整圖框均通過；繁中六個靜態字區及日文時刻原框對 v25 保持。兩版英文高清圖已目視，統帥、君主、軍別、將兵金米、州郡及日期完整留在既有字區。

十八份原始容器、四套字型、三份語系與 v21 素材包保持；618 筆登錄與 309 PNG 身份及尺寸相同。124 份建置輸入包含 119 份正式來源／依賴檔及五份既有工具試作，本輪只有兩份 UI 正式來源改變，回歸檔另記雜湊。修正只接顯示層，未新增存讀檔、原版 oracle、音畫或平台完成聲明。部隊／查看、寬窄版與上界數值屬正式 renderer 回歸，不外推三語正常子畫面；未知混合姓名完整排版、其他畫面及整份 HD 仍待完成，§6.36 留 DRAFT，整份 021 留 READY。

文件與公開檢查入口為 `workplace/verify-hd-locale-v27-docs.py`、`workplace/verify-hd-locale-v27-publish.py`。本輪新來源、候選、原版資料、素材包、完整提示詞及 PNG 收據保持本機；README 既有四張展示圖保持。

### 6.36.3 主畫面與人物卡的完整譯名

**狀態：CONFORMED，限已量裁切欄位及正常 Theme 切換**。沿用 014 的既有面板、小字與英文日期契約，核對兩版正常 001 曹操新局的主畫面與人物卡，逐語系切換原貌、原生 4× 與原貌恢復。完整畫布只排除實際肖像內部，圖框、日期與下面板全部比較；游標依下述已證實六格契約逐像素核對，不遮掉游標。本節不宣稱所有主畫面文字都已即時翻譯。

私人入口為 `workplace/hd-locale-v28-normal.py`、`workplace/hd-locale-v28-probe_test.go` 及 `workplace/hd-locale-v28-run-probe.sh`。正常控制組沿用 v27 正式執行檔，輸出在 `workplace/hd-window/player/locale-main-v28-control/`；資料檢查以兩版六劇本的原始州郡、人物、所屬與能力值為輸入，記錄完整譯名及目前實際準備的顯示字串，不以截短後的量寬證明資訊完整。

兩版六劇本共 12,600 個人物／語系案例及 1,158 個有主州郡／語系案例。首輪 3,276 組英文籍貫差異中，3,072 組只是既有去除州字的授權排法；另 204 組確實截短州名，例如「Jing Xiangyang」成為「Jin Xiangyang」。主事者姓名另有十組實際裁切，公孫瓚、夏侯淵與向寵均少了最後一個字母。實際君主、軍師及人物卡姓名沒有這項裁切，不擴大修改。原始收據為 `workplace/hd-locale-v28-probe.json`，分類收據為 `workplace/hd-locale-v28-classification.json`；354 份受版控 Go 來源及依賴、十八個原始容器、四字型、三語系與 v21 包雜湊在 `workplace/hd-locale-v28-before.json`。

2026-10-04 READY 證據審查：`[both] L1 remake 顯示驗證`，未新增原版 oracle。完整籍貫沿用先拿掉州字再轉的政策，最長十四個 ASCII 字，6×10 字級寬 84 像素，留在原 (424,84) 的 104×16 槽；只有 8×16 確實放不下且所有字模存在才用小字，垂直置中。主事者拼音在原 (536,220) 的 80×16 槽也依相同量寬判準完整顯示。正常尺寸可放入的文字與繁中／日文排法保持；小字缺字或未知混合姓名維持原有回退，不猜譯。

本節授權 `internal/ui/artscreen.go` 與 `internal/ui/card.go` 的顯示修正，以及實際字模、六劇本完整籍貫與主事者姓名回歸。原始人物、識別鍵、規則、亂數、等待、肖像身份與存檔不改。正式建置不得帶私人 overlay；兩版正常操作及獨立全圖回讀通過後才標記本節 CONFORMED。其他畫面與整份 HD 保持未完成。

驗收器的游標前提於本輪重新審查。強求三張擷取圖具有同一相位會漏掉實際顯示的合法幀；兩次差分都只在原有 8×16 游標，變化擷取節奏的單停點通過，完整補驗仍有一次取樣失敗。原控制與失敗圖全部保留，沒有改 TPS、速度或等待。改採 [014 §4.1](014-art-main-overlays.md#41-輸入游標l0l1baseissue-78) 已證實的六格輸入游標契約，入口為 `workplace/hd-locale-v28-cursor_test.go`、`workplace/hd-locale-v28-cursor-compare.py`：兩版 DATA1 的 sprite／mask 與下面板底色 2 依 `(底 AND 遮罩) OR 圖` 核對全 128 像素。比較仍涵蓋完整畫布；只允許最後一個字後同一 8×16 格內的六種已知相位，原貌與目標的整格都須符合原始圖，其他差異一律拒絕。高清另核對完整最近鄰複製；游標不遮罩、不改遊戲時間。

第三次原始失敗的全部恢復候選均通過上述有限預期，字區外差異與非法游標的兩個反例均拒絕，收據為 `workplace/hd-locale-v28-finite-cursor-proof.json`。原版已通過的八組正常停點保持，不重拍；加強版依 `workplace/hd-locale-v28-normal-r4.py` 從正常新局補驗八組。長姓名與籍貫的正常入口為 `workplace/hd-locale-v28-defects-r3.py`，兩版查看郡 2 的公孫瓚資料，再到郡 15 經他國確認選第三位華雄，繁中／英文及回切共十二組，仍採實際同相位全圖比較。

正式無 overlay 建置及原始事件為 `workplace/hd-locale-v28-build.sh`、`workplace/hd-locale-v28-build.json`、`workplace/hd-locale-v28-tests.jsonl`。UI、翻譯與控制器共 215 項通過，零 skip、零 fail。新回歸在 [card_locale_test.go](../../internal/ui/card_locale_test.go)，12,600 個實際籍貫案例涵蓋 123 組完整字模，386 個英文主事者案例涵蓋 105 種姓名。將同一回歸對回修改前的兩份顯示來源，三項皆確實失敗；小字缺字與未知混合姓名仍保留原回退。

最新正常曹操主畫面／人物卡共 16 組、82/82；原版取自 R2 已通過的八組，加強版取自 R4 的八組，使用同一份正式修改後執行檔。長字串兩版正常查看共 12 組、50/50。獨立入口與收據為 `workplace/verify-hd-locale-v28-delivery.py`、`workplace/verify-hd-locale-v28-delivery.json`，972 張完整 PNG 的雜湊、尺寸及擁有權符合；完整高清字區與圖框、原貌恢復、F000／F063／F052 原生肖像及六格游標均通過。兩種真實缺陷的四個英文欄位在控制組缺字，修改後完整字模相同；16 組繁中上面板及日期與控制相同。公孫瓚、華雄兩張加強版英文高清圖已查看。

左側日期另以 `workplace/hd-locale-v28-date_test.go` 與 `workplace/hd-locale-v28-date-proof.json` 核對兩版原貌圖的全部旋轉英文墨點，均為完整「Zhongping 6, month 1, Spring」。目視曾將旋轉字形誤認為繁中，字模回讀已排除，不列產品缺陷。真正仍保留繁中的已完成數字提示由 §6.36.4 接續。354 份接入前來源與依賴、355 份修改後來源、十八原始容器、四字型、三語系與 v21 包保持既定範圍；只有兩份正式 UI 行為檔及回歸改變。未新增原版 oracle、存讀檔、音畫或平台聲明，§6.36 仍 DRAFT，整份 HD 仍 READY。

### 6.36.4 已完成數字提示的即時切換

**狀態：CONFORMED，限顯示快照與已驗正常數字路徑**。§6.36.3 的兩版正常公孫瓚郡資料在英文停點仍顯示「查看那一郡」及 `(1-42):2`；英文 `ask.pref` 已有完整譯文，人物卡的「請按任一鍵」也能切換。證據為 `workplace/hd-window/player/locale-main-v28-defects-after/receipt.json` 及其兩版英文原貌／高清 PNG。左側英文日期已由完整字模核對，與此缺口分開。

2026-10-04 證據審查：`[both] L1 remake 顯示驗證`，未新增原版 oracle。`cmd/san1/roster.go` 的 `askPref` 經 `askRange`／`showNumber` 組出標題、`pick.range` 與原始輸入數字。`main.go` 的 Enter 路徑清掉 `a.num`，查看完成回呼只更新所選郡及狀態面板，保留整句 `View.Prompt`。`windowbar.go` 目前只將整句交給 `i18n.Relocalize`；它要求完整匹配字串表，不能辨認這種組合。翻譯掛勾的正式呼叫者均屬視窗、選單或戰場顯示，未接到規則比較、識別鍵或序列化。

本節授權在 `showNumber` 保存顯示用的標題、上下限、bare 旗標、原始 digits 與已畫整句。這份快照不保留輸入值、回呼、玩家狀態或存檔欄位。語言切換時只有數字輸入已完成、目前提示仍完整等於快照的已畫整句，才翻譯標題並以目標語系的 `pick.range` 重新組合。正在輸入仍走既有 `a.num` 更新；其他提示走原有完整匹配。畫面已被新提示取代時不得重播舊快照；未知標題保留原文，不拆猜片語。

空輸入、前導零、尚未確定的超界數字、bare 提示、不用原版美術的文字版面，以及完成回呼都保持原有契約。切換語言不確定輸入、不重播回呼、不改所選郡、規則、亂數、等待或存檔。連續運輸／調動的多段標題仍沿用既有整句匹配，未證實的組合不得拆猜；不以本節宣稱所有數字標題已完整翻譯。

正式回歸須涵蓋三語往返、正在輸入及已完成、前導零、bare、未知標題與被新提示取代的負例；正常玩家路徑須在兩版從新局進入查看郡，先切換正在輸入的提示，再確認已完成提示、原貌／4×／回復及繼續輸入／取消。來源與依賴、十八原始容器、四字型、三語系、v21 包與修改前正式執行檔的實際雜湊在 `workplace/hd-locale-v29-before.json`；原始輸入保持唯讀。通過正式建置、正常操作與獨立完整字區回讀後才標記本節 CONFORMED；整份 HD 仍 READY。

正式無 overlay 建置與回歸在 `workplace/hd-locale-v29-build.sh`、`workplace/hd-locale-v29-build.json`。UI、翻譯與控制器共 223 項、零 skip、零 fail；[number_locale_test.go](../../cmd/san1/number_locale_test.go) 涵蓋上述邊界。舊 `windowbar.go` 對同一完成提示回歸確實失敗，收據為 `workplace/hd-locale-v29-negative-tests.jsonl`。只有兩份控制器行為來源及新增回歸改變；356 份實際來源、原始容器、字型、三語系、v21 的 618 筆及 309 PNG 均獨立核對。

兩版從正常 001 曹操新局查看郡 `02`，先切換正在輸入的提示，再 Enter 完成；另輸入 `43` 重問、輸入 `02` 後刪兩位、查看 `2`，取消後重新查看 `2`。五種停點逐語系及回切、原貌／原生 4×／原貌恢復共四十組、220/220。九組完整來源字模涵蓋三語與空數字／`02`／`2`，下面板的 192×64 全區及每個游標像素均核對。控制組八個英文／日文完成提示保留繁中，修改後皆正確；完成後所選郡與原生 F063 肖像保持。兩版英文高清完成圖已查看。

原版來源擷取在完成二十組後進入加強版，因 720 秒外層上限退出 124，`receipt.json` 未保存；不補造它。原版 110 項由 `workplace/hd-locale-v29-base-capture-proof.py` 直接核對保存的完整圖及凍結來源；加強版另用 `workplace/hd-locale-v29-normal-r3.py` 從正常新局完成二十組、110/110。最終入口 `workplace/verify-hd-locale-v29-delivery-r2.py` 與 `workplace/verify-hd-locale-v29-delivery.json` 合併這兩類證據，核對 809 張現存完整 PNG。重複的開機擷取檔名只核對現存最後一筆，不宣稱舊覆寫幀仍存在。

正在挑郡的上面板是清單，沒有肖像；`workplace/hd-locale-v29-active-full-proof.json` 另核對兩版及控制／修正組的四十八張原貌／高清配對，完整畫布沒有排除任何上面板矩形。只有完成後的資料面板才排除真正的肖像並獨立核對原生素材。游標沿用 §6.36.3 的六格有限預期，不遮罩或改時間。原版擷取收據缺失仍明示，未新增原版 oracle、存讀檔、音畫、效能或平台聲明。挑郡譯名的裁切另由 §6.36.5 接續，組合標題、其他三語畫面與整份 HD 仍未完成。

### 6.36.5 挑郡清單的完整譯名

**狀態：CONFORMED**，限目前四十二郡的三語清單與下列正常路徑。§6.36.4 的正常英文停點原將 `Liaodong` 畫成 `Liaodo`；來源圖為 `workplace/hd-window/player/locale-main-v29-after-r2/base-active-02-en-original.png`，加強版同類完整圖在 `locale-main-v29-after-plus-r3/`。根因是 `DrawPrefPick` 先將完整譯名限為六格；目前已依下列 READY 契約修正。原始資料與選擇編號保持。

證據分級：來源身份為 `L0 [both]`；本節畫面與正常路徑為 `L1 [both]` 的 remake 驗證，不稱原版對拍。

量測入口為 `workplace/hd-locale-v30-probe.sh`，從兩版 DATA2 的 NAM／IDX／GRP 載入六劇本，逐一量四十二郡、三語及完整編號字串，共 1,512 列。輸入的十八容器、四字型、三 catalog、356 份目前來源與舊正式執行檔雜湊記在 `workplace/hd-locale-v30-before.json`；Go 1.24.13，工具為既有 `rich2-go-ebiten:latest`，量測只使用私人測試 overlay。原版座標及呼叫端沿用 [spec/014 §4.4](014-art-main-overlays.md)，位址空間與原版證據不變。本節只授權顯示差異，不新增原版 oracle 聲明。

| 已量項目 | 結果與證據範圍 |
|---|---|
| 中文、日文完整編號與地名 | 最長六格，8×16 字級需 48 px；每語各 504 列，原版三欄座標足夠 |
| 英文完整編號與地名 | 最長十一格，8×16 需 88 px、6×10 需 66 px；`Shangdang`、`Yingchuan`、`Xiangyang`、`Yongchang` 不能用 64 px 小字槽完整容納 |
| 表頭 | 中日各 24 格、192 px；英文 26 格、208 px，原直接繪製會覆蓋右框。英文三個 catalog 欄名各七格，6×10 每欄 42 px |
| 實際框線 | `workplace/hd-locale-v30-bounds.json`：表頭與底部角框內側 x 424–616，正文兩側直框內側 x 416–624。候選的實際墨點均未碰框 |
| 完整試作 | `workplace/hd-locale-v30-prototype-r3.json` SHA-256 `5904c8556c91f0913a81eb01d277b48861ca4416d5c7c5276fd49d7cb6615408`；兩版六劇本三語共 1,512 列的完整字模、編號、有效／無效字色及框線通過；中日整張畫布與目前實作相同。兩版英文及中文候選已目視核對 |

正式契約如下，英文排法是 remake 差異，原版三欄十四列及輸入方式維持。

- 先量 `%2d` 原始編號加完整 `PlaceName`。至少一列原字級超過 64 px、既有小字涵蓋整串且能放進 66 px 時，清單採緊湊排法；不以語言代碼直接判定。
- 緊湊排法三欄起點為 x 424、490、556，欄距 66。完整 ASCII 列使用既有 6×10 字模，原 y 60＋16j 的行框內向下置中 3 px。目前兩版四十二郡皆完整；無縮寫、無改字、無新字型。catalog 表頭可分為三個且皆放得下的欄名時，在各欄 x、y 47 畫完整欄名。
- 未採緊湊排法時維持 x 432＋64k、y 60＋16j。中日文目前整張輸出保持。混合或未知文字、缺小字及未來更長的名稱保留安全的原字級回退，仍需獨立完成驗收；不得拆字猜譯。表頭回退限制在原 192 px 文字範圍。
- 只改字模與文字座標。`Valid`、選擇編號、原始地名、callback、規則、亂數、等待與存檔不變。
- 驗收須包含兩版六劇本三語的完整字模、所有編號與兩種字色，舊正式 renderer 的負對照，以及正常片頭、新局、查看、非法編號重問、三語回切、原貌／原生 4×／恢復與選郡完成。完整上面板及框線逐像素核對；正在挑郡不排除任何肖像區。只有完成這些檢查才可轉 CONFORMED。

正式驗收為 UI／翻譯／控制器 225/225，零 skip、零 fail。新增全來源清單回歸覆蓋兩版六劇本三語 1,512 列、所有編號及有效／無效字色；缺小字與未知混合文字另有回退測例。把修改前 `prefpick.go` 接回相同回歸後，十二個英文劇本樣本確實失敗，不是編譯或環境失敗。正式執行檔未帶私人 overlay，SHA-256 `47618049acb1bfec3214d9e2f81abc13cc669e31ac6aaec745b5524aefacf771`；357 份來源及原始容器、字型、catalog 的身份皆回讀核對。

`workplace/hd-locale-v30-normal.py` 從兩版正常片頭與 001 曹操難度 5 新局，依序查看 `43` 重問、空輸入、正在輸入 `02`、確認郡 2、取消後重新查 2。兩種清單停點各作三語與回切、原貌／原生 4×／恢復，共十六組、108/108。對照使用修改前正式執行檔，四個英文正常停點的原貌及 HD 完整面板均不符完整字模；修正後四個皆相同。正在挑郡的整張畫布不排除任何矩形，確認後只於原生比對排除實際 F063 肖像 64×80，另核對該肖像全部原生像素。每次切回原貌仍核對全圖。六格游標依原始來源核對整格，不遮罩或改時間。兩版英文完整高清圖已查看。

獨立回讀入口為 `workplace/verify-hd-locale-v30-delivery-r2.py`，收據 `workplace/verify-hd-locale-v30-delivery.json` SHA-256 `073dbc287d63ded0434df8eabfcd15465f807b6616a322deb243e4530fc5ca16`。474 張完整 PNG、六組完整清單字模、所有正常與控制畫布、數字提示、原生肖像及 v21 的 618 筆／309 張素材全部回讀通過。擷取收據每個檔名唯一，每組完成即保存部分收據，四段皆完成；不沿用上一輪被覆寫的開機幀或缺失收據。README 四張已授權展示圖保持。未新增原版 oracle、存讀檔、音畫、效能、跨平台或 Release 聲明；其餘三語提示、對白、選單、混合姓名與 HD 全計畫仍待完成。

## 6.37 戰場旗幟

**狀態：READY**。B 寫實手繪與 4× 沿用使用者定案。本節先接一面主攻中軍旗；其餘十九面依同一契約後續製作。候選、完整提示詞與準備收據只留 `workplace/`；READY 不代表二十面素材或正常玩家驗收已完成。

來源與使用端：

- `[both] L0`：兩版 DATA1 三件套逐檔相同。GRP SHA-256 `958f44fe45e38624401af55f033ffb037bd3211a037eadbce90f827637d977a5`、IDX `9b89f9bdab4109a6ae35203bd0f787c9f7fdb977a81e5c317d75642de8110b8d`、NAM `8baf9d9a0b6fbe10ec035da221e1a14c3121abd6686b6d3ebc44a1883bab10b9`。正式 `assets.OpenContainer` 讀到 217 項，`assets.UnitFlags` 逐版驗證 24 張。
- `[both] L0`：四組各五面 24×15 旗，共二十面；第五號是 16×15 城門圖。兩版全部四十八個來源記錄的尺寸、索引像素、檔案位移及雜湊已回讀。私人入口 `workplace/hd-flags-v31-source.go`、收據 `workplace/hd-flags-v31-source.json`，工具 Go 1.24.13，位址空間為 DATA1.GRP 檔案位移。既有原版載入、繪製及城門未使用的證據見 [005 旗幟](005-main-screen.md#部隊的標記是旗幟)與 [RE 戰場疊層](../re/05-battle-layer.md)；不新增跨版原版行為對拍聲明。
- 試作來源 `DATA1/WFLAGA00.IMG` 在檔案區間 [624254,624438)，原始記錄 SHA-256 `4b326aec3ae147cf12bf7dea9b336620a4c13fcea7d7034545d4e6a1514de161`。保留原有向右旗形、紅色陣營與旗面隊伍辨識。守方兩組的凹尾旗形不同，不以攻方樣圖代替。
- `[both] L0`：實際二十面旗只用原色及黑色，RGB 三通道取補數與來源索引 XOR 15 逐像素等價。四面未使用城門含 EGA 棕色，沒有此等價性，不擴大開放。高清繪畫的新色彩及選取補色屬 remake 美化，不稱原版高清像素對拍。

試作與下一閘門：

- AI 只生成旗面及布料，不生成「帥、先、左、右、後」。隊伍文字在執行時用目前選定的既有字型另畫，不烘焙原版字模或鎖死字型。沿用旗上原作識別字，不因介面語系換字。原旗的識別字位於左側，攻方來源差分另含網點相位，不能直接當作字模遮罩。高清文字安全範圍為 [4,4,64,56)，16×16 字模三倍從 (10,6) 繪製，完整字模在框內。缺字或字模形狀不符時整面回退原圖，不顯示無字高清旗。
- 準備圖為 96×60，不透明，完整縮放且不裁切。原貌仍讀玩家的原版來源。高清選取、兵力牌、旗面後畫的面板及文字須分層，不能讓新旗蓋掉兵力牌或其他部隊。
- READY 前須查看真實來源及候選，確認旗形、陣營、五字覆蓋、反白，以及正式 `DrawArtBattle`／`DrawArtField` 的繪製順序。對戰子畫面使用原有將領標記，不擅加旗；四張城門不接入。
- 接入後須有兩版寬窄主戰場、選取／未選取、重疊部隊與缺圖回退的正式像素檢查，並從正常片頭、新局、整編及紮寨走到戰場，核對原貌／4×／回切與兵力牌。存檔、規則、亂數、TPS、等待、音訊與 Release 不由此美術批次改寫。

主攻中軍紅旗候選及五字三字型的三十張原色／反白試作已查看。`workplace/hd-flags-v31-prototype.go` 保存完整縮放、文字範圍、各字墨點及每張 5,760 像素的補色檢查，結果見 `workplace/hd-flags-v31-prototype.json`。原稿 1586×992，比例交叉誤差 18，符合既有一個來源像素容差；完整縮成 96×60，無裁切。此為 Codex 技術及候選審查，使用者逐張簽核仍為 0。

正式契約新增 DATA1 的 `WFLAG[DA][01][0-4].IMG` 二十個鍵；來源須為 24×15，二十面的原色必須符合 RGB 補色與索引 XOR 15 等價，高清 PNG 必須不透明。第五號城門、錯容器、錯尺寸、錯來源或透明圖逐項停用。完整包上限從 618 增至 658 筆，schema 仍為 1。每個鍵仍有獨立來源雜湊，可共用已驗證的無字旗面 PNG。

載入器另登錄原圖及其索引反白。Canvas 按目前字型合成隊伍字，快取以素材身份及字型分開，換楷書／隸書不沿用前字型字面；反白為合成後 RGB 三通道取補數，alpha 保持 255。原貌畫布及選取狀態不改寫。地形先畫，各部隊按原有順序畫旗及兵力牌，後一面缺圖時仍保留其原旗覆蓋權；場地線、面板、肖像與文字最後覆蓋。對戰子畫面持續使用原有將領標記。`DrawArtField` 同樣保留部隊次序與兵力牌。

正式 UI／assets／翻譯／控制器共 299 項通過，零 skip、零 fail。兩版正常片頭、001 曹操新局、出兵鄴郡、整編及合法紮寨，原貌／4×／回切與缺旗回退共四段、38/38。完整旗面 96×60、兵力牌 160×68 與原貌恢復逐像素相同；未以此宣稱全畫布回復或原版 oracle。第一次擷取的系統滑鼠在旗面上，保留失敗圖；R2 只移開系統滑鼠，沒有遮罩、改時間或改遊戲。兩版高清完整圖已查看。

最新私人包為 `workplace/hd-assets-flags-v31/`，兩版各 310 筆，共 620 筆、310 張 PNG；正式載入無警告。舊 v21 全部 618 筆欄位、PNG bytes 及準備紀錄保持。素材、來源、字型、正式建置與正常圖檔的獨立回讀入口為 `workplace/verify-hd-flags-v31-delivery.py`，收據 `workplace/verify-hd-flags-v31-delivery.json`；正式測試與建置見 `workplace/hd-flags-v31-tests-r2.jsonl`、`workplace/hd-flags-v31-build.json`。本節保持 READY，其餘十九面尚未製作；不新增音畫、存檔、平台或 Release 聲明。

## 6.38 操作面板

**狀態：DRAFT**。使用者要求操作畫面的面板也 HD 化。沿用已定案 B 寫實手繪、4×、原貌預設及原版位置。範圍包括主畫面資料／指令、人物卡、選單、戰場軍力／指令／查看與對話框。先核對各使用端，再製作私人空白底材及框線樣圖；文字、肖像、游標、點擊區與輸入保持獨立。

`[both] L0`：DATA1 輸入雜湊沿用 §6.37。兩版各 16 張 FBR 肖像框片、32 張 SIDE 拼件；位址空間為 DATA1.GRP 檔案位移。正式解碼器逐一核對 96 個記錄的原始雜湊、尺寸與偏移，入口及收據為 `workplace/hd-panels-v32-source.go`、`workplace/hd-panels-v32-source.json`，Go 1.24.13。原版幾何沿用 [005 主畫面與戰場](005-main-screen.md)及 [014 主畫面疊層](014-art-main-overlays.md)；不新增原版 oracle 聲明。

- 主畫面資料面板為 (408,36) 224×256、指令面板為 (408,292) 224×80。SIDE 角片 16×16、邊片 8×8，依正式 `assets.MainPanels`／`Image.DrawPanel` 拼接。底材必須避開實際框線，不能先拼框再用底色覆蓋。
- 戰場兩種版面共用 176×96 內部，凹框向四側各延伸 2 px，完整外框為 180×100。軍力／查看為藍底，指令為青底；軍力、查看肖像與四片 FBR 最後疊畫。不能把 176×96 內部誤當完整凹框尺寸。
- 樣圖只生成空白底材與框線，禁止烘焙文字、原版字模、人物或按鈕。保留原有色系及方框識別；內部紋理採低對比，先用實際三語文字核對可讀性。候選、原稿、完整提示詞及收據只留 `workplace/`。
- 正式接入前須補齊各類底材與框件的來源鍵、重用／縮放邊界、缺圖回退、覆蓋順序及快取契約，再審查為 READY。驗收包括兩版正常主畫面、人物卡、選單、對話與寬窄戰場，三語、原貌／4×／回切、肖像／游標完整覆蓋及原點擊區保持。樣圖不進正式路徑，不以它宣稱面板 HD 已完成。

主畫面上面板與戰場指令面板兩款 B 空白皮膚已生成並查看。完整原稿與請求為 `workplace/hd-b-panel-main-v1.png`、`workplace/hd-b-panel-battle-command-v1.png` 及 `workplace/hd-panels-v32-{main,battle}-request.json`，採內建 imagegen 的 style-transfer。私人入口 `workplace/hd-panels-v32-preview.sh`／`hd-panels-v32-preview_test.go` 使用正式 renderer overlay，兩版三語十二組完整 2560×1632 畫布通過。原貌 canvas 保持，面板外原生像素與控制組相同；覆蓋的文字像素完整留在底材上方。控制與候選共二十四張 PNG、兩張準備圖及收據在 `workplace/hd-panels-v32-preview.json`。中文、英文與日文完整樣圖已查看。這是 Codex 試作審查，沒有正常玩家、使用者逐張簽核或正式面板完成聲明。
