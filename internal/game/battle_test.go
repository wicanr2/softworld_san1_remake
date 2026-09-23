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

// TestAttackTakesPrefecture 釘住勝方軍團駐進戰場郡；原版寫入端
// `0x268d3`／`0x2695c`，見 `docs/spec/020`。
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

// TestPlayerBattleUsesArmyChiefAndReturnsAid 是 #102 的反向規則測試：
// 故意讓統帥魅力低於同軍另一位，戰後太守必須仍是統帥；勝方援軍
// 回來源郡，金米與戰場受損都按 `docs/spec/020` 寫回。
func TestPlayerBattleUsesArmyChiefAndReturnsAid(t *testing.T) {
	g := newGame(t)
	from, to := 0, 0
	var low, high *General
	var owner state.FactionID
	for id := 1; id <= state.PrefectureCount && from == 0; id++ {
		src := g.Prefecture(id)
		if !src.Owned() {
			continue
		}
		var roster []*General
		for _, x := range g.Garrison(id) {
			if x.Faction == src.Owner {
				roster = append(roster, x)
			}
		}
		if len(roster) < 2 {
			continue
		}
		for _, n := range src.Neighbours {
			if q := g.Prefecture(n); q != nil && !q.Owned() && len(g.Garrison(n)) == 0 {
				from, to, owner = id, n, src.Owner
				low, high = roster[0], roster[1]
				break
			}
		}
	}
	if from == 0 {
		t.Fatal("劇本 001 找不到兩位將領可進駐的空白鄰郡")
	}
	var aid *General
	for i := range g.generals {
		x := &g.generals[i]
		if x.Employed() && x.Faction == owner && x.Index != low.Index && x.Index != high.Index {
			aid = x
			break
		}
	}
	if aid == nil {
		t.Fatal("這個勢力沒有第三位可用作援軍的將領")
	}
	low.Status, high.Status, aid.Status = state.StatusOfficer, state.StatusOfficer, state.StatusOfficer
	low.Charm, high.Charm = 10, 90
	aid.Location = from
	src, dst := g.Prefecture(from), g.Prefecture(to)
	src.Gold, src.Rice = 300, 500
	dst.Gold, dst.Rice = 0, 0
	dst.PublicLoyalty, dst.LandValue, dst.FloodRate, dst.PriceLevel = 100, 100, 0, 40
	leader := func(x *General) battle.Leader {
		return battle.Leader{Index: x.Index, Name: x.Name, Soldiers: x.Soldiers, Stamina: x.Stamina}
	}
	b := &battle.Battle{Over: true, AttackerWon: true, Units: []*battle.Unit{
		{Side: battle.MainAttacker, Leaders: []battle.Leader{leader(low), leader(high)}},
		{Side: battle.AidAttacker, Leaders: []battle.Leader{leader(aid)}},
	}}
	b.Commander[battle.MainAttacker], b.Commander[battle.AidAttacker] = low.Index, aid.Index
	b.Gold[battle.MainAttacker], b.Gold[battle.MainDefender] = 100, 200
	b.Gold[battle.AidAttacker], b.Gold[battle.AidDefender] = 40, 30
	b.Rice[battle.MainAttacker], b.Rice[battle.MainDefender] = 1000, 2000
	b.Rice[battle.AidAttacker], b.Rice[battle.AidDefender] = 400, 300
	p := &Pending{B: b, from: from, to: to, by: owner,
		att: []*General{low, high}, aidAtt: []*General{aid},
		aid: Aid{Attacker: from}, result: &BattleResult{From: from, To: to}}
	for i := range p.factions {
		p.factions[i] = state.NoFaction
	}
	p.factions[battle.MainAttacker], p.factions[battle.AidAttacker] = owner, owner
	g.SeedRand(0x13579bdf)
	r := g.settle(p)
	if !r.PrefectureTook || dst.Owner != owner {
		t.Fatalf("勝方未進駐戰場郡：戰報 %v、所屬 %d", r.PrefectureTook, dst.Owner)
	}
	if gov := g.Governor(to); gov == nil || gov.Index != low.Index {
		t.Fatalf("太守應為低魅力統帥 %d，而非高魅力部將 %d；實際 %v", low.Index, high.Index, gov)
	}
	if low.Location != to || high.Location != to || aid.Location != from {
		t.Errorf("戰後所在郡：主軍 %d／%d，援軍 %d；應為 %d／%d／%d",
			low.Location, high.Location, aid.Location, to, to, from)
	}
	if dst.Gold != 330 || dst.Rice != 3300 || src.Gold != 340 || src.Rice != 900 {
		t.Errorf("戰後金米：戰場 %d／%d、援郡 %d／%d；應為 330／3300、340／900",
			dst.Gold, dst.Rice, src.Gold, src.Rice)
	}
	if dst.PublicLoyalty >= 100 || dst.LandValue >= 100 || dst.FloodRate <= 0 || dst.PriceLevel <= 40 {
		t.Errorf("玩家戰役沒有打殘戰場郡：民忠 %d、地力 %d、洪水 %d、物價 %d",
			dst.PublicLoyalty, dst.LandValue, dst.FloodRate, dst.PriceLevel)
	}
}

// TestPlayerBattleWritesRetreatDestination 釘住 `0x24318` 的人物 offset 19：
// 戰場上已退掉的部隊仍要把去處寫回戰略層，不能沿用出征中的 0。
func TestPlayerBattleWritesRetreatDestination(t *testing.T) {
	g := newGame(t)
	from, to := 0, 0
	var att, def *General
	for id := 1; id <= state.PrefectureCount && from == 0; id++ {
		src := g.Prefecture(id)
		if !src.Owned() {
			continue
		}
		for _, n := range src.Neighbours {
			dst := g.Prefecture(n)
			if dst == nil || !dst.Owned() || dst.Owner == src.Owner {
				continue
			}
			att, def = nil, nil
			for _, x := range g.Garrison(id) {
				if x.Faction == src.Owner {
					att = x
					break
				}
			}
			for _, x := range g.Garrison(n) {
				if x.Faction == dst.Owner {
					def = x
					break
				}
			}
			if att != nil && def != nil {
				from, to = id, n
				break
			}
		}
	}
	if from == 0 {
		t.Fatal("劇本 001 找不到兩個相鄰的敵對州郡")
	}
	by, defender := att.Faction, def.Faction
	att.Location = 0 // 原版整編 `0x20ce0` 先清掉出征者的所在郡
	b := &battle.Battle{Over: true, AttackerWon: false, Units: []*battle.Unit{
		{Side: battle.MainAttacker, Retreated: true, RetreatTo: from,
			Leaders: []battle.Leader{{Index: att.Index, Name: att.Name,
				Soldiers: att.Soldiers, Stamina: att.Stamina}}},
		{Side: battle.MainDefender, Leaders: []battle.Leader{{Index: def.Index,
			Name: def.Name, Soldiers: def.Soldiers, Stamina: def.Stamina}}},
	}}
	b.Commander[battle.MainDefender] = def.Index
	p := &Pending{B: b, from: from, to: to, by: by,
		att: []*General{att}, def: []*General{def},
		result: &BattleResult{From: from, To: to}}
	for i := range p.factions {
		p.factions[i] = state.NoFaction
	}
	p.factions[battle.MainAttacker], p.factions[battle.MainDefender] = by, defender
	g.SeedRand(0x13579bdf)
	g.settle(p)
	if att.Location != from {
		t.Fatalf("退兵者所在郡 %d，原版應寫回目的郡 %d", att.Location, from)
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

// TestFallenLordSpoilsKeepSeal 釘住原版 0x26c08：四類寶物只分走
// 一部分、四類總數因額外賞賜多 2，玉璽不在這一支裡。
func TestFallenLordSpoilsKeepSeal(t *testing.T) {
	g := newGame(t)
	const winnerID, loserID = 5, 13
	g.SeedRand(0x13579bdf)
	winner, loser := g.Faction(winnerID), g.Faction(loserID)
	winner.Treasury = [5]int{2, 1, 1, 1, 1}
	loser.Treasury = [5]int{1, 10, 9, 8, 7}
	g.spoilsFromFallenLord(winnerID, loserID)
	if winner.Treasury[TreasureSeal] != 2 || loser.Treasury[TreasureSeal] != 1 {
		t.Fatalf("分贓常式改動玉璽：勝方 %d、敗方 %d", winner.Treasury[TreasureSeal], loser.Treasury[TreasureSeal])
	}
	total := 0
	for i := TreasureBook; i < treasureCount; i++ {
		total += winner.Treasury[i] + loser.Treasury[i]
		if loser.Treasury[i] <= 0 {
			t.Errorf("敗方第 %d 類被整批搬光；原版只拿一部分", i)
		}
	}
	if total != 40 {
		t.Errorf("四類總數 %d，原版固定先增加 2 件後應為 40", total)
	}
}

// TestPlayerBattleExecutedLordCallsSpoils 驗玩家戰役收尾本身會叫分贓，
// 且呼叫端的正對照確實是原版第一擲 `0x26c31`。
func TestPlayerBattleExecutedLordCallsSpoils(t *testing.T) {
	g := newGame(t)
	lord := g.Lord(13)
	if lord == nil {
		t.Fatal("劇本 001 缺少勢力 13 的君主")
	}
	winner, loser := g.Faction(5), g.Faction(lord.Faction)
	winner.Treasury = [5]int{2, 1, 1, 1, 1}
	loser.Treasury = [5]int{1, 10, 9, 8, 7}
	g.SeedRand(0x13579bdf)
	rolls := 0
	g.TraceRolls(func(n, out int, salt []int) {
		if len(salt) > 0 && salt[len(salt)-1] == 0x26c31 {
			rolls++
		}
	})
	b := &battle.Battle{Over: true, AttackerWon: true, Units: []*battle.Unit{{
		Side: battle.MainDefender, Leaders: []battle.Leader{{
			Index: lord.Index, Name: lord.Name, Captured: true,
			Fate: battle.Executed, CapturedBy: battle.MainAttacker,
		}},
	}}}
	p := &Pending{B: b, from: 15, to: lord.Location, by: 5,
		def: []*General{lord}, result: &BattleResult{From: 15, To: lord.Location}}
	p.factions[battle.MainAttacker] = 5
	p.factions[battle.MainDefender] = lord.Faction
	g.settle(p)
	if rolls != 1 {
		t.Fatalf("退場君主分贓第一擲 %d 次，應為 1", rolls)
	}
	if winner.Treasury[TreasureSeal] != 2 {
		t.Fatalf("勝方玉璽被搬動：%d", winner.Treasury[TreasureSeal])
	}
	total := 0
	for i := TreasureBook; i < treasureCount; i++ {
		total += winner.Treasury[i] + loser.Treasury[i]
	}
	if total != 40 {
		t.Errorf("四類總數 %d，應為 40", total)
	}
}

// TestNonLordCaptureKeepsTreasures 釘住一般武將被俘不觸發退場君主分贓。
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
	winner.Treasury = [5]int{2, 1, 1, 1, 1}
	loser.Treasury = [5]int{1, 10, 9, 8, 7}
	beforeWinner, beforeLoser := winner.Treasury, loser.Treasury
	loserID := subordinate.Faction
	b := &battle.Battle{Over: true, AttackerWon: true, Units: []*battle.Unit{{
		Side: battle.MainDefender, Leaders: []battle.Leader{{
			Index: subordinate.Index, Name: subordinate.Name, Captured: true,
			Fate: battle.Jailed, CapturedBy: battle.MainAttacker,
		}},
	}}}
	p := &Pending{B: b, from: 15, to: subordinate.Location, by: 5,
		result: &BattleResult{From: 15, To: subordinate.Location}}
	p.factions[battle.MainDefender] = loserID
	g.settle(p)
	if winner.Treasury != beforeWinner || loser.Treasury != beforeLoser {
		t.Errorf("一般武將被擒卻搬寶庫：勝方 %v→%v，敗方 %v→%v",
			beforeWinner, winner.Treasury, beforeLoser, loser.Treasury)
	}
}

// TestBeginAttackDefersTheFight 釘住 BeginAttack 只擺陣不打，
// FinishAttack 才把結果搬回局面。
//
// 玩家親自指揮時，開打與收尾之間隔著幾十次按鍵。整編本身會把
// 四軍團人物的所在郡清成 0，戰場郡暫無主；這不是戰後易主。
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
	if g.Prefecture(to).Owner != state.NoFaction || g.Soldiers(to) != 0 ||
		p.factions[battle.MainDefender] != ownerBefore || menBefore <= 0 {
		t.Errorf("整編後目標郡狀態錯誤：當前所屬 %d、駐兵 %d、原守方 %d（預期 %d）",
			g.Prefecture(to).Owner, g.Soldiers(to), p.factions[battle.MainDefender], ownerBefore)
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

// TestRavageBattlefield 釘住戰場那個郡的四個欄位怎麼被打殘
// （原版 `0x1f8b2`–`0x1f9bd`，只在電腦對電腦那條路上做）。
func TestRavageBattlefield(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(1)
	if p == nil {
		t.Fatal("郡 1 不存在")
	}
	p.PublicLoyalty, p.LandValue, p.FloodRate, p.PriceLevel = 100, 100, 0, 30
	g.ravageBattlefield(1)

	if d := 100 - int(p.PublicLoyalty); d < 1 || d > 10 {
		t.Errorf("民眾忠誠掉了 %d，應該落在 1–10", d)
	}
	if d := 100 - int(p.LandValue); d < 1 || d > 10 {
		t.Errorf("土地價值掉了 %d，應該落在 1–10", d)
	}
	if d := int(p.FloodRate); d < 2 || d > 8 {
		t.Errorf("洪水率升了 %d，應該落在 2–8", d)
	}
	if d := int(p.PriceLevel) - 30; d < 2 || d > 19 {
		t.Errorf("物價升了 %d，應該落在 2–19", d)
	}

	// 兩個下降夾在 0（不繞成 255），兩個上升各有上限。
	p.PublicLoyalty, p.LandValue = 1, 1
	p.FloodRate, p.PriceLevel = RavageFloodCap, RavagePriceCap
	g.ravageBattlefield(1)
	if p.PublicLoyalty > 100 || p.LandValue > 100 {
		t.Errorf("低值掉完之後變成 %d／%d，應該夾在 0", p.PublicLoyalty, p.LandValue)
	}
	if int(p.FloodRate) != RavageFloodCap || int(p.PriceLevel) != RavagePriceCap {
		t.Errorf("洪水率 %d、物價 %d，應該停在上限 %d／%d",
			p.FloodRate, p.PriceLevel, RavageFloodCap, RavagePriceCap)
	}
}

// TestAutoAIPoolsSuppliesIntoTheBattlefield 釘住「電腦對電腦時，四個軍團的
// 隨軍錢糧不管誰贏都收進守方那一郡」（原版 `0x1f82f`／`0x1f8ae`）。
//
// 與玩家那條「補給跟著自己走」是兩條規則：攻方打輸的時候，帶去的錢糧
// 在原版是留給守方的。
func TestAutoAIPoolsSuppliesIntoTheBattlefield(t *testing.T) {
	g := newGame(t)
	var from, to int
	for n := 1; n <= 42 && from == 0; n++ {
		src := g.Prefecture(n)
		if src == nil || !src.Owned() || len(g.garrisonOf(n)) == 0 {
			continue
		}
		for _, m := range src.Neighbours {
			d := g.Prefecture(m)
			if d != nil && d.Owned() && d.Owner != src.Owner &&
				len(g.garrisonOf(m)) > 0 {
				from, to = n, m
				break
			}
		}
	}
	if from == 0 {
		t.Skip("找不到可以出兵的郡")
	}
	src := g.Prefecture(from)
	p := g.prepare(from, to, g.garrisonOf(from), g.garrisonOf(to),
		src.Owner, HalfSupply(), Aid{})
	p.autoAI = true
	p.B.Gold = [4]int{100, 200, 400, 800}
	p.B.Rice = [4]int{10, 20, 40, 80}
	p.B.Over, p.B.AttackerWon = true, false // 攻方打輸，補給照樣留給守方
	g.settle(p)

	dst := g.Prefecture(to)
	if dst.Gold != 1500 {
		t.Errorf("守方那一郡的金是 %d，四個軍團加起來應該是 1500", dst.Gold)
	}
	if dst.Rice != 150 {
		t.Errorf("守方那一郡的米是 %d，四個軍團加起來應該是 150", dst.Rice)
	}
}

// TestAutoAIPlacementLeavesLosersInTheBattlefield 釘住電腦對電腦戰役的安置
// （原版 `0x1fb26`）：**所有生還者都落在戰場那一郡**，敗方收得下來的改
// 勢力並降成一般武將，收不下來的維持原本的勢力留在那裡。
//
// 「留在那裡」是原版混編郡的來源——玩家那條打輸是退回原郡，兩者不同。
func TestAutoAIPlacementLeavesLosersInTheBattlefield(t *testing.T) {
	g := newGame(t)
	var from, to int
	for n := 1; n <= 42 && from == 0; n++ {
		src := g.Prefecture(n)
		if src == nil || !src.Owned() || len(g.garrisonOf(n)) == 0 {
			continue
		}
		for _, m := range src.Neighbours {
			d := g.Prefecture(m)
			if d != nil && d.Owned() && d.Owner != src.Owner &&
				len(g.garrisonOf(m)) > 0 {
				from, to = n, m
				break
			}
		}
	}
	if from == 0 {
		t.Skip("找不到可以出兵的郡")
	}
	src, dst := g.Prefecture(from), g.Prefecture(to)
	if f := g.Faction(dst.Owner); f != nil {
		f.Prestige = 100 // 人望拉滿，收得下來的那一條才走得到
	}
	loser, winner := src.Owner, dst.Owner
	att := g.garrisonOf(from)
	p := g.prepare(from, to, att, g.garrisonOf(to), src.Owner, HalfSupply(), Aid{})
	p.autoAI = true
	p.B.Over, p.B.AttackerWon = true, false // 攻方打輸
	g.settle(p)

	// 敗方的每一位只有四條路（`0x1fb26`，`docs/re/05` §7.1）：逃到退路候選
	// 的鄰郡（敗方主軍的勢力或無主）、留在戰場郡被收編、留在戰場郡失去
	// 勢力（身分 10）、或身分 12 的下野。**沒有「維持原本的勢力留在敵郡」
	// 這一條。**
	fled, joined, stranded, fallen := 0, 0, 0, 0
	for _, x := range att {
		switch {
		case x.Status == state.StatusFallen:
			fallen++
			if x.Employed() {
				t.Errorf("%s 下野了還掛著勢力 %d", x.Name, x.Faction)
			}
		case x.Status == state.StatusStranded:
			stranded++
			if x.Location != to || x.Employed() {
				t.Errorf("%s 失去勢力之後在郡 %d、勢力 %d，原版是留在戰場郡 %d、無勢力",
					x.Name, x.Location, x.Faction, to)
			}
		case x.Faction == winner:
			joined++
			if x.Location != to {
				t.Errorf("%s 被收編之後在郡 %d，原版是留在戰場郡 %d", x.Name, x.Location, to)
			}
			if x.Status != state.StatusOfficer {
				t.Errorf("%s 被收編之後身分是 %v，原版一律降成一般武將",
					x.Name, x.Status)
			}
		case x.Faction == loser:
			fled++
			q := g.Prefecture(x.Location)
			if q == nil || !g.Adjacent(to, x.Location) || (q.Owned() && q.Owner != loser) {
				t.Errorf("%s 打輸之後在郡 %d，原版只會逃到戰場郡 %d 旁邊自己的或無主的郡",
					x.Name, x.Location, to)
			}
		default:
			t.Errorf("%s 打輸之後勢力 %d 身分 %v 所在 %d，不在四條路上",
				x.Name, x.Faction, x.Status, x.Location)
		}
	}
	t.Logf("攻方 %d 位：逃走 %d、收編 %d、失去勢力 %d、下野 %d",
		len(att), fled, joined, stranded, fallen)
}
