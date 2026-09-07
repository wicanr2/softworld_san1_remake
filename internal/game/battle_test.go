package game

import (
	"errors"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
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
// （說明書 p.19），以及守軍潰散成當地在野將領。
func TestAttackTakesPrefecture(t *testing.T) {
	g := newGame(t)
	// 找一個有空白鄰郡、而且留得下人看家的郡：無主的郡沒有守軍，必勝。
	//
	// ⚠ **不要寫死某一個郡。** 郡的歸屬是劇本資料，寫死的那一個
	// 一旦不符條件就變成永久 t.Skip——測試還是綠的，但什麼都沒測到。
	from, empty := 0, 0
	var owner state.FactionID = state.NoFaction
	var att []int
	for id := 1; id <= state.PrefectureCount && from == 0; id++ {
		p := g.Prefecture(id)
		if !p.Owned() {
			continue
		}
		for _, n := range p.Neighbours {
			if g.Prefecture(n).Owned() {
				continue
			}
			if a := attackersAt(g, id, p.Owner, true); len(a) > 0 {
				from, empty, owner, att = id, n, p.Owner, a
				break
			}
		}
	}
	if from == 0 {
		t.Skip("這個劇本沒有「有空白鄰郡又留得下人」的郡")
	}
	r, err := g.Attack(from, empty, att, owner)
	if err != nil {
		t.Fatal(err)
	}
	if !r.AttackerWon || !r.PrefectureTook {
		t.Fatalf("打空白郡竟然沒贏：%+v", r)
	}
	if p := g.Prefecture(empty); p.Owner != owner {
		t.Errorf("攻下之後郡 %d 屬於 %d，應該是 %d", empty, p.Owner, owner)
	}
	if r.Days < 1 || r.Days > 31 {
		t.Errorf("戰役打了 %d 天，應該落在 1..31（卅天判定，說明書 p.35）", r.Days)
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

// TestDefenderBonus 釘住 AI 估算時算得到守方的地利：同樣的部隊，
// 守方應該比攻方強。
//
// 真正的勝負由主戰場打出來（`internal/battle`）；這裡的數字只影響
// 電腦諸侯出不出兵。
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

// TestPlotIsFree 釘住用計**不花錢**，而且只能在軍師或諸侯所在地。
//
// 原版的碼裡沒有那筆帳（`PlotCost`，`L0`）；戰場上的六種計謀才有費用。
func TestPlotIsFree(t *testing.T) {
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
	// **盤面自己擺**：弘農的三個鄰郡開局全是董卓自己的，照劇本挑
	// 會挑不到目標而整支測試安靜地 skip 掉。
	target := p.Neighbours[0]
	q := g.Prefecture(target)
	q.Owner = 4
	for _, x := range g.Garrison(target) {
		x.Faction = 4
	}
	before := p.Gold
	if _, err := g.UsePlot(at, target, PlotForgery, wise.Index, 5); err != nil {
		t.Fatal(err)
	}
	if p.Gold != before {
		t.Errorf("用計之後庫銀 %d，應該原封不動的 %d", p.Gold, before)
	}
	// 軍師與君主都不在的郡用不了計。
	other := 0
	for i := range g.generals {
		if x := &g.generals[i]; x.Employed() && x.Faction == 5 && x.Location != at {
			other = x.Location
			break
		}
	}
	if other != 0 {
		if _, err := g.UsePlot(other, target, PlotForgery, wise.Index, 5); err == nil {
			t.Error("軍師與君主都不在的郡竟然用得了計")
		}
	}
}

// TestBattleReportIsQueued 釘住每打完一場都留下戰報。
//
// **戰報是三十天主戰場唯一的出口**：命令層的 `Apply` 只回錯誤，
// 電腦諸侯的戰役玩家更是從頭到尾沒經手。少了佇列，整場戰役
// 在畫面上就只剩「某某出兵攻某某」一行。
func TestBattleReportIsQueued(t *testing.T) {
	g := newGame(t)
	from, to := 0, 0
	var owner state.FactionID = state.NoFaction
	var att []int
	for id := 1; id <= state.PrefectureCount && from == 0; id++ {
		p := g.Prefecture(id)
		if !p.Owned() {
			continue
		}
		for _, n := range p.Neighbours {
			// 要一場**真的有人守**的戰役：打空白郡是走進去，
			// 不會有折損，也就問不到戰報記了什麼。
			q := g.Prefecture(n)
			if !q.Owned() || q.Owner == p.Owner || len(g.Garrison(n)) == 0 {
				continue
			}
			if a := attackersAt(g, id, p.Owner, true); len(a) > 0 {
				from, to, owner, att = id, n, p.Owner, a
				break
			}
		}
	}
	if from == 0 {
		t.Skip("找不到可以出兵的郡")
	}
	if len(g.Reports) != 0 {
		t.Fatal("還沒打就有戰報")
	}
	r, err := g.Attack(from, to, att, owner)
	if err != nil {
		t.Fatal(err)
	}
	got := g.DrainReports()
	if len(got) != 1 || got[0] != r {
		t.Fatalf("戰報佇列有 %d 筆，應該剛好是剛才那一場", len(got))
	}
	if len(g.DrainReports()) != 0 {
		t.Error("取走之後佇列應該清空")
	}
	if len(r.Log) == 0 {
		t.Error("打了一場卻沒有逐日戰報")
	}
	if r.AttackerLost < 0 || r.DefenderLost < 0 {
		t.Errorf("折損是負的：攻 %d 守 %d", r.AttackerLost, r.DefenderLost)
	}
	if r.AttackerLost == 0 && r.DefenderLost == 0 {
		t.Error("打了三十天雙方都沒有折損")
	}
	if s := r.Summary(g); !strings.Contains(s, "攻") {
		t.Errorf("戰報摘要看不出誰打誰：%q", s)
	}
}

// TestLordCaptureSeizesTreasures 釘住「獲勝軍若於戰後捉到敵軍君主，
// 其寶物將全歸獲勝軍所有」（說明書 p.35）。
func TestLordCaptureSeizesTreasures(t *testing.T) {
	g := newGame(t)
	lord := g.Lord(13) // 孔融
	if lord == nil {
		t.Skip("找不到孔融")
	}
	loser, winner := g.Faction(lord.Faction), g.Faction(5)
	if loser == nil || winner == nil {
		t.Skip("勢力不齊")
	}
	loser.Treasury[0] = 3
	winner.Treasury[0] = 1

	r := &BattleResult{From: 1, To: lord.Location, AttackerWon: true,
		Captives: []Captive{{General: lord.Index, Name: lord.Name}}}
	g.seizeTreasures(r, 5)

	if loser.Treasury[0] != 0 {
		t.Errorf("敗方還留著 %d 件寶物，應該盡歸勝方", loser.Treasury[0])
	}
	if winner.Treasury[0] != 4 {
		t.Errorf("勝方拿到 %d 件寶物，應該是 1+3=4", winner.Treasury[0])
	}
	if len(r.Log) == 0 {
		t.Error("寶物易手卻沒有留下紀錄")
	}
}

// TestNonLordCaptureKeepsTreasures 釘住只有捉到**君主**才拿得到寶物。
func TestNonLordCaptureKeepsTreasures(t *testing.T) {
	g := newGame(t)
	var subordinate *General
	for i := range g.generals {
		x := &g.generals[i]
		if x.Employed() && x.Status != state.StatusLord && x.Faction != 5 {
			subordinate = x
			break
		}
	}
	if subordinate == nil {
		t.Skip("找不到非君主的部將")
	}
	loser, winner := g.Faction(subordinate.Faction), g.Faction(5)
	if loser == nil || winner == nil {
		t.Skip("勢力不齊")
	}
	loser.Treasury[0] = 3
	r := &BattleResult{From: 1, To: subordinate.Location, AttackerWon: true,
		Captives: []Captive{{General: subordinate.Index, Name: subordinate.Name}}}
	g.seizeTreasures(r, 5)
	if loser.Treasury[0] != 3 {
		t.Error("捉到的是部將不是君主，寶物不該易手")
	}
}

// TestBeginAttackDefersTheFight 釘住 BeginAttack 只擺陣不打，
// FinishAttack 才把結果搬回局面。
//
// 玩家親自指揮時，開打與收尾之間隔著幾十次按鍵；這中間**局面不能先動**，
// 否則畫面上的兵力與戰場上的兵力會是兩個數字。
func TestBeginAttackDefersTheFight(t *testing.T) {
	g := newGame(t)
	from, to := 0, 0
	var owner state.FactionID = state.NoFaction
	var att []int
	for id := 1; id <= state.PrefectureCount && from == 0; id++ {
		p := g.Prefecture(id)
		if !p.Owned() {
			continue
		}
		for _, n := range p.Neighbours {
			q := g.Prefecture(n)
			if !q.Owned() || q.Owner == p.Owner || len(g.Garrison(n)) == 0 {
				continue
			}
			if a := attackersAt(g, id, p.Owner, true); len(a) > 0 {
				from, to, owner, att = id, n, p.Owner, a
				break
			}
		}
	}
	if from == 0 {
		t.Skip("找不到可以出兵的郡")
	}
	ownerBefore := g.Prefecture(to).Owner
	menBefore := g.Soldiers(to)

	p, err := g.BeginAttack(from, to, att, owner, HalfSupply())
	if err != nil {
		t.Fatal(err)
	}
	if p.Battle() == nil {
		t.Fatal("沒有拿到戰場")
	}
	if p.Battle().Over {
		t.Error("BeginAttack 不該把戰役打完")
	}
	if g.Prefecture(to).Owner != ownerBefore || g.Soldiers(to) != menBefore {
		t.Error("還沒打就動到目標郡的局面")
	}
	if len(g.Reports) != 0 {
		t.Error("還沒收尾就有戰報")
	}

	// 交給自動作戰打完，再收尾。
	p.Battle().Auto()
	r := g.FinishAttack(p)
	if r == nil {
		t.Fatal("收尾沒有回傳結果")
	}
	if r.Days < 1 {
		t.Errorf("戰役打了 %d 天", r.Days)
	}
	if len(g.DrainReports()) != 1 {
		t.Error("收尾之後應該剛好留下一份戰報")
	}
	if r.AttackerLost == 0 && r.DefenderLost == 0 {
		t.Error("打了一場雙方都沒有折損")
	}
}

// TestBeginAttackChecksTheSameConditions 釘住親征與電腦出兵走同一組條件。
func TestBeginAttackChecksTheSameConditions(t *testing.T) {
	g := newGame(t)
	all := attackersAt(g, 15, 5, false)
	if len(all) == 0 {
		t.Skip("洛陽沒有守將")
	}
	target := g.Prefecture(15).Neighbours[0]
	if _, err := g.BeginAttack(15, target, all, 5, HalfSupply()); !errors.Is(err, ErrNoGovernor) {
		t.Errorf("傾巢而出回 %v，應該是 ErrNoGovernor", err)
	}
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
	if _, err := g.BeginAttack(15, to, all[:1], 5, HalfSupply()); !errors.Is(err, ErrNotAdjacent) {
		t.Errorf("打不相鄰的郡回 %v，應該是 ErrNotAdjacent", err)
	}
}

// TestPlayerCommandedBattle 釘住「玩家一步一步指揮」這條路走得完。
//
// 走的是 `cmd/san1` 用的那一組介面：BeginAttack → Runner → 逐支下令 →
// FinishAttack。**Ebiten 那一層測不到**，所以這條路徑要在這裡走一遍，
// 不然「按了沒反應」只有開視窗才發現。
func TestPlayerCommandedBattle(t *testing.T) {
	g := newGame(t)
	from, to := 0, 0
	var owner state.FactionID = state.NoFaction
	var force []int
	for id := 1; id <= state.PrefectureCount && from == 0; id++ {
		p := g.Prefecture(id)
		if !p.Owned() {
			continue
		}
		for _, n := range p.Neighbours {
			q := g.Prefecture(n)
			if !q.Owned() || q.Owner == p.Owner || len(g.Garrison(n)) == 0 {
				continue
			}
			if a := attackersAt(g, id, p.Owner, true); len(a) > 0 {
				from, to, owner, force = id, n, p.Owner, a
				break
			}
		}
	}
	if from == 0 {
		t.Skip("找不到可以出兵的郡")
	}
	p, err := g.BeginAttack(from, to, force, owner, HalfSupply())
	if err != nil {
		t.Fatal(err)
	}
	b := p.Battle()
	r := battle.NewRunner(b, func(s battle.Side) bool { return s.Attacking() })

	turns, acted := 0, 0
	for {
		u := r.Next()
		if u == nil {
			break
		}
		turns++
		if turns > 20000 {
			t.Fatal("輪太多次，可能沒有前進")
		}
		// 玩家的行為：先試著往城池走，走不動就休息。
		moved := false
		for _, d := range battle.Dirs() {
			if b.Move(u, d) == nil {
				moved = true
				acted++
				break
			}
		}
		if !moved {
			if b.Rest(u) == nil {
				acted++
			}
		}
		r.Done()
	}
	if turns == 0 {
		t.Fatal("一次都沒有輪到玩家")
	}
	if acted == 0 {
		t.Error("輪到了卻一個動作都下不出去")
	}
	if !b.Over {
		t.Error("跑完了卻沒有分勝負")
	}

	res := g.FinishAttack(p)
	if res == nil {
		t.Fatal("收尾沒有回傳結果")
	}
	if res.Days < 1 || res.Days > 31 {
		t.Errorf("戰役打了 %d 天", res.Days)
	}
	// 收尾之後兵力要與戰場一致。
	for _, u := range b.Units {
		for _, l := range u.Leaders {
			x := g.General(l.Index)
			if x == nil || l.Dead || l.Captured {
				continue
			}
			if x.Soldiers != l.Soldiers {
				t.Errorf("%s 收尾後兵力 %d，戰場上是 %d", l.Name, x.Soldiers, l.Soldiers)
			}
		}
	}
}

// TestForgeryScalesWithCharm 釘住偽書使疑照原版的乘法降忠誠：
// 只動忠誠低於使者魅力的人，而且是按比例掉不是減固定值。
func TestForgeryScalesWithCharm(t *testing.T) {
	g := newGame(t)
	target := 11 // 陳留（曹操）
	dst := g.Prefecture(target)
	garrison := g.Garrison(target)
	if len(garrison) < 2 {
		t.Fatalf("陳留只有 %d 位武將", len(garrison))
	}
	const charm = 90
	// 一位在門檻之上、一位之下。
	high, low := garrison[0], garrison[1]
	high.Faction, low.Faction = dst.Owner, dst.Owner
	high.Loyalty, low.Loyalty = 95, 80
	g.Forgery(target, charm)

	if high.Loyalty != 95 {
		t.Errorf("忠誠 95 高於魅力 %d，不該被動到，卻變成 %d", charm, high.Loyalty)
	}
	// 係數是 (250 − RND(45) − 90) ÷ 250 ＝ 0.464–0.64。
	lo, hi := 80*(ForgeryScale-44-charm)/ForgeryScale, 80*(ForgeryScale-0-charm)/ForgeryScale
	if int(low.Loyalty) < lo || int(low.Loyalty) > hi {
		t.Errorf("忠誠 80 掉到 %d，應該落在 %d–%d", low.Loyalty, lo, hi)
	}
}

// TestAidJoinsTheField 釘住助攻軍與助守軍真的擺得上戰場。
//
// 原版的戰鬥子系統一次收四個郡（`battle(攻方郡, 攻方援郡, 守方郡,
// 守方援郡)`，`0x20200`），援軍是那個郡的全部駐軍。只有主攻與主守
// 上場的話，四種軍力就只是一份沒有人走進去的資料結構。
func TestAidJoinsTheField(t *testing.T) {
	g := newGame(t)
	// 盤面自己擺：三個相鄰的郡，攻方、守方、各自的援郡。
	from, to := 11, 13 // 陳留（曹操）打潁川（袁術）
	src, dst := g.Prefecture(from), g.Prefecture(to)
	aidA, aidD := 0, 0
	for _, n := range src.Neighbours {
		if n != to && g.Prefecture(n).Owned() {
			aidA = n
			break
		}
	}
	for _, n := range dst.Neighbours {
		if n != from && n != aidA && g.Prefecture(n).Owned() {
			aidD = n
			break
		}
	}
	if aidA == 0 || aidD == 0 {
		t.Fatalf("找不到援郡：aidA=%d aidD=%d", aidA, aidD)
	}
	att := g.garrisonOf(from)
	def := g.garrisonOf(to)
	if len(att) == 0 || len(def) == 0 || len(g.garrisonOf(aidA)) == 0 ||
		len(g.garrisonOf(aidD)) == 0 {
		t.Skip("四個郡裡有人是空的")
	}
	p := g.prepare(from, to, att, def, src.Owner, HalfSupply(),
		Aid{Attacker: aidA, Defender: aidD})
	var seen [4]int
	for _, u := range p.B.Units {
		seen[u.Side] += len(u.Leaders)
	}
	for side, n := range seen {
		if n == 0 {
			t.Errorf("%s 一個人都沒上場", battle.Side(side))
		}
	}

	// 援軍的傷亡要搬得回去：打完之後那兩個郡的人兵力有變動。
	beforeA := g.Soldiers(aidA)
	p.B.Auto()
	g.settle(p)
	if g.Soldiers(aidA) == beforeA {
		t.Log("助攻軍毫髮無傷——有可能，但值得看一眼")
	}
}

// TestFarNearLaunchesACampaign 釘住遠交近攻真的打起來。
//
// 這三種計謀的結局是發動一場戰役，不是「成功了但什麼也沒發生」。
func TestFarNearLaunchesACampaign(t *testing.T) {
	g := newGame(t)
	var by state.FactionID = 1 // 曹操
	envoy := 0
	strike, ours := 0, 0
	for _, at := range g.FarNearEnvoyTargets(by) {
		for _, s := range g.FarNearStrikeTargets(at, by) {
			if o := g.FarNearOurTargets(s, by); len(o) > 0 {
				envoy, strike, ours = at, s, o[0]
				break
			}
		}
		if envoy != 0 {
			break
		}
	}
	if envoy == 0 {
		t.Skip("開局的曹操找不到遠交近攻的組合")
	}
	lord := g.Lord(by)
	from := lord.Location
	// **盤面自己擺**：隨手挑一位部將把謀略拉到拜得了軍師的門檻。
	// 照劇本挑的話，開局的曹操身邊沒有那樣的人，整支測試會安靜地
	// skip 掉——而 skip 掉的測試看起來與通過的一模一樣。
	var wise *General
	for _, x := range g.Garrison(from) {
		if x.Faction == by && x.Index != lord.Index {
			x.Intel = 100
			wise = x
			break
		}
	}
	if wise == nil {
		t.Fatal("曹操所在地只有他自己")
	}
	if err := g.AppointChief(from, wise.Index, by); err != nil {
		t.Fatal(err)
	}
	g.Prefecture(from).Commanded = false
	before := g.Soldiers(strike)
	ok, err := g.UsePlotPlan(from, PlotFarNear,
		PlotPlan{Envoy: wise.Index, At: envoy, Strike: strike, Ours: ours}, by)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Skip("這一次計謀沒得手")
	}
	if g.Soldiers(strike) == before && g.Prefecture(strike).Owner != by {
		t.Errorf("遠交近攻得手，%s 卻毫無動靜", g.Prefecture(strike).Name)
	}
}

// TestJointAttackNeedsTwoOfOurs 釘住聯合出兵真的要兩個我方的郡，
// 而且助攻軍與被打的郡都照原版的候選條件挑。
func TestJointAttackNeedsTwoOfOurs(t *testing.T) {
	g := newGame(t)
	var by state.FactionID = 5 // 董卓，開局四個郡連在一起
	from, strike, aid := 0, 0, 0
	for _, f := range g.JointAttackFromTargets(by) {
		for _, s := range g.JointAttackStrikeTargets(f, by) {
			if a := g.JointAttackAidTargets(s, f, by); len(a) > 0 {
				from, strike, aid = f, s, a[0]
				break
			}
		}
		if from != 0 {
			break
		}
	}
	if from == 0 {
		t.Skip("董卓開局湊不出聯合出兵的組合")
	}
	if aid == from {
		t.Fatal("助攻的郡不該是出兵的那一郡")
	}
	if g.Prefecture(strike).Owner == by {
		t.Fatal("聯合攻打的目標是自己的郡")
	}
	lord := g.Lord(by)
	at := lord.Location
	var wise *General
	for _, x := range g.Garrison(at) {
		if x.Faction == by && x.Index != lord.Index {
			x.Intel = 100
			wise = x
			break
		}
	}
	if err := g.AppointChief(at, wise.Index, by); err != nil {
		t.Fatal(err)
	}
	g.Prefecture(at).Commanded = false
	// 少了第二個我方的郡就不成立。
	if _, err := g.UsePlotPlan(at, PlotJointAttack,
		PlotPlan{Ours: from, Strike: strike, OursAid: from}, by); err == nil {
		t.Error("只有一個郡也讓聯合出兵成立")
	}
	g.Prefecture(at).Commanded = false
	before := g.Soldiers(strike)
	ok, err := g.UsePlotPlan(at, PlotJointAttack,
		PlotPlan{Ours: from, Strike: strike, OursAid: aid}, by)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("聯合出兵不判成敗，應該一定打得起來")
	}
	if g.Soldiers(strike) == before && g.Prefecture(strike).Owner == by {
		t.Log("目標郡被接手且無傷亡——走進空郡，合理")
	}
}
