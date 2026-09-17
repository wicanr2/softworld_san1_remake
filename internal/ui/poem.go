package ui

import (
	"image"
	"image/draw"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/opening"
)

// PoemColumns 由右到左回傳開場詞的每一欄（內容與出處見 `internal/opening`）。
func PoemColumns() []string { return opening.PoemColumns() }

// DrawPoem 畫寫完詞的那一張：底圖用原版的，字用 remake 的字庫。
//
// 疊法照原版片頭（`docs/spec/005`「片頭」）：遮罩在 +2,+2 與原位各 AND 一次
// 做出黑影，字再 OR 上去；每個字置中在原版的字框裡。
func DrawPoem(c *Canvas, im *assets.Image) {
	p := opening.NewPages()
	p.Put(im, 0, 0, opening.Copy)
	ink, mask := opening.FontPoem(c.face)
	p.Put(mask, opening.PoemX+2, opening.PoemY+2, opening.And)
	p.Put(mask, opening.PoemX, opening.PoemY, opening.And)
	p.Put(ink, opening.PoemX, opening.PoemY, opening.Or)
	DrawImage(c, p.Visible())
}

// DrawImage 把一整張 640×408 的圖貼滿畫布。開場的三英圖
// （`assets.TitleArt`）就只是一張圖，沒有疊字。
func DrawImage(c *Canvas, im *assets.Image) {
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		im.RGBA(), image.Point{}, draw.Src)
}

// DrawPages 畫片頭兩頁裡顯示中的那一頁，顏色照當時的屬性暫存器
// （淡出那一段會改）。
func DrawPages(c *Canvas, p *opening.Pages) {
	if p == nil {
		return
	}
	pal := p.Palette()
	vis := p.Visible()
	for y := 0; y < vis.H && y < c.Img.Bounds().Dy(); y++ {
		for x := 0; x < vis.W && x < c.Img.Bounds().Dx(); x++ {
			c.Img.SetRGBA(x, y, pal[vis.Pix[y*vis.W+x]&15])
		}
	}
}
