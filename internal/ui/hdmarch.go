package ui

import (
	"image"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

type hdMarchKey struct{ sprite, mask [32]byte }

const hdMarchCacheLimit = 16

// maskedMarch 只把原版遮罩清底或 OR 寫入的格設為不透明。
func (p *HDPack) maskedMarch(sprite, mask *assets.Image) *image.RGBA {
	if p == nil || sprite == nil || mask == nil || sprite.W != 32 || sprite.H != 32 || mask.W != 32 || mask.H != 32 || len(sprite.Pix) != 1024 || len(mask.Pix) != 1024 {
		return nil
	}
	key := hdMarchKey{hdImageKey(sprite), hdImageKey(mask)}
	high := p.images[key.sprite]
	if high == nil || high.Bounds() != image.Rect(0, 0, 128, 128) {
		return nil
	}
	if cached := p.marchCache[key]; cached != nil {
		return cached
	}
	if len(p.marchCache) >= hdMarchCacheLimit {
		return nil
	}
	if p.marchCache == nil {
		p.marchCache = map[hdMarchKey]*image.RGBA{}
	}
	out := image.NewRGBA(high.Bounds())
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if mask.At(x, y)&8 != 0 && sprite.At(x, y)&15 == 0 {
				continue
			}
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 4; dx++ {
					out.SetRGBA(x*4+dx, y*4+dy, high.RGBAAt(x*4+dx, y*4+dy))
				}
			}
		}
	}
	p.marchCache[key] = out
	return out
}

func (c *Canvas) drawHighMarch(m *March) {
	if c.HD == nil || !m.painted || m.frame == 0 {
		return
	}
	l := m.L
	clip := image.Rect(l.X, l.Y, l.X+l.W, l.Y+l.H).Intersect(c.Img.Bounds())
	for _, pose := range m.sprites(m.frame - 1) {
		sprite, mask := m.Art[pose.sprite], m.Art[pose.mask]
		if sprite == nil || mask == nil || sprite.W != 32 || sprite.H != 32 || mask.W != 32 || mask.H != 32 || len(sprite.Pix) != 1024 || len(mask.Pix) != 1024 {
			continue
		}
		r := image.Rect(pose.x, pose.y, pose.x+32, pose.y+32).Intersect(clip)
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				sx, sy := x-pose.x, y-pose.y
				if mask.At(sx, sy)&8 == 0 || sprite.At(sx, sy)&15 != 0 {
					c.trackPixel(x, y)
				}
			}
		}
		if high := c.HD.maskedMarch(sprite, mask); high != nil {
			before := len(c.highOps)
			c.addHigh(high, r, r.Min.Sub(image.Pt(pose.x, pose.y)).Mul(4))
			if len(c.highOps) > before {
				c.highOps[len(c.highOps)-1].over = true
			}
		}
	}
}
