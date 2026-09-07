package game

import (
	"errors"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestMoveNeedsSuccessor 釘住「主事者移出之前要先有接手的人」。
//
// **一個安靜地變成無主的郡，在畫面上只看得出顏色變了。**
func TestMoveNeedsSuccessor(t *testing.T) {
	g := newGame(t)
	// 董卓（勢力 5）在弘農(14)，鄰郡洛陽(15) 也是他的。
	dz := g.Lord(5)
	if dz == nil {
		t.Fatal("找不到董卓")
	}
	// 先把弘農其他人搬走，讓董卓變成唯一的守將。
	for _, x := range g.Garrison(dz.Location) {
		if x.Index != dz.Index {
			x.Location = 15
		}
	}

	if err := g.Move(dz.Location, 15, dz.Index, 0, 0, 5); !errors.Is(err, ErrNoGovernor) {
		t.Errorf("唯一的主事者移出回 %v，應該是 ErrNoGovernor", err)
	}
}

// TestMoveCarriesGoldAndRice 釘住「移防時當地金米可一併隨行」（說明書 p.19）。
func TestMoveCarriesGoldAndRice(t *testing.T) {
	g := newGame(t)
	from, to := 14, 15 // 董卓的弘農 → 洛陽
	src, dst := g.Prefecture(from), g.Prefecture(to)
	var mover *General
	for _, x := range g.Garrison(from) {
		if x.Faction == 5 && !x.Status.Governs() {
			mover = x
			break
		}
	}
	if mover == nil {
		t.Skip("弘農沒有可以移動的非主事者")
	}
	g0, r0, dg0, dr0 := src.Gold, src.Rice, dst.Gold, dst.Rice
	if err := g.Move(from, to, mover.Index, 100, 200, 5); err != nil {
		t.Fatal(err)
	}
	if src.Gold != g0-100 || src.Rice != r0-200 {
		t.Errorf("來源郡剩 %d 金 %d 米，應該是 %d／%d", src.Gold, src.Rice, g0-100, r0-200)
	}
	if dst.Gold != dg0+100 || dst.Rice != dr0+200 {
		t.Errorf("目的郡有 %d 金 %d 米，應該是 %d／%d", dst.Gold, dst.Rice, dg0+100, dr0+200)
	}
	if mover.Location != to {
		t.Errorf("將領還在郡 %d", mover.Location)
	}
}

// TestMoveNeedsAdjacency 釘住「調動只能到相鄰的己方州郡」。
func TestMoveNeedsAdjacency(t *testing.T) {
	g := newGame(t)
	if err := g.Move(14, 6, 0, 0, 0, 5); err == nil {
		t.Error("跨郡調動一個不存在的將領竟然成功")
	}
	// 6 上黨與 14 弘農都是董卓的，但相不相鄰要看資料。
	if g.Adjacent(14, 6) {
		t.Skip("弘農與上黨相鄰，這一條測不到")
	}
	for _, x := range g.Garrison(14) {
		if x.Faction == 5 && !x.Status.Governs() {
			if err := g.Move(14, 6, x.Index, 0, 0, 5); !errors.Is(err, ErrNotAdjacent) {
				t.Errorf("調到不相鄰的郡回 %v，應該是 ErrNotAdjacent", err)
			}
			return
		}
	}
}

// TestBuildFortRules 釘住建寨的三個條件（說明書 p.21）。
func TestBuildFortRules(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(15) // 洛陽
	p.Gold = MaxGold
	var dull, wise *General
	for _, x := range g.Garrison(15) {
		if x.Faction != 5 {
			continue
		}
		if x.Intel >= MinIntelForChief && wise == nil {
			wise = x
		}
		if x.Intel < MinIntelForChief && dull == nil {
			dull = x
		}
	}
	if dull != nil {
		if err := g.BuildFort(15, dull.Index, 5); !errors.Is(err, ErrNeedIntel80) {
			t.Errorf("謀略不足的人監工回 %v，應該是 ErrNeedIntel80", err)
		}
	}
	if wise == nil {
		t.Skip("洛陽沒有謀略 80 以上的人")
	}
	want := p.Gold - FortCost(p.PriceLevel)
	if err := g.BuildFort(15, wise.Index, 5); err != nil {
		t.Fatal(err)
	}
	if p.Gold != want {
		t.Errorf("建寨後庫銀 %d，應該是 %d（物價 %d × 100）", p.Gold, want, p.PriceLevel)
	}
	p.Forts = MaxForts
	p.Commanded = false
	if err := g.BuildFort(15, wise.Index, 5); !errors.Is(err, ErrTooManyForts) {
		t.Errorf("第六座城寨回 %v，應該是 ErrTooManyForts", err)
	}
}

// TestTradeCaps 釘住米糧與庫銀的上限（說明書 p.22：各 30000）。
func TestTradeCaps(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(8)
	p.Gold, p.Rice = MaxGold, 100
	if err := g.BuyRice(8, MaxRice, 0); err == nil {
		t.Error("買到超過糧倉上限竟然成功")
	}
	p.Commanded = false
	p.Rice = MaxRice
	p.Gold = MaxGold - 1
	if err := g.SellRice(8, MaxRice, 0); err != nil {
		t.Fatal(err)
	}
	if p.Gold != MaxGold {
		t.Errorf("賣完之後庫銀 %d，應該封頂在 %d", p.Gold, MaxGold)
	}
}

// TestChiefRules 釘住指定軍師的三個條件（說明書 p.23）。
func TestChiefRules(t *testing.T) {
	g := newGame(t)
	lord := g.Lord(5) // 董卓在弘農
	at := lord.Location
	if err := g.AppointChief(at, lord.Index, 5); !errors.Is(err, ErrLordCantBe) {
		t.Errorf("君主兼任軍師回 %v，應該是 ErrLordCantBe", err)
	}
	var dull, wise *General
	for _, x := range g.Garrison(at) {
		if x.Faction != 5 || x.Index == lord.Index {
			continue
		}
		if x.Intel >= MinIntelForChief && wise == nil {
			wise = x
		}
		if x.Intel < MinIntelForChief && dull == nil {
			dull = x
		}
	}
	if dull != nil {
		if err := g.AppointChief(at, dull.Index, 5); !errors.Is(err, ErrNeedIntel80) {
			t.Errorf("謀略不足回 %v，應該是 ErrNeedIntel80", err)
		}
	}
	if wise == nil {
		t.Skip("董卓所在地沒有謀略 80 以上的部將")
	}
	old := g.Chief(5)
	if err := g.AppointChief(at, wise.Index, 5); err != nil {
		t.Fatal(err)
	}
	if c := g.Chief(5); c == nil || c.Index != wise.Index {
		t.Errorf("軍師是 %v，應該是 %s", c, wise.Name)
	}
	// 同時只能有一位：前任要回任現役將領。
	if old != nil && old.Index != wise.Index && old.Status == state.StatusChief {
		t.Errorf("前任軍師 %s 還掛著軍師身分", old.Name)
	}
}

// TestGiftTreasure 釘住寶物的效果與上限（說明書 p.24）。
func TestGiftTreasure(t *testing.T) {
	g := newGame(t)
	lord := g.Lord(5)
	at := lord.Location
	f := g.Faction(5)
	f.Treasury[TreasureBook] = 2
	f.Treasury[TreasureSeal] = 1

	if err := g.GiftTreasure(at, lord.Index, TreasureSeal, 5); !errors.Is(err, ErrCantGift) {
		t.Errorf("送玉璽回 %v，應該是 ErrCantGift", err)
	}
	var target *General
	for _, x := range g.Garrison(at) {
		if x.Faction == 5 && x.Index != lord.Index && x.Intel < TreasureCap-2 {
			target = x
			break
		}
	}
	if target == nil {
		t.Skip("找不到謀略還有空間的部將")
	}
	before := target.Intel
	if err := g.GiftTreasure(at, target.Index, TreasureBook, 5); err != nil {
		t.Fatal(err)
	}
	if target.Intel != before+2 {
		t.Errorf("兵書之後謀略 %d，應該是 %d", target.Intel, before+2)
	}
	// 上限 90：已經高過的人不動。
	target.Intel = 95
	target.Rewarded = false
	p := g.Prefecture(at)
	p.Commanded = false
	if err := g.GiftTreasure(at, target.Index, TreasureBook, 5); err != nil {
		t.Fatal(err)
	}
	if target.Intel != 95 {
		t.Errorf("謀略 95 收到兵書變成 %d——賞賜上限不該把人拉低", target.Intel)
	}
}

// TestRewardOncePerMonth 釘住「各郡每月可賞每人一次」（說明書 p.23）。
func TestRewardOncePerMonth(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(8)
	p.Gold = MaxGold
	var target *General
	for _, x := range g.Garrison(8) {
		if x.Faction == 0 && x.HasLoyalty() {
			target = x
			break
		}
	}
	if target == nil {
		t.Skip("齊郡沒有可賞的人")
	}
	if err := g.Reward(8, target.Index, MaxReward, 0); err != nil {
		t.Fatal(err)
	}
	if err := g.Reward(8, target.Index, 10, 0); !errors.Is(err, ErrAlreadyPaid) {
		t.Errorf("同月第二次賞賜回 %v，應該是 ErrAlreadyPaid", err)
	}
	if err := g.Reward(8, target.Index, MaxReward+1, 0); err == nil {
		t.Error("賞金超過上限竟然成功")
	}
	g.EndMonth()
	if err := g.Reward(8, target.Index, 10, 0); err != nil {
		t.Errorf("換月之後還是不能賞：%v", err)
	}
}

// TestRedistributeAveragesWeighted 釘住「調整兵力後訓練度成為平均值」
// （說明書 p.21），而且是**以兵數加權**的平均。
func TestRedistributeAveragesWeighted(t *testing.T) {
	g := newGame(t)
	var us []*General
	for _, x := range g.Garrison(15) {
		if x.Faction == 5 {
			us = append(us, x)
		}
		if len(us) == 2 {
			break
		}
	}
	if len(us) < 2 {
		t.Skip("洛陽的守將不足兩位")
	}
	us[0].Soldiers, us[0].Training, us[0].Arms = 900, 100, 100
	us[1].Soldiers, us[1].Training, us[1].Arms = 100, 0, 0
	total := us[0].Soldiers + us[1].Soldiers
	if err := g.Redistribute(15, []int{us[0].Index, us[1].Index}, 5); err != nil {
		t.Fatal(err)
	}
	if us[0].Training != 90 || us[1].Training != 90 {
		t.Errorf("平均訓練度是 %d／%d，加權平均應該是 90",
			us[0].Training, us[1].Training)
	}
	if got := us[0].Soldiers + us[1].Soldiers; got != total {
		t.Errorf("重編之後總兵力 %d，原本 %d——兵員必須完全分配下去", got, total)
	}
}

// TestTrainGainMatchesTheOriginal 釘住訓練公式與原版的碼一致。
//
// 公式是從原版讀出來的（`L0`，`TrainGain` 的說明附了那幾行組語），
// 所以這裡釘的是**算式本身**，不是 remake 的手感。整數除法的截斷要
// 一起釘——`(智/3 + 武/2)/3` 與 `(智 + 1.5×武)/9` 差得出來。
func TestTrainGainMatchesTheOriginal(t *testing.T) {
	for _, c := range []struct{ intel, war, want int }{
		{0, 0, 0},
		{100, 100, 27}, // (33 + 50) / 3
		{80, 90, 23},   // (26 + 45) / 3
		{60, 60, 16},   // (20 + 30) / 3
		{10, 10, 2},    // (3 + 5) / 3
		{99, 1, 11},    // (33 + 0) / 3
		// **這一個是判準**：逐項截斷 (3+5)/3 ＝ 2，一次算完
		// (11 + 1.5×11)/9 ＝ 3.05 → 3。上面幾個案例兩種算法同值，
		// 分辨不出來。
		{11, 11, 2},
	} {
		got := TrainGain(c.intel, c.war, 5)
		if got != c.want {
			t.Errorf("智 %d 武 %d：訓練提升 %d，應該是 %d",
				c.intel, c.war, got, c.want)
		}
	}
	if got := TrainGain(100, 100, 5); got != (100/3+100/2)/3 {
		t.Errorf("整數除法沒有逐項截斷：%d", got)
	}

	// 除數由 AI 等級選，等級越高練得越快（`docs/re/03` §1.4）。
	for _, c := range []struct{ level, want int }{
		{0, 5}, {1, 5}, {2, 5}, {3, 4}, {4, 4}, {5, 3},
	} {
		if got := AITrainDivisor(c.level); got != c.want {
			t.Errorf("AI 等級 %d 的除數是 %d，應該是 %d", c.level, got, c.want)
		}
	}
	if TrainGain(80, 90, 5) <= TrainGain(80, 90, 0) {
		t.Error("等級高的練得應該比較快")
	}
}

// TestTrainZerosUnitsWithNoTroops 釘住沒有兵的人訓練度歸零。
//
// 原版的訓練常式在算增量之前先看兵士數：是零就把訓練度設成 0
// （`docs/re/03` §1.3，`L0`）。**說明書沒寫這一條**，而它會讓一支被
// 打光的部隊在補到兵之前一直是零訓練——影響戰力，不只是顯示。
func TestTrainZerosUnitsWithNoTroops(t *testing.T) {
	g := newGame(t)
	const pref = 15 // 洛陽
	var empty, manned *General
	for _, x := range g.Garrison(pref) {
		if x.Soldiers > 0 && manned == nil {
			manned = x
		}
	}
	if manned == nil {
		t.Skip("洛陽沒有帶兵的守將")
	}
	// 造一個沒有兵的守將出來。
	empty = manned
	for _, x := range g.Garrison(pref) {
		if x != manned {
			empty = x
			break
		}
	}
	if empty == manned {
		t.Skip("洛陽只有一位守將")
	}
	empty.Soldiers = 0
	empty.Training = 88
	empty.Arms = 77
	before := manned.Training

	if err := g.Train(pref, g.Prefecture(pref).Owner); err != nil {
		t.Fatal(err)
	}
	if empty.Training != 0 {
		t.Errorf("沒有兵的守將訓練度是 %d，應該被歸零", empty.Training)
	}
	// 武裝度同一個形狀（`0xc168` 的 `cmpw es:[bx+0x2226],0`）。
	if empty.Arms != 0 {
		t.Errorf("沒有兵的守將武裝度是 %d，應該被歸零", empty.Arms)
	}
	if manned.Training <= before {
		t.Errorf("帶兵的守將訓練度沒有提升（%d → %d）", before, manned.Training)
	}
}

// TestReclaimAndFloodMatchTheOriginal 釘住開墾與防洪的量與原版一致。
//
// **玩家與電腦走的是不同的常式**（`0x1a6d2`／`0x1a93a` 對上分派表
// `0x5534` 底下的六支）：玩家那條謀略不足就是加 0，電腦那條非正時
// 改擲 `RND(2)`。只測一邊會漏掉另一邊。
func TestReclaimAndFloodMatchTheOriginal(t *testing.T) {
	// 玩家：max(智 − 50, 0) / 12。
	for _, c := range []struct{ intel, want int }{
		{100, 4}, // (100−50)/12 = 4
		{74, 2},
		{62, 1},
		{61, 0},
		{10, 0}, // 智力低就是白做，不會變負也不會擲
	} {
		if got := ReclaimGain(c.intel); got != c.want {
			t.Errorf("玩家 智 %d：開墾 +%d，應該是 +%d", c.intel, got, c.want)
		}
	}
	// 電腦：底隨等級變，非正時擲 0 或 1。
	for _, c := range []struct{ intel, floor, roll, want int }{
		{100, 50, 0, 4},
		{100, 60, 0, 3}, // 等級 1／2 的底是 60
		{100, 40, 0, 5}, // 等級 4 的底是 40
		{61, 50, 0, 0},  // (61−50)/12 = 0 → 改用 RND(2)
		{61, 50, 1, 1},
		{10, 50, 1, 1}, // 智力很低也不會是負的
	} {
		if got := AIReclaimGain(c.intel, c.floor, c.roll); got != c.want {
			t.Errorf("電腦 智 %d、底 %d、擲 %d：開墾 +%d，應該是 +%d",
				c.intel, c.floor, c.roll, got, c.want)
		}
	}
	for _, c := range []struct{ intel, want int }{
		{100, 10}, {95, 9}, {50, 5}, {9, 0},
	} {
		if got := FloodDrop(c.intel); got != c.want {
			t.Errorf("玩家 智 %d：防洪 −%d，應該是 −%d", c.intel, got, c.want)
		}
	}
	// 六個等級的三個常數，逐位元組讀出來的。
	for level, want := range []AffairsTier{
		{4, 50, 10}, {4, 60, 15}, {4, 60, 15}, {3, 50, 14}, {3, 40, 12}, {2, 50, 10},
	} {
		if got := AffairsTierFor(level); got != want {
			t.Errorf("等級 %d 的內政常數 %+v，應該是 %+v", level, got, want)
		}
	}
	if AffairsTierFor(-1) != AffairsTierFor(0) || AffairsTierFor(99) != AffairsTierFor(5) {
		t.Error("等級越界沒有夾住")
	}
}

// TestArmsIsAPercentage 釘住武裝度是「有武器的兵的百分比」。
//
// 從原版的浮點序列還原出來的（`ArmsAfterPurchase`，`L0`）。
// **判準是稀釋**：不買武器而兵力變多，武裝度要下降——一個把武裝度
// 當成絕對數量的實作在「只買武器」的案例上看起來一模一樣，
// 只有兵力變動的案例分得出兩者。
func TestArmsIsAPercentage(t *testing.T) {
	for _, c := range []struct{ arms, soldiers, bought, want int }{
		{100, 1000, 0, 100}, // 全副武裝，什麼都不買
		{50, 1000, 500, 100},
		{50, 1000, 0, 50},
		{0, 1000, 1000, 100},
		{90, 1000, 500, 100}, // 買太多也不會超過 100
		{0, 0, 500, 0},       // 沒有兵就沒有武裝度
	} {
		if got := ArmsAfterPurchase(c.arms, c.soldiers, c.bought); got != c.want {
			t.Errorf("武裝 %d、兵力 %d、買 %d：算出 %d，應該是 %d",
				c.arms, c.soldiers, c.bought, got, c.want)
		}
	}
	// **稀釋**：武器數不變，兵力加倍，武裝度減半。
	w := Weapons(100, 1000)
	if got := ArmsOf(w, 2000); got != 50 {
		t.Errorf("兵力從 1000 加倍到 2000、武器 %d 不變，武裝度是 %d，應該是 50", w, got)
	}
}

// TestRiceRateFollowsThePrice 釘住「一金買到 (100 − 物價) ÷ 10 單位米」。
//
// **判準是方向與比例**：物價低買得多。原本 remake 用的是
// 「一單位米 ＝ 物價 ÷ 100 金」——那條在物價 50 給 2 單位/金，
// 原版給 5 單位/金，差了兩倍半，而且兩者都隨物價單調，
// 只驗一個點分不出來。
func TestRiceRateFollowsThePrice(t *testing.T) {
	for _, c := range []struct{ price, want int }{
		{30, 7}, {40, 6}, {50, 5}, {68, 3}, {95, 1}, {100, 1},
	} {
		if got := RicePerGold(uint8(c.price)); got != c.want {
			t.Errorf("物價 %d：一金買到 %d 單位，應該是 %d", c.price, got, c.want)
		}
	}

	g := newGame(t)
	p := g.Prefecture(8)
	p.PriceLevel, p.Gold, p.Rice = 50, 1000, 0
	// 想買 500 單位：一金 5 單位，所以花 100 金、拿到 500。
	if err := g.BuyRice(8, 500, 0); err != nil {
		t.Fatal(err)
	}
	if p.Gold != 900 || p.Rice != 500 {
		t.Errorf("物價 50 買 500 單位之後：金 %d、米 %d，應該是 900／500", p.Gold, p.Rice)
	}
	// **除不盡的零頭拿不到**：買 3 單位在一金 5 單位下花 0 金、拿 0。
	p.Commanded = false
	if err := g.BuyRice(8, 3, 0); err != nil {
		t.Fatal(err)
	}
	if p.Gold != 900 || p.Rice != 500 {
		t.Errorf("買不到一金份的零頭卻動了帳：金 %d、米 %d", p.Gold, p.Rice)
	}
}

// TestPlotScoreIsADuelOfWits 釘住計略是雙方謀略的對決（`L0`、`0x2dd66`）。
//
// **人望與使者魅力只扣分不加分**——到了 80／70 就封頂。
// 只驗「高人望比較容易成功」的話，這個封頂完全看不出來。
func TestPlotScoreIsADuelOfWits(t *testing.T) {
	// 軍師與君主取較高的那位。
	if got := PlotScore(90, 70, 80, 70); got != 90 {
		t.Errorf("軍師 90、君主 70 算出 %d，應該取 90", got)
	}
	if got := PlotScore(70, 95, 80, 70); got != 95 {
		t.Errorf("君主比較聰明時沒有取君主：%d", got)
	}
	// 人望 80 以上不加分。
	for _, p := range []int{80, 90, 100} {
		if got := PlotScore(90, 0, p, 70); got != 90 {
			t.Errorf("人望 %d 加了分：%d", p, got)
		}
	}
	// 人望不足才扣，每 10 點一分。
	if got := PlotScore(90, 0, 50, 70); got != 87 {
		t.Errorf("人望 50 算出 %d，應該是 90 + (50−80)/10 ＝ 87", got)
	}
	// 使者魅力 70 以上不加分，不足才扣，每 5 點一分。
	for _, c := range []int{70, 90, 100} {
		if got := PlotScore(90, 0, 80, c); got != 90 {
			t.Errorf("使者魅力 %d 加了分：%d", c, got)
		}
	}
	if got := PlotScore(90, 0, 80, 50); got != 86 {
		t.Errorf("使者魅力 50 算出 %d，應該是 90 + (50−70)/5 ＝ 86", got)
	}
}

// TestSabotageHitsFiveFields 釘住計略得手之後五個欄位都動（`L0`、`0x2d6e0`）。
//
// **判準是「五個都動」**：原版沒有把它們拆成不同的計謀，漏掉任何一個
// 都會讓被計的郡比原版好過，而那在對拍裡只會表現成幾個欄位對不上。
func TestSabotageHitsFiveFields(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(8)
	p.PublicLoyalty, p.FloodRate, p.LandValue = 90, 10, 90
	p.Rice, p.Gold = 10000, 10000
	before := *p
	g.Sabotage(8, 100)
	if p.PublicLoyalty >= before.PublicLoyalty {
		t.Errorf("民眾忠誠沒降：%d → %d", before.PublicLoyalty, p.PublicLoyalty)
	}
	if p.FloodRate <= before.FloodRate {
		t.Errorf("洪水率沒升：%d → %d", before.FloodRate, p.FloodRate)
	}
	if p.LandValue >= before.LandValue {
		t.Errorf("土地價值沒降：%d → %d", before.LandValue, p.LandValue)
	}
	if p.Rice >= before.Rice {
		t.Errorf("米沒少：%d → %d", before.Rice, p.Rice)
	}
	if p.Gold >= before.Gold {
		t.Errorf("金沒少：%d → %d", before.Gold, p.Gold)
	}
	// **米的比例比金重**：除數 300 對 500。
	if before.Rice-p.Rice <= before.Gold-p.Gold {
		t.Errorf("同樣的存量下米燒得不比金多：米 −%d、金 −%d",
			before.Rice-p.Rice, before.Gold-p.Gold)
	}
}

// TestBuildFortUpdatesTheMap 釘住蓋關寨會同時動到數量與地圖。
//
// **兩份記錄不能分家**：原版的 offset 25 永遠等於地圖上關寨格的數目
// （`state.TestFortCountMatchesTheField`）。只加數字的話，戰場上不會
// 多出那座關寨，而玩家付了一整筆錢。
func TestBuildFortUpdatesTheMap(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(11) // 陳留
	var who *General
	for _, x := range g.Garrison(11) {
		if x.Faction == p.Owner {
			x.Intel = 100
			who = x
			break
		}
	}
	if who == nil {
		t.Fatal("陳留沒有人可以監工")
	}
	count := func() int {
		n := 0
		for _, b := range p.BattleField {
			if b != 0xFF && b&0x0F == fortTerrain {
				n++
			}
		}
		return n
	}
	p.Gold = MaxGold
	before2 := append([]byte(nil), p.BattleField...)
	before, onMap := p.Forts, count()
	if before != onMap {
		t.Fatalf("開局就分家：關寨數 %d、地圖上 %d", before, onMap)
	}
	if err := g.BuildFort(11, who.Index, p.Owner); err != nil {
		t.Fatal(err)
	}
	if p.Forts != before+1 {
		t.Errorf("關寨數 %d，應該是 %d", p.Forts, before+1)
	}
	if got := count(); got != onMap+1 {
		t.Errorf("地圖上有 %d 格關寨，應該是 %d", got, onMap+1)
	}
	// **新蓋的那一格必須是沒有標記的平原**。原版的地圖上本來就有
	// 帶著軍團起點標記的關寨（陳留的第 65、88 格），所以判準是
	// 「新增的那一格」而不是「所有關寨格」——蓋在通道上會把鄰郡
	// 從地圖上封死，而那只看得出「敵軍再也沒有從那一邊來過」。
	added := -1
	for i := range p.BattleField {
		if p.BattleField[i] != before2[i] {
			if added >= 0 {
				t.Fatalf("動到不只一格：%d 與 %d", added, i)
			}
			added = i
		}
	}
	if added < 0 {
		t.Fatal("地圖沒有任何一格被改到")
	}
	if b := p.BattleField[added]; b>>4 != 15 || b&0x0F != fortTerrain {
		t.Errorf("新蓋的第 %d 格是 %#02x，應該是沒有標記的關寨", added, b)
	}
	if before2[added]&0x0F != 7 {
		t.Errorf("新蓋的第 %d 格原本是地形 %d，應該是平原", added, before2[added]&0x0F)
	}
}

// TestRiceTradeIsSymmetric 釘住買賣米走同一條比率。
//
// 原版的提示字串把這件事寫在臉上：買米問「1 金 = %d 米」，賣米問
// 「%d 米 = 1 金」。**同一個月買進再賣出不該憑空生出錢**——比率各走
// 各的話，來回操作就是一台印鈔機，而那要玩上幾十回合才會看出來。
func TestRiceTradeIsSymmetric(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(8)
	p.PriceLevel = 50 // rate = (100−50)/10 = 5
	rate := RicePerGold(p.PriceLevel)
	if rate != 5 {
		t.Fatalf("物價 50 的比率是 %d，應該是 5", rate)
	}
	p.Gold, p.Rice = 1000, 1000
	if err := g.BuyRice(8, 100, 0); err != nil { // 100 米 = 20 金
		t.Fatal(err)
	}
	if p.Gold != 980 || p.Rice != 1100 {
		t.Fatalf("買 100 米之後 金 %d 米 %d，應該是 980／1100", p.Gold, p.Rice)
	}
	g.EndMonth()
	p.PriceLevel = 50
	if err := g.SellRice(8, 100, 0); err != nil {
		t.Fatal(err)
	}
	if p.Gold != 1000 || p.Rice != 1000 {
		t.Errorf("賣回去之後 金 %d 米 %d，應該回到 1000／1000", p.Gold, p.Rice)
	}
	// 換不到一金的零頭留在倉裡。
	g.EndMonth()
	p.PriceLevel = 50
	if err := g.SellRice(8, rate-1, 0); err != nil {
		t.Fatal(err)
	}
	if p.Gold != 1000 || p.Rice != 1000 {
		t.Errorf("賣 %d 米之後 金 %d 米 %d，零頭不該換到錢",
			rate-1, p.Gold, p.Rice)
	}
}

// TestAutonomyMapsToAILevels 釘住自治型態與 AI 等級的對應。
//
// 原版把型態存在州郡 offset 12（0 正常、1 內政、2 軍事、3 自冶），
// 郡的回合入口拿 `值 − 1` 當 AI 等級去跑分派器（`0x17572`）。
// **君主在的郡不自治**——主公親自坐鎮的地方輪不到太守自作主張。
func TestAutonomyMapsToAILevels(t *testing.T) {
	for _, c := range []struct {
		mode  Autonomy
		level int
		auto  bool
	}{
		{AutoNormal, 0, false},
		{AutoCivil, 0, true},
		{AutoMilitary, 1, true},
		{AutoSelf, 2, true},
	} {
		level, ok := AutonomyAILevel(c.mode)
		if ok != c.auto || (ok && level != c.level) {
			t.Errorf("%s：等級 %d／代管 %v，應該是 %d／%v",
				c.mode, level, ok, c.level, c.auto)
		}
	}

	g := newGame(t)
	lord := g.Lord(5) // 董卓
	at := lord.Location
	if err := g.SetAutonomy(at, AutoMilitary, 5); err != nil {
		t.Fatal(err)
	}
	if _, ok := g.AutonomousFor(at); ok {
		t.Error("君主所在的郡不該自治")
	}
	// 換一個沒有君主的郡。
	other := 0
	for _, n := range g.Prefecture(at).Neighbours {
		if q := g.Prefecture(n); q.Owned() && q.Owner == 5 {
			other = n
			break
		}
	}
	if other == 0 {
		t.Fatal("董卓沒有第二個郡")
	}
	if err := g.SetAutonomy(other, AutoSelf, 5); err != nil {
		t.Fatal(err)
	}
	level, ok := g.AutonomousFor(other)
	if !ok || level != 2 {
		t.Errorf("自冶的郡回 %d／%v，應該是 2／true", level, ok)
	}
}

// TestRiceRateVariesByAILevel 釘住買米的匯率隨 AI 等級變。
//
// 玩家永遠是 `(100 − 物價) ÷ 10`；電腦那六個呼叫端的除數是
// `[10,10,10,10,9,8]`。第六支寫成 `sar cl,ax`（`cl` ＝ 3）而不是
// `idiv`，所以除數是 `2³ ＝ 8` 不是 3——只用玩家那條算式
// 會把等級 4 與 5 的糧倉低估一截。
func TestRiceRateVariesByAILevel(t *testing.T) {
	const price = 50 // (100−50) = 50
	if got := RicePerGold(price); got != 5 {
		t.Errorf("玩家 物價 50：一金換 %d 米，應該是 5", got)
	}
	for level, want := range []int{5, 5, 5, 5, 5, 6} {
		if got := AIRicePerGold(price, level); got != want {
			t.Errorf("電腦等級 %d 物價 50：一金換 %d 米，應該是 %d",
				level, got, want)
		}
	}
	if AIRicePerGold(price, -1) != AIRicePerGold(price, 0) ||
		AIRicePerGold(price, 99) != AIRicePerGold(price, 5) {
		t.Error("等級越界沒有夾住")
	}
	// 物價高到讓商數掉到 1 以下時仍然給 1（原版沒擋，remake 加的下限）。
	if got := AIRicePerGold(99, 0); got != 1 {
		t.Errorf("物價 99：一金換 %d 米，下限應該是 1", got)
	}
}

// TestCanBuildFortOn 釘住原版的兩道門（`0x1aeba`–`0x1aed1`，`L0`）。
//
// 地形要在 `DS:0x7134` 那張表裡是 1（山丘 2、平原 7、樹林 8），
// 而且高四位要 ≥ 10——**高四位 0–9 是通往鄰郡的通道**，蓋在那裡會把
// 鄰郡從地圖上封死。
func TestCanBuildFortOn(t *testing.T) {
	cases := []struct {
		cell byte
		want bool
		why  string
	}{
		{0xFF, false, "圖外"},
		{0xF7, true, "沒標記的平原"},
		{0xF2, true, "沒標記的山丘"},
		{0xF8, true, "沒標記的樹林"},
		{0xF1, false, "大山"},
		{0xF3, false, "淺水"},
		{0xF4, false, "深水"},
		{0xF5, false, "城池"},
		{0xF6, false, "已經是關寨"},
		{0xF9, false, "沙漠"},
		{0x07, false, "通往第 0 個鄰郡的平原"},
		{0x97, false, "通往第 9 個鄰郡的平原"},
		{0xA7, true, "城池標記的平原"},
		{0xB7, true, "軍團起點的平原"},
	}
	for _, c := range cases {
		if got := CanBuildFortOn(c.cell); got != c.want {
			t.Errorf("格 0x%02X（%s）回 %v，應該是 %v", c.cell, c.why, got, c.want)
		}
	}
}

// TestBuildFortAcceptsHillsAndWoods 釘住「平原被佔滿了還是蓋得起來」。
//
// remake 原本只挑沒有標記的平原，而原版收山丘與樹林。差別在平原用完
// 的地圖上會變成「remake 說蓋不了、原版蓋得起來」。
func TestBuildFortAcceptsHillsAndWoods(t *testing.T) {
	g := newGame(t)
	var p *Prefecture
	for i := range g.prefectures {
		if g.prefectures[i].Owned() && len(g.prefectures[i].BattleField) > 0 {
			p = &g.prefectures[i]
			break
		}
	}
	if p == nil {
		t.Fatal("找不到有地圖的郡")
	}
	// 把地圖擺成「一格樹林，其餘全是大山」——沒有平原。
	for i := range p.BattleField {
		p.BattleField[i] = 0xF1 // 沒標記的大山
	}
	p.BattleField[7] = 0xF8 // 樹林
	p.Forts = 0
	p.Gold = 30000
	p.PriceLevel = 50
	who := (*General)(nil)
	for _, x := range g.Garrison(p.ID) {
		if x.Intel > MinIntelForChief {
			who = x
			break
		}
	}
	if who == nil {
		x := g.Garrison(p.ID)
		if len(x) == 0 {
			t.Skip("這個郡沒有駐軍")
		}
		who = x[0]
		who.Intel = 90
	}
	p.Commanded = false
	if err := g.BuildFort(p.ID, who.Index, p.Owner); err != nil {
		t.Fatalf("只有樹林可以蓋時 BuildFort 回 %v", err)
	}
	if p.BattleField[7]&0x0F != fortTerrain {
		t.Errorf("樹林那一格是 0x%02X，應該變成關寨", p.BattleField[7])
	}
	if p.BattleField[7]>>4 != 15 {
		t.Errorf("高四位被動到了：0x%02X", p.BattleField[7])
	}
	if p.Forts != 1 {
		t.Errorf("關寨數是 %d", p.Forts)
	}
}
