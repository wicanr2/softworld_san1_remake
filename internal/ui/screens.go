package ui

import (
	"fmt"
	"image/color"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 畫面尺寸（格）。原版是 640×400 的版面：80 格 × 25 列 × 8×16 像素。
const (
	Cols = 80
	Rows = 25
)

// 調色。原版的實際色盤還沒解（`docs/formats/` 裡的 `.PAL` 未解），
// 這些是 remake 自己的暫時值。
var (
	ColBG    = color.RGBA{0x10, 0x10, 0x18, 0xFF}
	ColFG    = color.RGBA{0xE0, 0xE0, 0xD0, 0xFF}
	ColDim   = color.RGBA{0x80, 0x80, 0x90, 0xFF}
	ColFrame = color.RGBA{0x50, 0x60, 0x80, 0xFF}
	ColWarn  = color.RGBA{0xFF, 0x60, 0x60, 0xFF}
)

// DrawPrefectureList 畫州郡一覽。
//
// **這不是原版的版面**——原版的畫面還沒解（`docs/re/`）。這是 remake
// 自己的暫時畫面，用途是證明資料走完整條管線，日後要換掉。
//
// 放在這裡而不是放在 `cmd/san1` 裡，是為了讓**無頭環境也畫得出同一張**：
// Ebiten 那一層貼的是這張，PNG 輸出存的也是這張。畫面 bug 測試看不到，
// 但存成圖就看得到。
func DrawPrefectureList(c *Canvas, sc *state.Scenario, slot string) {
	c.Fill(ColBG)
	c.DrawText(2, 0, fmt.Sprintf("三國演義  劇本 %s  州郡一覽", slot), ColFG)
	c.DrawText(2, 1, "（remake 的暫時畫面，不是原版版面）", ColDim)
	c.DrawBox(1, 2, c.Cols-2, c.Rows-3, ColFrame)

	const slotW = 9 // 「NN 郡名」＝ 2 ＋ 1 ＋ 4，補到 9 格留間距
	perRow := (c.Cols - 4) / slotW
	for i, p := range sc.Prefectures() {
		col := 2 + (i%perRow)*slotW
		row := 4 + i/perRow
		if row >= c.Rows-2 {
			break
		}
		c.DrawText(col, row, fmt.Sprintf("%2d", p.ID), ColDim)
		c.DrawText(col+3, row, cells.Pad(p.Name, slotW-3), ColFG)
	}

	c.DrawText(2, c.Rows-2, fmt.Sprintf("州郡 %d   人物 %d",
		len(sc.Prefectures()), len(sc.People())), ColDim)

	// 缺字要說出來。畫面上的空白看起來像排版問題，不像缺字。
	if n := len(c.Missing); n > 0 {
		c.DrawText(40, c.Rows-2, fmt.Sprintf("⚠ %d 個字沒有字模", n), ColWarn)
	}
}
