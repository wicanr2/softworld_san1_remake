package battle

import "fmt"

// 單挑與六種計謀（說明書 p.30–34）。

// Duel 是「單挑」：部隊將領叫陣單打獨鬥（說明書 p.30）。
//
// 「**依其戰力強弱分高下，與率領軍力大小無關**；若於單挑時體力降到 0
// 即告落敗；若拒絕挑戰，麾下士兵將有部份逃跑；單挑落敗可能被擒，
// 或死於刀下。」
//
// accept 為假表示對方拒絕挑戰。要照原版判斷用 DuelAccepted。
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
		// 「若拒絕挑戰，麾下士兵將有部份逃跑」——**跑的是拒絕那一方的**
		// （`0x30de6`，三條分支的 `si` 都指向被挑戰者）。
		div := RefuseDuelDivisor(int(ct.Intel), int(ct.War), int(ca.War),
			b.roll(RefuseIntelSpread), b.roll(RefuseWarSpread),
			b.roll(RefuseIntelSpread), b.roll(max(1, int(ct.War)/RefuseWarDiv)))
		lost := ct.Soldiers / div
		if lost > 0 {
			b.casualty(t, lost)
		}
		b.note("%s 拒絕 %s 的挑戰，逃散 %d 人", ct.Name, ca.Name, lost)
		return nil
	}
	// 依戰力分高下，與兵力無關。**體能就是血條**，降到 0 即落敗。
	rounds := DuelRounds(int(ca.War), int(ct.War), b.roll(duelRoundSpread(int(ca.War), int(ct.War))))
	for i := 0; i < rounds; i++ {
		blow := DuelBlow(int(ca.War), int(ct.War),
			b.roll(DuelBlowSpread), b.roll(DuelBlowSpread), b.roll(DuelBlowWide))
		if int(ct.Stamina) <= blow {
			ct.Stamina = 0
			b.defeatInDuel(t, ct, ca)
			return nil
		}
		ct.Stamina -= uint8(blow)

		blow = DuelBlow(int(ct.War), int(ca.War),
			b.roll(DuelBlowSpread), b.roll(DuelBlowSpread), b.roll(DuelBlowWide))
		if int(ca.Stamina) <= blow {
			ca.Stamina = 0
			b.defeatInDuel(a, ca, ct)
			return nil
		}
		ca.Stamina -= uint8(blow)
	}
	b.note("%s 與 %s 大戰百合，不分勝負", ca.Name, ct.Name)
	return nil
}

// 單挑的兩條公式（`0x310b6`／`0x31170`，`L0`、`[base]`）。
//
//	回合數 ＝ RND((甲戰力 + 乙戰力) ÷ 2) + 甲戰力 ÷ 7 + 乙戰力 ÷ 7
//	每回合 ＝ max(0, RND(5) − RND(5) − RND(6) + (我方戰力 − 對方戰力) − 4)
//
// 打掉的是對方的**體能**（人物 offset 8），歸零就落敗。
// **期望值是「戰力差 − 6.5」**：`RND(5) − RND(5)` 的平均是 0、
// `RND(6)` 的平均是 2.5，所以戰力沒有高過對方七點左右就傷不了人
// ——正是說明書說的「依其戰力強弱分高下」。
const (
	DuelBlowSpread  = 5 // 兩次 RND(5)
	DuelBlowWide    = 6 // 一次 RND(6)
	DuelBlowEdge    = 4 // 再扣掉的常數
	DuelRoundDiv    = 7 // 回合數裡兩人戰力各除的數
	DuelRoundHalf   = 2 // 亂數上限是兩人戰力和的一半
	DuelStaminaBar  = 46 // 畫面上體能條的上限（0x2e），不影響判定
)

// duelRoundSpread 是回合數那個亂數的上限。
func duelRoundSpread(warA, warB int) int { return (warA + warB) / DuelRoundHalf }

// DuelRounds 是一場單挑打幾回合。
func DuelRounds(warA, warB, roll int) int {
	return roll + warA/DuelRoundDiv + warB/DuelRoundDiv
}

// DuelBlow 是一回合打掉對方多少體能。
func DuelBlow(mine, theirs, roll5a, roll5b, roll6 int) int {
	n := roll5a + (mine - theirs) - roll5b - roll6 - DuelBlowEdge
	if n < 0 {
		return 0
	}
	return n
}

// 接不接受單挑（`0x30c5b`–`0x30d5b`，`L0`、`[base]`）。
//
// **預設是拒絕**，被挑戰者由電腦控制時有三道機會翻成接受：
//
//	RND(10) + 被挑戰者的戰力 − 5 > 挑戰者的戰力                → 接受
//	被挑戰者的兵 ÷ 2 > 挑戰者的兵，且 RND(20) + 被挑戰者的戰力
//	                                 > 挑戰者的戰力            → 接受
//	被挑戰者的兵 ÷ 5 > 挑戰者的兵                              → 接受
//
// 也就是**強者才應戰**，兵力懸殊時更願意應戰。被挑戰者由玩家控制時
// 原版直接問「接受嗎(Y/N)」，不走這一套。
//
// 接受之後若被挑戰者的戰力 ≥ `RND(5) + 90`，原版會多印一句對白
// （`0x30d66`）——那只是台詞，不影響勝負。
const (
	DuelWarSpread   = 10 // 第一道的 RND 上限
	DuelWarEdge     = 5  // 第一道扣掉的常數
	DuelOddsSpread  = 20 // 第二道的 RND 上限
	DuelHalfTroops  = 2  // 第二道的兵力比
	DuelFifthTroops = 5  // 第三道的兵力比
)

// DuelAccepted 回報電腦控制的被挑戰者接不接受這場單挑。
func DuelAccepted(challengerWar, defenderWar, challengerTroops, defenderTroops, roll10, roll20 int) bool {
	if roll10+defenderWar-DuelWarEdge > challengerWar {
		return true
	}
	if defenderTroops/DuelHalfTroops > challengerTroops &&
		roll20+defenderWar > challengerWar {
		return true
	}
	return defenderTroops/DuelFifthTroops > challengerTroops
}

// 拒絕單挑之後跑掉多少兵（`0x30de6`–`0x30ef5`，`L0`、`[base]`）。
//
//	損失 ＝ 拒絕方的兵士數 ÷ 除數
//
// 除數分三條，**都用拒絕方（被挑戰者）自己的數字**：
//
//	謀略 > RND(10) + 80                → RND(10) − 戰力 ÷ 10 + 50   ; 約 2 %
//	否則 RND(5) + 戰力 < 挑戰者的戰力  → 25 − RND(戰力 ÷ 20)        ; 約 4–5 %
//	否則                               → 10 − RND(戰力 ÷ 20)        ; 約 10–14 %
//
// **除數越小掉得越多**，所以順序是：謀士拒絕損失最小，
// 明顯打不過而拒絕次之，**旗鼓相當卻拒絕的損失最重**——
// 那是怯戰，軍心散得最快。
const (
	RefuseIntelFloor  = 80
	RefuseIntelSpread = 10
	RefuseWarSpread   = 5
	RefuseWarDiv      = 20
	RefuseBaseWise    = 50
	RefuseBaseWeak    = 25
	RefuseBaseEven    = 10
	RefuseWarShare    = 10
)

// RefuseDuelDivisor 是拒絕單挑時兵士數要除的數。
func RefuseDuelDivisor(intel, war, challengerWar, roll10, roll5, roll10b, rollWar int) int {
	div := 0
	switch {
	case intel > roll10+RefuseIntelFloor:
		div = roll10b - war/RefuseWarShare + RefuseBaseWise
	case roll5+war < challengerWar:
		div = RefuseBaseWeak - rollWar
	default:
		div = RefuseBaseEven - rollWar
	}
	if div < 1 {
		div = 1
	}
	return div
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

// Spread 是成功判定的亂數上限（原版 `DS:0x81e0`，`L0`）。
//
// 判定是（`0x2ac4d`）
//
//	RND(Spread) + 目標領隊的謀略 < 施法者領隊的謀略 → 成功
//
// 所以上限愈大愈難：火攻要贏過對方 10 點以內的亂數，陷阱與誘敵
// 只要 2 點。**說明書完全沒提這一關**——它只寫了智力門檻與費用，
// 過了那兩關看起來就一定成功。
func (s Stratagem) Spread() int {
	switch s {
	case Fire:
		return 10
	case Flood:
		return 8
	case Trap, Lure:
		return 2
	case Burn:
		return 4
	case Siege:
		return 6
	}
	return 0
}

// StratagemSucceeds 是計謀成不成功。
func StratagemSucceeds(casterIntel, targetIntel, roll int) bool {
	return roll+targetIntel < casterIntel
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

// 火攻與水淹的地形殺傷基數（原版 `DS:0x8200`／`DS:0x8224`，`L0`）。
//
// **索引是 `地形碼 & 0x0D`，而表只有八格**（`0x2afee`、`0x2b35e`）。
// 那個遮罩把第 1 個位元抹掉，所以
//
//	山丘 2→0   大山 1／淺水 3→1   深水 4／關寨 6→4
//	城池 5／平原 7→5   樹林 8→8   沙漠 9→9
//
// 8 與 9 已經超出表尾，讀到的是後面那串字（`SCG03.IMG`）當成數字——
// 火攻讀到 17235、水淹讀到 17235 與 12615。**這不影響玩起來的結果**：
// 比率之後被 0.9 夾住（`0x2b061`），所以樹林與沙漠一律吃滿九成，
// 正是說明書寫的「樹林殺傷力最強」。這裡照原版的行為列表，
// 越界的兩格直接寫成上限。
var fireBase = [10]int{0: 25, 1: 15, 4: 20, 5: 55, 8: StratagemMaxLoss, 9: StratagemMaxLoss}

var floodBase = [10]int{0: 20, 1: 2, 4: 30, 5: 35, 8: StratagemMaxLoss, 9: StratagemMaxLoss}

const (
	// StratagemMaxLoss 是火攻與水淹的殺傷上限（原版 `DS:0xa9be` ＝ 0.9）。
	StratagemMaxLoss = 90
	// StratagemRollSpread 是加在地形基數上的亂數寬度（`0x2aff8`）。
	StratagemRollSpread = 10
	// StratagemGeniusIntel 起跳的領隊讓殺傷乘上 StratagemGeniusBonus％
	// （`0x2b047`：`謀略 >= 0x62`，`DS:0xa9b6` ＝ 1.6）。
	StratagemGeniusIntel = 98
	StratagemGeniusBonus = 160
)

// terrainSlot 是原版查地形殺傷表用的索引：`地形碼 & 0x0D`。
func terrainSlot(t Terrain) int { return int(terrainCode[t]) & 0x0D }

// FireLoss／FloodLoss 是火攻與水淹的殺傷百分比。
//
//	比率 ＝ min(90, (基數 + RND(10)) × (領隊謀略 ≥ 98 ? 1.6 : 1))
func FireLoss(t Terrain, casterIntel, roll int) int {
	return stratagemLoss(fireBase[terrainSlot(t)], casterIntel, roll)
}

func FloodLoss(t Terrain, casterIntel, roll int) int {
	return stratagemLoss(floodBase[terrainSlot(t)], casterIntel, roll)
}

func stratagemLoss(base, casterIntel, roll int) int {
	pct := base + roll
	if casterIntel >= StratagemGeniusIntel {
		pct = pct * StratagemGeniusBonus / 100
	}
	if pct > StratagemMaxLoss {
		pct = StratagemMaxLoss
	}
	return pct
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

	// 成功判定（`0x2ac4d`）。**錢與行動力先扣**——原版也是先付再賭，
	// 失敗一樣花掉。
	caster, victim := u.Smartest(), t.Smartest()
	ci, vi := 0, 0
	if caster != nil {
		ci = int(caster.Intel)
	}
	if victim != nil {
		vi = int(victim.Intel)
	}
	if !StratagemSucceeds(ci, vi, int(b.rng.next()%uint32(s.Spread()))) {
		b.note("%s 對 %s 用%s，被識破了", u.Name(), t.Name(), s)
		b.checkOver()
		return nil
	}

	switch s {
	case Fire:
		pct := FireLoss(terrain, ci, b.roll(StratagemRollSpread))
		loss := b.scorch(t, pct)
		b.note("%s 對 %s 火攻，折損 %d（%s，%d%%）",
			u.Name(), t.Name(), loss, terrain, pct)
	case Flood:
		pct := FloodLoss(terrain, ci, b.roll(StratagemRollSpread))
		loss := b.scorch(t, pct)
		b.note("%s 對 %s 水淹，折損 %d（%s，%d%%）",
			u.Name(), t.Name(), loss, terrain, pct)
	case Lure:
		t.Enraged = TuneLureDays
		b.note("%s 誘敵成功，%s 怒火攻心", u.Name(), t.Name())
	case Trap:
		t.Trapped = TrapDays(ci, b.roll(TrapSpread), b.roll(TrapSpread))
		b.note("%s 設陷阱困住 %s，%d 日內無法活動",
			u.Name(), t.Name(), t.Trapped)
	case Burn:
		side := t.Side
		keep := BurnKeep(ci, b.roll(BurnGeniusSpread), b.roll(BurnSpread))
		b.Gold[side] = b.Gold[side] * keep / 100
		b.Rice[side] = b.Rice[side] * keep / 100
		b.note("%s 燒了 %s 的補給，只剩 %d%%", u.Name(), side, keep)
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

// roll 是 `RND(n)`：0..n−1。
func (b *Battle) roll(n int) int {
	if n <= 0 {
		return 0
	}
	return int(b.rng.next() % uint32(n))
}

// scorch 把火攻或水淹的比率**逐將領**套上去，回傳總損失。
//
// 原版是一位一位算的（`0x2b102`）：`新兵 ＝ 兵 × (1 − 比率)`，
// 算出來不大於零就把那位從部隊裡除名（將領欄寫回 `0xFFFF`），
// 再擲一次 `RND(100)`——**大於 20 就燒死**（`0x2b138`），
// 否則只是離隊。
func (b *Battle) scorch(u *Unit, pct int) int {
	before := u.Soldiers()
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if x.Dead || x.Captured || x.Soldiers <= 0 {
			continue
		}
		x.Soldiers = x.Soldiers * (100 - pct) / 100
		if x.Soldiers > 0 {
			continue
		}
		x.Soldiers = 0
		if b.roll(100) > StratagemDeathRoll {
			x.Dead = true
			b.note("%s 被燒死", x.Name)
			continue
		}
		x.Captured = true
	}
	return before - u.Soldiers()
}

// StratagemDeathRoll 是火攻／水淹把人打光之後的生死判定門檻
// （`0x2b138`：`RND(100) > 20` 就死）。
const StratagemDeathRoll = 20

// 陷阱困住的天數（`0x2b648`，`L0`）：
//
//	天數 ＝ RND(5) + 1，領隊謀略 ≥ 98 再加 RND(5) + 2
//
// **說明書 p.33 寫「九日」，碼裡不是**——一般是 1–5 日，
// 謀略 98 起跳才可能到 11 日。以碼為準。
const TrapSpread = 5

func TrapDays(casterIntel, roll, bonusRoll int) int {
	days := roll + 1
	if casterIntel >= StratagemGeniusIntel {
		days += bonusRoll + 2
	}
	return days
}

// 燒糧留下的比例（`0x2ba6b`，`L0`）：
//
//	領隊謀略 ≥ 98：1 ÷ (RND(2) + 4)      → 剩 20–25%
//	否則        ：1 − 1 ÷ (RND(3) + 2)   → 剩 50–75%
//
// 金與米各乘一次，**同一個比例**。
const (
	BurnGeniusSpread = 2
	BurnSpread       = 3
)

func BurnKeep(casterIntel, geniusRoll, roll int) int {
	if casterIntel >= StratagemGeniusIntel {
		return 100 / (geniusRoll + 4)
	}
	return 100 - 100/(roll+2)
}

