package ui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
)

// 選君主（原版 `0x124ca(人數)`，`docs/spec/005` §9.4，`L0`、`[base]`）：
// 主畫面的地圖與年月照常，右側整塊換成一頁六位候選君主，三欄兩列。
//
//	外框      `SIDEB` 拼在 (408,36)–(631,323)，裡面留著底圖的雜訊（不清）
//	提示框    `SIDEA` 拼在 (408,324)–(631,371)，內部青 3；提示字淺綠 10 在 (424,332)
//	第 i 格   x ＝ 420 ＋ 68×(i mod 3)、y ＝ 56（i < 3）或 184
//	肖像      (x, y) 64×80，外圈一像素的空心框 (x−1,y−1)–(x+64,y+80) 色 9＋i
//	色塊      勢力的地圖填色圖樣，(x+2,y+86)–(x+13,y+98)，黑邊 (x+1,y+85)–(x+14,y+99)
//	編號      `%2d`（勢力槽號＋1）8×16 白字黑邊在 (x−2, y+100)
//	名字      6 位元組的姓名欄 16×32 白字黑邊在 (x+16, y+84)
//
// 原版靠數字鍵選、沒有反白；remake 用方向鍵時把反白那一格的框畫成白色
// （remake 差異）。
const (
	lordPickX0, lordPickY0 = 420, 56
	lordPickDX, lordPickDY = 68, 128
	LordPickPerPage        = 6
	lordPickPromptX        = 424
	lordPickPromptY        = 332
)

// LordPickSlot 是選君主那一格的一位候選。Lord 為 nil 是空的新君主欄：
// 肖像用 Portrait（原版的範本人物也有肖像，`state.CustomLordPortrait`）、
// 名字用 Label。
type LordPickSlot struct {
	Number   int // 1 起算（原版印的是勢力槽號＋1）
	Faction  int
	Lord     *game.General
	Portrait int
	Label    string
	// Player 不是 0 時這一位已經被第 Player 位玩家選走：肖像下緣印 `%2d`，
	// 兩倍寬、灰 7 黑邊，在 (格 x＋16, 格 y＋73)（`0x12291`，`docs/spec/005` §9.4）。
	Player int
}

// DrawLordPick 畫選君主那一格。slots 是這一頁的候選（最多六位）；sel 是
// 反白哪一格（−1 ＝ 沒有）；prompt 是下方提示框的字。prompt 是空字串時提示框
// 還沒被訊息常式清過（「請問有幾人玩」那一問，`0x12072`），內部留著底圖。
func DrawLordPick(c *Canvas, a *ArtScreen, g *game.State, slots []LordPickSlot, sel int, prompt string, cal game.Calendar) {
	im := a.Compose(g, 0)
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH), im.RGBA(), image.Point{}, draw.Src)
	drawArtDate(c, g.Date.FormatWithSeason(cal))

	ink := func(n int) color.RGBA { return assets.EGAPalette[n&15] }
	if a.havePanel {
		drawSideFrame(c, a.panels[0], assets.MainPanelX, 36, assets.MainPanelW, 288)
		if prompt != "" {
			c.FillRect(assets.MainPanelX+8, 324+8, assets.MainPanelX+assets.MainPanelW-8, 372-8, ink(3))
		}
		drawSideFrame(c, a.pickBox, assets.MainPanelX, 324, assets.MainPanelW, 48)
	}
	c.DrawTextPx(lordPickPromptX, lordPickPromptY, cells.Truncate(prompt, 25), ink(10))

	white, black := ink(15), ink(0)
	for i, s := range slots {
		if i >= LordPickPerPage {
			break
		}
		x := lordPickX0 + lordPickDX*(i%3)
		y := lordPickY0 + lordPickDY*(i/3)
		portrait := s.Portrait
		if s.Lord != nil {
			portrait = int(s.Lord.Portrait)
		}
		if face := a.Portrait(portrait); face != nil {
			drawImageAt(c, face, x, y)
		}
		box := ink(9 + i)
		if i == sel {
			box = white
		}
		c.StrokeRect(x-1, y-1, x+64, y+80, box)
		// 勢力色塊：與地圖同一組填色圖樣，網點對齊畫面。
		c.StrokeRect(x+1, y+85, x+14, y+99, black)
		for py := y + 86; py <= y+98; py++ {
			for px := x + 2; px <= x+13; px++ {
				col := artFactionColour[s.Faction%len(artFactionColour)]
				if a.fills != nil {
					col = a.fills[s.Faction%len(a.fills)].At(px, py)
				}
				c.setClipped(px, py, ink(int(col)))
			}
		}
		if s.Player > 0 {
			px := x + 16
			for _, r := range fmt.Sprintf("%2d", s.Player) {
				px += c.DrawRuneOutlinedPx(px, y+73, r, ink(7), black, 2, 1)
			}
		}
		nx := x - 2
		for _, r := range fmt.Sprintf("%2d", s.Number) {
			nx += c.DrawRuneOutlinedPx(nx, y+100, r, white, black, 1, 1)
		}
		// 名字：中文照原版 16×32 畫 6 位元組的姓名欄。譯名（拼音）那一格
		// 只有 51 像素寬（x+16 到下一格之前），改用小字級在空白處折成兩行
		// （`docs/spec/014` 的譯文槽位規則，記為 remake 差異）。
		label := s.Label
		if s.Lord != nil {
			label = PersonName(s.Lord.Name)
		}
		switch {
		case artAllWide(label):
			tx := x + 16
			for _, r := range paddedName(label) {
				tx += c.DrawRuneOutlinedPx(tx, y+84, r, white, black, 1, 2)
			}
		case c.FitsSmall(label):
			for i, line := range cells.Wrap(label, lordPickSmallCols) {
				if i >= 2 {
					break
				}
				line = cells.Truncate(line, lordPickSmallCols)
				ty := y + 88 + i*(SmallH+2)
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if dx != 0 || dy != 0 {
							c.DrawSmallTextPx(x+16+dx, ty+dy, line, black)
						}
					}
				}
				c.DrawSmallTextPx(x+16, ty, line, white)
			}
		default:
			tx := x + 16
			for _, r := range cells.Truncate(label, 6) {
				tx += c.DrawRuneOutlinedPx(tx, y+84+CellH/2, r, white, black, 1, 1)
			}
		}
	}
}

// lordPickSmallCols 是譯名一行放幾個小字級的字母（51 像素 ÷ 小字寬）。
const lordPickSmallCols = 51 / SmallW

// 新君主分配能力（原版 `0x12df4(諸侯)`，`docs/spec/005` §9.5，`L0`、`[base]`）：
// 與選君主同一塊面板（`SIDEB` 拼在 (408,36)–(631,323)，裡面留底圖；下方的
// 提示框由輸入常式寫「剩餘點數:%d」與「更改(1-7,0-結束):」兩行），
// 肖像 (536,64) 加 `FBRD` 框，名字 32×32 黑邊在 (524,175)、
// 色 9＋(諸侯 mod 7)；六行字黑邊從 (432,56) 起、行距 24（最後一行 180），
// 字色 10／13／14／14／14／12。原版第七項「7.」（改姓名）在 (536,155)——
// remake 沒有改名，不畫。
const (
	customFaceX, customFaceY = 536, 64
	customNameX, customNameY = 524, 175
	customTextX              = 432
	customTextCols           = (customFaceX - 8 - customTextX) / CellW
)

var customLineY = [6]int{56, 80, 104, 128, 152, 180}
var customLineInk = [6]int{10, 13, 14, 14, 14, 12}

// DrawCustomLord 畫新君主那一格。lines 是六行字；sel 是反白哪一行
// （原版靠數字鍵、沒有反白；remake 用方向鍵時把那一行畫成白色，remake 差異）。
func DrawCustomLord(c *Canvas, a *ArtScreen, g *game.State, faction, portrait int, name string, lines [6]string, sel int, prompt [2]string, cal game.Calendar) {
	im := a.Compose(g, 0)
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH), im.RGBA(), image.Point{}, draw.Src)
	drawArtDate(c, g.Date.FormatWithSeason(cal))
	ink := func(n int) color.RGBA { return assets.EGAPalette[n&15] }
	if a.havePanel {
		drawSideFrame(c, a.panels[0], assets.MainPanelX, 36, assets.MainPanelW, 288)
		c.FillRect(assets.MainPanelX+8, 324+8, assets.MainPanelX+assets.MainPanelW-8, 372-8, ink(3))
		drawSideFrame(c, a.pickBox, assets.MainPanelX, 324, assets.MainPanelW, 48)
		fx, fy := customFaceX-8, customFaceY-8
		drawImageAt(c, a.frame[0], fx, fy)
		drawImageAt(c, a.frame[1], fx, customFaceY+80)
		drawImageAt(c, a.frame[2], fx, customFaceY)
		drawImageAt(c, a.frame[3], fx+72, customFaceY)
	}
	if face := a.Portrait(portrait); face != nil {
		drawImageAt(c, face, customFaceX, customFaceY)
	}
	// 提示框兩行（原版輸入常式的「剩餘點數:%d」與「更改(1-7,0-結束):」在
	// 332 與 348）。
	for i, line := range prompt {
		c.DrawTextPx(lordPickPromptX, lordPickPromptY+i*CellH, cells.Truncate(line, 25), ink(10))
	}
	black := ink(0)
	nameInk := ink(9 + faction%7)
	x, sy := customNameX, 2
	if !artAllWide(name) {
		sy = 1
	}
	for _, r := range cells.Truncate(name, (624-customNameX)/CellW/sy) {
		x += c.DrawRuneOutlinedPx(x, customNameY+(2-sy)*CellH/2, r, nameInk, black, sy, sy)
	}
	for i, line := range lines {
		col := ink(customLineInk[i])
		if i == sel {
			col = ink(15)
		}
		x := customTextX
		for _, r := range cells.Truncate(line, customTextCols) {
			x += c.DrawRuneOutlinedPx(x, customLineY[i], r, col, black, 1, 1)
		}
	}
}

// 「新君主出現!!」那一格（`0x133f2`，`L0`）：分完能力之後提示框印
// `DS:0x6278`「新君主出現!!」，把右側面板 (424,52)–(623,211) 還原成底圖，
// 再以訊息常式在上格 (424,66)–(615,161) 畫新君主的肖像（右）與一句
// 498「天地英雄氣 千秋尚凜然」，等鍵。
const (
	newLordBubbleY1 = 66
	newLordBubbleY2 = 161
)

// DrawNewLordBorn 畫「新君主出現!!」那一格：面板框、提示、新君主的訊息框。
func DrawNewLordBorn(c *Canvas, a *ArtScreen, g *game.State, portrait int, name string, colour int, cal game.Calendar) {
	im := a.Compose(g, 0)
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH), im.RGBA(), image.Point{}, draw.Src)
	drawArtDate(c, g.Date.FormatWithSeason(cal))
	ink := func(n int) color.RGBA { return assets.EGAPalette[n&15] }
	if a.havePanel {
		drawSideFrame(c, a.panels[0], assets.MainPanelX, 36, assets.MainPanelW, 288)
		c.FillRect(assets.MainPanelX+8, 324+8, assets.MainPanelX+assets.MainPanelW-8, 372-8, ink(3))
		drawSideFrame(c, a.pickBox, assets.MainPanelX, 324, assets.MainPanelW, 48)
	}
	c.DrawTextPx(lordPickPromptX, lordPickPromptY, cells.Truncate(t("title.newLordBorn"), 25), ink(10))
	DrawBubbleAs(c, a, &game.Bubble{X1: game.BubbleX1, Y1: newLordBubbleY1, X2: game.BubbleX2, Y2: newLordBubbleY2,
		Color: colour, Text: t("bub.newLord")}, name, portrait)
}

// DrawLordPickCursor 在選君主那一格提示（(424,332)，`DrawLordPick` 的 prompt）
// 後面畫輸入游標（`CURD`）。
func DrawLordPickCursor(c *Canvas, a *ArtScreen, prompt string, in InputCursor) {
	if prompt == "" {
		return
	}
	drawInputCursor(c, a.setupCursor, in, lordPickPromptX+cells.Width(cells.Truncate(prompt, 25))*CellW, lordPickPromptY)
}

// DrawLordPickAskCursor 在「請問有幾人玩」那一行（y 340）後面畫輸入游標。
func DrawLordPickAskCursor(c *Canvas, a *ArtScreen, prompt string, in InputCursor) {
	drawInputCursor(c, a.setupCursor, in, lordPickPromptX+cells.Width(cells.Truncate(prompt, 25))*CellW, 340)
}

// DrawLordPickAsk 寫「請問有幾人玩(0-%d):」那一行：原版不走訊息列，直接在
// (424, 340) 以洋紅 13 寫（`0x120da`，`docs/spec/019` §1）。
func DrawLordPickAsk(c *Canvas, prompt string) {
	c.DrawTextPx(lordPickPromptX, 340, cells.Truncate(prompt, 25), assets.EGAPalette[13])
}
