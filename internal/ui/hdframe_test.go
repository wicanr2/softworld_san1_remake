package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func TestHDFrameSourceValidation(t *testing.T) {
	for _, source := range []struct {
		name, container string
		w, h            int
	}{
		{"MAINMAP1.IMG", "DATA3", 640, 36}, {"MAINMAP2.IMG", "DATA3", 640, 36},
		{"MAINMAP3.IMG", "DATA3", 72, 336}, {"MAINMAP7.IMG", "DATA3", 8, 336},
		{"MAINMAP8.IMG", "DATA3", 640, 36}, {assets.BattleBGTile, "DATA1", 8, 8},
	} {
		t.Run(source.name, func(t *testing.T) {
			for _, check := range []string{"valid", "shape", "alpha", "container", "hash", "missing"} {
				t.Run(check, func(t *testing.T) {
					w, h := source.w, source.h
					if check == "shape" {
						h++
					}
					im := &assets.Image{W: w, H: h, Pix: make([]byte, w*h)}
					dir, container, entry, _ := hdMenuFixture(t, source.name, im, check == "alpha")
					entry.Container = source.container
					if check == "container" {
						if entry.Container == "DATA1" {
							entry.Container = "DATA3"
						} else {
							entry.Container = "DATA1"
						}
					}
					if check == "hash" {
						entry.SourceSHA256 = "wrong"
					}
					if check == "missing" {
						if e := os.Remove(filepath.Join(dir, entry.File)); e != nil {
							t.Fatal(e)
						}
					}
					hdManifest(t, dir, []HDEntry{entry})
					p, e := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": container, "DATA3": container})
					if e != nil {
						t.Fatal(e)
					}
					if check == "valid" {
						if p.Count != 1 || len(p.Warnings) != 0 {
							t.Fatal("valid source rejected", p.Warnings)
						}
						if source.name == assets.BattleBGTile && p.battleBackground.Bounds() != image.Rect(0, 0, 2560, 1632) {
							t.Fatal("tile cache bounds")
						}
					} else if p.Count != 0 || len(p.Warnings) != 1 || p.battleBackground != nil {
						t.Fatal("invalid source accepted", check)
					}
				})
			}
		})
	}
	for _, name := range []string{"MAINMAP6.IMG", "MAINMAPB.IMG", "MAINMAPC.IMG", "8x8PAT1.IMG"} {
		if hdResource.MatchString(name) {
			t.Fatal("world map or other pattern accepted", name)
		}
	}
}

func TestHDMainFrameOrderAndOriginalCanvas(t *testing.T) {
	a, e := NewArtScreen(artContainer(t, "DATA3"), nil)
	if e != nil {
		t.Fatal(e)
	}
	c := testCanvasPx(t, 640, 408)
	c.drawRGBA(c.Img.Bounds(), a.base.RGBA(), image.Point{})
	original := append([]byte(nil), c.Img.Pix...)
	c.HD = &HDPack{images: map[[32]byte]*image.RGBA{}}
	for _, p := range a.layers {
		if p.Name == "MAINMAP4.IMG" || p.Name == "MAINMAP5.IMG" || p.Name == "MAINMAPB.IMG" || p.Name == "MAINMAPC.IMG" {
			continue
		}
		im := image.NewRGBA(image.Rect(0, 0, p.Image.W*4, p.Image.H*4))
		for i := 0; i < len(im.Pix); i += 4 {
			copy(im.Pix[i:i+4], []byte{17, byte(i / 4), 89, 255})
		}
		c.HD.images[hdImageKey(p.Image)] = im
	}
	a.drawHighFrame(c)
	out := c.Output(true)
	if !bytes.Equal(c.Img.Pix, original) || c.Output(false) != c.Img {
		t.Fatal("original canvas changed")
	}
	// 地圖及底圖右下面板的末列始終保持；末列會擋住先畫的下框。
	for _, r := range []image.Rectangle{image.Rect(72, 36, 408, 372), image.Rect(408, 372, 632, 373)} {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				for dy := 0; dy < 4; dy++ {
					for dx := 0; dx < 4; dx++ {
						if out.RGBAAt(x*4+dx, y*4+dy) != c.Img.RGBAAt(x, y) {
							t.Fatalf("source overlap at %d,%d", x, y)
						}
					}
				}
			}
		}
	}
	if out.RGBAAt(0, 0) != c.HD.images[hdImageKey(a.layers[0].Image)].RGBAAt(0, 0) {
		t.Fatal("top not replaced")
	}
	c.HD = &HDPack{images: map[[32]byte]*image.RGBA{}}
	c.drawRGBA(c.Img.Bounds(), a.base.RGBA(), image.Point{})
	a.drawHighFrame(c)
	fallback := c.Output(true)
	for y := 0; y < 408; y++ {
		for x := 0; x < 640; x++ {
			if fallback.RGBAAt(x*4, y*4) != c.Img.RGBAAt(x, y) {
				t.Fatal("empty pack fallback")
			}
		}
	}
}

func TestHDBattleBackgroundPreservesAllForeground(t *testing.T) {
	c1, c3 := artContainers(t)
	ab, e := NewArtBattle(c1, c3)
	if e != nil {
		t.Fatal(e)
	}
	sc, e := state.LoadScenario(artContainer(t, "DATA2"), state.Scenario1)
	if e != nil {
		t.Fatal(e)
	}
	tile := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			tile.SetRGBA(x, y, color.RGBA{byte(x + 30), byte(y + 20), 71, 255})
		}
	}
	pack := &HDPack{images: map[[32]byte]*image.RGBA{hdImageKey(ab.bgTile): tile}, battleBackground: tileBattleBackground(tile)}
	for _, id := range []int{25, 26} {
		for _, kind := range []string{"battle", "skirmish", "atlas"} {
			t.Run(fmt.Sprintf("pref%d-%s", id, kind), func(t *testing.T) {
				pref, e := sc.Prefecture(id)
				if e != nil {
					t.Fatal(e)
				}
				fld, e := battle.Load(pref.BattleField, pref.Neighbours)
				if e != nil {
					t.Fatal(e)
				}
				b := battle.New(battle.Setup{Field: fld, Seed: 1})
				info := ArtBattleInfo{Field: pref.BattleField, Portrait: [2]int{-1, -1}}
				if kind == "skirmish" {
					info.Skirmish = &battle.Skirmish{Hour: battle.SkirmishFirstHour}
				}
				plain, c := testCanvasPx(t, 640, 408), testCanvasPx(t, 640, 408)
				render := func(canvas *Canvas) {
					if kind == "atlas" {
						DrawArtAtlas(canvas, ab, pref.BattleField, fld)
					} else {
						DrawArtBattle(canvas, ab, b, BattleView{}, info)
					}
				}
				render(plain)
				c.HD = pack
				render(c)
				out := c.Output(true)
				if !bytes.Equal(c.Img.Pix, plain.Img.Pix) {
					t.Fatal("original canvas changed")
				}
				// 以正式索引繪製記錄覆蓋，不用顏色相同與否推測遮擋。
				saved := ab.bg
				ab.bg = &assets.Image{W: 640, H: 408, Pix: bytes.Repeat([]byte{255}, 640*408)}
				var mask *assets.Image
				if kind == "atlas" {
					mask = ab.bg.Clone()
					mask.Blit(ab.top, 0, 0)
					mask.Blit(ab.bottom, 0, 372)
					l := assets.BattleLayoutFor(fld.Narrow())
					mask.FieldEdges(l)
					mask.BlitField(ab.tiles, pref.BattleField)
					drawAtlasForeground(mask, l)
				} else {
					mask = ab.compose(b, BattleView{}, info)
				}
				ab.bg = saved
				backgroundPixels := 0
				for y := 0; y < 408; y++ {
					for x := 0; x < 640; x++ {
						isBG := mask.At(x, y) == 255
						if isBG {
							backgroundPixels++
						}
						for dy := 0; dy < 4; dy++ {
							for dx := 0; dx < 4; dx++ {
								want := plain.Img.RGBAAt(x, y)
								if isBG {
									want = tile.RGBAAt((x*4+dx)%32, (y*4+dy)%32)
								}
								if out.RGBAAt(x*4+dx, y*4+dy) != want {
									t.Fatalf("background/foreground mismatch %d,%d +%d,%d", x, y, dx, dy)
								}
							}
						}
					}
				}
				if backgroundPixels == 0 {
					t.Fatal("no background control region")
				}
				if pack.battleBackground == nil {
					t.Fatal("cache lost")
				}
			})
		}
	}
}
