package battle

import (
	"fmt"
	"sort"
)

// 主戰場的一場戰役（說明書 p.28–35）。

// Weather 是天氣。火攻要刮風、水淹要下雨、燒糧下雨不能用（p.32–34）。
type Weather uint8

const (
	Clear Weather = iota // 晴
	Windy                // 刮風
	Rainy                // 下雨
)

func (w Weather) String() string {
	switch w {
	case Windy:
		return "刮風"
	case Rainy:
		return "下雨"
	}
	return "晴"
}

// Battle 是一場進行中的戰役。
type Battle struct {
	Field *Field

	// Day 是第幾天，從 1 起。主戰場每一回合耗去一天（說明書 p.28）。
	Day     int
	Weather Weather

	Units []*Unit

	// Gold／Rice 是各軍隨軍攜帶的錢糧（說明書 p.28）。
	// 「沒有帶錢就無法用計」「沒米給軍隊吃，士兵將會陸續逃亡」。
	Gold [sideCount]int
	Rice [sideCount]int

	// CityHeld 是城池目前在誰手上。守方一開始持有。
	CityHeld Side

	Log []string

	// Over／AttackerWon 是結果。
	Over        bool
	AttackerWon bool

	rng *rand
}

// Setup 是開一場戰役要的東西。
type Setup struct {
	Field   *Field
	Weather Weather
	Seed    uint32

	// Attackers／Defenders 是各方的將領，會被分成五種隊伍。
	Attackers []Leader
	Defenders []Leader

	// AidAttackers／AidDefenders 是助攻軍與助守軍，可以是空的。
	AidAttackers []Leader
	AidDefenders []Leader

	// FromGate 是主攻軍的入口（來犯的鄰郡編號）。
	FromGate int

	AttackerGold, AttackerRice int
	DefenderGold, DefenderRice int
}

// New 開一場戰役。
//
// 編隊順序是主攻軍 → 助攻軍 → 主守軍 → 助守軍（說明書 p.27），
// 紮營順序是中軍 → 先鋒 → 左軍 → 右軍 → 後軍（p.28）。
func New(s Setup) *Battle {
	b := &Battle{Field: s.Field, Day: 1, Weather: s.Weather,
		CityHeld: MainDefender, rng: newRand(s.Seed)}
	b.Gold[MainAttacker], b.Rice[MainAttacker] = s.AttackerGold, s.AttackerRice
	b.Gold[MainDefender], b.Rice[MainDefender] = s.DefenderGold, s.DefenderRice

	entry := s.Field.Gates[s.FromGate]
	if !s.Field.InBounds(entry) {
		entry = FromOffset(0, FieldH/2)
	}
	for _, side := range SideDeployOrder() {
		var pool []Leader
		switch side {
		case MainAttacker:
			pool = s.Attackers
		case AidAttacker:
			pool = s.AidAttackers
		case MainDefender:
			pool = s.Defenders
		case AidDefender:
			pool = s.AidDefenders
		}
		if len(pool) == 0 {
			continue
		}
		base := entry
		if !side.Attacking() {
			base = s.Field.CityAt
		}
		b.Units = append(b.Units, b.formUp(side, pool, base)...)
	}
	b.note("戰役開始（%s）", b.Weather)
	return b
}

// formUp 把一批將領分成五種隊伍並紮營。
//
// 「如果派出全部兵力，各戰鬥組的將領人數必須平均分配」「每組最多 10 名將領」
// （說明書 p.27）。
func (b *Battle) formUp(side Side, pool []Leader, base Hex) []*Unit {
	// 依戰力排序再輪流分配：分組是決定性的，而且中軍先分到最強的
	// （紮營順序以中軍為首，p.28）。手冊只要求「人數平均分配」，
	// 沒說哪一隊該強，強弱的取捨是 remake 的。
	sorted := append([]Leader(nil), pool...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].War > sorted[j].War })

	groups := int(formationCount)
	if len(sorted) < groups {
		groups = len(sorted)
	}
	units := make([]*Unit, 0, groups)
	order := DeployOrder()
	for i := 0; i < groups; i++ {
		units = append(units, &Unit{Side: side, Formation: order[i]})
	}
	for i, l := range sorted {
		u := units[i%groups]
		if len(u.Leaders) >= MaxLeaders {
			continue
		}
		u.Leaders = append(u.Leaders, l)
	}
	// 紮營：依 DeployOrder 從基準點往外排。
	spot := base
	for i, u := range units {
		h := spot
		for n := 0; n < 6 && (!b.Field.At(h).Passable() || b.occupied(h)); n++ {
			h = spot.Step(Dirs()[n])
		}
		u.At = h
		u.Move = u.MovePoints()
		u.Started = u.Soldiers()
		spot = base.Step(Dirs()[i%6])
	}
	return units
}

// Camp 把一支部隊移到指定的格子上——**紮營，不是移動**：
// 不扣移動力、不看距離，但落點要走得進去而且沒有人。
//
// 原版逐隊問位置：`(%2d%s)%s之%s請%s將軍紮寨`（`AA.EXE` `0x46b67`，
// `docs/re/04` §5）。
func (b *Battle) Camp(u *Unit, at Hex) error {
	if u == nil || !u.Alive() {
		return fmt.Errorf("battle: 這支部隊不在場上")
	}
	if b.Day != 1 {
		return fmt.Errorf("battle: 紮營只在開戰前")
	}
	if !b.Field.InBounds(at) {
		return fmt.Errorf("battle: 出界了")
	}
	if !b.Field.At(at).Passable() {
		return fmt.Errorf("battle: %s 紮不了營", b.Field.At(at))
	}
	if x := b.UnitAt(at); x != nil && x != u {
		return fmt.Errorf("battle: 那一格已經有 %s", x.Name())
	}
	u.At = at
	if at == b.Field.CityAt {
		b.CityHeld = u.Side
	}
	return nil
}

// CampArea 回報一支部隊能不能在這一格紮營，給畫面先擋掉不能選的格子。
func (b *Battle) CampArea(u *Unit, at Hex) bool {
	if !b.Field.InBounds(at) || !b.Field.At(at).Passable() {
		return false
	}
	x := b.UnitAt(at)
	return x == nil || x == u
}

func (b *Battle) occupied(h Hex) bool {
	for _, u := range b.Units {
		if u.Alive() && u.At == h {
			return true
		}
	}
	return false
}

// UnitAt 回傳某一格上的部隊；沒有回 nil。
func (b *Battle) UnitAt(h Hex) *Unit {
	for _, u := range b.Units {
		if u.Alive() && u.At == h {
			return u
		}
	}
	return nil
}

func (b *Battle) note(format string, a ...any) {
	b.Log = append(b.Log, fmt.Sprintf("第 %d 日　", b.Day)+fmt.Sprintf(format, a...))
}

// sideAlive 回報某個軍力還有沒有部隊在場上。
func (b *Battle) sideAlive(s Side) bool {
	for _, u := range b.Units {
		if u.Side == s && u.Alive() {
			return true
		}
	}
	return false
}

// Rest 是「休息」：在原地不動，**增加移動力 2**（說明書 p.29、p.30）。
func (b *Battle) Rest(u *Unit) error {
	if err := b.canAct(u); err != nil {
		return err
	}
	u.Move += TuneRestMove
	// 「並恢復將領的體力」（p.30）。
	for i := range u.Leaders {
		if u.Leaders[i].Stamina < 100 {
			u.Leaders[i].Stamina++
		}
	}
	return nil
}

func (b *Battle) canAct(u *Unit) error {
	if b.Over {
		return fmt.Errorf("battle: 這場戰役已經結束")
	}
	if u == nil || !u.Alive() {
		return fmt.Errorf("battle: 這支部隊已經不在場上")
	}
	if u.Trapped > 0 {
		return fmt.Errorf("battle: %s 中了陷阱，還有 %d 天不能活動", u.Name(), u.Trapped)
	}
	return nil
}

// Move 是「移動」：往一個方向走一格。
//
// 「不得穿越其他軍隊」「大山無法穿越」（說明書 p.29）。
func (b *Battle) Move(u *Unit, d Dir) error {
	if err := b.canAct(u); err != nil {
		return err
	}
	to := u.At.Step(d)
	t := b.Field.At(to)
	if !t.Passable() {
		return fmt.Errorf("battle: %s 過不去", t)
	}
	if !b.Field.InBounds(to) {
		return fmt.Errorf("battle: 出界了")
	}
	if b.UnitAt(to) != nil {
		return fmt.Errorf("battle: 那一格有部隊，不得穿越")
	}
	cost := MoveCost(t, u.Troop())
	if u.Move < cost {
		return fmt.Errorf("battle: 移動力不足（要 %d，剩 %d）", cost, u.Move)
	}
	u.Move -= cost
	u.At = to
	if to == b.Field.CityAt {
		b.CityHeld = u.Side
		b.note("%s 進佔城池", u.Name())
	}
	return nil
}

// power 是一支部隊此刻的攻擊力。
//
// 因素照手冊 p.31：訓練度、武裝度、兵數、部隊位置地形、兵種、有無用計。
func (b *Battle) power(u *Unit) int {
	q := 100 + u.AvgTraining()*TuneHitTraining/100 + u.AvgArms()*TuneHitArms/100
	if c := u.Chief(); c != nil {
		q += int(c.War) * TuneHitWar / 100
	}
	t := b.Field.At(u.At)
	q = q * (100 + AttackMod(t)) / 100
	if u.Troop().Suits(t) {
		q = q * 110 / 100 // 「能適應地形的兵種攻防能力會更好」（p.31）
	}
	if u.Enraged > 0 {
		q = q * (100 - TuneEnragedPenalty) / 100
	}
	return u.Soldiers() * q / 100
}

// defence 是一支部隊此刻的防禦力。
func (b *Battle) defence(u *Unit) int {
	q := 100 + u.AvgTraining()*TuneHitTraining/100 + u.AvgArms()*TuneHitArms/100
	t := b.Field.At(u.At)
	q = q * (100 + DefenceMod(t)) / 100
	if u.Troop().Suits(t) {
		q = q * 110 / 100
	}
	return u.Soldiers() * q / 100
}

// hit 讓 a 打 b 一次，回傳 b 的損失。
//
// pct 是這一擊的強度百分比，100 是白刃相接的基準。弓箭比它輕
// （`TuneArrowDamage`），圍攻比它重（`TuneSiegeBonus`）。
// **用百分比不用整數倍**：整數倍表示不了「箭只有一半殺傷」。
func (b *Battle) hit(a, d *Unit, pct int) int {
	ap, dp := b.power(a), b.defence(d)
	if dp <= 0 {
		dp = 1
	}
	loss := d.Soldiers() * TuneHitBase * ap * pct / (dp * 100 * 100)
	if loss < 1 {
		loss = 1
	}
	if loss > d.Soldiers() {
		loss = d.Soldiers()
	}
	b.casualty(d, loss)
	return loss
}

// casualty 把損失分攤到隊伍裡的各將領。
//
// 「人員損耗後，其持有的軍械也隨同失去」（說明書 p.20）——所以
// 兵沒了武裝度不變（那是比率），但總量跟著少。
func (b *Battle) casualty(u *Unit, loss int) {
	total := u.Soldiers()
	if total <= 0 {
		return
	}
	left := loss
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if x.Dead || x.Captured || x.Soldiers == 0 {
			continue
		}
		n := loss * x.Soldiers / total
		if n > x.Soldiers {
			n = x.Soldiers
		}
		x.Soldiers -= n
		left -= n
	}
	for i := range u.Leaders {
		if left <= 0 {
			break
		}
		x := &u.Leaders[i]
		n := left
		if n > x.Soldiers {
			n = x.Soldiers
		}
		x.Soldiers -= n
		left -= n
	}
	if u.Soldiers() == 0 {
		u.Wiped = true
		b.note("%s 全滅", u.Name())
	}
}

// QuickBattle 是「快戰」：雙方直接正面作戰（說明書 p.32）。
func (b *Battle) QuickBattle(a *Unit, d Dir) error {
	return b.melee(a, d, false)
}

// DeathBattle 是「死戰」：一決生死的激戰，**雙方將互戰至分出勝負為止**
// （說明書 p.32）。
func (b *Battle) DeathBattle(a *Unit, d Dir) error {
	return b.melee(a, d, true)
}

func (b *Battle) melee(a *Unit, d Dir, toTheDeath bool) error {
	if err := b.canAct(a); err != nil {
		return err
	}
	t := b.UnitAt(a.At.Step(d))
	if t == nil {
		return fmt.Errorf("battle: 那個方向沒有部隊")
	}
	if t.Side.Attacking() == a.Side.Attacking() {
		return fmt.Errorf("battle: 那是友軍")
	}
	rounds := 1
	if toTheDeath {
		rounds = 50 // 打到分出勝負；上限避免無窮迴圈
	}
	for i := 0; i < rounds; i++ {
		la := b.hit(a, t, 100)
		if !t.Alive() {
			b.note("%s 擊潰 %s（斬 %d）", a.Name(), t.Name(), la)
			break
		}
		lb := b.hit(t, a, 100)
		if !a.Alive() {
			b.note("%s 反擊得手（斬 %d）", t.Name(), lb)
			break
		}
		if !toTheDeath {
			b.note("%s 攻 %s：斬 %d，被斬 %d", a.Name(), t.Name(), la, lb)
			break
		}
	}
	a.Move = 0
	// 一擊打光對方最後一支部隊時當場分勝負，不必等這一天結束。
	b.checkOver()
	return nil
}

// Archery 是「弓箭」：射箭削弱敵軍兵力（說明書 p.32）。
//
// 「攻擊目標必須**相間一格**，且不能隔著大山、城池或關寨。」
func (b *Battle) Archery(a *Unit, target Hex) error {
	if err := b.canAct(a); err != nil {
		return err
	}
	mid, ok := lineMid(a.At, target)
	if !ok {
		return fmt.Errorf("battle: 弓箭的目標必須在同一直線上相間一格")
	}
	switch b.Field.At(mid) {
	case Mountain, City, Fort:
		return fmt.Errorf("battle: 不能隔著%s射箭", b.Field.At(mid))
	}
	t := b.UnitAt(target)
	if t == nil {
		return fmt.Errorf("battle: 那裡沒有部隊")
	}
	if t.Side.Attacking() == a.Side.Attacking() {
		return fmt.Errorf("battle: 那是友軍")
	}
	n := a.Arrows()
	if n <= 0 {
		return fmt.Errorf("battle: 武裝度不足，射不出箭")
	}
	total := 0
	for i := 0; i < n; i++ {
		if !t.Alive() {
			break
		}
		total += b.hit(a, t, TuneArrowDamage*100/TuneHitBase)
	}
	b.note("%s 射了 %d 次箭，%s 折損 %d", a.Name(), n, t.Name(), total)
	a.Move = 0
	return nil
}

// lineMid 回傳 a 與 target 之間的那一格，target 不在同一直線上就回 false。
//
// **「相間一格」不等於「距離 2」。** 折線走兩步到的格子中間有兩格，
// 說不出箭「隔著什麼」飛過去；只有同一方向連走兩步才唯一。
func lineMid(a, target Hex) (Hex, bool) {
	for _, d := range Dirs() {
		m := a.Step(d)
		if m.Step(d) == target {
			return m, true
		}
	}
	return Hex{}, false
}

// Retreat 是「退兵」：逃至鄰郡（說明書 p.34）。
//
// 「若被地形和敵軍完全包圍則逃不掉」「若退兵成功，原先擁有的錢糧都會損失」。
func (b *Battle) Retreat(u *Unit) error {
	if err := b.canAct(u); err != nil {
		return err
	}
	free := false
	for _, d := range Dirs() {
		h := u.At.Step(d)
		if b.Field.At(h).Passable() && b.UnitAt(h) == nil && b.Field.InBounds(h) {
			free = true
			break
		}
	}
	if !free {
		return fmt.Errorf("battle: 被完全包圍，逃不掉")
	}
	// 「若退兵成功，原先擁有的錢糧都會損失」（p.34）——帶走的是**自己那一份**。
	// 整個軍力的錢糧一次歸零的話，一支殘兵退走會讓還在打的友軍突然用不起計。
	share := u.Soldiers()
	total := 0
	for _, x := range b.Units {
		if x.Side == u.Side && x.Alive() {
			total += x.Soldiers()
		}
	}
	if total <= 0 || share >= total {
		b.Gold[u.Side], b.Rice[u.Side] = 0, 0
	} else {
		b.Gold[u.Side] -= b.Gold[u.Side] * share / total
		b.Rice[u.Side] -= b.Rice[u.Side] * share / total
	}
	u.Retreated = true
	b.note("%s 撤退（隨身錢糧盡失）", u.Name())
	b.checkOver()
	return nil
}

// EndDay 結束這一天：重置移動力、遞減狀態、判定勝負。
func (b *Battle) EndDay() {
	for _, u := range b.Units {
		if !u.Alive() {
			continue
		}
		u.Move = u.MovePoints()
		if u.Trapped > 0 {
			u.Trapped--
		}
		if u.Enraged > 0 {
			u.Enraged--
		}
	}
	// 「沒米給軍隊吃，士兵將會陸續逃亡」（說明書 p.28）。
	for _, s := range []Side{MainAttacker, AidAttacker} {
		if !b.sideAlive(s) {
			continue
		}
		need := 0
		for _, u := range b.Units {
			if u.Side == s && u.Alive() {
				need += u.Soldiers() / 100
			}
		}
		if b.Rice[s] >= need {
			b.Rice[s] -= need
			continue
		}
		b.Rice[s] = 0
		for _, u := range b.Units {
			if u.Side == s && u.Alive() {
				b.casualty(u, u.Soldiers()/20)
			}
		}
		b.note("%s 缺糧，士兵陸續逃亡", s)
	}
	b.Day++
	b.checkOver()
}

// checkOver 判定勝負（說明書 p.35）。
//
//   - 守方：堅持滿卅天且城池未被奪去，或攻方全滅／全撤退
//   - 攻方：攻下城池並堅持到卅天為止
func (b *Battle) checkOver() {
	if b.Over {
		return
	}
	attackers := b.sideAlive(MainAttacker) || b.sideAlive(AidAttacker)
	defenders := b.sideAlive(MainDefender) || b.sideAlive(AidDefender)
	switch {
	case !attackers:
		b.Over, b.AttackerWon = true, false
		b.note("攻方全滅或撤退，守方衛郡成功")
	case !defenders:
		b.Over, b.AttackerWon = true, true
		b.note("守方全滅，攻方獲勝")
	case b.Day > BattleDays:
		b.Over = true
		b.AttackerWon = b.CityHeld.Attacking()
		if b.AttackerWon {
			b.note("卅天期滿，攻方據有城池，攻方獲勝")
		} else {
			b.note("卅天期滿，城池未失，守方衛郡成功")
		}
	}
}

// Order 依行動順序回傳還在場上的部隊（說明書 p.28）。
func (b *Battle) Order() []*Unit {
	var out []*Unit
	for _, s := range SideActionOrder() {
		for _, f := range ActionOrder() {
			for _, u := range b.Units {
				if u.Side == s && u.Formation == f && u.Alive() {
					out = append(out, u)
				}
			}
		}
	}
	return out
}
