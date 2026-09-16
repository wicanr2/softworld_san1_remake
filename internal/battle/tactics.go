package battle

import (
	"fmt"
	"math/big"
)

// 單挑與六種計謀（說明書 p.30–34）。

// Duel 是領隊對領隊的單挑：兩支部隊的領隊叫陣單打獨鬥（說明書 p.30）。
//
// 「**依其戰力強弱分高下，與率領軍力大小無關**；若於單挑時體力降到 0
// 即告落敗；若拒絕挑戰，麾下士兵將有部份逃跑；單挑落敗可能被擒，
// 或死於刀下。」
//
// ⚠ **這是 remake 的 `enhanced` 模式用的**（`auto.go`）。原版的主戰場
// 沒有這一步——單挑只在對戰子畫面裡、一位將領對一位將領
// （`Skirmish`，`docs/re/05` §9／§10）。accept 為假表示對方拒絕挑戰。
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
	loser, _ := b.duelLeaders(ca, ct, accept, nil)
	if loser != nil {
		u := a
		if loser == ct {
			u = t
		}
		// 「如果雙方領隊之一被擒或死亡，這場對戰便告一段落」。
		b.wipeCheck(u)
		b.checkOver()
	}
	return nil
}

// duelLeaders 是一場單挑的本體（`0x30a1e`，`docs/re/05` §9）：ca 叫陣、
// ct 應戰或拒絕。回落敗的那一位（平手或拒絕是 nil）與勝方。落敗被擒時
// 先叫 seized（對戰子畫面用它把人記進捕獲方的名單），戰死的當場標記。
func (b *Battle) duelLeaders(ca, ct *Leader, accept bool, seized func(loser *Leader)) (loser, winner *Leader) {
	if !accept {
		// 「若拒絕挑戰，麾下士兵將有部份逃跑」——**跑的是拒絕那一方的**
		// （`0x30de6`，三條分支的 `si` 都指向被挑戰者的人物記錄）。
		div := RefuseDuelDivisor(int(ct.Intel), int(ct.War), int(ca.War),
			b.roll(RefuseIntelSpread), b.roll(RefuseWarSpread),
			b.roll(RefuseIntelSpread), b.roll(max(1, int(ct.War)/RefuseWarDiv)))
		lost := ct.Soldiers / div
		if lost > 0 {
			ct.Soldiers -= lost
		}
		b.note("blog.refuse", pn(ct.Name), pn(ca.Name), lost)
		return nil, nil
	}
	// 依戰力分高下，與兵力無關。**體能就是血條**，降到 0 即落敗。
	rounds := DuelRounds(int(ca.War), int(ct.War), b.roll(duelRoundSpread(int(ca.War), int(ct.War))))
	for i := 0; i < rounds; i++ {
		blow := DuelBlow(int(ca.War), int(ct.War),
			b.roll(DuelBlowSpread), b.roll(DuelBlowSpread), b.roll(DuelBlowWide))
		if int(ct.Stamina) <= blow {
			ct.Stamina = 0
			b.defeatInDuel(ct, ca, seized)
			return ct, ca
		}
		ct.Stamina -= uint8(blow)

		blow = DuelBlow(int(ct.War), int(ca.War),
			b.roll(DuelBlowSpread), b.roll(DuelBlowSpread), b.roll(DuelBlowWide))
		if int(ca.Stamina) <= blow {
			ca.Stamina = 0
			b.defeatInDuel(ca, ct, seized)
			return ca, ct
		}
		ca.Stamina -= uint8(blow)
	}
	b.note("blog.draw", pn(ca.Name), pn(ct.Name))
	return nil, nil
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
	DuelBlowSpread = 5  // 兩次 RND(5)
	DuelBlowWide   = 6  // 一次 RND(6)
	DuelBlowEdge   = 4  // 再扣掉的常數
	DuelRoundDiv   = 7  // 回合數裡兩人戰力各除的數
	DuelRoundHalf  = 2  // 亂數上限是兩人戰力和的一半
	DuelStaminaBar = 46 // 畫面上體能條的上限（0x2e），不影響判定
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

// DuelDeathRoll 是單挑落敗的處置：`RND(7)`，**只有 0 才死**
// （`0x31b04`–`0x31b16`，`L0`）。
//
// 也就是被擒 6/7 ≈ 86%、死於刀下 1/7 ≈ 14%——說明書 p.30 只說
// 「可能被擒，或死於刀下」，沒給比例。原版擲 0 的那一支印 `SCG27.IMG`
// 與「死在%s的刀下」；非 0 的那一支印 `SCG28.IMG`，並把敗者的槽號寫進
// 勝方的俘虜欄（`es:[0x1732 + (軍力×10 + 將領)×2]`）。
const DuelDeathRoll = 7

// DuelKills 回報這一擲要不要當場斬殺。roll 是 `RND(DuelDeathRoll)`。
func DuelKills(roll int) bool { return roll == 0 }

// defeatInDuel 處理單挑落敗：可能被擒，或死於刀下（說明書 p.30）。
func (b *Battle) defeatInDuel(loser, winner *Leader, seized func(*Leader)) {
	if !DuelKills(b.roll(DuelDeathRoll)) {
		loser.Captured = true
		if seized != nil {
			seized(loser)
		}
		b.note("blog.duelCaptured", pn(loser.Name), pn(winner.Name))
	} else {
		loser.Dead = true
		b.note("blog.duelKilled", pn(loser.Name), pn(winner.Name))
	}
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
	// StratagemGeniusIntel 起跳的領隊讓殺傷乘上 StratagemGeniusFactor
	// （`0x2b047`：`謀略 >= 0x62`，`DS:0xa9b6` ＝ 1.6）。
	StratagemGeniusIntel = 98
)

// terrainSlot 是原版查地形殺傷表用的索引：`地形碼 & 0x0D`。
func terrainSlot(t Terrain) int { return int(terrainCode[t]) & 0x0D }

// FireRatio／FloodRatio 是火攻與水淹的殺傷比率（`0x2aff4`–`0x2b081`，`L0`）：
//
//	比率 ＝ (基數 + RND(10)) × 0.01          ; double
//	領隊謀略 ≥ 98 → 比率 × 1.6               ; DS:0xa9b6
//	比率 ≥ 0.9 → 0.9                         ; DS:0xa9be
//
// **是 double 不是百分比整數**：謀略 98 起跳那一乘會留下小數
// （28% × 1.6 ＝ 44.8%），截成整數會讓一千五的部隊多留 12 個人。
// 乘法照 x87 的寬度算再存成 double（`fmull` ＋ `fstpl`）。
func FireRatio(t Terrain, casterIntel, roll int) float64 {
	return stratagemRatio(fireBase[terrainSlot(t)], casterIntel, roll)
}

func FloodRatio(t Terrain, casterIntel, roll int) float64 {
	return stratagemRatio(floodBase[terrainSlot(t)], casterIntel, roll)
}

func stratagemRatio(base, casterIntel, roll int) float64 {
	r := x87mul(float64(base+roll), StratagemPercent)
	if casterIntel >= StratagemGeniusIntel {
		r = x87mul(r, StratagemGeniusFactor)
	}
	if r >= StratagemMaxRatio {
		r = StratagemMaxRatio
	}
	return r
}

// x87mul 是兩個 double 在 x87 上相乘再存回 double：先在 64 位元尾數
// 算，再捨到 53 位元。
func x87mul(a, b float64) float64 {
	f := new(big.Float).SetPrec(64).SetFloat64(a)
	f.Mul(f, new(big.Float).SetPrec(64).SetFloat64(b))
	v, _ := f.Float64()
	return v
}

// FireLoss／FloodLoss 是同一個比率化成百分比，給戰報印。
func FireLoss(t Terrain, casterIntel, roll int) int {
	return int(FireRatio(t, casterIntel, roll) * 100)
}

func FloodLoss(t Terrain, casterIntel, roll int) int {
	return int(FloodRatio(t, casterIntel, roll) * 100)
}

// 火攻與水淹比率的三個 double 常數（`DS:0xa9ae`、`DS:0xa9b6`、`DS:0xa9be`）。
const (
	StratagemPercent      = 0.01
	StratagemGeniusFactor = 1.6
	StratagemMaxRatio     = 0.9
)

// UseStratagem 施行一個計謀。
//
// 門檻與費用照原版的兩張表（`DS:0x7f6e`／`DS:0x7f62`，與手冊 p.32–34
// 逐格相同）；位置與天候那一道門照 `0x2bee8` 的六支檢查常式
// （`docs/re/05` §4.0，`L0`）：
//
//   - 火攻：智力 ≥ 80、600 金，**刮風**
//   - 水淹：智力 ≥ 75、500 金，**下雨**，而且目標的六個鄰格裡有一格**淺水**
//   - 陷阱：智力 ≥ 60、100 金，目標格不是淺水、深水、城池或關寨
//   - 誘敵：智力 ≥ 60、400 金，**沒有位置與天候的限制**
//   - 燒糧：智力 ≥ 70、300 金，目標格不是淺水也不是深水，而且不是下雨天
//   - 圍攻：智力 ≥ 65、200 金，目標的六個鄰格裡我方要有**兩支以上**
//     （含施法者，所以 `alliesAround` 的門檻是 1）
//
// 手冊那一句「水淹的目標必須在水上或岸邊」比碼寬，見 `CONTEXT.md` R40。
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
	// **目標只能是相鄰的六格**（`L0`）：原版下計謀時**先問方向**
	// （`0x28af5` 讀 `1`–`6`），`0x28d7a` 拿那個索引查
	// `DS:0x7c6a`（欄位移）與 `DS:0x7c82`（列位移）算出目標格
	// ——兩張各 12 格，`(欄 % 2) × 6 + 方向`——那一格沒有敵人就直接
	// 取消。所以計謀根本沒有「隔空指定一格」這回事。
	if Distance(u.At, target) != 1 {
		return fmt.Errorf("battle: 計謀只能對相鄰的格子用")
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
		// **岸邊的判準是「六個鄰格裡有一格淺水」**（`L0`、`0x2bfa2`）：
		// 原版掃目標的六個方向，只認地形碼 3（淺水），深水不算，
		// 目標自己站在水上也不算。
		if !b.nextToShallow(target) {
			return fmt.Errorf("battle: 水淹的目標旁邊要有淺水")
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
		// **目標周圍我方的部隊要有兩支以上**（`L0`、`0x2c140`）：原版數
		// 目標的六個鄰格，佔位圖（`es:[0x2532]`）不是 `0xFFFF` 而且
		// 陣營與目標不同的就記一筆，**施法者自己也在裡面**，
		// 最後要 `> 1`。這裡的 `alliesAround` 不含施法者，所以門檻是 1。
		if b.alliesAround(u, target) < 1 {
			return fmt.Errorf("battle: 圍攻要目標旁邊還有另一支我方部隊")
		}
	}

	// 費用成敗都扣（`0x2ada2`，判定與效果之後才扣，結果一樣）。
	// **移動力不歸零**：玩家與電腦的計謀常式都不碰 offset 36
	// （`0x28acc`／`0x29784`）；玩家那一邊是回合就此結束，回填時
	// 休息攢的 2 留得住。
	b.Gold[u.Side] -= s.Cost()

	// 成功判定（`0x2ac4d`）。
	caster, victim := u.Smartest(), t.Smartest()
	ci, vi := 0, 0
	if caster != nil {
		ci = int(caster.Intel)
	}
	if victim != nil {
		vi = int(victim.Intel)
	}
	//
	// 骰序（`docs/re/05` §12.2，`L0`）：判定那一擲 `RND(表)` 在 `0x2abfa`；
	// 不成就印一句「被看穿」（`0x2ad72` → `RND(8)`）。成了進各計謀的常式，
	// 每一支先印自己那一句對白，火攻、水淹、燒糧接著播特效（`RND(4)`），
	// 再擲效果的骰。
	if !StratagemSucceeds(ci, vi, b.roll(s.Spread())) {
		b.msg()
		b.note("blog.seen", u.Name(), t.Name(), s.Label())
		b.checkOver()
		return nil
	}

	switch s {
	case Fire:
		b.msg()
		b.fx()
		r := FireRatio(terrain, ci, b.roll(StratagemRollSpread))
		loss := b.scorch(u.Side, t, r)
		b.note("blog.fire", u.Name(), t.Name(), loss, terrain.Label(), int(r*100))
	case Flood:
		b.msg()
		b.fx()
		r := FloodRatio(terrain, ci, b.roll(StratagemRollSpread))
		loss := b.scorch(u.Side, t, r)
		b.note("blog.flood", u.Name(), t.Name(), loss, terrain.Label(), int(r*100))
	case Lure:
		// **誘敵是把敵人引過來打你。** 原版 `0x2b6aa` 播完動畫之後叫共同
		// 的交戰結算，而且推參數時把攻守對調（`0x2b877` 先推目標再推
		// 施法者），所以出手的是目標。倍率傳 8（謀略 ≥ 98 時 9），
		// 兩個都被 `0x2a2c9` 夾成 1 ＝ 100%。
		//
		// 划不划算看的是「目標的攻擊力 vs 施法者的防禦力」——引一支弱的
		// 部隊來撞自己的硬點才是這一招的用法。
		b.msg()
		lost, back := b.exchange(t, u, LureStrike)
		b.note("blog.lure", u.Name(), t.Name(), u.Name(), back, t.Name(), lost)
	case Trap:
		// 第二擲只有領隊謀略 ≥ 98 才有（`0x2b683`–`0x2b68f`）。
		b.msg()
		bonus := 0
		days := b.roll(TrapSpread)
		if ci >= StratagemGeniusIntel {
			bonus = b.roll(TrapSpread)
		}
		t.Trapped = TrapDays(ci, days, bonus)
		b.note("blog.trap", u.Name(), t.Name(), t.Trapped)
	case Burn:
		// 兩條路各擲一次，不是兩擲都擲（`0x2ba6b`–`0x2ba9b`）。
		b.msg()
		b.fx()
		side := t.Side
		genius, plain := 0, 0
		if ci >= StratagemGeniusIntel {
			genius = b.roll(BurnGeniusSpread)
		} else {
			plain = b.roll(BurnSpread)
		}
		keep := BurnKeep(ci, genius, plain)
		b.Gold[side] = b.Gold[side] * keep / 100
		b.Rice[side] = b.Rice[side] * keep / 100
		b.note("blog.burn", u.Name(), side.Label(), keep)
	case Siege:
		// 原版 `0x2bd49`–`0x2bdfb`：掃目標的六個鄰格，格內有部隊、
		// **與目標不同陣營**、而且目標的將領人數還大於 0，就各對目標
		// 打一次交戰。
		//
		// **判準是「與目標不同陣營」，不是「不是施法者」**，所以貼著
		// 目標的施法者自己也會打一次。打光就停——原版每一輪都重查
		// 目標的將領人數。
		// **倍率格不是常數**（`L0`，`0x2bc95`–`0x2bd44`）：從 0 起算，
		// 目標六個鄰格裡每有一支與目標不同陣營的部隊就加一
		// （`0x2bd04`），施法者的領隊謀略到 98 再加一（`0x2bd41`，
		// `[bp+6]`／`[bp+8]` 是施法者）。所以**圍的人愈多打得愈重**，
		// 名副其實。
		mode := 0
		for _, d := range Dirs() {
			x := b.UnitAt(target.Step(d))
			if x != nil && x.Alive() && x.Side.Attacking() != t.Side.Attacking() {
				mode++
			}
		}
		if w := u.Smartest(); w != nil && int(w.Intel) >= StratagemGeniusIntel {
			mode++
		}
		b.msg()
		total, n := 0, 0
		for _, d := range Dirs() {
			x := b.UnitAt(target.Step(d))
			if x == nil || !x.Alive() || x.Side.Attacking() == t.Side.Attacking() {
				continue
			}
			if !t.Alive() {
				break
			}
			lost, _ := b.exchange(x, t, mode)
			total += lost
			n++
		}
		b.note("blog.siege", u.Name(), t.Name(), n, total)
	}
	b.checkOver()
	return nil
}

// nextToShallow 回報一格的六個鄰格裡有沒有淺水（`0x2bfa2` 的岸邊判準）。
func (b *Battle) nextToShallow(h Hex) bool {
	for _, d := range Dirs() {
		if b.Field.At(h.Step(d)) == Shallow {
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
//
// 原版的 `RND(n)` 在 `n <= 0` 時直接回 0 **而且不抽**（`0x10b0c`）。
func (b *Battle) roll(n int) int {
	if n <= 0 {
		return 0
	}
	if b.rollFn != nil {
		return b.rollFn(n)
	}
	return int(b.rng.next() % uint32(n))
}

// UseRoll 把這場戰役的每一次 `RND(n)` 交給 fn。
//
// 給對拍用：原版的骰序是 MSC 的 LCG（`game.MSCRand`），remake 平常用的
// 是 xorshift32；要與原版逐次相同就得從這裡供應原版的序列（或原版
// 實際擲出的值）。正式遊戲不用它。
func (b *Battle) UseRoll(fn func(n int) int) { b.rollFn = fn }

// scorch 把火攻或水淹的比率**逐將領**套上去，回傳總損失。
//
// 原版是一位一位、從第 0 槽往後算的（`0x2b082`–`0x2b152`）：
// `新兵 ＝ ftol(兵 × (1 − 比率))`，算出來不大於零就把那位從部隊裡除名
// （將領欄寫回 `0xFFFF`），再擲一次 `RND(100)`——**大於 20 就燒死**
// （`0x2b138`），否則交給施法方處置（`0x2b090` → `capture`）。
func (b *Battle) scorch(by Side, u *Unit, ratio float64) int {
	before := u.Soldiers()
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if !x.InUnit() {
			continue
		}
		x.Soldiers = ArrowSurvivors(x.Soldiers, ratio)
		if x.Soldiers > 0 {
			continue
		}
		if b.roll(100) > StratagemDeathRoll {
			x.Dead = true
			b.note("blog.burned", pn(x.Name))
			continue
		}
		x.Captured = true
		b.note("blog.captured", pn(x.Name))
		b.capture(by, u, x)
	}
	b.wipeCheck(u)
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
