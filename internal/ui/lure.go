package ui

import (
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
)

// 誘敵的特效（`0x2b783`–`0x2b7f4`，`docs/spec/005` §8「誘敵的特效」）。
//
// 對白之後，原版在施法者那一格（地形圖塊的位置）直接貼 `EICON.GRP` 的
// 第 32、33 張，再 34／35 交替二十次；每貼一張叫一次 `speak(0, 速度)`。
// 之後重畫那一格的地形與旗（`0x23a6:0xc2e`、`0x2020:0x1940`），接交戰結算。

// LureStep 是一步：貼第幾張圖塊、之後那一聲的速度。
type LureStep struct{ Tile, Speed int }

// LureFlashSteps 回二十二步。
func LureFlashSteps() []LureStep {
	out := []LureStep{{0x20, 0x1b8}, {0x21, 0x1b8}}
	for i := 0; i < 20; i++ {
		out = append(out, LureStep{0x22 + i%2, 300 - 48*(i%2)})
	}
	return out
}

// DrawLureFlash 在 at 那一格貼第 tile 張圖塊（整塊覆蓋，`0x36a8:0xc0`）。
func DrawLureFlash(c *Canvas, ab *ArtBattle, at battle.Hex, tile int) {
	if ab == nil || tile < 0 || tile >= len(ab.tiles) {
		return
	}
	col, row := battle.ToOffset(at)
	x, y := assets.FieldCell(col, row)
	im := ab.tiles[tile]
	for yy := 0; yy < im.H; yy++ {
		for xx := 0; xx < im.W; xx++ {
			px, py := x+xx, y+yy
			if px < 0 || py < 0 || px >= c.Img.Bounds().Dx() || py >= c.Img.Bounds().Dy() {
				continue
			}
			c.Img.SetRGBA(px, py, assets.EGAPalette[im.Pix[yy*im.W+xx]&15])
		}
	}
}

// LureFlashCell 是那一格在畫面上的左上角與圖塊的寬高（對拍用）。
func LureFlashCell(at battle.Hex) (x, y, w, h int) {
	col, row := battle.ToOffset(at)
	x, y = assets.FieldCell(col, row)
	return x, y, assets.TileW, assets.TileH
}
