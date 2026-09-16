// Package battle 是戰役的戰術層：主戰場與對戰（說明書 p.26–35）。
//
// **地形版面來自劇本**：原版把每個郡的戰場地圖存在州郡記錄的
// 第 55–174 個位元組（`docs/spec/003`）。`generate.go` 的生成器是
// **沒有劇本資料時的退路**，決定性——同一個郡永遠得到同一張圖，
// 所以那條路徑也仍然可重現（`docs/design/03`）。
//
// 圖塊本身是美術素材，與主畫面的地圖一樣不重製也不散布。
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

// 地形的攻防值。原版的兩張表在 `DS:0x85c2`（攻方所在格）與
// `DS:0x85e2`（守方所在格），用地形碼索引（`0x2df44`／`0x2df8d`，`L0`）。
//
// 手冊 p.31–32 只給了方向（山丘強化攻防、樹林提供不錯的防禦掩護、
// 水域不便、城池最強、關寨次之）；**幅度是量到的**。
var terrainAttack = [terrainCount]int{
	Plain: 20, Desert: 15, Hill: 20, Forest: 16,
	Shallow: 8, Deep: 6, City: 27, Fort: 25, Mountain: 0,
}

var terrainDefence = [terrainCount]int{
	Plain: 17, Desert: 16, Hill: 22, Forest: 22,
	Shallow: 9, Deep: 8, City: 40, Fort: 30, Mountain: 0,
}

// TerrainAttack／TerrainDefence 是地形的攻防值（原版的絕對值）。
func TerrainAttack(t Terrain) int  { return terrainAttack[t] }
func TerrainDefence(t Terrain) int { return terrainDefence[t] }

// troopTerrain 是兵種對地形的加成。原版的表在 `DS:0x8602`，
// 七個兵種各 16 格，用地形碼索引（`0x2e049`／`0x2e169`，`L0`）。
//
// 這張表就是手冊 p.18 那句話的數字版：「山、陸、水各代表在山丘、
// 平地或水域有優良戰力」——陸軍加在平原與樹林，山軍加在山丘，
// 水軍加在淺水與深水；「強力軍是萬能兵種，而且也更能發揮地形特性」
// ——強力軍每一種地形都有，而且水上比水軍還高。
var troopTerrain = [7][terrainCount]int{
	TroopLand:      {Plain: 5, Forest: 5},
	TroopMountain:  {Hill: 10},
	TroopWaterOnly: {Shallow: 10, Deep: 10},
	TroopMtnLand:   {Hill: 10, Plain: 5, Forest: 5},
	TroopWaterLand: {Shallow: 10, Deep: 10, Plain: 5, Forest: 5},
	TroopMtnWater:  {Hill: 10, Shallow: 10, Deep: 10},
	TroopMighty: {
		Hill: 12, Shallow: 13, Deep: 15, City: 8, Fort: 8,
		Plain: 6, Forest: 6, Desert: 5,
	},
}

// TroopTerrainBonus 是兵種在某種地形上的加成。
func TroopTerrainBonus(k TroopKind, t Terrain) int {
	if int(k) >= len(troopTerrain) {
		return 0
	}
	return troopTerrain[k][t]
}

// 一位將領的戰力值：原版每一次對戰都對雙方的每一位將領各算一次
// （`0x2e01a` 守方、`0x2e13a` 攻方，`L0`）：
//
//	戰力值 ＝ (戰力 × 7 ＋ 3 × 武裝度) × (兵種適性 ＋ 地形值) ÷ 1000
//
// 地形值攻方取 terrainAttack、守方取 terrainDefence，兩邊各用
// **自己所在那一格**的地形。
const (
	LeaderWarWeight  = 7
	LeaderArmsWeight = 3
	LeaderPowerDiv   = 1000
)

// LeaderPower 是一位將領在某一格的戰力值。
func LeaderPower(war, arms int, k TroopKind, t Terrain, attacking bool) int {
	edge := terrainDefence[t]
	if attacking {
		edge = terrainAttack[t]
	}
	return (war*LeaderWarWeight + LeaderArmsWeight*arms) *
		(TroopTerrainBonus(k, t) + edge) / LeaderPowerDiv
}

// moveCost 是走進一格要花的移動力。
//
// 原版的表在 `DS:0x7c42`，16 個字，用地形碼（那一格的低四位）索引：
//
//	1 大山 999   2 山丘 3   3 淺水 4   4 深水 6   5 城池 3
//	6 關寨 3     7 平原 2   8 樹林 3   9 沙漠 2
//
// **與說明書 p.29 的表逐格相同**，兩份互為佐證（`L0`、`[base]`）。
// 999 是原版自己寫的「過不去」，這裡照抄，判斷仍走 Passable。
var moveCost = [terrainCount]int{
	Plain: 2, Desert: 2, Hill: 3, Forest: 3,
	Shallow: 4, Deep: 6, City: 3, Fort: 3, Mountain: 999,
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

	// Gates 是通往鄰郡的通道。索引是鄰郡的郡編號。
	//
	// 手冊 p.19：「圖中的數字位置代表前往鄰近州郡的通道，
	// **也是鄰郡攻入時的發兵地點**」——所以它同時是進攻方的入口
	// 與退兵的出口。
	//
	// **一個出口是一片格子不是一格**：原版的資料裡遼東通往鄰郡的
	// 通道佔五格。要單一入口用 Gate。
	Gates map[int][]Hex

	// CityAt 是城池的位置。守方守的就是它（說明書 p.35）。
	CityAt Hex

	// Starts 是四個軍團的起點（原版每張圖各標一格，高四位 11–14）。
	// 沒有標記的槽是 `{-1, -1}`。
	Starts []Hex

	// Neighbours 是這個郡的相鄰表，順序與原版相同。出口寫回原版格式時
	// 要靠它把郡編號換回索引。
	Neighbours []int

	// off 記哪些格在圖外。**不能只靠地形是大山來判斷**：圖外與大山
	// 走起來一樣，寫回原版格式時卻是兩個不同的位元組。
	off []bool
}

// NoHex 是「沒有這一格」的哨兵值。城池、軍團起點、入口都用它表示缺席。
var NoHex = Hex{Q: -1, R: -1}

// Gate 是通往某個鄰郡的入口，取那片通道的第一格。
// 沒有這個出口時回 NoHex。
func (f *Field) Gate(n int) Hex {
	if hs := f.Gates[n]; len(hs) > 0 {
		return hs[0]
	}
	return NoHex
}

// Outside 回報這一格在不在圖外（原版寫 `0xFF` 的那些格）。
func (f *Field) Outside(h Hex) bool {
	x, y := f.offset(h)
	if x < 0 || y < 0 || x >= f.W || y >= f.H {
		return true
	}
	return f.off != nil && f.off[y*f.W+x]
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

// offset 把軸座標換成矩形陣列的索引。
//
// **版面是原版的**：12 欄 × 10 列，**奇數欄往下移半格**（odd-q）。
// 畫面上一格 48 × 32 像素，`x ＝ 48欄 + 56`、`y ＝ 32列 + 36`，
// 奇數欄再 `+16`（`0x22742`–`0x2276c`）。走訪鄰格的 dx／dy 表在
// `DS:0x7c6a`／`DS:0x7c82`，依欄的奇偶各一組六向（`0x24c11`）——
// 換算過來與 dirDelta 的六個軸向差**逐格相同**。
func (f *Field) offset(h Hex) (int, int) {
	x := h.Q
	y := h.R + (h.Q-(h.Q&1))/2
	return x, y
}

// ToOffset 把軸座標換成矩形陣列的欄列。畫面用它算像素位置。
func ToOffset(h Hex) (x, y int) { return h.Q, h.R + (h.Q-(h.Q&1))/2 }

// FromOffset 把矩形座標換回軸座標。畫面與載入器用它。
func FromOffset(x, y int) Hex { return Hex{Q: x, R: y - (x-(x&1))/2} }

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
// **只看地形，不看兵種。** 原版取完 `DS:0x7c42` 那一格的值就直接跟
// 部隊剩下的移動力比（`0x27e71`，部隊記錄 offset 36），中間沒有任何
// 按兵種的調整。說明書 p.30 的「移動力來源是訓練度和兵種能否適應地形」
// 講的是**移動力本身**（`Unit.MovePoints`，看訓練度與武裝度）與
// 兵種在戰力上的加成（`TroopTerrainBonus`），不是每一格的花費。
//
// troop 留著是為了呼叫端不必知道這件事，也留一個位置給加強版——
// 兩版的表還沒比過。
func MoveCost(t Terrain, troop TroopKind) int { return moveCost[t] }

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

// Narrow 回報這張圖是不是 8 欄的窄圖：窄圖的 (8,0) 那一格在圖外
// （劇本 001 的 42 個郡只有 12×7 與 8×10 兩種形狀，§2.1）。
// 主戰場的版面與對戰子畫面的版型都照它挑。
func (f *Field) Narrow() bool { return f.Outside(FromOffset(8, 0)) }
