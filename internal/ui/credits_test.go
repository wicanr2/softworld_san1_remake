package ui

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// fakeCredits 造一組假素材：天空在上半、山在下半，字幕是一條全填的橫槓。
func fakeCredits(lines int) *assets.Credits {
	cr := &assets.Credits{}
	bd := &assets.Image{W: assets.CreditBackdropW, H: assets.CreditBackdropH,
		Pix: make([]byte, assets.CreditBackdropW*assets.CreditBackdropH)}
	cr.Backdrop[0] = bd
	// 遮罩：上半是天空（1）、下半是山（0）。
	m := &assets.Image{W: assets.CreditBackdropW, H: 151,
		Pix: make([]byte, assets.CreditBackdropW*151)}
	for y := 0; y < 75; y++ {
		for x := 0; x < m.W; x++ {
			m.Pix[y*m.W+x] = 1
		}
	}
	cr.Mask = m
	for i := 0; i < lines; i++ {
		ln := &assets.Image{W: 64, H: assets.CreditLineH,
			Pix: make([]byte, 64*assets.CreditLineH)}
		for j := range ln.Pix {
			ln.Pix[j] = 9
		}
		cr.Lines = append(cr.Lines, ln)
	}
	return cr
}

func TestCreditsLength(t *testing.T) {
	if got := CreditsLength(nil); got != 0 {
		t.Errorf("沒有素材時長度 ＝ %d", got)
	}
	cr := fakeCredits(22)
	want := assets.ScreenH + 22*CreditLineStep
	if got := CreditsLength(cr); got != want {
		t.Errorf("長度 ＝ %d，想要 %d", got, want)
	}
	// 字幕是一張長圖被切成 24 列一條，**條與條之間不能有行距**
	// ——加了行距就會把「三國演義」那個四條組成的標題拆開。
	if CreditLineStep != assets.CreditLineH {
		t.Errorf("行距 ＝ %d，想要 %d（緊貼）", CreditLineStep, assets.CreditLineH)
	}
}

// 字幕只在天空那一側畫得出來。
func TestDrawCreditsRespectsTheMask(t *testing.T) {
	face := testFace(t)
	c := NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
	cr := fakeCredits(22)
	ink := assets.EGAPalette[9]

	seenSky, seenHidden := 0, 0
	for _, scroll := range []int{100, 300, 500, 700} {
		DrawCredits(c, cr, scroll)
		for i := range cr.Lines {
			y := assets.ScreenH - scroll + i*CreditLineStep
			for dy := 0; dy < assets.CreditLineH; dy++ {
				yy := y + dy
				if yy < 0 || yy >= assets.ScreenH {
					continue
				}
				x := (assets.ScreenW - cr.Lines[i].W) / 2
				got := c.Img.RGBAAt(x, yy)
				if cr.SkyAt(x, yy-CreditsTop) {
					if got != ink {
						t.Fatalf("scroll %d：(%d,%d) 在天空裡卻沒畫字",
							scroll, x, yy)
					}
					seenSky++
				} else {
					if got == ink {
						t.Fatalf("scroll %d：(%d,%d) 在山後卻畫了字",
							scroll, x, yy)
					}
					seenHidden++
				}
			}
		}
	}
	// **兩種情形都要真的發生過**，否則這支測試等於沒驗到遮罩。
	if seenSky == 0 || seenHidden == 0 {
		t.Errorf("只驗到一種情形（露出 %d、擋住 %d）", seenSky, seenHidden)
	}
}

// 沒有素材時不該 panic，畫出一張空的就好。
func TestDrawCreditsWithoutAssets(t *testing.T) {
	face := testFace(t)
	c := NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
	DrawCredits(c, nil, 0)
	DrawCreditHall(c, nil)
	DrawCreditHall(c, &assets.Credits{})
}
