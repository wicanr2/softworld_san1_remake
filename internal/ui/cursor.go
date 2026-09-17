package ui

import (
	"image/color"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
)

// InputCursor 是提示後面那個等輸入的游標（`docs/spec/014` §4.1）。
//
// 原版讀鍵的 `0x1058:0xe24` 等鍵時每一輪叫回呼槽 0（`0x10d7a`）：在文字游標
// 那 8×16 格輪流合成 `CUR` 那一組的六格，拿到鍵就貼回底色收掉。文字游標
// 停在提示最後一個字之後，所以游標接在最後一行字的尾端。
type InputCursor struct {
	// On 為真表示正在等輸入；Frame 是六格裡的第幾格。
	On    bool
	Frame int
}

// CursorTicksPerFrame 是 remake 換一格游標的節拍（與主選單小飾框相同）。
// 原版是讀鍵輪詢每 512 次換一格，快慢跟著機器走；remake 固定節拍（remake 差異）。
const CursorTicksPerFrame = TitleOrnamentTicksPerFrame

// CursorFrameAt 把節拍換成六格裡的第幾格。
func CursorFrameAt(tick int) int {
	if tick < 0 {
		tick = 0
	}
	return tick / CursorTicksPerFrame % assets.MenuOrnamentFrameCount
}

// drawInputCursor 在 (x, y) 合成一格游標：`(底 AND 遮罩) OR 圖`。
func drawInputCursor(c *Canvas, frames *[assets.MenuOrnamentFrameCount]assets.CursorFrame, in InputCursor, x, y int) {
	if frames == nil || !in.On {
		return
	}
	f := frames[((in.Frame%len(frames))+len(frames))%len(frames)]
	if f.Sprite == nil || f.Mask == nil {
		return
	}
	for dy := 0; dy < f.Sprite.H; dy++ {
		for dx := 0; dx < f.Sprite.W; dx++ {
			px, py := x+dx, y+dy
			if px < 0 || py < 0 || px >= c.Img.Rect.Dx() || py >= c.Img.Rect.Dy() {
				continue
			}
			old := egaIndexOf(c.Img.RGBAAt(px, py))
			c.Img.SetRGBA(px, py, assets.EGAPalette[f.Over(dx, dy, old)&15])
		}
	}
}

// cursorAfterLines 是一疊逐列排好的字最後一行尾端那一格。
func cursorAfterLines(x0, y0, dy int, lines []string) (x, y int, ok bool) {
	if len(lines) == 0 {
		return 0, 0, false
	}
	n := len(lines) - 1
	return x0 + cells.Width(lines[n])*CellW, y0 + n*dy, true
}

// egaIndexOf 把畫布上的顏色換回 16 色的色號；不是 EGA 那 16 色的當 0。
func egaIndexOf(q color.RGBA) byte {
	for i, e := range assets.EGAPalette {
		if e == q {
			return byte(i)
		}
	}
	return 0
}
