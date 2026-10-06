package ui

import (
	"fmt"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"image"
	"testing"
)

func TestBattlePageDisplaysCompleteShellCancelHint(t *testing.T) {
	previous := i18n.Current
	defer func() { i18n.Current = previous }()
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(artContainer(t, "DATA2"), state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	for _, pid := range []int{13, 11} {
		p, err := sc.Prefecture(pid)
		if err != nil {
			t.Fatal(err)
		}
		fld, err := battle.Load(p.BattleField, p.Neighbours)
		if err != nil {
			t.Fatal(err)
		}
		b := battle.New(battle.Setup{Field: fld, Seed: 1})
		for _, locale := range i18n.Locales() {
			for _, scroll := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/%s/scroll=%t", pid, locale, scroll), func(t *testing.T) {
					i18n.Current = locale
					body := []string{"1234567890"}
					key := "window.hint.page"
					if scroll {
						for len(body) < 40 {
							body = append(body, "1234567890")
						}
						key = "window.hint.pageScroll"
					}
					view := BattleView{PageTitle: "123", Page: body}
					got := testCanvasPx(t, 640, 408)
					DrawArtBattle(got, ab, b, view, ArtBattleInfo{Field: p.BattleField})
					const cols = (battlePageX1-battlePageX0)/CellW - 2
					const rows = (battlePageY1-battlePageY0)/CellH - 2
					hint := i18n.S(key)
					if cells.Width(hint) > cols {
						t.Fatalf("完整取消提示超寬: %q", hint)
					}
					x, y := battlePageX0+CellW, battlePageY0+(rows+1)*CellH
					rect := image.Rect(x, y, x+cols*CellW, y+CellH)
					want := testCanvasPx(t, 640, 408)
					want.Fill(artInkPageBG)
					want.DrawTextPx(x, y, hint, artInkPageDim)
					assertBattleLocaleRegion(t, got, want, rect, "分頁完整 Shift+Esc 提示")
				})
			}
		}
	}
}

func TestFortSpotDisplaysShellCancelAndKeepsConfirmation(t *testing.T) {
	previous := i18n.Current
	defer func() { i18n.Current = previous }()
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(artContainer(t, "DATA2"), state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	p, err := sc.Prefecture(13)
	if err != nil {
		t.Fatal(err)
	}
	fld, err := battle.Load(p.BattleField, p.Neighbours)
	if err != nil {
		t.Fatal(err)
	}
	for _, locale := range i18n.Locales() {
		for _, confirm := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/confirm=%t", locale, confirm), func(t *testing.T) {
				i18n.Current = locale
				got := testCanvasPx(t, 640, 408)
				DrawArtFortSpot(got, ab, p.BattleField, fld, FortSpot{Confirm: confirm})
				line, ink := i18n.S("window.fort.help5"), fortSpotHelpInk[4]
				if confirm {
					line, ink = i18n.S("fort.confirm"), fortSpotConfirmInk
				}
				if cells.Width(line)*CellW > assets.BattlePanelW {
					t.Fatalf("完整築寨提示超寬: %q", line)
				}
				rect := image.Rect(fortSpotHelpX, fortSpotHelpY+4*CellH, fortSpotHelpX+assets.BattlePanelW, fortSpotHelpY+5*CellH)
				want := testCanvasPx(t, 640, 408)
				want.Fill(assets.EGAPalette[fortSpotHelpBG])
				if confirm && locale == "en" {
					want.DrawSmallTextPx(rect.Min.X, rect.Min.Y+3, line, assets.EGAPalette[ink])
				} else {
					want.DrawTextPx(rect.Min.X, rect.Min.Y, line, assets.EGAPalette[ink])
				}
				assertBattleLocaleRegion(t, got, want, rect, "築寨第五行完整提示及確認")
			})
		}
	}
}
