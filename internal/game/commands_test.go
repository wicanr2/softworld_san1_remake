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
	g.syncSoldiers(dz.Location)
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
//（說明書 p.21），而且是**以兵數加權**的平均。
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
