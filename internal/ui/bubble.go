package ui

import (
	"image"
	"image/color"
	"image/draw"
	"strings"

	xdraw "golang.org/x/image/draw"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
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

type bubbleTextPlan struct {
	lines        []string
	top, gap, sy int
	small        bool
}

// 長譯文沿用原框與既有字模縮排；繁中及可放下的兩行維持原版畫法。
// 契約：spec/021 §6.54.2。無法完整排入時保留舊回退，不增加續頁。
func planBubbleText(c *Canvas, b *game.Bubble) bubbleTextPlan {
	legacy := BubbleLines(b)
	out := bubbleTextPlan{lines: legacy[:], top: 12, gap: bubbleLineGap, sy: bubbleTextScale}
	if i18n.Current == i18n.ZhHant {
		return out
	}
	cols := BubbleColumns(b) * 2
	lines := cells.Wrap(b.Text, cols)
	if len(lines) <= 2 {
		return out
	}
	height := b.Y2 - b.Y1 - 12 // Y1+8 至 Y2−5，含下端。
	if len(lines)*CellH <= height {
		out = bubbleTextPlan{lines: lines, top: 8, gap: CellH, sy: 1}
	} else if c.FitsSmall(b.Text) {
		smallLines := cells.Wrap(b.Text, cols*CellW/SmallW)
		if len(smallLines)*SmallH <= height {
			out = bubbleTextPlan{lines: smallLines, top: 8, gap: SmallH, sy: 1, small: true}
		}
	}
	for i := range out.lines {
		out.lines[i] = strings.TrimRight(out.lines[i], " ")
	}
	return out
}

// DrawBubble 在畫布上畫一格訊息框。肖像從 `a` 取；`a` 為 nil（沒有
// 原版素材）時只畫名字、泡泡與字。
func DrawBubble(c *Canvas, a *ArtScreen, g *game.State, b *game.Bubble) {
	if b.Scene > 0 {
		// 場景圖那一格：整張拉進 (X1, Y1)（`docs/spec/010`）。
		DrawScene(c, a, b.Scene, WipeKind(b.Style), b.X1, b.Y1, sceneAllSteps)
		return
	}
	if b.Card {
		// 人物資料卡那一格：整塊右側面板換成說話者的卡（§9.2）。
		DrawPersonCard(c, a, g, b.Speaker)
		return
	}
	name := ""
	portrait := -1
	if x := g.General(b.Speaker); x != nil {
		name = i18n.PersonName(x.Name)
		portrait = int(x.Portrait)
	}
	DrawBubbleAs(c, a, b, name, portrait)
}

// DrawBubbleAs 是 DrawBubble 的本體：說話者不查人物表，名字與肖像編號
// 直接給（還沒進人物表的新君主用，`docs/spec/005` §9.5）。
func DrawBubbleAs(c *Canvas, a *ArtScreen, b *game.Bubble, name string, portrait int) {
	x1, y1, x2, y2 := b.X1, b.Y1, b.X2, b.Y2
	// 只亮肖像的那一格（尋訪找到人）：貼在 (x1, y1)，不翻面、沒有別的。
	if b.FaceOnly {
		if a != nil && portrait >= 0 {
			if face := a.Portrait(portrait); face != nil {
				drawImageAt(c, face, x1, y1)
			}
		}
		return
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
	if i18n.Current != i18n.En || cells.Width(name) <= 6 || !c.FitsSmall(name) {
		c.DrawTextPx(nx, ny, cells.Truncate(label, 6), ink)
	} else {
		c.drawBubbleName(nx, ny, name, ink)
	}

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

	// 對白字色照擲出來的那一格；長譯文在原白色字區內縮排。
	tx := x1 + 8
	if b.Left {
		tx = x1 + 72
	}
	fg := assets.EGAPalette[b.Color&7]
	plan := planBubbleText(c, b)
	for i, line := range plan.lines {
		y := y1 + plan.top + i*plan.gap
		if plan.small {
			c.DrawSmallTextPx(tx, y, line, fg)
			continue
		}
		x := tx
		for _, r := range line {
			x += c.DrawRuneScaledPx(x, y, r, fg, 1, plan.sy)
		}
	}
}

// spec/021 §6.61：原寬姓名牌以完整小字縮排。高清直接使用 4× 字模。
func (c *Canvas) drawBubbleName(x, y int, name string, ink color.RGBA) {
	w := len([]rune(name)) * SmallW
	text := image.NewRGBA(image.Rect(0, 0, w*4, SmallH*4))
	for i, r := range []rune(name) {
		g, _ := c.small.Glyph(r) // 呼叫端已核對完整字庫。
		for gy := 0; gy < g.H; gy++ {
			for gx := 0; gx < SmallW; gx++ {
				if g.At(gx, gy) {
					draw.Draw(text, image.Rect((i*SmallW+gx)*4, gy*4, (i*SmallW+gx+1)*4, (gy+1)*4), image.NewUniform(ink), image.Point{}, draw.Src)
				}
			}
		}
	}
	high := image.NewRGBA(image.Rect(0, 0, 48*4, CellH*4))
	draw.Draw(high, high.Bounds(), image.NewUniform(bubbleBlack), image.Point{}, draw.Src)
	tw, th := text.Bounds().Dx(), text.Bounds().Dy()
	if tw > high.Bounds().Dx() {
		th = max(1, th*high.Bounds().Dx()/tw)
		tw = high.Bounds().Dx()
	}
	left, top := (high.Bounds().Dx()-tw)/2, (high.Bounds().Dy()-th)/2
	xdraw.NearestNeighbor.Scale(high, image.Rect(left, top, left+tw, top+th), text, text.Bounds(), draw.Over, nil)
	// 原貌仍由 CPU 畫布供應，後續覆蓋保留同一圖層契約。
	for gy := 0; gy < CellH; gy++ {
		for gx := 0; gx < 48; gx++ {
			c.setClipped(x+gx, y+gy, high.RGBAAt(gx*4, gy*4))
		}
	}
	if c.HD != nil {
		c.addHigh(high, image.Rect(x, y, x+48, y+CellH), image.Point{})
	}
}

// DrawBattleSpeech 在主戰場上畫一句對白：那一塊面板先填成藍（原版
// `es:0x252e` 填 1），再照訊息框的規矩畫肖像、名字、泡泡與字
// （`docs/spec/005` §9.7）。那一塊在哪隨版面走（寬版面在下、窄版面在右）。
// 肖像從 `a` 取（`a` 可為 nil）。
func DrawBattleSpeech(c *Canvas, a *ArtScreen, g *game.State, b *battle.Battle, sp *battle.Speech) {
	if sp.Scene > 0 {
		DrawBattleScene(c, a, sp, sceneAllSteps)
		return
	}
	if sp.LureFlash {
		return // 閃爍那一格由 DrawLureFlash 畫
	}
	x1, y1, x2, y2 := assets.BattleLayoutFor(b.Field.Narrow()).Panel(sp.Box.Panel())
	c.FillRect(x1, y1, x2+1, y2+1, assets.EGAPalette[1])
	c.drawHighPaper(image.Rect(x1, y1, x2+1, y2+1), 1)
	DrawBubble(c, a, g, &game.Bubble{X1: x1, Y1: y1, X2: x2, Y2: y2, Left: sp.Left,
		Speaker: sp.Speaker, Color: sp.Color, Text: sp.Text})
}

// DrawBattleScene 把戰場上的場景圖那一格拉進第三塊面板 (448,268) 走到第
// step 步（`0x2a8ab` 先把面板填藍，場景圖 176×96 剛好蓋滿）。回傳總步數。
func DrawBattleScene(c *Canvas, a *ArtScreen, sp *battle.Speech, step int) int {
	return DrawScene(c, a, sp.Scene, WipeKind(sp.Style), assets.SceneBattleX, assets.SceneBattleY, step)
}

func clearPanelRects(x1, y1, x2, y2 int) [2]image.Rectangle {
	return [2]image.Rectangle{
		image.Rect(x1+16, y1+8, x2-16+1, y2-8+1),
		image.Rect(x1+8, y1+16, x2-8+1, y2-16+1),
	}
}

// ClearPanel 照原版的 `0x1058:0x27e8(x1, y1, x2, y2, 色)` 清一塊面板的
// **內部**：外框的拼件留著——角是 16×16、邊是 8 寬，所以清的是兩塊矩形
// 拼成的十字：(x1+16, y1+8)–(x2−16, y2−8) 與 (x1+8, y1+16)–(x2−8, y2−16)。
// 座標含端點。呼叫端在對白之前拿它把右側面板清成藍（`0x14899`／`0x1bb3c`）。
func ClearPanel(c *Canvas, x1, y1, x2, y2 int, ink color.RGBA) {
	rects := clearPanelRects(x1, y1, x2, y2)
	for _, r := range rects {
		c.FillRect(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, ink)
	}
	if ink == assets.EGAPalette[1] {
		for _, r := range rects {
			c.drawHighPaper(r, 1)
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
	c.drawHigh(im, x, y)
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
