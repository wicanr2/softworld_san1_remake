package ui

import (
	"image/color"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestDrawPersonCardWithoutArt 釘住沒有原版素材也畫得出資料卡：灰底蓋滿
// 右側面板、每一行字落在原版的 y、身分那一行照身分挑落點與字色、
// 只有在職的（`任%s%s`）才有忠心度那一行。
func TestDrawPersonCardWithoutArt(t *testing.T) {
	c := NewCanvasPx(assets.ScreenW, assets.ScreenH, testFace(t))
	g := loadGame(t)
	grey := assets.EGAPalette[7]
	at := func(x, y int) color.RGBA { return c.Img.RGBAAt(x, y) }
	// 一行文字區裡某一色的墨點數；x0 起算。
	inkOf := func(y, h, x0, x1 int, ink int) int {
		n := 0
		for yy := y; yy < y+h; yy++ {
			for xx := x0; xx < x1; xx++ {
				if at(xx, yy) == assets.EGAPalette[ink] {
					n++
				}
			}
		}
		return n
	}
	// 找三種身分：君主、在職的部將、在野。
	lord, officer, free := -1, -1, -1
	for i, x := range g.AllGenerals() {
		if x == nil || x.Name == "" {
			continue
		}
		switch {
		case x.Status == state.StatusLord && lord < 0:
			lord = i
		case x.Status == state.StatusOfficer && officer < 0:
			officer = i
		case x.Status == state.StatusAvailable && free < 0:
			free = i
		}
	}
	if lord < 0 || officer < 0 || free < 0 {
		t.Fatalf("劇本裡找不到三種身分：君主 %d、部將 %d、在野 %d", lord, officer, free)
	}

	DrawPersonCard(c, nil, g, lord)
	for _, p := range [][2]int{{408, 36}, {631, 291}, {520, 250}} {
		if at(p[0], p[1]) != grey {
			t.Errorf("(%d,%d) 該是灰底", p[0], p[1])
		}
	}
	if at(407, 36) == grey || at(632, 291) == grey {
		t.Error("灰底越過了 (408,36)–(631,291)")
	}
	// 名字 32×32 黃色從 (424,52) 起；籍貫淺青在 y 84；君主那一行白色從
	// 440 起（「  現為君主  」前面兩個半形空白）；沒有忠心度；年齡淺綠 y 132。
	if inkOf(52, 32, 424, 528, 14) == 0 {
		t.Error("名字沒有畫成黃色的 32×32")
	}
	if inkOf(84, 16, 424, 528, 11) == 0 {
		t.Error("籍貫那一行沒有淺青的字")
	}
	if inkOf(100, 16, 424, 440, 15) != 0 || inkOf(100, 16, 440, 528, 15) == 0 {
		t.Error("君主那一行的白字該從 440 起")
	}
	if inkOf(116, 16, 424, 528, 12) != 0 {
		t.Error("君主不該有忠心度那一行")
	}
	if inkOf(132, 16, 424, 528, 10) == 0 {
		t.Error("年齡那一行沒有淺綠的字")
	}
	for _, y := range []int{164, 180, 196, 212} {
		if inkOf(y, 16, 424, 624, 9) == 0 {
			t.Errorf("y %d 那一行沒有藍字", y)
		}
	}

	// 在職的部將：`任%s%s` 黃色從 424 起，忠心度淺紅在 y 116。
	c.FillRect(0, 0, assets.ScreenW, assets.ScreenH, color.RGBA{0, 0, 0, 0xFF})
	DrawPersonCard(c, nil, g, officer)
	if inkOf(100, 16, 424, 440, 14) == 0 {
		t.Error("部將那一行該是黃字、從 424 起")
	}
	if inkOf(116, 16, 424, 528, 12) == 0 {
		t.Error("部將該有淺紅的忠心度那一行")
	}

	// 在野：只畫身分名，藍色（9）從 456 起，沒有忠心度。
	c.FillRect(0, 0, assets.ScreenW, assets.ScreenH, color.RGBA{0, 0, 0, 0xFF})
	DrawPersonCard(c, nil, g, free)
	if inkOf(100, 16, 424, 456, 9) != 0 || inkOf(100, 16, 456, 528, 9) == 0 {
		t.Error("在野那一行的藍字該從 456 起")
	}
	if inkOf(116, 16, 424, 528, 12) != 0 {
		t.Error("在野不該有忠心度那一行")
	}
}

// TestCardFitsEveryLanguage 釘住三種語言的資料卡每一行都放得進它的槽
// （肖像旁 13 格、下半 25 格）、不壓到肖像框、不疊行；名字放得進肖像框前
// 的槽位（13 格）。
func TestCardFitsEveryLanguage(t *testing.T) {
	g := loadGame(t)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		for i, x := range g.AllGenerals() {
			if x == nil || x.Name == "" {
				continue
			}
			seen := map[int]bool{}
			for _, ln := range cardLines(g, x) {
				if w := cells.Width(ln.text); w > ln.cols {
					t.Errorf("%s 人物 %d 的 y %d「%s」%d 格，槽位只有 %d 格", l, i, ln.y, ln.text, w, ln.cols)
				}
				if ln.y < cardFaceBotY && ln.x+ln.cols*CellW > cardFaceX-8 {
					t.Errorf("%s 人物 %d 的 y %d 那一行的槽 %d 格壓到肖像框", l, i, ln.y, ln.cols)
				}
				if seen[ln.y] {
					t.Errorf("%s 人物 %d 的 y %d 排了兩行", l, i, ln.y)
				}
				seen[ln.y] = true
			}
			name := PersonName(x.Name)
			slot := (cardFaceX - 8 - cardNameX) / CellW
			w := cells.Width(name)
			if artAllWide(name) {
				w = cells.Width(paddedName(name)) * 2
			}
			if w > slot {
				t.Errorf("%s 人物 %d 的名字 %q %d 格，槽位只有 %d 格", l, i, name, w, slot)
			}
		}
	}
}
