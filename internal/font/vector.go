package font

import (
	"fmt"
	"image"
	"image/draw"
	"io"
	"sync"

	xdraw "golang.org/x/image/draw"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

const vectorCacheBytes = 16 << 20

type vectorKey struct {
	r    rune
	w, h int
}

type vectorFace struct {
	font  *sfnt.Font
	mu    sync.Mutex
	cache map[vectorKey]*image.Alpha
	bytes int
}

// LoadVector 附加可選的向量來源。只影響 VectorGlyph，原點陣與格寬保持。
// 在將 Face 交給 UI 前呼叫；解析失敗時保留既有字型。
func (f *Face) LoadVector(r io.Reader) error {
	const limit = 32 << 20
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return err
	}
	if len(b) > limit {
		return fmt.Errorf("向量字型超過 32 MiB")
	}
	parsed, err := opentype.Parse(b)
	if err != nil {
		return err
	}
	f.vector = &vectorFace{font: parsed, cache: map[vectorKey]*image.Alpha{}}
	return nil
}

// VectorIdentity 供合成快取辨識向量來源；未載入時為 nil。
func (f *Face) VectorIdentity() any {
	if f.vector == nil {
		return nil
	}
	return f.vector
}

// VectorGlyph 以目前字格直接畫向量覆蓋率。傳回的字模唯讀。
// 缺字、無向量來源或尺寸不適合時回 false，由 UI 沿用原點陣。
func (f *Face) VectorGlyph(r rune, w, h int) (*image.Alpha, bool) {
	g, ok := f.Glyph(r)
	if !ok || f.vector == nil || f.H != 16 || w < 1 || h < 1 || w > 512 || h > 512 {
		return nil, false
	}
	v := f.vector
	v.mu.Lock()
	defer v.mu.Unlock()
	key := vectorKey{r, w, h}
	if mask := v.cache[key]; mask != nil {
		return mask, true
	}
	var buf sfnt.Buffer
	index, err := v.font.GlyphIndex(&buf, r)
	if err != nil || index == 0 {
		return nil, false
	}
	face, err := opentype.NewFace(v.font, &opentype.FaceOptions{Size: float64(h), DPI: 72, Hinting: xfont.HintingNone})
	if err != nil {
		return nil, false
	}
	defer face.Close()
	naturalW := max(1, g.W*h/f.H)
	mask := image.NewAlpha(image.Rect(0, 0, naturalW, h))
	advance, ok := face.GlyphAdvance(r)
	if !ok {
		return nil, false
	}
	x := max(fixed.Int26_6(0), (fixed.I(naturalW)-advance)/2)
	d := xfont.Drawer{Dst: mask, Src: image.White, Face: face, Dot: fixed.Point26_6{X: x, Y: fixed.I(13 * h / 16)}}
	d.DrawString(string(r))
	if naturalW != w {
		resized := image.NewAlpha(image.Rect(0, 0, w, h))
		xdraw.NearestNeighbor.Scale(resized, resized.Bounds(), mask, mask.Bounds(), draw.Src, nil)
		mask = resized
	}
	if v.bytes+len(mask.Pix) > vectorCacheBytes {
		clear(v.cache)
		v.bytes = 0
	}
	v.cache[key] = mask
	v.bytes += len(mask.Pix)
	return mask, true
}
