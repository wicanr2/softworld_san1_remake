package assets

import "fmt"

// 主戰場整張畫面的版面。
//
// 座標讀自 `0x224c6`–`0x226ff`：州郡 offset 34 ＝ 9 走**寬版面**（12 欄，
// 場地滿版，三塊面板排在下方），否則走**窄版面**（8 欄，三塊面板疊在
// 右邊 x 448–623）。兩條路各自的數字放在 `BattleWide`／`BattleNarrow`，
// 每一項都拿紮完寨的基準畫面驗過（`orig-battle.png`／`orig-battle-narrow.png`）。
//
// 原版先把整個畫面鋪滿 8×8 的底紋（`0x223e4` 的雙重迴圈，一次 8 像素，
// 鋪到 640×408），再蓋上方花邊、場地、面板。底紋是 `8x8PAT0.IMG`
// ——黃／青的 2×2 棋盤，畫面左緣 x 0–5 那一條逐像素對得上。
//
// 三個面板的外框由 `0x22200` 畫：黑線在上與左、白線在下與右，
// 也就是**下凹**的立體邊。面板本身沒有底色，內容自己填：
// 兩個軍力面板是藍底（1）配淺紅字（12），指令面板是青底（3）配黃字（14）。
//
// 寬版面的面板在 y ＝ 268 高 96，下緣 364；`MAINMAP8` 畫在 y ＝ 372 高 36，
// 下緣正好 408。**兩個都完整落在畫面裡**——上面那句「鋪到 640×408」
// 就是原版自己的底紋迴圈給的高度（`docs/spec/006`）。
const (
	// BattleBGTile 是鋪滿畫面的底紋。
	BattleBGTile = "8x8PAT0.IMG"

	// BattleFieldLeft／Top 是場地區的左緣與上緣（`0x22527`、`0x2250c`）；
	// 右緣隨版面走（`BattleLayout.RightX`）。
	BattleFieldLeft = 54
	BattleFieldTop  = 34

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

	// 三個面板的大小：兩個軍力 ＋ 一個指令列（`0x2259e`、`0x226cb`）；
	// 位置在 `BattleLayout.PanelX`／`PanelY`。
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

// BattleLayout 是主戰場的一種版面：場地邊框有幾組、三塊面板在哪。
//
// 兩種版面的差別全在 `0x224c6`–`0x226ff` 那條 if：寬版面（`BattleWide`）
// 六組階梯、面板排在場地下方；窄版面（`BattleNarrow`）四組階梯、
// 面板疊在場地右邊。左欄、天氣圖示、上下花邊、年月那一行兩種版面相同。
type BattleLayout struct {
	// Narrow 是窄版面（8 欄）。
	Narrow bool

	// Groups 是場地上緣與下緣的階梯邊框各幾組（一組 96 像素、兩欄；
	// `0x21ee2` 與 `0x220f0`／`0x21fe0` 各叫這麼多次）。
	Groups int

	// BottomY 是下緣那一排階梯的基準 y（`0x220f0`／`0x21fe0` 的第二個參數）。
	// 寬版面 228（第 6 列的上緣，那一列只有偶數欄），窄版面 324（第 9 列）。
	BottomY int

	// LeftY1 是場地左緣兩條黑線（x 54、55）的下端，第二條少一格。
	LeftY1 int

	// RightX／RightY1 是場地右緣兩條白線的 x 與下端：第一條在 RightX 從
	// y ＝ 51 起、第二條在 RightX+1 從 y ＝ 50 起，(RightX, 50) 是黑的。
	RightX, RightY1 int

	// PanelX／PanelY 是三塊面板的左上角：攻方、守方、指令列。
	PanelX, PanelY [3]int
}

// BattleWide 是 12 欄的寬版面（州郡 offset 34 ＝ 9）。
var BattleWide = BattleLayout{
	Groups: 6, BottomY: 228, LeftY1: 260, RightX: 632, RightY1: 245,
	PanelX: [3]int{64, 256, 448}, PanelY: [3]int{268, 268, 268},
}

// BattleNarrow 是 8 欄的窄版面：面板從上到下攻方、守方、指令列
// （`0x226a9`、`0x226c0`、`0x226d7`）。
var BattleNarrow = BattleLayout{
	Narrow: true, Groups: 4, BottomY: 324, LeftY1: 356, RightX: 440, RightY1: 373,
	PanelX: [3]int{448, 448, 448}, PanelY: [3]int{44, 156, 268},
}

// BattleLayoutFor 挑版面：窄圖的 (8,0) 那一格在圖外。
func BattleLayoutFor(narrow bool) BattleLayout {
	if narrow {
		return BattleNarrow
	}
	return BattleWide
}

// 軍力面板裡的東西各在哪（`0x22c94`，`L0`）：每一側從 `DS:0x7b8c` 那張
// 表拿三個位移——肖像、統帥名、五行資料——加在面板左緣上。順序是
// 攻方、守方，與 `PanelX` 的前兩項相同；**兩種版面用同一張表**，
// 只有面板的左上角不同。
//
//	攻方（主攻軍，側 2）：肖像 +8、統帥名 +80、資料 +112；字色 12
//	守方（主守軍，側 0）：肖像 +104、統帥名 +64、資料 +0；字色 10
//
// 肖像框 `FBRC0.IMG` 畫在肖像左上角往左上各 8（寬版面 (64,268) 與
// (352,268) 各 100% 相符）。統帥名是 32×32 直排（`0x22fd4`）：兩字名在
// y+16／y+48，三字名在 y+0／+32／+64。五行資料 16×16，在 y+0、+16、
// +32、+48、+64（`%s軍`、` %s `、`%s軍%2d將`、`兵%4d00`、`金%6d`）。
var (
	battleSideFaceX = [2]int{8, 104}
	battleSideNameX = [2]int{80, 64}
	battleSideTextX = [2]int{112, 0}

	// BattlePanelInks 是兩個軍力面板的字色：攻方、守方。
	BattlePanelInks = [2]int{12, 10}
)

// FieldY1 是場地最下面那一列白線的 y：下緣階梯的最後一條。
func (l BattleLayout) FieldY1() int {
	if l.Narrow {
		return l.BottomY + 49
	}
	return l.BottomY + 33
}

// Panel 回傳第 i 塊面板的四個角（含端點）：攻方、守方、指令列。
func (l BattleLayout) Panel(i int) (x0, y0, x1, y1 int) {
	return l.PanelX[i], l.PanelY[i], l.PanelX[i] + BattlePanelW - 1, l.PanelY[i] + BattlePanelH - 1
}

// Frame 回傳第 i 側肖像框的左上角。
func (l BattleLayout) Frame(i int) (x, y int) {
	return l.PanelX[i] + battleSideFaceX[i] - 8, l.PanelY[i]
}

// Face 回傳第 i 側肖像的左上角（框內縮 8）。
func (l BattleLayout) Face(i int) (x, y int) {
	return l.PanelX[i] + battleSideFaceX[i], l.PanelY[i] + 8
}

// NameX 回傳第 i 側統帥名那一欄的左緣。
func (l BattleLayout) NameX(i int) int { return l.PanelX[i] + battleSideNameX[i] }

// TextX 回傳第 i 側五行資料的左緣。
func (l BattleLayout) TextX(i int) int { return l.PanelX[i] + battleSideTextX[i] }

// LineY 回傳第 i 側面板第 k 行資料的上緣。
func (l BattleLayout) LineY(i, k int) int { return l.PanelY[i] + k*16 }

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

// BattleFaceMirror 說某一側的肖像要不要左右翻。
//
// **攻方的肖像是鏡像**：基準畫面上守方的 `F014.FAC` 直接對得上，
// 攻方的 `F228.FAC` 要左右翻過來才 100%（各 4736 格，下緣被畫面切掉）。
// 攻方的框在左（寬版面）或在上（窄版面），兩人因此面對面。
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
//	日數與時辰          y 228–323  （`0x22200(8, 228, 39, 323)`，與面板同高）
//
// 框與框之間露出底紋。右邊 x 54–55 那兩條黑線是場地的左緣，不屬於框
// （`0x22527`、`0x22542`），長度隨版面走（`FieldLines`）。
var battleLeftBoxes = [4][2]int{{52, 147}, {155, 187}, {196, 211}, {228, 323}}

// BattleLeftBox 回傳左欄第 i 個框的上下緣。
func BattleLeftBox(i int) (y0, y1 int) { return battleLeftBoxes[i][0], battleLeftBoxes[i][1] }

// BattleLeftBoxX0／X1 是左欄三個框共用的左右緣。
const (
	BattleLeftBoxX0 = 8
	BattleLeftBoxX1 = 39
)

// LeftColumn 畫左欄的四個框。
func (im *Image) LeftColumn() {
	for _, b := range battleLeftBoxes {
		im.FillRect(BattleLeftBoxX0, b[0],
			BattleLeftBoxX1-BattleLeftBoxX0+1, b[1]-b[0]+1, BattleOrderPaper)
		im.BevelBox(BattleLeftBoxX0, b[0], BattleLeftBoxX1, b[1])
	}
}

// FieldLines 畫場地左緣的兩條黑線與右緣的兩條白線（`0x2250c`–`0x22594`、
// `0x2260e`–`0x22694`）：左邊 x 54 畫到 LeftY1、x 55 少一格；右邊
// (RightX, 50–51) 先畫黑，再從 51 起蓋白到 RightY1，RightX+1 從 50 起。
func (im *Image) FieldLines(l BattleLayout) {
	im.vline(l.RightX, 50, 51, 0)
	im.vline(BattleFieldLeft, BattleFieldTop, l.LeftY1, 0)
	im.vline(BattleFieldLeft+1, BattleFieldTop, l.LeftY1-1, 0)
	im.vline(l.RightX, 51, l.RightY1, 15)
	im.vline(l.RightX+1, 50, l.RightY1, 15)
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

// 智冠的商標畫面是 `DATA1` 的 `CMARKL`／`CMARKR` 兩半，各 320×290，
// 原版 `AA.EXE` 開機問完三個裝置之後的第一幕（`docs/spec/005` §「商標畫面」）。
// 底是黑的，兩半並排在 y ＝ TrademarkY。
const (
	TrademarkPieceW = 320
	TrademarkPieceH = 290
	TrademarkY      = 64
)

// TrademarkScreen 拼出商標畫面。
func TrademarkScreen(data1 *Container) (*Image, error) {
	im := &Image{W: ScreenW, H: ScreenH, Pix: make([]byte, ScreenW*ScreenH)}
	for i, name := range []string{"CMARKL.IMG", "CMARKR.IMG"} {
		j, ok := data1.ByName(name)
		if !ok {
			return nil, fmt.Errorf("assets: DATA1 裡沒有 %s", name)
		}
		p, err := DecodeImage(data1.Data(j))
		if err != nil {
			return nil, err
		}
		if p.W != TrademarkPieceW || p.H != TrademarkPieceH {
			return nil, fmt.Errorf("assets: %s 是 %d×%d，商標畫面的一半應該是 %d×%d",
				name, p.W, p.H, TrademarkPieceW, TrademarkPieceH)
		}
		im.Blit(p, i*TrademarkPieceW, TrademarkY)
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
// 原版每 96 像素（兩欄）畫一組：上緣 `0x21ee2`，兩種版面共用，
// x ＝ 96i + 56、y ＝ 36；下緣寬版面用 `0x220f0`（y ＝ 228，第 6 列只有
// 偶數欄，所以奇數欄那一半的白線高 16）、窄版面用 `0x21fe0`（y ＝ 324，
// 第 9 列兩欄都有）。每一支都是把黑線畫在上與左、白線畫在下與右——
// **奇數欄低 16 像素**，所以每一組都是一個階梯。
//
// 端點逐條抄自那三支常式；差一格的參差就是立體感的來源。
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

func (im *Image) fieldEdgeBottomWide(x, y int) {
	im.vline(x-1, y+17, y+32, 0)
	im.vline(x-2, y+16, y+31, 0)
	im.hline(x-1, x+49, y+32, 15)
	im.hline(x-2, x+49, y+33, 15)
	im.vline(x+48, y+16, y+33, 15)
	im.vline(x+49, y+16, y+33, 15)
	im.hline(x+48, x+95, y+16, 15)
	im.hline(x+48, x+94, y+17, 15)
}

func (im *Image) fieldEdgeBottomNarrow(x, y int) {
	im.hline(x-2, x+46, y+32, 15)
	im.hline(x-2, x+45, y+33, 15)
	im.vline(x+46, y+33, y+48, 0)
	im.vline(x+47, y+32, y+47, 0)
	im.hline(x+47, x+95, y+48, 15)
	im.hline(x+46, x+95, y+49, 15)
	im.vline(x+96, y+32, y+49, 15)
	im.vline(x+97, y+32, y+49, 15)
}

// FieldEdges 畫整個場地的邊框。
func (im *Image) FieldEdges(l BattleLayout) {
	for i := 0; i < l.Groups; i++ {
		x := i*96 + FieldOriginX
		im.fieldEdgeTop(x, FieldOriginY)
		if l.Narrow {
			im.fieldEdgeBottomNarrow(x, l.BottomY)
		} else {
			im.fieldEdgeBottomWide(x, l.BottomY)
		}
	}
}
