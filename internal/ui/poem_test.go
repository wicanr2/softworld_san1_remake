package ui

import (
	"image/color"
	imgpng "image/png"
	"os"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// TestPoemScreenMatchesTheOriginal 把開場詩的底圖對回原版。
//
// 基準是開場的第一格——**詩還沒寫上去那一張**，所以整張都比得動。
func TestPoemScreenMatchesTheOriginal(t *testing.T) {
	f, err := os.Open("../../workplace/shots/open/open-00.png")
	if err != nil {
		t.Skipf("沒有開場的基準畫面：%v", err)
	}
	shot, err := imgpng.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	c1, _ := artContainers(t)
	im, err := assets.PoemScreen(c1)
	if err != nil {
		t.Fatal(err)
	}
	bad, n := 0, 0
	for y := assets.PoemY; y < assets.PoemY+assets.PoemPieceH; y++ {
		for x := 0; x < assets.ScreenW; x++ {
			n++
			a := assets.EGAPalette[im.At(x, y)&15]
			o := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
			if a != o {
				bad++
			}
		}
	}
	if bad != 0 {
		t.Errorf("底圖有 %d／%d 格對不上", bad, n)
	}
}

// TestPoemColumns 釘住詞的欄數與字數。
func TestPoemColumns(t *testing.T) {
	cols := PoemColumns()
	if len(cols) != 11 {
		t.Fatalf("有 %d 欄", len(cols))
	}
	if cols[0] != "詞曰" {
		t.Errorf("最右邊那一欄是 %q", cols[0])
	}
	for i, col := range cols[1:] {
		if n := len([]rune(col)); n < 5 || n > 7 {
			t.Errorf("第 %d 欄有 %d 個字", i+1, n)
		}
	}
}
