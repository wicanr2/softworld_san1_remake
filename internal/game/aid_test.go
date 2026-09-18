package game

import (
	"testing"

)

// plantAidBoard 擺出這一問要的盤面：玩家的郡 `strike` 有一個同勢力的
// 鄰郡（求援的對象），另一個鄰郡是電腦的（來打的人）。
//
// **不能等劇本剛好湊出來**：劇本一的劉備只有一個郡，等下去這支測試會
// 永遠 skip，而 skip 不是綠（`CLAUDE.md` §7 第 18 條）。
func plantAidBoard(t *testing.T) (g *State, ours, strike, aid int) {
	t.Helper()
	g = newGame(t)
	for id := 1; id <= 42 && strike == 0; id++ {
		p := g.Prefecture(id)
		if p == nil || !p.Owned() || !g.IsHuman(p.Owner) {
			continue
		}
		var mine, theirs int
		for _, n := range p.Neighbours {
			q := g.Prefecture(n)
			if q == nil || !q.Owned() || q.Owner == p.Owner {
				continue
			}
			if len(g.ActorRoster(n)) == 0 {
				continue
			}
			if mine == 0 {
				mine = n // 這一個改成玩家的，當求援對象
				continue
			}
			theirs = n // 這一個留給電腦，當來打的人
		}
		if mine == 0 || theirs == 0 {
			continue
		}
		// 把 mine 改成玩家的：郡的所屬與郡裡每一位的勢力一起改，
		// 只改一半的話 `DefenderAidTargets` 看得到、`garrisonOf` 看不到。
		q := g.Prefecture(mine)
		for _, x := range g.Garrison(mine) {
			x.Faction = p.Owner
		}
		q.Owner = p.Owner
		strike, ours, aid = id, theirs, mine
	}
	if strike == 0 {
		t.Fatal("劇本一裡擺不出這一問的盤面")
	}
	return g, ours, strike, aid
}

// TestDefenderAidAsksThePlayer 釘住「聯合守方那一郡合守」（Issue #99）：
// 守方是玩家就停下來問，電腦守方仍然取掃到的最後一個（`0x2dd24`）。
func TestDefenderAidAsksThePlayer(t *testing.T) {
	g, _, strike, want := plantAidBoard(t)
	list := g.DefenderAidTargets(strike)
	if len(list) == 0 {
		t.Fatalf("郡 %d 一個求援對象都沒有（擺好的是 %d）", strike, want)
	}
	// 電腦那一條：掃到的最後一個。
	if got := g.DefenderAid(strike); got != list[len(list)-1] {
		t.Errorf("電腦守方求到郡 %d，原版取掃到的最後一個 %d", got, list[len(list)-1])
	}
	// 候選一定是同勢力的鄰郡。
	dst := g.Prefecture(strike)
	for _, n := range list {
		q := g.Prefecture(n)
		if q == nil || q.Owner != dst.Owner {
			t.Errorf("候選郡 %d 不是守方的同勢力鄰郡", n)
		}
	}
}

// TestAnswerDefenderAidLaunches 釘住答完就把那一場打下去，
// 而且 0 ＝ 不求援（原版空欄位 Enter 回 0xFFFF，印「不聯合守方」）。
func TestAnswerDefenderAidLaunches(t *testing.T) {
	setup := func() (*State, int, int, int) {
		g, ours, strike, aid := plantAidBoard(t)
		return g, ours, strike, aid
	}

	// 求援：助守軍那一郡進戰役。
	g, ours, strike, aid := setup()
	g.aidAsk = &aidAsk{Ours: ours, Strike: strike, Attacker: ours,
		List: g.DefenderAidTargets(strike)}
	if g.PendingAid() == nil {
		t.Fatal("沒有排進佇列")
	}
	if err := g.AnswerDefenderAid(aid); err != nil {
		t.Fatal(err)
	}
	if g.PendingAid() != nil {
		t.Error("答完了還留著那一問")
	}

	// 不求援（0）也要打得下去。
	g2, ours2, strike2, _ := setup()
	g2.aidAsk = &aidAsk{Ours: ours2, Strike: strike2, Attacker: ours2,
		List: g2.DefenderAidTargets(strike2)}
	if err := g2.AnswerDefenderAid(0); err != nil {
		t.Fatal(err)
	}
	if g2.PendingAid() != nil {
		t.Error("不求援之後還留著那一問")
	}

	// 反向對照：不在候選裡的郡要被擋下來。
	g3, ours3, strike3, _ := setup()
	g3.aidAsk = &aidAsk{Ours: ours3, Strike: strike3, Attacker: ours3,
		List: g3.DefenderAidTargets(strike3)}
	bad := 0
	for id := 1; id <= 42; id++ {
		inList := false
		for _, n := range g3.DefenderAidTargets(strike3) {
			if n == id {
				inList = true
			}
		}
		if !inList {
			bad = id
			break
		}
	}
	if err := g3.AnswerDefenderAid(bad); err == nil {
		t.Errorf("郡 %d 不在候選裡卻收下了", bad)
	}
}

// TestJointAttackStopsToAskThePlayer 釘住這一問真的會停下來（Issue #99）：
// 守方是玩家、而且求得到援軍時，`jointAttack` **不打**，把問題排進佇列。
//
// 反向對照在最後一段：守方是電腦時照舊直接打完。
func TestJointAttackStopsToAskThePlayer(t *testing.T) {
	g, ours, strike, _ := plantAidBoard(t)
	by := g.Prefecture(ours).Owner
	// 「聯合」至少要兩個郡：助攻郡另外挑一個同勢力的。
	aidFrom := 0
	for id := 1; id <= 42 && aidFrom == 0; id++ {
		if p := g.Prefecture(id); p != nil && p.Owned() && p.Owner == by && id != ours {
			aidFrom = id
		}
	}
	if aidFrom == 0 {
		// 那一方只有一個郡：借一個無主（或別人的）郡改成它的，
		// 「聯合」這一步才成立。要驗的是「停不停下來問」，不是誰出兵。
		for id := 1; id <= 42 && aidFrom == 0; id++ {
			if p := g.Prefecture(id); p != nil && id != ours && id != strike &&
				(!p.Owned() || p.Owner != g.Prefecture(strike).Owner) {
				p.Owner = by
				aidFrom = id
			}
		}
	}
	if aidFrom == 0 {
		t.Fatalf("勢力 %d 湊不出第二個郡", by)
	}
	plan := PlotPlan{Ours: ours, OursAid: aidFrom, Strike: strike}
	// 求不求得到援由 `outwitsDefender` 判；擺成「求得到」才問得到玩家。
	if g.outwitsDefender(by, strike) {
		// 我方壓過守方就孤立無援——把守方的人望墊高，讓分數倒過來。
		if f := g.Faction(g.Prefecture(strike).Owner); f != nil {
			f.Prestige = 100
		}
		if f := g.Faction(by); f != nil {
			f.Prestige = 0
		}
	}
	if g.outwitsDefender(by, strike) {
		t.Skip("這個盤面壓不出「守方求得到援軍」")
	}
	if _, err := g.jointAttack(plan, by); err != nil {
		t.Fatal(err)
	}
	if g.PendingAid() == nil {
		t.Fatal("守方是玩家卻沒有停下來問")
	}
	if got := g.PendingAid().Strikes(); got != strike {
		t.Errorf("問的是郡 %d，被打的是 %d", got, strike)
	}

	// 反向對照：守方換成電腦就不問，當場打完。
	g2, ours2, strike2, _ := plantAidBoard(t)
	by2 := g2.Prefecture(ours2).Owner
	g2.Prefecture(strike2).Owner = by2 // 借用來打的那一方，總之不是玩家
	if g2.IsHuman(g2.Prefecture(strike2).Owner) {
		t.Fatal("反向對照沒擺成電腦")
	}
	aidFrom2 := 0
	for id := 1; id <= 42 && aidFrom2 == 0; id++ {
		if p := g2.Prefecture(id); p != nil && id != ours2 && id != strike2 {
			p.Owner = by2
			aidFrom2 = id
		}
	}
	_, _ = g2.jointAttack(PlotPlan{Ours: ours2, OursAid: aidFrom2, Strike: strike2}, by2)
	if g2.PendingAid() != nil {
		t.Error("守方是電腦卻停下來問了")
	}
}
