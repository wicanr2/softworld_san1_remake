# 021：B 寫實手繪 HD 素材與渲染

狀態：`READY`，授權視窗選項列、首批四張素材及 §6.7–6.10 已審查的肖像批次；其餘美術依 #108、#109 分批驗收。

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
