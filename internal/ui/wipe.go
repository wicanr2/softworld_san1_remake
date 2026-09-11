package ui

import "image"

// 畫面轉場：四種方向的拉幕（`docs/spec/010`）。
//
// 原版在計謀得手之後跑一次（`0x32e40`）：`RND(4)` 四選一，每一步搬一塊、
// 並送一聲 PC 喇叭的音效（`docs/spec/008` §4——**音效是動畫的節拍聲**，
// 不是事件音）。
//
// **不是「逐條露出新畫面」，是「新畫面整塊滑進來」**：第 n 步把新畫面
// 靠**對邊**的那一塊（高 i+1）貼到這一邊，i 每一步變大，最後一步兩邊
// 重合、新畫面歸位。所以中間每一步看到的是新畫面的一部分被移到別的
// 位置——`internal/parity` 的 `TestZZTransitionFrames` 量到的「與前一步
// 差異的範圍」是**整個已蓋區**（第 5 步 y80–99，不是只有新增的 y96–99），
// 逐條露出的話那個範圍只會是新增的那一條。

// WipeRect 是原版拉幕的那一塊：x 432..607、y 80..175（96 高、176 寬）。
//
// 數字是從**唯一的呼叫端**（`0x1bbb9`：`push 0x50; push 0x1b0`）配上
// 迴圈界限（`0x5f` 與 `0xaf`）算出來的，再由對拍逐列確認
//（`TestZZTransitionFrames`）。
var WipeRect = image.Rect(432, 80, 608, 176)

// WipeKind 是四種方向。編號就是原版 `RND(4)` 的結果。
type WipeKind int

const (
	WipeDown  WipeKind = 0 // 由上往下
	WipeUp    WipeKind = 1 // 由下往上
	WipeRight WipeKind = 2 // 由左往右
	WipeLeft  WipeKind = 3 // 由右往左
)

// 步進（`L0`，`0x32e6c` 與 `0x32f14` 的迴圈界限）。
//
// 縱向從 3 起每步 4、走到高度為止；橫向從 0 起每步 8、走到「寬度 −1」
// 為止。**橫向的粒度是 8**：EGA 一個位元組八個像素，搬運以位元組為單位。
const (
	wipeVertStart = 3
	wipeVertStep  = 4
	wipeHorzStart = 0
	wipeHorzStep  = 8
	wipeHorzGrain = 8
)

// Wipe 是一次拉幕。
//
// From 與 To 是同尺寸的兩張圖，Rect 是要換掉的那一塊。走完之後
// Rect 裡就是 To 的內容。
type Wipe struct {
	Kind WipeKind
	Rect image.Rectangle
	From *image.RGBA
	To   *image.RGBA

	n int // 已經走了幾步
}

// vertical 回「這個方向走的是列」。
func (w *Wipe) vertical() bool { return w.Kind == WipeDown || w.Kind == WipeUp }

// Steps 是總步數。原版是 24（縱向）或 22（橫向 96×176 那一塊）。
func (w *Wipe) Steps() int {
	if w.Rect.Empty() {
		return 0
	}
	n := 0
	if w.vertical() {
		for i := wipeVertStart; i < w.Rect.Dy(); i += wipeVertStep {
			n++
		}
		return n
	}
	for i := wipeHorzStart; i < w.Rect.Dx()-1; i += wipeHorzStep {
		n++
	}
	return n
}

// Done 回「走完了沒」。
func (w *Wipe) Done() bool { return w.n >= w.Steps() }

// Reveal 是第 n 步（1 起算）要**寫進去**的那一塊。
//
// 分開成一個函式是為了測得到：**動畫在畫面上對不對，測試看不到**
//（`CLAUDE.md` §7 第 13 條），但「第幾步動到哪一塊」量得出來——
// 那正好是對拍量到的 bounding box。
func (w *Wipe) Reveal(n int) image.Rectangle {
	r := w.Rect
	if r.Empty() || n <= 0 {
		return image.Rectangle{}
	}
	if n >= w.Steps() {
		return r
	}
	if w.vertical() {
		i := wipeVertStart + (n-1)*wipeVertStep
		if w.Kind == WipeDown {
			return image.Rect(r.Min.X, r.Min.Y, r.Max.X, min(r.Min.Y+i+1, r.Max.Y))
		}
		return image.Rect(r.Min.X, max(r.Max.Y-1-i, r.Min.Y), r.Max.X, r.Max.Y)
	}
	i := wipeHorzStart + (n-1)*wipeHorzStep + wipeHorzGrain
	if w.Kind == WipeRight {
		return image.Rect(r.Min.X, r.Min.Y, min(r.Min.X+i, r.Max.X), r.Max.Y)
	}
	return image.Rect(max(r.Max.X-i, r.Min.X), r.Min.Y, r.Max.X, r.Max.Y)
}

// Advance 走一步：把新畫面對邊的那一塊搬到這一邊。
//
// 回傳「這一步有沒有真的走」——走完之後回 false，呼叫端就知道要停。
// **每走一步要送一聲音效**（`docs/spec/008` §4），那是呼叫端的事：
// 這個檔不依賴音訊，也不依賴 Ebiten。
func (w *Wipe) Advance(dst *image.RGBA) bool {
	if w.Done() || dst == nil || w.To == nil {
		return false
	}
	w.n++
	to := w.Reveal(w.n)
	from := w.Source(to)
	dx, dy := to.Min.X-from.Min.X, to.Min.Y-from.Min.Y
	r := to.Intersect(dst.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			sx, sy := x-dx, y-dy
			if !image.Pt(sx, sy).In(w.To.Bounds()) {
				continue
			}
			dst.SetRGBA(x, y, w.To.RGBAAt(sx, sy))
		}
	}
	return true
}

// Source 是「這一塊要從新畫面的哪裡取」：**同樣大小、貼在對邊**。
//
// 分支 0（由上往下）第 n 步把新畫面下緣那 i+1 列貼到上緣，i 每步變大；
// 最後一步 i ＝ 高度 −1，兩邊重合，新畫面就歸位了。其餘三個方向同理。
func (w *Wipe) Source(dst image.Rectangle) image.Rectangle {
	r := w.Rect
	switch w.Kind {
	case WipeDown:
		return image.Rect(r.Min.X, r.Max.Y-dst.Dy(), r.Max.X, r.Max.Y)
	case WipeUp:
		return image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+dst.Dy())
	case WipeRight:
		return image.Rect(r.Max.X-dst.Dx(), r.Min.Y, r.Max.X, r.Max.Y)
	default: // WipeLeft
		return image.Rect(r.Min.X, r.Min.Y, r.Min.X+dst.Dx(), r.Max.Y)
	}
}

// Restart 從頭來過。
func (w *Wipe) Restart() { w.n = 0 }
