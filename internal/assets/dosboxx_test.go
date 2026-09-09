package assets

import (
	"image"
	"image/color"
	imgpng "image/png"
	"os"
	"testing"
)

// dosboxxMenuPath 是 DOSBox-X 跑出來的主選單，由 `tools/dosboxx.sh` 產。
const dosboxxMenuPath = "../../workplace/shots/dosboxx/menu.png"

// menuAnim 是右下角小飾框裡那一格：原版在那裡放了一段動畫，
// 同一台原版連拍兩張就會不同，所以它不參加逐點比對。
var menuAnim = image.Rect(590, 328, 602, 345)

// TestDosgolemMenuMatchesDosboxX 拿 DOSBox-X 驗 dosgolem 的主選單。
//
// `workplace/shots/open/` 那幾張是 dosgolem 畫的，而主選單的圖塊位置
// 是從那幾張比對出來的。**拿 dosgolem 自己畫的圖去驗從它比出來的版面
// 等於自己驗自己**（`CLAUDE.md` §4）——所以要有第二個獨立實作。
//
// 兩邊唯一該不同的是那格動畫；其餘 640×350 逐點相同。
func TestDosgolemMenuMatchesDosboxX(t *testing.T) {
	dbx := openShot(t, dosboxxMenuPath, "跑 tools/dosboxx.sh 產")
	dg := openShot(t, menuShotPath, "跑 internal/parity 的 TestZZOriginalOpeningScreens 產")
	bad, n, outside := 0, 0, 0
	for y := 0; y < ScreenH; y++ {
		for x := 0; x < ScreenW; x++ {
			a := color.RGBAModel.Convert(dbx.At(x, y)).(color.RGBA)
			b := color.RGBAModel.Convert(dg.At(x, y)).(color.RGBA)
			if a == b {
				n++
				continue
			}
			bad++
			if !image.Pt(x, y).In(menuAnim) {
				outside++
				if outside <= 5 {
					t.Errorf("(%d,%d) DOSBox-X %v dosgolem %v", x, y, a, b)
				}
			}
		}
	}
	if outside > 0 {
		t.Errorf("那格動畫以外還有 %d 點兩個實作對不上", outside)
		return
	}
	t.Logf("兩個實作 %d 點逐點相同，只有動畫那一格的 %d 點不同", n, bad)
}

func openShot(t *testing.T, path, how string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("沒有 %s（%s）", path, how)
	}
	defer f.Close()
	im, err := imgpng.Decode(f)
	if err != nil {
		t.Fatalf("解 %s：%v", path, err)
	}
	if b := im.Bounds(); b.Dx() != ScreenW || b.Dy() != ScreenH {
		t.Fatalf("%s 是 %d×%d，應該是 %d×%d", path, b.Dx(), b.Dy(), ScreenW, ScreenH)
	}
	return im
}
