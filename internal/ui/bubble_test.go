package ui

import (
	"image/color"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
)

// TestBubbleLinesSplitLikeTheOriginal 釘住兩行的切法：一行七個全形字
// （框寬 (615 − 424 − 79) ÷ 16），放不下的接到第二行、切在字的邊界上。
func TestBubbleLinesSplitLikeTheOriginal(t *testing.T) {
	b := &game.Bubble{X1: 424, Y1: 80, X2: 615, Y2: 175, Text: "主公 呂蒙 隨某加入"}
	if n := BubbleColumns(b); n != 7 {
		t.Fatalf("一行 %d 個全形字，該是 7", n)
	}
	lines := BubbleLines(b)
	if lines[0] != "主公 呂蒙 隨某" || lines[1] != "加入" {
		t.Errorf("切成 %q，該是「主公 呂蒙 隨某」／「加入」", lines)
	}
	// 英文在空白處折，不切字。
	b.Text = "My lord, Lü Meng joins us with me"
	lines = BubbleLines(b)
	if lines[0] != "My lord, Lü" || lines[1] != "Meng joins us" {
		t.Errorf("英文切成 %q，該在空白處折成「My lord, Lü」／「Meng joins us」", lines)
	}
}

// TestDrawBubbleWithoutArt 釘住沒有原版素材也畫得出泡泡：白底落在
// 原版的位置、尾巴朝肖像那一側、字色是擲出來的那一格。
func TestDrawBubbleWithoutArt(t *testing.T) {
	c := NewCanvasPx(assets.ScreenW, assets.ScreenH, testFace(t))
	blue := assets.EGAPalette[1]
	c.FillRect(408, 36, 632, 292, blue)
	g := loadGame(t)
	b := &game.Bubble{X1: 424, Y1: 180, X2: 615, Y2: 275, Left: true, Speaker: 0, Color: 4, Text: "吾命休矣"}
	DrawBubble(c, nil, g, b)
	white := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	at := func(x, y int) color.RGBA { return c.Img.RGBAAt(x, y) }
	// 白底 (494,185)–(610,270)，上下各多一條、左右各一條。
	for _, p := range [][2]int{{494, 185}, {610, 270}, {494, 184}, {610, 271}, {493, 185}, {611, 270}} {
		if at(p[0], p[1]) != white {
			t.Errorf("(%d,%d) 該是白的", p[0], p[1])
		}
	}
	for _, p := range [][2]int{{493, 184}, {611, 271}, {492, 200}, {612, 200}, {494, 183}} {
		if at(p[0], p[1]) != blue {
			t.Errorf("(%d,%d) 該還是藍底", p[0], p[1])
		}
	}
	// 尾巴：x1+65 只有 y1+47、y1+48 兩格，x1+68 有 y1+44..y1+51。
	if at(489, 227) != white || at(489, 228) != white || at(489, 226) != blue {
		t.Error("尾巴的尖端不在 (489,227–228)")
	}
	if at(492, 224) != white || at(492, 231) != white || at(492, 223) != blue {
		t.Error("尾巴的根部不是 (492,224–231)")
	}
	// 名字的黑底 48×16 在 (432,260)。
	if at(432, 260) != (color.RGBA{0, 0, 0, 0xFF}) || at(479, 275) != (color.RGBA{0, 0, 0, 0xFF}) {
		t.Error("名字的黑底不在 (432,260)–(479,275)")
	}
	// 對白第一行有擲出來那一色的墨。
	ink := 0
	for y := 192; y < 224; y++ {
		for x := 496; x < 610; x++ {
			if at(x, y) == assets.EGAPalette[4] {
				ink++
			}
		}
	}
	if ink == 0 {
		t.Error("對白沒有畫出色 4 的字")
	}
}
