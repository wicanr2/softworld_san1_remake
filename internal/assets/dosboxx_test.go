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

// menuOrnament 是右下角完整小飾框；兩個獨立執行器可能截到不同 CURA
// 相位，所以先各自逐點驗成六格之一，再比較其餘畫面。
var menuOrnament = image.Rect(576, 320, 616, 361)

// TestDosgolemMenuMatchesDosboxX 拿 DOSBox-X 驗 dosgolem 的主選單。
//
// `workplace/shots/open/` 那幾張是 dosgolem 畫的，而主選單的圖塊位置
// 是從那幾張比對出來的。**拿 dosgolem 自己畫的圖去驗從它比出來的版面
// 等於自己驗自己**（`CLAUDE.md` §4）——所以要有第二個獨立實作。
//
// 動畫區也參加驗證：兩張各自必須逐點等於 `MENU3 + CURAnM + CURAn` 的
// 某一合法畫格；其餘 640×408 再跨執行器逐點相同。
func TestDosgolemMenuMatchesDosboxX(t *testing.T) {
	dbx := openShot(t, dosboxxMenuPath, "跑 tools/dosboxx.sh 產")
	dg := openShot(t, menuShotPath, "跑 internal/parity 的 TestZZOriginalOpeningScreens 產")
	frames, err := MenuScreenFrames(container(t, "DATA1"), container(t, "DATA3"))
	if err != nil {
		t.Fatal(err)
	}
	phase := func(im image.Image) int {
		for i, frame := range frames {
			want := frame.RGBA()
			ok := true
			for y := menuOrnament.Min.Y; y < menuOrnament.Max.Y && ok; y++ {
				for x := menuOrnament.Min.X; x < menuOrnament.Max.X; x++ {
					if color.RGBAModel.Convert(im.At(x, y)) != color.RGBAModel.Convert(want.At(x, y)) {
						ok = false
						break
					}
				}
			}
			if ok {
				return i
			}
		}
		return -1
	}
	dbxPhase, dgPhase := phase(dbx), phase(dg)
	if dgPhase < 0 {
		t.Fatalf("dosgolem 小飾框不是合法 CURA 畫格：%d", dgPhase)
	}
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
			if !image.Pt(x, y).In(menuOrnament) {
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
	if dbxPhase < 0 {
		t.Logf("兩個實作 %d 點逐點相同；dosgolem=CURA%d；DOSBox-X 截在搬運中間態，小飾框差 %d 點",
			n, dgPhase, bad)
	} else {
		t.Logf("兩個實作 %d 點逐點相同；DOSBox-X=CURA%d、dosgolem=CURA%d，相位差 %d 點",
			n, dbxPhase, dgPhase, bad)
	}
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

// dosboxxTrademarkPath 是 DOSBox-X 開機第一幕，由
// `SAN1_DOSBOX_MODE=trademark SAN1_DOSBOX_OUT=workplace/shots/dosboxx-trademark tools/dosboxx.sh` 產。
const dosboxxTrademarkPath = "../../workplace/shots/dosboxx-trademark/trademark-00.png"

// TestTrademarkMatchesDosboxX 拿 DOSBox-X 驗商標畫面（Issue #34）：
// dosgolem 那一邊是 `TestZZTrademarkMatchesTheOriginal`，這一支是獨立的
// 第二個實作，整張 640×408 與 `TrademarkScreen` 逐點相同才算數。
func TestTrademarkMatchesDosboxX(t *testing.T) {
	dbx := openShot(t, dosboxxTrademarkPath, "跑 SAN1_DOSBOX_MODE=trademark tools/dosboxx.sh 產")
	want, err := TrademarkScreen(container(t, "DATA1"))
	if err != nil {
		t.Fatal(err)
	}
	rgba := want.RGBA()
	bad := 0
	for y := 0; y < ScreenH; y++ {
		for x := 0; x < ScreenW; x++ {
			a := color.RGBAModel.Convert(dbx.At(x, y)).(color.RGBA)
			b := color.RGBAModel.Convert(rgba.At(x, y)).(color.RGBA)
			if a != b {
				if bad < 5 {
					t.Errorf("(%d,%d) DOSBox-X %v remake %v", x, y, a, b)
				}
				bad++
			}
		}
	}
	if bad != 0 {
		t.Fatalf("商標畫面與 DOSBox-X 差 %d 點", bad)
	}
	t.Logf("商標畫面與 DOSBox-X 整張 %d×%d 逐點相同", ScreenW, ScreenH)
}
