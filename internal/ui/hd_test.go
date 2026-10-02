package ui

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

func hdFixture(t *testing.T) (string, *assets.Container, HDEntry, *assets.Image, *image.RGBA) {
	t.Helper()
	dir := t.TempDir()
	raw := make([]byte, 4+64*80/2)
	binary.LittleEndian.PutUint16(raw, 80)
	binary.LittleEndian.PutUint16(raw[2:], 64)
	raw[4] = 0x80 // 不對稱，才能驗證鏡像。
	nam := make([]byte, 16)
	copy(nam, "F000")
	copy(nam[9:], "FAC")
	idx := make([]byte, 4)
	binary.LittleEndian.PutUint32(idx, uint32(len(raw)))
	c, err := assets.OpenContainer(nam, idx, raw)
	if err != nil {
		t.Fatal(err)
	}
	im, err := assets.DecodeImage(raw)
	if err != nil {
		t.Fatal(err)
	}
	high := image.NewRGBA(image.Rect(0, 0, 256, 320))
	for y := 0; y < 320; y++ {
		for x := 0; x < 256; x++ {
			high.SetRGBA(x, y, color.RGBA{byte(x), byte(y % 256), 77, 255})
		}
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, high); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "face.png"), pngData.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	e := HDEntry{Edition: "base", Container: "DATA3", Name: "F000.FAC", File: "face.png", Width: 256, Height: 320,
		SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), SHA256: fmt.Sprintf("%x", sha256.Sum256(pngData.Bytes()))}
	return dir, c, e, im, high
}

func hdManifest(t *testing.T, dir string, entries []HDEntry) {
	t.Helper()
	b, err := json.Marshal(HDManifest{Schema: 1, Style: "b", Scale: 4, Entries: entries})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestHDPackValidationAndMirror(t *testing.T) {
	dir, container, e, im, high := hdFixture(t)
	hdManifest(t, dir, []HDEntry{e})
	p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA3": container})
	if err != nil || p.Count != 1 || len(p.Warnings) != 0 {
		t.Fatalf("pack=%+v err=%v", p, err)
	}
	if got := p.images[hdImageKey(im.Mirror())].RGBAAt(0, 0); got != high.RGBAAt(255, 0) {
		t.Fatalf("鏡像=%v", got)
	}
	for _, tc := range []struct {
		name   string
		change func(*HDEntry)
	}{
		{"source", func(e *HDEntry) { e.SourceSHA256 = "wrong" }},
		{"size", func(e *HDEntry) { e.Width = 320 }},
		{"png_hash", func(e *HDEntry) { e.SHA256 = "wrong" }},
		{"path", func(e *HDEntry) { e.File = "../face.png" }},
		{"resource", func(e *HDEntry) { e.Name = "F999.FAC" }},
		{"container", func(e *HDEntry) { e.Container = "DATA1" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := e
			tc.change(&bad)
			hdManifest(t, dir, []HDEntry{bad})
			p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA3": container})
			if err != nil || p.Count != 0 || len(p.Warnings) != 1 {
				t.Fatalf("錯圖沒有回退: %+v %v", p, err)
			}
		})
	}
	hdManifest(t, dir, []HDEntry{e, e})
	p, err = LoadHDPack(dir, "base", map[string]*assets.Container{"DATA3": container})
	if err != nil || p.Count != 0 || len(p.Warnings) != 2 {
		t.Fatalf("重複鍵: %+v %v", p, err)
	}
	hdManifest(t, dir, []HDEntry{e})
	p, err = LoadHDPack(dir, "plus", map[string]*assets.Container{"DATA3": container})
	if err != nil || p.Count != 0 {
		t.Fatalf("錯版本被使用: %+v %v", p, err)
	}
}

func TestHDPackSymmetricPortraitKeepsNormalOrientation(t *testing.T) {
	dir, container, e, _, high := hdFixture(t)
	raw := container.Data(0)
	raw[4] = 0
	e.SourceSHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
	im, err := assets.DecodeImage(raw)
	if err != nil {
		t.Fatal(err)
	}
	hdManifest(t, dir, []HDEntry{e})
	p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA3": container})
	if err != nil || p.Count != 1 {
		t.Fatalf("pack=%+v err=%v", p, err)
	}
	if got := p.images[hdImageKey(im)].RGBAAt(0, 0); got != high.RGBAAt(0, 0) {
		t.Fatalf("對稱來源把正常高清圖覆蓋為鏡像：%v", got)
	}
}

func TestHDKeepsDetailForegroundAndOriginal(t *testing.T) {
	dir, container, e, im, high := hdFixture(t)
	hdManifest(t, dir, []HDEntry{e})
	pack, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA3": container})
	if err != nil {
		t.Fatal(err)
	}
	c := NewCanvasPx(100, 100, nil)
	c.HD = pack
	c.Fill(bg)
	drawImageAt(c, im, 8, 8)
	before := append([]byte(nil), c.Img.Pix...)
	if c.Output(false) != c.Img {
		t.Fatal("原貌輸出被替換")
	}
	out := c.Output(true)
	if out.RGBAAt(32, 32) != high.RGBAAt(0, 0) || out.RGBAAt(33, 32) != high.RGBAAt(1, 0) {
		t.Fatal("高清細節先被降解析")
	}
	if !bytes.Equal(before, c.Img.Pix) {
		t.Fatal("HD 改寫原畫布")
	}
	c.FillRect(9, 9, 11, 11, fg)
	out = c.Output(true)
	for y := 36; y < 44; y++ {
		for x := 36; x < 44; x++ {
			if out.RGBAAt(x, y) != fg {
				t.Fatal("前景被 HD 蓋掉")
			}
		}
	}
	c.drawRGBA(c.Img.Bounds(), image.NewUniform(bg), image.Point{})
	if c.Output(true).RGBAAt(32, 32) != bg {
		t.Fatal("前頁高清素材殘留")
	}
}

func TestHDWipePreservesSourceAtEveryStep(t *testing.T) {
	scene := &assets.Image{W: 176, H: 96, Pix: make([]byte, 176*96)}
	high := image.NewRGBA(image.Rect(0, 0, 704, 384))
	for y := 0; y < 384; y++ {
		for x := 0; x < 704; x++ {
			high.SetRGBA(x, y, color.RGBA{byte(x % 256), byte(y % 256), 77, 255})
		}
	}
	for kind := WipeDown; kind <= WipeLeft; kind++ {
		c := NewCanvasPx(200, 110, nil)
		c.HD = &HDPack{images: map[[32]byte]*image.RGBA{hdImageKey(scene): high}, Count: 1}
		c.Fill(bg)
		w := NewSceneWipe(c, scene, kind, 8, 8)
		for n := 1; w.Advance(c.Img); n++ {
			out := c.Output(true)
			r := w.Reveal(n)
			s := w.Source(r)
			for _, p := range []image.Point{r.Min, r.Max.Sub(image.Pt(1, 1))} {
				src := s.Min.Add(p.Sub(r.Min)).Sub(image.Pt(8, 8)).Mul(4)
				if got := out.RGBAAt(p.X*4, p.Y*4); got != high.RGBAAt(src.X, src.Y) {
					t.Fatalf("kind=%d step=%d src=%v got=%v", kind, n, src, got)
				}
			}
			if out.RGBAAt(0, 0) != bg {
				t.Fatal("拉幕越界")
			}
		}
	}
}

func TestScene30And31UseDATA2(t *testing.T) {
	c1, c3 := artContainers(t)
	c2 := artContainer(t, "DATA2")
	a, err := NewArtScreen(c3, c1, c2)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{30, 31} {
		if im := a.Scene(n); im == nil || im.W != 176 || im.H != 96 {
			t.Fatalf("SCG%d 路由失敗", n)
		}
	}
}
