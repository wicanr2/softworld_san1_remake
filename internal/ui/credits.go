package ui

import (
	"image"
	"image/draw"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// 製作群畫面（`docs/spec/012`）。
//
// 字幕從**山後面**升起來：背景是一張山城風景（`REC10`），而
// `ENDO4.MSK` 是它的天空遮罩——字只在天空那一側畫得出來，
// 進到山與水的範圍就被擋住。遮罩與那張圖逐點 99.39% 吻合
//（位移 `assets.CreditMaskY`），所以「它是天空與山的分界」是量出來的。
//
// ⚠ **合成方式是推測**（`L3`）：原版那一段跑在 `DATA0.GRP`／`DATA4.GRP`
// 兩支 overlay，還沒反組譯。量到的是素材與遮罩的對應，不是它怎麼動。

// 版面。
const (
	// CreditsTop 是背景圖的上緣。畫面 408 高、圖 336 高，**上下各留 36**
	// ——那與主畫面上下花邊的高度相同（`assets.MapBorderBottomY` ＝ 372，
	// 408 − 372 ＝ 36），所以這個擺法不是隨便取的中間值（`L2`）。
	CreditsTop = (assets.ScreenH - assets.CreditBackdropH) / 2

	// CreditLineStep 是兩條字幕的間距。**緊貼，沒有行距**：
	// 那 22 條不是 22 行字，是一張長圖被切成 24 列一條——`UPR00`–`UPR03`
	// 四條同樣 272 寬，接起來才是「三國演義」那個大標題；
	// `UPR06`／`UPR07` 兩條 584 寬接成一行名字加頭像。
	// 加行距會把一個圖案拆成好幾塊。
	CreditLineStep = assets.CreditLineH
)

// CreditsLength 是字幕從畫面底部捲到全部離開畫面頂端要走幾個像素。
func CreditsLength(cr *assets.Credits) int {
	if cr == nil || len(cr.Lines) == 0 {
		return 0
	}
	return assets.ScreenH + len(cr.Lines)*CreditLineStep
}

// DrawCredits 畫製作群的一格。scroll 是字幕已經往上捲了幾個像素。
func DrawCredits(c *Canvas, cr *assets.Credits, scroll int) {
	im := &assets.Image{W: assets.ScreenW, H: assets.ScreenH,
		Pix: make([]byte, assets.ScreenW*assets.ScreenH)}
	if cr != nil {
		if cr.Backdrop[0] != nil {
			im.Blit(cr.Backdrop[0], 0, CreditsTop)
		}
		for i, ln := range cr.Lines {
			y := assets.ScreenH - scroll + i*CreditLineStep
			blitCreditLine(im, cr, ln, (assets.ScreenW-ln.W)/2, y)
		}
	}
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		im.RGBA(), image.Point{}, draw.Src)
}

// blitCreditLine 畫一條字幕：**色號 0 當透明**（字條是黑底彩字），
// 而且只有落在天空那一側的像素畫得出來。
func blitCreditLine(dst *assets.Image, cr *assets.Credits, src *assets.Image, x, y int) {
	for sy := 0; sy < src.H; sy++ {
		dy := y + sy
		if dy < 0 || dy >= dst.H {
			continue
		}
		for sx := 0; sx < src.W; sx++ {
			v := src.Pix[sy*src.W+sx]
			if v == 0 {
				continue
			}
			dx := x + sx
			if dx < 0 || dx >= dst.W {
				continue
			}
			if !cr.SkyAt(dx, dy-CreditsTop) {
				continue
			}
			dst.Pix[dy*dst.W+dx] = v
		}
	}
}

// DrawCreditHall 畫朝堂圖那一張（字幕之前的定格）。
func DrawCreditHall(c *Canvas, cr *assets.Credits) {
	im := &assets.Image{W: assets.ScreenW, H: assets.ScreenH,
		Pix: make([]byte, assets.ScreenW*assets.ScreenH)}
	if cr != nil && cr.Hall != nil {
		im.Blit(cr.Hall, 0, CreditsTop)
	}
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		im.RGBA(), image.Point{}, draw.Src)
}
