package ui

import (
	"image/color"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

// 訊息框（原版的訊息常式 `0x3273e`，`docs/spec/005` §9，`L0`、`[base]`）：
// 說話者的肖像、名字、一個白色的對白泡泡與兩行字。呼叫端給框的四個角
// （`x1, y1, x2, y2`）與肖像在左還是右，其餘的落點全是從那四個數字算的：
//
//	肖像 64×80    左：(x1, y1)              右：(x2−63, y1)
//	名字 16×16    左：(x1+8, y1+80)          右：(x2−55, y1+80)   黑底
//	泡泡（白）    左：(x1+70, y1+5)–(x2−5, y2−5)   右：(x1+4, y1+5)–(x2−70, y2−5)
//	上下緣        y1+4、y2−4 各一條，與泡泡同寬
//	左右緣        左：x1+69、x2−4            右：x1+3、x2−69
//	尾巴          四條直線往肖像那一側收成三角，尖端在 y1+47～48
//	對白          左：(x1+72, y1+12)         右：(x1+8, y1+12)；第二行 +40
//
// 對白的字是 16×32（原版 `sy=2`），**字色是進去就擲的 `RND(8)`**
// （`0x32d4d`），名字左邊那一格淺紅（12）、右邊那一格淺綠（10）。
// 一行放 `(x2−x1−79) ÷ 16` 個全形字，放不下的接到第二行。
const (
	bubblePortraitW = 64
	bubblePortraitH = 80
	bubbleLineGap   = 40
	bubbleTextScale = 2
)

var (
	bubbleWhite     = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF} // 15
	bubbleBlack     = color.RGBA{0x00, 0x00, 0x00, 0xFF} // 0
	bubbleNameLeft  = color.RGBA{0xFF, 0x55, 0x55, 0xFF} // 12
	bubbleNameRight = color.RGBA{0x55, 0xFF, 0x55, 0xFF} // 10
)

// BubbleColumns 是一行放幾個全形字（原版 `(x2 − x1 − 79) ÷ 16`）。
func BubbleColumns(b *game.Bubble) int {
	n := (b.X2 - b.X1 - 79) / 16
	if n < 1 {
		n = 1
	}
	return n
}

// BubbleLines 把對白拆成兩行：原版是照位元組數硬切（全形字剛好切在
// 字的邊界），remake 照顯示寬度切；**有拉丁字母的譯文改在空白處折**
// （`docs/spec/014` 的規則，記為 remake 差異——原版沒有英文對白）。
// 第二行放不下的部分**截掉**，原版也只畫兩行。
func BubbleLines(b *game.Bubble) [2]string {
	cols := BubbleColumns(b) * 2
	var out [2]string
	if artHasLatin(b.Text) {
		for i, line := range cells.Wrap(b.Text, cols) {
			if i >= len(out) {
				break
			}
			out[i] = strings.TrimRight(cells.Truncate(line, cols), " ")
		}
		return out
	}
	rest := b.Text
	for i := range out {
		if cells.Width(rest) <= cols {
			out[i] = rest
			break
		}
		cut := cells.Truncate(rest, cols)
		out[i] = cut
		rest = rest[len(cut):]
	}
	return out
}

// DrawBubble 在畫布上畫一格訊息框。肖像從 `a` 取；`a` 為 nil（沒有
// 原版素材）時只畫名字、泡泡與字。
func DrawBubble(c *Canvas, a *ArtScreen, g *game.State, b *game.Bubble) {
	x1, y1, x2, y2 := b.X1, b.Y1, b.X2, b.Y2
	name := ""
	portrait := -1
	if x := g.General(b.Speaker); x != nil {
		name = i18n.PersonName(x.Name)
		portrait = int(x.Portrait)
	}
	// 肖像：左邊那一張左右翻（原版把 side 當翻面旗傳給畫肖像常式，
	// 主戰場攻方那一張同一個形狀），臉朝著泡泡。
	px := x2 - bubblePortraitW + 1
	if b.Left {
		px = x1
	}
	if a != nil && portrait >= 0 {
		if face := a.Portrait(portrait); face != nil {
			if b.Left {
				face = face.Mirror()
			}
			drawImageAt(c, face, px, y1)
		}
	}
	// 名字：原版畫的是人物表那 6 個位元組（兩字名前後各一個空白），
	// 底色黑——所以黑底剛好是三個全形字寬。
	nx, ink := x1+8, bubbleNameLeft
	if !b.Left {
		nx, ink = x2-55, bubbleNameRight
	}
	ny := y1 + bubblePortraitH
	c.FillRect(nx, ny, nx+3*CellW*2, ny+CellH, bubbleBlack)
	label := name
	if w := cells.Width(label); w < 6 {
		pad := (6 - w) / 2
		label = cells.Pad(spaces(pad)+label, 6)
	}
	c.DrawTextPx(nx, ny, cells.Truncate(label, 6), ink)

	// 泡泡：白底、上下各多一條、左右各一條（四角因此是圓的），
	// 再四條直線往肖像那邊收成尾巴。座標全是**含端點**的。
	var bx1, bx2 int
	if b.Left {
		bx1, bx2 = x1+70, x2-5
	} else {
		bx1, bx2 = x1+4, x2-70
	}
	by1, by2 := y1+5, y2-5
	c.FillRect(bx1, by1, bx2+1, by2+1, bubbleWhite)
	c.FillRect(bx1, y1+4, bx2+1, y1+5, bubbleWhite)
	c.FillRect(bx1, y2-4, bx2+1, y2-3, bubbleWhite)
	c.FillRect(bx1-1, by1, bx1, by2+1, bubbleWhite)
	c.FillRect(bx2+1, by1, bx2+2, by2+1, bubbleWhite)
	for i := 0; i < 4; i++ {
		// 左：x1+65..x1+68，右：x2−65..x2−68；越靠泡泡越長。
		tx := x2 - 65 - i
		if b.Left {
			tx = x1 + 65 + i
		}
		c.FillRect(tx, y1+47-i, tx+1, y1+49+i, bubbleWhite)
	}

	// 對白：兩行 16×32，字色照擲出來的那一格。
	tx := x1 + 8
	if b.Left {
		tx = x1 + 72
	}
	fg := assets.EGAPalette[b.Color&7]
	for i, line := range BubbleLines(b) {
		x := tx
		for _, r := range line {
			x += c.DrawRuneScaledPx(x, y1+12+i*bubbleLineGap, r, fg, 1, bubbleTextScale)
		}
	}
}

// drawImageAt 把一張原版的圖貼到畫布的像素座標上。
func drawImageAt(c *Canvas, im *assets.Image, x, y int) {
	for sy := 0; sy < im.H; sy++ {
		for sx := 0; sx < im.W; sx++ {
			c.setClipped(x+sx, y+sy, assets.EGAPalette[im.Pix[sy*im.W+sx]&15])
		}
	}
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
