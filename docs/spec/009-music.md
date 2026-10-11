# 009：配樂

狀態：`CONFORMED`

範圍：原版的 AdLib／OPL2 配樂——資料在哪、怎麼解、remake 怎麼合成。
PC 喇叭的音效與語音是另一條路，在 `docs/spec/008`。

證據等級：`L0`（`DATA1` 的位元組、`AA.EXE` 的曲名字串）＋
`L1`（`internal/music` 的解碼與合成、`cmd/san1music` 實跑）。
版本：`[both]`——`DATA1.GRP` 兩版**逐位元組相同**（`CLAUDE.md` §2.1）。

格式與量測過程在 `docs/formats/06`；這一份只寫 remake 要守的契約。

> 這一份是**追認**：`internal/music` 先寫完才補規格。追認的規格只描述
> 已經實作而且驗過的行為，**不新增任何還沒驗的斷言**——沒驗的那幾項
> 列在 §5。

## 1. 資料（`L0`）

現行時序與片頭接線以 §7 的勘誤為準。先前 120 tick 的推論已撤回。

`DATA1` 裡的四個項目，本身又是一組 `.IDX`／`.GRP`（**巢狀容器**，
版面與外層相同）：

| 項目 | 內容 |
|---|---|
| `MUS.IDX`／`MUS.GRP` | 五首曲子 ＋ 五份音色庫 |
| `MUSV.IDX`／`MUSV.GRP` | 一首長曲 ＋ 一份音色庫 |

**項目成對**：偶數項是曲子，奇數項是那首用的音色庫。五首的順序與
開機主選單第五項「音樂欣賞」列的曲名一一對上（思古／小徑／風雲／
戰鼓／末路，`AA.EXE` `0x47bb9`）。

## 2. 曲子的版面（`L0`）

前 `0x46` 個位元組是表頭，其後是事件流：

| 位移 | 型別 | 內容 |
|---|---|---|
| `0x00` | uint16 | 版本，全部是 1 |
| `0x24` | uint8 | 每拍刻度數，本作為 240 |
| `0x25` | uint8 | 每小節拍數，本作為 4 |
| `0x26` | uint32 | 全曲長度（tick）|
| `0x2A` | uint32 | 事件流的位元組數 |
| `0x2E` | uint32 | 事件數 |
| `0x3A` | uint8 | 1 ＝ 節奏模式 |
| `0x3C` | uint16 | 速度（BPM），小端 |

版本須為 1.0，刻度數、每小節拍數及 BPM 不得為零。事件數與停止刻度
須同時對上表頭；兩個相鄰欄位不能合稱 magic。勘誤證據見 §7。

`0x2A` 對每一首都精準等於「項目長度 − `0x46`」——三個獨立的數字同時
成立，版面因此是確定的。

## 3. 事件流（`L0`）

形狀是標準 MIDI（含 running status），四處不同寫在 `docs/formats/06` §3。
聲部怎麼對到 OPL2 的九個（或節奏模式的六個旋律 ＋ 五件打擊）在 §4，
音色庫的版面在 §5。

## 4. remake 這一側

### R1 解碼（`internal/music`）

`ParseAll` 讀巢狀容器，成對取出曲子與音色庫。版本、時序、事件數或
停止刻度不符即回錯誤。

### R2 邊播邊合成（`internal/music.Stream`）

一首曲子整份算完要好幾秒 CPU，開遊戲不能停在那裡等。`Stream` 是
`io.Reader`：音訊裝置要多少取樣就合多少，播完自動從頭開始。

合成走自己的 OPL2：事件照 §3 送進晶片，音色參數照版面填進暫存器，
內部固定 49,716 Hz（OPL2 的原生速率），再線性內插到輸出取樣率。

### R3 「音樂狀態」是靜音不是停止（**remake 差異**）

關掉音樂時**曲子照走，只是不出聲**，
回來的時候接得上原本的位置而不是從頭開始。

正式 `jukebox` 用 Ebiten 播放器的 `SetVolume(0/1)` 切換，讓已緩衝的樣本也受音量控制；來源照常合成。`Stream.SetSilent` 保留作來源層單測，不用它控制正式播放器。

原版怎麼做的沒量過。這樣選是因為「從頭開始」在來回切換時會把同一段
前奏聽很多次。**記在這裡，不在程式碼裡默默改。**

### R4 依表頭換算刻度

每秒刻度為 `BPM × ticksPerBeat ÷ 60`。本作表頭為每拍 240 tick，
速度事件沿 AdLib MUS 契約計算；MIDI 匯出保留最後一個音符到全曲停止的延遲。
先前固定 120 的推論已撤回，見 §7 與 `CONTEXT.md` R91。

## 5. 還沒驗的

| 項 | 現況 |
|---|---|
| 滑音的算法 | 倒推的幅度七組量測符合六組，差在整數運算（`docs/formats/06` §9）|
| 戰略與戰役中的原版換曲時機 | 尚未完整觀測 |

## 6. 驗收

| 項 | 怎麼驗 |
|---|---|
| 解碼與版面 | `internal/music` 單測（`music_test.go`／`render_test.go`）|
| 串流與靜音 | `stream_test.go` |
| 離線匯出 | `tools/assets.sh` 轉出 OGG；`cmd/san1music` 列曲目 |
| 正式遊戲播放 | `cmd/san1` 的 `jukebox`；[錄音驗證入口](../../tools/verify-music.sh) 與 §6.1 收據 |

### 6.1 正式音訊串流契約

狀態：`CONFORMED`；範圍：remake 播放器接線，`[both]`；2026-10-02 驗證。

`Stream` 是持續合成、無盡循環的 `io.Reader`，不公開 `io.Seeker`。
鎖版 Ebiten 2.9.9 的 `audio/player.go:newTimeStream` 會向可定位來源
呼叫 `Seek(0, io.SeekCurrent)` 查詢位置；來源若只支援回到開頭，
播放器初始化便會失敗。原問題與訂正證據集中於 [`CONTEXT.md`](../../CONTEXT.md) R87。

驗收入口為 [`tools/verify-music.sh`](../../tools/verify-music.sh)。
重跑前須備好兩版 `org_game/`、既有建置與錄音映像，以及本機
`workplace/audio/`、`workplace/gocache/`、`workplace/gomodcache/`。
Go 模組快取須已含鎖版依賴；腳本採無網路容器，缺少前置環境會直接報錯。
素材根目錄可用 `SAN1_ORIG` 指定，建置／錄音映像可用
`SAN1_GO_IMAGE`／`SAN1_AUDIO_IMAGE` 指定。

它在容器內啟動正式 Ebiten 視窗，透過 XTEST 按鍵走片頭、主選單、
逐首音樂欣賞、單人新局及音樂狀態切換；PulseAudio 的 monitor 錄製
正式播放器輸出。每個按鍵按住 80 ms；新局主命令需 `9`、Enter，
再選 `3`。靜音提示後已離開數字輸入，恢復操作為 `9`、`3`。

有聲段需均方根音量（RMS）> −60 dBFS、峰值 > −45 dBFS；
靜音段的 16 位元樣本峰值 ≤ 2。錄音長度須在目標秒數 ±1 秒內。
這份收據只證明 Linux 正式播放器的音訊輸出；人耳聽辨、Windows／macOS
原生音訊與原版逐波形一致均未驗。原版素材、截圖及錄音只留 `workplace/audio/`。

| 正常操作 | RMS dBFS | 峰值 dBFS | 結果 |
|---|---:|---:|---|
| 原版主選單預設曲 | −22.51 | −7.51 | 通過 |
| 音樂欣賞：思古 | −21.65 | −8.64 | 通過 |
| 音樂欣賞：小徑 | −23.30 | −9.86 | 通過 |
| 音樂欣賞：風雲 | −18.92 | −8.13 | 通過 |
| 音樂欣賞：戰鼓 | −18.13 | −3.94 | 通過 |
| 音樂欣賞：末路 | −21.98 | −8.16 | 通過 |
| 原版正常單人新局 | −22.42 | −8.89 | 通過 |
| 其他 → 音樂狀態關閉 | 全零 | 全零 | 通過 |
| 其他 → 音樂狀態恢復 | −22.64 | −7.86 | 通過 |
| 加強版正常單人新局 | −22.01 | −7.88 | 通過 |

收據為 `workplace/audio/music-check-receipt.json`，包含 10/10 結果、
每段長度與 SHA-256、輸入雜湊、工具版本、按鍵序列、基底提交及修改檔雜湊。
最新兩版視窗及靜音／恢復截圖已目視核對。重跑會覆寫同名的本機驗證產物。

| 驗證輸入／環境 | 定位 |
|---|---|
| 基底提交 | `df773e08c77cbf0c97ded48489320bf9924ddd69`，加視窗選項列、完整畫布及播放器靜音修正 |
| `internal/music/stream.go` SHA-256 | `195c4203ecf36566aec462f6b8ad357e4abf674f10faf4971f06d7ae0096010c` |
| `cmd/san1/music.go` SHA-256 | `99d1b5d55d41d3465686ee3fdaa7fe12b4443fa085f3d9ad4d043fb002d5cb32` |
| 實際執行檔 SHA-256 | `a258481d7ac9d41c89171cc3dc4a1a566f742964d339bd74662a369d77bea614` |
| 兩版 `DATA1.GRP` SHA-256 | `958f44fe45e38624401af55f033ffb037bd3211a037eadbce90f827637d977a5` |
| 建置映像 | `rich2-go-ebiten:latest`，Go 1.24.13；映像 ID `sha256:fd2bdff81a22f5c5b7c49bb0983e2e3e4e9e82df5c789831c525e1af4e22ec8a` |
| 錄音映像 | `eob-audio-capture:20260922-r2`；映像 ID `sha256:90933bf64c453a75d307d0b1c2b591bb67aa1eca57b82e779859164b92f9cde6` |
| 音訊與擷取工具 | Ebiten 2.9.9、PulseAudio 16.1、FFmpeg 5.1.9；48000 Hz、雙聲道、PCM 16-bit |
| 最新收據 SHA-256 | `2efe2457cb196e32a917af926a5519238aaacacc85e06fdb75d62b79b8fe3ae3` |

主選單狀態依正式渲染的固定上圖辨識。現行 `app.Layout` 保留完整 640×408，
選項列預設隱藏，返回主選單的原操作改用 Shift＋Esc；腳本按目前幾何及輸入契約重跑。
先前 Seek 修正的收據與 640×400 擷取條件保留於 Git 歷史及 `WORKLOG.md`，不套到現行視窗。

高清模式另以 `bash tools/verify-music.sh --hd` 驗證，錄音保存在 `workplace/audio/hd-v4/`，不覆寫上述原貌收據。兩版高清新局、五曲、靜音及恢復共 10/10 通過；環境、雜湊與範圍見 [021 §6.13](021-hd-art.md#613-高清模式配樂驗證)。

## 7. 曲檔時序與片頭曲勘誤（2026-10-11）

狀態：`CONFORMED`，限曲檔時序、選曲接線及 Linux 正式播放器抽樣。
版本：`[both]`。本節取代 R4、§5 的刻度數與片頭曲未知項。

- `L0`：兩版 DATA1.GRP 的 SHA-256 均為
  `958f44fe45e38624401af55f033ffb037bd3211a037eadbce90f827637d977a5`。
  六首曲子的 `0x24` 是每拍刻度數 240，`0x25` 是每小節拍數 4；兩個位元組
  不能合稱 magic。`0x3C` 是小端 uint16 BPM。原來固定 120 使配樂慢一倍。
- 格式契約採成熟播放器 [AdPlug mus.cpp](https://github.com/adplug/adplug/blob/master/src/mus.cpp)
  的表頭、GetTicks、SetTempo 與 tempo multiplier 定義。這是平台格式契約，
  不再深入遊戲 PIT／ISR。`0xF8` 在延遲欄累加 240 tick；不當作 MIDI 時脈忽略。
  `F0 7F 00 integer fraction F7` 改變速度，倍率是 integer＋fraction/128。
  本作六首均只有開頭倍率 1。長曲六個 overflow 必須保留其延遲。
- 每首時長、串流及 MIDI 匯出使用該曲的刻度數與速度事件。拒收零刻度與零 BPM。
  事件數及總 tick 同時對帳，不能只以事件數正確證明時間軸正確。
- 片頭從 `MUSV.IDX`／`MUSV.GRP` 讀獨立長曲；五首欣賞曲仍來自 `MUS`，
  曲目數維持五首。原版讀取證據入口為
  [兩版開機觀測](../../internal/parity/music_boot_oracle_test.go)。
- remake 商標與製作名單保持無配樂，海景第一拍開始長曲；片頭結束或跳過後
  從頭開始 `MUS` 第 0 首。直接進遊戲仍播第 0 首。缺長曲時保持無聲，
  進主選單仍能播五首；不把標題曲冒充片頭曲。
- 片頭動畫時長沿用 005 的既有外殼契約，不新增硬體逐週期對拍。
  存檔與規則無變更。驗收包含六首原始時間軸、實際兩版開機及主選單錄音。

驗證入口為 `internal/music/timing_test.go`、`internal/parity/music_boot_oracle_test.go`
及 `tools/verify-mouse-audio-inner.py`。原版開機觀測收據為
`workplace/music-boot-20261011.log`，兩版皆讀取獨立 MUSV 項目。
正式視窗收據為 `workplace/mouse-audio-final-20261011/receipt.json`；兩版片頭及
標題共四段約四秒錄音非靜音，完整 PCM 均匹配對應曲子的串流參考。
音訊格式為 48000 Hz、雙聲道、16-bit PCM，使用 PulseAudio 16.1、FFmpeg 5.1.9。
錄音與參考使用同一合成器，證明正式播放的選曲及時間軸接線；格式時序另由
原始表頭、AdPlug 契約與六首時間軸測試驗證，不稱為原版逐波形對拍。
人耳與 Windows／macOS 原生音訊尚未驗；既有發行包未覆寫。
