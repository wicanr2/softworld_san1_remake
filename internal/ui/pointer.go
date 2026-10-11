package ui

import (
	"image"
	"unicode"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"
)

// PointerTarget 的座標採原始畫布像素，與原貌／高清共用同一份。
type PointerTarget struct {
	Rect  image.Rectangle
	Kind  PointerKind
	Value int
}

type PointerKind int

const (
	PointerKey PointerKind = iota
	PointerTitle
	PointerRoster
	PointerPref
	PointerMap
	PointerCommand
	PointerNumber // 畫面上的編號選項，可直接提交；數字鍵盤仍逐位輸入。
)

const (
	PointerPrevious = -1
	PointerNext     = -2
	PointerLess     = -3
	PointerMore     = -4
	PointerCommands = -5
)

func (c *Canvas) AddPointer(r image.Rectangle, kind PointerKind, value int) {
	if !r.Empty() {
		c.PointerTargets = append(c.PointerTargets, PointerTarget{r, kind, value})
	}
}

func (c *Canvas) PointerAt(p image.Point) (PointerTarget, bool) {
	for i := len(c.PointerTargets) - 1; i >= 0; i-- {
		if h := c.PointerTargets[i]; p.In(h.Rect) {
			return h, true
		}
	}
	return PointerTarget{}, false
}

// pointerLine 沿實際排出的 Unicode 文字尋找「鍵.名稱」，不重建第二份排版。
func (c *Canvas) pointerLine(x, y int, line string, cw, ch int) {
	r := []rune(line)
	type option struct {
		start, value int
		kind         PointerKind
	}
	var options []option
	for i := 0; i+1 < len(r); i++ {
		if i > 0 && !unicode.IsSpace(r[i-1]) {
			continue
		}
		j, value, kind := i, 0, PointerNumber
		for j < len(r) && r[j] >= '0' && r[j] <= '9' {
			value = value*10 + int(r[j]-'0')
			j++
		}
		if j == i && r[i] >= 'A' && r[i] <= 'Z' {
			j, value, kind = i+1, int(r[i]), PointerKey
		}
		if j > i && j < len(r) && r[j] == '.' {
			options = append(options, option{i, value, kind})
			i = j
		}
	}
	for i, o := range options {
		end := len(r)
		if i+1 < len(options) {
			end = options[i+1].start
		}
		px := x + cells.Width(string(r[:o.start]))*cw
		width := cells.Width(string(r[o.start:end])) * cw
		c.AddPointer(image.Rect(px, y, min(px+width, c.Img.Bounds().Dx()), y+ch), o.kind, o.value)
	}
}

// DrawPointerButton 只用於新增的操作外殼，與原版畫布內容分開。
func DrawPointerButton(c *Canvas, r image.Rectangle, label string, key int) {
	c.FillRect(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, ColBG)
	c.StrokeRect(r.Min.X, r.Min.Y, r.Max.X-1, r.Max.Y-1, ColFrame)
	x := r.Min.X + (r.Dx()-cells.Width(label)*CellW)/2
	c.DrawTextPx(x, r.Min.Y+(r.Dy()-CellH)/2, label, ColFG)
	c.AddPointer(r, PointerKey, key)
}
