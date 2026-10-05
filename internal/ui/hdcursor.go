package ui

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

func hdCursorSource(c *assets.Container, name string) (assets.CursorFrame, string, error) {
	f := assets.CursorFrame{Name: name}
	maskName := strings.TrimSuffix(name, ".IMG") + "M.IMG"
	var raws [2][]byte
	for n, key := range []string{name, maskName} {
		i, ok := c.ByName(key)
		if !ok {
			return f, "", fmt.Errorf("游標缺少 %s", key)
		}
		raw := c.Data(i)
		im, err := assets.DecodeImage(raw)
		if err != nil || im.W != 8 || im.H != 16 {
			return f, "", fmt.Errorf("游標或遮罩尺寸不符")
		}
		raws[n] = raw
		if n == 0 {
			f.Sprite = im
		} else {
			f.Mask = im
		}
	}
	for i, m := range f.Mask.Pix {
		if m != 0 && m != 15 {
			return f, "", fmt.Errorf("游標遮罩不是二值")
		}
		if m == 15 && f.Sprite.Pix[i] != 0 {
			return f, "", fmt.Errorf("游標透明格有 OR 寫入")
		}
	}
	h := sha256.New()
	h.Write([]byte("san1-hd-cursor-v1\x00"))
	var length [4]byte
	for _, raw := range raws {
		binary.LittleEndian.PutUint32(length[:], uint32(len(raw)))
		h.Write(length[:])
		h.Write(raw)
	}
	return f, fmt.Sprintf("%x", h.Sum(nil)), nil
}

// 名稱區分同圖的 CURC1／5；兩個像素身份同時綁定來源與遮罩。
func hdCursorKey(f assets.CursorFrame) [32]byte {
	h := sha256.New()
	h.Write([]byte(f.Name))
	a, b := hdImageKey(f.Sprite), hdImageKey(f.Mask)
	h.Write(a[:])
	h.Write(b[:])
	var key [32]byte
	copy(key[:], h.Sum(nil))
	return key
}

func validateHighCursor(f assets.CursorFrame, high *image.RGBA) error {
	for y := 0; y < 64; y++ {
		for x := 0; x < 32; x++ {
			col := high.RGBAAt(x, y)
			if f.Mask.At(x/4, y/4) == 15 {
				if col != (color.RGBA{}) {
					return fmt.Errorf("游標 identity 格不是透明")
				}
			} else if col.A != 255 {
				return fmt.Errorf("游標輪廓格不是不透明")
			}
		}
	}
	return nil
}

func (c *Canvas) highCursor(f assets.CursorFrame) *image.RGBA {
	if c.HD == nil || f.Name == "" || f.Sprite == nil || f.Mask == nil {
		return nil
	}
	return c.HD.cursors[hdCursorKey(f)]
}

func (c *Canvas) drawHighCursor(high *image.RGBA, x, y int) {
	if high == nil {
		return
	}
	before := len(c.highOps)
	c.addHigh(high, image.Rect(x, y, x+8, y+16), image.Point{})
	if len(c.highOps) > before {
		c.highOps[len(c.highOps)-1].over = true
	}
}
