package assets

import "sync"

var expandedPortraits sync.Map

// ExpandedPortrait 是新增人物在原貌或缺圖時的通用框，不冒用原版肖像。
// 小色塊保留來源識別，讓 HD 對應鍵彼此獨立；不含原版或 San5 美術。
func ExpandedPortrait(source int) *Image {
	if source < 0 || source > 999 {
		return nil
	}
	if v, ok := expandedPortraits.Load(source); ok {
		return v.(*Image)
	}
	im := &Image{W: 64, H: 80, Pix: make([]byte, 64*80)}
	for y := 0; y < 80; y++ {
		for x := 0; x < 64; x++ {
			dx, dy := x-32, y-27
			if dx*dx*20*20+dy*dy*14*14 <= 14*14*20*20 || (y >= 50 && (x-32)*(x-32)*28*28+(y-79)*(y-79)*30*30 <= 30*30*28*28) {
				im.Pix[y*64+x] = 7
			}
		}
	}
	for i := 0; i < 3; i++ {
		im.Pix[79*64+i] = byte((source >> (4 * i)) & 15)
	}
	v, _ := expandedPortraits.LoadOrStore(source, im)
	return v.(*Image)
}
