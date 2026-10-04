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
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

func hdMenuFixture(t *testing.T, name string, im *assets.Image, transparent bool) (string, *assets.Container, HDEntry, *image.RGBA) {
	t.Helper()
	stride := (im.W + 7) / 8
	raw := make([]byte, 4+stride*im.H*4)
	binary.LittleEndian.PutUint16(raw, uint16(im.H))
	binary.LittleEndian.PutUint16(raw[2:], uint16(im.W))
	for y := 0; y < im.H; y++ {
		for x := 0; x < im.W; x++ {
			for plane, bit := range []uint{3, 2, 1, 0} {
				if im.At(x, y)&(1<<bit) != 0 {
					raw[4+plane*stride*im.H+y*stride+x/8] |= 1 << uint(7-x%8)
				}
			}
		}
	}
	nam := make([]byte, 16)
	parts := strings.Split(name, ".")
	copy(nam, parts[0])
	copy(nam[9:], parts[1])
	idx := make([]byte, 4)
	binary.LittleEndian.PutUint32(idx, uint32(len(raw)))
	container, err := assets.OpenContainer(nam, idx, raw)
	if err != nil {
		t.Fatal(err)
	}
	high := image.NewRGBA(image.Rect(0, 0, im.W*4, im.H*4))
	for y := 0; y < im.H*4; y++ {
		for x := 0; x < im.W*4; x++ {
			high.SetRGBA(x, y, color.RGBA{byte(x), byte(y), 77, 255})
		}
	}
	if transparent {
		high.SetRGBA(5, 5, color.RGBA{})
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, high); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "menu.png"), encoded.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	entry := HDEntry{Edition: "base", Container: "DATA3", Name: name, File: "menu.png", Width: im.W * 4, Height: im.H * 4,
		SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), SHA256: fmt.Sprintf("%x", sha256.Sum256(encoded.Bytes()))}
	return dir, container, entry, high
}

func hdMenuSource(w, h int) *assets.Image {
	im := &assets.Image{W: w, H: h, Pix: make([]byte, w*h)}
	for i := range im.Pix {
		im.Pix[i] = 9
	}
	for y := 2; y < h-8; y++ {
		for x := 4; x < w-4; x++ {
			im.Set(x, y, 3)
		}
	}
	im.Set(4, 2, 15)
	im.Set(8, 7, 0) // 框內黑色線條須能重繪。
	for y := 8; y < h; y++ {
		for x := w - 4; x < w; x++ {
			im.Set(x, y, 0)
		}
	} // 框外投影須保持。
	return im
}

func TestHDMenuPackValidation(t *testing.T) {
	for _, size := range []struct {
		name string
		w, h int
	}{{"MENU1.IMG", 96, 151}, {"MENU2.IMG", 200, 46}, {"MENU3.IMG", 40, 41}} {
		t.Run(size.name, func(t *testing.T) {
			for _, test := range []string{"valid", "wrong-size", "wrong-color", "empty-frame", "transparent", "wrong-container", "wrong-source", "missing-png"} {
				t.Run(test, func(t *testing.T) {
					im := hdMenuSource(size.w, size.h)
					if test == "wrong-size" {
						im = hdMenuSource(size.w-1, size.h)
					}
					if test == "wrong-color" {
						im.Set(10, 10, 4)
					}
					if test == "empty-frame" {
						for i := range im.Pix {
							im.Pix[i] = 0
						}
					}
					dir, container, entry, _ := hdMenuFixture(t, size.name, im, test == "transparent")
					if test == "wrong-container" {
						entry.Container = "DATA1"
					}
					if test == "wrong-source" {
						entry.SourceSHA256 = "wrong"
					}
					if test == "missing-png" {
						if err := os.Remove(filepath.Join(dir, entry.File)); err != nil {
							t.Fatal(err)
						}
					}
					hdManifest(t, dir, []HDEntry{entry})
					pack, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA3": container, "DATA1": container})
					if err != nil {
						t.Fatal(err)
					}
					if test == "valid" {
						if pack.Count != 1 || len(pack.Warnings) != 0 {
							t.Fatalf("%+v", pack)
						}
					} else if pack.Count != 0 || len(pack.Warnings) != 1 {
						t.Fatalf("錯項未回退: %+v", pack)
					}
				})
			}
		})
	}
}

func TestHDMenuInnerFrameShadowAndLayerText(t *testing.T) {
	var title TitleScreen
	title.bg = &assets.Image{W: assets.ScreenW, H: assets.ScreenH, Pix: make([]byte, assets.ScreenW*assets.ScreenH)}
	for i := range title.bg.Pix {
		title.bg.Pix[i] = 9
	}
	pack := &HDPack{images: map[[32]byte]*image.RGBA{}}
	for i, size := range [][2]int{{96, 151}, {200, 46}, {40, 41}} {
		im := hdMenuSource(size[0], size[1])
		title.menuPieces[i] = im
		_, _, _, high := hdMenuFixture(t, fmt.Sprintf("MENU%d.IMG", i+1), im, false)
		pack.images[hdImageKey(im)] = high
	}
	pieces := []struct {
		im   *assets.Image
		x, y int
	}{{title.menuPieces[0], 56, 215}, {title.menuPieces[2], 576, 320}}
	for _, at := range assets.MenuButtons() {
		pieces = append(pieces, struct {
			im   *assets.Image
			x, y int
		}{title.menuPieces[1], at[0], at[1]})
	}
	for _, p := range pieces {
		for y := 0; y < p.im.H; y++ {
			for x := 0; x < p.im.W; x++ {
				title.bg.Set(p.x+x, p.y+y, p.im.At(x, y))
			}
		}
	}
	for frame := range title.frames {
		bg := &assets.Image{W: title.bg.W, H: title.bg.H, Pix: append([]byte(nil), title.bg.Pix...)}
		for y := assets.MenuOrnamentY; y < assets.MenuOrnamentY+16; y++ {
			for x := assets.MenuOrnamentX; x < assets.MenuOrnamentX+8; x++ {
				bg.Set(x, y, byte(frame+1))
			}
		}
		title.frames[frame] = bg
	}
	for frame := 0; frame < 6; frame++ {
		c := testCanvasPx(t, 640, 408)
		control := testCanvasPx(t, 640, 408)
		c.HD = pack
		DrawTitleLayer(c, &title, frame, "", 11, []string{"A"}, 0)
		DrawTitleLayer(control, &title, frame, "", 11, []string{"A"}, 0)
		if !bytes.Equal(c.Img.Pix, control.Img.Pix) {
			t.Fatal("原貌改變")
		}
		high := c.Output(true)
		button := assets.MenuButtons()[0]
		skin := pack.images[hdImageKey(title.menuPieces[1])]
		for _, p := range []image.Point{{8, 7}, {100, 18}} { // 黑色內框及清字區的空白位置。
			if got := high.RGBAAt((button[0]+p.X)*4, (button[1]+p.Y)*4); got != skin.RGBAAt(p.X*4, p.Y*4) {
				t.Fatalf("框內被舊黑線或清字底色蓋住: %v", p)
			}
		}
		for y := assets.MenuOrnamentY; y < assets.MenuOrnamentY+16; y++ {
			for x := assets.MenuOrnamentX; x < assets.MenuOrnamentX+8; x++ {
				if high.RGBAAt(x*4, y*4) != control.Img.RGBAAt(x, y) {
					t.Fatal("游標格被框材覆蓋")
				}
			}
		}
		for _, p := range []image.Point{{2, 3}, {199, 20}} {
			x, y := button[0]+p.X, button[1]+p.Y
			if high.RGBAAt(x*4, y*4) != control.Img.RGBAAt(x, y) {
				t.Fatal("框外背景或投影改變")
			}
		}
		textPixels := 0
		for y := button[1] + 9; y < button[1]+25; y++ {
			for x := button[0] + 16; x < button[0]+176; x++ {
				if control.Img.RGBAAt(x, y) == assets.EGAPalette[15] {
					textPixels++
					if high.RGBAAt(x*4, y*4) != assets.EGAPalette[15] {
						t.Fatal("文字被框材覆蓋")
					}
				}
			}
		}
		if textPixels == 0 {
			t.Fatal("未畫出文字")
		}
		if !bytes.Equal(c.Output(false).Pix, control.Img.Pix) {
			t.Fatal("原貌回切失敗")
		}
		delete(pack.images, hdImageKey(title.menuPieces[1]))
		DrawTitleLayer(c, &title, frame, "", 11, []string{"A"}, 0)
		high = c.Output(true)
		for y := 0; y < 46; y++ {
			for x := 0; x < 200; x++ {
				if high.RGBAAt((button[0]+x)*4, (button[1]+y)*4) != control.Img.RGBAAt(button[0]+x, button[1]+y) {
					t.Fatal("缺圖未完整回退")
				}
			}
		}
		_, _, _, skin = hdMenuFixture(t, "MENU2.IMG", title.menuPieces[1], false)
		pack.images[hdImageKey(title.menuPieces[1])] = skin
	}
}
