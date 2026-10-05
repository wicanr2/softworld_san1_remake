package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
)

func hdSearchFixture(t *testing.T, paper, portrait bool) (*Canvas, *ArtScreen) {
	t.Helper()
	_, source, _, face, high := hdFixture(t)
	c := NewCanvasPx(640, 408, nil)
	c.HD = &HDPack{images: map[[32]byte]*image.RGBA{}, panels: map[string]hdPanel{}, panelCache: map[hdPanelSize]*image.RGBA{}}
	if portrait {
		c.HD.images[hdImageKey(face)] = high
	}
	if paper {
		skin := image.NewRGBA(image.Rect(0, 0, 720, 400))
		for y := 0; y < 400; y++ {
			for x := 0; x < 720; x++ {
				skin.SetRGBA(x, y, color.RGBA{byte(x), byte(y), 73, 255})
			}
		}
		c.HD.panels["PANEL.BEVEL#1"] = hdPanel{skin, 2}
	}
	return c, &ArtScreen{faces: source}
}

func TestHDSearchPreparedMatchesCompletedPanel(t *testing.T) {
	if got := clearPanelRects(408, 36, 631, 291); got != [2]image.Rectangle{image.Rect(424, 44, 616, 284), image.Rect(416, 52, 624, 276)} {
		t.Fatalf("清底幾何改變：%v", got)
	}
	for _, paper := range []bool{false, true} {
		for _, portrait := range []bool{false, true} {
			for _, slot := range []int{0, -1, 999} {
				t.Run(fmt.Sprintf("paper-%t/portrait-%t/slot-%d", paper, portrait, slot), func(t *testing.T) {
					c, a := hdSearchFixture(t, paper, portrait)
					prepared := c.SearchHighScene(a, slot)
					if !paper && (!portrait || slot != 0) {
						if prepared != nil {
							t.Fatal("無素材時應使用原貌拉幕")
						}
						return
					}
					if prepared == nil || prepared.Bounds() != image.Rect(0, 0, 704, 384) {
						t.Fatal("尋訪準備圖缺失或尺寸錯誤")
					}
					completed := NewCanvasPx(640, 408, nil)
					completed.HD = c.HD
					ClearPanel(completed, 408, 36, 631, 291, assets.EGAPalette[1])
					DrawBubbleAs(completed, a, &game.Bubble{X1: 488, Y1: 88, FaceOnly: true}, "", slot)
					out := completed.Output(true)
					for y := 0; y < 384; y++ {
						for x := 0; x < 704; x++ {
							if prepared.RGBAAt(x, y) != out.RGBAAt(432*4+x, 80*4+y) {
								t.Fatalf("尋訪拉幕與完成畫面不同：%d,%d", x, y)
							}
						}
					}
				})
			}
		}
	}
}

func TestHDSearchWipePreservesFullFrames(t *testing.T) {
	for _, kind := range []WipeKind{WipeDown, WipeUp, WipeRight, WipeLeft} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			c, a := hdSearchFixture(t, true, true)
			plain := NewCanvasPx(640, 408, nil)
			background := color.RGBA{19, 43, 61, 255}
			c.Fill(background)
			plain.Fill(background)
			prepared := c.SearchHighScene(a, 0)
			scene := SearchPanel(a, 0)
			w := NewSceneWipe(c, scene, kind, 432, 80, prepared)
			original := NewSceneWipe(plain, scene, kind, 432, 80)
			expected := image.NewRGBA(image.Rect(0, 0, 2560, 1632))
			draw.Draw(expected, expected.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
			steps := 0
			for w.Advance(c.Img) {
				steps++
				if !original.Advance(plain.Img) || !bytes.Equal(c.Img.Pix, plain.Img.Pix) {
					t.Fatal("原貌動畫或步數改變")
				}
				draw.Draw(expected, image.Rect(432*4, 80*4, 608*4, 176*4), image.NewUniform(background), image.Point{}, draw.Src)
				r := w.Reveal(steps)
				source := w.Source(r).Min.Sub(image.Pt(432, 80)).Mul(4)
				draw.Draw(expected, image.Rect(r.Min.X*4, r.Min.Y*4, r.Max.X*4, r.Max.Y*4), prepared, source, draw.Src)
				if !bytes.Equal(c.Output(true).Pix, expected.Pix) {
					t.Fatalf("完整高清拉幕錯誤：方向%d，步%d", kind, steps)
				}
			}
			if steps != w.Steps() || original.Advance(plain.Img) {
				t.Fatal("完整拉幕步數改變")
			}
		})
	}
}

func TestHDSearchCanvasAndCache(t *testing.T) {
	c, a := hdSearchFixture(t, true, true)
	c.Fill(color.RGBA{19, 43, 61, 255})
	c.addHigh(c.HD.images[hdImageKey(a.Portrait(0))], image.Rect(0, 0, 64, 80), image.Point{})
	before := append([]byte(nil), c.Img.Pix...)
	op := c.highOps[0]
	for range 20 {
		if c.SearchHighScene(a, 0) == nil {
			t.Fatal("重複準備失敗")
		}
	}
	if !bytes.Equal(before, c.Img.Pix) || len(c.highOps) != 1 || c.highOps[0] != op || c.Output(false) != c.Img {
		t.Fatal("準備拉幕改寫現有畫布或圖層")
	}
	if len(c.HD.panelCache) != 1 || c.HD.panelCacheBytes != 208*224*4*4*4 {
		t.Fatalf("尋訪未重用完成底紙：%d尺寸，%d bytes", len(c.HD.panelCache), c.HD.panelCacheBytes)
	}
	if c.SearchHighScene(nil, 0) != nil {
		t.Fatal("缺 ArtScreen 未回退")
	}
	c.HD = nil
	if c.SearchHighScene(a, 0) != nil {
		t.Fatal("原貌未回退")
	}
}
