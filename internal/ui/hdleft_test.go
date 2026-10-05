package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func TestHDBattleLeftColumnForegroundAndFallback(t *testing.T) {
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(artContainer(t, "DATA2"), state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	fonts := testCanvasPx(t, 640, 408)
	canvas := func() *Canvas {
		c := NewCanvasPx(640, 408, fonts.face)
		c.SetSmallFace(fonts.small)
		return c
	}
	detail := func(w, h int) *image.RGBA {
		im := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				im.SetRGBA(x, y, color.RGBA{byte(x), byte(y), 71, 255})
			}
		}
		return im
	}
	skin, tile := detail(720, 400), detail(32, 32)
	images := map[[32]byte]*image.RGBA{hdImageKey(ab.bgTile): tile}
	for _, im := range ab.weather {
		images[hdImageKey(im)] = detail(128, 128)
	}
	// 明示既有幾何，避免 UI 和資產層同時偏移後仍自我驗證通過。
	boxes := []image.Rectangle{
		image.Rect(6, 50, 42, 150), image.Rect(6, 153, 42, 190),
		image.Rect(6, 194, 42, 214), image.Rect(6, 226, 42, 326),
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
		for _, locale := range i18n.Locales() {
			for _, weather := range []battle.Weather{battle.Clear, battle.Windy, battle.Rainy} {
				for _, kind := range []string{"battle", "skirmish", "inspect"} {
					t.Run(fmt.Sprintf("pref%d-%s-%d-%s", id, locale, weather, kind), func(t *testing.T) {
						i18n.Current = locale
						b := battle.New(battle.Setup{Field: fld, Seed: 1})
						b.Day, b.Weather = 99, weather
						info := ArtBattleInfo{Field: pref.BattleField, Prefecture: pref.Name,
							Province: "豫州", ID: id, Portrait: [2]int{-1, -1}}
						if kind == "skirmish" {
							info.Skirmish = &battle.Skirmish{Hour: 23}
						}
						if kind == "inspect" {
							info.Inspect = &InspectPanel{Leader: &battle.Leader{Name: "曹操"}, Portrait: -1}
						}
						plain, control, c := canvas(), canvas(), canvas()
						control.HD = &HDPack{images: images, battleBackground: tileBattleBackground(tile)}
						c.HD = &HDPack{images: images, battleBackground: control.HD.battleBackground,
							panels: map[string]hdPanel{"PANEL.BEVEL#3": {skin, 2}}, panelCache: map[hdPanelSize]*image.RGBA{}}
						for _, target := range []*Canvas{plain, control, c} {
							DrawArtBattle(target, ab, b, BattleView{}, info)
						}
						if !bytes.Equal(c.Img.Pix, plain.Img.Pix) || c.Output(false) != c.Img {
							t.Fatal("original canvas changed")
						}
						out, fallback := c.Output(true), control.Output(true)
						// 正式字模的寫入記錄包括同色像素，不能靠畫前畫後顏色推測。
						glyphs := canvas()
						glyphs.addHigh(nil, glyphs.Img.Bounds(), image.Point{})
						ab.drawText(glyphs, b, BattleView{}, info)
						if len(glyphs.Missing) != 0 || glyphs.Clipped != 0 {
							t.Fatalf("glyphs missing=%v clipped=%d", glyphs.Missing, glyphs.Clipped)
						}
						for _, r := range boxes {
							panel := c.highPanel("PANEL.BEVEL#3", r, false)
							for y := r.Min.Y * 4; y < r.Max.Y*4; y++ {
								for x := r.Min.X * 4; x < r.Max.X*4; x++ {
									lx, ly := x/4, y/4
									want := panel.RGBAAt(x-r.Min.X*4, y-r.Min.Y*4)
									if image.Pt(lx, ly).In(image.Rect(8, 155, 40, 187)) {
										want = images[hdImageKey(ab.weather[weather.OriginalIndex()])].RGBAAt(x-32, y-620)
									}
									if glyphs.highOps[0].covered[ly*640+lx] {
										want = plain.Img.RGBAAt(lx, ly)
									}
									if out.RGBAAt(x, y) != want {
										t.Fatalf("left frame/weather/glyph mismatch %d,%d", x, y)
									}
								}
							}
							// 四角原生 8×8 不經縮放。
							for _, corner := range []image.Point{{0, 0}, {r.Dx()*4 - 8, 0}, {0, r.Dy()*4 - 8}, {r.Dx()*4 - 8, r.Dy()*4 - 8}} {
								sx, sy := 0, 0
								if corner.X > 0 {
									sx = 712
								}
								if corner.Y > 0 {
									sy = 392
								}
								for y := 0; y < 8; y++ {
									for x := 0; x < 8; x++ {
										if out.RGBAAt(r.Min.X*4+corner.X+x, r.Min.Y*4+corner.Y+y) != skin.RGBAAt(sx+x, sy+y) {
											t.Fatal("left corner rescaled or covered")
										}
									}
								}
							}
						}
						l := assets.BattleLayoutFor(fld.Narrow())
						x0, y0, x1, y1 := l.Panel(2)
						command := image.Rect(x0-2, y0-2, x1+3, y1+3)
						for y := 0; y < 408; y++ {
							for x := 0; x < 640; x++ {
								point, changed := image.Pt(x, y), false
								for _, r := range boxes {
									changed = changed || point.In(r)
								}
								if changed || kind != "inspect" && point.In(command) {
									continue
								}
								for dy := 0; dy < 4; dy++ {
									for dx := 0; dx < 4; dx++ {
										if out.RGBAAt(x*4+dx, y*4+dy) != fallback.RGBAAt(x*4+dx, y*4+dy) {
											t.Fatalf("outside frame or fallback mismatch %d,%d", x, y)
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

func TestHDBattleLeftColumnCacheAndMissingSkin(t *testing.T) {
	c := NewCanvasPx(640, 408, nil)
	c.HD = &HDPack{panels: map[string]hdPanel{}, panelCache: map[hdPanelSize]*image.RGBA{}}
	c.drawHighBattleLeftColumn()
	if len(c.highOps) != 0 || len(c.HD.panelCache) != 0 {
		t.Fatal("missing skin did not fall back")
	}
	c.HD.panels["PANEL.BEVEL#3"] = hdPanel{image.NewRGBA(image.Rect(0, 0, 720, 400)), 2}
	for range 100 {
		c.highOps = nil
		c.drawHighBattleLeftColumn()
		if len(c.highOps) != 4 || len(c.HD.panelCache) != 3 || c.HD.panelCacheBytes != 361728 {
			t.Fatalf("unbounded or incomplete cache: ops=%d sizes=%d bytes=%d", len(c.highOps), len(c.HD.panelCache), c.HD.panelCacheBytes)
		}
	}
}

func TestHDAtlasDoesNotAddBattleLeftColumn(t *testing.T) {
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
		c := testCanvasPx(t, 640, 408)
		c.HD = &HDPack{panels: map[string]hdPanel{"PANEL.BEVEL#3": {image.NewRGBA(image.Rect(0, 0, 720, 400)), 2}}, panelCache: map[hdPanelSize]*image.RGBA{}}
		DrawArtAtlas(c, ab, pref.BattleField, fld)
		if len(c.HD.panelCache) != 0 || len(c.highOps) != 0 {
			t.Fatal("atlas unexpectedly added battle left frames")
		}
	}
}
