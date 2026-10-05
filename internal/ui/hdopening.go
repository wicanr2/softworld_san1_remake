package ui

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/opening"
)

const hdOpeningCaptureBudget = assets.ScreenW * assets.ScreenH * 16

// hdOpening 跟隨兩頁實際操作；擷取圖只留本次片頭，最多80條及一頁的位元組。
type hdOpening struct {
	pages        *opening.Pages
	pack         *HDPack
	p            [2]*image.RGBA
	captures     map[*assets.Image]*image.RGBA
	captureBytes int
	faded        *image.RGBA
}

func BindHDOpening(p *opening.Pages, pack *HDPack) {
	if p == nil || pack == nil || len(pack.images) == 0 {
		return
	}
	h := &hdOpening{pages: p, pack: pack, captures: map[*assets.Image]*image.RGBA{}}
	for i := range h.p {
		h.p[i] = image.NewRGBA(image.Rect(0, 0, assets.ScreenW*4, assets.ScreenH*4))
		scaleRGBA4(h.p[i], p.P[i].RGBA())
	}
	p.Observer = h
}

func (h *hdOpening) Clear(page int, c byte) {
	draw.Draw(h.p[page], h.p[page].Bounds(), image.NewUniform(assets.EGAPalette[c&15]), image.Point{}, draw.Src)
}

func (h *hdOpening) block(page, x, y int, c color.RGBA) {
	dst := h.p[page]
	o := dst.PixOffset(x*4, y*4)
	for sy := 0; sy < 4; sy++ {
		r := dst.Pix[o+sy*dst.Stride : o+sy*dst.Stride+16]
		for sx := 0; sx < 16; sx += 4 {
			r[sx], r[sx+1], r[sx+2], r[sx+3] = c.R, c.G, c.B, 255
		}
	}
}

func (h *hdOpening) source(im *assets.Image) *image.RGBA {
	if high := h.captures[im]; high != nil {
		return high
	}
	return h.pack.images[hdImageKey(im)]
}

func (h *hdOpening) Put(page int, im *assets.Image, x, y int, mode opening.Mode) {
	high := h.source(im)
	dst := h.p[page]
	w := im.W &^ 7
	if mode == opening.Copy && high != nil {
		r := image.Rect(x, y, x+w, y+im.H).Intersect(image.Rect(0, 0, assets.ScreenW, assets.ScreenH))
		if !r.Empty() {
			draw.Draw(dst, image.Rectangle{Min: r.Min.Mul(4), Max: r.Max.Mul(4)}, high, r.Min.Sub(image.Pt(x, y)).Mul(4), draw.Src)
		}
		return
	}
	for sy := max(0, -y); sy < min(im.H, assets.ScreenH-y); sy++ {
		for sx := max(0, -x); sx < min(w, assets.ScreenW-x); sx++ {
			v := im.At(sx, sy) & 15
			dx, dy := x+sx, y+sy
			if (mode == opening.And && v == 15) || ((mode == opening.Or || mode == opening.Xor) && v == 0) {
				continue
			}
			if mode < opening.Copy || mode > opening.And {
				continue
			}
			if mode == opening.And && v == 0 && high != nil {
				for ky := 0; ky < 4; ky++ {
					so := high.PixOffset(sx*4, sy*4+ky)
					do := dst.PixOffset(dx*4, dy*4+ky)
					copy(dst.Pix[do:do+16], high.Pix[so:so+16])
				}
			} else {
				h.block(page, dx, dy, assets.EGAPalette[h.pages.P[page].At(dx, dy)&15])
			}
		}
	}
}

func (h *hdOpening) CopyPage(from, to int) { copy(h.p[to].Pix, h.p[from].Pix) }

func (h *hdOpening) CopyRect(from, to, x1, y1, x2, y2, dx, dy int) {
	src, dst := h.p[from], h.p[to]
	w := (((x2 - x1) >> 3) + 1) * 8
	sx0, dx0 := (x1>>3)*8, (dx>>3)*8
	lo, hi := max(0, -sx0, -dx0), min(w, assets.ScreenW-sx0, assets.ScreenW-dx0)
	if hi <= lo {
		return
	}
	for r := 0; r <= y2-y1; r++ {
		sy, ty := y1+r, dy+r
		if sy < 0 || sy >= assets.ScreenH || ty < 0 || ty >= assets.ScreenH {
			continue
		}
		if from == to && sy == ty && dx0 > sx0 && dx0 < sx0+w {
			// 原版逐邏輯像素向右搬，會覆寫下一個來源格。
			for i := lo; i < hi; i++ {
				for ky := 0; ky < 4; ky++ {
					so := src.PixOffset((sx0+i)*4, sy*4+ky)
					do := dst.PixOffset((dx0+i)*4, ty*4+ky)
					copy(dst.Pix[do:do+16], src.Pix[so:so+16])
				}
			}
		} else {
			for ky := 0; ky < 4; ky++ {
				so := src.PixOffset((sx0+lo)*4, sy*4+ky)
				do := dst.PixOffset((dx0+lo)*4, ty*4+ky)
				n := (hi - lo) * 16
				copy(dst.Pix[do:do+n], src.Pix[so:so+n])
			}
		}
	}
}

func (h *hdOpening) Capture(page, x1, y1 int, im *assets.Image) {
	if len(h.captures) >= 80 || im.W <= 0 || im.H <= 0 || im.W > assets.ScreenW || im.H > assets.ScreenH {
		return
	}
	n := im.W * im.H * 16
	if h.captureBytes+n > hdOpeningCaptureBudget {
		return
	}
	out := image.NewRGBA(image.Rect(0, 0, im.W*4, im.H*4))
	draw.Draw(out, out.Bounds(), image.Black, image.Point{}, draw.Src)
	r := image.Rect(x1, y1, x1+im.W, y1+im.H).Intersect(image.Rect(0, 0, assets.ScreenW, assets.ScreenH))
	if !r.Empty() {
		d := r.Sub(image.Pt(x1, y1))
		draw.Draw(out, image.Rectangle{Min: d.Min.Mul(4), Max: d.Max.Mul(4)}, h.p[page], r.Min.Mul(4), draw.Src)
	}
	h.captures[im] = out
	h.captureBytes += n
}

// visible 的調色盤以原格色號分組。基色改成當拍EGA色，高清色差按亮度縮放。
// 最近鄰格可精確還原調色盤；高清的色差轉換是 remake 顯示近似。
func (h *hdOpening) visible() *image.RGBA {
	page := h.pages.Show & 1
	src := h.p[page]
	if h.pages.Pal == opening.DefaultPal {
		return src
	}
	if h.faded == nil {
		h.faded = image.NewRGBA(src.Bounds())
	}
	var base, current [16]color.RGBA
	var gain [16]int
	for i, v := range h.pages.Pal {
		base[i] = assets.EGAPalette[i]
		current[i] = opening.EGAColor(v)
		d := max(int(base[i].R), int(base[i].G), int(base[i].B))
		n := max(int(current[i].R), int(current[i].G), int(current[i].B))
		if d > 0 {
			gain[i] = n * 256 / d
		}
	}
	pix := h.pages.P[page].Pix
	for y := 0; y < assets.ScreenH; y++ {
		for x := 0; x < assets.ScreenW; x++ {
			code := pix[y*assets.ScreenW+x] & 15
			b, c := base[code], current[code]
			g := gain[code]
			for sy := 0; sy < 4; sy++ {
				o := src.PixOffset(x*4, y*4+sy)
				for sx := 0; sx < 16; sx += 4 {
					for ch := 0; ch < 3; ch++ {
						bv, cv := []uint8{b.R, b.G, b.B}[ch], []uint8{c.R, c.G, c.B}[ch]
						v := int(cv) + (int(src.Pix[o+sx+ch])-int(bv))*g/256
						h.faded.Pix[o+sx+ch] = uint8(max(0, min(255, v)))
					}
					h.faded.Pix[o+sx+3] = 255
				}
			}
		}
	}
	return h.faded
}
