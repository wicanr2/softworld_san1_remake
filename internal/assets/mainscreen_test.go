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

	// bad 收前幾個不合的點：**一個百分比看不出「差兩點」與「差兩千點」**，
	// 而 99.99% 印出來就是 100.0%。
	var bad []image.Point
	match := func(r image.Rectangle) float64 {
		same, n := 0, 0
		bad = bad[:0]
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				a := color.RGBAModel.Convert(rgba.At(x, y)).(color.RGBA)
				b := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
				n++
				if a == b {
					same++
				} else if len(bad) < 8 {
					bad = append(bad, image.Pt(x, y))
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
		// 下方花邊（`MAINMAP2`，372–408）與上方花邊一樣沒有東西蓋上去，
		// 所以同樣要 100%。**它是這次把畫面高度改回 408 才進得了畫面的**
		// （`docs/spec/006`）；先前記著「整張落在畫面外」。
		{"下方花邊", image.Rect(0, MapBorderBottomY, 640, ScreenH), 100},
		{"最右直條", image.Rect(632, 36, 640, MapBorderBottomY), 100},
		{"左側直條（年月蓋在上面）", image.Rect(0, 36, 72, MapBorderBottomY), 90},
		{"地圖區（換色與編號蓋在上面）", image.Rect(72, 36, 408, MapBorderBottomY), 60},
	} {
		if got := match(tc.r); got < tc.min {
			t.Errorf("%s 相符 %.4f%%，至少要 %.0f%%（前幾個不合的點 %v）",
				tc.name, got, tc.min, bad)
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

// menuShotPath 是原版主選單的基準圖，由 `internal/parity` 的
// `TestZZOriginalOpeningScreens` 產（第 6 步）。
const menuShotPath = "../../workplace/shots/open/open-06.png"

// TestMenuScreenMatchesTheOriginal 釘住主選單五張圖的位置。
//
// 主選單跑在開機鏈的第二層（`DATA0.GRP`），碼段 dump 涵蓋不到，
// 所以位置**只能拿畫面比對出來**。判準因此要嚴：沒有字蓋在上面的
// 兩塊標題牌要 100%，其餘三張扣掉字與那格動畫之後也要接近滿分。
func TestMenuScreenMatchesTheOriginal(t *testing.T) {
	f, err := os.Open(menuShotPath)
	if err != nil {
		t.Skipf("沒有基準畫面 %s", menuShotPath)
	}
	shot, err := imgpng.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	c := container(t, "DATA3")
	bg, err := MenuScreen(c)
	if err != nil {
		t.Fatal(err)
	}
	rgba := bg.RGBA()
	for _, tc := range []struct {
		name string
		r    image.Rectangle
		min  float64
	}{
		{"標題牌左半 MENU0A", image.Rect(40, 27, 320, 207), 100},
		{"標題牌右半 MENU0B", image.Rect(320, 27, 608, 207), 100},
		{"按鈕列（字寫在上面）", image.Rect(152, 215, 352, 261), 90},
		{"第三列按鈕（y=320 不是 319）", image.Rect(152, 320, 352, 350), 90},
		{"左側直牌 MENU1（字寫在上面）", image.Rect(56, 215, 152, 350), 92},
		{"右下角小飾框 MENU3（裡面會動）", image.Rect(576, 320, 616, 350), 88},
	} {
		same, n := 0, 0
		for y := tc.r.Min.Y; y < tc.r.Max.Y; y++ {
			for x := tc.r.Min.X; x < tc.r.Max.X; x++ {
				a := color.RGBAModel.Convert(rgba.At(x, y)).(color.RGBA)
				b := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
				n++
				if a == b {
					same++
				}
			}
		}
		got := float64(same) * 100 / float64(n)
		if got < tc.min {
			t.Errorf("%s 相符 %.1f%%，至少要 %.0f%%", tc.name, got, tc.min)
		} else {
			t.Logf("%-22s %.1f%%", tc.name, got)
		}
	}
}
