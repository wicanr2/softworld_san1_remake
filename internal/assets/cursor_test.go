package assets

import "testing"

// TestCursorFramesLoadEveryStyle 釘住四組游標（`CURA`–`CURD`）各六格都讀得出來，
// 而且每一格的圖只畫在遮罩挖空的地方——`(底 AND 遮罩) OR 圖` 才不會把底色
// 混進游標的顏色。
func TestCursorFramesLoadEveryStyle(t *testing.T) {
	data1 := container(t, "DATA1")
	for style := CursorStyle(0); style < 4; style++ {
		frames, err := CursorFrames(data1, style)
		if err != nil {
			t.Fatal(err)
		}
		ink := 0
		for k, f := range frames {
			for y := 0; y < 16; y++ {
				for x := 0; x < 8; x++ {
					if f.Sprite.At(x, y) == 0 {
						continue
					}
					ink++
					if f.Mask.At(x, y) != 0 {
						t.Errorf("CUR%c%d 在 (%d,%d) 有圖但遮罩沒挖空", 'A'+rune(style), k, x, y)
					}
				}
			}
		}
		if ink == 0 {
			t.Errorf("CUR%c 六格都沒有圖", 'A'+rune(style))
		}
	}
}
