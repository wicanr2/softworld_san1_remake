package assets

import (
	"image/color"
	"testing"
)

// TestDosboxXCursorIsACurFrame 拿 DOSBox-X 的錄影驗輸入游標：提示後面那 8×16 格
// 必須是該組 `CUR` 六格之一——遮罩挖空處逐點等於圖，遮罩留底處等於右邊一格的底色。
// dosgolem 那一側由 `internal/parity` 的對拍逐像素比過；這一支是第二個獨立實作。
func TestDosboxXCursorIsACurFrame(t *testing.T) {
	data1 := container(t, "DATA1")
	for _, c := range []struct {
		name  string
		path  string
		style CursorStyle
		x, y  int
	}{
		{"存檔(1-6) rec17#20", "../../workplace/rec17/frames/020-2Return.png", CursorMain, 472, 316},
		{"存檔(1-6) rec17#20b", "../../workplace/rec17/frames/020-2Return.b.png", CursorMain, 472, 316},
		{"查看卡片「請按任一鍵」 rec11#22", "../../workplace/rec11/frames/022-1Return.png", CursorMain, 504, 300},
		{"查看卡片「請按任一鍵」 rec11#22b", "../../workplace/rec11/frames/022-1Return.b.png", CursorMain, 504, 300},
		{"人數 rec7#9", "../../workplace/rec7/frames/009-1.png", CursorSetup, 576, 340},
		{"人數 rec7#9b", "../../workplace/rec7/frames/009-1.b.png", CursorSetup, 576, 340},
	} {
		im := openShot(t, c.path, "tools/dosboxx-record.sh 錄")
		frames, err := CursorFrames(data1, c.style)
		if err != nil {
			t.Fatal(err)
		}
		index := func(x, y int) int {
			q := color.RGBAModel.Convert(im.At(x, y)).(color.RGBA)
			for i, e := range EGAPalette {
				if e == q {
					return i
				}
			}
			return -1
		}
		// 遮罩挖空的格子（圖本身）要逐點相同；留底的格子拿右邊隔 8 點那一點估底色，
		// 地圖的網點底有雜點，估不準的只記數量。
		match, guessed := -1, 0
		for k, f := range frames {
			ok, miss := true, 0
			for dy := 0; dy < 16 && ok; dy++ {
				for dx := 0; dx < 8; dx++ {
					got := index(c.x+dx, c.y+dy)
					if f.Mask.At(dx, dy) == 0 {
						if got != int(f.Sprite.At(dx, dy)) {
							ok = false
							break
						}
						continue
					}
					if got != index(c.x+dx+8, c.y+dy)|int(f.Sprite.At(dx, dy)) {
						miss++
					}
				}
			}
			if ok && miss <= 8 {
				match, guessed = k, miss
				break
			}
		}
		if match < 0 {
			t.Errorf("%s：(%d,%d) 那一格不是 CUR%c 六格之一", c.name, c.x, c.y, 'A'+rune(c.style))
			continue
		}
		t.Logf("%s：(%d,%d) 是 CUR%c%d（留底的格子有 %d 點與估的底色不同）",
			c.name, c.x, c.y, 'A'+rune(c.style), match, guessed)
	}
}
