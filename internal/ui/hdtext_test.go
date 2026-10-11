package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"os"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
)

func vectorTestFace(t *testing.T, kind string) *font.Face {
	t.Helper()
	f := namedFace(t, kind+".hex.gz")
	h, err := os.Open("../../fonts/" + kind + ".ttf")
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := f.LoadVector(h); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestHDVectorBackgroundClippingAndForeground(t *testing.T) {
	for _, kind := range []string{"kai", "li"} {
		t.Run(kind, func(t *testing.T) {
			f := vectorTestFace(t, kind)
			for _, pos := range []image.Point{{8, 8}, {-3, -2}, {37, 25}} {
				c := NewCanvasPx(48, 32, f)
				c.HD = &HDPack{}
				c.Fill(bg)
				back := image.NewRGBA(image.Rect(0, 0, 192, 128))
				for y := 0; y < 128; y++ {
					for x := 0; x < 192; x++ {
						back.SetRGBA(x, y, color.RGBA{byte(x), byte(y), 77, 255})
					}
				}
				c.addHigh(back, c.Img.Bounds(), image.Point{})
				want := image.NewRGBA(back.Bounds())
				draw.Draw(want, want.Bounds(), back, image.Point{}, draw.Src)
				mask, _ := f.VectorGlyph('劉', 64, 64)
				draw.DrawMask(want, mask.Bounds().Add(pos.Mul(4)), image.White, image.Point{}, mask, image.Point{}, draw.Over)
				c.DrawTextPx(pos.X, pos.Y, "劉", fg)
				if !bytes.Equal(c.Output(true).Pix, want.Pix) {
					t.Fatalf("%v：向量文字或 HD 背景不符", pos)
				}
				original := NewCanvasPx(48, 32, f)
				original.Fill(bg)
				original.DrawTextPx(pos.X, pos.Y, "劉", fg)
				if !bytes.Equal(c.Img.Pix, original.Img.Pix) {
					t.Fatal("原貌點陣被修改")
				}
				c.FillRect(10, 10, 13, 13, color.RGBA{255, 0, 0, 255})
				draw.Draw(want, image.Rect(40, 40, 52, 52), image.NewUniform(color.RGBA{255, 0, 0, 255}), image.Point{}, draw.Src)
				if !bytes.Equal(c.Output(true).Pix, want.Pix) {
					t.Fatal("後畫前景未遮住向量字")
				}
				c.Fill(bg)
				if len(c.highOps) != 0 || c.Output(true).RGBAAt(32, 32) != bg {
					t.Fatal("清畫布殘留向量字")
				}
			}
		})
	}
}

func TestHDVectorRotatedDitherAndFallback(t *testing.T) {
	f := vectorTestFace(t, "li")
	c := NewCanvasPx(48, 32, f)
	c.HD = &HDPack{}
	c.Fill(bg)
	c.DrawTextRotatedPx(4, 4, "A", fg)
	m, _ := f.VectorGlyph('A', 32, 64)
	want := image.NewRGBA(image.Rect(0, 0, 192, 128))
	draw.Draw(want, want.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	for y := 0; y < 64; y++ {
		for x := 0; x < 32; x++ {
			a := m.AlphaAt(x, y).A
			want.SetRGBA(16+63-y, 16+x, color.RGBA{a, a, a, 255})
		}
	}
	if !bytes.Equal(c.Output(true).Pix, want.Pix) {
		t.Fatal("旋轉向量字不符")
	}
	c.Fill(bg)
	red, blue := color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}
	c.DrawRuneBoxDitherPx(4, 4, 16, 16, '劉', red, blue)
	out := c.Output(true)
	ink := 0
	for y := 16; y < 80; y++ {
		for x := 16; x < 80; x++ {
			p := out.RGBAAt(x, y)
			if p.G != 0 || ((x/4+y/4)%2 == 1 && p.B != 0) || ((x/4+y/4)%2 == 0 && p.R != 0) {
				t.Fatal("雙色字格顏色不符")
			}
			if p.R != 0 || p.B != 0 {
				ink++
			}
		}
	}
	if ink == 0 {
		t.Fatal("雙色字格全空")
	}
	bitmap := namedFace(t, "li.hex.gz")
	c.SetFace(bitmap)
	c.Fill(bg)
	c.DrawTextPx(4, 4, "劉", fg)
	if len(c.highOps) != 0 {
		t.Fatal("沒有向量來源未回退")
	}
}

func TestHDVectorFlagSwitchAndComplement(t *testing.T) {
	dir, source, entry, im, _ := hdFlagFixture(t)
	hdManifest(t, dir, []HDEntry{entry})
	pack, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
	if err != nil {
		t.Fatal(err)
	}
	face := vectorTestFace(t, "kai")
	c := NewCanvasPx(24, 15, face)
	c.HD = pack
	kai := c.HighImage(im)
	selected := c.HighImage(im.Complement())
	for i := 0; i < len(kai.Pix); i += 4 {
		for channel := 0; channel < 3; channel++ {
			if kai.Pix[i+channel]^255 != selected.Pix[i+channel] {
				t.Fatal("向量旗字反白不符")
			}
		}
		if kai.Pix[i+3] != selected.Pix[i+3] {
			t.Fatal("反白改變 alpha")
		}
	}
	if c.HighImage(im) != kai {
		t.Fatal("相同向量旗字未使用快取")
	}
	c.SetFace(vectorTestFace(t, "li"))
	li := c.HighImage(im)
	if bytes.Equal(kai.Pix, li.Pix) {
		t.Fatal("楷隸旗字未更新")
	}
	c.SetFace(namedFace(t, "li.hex.gz"))
	bitmap := c.HighImage(im)
	if bytes.Equal(bitmap.Pix, li.Pix) {
		t.Fatal("點陣回退沿用向量旗字")
	}
}
