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
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// localeScenarioGames 載入兩版六劇本，避免只用 001 開局證明所有姓名都放得下。
func localeScenarioGames(t *testing.T, check func(*game.State)) {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過原版資料驗證")
	}
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		dir := "三國演義"
		if edition == state.EditionPlus {
			dir += "1加強版"
		}
		read := func(ext string) []byte {
			b, err := os.ReadFile(filepath.Join(root, dir, "DATA2."+ext))
			if err != nil {
				t.Fatal(err)
			}
			return b
		}
		container, err := assets.OpenContainer(read("NAM"), read("IDX"), read("GRP"))
		if err != nil {
			t.Fatal(err)
		}
		for slot := 1; slot <= 6; slot++ {
			scenario, err := state.LoadScenario(container, state.Slot(fmt.Sprintf("%03d", slot)))
			if err != nil {
				t.Fatal(err)
			}
			g, err := game.New(scenario, 0, 5, edition)
			if err != nil {
				t.Fatal(err)
			}
			check(g)
		}
	}
}

func localeRegion(im *image.RGBA, r image.Rectangle) []byte {
	var out []byte
	for y := r.Min.Y; y < r.Max.Y; y++ {
		i := im.PixOffset(r.Min.X, y)
		out = append(out, im.Pix[i:i+r.Dx()*4]...)
	}
	return out
}

func TestCardOriginCompleteAcrossScenarios(t *testing.T) {
	c := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
	expected := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	seen := map[string]bool{}
	cases := 0
	localeScenarioGames(t, func(g *game.State) {
		for _, locale := range i18n.Locales() {
			i18n.Current = locale
			for _, x := range g.AllGenerals() {
				if x == nil || x.Name == "" {
					continue
				}
				p := g.Prefecture(x.Origin)
				if p == nil {
					continue
				}
				cases++
				province := state.ProvinceName(int(p.Province))
				want := tf("card.origin", PlaceName(province), PlaceName(p.Name))
				if cells.Width(want) > cardNarrow {
					want = tf("card.origin", PlaceName(strings.TrimSuffix(province, "州")), PlaceName(p.Name))
				}
				key := string(locale) + "/" + province + "/" + p.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				var actual string
				for _, line := range cardLines(g, x) {
					if line.y == 84 {
						actual = line.text
					}
				}
				if actual != want {
					t.Errorf("%s 籍貫 %q，完整資料為 %q", key, actual, want)
				}
				DrawPersonCard(c, nil, g, x.Index)
				expected.FillRect(424, 84, 528, 100, assets.EGAPalette[cardPaper])
				if cells.Width(want)*CellW <= 104 {
					expected.DrawTextPx(424, 84, want, assets.EGAPalette[11])
				} else {
					if !expected.FitsSmall(want) || cells.Width(want)*SmallW > 104 {
						t.Fatalf("%s 完整籍貫仍放不下：%q", key, want)
					}
					expected.DrawSmallTextPx(424, 87, want, assets.EGAPalette[11])
				}
				r := image.Rect(424, 84, 528, 100)
				if !bytes.Equal(localeRegion(c.Img, r), localeRegion(expected.Img, r)) {
					t.Errorf("%s 正式卡片未畫出完整籍貫 %q", key, want)
				}
			}
		}
	})
	t.Logf("實際籍貫案例 %d，完整字模 %d 組", cases, len(seen))
}

func TestArtGovernorCompleteAcrossScenarios(t *testing.T) {
	c := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
	expected := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	i18n.Current = i18n.En
	seen := map[string]bool{}
	cases := 0
	localeScenarioGames(t, func(g *game.State) {
		for _, pv := range g.Prefectures() {
			x := g.Governor(pv.ID)
			if x == nil {
				continue
			}
			name := PersonName(x.Name)
			if !c.FitsSmall(name) {
				continue
			}
			cases++
			if seen[name] {
				continue
			}
			seen[name] = true
			c.Fill(color.RGBA{})
			expected.Fill(color.RGBA{})
			drawArtStatus(c, g, g.Prefecture(pv.ID), pv.ID)
			if len(name)*CellW <= 80 {
				expected.DrawTextPx(536, 220, name, artInkGov)
			} else {
				if len(name)*SmallW > 80 {
					t.Fatalf("主事者完整姓名放不下：%q", name)
				}
				expected.DrawSmallTextPx(536, 223, name, artInkGov)
			}
			r := image.Rect(536, 212, 616, 244)
			if !bytes.Equal(localeRegion(c.Img, r), localeRegion(expected.Img, r)) {
				t.Errorf("主事者 %q 未完整顯示", name)
			}
		}
	})
	t.Logf("實際主事者案例 %d，完整字模 %d 組", cases, len(seen))
}

func TestArtBigNameFontFallback(t *testing.T) {
	c := testCanvasPx(t, 200, 80)
	expected := testCanvasPx(t, 200, 80)
	for _, tc := range []struct {
		name  string
		small bool
	}{
		{"Cao Cao", false},
		{"Gongsun Zan", true},
		{"Zhuge Shang", true},
		{"Bob龘 Name", false},
	} {
		c.Fill(bg)
		expected.Fill(bg)
		artBigName(c, 20, 20, 80, tc.name, fg)
		if tc.small {
			expected.DrawSmallTextPx(20, 31, tc.name, fg)
		} else {
			expected.DrawTextPx(20, 28, cells.Truncate(tc.name, 10), fg)
		}
		if !bytes.Equal(c.Img.Pix, expected.Img.Pix) {
			t.Errorf("%q 字型、位置或槽外像素不同", tc.name)
		}
	}
	// 未載入小字時，保留既有正常字級的回退。
	c.SetSmallFace(nil)
	c.Fill(bg)
	expected.Fill(bg)
	artBigName(c, 20, 20, 80, "Gongsun Zan", fg)
	expected.DrawTextPx(20, 28, "Gongsun Za", fg)
	if !bytes.Equal(c.Img.Pix, expected.Img.Pix) {
		t.Error("沒有小字時未保留原回退")
	}
}
