package ui

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// PrefPick 是「那一郡」的挑郡清單（`0x1d4ec`，`docs/spec/014` §4.4）：右側面板清藍、
// 外框樣式 8 → `SIDED`，(424,44) 表頭，42 郡分三欄每欄 14 列，收得下的黃 14、其餘棕 6。
type PrefPick struct {
	// Valid[郡] 為真表示這一問收那一郡（原版 `es:0x2102[郡]` ＝ 0）。
	Valid [state.PrefectureCount + 1]bool
}

const (
	prefPickHeadX, prefPickHeadY = 424, 44
	prefPickX0, prefPickDX       = 432, 64 // 第 k 欄 x ＝ 432 ＋ 64k（`0x1d5a9`）
	prefPickY0                   = 60      // 第 j 列 y ＝ 60 ＋ 16j（`0x1d5bb`）
	prefPickRows                 = 14
	prefPickValidInk             = 14
	prefPickOtherInk             = 6
)

// DrawPrefPick 畫挑郡清單。
func DrawPrefPick(c *Canvas, a *ArtScreen, g *game.State, p *PrefPick) {
	c.FillRect(rosterX0, rosterY0, rosterX1+1, rosterY1+1, assets.EGAPalette[rosterBG])
	if a != nil && a.havePanel {
		drawSideFrame(c, a.prefBox, rosterX0, rosterY0, rosterX1-rosterX0+1, rosterY1-rosterY0+1)
	}
	c.DrawTextPx(prefPickHeadX, prefPickHeadY, t("pick.prefHead"), assets.EGAPalette[rosterHeadInk])
	for id := 1; id <= 42; id++ {
		q := g.Prefecture(id)
		if q == nil {
			continue
		}
		ink := prefPickOtherInk
		if id < len(p.Valid) && p.Valid[id] {
			ink = prefPickValidInk
		}
		x := prefPickX0 + (id-1)/prefPickRows*prefPickDX
		y := prefPickY0 + (id-1)%prefPickRows*CellH
		c.DrawTextPx(x, y, fmt.Sprintf("%2d", id)+cells.Truncate(PlaceName(q.Name), 6), assets.EGAPalette[ink])
	}
}
