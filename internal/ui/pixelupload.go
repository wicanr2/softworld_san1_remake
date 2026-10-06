package ui

import (
	"bytes"
	"image"
	"image/draw"
)

// PixelUpload 保存上次送出的像素，只傳送變更矩形。來源畫布保持。
type PixelUpload struct {
	previous *image.RGBA
	buffer   []byte
}

// Changed 回傳要更新的矩形及緊密排列的 RGBA；相同畫面回空矩形。
// 首次與尺寸變更會送整張；返回的封包只保證到下一次呼叫前有效。
func (u *PixelUpload) Changed(src *image.RGBA) (image.Rectangle, []byte) {
	b := src.Bounds()
	r := b
	if u.previous == nil || u.previous.Bounds() != b {
		u.previous = image.NewRGBA(b)
		u.buffer = nil
	} else {
		r = image.Rectangle{Min: b.Max, Max: b.Min}
		for y := b.Min.Y; y < b.Max.Y; y++ {
			a := src.PixOffset(b.Min.X, y)
			z := u.previous.PixOffset(b.Min.X, y)
			row, old := src.Pix[a:a+b.Dx()*4], u.previous.Pix[z:z+b.Dx()*4]
			if bytes.Equal(row, old) {
				continue
			}
			left, right := 0, len(row)-4
			for bytes.Equal(row[left:left+4], old[left:left+4]) {
				left += 4
			}
			for bytes.Equal(row[right:right+4], old[right:right+4]) {
				right -= 4
			}
			r.Min.X = min(r.Min.X, b.Min.X+left/4)
			r.Max.X = max(r.Max.X, b.Min.X+right/4+1)
			r.Min.Y = min(r.Min.Y, y)
			r.Max.Y = y + 1
		}
		if r.Empty() {
			return image.Rectangle{}, nil
		}
	}
	draw.Draw(u.previous, r, src, r.Min, draw.Src)
	if r == b && src.Stride == b.Dx()*4 {
		return r, src.Pix[:b.Dx()*b.Dy()*4]
	}
	n := r.Dx() * r.Dy() * 4
	if cap(u.buffer) < n {
		u.buffer = make([]byte, n)
	} else {
		u.buffer = u.buffer[:n]
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		i := src.PixOffset(r.Min.X, y)
		copy(u.buffer[(y-r.Min.Y)*r.Dx()*4:], src.Pix[i:i+r.Dx()*4])
	}
	return r, u.buffer
}
