package ui

import (
	"fmt"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"image"
	"image/color"
	"image/draw"
)

func validateHighMapCursor(im *assets.Image, high *image.RGBA) error {
	if im.W != 48 || im.H != 32 || high.Bounds() != image.Rect(0, 0, 192, 128) {
		return fmt.Errorf("位置標記尺寸不符")
	}
	for y := 0; y < im.H; y++ {
		for x := 0; x < im.W; x++ {
			p := im.At(x, y)
			if p != 0 && p != 15 {
				return fmt.Errorf("位置標記色號不符")
			}
			expected := color.RGBA{}
			if p == 15 {
				expected = color.RGBA{255, 255, 255, 255}
			}
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					if high.RGBAAt(x*4+sx, y*4+sy) != expected {
						return fmt.Errorf("位置標記遮罩不符")
					}
				}
			}
		}
	}
	return nil
}

// composeHigh 在指定原生矩形重播圖層；owned 記錄最後可見材料的來源。
func (c *Canvas) composeHigh(dst *image.RGBA, owned []bool) {
	b := dst.Bounds()
	area := image.Rectangle{Min: b.Min.Div(4), Max: b.Max.Div(4)}
	scaleRGBA4(dst, c.Img.SubImage(area).(*image.RGBA))
	if owned != nil {
		clear(owned)
	}
	for _, op := range c.highOps {
		clip := op.rect.Intersect(area)
		if clip.Empty() {
			continue
		}
		r := image.Rectangle{Min: clip.Min.Mul(4), Max: clip.Max.Mul(4)}
		source := op.source.Add(clip.Min.Sub(op.rect.Min).Mul(4))
		mode := draw.Src
		if op.over {
			mode = draw.Over
		}
		draw.Draw(dst, r, op.image, source, mode)
		if owned != nil {
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					if !op.over || op.image.RGBAAt(source.X+x-r.Min.X, source.Y+y-r.Min.Y).A != 0 {
						owned[(y-b.Min.Y)*b.Dx()+x-b.Min.X] = true
					}
				}
			}
		}
		for y := clip.Min.Y; y < clip.Max.Y; y++ {
			for x := clip.Min.X; x < clip.Max.X; x++ {
				if !op.covered[(y-op.rect.Min.Y)*op.rect.Dx()+x-op.rect.Min.X] {
					continue
				}
				col := c.Img.RGBAAt(x, y)
				for sy := 0; sy < 4; sy++ {
					for sx := 0; sx < 4; sx++ {
						xx, yy := x*4+sx, y*4+sy
						dst.SetRGBA(xx, yy, col)
						if owned != nil {
							owned[(yy-b.Min.Y)*b.Dx()+xx-b.Min.X] = false
						}
					}
				}
			}
		}
	}
}

func (c *Canvas) highMapCursorPrefix(im *assets.Image, x, y int) *image.RGBA {
	if c.HD == nil || c.HD.mapCursor == nil || c.HD.mapCursorKey != hdImageKey(im) {
		return nil
	}
	r := image.Rect(x, y, x+im.W, y+im.H).Intersect(c.Img.Bounds())
	if r.Empty() {
		return nil
	}
	if c.mapCursorNative == nil {
		c.mapCursorNative = image.NewRGBA(image.Rect(0, 0, 192, 128))
		c.mapCursorOwned = make([]bool, 192*128)
	}
	c.mapCursorNative.Rect = image.Rectangle{Min: r.Min.Mul(4), Max: r.Max.Mul(4)}
	c.composeHigh(c.mapCursorNative, c.mapCursorOwned[:r.Dx()*r.Dy()*16])
	return c.mapCursorNative
}

// finishHighMapCursor 在 CPU XOR 完成後保留原 EGA 前景與高清材料補色。
func (c *Canvas) finishHighMapCursor(im *assets.Image, x, y int, high *image.RGBA) {
	if high == nil {
		return
	}
	b := high.Bounds()
	for yy := b.Min.Y; yy < b.Max.Y; yy++ {
		for xx := b.Min.X; xx < b.Max.X; xx++ {
			if im.At(xx/4-x, yy/4-y) == 0 {
				high.SetRGBA(xx, yy, color.RGBA{})
				continue
			}
			if c.mapCursorOwned[(yy-b.Min.Y)*b.Dx()+xx-b.Min.X] {
				q := high.RGBAAt(xx, yy)
				q.R ^= 255
				q.G ^= 255
				q.B ^= 255
				high.SetRGBA(xx, yy, q)
			} else {
				high.SetRGBA(xx, yy, c.Img.RGBAAt(xx/4, yy/4))
			}
		}
	}
	c.addHigh(high, image.Rectangle{Min: b.Min.Div(4), Max: b.Max.Div(4)}, b.Min)
	c.highOps[len(c.highOps)-1].over = true
}
