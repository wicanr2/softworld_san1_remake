// Package ui 把文字與圖塊畫到畫布上。
//
// **這個檔不依賴 Ebiten**：它畫到 `image.RGBA`，所以無頭環境測得到、
// 也能在 CI 裡做像素比對。Ebiten 那一層只負責把畫布貼到視窗上
// （`cmd/san1`）。
//
// 分這一刀的理由與 `internal/cells` 相同：**畫面 bug 測試看不到**
// （`CLAUDE.md` §7 第 13 條）。畫到記憶體裡的圖就看得到了。
package ui

import (
	"image"
	"image/color"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
)

// CellW 是一個半形格的像素寬，CellH 是列高。
//
// 原版一般字級是 8×16／16×16（`CLAUDE.md` §3.3）。半形格 8 像素、
// 全形字兩格 16 像素，與點陣字型的尺寸一致，所以不需要縮放——
// **CJK 點陣字縮放會糊掉**，這是不縮放的理由。
const (
	CellW = 8
	CellH = 16
)

// Canvas 是一張以格為單位的畫布。
type Canvas struct {
	Img        *image.RGBA
	Cols, Rows int
	face       *font.Face

	// Missing 累計畫不出來的字元。
	//
	// **缺字在畫面上是空白，而空白看起來像排版問題。** 累計起來才問得到
	// 「這一畫面有沒有字沒畫出來」，不然只能靠眼睛看。
	Missing map[rune]int

	// small 是小字級（`fonts/ascii6x10.hex.gz`，6×10）；nil 表示沒有，
	// 那時 `FitsSmall` 一律回 false、呼叫端照原尺寸畫。
	small *font.Face

	// Clipped 累計 `DrawText` 在畫布右緣丟掉的字數。
	//
	// **溢出在畫面上是少了幾個字，而少了幾個字看起來像譯文本來就這樣。**
	// 貼著右緣的那一欄（文字版的資料欄與指令欄）一溢出就是在這裡被丟掉，
	// 累計起來才問得到「這一畫面有沒有字被截」。
	Clipped int
}

// NewCanvas 開一張 cols × rows 格的畫布。
func NewCanvas(cols, rows int, face *font.Face) *Canvas {
	return NewCanvasPx(cols*CellW, rows*CellH, face)
}

// NewCanvasPx 開一張指定像素尺寸的畫布。
//
// 接原版素材的畫面要 640×408，那個高度不是列高的整數倍
// （408 ÷ 16 ＝ 25.5）。**格數往下取整**，最後那一列會被裁掉一半
// ——原版的版面本來就是按像素排的，不是按格。
func NewCanvasPx(w, h int, face *font.Face) *Canvas {
	return &Canvas{
		Img:     image.NewRGBA(image.Rect(0, 0, w, h)),
		Cols:    w / CellW,
		Rows:    h / CellH,
		face:    face,
		Missing: map[rune]int{},
	}
}

// 小字級的尺寸：X11 misc-fixed 6×10，一字 6 像素寬、10 像素高
//（HEX 裡存成 8 寬，右邊兩行是空的）。
const (
	SmallW = 6
	SmallH = 10
)

// SetSmallFace 接上小字級。
//
// **小字級只用在「量得出原尺寸放不下」的地方**（`fonts/README.md`）：
// 判準是寬度不是語系。英文的子選單在原版的下面板裡原尺寸排不下，
// 用小字就排得下，而且留在原版的位置（`docs/spec/014` §3.2）。
func (c *Canvas) SetSmallFace(f *font.Face) { c.small = f }

// FitsSmall 回報這串字能不能全部用小字級畫（每個字都有字模）。
// 小字級只有 ASCII，中日文一律回 false。
func (c *Canvas) FitsSmall(s string) bool {
	if c.small == nil {
		return false
	}
	return len(c.small.Covers(s)) == 0
}

// DrawSmallTextPx 用小字級從像素座標畫一行字，回傳畫掉的寬度。
// 字型沒有的字什麼都不畫、記進 Missing——呼叫端應該先問 `FitsSmall`。
func (c *Canvas) DrawSmallTextPx(x, y int, s string, fg color.RGBA) int {
	x0 := x
	b := c.Img.Bounds()
	for _, r := range s {
		g, ok := c.small.Glyph(r)
		if !ok {
			c.Missing[r]++
			x += SmallW
			continue
		}
		for gy := 0; gy < g.H; gy++ {
			for gx := 0; gx < g.W; gx++ {
				if !g.At(gx, gy) {
					continue
				}
				xx, yy := x+gx, y+gy
				if xx >= b.Min.X && xx < b.Max.X && yy >= b.Min.Y && yy < b.Max.Y {
					c.Img.SetRGBA(xx, yy, fg)
				}
			}
		}
		x += SmallW
	}
	return x - x0
}

// Fill 把整張畫布塗成單色。
func (c *Canvas) Fill(col color.RGBA) {
	b := c.Img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c.Img.SetRGBA(x, y, col)
		}
	}
}

// FillRect 以**像素**為單位塗一塊。
//
// 接原版素材的畫面要按像素排版，格對不齊——訊息列在原版是壓在地圖
// 底下的一條，不是整格對齊的。
func (c *Canvas) FillRect(x0, y0, x1, y1 int, col color.RGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c.setClipped(x, y, col)
		}
	}
}

// DrawText 在第 (col, row) 格畫一串字，回傳畫了幾格寬。
//
// 超出畫布右緣的部分**不畫也不繞行**：原版的版面是固定格的，
// 繞行會蓋掉下一列的東西。要換行請先用 `cells.Wrap`。
func (c *Canvas) DrawText(col, row int, s string, fg color.RGBA) int {
	x := col
	for _, r := range s {
		w := cells.RuneWidth(r)
		if w == 0 {
			continue
		}
		if x+w > c.Cols {
			c.Clipped += cells.Width(s) - (x - col)
			break
		}
		c.drawRune(x, row, r, fg)
		x += w
	}
	return x - col
}

// DrawTextPx 從任意像素座標畫一行字。
//
// 接原版素材的畫面上，字要對齊的是圖裡的位置，不是格線——例如戰場
// 的兵力牌在旗幟下方 15 個像素（`assets.FlagPlateOffsetY`），那不是
// 列高的倍數。回傳畫掉的寬度。
func (c *Canvas) DrawTextPx(x, y int, s string, fg color.RGBA) int {
	x0 := x
	for _, r := range s {
		w := cells.RuneWidth(r)
		if w == 0 {
			continue
		}
		c.drawRunePx(x, y, r, fg)
		x += w * CellW
	}
	return x - x0
}

// drawRune 畫一個字。字型沒有這個碼位時**什麼都不畫**，只記進 Missing。
func (c *Canvas) drawRune(col, row int, r rune, fg color.RGBA) {
	c.drawRunePx(col*CellW, row*CellH, r, fg)
}

func (c *Canvas) drawRunePx(px, py int, r rune, fg color.RGBA) {
	c.drawRuneWidePx(px, py, r, fg, 1)
}

// DrawRuneWidePx 從像素座標畫一個橫向放大的字，回傳畫掉的寬度。
//
// 原版主選單左邊那面直牌上的四個字是這樣畫的：16×16 的字模畫成 32×16
// （`assets.MenuLabelScaleX`）。**只放寬不放高**——高度跟著放大就變成
// 32 列，牌子的內框裝不下四個字。
func (c *Canvas) DrawRuneWidePx(px, py int, r rune, fg color.RGBA, sx int) int {
	return c.DrawRuneScaledPx(px, py, r, fg, sx, 1)
}

// DrawRuneScaledPx 從像素座標畫一個放大的字，回傳畫掉的寬度。
//
// 主畫面的郡名是 32×32（`sx=2, sy=2`），面板上的編號是雙倍寬
// （`sx=2, sy=1`）——原版兩種都用。
func (c *Canvas) DrawRuneScaledPx(px, py int, r rune, fg color.RGBA, sx, sy int) int {
	if sx < 1 {
		sx = 1
	}
	if sy < 1 {
		sy = 1
	}
	c.drawRuneScaledPx(px, py, r, fg, sx, sy)
	return cells.RuneWidth(r) * CellW * sx
}

// DrawRuneOutlinedPx 畫一個放大的字，四周描一圈 `edge` 色的邊（八個方向
// 各一像素），回傳畫掉的寬度。原版選君主那一格的名字與編號是白字黑邊
// （`docs/spec/005` §9.4）。
func (c *Canvas) DrawRuneOutlinedPx(px, py int, r rune, fg, edge color.RGBA, sx, sy int) int {
	if sx < 1 {
		sx = 1
	}
	if sy < 1 {
		sy = 1
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx != 0 || dy != 0 {
				c.drawRuneScaledPx(px+dx, py+dy, r, edge, sx, sy)
			}
		}
	}
	c.drawRuneScaledPx(px, py, r, fg, sx, sy)
	return cells.RuneWidth(r) * CellW * sx
}

// StrokeRect 畫一個空心矩形，四個角含端點。
func (c *Canvas) StrokeRect(x0, y0, x1, y1 int, col color.RGBA) {
	for x := x0; x <= x1; x++ {
		c.setClipped(x, y0, col)
		c.setClipped(x, y1, col)
	}
	for y := y0; y <= y1; y++ {
		c.setClipped(x0, y, col)
		c.setClipped(x1, y, col)
	}
}

func (c *Canvas) drawRuneWidePx(px, py int, r rune, fg color.RGBA, sx int) {
	c.drawRuneScaledPx(px, py, r, fg, sx, 1)
}

func (c *Canvas) drawRuneScaledPx(px, py int, r rune, fg color.RGBA, sx, sy int) {
	g, ok := c.face.Glyph(r)
	if !ok {
		c.Missing[r]++
		return
	}

	// 字型的高度可能與列高不同（例如 6×10 的小字級放進 16 像素的列）。
	// 垂直置底對齊，讓基線一致。
	off := CellH - g.H
	if off < 0 {
		off = 0
	}
	for gy := 0; gy < g.H; gy++ {
		for ky := 0; ky < sy; ky++ {
			yy := py + (gy+off)*sy + ky
			if yy < 0 || yy >= c.Img.Bounds().Dy() {
				continue
			}
			for gx := 0; gx < g.W; gx++ {
				if !g.At(gx, gy) {
					continue
				}
				for kx := 0; kx < sx; kx++ {
					xx := px + gx*sx + kx
					if xx < 0 || xx >= c.Img.Bounds().Dx() {
						continue
					}
					c.Img.SetRGBA(xx, yy, fg)
				}
			}
		}
	}
}

// DrawTextRotatedPx 把一行字順時針轉 90° 畫：字頂朝右、由上往下讀，
// 與英文書脊同一個方向。回傳畫掉的高度。
//
// 給窄直條上的橫書文字用：原版左側直條直排七個全形字，英文逐字母一列
// 直排讀不下去，按字折行又會把「Zhongping」切斷——轉過來排才塞得進
// 同一條直條，而且讀得通。
func (c *Canvas) DrawTextRotatedPx(x, y int, s string, fg color.RGBA) int {
	y0 := y
	b := c.Img.Bounds()
	for _, r := range s {
		w := cells.RuneWidth(r)
		if w == 0 {
			continue
		}
		g, ok := c.face.Glyph(r)
		if !ok {
			c.Missing[r]++
			y += w * CellW
			continue
		}
		off := CellH - g.H
		if off < 0 {
			off = 0
		}
		for gy := 0; gy < g.H; gy++ {
			for gx := 0; gx < g.W; gx++ {
				if !g.At(gx, gy) {
					continue
				}
				// 順時針 90°：字模的 (gx, gy) → 畫布的 (x + 列高−1−列, y + 行)。
				xx, yy := x+CellH-1-(gy+off), y+gx
				if xx >= b.Min.X && xx < b.Max.X && yy >= b.Min.Y && yy < b.Max.Y {
					c.Img.SetRGBA(xx, yy, fg)
				}
			}
		}
		y += w * CellW
	}
	return y - y0
}

// DrawBox 畫一個用單線框起來的方框（格座標，含邊框）。
//
// 框線用點陣畫在格的邊緣，不佔用格——原版的框與字是分開的圖層。
func (c *Canvas) DrawBox(col, row, cols, rows int, col2 color.RGBA) {
	x0, y0 := col*CellW, row*CellH
	x1, y1 := x0+cols*CellW-1, y0+rows*CellH-1
	for x := x0; x <= x1; x++ {
		c.setClipped(x, y0, col2)
		c.setClipped(x, y1, col2)
	}
	for y := y0; y <= y1; y++ {
		c.setClipped(x0, y, col2)
		c.setClipped(x1, y, col2)
	}
}

func (c *Canvas) setClipped(x, y int, col color.RGBA) {
	b := c.Img.Bounds()
	if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
		return
	}
	c.Img.SetRGBA(x, y, col)
}

// InkAt 回傳這一格裡有幾個實心像素。
//
// 測試用：「這一格有沒有畫東西」比「畫得對不對」容易問，
// 而且**畫成空白**是最常見的失敗（缺字、色相同、座標算錯）。
func (c *Canvas) InkAt(col, row int, bg color.RGBA) int {
	n := 0
	for y := row * CellH; y < (row+1)*CellH; y++ {
		for x := col * CellW; x < (col+1)*CellW; x++ {
			if x >= c.Img.Bounds().Dx() || y >= c.Img.Bounds().Dy() {
				continue
			}
			if c.Img.RGBAAt(x, y) != bg {
				n++
			}
		}
	}
	return n
}

// DrawRuneBoxDitherPx 把一個字縮放進一個方框，前景用兩色棋盤畫。
//
// 兩件事是原版的（`docs/spec/011`）：
//
//   - **方框不是格**：主戰場下方花邊上的年月是十個 24 寬的格子，
//     而 remake 的字模是 16×16。整數倍放不進去（16→24 是 1.5 倍），
//     所以這裡按比例取樣，不是整數倍複製。
//   - **前景是兩色棋盤**：`(x+y)` 是奇數用 a、偶數用 b。原版那一行字
//     量到灰（色 7）538 點與綠（色 2）502 點逐像素交錯，相位就是這個
//     ——不是「主色 ＋ 陰影」（右下只有 4 點）。
//
// 字模缺字時記一筆 Missing 就回來，與其他畫字的路徑一致。
func (c *Canvas) DrawRuneBoxDitherPx(px, py, w, h int, r rune, a, b color.RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	g, ok := c.face.Glyph(r)
	if !ok {
		c.Missing[r]++
		return
	}
	if g.W <= 0 || g.H <= 0 {
		return
	}
	for dy := 0; dy < h; dy++ {
		yy := py + dy
		if yy < 0 || yy >= c.Img.Bounds().Dy() {
			continue
		}
		gy := dy * g.H / h
		for dx := 0; dx < w; dx++ {
			xx := px + dx
			if xx < 0 || xx >= c.Img.Bounds().Dx() {
				continue
			}
			if !g.At(dx*g.W/w, gy) {
				continue
			}
			fg := b
			if (xx+yy)%2 == 1 {
				fg = a
			}
			c.Img.SetRGBA(xx, yy, fg)
		}
	}
}
