package battle

import "testing"

// TestStratagemThresholdsAndCosts 釘住六種計謀的智力門檻與費用（說明書 p.32–34）。
//
// **這張表是手冊寫死的數字**，不是 remake 調得動的參數。
func TestStratagemThresholdsAndCosts(t *testing.T) {
	want := []struct {
		s     Stratagem
		name  string
		intel int
		cost  int
	}{
		{Fire, "火攻", 80, 600},
		{Flood, "水淹", 75, 500},
		{Trap, "陷阱", 60, 100},
		{Lure, "誘敵", 60, 400},
		{Burn, "燒糧", 70, 300},
		{Siege, "圍攻", 65, 200},
	}
	for _, w := range want {
		if w.s.String() != w.name {
			t.Errorf("計謀 %d 叫 %s，應該是 %s", w.s, w.s, w.name)
		}
		if got := w.s.MinIntel(); got != w.intel {
			t.Errorf("%s 的智力門檻是 %d，手冊寫 %d", w.name, got, w.intel)
		}
		if got := w.s.Cost(); got != w.cost {
			t.Errorf("%s 要 %d 金，手冊寫 %d", w.name, got, w.cost)
		}
	}
	// 編號以原版執行檔的策略選單為準（`docs/re/04` §4）：
	// 「1.火攻 2.水洽 3.陷阱／4.誘敵 5.燒糧 6.圍攻」。
	// **手冊把誘敵排第 3、陷阱排第 4**，與程式不同；程式是實際跑的東西。
	if Fire != 1 || Flood != 2 || Trap != 3 || Lure != 4 || Burn != 5 || Siege != 6 {
		t.Error("計謀編號要與原版執行檔的策略選單相同")
	}
}

// plotting 擺一個「用計方在左、目標在右」的局面。
func plotting(terrain Terrain, w Weather, intel uint8, gold int) (*Battle, *Unit, *Unit) {
	f := flat(terrain)
	b := arena(f)
	b.Weather = w
	spot := FromOffset(6, 6)
	u := place(b, MainAttacker, Centre, spot, lead("軍師", 50, intel, 2000))
	t := place(b, MainDefender, Centre, spot.Step(DirDownRight), lead("敵", 50, 50, 2000))
	b.Gold[MainAttacker] = gold
	b.Gold[MainDefender] = 1000
	b.Rice[MainDefender] = 1000
	return b, u, t
}

// TestStratagemNeedsIntelAndGold 釘住門檻與「沒有帶錢就無法用計」（說明書 p.28）。
func TestStratagemNeedsIntelAndGold(t *testing.T) {
	b, u, e := plotting(Plain, Windy, 79, 10000)
	if err := b.UseStratagem(u, Fire, e.At); err == nil {
		t.Error("智力 79 不該用得了火攻（門檻 80）")
	}

	b, u, e = plotting(Plain, Windy, 80, 599)
	if err := b.UseStratagem(u, Fire, e.At); err == nil {
		t.Error("隨軍只有 599 金不該用得了火攻（要 600）")
	}

	b, u, e = plotting(Plain, Windy, 80, 600)
	if err := b.UseStratagem(u, Fire, e.At); err != nil {
		t.Fatalf("智力 80、帶 600 金、刮風天，火攻應該成立：%v", err)
	}
	if b.Gold[MainAttacker] != 0 {
		t.Errorf("用完火攻剩 %d 金，600 應該扣光", b.Gold[MainAttacker])
	}
	if u.Move != 0 {
		t.Error("用計之後這一回合就結束了")
	}
}

// TestStratagemUsesSmartestLeader 釘住門檻看隊伍裡謀略最高的那一位。
func TestStratagemUsesSmartestLeader(t *testing.T) {
	f := flat(Plain)
	b := arena(f)
	b.Weather = Windy
	spot := FromOffset(6, 6)
	u := place(b, MainAttacker, Centre, spot,
		lead("莽夫", 99, 10, 1000), lead("軍師", 20, 90, 1000))
	e := place(b, MainDefender, Centre, spot.Step(DirDownRight), lead("敵", 50, 50, 2000))
	b.Gold[MainAttacker] = 10000
	if err := b.UseStratagem(u, Fire, e.At); err != nil {
		t.Errorf("隊裡有智力 90 的軍師，火攻應該用得出來：%v", err)
	}
}

// TestWeatherGates 釘住天氣限制（說明書 p.32–34）：
// 火攻要刮風、水淹要下雨、燒糧下雨天不能用。
func TestWeatherGates(t *testing.T) {
	for _, w := range []Weather{Clear, Rainy} {
		b, u, e := plotting(Plain, w, 100, 10000)
		if err := b.UseStratagem(u, Fire, e.At); err == nil {
			t.Errorf("%s 的時候不該用得了火攻", w)
		}
	}
	for _, w := range []Weather{Clear, Windy} {
		b, u, e := plotting(Shallow, w, 100, 10000)
		if err := b.UseStratagem(u, Flood, e.At); err == nil {
			t.Errorf("%s 的時候不該用得了水淹", w)
		}
	}
	b, u, e := plotting(Plain, Rainy, 100, 10000)
	if err := b.UseStratagem(u, Burn, e.At); err == nil {
		t.Error("下雨天無法燒糧")
	}
	b, u, e = plotting(Plain, Clear, 100, 10000)
	if err := b.UseStratagem(u, Burn, e.At); err != nil {
		t.Errorf("晴天燒糧應該成立：%v", err)
	}
}

// TestFloodNeedsWaterOrShore 釘住「目標必須在水上或岸邊」（說明書 p.33）。
func TestFloodNeedsWaterOrShore(t *testing.T) {
	// 一片乾地：不成立。
	b, u, e := plotting(Plain, Rainy, 100, 10000)
	if err := b.UseStratagem(u, Flood, e.At); err == nil {
		t.Error("目標在乾地上不該水淹得了")
	}
	// 目標腳下就是水：成立。
	b, u, e = plotting(Plain, Rainy, 100, 10000)
	b.Field.Set(e.At, Shallow)
	if err := b.UseStratagem(u, Flood, e.At); err != nil {
		t.Errorf("目標在水上，水淹應該成立：%v", err)
	}
	// 岸邊也算。
	b, u, e = plotting(Plain, Rainy, 100, 10000)
	b.Field.Set(e.At.Step(DirUp), Deep)
	if err := b.UseStratagem(u, Flood, e.At); err != nil {
		t.Errorf("目標在岸邊，水淹應該成立：%v", err)
	}
}

// TestBurnNotOnWater 釘住「也不能用於水上的敵軍」（說明書 p.34）。
func TestBurnNotOnWater(t *testing.T) {
	b, u, e := plotting(Plain, Clear, 100, 10000)
	b.Field.Set(e.At, Deep)
	if err := b.UseStratagem(u, Burn, e.At); err == nil {
		t.Error("不能對水上的敵軍燒糧")
	}
}

// TestBurnReducesSupplies 釘住燒糧燒的是對方的錢糧。
func TestBurnReducesSupplies(t *testing.T) {
	b, u, e := plotting(Plain, Clear, 100, 10000)
	gold, rice := b.Gold[MainDefender], b.Rice[MainDefender]
	if err := b.UseStratagem(u, Burn, e.At); err != nil {
		t.Fatalf("燒糧失敗：%v", err)
	}
	if b.Gold[MainDefender] >= gold || b.Rice[MainDefender] >= rice {
		t.Errorf("燒糧後對方錢糧是 %d/%d，原本 %d/%d",
			b.Gold[MainDefender], b.Rice[MainDefender], gold, rice)
	}
	if e.Soldiers() != 2000 {
		t.Error("燒糧燒的是補給，不該當場殺兵")
	}
}

// TestTrapPlacementAndDuration 釘住「不得用在水上、城池或關寨中」
// 與「中計的部隊在九日內無法活動」（說明書 p.33）。
func TestTrapPlacementAndDuration(t *testing.T) {
	for _, bad := range []Terrain{Shallow, Deep, City, Fort} {
		b, u, e := plotting(Plain, Clear, 100, 10000)
		b.Field.Set(e.At, bad)
		if err := b.UseStratagem(u, Trap, e.At); err == nil {
			t.Errorf("陷阱不該設在%s", bad)
		}
	}
	b, u, e := plotting(Plain, Clear, 100, 10000)
	if err := b.UseStratagem(u, Trap, e.At); err != nil {
		t.Fatalf("平原設陷阱應該成立：%v", err)
	}
	if e.Trapped != 9 {
		t.Errorf("困住 %d 天，手冊寫九日", e.Trapped)
	}
	if e.Soldiers() != 2000 {
		t.Error("陷阱困住敵軍，不是當場殺兵")
	}
}

// TestLureLowersAttack 釘住誘敵讓「來犯敵軍攻擊力暫時下降」（說明書 p.33）。
func TestLureLowersAttack(t *testing.T) {
	b, u, e := plotting(Plain, Clear, 100, 10000)
	before := b.power(e)
	if err := b.UseStratagem(u, Lure, e.At); err != nil {
		t.Fatalf("誘敵失敗：%v", err)
	}
	if e.Enraged <= 0 {
		t.Fatal("誘敵之後應該掛著效果")
	}
	if after := b.power(e); after >= before {
		t.Errorf("中了誘敵的攻擊力是 %d，原本 %d，應該下降", after, before)
	}
}

// TestSiegeNeedsAllyNextToTarget 釘住「目標旁邊必須尚有其他友軍」（說明書 p.34）。
func TestSiegeNeedsAllyNextToTarget(t *testing.T) {
	b, u, e := plotting(Plain, Clear, 100, 10000)
	if err := b.UseStratagem(u, Siege, e.At); err == nil {
		t.Error("目標旁邊沒有友軍，圍攻不該成立")
	}
	// 派一支友軍貼上去。
	place(b, MainAttacker, Left, e.At.Step(DirDown), lead("友", 50, 50, 1000))
	before := e.Soldiers()
	if err := b.UseStratagem(u, Siege, e.At); err != nil {
		t.Fatalf("旁邊有友軍，圍攻應該成立：%v", err)
	}
	if e.Soldiers() >= before {
		t.Error("圍攻之後對方兵力沒有減少")
	}
}

// TestSiegeGetsStrongerWithMoreAllies 釘住圍的人越多打得越重。
func TestSiegeGetsStrongerWithMoreAllies(t *testing.T) {
	loss := func(allies int) int {
		b, u, e := plotting(Plain, Clear, 100, 10000)
		dirs := []Dir{DirDown, DirUp, DirUpRight}
		for i := 0; i < allies; i++ {
			place(b, MainAttacker, Formation(i), e.At.Step(dirs[i]),
				lead("友", 50, 50, 1000))
		}
		before := e.Soldiers()
		if err := b.UseStratagem(u, Siege, e.At); err != nil {
			t.Fatalf("%d 支友軍的圍攻失敗：%v", allies, err)
		}
		return before - e.Soldiers()
	}
	one, three := loss(1), loss(3)
	if three <= one {
		t.Errorf("三支友軍圍攻只打掉 %d，一支打掉 %d——人多應該打得重", three, one)
	}
}

// TestFireAndFloodTables 釘住火攻與水淹的地形殺傷（`DS:0x8200`／
// `DS:0x8224`，`L0`），以及它們與說明書 p.32–33 的排序關係。
//
// **原版的索引是 `地形碼 & 0x0D` 而表只有八格**，所以樹林（8）與
// 沙漠（9）讀到表外；比率之後被 0.9 夾住，結果就是那兩種地形一律
// 吃滿九成。這一條同時釘住量到的數字與那個折疊。
func TestFireAndFloodTables(t *testing.T) {
	const noRoll, plain = 0, 50 // 亂數 0、領隊謀略未達 98
	for _, c := range []struct {
		t          Terrain
		fire, flood int
	}{
		{Hill, 25, 20},
		{Mountain, 15, 2}, {Shallow, 15, 2},
		{Deep, 20, 30}, {Fort, 20, 30},
		{City, 55, 35}, {Plain, 55, 35},
		{Forest, 90, 90}, {Desert, 90, 90},
	} {
		if got := FireLoss(c.t, plain, noRoll); got != c.fire {
			t.Errorf("火攻在%s是 %d%%，原版是 %d%%", c.t, got, c.fire)
		}
		if got := FloodLoss(c.t, plain, noRoll); got != c.flood {
			t.Errorf("水淹在%s是 %d%%，原版是 %d%%", c.t, got, c.flood)
		}
	}
	// 排序：樹林最強、水上最輕（說明書 p.32–33）。
	if FireLoss(Forest, plain, 0) <= FireLoss(Plain, plain, 0) {
		t.Error("火攻在樹林應該最強")
	}
	if FloodLoss(Forest, plain, 0) <= FloodLoss(Plain, plain, 0) {
		t.Error("水淹在樹林應該最強")
	}
	if FireLoss(Shallow, plain, 0) >= FireLoss(Hill, plain, 0) {
		t.Error("火攻在水上應該最輕")
	}
	// **城池是兩張表分岔的地方**：火攻 55、水淹 35。
	if FireLoss(City, plain, 0) == FloodLoss(City, plain, 0) {
		t.Error("城池的火攻與水淹殺傷不該相同")
	}
	// 亂數加上去，上限 90 不會被突破。
	if got := FireLoss(Plain, plain, 9); got != 64 {
		t.Errorf("平原火攻 + RND 9 是 %d%%，應該是 64%%", got)
	}
	if got := FireLoss(Forest, plain, 9); got != StratagemMaxLoss {
		t.Errorf("樹林火攻 + RND 9 是 %d%%，應該夾在 %d%%", got, StratagemMaxLoss)
	}
	// 謀略 98 起跳乘 1.6，一樣夾在 90。
	if got := FireLoss(Plain, StratagemGeniusIntel, 0); got != 88 {
		t.Errorf("謀略 98 的平原火攻是 %d%%，應該是 55×1.6 ＝ 88%%", got)
	}
	if got := FireLoss(City, StratagemGeniusIntel, 9); got != StratagemMaxLoss {
		t.Errorf("謀略 98 的城池火攻是 %d%%，應該夾在 %d%%", got, StratagemMaxLoss)
	}
}

// TestDuelIsDecidedByWarNotSoldiers 釘住「依其戰力強弱分高下，
// 與率領軍力大小無關」（說明書 p.30）。
func TestDuelIsDecidedByWarNotSoldiers(t *testing.T) {
	b := arena(flat(Plain))
	spot := FromOffset(6, 6)
	// 猛將帶五十人，庸將帶兩萬人。
	a := place(b, MainAttacker, Vanguard, spot, lead("呂布", 100, 30, 50))
	e := place(b, MainDefender, Centre, spot.Step(DirDownRight), lead("庸將", 10, 30, 20000))
	if err := b.Duel(a, DirDownRight, true); err != nil {
		t.Fatalf("單挑失敗：%v", err)
	}
	loser := &e.Leaders[0]
	if !loser.Captured && !loser.Dead {
		t.Errorf("戰力 10 對上 100 應該落敗（體能剩 %d）", loser.Stamina)
	}
	if loser.Stamina != 0 {
		t.Errorf("落敗者體能是 %d，應該降到 0", loser.Stamina)
	}
	if a.Leaders[0].Captured || a.Leaders[0].Dead {
		t.Error("戰力 100 的一方不該落敗")
	}
	if a.Move != 0 {
		t.Error("單挑之後這一回合就結束了")
	}
}

// TestRefusingDuelCostsSoldiers 釘住「若拒絕挑戰，麾下士兵將有部份逃跑」
// （說明書 p.30）。
func TestRefusingDuelCostsSoldiers(t *testing.T) {
	b := arena(flat(Plain))
	spot := FromOffset(6, 6)
	a := place(b, MainAttacker, Vanguard, spot, lead("挑戰者", 90, 30, 1000))
	e := place(b, MainDefender, Centre, spot.Step(DirDownRight), lead("怯戰", 20, 30, 1000))
	if err := b.Duel(a, DirDownRight, false); err != nil {
		t.Fatalf("拒絕單挑不該回錯誤：%v", err)
	}
	if e.Soldiers() != 900 {
		t.Errorf("拒戰後剩 %d 兵，%d%% 逃跑應該剩 900",
			e.Soldiers(), TuneRefuseDuelLoss)
	}
	if e.Leaders[0].Captured || e.Leaders[0].Dead {
		t.Error("拒絕挑戰的人不會因此被擒或被斬")
	}
}

// TestDuelRejectsFriendlyAndEmpty 釘住單挑一樣不能挑友軍或空氣。
func TestDuelRejectsFriendlyAndEmpty(t *testing.T) {
	b := arena(flat(Plain))
	spot := FromOffset(6, 6)
	a := place(b, MainAttacker, Vanguard, spot, lead("甲", 50, 50, 1000))
	place(b, AidAttacker, Left, spot.Step(DirUp), lead("友", 50, 50, 1000))
	if err := b.Duel(a, DirUp, true); err == nil {
		t.Error("不該跟友軍單挑")
	}
	if err := b.Duel(a, DirDown, true); err == nil {
		t.Error("那個方向沒有部隊，單挑應該失敗")
	}
}

// TestStratagemRejectsFriendlyAndEmpty 釘住計謀的目標也要是敵軍。
func TestStratagemRejectsFriendlyAndEmpty(t *testing.T) {
	b := arena(flat(Plain))
	spot := FromOffset(6, 6)
	u := place(b, MainAttacker, Centre, spot, lead("軍師", 50, 100, 1000))
	place(b, AidAttacker, Left, spot.Step(DirUp), lead("友", 50, 50, 1000))
	b.Gold[MainAttacker] = 10000
	if err := b.UseStratagem(u, Trap, spot.Step(DirUp)); err == nil {
		t.Error("不該對友軍用計")
	}
	if err := b.UseStratagem(u, Trap, spot.Step(DirDown)); err == nil {
		t.Error("那裡沒有部隊，用計應該失敗")
	}
	if b.Gold[MainAttacker] != 10000 {
		t.Error("用計失敗不該扣錢")
	}
}

// TestStratagemTablesMatchTheOriginal 釘住六種計謀的費用與智力門檻
// （`DS:0x7f62`／`DS:0x7f6e`，`L0`）。
//
// 原本的出處是說明書 p.32–34；量到的兩張表**與它逐格相同**，
// 而且電腦諸侯讀的是同一組（`0x297e6`），所以玩家與電腦的門檻一致。
func TestStratagemTablesMatchTheOriginal(t *testing.T) {
	for _, c := range []struct {
		s              Stratagem
		cost, minIntel int
	}{
		{Fire, 600, 80}, {Flood, 500, 75}, {Trap, 100, 60},
		{Lure, 400, 60}, {Burn, 300, 70}, {Siege, 200, 65},
	} {
		if got := c.s.Cost(); got != c.cost {
			t.Errorf("%s 要 %d 金，原版是 %d", c.s, got, c.cost)
		}
		if got := c.s.MinIntel(); got != c.minIntel {
			t.Errorf("%s 的智力門檻是 %d，原版是 %d", c.s, got, c.minIntel)
		}
	}
	// 選單的順序是原版的：3 是陷阱、4 是誘敵（手冊排反了）。
	if Fire != 1 || Flood != 2 || Trap != 3 || Lure != 4 || Burn != 5 || Siege != 6 {
		t.Error("計謀的編號與原版選單不同")
	}
}

// TestStratagemSucceeds 釘住計謀的成功判定（`0x2ac4d`，`L0`）：
//
//	RND(上限) + 目標領隊的謀略 < 施法者領隊的謀略
//
// **說明書完全沒提這一關**——它只寫了智力門檻與費用，過了那兩關
// 看起來就一定成功。
func TestStratagemSucceeds(t *testing.T) {
	for _, c := range []struct {
		s    Stratagem
		want int
	}{
		{Fire, 10}, {Flood, 8}, {Trap, 2}, {Lure, 2}, {Burn, 4}, {Siege, 6},
	} {
		if got := c.s.Spread(); got != c.want {
			t.Errorf("%s 的亂數上限是 %d，原版是 %d", c.s, got, c.want)
		}
	}
	// 謀略相同一次都不會成功——亂數最小是 0。
	for r := 0; r < 10; r++ {
		if StratagemSucceeds(80, 80, r) {
			t.Errorf("謀略同樣是 80、RND ＝ %d 卻成功了", r)
		}
	}
	// 高出 10 點的話，火攻（上限 10）一定成功。
	for r := 0; r < Fire.Spread(); r++ {
		if !StratagemSucceeds(90, 80, r) {
			t.Errorf("謀略 90 對 80、RND ＝ %d 卻失敗了", r)
		}
	}
	// 高出 5 點：火攻一半機率、陷阱（上限 2）一定成功。
	n := 0
	for r := 0; r < Fire.Spread(); r++ {
		if StratagemSucceeds(85, 80, r) {
			n++
		}
	}
	if n != 5 {
		t.Errorf("謀略 85 對 80 的火攻成功 %d/10 次，應該是 5", n)
	}
	for r := 0; r < Trap.Spread(); r++ {
		if !StratagemSucceeds(85, 80, r) {
			t.Errorf("謀略 85 對 80 的陷阱、RND ＝ %d 卻失敗了", r)
		}
	}
}
