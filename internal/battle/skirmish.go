package battle

import (
	"fmt"
	"math/big"
)

// 對戰子畫面（原版 `0x2deb0`，`docs/re/05` §10，`L0`＋`L1`、`[base]`；
// 對拍 `TestZZUnitAIDayParity` 盤面丁）。
//
// 主戰場上一支部隊對另一支部隊「對戰」（玩家命令 2、電腦選項 7），
// 畫面切到一張 12 欄 × 10 列的子地圖：兩支部隊的每一位將領各佔一格，
// 從第 6 時到第 18 時一位一位下令——行軍、單挑、攻擊、休息。
// 子畫面自己有一套亂數的擲法與順序，這裡逐條照原版。
//
// 陣營只有兩格：0 是守方（被對戰的那一支）、1 是攻方（發動的那一支）
// （`es:0x3ef0`／`0x3ef2`）。各種陣列用 `陣營 × 10 ＋ 將領槽` 索引。

// 子地圖的大小與時刻。
const (
	SkirmishCols = 12
	SkirmishRows = 10
	// SkirmishFirstHour／SkirmishLastHour：從第 6 時起，過了第 18 時結束
	//（`0x2e67a`、`0x2e6f4`）。
	SkirmishFirstHour = 6
	SkirmishLastHour  = 18
	// skirmishSlots 是一支部隊的將領槽數。
	skirmishSlots = 10
	// skirmishLeftCap：每一步開始時剩餘行動力夾到 15（`0x2e9b1`）。
	skirmishLeftCap = 15
	// skirmishRestLeft：休息一次加 2 步（`0x2eb48`／`0x2ec70`／`0x2fd43`）。
	skirmishRestLeft = 2
	// skirmishTired：體能不到 10 的將領自動休息（`0x2ea5b`）。
	skirmishTired = 10
	// skirmishStepStamina／skirmishAttackStamina／skirmishStruckStamina：
	// 走一格扣 1 體能，攻擊扣 5、被攻擊扣 3（`0x323f4` 的三個呼叫端）。
	skirmishStepStamina   = 1
	skirmishAttackStamina = 5
	skirmishStruckStamina = 3
	// SkirmishSoldierCap：攻擊之後兵扣完小於 0 **或大於 5000** 都當成 0
	//（`0x30628`／`0x306c5`）——一位將領最多五千兵是這一層的前提。
	SkirmishSoldierCap = 5000
	// skirmishActRange：電腦每一步先擲 `RND(30)`，難度 × 4 ＋ 10 不小於它
	// 才行動，否則休息（`0x2ebe5`–`0x2ebff`）。加強版擲 `RND(40)`，門檻是
	// ((難度 − 1) mod 10) × 4（`0x2bade`–`0x2bb00`，`[plus]`）。
	skirmishActRange     = 30
	skirmishActBase      = 10
	plusSkirmishActRange = 40
	// skirmishFleeSpread／skirmishFleeTurn：兵不到最強鄰敵的 1/(RND(3)+1)
	// 就想逃，逃的方向是 (RND(3) ＋ 最強鄰敵的方向 ＋ 2) mod 6。
	skirmishFleeSpread = 3
	// skirmishEngageGate：敵帥不在旁邊時 `RND(16)==0` 才就地交手，
	// 否則往敵帥靠（`0x2f086`）。加強版比的是 `== 15`（`0x2bf55`，`[plus]`）
	// ——機率一樣，值不一樣，接原版的骰序時分得出來。
	skirmishEngageGate    = 16
	plusSkirmishEngageHit = 15
	// skirmishDuelSpread：`我方戰力 > 對方戰力 ＋ RND(20)` 就單挑（`0x2f1b7`）。
	// 加強版是 `RND(5)`，過了再擲 `RND(3)`，擲到 1 就不單挑
	//（`0x2c068`–`0x2c09f`，`[plus]`）。
	skirmishDuelSpread     = 20
	plusSkirmishDuelSpread = 5
	plusSkirmishDuelSkip   = 3
	// skirmishDesperateSpread：兵不到對方三分之一時
	// `我方戰力 ＋ RND(10) − 5 > 對方戰力` 也單挑（`0x2f265`）。
	skirmishDesperateSpread = 10
	skirmishDesperateBias   = 5
	// skirmishBlockedSpread 是走一步時對鄰格佔位者的那一擲 `RND(20)`
	//（`0x2f42b`）。**原版查的是目的格自己的佔位而不是鄰格**
	//（`0x2f408` 用的是目的格的欄列），目的格一定是空的，所以這一擲
	// 從來不會發生；留著是為了與碼對得上。
	skirmishBlockedSpread = 20
)

// 子地圖的地形碼。0–9 與主戰場相同（`terrainOf`），10–14 只出現在
// 城池與關寨的版型裡（城內、牆）。移動力消耗查 `DS:0x7c42` 的
// 16 格版（`skirmishMoveCost`），尋路把 1、13、14 與圖外當成走不過
//（`0x2f665`–`0x2f6ad`）。
const (
	skirmishCellNone = 0xff
	skirmishBlocked  = 999
	// plusModeSpan10：加強版的行動門檻用 (難度 − 1) mod 10（`0x2baf2`）。
	plusModeSpan10 = 10
)

// skirmishMoveCost 是 `DS:0x7c42` 的整張表（16 個字，`L0`）：
// 1–9 與 `moveCost` 逐格相同，10–12 是 2，0 與 13–15 是 999。
var skirmishMoveCost = [16]int{
	999, 999, 3, 4, 6, 3, 3, 2, 3, 2, 2, 2, 2, 999, 999, 999,
}

// skirmishDX／skirmishDY 是 `DS:0x7c6a`／`DS:0x7c82`：依欄的奇偶各六向，
// 方向 0–5 就是按鍵 1–6（`docs/re/05` §2.1）。
var (
	skirmishDX = [2][6]int{{-1, 0, 1, -1, 0, 1}, {-1, 0, 1, -1, 0, 1}}
	skirmishDY = [2][6]int{{0, 1, 0, -1, -1, -1}, {1, 1, 1, 0, -1, 0}}
)

// skirmishAttackerStart 是攻方十位將領的起點（`0x2e228`–`0x2e30d`，`L0`）：
// 寬圖一組、窄圖一組，寫死在碼裡。
var (
	skirmishAttackerWide   = [skirmishSlots][2]int{{0, 3}, {1, 3}, {0, 4}, {1, 2}, {1, 4}, {0, 5}, {0, 2}, {1, 1}, {0, 1}, {2, 3}}
	skirmishAttackerNarrow = [skirmishSlots][2]int{{3, 9}, {2, 9}, {4, 9}, {3, 8}, {1, 9}, {5, 9}, {1, 8}, {5, 8}, {2, 8}, {4, 8}}
	// skirmishEmptySlotAt 是空槽的佔位座標（`0x2e419`）——圖外的那一格。
	skirmishEmptySlotAt = [2]int{11, 9}
)

// SkirmishSide 是子畫面裡的陣營：0 守方、1 攻方。
type SkirmishSide int

const (
	SkirmishDefender SkirmishSide = 0
	SkirmishAttacker SkirmishSide = 1
)

func (s SkirmishSide) String() string {
	if s == SkirmishAttacker {
		return "攻方"
	}
	return "守方"
}

// SkirmishGeneral 是子畫面裡的一位將領。
type SkirmishGeneral struct {
	Leader *Leader
	Unit   *Unit
	Side   SkirmishSide
	Slot   int
	// Col／Row 是子地圖上的欄列（`es:0x9a6`／`0x15d8`）。
	Col, Row int
	// Power 是戰力值（`es:0x548`，`LeaderPower`，用**主戰場上部隊所在格**
	// 的地形算，進來時算一次）。
	Power int
	// MoveCap 是移動力（`es:0x1706`）、Left 是剩餘行動力（`es:0x3c9c`）。
	MoveCap, Left int
	// StaminaCap 是進來時的體能（`es:0x2f80`／`0x2f94`）：休息回不過它，
	// 結束時整個寫回去——子畫面裡的消耗只在子畫面裡算數。
	StaminaCap int
	// Gone：這一位在子畫面裡被抓走了（槽清成 FFFF）。
	Gone bool
}

func (g *SkirmishGeneral) index() int { return int(g.Side)*skirmishSlots + g.Slot }

// SkirmishCommand 是玩家那一方一位將領的一道命令（原版 `DS:0x871a` 的選單
// `1.行軍 2.單挑 3.攻擊 7.查看 0.休息`）。
type SkirmishCommand struct {
	Kind SkirmishCommandKind
	// Dir 是行軍、單挑、攻擊的方向（1–6，`Dir`）。
	Dir Dir
}

// SkirmishCommandKind 是命令的種類。
type SkirmishCommandKind int

const (
	// SkirmishRest 休息：加體能、加 2 步，這一位這一時刻結束。
	SkirmishRest SkirmishCommandKind = iota
	// SkirmishMarch 往 Dir 走一格。走得動就再問一次（原版的行軍模式
	// 要按 Enter 才離開）；走不動照原版留在行軍模式，也是再問一次。
	SkirmishMarch
	// SkirmishMarchDone 離開行軍模式（Enter）：這一時刻走過至少一格就
	// 結束，一格都沒走回到選單再問。
	SkirmishMarchDone
	// SkirmishDuel 向 Dir 的敵將叫陣單挑；SkirmishAttack 向 Dir 的敵將攻擊。
	// 那一格沒有敵將就回到選單再問。
	SkirmishDuel
	SkirmishAttack
)

// SkirmishPlayer 是玩家那一方的介面：每一位將領輪到時被問一次（行軍
// 模式裡走一格問一次），回一道命令。nil 表示沒有介面——那一方的將領
// 照電腦的判斷式走（registered remake 差異，`docs/mechanics/40` §8）。
type SkirmishPlayer func(s *Skirmish, g *SkirmishGeneral) SkirmishCommand

// SkirmishAnswer 是玩家那一方的將領 t 被 g 叫陣時接不接受。
type SkirmishAnswer func(s *Skirmish, g, t *SkirmishGeneral) bool

// Skirmish 是一場對戰。
type Skirmish struct {
	b *Battle
	// Units[0] 是守方、[1] 是攻方。
	Units [2]*Unit
	// Layout 是用的版型（0–15）；Narrow 表示主戰場是 8 欄的窄圖。
	Layout int
	Narrow bool
	// Map 是子地圖：每一格是版型的位元組（低四位地形碼），圖外 0xff。
	Map [SkirmishRows][SkirmishCols]int
	// Occ 是佔位圖：−1 空，否則 `陣營 × 10 ＋ 槽`。
	Occ [SkirmishRows][SkirmishCols]int
	// Gens 是雙方的將領，nil 是空槽。
	Gens [2][skirmishSlots]*SkirmishGeneral
	// Hour 是現在的時刻。
	Hour int
	// Captured 是各方抓到的人，結束時照原版的順序處置。
	Captured [2][skirmishSlots]*SkirmishGeneral
	// Player 是玩家那一方的介面（見 SkirmishPlayer）。
	Player SkirmishPlayer
	// Answer 是玩家那一方的將領被叫陣時的答案（原版問「接受嗎(Y/N)」，
	// `0x30d06`）；nil 就當接受。
	Answer SkirmishAnswer

	// dist／goal 是尋路的距離圖與路徑圖（`es:0x2e7c`／`0x300c`）。
	dist, goal [SkirmishRows][SkirmishCols]int
	// acted 對應 `es:0x31c0 == 0`：這一步已經做了事（休息、走了一格、
	// 攻擊、單挑）。
	acted bool
	// Trace 是每一步的紀錄，給對拍與單測看。
	Trace []string
}

// NewSkirmish 照原版的前置（`0x2deb0`–`0x2e493`）擺好一場對戰：
// att 是發動的那一支、def 是被對戰的那一支。
func (b *Battle) NewSkirmish(att, def *Unit) *Skirmish {
	s := &Skirmish{b: b, Units: [2]*Unit{def, att}, Hour: SkirmishFirstHour,
		Player: b.PlayerSkirmish, Answer: b.PlayerDuelAnswer}
	// 版型：守方部隊所在格的地形碼，窄圖再加一。
	s.Narrow = b.Field.Outside(FromOffset(8, 0))
	s.Layout = (int(terrainCode[b.Field.At(def.At)]) - 2) * 2
	if s.Narrow {
		s.Layout++
	}
	if s.Layout < 0 || s.Layout >= len(skirmishTemplates) {
		// 大山上沒有部隊，走不到這裡；真走到了原版會讀到版型表以外的
		// 記憶體，remake 夾回第一張。
		s.Layout = 0
	}
	for side, u := range s.Units {
		for slot := range u.Leaders {
			if slot >= skirmishSlots {
				break
			}
			x := &u.Leaders[slot]
			if !x.InUnit() {
				continue
			}
			g := &SkirmishGeneral{Leader: x, Unit: u, Side: SkirmishSide(side), Slot: slot}
			g.Power = LeaderPower(int(x.War), int(x.Arms), x.Troop, b.Field.At(u.At), side == int(SkirmishAttacker))
			g.MoveCap = SkirmishMove(int(x.Training), int(x.Arms))
			g.Left = g.MoveCap
			g.StaminaCap = int(x.Stamina)
			s.Gens[side][slot] = g
		}
	}
	// 子地圖：版型的每一格照抄，高四位小於 10 的是守方那一槽的起點。
	for r := 0; r < SkirmishRows; r++ {
		for c := 0; c < SkirmishCols; c++ {
			v := templateByte(s.Layout, r, c)
			s.Map[r][c] = v
			s.Occ[r][c] = -1
			if mark := v >> 4; mark < skirmishSlots {
				if g := s.Gens[SkirmishDefender][mark]; g != nil {
					g.Col, g.Row = c, r
				}
			}
		}
	}
	// 攻方的起點寫死在碼裡。
	start := skirmishAttackerWide
	if s.Narrow {
		start = skirmishAttackerNarrow
	}
	for slot, g := range s.Gens[SkirmishAttacker] {
		if g != nil {
			g.Col, g.Row = start[slot][0], start[slot][1]
		}
	}
	// 佔位：兩方十槽都寫，空槽寫在圖外那一格（原版也是，那一格用不到）。
	for side := range s.Gens {
		for slot, g := range s.Gens[side] {
			c, r := skirmishEmptySlotAt[0], skirmishEmptySlotAt[1]
			if g != nil {
				c, r = g.Col, g.Row
			}
			s.Occ[r][c] = side*skirmishSlots + slot
		}
	}
	for side := range s.Captured {
		for slot := range s.Captured[side] {
			s.Captured[side][slot] = nil
		}
	}
	return s
}

// SkirmishMove 是一位將領在子畫面裡的移動力（`0x2e07a`，`docs/re/05` §3.2）：
//
//	min(15, (訓練度 − 武裝度 + 100) ÷ 10 + 1)
func SkirmishMove(training, arms int) int {
	v := (training-arms+100)/10 + 1
	if v > skirmishLeftCap {
		v = skirmishLeftCap
	}
	return v
}

// templateByte 讀版型的一格。
func templateByte(layout, row, col int) int {
	s := skirmishTemplates[layout][row]
	hi, lo := hexNibble(s[col*2]), hexNibble(s[col*2+1])
	return hi<<4 | lo
}

func hexNibble(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	}
	return 0
}

// terrainAt 是一格的地形碼（低四位）；圖外回 0xf。
func (s *Skirmish) terrainAt(c, r int) int { return s.Map[r][c] & 0xf }

// inBounds 是原版每一處鄰格判斷的範圍檢查（0 ≤ 欄 < 12、0 ≤ 列 < 10）
// ——圖外的格（0xff）在範圍內，各處另外判。
func inBounds(c, r int) bool { return c >= 0 && c < SkirmishCols && r >= 0 && r < SkirmishRows }

// neighbour 是 (c, r) 往方向 d（0–5）的鄰格。
func neighbour(c, r, d int) (int, int) {
	p := c & 1
	return c + skirmishDX[p][d], r + skirmishDY[p][d]
}

// occupant 是某一格站著誰；空的回 nil。
func (s *Skirmish) occupant(c, r int) *SkirmishGeneral {
	if !inBounds(c, r) {
		return nil
	}
	i := s.Occ[r][c]
	if i < 0 {
		return nil
	}
	return s.Gens[i/skirmishSlots][i%skirmishSlots]
}

// over 對應 `0x2e68a`：任一方第 0 槽的將領不在了、或過了第 18 時，
// 對戰就結束。
func (s *Skirmish) over() bool {
	for side := range s.Gens {
		if g := s.Gens[side][0]; g == nil || g.Gone {
			return true
		}
	}
	return s.Hour > SkirmishLastHour
}

func (s *Skirmish) log(format string, a ...any) {
	s.Trace = append(s.Trace, fmt.Sprintf(format, a...))
}

// Run 把一場對戰從頭走到尾（`0x2e498`–`0x2e688`）。
func (s *Skirmish) Run() {
	b := s.b
	b.note("blog.skirmish", s.Units[SkirmishAttacker].Name(), s.Units[SkirmishDefender].Name())
	for {
		s.log("── 時刻 %d", s.Hour)
		for side := range s.Gens {
			// 原版每一時刻第 0 槽先動、第 9 槽最後（`0x2e4a0`–`0x2e4f6`）；
			// 加強版反過來，**第 9 槽先動、帥隊最後**（`0x2b418`–`0x2b47a`
			// 的迴圈從 9 數到 0，`[plus]`）。
			for k := 0; k < skirmishSlots; k++ {
				slot := k
				if b.AI == AIPlus {
					slot = skirmishSlots - 1 - k
				}
				g := s.Gens[side][slot]
				if s.over() || g == nil || g.Gone {
					continue
				}
				s.step(g)
			}
		}
		s.Hour++
		if s.over() {
			break
		}
	}
	s.finish()
}

// step 是一位將領的一步（`0x2e952`）。
func (s *Skirmish) step(g *SkirmishGeneral) {
	if g.Left > skirmishLeftCap {
		g.Left = skirmishLeftCap
	}
	s.log("步 %d/%d 兵%d 體%d 步%d/%d @(%d,%d)", g.Side, g.Slot, g.Leader.Soldiers, g.Leader.Stamina, g.Left, g.MoveCap, g.Col, g.Row)
	switch {
	case int(g.Leader.Stamina) < skirmishTired:
		s.log("  體能不足，自動休息")
		s.rest(g)
	case s.b.Computer[g.Unit.Side] || s.Player == nil:
		s.auto(g)
	default:
		s.player(g)
	}
	if g.Left < g.MoveCap {
		g.Left = g.MoveCap
	}
}

// rest 是休息（`0x2ec01`／`0x2eaa2`／`0x2fcf0` 三處同一條）：
// 體能加 戰力 ÷ 20 ＋ 1、不超過進來時的值；剩餘行動力加 2。
func (s *Skirmish) rest(g *SkirmishGeneral) {
	x := g.Leader
	v := int(x.Stamina) + int(int8(x.War))/20 + 1
	if v > g.StaminaCap {
		v = g.StaminaCap
	}
	x.Stamina = uint8(v)
	g.Left += skirmishRestLeft
	s.acted = true
}

// spend 扣體能，下限 0（`0x323f4`）。
func spend(x *Leader, n int) {
	v := int(x.Stamina) - n
	if v < 0 {
		v = 0
	}
	x.Stamina = uint8(v)
}

// auto 是電腦那一方一位將領的判斷式（`0x2eb8a`）。
func (s *Skirmish) auto(g *SkirmishGeneral) {
	b := s.b
	s.acted = false
	if b.AI == AIPlus {
		if r := b.roll(plusSkirmishActRange); ((b.Difficulty-1)%plusModeSpan10)*4 < r {
			s.log("  RND(40)=%d 沒過 → 休息", r)
			s.rest(g)
			return
		}
	} else if r := b.roll(skirmishActRange); b.Difficulty*4+skirmishActBase < r {
		s.log("  RND(30)=%d 沒過 → 休息", r)
		s.rest(g)
		return
	}
	leader := s.Gens[g.Side][0]
	if g.Slot != 0 {
		// 帥隊旁邊沒有敵人就往帥隊靠；靠得動這一步就結束。
		near := false
		for d := 0; d < 6; d++ {
			if o := s.occupant(neighbour(leader.Col, leader.Row, d)); o != nil && o.Side != g.Side {
				near = true
			}
		}
		if !near {
			s.log("  帥隊旁邊沒有敵人 → 往帥隊靠")
			s.route(g.Col, g.Row, leader.Col, leader.Row, g.Side)
			for s.stepAlong(g) {
			}
			if s.acted {
				return
			}
		}
	}
	// 找相鄰的敵人（`0x2edb8`）：目標是**方向順序最後**那一位，
	// 最強的另外記著；敵帥相鄰另外記著。
	best, bestDir := -1, 0
	leaderAdj, hasAdj := false, false
	var target *SkirmishGeneral
	for d := 0; d < 6; d++ {
		o := s.occupant(neighbour(g.Col, g.Row, d))
		if o == nil || o.Side == g.Side {
			continue
		}
		hasAdj = true
		target = o
		if o.Slot == 0 {
			leaderAdj = true
		}
		if v := s16(o.Leader.Soldiers); v > best {
			best, bestDir = v, d
		}
	}
	r := b.roll(skirmishFleeSpread)
	if s16(g.Leader.Soldiers) < cQuo(best, r+1) {
		// 想逃（`0x2ef4a`）：往「最強鄰敵的反方向附近」找一格空的走。
		d := (b.roll(skirmishFleeSpread) + bestDir + 2) % 6
		s.clearGoal()
		if c, r := neighbour(g.Col, g.Row, d); inBounds(c, r) && s.Occ[r][c] < 0 {
			s.goal[r][c] = 0
		}
		s.log("  兵少於最強鄰敵 %d ÷ %d → 想逃（方向 %d）", best, r+1, d)
		if s.stepAlong(g) {
			return
		}
	}
	// 攻擊分支（`0x2f067`）。
	if leaderAdj {
		target = s.Gens[1-g.Side][0]
	}
	if !(hasAdj && target != nil && target.Slot == 0) {
		hit := 0
		if b.AI == AIPlus {
			hit = plusSkirmishEngageHit
		}
		if b.roll(skirmishEngageGate) != hit {
			s.log("  RND(16)!=%d → 往敵帥靠", hit)
			enemy := s.Gens[1-g.Side][0]
			s.route(g.Col, g.Row, enemy.Col, enemy.Row, g.Side)
			for s.stepAlong(g) {
			}
			if s.acted {
				return
			}
		}
	}
	s.engage(g, target)
}

// cQuo 是 C 的整數除法（往零截尾）。
func cQuo(a, b int) int {
	q := a / b
	return q
}

// engage 是交手（`0x2f122`）：沒有目標就休息；戰力壓得過就單挑；
// 兵不少於對方就攻擊；兵不到對方三分之一再賭一次單挑；否則休息。
func (s *Skirmish) engage(g, t *SkirmishGeneral) {
	b := s.b
	if t == nil {
		s.log("  沒有目標 → 休息")
		s.rest(g)
		return
	}
	my, their := int(int8(g.Leader.War)), int(int8(t.Leader.War))
	if b.AI == AIPlus {
		if r := b.roll(plusSkirmishDuelSpread); my > their+r && b.roll(plusSkirmishDuelSkip) != 1 {
			s.log("  戰力 %d > %d + %d → 單挑", my, their, r)
			s.duel(g, t)
			s.acted = true
			return
		}
	} else if r := b.roll(skirmishDuelSpread); my > their+r {
		s.log("  戰力 %d > %d + %d → 單挑", my, their, r)
		s.duel(g, t)
		s.acted = true
		return
	}
	ms, ts := s16(g.Leader.Soldiers), s16(t.Leader.Soldiers)
	if ms >= ts {
		s.log("  兵 %d ≥ %d → 攻擊", ms, ts)
		s.attack(g, t)
		s.acted = true
		return
	}
	if cQuo(ts, 3) > ms {
		if r := b.roll(skirmishDesperateSpread); my+r-skirmishDesperateBias > their {
			s.log("  兵不到三分之一，戰力 %d + %d − 5 > %d → 單挑", my, r, their)
			s.duel(g, t)
			s.acted = true
			return
		}
	}
	s.log("  → 休息")
	s.rest(g)
}

// player 是玩家那一方一位將領的一步（`0x2fb14` 的選單迴圈）。
func (s *Skirmish) player(g *SkirmishGeneral) {
	marched := false
	for {
		cmd := s.Player(s, g)
		switch cmd.Kind {
		case SkirmishRest:
			s.rest(g)
			return
		case SkirmishMarch:
			// 行軍（`0x2fe70`）：那一格要在圖內、空著、走得起。
			// **不擲骰**——鄰格佔位者那一擲是電腦走法才有的。
			if s.marchTo(g, dirIndex(cmd.Dir)) {
				marched = true
			}
			if g.Left == 0 {
				return
			}
			continue
		case SkirmishMarchDone:
			if marched {
				return
			}
			continue
		case SkirmishDuel, SkirmishAttack:
			t := s.occupant(neighbour(g.Col, g.Row, dirIndex(cmd.Dir)))
			if t == nil || t.Side == g.Side {
				continue
			}
			if cmd.Kind == SkirmishDuel {
				s.duel(g, t)
			} else {
				s.attack(g, t)
			}
			// 玩家的攻擊與單挑把剩餘行動力歸零（`0x301cd`／`0x30359`）。
			g.Left = 0
			return
		}
		if g.Left == 0 {
			return
		}
	}
}

// dirIndex 把 Dir（按鍵 1–6）換成方向表的索引 0–5。
func dirIndex(d Dir) int {
	i := int(d) - 1
	if i < 0 || i > 5 {
		return 0
	}
	return i
}

// marchTo 是玩家往某方向走一格（`0x2fe70`）：圖外、有人、步數不夠都走不了。
func (s *Skirmish) marchTo(g *SkirmishGeneral, d int) bool {
	c, r := neighbour(g.Col, g.Row, d)
	if !inBounds(c, r) || s.Map[r][c] == skirmishCellNone || s.Occ[r][c] >= 0 {
		return false
	}
	cost := skirmishMoveCost[s.terrainAt(c, r)]
	if cost > g.Left {
		return false
	}
	s.moveTo(g, c, r, cost)
	return true
}

// moveTo 把一位將領搬到一格：扣 1 體能、扣行動力、改佔位。
func (s *Skirmish) moveTo(g *SkirmishGeneral, c, r, cost int) {
	spend(g.Leader, skirmishStepStamina)
	s.Occ[g.Row][g.Col] = -1
	g.Left -= cost
	g.Col, g.Row = c, r
	s.Occ[r][c] = g.index()
	s.acted = true
	s.log("    走到 (%d,%d) 花 %d", c, r, cost)
}

// clearGoal 把路徑圖全清成 −1。
func (s *Skirmish) clearGoal() {
	for r := range s.goal {
		for c := range s.goal[r] {
			s.goal[r][c] = -1
		}
	}
}

// route 是尋路（`0x2f5d8`）：從 (sc, sr) 出發鋪一張距離圖，再從 (dc, dr)
// 往回找一條路寫進路徑圖。
//
// 距離圖：圖外、大山（1）、13、14、**自己這一方**站著的格走不過（敵方
// 站著的格算得過）；起點是 1，一圈一圈往外加一。路徑圖：終點是 0，
// 從終點往回每一步挑「距離剛好少一、移動力消耗最低」的鄰格（相同取
// 方向順序在前的），一路標 0 到起點。終點到不了就什麼都不標。
func (s *Skirmish) route(sc, sr, dc, dr int, side SkirmishSide) {
	s.log("  尋路 (%d,%d) → (%d,%d)", sc, sr, dc, dr)
	for r := 0; r < SkirmishRows; r++ {
		for c := 0; c < SkirmishCols; c++ {
			v := 9999
			if s.Map[r][c] == skirmishCellNone {
				v = -skirmishBlocked
			}
			switch s.terrainAt(c, r) {
			case 1, 13, 14:
				v = -skirmishBlocked
			}
			if o := s.occupant(c, r); o != nil && o.Side == side {
				v = -skirmishBlocked
			}
			s.dist[r][c] = v
		}
	}
	s.dist[sr][sc] = 1
	for k := 1; ; k++ {
		changed := false
		for r := 0; r < SkirmishRows; r++ {
			for c := 0; c < SkirmishCols; c++ {
				if s.dist[r][c] != k {
					continue
				}
				for d := 0; d < 6; d++ {
					nc, nr := neighbour(c, r, d)
					if inBounds(nc, nr) && s.dist[nr][nc] > k {
						s.dist[nr][nc] = k + 1
						changed = true
					}
				}
			}
		}
		if !changed {
			break
		}
	}
	s.clearGoal()
	d := s.dist[dr][dc]
	if d == 9999 || d == -skirmishBlocked {
		return
	}
	s.goal[dr][dc] = 0
	c, r := dc, dr
	for k := d - 1; ; k-- {
		bestCost, bestDir := 99, -1
		for dd := 0; dd < 6; dd++ {
			nc, nr := neighbour(c, r, dd)
			if !inBounds(nc, nr) || s.dist[nr][nc] != k {
				continue
			}
			if cost := skirmishMoveCost[s.terrainAt(nc, nr)]; cost < bestCost {
				bestCost, bestDir = cost, dd
			}
		}
		if bestDir < 0 {
			return
		}
		c, r = neighbour(c, r, bestDir)
		s.goal[r][c] = 0
	}
}

// stepAlong 照路徑圖走一格（`0x2f28e`）：自己站的格從路徑圖上抹掉，
// 六個鄰格裡挑**方向順序最後**一個「在路徑上而且空著」的；沒有、或走進去
// 的消耗超過剩餘行動力就回 false。
func (s *Skirmish) stepAlong(g *SkirmishGeneral) bool {
	s.goal[g.Row][g.Col] = -1
	dc, dr := -1, -1
	for d := 0; d < 6; d++ {
		c, r := neighbour(g.Col, g.Row, d)
		if inBounds(c, r) && s.goal[r][c] == 0 && s.Occ[r][c] < 0 {
			dc, dr = c, r
		}
	}
	if dc < 0 {
		s.log("    沒走（沒路）")
		return false
	}
	cost := skirmishMoveCost[s.terrainAt(dc, dr)]
	if cost > g.Left {
		s.log("    沒走（步數不夠：要 %d 剩 %d）", cost, g.Left)
		return false
	}
	// 目的格周圍要是站著比自己強的就不走（`0x2f3b3`–`0x2f4e5`）——
	// 但原版查佔位時用的是目的格自己的欄列，目的格一定是空的，
	// 這一圈從來不會擲骰、也從來擋不住人。照碼寫，不照意圖寫。
	for d := 0; d < 6; d++ {
		if c, r := neighbour(dc, dr, d); !inBounds(c, r) {
			continue
		}
		o := s.occupant(dc, dr)
		if o == nil {
			continue
		}
		if s16(o.Leader.Soldiers) > s16(g.Leader.Soldiers)+s.b.roll(skirmishBlockedSpread) {
			return false
		}
	}
	s.moveTo(g, dc, dr, cost)
	return true
}

// attack 是攻擊（`0x30364`）：雙方各以「兵 × 戰力值 ÷ 100」殺傷對方，
// **同時**結算；扣完小於 0 或大於 5000 都當成 0；兵歸零的被對方抓走
// ——攻方先看，攻方沒事守方才看（`0x30716`／`0x307e5`）。
func (s *Skirmish) attack(g, t *SkirmishGeneral) {
	b := s.b
	b.msg()
	spend(g.Leader, skirmishAttackStamina)
	spend(t.Leader, skirmishStruckStamina)
	hitBy := func(x *SkirmishGeneral) int { return SkirmishDamage(s16(x.Leader.Soldiers), x.Power) }
	da, dd := hitBy(g), hitBy(t)
	newA := s16(g.Leader.Soldiers) - dd
	if newA < 0 || newA > SkirmishSoldierCap {
		newA = 0
	}
	g.Leader.Soldiers = newA
	newD := s16(t.Leader.Soldiers) - da
	if newD < 0 || newD > SkirmishSoldierCap {
		newD = 0
	}
	t.Leader.Soldiers = newD
	s.log("  攻擊 %d/%d ⇒ %d/%d：傷亡後 攻 %d 守 %d", g.Side, g.Slot, t.Side, t.Slot, newA, newD)
	b.note("blog.clash", pn(g.Leader.Name), pn(t.Leader.Name), da, dd)
	switch {
	case newA <= 0:
		s.seize(t.Side, g)
	case newD <= 0:
		s.seize(g.Side, t)
	}
}

// SkirmishDamage 是一次攻擊的殺傷：`ftol(兵 × 戰力值 × 0.01)`（`0x30598`
// –`0x30611`）。0.01 的 double 比百分之一略大，乘積截尾與 `÷ 100` 相同；
// 照指令用 64 位元尾數算，不代數化簡。
func SkirmishDamage(soldiers, power int) int {
	f := x87(int64(soldiers))
	f.Mul(f, x87(int64(power)))
	f.Mul(f, new(big.Float).SetPrec(64).SetFloat64(qualityPercent))
	n, _ := f.Int64()
	return int(n)
}

// seize 把一位將領從子畫面上抓走：記進捕獲方的名單（`es:0x1732`），
// 槽清掉、格空出來。處置留到結束時（`finish`）。
func (s *Skirmish) seize(by SkirmishSide, x *SkirmishGeneral) {
	x.Gone = true
	x.Leader.Captured = true
	s.Captured[by][x.Slot] = x
	s.Occ[x.Row][x.Col] = -1
	s.log("  %d/%d 被抓", x.Side, x.Slot)
	s.b.note("blog.captured", pn(x.Leader.Name))
}

// duel 是子畫面裡的單挑（`0x30a1e`，`docs/re/05` §9）：對方接不接受、
// 拒絕的損兵、回合與勝負都是主戰場那一套（`duelLeaders`），落敗的
// 被擒寫進捕獲方的名單，戰死的當場處理。
func (s *Skirmish) duel(g, t *SkirmishGeneral) {
	b := s.b
	// 被挑戰的一方由玩家控制時原版問「接受嗎(Y/N)」：答案由 Answer 給，
	// 沒有就當接受。
	var answer func() bool
	if !b.Computer[t.Unit.Side] {
		answer = func() bool {
			if s.Answer != nil {
				return s.Answer(s, g, t)
			}
			return true
		}
	}
	s.log("  單挑 %d/%d ⇒ %d/%d", g.Side, g.Slot, t.Side, t.Slot)
	loser, _ := b.duelLeaders(g.Leader, t.Leader, answer, func(x *Leader) {
		// 落敗被擒：寫進捕獲方的名單。
		var lg, winner *SkirmishGeneral
		if x == g.Leader {
			lg, winner = g, t
		} else {
			lg, winner = t, g
		}
		lg.Gone = true
		s.Captured[winner.Side][lg.Slot] = lg
		s.Occ[lg.Row][lg.Col] = -1
	})
	if loser != nil && loser.Dead {
		// 戰死：槽空掉，不進名單。
		lg := g
		if loser == t.Leader {
			lg = t
		}
		lg.Gone = true
		s.Occ[lg.Row][lg.Col] = -1
	}
}

// finish 是結束（`0x2e51b`–`0x2e688`）：體能寫回進來時的值，然後
// 守方抓到的人從第 9 槽往第 0 槽、再攻方的，逐一交給捕獲方處置
//（`0x259fe`，`capture`）。
func (s *Skirmish) finish() {
	s.log("對戰結束：時刻 %d", s.Hour)
	for side := range s.Gens {
		for _, g := range s.Gens[side] {
			if g != nil {
				g.Leader.Stamina = uint8(g.StaminaCap)
			}
		}
	}
	for side := range s.Captured {
		for slot := skirmishSlots - 1; slot >= 0; slot-- {
			x := s.Captured[side][slot]
			if x == nil {
				continue
			}
			s.b.capture(s.Units[side].Side, x.Unit, x.Leader)
		}
	}
	for _, u := range s.Units {
		s.b.wipeCheck(u)
	}
	s.b.checkOver()
}

// Engage 是玩家部隊的「對戰」（主戰場命令 2，`0x28e6a`）：往 d 那一格的
// 敵軍開一場對戰子畫面，玩家那一方的將領由 player 下令（nil 就照電腦的
// 判斷式走）。打完把這支部隊的移動力歸零（`0x29009`）。
func (b *Battle) Engage(a *Unit, d Dir, player SkirmishPlayer) (*Skirmish, error) {
	if err := b.canAct(a); err != nil {
		return nil, err
	}
	t := b.UnitAt(a.At.Step(d))
	if t == nil {
		return nil, fmt.Errorf("battle: 那個方向沒有部隊")
	}
	if t.Side.Attacking() == a.Side.Attacking() {
		return nil, fmt.Errorf("battle: 那是友軍")
	}
	s := b.NewSkirmish(a, t)
	if player != nil {
		s.Player = player
	}
	s.Run()
	a.Move = 0
	return s, nil
}
