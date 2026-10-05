package ui

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestHDTitlePreservesSourceInk(t *testing.T) {
	for _, name := range []string{"MENU0A.IMG", "MENU0B.IMG"} {
		for _, edition := range []string{"base", "plus"} {
			for _, check := range []string{"valid", "modified-ink", "modified-decoration", "transparent", "no-glyph", "outside-glyph", "wrong-container", "missing-png"} {
				t.Run(name+"/"+edition+"/"+check, func(t *testing.T) {
					w := 280
					if name == "MENU0B.IMG" {
						w = 288
					}
					im := &assets.Image{W: w, H: 180, Pix: make([]byte, w*180)}
					for i := range im.Pix {
						im.Pix[i] = 14
					}
					im.Set(64, 48, 13)
					im.Set(66, 48, 13)
					im.Set(63, 48, 15)
					im.Set(65, 48, 3)
					im.Set(67, 49, 0)
					if check == "no-glyph" {
						im.Set(64, 48, 14)
						im.Set(66, 48, 14)
					}
					if check == "outside-glyph" {
						im.Set(1, 1, 13)
					}
					dir, container, e, high := hdMenuFixture(t, name, im, check == "transparent")
					e.Edition = edition
					expected := map[image.Point]color.RGBA{
						{64, 48}: {238, 242, 226, 255}, {66, 48}: {238, 242, 226, 255},
						{63, 48}: {255, 255, 242, 255}, {65, 48}: {238, 242, 226, 255}, {67, 49}: {18, 36, 38, 255},
					}
					for at, c := range expected {
						for y := 0; y < 4; y++ {
							for x := 0; x < 4; x++ {
								high.SetRGBA(at.X*4+x, at.Y*4+y, c)
							}
						}
					}
					if check == "modified-ink" {
						high.SetRGBA(64*4, 48*4, color.RGBA{1, 2, 3, 255})
					}
					if check == "modified-decoration" {
						high.SetRGBA(40, 40, color.RGBA{1, 2, 3, 255})
					}
					if check == "wrong-container" {
						e.Container = "DATA1"
					}
					var data bytes.Buffer
					if err := png.Encode(&data, high); err != nil {
						t.Fatal(err)
					}
					e.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data.Bytes()))
					if err := os.WriteFile(filepath.Join(dir, e.File), data.Bytes(), 0644); err != nil {
						t.Fatal(err)
					}
					if check == "missing-png" {
						if err := os.Remove(filepath.Join(dir, e.File)); err != nil {
							t.Fatal(err)
						}
					}
					hdManifest(t, dir, []HDEntry{e})
					pack, err := LoadHDPack(dir, edition, map[string]*assets.Container{"DATA3": container, "DATA1": container})
					if err != nil {
						t.Fatal(err)
					}
					valid := check == "valid" || check == "modified-decoration"
					if valid {
						if pack.Count != 1 || len(pack.Warnings) != 0 {
							t.Fatalf("valid title rejected: %+v", pack)
						}
					} else if pack.Count != 0 || len(pack.Warnings) != 1 {
						t.Fatalf("invalid title accepted: count=%d warnings=%v", pack.Count, pack.Warnings)
					}
				})
			}
		}
	}
}
