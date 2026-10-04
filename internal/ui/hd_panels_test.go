package ui

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func panelFixtureEntry(t *testing.T, dir, name string, source []byte, w, h int) HDEntry {
	t.Helper()
	high := image.NewRGBA(image.Rect(0, 0, w*4, h*4))
	for y := 0; y < h*4; y++ {
		for x := 0; x < w*4; x++ {
			high.SetRGBA(x, y, color.RGBA{byte(x), byte(y), 71, 255})
		}
	}
	var b bytes.Buffer
	if e := png.Encode(&b, high); e != nil {
		t.Fatal(e)
	}
	file := fmt.Sprintf("skin-%d-%d.png", w, h)
	if e := os.WriteFile(filepath.Join(dir, file), b.Bytes(), 0644); e != nil {
		t.Fatal(e)
	}
	return HDEntry{Edition: "base", Container: "DATA1", Name: name, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(source)), File: file, SHA256: fmt.Sprintf("%x", sha256.Sum256(b.Bytes())), Width: w * 4, Height: h * 4}
}

func TestHDPanelValidatesOriginalAndFallback(t *testing.T) {
	source := artContainer(t, "DATA1")
	dir := t.TempDir()
	im, raw, e := hdPanelSource(source, "PANEL.SIDE#A3")
	if e != nil {
		t.Fatal(e)
	}
	entry := panelFixtureEntry(t, dir, "PANEL.SIDE#A3", raw, im.W, im.H)
	for _, tc := range []struct {
		name  string
		edit  func(*HDEntry)
		count int
	}{
		{"valid", func(e *HDEntry) {}, 1}, {"wrong_source", func(e *HDEntry) { e.SourceSHA256 = "wrong" }, 0}, {"wrong_container", func(e *HDEntry) { e.Container = "DATA3" }, 0}, {"unknown_paper", func(e *HDEntry) { e.Name = "PANEL.SIDE#A7" }, 0}, {"old_short_canvas", func(e *HDEntry) { e.Height = 192 }, 0}, {"wrong_png", func(e *HDEntry) { e.SHA256 = "wrong" }, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := entry
			tc.edit(&bad)
			hdManifest(t, dir, []HDEntry{bad})
			p, e := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source, "DATA3": source})
			if e != nil || p.Count != tc.count || len(p.Warnings) != 1-tc.count {
				t.Fatalf("count=%d warnings=%v err=%v", p.Count, p.Warnings, e)
			}
		})
	}
	hdManifest(t, dir, []HDEntry{entry, entry})
	p, e := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
	if e != nil || p.Count != 0 || len(p.Warnings) != 2 {
		t.Fatal("duplicate", p, e)
	}
	// 原拼件仍可解碼，但改變尺寸後不能靠新雜湊冒充正常 SIDE 框。
	i, _ := source.ByName("SIDEA16.IMG")
	original := append([]byte(nil), source.Data(i)...)
	defer copy(source.Data(i), original)
	source.Data(i)[0] = 8
	if _, _, e = hdPanelSource(source, "PANEL.SIDE#A3"); e == nil {
		t.Fatal("malformed corner accepted")
	}
}

func TestHDPanelShortFramePreservesEveryCornerPixel(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 896, 1024))
	for y := 0; y < 1024; y++ {
		for x := 0; x < 896; x++ {
			src.SetRGBA(x, y, color.RGBA{byte(x), byte(y), byte(x + y), 255})
		}
	}
	out := hdSlicePanel(src, 224, 48, 16)
	for _, pt := range []image.Point{{0, 0}, {208, 0}, {0, 32}, {208, 32}} {
		sy := pt.Y * 4
		if pt.Y != 0 {
			sy = 960
		}
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				if out.RGBAAt(pt.X*4+x, pt.Y*4+y) != src.RGBAAt(pt.X*4+x, sy+y) {
					t.Fatal("corner rescaled", pt, x, y)
				}
			}
		}
	}
}

func TestHDPanelBorderOnlyKeepsNoiseAndText(t *testing.T) {
	source := artContainer(t, "DATA1")
	dir := t.TempDir()
	im, raw, e := hdPanelSource(source, "PANEL.SIDE#B3")
	if e != nil {
		t.Fatal(e)
	}
	entry := panelFixtureEntry(t, dir, "PANEL.SIDE#B3", raw, im.W, im.H)
	hdManifest(t, dir, []HDEntry{entry})
	p, e := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
	if e != nil {
		t.Fatal(e)
	}
	f, e := assets.LoadSideFrame(source, 'B')
	if e != nil {
		t.Fatal(e)
	}
	c := NewCanvasPx(640, 408, nil)
	c.HD = p
	for y := 0; y < 408; y++ {
		for x := 0; x < 640; x++ {
			c.Img.SetRGBA(x, y, color.RGBA{byte(x), byte(y), byte(x ^ y), 255})
		}
	}
	before := append([]byte(nil), c.Img.Pix...)
	c.drawHighSidePanel(f, 408, 36, 224, 288)
	c.FillRect(432, 100, 440, 108, color.RGBA{255, 22, 31, 255})
	out := c.Output(true)
	for y := 36; y < 324; y++ {
		for x := 408; x < 632; x++ {
			border := y < 44 || y >= 316 || x < 416 || x >= 624 || (x < 424 || x >= 616) && (y < 52 || y >= 308)
			for j := 0; j < 4; j++ {
				for i := 0; i < 4; i++ {
					if !border && out.RGBAAt(x*4+i, y*4+j) != c.Img.RGBAAt(x, y) {
						t.Fatal("original noise or text covered", x, y)
					}
				}
			}
		}
	}
	for y := 0; y < 408; y++ {
		for x := 0; x < 640; x++ {
			if image.Pt(x, y).In(image.Rect(432, 100, 440, 108)) {
				continue
			}
			off := c.Img.PixOffset(x, y)
			if !bytes.Equal(before[off:off+4], c.Img.Pix[off:off+4]) {
				t.Fatal("CPU canvas modified")
			}
		}
	}
	c.HD = &HDPack{}
	if c.highPanel(entry.Name, image.Rect(0, 0, 224, 256), false) != nil {
		t.Fatal("missing pack used")
	}
}

func TestHDPortraitFrameUsesDATA1WithoutMirror(t *testing.T) {
	source := artContainer(t, "DATA1")
	dir := t.TempDir()
	i, _ := source.ByName("FBRD0.IMG")
	raw := source.Data(i)
	entry := panelFixtureEntry(t, dir, "FBRD0.IMG", raw, 80, 8)
	hdManifest(t, dir, []HDEntry{entry})
	p, e := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
	if e != nil || p.Count != 1 || len(p.Warnings) != 0 {
		t.Fatal(p, e)
	}
	im, e := assets.DecodeImage(raw)
	if e != nil {
		t.Fatal(e)
	}
	high := p.images[hdImageKey(im)]
	if high == nil || high.RGBAAt(0, 0).R != 0 || high.RGBAAt(319, 0).R != 63 {
		t.Fatal("frame wrongly treated as portrait")
	}
	entry.Container = "DATA3"
	hdManifest(t, dir, []HDEntry{entry})
	p, e = LoadHDPack(dir, "base", map[string]*assets.Container{"DATA3": source})
	if e != nil || p.Count != 0 || len(p.Warnings) != 1 {
		t.Fatal("wrong source accepted")
	}
}

func TestHDPanelRejectsTransparency(t *testing.T) {
	source := artContainer(t, "DATA1")
	dir := t.TempDir()
	for _, name := range []string{"PANEL.BEVEL#1", "FBRD0.IMG"} {
		var raw []byte
		w, h := 80, 8
		if name == "PANEL.BEVEL#1" {
			im, b, e := hdPanelSource(source, name)
			if e != nil {
				t.Fatal(e)
			}
			raw, w, h = b, im.W, im.H
		} else {
			i, _ := source.ByName(name)
			raw = source.Data(i)
		}
		entry := panelFixtureEntry(t, dir, name, raw, w, h)
		high := image.NewRGBA(image.Rect(0, 0, w*4, h*4))
		draw.Draw(high, high.Bounds(), image.White, image.Point{}, draw.Src)
		high.SetRGBA(0, 0, color.RGBA{})
		var b bytes.Buffer
		png.Encode(&b, high)
		os.WriteFile(filepath.Join(dir, entry.File), b.Bytes(), 0644)
		entry.SHA256 = fmt.Sprintf("%x", sha256.Sum256(b.Bytes()))
		hdManifest(t, dir, []HDEntry{entry})
		p, e := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
		if e != nil || p.Count != 0 || len(p.Warnings) != 1 {
			t.Fatal("transparent panel/frame accepted", name, p, e)
		}
	}
}
