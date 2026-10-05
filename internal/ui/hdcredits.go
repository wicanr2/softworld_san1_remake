package ui

import (
	"fmt"
	"image"
	"image/draw"
	"strconv"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

var hdCreditWidths = [22]int{272, 272, 272, 272, 112, 64, 584, 584, 64, 64, 584, 584, 64, 64, 568, 568, 64, 480, 240, 448, 160, 576}

func validateCreditText(im *assets.Image, high *image.RGBA, name string) error {
	regions := hdCreditFigures(name, im.W)
	for y := 0; y < im.H; y++ {
		for x := 0; x < im.W; x++ {
			art := false
			for _, r := range regions {
				art = art || image.Pt(x, y).In(r)
			}
			if art {
				continue
			}
			want := assets.EGAPalette[im.At(x, y)]
			for ky := 0; ky < 4; ky++ {
				for kx := 0; kx < 4; kx++ {
					if high.RGBAAt(x*4+kx, y*4+ky) != want {
						return fmt.Errorf("製作群文字或保護區不符")
					}
				}
			}
		}
	}
	return nil
}

// hdCreditFigures 只涵蓋能與文字分離的美術；中央的中間兩條不列入。
func hdCreditFigures(name string, width int) []image.Rectangle {
	if len(name) != 9 {
		return nil
	}
	n, err := strconv.Atoi(name[3:5])
	if err != nil || n < 0 || n >= len(hdCreditWidths) || width != hdCreditWidths[n] {
		return nil
	}
	origin := image.Pt((assets.ScreenW-width)/2, n*assets.CreditLineH)
	regions := []image.Rectangle{
		image.Rect(288, 120, 352, 144), image.Rect(288, 192, 352, 216),
		image.Rect(288, 216, 352, 240), image.Rect(288, 288, 352, 312),
		image.Rect(288, 312, 352, 336), image.Rect(288, 384, 352, 408),
		image.Rect(548, 144, 612, 192), image.Rect(548, 240, 612, 288), image.Rect(540, 336, 604, 384),
	}
	var out []image.Rectangle
	for _, r := range regions {
		if r = r.Sub(origin).Intersect(image.Rect(0, 0, width, assets.CreditLineH)); !r.Empty() {
			out = append(out, r)
		}
	}
	return out
}

// drawHDCredits 保留原 CPU 頁，僅在相同畫面合成高清背景與字幕。
// 人物框內的原色號0可成為高清五官；框外色號0仍透明。天空裁切共用原判斷。
func (c *Canvas) drawHDCredits(cr *assets.Credits, scroll int, hall bool) {
	if c.HD == nil || cr == nil {
		return
	}
	parts := cr.BackdropParts[0][:]
	if hall {
		parts = cr.HallParts[:]
	}
	highs := make([]*image.RGBA, len(parts))
	available := false
	for i, part := range parts {
		highs[i] = c.HighImage(part)
		available = available || highs[i] != nil
	}
	lines := make([]*image.RGBA, len(cr.Lines))
	if !hall {
		for i, ln := range cr.Lines {
			lines[i] = c.HighImage(ln)
			available = available || lines[i] != nil
		}
	}
	if !available {
		return
	}
	b := c.Img.Bounds()
	if c.creditNative == nil || c.creditNative.Bounds().Dx() != b.Dx()*4 || c.creditNative.Bounds().Dy() != b.Dy()*4 {
		c.creditNative = image.NewRGBA(image.Rect(0, 0, b.Dx()*4, b.Dy()*4))
	}
	out := c.creditNative
	scaleRGBA4(out, c.Img)
	x := 0
	for i, part := range parts {
		if part == nil {
			continue
		}
		if high := highs[i]; high != nil {
			draw.Draw(out, image.Rect(x*4, CreditsTop*4, (x+part.W)*4, (CreditsTop+part.H)*4), high, image.Point{}, draw.Src)
		}
		x += part.W
	}
	if !hall {
		for i, ln := range cr.Lines {
			if ln == nil {
				continue
			}
			high := lines[i]
			art := c.HD.creditFigures[hdImageKey(ln)]
			x, y := (assets.ScreenW-ln.W)/2, assets.ScreenH-scroll+i*CreditLineStep
			for sy := max(0, -y); sy < min(ln.H, b.Dy()-y); sy++ {
				for sx := max(0, -x); sx < min(ln.W, b.Dx()-x); sx++ {
					v := ln.At(sx, sy)
					owned := v != 0
					if !owned && high != nil {
						for _, r := range art {
							if image.Pt(sx, sy).In(r) {
								owned = true
								break
							}
						}
					}
					dx, dy := x+sx, y+sy
					if !owned || !cr.SkyAt(dx, dy-CreditsTop) {
						continue
					}
					for ky := 0; ky < 4; ky++ {
						off := out.PixOffset(dx*4, dy*4+ky)
						if high != nil {
							so := high.PixOffset(sx*4, sy*4+ky)
							row := high.Pix[so : so+16]
							if row[3] == 255 && row[7] == 255 && row[11] == 255 && row[15] == 255 {
								copy(out.Pix[off:off+16], row)
							} else {
								draw.Draw(out, image.Rect(dx*4, dy*4+ky, dx*4+4, dy*4+ky+1), high, image.Pt(sx*4, sy*4+ky), draw.Over)
							}
						} else {
							col := assets.EGAPalette[v]
							for kx := 0; kx < 16; kx += 4 {
								out.Pix[off+kx], out.Pix[off+kx+1], out.Pix[off+kx+2], out.Pix[off+kx+3] = col.R, col.G, col.B, 255
							}
						}
					}
				}
			}
		}
	}
	c.addHigh(out, c.Img.Bounds(), image.Point{})
}
