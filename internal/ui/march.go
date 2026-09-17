package ui

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// 大地圖上的戰役（`0x1ecfc`，`docs/spec/005`「大地圖上的戰役」）。
//
// 電腦對電腦的戰役在大地圖上播：攻方的旗隊從攻方郡往守方郡走，守方的
// 旗隊站在守方郡，兩張都兩格交替。原版先把兩郡之間那一塊存到第二頁
// (408,36)，每一格在第二頁還原那一塊、按 y 的先後以遮罩（模式 5）＋
// OR（模式 2）貼兩隊，再把那一塊搬回第一頁。
//
// 先播十格原地踏步，然後每天一格、每天往守方郡走兩郡距離的 1/32；
// 日迴圈照結算（`0x1f538`）分出勝負為止。收尾把存著的底搬回第一頁。

// MarchPreroll 是原地踏步的格數（`0x1f1e7` 的迴圈）。
const MarchPreroll = 10

// MarchArtCount 是 `CVSC00`–`CVSC23` 的張數。
const MarchArtCount = 24

// marchSave 是第二頁上存底的位置（`0x1ee14`：`0x198`、`0x24`）。
const marchSaveX, marchSaveY = 0x198, 0x24

// MarchLayout 是一場戰役動畫的版面：兩郡的座標、存底的那一塊、四組圖。
type MarchLayout struct {
	AX, AY, DX, DY int // 攻方郡、守方郡的州郡座標（記錄 offset 6／8）
	X, Y, W, H     int // 存底並搬回的那一塊（畫面座標）
	// Att／AttMask／Def／DefMask 是兩格各自的第一張 `CVSC` 編號（第二格加一）。
	Att, AttMask, Def, DefMask int
}

// NewMarchLayout 照 `0x1ed09`–`0x1eef3` 算版面。
func NewMarchLayout(ax, ay, dx, dy int) MarchLayout {
	l := MarchLayout{AX: ax, AY: ay, DX: dx, DY: dy}
	tr8 := func(v int) int { // (v − 32) ÷ 8 往零截
		v -= 0x20
		if v < 0 {
			return -((-v) >> 3)
		}
		return v >> 3
	}
	abs := func(v int) int {
		if v < 0 {
			return -v
		}
		return v
	}
	l.X = (tr8(min(ax, dx)) + 10) << 3
	l.Y = tr8(min(ay, dy))<<3 + 0x2c
	l.W = (abs(ax-dx) + 0x40) >> 3 << 3
	l.H = (abs(ay-dy) + 0x40) >> 3 << 3
	switch {
	case l.H > l.W && dy < ay: // 守方在上
		l.Att, l.AttMask, l.Def, l.DefMask = 12, 20, 6, 22
	case l.H > l.W:
		l.Att, l.AttMask, l.Def, l.DefMask = 14, 22, 4, 20
	case dx < ax: // 守方在左
		l.Att, l.AttMask, l.Def, l.DefMask = 8, 16, 2, 18
	default:
		l.Att, l.AttMask, l.Def, l.DefMask = 10, 18, 0, 16
	}
	return l
}

// MarchArt 讀 `DATA3` 的 `CVSC00`–`CVSC23`。
func MarchArt(data3 *assets.Container) ([MarchArtCount]*assets.Image, error) {
	var out [MarchArtCount]*assets.Image
	for i := range out {
		name := fmt.Sprintf("CVSC%02d.IMG", i)
		j, ok := data3.ByName(name)
		if !ok {
			return out, fmt.Errorf("ui: DATA3 裡沒有 %s", name)
		}
		im, err := assets.DecodeImage(data3.Data(j))
		if err != nil {
			return out, err
		}
		out[i] = im
	}
	return out, nil
}

// March 是一場播放中的戰役動畫。Page1 是第二頁（640×408）：存底放在
// (408,36)，合成也在這一頁做；每一格的結果照原版搬回 Screen。
type March struct {
	L      MarchLayout
	Art    [MarchArtCount]*assets.Image
	Days   int // 日迴圈跑幾天（＝ 結算呼叫幾次）
	Screen *assets.Image
	Page1  *assets.Image
	frame  int
}

// NewMarch 從目前的畫面 screen（第一頁）起一場。page1 可以是 nil（全黑）。
func NewMarch(l MarchLayout, art [MarchArtCount]*assets.Image, days int, screen, page1 *assets.Image) *March {
	if page1 == nil {
		page1 = &assets.Image{W: assets.ScreenW, H: assets.ScreenH, Pix: make([]byte, assets.ScreenW*assets.ScreenH)}
	}
	m := &March{L: l, Art: art, Days: days, Screen: screen, Page1: page1}
	// 0x1ee4e：第一頁那一塊存到第二頁 (408,36)。
	marchCopy(screen, page1, l.X, l.Y, l.X+l.W-1, l.Y+l.H-1, marchSaveX, marchSaveY)
	return m
}

// Frames 是總格數：原地踏步十格加每天一格。
func (m *March) Frames() int { return MarchPreroll + m.Days }

// Step 畫下一格並搬回 Screen；已經畫完最後一格時收尾（存底搬回）並回 false。
func (m *March) Step() bool {
	l := m.L
	if m.frame >= m.Frames() {
		// 0x1f52a：第二頁存著的底搬回第一頁。
		marchCopy(m.Page1, m.Screen, marchSaveX, marchSaveY, marchSaveX+l.W-1, marchSaveY+l.H-1, l.X, l.Y)
		return false
	}
	k := m.frame
	m.frame++
	moved := 0 // 已經走了幾天
	if k > MarchPreroll {
		moved = k - MarchPreroll
	}
	f := k & 1
	// 還原那一塊（第二頁內 (408,36) → (X,Y)）。
	marchCopy(m.Page1, m.Page1, marchSaveX, marchSaveY, marchSaveX+l.W-1, marchSaveY+l.H-1, l.X, l.Y)
	// 攻方的位置：x ＝ trunc(攻X − 8 ＋ 天 × (守X − 攻X)/32 ＋ 80)（每一項都是 1/32 的倍數，
	// 單精度加起來沒有誤差）。
	ax := marchTrunc(float64(l.AX-8) + float64(moved*(l.DX-l.AX))/32 + 80)
	ay := marchTrunc(float64(l.AY-24) + float64(moved*(l.DY-l.AY))/32 + 44)
	dx, dy := l.DX+72, l.DY+20
	att := func() {
		marchPut(m.Page1, m.Art[l.AttMask+f], ax, ay, 5)
		marchPut(m.Page1, m.Art[l.Att+f], ax, ay, 2)
	}
	def := func() {
		marchPut(m.Page1, m.Art[l.DefMask+f], dx, dy, 5)
		marchPut(m.Page1, m.Art[l.Def+f], dx, dy, 2)
	}
	// 0x1f24a：y 大的後畫（蓋在上面）。
	if dy >= ay {
		att()
		def()
	} else {
		def()
		att()
	}
	marchCopy(m.Page1, m.Screen, l.X, l.Y, l.X+l.W-1, l.Y+l.H-1, l.X, l.Y)
	return true
}

// Speeds 是這一格（Step 之前的第 k 格）前後原版叫 `speak(0, 速度)` 的速度：
// before 在合成之前，after 在搬回之後。k ＝ Frames() 是收尾那五聲。
func (m *March) Speeds(k int) (before, after []int) {
	switch {
	case k < MarchPreroll:
		return []int{k}, []int{(k + 6) * 3}
	case k < m.Frames():
		day := k - MarchPreroll + 1
		return nil, []int{(day + 10) * 2}
	default:
		out := make([]int, 5)
		for j := range out {
			out[j] = j*3 + 0x19
		}
		return out, nil
	}
}

// Frame 是下一次 Step 要畫的格號。
func (m *March) Frame() int { return m.frame }

func marchTrunc(v float64) int {
	if v < 0 {
		return -int(-v)
	}
	return int(v)
}

// marchCopy 以位元組為單位搬矩形（`0110:1b3c`／`1bab`／`1c3b`／`1d5b` 的共同形狀）：
// 從 x1>>3 起搬 ((x2−x1)>>3)+1 個位元組，逐列由上往下、列內由左往右。
func marchCopy(src, dst *assets.Image, x1, y1, x2, y2, tx, ty int) {
	bytes := (x2-x1)>>3 + 1
	sx0, tx0 := (x1>>3)*8, (tx>>3)*8
	for r := 0; r <= y2-y1; r++ {
		sy, dy := y1+r, ty+r
		if sy < 0 || sy >= src.H || dy < 0 || dy >= dst.H {
			continue
		}
		for i := 0; i < bytes*8; i++ {
			sx, dx := sx0+i, tx0+i
			if sx < 0 || sx >= src.W || dx < 0 || dx >= dst.W {
				continue
			}
			dst.Pix[dy*dst.W+dx] = src.Pix[sy*src.W+sx]
		}
	}
}

// marchPut 是 `0x36c9:0x4c8` 的兩種模式：5 用圖的第 3 平面當遮罩把四個平面都
// AND 掉（遮罩 0 的格清成 0），2 把圖 OR 上去。
func marchPut(dst, im *assets.Image, x, y, mode int) {
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
			case 5:
				if v&8 == 0 {
					*d = 0
				}
			case 2:
				*d |= v
			}
		}
	}
}

// DrawMarch 把動畫那一塊（畫面座標）從 m.Screen 貼到畫布上。
func DrawMarch(c *Canvas, m *March) {
	l := m.L
	for y := l.Y; y < l.Y+l.H; y++ {
		for x := l.X; x < l.X+l.W; x++ {
			if x < 0 || y < 0 || x >= c.Img.Bounds().Dx() || y >= c.Img.Bounds().Dy() {
				continue
			}
			c.Img.SetRGBA(x, y, assets.EGAPalette[m.Screen.At(x, y)&15])
		}
	}
}
