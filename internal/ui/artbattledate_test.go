package ui

import (
	"image"
	"image/color"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
)

// 主戰場下方花邊上那一行年月的版面（`docs/spec/011`）。
//
// 基準是原版畫面上那一個樣本：**建安二年九月秋**，十格裡有字的是
// 0、1、3、5、7、8、9。
func TestBattleDateCellsMatchTheOriginalSample(t *testing.T) {
	got := battleDateCells(game.Date{Year: 197, Month: 9}, game.ChineseEra)
	want := [assets.BattleDateCells]string{
		"建", "安", "", "二", "", "年", "", "九", "月", "秋",
	}
	if got != want {
		t.Errorf("建安二年九月的十格 ＝ %q，原版量到 %q", got, want)
	}
}

// 靠右對齊：兩位數佔兩格、個位數只佔右邊那一格。
func TestBattleDateCellsAlignRight(t *testing.T) {
	for _, tc := range []struct {
		name       string
		year, mon  int
		y0, y1     string // 格 2、格 3
		m0, m1     string // 格 6、格 7
	}{
		{"建安元年元月", 196, 1, "", "元", "", "元"},
		{"建安二年九月", 197, 9, "", "二", "", "九"},
		{"建安十三年十月", 208, 10, "十", "三", "", "十"},
		{"建安十三年十二月", 208, 12, "十", "三", "十", "二"},
		// 年數三個中文字（「二十一」）放不進兩格——**沒有樣本**，
		// 退回半形數字（見 battleDateCells 的 ⚠）。
		{"建安二十一年", 216, 5, "2", "1", "", "五"},
	} {
		c := battleDateCells(game.Date{Year: tc.year, Month: tc.mon}, game.ChineseEra)
		if c[2] != tc.y0 || c[3] != tc.y1 {
			t.Errorf("%s：年數格 ＝ %q %q，想要 %q %q",
				tc.name, c[2], c[3], tc.y0, tc.y1)
		}
		if c[6] != tc.m0 || c[7] != tc.m1 {
			t.Errorf("%s：月份格 ＝ %q %q，想要 %q %q",
				tc.name, c[6], c[7], tc.m0, tc.m1)
		}
		// 固定的三格不會變。
		if c[4] != "" {
			t.Errorf("%s：格 4 應該一直是空的，卻有 %q", tc.name, c[4])
		}
		if c[5] == "" || c[8] == "" || c[9] == "" {
			t.Errorf("%s：「年」「月」與季節那三格不該是空的（%q %q %q）",
				tc.name, c[5], c[8], c[9])
		}
	}
}

// 年號表涵蓋不到的年份退回西曆，不硬掰一個年號出來。
func TestBattleDateFallsBackToWestern(t *testing.T) {
	c := battleDateCells(game.Date{Year: 400, Month: 3}, game.ChineseEra)
	if c[0] != "4" || c[1] != "0" || c[2] != "0" {
		t.Errorf("年號查不到時前三格 ＝ %q %q %q，想要 4 0 0", c[0], c[1], c[2])
	}
	w := battleDateCells(game.Date{Year: 197, Month: 9}, game.Western)
	if w[0] != "1" || w[9] == "" {
		t.Errorf("西曆的十格 ＝ %q", w)
	}
}

// 字用兩色棋盤畫：`(x+y)` 奇數一個色、偶數另一個色。
//
// **不是「主色 ＋ 陰影」**——原版量到右下 (+1,+1) 只有 4 點，
// 而 (+1,0) 有 388、(0,+1) 有 329。
func TestDrawRuneBoxDitherAlternates(t *testing.T) {
	face := testFace(t)
	c := NewCanvasPx(64, 64, face)
	bg := color.RGBA{0, 0, 0, 255}
	c.Fill(bg)
	odd := color.RGBA{170, 170, 170, 255}
	even := color.RGBA{0, 170, 0, 255}
	c.DrawRuneBoxDitherPx(8, 8, 24, 23, '年', odd, even)

	nOdd, nEven, bad := 0, 0, 0
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			switch c.Img.RGBAAt(x, y) {
			case bg:
			case odd:
				nOdd++
				if (x+y)%2 != 1 {
					bad++
				}
			case even:
				nEven++
				if (x+y)%2 != 0 {
					bad++
				}
			default:
				bad++
			}
		}
	}
	if nOdd == 0 || nEven == 0 {
		t.Fatalf("兩個色號沒有都用到（奇 %d、偶 %d）", nOdd, nEven)
	}
	if bad != 0 {
		t.Errorf("有 %d 個像素的相位不對——那就不是 (x+y) 棋盤了", bad)
	}
	// 縮放：畫出來的東西要落在方框內，一個像素都不准溢出。
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if c.Img.RGBAAt(x, y) == bg {
				continue
			}
			if !image.Pt(x, y).In(image.Rect(8, 8, 8+24, 8+23)) {
				t.Fatalf("(%d,%d) 畫到方框外面了", x, y)
			}
		}
	}
	// 缺字要記一筆，不要靜靜地不畫。
	before := len(c.Missing)
	c.DrawRuneBoxDitherPx(0, 0, 8, 8, '￿', odd, even)
	if len(c.Missing) == before {
		t.Error("畫不出來的字沒有記進 Missing")
	}
}
