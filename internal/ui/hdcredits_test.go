package ui

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

func TestHDCreditsOwnershipMaskAndCPU(t *testing.T) {
	cr := fakeCredits(1)
	part := &assets.Image{W: 320, H: 336, Pix: make([]byte, 320*336)}
	for i := range part.Pix {
		part.Pix[i] = 1
	}
	cr.BackdropParts[0][0] = part
	cr.Backdrop[0].Blit(part, 0, 0)
	ln := cr.Lines[0]
	ln.Pix[0], ln.Pix[1], ln.Pix[2] = 0, 0, 1
	high := image.NewRGBA(image.Rect(0, 0, 256, 96))
	back := image.NewRGBA(image.Rect(0, 0, 1280, 1344))
	for y := 0; y < high.Bounds().Dy(); y++ {
		for x := 0; x < high.Bounds().Dx(); x++ {
			high.SetRGBA(x, y, color.RGBA{byte(x), byte(y), 71, 255})
		}
	}
	for y := 0; y < back.Bounds().Dy(); y++ {
		for x := 0; x < back.Bounds().Dx(); x++ {
			back.SetRGBA(x, y, color.RGBA{11, byte(x % 251), byte(y % 251), 255})
		}
	}
	c := NewCanvasPx(640, 408, nil)
	plain := NewCanvasPx(640, 408, nil)
	c.HD = &HDPack{images: map[[32]byte]*image.RGBA{hdImageKey(part): back, hdImageKey(ln): high}, creditFigures: map[[32]byte][]image.Rectangle{hdImageKey(ln): {image.Rect(1, 0, 2, 1)}}}
	for _, missing := range []bool{false, true} {
		if missing {
			delete(c.HD.images, hdImageKey(ln))
		}
		for _, scroll := range []int{250, 330, 410} {
			DrawCredits(c, cr, scroll)
			DrawCredits(plain, cr, scroll)
			if !bytes.Equal(c.Img.Pix, plain.Img.Pix) {
				t.Fatal("高清改動 CPU 畫布")
			}
			out := c.Output(true)
			origin := image.Pt((640-ln.W)/2, 408-scroll)
			for y := 0; y < 408; y++ {
				for x := 0; x < 640; x++ {
					sx, sy := x-origin.X, y-origin.Y
					own := sx >= 0 && sx < ln.W && sy >= 0 && sy < 24 && cr.SkyAt(x, y-CreditsTop)
					if own {
						own = ln.At(sx, sy) != 0 || (!missing && sx == 1 && sy == 0)
					}
					for ky := 0; ky < 4; ky++ {
						for kx := 0; kx < 4; kx++ {
							want := plain.Img.RGBAAt(x, y)
							if x < 320 && y >= CreditsTop && y < CreditsTop+336 {
								want = back.RGBAAt(x*4+kx, (y-CreditsTop)*4+ky)
							}
							if own {
								if missing {
									want = assets.EGAPalette[ln.At(sx, sy)]
								} else {
									want = high.RGBAAt(sx*4+kx, sy*4+ky)
								}
							}
							if got := out.RGBAAt(x*4+kx, y*4+ky); got != want {
								t.Fatalf("missing=%v scroll=%d at (%d,%d)/(%d,%d): %v != %v", missing, scroll, x, y, kx, ky, got, want)
							}
						}
					}
				}
			}
		}
	}
	col := assets.EGAPalette[1]
	c.FillRect(290, 0, 291, 1, col)
	if c.Output(true).RGBAAt(1161, 1) != col {
		t.Fatal("後畫介面失去覆蓋權")
	}
	DrawCreditHall(c, nil)
	if len(c.highOps) != 0 {
		t.Fatal("前頁高清人物殘留")
	}
}

func TestHDCreditFigureRectsProtectHistoricalText(t *testing.T) {
	var area int
	for n, w := range hdCreditWidths {
		for _, r := range hdCreditFigures(fmt.Sprintf("UPR%02d.IMG", n), w) {
			global := r.Add(image.Pt((640-w)/2, n*24))
			area += r.Dx() * r.Dy()
			for _, top := range []int{120, 216, 312} {
				if !global.Intersect(image.Rect(288, top+24, 352, top+72)).Empty() {
					t.Fatal("人物覆蓋歷史文字區")
				}
			}
		}
	}
	if area != 18432 {
		t.Fatalf("人物美術範圍=%d", area)
	}
}

func TestHDCreditLoadRejectsWrongSourceShapeAndContainer(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int
	}{{"ENDO2.IMG", 160, 336}, {"REC10L.IMG", 320, 336}, {"UPR06.IMG", 584, 24}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			raw := make([]byte, 4+tc.w*tc.h/2)
			binary.LittleEndian.PutUint16(raw, uint16(tc.h))
			binary.LittleEndian.PutUint16(raw[2:], uint16(tc.w))
			nam := make([]byte, 16)
			base, ext := tc.name[:len(tc.name)-4], "IMG"
			copy(nam, base)
			copy(nam[9:], ext)
			idx := make([]byte, 4)
			binary.LittleEndian.PutUint32(idx, uint32(len(raw)))
			cont, e := assets.OpenContainer(nam, idx, raw)
			if e != nil {
				t.Fatal(e)
			}
			high := image.NewRGBA(image.Rect(0, 0, tc.w*4, tc.h*4))
			for i := 3; i < len(high.Pix); i += 4 {
				high.Pix[i] = 255
			}
			var b bytes.Buffer
			if e = png.Encode(&b, high); e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(dir, "art.png"), b.Bytes(), 0644); e != nil {
				t.Fatal(e)
			}
			item := HDEntry{Edition: "base", Container: "DATA2", Name: tc.name, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), File: "art.png", SHA256: fmt.Sprintf("%x", sha256.Sum256(b.Bytes())), Width: tc.w * 4, Height: tc.h * 4}
			hdManifest(t, dir, []HDEntry{item})
			p, e := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA2": cont})
			if e != nil || p.Count != 1 || len(p.Warnings) != 0 {
				t.Fatalf("valid: %v %+v", e, p)
			}
			item.Container = "DATA1"
			hdManifest(t, dir, []HDEntry{item})
			p, e = LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": cont})
			if e != nil || p.Count != 0 || len(p.Warnings) != 1 {
				t.Fatal("錯容器未回退")
			}
			item.Container = "DATA2"
			binary.LittleEndian.PutUint16(raw, uint16(tc.h-1))
			item.SourceSHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
			hdManifest(t, dir, []HDEntry{item})
			p, e = LoadHDPack(dir, "base", map[string]*assets.Container{"DATA2": cont})
			if e != nil || p.Count != 0 || len(p.Warnings) != 1 {
				t.Fatal("來源尺寸變動未回退")
			}
		})
	}
}
