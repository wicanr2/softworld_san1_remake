package assets

import (
	"fmt"
	"image"
	"image/color"
)

// 原版的點陣圖：`.IMG`（畫面、圖塊）與 `.FAC`（人物肖像）。
//
// 版面（`docs/formats/07`）：
//
//	uint16  高（列）
//	uint16  寬（像素）
//	···     四個**完整的位元平面**，每個平面 (寬+7)/8 × 高 個位元組，
//	        每個位元組由左而右（最高位在左）
//
// ⚠ **表頭是「高、寬」不是「寬、高」。** `F000.FAC` 的前四個位元組是
// `50 00 40 00`＝80、64，而肖像是 64 寬 80 高。反過來讀不會報錯，
// 只會得到一張寬高互換的雜訊。

// EGAPalette 是 EGA 的十六色。
//
// ⚠ **這是 EGA 的預設色盤，不是原版設定的那一份。** 原版跑 mode 10h，
// 有沒有重寫色盤暫存器還沒查（`EGAFILL.PAL` 是填色圖樣表不是色盤，
// 1024 個位元組，`docs/formats/07` §4）。
var EGAPalette = [16]color.RGBA{
	{0x00, 0x00, 0x00, 0xFF}, {0x00, 0x00, 0xAA, 0xFF},
	{0x00, 0xAA, 0x00, 0xFF}, {0x00, 0xAA, 0xAA, 0xFF},
	{0xAA, 0x00, 0x00, 0xFF}, {0xAA, 0x00, 0xAA, 0xFF},
	{0xAA, 0x55, 0x00, 0xFF}, {0xAA, 0xAA, 0xAA, 0xFF},
	{0x55, 0x55, 0x55, 0xFF}, {0x55, 0x55, 0xFF, 0xFF},
	{0x55, 0xFF, 0x55, 0xFF}, {0x55, 0xFF, 0xFF, 0xFF},
	{0xFF, 0x55, 0x55, 0xFF}, {0xFF, 0x55, 0xFF, 0xFF},
	{0xFF, 0xFF, 0x55, 0xFF}, {0xFF, 0xFF, 0xFF, 0xFF},
}

// planeBit 是第 n 個平面對到色號的哪一個位元。
//
// 平面依 **I、R、G、B** 的順序存：平面 0 是亮度、1 是紅、2 是綠、3 是藍，
// 對到色號的 bit 3、2、1、0。
//
// `L0`：拿 dosgolem 跑原版到君主選擇畫面，把 EGA framebuffer 存成圖，
// 再把解出來的 `F000.FAC`／`F005.FAC` 拿去畫面裡找——在 (488,56) 與
// (420,56) **逐像素 100% 相符**（5,120 個像素全中）。
// 平面與顏色的對應只換顏色不換結構，二十四種排列在結構上一樣，
// 所以只有拿原版自己算出來的色號比對才分得出來；
// `cmd/san1imgcheck` 是那個比對工具，隨時可以重跑。
var planeBit = [4]uint{3, 2, 1, 0}

// Image 是一張解出來的點陣圖。Pix 每個位元組是一個像素的色號（0–15）。
type Image struct {
	W, H int
	Pix  []byte
}

// ImageHeader 是表頭的長度。
const ImageHeader = 4

// DecodeImage 解一張 `.IMG` 或 `.FAC`。
func DecodeImage(b []byte) (*Image, error) {
	if len(b) < ImageHeader {
		return nil, fmt.Errorf("assets: 圖只有 %d 個位元組", len(b))
	}
	h := int(b[0]) | int(b[1])<<8
	w := int(b[2]) | int(b[3])<<8
	if w <= 0 || h <= 0 || w > 4096 || h > 4096 {
		return nil, fmt.Errorf("assets: 圖的尺寸是 %d×%d", w, h)
	}
	stride := (w + 7) / 8
	body := b[ImageHeader:]
	if want := stride * h * 4; len(body) != want {
		return nil, fmt.Errorf("assets: %d×%d 的圖要 %d 個位元組，有 %d",
			w, h, want, len(body))
	}
	im := &Image{W: w, H: h, Pix: make([]byte, w*h)}
	plane := stride * h
	for p := 0; p < 4; p++ {
		bit := byte(1) << planeBit[p]
		base := p * plane
		for y := 0; y < h; y++ {
			row := base + y*stride
			for x := 0; x < w; x++ {
				if body[row+x/8]&(0x80>>(uint(x)%8)) != 0 {
					im.Pix[y*w+x] |= bit
				}
			}
		}
	}
	return im, nil
}

// RGBA 把色號換成顏色。
func (im *Image) RGBA() *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, im.W, im.H))
	for y := 0; y < im.H; y++ {
		for x := 0; x < im.W; x++ {
			out.SetRGBA(x, y, EGAPalette[im.Pix[y*im.W+x]&15])
		}
	}
	return out
}

// IsImage 回報一個項目看起來是不是圖：表頭的尺寸與長度對得上。
//
// **用尺寸驗而不是用副檔名**：容器裡有 `.IMG`、`.FAC` 兩種副檔名，
// 而且將來可能還有別的；對得上就是圖，對不上就不要硬解。
func IsImage(b []byte) bool {
	_, err := DecodeImage(b)
	return err == nil
}
