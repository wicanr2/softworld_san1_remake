package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"testing"

	xdraw "golang.org/x/image/draw"
)

func TestScaleRGBA4MatchesNearestNeighbor(t *testing.T) {
	for _, wh := range [][2]int{{0, 0}, {1, 1}, {3, 2}, {17, 3}, {176, 96}, {640, 408}} {
		for _, sub := range []bool{false, true} {
			t.Run(fmt.Sprintf("%dx%d/sub=%t", wh[0], wh[1], sub), func(t *testing.T) {
				w, h := wh[0], wh[1]
				src := image.NewRGBA(image.Rect(0, 0, w, h))
				if sub {
					src = image.NewRGBA(image.Rect(-3, 7, w+2, h+11))
				}
				for n := range src.Pix {
					src.Pix[n] = byte(n*17 + 11)
				}
				if sub {
					src = src.SubImage(image.Rect(-1, 8, w-1, h+8)).(*image.RGBA)
				}
				got := image.NewRGBA(image.Rect(-9, 5, w*4+17, h*4+23))
				for n := range got.Pix {
					got.Pix[n] = 71
				}
				want := image.NewRGBA(got.Rect)
				copy(want.Pix, got.Pix)
				r := image.Rect(-2, 11, w*4-2, h*4+11)
				dst := got.SubImage(r).(*image.RGBA)
				expected := want.SubImage(r).(*image.RGBA)
				xdraw.NearestNeighbor.Scale(expected, expected.Bounds(), src, src.Bounds(), draw.Src, nil)
				scaleRGBA4(dst, src)
				if !bytes.Equal(got.Pix, want.Pix) {
					t.Fatal("RGBA、alpha、stride 或矩形外像素與最近鄰不同")
				}
			})
		}
	}
}
