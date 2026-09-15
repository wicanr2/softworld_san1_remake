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
	// 計謀不動移動力（`0x28acc`／`0x29784` 都不碰 offset 36）：玩家那一邊
	// 回合照樣結束，那是命令迴圈的事，不是把點數歸零。
	if u.Move == 0 {
		t.Error("用計不該把移動力歸零")
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

// TestFloodNeedsShallowNextDoor 釘住水淹的岸邊判準（`L0`、`0x2bfa2`）。
//
// 說明書 p.33 寫「目標必須在水上或岸邊」，原版的碼比這句窄：它掃目標的
// 六個鄰格，**只認地形碼 3（淺水）**——深水不算，目標自己站在水上
// 也不算。位階以反組譯為準（`CLAUDE.md` §4）。
func TestFloodNeedsShallowNextDoor(t *testing.T) {
	// 一片乾地：不成立。
	b, u, e := plotting(Plain, Rainy, 100, 10000)
	if err := b.UseStratagem(u, Flood, e.At); err == nil {
		t.Error("目標在乾地上不該水淹得了")
	}
	// 目標腳下是淺水，但六個鄰格都是乾地：原版不算岸邊。
	b, u, e = plotting(Plain, Rainy, 100, 10000)
	b.Field.Set(e.At, Shallow)
	if err := b.UseStratagem(u, Flood, e.At); err == nil {
		t.Error("原版只看鄰格，目標自己站在水上不該讓水淹成立")
	}
	// 鄰格是深水：也不算。
	b, u, e = plotting(Plain, Rainy, 100, 10000)
	b.Field.Set(e.At.Step(DirUp), Deep)
	if err := b.UseStratagem(u, Flood, e.At); err == nil {
		t.Error("原版只認淺水，鄰格是深水不該讓水淹成立")
	}
	// 鄰格是淺水：成立。
	b, u, e = plotting(Plain, Rainy, 100, 10000)
	b.Field.Set(e.At.Step(DirUp), Shallow)
	if err := b.UseStratagem(u, Flood, e.At); err != nil {
		t.Errorf("鄰格有淺水，水淹應該成立：%v", err)
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
	// 天數是 `RND(5)+1`（領隊謀略 100 ＞ 98，再加 `RND(5)+2`），
	// 所以落在 3–11 之間。**手冊寫的九日不對**（`0x2b648`）。
	if e.Trapped < 3 || e.Trapped > 11 {
		t.Errorf("困住 %d 天，應該落在 3–11", e.Trapped)
	}
	if e.Soldiers() != 2000 {
		t.Error("陷阱困住敵軍，不是當場殺兵")
	}
}

// TestLureMakesTheEnemyStrike 釘住誘敵是**把敵人引過來打你**
// （`0x2b6aa` ＋ `0x2a224`，`L0`）。
//
// 說明書 p.33 寫的是「來犯敵軍攻擊力暫時下降」，碼裡沒有這回事：
// 誘敵播完動畫就叫共同的交戰結算，而且推參數時把攻守對調，出手的是
// 目標。**兩邊都會掉兵**，划不划算看目標的攻擊力比不比得過施法者的
// 防禦力——這一條的判準因此是「雙方都掉兵」，不是「敵人變弱」。
func TestLureMakesTheEnemyStrike(t *testing.T) {
	b, u, e := plotting(Plain, Clear, 100, 10000)
	mine, theirs := u.Soldiers(), e.Soldiers()
	if err := b.UseStratagem(u, Lure, e.At); err != nil {
		t.Fatalf("誘敵失敗：%v", err)
	}
	if u.Soldiers() >= mine {
		t.Errorf("施法者剩 %d 兵，原本 %d——誘敵是引敵人來打，自己也會掉兵",
			u.Soldiers(), mine)
	}
	if e.Soldiers() >= theirs {
		t.Errorf("目標剩 %d 兵，原本 %d——出手的一方也會被還擊",
			e.Soldiers(), theirs)
	}
}

// TestStrikeMultiplierTable 釘住傷害倍率表與它的越界行為
// （`DS:0x81a2` ＝ 80/100/150/200/250/300/350/400，夾在 `0x2a2c9`）。
//
// **越界回第 1 格不是防禦式寫法**：誘敵傳的 8 與 9 就是靠這個落點
// 變成 100 的，所以「謀略 ≥ 98 加碼」在原版完全沒有作用。改成夾到
// 第 0 格或直接用 8 都會讓誘敵的傷害不一樣。
func TestStrikeMultiplierTable(t *testing.T) {
	want := []int{80, 100, 150, 200, 250, 300, 350, 400}
	for i, w := range want {
		if got := StrikeMultiplier(i); got != w {
			t.Errorf("模式 %d 的倍率是 %d，原版是 %d", i, got, w)
		}
	}
	for _, bad := range []int{-1, 8, 9, 99} {
		if got := StrikeMultiplier(bad); got != 100 {
			t.Errorf("模式 %d 越界，倍率是 %d，原版夾成第 1 格 ＝ 100", bad, got)
		}
	}
	if StrikeMultiplier(LureStrike) != 100 {
		t.Error("誘敵落在第 1 格（100）")
	}
	// **圍攻沒有固定的格子**：從 0 起算，每一支圍著目標的敵方部隊加一，
	// 施法者的領隊謀略到 98 再加一（`0x2bc95`–`0x2bd44`）。所以最少是
	// 1（施法者自己貼著），圍滿六格又有神算的話是 7。
	if StrikeMultiplier(1) != 100 || StrikeMultiplier(7) != 400 {
		t.Error("圍攻的倍率格會在 1..7 之間跑")
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
		t           Terrain
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
	// 除數最小是 10 − RND(戰力÷20)，最大是 RND(10) − 戰力÷10 + 50，
	// 所以損失落在 1/50 到 1/8 之間；拒絕的一方一定掉一些兵。
	if lost := 1000 - e.Soldiers(); lost <= 0 || lost > 1000/8 {
		t.Errorf("拒戰後逃散 %d 人，應該落在 1..125", lost)
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

// TestDuelAccepted 釘住接不接受單挑（`0x30c5b`–`0x30d5b`，`L0`）。
//
// **預設是拒絕**，三道機會翻成接受。這一條同時擋住「一律應戰」與
// 「一律拒絕」兩種寫法——兩者都會讓整條單挑看起來正常卻不對。
func TestDuelAccepted(t *testing.T) {
	const same = 1000
	// 戰力相同：`RND(10) + 戰力 − 5 > 戰力` ⇒ RND 要大於 5，十分之四。
	n := 0
	for r := 0; r < DuelWarSpread; r++ {
		if DuelAccepted(80, 80, same, same, r, 0) {
			n++
		}
	}
	if n != 4 {
		t.Errorf("戰力相同時接受 %d/10 次，應該是 4", n)
	}
	// 被挑戰者強五點：RND 只要不是 0 就接受。
	n = 0
	for r := 0; r < DuelWarSpread; r++ {
		if DuelAccepted(80, 85, same, same, r, 0) {
			n++
		}
	}
	if n != 9 {
		t.Errorf("被挑戰者強五點時接受 %d/10 次，應該是 9", n)
	}
	// 被挑戰者弱五點：第一道永遠過不了，兵力也相當 → 一律拒絕。
	for r := 0; r < DuelWarSpread; r++ {
		if DuelAccepted(85, 80, same, same, r, 0) {
			t.Errorf("被挑戰者弱五點、兵力相當，RND ＝ %d 卻接受了", r)
		}
	}
	// **兵力懸殊會翻盤**：對方兵是我方兩倍以上時第二道開，
	// 五倍以上時第三道無條件接受——「猛將帶寡兵」靠的就是這一條。
	if !DuelAccepted(99, 60, 400, 3000, 0, 0) {
		t.Error("對方兵力五倍以上應該無條件接受")
	}
	if DuelAccepted(99, 60, 1600, 3000, 9, 19) {
		t.Error("兵力不到兩倍、戰力又差很多，不該接受")
	}
	if !DuelAccepted(62, 60, 1000, 2500, 0, 5) {
		t.Error("對方兵力兩倍以上、戰力接近時，第二道應該讓它接受")
	}
}

// TestDuelFormula 釘住單挑的兩條公式（`0x310b6`／`0x31170`，`L0`）。
//
// **期望值是「戰力差 − 6.5」**：戰力沒有高過對方七點左右就傷不了人。
// 這一條擋的是「把傷害寫成一個固定值再乘戰力比」——那樣寫的話
// 五十對五十也會分出勝負，而原版是打到回合用完平手。
func TestDuelFormula(t *testing.T) {
	// 回合數：RND((甲+乙)/2) + 甲/7 + 乙/7。
	if got := DuelRounds(90, 70, 0); got != 12+10 {
		t.Errorf("戰力 90 對 70、亂數 0 打 %d 回合，應該是 22", got)
	}
	if got := DuelRounds(90, 70, 79); got != 79+22 {
		t.Errorf("亂數 79 時打 %d 回合，應該是 101", got)
	}
	// 每回合：RND(5) + 差 − RND(5) − RND(6) − 4，不小於 0。
	if got := DuelBlow(90, 70, 4, 0, 0); got != 20 {
		t.Errorf("差 20、亂數最有利時打掉 %d，應該是 4+20-0-0-4 ＝ 20", got)
	}
	if got := DuelBlow(90, 70, 0, 4, 5); got != 7 {
		t.Errorf("差 20、亂數最不利時打掉 %d，應該是 0+20-4-5-4 ＝ 7", got)
	}
	// 戰力相同：最好的一次也只有 1，多數回合是 0。
	best := 0
	for a := 0; a < DuelBlowSpread; a++ {
		for b := 0; b < DuelBlowSpread; b++ {
			for c := 0; c < DuelBlowWide; c++ {
				if n := DuelBlow(70, 70, a, b, c); n > best {
					best = n
				}
			}
		}
	}
	if best != 0 {
		t.Errorf("戰力相同時最多打掉 %d，應該是 0——傷不了人", best)
	}
	// 方向要對：強的一方傷得了人，弱的一方傷不了。
	if DuelBlow(76, 70, 4, 0, 0) != 6 || DuelBlow(70, 76, 4, 0, 0) != 0 {
		t.Errorf("戰力差的方向不對：強方 %d、弱方 %d",
			DuelBlow(76, 70, 4, 0, 0), DuelBlow(70, 76, 4, 0, 0))
	}
}

// TestRefuseDuelDivisor 釘住拒絕單挑的除數（`0x30de6`–`0x30ef5`，`L0`）。
//
// **三條分支的順序才是重點**：謀士拒絕損失最小，明顯打不過而拒絕
// 次之，**旗鼓相當卻拒絕的損失最重**——那是怯戰。
func TestRefuseDuelDivisor(t *testing.T) {
	// 謀略 95 > RND(10)+80 的上界 89 → 一定走第一條：
	// RND(10) − 戰力/10 + 50，戰力 70、RND 0 → 0 − 7 + 50 ＝ 43。
	if got := RefuseDuelDivisor(95, 70, 99, 0, 0, 0, 0); got != 43 {
		t.Errorf("謀士拒絕的除數是 %d，應該是 43", got)
	}
	// 謀略 50：第二條，RND(5)+戰力 < 挑戰者戰力 → 25 − RND(戰力/20)。
	if got := RefuseDuelDivisor(50, 70, 99, 0, 0, 0, 0); got != 25 {
		t.Errorf("打不過而拒絕的除數是 %d，應該是 25", got)
	}
	// 謀略 50、戰力相當：第三條 → 10 − RND(戰力/20)。
	if got := RefuseDuelDivisor(50, 70, 70, 0, 0, 0, 0); got != 10 {
		t.Errorf("旗鼓相當卻拒絕的除數是 %d，應該是 10", got)
	}
	// **順序**：謀士 > 打不過 > 旗鼓相當（除數越大掉得越少）。
	wise := RefuseDuelDivisor(95, 70, 99, 0, 0, 0, 0)
	weak := RefuseDuelDivisor(50, 70, 99, 0, 0, 0, 0)
	even := RefuseDuelDivisor(50, 70, 70, 0, 0, 0, 0)
	if !(wise > weak && weak > even) {
		t.Errorf("三條分支的輕重順序不對：謀士 %d、打不過 %d、旗鼓相當 %d",
			wise, weak, even)
	}
	// 除數不會掉到 0——戰力滿的話 10 − RND(5) 最小是 6，但資料越界時要擋。
	if RefuseDuelDivisor(50, 255, 70, 0, 0, 0, 99) < 1 {
		t.Error("除數不該小於 1")
	}
}

// TestDuelDefeatIsUsuallyCapture 釘住單挑落敗的處置：`RND(7)`，只有 0 才死
// （`0x31b04`–`0x31b16`，`L0`）。
//
// **比例是 6/7 被擒、1/7 死**，不是各半。說明書 p.30 只說「可能被擒，
// 或死於刀下」——照字面實作成五五開，猛將的損耗會是原版的三倍多，
// 而那要玩很久才看得出來。
func TestDuelDefeatIsUsuallyCapture(t *testing.T) {
	if !DuelKills(0) {
		t.Error("擲 0 應該是死於刀下")
	}
	for roll := 1; roll < DuelDeathRoll; roll++ {
		if DuelKills(roll) {
			t.Errorf("擲 %d 應該是被擒", roll)
		}
	}
	if DuelDeathRoll != 7 {
		t.Errorf("擲的範圍是 RND(%d)，原版是 RND(7)", DuelDeathRoll)
	}
}
