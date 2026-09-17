package ui

import (
	"image"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// TestInputCursorSitsAfterTheLastLine 釘住主畫面的輸入游標：只畫在下面板最後一行
// 字後面那 8×16 格（主命令「…下您的命令:」之後，原版是 (544,316)），
// 六格一格接一格都不同，沒在等輸入就不畫。
func TestInputCursorSitsAfterTheLastLine(t *testing.T) {
	a, g := artSessionFixture(t)
	face := testFace(t)
	draw := func(in InputCursor) *Canvas {
		c := NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
		DrawArtSession(c, a, g, nil, View{Prompt: "新君主主公,請到(41)\n南海下您的命令:", Input: in})
		return c
	}
	plain := draw(InputCursor{})
	cell := image.Rect(544, 316, 552, 332)
	var prev *Canvas
	for k := 0; k < assets.MenuOrnamentFrameCount; k++ {
		cv := draw(InputCursor{On: true, Frame: k})
		inside := 0
		for y := 0; y < assets.ScreenH; y++ {
			for x := 0; x < assets.ScreenW; x++ {
				if cv.Img.RGBAAt(x, y) == plain.Img.RGBAAt(x, y) {
					continue
				}
				if !(image.Point{x, y}).In(cell) {
					t.Fatalf("第 %d 格：(%d,%d) 在游標那一格外面也變了", k, x, y)
				}
				inside++
			}
		}
		if inside == 0 {
			t.Errorf("第 %d 格游標沒有畫出來", k)
		}
		if prev != nil && string(prev.Img.Pix) == string(cv.Img.Pix) {
			t.Errorf("第 %d 格與前一格一樣", k)
		}
		prev = cv
	}
	if got := CursorFrameAt(CursorTicksPerFrame*7 + 1); got != 1 {
		t.Errorf("第 %d 個節拍該是第 1 格，得到 %d", CursorTicksPerFrame*7+1, got)
	}
}

// TestMessageLinesWrapLikeTheOriginal 釘住訊息常式的排法：一行剛好寫滿，游標立刻換行，
// 緊接的換行字元再換一行（「<偽書使疑>派細作到那一郡」24 格，原版「(1-42):」在第三行）。
func TestMessageLinesWrapLikeTheOriginal(t *testing.T) {
	got := MessageLines("<偽書使疑>派細作到那一郡\n(1-42):", 24)
	want := []string{"<偽書使疑>派細作到那一郡", "", "(1-42):"}
	if len(got) != len(want) {
		t.Fatalf("排成 %q，該是 %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("排成 %q，該是 %q", got, want)
		}
	}
	if got := MessageLines("儲存進度\n(1-6):", 24); len(got) != 2 {
		t.Errorf("沒寫滿的行不該多一行：%q", got)
	}
}
