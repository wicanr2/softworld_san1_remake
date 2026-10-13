package ui

import (
	"image"
	"image/color"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

const WindowBarHeight = 32
const WindowBarRow = 24
const WindowBarOptionCount = 4
const WindowBarAIHelpLines = 5
const windowBarAIHelpHeight = 16 + WindowBarAIHelpLines*16
const WindowBarCanvasHeight = WindowBarHeight + 6*WindowBarRow + windowBarAIHelpHeight

// WindowBarHeightDelta 按目前遊戲的顯示倍率計算，寬視窗以高度為準。
func WindowBarHeightDelta(width, height, gameHeight int, wasVisible bool) int {
	if width <= 0 || height <= 0 || gameHeight <= 0 {
		return 0
	}
	logicalHeight := gameHeight
	if wasVisible {
		logicalHeight += WindowBarHeight
	}
	scale := min(float64(width)/640, float64(height)/float64(logicalHeight))
	return int(WindowBarHeight * scale)
}

var WindowBarFields = [WindowBarOptionCount + 1]image.Rectangle{
	image.Rect(8, 4, 152, 28), image.Rect(160, 4, 272, 28), image.Rect(280, 4, 416, 28),
	image.Rect(424, 4, 592, 28), image.Rect(600, 4, 632, 28),
}

func windowBarListRect(field, rows int) image.Rectangle {
	r := WindowBarFields[field]
	r.Min.Y, r.Max.Y = WindowBarHeight, WindowBarHeight+rows*WindowBarRow
	if field == 2 {
		r.Min.X, r.Max.X = 8, 632
	} else if field == 3 {
		r.Min.X, r.Max.X = 376, 632
	}
	return r
}

func windowBarMenuRect(field, rows int) image.Rectangle {
	r := windowBarListRect(field, rows)
	if field == 2 {
		r.Max.Y += windowBarAIHelpHeight
	}
	return r
}

type WindowBarState struct {
	Visible, Pinned bool
	Menu            int
	Hover           int
	blockHover      bool
}

func NewWindowBarState() WindowBarState { return WindowBarState{Menu: -1, Hover: -1} }

// Reveal 只控制外部選項列，不修改遊戲輸入或畫布。
func (b *WindowBarState) Reveal(escape bool, cursor image.Point, rows int) bool {
	old := b.Visible
	if escape {
		b.Visible = !b.Visible
		b.Pinned = b.Visible
		b.blockHover = !b.Visible
		b.Menu = -1
		return old != b.Visible
	}
	insideTop := cursor.X >= 0 && cursor.X < 640 && cursor.Y >= 0 && cursor.Y < 6
	if !insideTop {
		b.blockHover = false
	}
	if b.Pinned {
		return false
	}
	inside := cursor.X >= 0 && cursor.X < 640 && cursor.Y >= 0 && cursor.Y < WindowBarHeight
	if b.Menu >= 0 && b.Menu < WindowBarOptionCount {
		inside = inside || cursor.In(windowBarMenuRect(b.Menu, rows))
	}
	if b.Visible {
		b.Visible = inside
	} else {
		b.Visible = insideTop && !b.blockHover
	}
	if !b.Visible {
		b.Menu = -1
	}
	return old != b.Visible
}

type WindowBarView struct {
	Values             [WindowBarOptionCount]string
	Lists              [WindowBarOptionCount][]string
	Selected           [WindowBarOptionCount]int
	Disabled           [WindowBarOptionCount][]bool
	AIHelp             [WindowBarAIHelpLines]string
	Menu, Hover, Focus int
}

// WindowBarHit 的 row=-1 是欄位按鈕，row=-2 是說明區，field=4 是關閉。
func WindowBarHit(p image.Point, v WindowBarView) (field, row int) {
	if v.Menu >= 0 && v.Menu < WindowBarOptionCount {
		r := windowBarListRect(v.Menu, len(v.Lists[v.Menu]))
		if p.In(r) {
			return v.Menu, (p.Y - WindowBarHeight) / WindowBarRow
		}
		if p.In(windowBarMenuRect(v.Menu, len(v.Lists[v.Menu]))) {
			return v.Menu, -2
		}
	}
	for k, r := range WindowBarFields {
		if p.In(r) {
			return k, -1
		}
	}
	return -1, -1
}

func DrawWindowBar(c *Canvas, v WindowBarView) {
	c.Fill(color.RGBA{})
	bg := color.RGBA{24, 29, 38, 255}
	fg := color.RGBA{230, 234, 241, 255}
	button := color.RGBA{44, 52, 65, 255}
	active := color.RGBA{44, 92, 112, 255}
	c.FillRect(0, 0, 640, WindowBarHeight, bg)
	labels := [WindowBarOptionCount]string{i18n.S("window.language"), i18n.S("window.theme"), i18n.S("window.ai"), i18n.S("window.input")}
	for k, r := range WindowBarFields {
		ink := button
		if k == v.Focus || k == v.Menu {
			ink = active
		}
		c.FillRect(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, ink)
		if k == WindowBarOptionCount {
			c.DrawTextPx(r.Min.X+8, 8, "×", fg)
			continue
		}
		text := labels[k] + " " + v.Values[k]
		if c.FitsSmall(text) && cells.Width(text)*CellW > r.Dx()-32 {
			c.DrawSmallTextPx(r.Min.X+8, 11, text, fg)
		} else {
			c.DrawTextPx(r.Min.X+8, 8, text, fg)
		}
		c.DrawTextPx(r.Max.X-16, 8, "▾", fg)
	}
	if v.Menu < 0 || v.Menu >= WindowBarOptionCount {
		return
	}
	r := windowBarListRect(v.Menu, len(v.Lists[v.Menu]))
	for row, label := range v.Lists[v.Menu] {
		y := WindowBarHeight + row*WindowBarRow
		ink := button
		if row == v.Hover || row == v.Selected[v.Menu] {
			ink = active
		}
		c.FillRect(r.Min.X, y, r.Max.X, y+WindowBarRow, ink)
		textInk := fg
		if row < len(v.Disabled[v.Menu]) && v.Disabled[v.Menu][row] {
			textInk = color.RGBA{133, 143, 158, 255}
		}
		if c.FitsSmall(label) && cells.Width(label)*CellW > r.Dx()-16 {
			c.DrawSmallTextPx(r.Min.X+8, y+7, label, textInk)
		} else {
			c.DrawTextPx(r.Min.X+8, y+4, label, textInk)
		}
	}
	if v.Menu == 2 {
		y := r.Max.Y
		c.FillRect(r.Min.X, y, r.Max.X, y+windowBarAIHelpHeight, bg)
		for i, line := range v.AIHelp {
			c.DrawTextPx(r.Min.X+8, y+8+i*16, line, fg)
		}
	}
}
