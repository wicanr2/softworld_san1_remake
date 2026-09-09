package assets

import (
	"fmt"
	"image"
)

// 遊戲主畫面的底圖：原版拿 `DATA3` 的七張 `MAINMAP*.IMG` 拼出來。
//
// 位置直接從畫底圖那一段讀（`0x11ebc`–`0x11fc5`，`L0`）。原版每一張都
// 走同一對呼叫——`0x36c9:0x02b0` 依名字載入、`0x36c9:0x0426` 畫到
// `(x, y)`，三個參數推進去的順序是 page、y、x：
//
//	MAINMAP1  (  0,   0)   640×36   上方花邊
//	MAINMAP3  (  0,  36)    72×336  左側直條（年月直排在這裡）
//	MAINMAP4  ( 72,  36)   168×336  地圖左半
//	MAINMAP5  (240,  36)   168×336  地圖右半
//	MAINMAPB  (408,  36)   224×153  右上面板
//	MAINMAPC  (408, 189)   224×184  右下面板
//	MAINMAP7  (632,  36)     8×336  最右直條
//
// `MAINMAP2` 也被畫，但落在 `(0, 372)`——螢幕只有 350 列，它在畫面外。
//
// **底圖之外的東西都是後來蓋上去的**：州郡依所屬換色、郡編號、右面板的
// 文字與肖像、底部的訊息列。所以拿這一份與原版的畫面比，沒被蓋到的地方
// 要 100% 相同，被蓋到的地方不會。實測（`TestMainScreenMatchesTheOriginal`）：
// 上方花邊與最右直條 100%，左側直條 94.3%（年月蓋在上面），
// 地圖區 68.7%（換色與編號），右面板 3.8%（整片被文字蓋掉）。
const (
	// ScreenW／ScreenH 是原版畫面的像素尺寸（EGA mode 10h）。
	ScreenW = 640
	ScreenH = 350
)

// mainScreenPieces 是七張底圖與它們的位置。
var mainScreenPieces = []struct {
	Name string
	X, Y int
}{
	{"MAINMAP1.IMG", 0, 0},
	{"MAINMAP3.IMG", 0, 36},
	{"MAINMAP4.IMG", 72, 36},
	{"MAINMAP5.IMG", 240, 36},
	{"MAINMAPB.IMG", 408, 36},
	{"MAINMAPC.IMG", 408, 189},
	{"MAINMAP7.IMG", 632, 36},
}

// MainScreen 從 `DATA3` 拼出主畫面的底圖，回傳 640×350 的索引色圖。
//
// 回傳的是**色號**（0–15）不是 RGBA：州郡換色是在色號上做的，
// 換成 RGBA 之後再比對顏色會被色盤差異干擾。要畫出來用 `Image.RGBA()`。
func MainScreen(data3 *Container) (*Image, error) {
	dst := &Image{W: ScreenW, H: ScreenH, Pix: make([]byte, ScreenW*ScreenH)}
	for _, p := range mainScreenPieces {
		i, ok := data3.ByName(p.Name)
		if !ok {
			return nil, fmt.Errorf("assets: DATA3 裡沒有 %s", p.Name)
		}
		im, err := DecodeImage(data3.Data(i))
		if err != nil {
			return nil, fmt.Errorf("assets: 解 %s：%w", p.Name, err)
		}
		dst.Blit(im, p.X, p.Y)
	}
	return dst, nil
}

// Blit 把一張圖畫到 (x, y)，超出邊界的部分**裁掉不繞行**。
//
// 裁掉是原版的行為：地圖那三條 336 高的直條畫在 y=36，
// 到 372 才畫完，而畫面只有 350 列。
func (im *Image) Blit(src *Image, x, y int) {
	for sy := 0; sy < src.H; sy++ {
		dy := y + sy
		if dy < 0 || dy >= im.H {
			continue
		}
		for sx := 0; sx < src.W; sx++ {
			dx := x + sx
			if dx < 0 || dx >= im.W {
				continue
			}
			im.Pix[dy*im.W+dx] = src.Pix[sy*src.W+sx]
		}
	}
}

// At 取一個像素的色號；界外回 0。
func (im *Image) At(x, y int) byte {
	if x < 0 || y < 0 || x >= im.W || y >= im.H {
		return 0
	}
	return im.Pix[y*im.W+x]
}

// Set 寫一個像素的色號；界外不做事。
func (im *Image) Set(x, y int, v byte) {
	if x < 0 || y < 0 || x >= im.W || y >= im.H {
		return
	}
	im.Pix[y*im.W+x] = v
}

// FillRect 把一塊矩形塗成同一個顏色。超出邊界的部分裁掉。
func (im *Image) FillRect(x, y, w, h int, v byte) {
	for dy := 0; dy < h; dy++ {
		yy := y + dy
		if yy < 0 || yy >= im.H {
			continue
		}
		for dx := 0; dx < w; dx++ {
			xx := x + dx
			if xx < 0 || xx >= im.W {
				continue
			}
			im.Pix[yy*im.W+xx] = v
		}
	}
}

// Clone 複製一張圖。底圖只拼一次，換色每回合都要重來。
func (im *Image) Clone() *Image {
	out := &Image{W: im.W, H: im.H, Pix: make([]byte, len(im.Pix))}
	copy(out.Pix, im.Pix)
	return out
}

// Complement 把每一格的顏色取補數（`^15`），也就是原版的反白。
//
// 選到的部隊會閃：原版把反白旗標打開畫一次、關掉再畫一次
// （`0x275b9`–`0x275ec`，旗標由 `es:[0x2e78]` 的間接呼叫切換）。
// 紮完寨的基準畫面剛好停在亮的那一格，陳就的中軍就是 `WFLAGA00`
// 每格 `^15`，360 格逐格相同。
func (im *Image) Complement() *Image {
	out := im.Clone()
	for i, v := range out.Pix {
		out.Pix[i] = v ^ 0x0F
	}
	return out
}

// SubImage 取一塊矩形，給比對用。
func (im *Image) SubImage(r image.Rectangle) *Image {
	r = r.Intersect(image.Rect(0, 0, im.W, im.H))
	out := &Image{W: r.Dx(), H: r.Dy(), Pix: make([]byte, r.Dx()*r.Dy())}
	for y := 0; y < out.H; y++ {
		copy(out.Pix[y*out.W:(y+1)*out.W], im.Pix[(r.Min.Y+y)*im.W+r.Min.X:])
	}
	return out
}

// MapOriginX／MapOriginY 是州郡座標（記錄 offset 6–9）對到畫面的位移。
//
// **直接讀碼不要用擬合**（`0x10ceb`／`0x10cf4`）：
//
//	ax = MapY(offset 8); ax += 0x2c   → 螢幕 y
//	ax = MapX(offset 6); ax += 0x50   → 螢幕 x
//
// 拿「42 個種子點都落在白色」去掃位移會得到 99 組解——白色的區域很大，
// 那個判準太鬆。碼裡是一個數字。
const (
	MapOriginX = 0x50
	MapOriginY = 0x2c
)

// FloodFill 從 (x, y) 把相連的同色像素換成 to，回傳換了幾個像素。
//
// 四方向相連。原版的州郡是用黑色邊界圍起來的白色區塊，所以從州郡座標
// 灌下去就會停在邊界上。
func (im *Image) FloodFill(x, y int, to byte) int {
	from := im.At(x, y)
	if from == to || x < 0 || y < 0 || x >= im.W || y >= im.H {
		return 0
	}
	n := 0
	stack := []int{y*im.W + x}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if im.Pix[p] != from {
			continue
		}
		im.Pix[p] = to
		n++
		px, py := p%im.W, p/im.W
		if px > 0 {
			stack = append(stack, p-1)
		}
		if px < im.W-1 {
			stack = append(stack, p+1)
		}
		if py > 0 {
			stack = append(stack, p-im.W)
		}
		if py < im.H-1 {
			stack = append(stack, p+im.W)
		}
	}
	return n
}

// 主選單畫面（`DATA3` 的 `MENU*`）。
//
// 這一張的程式不在主程式裡——主選單跑在開機鏈的第二層（`DATA0.GRP`），
// 而我們的碼段 dump 只涵蓋 `DATA5.GRP`（`docs/re/02` §1）。所以位置不是
// 讀碼讀出來的，是**拿原版的畫面逐像素比對出來的**（`docs/playtest/03`）：
//
//	MENU0A.IMG  (40, 27)   280×180  標題牌左半      100%
//	MENU0B.IMG  (320, 27)  288×180  標題牌右半      100%
//	MENU1.IMG   (56, 215)  96×151   左側直牌        95.3%（字寫在上面）
//	MENU2.IMG   六格按鈕，x ∈ {152, 376}、y ∈ {215, 267, 320}  93–97%
//	MENU3.IMG   (576, 320) 40×41    右下角小飾框    91%（裡面那格會動）
//
// 三列的間距是 52、53 不是兩個 52：第三列在 `y=320`，比等距多一個像素。
// 直牌與第三列按鈕的下緣都超出 350 被裁掉，畫面上只看得到上半。
//
// 不足 100% 的四張都是「圖上面還寫了東西」：按鈕與直牌上有字，
// 小飾框裡那 8×12 格是一段動畫（同一台原版連拍兩張就會不同）。
var menuScreenPieces = []struct {
	Name string
	X, Y int
}{
	{"MENU0A.IMG", 40, 27},
	{"MENU0B.IMG", 320, 27},
	{"MENU1.IMG", 56, 215},
	{"MENU2.IMG", 152, 215},
	{"MENU2.IMG", 376, 215},
	{"MENU2.IMG", 152, 267},
	{"MENU2.IMG", 376, 267},
	{"MENU2.IMG", 152, 320},
	{"MENU2.IMG", 376, 320},
	{"MENU3.IMG", 576, 320},
}

// MenuScreenBG 是主選單的底色（原版是一片藍）。
const MenuScreenBG = 9

// MenuScreen 從 `DATA3` 拼出主選單畫面。
func MenuScreen(data3 *Container) (*Image, error) {
	dst := &Image{W: ScreenW, H: ScreenH, Pix: make([]byte, ScreenW*ScreenH)}
	for i := range dst.Pix {
		dst.Pix[i] = MenuScreenBG
	}
	for _, p := range menuScreenPieces {
		i, ok := data3.ByName(p.Name)
		if !ok {
			return nil, fmt.Errorf("assets: DATA3 裡沒有 %s", p.Name)
		}
		im, err := DecodeImage(data3.Data(i))
		if err != nil {
			return nil, fmt.Errorf("assets: 解 %s：%w", p.Name, err)
		}
		dst.Blit(im, p.X, p.Y)
	}
	return dst, nil
}

// MenuButtons 是六個按鈕的左上角，順序與原版的編號相同
// （1 開始新遊戲、2 載入舊進度、3 使用楷書字、4 使用隸書字、
// 5 音樂欣賞、6 回作業系統）。
//
// **編號是橫著走的**：1、4 在第一列，2、5 在第二列——原版畫面上
// 左欄是 1／2／3、右欄是 4／5／6，所以左欄先排完再排右欄。
func MenuButtons() [6][2]int {
	return [6][2]int{
		{152, 215}, {152, 267}, {152, 320},
		{376, 215}, {376, 267}, {376, 320},
	}
}

// 按鈕上那一行字的版面，相對於按鈕左上角，量自原版畫面。
//
// 原版把編號寫成半形的「數字 ＋ 句點」，中文從第三個半形格之後才開始，
// **而且每個中文字之間空一個半形格**——所以中文的字距是 24 不是 16。
// 照 16 排會擠在左半邊，右邊留一塊空白，看起來像沒有置中。
const (
	// MenuTextX 是編號那個半形格的左緣。
	MenuTextX = 24
	// MenuTextY 是文字格的上緣。字高 16，格內上下各留一列。
	MenuTextY = 9
	// MenuTextCJKX 是第一個中文字格的左緣。
	MenuTextCJKX = 48
	// MenuTextCJKPitch 是中文字距。
	MenuTextCJKPitch = 24
)

// 左側直牌上那四個字（主／選／擇／單）的版面，絕對座標。
//
// **這四個字是橫向拉成兩倍寬畫的**：16×16 的字模畫成 32×16，
// 所以牌子雖然只有 96 像素寬，字看起來比按鈕上的大一號。
const (
	// MenuLabelX 是字格的左緣。
	MenuLabelX = 80
	// MenuLabelY 是第一個字格的上緣。
	MenuLabelY = 242
	// MenuLabelPitch 是四個字的行距。
	MenuLabelPitch = 24
	// MenuLabelScaleX 是橫向放大倍率。
	MenuLabelScaleX = 2
)

// FillPatternSize 是一塊填色圖樣的邊長。
const FillPatternSize = 8

// FillPattern 是一個勢力的填色圖樣：8×8 的顏色索引。
type FillPattern [FillPatternSize * FillPatternSize]byte

// At 取圖樣在畫面座標 (x, y) 該用的顏色。**看的是畫面座標不是區塊內
// 座標**——原版的網點是對齊畫面的，跨區塊接得起來。
func (p *FillPattern) At(x, y int) byte {
	return p[(y%FillPatternSize)*FillPatternSize+x%FillPatternSize]
}

// FillPatterns 解 `EGAFILL.PAL`：十六個勢力各一塊 8×8 的填色圖樣。
//
// 檔案 1024 byte ＝ 16 × 64，一格一個位元組、值就是 EGA 的顏色索引。
// 前四個是純色（12、9、10、14），其餘十二個是兩色的 2×2 網點
// （例如第五個是 13／14 交錯）——所以**填色不是「一個勢力一個顏色」**，
// 拿單一顏色去畫，十六個勢力裡有十二個會錯。
func FillPatterns(data1 *Container) ([16]FillPattern, error) {
	var out [16]FillPattern
	i, ok := data1.ByName("EGAFILL.PAL")
	if !ok {
		return out, fmt.Errorf("assets: DATA1 裡沒有 EGAFILL.PAL")
	}
	d := data1.Data(i)
	if len(d) != len(out)*len(out[0]) {
		return out, fmt.Errorf("assets: EGAFILL.PAL 是 %d bytes，應該是 %d",
			len(d), len(out)*len(out[0]))
	}
	for k := range out {
		copy(out[k][:], d[k*len(out[k]):])
	}
	return out, nil
}

// FloodFillPattern 與 FloodFill 一樣，但填的是圖樣。
//
// **要另外記走過哪些格**：填進去的顏色每一格不同，不能再用「顏色還等於
// 起點的顏色」當作沒填過——那樣網點的第二個顏色會被當成沒填過而重來。
func (im *Image) FloodFillPattern(x, y int, pat *FillPattern) int {
	if x < 0 || y < 0 || x >= im.W || y >= im.H {
		return 0
	}
	from := im.At(x, y)
	seen := make([]bool, len(im.Pix))
	n := 0
	stack := []int{y*im.W + x}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[p] || im.Pix[p] != from {
			continue
		}
		seen[p] = true
		px, py := p%im.W, p/im.W
		im.Pix[p] = pat.At(px, py)
		n++
		if px > 0 {
			stack = append(stack, p-1)
		}
		if px < im.W-1 {
			stack = append(stack, p+1)
		}
		if py > 0 {
			stack = append(stack, p-im.W)
		}
		if py < im.H-1 {
			stack = append(stack, p+im.W)
		}
	}
	return n
}
