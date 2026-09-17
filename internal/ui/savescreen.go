package ui

import (
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
)

// SaveScreen 是「其他 → 儲存進度」那一格（`0x1e440`–`0x1e668`，`docs/spec/005` §9.9，
// `L0`、`[base]`、Issue #74）：右側面板清成藍、六筆進度名稱從 (424, 52) 起每列
// 16 像素（字 14、底 1），選好的那一筆換成新名稱畫白字，備註打在同一列第 14 格起。
type SaveScreen struct {
	// Names 是六筆名稱原樣（`SAVENAME.SVP` 的形狀，開頭自帶「n.」）。
	Names [6]string
	// Slot 是選好的槽（1–6）；0 表示還在問「(1-6)」。
	Slot int
}

// 存檔畫面的版面（`0x1e529`／`0x1e631`）。
const (
	saveRowX, saveRowY = 424, 52
	saveRowCells       = 20
	saveInk, savePick  = 14, 15
	saveBG             = 1
)

// DrawSaveScreen 畫存檔那一格的右側面板：外框裡清成藍（`0x1058:0x27e8(…, 1)`），
// 外框重拼成樣式 2（`0x262c`，`(2＋500) mod 5` → `SIDEC`，與人物卡同一組）。
func DrawSaveScreen(c *Canvas, a *ArtScreen, s *SaveScreen) {
	ClearPanel(c, 408, 36, 631, 291, assets.EGAPalette[saveBG])
	if a != nil && a.havePanel {
		drawSideFrame(c, a.cardPanel, 408, 36, 224, 256)
	}
	for k, name := range s.Names {
		ink := saveInk
		if k == s.Slot-1 {
			ink = savePick
		}
		c.DrawTextPx(saveRowX, saveRowY+k*CellH, cells.Truncate(cells.Pad(name, saveRowCells), saveRowCells), assets.EGAPalette[ink])
	}
}
