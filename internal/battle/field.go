// Package battle 是戰役的戰術層：主戰場與對戰（說明書 p.26–35）。
//
// **地形版面是 remake 自己生成的。** 原版的郡地理誌是美術素材
// （`DATA2`／`DATA3` 的 `.OKR`，465 項，壓縮過、格式未解），
// 與主畫面的地圖一樣不重製也不散布。生成器是決定性的：
// 同一個郡永遠得到同一張圖，所以對拍與重現都成立。
//
// 除了版面之外，**這一層的機制全部照手冊**：六方向、地形效應、
// 移動力、五種戰鬥隊伍、四種軍力、弓箭次數公式、六種計謀的
// 智力門檻與費用、天氣限制、三十天的勝負判定。
// 沒有出處的係數走 `Tune*`（`docs/design/02`）。
package battle

import "fmt"

// Terrain 是一格的地形。名稱與手冊 p.31–33 的表相同。
type Terrain uint8

const (
	Plain    Terrain = iota // 平原：無特殊效果
	Desert                  // 沙漠：無特殊效果
	Hill                    // 山丘：強化攻擊力和防禦力
	Forest                  // 樹林：相當不錯的防禦掩護
	Shallow                 // 淺水：不利攻擊與防禦
	Deep                    // 深水：也不利攻擊與防禦
	City                    // 城池：發揮部隊最大戰力，一流防禦工事
	Fort                    // 關寨：少許攻擊優勢，簡陋的防禦工事
	Mountain                // 大山：無法穿越
	terrainCount
)

// String 讓地形印得出中文。
func (t Terrain) String() string {
	switch t {
	case Plain:
		return "平原"
	case Desert:
		return "沙漠"
	case Hill:
		return "山丘"
	case Forest:
		return "樹林"
	case Shallow:
		return "淺水"
	case Deep:
		return "深水"
	case City:
		return "城池"
	case Fort:
		return "關寨"
	case Mountain:
		return "大山"
	}
	return "?"
}

// Passable 回報部隊過不過得去。
//
// 「大山無法穿越」「城寨的牆無法穿越」「水坑無法穿越」（說明書 p.29–30）
// ——這裡把大山當成絕對不可通行，深水只有水軍過得去（見 MoveCost）。
func (t Terrain) Passable() bool { return t != Mountain }

// Water 回報是不是水域。火攻與水淹的殺傷、以及水軍的適性都看它。
func (t Terrain) Water() bool { return t == Shallow || t == Deep }

// 攻防修正（百分比）。出處是手冊 p.31–32 的兩張地形效應表：
// 山丘強化攻防、樹林提供不錯的防禦掩護、水域對攻防都不便、
// 城池發揮最大戰力與一流防禦、關寨少許攻擊優勢與簡陋防禦、
// 平原沙漠無特殊效果。**方向是手冊的，幅度是 remake 選的。**
var attackMod = [terrainCount]int{
	Plain: 0, Desert: 0, Hill: +20, Forest: 0,
	Shallow: -20, Deep: -30, City: +30, Fort: +10, Mountain: 0,
}

var defenceMod = [terrainCount]int{
	Plain: 0, Desert: 0, Hill: +20, Forest: +25,
	Shallow: -20, Deep: -30, City: +50, Fort: +20, Mountain: 0,
}

// AttackMod／DefenceMod 是地形對攻擊力與防禦力的修正百分比。
func AttackMod(t Terrain) int  { return attackMod[t] }
func DefenceMod(t Terrain) int { return defenceMod[t] }

// moveCost 是走進一格要花的移動力。大山是不可通行（見 Passable）。
var moveCost = [terrainCount]int{
	Plain: 1, Desert: 2, Hill: 3, Forest: 2,
	Shallow: 3, Deep: 5, City: 1, Fort: 1, Mountain: 99,
}

// Hex 是軸座標。六方向與手冊 p.5 的方向圖相同：
// 上、下是垂直，其餘四個是斜向。
type Hex struct{ Q, R int }

// Dir 是六個方向，編號與手冊的按鍵相同。
type Dir int

const (
	DirDownLeft  Dir = 1 // 1 左下
	DirDown      Dir = 2 // 2 下
	DirDownRight Dir = 3 // 3 右下
	DirUpLeft    Dir = 4 // 4 左上
	DirUp        Dir = 5 // 5 上
	DirUpRight   Dir = 6 // 6 右上
)

var dirDelta = map[Dir]Hex{
	DirUp:        {0, -1},
	DirDown:      {0, +1},
	DirUpRight:   {+1, -1},
	DirDownRight: {+1, 0},
	DirUpLeft:    {-1, 0},
	DirDownLeft:  {-1, +1},
}

// Dirs 是六個方向，順序固定。
func Dirs() []Dir {
	return []Dir{DirDownLeft, DirDown, DirDownRight, DirUpLeft, DirUp, DirUpRight}
}

// Step 回傳往某個方向走一格的座標。
func (h Hex) Step(d Dir) Hex {
	v, ok := dirDelta[d]
	if !ok {
		return h
	}
	return Hex{h.Q + v.Q, h.R + v.R}
}

// Distance 是兩格之間的六方向距離。
func Distance(a, b Hex) int {
	dq, dr := a.Q-b.Q, a.R-b.R
	ds := -dq - dr
	m := abs(dq)
	if abs(dr) > m {
		m = abs(dr)
	}
	if abs(ds) > m {
		m = abs(ds)
	}
	return m
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Field 是一個主戰場。
type Field struct {
	W, H int
	cell []Terrain

	// Gates 是通往鄰郡的通道位置。
	//
	// 手冊 p.19：「圖中的數字位置代表前往鄰近州郡的通道，
	// **也是鄰郡攻入時的發兵地點**」——所以它同時是進攻方的入口
	// 與退兵的出口。索引是鄰郡的郡編號。
	Gates map[int]Hex

	// CityAt 是城池的位置。守方守的就是它（說明書 p.35）。
	CityAt Hex
}

// At 取一格的地形。越界回大山（不可通行），這樣邊界不必另外判斷。
func (f *Field) At(h Hex) Terrain {
	x, y := f.offset(h)
	if x < 0 || y < 0 || x >= f.W || y >= f.H {
		return Mountain
	}
	return f.cell[y*f.W+x]
}

// Set 設一格的地形。越界忽略。
func (f *Field) Set(h Hex, t Terrain) {
	x, y := f.offset(h)
	if x < 0 || y < 0 || x >= f.W || y >= f.H {
		return
	}
	f.cell[y*f.W+x] = t
}

// offset 把軸座標換成矩形陣列的索引（奇數列右移半格）。
func (f *Field) offset(h Hex) (int, int) {
	y := h.R
	x := h.Q + (h.R-(h.R&1))/2
	return x, y
}

// FromOffset 把矩形座標換回軸座標。畫面與生成器用它。
func FromOffset(x, y int) Hex { return Hex{Q: x - (y-(y&1))/2, R: y} }

// InBounds 回報這一格在不在場上。
func (f *Field) InBounds(h Hex) bool {
	x, y := f.offset(h)
	return x >= 0 && y >= 0 && x < f.W && y < f.H
}

// Cells 依序走訪每一格。
func (f *Field) Cells(fn func(h Hex, t Terrain)) {
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			fn(FromOffset(x, y), f.cell[y*f.W+x])
		}
	}
}

// String 把戰場印成文字，測試與除錯用。
func (f *Field) String() string {
	glyph := [terrainCount]rune{
		Plain: '.', Desert: ',', Hill: '^', Forest: '#',
		Shallow: '~', Deep: '≈', City: '田', Fort: '凸', Mountain: '▲',
	}
	s := ""
	for y := 0; y < f.H; y++ {
		if y&1 == 1 {
			s += " "
		}
		for x := 0; x < f.W; x++ {
			s += string(glyph[f.cell[y*f.W+x]])
		}
		s += "\n"
	}
	return s
}

// MoveCost 是走進某一格要花的移動力。
//
// 兵種對地形的適性會改變花費——「移動力來源是訓練度和兵種能否適應地形」
// （說明書 p.30）。水軍在水上、山軍在山丘、陸軍在平地各自省力。
func MoveCost(t Terrain, troop TroopKind) int {
	c := moveCost[t]
	if c >= 99 {
		return c
	}
	if troop.Suits(t) && c > 1 {
		c--
	}
	return c
}

// TroopKind 是兵種。編號與原版的字串表相同（`docs/spec/003` §2.1）。
type TroopKind uint8

const (
	TroopLand      TroopKind = 0 // 陸
	TroopMountain  TroopKind = 1 // 山
	TroopWaterOnly TroopKind = 2 // 水
	TroopMtnLand   TroopKind = 3 // 山陸
	TroopWaterLand TroopKind = 4 // 水陸
	TroopMtnWater  TroopKind = 5 // 山水
	TroopMighty    TroopKind = 6 // 強力：萬能兵種
)

// Suits 回報這個兵種適不適應某種地形（說明書 p.18）。
//
// 「強力軍是萬能兵種，山、陸、水各代表在山丘、平地或水域有優良戰力，
// 而且也更能發揮地形特性。」
func (k TroopKind) Suits(t Terrain) bool {
	if k == TroopMighty {
		return true
	}
	mtn := t == Hill || t == Mountain
	water := t.Water()
	land := !mtn && !water
	switch k {
	case TroopLand:
		return land
	case TroopMountain:
		return mtn
	case TroopWaterOnly:
		return water
	case TroopMtnLand:
		return mtn || land
	case TroopWaterLand:
		return water || land
	case TroopMtnWater:
		return mtn || water
	}
	return false
}

// String 讓兵種印得出中文。
func (k TroopKind) String() string {
	names := []string{"陸", "山", "水", "山陸", "水陸", "山水", "強力"}
	if int(k) < len(names) {
		return names[k]
	}
	return fmt.Sprintf("?%d", k)
}
