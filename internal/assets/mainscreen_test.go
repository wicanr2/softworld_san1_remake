package assets

import (
	"image"
	"image/color"
	imgpng "image/png"
	"os"
	"testing"
)

// shotPath 是原版主畫面的基準圖，由 `internal/parity` 的
// `TestZZOriginalMainScreen` 產（要設 `SAN1_SHOTS`）。沒有就 skip。
const shotPath = "../../workplace/shots/orig-main.png"

func loadShot(t *testing.T) image.Image {
	t.Helper()
	f, err := os.Open(shotPath)
	if err != nil {
		t.Skipf("沒有基準畫面 %s（先跑 internal/parity 的 TestZZOriginalMainScreen）", shotPath)
	}
	defer f.Close()
	im, err := imgpng.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return im
}

// TestMainScreenMatchesTheOriginal 把拼出來的底圖與原版的畫面逐像素比。
//
// **判準分區看**：底圖之外的東西是後來蓋上去的，所以沒被蓋到的區域
// 要 100% 相同，被蓋到的區域不會。整張比一個數字看不出對錯——
// 54% 這種數字既可能是「位置對、被蓋掉一半」也可能是「位置差幾像素」。
func TestMainScreenMatchesTheOriginal(t *testing.T) {
	c := container(t, "DATA3")
	bg, err := MainScreen(c)
	if err != nil {
		t.Fatal(err)
	}
	if bg.W != ScreenW || bg.H != ScreenH {
		t.Fatalf("底圖是 %d×%d", bg.W, bg.H)
	}
	shot := loadShot(t)
	rgba := bg.RGBA()

	match := func(r image.Rectangle) float64 {
		same, n := 0, 0
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				a := color.RGBAModel.Convert(rgba.At(x, y)).(color.RGBA)
				b := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
				n++
				if a == b {
					same++
				}
			}
		}
		return float64(same) * 100 / float64(n)
	}
	for _, tc := range []struct {
		name string
		r    image.Rectangle
		min  float64
	}{
		{"上方花邊", image.Rect(0, 0, 640, 36), 100},
		{"最右直條", image.Rect(632, 36, 640, 350), 100},
		{"左側直條（年月蓋在上面）", image.Rect(0, 36, 72, 350), 90},
		{"地圖區（換色與編號蓋在上面）", image.Rect(72, 36, 408, 350), 60},
	} {
		if got := match(tc.r); got < tc.min {
			t.Errorf("%s 相符 %.1f%%，至少要 %.0f%%", tc.name, got, tc.min)
		} else {
			t.Logf("%-28s %.1f%%", tc.name, got)
		}
	}
}

// TestMainScreenPiecesCoverTheWidth 釘住七張底圖橫向剛好蓋滿 640。
//
// 這一條擋的是「少拼一張也看不出來」：右邊那 8 像素的直條漏掉時，
// 畫面看起來完全正常，只是最右邊少一條花邊。
func TestMainScreenPiecesCoverTheWidth(t *testing.T) {
	c := container(t, "DATA3")
	covered := make([]bool, ScreenW)
	for _, p := range mainScreenPieces {
		i, ok := c.ByName(p.Name)
		if !ok {
			t.Fatalf("DATA3 裡沒有 %s", p.Name)
		}
		im, err := DecodeImage(c.Data(i))
		if err != nil {
			t.Fatal(err)
		}
		if p.Y != 0 { // 只數畫在 y=36 以下那幾張
			for x := p.X; x < p.X+im.W && x < ScreenW; x++ {
				covered[x] = true
			}
		}
	}
	for x, ok := range covered {
		if !ok {
			t.Fatalf("第 %d 欄沒有任何底圖蓋到", x)
		}
	}
}
