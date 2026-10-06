package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

// 句尾及整張畫布由固定完整句與自由字模重建，不能從實際墨點抄答案。
func TestLongBubbleTranslationDrawsWholeSentence(t *testing.T) {
	old := i18n.Current
	t.Cleanup(func() { i18n.Current = old })
	standard, small := testFace(t), testSmallFace(t)
	tests := []struct {
		locale    i18n.Locale
		key, name string
		lines     []string
		small     bool
	}{
		{i18n.En, "bub.duelChallenge", "陳宮", []string{"Chen Gong,", "come out and", "fight me to", "the death"}, false},
		{i18n.En, "bub.duelAccept", "呂布", []string{"Lu Bu, you", "rat, do you", "think I fear", "you"}, false},
		{i18n.Ja, "bub.duelChallenge", "陳宮", []string{"陳宮 出てき", "て我と決死の", "勝負を"}, false},
		{i18n.En, "bub.captiveRefuse", "淳于瓊", []string{"Chunyu Qiong,", "though unworthy,", "will not yield", "to rebels"}, true},
	}
	for _, tc := range tests {
		for _, box := range [][4]int{{424, 80, 615, 175}, {448, 44, 623, 139}, {64, 268, 239, 363}} {
			for _, left := range []bool{false, true} {
				b := &game.Bubble{X1: box[0], Y1: box[1], X2: box[2], Y2: box[3], Left: left, Color: 4, Text: i18n.Tf(tc.locale, tc.key, i18n.PersonNameFor(tc.locale, tc.name))}
				c := NewCanvasPx(assets.ScreenW, assets.ScreenH, standard)
				c.SetSmallFace(small)
				c.FillRect(0, 0, assets.ScreenW, assets.ScreenH, assets.EGAPalette[1])
				// 同一泡泡的空白控制只提供背景、尾巴及姓名牌。
				blank := *b
				blank.Text = ""
				DrawBubbleAs(c, nil, &blank, "陳宮", -1)
				expected := image.NewRGBA(c.Img.Bounds())
				draw.Draw(expected, expected.Bounds(), c.Img, image.Point{}, draw.Src)
				x := b.X1 + 8
				if left {
					x = b.X1 + 72
				}
				face, advance, height, lines := standard, 8, 16, tc.lines
				if tc.small {
					face, advance, height = small, 6, 10
				}
				if box[0] == 424 {
					if tc.locale == i18n.Ja {
						lines = []string{"陳宮 出てきて", "我と決死の勝負", "を"}
					}
					if tc.small {
						face, advance, height = standard, 8, 16
						lines = []string{"Chunyu Qiong,", "though", "unworthy, will", "not yield to", "rebels"}
					}
				}
				for row, line := range lines {
					bubbleExpectedGlyphs(t, expected, face, line, x, b.Y1+8+row*height, advance, assets.EGAPalette[4])
				}
				i18n.Current = tc.locale
				DrawBubbleAs(c, nil, b, "陳宮", -1)
				if !bytes.Equal(c.Img.Pix, expected.Pix) {
					t.Fatalf("%s %s left=%t box=%v：完整字模或框外畫面不同", tc.locale, tc.key, left, box)
				}
				if len(c.Missing) != 0 {
					t.Fatalf("缺字：%v", c.Missing)
				}
			}
		}
	}
}

func bubbleExpectedGlyphs(t *testing.T, dst *image.RGBA, face *font.Face, text string, x, y, advance int, ink color.RGBA) {
	t.Helper()
	for _, r := range text {
		g, ok := face.Glyph(r)
		if !ok {
			t.Fatalf("缺少 %U", r)
		}
		for gy := 0; gy < g.H; gy++ {
			for gx := 0; gx < g.W; gx++ {
				if g.At(gx, gy) {
					dst.SetRGBA(x+gx, y+gy, ink)
				}
			}
		}
		x += advance * cells.RuneWidth(r)
	}
}

func TestBubbleTranslationLayoutPreservesTextAndFallback(t *testing.T) {
	old := i18n.Current
	t.Cleanup(func() { i18n.Current = old })
	c := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
	b := &game.Bubble{X1: 448, Y1: 44, X2: 623, Y2: 139}
	compact := func(s string) string { return strings.Join(strings.Fields(s), "") }
	for _, locale := range []i18n.Locale{i18n.En, i18n.Ja} {
		i18n.Current = locale
		for _, key := range i18n.Keys() {
			if !strings.HasPrefix(key, "bub.") {
				continue
			}
			// 102 個真實模板，使用量測中最長的姓名及整數壓力值。
			text := strings.ReplaceAll(i18n.T(locale, key), "%s", i18n.PersonNameFor(locale, "刑道榮"))
			b.Text = strings.ReplaceAll(text, "%d", "999")
			p := planBubbleText(c, b)
			if compact(strings.Join(p.lines, "")) != compact(b.Text) {
				t.Fatalf("%s %s 丟失文字", locale, key)
			}
			if p.top+(len(p.lines)-1)*p.gap+16*p.sy > b.Y2-b.Y1-4 && !p.small {
				t.Fatalf("%s %s 高度溢出", locale, key)
			}
		}
	}
	i18n.Current = i18n.ZhHant
	b.Text = "兵貴神速  可使敵人措手不及"
	p := planBubbleText(c, b)
	legacy := BubbleLines(b)
	if p.top != 12 || p.gap != 40 || p.sy != 2 || p.small || strings.Join(p.lines, "|") != strings.Join(legacy[:], "|") {
		t.Fatal("繁中原版排法改變")
	}
	i18n.Current = i18n.En
	b.Text = "Hold"
	p = planBubbleText(c, b)
	if p.sy != 2 || p.top != 12 || p.gap != 40 {
		t.Fatal("可放下的譯文改變")
	}
	c.SetSmallFace(nil)
	b.Text = i18n.Tf(i18n.En, "bub.captiveRefuse", "Chunyu Qiong")
	p = planBubbleText(c, b)
	if p.small || p.sy != 2 || p.top != 12 {
		t.Fatal("缺少小字時應保留既有回退")
	}
}
