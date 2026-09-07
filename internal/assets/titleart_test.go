package assets

import (
	imgpng "image/png"
	"os"
	"testing"
)

// titleArtShotPath 是原版開場的三英圖，由 `internal/parity` 的
// `TestTitleArtLayoutMatchesTheOriginal` 產（`SAN1_SHOTS` 指到輸出目錄）。
const titleArtShotPath = "../../workplace/shots/orig-title-art.png"

// TestTitleArtMatchesTheOriginal 把 `TITL0`–`3` 拼出來的三英圖與原版
// 的畫面逐格比。
//
// 這一張**整張都要相同**——它是一張純圖，沒有任何東西疊在上面。
func TestTitleArtMatchesTheOriginal(t *testing.T) {
	c := container(t, "DATA1")
	im, err := TitleArt(c)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(titleArtShotPath)
	if err != nil {
		t.Skipf("沒有基準畫面 %s（先跑 internal/parity 的 "+
			"TestTitleArtLayoutMatchesTheOriginal）", titleArtShotPath)
	}
	defer f.Close()
	shot, err := imgpng.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	bad := 0
	for y := 0; y < ScreenH; y++ {
		for x := 0; x < ScreenW; x++ {
			r, g, b, _ := shot.At(x, y).RGBA()
			w := EGAPalette[im.Pix[y*ScreenW+x]&15]
			if uint8(r>>8) != w.R || uint8(g>>8) != w.G || uint8(b>>8) != w.B {
				bad++
			}
		}
	}
	if bad != 0 {
		t.Errorf("%d 格與原版不同（共 %d）", bad, ScreenW*ScreenH)
	}
	t.Logf("三英圖 %d 格逐格相同", ScreenW*ScreenH)
}
