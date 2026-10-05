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

func TestHDCreditTransparentFigureRetainsSky(t *testing.T) {
	cr := fakeCredits(1)
	part := &assets.Image{W: 320, H: 336, Pix: make([]byte, 320*336)}
	cr.BackdropParts[0][0] = part
	ln := cr.Lines[0]
	ln.Pix[0] = 0
	back := image.NewRGBA(image.Rect(0, 0, 1280, 1344))
	for i := 0; i < len(back.Pix); i += 4 {
		back.Pix[i+1], back.Pix[i+3] = 255, 255
	}
	high := image.NewRGBA(image.Rect(0, 0, 256, 96))
	for i := 3; i < len(high.Pix); i += 4 {
		high.Pix[i] = 255
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			high.SetRGBA(x, y, color.RGBA{})
		}
	}
	high.SetRGBA(2, 0, color.RGBA{64, 0, 0, 128})
	c := NewCanvasPx(640, 408, nil)
	c.HD = &HDPack{images: map[[32]byte]*image.RGBA{hdImageKey(part): back, hdImageKey(ln): high}, creditFigures: map[[32]byte][]image.Rectangle{hdImageKey(ln): {image.Rect(0, 0, 1, 1)}}}
	DrawCredits(c, cr, 330)
	out := c.Output(true)
	if got := out.RGBAAt(1152, 312); got != (color.RGBA{0, 255, 0, 255}) {
		t.Fatalf("透明邊緣留下黑塊：%v", got)
	}
	if got := out.RGBAAt(1154, 312); got != (color.RGBA{64, 127, 0, 255}) {
		t.Fatalf("半透明五官未合成：%v", got)
	}
	if c.Img.RGBAAt(288, 78) != assets.EGAPalette[0] {
		t.Fatal("原貌被高清透明合成改寫")
	}
}

func TestHDCreditLoaderProtectsTextAndAllowsFigureAlpha(t *testing.T) {
	for _, tc := range []struct {
		name string
		w    int
		art  bool
	}{{"UPR00.IMG", 272, false}, {"UPR05.IMG", 64, true}} {
		t.Run(tc.name, func(t *testing.T) {
			raw := make([]byte, 4+tc.w*24/2)
			binary.LittleEndian.PutUint16(raw, 24)
			binary.LittleEndian.PutUint16(raw[2:], uint16(tc.w))
			nam := make([]byte, 16)
			copy(nam, tc.name[:5])
			copy(nam[9:], "IMG")
			idx := make([]byte, 4)
			binary.LittleEndian.PutUint32(idx, uint32(len(raw)))
			cont, e := assets.OpenContainer(nam, idx, raw)
			if e != nil {
				t.Fatal(e)
			}
			dir := t.TempDir()
			im := image.NewRGBA(image.Rect(0, 0, tc.w*4, 96))
			for i := 3; i < len(im.Pix); i += 4 {
				im.Pix[i] = 255
			}
			for _, change := range []string{"original", "transparent", "changed_colour"} {
				switch change {
				case "transparent":
					im.SetRGBA(0, 0, color.RGBA{})
				case "changed_colour":
					im.SetRGBA(0, 0, color.RGBA{1, 0, 0, 255})
				}
				var b bytes.Buffer
				if e := png.Encode(&b, im); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(filepath.Join(dir, "line.png"), b.Bytes(), 0644); e != nil {
					t.Fatal(e)
				}
				item := HDEntry{Edition: "base", Container: "DATA2", Name: tc.name, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), File: "line.png", SHA256: fmt.Sprintf("%x", sha256.Sum256(b.Bytes())), Width: tc.w * 4, Height: 96}
				hdManifest(t, dir, []HDEntry{item})
				p, e := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA2": cont})
				if e != nil {
					t.Fatal(e)
				}
				valid := change == "original" || tc.art
				if (p.Count == 1) != valid {
					t.Fatalf("%s: art=%v count=%d warnings=%v", change, tc.art, p.Count, p.Warnings)
				}
			}
		})
	}
}
