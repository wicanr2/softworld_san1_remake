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
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
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

func hdWeatherFixture(t *testing.T) (string, *assets.Container, []HDEntry) {
	t.Helper()
	dir := t.TempDir()
	var nam, idx, grp []byte
	var entries []HDEntry
	for n := 0; n < 3; n++ {
		raw := make([]byte, 4+32*32/2)
		binary.LittleEndian.PutUint16(raw, 32)
		binary.LittleEndian.PutUint16(raw[2:], 32)
		raw[4] = byte(1 << n)
		name := fmt.Sprintf("WEATHER%d", n)
		nm := make([]byte, 16)
		copy(nm, name)
		copy(nm[9:], "IMG")
		nam = append(nam, nm...)
		grp = append(grp, raw...)
		end := make([]byte, 4)
		binary.LittleEndian.PutUint32(end, uint32(len(grp)))
		idx = append(idx, end...)
		high := image.NewRGBA(image.Rect(0, 0, 128, 128))
		for y := 0; y < 128; y++ {
			for x := 0; x < 128; x++ {
				high.SetRGBA(x, y, color.RGBA{byte(x), byte(y), byte(60 + n), 255})
			}
		}
		var out bytes.Buffer
		if err := png.Encode(&out, high); err != nil {
			t.Fatal(err)
		}
		file := name + ".png"
		if err := os.WriteFile(filepath.Join(dir, file), out.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, HDEntry{Edition: "base", Container: "DATA1", Name: name + ".IMG", File: file,
			Width: 128, Height: 128, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), SHA256: fmt.Sprintf("%x", sha256.Sum256(out.Bytes()))})
	}
	c, err := assets.OpenContainer(nam, idx, grp)
	if err != nil {
		t.Fatal(err)
	}
	return dir, c, entries
}

func TestHDWeatherValidationAndFallback(t *testing.T) {
	dir, source, entries := hdWeatherFixture(t)
	hdManifest(t, dir, entries)
	pack, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
	if err != nil || pack.Count != 3 || len(pack.Warnings) != 0 {
		t.Fatalf("三種天候載入：%+v %v", pack, err)
	}
	for _, tc := range []struct {
		name    string
		change  func(*HDEntry)
		missing bool
	}{
		{"missing_container", func(e *HDEntry) {}, true},
		{"wrong_container", func(e *HDEntry) { e.Container = "DATA3" }, false},
		{"unknown_key", func(e *HDEntry) { e.Name = "WEATHER3.IMG" }, false},
		{"source_hash", func(e *HDEntry) { e.SourceSHA256 = "wrong" }, false},
		{"size", func(e *HDEntry) { e.Width = 256 }, false},
		{"png_hash", func(e *HDEntry) { e.SHA256 = "wrong" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := append([]HDEntry(nil), entries...)
			tc.change(&bad[1])
			hdManifest(t, dir, bad)
			containers := map[string]*assets.Container{"DATA1": source, "DATA3": source}
			want, warnings := 2, 1
			if tc.missing {
				delete(containers, "DATA1")
				want, warnings = 0, 3
			}
			p, err := LoadHDPack(dir, "base", containers)
			if err != nil || p.Count != want || len(p.Warnings) != warnings {
				t.Fatalf("逐項回退：%+v %v", p, err)
			}
		})
	}
	hdManifest(t, dir, append(entries, entries[0]))
	p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
	if err != nil || p.Count != 2 || len(p.Warnings) != 2 {
		t.Fatalf("重複天候：%+v %v", p, err)
	}
	hdManifest(t, dir, entries)
	p, err = LoadHDPack(dir, "plus", map[string]*assets.Container{"DATA1": source})
	if err != nil || p.Count != 0 || len(p.Warnings) != 0 {
		t.Fatalf("隔離版本：%+v %v", p, err)
	}
	// 原圖仍能解碼，但非 32×32 的來源不能註冊為天候。
	raw := source.Data(0)
	binary.LittleEndian.PutUint16(raw, 16)
	bad := entries[0]
	bad.SourceSHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
	hdManifest(t, dir, []HDEntry{bad})
	p, err = LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
	if err != nil || p.Count != 0 || len(p.Warnings) != 1 {
		t.Fatalf("來源尺寸：%+v %v", p, err)
	}
	for _, n := range []int{580, 581} {
		many := make([]HDEntry, n)
		for i := range many {
			many[i].Edition = "plus"
		}
		hdManifest(t, dir, many)
		_, err = LoadHDPack(dir, "base", nil)
		if (err == nil) != (n == 580) {
			t.Fatalf("manifest 上限 %d：%v", n, err)
		}
	}
}

func TestHDWeatherBattleLayers(t *testing.T) {
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(artContainer(t, "DATA2"), state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	pack := &HDPack{images: map[[32]byte]*image.RGBA{}}
	for n, im := range ab.weather {
		if im == nil {
			t.Fatalf("缺天候 %d", n)
		}
		high := image.NewRGBA(image.Rect(0, 0, 128, 128))
		for y := 0; y < 128; y++ {
			for x := 0; x < 128; x++ {
				high.SetRGBA(x, y, color.RGBA{byte(x), byte(y), byte(60 + n), 255})
			}
		}
		pack.images[hdImageKey(im)] = high
	}
	for _, prefID := range []int{25, 26} {
		pref, err := sc.Prefecture(prefID)
		if err != nil {
			t.Fatal(err)
		}
		field, err := battle.Load(pref.BattleField, pref.Neighbours)
		if err != nil {
			t.Fatal(err)
		}
		for _, weather := range []battle.Weather{battle.Clear, battle.Rainy, battle.Windy} {
			for _, child := range []bool{false, true} {
				t.Run(fmt.Sprintf("pref%d-weather%d-child%v", prefID, weather, child), func(t *testing.T) {
					b := battle.New(battle.Setup{Field: field, Weather: weather, FixedWeather: true, Seed: 1})
					info := ArtBattleInfo{Field: pref.BattleField, Portrait: [2]int{-1, -1}}
					if child {
						info.Skirmish = &battle.Skirmish{Hour: battle.SkirmishFirstHour}
					}
					plain := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
					DrawArtBattle(plain, ab, b, BattleView{}, info)
					c := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
					c.HD = pack
					DrawArtBattle(c, ab, b, BattleView{}, info)
					if !bytes.Equal(c.Img.Pix, plain.Img.Pix) || c.Output(false) != c.Img {
						t.Fatal("原貌被改寫")
					}
					out := c.Output(true)
					high := pack.images[hdImageKey(ab.weather[weather.OriginalIndex()])]
					for y := 0; y < 128; y++ {
						for x := 0; x < 128; x++ {
							if out.RGBAAt(32+x, 620+y) != high.RGBAAt(x, y) {
								t.Fatalf("天候索引或原生細節 %d,%d", x, y)
							}
						}
					}
					for y := 0; y < out.Bounds().Dy(); y++ {
						for x := 0; x < out.Bounds().Dx(); x++ {
							if x >= 32 && x < 160 && y >= 620 && y < 748 {
								continue
							}
							if out.RGBAAt(x, y) != plain.Img.RGBAAt(x/4, y/4) {
								t.Fatalf("越界 %d,%d", x, y)
							}
						}
					}
					c.FillRect(10, 160, 12, 162, fg)
					if c.Output(true).RGBAAt(40, 640) != fg {
						t.Fatal("前景被天候蓋住")
					}
					c.HD = &HDPack{images: map[[32]byte]*image.RGBA{}}
					DrawArtBattle(c, ab, b, BattleView{}, info)
					if c.Output(true).RGBAAt(33, 620) != plain.Img.RGBAAt(8, 155) {
						t.Fatal("缺圖回退或舊圖殘留")
					}
				})
			}
		}
	}
}
