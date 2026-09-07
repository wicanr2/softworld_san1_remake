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

// Clone 複製一張圖。底圖只拼一次，換色每回合都要重來。
func (im *Image) Clone() *Image {
	out := &Image{W: im.W, H: im.H, Pix: make([]byte, len(im.Pix))}
	copy(out.Pix, im.Pix)
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
