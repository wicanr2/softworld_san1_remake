package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 獨立以來源列的聯集重建完整字模，避免只檢查「有文字」而漏掉姓名尾字。
func markerNameReference(c *Canvas, x, y int, name string, ink color.RGBA) {
	if cells.Width(name)*CellW <= 48 || !c.FitsSmall(name) {
		c.DrawTextPx(x, y, name, ink)
		return
	}
	if cells.Width(name)*SmallW <= 47 {
		c.DrawSmallTextPx(x, y+3, name, ink)
		return
	}
	groups := [][]int{{1, 2}, {3}, {4}, {5, 6}, {7}, {8}, {9}}
	for row, line := range cells.Wrap(name, 7) {
		for i, r := range strings.TrimSpace(line) {
			g, _ := c.small.Glyph(r)
			for dy, sourceRows := range groups {
				for gx := 0; gx < 6; gx++ {
					for _, sy := range sourceRows {
						if g.At(gx, sy) {
							c.Img.SetRGBA(x+i*6+gx, y+row*8+dy, ink)
						}
					}
				}
			}
		}
	}
}

func TestSkirmishMarkerLocalizesRawNamesAndKeepsOtherFields(t *testing.T) {
	prior := i18n.Current
	defer func() { i18n.Current = prior }()
	fonts := testCanvasPx(t, 640, 408)
	canvas := func() *Canvas {
		c := NewCanvasPx(640, 408, fonts.face)
		c.SetSmallFace(fonts.small)
		c.Fill(assets.EGAPalette[0])
		return c
	}
	cases := []struct {
		raw string
		en  string
		ja  string
	}{
		{"呂布", "Lu Bu", "呂布"}, {"陳宮", "Chen Gong", "陳宮"},
		{"關羽", "Guan Yu", "関羽"}, {"諸葛亮", "Zhuge Liang", "諸葛亮"},
		{"公孫瓚", "Gongsun Zan", "公孫瓚"}, {"新君主", "New lord", "新君主"},
		{"龘甲", "龘甲", "龘甲"},
	}
	for _, tc := range cases {
		for _, locale := range i18n.Locales() {
			for _, blink := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/%t", tc.raw, locale, blink), func(t *testing.T) {
					i18n.Current = locale
					leader := battle.Leader{Name: tc.raw, Soldiers: 3000}
					u := &battle.Unit{Side: battle.MainAttacker}
					g := &battle.SkirmishGeneral{Leader: &leader, Unit: u, Side: battle.SkirmishAttacker, Col: 1, Row: 2}
					s := &battle.Skirmish{}
					s.Gens[1][0] = g
					v := BattleView{SkirmishActing: g, Blink: blink}
					got, want := canvas(), canvas()
					drawSkirmishMarkerText(got, s, v)
					x, y := assets.FieldCell(1, 2)
					ink, side, num := skirmishColour(g), byte(0), skirmishColour(g)&7
					if blink {
						ink, side, num = ink^15, side^15, num^15
					}
					name := tc.raw
					if locale == i18n.En {
						name = tc.en
					} else if locale == i18n.Ja {
						name = tc.ja
					}
					markerNameReference(want, x, y, battlePaddedName(name), assets.EGAPalette[ink])
					want.DrawTextPx(x, y+15, i18n.S("skm.attacker"), assets.EGAPalette[side])
					want.DrawTextPx(x+16, y+15, "3000", assets.EGAPalette[num])
					if !bytes.Equal(got.Img.Pix, want.Img.Pix) {
						t.Fatal("完整姓名、兵數、攻方標記或反白字色不符")
					}
					if leader.Name != tc.raw || leader.Soldiers != 3000 || g.Col != 1 || g.Row != 2 || g.Gone {
						t.Fatal("顯示改動了部隊狀態")
					}
					if locale == i18n.ZhHant {
						old := canvas()
						old.DrawTextPx(x, y, battlePaddedName(tc.raw), assets.EGAPalette[ink])
						assertBattleLocaleRegion(t, got, old, image.Rect(x, y, x+48, y+15), "繁中保持")
					}
				})
			}
		}
	}
}

func TestSkirmishAllScenarioNamesFitAndRetainEveryGlyph(t *testing.T) {
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		root = "../../org_game"
	}
	if _, err := os.Stat(filepath.Join(root, "三國演義", "DATA2.GRP")); os.IsNotExist(err) {
		t.Skip("未掛載原版姓名資料")
	}
	fonts := testCanvasPx(t, 80, 48)
	prior := i18n.Current
	defer func() { i18n.Current = prior }()
	seen := map[string]bool{}
	for _, edition := range []string{"三國演義", "三國演義1加強版"} {
		read := func(ext string) []byte {
			b, err := os.ReadFile(filepath.Join(root, edition, "DATA2."+ext))
			if err != nil {
				t.Fatalf("原始姓名來源 %s: %v", edition, err)
			}
			return b
		}
		d, err := assets.OpenContainer(read("NAM"), read("IDX"), read("GRP"))
		if err != nil {
			t.Fatal(err)
		}
		for _, slot := range []state.Slot{state.Scenario1, state.Scenario2, state.Scenario3, state.Scenario4, state.Scenario5, state.Scenario6} {
			sc, err := state.LoadScenario(d, slot)
			if err != nil {
				t.Fatal(err)
			}
			for _, general := range sc.Generals() {
				if general.Name == "" || seen[general.Name] {
					continue
				}
				seen[general.Name] = true
				for _, locale := range i18n.Locales() {
					i18n.Current = locale
					name := battlePaddedName(battleDisplayName(general.Name))
					if fonts.FitsSmall(name) && len(cells.Wrap(name, 7)) > 2 {
						t.Fatalf("%s %s 需要三行", locale, name)
					}
					got := NewCanvasPx(80, 48, fonts.face)
					got.SetSmallFace(fonts.small)
					want := NewCanvasPx(80, 48, fonts.face)
					want.SetSmallFace(fonts.small)
					drawSkirmishName(got, 8, 8, name, fg)
					markerNameReference(want, 8, 8, name, fg)
					if !bytes.Equal(got.Img.Pix, want.Img.Pix) || len(got.Missing) != 0 {
						t.Fatalf("%s %s 完整字模不符，缺字 %v", locale, name, got.Missing)
					}
					for y := 0; y < 48; y++ {
						for x := 0; x < 80; x++ {
							height := 16
							if fonts.FitsSmall(name) && cells.Width(name)*CellW > 48 {
								height = 15
							}
							if got.Img.RGBAAt(x, y).A != 0 && !image.Pt(x, y).In(image.Rect(8, 8, 56, 8+height)) {
								t.Fatalf("%s %s 墨點越界 %d,%d", locale, name, x, y)
							}
						}
					}
				}
			}
		}
	}
	t.Logf("兩版六劇本 %d 個獨立原始姓名，三語全文及區界通過", len(seen))
}

func TestSkirmishLongNameTracksHDForeground(t *testing.T) {
	c := testCanvasPx(t, 80, 48)
	c.Fill(bg)
	c.addHigh(nil, c.Img.Bounds(), image.Point{})
	drawSkirmishName(c, 8, 8, "Xing Daorong", fg)
	ink := 0
	for y := 0; y < 48; y++ {
		for x := 0; x < 80; x++ {
			if c.Img.RGBAAt(x, y) == fg {
				ink++
				if !c.highOps[0].covered[y*80+x] {
					t.Fatalf("高清未保護姓名墨點 %d,%d", x, y)
				}
			}
		}
	}
	if ink == 0 || len(c.Missing) != 0 {
		t.Fatal("姓名未完整繪製")
	}
}
