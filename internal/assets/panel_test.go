package assets

import (
	"image"
	"image/color"
	imgpng "image/png"
	"os"
	"testing"
)

// mainShotPath 是原版的遊戲主畫面，由 `internal/parity` 的
// `TestZZOriginalMainScreen` 產。
const mainShotPath = "../../workplace/shots/orig-main.png"

// TestMainPanelsMatchTheOriginal 釘住右側兩塊面板的外框。
//
// 外框是**拼件**不是畫出來的：四個角用 `SIDE?16`、四條邊用 `SIDE?8`
// 平鋪（`docs/spec/005` §6.2）。這一支只比外框那一圈與底色，
// 不比內部——內部有文字、數字與肖像，隨局面變。
//
// **只比得動的地方就要 100%。** 拼件差一格、底色差一號，
// 畫面上看起來都「差不多」，只有逐點比才分得出來。
func TestMainPanelsMatchTheOriginal(t *testing.T) {
	f, err := os.Open(mainShotPath)
	if err != nil {
		t.Skipf("沒有原版主畫面 %s", mainShotPath)
	}
	shot, err := imgpng.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	c1 := container(t, "DATA1")
	im := &Image{W: ScreenW, H: ScreenH, Pix: make([]byte, ScreenW*ScreenH)}
	for i, p := range MainPanels() {
		fr, err := LoadSideFrame(c1, p.Letter)
		if err != nil {
			t.Fatalf("第 %d 塊面板的拼件：%v", i+1, err)
		}
		im.DrawPanel(p, fr)
	}
	rgba := im.RGBA()

	// 只比外框那一圈（外緣往內 16 像素）與內部靠邊的一條底色。
	inFrame := func(p MainPanel, x, y int) bool {
		if x < p.X || x >= p.X+p.W || y < p.Y || y >= p.Y+p.H {
			return false
		}
		// 邊是 8 像素厚，角是 16×16。**內部不算**——那裡有文字與肖像，
		// 隨局面變；把「角的高度」整條當成外框會把訊息列的字也比進來。
		if x < p.X+8 || x >= p.X+p.W-8 || y < p.Y+8 || y >= p.Y+p.H-8 {
			return true
		}
		cx := x < p.X+16 || x >= p.X+p.W-16
		cy := y < p.Y+16 || y >= p.Y+p.H-16
		return cx && cy
	}
	var same, n int
	var firstBad image.Point
	for _, p := range MainPanels() {
		for y := p.Y; y < p.Y+p.H && y < ScreenH; y++ {
			for x := p.X; x < p.X+p.W && x < ScreenW; x++ {
				if !inFrame(p, x, y) {
					continue
				}
				n++
				a := color.RGBAModel.Convert(rgba.At(x, y)).(color.RGBA)
				b := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
				if a == b {
					same++
				} else if firstBad.X == 0 {
					firstBad = image.Pt(x, y)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("一個像素都沒比到")
	}
	if same != n {
		t.Errorf("外框比了 %d 點，對不上 %d 點（第一個在 %v）",
			n, n-same, firstBad)
	} else {
		t.Logf("兩塊面板的外框 %d 點逐點相同", n)
	}
}
