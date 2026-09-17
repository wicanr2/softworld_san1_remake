// Package opening 是原版開機片頭（`DATA0.GRP` 那支 overlay 的 `0ad0:011a`）
// 的畫面腳本：商標、標題字、製作人員、海景船隊、寫詞、淡出、頭像橫幅、
// 三英圖捲入、載入中（`docs/spec/005`「片頭」）。
//
// 原版在 640×408 的 EGA 平面上開兩頁（`A000`／`A800`），畫在看不到的那一頁、
// 再切顯示頁或整頁互拷。這裡照同一組原語重做——每一步搬什麼都對得回
// 原版的一支常式，對拍（`internal/parity` 的 `TestZZOpeningMatchesTheOriginal`）
// 就能在原版每一次等待、每一步動畫的同一刻逐格比顯示中的那一頁。
//
// 不依賴 Ebiten：畫面是索引色的 `assets.Image`，無頭環境也跑得動。
package opening

import (
	"image/color"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// Mode 是貼圖的運算（`0110:1486` 依第四個參數分派）。
type Mode int

const (
	Copy Mode = iota // 覆蓋
	Xor
	Or
	And // 圖外那幾個位元補 1，所以只清掉圖裡是 0 的格
)

// DefaultPal 是 EGA 開機時的屬性暫存器（`0aaa:000a` 那份表）。
var DefaultPal = [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x14, 0x07,
	0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e, 0x3f}

// Pages 是兩頁畫面與顯示狀態。
type Pages struct {
	P [2]*assets.Image
	// Draw 是貼圖、清畫面寫進哪一頁（`0110:0066`：1 → `A800`，0 → `A000`）。
	Draw int
	// Show 是顯示哪一頁（`0110:1deb`，`int 10h AH=05h`）。
	Show int
	// Pal 是 16 個屬性暫存器（EGA 6 位元色）：`0aaa:0232` 改一格再整份送出，
	// `0aaa:0251` 還原成 DefaultPal。
	Pal [16]byte
}

// NewPages 開兩頁全黑的畫面。
func NewPages() *Pages {
	p := &Pages{Pal: DefaultPal}
	for i := range p.P {
		p.P[i] = &assets.Image{W: assets.ScreenW, H: assets.ScreenH,
			Pix: make([]byte, assets.ScreenW*assets.ScreenH)}
	}
	return p
}

// Visible 是顯示中的那一頁。
func (p *Pages) Visible() *assets.Image { return p.P[p.Show&1] }

// Clear 把畫的那一頁整頁填成 c（`0110:0c5f`）。
func (p *Pages) Clear(c byte) {
	pix := p.P[p.Draw&1].Pix
	for i := range pix {
		pix[i] = c & 15
	}
}

// Put 把 im 以 mode 貼到畫的那一頁的 (x, y)（`0110:0dd8` 是 Copy，
// `0110:1486` 依 mode 分派）。原版逐平面、逐位元組寫，x 不在位元組邊界時
// 用字組右移把圖的位元錯開——換成逐像素就是下面這四種運算。
func (p *Pages) Put(im *assets.Image, x, y int, mode Mode) {
	dst := p.P[p.Draw&1]
	// 原版一列搬 `寬 >> 3` 個位元組，寬不是 8 的倍數時右邊那幾格不畫。
	w := im.W &^ 7
	for sy := 0; sy < im.H; sy++ {
		dy := y + sy
		if dy < 0 || dy >= dst.H {
			continue
		}
		for sx := 0; sx < w; sx++ {
			dx := x + sx
			if dx < 0 || dx >= dst.W {
				continue
			}
			v := im.Pix[sy*im.W+sx] & 15
			d := &dst.Pix[dy*dst.W+dx]
			switch mode {
			case Copy:
				*d = v
			case Xor:
				*d ^= v
			case Or:
				*d |= v
			case And:
				*d &= v
			}
		}
	}
}

// CopyPage 整頁複製（`0110:1a40` 是 0 → 1，`0110:1a6a` 是 1 → 0）。
func (p *Pages) CopyPage(from, to int) {
	copy(p.P[to&1].Pix, p.P[from&1].Pix)
}

// CopyRect 把 from 頁的 (x1..x2, y1..y2) 搬到 to 頁的 (dx, dy)。
//
// **以位元組為單位**：從第 x1>>3 個位元組起搬 ((x2−x1)>>3)+1 個，落點是
// 第 dx>>3 個位元組。所以 (212, 251) 搬的是 208..247，不是 212..251
// （`0110:1acd` 0→1、`0110:1c3b` 1→0、`0110:1ccb` 0→0、`0110:1bab`
// 0→1 帶落點）。逐列由上往下、列內由左往右，同一頁往左搬時不會自己蓋自己。
func (p *Pages) CopyRect(from, to, x1, y1, x2, y2, dx, dy int) {
	src, dst := p.P[from&1], p.P[to&1]
	bytes := (x2-x1)>>3 + 1
	sx0, dx0 := (x1>>3)*8, (dx>>3)*8
	for r := 0; r <= y2-y1; r++ {
		sy, ty := y1+r, dy+r
		if sy < 0 || sy >= src.H || ty < 0 || ty >= dst.H {
			continue
		}
		for i := 0; i < bytes*8; i++ {
			sx, tx := sx0+i, dx0+i
			if sx < 0 || sx >= src.W || tx < 0 || tx >= dst.W {
				continue
			}
			dst.Pix[ty*dst.W+tx] = src.Pix[sy*src.W+sx]
		}
	}
}

// Capture 從畫的那一頁取一塊 (x1..x2, y1..y2) 成圖（`0ad0` 經 `0eba:0164`）。
func (p *Pages) Capture(x1, y1, x2, y2 int) *assets.Image {
	src := p.P[p.Draw&1]
	w, h := x2-x1+1, y2-y1+1
	im := &assets.Image{W: w, H: h, Pix: make([]byte, w*h)}
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w; xx++ {
			im.Pix[yy*w+xx] = src.At(x1+xx, y1+yy)
		}
	}
	return im
}

// EGAColor 把 6 位元的 EGA 色（`rgbRGB`：大寫是 2/3 強度、小寫是 1/3）
// 換成 RGBA。預設那份表換出來就是 `assets.EGAPalette`。
func EGAColor(v byte) color.RGBA {
	c := func(hi, lo byte) uint8 {
		return uint8(hi*0xAA + lo*0x55)
	}
	return color.RGBA{
		R: c(v>>2&1, v>>5&1),
		G: c(v>>1&1, v>>4&1),
		B: c(v&1, v>>3&1),
		A: 0xFF,
	}
}

// Palette 是目前屬性暫存器換出來的 16 色。
func (p *Pages) Palette() [16]color.RGBA {
	var out [16]color.RGBA
	for i, v := range p.Pal {
		out[i] = EGAColor(v)
	}
	return out
}
