package ui

import (
	"fmt"
	"image"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func TestFortSpotConfirmationFitsBeforeCursor(t *testing.T) {
	previous := i18n.Current
	defer func() { i18n.Current = previous }()
	face, small := testFace(t), testSmallFace(t)
	for _, ed := range []string{"base", "plus"} {
		cs, ab := mapCursorTestSource(t, ed)
		sc, err := state.LoadScenario(cs["DATA2"], state.Scenario3)
		if err != nil {
			t.Fatal(err)
		}
		p, err := sc.Prefecture(15)
		if err != nil {
			t.Fatal(err)
		}
		fld, err := battle.Load(p.BattleField, p.Neighbours)
		if err != nil {
			t.Fatal(err)
		}
		for _, locale := range i18n.Locales() {
			i18n.Current = locale
			line := i18n.S("fort.confirm")
			for _, marked := range []bool{false, true} {
				for frame := 0; frame < 6; frame++ {
					t.Run(fmt.Sprintf("%s/%s/marked=%t/frame%d", ed, locale, marked, frame), func(t *testing.T) {
						got := NewCanvasPx(640, 408, face)
						got.SetSmallFace(small)
						got.HD = mapCursorTestPack(t, ed, cs, ab)
						DrawArtFortSpot(got, ab, p.BattleField, fld, FortSpot{Col: 3, Row: 1, Marked: marked, Confirm: true, Input: InputCursor{On: true, Frame: frame}})
						want := NewCanvasPx(640, 408, face)
						want.SetSmallFace(small)
						want.Fill(assets.EGAPalette[1])
						if locale == "en" {
							if line != "Confirm(Y/N)" || cells.Width(line)*SmallW != 72 {
								t.Fatalf("完整英文已改動: %q", line)
							}
							want.DrawSmallTextPx(448, 335, line, assets.EGAPalette[12])
						} else {
							want.DrawTextPx(448, 332, line, assets.EGAPalette[12])
						}
						// 用來源 AND/OR 逐像素重生固定游標，確認沒有文字留在 identity 格。
						f := ab.cursor[frame]
						for y := 0; y < 16; y++ {
							for x := 0; x < 8; x++ {
								if want.Img.RGBAAt(536+x, 332+y) != assets.EGAPalette[1] {
									t.Fatal("期望文字進入游標")
								}
								index := (byte(1) & f.Mask.At(x, y)) | f.Sprite.At(x, y)
								want.Img.SetRGBA(536+x, 332+y, assets.EGAPalette[index])
							}
						}
						rect := image.Rect(448, 332, 616, 348)
						assertBattleLocaleRegion(t, got, want, rect, "完整確認文字與來源游標")
						native := got.Output(true)
						for y := rect.Min.Y; y < rect.Max.Y; y++ {
							for x := rect.Min.X; x < rect.Max.X; x++ {
								for sy := 0; sy < 4; sy++ {
									for sx := 0; sx < 4; sx++ {
										if native.RGBAAt(x*4+sx, y*4+sy) != want.Img.RGBAAt(x, y) {
											t.Fatalf("原生文字或游標不符 %d,%d", x, y)
										}
									}
								}
							}
						}
						if len(got.Missing) != 0 || got.Clipped != 0 {
							t.Fatalf("字模或裁切: %v %d", got.Missing, got.Clipped)
						}
					})
				}
			}
		}
	}
}

func TestFortSpotConfirmationMissingSmallFontBounds(t *testing.T) {
	previous := i18n.Current
	i18n.Current = "en"
	defer func() { i18n.Current = previous }()
	face := testFace(t)
	cs, ab := mapCursorTestSource(t, "base")
	sc, err := state.LoadScenario(cs["DATA2"], state.Scenario3)
	if err != nil {
		t.Fatal(err)
	}
	p, err := sc.Prefecture(15)
	if err != nil {
		t.Fatal(err)
	}
	fld, err := battle.Load(p.BattleField, p.Neighbours)
	if err != nil {
		t.Fatal(err)
	}
	got := NewCanvasPx(640, 408, face)
	DrawArtFortSpot(got, ab, p.BattleField, fld, FortSpot{Confirm: true})
	want := NewCanvasPx(640, 408, face)
	want.Fill(assets.EGAPalette[1])
	want.DrawTextPx(448, 332, "Confirm(Y/N", assets.EGAPalette[12])
	assertBattleLocaleRegion(t, got, want, image.Rect(448, 332, 616, 348), "缺小字的既有安全截短回退")
}
