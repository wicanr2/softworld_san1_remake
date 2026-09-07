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
// 兩個都是從碼讀出來的（`ReclaimGain`／`FloodDrop`，`L0`）。
// **開墾那條的重點是「智力低不會變負」**：呼叫端算的是 `(智−50)/12`，
// 而常式先擋掉非正的量、改成擲 0 或 1。只釘高智力的案例會漏掉這一段。
func TestReclaimAndFloodMatchTheOriginal(t *testing.T) {
	for _, c := range []struct{ intel, roll, want int }{
		{100, 0, 4}, // (100−50)/12 = 4
		{74, 1, 2},  // (74−50)/12 = 2
		{62, 0, 1},  // (62−50)/12 = 1
		{61, 0, 0},  // (61−50)/12 = 0 → 改用 RND(2)
		{61, 1, 1},
		{10, 1, 1}, // 智力很低也不會是負的
		{10, 0, 0},
	} {
		if got := ReclaimGain(c.intel, c.roll); got != c.want {
			t.Errorf("智 %d、擲 %d：開墾 +%d，應該是 +%d",
				c.intel, c.roll, got, c.want)
		}
	}
	for _, c := range []struct{ intel, want int }{
		{100, 10}, {95, 9}, {50, 5}, {9, 0},
	} {
		if got := FloodDrop(c.intel); got != c.want {
			t.Errorf("智 %d：防洪 −%d，應該是 −%d", c.intel, got, c.want)
		}
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
