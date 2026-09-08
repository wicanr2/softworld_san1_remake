package battle

import "fmt"

// 原版的戰場地圖。
//
// 每個郡的戰場**是劇本檔裡的靜態資料**，不是算出來的：州郡記錄
// offset 55–174 共 120 個位元組，戰鬥一開始就整份複製到工作區
// （`0x20200` 的第一段迴圈，`docs/re/05` §1）。42 個郡 42 張圖，
// 沒有兩張相同。

// 原版的戰場是 12 欄 × 10 列，索引 `列 × 12 + 欄`。
const (
	FieldW = 12
	FieldH = 10
)

// FieldBytes 是一張戰場在州郡記錄裡佔的位元組數。
const FieldBytes = FieldW * FieldH

// 一格位元組的兩半：低四位是地形，高四位是標記。
const (
	cellTerrain = 0x0F
	cellMark    = 0xF0
	cellNone    = 0xFF // 圖外
)

// 高四位的標記。0–9 是通往第幾個鄰郡（索引 Neighbours），
// 10 是城池，11–14 是四個軍團的起點，15 是沒有標記。
const (
	markCity  = 10
	markArmy0 = 11
	markNone  = 15
)

// ArmySlots 是一張戰場上的軍團起點數。原版一場最多四個軍團
// （攻方兩郡、守方兩郡，`0x2053c` 依序放四次）。
const ArmySlots = 4

// terrainOf 把原版的地形碼換成 Terrain。
//
// 對照是量出來的，兩條獨立證據：移動力表 `DS:0x7c42` 與說明書 p.29
// 的表逐格相同，而地形碼 5 在每一張圖上剛好出現一次、位置就是
// 城池的標記格。
var terrainOf = [16]Terrain{
	1: Mountain, 2: Hill, 3: Shallow, 4: Deep, 5: City,
	6: Fort, 7: Plain, 8: Forest, 9: Desert,
}

// TerrainOfCode 把原版的地形碼（`0..15`，戰場地圖每格的低四位）
// 換成 Terrain。碼 0 與 10–15 沒有定義，一律回 Plain。
//
// 對拍要拿原版自己的地圖去查花費，所以這個對照要能從外面呼叫。
func TerrainOfCode(code byte) Terrain { return terrainOf[code&0x0f] }

// terrainCode 是 terrainOf 的反向，寫回原版格式用。
//
// **不能直接把 terrainOf 倒過來走**：`Plain` 的列舉值是 0，而
// terrainOf 沒定義的碼（0、10–15）預設也是 0，倒著走會讓平原
// 對到最後一個沒定義的碼。只認 1–9 這幾個真的有意義的。
var terrainCode = func() [terrainCount]byte {
	var m [terrainCount]byte
	for code := 1; code <= 9; code++ {
		m[terrainOf[code]] = byte(code)
	}
	return m
}()

// Load 把州郡記錄裡的 120 個位元組讀成一張戰場。
//
// neighbours 是那個郡的相鄰表（州郡 offset 45–54），高四位 0–9
// 就是它的索引；超出清單長度的標記當成沒有出口——資料裡沒有這種格，
// 但 remake 讀存檔時不保證。
func Load(data []byte, neighbours []int) (*Field, error) {
	if len(data) != FieldBytes {
		return nil, fmt.Errorf("battle: 戰場資料是 %d 個位元組，應該是 %d",
			len(data), FieldBytes)
	}
	f := &Field{
		W: FieldW, H: FieldH,
		cell:       make([]Terrain, FieldBytes),
		off:        make([]bool, FieldBytes),
		Gates:      map[int][]Hex{},
		Neighbours: append([]int(nil), neighbours...),
	}
	f.CityAt = NoHex
	starts := make([]Hex, ArmySlots)
	for i := range starts {
		starts[i] = NoHex
	}
	for i, v := range data {
		h := FromOffset(i%FieldW, i/FieldW)
		if v == cellNone {
			// 圖外：當成大山，走位判斷不必另外分一條路；
			// off 另外記著，寫回去的時候才分得出來。
			f.cell[i], f.off[i] = Mountain, true
			continue
		}
		f.cell[i] = terrainOf[v&cellTerrain]
		switch mark := v >> 4; {
		case mark == markNone:
		case mark == markCity:
			f.CityAt = h
		case mark >= markArmy0:
			starts[mark-markArmy0] = h
		case int(mark) < len(neighbours):
			n := neighbours[mark]
			f.Gates[n] = append(f.Gates[n], h)
		}
	}
	f.Starts = starts
	return f, nil
}

// Bytes 把戰場寫回原版的 120 個位元組。存檔要靠它。
func (f *Field) Bytes() []byte {
	out := make([]byte, FieldBytes)
	for i := range out {
		out[i] = cellNone
	}
	mark := make([]byte, FieldBytes)
	for i := range mark {
		mark[i] = markNone
	}
	put := func(h Hex, m byte) {
		x, y := f.offset(h)
		if x < 0 || y < 0 || x >= f.W || y >= f.H {
			return
		}
		mark[y*f.W+x] = m
	}
	put(f.CityAt, markCity)
	for i, h := range f.Starts {
		put(h, byte(markArmy0+i))
	}
	// 出口寫回的是**鄰郡在相鄰表裡的位置**，所以要有那份清單才寫得回去；
	// Gates 只記郡編號，這裡照 Neighbours 的順序還原。
	for i, n := range f.Neighbours {
		for _, h := range f.Gates[n] {
			put(h, byte(i))
		}
	}
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			i := y*f.W + x
			if f.off != nil && f.off[i] {
				continue // 圖外，留 0xFF
			}
			out[i] = mark[i]<<4 | terrainCode[f.cell[i]]
		}
	}
	return out
}
