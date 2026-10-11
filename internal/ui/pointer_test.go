package ui

import (
	"image"
	"testing"
)

func TestPointerUsesRenderedTextPositions(t *testing.T) {
	c := &Canvas{Img: image.NewRGBA(image.Rect(0, 0, 640, 408))}
	c.pointerLine(424, 300, "1.訓練 2.招募", CellW, CellH)
	for _, v := range []struct{ x, key int }{{425, 1}, {480, 2}} {
		h, ok := c.PointerAt(image.Pt(v.x, 305))
		if !ok || h.Value != v.key {
			t.Fatalf("x=%d: %v %v", v.x, h, ok)
		}
	}
	c.pointerLine(424, 320, "10.將軍姓名", CellW, CellH)
	if h, ok := c.PointerAt(image.Pt(425, 321)); !ok || h.Kind != PointerNumber || h.Value != 10 {
		t.Fatal("兩位數人物編號未完整辨識")
	}
	if _, ok := c.PointerAt(image.Pt(400, 305)); ok {
		t.Fatal("點選區溢出文字區")
	}
	c.AddPointer(image.Rect(424, 300, 440, 316), PointerKey, 27)
	if h, _ := c.PointerAt(image.Pt(425, 305)); h.Value != 27 {
		t.Fatal("上層控制未攔截")
	}
}
