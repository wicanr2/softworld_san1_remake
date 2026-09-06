package battle

import "fmt"

// 單挑與六種計謀（說明書 p.30–34）。

// Duel 是「單挑」：部隊將領叫陣單打獨鬥（說明書 p.30）。
//
// 「**依其戰力強弱分高下，與率領軍力大小無關**；若於單挑時體力降到 0
// 即告落敗；若拒絕挑戰，麾下士兵將有部份逃跑；單挑落敗可能被擒，
// 或死於刀下。」
//
// accept 為假表示對方拒絕挑戰。
func (b *Battle) Duel(a *Unit, d Dir, accept bool) error {
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
	ca, ct := a.Chief(), t.Chief()
	if ca == nil || ct == nil {
		return fmt.Errorf("battle: 有一方沒有領隊")
	}
	a.Move = 0
	if !accept {
		// 「若拒絕挑戰，麾下士兵將有部份逃跑」。
		b.casualty(t, t.Soldiers()*TuneRefuseDuelLoss/100)
		b.note("%s 拒絕 %s 的挑戰，士兵逃散", ct.Name, ca.Name)
		return nil
	}
	// 依戰力分高下，與兵力無關。體能降到 0 即落敗。
	for round := 0; round < 100; round++ {
		dmg := TuneDuelDamage * (100 + int(ca.War) - int(ct.War)) / 100
		if dmg < 1 {
			dmg = 1
		}
		if int(ct.Stamina) <= dmg {
			ct.Stamina = 0
			b.defeatInDuel(t, ct, ca)
			return nil
		}
		ct.Stamina -= uint8(dmg)

		dmg = TuneDuelDamage * (100 + int(ct.War) - int(ca.War)) / 100
		if dmg < 1 {
			dmg = 1
		}
		if int(ca.Stamina) <= dmg {
			ca.Stamina = 0
			b.defeatInDuel(a, ca, ct)
			return nil
		}
		ca.Stamina -= uint8(dmg)
	}
	b.note("%s 與 %s 大戰百合，不分勝負", ca.Name, ct.Name)
	return nil
}

// defeatInDuel 處理單挑落敗：可能被擒，或死於刀下（說明書 p.30）。
func (b *Battle) defeatInDuel(u *Unit, loser, winner *Leader) {
	if int(b.rng.next()%100) < TuneCaptureOnDuel {
		loser.Captured = true
		b.note("%s 單挑不敵 %s，被擒", loser.Name, winner.Name)
	} else {
		loser.Dead = true
		b.note("%s 單挑不敵 %s，死於刀下", loser.Name, winner.Name)
	}
	// 「如果雙方領隊之一被擒或死亡，這場對戰便告一段落」。
	if u.Soldiers() == 0 {
		u.Wiped = true
	}
	b.checkOver()
}

// Stratagem 是六種計謀（說明書 p.32–34）。
//
// ⚠ **編號以原版執行檔為準，不是手冊。** 原版的策略選單寫的是
// 「1.火攻 2.水洽 3.陷阱／4.誘敵 5.燒糧 6.圍攻」
// （`AA.EXE` 位移 `0x46df3`，`docs/re/04` §4），手冊把誘敵排第 3、
// 陷阱排第 4。程式是實際跑的東西，手冊是二手轉錄。
// 門檻與費用兩邊一致，只有第 3、4 兩項的次序不同。
type Stratagem int

const (
	Fire  Stratagem = iota + 1 // 1 火攻
	Flood                      // 2 水淹（原版的選單寫成「水洽」）
	Trap                       // 3 陷阱
	Lure                       // 4 誘敵
	Burn                       // 5 燒糧
	Siege                      // 6 圍攻
)

func (s Stratagem) String() string {
	switch s {
	case Fire:
		return "火攻"
	case Flood:
		return "水淹"
	case Lure:
		return "誘敵"
	case Trap:
		return "陷阱"
	case Burn:
		return "燒糧"
	case Siege:
		return "圍攻"
	}
	return "?"
}

// MinIntel 是用這個計謀需要的領隊智力（說明書 p.32–34）。
func (s Stratagem) MinIntel() int {
	switch s {
	case Fire:
		return 80
	case Flood:
		return 75
	case Lure, Trap:
		return 60
	case Burn:
		return 70
	case Siege:
		return 65
	}
	return 0
}

// Cost 是用這個計謀要花多少金（說明書 p.32–34）。
func (s Stratagem) Cost() int {
	switch s {
	case Fire:
		return 600
	case Flood:
		return 500
	case Lure:
		return 400
	case Burn:
		return 300
	case Siege:
		return 200
	case Trap:
		return 100
	}
	return 0
}

// fireDamage 是火攻在各地形的殺傷百分比（說明書 p.32–33）。
//
// 「水上損失不大；山丘、關寨、城池損失普通；平原、沙漠傷害較大；
// **樹林殺傷力最強**。」
func fireDamage(t Terrain) int {
	switch {
	case t.Water():
		return TuneFireBase * 30 / 100
	case t == Hill || t == Fort || t == City:
		return TuneFireBase * 70 / 100
	case t == Forest:
		return TuneFireBase * 150 / 100
	}
	return TuneFireBase
}

// floodDamage 是水淹在各地形的殺傷百分比（說明書 p.33）。
//
// 「水上損失不大；山上、關寨損失普通；平原、沙漠、城池傷害較大；
// **樹林傷害力最強**。」
func floodDamage(t Terrain) int {
	switch {
	case t.Water():
		return TuneFloodBase * 30 / 100
	case t == Hill || t == Fort:
		return TuneFloodBase * 70 / 100
	case t == Forest:
		return TuneFloodBase * 150 / 100
	}
	return TuneFloodBase
}

// UseStratagem 施行一個計謀。
//
// 每一種的門檻、費用與限制都照手冊：
//
//   - 火攻：智力 ≥ 80、600 金、**刮風時節才能使用**
//   - 水淹：智力 ≥ 75、500 金、目標必須在**水上或岸邊**、**下雨天**
//   - 誘敵：智力 ≥ 60、400 金，來犯敵軍攻擊力暫時下降
//   - 陷阱：智力 ≥ 60、100 金，不得用在水上、城池或關寨中，困住九日
//   - 燒糧：智力 ≥ 70、300 金，下雨天無法使用，也不能用於水上的敵軍
//   - 圍攻：智力 ≥ 65、200 金，目標旁邊必須尚有其他友軍
func (b *Battle) UseStratagem(u *Unit, s Stratagem, target Hex) error {
	if err := b.canAct(u); err != nil {
		return err
	}
	wise := u.Smartest()
	if wise == nil || int(wise.Intel) < s.MinIntel() {
		return fmt.Errorf("battle: %s 要領隊智力不小於 %d", s, s.MinIntel())
	}
	// 「沒有帶錢就無法用計」（說明書 p.28）。
	if b.Gold[u.Side] < s.Cost() {
		return fmt.Errorf("battle: %s 要 %d 金，隨軍只有 %d", s, s.Cost(), b.Gold[u.Side])
	}
	t := b.UnitAt(target)
	if t == nil {
		return fmt.Errorf("battle: 那裡沒有部隊")
	}
	if t.Side.Attacking() == u.Side.Attacking() {
		return fmt.Errorf("battle: 那是友軍")
	}
	terrain := b.Field.At(target)

	switch s {
	case Fire:
		if b.Weather != Windy {
			return fmt.Errorf("battle: 火攻要刮風時節才能使用")
		}
	case Flood:
		if b.Weather != Rainy {
			return fmt.Errorf("battle: 水淹要下雨天才能用")
		}
		if !terrain.Water() && !b.nextToWater(target) {
			return fmt.Errorf("battle: 水淹的目標必須在水上或岸邊")
		}
	case Burn:
		if b.Weather == Rainy {
			return fmt.Errorf("battle: 下雨天無法燒糧")
		}
		if terrain.Water() {
			return fmt.Errorf("battle: 不能對水上的敵軍燒糧")
		}
	case Trap:
		if terrain.Water() || terrain == City || terrain == Fort {
			return fmt.Errorf("battle: 陷阱不得用在水上、城池或關寨中")
		}
	case Siege:
		if b.alliesAround(u, target) == 0 {
			return fmt.Errorf("battle: 圍攻要目標旁邊尚有其他友軍")
		}
	}

	b.Gold[u.Side] -= s.Cost()
	u.Move = 0

	switch s {
	case Fire:
		loss := t.Soldiers() * fireDamage(terrain) / 100
		b.casualty(t, loss)
		b.note("%s 對 %s 火攻，折損 %d（%s）", u.Name(), t.Name(), loss, terrain)
	case Flood:
		loss := t.Soldiers() * floodDamage(terrain) / 100
		b.casualty(t, loss)
		b.note("%s 對 %s 水淹，折損 %d（%s）", u.Name(), t.Name(), loss, terrain)
	case Lure:
		t.Enraged = 3
		b.note("%s 誘敵成功，%s 怒火攻心", u.Name(), t.Name())
	case Trap:
		t.Trapped = TuneTrapDays
		b.note("%s 設陷阱困住 %s，九日內無法活動", u.Name(), t.Name())
	case Burn:
		side := t.Side
		b.Gold[side] = b.Gold[side] * (100 - TuneBurnLoss) / 100
		b.Rice[side] = b.Rice[side] * (100 - TuneBurnLoss) / 100
		b.note("%s 燒了 %s 的補給", u.Name(), side)
	case Siege:
		n := b.alliesAround(u, target)
		loss := b.hit(u, t, 100+n*TuneSiegeBonus)
		b.note("%s 聯合 %d 支友軍圍攻 %s，折損 %d", u.Name(), n, t.Name(), loss)
	}
	b.checkOver()
	return nil
}

// nextToWater 回報一格是不是岸邊。
func (b *Battle) nextToWater(h Hex) bool {
	for _, d := range Dirs() {
		if b.Field.At(h.Step(d)).Water() {
			return true
		}
	}
	return false
}

// alliesAround 數目標旁邊有幾支與 u 同立場的部隊（不含 u 自己）。
func (b *Battle) alliesAround(u *Unit, target Hex) int {
	n := 0
	for _, d := range Dirs() {
		x := b.UnitAt(target.Step(d))
		if x != nil && x != u && x.Side.Attacking() == u.Side.Attacking() {
			n++
		}
	}
	return n
}
