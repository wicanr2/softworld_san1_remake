package ui

import (
	"image"
	"image/color"
	"image/draw"
)

// vectorTextBackground 保存畫字前的同一字格，避免 CPU 點陣在 HD 留下舊筆畫。
// 背景及向量字一起加入目前操作順序，後畫的前景仍具有覆蓋權。
func (c *Canvas) vectorTextBackground(r image.Rectangle) *image.RGBA {
	r = r.Intersect(c.Img.Bounds())
	if c.HD == nil || r.Empty() {
		return nil
	}
	bg := image.NewRGBA(image.Rectangle{Min: r.Min.Mul(4), Max: r.Max.Mul(4)})
	c.composeHigh(bg, nil)
	return bg
}

func (c *Canvas) finishVectorText(bg *image.RGBA, mask *image.Alpha, origin image.Point, fg color.RGBA) {
	if bg == nil || mask == nil {
		return
	}
	r := mask.Bounds().Add(origin.Mul(4))
	draw.DrawMask(bg, r, image.NewUniform(fg), image.Point{}, mask, image.Point{}, draw.Over)
	b := bg.Bounds()
	c.addHigh(bg, image.Rectangle{Min: b.Min.Div(4), Max: b.Max.Div(4)}, b.Min)
}

func (c *Canvas) vectorRuneBackground(r rune, box image.Rectangle) (*image.RGBA, *image.Alpha) {
	if c.HD == nil {
		return nil, nil
	}
	mask, ok := c.face.VectorGlyph(r, box.Dx()*4, box.Dy()*4)
	if !ok {
		return nil, nil
	}
	return c.vectorTextBackground(box), mask
}

func (c *Canvas) vectorRotatedBackground(r rune, origin image.Point, w, h int) (*image.RGBA, *image.Alpha) {
	if c.HD == nil {
		return nil, nil
	}
	mask, ok := c.face.VectorGlyph(r, w*4, h*4)
	if !ok {
		return nil, nil
	}
	rotated := image.NewAlpha(image.Rect(0, 0, h*4, w*4))
	for y := 0; y < h*4; y++ {
		for x := 0; x < w*4; x++ {
			rotated.SetAlpha(h*4-1-y, x, mask.AlphaAt(x, y))
		}
	}
	return c.vectorTextBackground(image.Rectangle{Min: origin, Max: origin.Add(image.Pt(h, w))}), rotated
}

func (c *Canvas) finishVectorDither(bg *image.RGBA, mask *image.Alpha, origin image.Point, a, b color.RGBA) {
	if bg == nil || mask == nil {
		return
	}
	colors := image.NewRGBA(bg.Bounds())
	for y := colors.Rect.Min.Y; y < colors.Rect.Max.Y; y++ {
		for x := colors.Rect.Min.X; x < colors.Rect.Max.X; x++ {
			fg := b
			if (x/4+y/4)%2 == 1 {
				fg = a
			}
			colors.SetRGBA(x, y, fg)
		}
	}
	r := mask.Bounds().Add(origin.Mul(4))
	draw.DrawMask(bg, r, colors, r.Min, mask, image.Point{}, draw.Over)
	bounds := bg.Bounds()
	c.addHigh(bg, image.Rectangle{Min: bounds.Min.Div(4), Max: bounds.Max.Div(4)}, bounds.Min)
}
