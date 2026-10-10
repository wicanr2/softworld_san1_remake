package ui

import (
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"image"
	"testing"
)

func assertSelectionPhase(t *testing.T, a, b *Canvas, rectangles []image.Rectangle) {
	t.Helper()
	changed := 0
	for y := 0; y < assets.ScreenH; y++ {
		for x := 0; x < assets.ScreenW; x++ {
			before, after := a.Img.RGBAAt(x, y), b.Img.RGBAAt(x, y)
			inside := false
			for _, r := range rectangles {
				if image.Pt(x, y).In(r) {
					inside = true
				}
			}
			if !inside {
				if before != after {
					t.Fatalf("unexpected change outside selection at %d,%d", x, y)
				}
				continue
			}
			found := false
			for i, col := range assets.EGAPalette {
				if col == before {
					found = true
					if after != assets.EGAPalette[i^15] {
						t.Fatalf("selection is not indexed XOR 15 at %d,%d", x, y)
					}
				}
			}
			if !found {
				t.Fatalf("non-indexed selection color at %d,%d", x, y)
			}
			if before != after {
				changed++
			}
		}
	}
	if changed == 0 {
		t.Fatal("selection stayed fixed instead of alternating")
	}
}

func TestCurrentPrefectureBlinkUsesTurnNotInspection(t *testing.T) {
	art, g := artSessionFixture(t)
	for _, id := range []int{1, 11, 41} {
		p := g.Prefecture(id)
		a, b := testCanvasPx(t, 640, 408), testCanvasPx(t, 640, 408)
		v := View{Sel: 21, CurrentPrefecture: id}
		DrawArtSession(a, art, g, nil, v)
		v.SelectionBlink = true
		DrawArtSession(b, art, g, nil, v)
		x, y := int(p.MapX)+80, int(p.MapY)+44
		assertSelectionPhase(t, a, b, []image.Rectangle{image.Rect(x, y, x+16, y+9)})
	}
}

func TestMainBattleFlagAndSoldierPlateBlink(t *testing.T) {
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(artContainer(t, "DATA2"), state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	p, err := sc.Prefecture(25)
	if err != nil {
		t.Fatal(err)
	}
	fld, err := battle.Load(p.BattleField, p.Neighbours)
	if err != nil {
		t.Fatal(err)
	}
	b := battle.New(battle.Setup{Field: fld, Seed: 1})
	u := &battle.Unit{Side: battle.MainAttacker, Leaders: []battle.Leader{{Name: "測試", Soldiers: 1000}}, At: battle.FromOffset(2, 2)}
	b.Units = []*battle.Unit{u}
	a, c := testCanvasPx(t, 640, 408), testCanvasPx(t, 640, 408)
	v := BattleView{Acting: u}
	info := ArtBattleInfo{Field: p.BattleField, Portrait: [2]int{-1, -1}}
	DrawArtBattle(a, ab, b, v, info)
	v.Blink = true
	DrawArtBattle(c, ab, b, v, info)
	x, y := assets.FlagCell(2, 2)
	flag := ab.flags[u.Side.OriginalIndex()][u.Formation.OriginalIndex()]
	assertSelectionPhase(t, a, c, []image.Rectangle{image.Rect(x, y, x+flag.W, y+flag.H), image.Rect(x, y+15, x+40, y+32)})
}
