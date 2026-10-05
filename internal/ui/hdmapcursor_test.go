package ui

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func TestHDMapCursorEntryValidation(t *testing.T) {
	for _, check := range []string{"valid", "shape", "palette", "white", "identity", "alpha", "container", "source", "png-hash", "missing", "unknown"} {
		t.Run(check, func(t *testing.T) {
			im := &assets.Image{W: 48, H: 32, Pix: make([]byte, 48*32)}
			for i := range im.Pix {
				if i%7 != 0 {
					im.Pix[i] = 15
				}
			}
			if check == "shape" {
				im.W = 40
				im.Pix = im.Pix[:40*32]
			}
			if check == "palette" {
				im.Pix[1] = 6
			}
			dir, source, e, _ := hdMenuFixture(t, "MAPCUR1.IMG", im, false)
			e.Container = "DATA1"
			high := image.NewRGBA(image.Rect(0, 0, im.W*4, im.H*4))
			for y := 0; y < im.H*4; y++ {
				for x := 0; x < im.W*4; x++ {
					if im.At(x/4, y/4) == 15 {
						high.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
					}
				}
			}
			switch check {
			case "white":
				high.SetRGBA(4, 0, color.RGBA{254, 255, 255, 255})
			case "identity":
				high.SetRGBA(0, 0, color.RGBA{1, 0, 0, 255})
			case "alpha":
				high.SetRGBA(4, 0, color.RGBA{127, 127, 127, 127})
			case "container":
				e.Container = "DATA3"
			case "source":
				e.SourceSHA256 = "wrong"
			case "unknown":
				e.Name = "MAPCUR0.IMG"
			}
			var buf bytes.Buffer
			if err := png.Encode(&buf, high); err != nil {
				t.Fatal(err)
			}
			e.SHA256 = fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))
			if check == "png-hash" {
				e.SHA256 = "wrong"
			}
			if err := os.WriteFile(filepath.Join(dir, e.File), buf.Bytes(), 0644); err != nil {
				t.Fatal(err)
			}
			if check == "missing" {
				if err := os.Remove(filepath.Join(dir, e.File)); err != nil {
					t.Fatal(err)
				}
			}
			_, _, err := loadHDEntry(dir, e, map[string]*assets.Container{"DATA1": source, "DATA3": source})
			if (err == nil) != (check == "valid") {
				t.Fatalf("validation: %v", err)
			}
			m := HDManifest{Schema: 1, Style: "b", Scale: 4, Entries: []HDEntry{e}}
			b, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0644); err != nil {
				t.Fatal(err)
			}
			pack, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source, "DATA3": source})
			if err != nil {
				t.Fatal(err)
			}
			if (pack.mapCursor != nil) != (check == "valid") {
				t.Fatal("invalid marker registered")
			}
		})
	}
}

func TestHDMapCursorClippingAndForeground(t *testing.T) {
	im := &assets.Image{W: 48, H: 32, Pix: make([]byte, 48*32)}
	for i := range im.Pix {
		im.Pix[i] = 15
	}
	im.Pix[0] = 0
	for _, at := range []image.Point{{-7, -5}, {9, 8}, {55, 40}, {80, 60}} {
		c := testCanvasPx(t, 64, 48)
		p := &HDPack{mapCursor: image.NewRGBA(image.Rect(0, 0, 192, 128)), mapCursorKey: hdImageKey(im)}
		c.HD = p
		c.Fill(assets.EGAPalette[6])
		native := image.NewRGBA(image.Rect(0, 0, 256, 192))
		draw.Draw(native, native.Bounds(), image.NewUniform(color.RGBA{13, 79, 123, 255}), image.Point{}, draw.Src)
		c.addHigh(native, c.Img.Bounds(), image.Point{})
		c.FillRect(16, 14, 20, 18, assets.EGAPalette[6])
		prefix := legacyHighOutput(c)
		high := c.highMapCursorPrefix(im, at.X, at.Y)
		for y := 0; y < 32; y++ {
			for x := 0; x < 48; x++ {
				if im.At(x, y) != 0 {
					xx, yy := at.X+x, at.Y+y
					if image.Pt(xx, yy).In(c.Img.Bounds()) {
						c.setClipped(xx, yy, assets.EGAPalette[egaIndexOf(c.Img.RGBAAt(xx, yy))^15])
					}
				}
			}
		}
		c.finishHighMapCursor(im, at.X, at.Y, high)
		out := c.Output(true)
		for yy := 0; yy < 192; yy++ {
			for xx := 0; xx < 256; xx++ {
				want := prefix.RGBAAt(xx, yy)
				lx, ly := xx/4, yy/4
				if image.Pt(lx-at.X, ly-at.Y).In(image.Rect(0, 0, 48, 32)) && im.At(lx-at.X, ly-at.Y) != 0 {
					if image.Pt(lx, ly).In(image.Rect(16, 14, 20, 18)) {
						want = assets.EGAPalette[9]
					} else {
						want = color.RGBA{242, 176, 132, 255}
					}
				}
				if out.RGBAAt(xx, yy) != want {
					t.Fatalf("clip %v at %d,%d", at, xx, yy)
				}
			}
		}
		c.FillRect(12, 11, 24, 22, assets.EGAPalette[3])
		out = c.Output(true)
		for yy := 44; yy < 88; yy++ {
			for xx := 48; xx < 96; xx++ {
				if out.RGBAAt(xx, yy) != assets.EGAPalette[3] {
					t.Fatal("later foreground lost")
				}
			}
		}
	}
}

// legacyHighOutput is the pre-change compositor, retained only as independent test evidence.
func legacyHighOutput(c *Canvas) *image.RGBA {
	b := c.Img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx()*4, b.Dy()*4))
	scaleRGBA4(out, c.Img)
	for _, op := range c.highOps {
		r := image.Rectangle{Min: op.rect.Min.Mul(4), Max: op.rect.Max.Mul(4)}
		mode := draw.Src
		if op.over {
			mode = draw.Over
		}
		draw.Draw(out, r, op.image, op.source, mode)
		for y := op.rect.Min.Y; y < op.rect.Max.Y; y++ {
			for x := op.rect.Min.X; x < op.rect.Max.X; x++ {
				if op.covered[(y-op.rect.Min.Y)*op.rect.Dx()+x-op.rect.Min.X] {
					for sy := 0; sy < 4; sy++ {
						for sx := 0; sx < 4; sx++ {
							out.SetRGBA(x*4+sx, y*4+sy, c.Img.RGBAAt(x, y))
						}
					}
				}
			}
		}
	}
	return out
}
func mapCursorTestSource(t *testing.T, ed string) (map[string]*assets.Container, *ArtBattle) {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("未掛原始資料")
	}
	folder := "三國演義"
	if ed == "plus" {
		folder = "三國演義1加強版"
	}
	cs := map[string]*assets.Container{}
	for _, n := range []string{"DATA1", "DATA2", "DATA3"} {
		var raw [3][]byte
		for j, ext := range []string{"NAM", "IDX", "GRP"} {
			b, e := os.ReadFile(filepath.Join(root, folder, n+"."+ext))
			if e != nil {
				t.Fatal(e)
			}
			raw[j] = b
		}
		c, e := assets.OpenContainer(raw[0], raw[1], raw[2])
		if e != nil {
			t.Fatal(e)
		}
		cs[n] = c
	}
	ab, e := NewArtBattle(cs["DATA1"], cs["DATA3"])
	if e != nil {
		t.Fatal(e)
	}
	return cs, ab
}
func mapCursorTestPack(t *testing.T, ed string, cs map[string]*assets.Container, ab *ArtBattle) *HDPack {
	t.Helper()
	p := &HDPack{images: map[[32]byte]*image.RGBA{}, mapCursor: image.NewRGBA(image.Rect(0, 0, 192, 128)), mapCursorKey: hdImageKey(ab.mapCursor)}
	for n := 0; n < 15; n++ {
		p.images[hdImageKey(ab.tiles[n])] = hdTerrainDetail(n)
	}
	for y := 0; y < 32; y++ {
		for x := 0; x < 48; x++ {
			if ab.mapCursor.At(x, y) == 15 {
				for sy := 0; sy < 4; sy++ {
					for sx := 0; sx < 4; sx++ {
						p.mapCursor.SetRGBA(x*4+sx, y*4+sy, color.RGBA{255, 255, 255, 255})
					}
				}
			}
		}
	}
	if e := validateHighMapCursor(ab.mapCursor, p.mapCursor); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestHDMapCursorCompositor(t *testing.T) {
	c := testCanvasPx(t, 64, 48)
	c.HD = &HDPack{}
	c.Fill(assets.EGAPalette[6])
	a := image.NewRGBA(image.Rect(0, 0, 256, 192))
	for y := 0; y < 192; y++ {
		for x := 0; x < 256; x++ {
			a.SetRGBA(x, y, color.RGBA{byte(x), byte(y), 99, 255})
		}
	}
	c.addHigh(a, image.Rect(0, 0, 64, 48), image.Point{})
	c.FillRect(8, 7, 19, 12, assets.EGAPalette[9])
	over := image.NewRGBA(image.Rect(0, 0, 96, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 96; x++ {
			if x%3 == 0 {
				over.SetRGBA(x, y, color.RGBA{30, 50, 70, 128})
			}
		}
	}
	c.addHigh(over, image.Rect(17, 11, 41, 31), image.Point{})
	c.highOps[len(c.highOps)-1].over = true
	c.FillRect(32, 17, 40, 25, assets.EGAPalette[6])
	want := legacyHighOutput(c)
	if !bytes.Equal(c.Output(true).Pix, want.Pix) {
		t.Fatal("full compositor changed")
	}
	for _, r := range []image.Rectangle{image.Rect(0, 0, 12, 8), image.Rect(4, 5, 25, 22), image.Rect(31, 15, 63, 46)} {
		b := image.Rectangle{Min: r.Min.Mul(4), Max: r.Max.Mul(4)}
		got := image.NewRGBA(b)
		owned := make([]bool, b.Dx()*b.Dy())
		c.composeHigh(got, owned)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if got.RGBAAt(x, y) != want.RGBAAt(x, y) {
					t.Fatalf("prefix at %v", image.Pt(x, y))
				}
			}
		}
	}
}
func TestHDMapCursorAllCells(t *testing.T) {
	for _, ed := range []string{"base", "plus"} {
		t.Run(ed, func(t *testing.T) {
			cs, ab := mapCursorTestSource(t, ed)
			p := mapCursorTestPack(t, ed, cs, ab)
			sc, e := state.LoadScenario(cs["DATA2"], state.Scenario3)
			if e != nil {
				t.Fatal(e)
			}
			pref, e := sc.Prefecture(15)
			if e != nil {
				t.Fatal(e)
			}
			fld, e := battle.Load(pref.BattleField, pref.Neighbours)
			if e != nil {
				t.Fatal(e)
			}
			for _, confirm := range []bool{false, true} {
				off := testCanvasPx(t, 640, 408)
				off.HD = p
				DrawArtFortSpot(off, ab, pref.BattleField, fld, FortSpot{Confirm: confirm})
				nativeOff := legacyHighOutput(off)
				for row := 0; row < 10; row++ {
					for col := 0; col < 12; col++ {
						plain, c := testCanvasPx(t, 640, 408), testCanvasPx(t, 640, 408)
						c.HD = p
						s := FortSpot{Col: col, Row: row, Marked: true, Confirm: confirm}
						DrawArtFortSpot(plain, ab, pref.BattleField, fld, s)
						DrawArtFortSpot(c, ab, pref.BattleField, fld, s)
						if !bytes.Equal(plain.Img.Pix, c.Img.Pix) {
							t.Fatal("CPU changed")
						}
						x, y := assets.FieldCell(col, row)
						r := image.Rect(x, y, x+48, y+32).Intersect(c.Img.Bounds())
						b := image.Rectangle{Min: r.Min.Mul(4), Max: r.Max.Mul(4)}
						got := image.NewRGBA(b)
						c.composeHigh(got, nil)
						for yy := b.Min.Y; yy < b.Max.Y; yy++ {
							for xx := b.Min.X; xx < b.Max.X; xx++ {
								want := nativeOff.RGBAAt(xx, yy)
								if ab.mapCursor.At(xx/4-x, yy/4-y) == 15 {
									if c.mapCursorOwned[(yy-b.Min.Y)*b.Dx()+xx-b.Min.X] {
										want.R ^= 255
										want.G ^= 255
										want.B ^= 255
									} else {
										want = plain.Img.RGBAAt(xx/4, yy/4)
									}
								}
								if got.RGBAAt(xx, yy) != want {
									t.Fatalf("%s cell %d,%d native %d,%d got %v want %v", ed, col, row, xx, yy, got.RGBAAt(xx, yy), want)
								}
							}
						}
					}
				}
			}
			for _, s := range []FortSpot{{Marked: true}, {Col: 3, Row: 1, Marked: true}, {Col: 11, Row: 9, Marked: true, Confirm: true}, {Col: 3, Row: 1}} {
				c := testCanvasPx(t, 640, 408)
				c.HD = p
				DrawArtFortSpot(c, ab, pref.BattleField, fld, s)
				if !bytes.Equal(c.Output(true).Pix, legacyHighOutput(c).Pix) {
					t.Fatal("full / shared compositor mismatch")
				}
			}
		})
	}
}
