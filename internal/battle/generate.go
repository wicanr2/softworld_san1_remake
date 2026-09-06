package battle

import "sort"

// 戰場的地形生成。
//
// ⚠ **這是 remake 自己畫的，不是原版的郡地理誌。** 原版那張圖在
// `DATA2`／`DATA3` 的 `.OKR`（465 項，壓縮過、格式未解），而且是美術素材，
// 不重製也不散布。
//
// 生成器是**決定性**的：輸入是郡編號、郡的屬性與鄰郡清單，
// 同一個郡永遠得到同一張圖。所以整場戰役仍然可重現，對拍也成立。
//
// 生成的規則來自手冊對戰場的描述：
//
//   - 「圖中的數字位置代表前往鄰近州郡的通道，也是鄰郡攻入時的發兵地點」
//     （p.19）——所以每個鄰郡在邊界上有一個入口
//   - 城池在場上（守方守的就是它，p.35）
//   - 關寨是玩家蓋的，每郡最多五座（p.21）
//   - 地形有平原、沙漠、山丘、樹林、淺水、深水、城池、關寨、大山（p.31–33）

// Params 是生成一張戰場要的東西。
type Params struct {
	Prefecture int   // 郡編號，當種子
	Neighbours []int // 鄰郡編號，決定入口
	Forts      int   // 城寨數（0..5）

	// LandValue／FloodRate 影響地貌：開發得好的地方平原多，
	// 常淹水的地方水域多。**這是 remake 的詮釋**，手冊沒說。
	LandValue uint8
	FloodRate uint8
}

// 戰場尺寸。手冊沒給；取一個「三十天走得完但走不快」的大小。
const (
	FieldW = 21
	FieldH = 15
)

// Generate 造一張戰場。
func Generate(p Params) *Field {
	f := &Field{W: FieldW, H: FieldH, cell: make([]Terrain, FieldW*FieldH),
		Gates: map[int]Hex{}}
	rng := newRand(uint32(p.Prefecture)*2654435761 + 0x9E3779B9)

	// 底：平原為主，土地價值越低沙漠越多。
	for y := 0; y < FieldH; y++ {
		for x := 0; x < FieldW; x++ {
			t := Plain
			if int(rng.next()%100) >= int(p.LandValue)+40 {
				t = Desert
			}
			f.cell[y*FieldW+x] = t
		}
	}
	// 山脈：從一側長進來，中央留通道。
	ridges := 2 + int(rng.next()%3)
	for i := 0; i < ridges; i++ {
		x := int(rng.next() % FieldW)
		y := int(rng.next() % FieldH)
		h := FromOffset(x, y)
		for n := 0; n < 4+int(rng.next()%6); n++ {
			f.Set(h, Hill)
			if rng.next()%4 == 0 {
				f.Set(h, Mountain)
			}
			h = h.Step(Dirs()[rng.next()%6])
		}
	}
	// 樹林。
	for i := 0; i < 6+int(rng.next()%6); i++ {
		h := FromOffset(int(rng.next()%FieldW), int(rng.next()%FieldH))
		for n := 0; n < 3+int(rng.next()%4); n++ {
			if f.At(h) != Mountain {
				f.Set(h, Forest)
			}
			h = h.Step(Dirs()[rng.next()%6])
		}
	}
	// 水域：洪水率越高越多。河從一邊流到另一邊。
	if p.FloodRate > 20 {
		streams := 1 + int(p.FloodRate)/40
		for i := 0; i < streams; i++ {
			y := int(rng.next() % FieldH)
			h := FromOffset(0, y)
			for x := 0; x < FieldW; x++ {
				f.Set(h, Shallow)
				if rng.next()%5 == 0 {
					f.Set(h, Deep)
				}
				d := DirDownRight
				switch rng.next() % 3 {
				case 0:
					d = DirUpRight
				case 1:
					d = DirDownRight
				default:
					d = DirDownRight
				}
				h = h.Step(d)
				if !f.InBounds(h) {
					h = FromOffset(x+1, y)
				}
			}
		}
	}

	// 城池放中央。
	f.CityAt = FromOffset(FieldW/2, FieldH/2)
	f.Set(f.CityAt, City)

	// 城寨：城池周圍，最多五座。
	forts := p.Forts
	if forts > 5 {
		forts = 5
	}
	for i := 0; i < forts; i++ {
		h := f.CityAt.Step(Dirs()[i%6]).Step(Dirs()[(i+2)%6])
		if f.At(h).Passable() && h != f.CityAt {
			f.Set(h, Fort)
		}
	}

	// 入口：每個鄰郡在邊界上一個，依編號排序讓結果穩定。
	ns := append([]int(nil), p.Neighbours...)
	sort.Ints(ns)
	border := borderRing()
	for i, n := range ns {
		if len(border) == 0 {
			break
		}
		h := border[(i*len(border)/max(1, len(ns)))%len(border)]
		// 入口一定走得進去。
		if !f.At(h).Passable() {
			f.Set(h, Plain)
		}
		f.Gates[n] = h
	}
	return f
}

// borderRing 是戰場邊界上的格子，順時針。
func borderRing() []Hex {
	var out []Hex
	for x := 0; x < FieldW; x++ {
		out = append(out, FromOffset(x, 0))
	}
	for y := 1; y < FieldH; y++ {
		out = append(out, FromOffset(FieldW-1, y))
	}
	for x := FieldW - 2; x >= 0; x-- {
		out = append(out, FromOffset(x, FieldH-1))
	}
	for y := FieldH - 2; y > 0; y-- {
		out = append(out, FromOffset(0, y))
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// rand 是決定性的亂數，只給生成器用。
//
// **不要拿系統亂數。** 同一個郡要永遠得到同一張戰場，
// 否則同一局重跑會不一樣，對拍就沒有基礎。
type rand struct{ s uint32 }

func newRand(seed uint32) *rand {
	if seed == 0 {
		seed = 1
	}
	return &rand{s: seed}
}

func (r *rand) next() uint32 {
	// xorshift32
	r.s ^= r.s << 13
	r.s ^= r.s >> 17
	r.s ^= r.s << 5
	return r.s
}
