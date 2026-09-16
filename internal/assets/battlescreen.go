package assets

import "fmt"

// 主戰場整張畫面的版面。
//
// 座標讀自 `0x224c6`–`0x226ff`（寬版面那一條路，州郡 offset 34 ＝ 9），
// 每一項都拿紮完寨的基準畫面驗過。
//
// 原版先把整個畫面鋪滿 8×8 的底紋（`0x223e4` 的雙重迴圈，一次 8 像素，
// 鋪到 640×408），再蓋上方花邊、場地、面板。底紋是 `8x8PAT0.IMG`
// ——黃／青的 2×2 棋盤，畫面左緣 x 0–5 那一條逐像素對得上。
//
// 三個面板的外框由 `0x22200` 畫：黑線在上與左、白線在下與右，
// 也就是**下凹**的立體邊。面板本身沒有底色，內容自己填：
// 兩個軍力面板是藍底（1）配淺紅字（12），指令面板是青底（3）配黃字（14）。
//
// 面板在 y ＝ 268 高 96，下緣 364；`MAINMAP8` 畫在 y ＝ 372 高 36，
// 下緣正好 408。**兩個都完整落在畫面裡**——上面那句「鋪到 640×408」
// 就是原版自己的底紋迴圈給的高度（`docs/spec/006`）。
const (
	// BattleBGTile 是鋪滿畫面的底紋。
	BattleBGTile = "8x8PAT0.IMG"

	// BattleFieldLeft／Right／Top 是場地區的邊（`0x22527`、`0x2250c`）。
	BattleFieldLeft  = 54
	BattleFieldRight = 632
	BattleFieldTop   = 34

	// BattleWeatherX／Y 是天氣圖示的位置（`0x21969`，`WEATHER%d.IMG`）。
	BattleWeatherX = 8
	BattleWeatherY = 155

	// BattleNameX／Y／Step 是左欄郡名的位置（`0x2181d`–`0x218a4`）：
	// 兩個字各 32×32（`0x33d8:0x16b4` 的 `sx=2, sy=2`），黃字青底。
	// remake 用自己的字庫放大成同一個字級。
	BattleNameX     = 8
	BattleNameY     = 52
	BattleNameStep  = 32
	BattleNameScale = 2

	// BattleProvinceY 是州名（`0x218d7`，白色）、BattleNumberY 是郡編號
	// （`0x21921`，`%2d`、洋紅）。兩個都在左欄第一個框裡。
	BattleProvinceY = 116
	BattleNumberY   = 132

	// 三個面板：兩個軍力 ＋ 一個指令列（`0x2259e`、`0x226cb`）。
	BattlePanelY = 268
	BattlePanelW = 176
	BattlePanelH = 96

	// 肖像框 80×80，框內的肖像 64×80（`FBRC0.IMG` 在 (64,268) 與
	// (352,268) 各 100% 相符）。
	BattleFrameW = 80
	BattleFaceW  = 64
	BattleFaceH  = 80

	// 面板的底色與字色，量自基準畫面；四個軍團各一色（`DS:0x796a`：
	// 主守 10、助守 11、主攻 12、助攻 13）。
	BattlePanelInk   = 12 // 淺紅（主攻）
	BattlePanelPaper = 1  // 藍
	BattleOrderInk   = 14 // 黃
	BattleOrderPaper = 3  // 青
)

// 軍力面板裡的東西各在哪（`0x22c94`，`L0`）：每一側從 `DS:0x7b8c` 那張
// 表拿三個位移——肖像、統帥名、五行資料——加在面板左緣上。順序是
// 攻方、守方，與 `BattleFrameX` 相同。
//
//	攻方（主攻軍，側 2）：肖像 +8、統帥名 +80、資料 +112；字色 12
//	守方（主守軍，側 0）：肖像 +104、統帥名 +64、資料 +0；字色 10
//
// 統帥名是 32×32 直排（`0x22fd4`）：兩字名在 y+16／y+48，三字名在
// y+0／+32／+64。五行資料 16×16，在 y+0、+16、+32、+48、+64
// （`%s軍`、` %s `、`%s軍%2d將`、`兵%4d00`、`金%6d`）。
var (
	BattlePanelNameX = [2]int{144, 320}
	BattlePanelTextX = [2]int{176, 256}
	BattlePanelInks  = [2]int{12, 10}
)

// BattlePanelLineY 是軍力面板第 k 行資料的上緣。
func BattlePanelLineY(k int) int { return BattlePanelY + k*16 }

// BattlePanelX 是三個面板的左緣：攻方、守方、指令列。
var BattlePanelX = [3]int{64, 256, 448}

// 主戰場下方花邊上那一行年月（`docs/spec/011`）。
//
// 量在原版基準畫面上（`workplace/shots/bf/orig-battle.png`，
// 「建安二年九月秋」）：**十個格子**，每格 24 寬、左緣
// `BattleDateX + i*BattleDateStep`，字的上緣 `BattleDateY`、高
// `BattleDateH`。
//
// 十格裡只有七格有字——空的那三格（2、4、6 之中）是版面的一部分，
// 不是漏畫：年數與月份各佔兩格且**靠右**，個位數就只用右邊那一格。
const (
	BattleDateX     = 128
	BattleDateStep  = 40
	BattleDateW     = 24
	BattleDateY     = 378
	BattleDateH     = 23
	BattleDateCells = 10
)

// 那一行字的兩個色號。
//
// **不是「主色 ＋ 陰影」**：量到灰（7）538 點與綠（2）502 點**逐像素
// 交錯**，(+1,0) 有 388、(0,+1) 有 329，而右下 (+1,+1) 只有 4 點——
// 那是 `(x+y)` 的棋盤網點，不是位移的影子。
const (
	BattleDateInkOdd  = 7 // (x+y) 奇數
	BattleDateInkEven = 2 // (x+y) 偶數
)

// BattleFrameX 是兩個肖像框的左緣。**攻方的框在左、守方的在右**，
// 兩人因此面對面。
var BattleFrameX = [2]int{64, 352}

// BattleFaceX 是兩張肖像的左緣（框內縮 8）。
var BattleFaceX = [2]int{72, 360}

// BattleFaceY 是兩張肖像的上緣。
const BattleFaceY = 276

// BattleFaceMirror 說某一側的肖像要不要左右翻。
//
// **攻方的肖像是鏡像**：基準畫面上守方的 `F014.FAC` 直接對得上，
// 攻方的 `F228.FAC` 要左右翻過來才 100%（各 4736 格，下緣被畫面切掉）。
var BattleFaceMirror = [2]bool{true, false}

// BattleBackground 鋪出整張底紋。
func BattleBackground(data1 *Container) (*Image, error) {
	i, ok := data1.ByName(BattleBGTile)
	if !ok {
		return nil, fmt.Errorf("assets: DATA1 裡沒有 %s", BattleBGTile)
	}
	tile, err := DecodeImage(data1.Data(i))
	if err != nil {
		return nil, err
	}
	im := &Image{W: ScreenW, H: ScreenH, Pix: make([]byte, ScreenW*ScreenH)}
	for y := 0; y < ScreenH; y++ {
		for x := 0; x < ScreenW; x++ {
			im.Pix[y*ScreenW+x] = tile.Pix[(y%tile.H)*tile.W+x%tile.W]
		}
	}
	return im, nil
}

// PortraitFrame 取一組肖像框：上、下、左、右四片（`FBR%c0`–`FBR%c3`）。
//
// 主戰場用的是 `C` 那一組。上片在框的左上角 100% 相符；其餘三片原版
// 畫在哪還沒定，所以呼叫端目前只用得到上片。
func PortraitFrame(data1 *Container, style byte) ([4]*Image, error) {
	var out [4]*Image
	for i := range out {
		name := fmt.Sprintf("FBR%c%d.IMG", style, i)
		j, ok := data1.ByName(name)
		if !ok {
			return out, fmt.Errorf("assets: DATA1 裡沒有 %s", name)
		}
		im, err := DecodeImage(data1.Data(j))
		if err != nil {
			return out, err
		}
		out[i] = im
	}
	return out, nil
}

// Mirror 左右翻一張圖。
func (im *Image) Mirror() *Image {
	out := &Image{W: im.W, H: im.H, Pix: make([]byte, len(im.Pix))}
	for y := 0; y < im.H; y++ {
		for x := 0; x < im.W; x++ {
			out.Pix[y*im.W+x] = im.Pix[y*im.W+im.W-1-x]
		}
	}
	return out
}

// BevelBox 畫一個下凹的方框：黑線在上與左、白線在下與右。
//
// 八條線的端點照 `0x22200` 逐條抄：黑線在 `x0−2`／`x0−1`／`y0−2`／`y0−1`，
// 白線在 `x1+1`／`x1+2`／`y1+1`／`y1+2`，而且**上下兩條的端點各差一格**
// ——那個一格的參差就是立體感的來源，抄整齊了反而不對。
// x1／y1 是含在框內的最後一格。
func (im *Image) BevelBox(x0, y0, x1, y1 int) {
	const black, white = 0, 15
	im.hline(x0-2, x1+2, y0-2, black)
	im.hline(x0-2, x1+1, y0-1, black)
	im.vline(x0-2, y0-2, y1+2, black)
	im.vline(x0-1, y0-1, y1+1, black)
	im.hline(x0, x1+2, y1+1, white)
	im.hline(x0-1, x1+2, y1+2, white)
	im.vline(x1+1, y0, y1+2, white)
	im.vline(x1+2, y0-1, y1+2, white)
}

// 左欄是**四個**下凹的小框，內部一律 32 像素寬（x 8–39），
// 上下緣量自基準畫面：
//
//	郡名／州名／郡編號  y 52–147   （32＋32＋16＋16 剛好填滿）
//	天氣圖示            y 155–187  （圖示 32×32 畫在 (8,155)）
//	天氣名              y 196–211
//	日數                y 228 起，與面板一樣被畫面下緣切掉
//
// 框與框之間露出底紋。右邊 x 54–55 那兩條黑線是場地的左緣，不屬於框
// （`0x22527`、`0x22542`）。
var battleLeftBoxes = [4][2]int{{52, 147}, {155, 187}, {196, 211},
	{228, BattlePanelY + BattlePanelH - 1}}

// BattleLeftBox 回傳左欄第 i 個框的上下緣。
func BattleLeftBox(i int) (y0, y1 int) { return battleLeftBoxes[i][0], battleLeftBoxes[i][1] }

// BattleLeftBoxX0／X1 是左欄三個框共用的左右緣。
const (
	BattleLeftBoxX0 = 8
	BattleLeftBoxX1 = 39
)

// LeftColumn 畫左欄的三個框與場地的左緣。
func (im *Image) LeftColumn() {
	for _, b := range battleLeftBoxes {
		im.FillRect(BattleLeftBoxX0, b[0],
			BattleLeftBoxX1-BattleLeftBoxX0+1, b[1]-b[0]+1, BattleOrderPaper)
		im.BevelBox(BattleLeftBoxX0, b[0], BattleLeftBoxX1, b[1])
	}
	im.vline(BattleFieldLeft, BattleFieldTop, 260, 0)
	im.vline(BattleFieldLeft+1, BattleFieldTop, 259, 0)
}

func (im *Image) hline(x0, x1, y int, v byte) {
	for x := x0; x <= x1; x++ {
		im.Set(x, y, v)
	}
}

func (im *Image) vline(x, y0, y1 int, v byte) {
	for y := y0; y <= y1; y++ {
		im.Set(x, y, v)
	}
}

// 開場詩那一張的底圖是 `DATA1` 的兩半，各 320×295。
//
// 位置量自原版開場的第一格（詩還沒寫上去的那一張）：`SANTL` 在 (0,49)、
// `SANTR` 在 (320,49)，兩張各 94400 格**逐格相同**。
const (
	PoemPieceW = 320
	PoemPieceH = 295
	PoemY      = 49
)

// PoemScreen 拼出開場詩的底圖（沒有字）。
func PoemScreen(data1 *Container) (*Image, error) {
	im := &Image{W: ScreenW, H: ScreenH, Pix: make([]byte, ScreenW*ScreenH)}
	for i, name := range []string{"SANTL.IMG", "SANTR.IMG"} {
		j, ok := data1.ByName(name)
		if !ok {
			return nil, fmt.Errorf("assets: DATA1 裡沒有 %s", name)
		}
		p, err := DecodeImage(data1.Data(j))
		if err != nil {
			return nil, err
		}
		if p.W != PoemPieceW || p.H != PoemPieceH {
			return nil, fmt.Errorf("assets: %s 是 %d×%d，開場詩的底圖應該是 %d×%d",
				name, p.W, p.H, PoemPieceW, PoemPieceH)
		}
		im.Blit(p, i*PoemPieceW, PoemY)
	}
	return im, nil
}

// 開場的三英圖是 `DATA1` 的 `TITL0`–`TITL3` 四塊，各 160×400。
//
// 四塊**並排**：`TITL<i>` 放在 x ＝ 160i，縱向從第 0 列取到第 349 列，
// 資料裡多出來的 50 列畫面上看不到。原版跑到那一格倒出來的畫面
// 224,000 格**逐格相同**（`internal/parity` 的
// `TestTitleArtLayoutMatchesTheOriginal`）。
const (
	TitleArtPieceW = 160
	TitleArtPieceH = 400
	TitleArtPieces = 4
)

// TitleArt 拼出開場的三英圖。
func TitleArt(data1 *Container) (*Image, error) {
	im := &Image{W: ScreenW, H: ScreenH, Pix: make([]byte, ScreenW*ScreenH)}
	for i := 0; i < TitleArtPieces; i++ {
		name := fmt.Sprintf("TITL%d.IMG", i)
		j, ok := data1.ByName(name)
		if !ok {
			return nil, fmt.Errorf("assets: DATA1 裡沒有 %s", name)
		}
		p, err := DecodeImage(data1.Data(j))
		if err != nil {
			return nil, err
		}
		if p.W != TitleArtPieceW || p.H != TitleArtPieceH {
			return nil, fmt.Errorf("assets: %s 是 %d×%d，三英圖的一塊應該是 %d×%d",
				name, p.W, p.H, TitleArtPieceW, TitleArtPieceH)
		}
		// 四塊各 400 列，畫面 408 列——**底下 8 列不是素材畫的**，
		// 原版在貼圖之前把畫面清成黑，那 8 列就一直是黑的
		//（`TestTitleArtMatchesTheOriginal` 釘著）。
		im.Blit(p, i*TitleArtPieceW, 0)
	}
	return im, nil
}

// 場地四周的階梯狀邊框。
//
// 原版每 96 像素（兩欄）畫一組：上緣 `0x21ee2`、下緣 `0x220f0`，
// 寬版面各叫六次，x ＝ 96i + 56，上緣的 y ＝ 36、下緣的 y ＝ 228。
// 兩支都是把黑線畫在上與左、白線畫在下與右——**奇數欄低 16 像素**，
// 所以每一組都是一個階梯。
//
// 端點逐條抄自那兩支常式；差一格的參差就是立體感的來源。
func (im *Image) fieldEdgeTop(x, y int) {
	im.vline(x-2, y-2, y+15, 0)
	im.vline(x-1, y-2, y+15, 0)
	im.hline(x-2, x+49, y-2, 0)
	im.hline(x-2, x+48, y-1, 0)
	im.vline(x+48, y, y+15, 15)
	im.vline(x+49, y-1, y+14, 15)
	im.hline(x+50, x+95, y+14, 0)
	im.hline(x+49, x+95, y+15, 0)
}

func (im *Image) fieldEdgeBottom(x, y int) {
	im.vline(x-1, y+17, y+32, 0)
	im.vline(x-2, y+16, y+31, 0)
	im.hline(x-1, x+49, y+32, 15)
	im.hline(x-2, x+49, y+33, 15)
	im.vline(x+48, y+16, y+33, 15)
	im.vline(x+49, y+16, y+33, 15)
	im.hline(x+48, x+95, y+16, 15)
	im.hline(x+48, x+94, y+17, 15)
}

// FieldEdges 畫整個場地的邊框。
func (im *Image) FieldEdges() {
	for i := 0; i < FieldCols/2; i++ {
		x := i*96 + FieldOriginX
		im.fieldEdgeTop(x, FieldOriginY)
		im.fieldEdgeBottom(x, FieldOriginY+6*TileH)
	}
}
