package battle

import (
	"strings"
	"testing"
)

// TestFormUpSplitsEvenlyAndCaps 釘住「各戰鬥組的將領人數必須平均分配」
// 與「每組最多 10 名將領」（說明書 p.27）。
func TestFormUpSplitsEvenlyAndCaps(t *testing.T) {
	b := arena(flat(Plain))
	var pool []Leader
	for i := 0; i < 25; i++ {
		pool = append(pool, lead("將", uint8(i*4), 50, 100))
	}
	units := b.formUp(MainAttacker, pool, FromOffset(2, 2))
	if len(units) != int(formationCount) {
		t.Fatalf("分成 %d 隊，應該是 5 隊", len(units))
	}
	for _, u := range units {
		if len(u.Leaders) != 5 {
			t.Errorf("%s 有 %d 名將領，25 人分 5 隊應該每隊 5 名", u.Formation, len(u.Leaders))
		}
	}

	// 超過 50 人時每組封頂 10 名。
	b2 := arena(flat(Plain))
	pool = nil
	for i := 0; i < 70; i++ {
		pool = append(pool, lead("將", uint8(i%100), 50, 100))
	}
	for _, u := range b2.formUp(MainAttacker, pool, FromOffset(2, 2)) {
		if len(u.Leaders) > MaxLeaders {
			t.Errorf("%s 有 %d 名將領，超過上限 %d", u.Formation, len(u.Leaders), MaxLeaders)
		}
	}

	// 人少於五個就不開空隊伍——空隊伍在畫面上是一支不存在的軍。
	b3 := arena(flat(Plain))
	small := b3.formUp(MainAttacker, pool[:3], FromOffset(2, 2))
	if len(small) != 3 {
		t.Errorf("三個人分成 %d 隊，應該是 3 隊", len(small))
	}
	for _, u := range small {
		if len(u.Leaders) == 0 {
			t.Errorf("%s 是空的", u.Formation)
		}
	}
}

// TestFormUpDoesNotStack 釘住紮營不會把兩支部隊放在同一格。
func TestFormUpDoesNotStack(t *testing.T) {
	b := arena(flat(Plain))
	var pool []Leader
	for i := 0; i < 25; i++ {
		pool = append(pool, lead("將", uint8(i*4), 50, 100))
	}
	b.Units = b.formUp(MainAttacker, pool, FromOffset(5, 5))
	seen := map[Hex]bool{}
	for _, u := range b.Units {
		if seen[u.At] {
			t.Errorf("%s 與別人疊在 %v", u.Name(), u.At)
		}
		seen[u.At] = true
	}
}

// TestOrderFollowsManual 釘住行動順序：主守軍 → 助守軍 → 主攻軍 → 助攻軍，
// 各軍內部先鋒 → 左軍 → 右軍 → 中軍 → 後軍（說明書 p.28）。
func TestOrderFollowsManual(t *testing.T) {
	b := arena(flat(Plain))
	place(b, MainAttacker, Rear, FromOffset(1, 1), lead("攻後", 50, 50, 100))
	place(b, MainAttacker, Vanguard, FromOffset(2, 1), lead("攻先", 50, 50, 100))
	place(b, MainDefender, Centre, FromOffset(10, 10), lead("守中", 50, 50, 100))
	place(b, MainDefender, Vanguard, FromOffset(11, 10), lead("守先", 50, 50, 100))
	place(b, AidDefender, Left, FromOffset(12, 10), lead("助守左", 50, 50, 100))

	var names []string
	for _, u := range b.Order() {
		names = append(names, u.Chief().Name)
	}
	want := []string{"守先", "守中", "助守左", "攻先", "攻後"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("行動順序是 %v，應該是 %v", names, want)
	}
}

// TestRestAddsTwoMovePoints 釘住「每休息一次可增加移動力 2」（說明書 p.29、p.30）。
func TestRestAddsTwoMovePoints(t *testing.T) {
	b := arena(flat(Plain))
	u := place(b, MainAttacker, Centre, FromOffset(3, 3), lead("甲", 50, 50, 1000))
	u.Leaders[0].Stamina = 80
	before := u.Move
	if err := b.Rest(u); err != nil {
		t.Fatalf("休息失敗：%v", err)
	}
	if u.Move != before+2 {
		t.Errorf("休息後移動力 %d，應該是 %d（+2）", u.Move, before+2)
	}
	// 「並恢復將領的體力」（p.30）。
	if u.Leaders[0].Stamina <= 80 {
		t.Error("休息應該恢復體力")
	}
	if TuneRestMove != 2 {
		t.Errorf("TuneRestMove 是 %d，手冊寫的是 2", TuneRestMove)
	}
}

// TestMoveRules 釘住「大山無法穿越」「不得穿越其他軍隊」（說明書 p.29）
// 與移動力不足時走不動。
func TestMoveRules(t *testing.T) {
	f := flat(Plain)
	// 離城池遠一點：城池在正中央，而走上去要花 3 不是 2。
	here := FromOffset(2, 6)
	f.Set(here.Step(DirUp), Mountain)
	b := arena(f)
	u := place(b, MainAttacker, Centre, here, lead("甲", 50, 50, 1000))
	place(b, MainDefender, Centre, here.Step(DirDown), lead("乙", 50, 50, 1000))

	if err := b.Move(u, DirUp); err == nil {
		t.Error("走進大山應該失敗")
	}
	if err := b.Move(u, DirDown); err == nil {
		t.Error("穿越其他軍隊應該失敗")
	}
	// 走得動的方向要真的扣移動力。
	before := u.Move
	if err := b.Move(u, DirUpRight); err != nil {
		t.Fatalf("走平原失敗：%v", err)
	}
	if u.Move != before-MoveCost(Plain, u.Troop()) {
		t.Errorf("移動力剩 %d，應該扣掉 %d", u.Move, MoveCost(Plain, u.Troop()))
	}
	// 移動力歸零就走不動。
	u.Move = 0
	if err := b.Move(u, DirUpRight); err == nil {
		t.Error("移動力 0 還走得動")
	}
	// 走出邊界要擋下來。
	edge := place(b, MainAttacker, Rear, FromOffset(0, 4), lead("丙", 50, 50, 1000))
	if err := b.Move(edge, DirUpLeft); err == nil {
		t.Error("走出邊界應該失敗")
	}
}

// TestEnteringCityFlipsHolder 釘住進佔城池會換手（說明書 p.35 的勝負看城池）。
func TestEnteringCityFlipsHolder(t *testing.T) {
	f := flat(Plain)
	b := arena(f)
	start := f.CityAt.Step(DirUp)
	u := place(b, MainAttacker, Vanguard, start, lead("甲", 50, 50, 1000))
	if b.CityHeld != MainDefender {
		t.Fatal("開場城池應該在守方手上")
	}
	if err := b.Move(u, DirDown); err != nil {
		t.Fatalf("走進城池失敗：%v", err)
	}
	if b.CityHeld != MainAttacker {
		t.Errorf("城池仍在 %s 手上，應該換成主攻軍", b.CityHeld)
	}
}

// TestThirtyDayRule 釘住「守方能堅持抗戰滿卅天，且城池未被奪去就算衛郡成功」
// （說明書 p.35）。
func TestThirtyDayRule(t *testing.T) {
	b := arena(flat(Plain))
	place(b, MainAttacker, Centre, FromOffset(1, 1), lead("攻", 50, 50, 1000))
	place(b, MainDefender, Centre, FromOffset(19, 13), lead("守", 50, 50, 1000))
	b.Rice[MainAttacker] = 100000

	for i := 0; i < BattleDays && !b.Over; i++ {
		b.EndDay()
	}
	if !b.Over {
		t.Fatalf("打了 %d 天還沒結束", b.Day)
	}
	if b.AttackerWon {
		t.Error("三十天期滿而城池未失，應該是守方衛郡成功")
	}
	if b.Day != BattleDays+1 {
		t.Errorf("結束在第 %d 天，應該是滿 %d 天之後", b.Day, BattleDays)
	}

	// 攻方據有城池時，同樣的三十天判給攻方。
	b2 := arena(flat(Plain))
	place(b2, MainAttacker, Centre, FromOffset(1, 1), lead("攻", 50, 50, 1000))
	place(b2, MainDefender, Centre, FromOffset(19, 13), lead("守", 50, 50, 1000))
	b2.Rice[MainAttacker] = 100000
	b2.CityHeld = MainAttacker
	for i := 0; i < BattleDays && !b2.Over; i++ {
		b2.EndDay()
	}
	if !b2.AttackerWon {
		t.Error("攻下城池並堅持到卅天，應該是攻方獲勝")
	}
}

// TestWipeoutEndsBattle 釘住一方全滅就分勝負。
func TestWipeoutEndsBattle(t *testing.T) {
	b := arena(flat(Plain))
	a := place(b, MainAttacker, Vanguard, FromOffset(5, 5), lead("攻", 90, 50, 20000))
	d := place(b, MainDefender, Centre, FromOffset(5, 5).Step(DirDown), lead("守", 10, 50, 50))
	for i := 0; i < 50 && !b.Over; i++ {
		a.Move = a.MovePoints()
		if err := b.DeathBattle(a, DirDown); err != nil {
			break
		}
	}
	if d.Alive() {
		t.Skipf("五十回合死戰還沒分出勝負（守方剩 %d）", d.Soldiers())
	}
	if !b.Over || !b.AttackerWon {
		t.Errorf("守方全滅，應該是攻方獲勝（Over=%v Won=%v）", b.Over, b.AttackerWon)
	}
}

// TestArcheryNeedsStraightLine 釘住「攻擊目標必須相間一格，
// 且不能隔著大山、城池或關寨」（說明書 p.32）。
func TestArcheryNeedsStraightLine(t *testing.T) {
	f := flat(Plain)
	from := FromOffset(4, 6)
	b := arena(f)
	a := place(b, MainAttacker, Vanguard, from, lead("射", 50, 50, 1000))

	near := from.Step(DirUpRight)
	far := near.Step(DirUpRight)
	bent := from.Step(DirUpRight).Step(DirDownRight)

	place(b, MainDefender, Centre, near, lead("近", 50, 50, 1000))
	if err := b.Archery(a, near); err == nil {
		t.Error("貼身的目標不該射得到——弓箭要相間一格")
	}

	b2 := arena(flat(Plain))
	a2 := place(b2, MainAttacker, Vanguard, from, lead("射", 50, 50, 1000))
	place(b2, MainDefender, Centre, bent, lead("折", 50, 50, 1000))
	if Distance(from, bent) != 2 {
		t.Fatalf("折線目標的距離是 %d，這題要的是 2", Distance(from, bent))
	}
	if err := b2.Archery(a2, bent); err == nil {
		t.Error("折線走兩步的格子不在同一直線上，不該射得到")
	}

	b3 := arena(flat(Plain))
	a3 := place(b3, MainAttacker, Vanguard, from, lead("射", 50, 50, 1000))
	target := place(b3, MainDefender, Centre, far, lead("遠", 50, 50, 1000))
	before := target.Soldiers()
	if err := b3.Archery(a3, far); err != nil {
		t.Fatalf("同一直線相間一格應該射得到：%v", err)
	}
	if target.Soldiers() >= before {
		t.Error("中箭之後兵力沒有減少")
	}
	if a3.Move != 0 {
		t.Error("射完箭這一回合就結束了")
	}

	// 隔著大山、城池、關寨射不過去。
	for _, blocker := range []Terrain{Mountain, City, Fort} {
		f4 := flat(Plain)
		f4.Set(near, blocker)
		b4 := arena(f4)
		a4 := place(b4, MainAttacker, Vanguard, from, lead("射", 50, 50, 1000))
		place(b4, MainDefender, Centre, far, lead("遠", 50, 50, 1000))
		if err := b4.Archery(a4, far); err == nil {
			t.Errorf("隔著%s不該射得到", blocker)
		}
	}
}

// TestArcheryIsLighterThanMelee 釘住箭比白刃輕。
//
// 這一條擋的是「倍率算式恆等於 1」那種錯：兩邊看起來都會扣血，
// 只有把數字擺在一起比才看得出箭沒有比較輕。
func TestArcheryIsLighterThanMelee(t *testing.T) {
	from := FromOffset(4, 6)

	b1 := arena(flat(Plain))
	a1 := place(b1, MainAttacker, Vanguard, from, lead("射", 50, 50, 5000))
	t1 := place(b1, MainDefender, Centre, from.Step(DirUpRight).Step(DirUpRight),
		lead("靶", 50, 50, 5000))
	if err := b1.Archery(a1, t1.At); err != nil {
		t.Fatalf("射箭失敗：%v", err)
	}
	arrowLoss := 5000 - t1.Soldiers()

	b2 := arena(flat(Plain))
	a2 := place(b2, MainAttacker, Vanguard, from, lead("射", 50, 50, 5000))
	t2 := place(b2, MainDefender, Centre, from.Step(DirUpRight), lead("靶", 50, 50, 5000))
	if err := b2.QuickBattle(a2, DirUpRight); err != nil {
		t.Fatalf("快戰失敗：%v", err)
	}
	meleeLoss := 5000 - t2.Soldiers()

	perArrow := arrowLoss / a1.Arrows()
	if perArrow >= meleeLoss {
		t.Errorf("一次箭傷 %d，白刃一次傷 %d——箭應該比較輕", perArrow, meleeLoss)
	}
	if arrowLoss == 0 {
		t.Error("射了箭卻沒有損失")
	}
}

// TestRetreatLosesSuppliesAndNeedsAWayOut 釘住「若被地形和敵軍完全包圍則逃不掉」
// 「若退兵成功，原先擁有的錢糧都會損失」（說明書 p.34）。
func TestRetreatLosesSuppliesAndNeedsAWayOut(t *testing.T) {
	f := flat(Plain)
	spot := FromOffset(6, 6)
	b := arena(f)
	u := place(b, MainAttacker, Centre, spot, lead("甲", 50, 50, 1000))
	b.Gold[MainAttacker], b.Rice[MainAttacker] = 900, 800
	// 五個方向封死、留一個活路。
	for i, d := range Dirs() {
		if i == 0 {
			continue
		}
		f.Set(spot.Step(d), Mountain)
	}
	if err := b.Retreat(u); err != nil {
		t.Fatalf("還有一條活路卻退不了：%v", err)
	}
	if u.Alive() {
		t.Error("退兵之後不該還在場上")
	}
	if b.Gold[MainAttacker] != 0 || b.Rice[MainAttacker] != 0 {
		t.Errorf("退兵後錢糧是 %d/%d，應該盡失", b.Gold[MainAttacker], b.Rice[MainAttacker])
	}

	// 六個方向全封死就逃不掉。
	f2 := flat(Plain)
	b2 := arena(f2)
	u2 := place(b2, MainAttacker, Centre, spot, lead("甲", 50, 50, 1000))
	for _, d := range Dirs() {
		f2.Set(spot.Step(d), Mountain)
	}
	if err := b2.Retreat(u2); err == nil {
		t.Error("被完全包圍應該逃不掉")
	}
}

// TestRetreatOfLastUnitEndsBattle 釘住攻方全撤退等於守方衛郡成功。
func TestRetreatOfLastUnitEndsBattle(t *testing.T) {
	b := arena(flat(Plain))
	u := place(b, MainAttacker, Centre, FromOffset(3, 3), lead("攻", 50, 50, 1000))
	place(b, MainDefender, Centre, FromOffset(15, 10), lead("守", 50, 50, 1000))
	if err := b.Retreat(u); err != nil {
		t.Fatalf("退兵失敗：%v", err)
	}
	if !b.Over || b.AttackerWon {
		t.Errorf("攻方撤光，應該是守方衛郡成功（Over=%v Won=%v）", b.Over, b.AttackerWon)
	}
}

// TestStarvationCausesDesertion 釘住「沒米給軍隊吃，士兵將會陸續逃亡」
// （說明書 p.28）。
func TestStarvationCausesDesertion(t *testing.T) {
	b := arena(flat(Plain))
	u := place(b, MainAttacker, Centre, FromOffset(3, 3), lead("攻", 50, 50, 10000))
	place(b, MainDefender, Centre, FromOffset(15, 10), lead("守", 50, 50, 1000))
	b.Rice[MainAttacker] = 0
	before := u.Soldiers()
	b.EndDay()
	if u.Soldiers() >= before {
		t.Errorf("缺糧一天後兵力 %d，應該少於 %d", u.Soldiers(), before)
	}

	// 帶夠糧就不會逃兵，但米會消耗。
	b2 := arena(flat(Plain))
	u2 := place(b2, MainAttacker, Centre, FromOffset(3, 3), lead("攻", 50, 50, 10000))
	place(b2, MainDefender, Centre, FromOffset(15, 10), lead("守", 50, 50, 1000))
	b2.Rice[MainAttacker] = 5000
	n := u2.Soldiers()
	b2.EndDay()
	if u2.Soldiers() != n {
		t.Errorf("糧食充足卻掉了兵：%d → %d", n, u2.Soldiers())
	}
	if b2.Rice[MainAttacker] >= 5000 {
		t.Error("行軍一天卻沒吃米")
	}
}

// TestEndDayResetsMoveAndTicksStatus 釘住每天重置移動力、遞減中計天數。
func TestEndDayResetsMoveAndTicksStatus(t *testing.T) {
	b := arena(flat(Plain))
	u := place(b, MainAttacker, Centre, FromOffset(3, 3), lead("攻", 50, 50, 1000))
	place(b, MainDefender, Centre, FromOffset(15, 10), lead("守", 50, 50, 1000))
	b.Rice[MainAttacker] = 10000
	u.Move = 0
	u.Trapped = 2
	u.Enraged = 1
	b.EndDay()
	if u.Move != u.MovePoints() {
		t.Errorf("隔天移動力 %d，應該回到 %d", u.Move, u.MovePoints())
	}
	if u.Trapped != 1 {
		t.Errorf("中陷阱剩 %d 天，應該遞減成 1", u.Trapped)
	}
	if u.Enraged != 0 {
		t.Errorf("誘敵效果剩 %d 天，應該遞減成 0", u.Enraged)
	}
}

// TestTrappedUnitCannotAct 釘住「中計的部隊在九日內無法活動」（說明書 p.33）。
func TestTrappedUnitCannotAct(t *testing.T) {
	b := arena(flat(Plain))
	u := place(b, MainAttacker, Centre, FromOffset(3, 3), lead("攻", 50, 50, 1000))
	u.Trapped = TuneTrapDays
	if err := b.Move(u, DirUp); err == nil {
		t.Error("中陷阱的部隊不該走得動")
	}
	if err := b.Rest(u); err == nil {
		t.Error("中陷阱的部隊不該休息得了")
	}
	if TuneTrapDays != 9 {
		t.Errorf("陷阱困住 %d 天，手冊寫的是九日", TuneTrapDays)
	}
}

// TestMeleeRejectsFriendlyAndEmpty 釘住打空氣與打友軍都要擋。
func TestMeleeRejectsFriendlyAndEmpty(t *testing.T) {
	b := arena(flat(Plain))
	spot := FromOffset(6, 6)
	a := place(b, MainAttacker, Vanguard, spot, lead("甲", 50, 50, 1000))
	place(b, AidAttacker, Left, spot.Step(DirUp), lead("友", 50, 50, 1000))
	if err := b.QuickBattle(a, DirUp); err == nil {
		t.Error("打友軍應該失敗")
	}
	if err := b.QuickBattle(a, DirDown); err == nil {
		t.Error("那個方向沒有部隊，應該失敗")
	}
}

// TestStrikeMatchesTheOriginal 釘住一次交手的殺傷（`0x305a9`，`L0`）：
//
//	殺傷 ＝ 兵士數 × 戰力值 ÷ 100
//
// **兩件事要各驗一次**：數字要對得上手算，而且**主動的一邊用地形攻值、
// 被打的一邊用守值**。只驗數字的話，攻守表接反了也照樣綠。
func TestStrikeMatchesTheOriginal(t *testing.T) {
	f := flat(Plain)
	b := arena(f)
	at := FromOffset(2, 2)
	u := place(b, MainAttacker, Centre, at, Leader{
		Name: "甲", War: 80, Arms: 60, Training: 50,
		Soldiers: 2000, Troop: TroopLand,
	})
	// 陸軍在平原：LeaderPower(80,60,陸,平原,攻) ＝ 740×25/1000 ＝ 18
	// 殺傷 ＝ 2000 × 18 / 100 ＝ 360
	if got := b.power(u); got != 360 {
		t.Errorf("攻方的殺傷是 %d，手算是 360", got)
	}
	// 守值 17：740×22/1000 ＝ 16 → 2000 × 16 / 100 ＝ 320
	if got := b.defence(u); got != 320 {
		t.Errorf("守方的殺傷是 %d，手算是 320", got)
	}
	// 兵一半的部隊殺傷也一半——殺傷與兵士數是線性的。
	u.Leaders[0].Soldiers = 1000
	if got := b.power(u); got != 180 {
		t.Errorf("兵減半之後的殺傷是 %d，應該是 180", got)
	}
	// 站到城池上：守值 40 遠高於平原的 17，攻值 27 高於 20。
	f.Set(at, City)
	if b.defence(u) <= 160 || b.power(u) <= 90 {
		t.Errorf("城池上的殺傷 攻 %d 守 %d，應該都高過平原",
			b.power(u), b.defence(u))
	}
}

// TestMeleeIsSimultaneous 釘住雙方**同時**算傷亡（`0x30618`／`0x306bb`）。
//
// 先扣一邊再算另一邊的話，先手會佔到不該有的便宜——把守方的兵設成
// 剛好被一擊打光，看攻方有沒有照樣挨打。
func TestMeleeIsSimultaneous(t *testing.T) {
	f := flat(Plain)
	b := arena(f)
	a := place(b, MainAttacker, Centre, FromOffset(2, 2), Leader{
		Name: "甲", War: 80, Arms: 60, Soldiers: 2000, Troop: TroopLand,
	})
	d := place(b, MainDefender, Centre, FromOffset(2, 2).Step(DirDown), Leader{
		Name: "乙", War: 80, Arms: 60, Soldiers: 100, Troop: TroopLand,
	})
	before := a.Soldiers()
	if err := b.QuickBattle(a, DirDown); err != nil {
		t.Fatalf("快戰失敗：%v", err)
	}
	if d.Soldiers() != 0 {
		t.Errorf("守方剩 %d 兵，應該被一擊打光", d.Soldiers())
	}
	if a.Soldiers() >= before {
		t.Errorf("攻方一兵未損（%d → %d）——反擊應該同時發生",
			before, a.Soldiers())
	}
	// 兵打光的將領被俘（原版 0x30716「我們抓到%s」）。
	if !d.Leaders[0].Captured {
		t.Error("兵打光的將領應該被擒")
	}
}
