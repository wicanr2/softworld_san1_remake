package ui

import (
	"image"
	"image/color"
	"testing"
)

// 接收端逐包還原完整畫面，涵蓋四通道、stride 與非零原點。
func TestPixelUploadReconstructsFrames(t *testing.T) {
	for _, b := range []image.Rectangle{image.Rect(0, 0, 17, 11), image.Rect(-5, 7, 19, 31)} {
		parent := image.NewRGBA(b.Inset(-3))
		src := parent.SubImage(b).(*image.RGBA)
		back := image.NewRGBA(b)
		var u PixelUpload
		apply := func(r image.Rectangle, packet []byte) {
			t.Helper()
			if len(packet) != r.Dx()*r.Dy()*4 {
				t.Fatalf("矩形 %v 的封包長度 %d", r, len(packet))
			}
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					i := ((y-r.Min.Y)*r.Dx() + x - r.Min.X) * 4
					back.SetRGBA(x, y, color.RGBA{R: packet[i], G: packet[i+1], B: packet[i+2], A: packet[i+3]})
				}
			}
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					if back.RGBAAt(x, y) != src.RGBAAt(x, y) {
						t.Fatalf("完整還原差異 %d,%d", x, y)
					}
				}
			}
		}
		r, packet := u.Changed(src)
		if r != b {
			t.Fatal("首次空白畫面沒有完整上傳")
		}
		apply(r, packet)
		points := []image.Point{b.Min, b.Max.Sub(image.Pt(1, 1)), image.Pt(b.Min.X, b.Max.Y-1), image.Pt(b.Max.X-1, b.Min.Y), b.Min.Add(image.Pt(5, 5))}
		for i, p := range points {
			src.SetRGBA(p.X, p.Y, color.RGBA{R: byte(i + 1), G: 7, B: 9, A: 255})
			r, packet = u.Changed(src)
			if r != (image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))}) {
				t.Fatalf("單像素矩形 %v", r)
			}
			apply(r, packet)
			// 單獨 alpha 改變也要上傳。
			c := src.RGBAAt(p.X, p.Y)
			c.A = 128
			src.SetRGBA(p.X, p.Y, c)
			r, packet = u.Changed(src)
			apply(r, packet)
			if r.Empty() {
				t.Fatal("alpha 改變漏傳")
			}
		}
		// 清除位於相對兩角的前景，包圍框內既有內容必須保持。
		src.SetRGBA(b.Min.X, b.Min.Y, color.RGBA{})
		src.SetRGBA(b.Max.X-1, b.Max.Y-1, color.RGBA{})
		r, packet = u.Changed(src)
		apply(r, packet)
		if r != b {
			t.Fatal("分離區域沒有完整包圍")
		}
		if r, packet = u.Changed(src); !r.Empty() || len(packet) != 0 {
			t.Fatal("相同畫面仍上傳")
		}
	}
}

func TestPixelUploadResetsOnThemeSizeChange(t *testing.T) {
	var u PixelUpload
	for _, b := range []image.Rectangle{image.Rect(0, 0, 640, 408), image.Rect(0, 0, 2560, 1632), image.Rect(0, 0, 640, 408)} {
		src := image.NewRGBA(b)
		r, packet := u.Changed(src)
		if r != b || len(packet) != len(src.Pix) || len(u.previous.Pix) != len(src.Pix) || len(u.buffer) != 0 {
			t.Fatal("Theme 更換沒有完整刷新或保留舊大緩衝")
		}
	}
}
