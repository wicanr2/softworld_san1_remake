# 字型

**這些不是原版的字模。** 原版的中文字模在 `DATA1`／`DATA5` 容器裡
（`ZHONG.PAT`／`ZHONG.COD`，見 `docs/formats/01`），那是原版素材，
不散布、也不內嵌進 remake（`CLAUDE.md` §3.3）。

這裡放的是自由授權的點陣字型與向量字型，由 remake 自己提供字模。

| 檔 | 來源 | 授權 | 用途 |
|---|---|---|---|
| `unifont.hex.gz` | GNU Unifont 16×16 點陣 | GPL v2 ＋ 字型例外，見 `LICENSE-unifont.txt` | 繁中、日文漢字（預設）|
| `ascii6x10.hex.gz` | X11 misc-fixed 6×10 | 公有領域，見 `LICENSE-x11-misc-fixed.txt` | 原尺寸放不下的地方的小字級 |
| `kai.hex.gz` | 王漢宗顏楷體繁（`wt064.ttf`）點陣化 | **GPL v2**（沒有字型例外），見 `LICENSE-wangfonts.txt` | 主選單「3 使用楷書字」|
| `li.hex.gz` | 王漢宗中隸書繁（`wt021.ttf`）點陣化 | **GPL v2**（沒有字型例外），見 `LICENSE-wangfonts.txt` | 主選單「4 使用隸書字」|
| `kai.ttf` | 王漢宗顏楷體繁（`wt064.ttf`）原向量檔 | GPL v2 或更新版，沒有字型例外 | HD 楷書 |
| `li.ttf` | 王漢宗中隸書繁（`wt021.ttf`）原向量檔 | GPL v2 或更新版，沒有字型例外 | HD 隸書 |

## 可切換的兩套（Issue #71）

原版主選單的第三、四項換的是 `ZHONG.PAT`／`YING.PAT`（`0x11bb4`／`0x11bc2`
→ `0x33d8:0x15a`）。remake 不內嵌原版字模，改成兩套自由授權字型：

- 原來源為王漢宗字型包的 `wt064.ttf` 顏楷與 `wt021.ttf` 中隸書，
  repo 已分別保存為本目錄的 `kai.ttf`、`li.ttf`。用 `tools/mkhexfont` 重生 16×16：

  ```sh
  tools/go.sh run ./tools/mkhexfont -ttf fonts/kai.ttf -out fonts/kai.hex.gz
  tools/go.sh run ./tools/mkhexfont -ttf fonts/li.ttf -out fonts/li.hex.gz
  ```

  `-show 三國` 只把幾個字印成 ASCII 圖，用來看字級與基線擺對了沒有。
- **碼位與字寬照 `unifont.hex.gz`**：少一個碼位在畫面上是空白，
  而空白看起來像排版問題；字寬變了整個版面跟著跑
  （`TestSwitchableFontsCoverTheSameRunes`）。TTF 裡沒有的碼位沿用 unifont
  的字模（兩套各有約 11.2 萬個碼位是這樣來的，Big5 範圍的 1.3 萬個才是新畫的）。
- **點陣化要超取樣**：楷書與隸書是毛筆字，一筆在 16 像素高的框裡只有一個
  像素寬。直接用 16 點渲染，反鋸齒會把同一筆切成一段一段——看起來像雜訊
  不像字。`mkhexfont` 先用 3 倍字級畫再按覆蓋率（0.35）降取樣。
- **這兩套是 GPL v2，而且沒有字型例外**。程式只在執行時讀檔，不內嵌
  （`cmd/san1` 的 `-font` 指目錄），發行包把 `LICENSE-wangfonts.txt`
  一起帶著。remake 自己的程式碼仍是 RRSAL-1.0。
- 原貌使用既有 16×16 點陣；HD 在相同字格內重新繪製向量筆畫。
  一般全形字為 64×64、半形為 32×64，基線為 52。字距及面板配置保持。
  `internal/font/vector.go` 使用無 hinting 的 OpenType 繪製，字模快取上限 16 MiB。
- 兩份 TTF 與 HEX 放在同一字型目錄，執行時讀取。缺檔、解析失敗或缺字時
  沿用點陣；專用 6×10 ASCII 小字也維持點陣。
  向量來源與選定樣張的 SHA-256 見 [HD 向量文字規格](../docs/spec/021-hd-art.md#665-hd-楷書與隸書的向量文字)。

## 格式

GNU Unifont 的 HEX 格式：一行一個字，`<碼位十六進位>:<點陣十六進位>`。
每列一個 byte（8×16）或兩個 byte（16×16），由上而下。
解析器在 `internal/font`。

## 小字級的用法

`ascii6x10` **不是拿來擠版面的通用手段**。判準是「量得出放不下」，
不是「是不是英文」——語系不是原因，寬度才是。要用先問「這個框是不是原版的」。

目前用在（`docs/spec/014` §3.2、§7）：下面板的子選單、文字版指令欄的
名字、戰場的指令面板與軍力面板——都是英文在原版的格子裡原尺寸放不下的
地方。只有 ASCII，中日文放不下時各處另有退路。發行包連同授權檔一起帶。

## 為什麼是 unifont 而不是烘過的子集

一開始用的是隔壁專案（sangokushi）烘好的 16×16 子集（48 KB）。
`internal/font` 的涵蓋率測試當場發現它**缺 21 個郡名用字**——
那份子集是為那款遊戲的日文文本烘的，涵蓋不了這款的繁中用字。

缺字在畫面上是**空白**，而空白看起來像排版問題不像缺字。
所以涵蓋率要用測試釘住（`TestRealFont`），不能靠眼睛看。

unifont 大（1.6 MB）但全涵蓋。要縮的話得**用這款自己的文字**烘子集，
而且烘完要讓涵蓋率測試繼續綠——不是換一份別人的子集。
