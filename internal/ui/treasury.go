package ui

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TreasuryPanel 是君主物品表（`0x14de6`，`1479:0656`，`docs/spec/014` §4.4）：右側面板
// (408,36)–(631,291) 清成色 5、外框樣式 2 → `SIDEC`；(432,52) 洋紅 13「%s主公 您的物品列表」；
// (416, 68＋16i) 黃 14 五列——第 0 列有玉璽才寫「   玉  璽   」、沒有寫十二個空白，
// 第 1–4 列「%2d)%s%2d」（編號 2–5、名字、件數）。查看→物品與賞賜物品共用。
type TreasuryPanel struct {
	Faction state.FactionID
}

const (
	treasuryBG             = 5
	treasuryHeadX          = 432
	treasuryHeadY          = 52
	treasuryRowX           = 416
	treasuryRowY           = 68
	treasuryHeadInk        = 13
	treasuryRowInk         = 14
	treasuryEmptySealWidth = 12 // `DS:0x65b5` 十二個空白
)

// DrawTreasuryPanel 畫君主物品表。諸侯記錄第一個字是 0xFFFF（滅亡）時原版不畫（`0x14e04`）。
func DrawTreasuryPanel(c *Canvas, a *ArtScreen, g *game.State, p *TreasuryPanel) {
	f := g.Faction(p.Faction)
	lord := g.Lord(p.Faction)
	if f == nil || lord == nil {
		return
	}
	c.FillRect(rosterX0, rosterY0, rosterX1+1, rosterY1+1, assets.EGAPalette[treasuryBG])
	if a != nil && a.havePanel {
		drawSideFrame(c, a.cardPanel, rosterX0, rosterY0, rosterX1-rosterX0+1, rosterY1-rosterY0+1)
	}
	c.DrawTextPx(treasuryHeadX, treasuryHeadY, tf("tre.head", NameField(PersonName(lord.Name))), assets.EGAPalette[treasuryHeadInk])
	keys := []string{"tre.row.seal", "tre.row.book", "tre.row.blade", "tre.row.beauty", "tre.row.horse"}
	ink := assets.EGAPalette[treasuryRowInk]
	if f.Treasury[game.TreasureSeal] > 0 {
		c.DrawTextPx(treasuryRowX, treasuryRowY, tf("tre.sealRow", t(keys[0])), ink)
	}
	for i := 1; i < len(keys); i++ {
		c.DrawTextPx(treasuryRowX, treasuryRowY+i*CellH, fmt.Sprintf("%2d)%s%2d", i+1, t(keys[i]), f.Treasury[i]), ink)
	}
}
