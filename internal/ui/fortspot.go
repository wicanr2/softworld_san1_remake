package ui

import (
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
)

// FortSpot 是建築關寨挑位置那個畫面（`0x1acba`，`docs/spec/014` §4.4）：郡地理誌同一張場地圖
// （`0x2020:0x2244(郡, −1)` ＋ `0x18742(郡, 0)`），第三塊面板 (448, 268＋16i) 寫五行說明，
// 游標那一格由計時器回呼（槽 2，`0x1ac2c`）每 512 次輪詢把 `MAPCUR1` 用 XOR 疊上去、再疊一次拿掉。
type FortSpot struct {
	Col, Row int
	// Marked 為真表示 `MAPCUR1` 此刻疊在那一格上（閃爍的亮相）。
	Marked bool
	// Confirm 為真表示那一格可以蓋、正在問「確認(Y/N):」。
	Confirm bool
	Input   InputCursor
}

const (
	fortSpotHelpX, fortSpotHelpY = 448, 268
	fortSpotInputX               = 536 // `0x33d8:0x15a4(0x218, 0x14c)`
	fortSpotInputY               = 332
	fortSpotHelpBG               = 1
	fortSpotConfirmInk           = 12
)

// fortSpotHelpInk 是五行說明的字色（`0x1ad8c`–`0x1ae00`）。
var fortSpotHelpInk = [5]int{15, 14, 14, 13, 15}

// DrawArtFortSpot 畫挑位置的畫面。
func DrawArtFortSpot(c *Canvas, ab *ArtBattle, field []byte, fld *battle.Field, s FortSpot) {
	DrawArtAtlas(c, ab, field, fld)
	bg := assets.EGAPalette[fortSpotHelpBG]
	for i := 0; i < 5; i++ {
		y := fortSpotHelpY + i*CellH
		line := t([]string{"fort.help1", "fort.help2", "fort.help3", "fort.help4", "fort.help5"}[i])
		ink := fortSpotHelpInk[i]
		if i == 4 && s.Confirm {
			// `0x1aee8`：確認那一句蓋在第五行上，紅 12。
			line, ink = t("fort.confirm"), fortSpotConfirmInk
		}
		c.FillRect(fortSpotHelpX, y, fortSpotHelpX+cells.Width(line)*CellW, y+CellH, bg)
		c.DrawTextPx(fortSpotHelpX, y, line, assets.EGAPalette[ink])
	}
	if s.Marked && ab.mapCursor != nil {
		x, y := assets.FieldCell(s.Col, s.Row)
		im := ab.mapCursor
		for dy := 0; dy < im.H; dy++ {
			for dx := 0; dx < im.W; dx++ {
				p := im.Pix[dy*im.W+dx] & 15
				px, py := x+dx, y+dy
				if p == 0 || px >= c.Img.Rect.Dx() || py >= c.Img.Rect.Dy() {
					continue
				}
				c.Img.SetRGBA(px, py, assets.EGAPalette[(egaIndexOf(c.Img.RGBAAt(px, py))^p)&15])
			}
		}
	}
	drawInputCursor(c, ab.cursor, s.Input, fortSpotInputX, fortSpotInputY)
}
