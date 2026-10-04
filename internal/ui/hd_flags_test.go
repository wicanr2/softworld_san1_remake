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
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func hdFlagFixture(t *testing.T) (string, *assets.Container, HDEntry, *assets.Image, *image.RGBA) {
	t.Helper()
	dir := t.TempDir()
	raw := make([]byte, 4+184-4)
	binary.LittleEndian.PutUint16(raw, 15)
	binary.LittleEndian.PutUint16(raw[2:], 24)
	raw[4+45] = 0x80
	nam := make([]byte, 16)
	copy(nam, "WFLAGA00")
	copy(nam[9:], "IMG")
	idx := make([]byte, 4)
	binary.LittleEndian.PutUint32(idx, uint32(len(raw)))
	source, err := assets.OpenContainer(nam, idx, raw)
	if err != nil {
		t.Fatal(err)
	}
	im, err := assets.DecodeImage(raw)
	if err != nil {
		t.Fatal(err)
	}
	high := image.NewRGBA(image.Rect(0, 0, 96, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 96; x++ {
			high.SetRGBA(x, y, color.RGBA{byte(100 + x), byte(10 + y), 31, 255})
		}
	}
	var buf bytes.Buffer
	if err = png.Encode(&buf, high); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "flag.png"), buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	entry := HDEntry{Edition: "base", Container: "DATA1", Name: "WFLAGA00.IMG", File: "flag.png", Width: 96, Height: 60, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), SHA256: fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))}
	return dir, source, entry, im, high
}

func TestHDFlagValidationAndFontSwitch(t *testing.T) {
	dir, source, e, im, high := hdFlagFixture(t)
	hdManifest(t, dir, []HDEntry{e})
	p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
	if err != nil || p.Count != 1 || len(p.Warnings) != 0 {
		t.Fatalf("%+v %v", p, err)
	}
	c := testCanvasPx(t, 24, 15)
	c.HD = p
	before := append([]byte(nil), im.Pix...)
	normal, selected := c.HighImage(im), c.HighImage(im.Complement())
	if normal == nil || selected == nil {
		t.Fatal("原旗／反白沒有高清")
	}
	for i := 0; i < len(normal.Pix); i += 4 {
		for k := 0; k < 3; k++ {
			if normal.Pix[i+k]^255 != selected.Pix[i+k] {
				t.Fatal("完整反白不符")
			}
		}
		if normal.Pix[i+3] != 255 || selected.Pix[i+3] != 255 {
			t.Fatal("alpha")
		}
	}
	for y := 0; y < 60; y++ {
		for x := 0; x < 96; x++ {
			if !image.Pt(x, y).In(image.Rect(4, 4, 64, 56)) && normal.RGBAAt(x, y) != high.RGBAAt(x, y) {
				t.Fatal("文字越過來源識別區")
			}
		}
	}
	if !bytes.Equal(before, im.Pix) {
		t.Fatal("修改來源索引")
	}
	if c.HighImage(im) != normal {
		t.Fatal("相同字型未使用快取")
	}
	var previous = *normal
	for _, name := range []string{"kai", "li"} {
		f, err := os.Open(filepath.Join("..", "..", "fonts", name+".hex.gz"))
		if err != nil {
			t.Fatal(err)
		}
		face, err := font.ParseHexGz(f, 16)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		c.SetFace(face)
		got := c.HighImage(im)
		if got == nil || bytes.Equal(previous.Pix, got.Pix) {
			t.Fatal("換字型仍沿用原旗字面")
		}
		previous = *got
	}
	missing, err := font.ParseHex(bytes.NewBufferString("0041:80000000000000000000000000000000\n"), 16)
	if err != nil {
		t.Fatal(err)
	}
	c.SetFace(missing)
	if c.HighImage(im) != nil || c.HighImage(im.Complement()) != nil {
		t.Fatal("缺字應整面回退")
	}
}

func TestHDFlagRejectsInvalidResources(t *testing.T) {
	dir, source, e, _, _ := hdFlagFixture(t)
	for _, tc := range []struct {
		name string
		edit func(*HDEntry)
	}{
		{"gate", func(e *HDEntry) { e.Name = "WFLAGA05.IMG" }}, {"army", func(e *HDEntry) { e.Name = "WFLAGA20.IMG" }}, {"formation", func(e *HDEntry) { e.Name = "WFLAGD09.IMG" }}, {"container", func(e *HDEntry) { e.Container = "DATA3" }}, {"source", func(e *HDEntry) { e.SourceSHA256 = "wrong" }}, {"size", func(e *HDEntry) { e.Height = 64 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := e
			tc.edit(&bad)
			hdManifest(t, dir, []HDEntry{bad})
			p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source, "DATA3": source})
			if err != nil || p.Count != 0 || len(p.Warnings) != 1 {
				t.Fatalf("錯旗未回退 %+v %v", p, err)
			}
		})
	}
	t.Run("brown", func(t *testing.T) {
		source.Data(0)[4+90] = 0x80
		e.SourceSHA256 = fmt.Sprintf("%x", sha256.Sum256(source.Data(0)))
		hdManifest(t, dir, []HDEntry{e})
		p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
		if err != nil || p.Count != 0 || len(p.Warnings) != 1 {
			t.Fatalf("反白不等價來源未回退 %+v %v", p, err)
		}
	})
	t.Run("transparent", func(t *testing.T) {
		source.Data(0)[4+90] = 0
		e.SourceSHA256 = fmt.Sprintf("%x", sha256.Sum256(source.Data(0)))
		high := image.NewRGBA(image.Rect(0, 0, 96, 60))
		var buf bytes.Buffer
		if err := png.Encode(&buf, high); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "flag.png"), buf.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}
		e.SHA256 = fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))
		hdManifest(t, dir, []HDEntry{e})
		p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": source})
		if err != nil || p.Count != 0 || len(p.Warnings) != 1 {
			t.Fatalf("透明旗未回退 %+v %v", p, err)
		}
	})
}

func TestHDFlagBattleLayersAndFallback(t *testing.T) {
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
		for _, selected := range []bool{false, true} {
			for _, overlap := range []bool{false, true} {
				for _, fieldView := range []bool{false, true} {
					t.Run(fmt.Sprintf("pref%d-selected%v-overlap%v-field%v", id, selected, overlap, fieldView), func(t *testing.T) {
						b := battle.New(battle.Setup{Field: fld, Seed: 1})
						u := &battle.Unit{Side: battle.MainAttacker, Formation: battle.Centre, Leaders: []battle.Leader{{Name: "測試", Soldiers: 1000}}, At: battle.FromOffset(2, 2)}
						b.Units = []*battle.Unit{u}
						if overlap {
							later := *u
							later.Side = battle.MainDefender
							b.Units = append(b.Units, &later)
						}
						_, _, _, _, high := hdFlagFixture(t)
						pack := &HDPack{images: map[[32]byte]*image.RGBA{}, flags: map[[32]byte]highFlag{}, flagText: map[highFlagTextKey]*image.RGBA{}, Count: 1}
						flag := ab.flags[2][0]
						pack.flags[hdImageKey(flag)] = highFlag{image: high, label: '帥'}
						pack.flags[hdImageKey(flag.Complement())] = highFlag{image: high, label: '帥', selected: true}
						for n := 0; n < 15; n++ {
							pack.images[hdImageKey(ab.tiles[n])] = hdTerrainDetail(n)
						}
						acting := (*battle.Unit)(nil)
						if selected {
							acting = u
						}
						v := BattleView{Acting: acting}
						info := ArtBattleInfo{Field: pref.BattleField, Portrait: [2]int{-1, -1}}
						original, hd := testCanvasPx(t, 640, 408), testCanvasPx(t, 640, 408)
						hd.HD = pack
						if fieldView {
							DrawArtField(original, ab, "測試", pref.BattleField, b.Units, acting)
							DrawArtField(hd, ab, "測試", pref.BattleField, b.Units, acting)
						} else {
							DrawArtBattle(original, ab, b, v, info)
							DrawArtBattle(hd, ab, b, v, info)
						}
						if !bytes.Equal(original.Img.Pix, hd.Img.Pix) {
							t.Fatal("改寫原貌")
						}
						out := hd.Output(true)
						x, y := assets.FlagCell(2, 2)
						expectedFlag := flag
						if selected {
							expectedFlag = flag.Complement()
						}
						target := hd.HighImage(expectedFlag)
						for yy := 0; yy < 15; yy++ {
							for xx := 0; xx < 24; xx++ {
								for sy := 0; sy < 4; sy++ {
									for sx := 0; sx < 4; sx++ {
										want := target.RGBAAt(xx*4+sx, yy*4+sy)
										if overlap {
											want = original.Img.RGBAAt(x+xx, y+yy)
										}
										if out.RGBAAt((x+xx)*4+sx, (y+yy)*4+sy) != want {
											t.Fatal("旗面或後畫缺圖覆蓋錯誤")
										}
									}
								}
							}
						}
						for yy := 15; yy < 32; yy++ {
							for xx := 0; xx < 40; xx++ {
								for sy := 0; sy < 4; sy++ {
									for sx := 0; sx < 4; sx++ {
										if out.RGBAAt((x+xx)*4+sx, (y+yy)*4+sy) != original.Img.RGBAAt(x+xx, y+yy) {
											t.Fatal("兵力牌被高清圖蓋掉")
										}
									}
								}
							}
						}
					})
				}
			}
		}
	}
}
