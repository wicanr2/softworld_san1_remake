package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/opening"
)

func TestHDOpeningSourceValidation(t *testing.T) {
	for _, s := range []struct {
		n    string
		w, h int
	}{{"CMARKL.IMG", 320, 290}, {"CMARKR.IMG", 320, 290}, {"SANTL.IMG", 320, 295}, {"SANTR.IMG", 320, 295}, {"SANTBB.IMG", 40, 64}, {"SANTBM1.IMG", 24, 32}, {"SANTBM2.IMG", 16, 20}, {"SANTBS.IMG", 16, 16}, {"TITL0.IMG", 160, 400}, {"TITL1.IMG", 160, 400}, {"TITL2.IMG", 160, 400}, {"TITL3.IMG", 160, 400}} {
		t.Run(s.n, func(t *testing.T) {
			for _, check := range []string{"valid", "shape", "alpha", "container", "source_hash", "png_hash", "missing"} {
				t.Run(check, func(t *testing.T) {
					h := s.h
					if check == "shape" {
						h--
					}
					im := &assets.Image{W: s.w, H: h, Pix: make([]byte, s.w*h)}
					dir, c, e, _ := hdMenuFixture(t, s.n, im, check == "alpha")
					e.Container = "DATA1"
					switch check {
					case "container":
						e.Container = "DATA3"
					case "source_hash":
						e.SourceSHA256 = "wrong"
					case "png_hash":
						e.SHA256 = "wrong"
					case "missing":
						if err := os.Remove(filepath.Join(dir, e.File)); err != nil {
							t.Fatal(err)
						}
					}
					hdManifest(t, dir, []HDEntry{e})
					p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": c, "DATA3": c})
					if err != nil {
						t.Fatal(err)
					}
					if check == "valid" {
						if p.Count != 1 || len(p.Warnings) != 0 {
							t.Fatal(p.Warnings)
						}
					} else if p.Count != 0 || len(p.Warnings) != 1 {
						t.Fatal("invalid accepted", check, p.Count, p.Warnings)
					}
				})
			}
		})
	}
	for _, n := range []string{"SANTBB.IMG", "SANTBM1.IMG", "SANTBM2.IMG", "SANTBS.IMG"} {
		t.Run(n+"-mask", func(t *testing.T) {
			w, h := 40, 64
			switch n {
			case "SANTBM1.IMG":
				w, h = 24, 32
			case "SANTBM2.IMG":
				w, h = 16, 20
			case "SANTBS.IMG":
				w, h = 16, 16
			}
			im := &assets.Image{W: w, H: h, Pix: make([]byte, w*h)}
			im.Pix[0] = 1
			dir, c, e, _ := hdMenuFixture(t, n, im, false)
			e.Container = "DATA1"
			hdManifest(t, dir, []HDEntry{e})
			p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": c})
			if err != nil || p.Count != 0 || len(p.Warnings) != 1 {
				t.Fatal("invalid boat mask accepted", err, p)
			}
		})
	}
	for _, n := range []string{"TITFONT.IMG", "PRV0.IMG", "TZUE.IMG", "TZUE1.IMG", "LOADS.IMG", "SANTBM3.IMG", "TITL4.IMG"} {
		if hdResource.MatchString(n) {
			t.Fatal("text/mask/outside art accepted", n)
		}
	}
}

func openingPattern(w, h int) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			im.SetRGBA(x, y, color.RGBA{byte(x*7 + y), byte(y*3 + x), byte(x ^ y), 255})
		}
	}
	return im
}

func openingMirror(t *testing.T) (*opening.Pages, *hdOpening) {
	t.Helper()
	p := opening.NewPages()
	pack := &HDPack{images: map[[32]byte]*image.RGBA{{1}: image.NewRGBA(image.Rect(0, 0, 4, 4))}}
	BindHDOpening(p, pack)
	return p, p.Observer.(*hdOpening)
}

func checkOpeningNearest(t *testing.T, p *opening.Pages, h *hdOpening) {
	t.Helper()
	for i := range h.p {
		want := image.NewRGBA(h.p[i].Bounds())
		scaleRGBA4(want, p.P[i].RGBA())
		if !bytes.Equal(want.Pix, h.p[i].Pix) {
			t.Fatalf("fallback page %d differs", i)
		}
	}
}

func TestHDOpeningFallbackBitOperations(t *testing.T) {
	p, h := openingMirror(t)
	im := &assets.Image{W: 23, H: 7, Pix: make([]byte, 23*7)}
	for i := range im.Pix {
		im.Pix[i] = byte(i % 16)
	}
	for _, mode := range []opening.Mode{opening.Copy, opening.And, opening.Or, opening.Xor, opening.Mode(-1)} {
		for _, pos := range [][2]int{{-3, -2}, {9, 13}, {635, 405}} {
			p.Draw = 0
			p.Clear(13)
			p.Draw = 1
			p.Clear(6)
			p.Put(im, pos[0], pos[1], mode)
			checkOpeningNearest(t, p, h)
			p.CopyPage(1, 0)
			checkOpeningNearest(t, p, h)
		}
	}
	for _, v := range []byte{0, 7, 0x38, 0x3f} {
		for i := range p.Pal {
			p.Pal[i] = v
		}
		p.Show = 1
		high := h.visible()
		want := opening.EGAColor(v)
		for _, pt := range []image.Point{{0, 0}, {44, 60}, {2559, 1631}} {
			if got := high.RGBAAt(pt.X, pt.Y); got != want {
				t.Fatal("palette fallback", v, pt, got, want)
			}
		}
	}
}

func TestHDOpeningNativePutAndOwnership(t *testing.T) {
	p, h := openingMirror(t)
	im := &assets.Image{W: 24, H: 12, Pix: make([]byte, 24*12)}
	high := openingPattern(96, 48)
	h.pack.images[hdImageKey(im)] = high
	p.Clear(11)
	p.Put(im, -3, 7, opening.Copy)
	want := image.NewRGBA(h.p[0].Bounds())
	draw.Draw(want, want.Bounds(), image.NewUniform(assets.EGAPalette[11]), image.Point{}, draw.Src)
	draw.Draw(want, image.Rect(-12, 28, 84, 76), high, image.Point{}, draw.Src)
	if !bytes.Equal(want.Pix, h.p[0].Pix) {
		t.Fatal("clipped native Put")
	}
	mask := &assets.Image{W: 16, H: 4, Pix: make([]byte, 64)}
	for i := range mask.Pix {
		if i%2 == 0 {
			mask.Pix[i] = 15
		}
	}
	boat := openingPattern(64, 16)
	h.pack.images[hdImageKey(mask)] = boat
	p.Put(mask, 0, 8, opening.And)
	for y := 0; y < 4; y++ {
		for x := 0; x < 16; x++ {
			if mask.At(x, y) == 15 {
				continue
			}
			draw.Draw(want, image.Rect(x*4, (8+y)*4, x*4+4, (8+y)*4+4), boat, image.Pt(x*4, y*4), draw.Src)
		}
	}
	if !bytes.Equal(want.Pix, h.p[0].Pix) {
		t.Fatal("AND ownership")
	}
	// 同色0仍有覆蓋權；後畫缺高清的字也必須蓋住前圖。
	p.Put(&assets.Image{W: 8, H: 1, Pix: make([]byte, 8)}, 0, 8, opening.And)
	draw.Draw(want, image.Rect(0, 32, 32, 36), image.Black, image.Point{}, draw.Src)
	if !bytes.Equal(want.Pix, h.p[0].Pix) {
		t.Fatal("same color zero write or missing glyph")
	}
	p.CopyPage(0, 1)
	p.Show = 1
	c := NewCanvasPx(640, 408, nil)
	c.HD = h.pack
	for i := 0; i < 3; i++ {
		DrawPages(c, p)
		if len(c.highOps) != 1 {
			t.Fatal("old page layer retained")
		}
		if !bytes.Equal(c.Output(true).Pix, want.Pix) {
			t.Fatal("complete native output")
		}
	}
	DrawPages(c, p)
	if got := c.Output(false); got.Bounds() != image.Rect(0, 0, 640, 408) {
		t.Fatal("original output dimensions")
	}
}

func TestHDOpeningCopyRectAndCapture(t *testing.T) {
	for _, q := range [][8]int{{0, 1, 212, 8, 251, 12, 216, 30}, {0, 0, 0, 0, 63, 4, 8, 0}, {0, 0, 8, 0, 71, 4, 0, 0}, {0, 0, 0, 0, 31, 7, 0, 2}, {0, 1, -8, -2, 24, 3, -16, -1}, {1, 0, 632, 402, 660, 420, 624, 400}} {
		t.Run(fmt.Sprint(q), func(t *testing.T) {
			p, h := openingMirror(t)
			for i := range h.p {
				im := openingPattern(2560, 1632)
				copy(h.p[i].Pix, im.Pix)
			}
			expected := [2]*image.RGBA{}
			for i := range expected {
				expected[i] = image.NewRGBA(h.p[i].Bounds())
				copy(expected[i].Pix, h.p[i].Pix)
			}
			from, to, x1, y1, x2, y2, dx, dy := q[0], q[1], q[2], q[3], q[4], q[5], q[6], q[7]
			sx0, dx0 := (x1>>3)*8, (dx>>3)*8
			w := (((x2 - x1) >> 3) + 1) * 8
			for r := 0; r <= y2-y1; r++ {
				for i := 0; i < w; i++ {
					sx, sy, tx, ty := sx0+i, y1+r, dx0+i, dy+r
					if sx < 0 || sx >= 640 || sy < 0 || sy >= 408 || tx < 0 || tx >= 640 || ty < 0 || ty >= 408 {
						continue
					}
					for ky := 0; ky < 4; ky++ {
						for kx := 0; kx < 4; kx++ {
							expected[to].SetRGBA(tx*4+kx, ty*4+ky, expected[from].RGBAAt(sx*4+kx, sy*4+ky))
						}
					}
				}
			}
			p.CopyRect(from, to, x1, y1, x2, y2, dx, dy)
			for i := range expected {
				if !bytes.Equal(h.p[i].Pix, expected[i].Pix) {
					t.Fatal("byte alignment/forward overlap", i)
				}
			}
			p.Draw = to
			captured := p.Capture(-2, 2, 9, 6)
			p.Draw = from
			p.Put(captured, 30, 40, opening.Copy)
			for y := 0; y < 5; y++ {
				for x := 0; x < 8; x++ {
					for sy := 0; sy < 4; sy++ {
						for sx := 0; sx < 4; sx++ {
							want := color.RGBA{A: 255}
							if x >= 2 {
								want = expected[to].RGBAAt((x-2)*4+sx, (y+2)*4+sy)
							}
							if got := h.p[from].RGBAAt((30+x)*4+sx, (40+y)*4+sy); got != want {
								t.Fatal("capture/moved strip", x, y, got, want)
							}
						}
					}
				}
			}
		})
	}
	p, h := openingMirror(t)
	for i := 0; i < 81; i++ {
		p.Capture(i*8, 0, i*8+7, 407)
	}
	if len(h.captures) != 80 || h.captureBytes != hdOpeningCaptureBudget {
		t.Fatal("capture bound", len(h.captures), h.captureBytes)
	}
}

func TestHDOpeningFullScriptCPUAndTitle(t *testing.T) {
	art, err := opening.LoadArt(artContainer(t, "DATA1"))
	if err != nil {
		t.Fatal(err)
	}
	art.PoemInk = &assets.Image{W: 464, H: 166, Pix: make([]byte, 464*166)}
	art.PoemMask = art.PoemInk.Clone()
	for i := range art.PoemMask.Pix {
		art.PoemMask.Pix[i] = 15
	}
	pack := &HDPack{images: map[[32]byte]*image.RGBA{}}
	for _, im := range append(append([]*assets.Image{art.CMarkL, art.CMarkR, art.SantL, art.SantR}, art.Boats[:]...), append(art.Titl[:], art.Faces[:]...)...) {
		pack.images[hdImageKey(im)] = openingPattern(im.W*4, im.H*4)
	}
	original := &opening.Script{Art: art, Rand: func(int) int { return 0 }}
	high := &opening.Script{Art: art, Rand: func(int) int { return 0 }, PagesReady: func(p *opening.Pages) { BindHDOpening(p, pack) }}
	next, stop := iter.Pull(original.Beats())
	defer stop()
	count := 0
	sites := map[opening.Site]int{}
	for b := range high.Beats() {
		a, ok := next()
		if !ok || a.Kind != b.Kind || a.N != b.N || a.Site != b.Site || a.Pages.Pal != b.Pages.Pal || a.Pages.Draw != b.Pages.Draw || a.Pages.Show != b.Pages.Show {
			t.Fatal("CPU sequence changed", count)
		}
		for i := 0; i < 2; i++ {
			if !bytes.Equal(a.Pages.P[i].Pix, b.Pages.P[i].Pix) {
				t.Fatal("CPU page changed", count, i)
			}
		}
		count++
		sites[b.Site]++
		if b.Site == opening.SiteTitleKey {
			h := b.Pages.Observer.(*hdOpening)
			want := image.NewRGBA(h.p[0].Bounds())
			draw.Draw(want, want.Bounds(), image.Black, image.Point{}, draw.Src)
			for i, im := range art.Titl {
				draw.Draw(want, image.Rect(i*640, 0, (i+1)*640, 1600), pack.images[hdImageKey(im)], image.Point{}, draw.Src)
			}
			for i := 0; i < 2; i++ {
				if !bytes.Equal(want.Pix, h.p[i].Pix) {
					t.Fatal("80 captured strips do not reconstruct complete title", i)
				}
			}
		}
	}
	if _, ok := next(); ok || count != 1308 || len(sites) != 15 {
		t.Fatal("incomplete script", count, len(sites), ok)
	}
}
