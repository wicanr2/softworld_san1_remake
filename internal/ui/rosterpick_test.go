package ui

import (
	"image/color"
	imgpng "image/png"
	"os"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
)

// TestRosterRowFollowsTheOriginalFormat 釘住一列的三段字（`0x17d0e` 的格式）。
func TestRosterRowFollowsTheOriginalFormat(t *testing.T) {
	x := &game.General{Name: "劉備", Intel: 76, Loyalty: 100, Soldiers: 3000, Arms: 45}
	for _, c := range []struct {
		key        game.PickKey
		lead, tail string
	}{
		{game.PickByIntel, " 3.", ".  76"},
		{game.PickByLoyalty, " 3.", ". --"},
		{game.PickByBoth, " 3.", ".3000. 45"},
		{game.PickByStatus, " 3.", "."},
	} {
		lead, name, tail := RosterRow(x, 2, c.key)
		if lead != c.lead || name != " 劉備 " || tail != c.tail {
			t.Errorf("鍵 %d：%q %q %q，該是 %q \" 劉備 \" %q", c.key, lead, name, tail, c.lead, c.tail)
		}
	}
}

// TestDosboxXRosterPanelMatches 拿 DOSBox-X 的錄影驗挑人清單的面板：`rec11` 第 18 步
// （查看→武將的「檢視那位」；那一輪之後幾步停在人物卡上，沒有第二張清單）。字格以外的底色與 `SIDEA` 外框
// 逐點相同，表頭與欄名兩處都有墨。dosgolem 那一側由 `TestZZPickListMatchesTheOriginal` 比過。
func TestDosboxXRosterPanelMatches(t *testing.T) {
	a, g := artSessionFixture(t)
	face := testFace(t)
	for _, path := range []string{
		"../../workplace/rec11/frames/018-3Return.png",
	} {
		f, err := os.Open(path)
		if err != nil {
			t.Skipf("沒有 %s（tools/dosboxx-record.sh 錄）", path)
		}
		im, err := imgpng.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		c := NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
		DrawRosterPick(c, a, g, &RosterPick{List: []int{-1}})
		text := func(x, y int) bool {
			return x >= rosterHeadX && x < 624 && y >= rosterHeadY && y < rosterRowY+RosterPageRows*CellH
		}
		bad, headInk, colInk := 0, 0, 0
		for y := rosterY0; y <= rosterY1; y++ {
			for x := rosterX0; x <= rosterX1; x++ {
				got := color.RGBAModel.Convert(im.At(x, y)).(color.RGBA)
				if text(x, y) {
					if got != assets.EGAPalette[rosterBG] && y < rosterHeadY+CellH {
						if x < rosterColX {
							headInk++
						} else {
							colInk++
						}
					}
					continue
				}
				if got != c.Img.RGBAAt(x, y) {
					bad++
				}
			}
		}
		if bad != 0 || headInk == 0 || colInk == 0 {
			t.Errorf("%s：字格以外 %d 點不同；表頭墨 %d、欄名墨 %d", path, bad, headInk, colInk)
			continue
		}
		t.Logf("%s：字格以外逐點相同；表頭墨 %d、欄名墨 %d", path, headInk, colInk)
	}
}
