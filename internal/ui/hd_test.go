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
	for _, n := range []int{730, 731} {
		many := make([]HDEntry, n)
		for i := range many {
			many[i].Edition = "plus"
		}
		hdManifest(t, dir, many)
		_, err = LoadHDPack(dir, "base", nil)
		if (err == nil) != (n == 730) {
			t.Fatalf("manifest 上限 %d：%v", n, err)
		}
	}
}

func hdTerrainContainer(t *testing.T, raw []byte) *assets.Container {
	t.Helper()
	nam := make([]byte, 16)
	copy(nam, "EICON")
	copy(nam[9:], "GRP")
	idx := make([]byte, 4)
	binary.LittleEndian.PutUint32(idx, uint32(len(raw)))
	c, err := assets.OpenContainer(nam, idx, raw)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func hdTerrainFixture(t *testing.T) (string, *assets.Container, []HDEntry) {
	t.Helper()
	dir := t.TempDir()
	var raw []byte
	var entries []HDEntry
	for n := 0; n < assets.TileCount; n++ {
		record := make([]byte, assets.ImageHeader+assets.TileW/8*assets.TileH*4)
		binary.LittleEndian.PutUint16(record, assets.TileH)
		binary.LittleEndian.PutUint16(record[2:], assets.TileW)
		record[4], record[5] = byte(n+1), byte(127-n)
		raw = append(raw, record...)
		if n > assets.SkirmishMaxTerrain {
			continue
		}
		hi := hdTerrainDetail(n)
		var buf bytes.Buffer
		if err := png.Encode(&buf, hi); err != nil {
			t.Fatal(err)
		}
		file := fmt.Sprintf("EICON%02d.png", n)
		if err := os.WriteFile(filepath.Join(dir, file), buf.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, HDEntry{Edition: "base", Container: "DATA1", Name: fmt.Sprintf("EICON.GRP#%02d", n), File: file,
			Width: 192, Height: 128, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(record)), SHA256: fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))})
	}
	return dir, hdTerrainContainer(t, raw), entries
}

func hdTerrainDetail(n int) *image.RGBA {
	hi := image.NewRGBA(image.Rect(0, 0, 192, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 192; x++ {
			hi.SetRGBA(x, y, color.RGBA{byte(x), byte(y), byte(30 + n), 255})
		}
	}
	return hi
}

func TestHDTerrainRecordValidation(t *testing.T) {
	dir, source, entries := hdTerrainFixture(t)
	hdManifest(t, dir, entries)
	p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
	if err != nil || p.Count != 15 || len(p.Warnings) != 0 {
		t.Fatalf("地形來源：%+v %v", p, err)
	}
	tiles, err := assets.BattleTiles(source)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 15; n++ {
		if p.images[hdImageKey(tiles[n])].RGBAAt(1, 0) != hdTerrainDetail(n).RGBAAt(1, 0) {
			t.Fatalf("子記錄選錯 %d", n)
		}
	}
	for _, tc := range []struct {
		name string
		edit func(*HDEntry)
	}{
		{"unknown_15", func(e *HDEntry) { e.Name = "EICON.GRP#15" }},
		{"bad_fragment", func(e *HDEntry) { e.Name = "EICON.GRP#1" }},
		{"wrong_container", func(e *HDEntry) { e.Container = "DATA3" }},
		{"parent_hash", func(e *HDEntry) { e.SourceSHA256 = fmt.Sprintf("%x", sha256.Sum256(source.Data(0))) }},
		{"wrong_record", func(e *HDEntry) { e.SourceSHA256 = entries[2].SourceSHA256 }},
		{"high_shape", func(e *HDEntry) { e.Height = 192 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := append([]HDEntry(nil), entries...)
			tc.edit(&bad[1])
			hdManifest(t, dir, bad)
			p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source, "DATA3": source})
			if err != nil || p.Count != 14 || len(p.Warnings) != 1 {
				t.Fatalf("逐格回退：%+v %v", p, err)
			}
		})
	}
	for _, tc := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{"35_records", func(b []byte) []byte { return b[:len(b)-772] }},
		{"37_records", func(b []byte) []byte { return append(b, b[:772]...) }},
		{"wrong_other_record_shape", func(b []byte) []byte { b[35*772] = 31; return b }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := hdTerrainContainer(t, tc.edit(append([]byte(nil), source.Data(0)...)))
			hdManifest(t, dir, entries)
			p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": bad})
			if err != nil || p.Count != 0 || len(p.Warnings) != 15 {
				t.Fatalf("實際 archive 形狀：%+v %v", p, err)
			}
		})
	}
	hdManifest(t, dir, entries)
	for _, containers := range []map[string]*assets.Container{nil, {"DATA3": source}} {
		p, err = LoadHDPack(dir, "base", containers)
		if err != nil || p.Count != 0 || len(p.Warnings) != 15 {
			t.Fatalf("缺來源：%+v %v", p, err)
		}
	}
	p, err = LoadHDPack(dir, "plus", map[string]*assets.Container{"DATA1": source})
	if err != nil || p.Count != 0 || len(p.Warnings) != 0 {
		t.Fatalf("版本隔離：%+v %v", p, err)
	}
	hdManifest(t, dir, append(entries, entries[0]))
	p, err = LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
	if err != nil || p.Count != 14 || len(p.Warnings) != 2 {
		t.Fatalf("重複記錄：%+v %v", p, err)
	}
	transparent := hdTerrainDetail(0)
	transparent.SetRGBA(1, 0, color.RGBA{})
	var buf bytes.Buffer
	if err := png.Encode(&buf, transparent); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "transparent.png"), buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	bad := entries[0]
	bad.File, bad.SHA256 = "transparent.png", fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))
	hdManifest(t, dir, []HDEntry{bad})
	p, err = LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
	if err != nil || p.Count != 0 || len(p.Warnings) != 1 {
		t.Fatalf("透明地形：%+v %v", p, err)
	}
}

func TestHDTerrainBattleForegroundAndChildFallback(t *testing.T) {
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(artContainer(t, "DATA2"), state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int{25, 26} {
		pref, err := sc.Prefecture(id)
		if err != nil {
			t.Fatal(err)
		}
		fld, err := battle.Load(pref.BattleField, pref.Neighbours)
		if err != nil {
			t.Fatal(err)
		}
		for _, child := range []bool{false, true} {
			for _, blink := range []bool{false, true} {
				t.Run(fmt.Sprintf("pref%d-child%v-blink%v", id, child, blink), func(t *testing.T) {
					pack := &HDPack{images: map[[32]byte]*image.RGBA{}}
					for n := 0; n < 15; n++ {
						pack.images[hdImageKey(ab.tiles[n])] = hdTerrainDetail(n)
					}
					b := battle.New(battle.Setup{Field: fld, Seed: 1})
					u := &battle.Unit{Side: battle.MainAttacker, Leaders: []battle.Leader{{Name: "測試", Soldiers: 1000}}, At: battle.FromOffset(2, 2)}
					b.Units = []*battle.Unit{u}
					field := append([]byte(nil), pref.BattleField...)
					for _, at := range [][2]int{{2, 0}, {3, 1}, {2, 2}, {2, 4}} {
						field[at[1]*12+at[0]] = 7
					}
					v := BattleView{Acting: u, Blink: blink}
					info := ArtBattleInfo{Field: field, Portrait: [2]int{-1, -1}}
					if child {
						s := &battle.Skirmish{Hour: battle.SkirmishFirstHour}
						for r := range s.Map {
							for col := range s.Map[r] {
								s.Map[r][col] = 15
							}
						}
						s.Map[2][2], s.Map[4][2] = 7, 13
						g := &battle.SkirmishGeneral{Leader: &u.Leaders[0], Unit: u, Side: battle.SkirmishAttacker, Col: 2, Row: 2}
						s.Gens[1][0], v.SkirmishActing, info.Skirmish = g, g, s
					}
					plain, c := testCanvasPx(t, 640, 408), testCanvasPx(t, 640, 408)
					DrawArtBattle(plain, ab, b, v, info)
					c.HD = pack
					DrawArtBattle(c, ab, b, v, info)
					if !bytes.Equal(c.Img.Pix, plain.Img.Pix) || c.Output(false) != c.Img {
						t.Fatal("原貌 CPU 改動")
					}
					out := c.Output(true)
					same := func(r image.Rectangle) {
						t.Helper()
						for y := r.Min.Y * 4; y < r.Max.Y*4; y++ {
							for x := r.Min.X * 4; x < r.Max.X*4; x++ {
								if out.RGBAAt(x, y) != plain.Img.RGBAAt(x/4, y/4) {
									t.Fatalf("前景被地形蓋掉 %d,%d", x, y)
								}
							}
						}
					}
					fx, fy := assets.FlagCell(2, 2)
					if child {
						x, y := assets.FieldCell(2, 2)
						same(image.Rect(x, y, x+48, y+31))
						if blink {
							same(image.Rect(x, y, x+48, y+32))
						}
					} else {
						same(image.Rect(fx, fy, fx+ab.flags[u.Side.OriginalIndex()][u.Formation.OriginalIndex()].W, fy+15))
						same(image.Rect(fx, fy+15, fx+40, fy+32))
					}
					l := assets.BattleLayoutFor(fld.Narrow())
					for i := range l.PanelX {
						x0, y0, x1, y1 := l.Panel(i)
						same(image.Rect(x0, y0, x1+1, y1+1))
						// 下凹框的上方第一條黑線，含參差端點。
						same(image.Rect(x0-2, y0-2, x1+3, y0-1))
					}
					for _, at := range [][3]int{{2, 0, 7}, {3, 1, 7}, {2, 4, 7}} {
						n := at[2]
						if child && at[1] == 4 {
							n = 13
						}
						x, y := assets.FieldCell(at[0], at[1])
						if out.RGBAAt(x*4+1, y*4+1) != hdTerrainDetail(n).RGBAAt(1, 1) {
							t.Fatal("原生細節、子圖或地形 15 跳過")
						}
					}
					if child {
						delete(pack.images, hdImageKey(ab.tiles[13]))
						DrawArtBattle(c, ab, b, v, info)
						out = c.Output(true)
						x, y := assets.FieldCell(2, 4)
						same(image.Rect(x, y, x+48, y+32))
					}
					c.FillRect(153, 42, 157, 45, fg)
					if c.Output(true).RGBAAt(612, 168) != fg {
						t.Fatal("後畫文字／面板失去覆蓋權")
					}
				})
			}
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

func TestHDTerrainAtlasLabelsAndFortCursor(t *testing.T) {
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	if ab.mapCursor == nil {
		t.Fatal("缺少關寨游標")
	}
	sc, err := state.LoadScenario(artContainer(t, "DATA2"), state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	pack := &HDPack{images: map[[32]byte]*image.RGBA{}}
	for n := 0; n < 15; n++ {
		pack.images[hdImageKey(ab.tiles[n])] = hdTerrainDetail(n)
	}
	for _, id := range []int{25, 26} {
		pref, err := sc.Prefecture(id)
		if err != nil {
			t.Fatal(err)
		}
		fld, err := battle.Load(pref.BattleField, pref.Neighbours)
		if err != nil {
			t.Fatal(err)
		}
		field := append([]byte(nil), pref.BattleField...)
		field[2] = 7
		for _, marked := range []bool{false, true, false} {
			plain, c := testCanvasPx(t, 640, 408), testCanvasPx(t, 640, 408)
			c.HD = pack
			s := FortSpot{Col: 2, Row: 0, Marked: marked, Confirm: true}
			DrawArtFortSpot(plain, ab, field, fld, s)
			DrawArtFortSpot(c, ab, field, fld, s)
			if !bytes.Equal(c.Img.Pix, plain.Img.Pix) {
				t.Fatal("地理誌或關寨 CPU 改動")
			}
			out := c.Output(true)
			x0, y0 := assets.FieldCell(2, 0)
			for y := 0; y < 32; y++ {
				for x := 0; x < 48; x++ {
					if marked && ab.mapCursor.Pix[y*48+x]&15 != 0 {
						for sy := 0; sy < 4; sy++ {
							for sx := 0; sx < 4; sx++ {
								if out.RGBAAt((x0+x)*4+sx, (y0+y)*4+sy) != plain.Img.RGBAAt(x0+x, y0+y) {
									t.Fatal("游標 XOR 像素被高清蓋掉")
								}
							}
						}
					}
				}
			}
			if !marked && out.RGBAAt(x0*4+1, y0*4+1) != hdTerrainDetail(7).RGBAAt(1, 1) {
				t.Fatal("游標隱藏後未恢復高清")
			}
		}
		plain, c := testCanvasPx(t, 640, 408), testCanvasPx(t, 640, 408)
		c.HD = pack
		DrawArtAtlas(plain, ab, field, fld)
		DrawArtAtlas(c, ab, field, fld)
		out := c.Output(true)
		for n, gates := range fld.Gates {
			for _, h := range gates {
				col, row := battle.ToOffset(h)
				x, y := assets.FieldCell(col, row)
				x, y = x+atlasLabelDX, y+atlasLabelDY
				for yy := y * 4; yy < (y+CellH)*4; yy++ {
					for xx := x * 4; xx < (x+len(fmt.Sprint(n))*CellW)*4; xx++ {
						if out.RGBAAt(xx, yy) != plain.Img.RGBAAt(xx/4, yy/4) {
							t.Fatal("鄰郡標籤被高清蓋掉")
						}
					}
				}
			}
		}
	}
}

func TestHDLureRecordValidation(t *testing.T) {
	dir, source, _ := hdTerrainFixture(t)
	const stride = assets.ImageHeader + assets.TileW/8*assets.TileH*4
	var entries []HDEntry
	for n := 32; n < 36; n++ {
		var buf bytes.Buffer
		if err := png.Encode(&buf, hdTerrainDetail(n)); err != nil {
			t.Fatal(err)
		}
		file := fmt.Sprintf("EICON%02d.png", n)
		if err := os.WriteFile(filepath.Join(dir, file), buf.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, HDEntry{Edition: "base", Container: "DATA1", Name: fmt.Sprintf("EICON.GRP#%02d", n), File: file, Width: 192, Height: 128,
			SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(source.Data(0)[n*stride:(n+1)*stride])), SHA256: fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))})
	}
	tiles, err := assets.BattleTiles(source)
	if err != nil {
		t.Fatal(err)
	}
	both := append([]HDEntry(nil), entries...)
	for _, e := range entries {
		e.Edition = "plus"
		both = append(both, e)
	}
	hdManifest(t, dir, both)
	for _, edition := range []string{"base", "plus"} {
		p, err := LoadHDPack(dir, edition, map[string]*assets.Container{"DATA1": source})
		if err != nil || p.Count != 4 || len(p.Warnings) != 0 {
			t.Fatalf("兩版誘敵載入：%+v %v", p, err)
		}
		for n := 32; n < 36; n++ {
			if !bytes.Equal(p.images[hdImageKey(tiles[n])].Pix, hdTerrainDetail(n).Pix) {
				t.Fatal("來源子記錄或幀索引選錯")
			}
		}
	}
	for _, tc := range []struct {
		name string
		edit func(*HDEntry)
	}{
		{"unreviewed_15", func(e *HDEntry) { e.Name = "EICON.GRP#15" }},
		{"unreviewed_31", func(e *HDEntry) { e.Name = "EICON.GRP#31" }},
		{"out_of_archive", func(e *HDEntry) { e.Name = "EICON.GRP#36" }},
		{"wrong_record_hash", func(e *HDEntry) { e.SourceSHA256 = entries[1].SourceSHA256 }},
		{"parent_hash", func(e *HDEntry) { e.SourceSHA256 = fmt.Sprintf("%x", sha256.Sum256(source.Data(0))) }},
		{"wrong_container", func(e *HDEntry) { e.Container = "DATA3" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := append([]HDEntry(nil), entries...)
			tc.edit(&bad[0])
			hdManifest(t, dir, bad)
			p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source, "DATA3": source})
			if err != nil || p.Count != 3 || len(p.Warnings) != 1 {
				t.Fatalf("逐幀回退：%+v %v", p, err)
			}
		})
	}
	badRaw := append([]byte(nil), source.Data(0)...)
	badRaw[31*stride] = 31
	hdManifest(t, dir, entries)
	p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": hdTerrainContainer(t, badRaw)})
	if err != nil || p.Count != 0 || len(p.Warnings) != 4 {
		t.Fatalf("其他記錄的錯形也須拒絕：%+v %v", p, err)
	}
	transparent := hdTerrainDetail(32)
	transparent.SetRGBA(1, 1, color.RGBA{})
	var buf bytes.Buffer
	if err := png.Encode(&buf, transparent); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "transparent-lure.png"), buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	bad := entries[0]
	bad.File = "transparent-lure.png"
	bad.SHA256 = fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))
	hdManifest(t, dir, []HDEntry{bad})
	p, err = LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
	if err != nil || p.Count != 0 || len(p.Warnings) != 1 {
		t.Fatalf("誘敵整塊必須不透明：%+v %v", p, err)
	}
}

func TestHDLureAnimationLayers(t *testing.T) {
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(artContainer(t, "DATA2"), state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	pref, err := sc.Prefecture(25)
	if err != nil {
		t.Fatal(err)
	}
	fld, err := battle.Load(pref.BattleField, pref.Neighbours)
	if err != nil {
		t.Fatal(err)
	}
	b := battle.New(battle.Setup{Field: fld, Seed: 1})
	info := ArtBattleInfo{Field: pref.BattleField, Portrait: [2]int{-1, -1}}
	wantTiles := []int{32, 33, 34, 35, 34, 35, 34, 35, 34, 35, 34, 35, 34, 35, 34, 35, 34, 35, 34, 35, 34, 35}
	wantSpeeds := []int{440, 440, 300, 252, 300, 252, 300, 252, 300, 252, 300, 252, 300, 252, 300, 252, 300, 252, 300, 252, 300, 252}
	steps := LureFlashSteps()
	if len(steps) != 22 {
		t.Fatal("22 步")
	}
	checks := 0
	for _, at := range []battle.Hex{battle.FromOffset(2, 2), battle.FromOffset(3, 2), battle.FromOffset(-2, 0), battle.FromOffset(12, 0)} {
		plain, c := testCanvasPx(t, 640, 408), testCanvasPx(t, 640, 408)
		pack := &HDPack{images: map[[32]byte]*image.RGBA{}}
		for n := 0; n < 15; n++ {
			pack.images[hdImageKey(ab.tiles[n])] = hdTerrainDetail(n)
		}
		for n := 32; n < 36; n++ {
			pack.images[hdImageKey(ab.tiles[n])] = hdTerrainDetail(n)
		}
		c.HD = pack
		DrawArtBattle(plain, ab, b, BattleView{}, info)
		DrawArtBattle(c, ab, b, BattleView{}, info)
		before := append([]byte(nil), c.Output(true).Pix...)
		x, y, w, h := LureFlashCell(at)
		rect := image.Rect(x*4, y*4, (x+w)*4, (y+h)*4)
		for k, step := range steps {
			if step.Tile != wantTiles[k] || step.Speed != wantSpeeds[k] {
				t.Fatal("來源步序／速度")
			}
			DrawLureFlash(plain, ab, at, step.Tile)
			DrawLureFlash(c, ab, at, step.Tile)
			if !bytes.Equal(c.Img.Pix, plain.Img.Pix) || c.Output(false) != c.Img {
				t.Fatal("原貌畫布")
			}
			out := c.Output(true)
			high := hdTerrainDetail(step.Tile)
			for yy := 0; yy < out.Rect.Dy(); yy++ {
				for xx := 0; xx < out.Rect.Dx(); xx++ {
					if image.Pt(xx, yy).In(rect) {
						if out.RGBAAt(xx, yy) != high.RGBAAt(xx-x*4, yy-y*4) {
							t.Fatalf("原生圖或前幀殘留：%d", k)
						}
					} else {
						o := out.PixOffset(xx, yy)
						if !bytes.Equal(out.Pix[o:o+4], before[o:o+4]) {
							t.Fatal("矩形外像素")
						}
					}
				}
			}
			checks++
		}
		DrawArtBattle(c, ab, b, BattleView{}, info)
		if !bytes.Equal(c.Output(true).Pix, before) {
			t.Fatal("完成後未恢復地形／旗幟")
		}
		checks++
		// 缺任一相位時仍由整塊原圖蓋掉先前高清，不能殘留上一幀。
		for missing := 32; missing < 36; missing++ {
			DrawArtBattle(plain, ab, b, BattleView{}, info)
			DrawArtBattle(c, ab, b, BattleView{}, info)
			DrawLureFlash(c, ab, at, 35)
			saved := pack.images[hdImageKey(ab.tiles[missing])]
			delete(pack.images, hdImageKey(ab.tiles[missing]))
			DrawLureFlash(plain, ab, at, missing)
			DrawLureFlash(c, ab, at, missing)
			out := c.Output(true)
			for yy := rect.Min.Y; yy < rect.Max.Y; yy++ {
				for xx := rect.Min.X; xx < rect.Max.X; xx++ {
					if image.Pt(xx, yy).In(out.Rect) && out.RGBAAt(xx, yy) != plain.Img.RGBAAt(xx/4, yy/4) {
						t.Fatal("缺圖回退")
					}
				}
			}
			pack.images[hdImageKey(ab.tiles[missing])] = saved
			checks++
		}
	}
	t.Logf("誘敵圖層：%d 項檢查", checks)
}
