package battle

// 原版的部隊 AI：每支電腦部隊每天走一次的九支判斷式（`0x29014`，
// `docs/re/05` §12.1，`L0`＋`L1`、`[base]`；對拍 `TestZZUnitAIDayParity`）。
//
// 入口清掉決策槽（`es:0x31c0` ← `0xFFFF`），九支依序試，某一支定案就
// 寫 0、後面的不再試。有亂數門的幾支，門在「槽還空著」時才擲——
// 所以擲骰的順序與次數就是這段程式的順序，改動順序會讓亂數序列岔開。
//
//	1 選目標與評估   無條件
//	2 退兵           無條件
//	3 移動           槽空
//	4 弓箭           槽空 ＋ RND(2)==0
//	   （目標軍力 == 0xFFFF → 直接跳到 9）
//	5 策略           槽空 ＋ RND(3)!=0
//	6 死戰           槽空 ＋ RND(4)==0
//	7 對戰           槽空 ＋ RND(16)==0
//	8 快戰           槽空
//	9 休息           槽空
//
// remake 的 `enhanced` 模式不走這裡（`auto.go`）。

// AI 是自動作戰用哪一套判斷式。零值是原版（`docs/design/01`：還原版本
// 是預設，創作版本要明說）。
type AI int

const (
	// AIBase 是原版的九支（本檔）。
	AIBase AI = iota
	// AIPlus 是加強版：同一條鏈，只有行軍目標不同（`docs/re/05` §8.2）。
	AIPlus
	// AIEnhanced 是 remake 自己的策略（`auto.go`）。
	AIEnhanced
)

// Escape 是退兵時逃得去的一個鄰郡（原版 `0x23dd4`）：戰場所在郡的鄰郡
// 裡，無主或自己勢力的、而且**不是**對方助軍出兵的那一郡。Active 是
// 那一郡目前的現役武將數——原版要求「部隊的將領數 ＋ 現役 ≤ 50」才逃得進去。
type Escape struct {
	Prefecture int
	Active     int
}

// 原版判斷式裡的常數（`L0`，`DS:` 的 double）。
const (
	// baseAttackRatioCap：守著城池而主帥沒被貼身時，只有「敵方總兵力 ÷
	// 我方總兵力」低於它才主動快戰／對戰（`DS:0xa95a`）。
	baseAttackRatioCap = 0.4
	// baseDeathRatioCap：死戰只打「(目標兵力＋1) ÷ (我方兵力＋1)」不超過它
	// 的目標（`DS:0xa964`）。
	baseDeathRatioCap = 0.23
	// baseMoveCap 是休息一天回填之後移動力的上限（`0x29e5f`）。
	baseMoveCap = 15
	// baseRestGain 是休息一天回填的移動力（`0x29e53`）。
	baseRestGain = 2
	// baseEscapeRoom 是逃進鄰郡的將領上限（`0x23ebf`）。
	baseEscapeRoom = 50
)

// baseEval 是選項 1 算出來、後面幾支共用的評估值。
type baseEval struct {
	// ratio 是敵方總兵力 ÷ 我方總兵力（`es:0x1bf0`）。兩邊各是
	// 100 × Σ(軍力記錄 offset 14 兵士（百）) ＋ 1。
	ratio float64
	// holdsCity 對應 `es:0xa6 == 0`：主軍、城池那一格站著我方部隊、
	// 而且主帥（帥隊）旁邊沒有敵人。
	holdsCity bool
	// target 是選中的相鄰敵軍（`es:0x31a8`／`es:0x1604`），沒有就是 nil。
	target *Unit
	// soldierRatio 是 (目標兵力＋1) ÷ (我方兵力＋1)（`es:0x1610`）。
	soldierRatio float64
}

// BaseDecision 是原版那條鏈定案的結果：哪一支（1–9，`docs/re/05` §12 的
// 編號）、對誰（選項 4–8 的目標；弓箭是它自己挑的）。給對拍讀。
type BaseDecision struct {
	Option int
	Target *Unit
}

// autoTurnBase 是原版的一天一決策。
func (b *Battle) autoTurnBase(u *Unit) { b.DecideBase(u) }

// DecideBase 走一遍原版的九支，回傳定案的是哪一支。
func (b *Battle) DecideBase(u *Unit) BaseDecision {
	ev := b.baseEvaluate(u)
	if b.baseRetreat(u, ev) {
		return BaseDecision{Option: 2}
	}
	if b.baseMove(u, ev) {
		return BaseDecision{Option: 3}
	}
	if b.roll(2) == 0 {
		if t := b.baseArchery(u); t != nil {
			return BaseDecision{Option: 4, Target: t}
		}
	}
	if ev.target == nil {
		b.baseRest(u)
		return BaseDecision{Option: 9}
	}
	if b.roll(3) != 0 && b.baseStratagem(u, ev) {
		return BaseDecision{Option: 5, Target: ev.target}
	}
	if b.roll(4) == 0 && b.baseDeathBattle(u, ev) {
		return BaseDecision{Option: 6, Target: ev.target}
	}
	if b.roll(16) == 0 && b.baseEngage(u, ev) {
		return BaseDecision{Option: 7, Target: ev.target}
	}
	if b.baseQuickBattle(u, ev) {
		return BaseDecision{Option: 8, Target: ev.target}
	}
	b.baseRest(u)
	return BaseDecision{Option: 9}
}

// s16 把部隊的兵士數換成原版讀到的樣子：部隊記錄 offset 30 是 16 位元，
// 判斷式用的是**有號**比較與 `fild word`（`0x292e2` 的 `cmp`＋`jle`、
// `0x2a1ee` 的 `fild`）。兩位將領各兩萬七的部隊在原版眼裡是負的——
// 於是「被貼身的敵軍壓倒」那一道門會過。量到的（`TestZZUnitAIDayParity`
// 盤面丙），照做，不修。
func s16(v int) int { return int(int16(v)) }

// armyHundreds 是一個軍力的「兵士（百）」（軍力記錄 offset 14）：
// 五支部隊的兵士數合計 ÷ 100。
func (b *Battle) armyHundreds(s Side) int {
	n := 0
	for _, x := range b.Units {
		if x.Side == s && x.Alive() {
			n += x.Soldiers()
		}
	}
	return n / 100
}

// baseEvaluate 是選項 1（`0x29e78`）。
func (b *Battle) baseEvaluate(u *Unit) baseEval {
	def := float64(100*(b.armyHundreds(MainDefender)+b.armyHundreds(AidDefender)) + 1)
	att := float64(100*(b.armyHundreds(MainAttacker)+b.armyHundreds(AidAttacker)) + 1)
	ev := baseEval{soldierRatio: 1}
	if u.Side.Attacking() {
		ev.ratio = def / att
	} else {
		ev.ratio = att / def
	}

	// `es:0xa6`：先看城池那一格是不是我方的（0），再看主帥旁邊有沒有
	// 敵人（有 → 0xFFFF），助軍一律 0xFFFF。
	holds := false
	if c := b.UnitAt(b.Field.CityAt); c != nil && c.Side.Attacking() == u.Side.Attacking() {
		holds = true
	}
	if lead := b.unitOf(u.Side, Centre); lead != nil {
		for _, d := range Dirs() {
			if t := b.UnitAt(lead.At.Step(d)); t != nil && t.Side.Attacking() != u.Side.Attacking() {
				holds = false
			}
		}
	}
	if u.Side == AidAttacker || u.Side == AidDefender {
		holds = false
	}
	ev.holdsCity = holds

	// 相鄰的敵軍，六個方向照順序收（`es:[0x1ece]`／`es:[0x16f6]`）。
	var near []*Unit
	for _, d := range Dirs() {
		if t := b.UnitAt(u.At.Step(d)); t != nil && t.Side.Attacking() != u.Side.Attacking() {
			near = append(near, t)
		}
	}
	if len(near) == 0 {
		return ev
	}
	pick := b.roll(len(near))
	// 帥隊在清單裡就改挑它（掃六格，後面的蓋前面的）。
	for i, t := range near {
		if t.Formation == Centre {
			pick = i
		}
	}
	ev.target = near[pick]
	ev.soldierRatio = float64(s16(ev.target.Soldiers())+1) / float64(s16(u.Soldiers())+1)
	return ev
}

// unitOf 找某一方的某一隊，不在場上回 nil。
func (b *Battle) unitOf(s Side, f Formation) *Unit {
	for _, x := range b.Units {
		if x.Side == s && x.Formation == f && x.Alive() {
			return x
		}
	}
	return nil
}

// baseRetreat 是選項 2（`0x29138`）：
//
//	相鄰敵軍裡兵力最多的那一支 ＝ M；沒有相鄰敵軍就不退
//	我方兵力 > M ÷ (4 + RND(2)) → 不退
//	RND(8) + 3 <= 敵方總兵力 ÷ 我方總兵力 → 退，否則不退
//	退兵常式 `0x23dd4`：有逃得去的鄰郡才成立
func (b *Battle) baseRetreat(u *Unit, ev baseEval) bool {
	most := 0
	seen := false
	for _, d := range Dirs() {
		if t := b.UnitAt(u.At.Step(d)); t != nil && t.Side.Attacking() != u.Side.Attacking() {
			seen = true
			if n := s16(t.Soldiers()); n > most {
				most = n
			}
		}
	}
	if !seen {
		return false
	}
	if s16(u.Soldiers()) > most/(4+b.roll(2)) {
		return false
	}
	if float64(b.roll(8)+3) > ev.ratio {
		return false
	}
	if !b.baseCanEscape(u) {
		return false
	}
	return b.Retreat(u) == nil
}

// baseCanEscape 對應退兵常式列出的鄰郡清單非空（`0x23e34`–`0x23f0d`）。
func (b *Battle) baseCanEscape(u *Unit) bool {
	n := 0
	for i := range u.Leaders {
		if x := &u.Leaders[i]; !x.Dead && !x.Captured {
			n++
		}
	}
	if n <= 0 {
		return false
	}
	for _, e := range b.Escapes[u.Side] {
		if n+e.Active <= baseEscapeRoom {
			return true
		}
	}
	return false
}

// baseMove 是選項 3（`0x29344`）：往行軍目標走，走到移動力不夠或路被
// 擋住為止。
//
// 守著城池、主帥沒被貼身的主軍（`es:0xa6 == 0`）多一道門：城池那一格
// 的我方部隊兵力（百）大於城池六個鄰格敵軍兵力（百）的和就不動——
// 城還守得住，不必出去。
func (b *Battle) baseMove(u *Unit, ev baseEval) bool {
	goal := b.baseGoal(u)
	if ev.holdsCity {
		holder := b.UnitAt(goal)
		around := 0
		for _, d := range Dirs() {
			if t := b.UnitAt(goal.Step(d)); t != nil && t.Side.Attacking() != u.Side.Attacking() {
				around += s16(t.Soldiers()) / 100
			}
		}
		if holder != nil && s16(holder.Soldiers())/100 > around {
			return false
		}
	}
	path := b.basePath(u, goal)
	if path == nil {
		return false
	}
	moved := false
	for b.baseStep(u, path) {
		moved = true
	}
	return moved
}

// baseGoal 是行軍目標（`es:0x584`／`es:0x586`）。原版開戰時掃一次地圖，
// 城池那一格（地形碼 5）就是它，之後不變（`0x23afe`）。
//
// 加強版預設是對方主帥的部隊所在格；難度 ≥ 11 而對方主帥不是君主時
// 才換成城池（`0x26758`，`docs/re/05` §8.2）——那一條的「不是君主」
// 要看人物的身分，這一層沒有，先一律用城池；量到再補。
func (b *Battle) baseGoal(u *Unit) Hex { return b.Field.CityAt }

const (
	pathOpen    = 9999 // 還沒走到的格子（`0x270f`）
	pathBlocked = -999 // 大山、圖外、我方部隊佔的格子（`0xfc19`）
)

// basePath 照原版三段算出這支部隊要走的路：
//
//	`0x24842`  距離圖：大山／圖外／我方部隊 ← −999，其餘 ← 9999；
//	           行軍目標那一格無條件 ← 9999
//	`0x24918`  從自己這一格起 BFS，自己 ＝ 1，鄰格 ＝ 2……
//	`0x24b1a`  從目標倒著走回來，每一步挑「距離剛好少一」的鄰格裡
//	           地形移動花費最小的那一格（先到先贏），標成路
//
// 目標到不了（距離還是 9999 或 −999）回 nil。回傳的是「哪些格在路上」。
func (b *Battle) basePath(u *Unit, goal Hex) map[Hex]bool {
	dist := map[Hex]int{}
	b.Field.Cells(func(h Hex, t Terrain) {
		v := pathOpen
		if t == Mountain || b.Field.Outside(h) {
			v = pathBlocked
		}
		if x := b.UnitAt(h); x != nil && x.Side.Attacking() == u.Side.Attacking() {
			v = pathBlocked
		}
		dist[h] = v
	})
	dist[goal] = pathOpen

	dist[u.At] = 1
	for level := 1; ; level++ {
		grew := false
		b.Field.Cells(func(h Hex, _ Terrain) {
			if dist[h] != level {
				return
			}
			for _, d := range Dirs() {
				n := h.Step(d)
				if v, ok := dist[n]; ok && v > level {
					dist[n] = level + 1
					grew = true
				}
			}
		})
		if !grew {
			break
		}
	}

	d := dist[goal]
	if d == pathOpen || d == pathBlocked {
		return nil
	}
	path := map[Hex]bool{goal: true}
	cur := goal
	for level := d - 1; ; level-- {
		best, bestCost := NoHex, 99
		for _, dir := range Dirs() {
			n := cur.Step(dir)
			if v, ok := dist[n]; !ok || v != level {
				continue
			}
			if c := MoveCost(b.Field.At(n), u.Troop()); c < bestCost {
				best, bestCost = n, c
			}
		}
		if best == NoHex {
			break
		}
		path[best] = true
		cur = best
	}
	return path
}

// baseStep 走一步（`0x294ae`）：六個鄰格裡「在路上而且空著」的最後一格，
// 地形花費不超過剩下的移動力才走。走過的格子從路上劃掉。
func (b *Battle) baseStep(u *Unit, path map[Hex]bool) bool {
	delete(path, u.At)
	next, nd := NoHex, Dir(0)
	for _, d := range Dirs() {
		n := u.At.Step(d)
		if path[n] && b.UnitAt(n) == nil && b.Field.InBounds(n) {
			next, nd = n, d
		}
	}
	if next == NoHex {
		return false
	}
	if MoveCost(b.Field.At(next), u.Troop()) > u.Move {
		return false
	}
	return b.Move(u, nd) == nil
}

// baseArchery 是選項 4（`0x2985c`）：箭要還有；六個方向各**同方向連走
// 兩步**，中間那格不是大山／城池／關寨、落點站著敵軍的收進候選
// （照方向存，`es:[0x1ece + 方向×2]`）；RND(候選數) 起算、帥隊優先，
// 空槽往後找；射完移動力歸零。
func (b *Battle) baseArchery(u *Unit) *Unit {
	if u.Arrows <= 0 {
		return nil
	}
	var slots [6]*Unit
	count := 0
	for i, d := range Dirs() {
		mid := u.At.Step(d)
		far := mid.Step(d)
		if !b.Field.InBounds(far) {
			continue
		}
		t := b.UnitAt(far)
		if t == nil {
			continue
		}
		switch b.Field.At(mid) {
		case Mountain, City, Fort:
			continue
		}
		if t.Side.Attacking() == u.Side.Attacking() {
			continue
		}
		slots[i] = t
		count++
	}
	if count == 0 {
		return nil
	}
	pick := b.roll(count)
	for i, t := range slots {
		if t != nil && t.Formation == Centre {
			pick = i
		}
	}
	for slots[pick] == nil {
		pick = (pick + 1) % 6
	}
	if err := b.ArcheryOnce(u, slots[pick].At); err != nil {
		return nil
	}
	u.Move = 0
	return slots[pick]
}

// baseStratagem 是選項 5（`0x29784`）：先擲 RND(6) 挑計，錢不夠或最聰明
// 那一位的謀略不到門檻就不用；再過天候門與判定（成敗都扣錢）。
func (b *Battle) baseStratagem(u *Unit, ev baseEval) bool {
	// 原版的表照 火攻、水淹、陷阱、誘敵、燒糧、圍攻 排（`DS:0x7f62`／
	// `DS:0x7f6e`），與 remake 的列舉同序，只差列舉從 1 起。
	s := Stratagem(b.roll(6) + 1)
	if b.Gold[u.Side] < s.Cost() {
		return false
	}
	wise := u.Smartest()
	if wise == nil || int(wise.Intel) < s.MinIntel() {
		return false
	}
	return b.UseStratagem(u, s, ev.target.At) == nil
}

// baseDeathBattle 是選項 6（`0x29c56`）：目標兵力（＋1）不超過我方的
// 23%；站在城池或關寨上時只打帥隊；模式 ＝ 難度 ÷ 5，打到一方無將。
func (b *Battle) baseDeathBattle(u *Unit, ev baseEval) bool {
	if ev.soldierRatio > baseDeathRatioCap {
		return false
	}
	switch b.Field.At(u.At) {
	case City, Fort:
		if ev.target.Formation != Centre {
			return false
		}
	}
	d, ok := b.dirTo(u, ev.target)
	if !ok {
		return false
	}
	if err := b.meleeMode(u, d, b.Difficulty/5, true); err != nil {
		return false
	}
	// 本隊還有將領就佔進對方那一格（`0x29dd6`–`0x29e16`；迴圈是打到
	// 一方無將才停，所以這時那一格已經空了）。不花移動力。
	if u.Alive() && !ev.target.Alive() {
		u.At = ev.target.At
		if u.At == b.Field.CityAt {
			b.CityHeld = u.Side
			b.note("blog.takeCity", u.Name())
		}
	}
	return true
}

// baseEngage 是選項 7（`0x29b82`）：RND(100) ＋ 我方兵力 ÷ 2 要不小於
// 目標兵力；守著城池且主帥沒被貼身時另要敵我總兵力比 < 0.4。原版接
// 對戰子畫面（`0x2deb0`），remake 的「對戰」是叫陣單挑。
func (b *Battle) baseEngage(u *Unit, ev baseEval) bool {
	if b.roll(100)+s16(u.Soldiers())/2 < s16(ev.target.Soldiers()) {
		return false
	}
	if ev.holdsCity && ev.ratio >= baseAttackRatioCap {
		return false
	}
	d, ok := b.dirTo(u, ev.target)
	if !ok {
		return false
	}
	ca, ct := u.Chief(), ev.target.Chief()
	accept := false
	if ca != nil && ct != nil {
		accept = DuelAccepted(int(ca.War), int(ct.War), u.Soldiers(), ev.target.Soldiers(),
			b.roll(DuelWarSpread), b.roll(DuelOddsSpread))
	}
	return b.Duel(u, d, accept) == nil
}

// baseQuickBattle 是選項 8（`0x29ade`）：守著城池且主帥沒被貼身時要
// 敵我總兵力比 < 0.4；模式 ＝ 難度 ÷ 5 ＋ 1。
func (b *Battle) baseQuickBattle(u *Unit, ev baseEval) bool {
	if ev.holdsCity && ev.ratio >= baseAttackRatioCap {
		return false
	}
	d, ok := b.dirTo(u, ev.target)
	if !ok {
		return false
	}
	return b.meleeMode(u, d, b.Difficulty/5+1, false) == nil
}

// baseRest 是選項 9（`0x29e2e`）：移動力 ＋2，上限 15。
func (b *Battle) baseRest(u *Unit) {
	u.Move += baseRestGain
	if u.Move > baseMoveCap {
		u.Move = baseMoveCap
	}
}

// dirTo 找目標在哪個方向（要相鄰）。
func (b *Battle) dirTo(u, t *Unit) (Dir, bool) {
	for _, d := range Dirs() {
		if u.At.Step(d) == t.At {
			return d, true
		}
	}
	return 0, false
}
