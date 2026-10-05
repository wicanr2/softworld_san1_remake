package ui

import (
	"image"
	"image/draw"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// drawHighFrame 照原版八片的順序，保留 MAINMAPC 覆蓋下框的末列。
func (a *ArtScreen) drawHighFrame(c *Canvas) {
	if c.HD == nil {
		return
	}
	for _, p := range a.layers {
		c.trackRect(image.Rect(p.X, p.Y, p.X+p.Image.W, p.Y+p.Image.H))
		switch p.Name {
		case "MAINMAP1.IMG", "MAINMAP2.IMG", "MAINMAP3.IMG", "MAINMAP7.IMG":
			c.drawHigh(p.Image, p.X, p.Y)
		}
	}
}

// tileBattleBackground 每包只建立一次，不為每幀記錄數千片底紋。
func tileBattleBackground(tile *image.RGBA) *image.RGBA {
	bg := image.NewRGBA(image.Rect(0, 0, assets.ScreenW*4, assets.ScreenH*4))
	for y := 0; y < bg.Bounds().Dy(); y += tile.Bounds().Dy() {
		for x := 0; x < bg.Bounds().Dx(); x += tile.Bounds().Dx() {
			r := image.Rect(x, y, x+tile.Bounds().Dx(), y+tile.Bounds().Dy())
			draw.Draw(bg, r, tile, tile.Bounds().Min, draw.Src)
		}
	}
	return bg
}

// drawHighBackdrop 在高清地形之前補底圖與花邊；原版階梯仍先於地形。
func (ab *ArtBattle) drawHighBackdrop(c *Canvas, l assets.BattleLayout) {
	if c.HD == nil {
		return
	}
	if bg := c.HD.battleBackground; bg != nil && c.HighImage(ab.bgTile) != nil {
		c.addHigh(bg, image.Rect(0, 0, assets.ScreenW, assets.ScreenH), image.Point{})
	}
	for _, p := range []struct {
		im *assets.Image
		y  int
	}{{ab.top, 0}, {ab.bottom, assets.MapBorderBottomY}} {
		if p.im != nil {
			c.trackRect(image.Rect(0, p.y, p.im.W, p.y+p.im.H))
			c.drawHigh(p.im, 0, p.y)
		}
	}
	mask := c.indexedCoverage()
	mask.FieldEdges(l)
	c.coverIndexed(mask)
}
