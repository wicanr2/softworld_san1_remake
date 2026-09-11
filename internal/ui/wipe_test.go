package ui

import (
	"image"
	"image/color"
	"testing"
)

// 原版那一塊是 x 432..607、y 80..175（`docs/spec/010`）。
var wipeRect = image.Rect(432, 80, 608, 176)

func TestWipeStepCountsMatchTheOriginal(t *testing.T) {
	for _, tc := range []struct {
		kind WipeKind
		want int
	}{
		{WipeDown, 24}, {WipeUp, 24}, {WipeRight, 22}, {WipeLeft, 22},
	} {
		w := &Wipe{Kind: tc.kind, Rect: wipeRect}
		if got := w.Steps(); got != tc.want {
			t.Errorf("方向 %d 走 %d 步，原版是 %d", tc.kind, got, tc.want)
		}
	}
}

// 每一步露出來的那一塊，對照對拍量到的邊界。
func TestWipeRevealMatchesTheOriginal(t *testing.T) {
	type want struct {
		step           int
		lo, hi         int // 露出來的那一段（含兩端）
	}
	for _, tc := range []struct {
		kind  WipeKind
		axisY bool
		want  []want
	}{
		// 由上往下：第 1 步 80–83、第 5 步 80–99、第 9 步 80–115…
		{WipeDown, true, []want{{1, 80, 83}, {5, 80, 99}, {9, 80, 115},
			{13, 80, 131}, {17, 80, 147}, {21, 80, 163}, {24, 80, 175}}},
		// 由下往上：第 1 步 172–175、第 5 步 156–175…
		{WipeUp, true, []want{{1, 172, 175}, {5, 156, 175}, {9, 140, 175},
			{13, 124, 175}, {17, 108, 175}, {21, 92, 175}, {24, 80, 175}}},
		// 由左往右：第 1 步 432–439、第 5 步 432–471…**粒度 8**
		{WipeRight, false, []want{{1, 432, 439}, {5, 432, 471}, {9, 432, 503},
			{13, 432, 535}, {17, 432, 567}, {21, 432, 599}, {22, 432, 607}}},
		// 由右往左：第 1 步 600–607、第 5 步 568–607…
		{WipeLeft, false, []want{{1, 600, 607}, {5, 568, 607}, {9, 536, 607},
			{13, 504, 607}, {17, 472, 607}, {21, 440, 607}, {22, 432, 607}}},
	} {
		w := &Wipe{Kind: tc.kind, Rect: wipeRect}
		for _, x := range tc.want {
			r := w.Reveal(x.step)
			lo, hi := r.Min.X, r.Max.X-1
			if tc.axisY {
				lo, hi = r.Min.Y, r.Max.Y-1
			}
			if lo != x.lo || hi != x.hi {
				t.Errorf("方向 %d 第 %d 步露出 %d–%d，原版是 %d–%d",
					tc.kind, x.step, lo, hi, x.lo, x.hi)
			}
		}
		// 走到最後一定要蓋滿，否則會留一條舊畫面。
		if r := w.Reveal(w.Steps()); r != wipeRect {
			t.Errorf("方向 %d 最後一步露出 %v，想要 %v", tc.kind, r, wipeRect)
		}
		if r := w.Reveal(0); !r.Empty() {
			t.Errorf("方向 %d 第 0 步就露出了 %v", tc.kind, r)
		}
	}
}

// 每一步搬的是**新畫面對邊的那一塊**，不是逐條露出。
//
// 判準對著對拍量到的兩件事：
//   ① 動到的範圍是整個已蓋區（第 5 步 y80–99，不是只有新增的 y96–99）
//   ② 最後一步之後整塊就是新畫面
func TestWipeSlidesTheNewPictureIn(t *testing.T) {
	// 新畫面每一列（行）給一個不同的值，搬到哪裡一眼看得出來。
	mark := func(x, y int) color.RGBA {
		return color.RGBA{R: uint8(x & 0xff), G: uint8(y & 0xff), B: 0x40, A: 255}
	}
	for _, kind := range []WipeKind{WipeDown, WipeUp, WipeRight, WipeLeft} {
		from := image.NewRGBA(image.Rect(0, 0, 640, 408))
		to := image.NewRGBA(from.Bounds())
		old := color.RGBA{R: 0, G: 0, B: 0, A: 255}
		for y := 0; y < 408; y++ {
			for x := 0; x < 640; x++ {
				from.SetRGBA(x, y, old)
				to.SetRGBA(x, y, mark(x, y))
			}
		}
		dst := image.NewRGBA(from.Bounds())
		copy(dst.Pix, from.Pix)
		w := &Wipe{Kind: kind, Rect: wipeRect, From: from, To: to}
		n := 0
		for w.Advance(dst) {
			n++
			put := w.Reveal(n)
			src := w.Source(put)
			if src.Dx() != put.Dx() || src.Dy() != put.Dy() {
				t.Fatalf("方向 %d 第 %d 步：來源 %v 與去處 %v 不一樣大",
					kind, n, src, put)
			}
			dx, dy := put.Min.X-src.Min.X, put.Min.Y-src.Min.Y
			for y := wipeRect.Min.Y; y < wipeRect.Max.Y; y++ {
				for x := wipeRect.Min.X; x < wipeRect.Max.X; x++ {
					want := old
					if image.Pt(x, y).In(put) {
						want = mark(x-dx, y-dy)
					}
					if got := dst.RGBAAt(x, y); got != want {
						t.Fatalf("方向 %d 第 %d 步 (%d,%d) ＝ %v，想要 %v",
							kind, n, x, y, got, want)
					}
				}
			}
		}
		if n != w.Steps() {
			t.Errorf("方向 %d 走了 %d 步，總步數是 %d", kind, n, w.Steps())
		}
		// 最後一步之後整塊要就是新畫面——差一點點就會留下一條錯位的舊圖。
		for y := wipeRect.Min.Y; y < wipeRect.Max.Y; y++ {
			for x := wipeRect.Min.X; x < wipeRect.Max.X; x++ {
				if dst.RGBAAt(x, y) != to.RGBAAt(x, y) {
					t.Fatalf("方向 %d 走完之後 (%d,%d) 還不是新畫面", kind, x, y)
				}
			}
		}
		// 區域外一個像素都不准動。
		for y := 0; y < 408; y++ {
			for x := 0; x < 640; x++ {
				if image.Pt(x, y).In(wipeRect) {
					continue
				}
				if dst.RGBAAt(x, y) != old {
					t.Fatalf("方向 %d 動到區域外的 (%d,%d)", kind, x, y)
				}
			}
		}
	}
}

// 對拍量到的「與前一步差異的範圍」：那是整個已蓋區，不是新增的那一條。
func TestWipeTouchedAreaGrowsFromTheEdge(t *testing.T) {
	for _, tc := range []struct {
		kind   WipeKind
		step   int
		x0, y0 int
		x1, y1 int
	}{
		{WipeDown, 1, 432, 80, 607, 83}, {WipeDown, 5, 432, 80, 607, 99},
		{WipeDown, 9, 432, 80, 607, 115}, {WipeDown, 24, 432, 80, 607, 175},
		{WipeUp, 1, 432, 172, 607, 175}, {WipeUp, 5, 432, 156, 607, 175},
		{WipeRight, 1, 432, 80, 439, 175}, {WipeRight, 5, 432, 80, 471, 175},
		{WipeRight, 22, 432, 80, 607, 175},
		{WipeLeft, 1, 600, 80, 607, 175}, {WipeLeft, 5, 568, 80, 607, 175},
	} {
		w := &Wipe{Kind: tc.kind, Rect: wipeRect}
		r := w.Reveal(tc.step)
		want := image.Rect(tc.x0, tc.y0, tc.x1+1, tc.y1+1)
		if r != want {
			t.Errorf("方向 %d 第 %d 步動到 %v，原版量到 %v", tc.kind, tc.step, r, want)
		}
	}
}

func TestWipeEmptyRectDoesNothing(t *testing.T) {
	w := &Wipe{Kind: WipeDown}
	if w.Steps() != 0 || !w.Done() {
		t.Errorf("空矩形的步數 ＝ %d、Done ＝ %v", w.Steps(), w.Done())
	}
	if w.Advance(image.NewRGBA(image.Rect(0, 0, 4, 4))) {
		t.Error("空矩形還走得動")
	}
}
