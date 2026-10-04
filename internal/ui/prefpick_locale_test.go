package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

// 完整字串直接畫成預期圖，不能先截短再證明排得進。
func prefPickLocaleExpected(t *testing.T, c *Canvas, a *ArtScreen, g *game.State, p *PrefPick, small bool) {
	t.Helper()
	c.Fill(bg)
	c.FillRect(408, 36, 632, 292, assets.EGAPalette[1])
	drawSideFrame(c, a.prefBox, 408, 36, 224, 256)
	if small {
		heads := strings.Fields(tStringPrefPick("pick.prefHead"))
		if len(heads) != 3 {
			t.Fatal("三欄表頭", heads)
		}
		for k, h := range heads {
			if !c.FitsSmall(h) || cells.Width(h)*6 > 66 {
				t.Fatal("完整表頭放不下", h)
			}
			c.DrawSmallTextPx(424+k*66, 47, h, assets.EGAPalette[15])
		}
	} else {
		c.DrawTextPx(424, 44, cells.Truncate(tStringPrefPick("pick.prefHead"), 24), assets.EGAPalette[15])
	}
	for id := 1; id <= 42; id++ {
		name := PlaceName(g.Prefecture(id).Name)
		text := fmt.Sprintf("%2d", id) + name
		ink := 6
		if p.Valid[id] {
			ink = 14
		}
		y := 60 + (id-1)%14*16
		if small {
			x := 424 + (id-1)/14*66
			if c.FitsSmall(text) {
				if cells.Width(text)*6 > 66 {
					t.Fatal("完整地名放不下", text)
				}
				c.DrawSmallTextPx(x, y+3, text, assets.EGAPalette[ink])
			} else {
				if cells.Width(text)*8 > 66 {
					t.Fatal("混合文字測例放不下", text)
				}
				c.DrawTextPx(x, y, text, assets.EGAPalette[ink])
			}
		} else {
			if i18n.Current == i18n.En {
				text = fmt.Sprintf("%2d", id) + cells.Truncate(name, 6)
			} else if cells.Width(text)*8 > 64 {
				t.Fatal("完整中日地名放不下", text)
			}
			c.DrawTextPx(432+(id-1)/14*64, y, text, assets.EGAPalette[ink])
		}
	}
}

func tStringPrefPick(key string) string { return i18n.T(i18n.Current, key) }

func TestPrefPickCompleteAcrossScenarios(t *testing.T) {
	a, _ := artSessionFixture(t)
	c := testCanvasPx(t, 640, 408)
	want := testCanvasPx(t, 640, 408)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	cases, scenarios := 0, 0
	localeScenarioGames(t, func(g *game.State) {
		scenarios++
		p := &PrefPick{}
		for id := 1; id <= 42; id++ {
			p.Valid[id] = id%2 == 0
		}
		originalValid := p.Valid
		for _, locale := range i18n.Locales() {
			i18n.Current = locale
			c.Fill(bg)
			DrawPrefPick(c, a, g, p)
			prefPickLocaleExpected(t, want, a, g, p, locale == i18n.En)
			if !bytes.Equal(c.Img.Pix, want.Img.Pix) {
				t.Errorf("劇本樣本 %d、%s 的完整清單、編號、字色或框線不同", scenarios, locale)
			}
			if len(c.Missing) != 0 || p.Valid != originalValid {
				t.Fatal("缺字或顯示改動選擇條件", c.Missing)
			}
			cases += 42
		}
	})
	if scenarios != 12 || cases != 1512 {
		t.Fatal("兩版六劇本三語分母不足", scenarios, cases)
	}
	t.Logf("兩版六劇本三語完整列 %d，所有編號與兩種字色", cases)
}

func TestPrefPickFontAndMixedNameFallback(t *testing.T) {
	a, g := artSessionFixture(t)
	c := testCanvasPx(t, 640, 408)
	want := testCanvasPx(t, 640, 408)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	i18n.Current = i18n.En
	p := &PrefPick{}
	for id := 1; id <= 42; id++ {
		p.Valid[id] = true
	}
	c.SetSmallFace(nil)
	c.Fill(bg)
	DrawPrefPick(c, a, g, p)
	prefPickLocaleExpected(t, want, a, g, p, false)
	if !bytes.Equal(c.Img.Pix, want.Img.Pix) {
		t.Error("未載小字時的原字級及框線回退不同")
	}
	c.SetSmallFace(want.small)
	g.Prefecture(1).Name = "Bob龘"
	c.Fill(bg)
	DrawPrefPick(c, a, g, p)
	prefPickLocaleExpected(t, want, a, g, p, true)
	if !bytes.Equal(c.Img.Pix, want.Img.Pix) || g.Prefecture(1).Name != "Bob龘" {
		t.Error("未知混合地名被猜譯、資料改寫或小字漏畫")
	}
}
