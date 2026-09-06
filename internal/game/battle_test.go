package game

import (
	"errors"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func attackersAt(g *State, id int, f state.FactionID, keepOne bool) []int {
	var out []int
	all := g.Garrison(id)
	for _, x := range all {
		if x.Faction == f {
			out = append(out, x.Index)
		}
	}
	if keepOne && len(out) > 1 {
		out = out[:len(out)-1]
	}
	return out
}

// TestAttackNeedsAdjacency 釘住「由該州郡獨力進犯**鄰郡**」（說明書 p.19）。
func TestAttackNeedsAdjacency(t *testing.T) {
	g := newGame(t)
	att := attackersAt(g, 15, 5, true)
	if len(att) == 0 {
		t.Skip("洛陽沒有可出征的守將")
	}
	// 找一個不相鄰的郡。
	to := 0
	for id := 1; id <= state.PrefectureCount; id++ {
		if id != 15 && !g.Adjacent(15, id) {
			to = id
			break
		}
	}
	if to == 0 {
		t.Skip("洛陽與所有郡都相鄰")
	}
	if _, err := g.Attack(15, to, att, 5); !errors.Is(err, ErrNotAdjacent) {
		t.Errorf("打不相鄰的郡回 %v，應該是 ErrNotAdjacent", err)
	}
}

// TestAttackCannotLeaveNobody 釘住「傾巢而出會讓原郡沒人治理」。
func TestAttackCannotLeaveNobody(t *testing.T) {
	g := newGame(t)
	all := attackersAt(g, 15, 5, false)
	if len(all) == 0 {
		t.Skip("洛陽沒有守將")
	}
	target := g.Prefecture(15).Neighbours[0]
	if _, err := g.Attack(15, target, all, 5); !errors.Is(err, ErrNoGovernor) {
		t.Errorf("全員出征回 %v，應該是 ErrNoGovernor", err)
	}
}

// TestAttackTakesPrefecture 釘住「進攻順利則軍隊駐進被攻下的州郡」
//（說明書 p.19），以及守軍潰散成當地在野將領。
func TestAttackTakesPrefecture(t *testing.T) {
	g := newGame(t)
	// 讓洛陽打一個空白鄰郡：無主的郡沒有守軍，必勝。
	var empty int
	for _, n := range g.Prefecture(15).Neighbours {
		if !g.Prefecture(n).Owned() {
			empty = n
			break
		}
	}
	if empty == 0 {
		t.Skip("洛陽沒有空白鄰郡")
	}
	att := attackersAt(g, 15, 5, true)
	if len(att) == 0 {
		t.Skip("洛陽沒有可出征的守將")
	}
	r, err := g.Attack(15, empty, att, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !r.AttackerWon || !r.PrefectureTook {
		t.Fatalf("打空白郡竟然沒贏：%+v", r)
	}
	if p := g.Prefecture(empty); p.Owner != 5 {
		t.Errorf("攻下之後郡 %d 屬於 %d，應該是 5", empty, p.Owner)
	}
	if g.Governor(empty) == nil {
		t.Error("攻下之後沒有主事者")
	}
	// 總兵力要與駐軍加總一致。
	sum := 0
	for _, x := range g.Garrison(empty) {
		sum += x.Soldiers
	}
	if got := g.Soldiers(empty); got != sum {
		t.Errorf("郡的總兵力 %d，駐軍加總 %d——兩個數字分家了", got, sum)
	}
}

// TestDefenderBonus 釘住守方的地利：同樣的部隊，守方應該比攻方強。
func TestDefenderBonus(t *testing.T) {
	g := newGame(t)
	a := g.Garrison(15)[0]
	d := g.Garrison(14)[0]
	a.Soldiers, a.Training, a.Arms, a.War = 1000, 50, 50, 50
	d.Soldiers, d.Training, d.Arms, d.War = 1000, 50, 50, 50
	if unitPower(a) != unitPower(d) {
		t.Fatal("兩支部隊的基礎戰力應該相同")
	}
	p := g.Prefecture(14)
	p.Forts = 2
	want := unitPower(d) * (100 + TuneDefenceBonus + 2*TuneFortBonus) / 100
	if want <= unitPower(a) {
		t.Errorf("守方加成之後 %d，沒有比攻方 %d 高", want, unitPower(a))
	}
}

// TestLordCaptiveDisposal 釘住「諸侯被擒只能斬首或釋放」（說明書 p.35）。
func TestLordCaptiveDisposal(t *testing.T) {
	g := newGame(t)
	lord := g.Lord(13) // 孔融
	if lord == nil {
		t.Skip("找不到孔融")
	}
	at := lord.Location
	if err := g.DisposeCaptive(at, lord.Index, Imprison, 5); err == nil {
		t.Error("囚禁諸侯竟然可以")
	}
	if err := g.DisposeCaptive(at, lord.Index, Enlist, 5); err == nil {
		t.Error("招降諸侯竟然可以")
	}
	if err := g.DisposeCaptive(at, lord.Index, Release, 5); err != nil {
		t.Errorf("釋放諸侯失敗：%v", err)
	}
}

// TestPlotNeedsChief 釘住「諸侯拜封軍師後才能用計」（說明書 p.24）。
func TestPlotNeedsChief(t *testing.T) {
	g := newGame(t)
	f := g.Faction(0) // 劉備開局沒有軍師
	if f.Chief >= 0 {
		t.Skip("劉備開局就有軍師")
	}
	lord := g.Lord(0)
	if _, err := g.UsePlot(lord.Location, 11, PlotForgery, lord.Index, 0); !errors.Is(err, ErrNoChief) {
		t.Errorf("沒有軍師卻能用計，回 %v", err)
	}
}

// TestPlotCostsGold 釘住用計要花錢，而且只能在軍師或諸侯所在地。
func TestPlotCostsGold(t *testing.T) {
	g := newGame(t)
	lord := g.Lord(5) // 董卓
	at := lord.Location
	// 隨便指一位謀略夠的人當軍師。
	var wise *General
	for _, x := range g.Garrison(at) {
		if x.Faction == 5 && x.Index != lord.Index && x.Intel >= MinIntelForChief {
			wise = x
			break
		}
	}
	if wise == nil {
		t.Skip("董卓所在地沒有謀略 80 以上的部將")
	}
	if err := g.AppointChief(at, wise.Index, 5); err != nil {
		t.Fatal(err)
	}
	p := g.Prefecture(at)
	p.Commanded = false
	p.Gold = MaxGold
	target := 0
	for _, n := range p.Neighbours {
		if q := g.Prefecture(n); q.Owned() && q.Owner != 5 {
			target = n
			break
		}
	}
	if target == 0 {
		t.Skip("董卓所在地沒有敵方鄰郡")
	}
	before := p.Gold
	if _, err := g.UsePlot(at, target, PlotForgery, wise.Index, 5); err != nil {
		t.Fatal(err)
	}
	if p.Gold != before-PlotCost(PlotForgery) {
		t.Errorf("用計之後庫銀 %d，應該是 %d", p.Gold, before-PlotCost(PlotForgery))
	}
}
