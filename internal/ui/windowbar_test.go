package ui

import (
	"image"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

func TestWindowBarHeightPreservesGameScale(t *testing.T) {
	for _, tc := range []struct {
		width, height, gameHeight, delta int
	}{
		{640, 408, 408, 32},
		{2560, 1632, 408, 128},
		{1280, 600, 408, 47},
		{640, 800, 408, 32},
		{640, 400, 400, 32},
	} {
		if got := WindowBarHeightDelta(tc.width, tc.height, tc.gameHeight, false); got != tc.delta {
			t.Fatalf("展開 %dx%d：加高 %d，預期 %d", tc.width, tc.height, got, tc.delta)
		}
		if got := WindowBarHeightDelta(tc.width, tc.height+tc.delta, tc.gameHeight, true); got != tc.delta {
			t.Fatalf("收起 %dx%d：減高 %d，預期 %d", tc.width, tc.height+tc.delta, got, tc.delta)
		}
	}
}

func TestTitleLocalesStayInsideButtonFrames(t *testing.T) {
	old := i18n.Current
	defer func() { i18n.Current = old }()
	for _, locale := range i18n.Locales() {
		i18n.Current = locale
		for i, label := range TitleItems() {
			c := testCanvasPx(t, 640, 408)
			c.Fill(bg)
			b := assets.MenuButtons()[i]
			DrawMenuItem(c, b[0], b[1], label, fg)
			_, _, x1, _, n := inkBox(c, fg)
			if n == 0 || x1 >= b[0]+TitleLayerTextX+TitleLayerTextCells*CellW || len(c.Missing) != 0 {
				t.Fatalf("%s 的 %q 超出按鈕或缺字：x=%d missing=%v", locale, label, x1, c.Missing)
			}
		}
	}
}

func TestWindowBarRevealAndPin(t *testing.T) {
	b := NewWindowBarState()
	if b.Visible {
		t.Fatal("預設顯示")
	}
	b.Reveal(false, image.Pt(100, 5), 0)
	if !b.Visible || b.Pinned {
		t.Fatal("上緣未展開")
	}
	b.Reveal(false, image.Pt(100, 80), 0)
	if b.Visible {
		t.Fatal("移開未隱藏")
	}
	b.Reveal(true, image.Pt(100, 80), 0)
	if !b.Visible || !b.Pinned {
		t.Fatal("Esc 未鎖定展開")
	}
	b.Reveal(false, image.Pt(100, 80), 0)
	if !b.Visible {
		t.Fatal("鍵盤展開被滑鼠收掉")
	}
	b.Reveal(true, image.Pt(100, 2), 0)
	if b.Visible {
		t.Fatal("Esc 未隱藏")
	}
	b.Reveal(false, image.Pt(100, 2), 0)
	if b.Visible {
		t.Fatal("隱藏後立即被上緣重開")
	}
	b.Reveal(false, image.Pt(100, 80), 0)
	b.Reveal(false, image.Pt(100, 2), 0)
	b.Menu = 0
	b.Reveal(false, image.Pt(100, 50), 3)
	if !b.Visible {
		t.Fatal("移到下拉選單意外隱藏")
	}
	b.Reveal(false, image.Pt(220, 50), 3)
	if b.Visible {
		t.Fatal("移開下拉選單未隱藏")
	}
}

func TestWindowBarHitAndLocales(t *testing.T) {
	v := WindowBarView{Menu: 0, Lists: [3][]string{{"繁體中文", "English", "日本語"}, {"原貌", "B 高清 4×"}, {"還原", "強化 1", "強化 2", "強化 3", "強化 4", "強化 5"}}}
	if f, r := WindowBarHit(image.Pt(100, WindowBarHeight+WindowBarRow+1), v); f != 0 || r != 1 {
		t.Fatalf("下拉選取=%d,%d", f, r)
	}
	if f, _ := WindowBarHit(image.Pt(639, 407), v); f != -1 {
		t.Fatal("遊戲被當成選項列")
	}
	old := i18n.Current
	defer func() { i18n.Current = old }()
	for _, l := range i18n.Locales() {
		i18n.Current = l
		c := testCanvasPx(t, 640, WindowBarCanvasHeight)
		for _, field := range []int{-1, 0, 1, 2} {
			v.Menu = field
			DrawWindowBar(c, v)
			if len(c.Missing) > 0 || c.Clipped > 0 {
				t.Fatalf("%s 缺字或裁切: %v %d", l, c.Missing, c.Clipped)
			}
		}
	}
}
