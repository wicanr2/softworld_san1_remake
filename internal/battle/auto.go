package battle

import "sort"

// 自動作戰。
//
// 電腦指揮的部隊要有人替它下令；玩家不看的戰役也要打得完
//（原版的「其他 → 戰役」就是開關要不要看電腦作戰，說明書 p.26）。
//
// 策略是 remake 自己的，而且**決定性**：同一場戰役重跑得到同一個結果。

// AutoTurn 讓一支部隊自己走一步。
//
// 前段兩邊相同：中陷阱就不動 → 退兵 → 用計。之後攻守分家
//（`autoAttacker`／`autoDefender`）。
//
// **攻守不對稱**，那是規則逼出來的：打滿三十天而城池未被奪就算守方
// 衛郡成功（說明書 p.35）。所以
//
//   - **攻方非打不可**：拖到期滿就輸，打不贏也得換兵。
//   - **守方可以拖**：不划算的近戰不打，改用不挨反擊的手段
//     （弓箭、計謀）消耗，並守在城池旁邊。
//
// 原版的九支判斷式在 `autobase.go`（`AIBase`／`AIPlus`）；這一套是
// `AIEnhanced`——它是創作不是還原（`docs/design/02`），四個 `Tune*`
// 只有這裡在用。
func (b *Battle) AutoTurn(u *Unit) {
	if b.Over || !u.Alive() || u.Trapped > 0 {
		return
	}
	if b.AI != AIEnhanced {
		b.autoTurnBase(u)
		return
	}
	// 敗得夠慘而且整體居於劣勢就退兵（說明書 p.34）。
	// 少了這一步，每一場戰役都只有「全滅」或「打滿卅天」兩種結局，
	// 而戰略層看到的是攻方的將領全部戰死或被擒。
	if u.Started > 0 && u.Soldiers()*100 < u.Started*TuneRetreatShare &&
		b.sideStrength(u.Side.Attacking()) < b.sideStrength(!u.Side.Attacking()) {
		if err := b.Retreat(u); err == nil {
			return
		}
	}
	// 用計：把付得起的一個一個試過去。
	//
	// ⚠ **不能只試第一個。** 六種計謀各有天氣與地形限制，只試最貴的
	// 那一種，就會變成「刮風天才用得出計，其餘時候一招都不用」。
	//
	// **排在近戰前面是 remake 的取捨**（自動作戰那一層本來就是，
	// `docs/design/03` §6）：計謀與近戰的目標都只能是相鄰的格子
	//（`UseStratagem`），排在後面的話「貼身就開打」會把它整個蓋掉
	// ——量到過，一整批戰役六種計謀一次都沒用出來。
	// 計謀花錢、有門檻，而且不會挨反擊，付得起就先用。
	for _, c := range b.stratagemOptions(u) {
		if err := b.UseStratagem(u, c.what, c.at); err == nil {
			return
		}
	}
	if u.Side.Attacking() {
		b.autoAttacker(u)
		return
	}
	b.autoDefender(u)
}

// autoAttacker 是攻方的一步。
//
// **拖到第三十天就輸**（說明書 p.35），所以推進優先：貼身就打，
// 打不到就往城池走，走不動才射箭。
//
// ⚠ **射箭不能排在推進前面。** 射得到的距離是 2，而那正是「再走一步
// 就貼上」的距離——排前面的話攻方會停在兩格外把箭射完，等到接觸時
// 日子已經過了一半。量到過：單挑一次都沒出現，因為根本沒走到貼身。
func (b *Battle) autoAttacker(u *Unit) {
	if b.meleeTurn(u, true) {
		return
	}
	goal := b.Field.CityAt
	if b.CityHeld.Attacking() {
		// 城池已經拿下，剩下的是清殘敵。
		if t := b.nearestEnemy(u); t != nil {
			goal = t.At
		}
	}
	if b.advance(u, goal) {
		return
	}
	if b.shoot(u) {
		return
	}
	if u.Move > 0 {
		_ = b.Rest(u)
	}
}

// autoDefender 是守方的一步。
//
// **守滿三十天、城池未失就贏**（p.35），所以保存實力：不挨反擊的手段
// 先用（計謀在 `AutoTurn` 已經試過，這裡是弓箭），近戰只在佔上風時打，
// 其餘時間守在城池旁邊——**不追出去**。
//
// 先前守方走的是「最近的敵人」，那會把守軍一個一個引離城池，
// 而城池空著的時候攻方直接走進去。
func (b *Battle) autoDefender(u *Unit) {
	if b.shoot(u) {
		return
	}
	if b.meleeTurn(u, false) {
		return
	}
	if b.advance(u, b.Field.CityAt) {
		return
	}
	if u.Move > 0 {
		_ = b.Rest(u)
	}
}

// meleeTurn 處理貼身的敵人。desperate 為真表示「不划算也要打」。
//
// 回傳「這一步用掉了」。
func (b *Battle) meleeTurn(u *Unit, desperate bool) bool {
	for _, d := range Dirs() {
		t := b.UnitAt(u.At.Step(d))
		if t == nil || t.Side.Attacking() == u.Side.Attacking() {
			continue
		}
		// 領隊戰力明顯佔優就單挑——「依其戰力強弱分高下，
		// **與率領軍力大小無關**」（p.30），所以正面打不贏的時候
		// 這是唯一的翻盤手段。
		ca, ct := u.Chief(), t.Chief()
		if ca != nil && ct != nil &&
			int(ca.War) >= int(ct.War)+TuneDuelWarEdge &&
			b.power(u) < b.defence(t) {
			// **對方接不接受是原版的判定**，不是一律應戰
			// （`0x30c5b`，`docs/re/05` §9）。
			accept := DuelAccepted(
				int(ca.War), int(ct.War), u.Soldiers(), t.Soldiers(),
				b.roll(DuelWarSpread), b.roll(DuelOddsSpread))
			_ = b.Duel(u, d, accept)
			return true
		}
		// 「死戰：一決生死的激戰，雙方將互戰至分出勝負為止」（p.32）。
		// 佔上風才敢賭這一把；沒把握就打快戰，留著兵再看。
		if b.power(u) > b.defence(t)*TuneDeathBattleEdge/100 {
			_ = b.DeathBattle(u, d)
			return true
		}
		if !desperate {
			// 守方不必拿兵去換：時間站在它那邊。
			return false
		}
		// 日子過了 `TuneAttackerDesperateDay` 之後連死戰都要賭：
		// 那時保存實力已經沒有意義。
		if b.Day*100 >= BattleDays*TuneAttackerDesperateDay {
			_ = b.DeathBattle(u, d)
			return true
		}
		_ = b.QuickBattle(u, d)
		return true
	}
	return false
}

// shoot 射一箭。**不挨反擊**，次數有限（武裝度加權平均 ÷ 20），
// 用不完就浪費了。
func (b *Battle) shoot(u *Unit) bool {
	if u.Arrows <= 0 {
		return false
	}
	for _, t := range b.enemies(u) {
		if Distance(u.At, t.At) != 2 {
			continue
		}
		if err := b.Archery(u, t.At); err == nil {
			return true
		}
	}
	return false
}

// advance 往目標走，回傳「有沒有真的動過」。
func (b *Battle) advance(u *Unit, goal Hex) bool {
	moved := false
	for u.Move > 0 {
		d, ok := b.stepToward(u, goal)
		if !ok {
			break
		}
		if err := b.Move(u, d); err != nil {
			break
		}
		moved = true
		if u.At == goal {
			break
		}
	}
	return moved
}

// sideStrength 是某一立場還在場上的總兵力。
func (b *Battle) sideStrength(attacking bool) int {
	n := 0
	for _, x := range b.Units {
		if x.Alive() && x.Side.Attacking() == attacking {
			n += x.Soldiers()
		}
	}
	return n
}

func (b *Battle) enemies(u *Unit) []*Unit {
	var out []*Unit
	for _, x := range b.Units {
		if x.Alive() && x.Side.Attacking() != u.Side.Attacking() {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return Distance(u.At, out[i].At) < Distance(u.At, out[j].At)
	})
	return out
}

func (b *Battle) nearestEnemy(u *Unit) *Unit {
	e := b.enemies(u)
	if len(e) == 0 {
		return nil
	}
	return e[0]
}

// stepToward 挑一個讓距離變近而且走得過去的方向。
func (b *Battle) stepToward(u *Unit, goal Hex) (Dir, bool) {
	best, bestD := Dir(0), Distance(u.At, goal)
	found := false
	for _, d := range Dirs() {
		h := u.At.Step(d)
		if !b.Field.InBounds(h) || !b.Field.At(h).Passable() || b.UnitAt(h) != nil {
			continue
		}
		if MoveCost(b.Field.At(h), u.Troop()) > u.Move {
			continue
		}
		if dist := Distance(h, goal); dist < bestD {
			best, bestD, found = d, dist, true
		}
	}
	return best, found
}

// scheme 是一個「用哪個計、打哪一格」的候選。
type scheme struct {
	what Stratagem
	at   Hex
}

// stratagemOptions 列出智力與錢都過得了的計謀，依費用由高而低。
//
// 天氣、地形、旁邊有沒有友軍這些條件**不在這裡判**：那是
// `UseStratagem` 的職責，重複一份就會有兩個真相。呼叫端一個一個試。
func (b *Battle) stratagemOptions(u *Unit) []scheme {
	wise := u.Smartest()
	if wise == nil {
		return nil
	}
	// **計謀只能對相鄰的格子用**（`L0`）：原版下計謀先問方向，
	// 目標一定是六個鄰格之一（`UseStratagem` 的註解）。這不是自動作戰
	// 的取捨，是規則。
	var near []*Unit
	for _, t := range b.enemies(u) {
		if Distance(u.At, t.At) == 1 {
			near = append(near, t)
		}
	}
	if len(near) == 0 {
		return nil
	}
	var out []scheme
	for _, s := range []Stratagem{Fire, Flood, Lure, Burn, Siege, Trap} {
		if int(wise.Intel) < s.MinIntel() || b.Gold[u.Side] < s.Cost() {
			continue
		}
		for _, t := range near {
			if !worthIt(b, s, t) {
				continue
			}
			// **必敗的計不要用**：判定是
			// `RND(Spread) + 目標謀略 < 施法者謀略`，亂數最小是 0，
			// 所以謀略不高過對方就一次都不會成功——而錢與行動力
			// 照樣賠進去。這條不是調校，是那個公式的直接推論。
			if x := t.Smartest(); x != nil && int(wise.Intel) <= int(x.Intel) {
				continue
			}
			out = append(out, scheme{s, t.At})
		}
	}
	return out
}

// worthIt 擋掉白花錢的計。
//
// 效果還掛著就再下一次，錢照扣、效果沒有變好——而且**貴的計會一直
// 排在便宜的前面**，於是陷阱一招就把整袋錢用完，燒糧與圍攻一次都輪不到。
//
// 誘敵不在這裡：它不掛效果，是當場結算的一次交戰（`0x2b6aa`）。
func worthIt(b *Battle, s Stratagem, t *Unit) bool {
	switch s {
	case Trap:
		return t.Trapped == 0
	case Burn:
		return b.Gold[t.Side] > 0 || b.Rice[t.Side] > 0
	}
	return true
}

// Auto 把整場戰役打完，回傳打了幾天。
//
// 行動順序照手冊 p.28：主守軍 → 助守軍 → 主攻軍 → 助攻軍，
// 各軍內部先鋒 → 左軍 → 右軍 → 中軍 → 後軍。
func (b *Battle) Auto() int {
	for !b.Over && b.Day <= BattleDays {
		for _, u := range b.Order() {
			if b.Over {
				break
			}
			b.AutoTurn(u)
		}
		if b.Over {
			break
		}
		b.EndDay()
	}
	if !b.Over {
		b.Over = true
		b.AttackerWon = b.CityHolder().Attacking()
	}
	return b.Day
}

// Captives 回傳被擒的將領（決勝時要處置，說明書 p.35）。
func (b *Battle) Captives() []Leader {
	var out []Leader
	for _, u := range b.Units {
		for _, l := range u.Leaders {
			if l.Captured {
				out = append(out, l)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

// Survivors 回傳某一方還活著的將領與他們剩下的兵。
func (b *Battle) Survivors(attacking bool) []Leader {
	var out []Leader
	for _, u := range b.Units {
		if u.Side.Attacking() != attacking || u.Retreated {
			continue
		}
		for _, l := range u.Leaders {
			if !l.Dead && !l.Captured {
				out = append(out, l)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}
