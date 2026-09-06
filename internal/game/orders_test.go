package game

import (
	"errors"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func newGame(t *testing.T) *State {
	t.Helper()
	sc := loadScenario(t, state.Scenario1)
	g, err := New(sc, 0, 5) // 劉備
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestOrdersRejectOtherPeoplesLand 釘住「不是你的郡不能下令」。
func TestOrdersRejectOtherPeoplesLand(t *testing.T) {
	g := newGame(t)
	if err := g.Reclaim(11, -1, 0); !errors.Is(err, ErrNotYours) { // 陳留是曹操的
		t.Errorf("對別人的郡開墾回 %v，應該是 ErrNotYours", err)
	}
	if err := g.Reclaim(1, -1, 0); !errors.Is(err, ErrNotYours) { // 遼東無主
		t.Errorf("對空白郡開墾回 %v，應該是 ErrNotYours", err)
	}
}

// TestOneOrderPerMonth 釘住「每郡每月一次」（說明書 p.17）。
func TestOneOrderPerMonth(t *testing.T) {
	g := newGame(t)
	if err := g.Reclaim(8, -1, 0); err != nil {
		t.Fatalf("第一次開墾就失敗：%v", err)
	}
	if err := g.Reclaim(8, -1, 0); !errors.Is(err, ErrAlreadyMoved) {
		t.Errorf("同月第二次開墾回 %v，應該是 ErrAlreadyMoved", err)
	}
	g.EndMonth()
	if err := g.Reclaim(8, -1, 0); err != nil {
		t.Errorf("換月之後開墾還是失敗：%v", err)
	}
}

// TestReclaimCostsGold 釘住花費。
func TestReclaimCostsGold(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(8)
	before, land := p.Gold, p.LandValue
	if err := g.Reclaim(8, -1, 0); err != nil {
		t.Fatal(err)
	}
	if p.Gold != before-CostReclaim {
		t.Errorf("開墾後庫銀 %d，應該是 %d", p.Gold, before-CostReclaim)
	}
	if p.LandValue <= land {
		t.Errorf("開墾後土地價值 %d 沒有比 %d 高", p.LandValue, land)
	}
	// 「若財庫已空則徒手開墾」（說明書 p.21）——**錢不夠不是失敗**，
	// 只是效果減半。這與防洪不同（防洪「沒錢就不能修浚」）。
	g.EndMonth()
	p.Gold = CostReclaim - 1
	land2 := p.LandValue
	if err := g.Reclaim(8, -1, 0); err != nil {
		t.Errorf("財庫空時開墾回 %v，應該還是能徒手開墾", err)
	}
	if p.LandValue <= land2 {
		t.Errorf("徒手開墾之後土地價值 %d 沒有比 %d 高", p.LandValue, land2)
	}
	g.EndMonth()
	p.Gold = CostFloodControl - 1
	if err := g.FloodControl(8, -1, 0); !errors.Is(err, ErrNoGold) {
		t.Errorf("錢不夠時防洪回 %v，應該是 ErrNoGold", err)
	}
}

// TestConscriptRespectsCap 釘住帶兵上限與花費。
//
// 上限是**兩個獨立來源同意的數字**（說明書 p.18 ＋ 原版資料），
// 所以這一條擋得住「多募一點應該沒關係」這種改動。
func TestConscriptRespectsCap(t *testing.T) {
	g := newGame(t)
	lord := g.Lord(0)
	if lord == nil {
		t.Fatal("找不到劉備")
	}
	p := g.Prefecture(8)
	p.Gold = MaxGold // 錢不是這一條要測的

	room := lord.TroopCap() - lord.Soldiers
	if err := g.Conscript(8, lord.Index, room+1, 0); !errors.Is(err, ErrNoRoom) {
		t.Errorf("募到超過上限回 %v，應該是 ErrNoRoom", err)
	}
	before := g.Soldiers(8)
	if err := g.Conscript(8, lord.Index, room, 0); err != nil {
		t.Fatalf("募到剛好上限卻失敗：%v", err)
	}
	if lord.Soldiers != lord.TroopCap() {
		t.Errorf("募完之後 %s 有 %d 兵，應該是 %d", lord.Name, lord.Soldiers, lord.TroopCap())
	}
	if got := g.Soldiers(8); got != before+room {
		t.Errorf("郡的總兵力 %d，應該是 %d", got, before+room)
	}
	if p.Gold != MaxGold-room*CostConscriptPerSoldier {
		t.Errorf("募完之後庫銀 %d，應該是 %d", p.Gold, MaxGold-room*CostConscriptPerSoldier)
	}
}

// TestConscriptNeedsPeople 釘住人口下限。
//
// 手冊寫的是「**不得因徵兵而導致**人口少於 3000 人」（p.20）——
// 所以徵兵會減人口，而且判準是**徵完之後**的人口，不是徵之前的。
// 人口剛好 3000 時一個兵都徵不了。
func TestConscriptNeedsPeople(t *testing.T) {
	g := newGame(t)
	lord := g.Lord(0)
	p := g.Prefecture(8)
	p.Gold = MaxGold

	p.Population = MinPopulationToConscript
	if err := g.Conscript(8, lord.Index, 1, 0); !errors.Is(err, ErrNoPeople) {
		t.Errorf("人口剛好 %d 時徵 1 人回 %v，應該是 ErrNoPeople", p.Population, err)
	}
	p.Population = MinPopulationToConscript + 10
	if err := g.Conscript(8, lord.Index, 11, 0); !errors.Is(err, ErrNoPeople) {
		t.Errorf("徵完會低於下限卻沒有擋下來")
	}
	before := p.Population
	if err := g.Conscript(8, lord.Index, 10, 0); err != nil {
		t.Fatalf("徵到剛好剩下限卻失敗：%v", err)
	}
	if p.Population != before-10 {
		t.Errorf("徵 10 人之後人口 %d，應該是 %d——徵兵要減人口", p.Population, before-10)
	}
	if p.Population != MinPopulationToConscript {
		t.Errorf("剩下的人口是 %d，應該剛好是下限 %d", p.Population, MinPopulationToConscript)
	}
}

// TestConscriptLowersTraining 釘住「新兵毫無訓練，會把部隊拉低」（說明書 p.20）。
func TestConscriptLowersTraining(t *testing.T) {
	g := newGame(t)
	lord := g.Lord(0)
	p := g.Prefecture(8)
	p.Gold = MaxGold
	p.Population = 100000
	lord.Soldiers = 100
	lord.Training = 80
	lord.Arms = 60

	if err := g.Conscript(8, lord.Index, 100, 0); err != nil {
		t.Fatal(err)
	}
	// 一百個訓練度 80 的老兵加一百個 0 的新兵 → 40。
	if lord.Training != 40 {
		t.Errorf("徵兵後訓練度 %d，應該是 40（加權平均）", lord.Training)
	}
	if lord.Arms != 30 {
		t.Errorf("徵兵後武裝度 %d，應該是 30（加權平均）", lord.Arms)
	}
}

// TestConscriptRejectsForeignGeneral 釘住「只能命令自己人、而且要在當地」。
func TestConscriptRejectsForeignGeneral(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(8)
	p.Gold = MaxGold
	caocao := g.Lord(1)
	if caocao == nil {
		t.Fatal("找不到曹操")
	}
	if err := g.Conscript(8, caocao.Index, 1, 0); !errors.Is(err, ErrUnknownUnit) {
		t.Errorf("命令曹操募兵回 %v，應該是 ErrUnknownUnit", err)
	}
}
