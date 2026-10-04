package ui

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 呼叫端傳原文。這裡從正式 renderer 核對姓名、六行資料及肖像區，
// 避免預先翻譯的測試掩蓋顯示層漏接。
func TestArtBattleLocalizesRawNamesAndPreservesPanels(t *testing.T) {
	prior := i18n.Current
	defer func() { i18n.Current = prior }()
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(artContainer(t, "DATA2"), state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	portraits := map[string]int{}
	for _, g := range sc.Generals() {
		portraits[g.Name] = int(g.Portrait)
	}
	for _, pid := range []int{4, 25} {
		for _, locale := range i18n.Locales() {
			for _, mode := range []string{"main", "unit", "inspect"} {
				t.Run(fmt.Sprintf("%d/%s/%s", pid, locale, mode), func(t *testing.T) {
					i18n.Current = locale
					pref, err := sc.Prefecture(pid)
					if err != nil {
						t.Fatal(err)
					}
					fld, err := battle.Load(pref.BattleField, pref.Neighbours)
					if err != nil {
						t.Fatal(err)
					}
					b := battle.New(battle.Setup{Field: fld, Seed: 1})
					raw := [2]string{"關羽", "諸葛亮"}
					units := [2]*battle.Unit{}
					for i, side := range artBattleSides {
						units[i] = &battle.Unit{Side: side, Formation: battle.Centre, Leaders: []battle.Leader{{Name: raw[i], Index: 13 + i, Stamina: 82, Intel: 90, War: 80, Soldiers: 3000, Training: 80, Arms: 80}}}
						b.Units = append(b.Units, units[i])
						b.Gold[side] = 65535
						b.Rice[side] = 65535
					}
					info := ArtBattleInfo{Field: pref.BattleField, Prefecture: pref.Name, Province: state.ProvinceName(int(pref.Province)), ID: pref.ID, Commander: raw, Lord: [2]string{"劉備", "劉備"}, Portrait: [2]int{portraits[raw[0]], portraits[raw[1]]}, Date: game.Date{Year: 189, Month: 1}}
					if mode == "unit" {
						info.Units = &[2]UnitPanel{{Unit: units[0], Portrait: info.Portrait[0], Lord: "劉備"}, {Unit: units[1], Portrait: info.Portrait[1], Lord: "劉備"}}
					}
					if mode == "inspect" {
						info.Inspect = &InspectPanel{Leader: &units[1].Leaders[0], Side: battle.MainDefender, Portrait: info.Portrait[1]}
					}
					got := testCanvasPx(t, 640, 408)
					DrawArtBattle(got, ab, b, BattleView{}, info)
					layout := assets.BattleLayoutFor(fld.Narrow())
					wantNames := raw
					if locale == i18n.En {
						wantNames = [2]string{"Guan Yu", "Zhuge Liang"}
					}
					if locale == i18n.Ja {
						wantNames[0] = "関羽"
					}
					for i, side := range artBattleSides {
						ink := assets.EGAPalette[assets.BattlePanelInks[i]]
						if mode == "unit" {
							ink = assets.EGAPalette[assets.FlagPlateColour[side.OriginalIndex()]]
						}
						want := testCanvasPx(t, 640, 408)
						rect := image.Rect(layout.NameX(i), layout.PanelY[i], layout.NameX(i)+32, layout.PanelY[i]+assets.BattlePanelH)
						if locale == i18n.En {
							x := min(layout.NameX(i), layout.TextX(i))
							rect = image.Rect(x, layout.PanelY[i], x+96, layout.PanelY[i]+assets.BattlePanelH)
							want.FillRect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y, assets.EGAPalette[assets.BattlePanelPaper])
							lines := []string{wantNames[i], tf("bat.armyOf", "Liu Bei"), tf("bat.sideLine", SideName(side))}
							if mode == "unit" {
								lines = append(lines, "", tf("bat.panelUnit", battle.Centre.Label(), 1), tf("bat.panelMen", 3000))
							} else {
								lines = append(lines, tf("bat.forcesLine", battleUnitsNumeral(1), 1), tf("bat.menLine", 3000), tf("bat.goldLine", 65535), tf("bat.riceLine", 65535))
							}
							var rows []string
							for _, line := range lines {
								rows = append(rows, cells.Wrap(line, 96/SmallW)...)
							}
							if len(rows)*SmallH > assets.BattlePanelH {
								t.Fatal("預期完整資料超高", rows)
							}
							for k, line := range rows {
								want.DrawSmallTextPx(x, rect.Min.Y+k*SmallH, line, ink)
							}
						} else {
							want.FillRect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y, assets.EGAPalette[assets.BattlePanelPaper])
							y := rect.Min.Y
							if len([]rune(wantNames[i])) == 2 {
								y += 16
							}
							for k, r := range []rune(wantNames[i]) {
								want.DrawRuneScaledPx(rect.Min.X, y+k*32, r, ink, 2, 2)
							}
						}
						assertBattleLocaleRegion(t, got, want, rect, "原始統帥及完整資料")
					}
					if mode == "inspect" {
						rect := image.Rect(inspectNameX, layout.PanelY[2], inspectFaceX-8, layout.PanelY[2]+assets.BattlePanelH)
						want := testCanvasPx(t, 640, 408)
						want.FillRect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y, assets.EGAPalette[assets.BattlePanelPaper])
						ink := assets.EGAPalette[assets.FlagPlateColour[battle.MainDefender.OriginalIndex()]]
						if locale == i18n.En {
							for k, line := range []string{"Zhuge", "Liang"} {
								want.DrawSmallTextPx(rect.Min.X, rect.Min.Y+k*SmallH, line, ink)
							}
						} else {
							for k, r := range []rune("諸葛亮") {
								want.DrawRuneScaledPx(rect.Min.X, rect.Min.Y+k*32, r, ink, 2, 2)
							}
						}
						assertBattleLocaleRegion(t, got, want, rect, "原始查看姓名")
					}
					layers := ab.compose(b, BattleView{}, info)
					for i := 0; i < 2; i++ {
						x, y := layout.Frame(i)
						for py := y; py < y+assets.BattlePanelH; py++ {
							for px := x; px < x+assets.BattleFrameW; px++ {
								if got.Img.RGBAAt(px, py) != assets.EGAPalette[layers.At(px, py)&15] {
									t.Fatalf("字壓到肖像框 (%d,%d)", px, py)
								}
							}
						}
					}
					if info.Commander != raw || info.Lord != [2]string{"劉備", "劉備"} || units[0].Head().Name != raw[0] || units[1].Head().Name != raw[1] {
						t.Fatal("繪圖修改了原始姓名")
					}
				})
			}
		}
	}
}

func assertBattleLocaleRegion(t *testing.T, got, want *Canvas, rect image.Rectangle, label string) {
	t.Helper()
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if got.Img.RGBAAt(x, y) != want.Img.RGBAAt(x, y) {
				t.Fatalf("%s不符 (%d,%d)", label, x, y)
			}
		}
	}
}

func TestBattleDisplayNameKeepsRawAndTemplateNames(t *testing.T) {
	prior := i18n.Current
	defer func() { i18n.Current = prior }()
	for _, locale := range i18n.Locales() {
		i18n.Current = locale
		want := "新君主"
		if locale == i18n.En {
			want = "New lord"
		}
		if got := battleDisplayName("新君主"); got != want {
			t.Fatalf("%s 模板顯示 %q", locale, got)
		}
		if got := battleDisplayName("龘甲"); got != "龘甲" {
			t.Fatalf("%s 未知姓名須整串回退：%q", locale, got)
		}
		if got := i18n.T(locale, "title.newLordName"); got != "新君主" {
			t.Fatalf("%s 改了存入新局的模板名：%q", locale, got)
		}
	}
}

func TestBattleSidePanelUsesHeightToChooseFont(t *testing.T) {
	c := testCanvasPx(t, 640, 408)
	lines := []string{"A", "B", "C", "D", "E", "F", "G"}
	rows, small := sidePanelLayout(c, lines, 96)
	if !small || len(rows)*SmallH > assets.BattlePanelH || strings.Join(rows, "") != strings.Join(lines, "") {
		t.Fatal("七行完整 ASCII 須留在面板內", rows, small)
	}
}

func TestBattleEnglishDateRendersTheWholeString(t *testing.T) {
	prior := i18n.Current
	defer func() { i18n.Current = prior }()
	i18n.Current = i18n.En
	maxInt := int(^uint(0) >> 1)
	actual := testCanvasPx(t, 640, 408)
	normal := testCanvasPx(t, 640, 408)
	for _, d := range []game.Date{{Year: 189, Month: 1}, {Year: 190, Month: 4}, {Year: 219, Month: 8}, {Year: 220, Month: 12}, {Year: 289, Month: 11}, {Year: 290, Month: 1}, {Year: 65535, Month: 12}, {Year: maxInt, Month: 12}, {Year: -maxInt - 1, Month: 1}} {
		for _, cal := range []game.Calendar{game.ChineseEra, game.Western} {
			clear(actual.Img.Pix)
			clear(normal.Img.Pix)
			text := d.FormatWithSeason(cal)
			width := cells.Width(text) * CellW
			end := assets.BattleDateX + (assets.BattleDateCells-1)*assets.BattleDateStep + assets.BattleDateW
			if assets.BattleDateX+width > end {
				t.Fatal("完整日期超寬", text)
			}
			drawBattleDate(actual, d, cal)
			normal.DrawTextPx(0, 0, text, assets.EGAPalette[15])
			for y := 0; y < 408; y++ {
				for x := 0; x < 640; x++ {
					ink := false
					if x >= assets.BattleDateX && x < assets.BattleDateX+width && y >= assets.BattleDateY && y < assets.BattleDateY+assets.BattleDateH {
						ink = normal.Img.RGBAAt(x-assets.BattleDateX, (y-assets.BattleDateY)*CellH/assets.BattleDateH).A != 0
					}
					got := actual.Img.RGBAAt(x, y)
					if (got.A != 0) != ink {
						t.Fatalf("完整日期 %q 字模不符 (%d,%d)", text, x, y)
					}
					if ink {
						index := assets.BattleDateInkEven
						if (x+y)%2 == 1 {
							index = assets.BattleDateInkOdd
						}
						if got != assets.EGAPalette[index] {
							t.Fatal("日期網點變色", text)
						}
					}
				}
			}
		}
	}
}

func TestArtBattleMixedCommanderKeepsTextInPanel(t *testing.T) {
	prior := i18n.Current
	defer func() { i18n.Current = prior }()
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(artContainer(t, "DATA2"), state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	pref, err := sc.Prefecture(4)
	if err != nil {
		t.Fatal(err)
	}
	fld, err := battle.Load(pref.BattleField, pref.Neighbours)
	if err != nil {
		t.Fatal(err)
	}
	for _, locale := range i18n.Locales() {
		t.Run(string(locale), func(t *testing.T) {
			i18n.Current = locale
			b := battle.New(battle.Setup{Field: fld, Seed: 1})
			info := ArtBattleInfo{Field: pref.BattleField, Prefecture: pref.Name,
				Province: state.ProvinceName(int(pref.Province)), ID: pref.ID,
				Commander: [2]string{"Bob龘", ""}, Lord: [2]string{"劉備", ""},
				Date: game.Date{Year: 189, Month: 1}}
			got := testCanvasPx(t, 640, 408)
			DrawArtBattle(got, ab, b, BattleView{}, info)
			background := ab.compose(b, BattleView{}, info)
			l := assets.BattleLayoutFor(fld.Narrow())
			// 原文回退只佔原有六行，面板下的完整空隙不得出現第七行。
			x0 := min(l.NameX(0), l.TextX(0))
			y0 := l.PanelY[0] + assets.BattlePanelH
			for y := y0; y < l.PanelY[1]-2; y++ {
				for x := x0; x < l.PanelX[0]+assets.BattlePanelW; x++ {
					if got.Img.RGBAAt(x, y) != assets.EGAPalette[background.At(x, y)&15] {
						t.Fatalf("混合姓名讓軍力文字溢出面板 (%d,%d)", x, y)
					}
				}
			}
			if info.Commander[0] != "Bob龘" || info.Lord[0] != "劉備" {
				t.Fatal("繪圖更改原始姓名")
			}
		})
	}
}
