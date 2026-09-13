package ui

import (
	"image/color"
	imgpng "image/png"
	"os"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// 原版主選單上每一行字佔的字格，量自 `workplace/shots/open/open-06.png`
// （`docs/playtest/03`）。**格是原版定的，字模是 remake 自己的**——
// 形狀不會一樣，但每個字該落在哪一格要一樣。
//
// 一行的第一格是編號那個半形格（按鈕左緣 ＋24），最後一格是最後一個
// 全形字；上緣是按鈕上緣 ＋9，格高 16。
var originalMenuInk = []struct {
	name           string
	x0, y0, x1, y1 int
}{
	{"1. 開始新遊戲", 176, 224, 311, 239},
	{"2. 載入舊進度", 176, 276, 311, 291},
	{"3. 使用楷書字", 176, 329, 311, 344},
	{"4. 使用隸書字", 400, 224, 535, 239},
	{"5. 音樂欣賞", 400, 276, 511, 291},
	{"6. 回作業系統", 400, 329, 535, 344},
}

// 直牌上「主選擇單」四個字佔的字格（橫向拉兩倍寬，一格 32×16）。
var originalLabelInk = struct{ x0, y0, x1, y1 int }{80, 242, 111, 337}

// inkBox 回報畫布上某個顏色的墨水外框。
func inkBox(c *Canvas, want color.RGBA) (x0, y0, x1, y1 int, n int) {
	b := c.Img.Bounds()
	x0, y0, x1, y1 = b.Dx(), b.Dy(), -1, -1
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			if c.Img.RGBAAt(x, y) != want {
				continue
			}
			n++
			if x < x0 {
				x0 = x
			}
			if y < y0 {
				y0 = y
			}
			if x > x1 {
				x1 = x
			}
			if y > y1 {
				y1 = y
			}
		}
	}
	return
}

// TestMenuItemLayoutMatchesTheOriginal 釘住六行選單字的落點。
//
// 判準不是「像不像」，是**每個字佔的格與原版相同**：一行字畫完之後
// 墨水不得超出原版那一行的外框。原版每個全形字前面空一個半形格
// （`assets.MenuTextCJKPitch` ＝ 24），照 16 排會擠在按鈕左半邊——
// 那種錯誤畫面上看得出來，但沒有測試會紅。
func TestMenuItemLayoutMatchesTheOriginal(t *testing.T) {
	f := testFace(t)
	items := TitleItems()
	for i, want := range originalMenuInk {
		if items[i] != want.name {
			t.Fatalf("第 %d 項是 %q，基準量的是 %q", i+1, items[i], want.name)
		}
		c := NewCanvasPx(assets.ScreenW, assets.ScreenH, f)
		c.Fill(color.RGBA{0, 0, 0, 255})
		b := assets.MenuButtons()[i]
		DrawMenuItem(c, b[0], b[1], items[i], fg)
		x0, y0, x1, y1, n := inkBox(c, fg)
		if n == 0 {
			t.Fatalf("%s：一個像素都沒畫", want.name)
		}
		if x0 < want.x0 || x1 > want.x1 || y0 < want.y0 || y1 > want.y1 {
			t.Errorf("%s 的墨水在 x %d..%d y %d..%d，原版是 x %d..%d y %d..%d",
				want.name, x0, x1, y0, y1, want.x0, want.x1, want.y0, want.y1)
			continue
		}
		// 右端不得縮太多：字距排錯（16 而不是 24）時整行只有 80 像素寬，
		// 上面那個「不超出」的檢查照樣過。
		if x1 < want.x1-8 {
			t.Errorf("%s 只排到 x=%d，原版排到 %d——字距太小",
				want.name, x1, want.x1)
		}
	}
}

// TestMenuLabelIsDoubleWidth 釘住左側直牌上那四個字。
func TestMenuLabelIsDoubleWidth(t *testing.T) {
	f := testFace(t)
	c := NewCanvasPx(assets.ScreenW, assets.ScreenH, f)
	c.Fill(color.RGBA{0, 0, 0, 255})
	DrawMenuLabel(c, "主選擇單", fg)
	x0, y0, x1, y1, n := inkBox(c, fg)
	if n == 0 {
		t.Fatal("直牌上一個像素都沒畫")
	}
	w := originalLabelInk
	if x0 < w.x0 || x1 > w.x1 || y0 < w.y0 || y1 > w.y1 {
		t.Errorf("直牌的墨水在 x %d..%d y %d..%d，原版是 x %d..%d y %d..%d",
			x0, x1, y0, y1, w.x0, w.x1, w.y0, w.y1)
	}
	// 沒有橫向放大的話寬度只有 16，落在 80..95。
	if x1-x0 < 24 {
		t.Errorf("直牌的字只有 %d 像素寬，原版是拉成兩倍寬的 %d",
			x1-x0+1, w.x1-w.x0+1)
	}
}

// TestTitleScreenMatchesTheOriginal 把整張主選單對回原版。
//
// 對不上的只准是**字的形狀**（remake 自建字庫，`CLAUDE.md` §3.3）；
// 框、牌子、底色與右下角動畫畫格都要逐點相同。
func TestTitleScreenMatchesTheOriginal(t *testing.T) {
	fh, err := os.Open("../../workplace/shots/open/open-06.png")
	if err != nil {
		t.Skipf("沒有主選單的基準畫面：%v", err)
	}
	shot, err := imgpng.Decode(fh)
	fh.Close()
	if err != nil {
		t.Fatal(err)
	}
	c1, c3 := artContainers(t)
	ts, err := NewTitleScreen(c3, c1)
	if err != nil {
		t.Fatal(err)
	}
	c := NewCanvasPx(assets.ScreenW, assets.ScreenH, testFace(t))
	// 這張原版收據停在 CURA5；其他五格由 ornament oracle 逐格驗。
	DrawTitleFrame(c, ts, -1, 5)
	inText := func(x, y int) bool {
		for _, w := range originalMenuInk {
			if x >= w.x0 && x <= w.x1 && y >= w.y0 && y <= w.y1 {
				return true
			}
		}
		w := originalLabelInk
		if x >= w.x0 && x <= w.x1 && y >= w.y0 && y <= w.y1 {
			return true
		}
		// 最上面兩個角落原版是黑的（dosgolem 與 DOSBox-X 一致，
		// `docs/playtest/03`），remake 那兩點畫的是底色。
		return y == 0 && (x == 0 || x == assets.ScreenW-1)
	}
	bad, n := 0, 0
	for y := 0; y < assets.ScreenH; y++ {
		for x := 0; x < assets.ScreenW; x++ {
			if inText(x, y) {
				continue
			}
			n++
			o := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
			if c.Img.RGBAAt(x, y) != o {
				bad++
				if bad <= 8 {
					t.Logf("(%d,%d) remake %v 原版 %v", x, y, c.Img.RGBAAt(x, y), o)
				}
			}
		}
	}
	if bad != 0 {
		t.Errorf("扣掉字之後還有 %d／%d 點對不上", bad, n)
	} else {
		t.Logf("扣掉字之後 %d 點逐點相同（含 CURA5 小飾框）", n)
	}
}
