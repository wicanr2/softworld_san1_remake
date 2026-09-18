package game

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestPlayerDefenceHandsOverTheBattle 釘住「守城」開關的兩邊（Issue #64）。
//
// 關著（預設）：電腦打玩家的郡整場自動打完，回一份戰報，沒有交出去的戰役。
// 開著：同一場整編完之後停下來，戰役交給 `PendingDefence`，戰報等
// `FinishDefence` 才有。
//
// **關著的那一邊不能岔開骰序**：預設就是關著，所以守的是既有的
// `TestZZMonthParity`／`TestZZUnitAIDayParity`（月流程逐位元組／逐鏈相同）。
// 這一支只釘開關的兩種行為，不重複那件事。
func TestPlayerDefenceHandsOverTheBattle(t *testing.T) {
	// 被打的是玩家的郡：劉備（勢力 0）的齊郡，打他的是鄰郡的電腦諸侯。
	setup := func(defend bool) (*State, int, int, state.FactionID) {
		g := newGame(t)
		g.Options.PlayerDefends = defend
		var from, to int
		var by state.FactionID
		for id := 1; id <= len(g.prefectures) && from == 0; id++ {
			p := g.Prefecture(id)
			if p == nil || !p.Owned() || !g.IsHuman(p.Owner) {
				continue
			}
			for _, n := range p.Neighbours {
				q := g.Prefecture(n)
				if q != nil && q.Owned() && !g.IsHuman(q.Owner) && len(g.ActorRoster(n)) > 1 {
					from, to, by = n, id, q.Owner
					break
				}
			}
		}
		if from == 0 {
			t.Fatal("劇本一裡找不到「電腦郡挨著玩家郡」的一對")
		}
		return g, from, to, by
	}
	keep := func(*State, int) int { return 0 }

	g0, from, to, by := setup(false)
	r0, err := g0.ComputerAttack(from, to, by, keep)
	if err != nil {
		t.Fatal(err)
	}
	if r0 == nil {
		t.Fatal("關著開關時應該當場打完並回戰報")
	}
	if g0.PendingDefence() != nil {
		t.Error("關著開關卻交出了一場戰役")
	}

	g1, from1, to1, by1 := setup(true)
	if from1 != from || to1 != to || by1 != by {
		t.Fatalf("兩次擺出來的不是同一場：%d→%d 對 %d→%d", from, to, from1, to1)
	}
	r1, err := g1.ComputerAttack(from1, to1, by1, keep)
	if err != nil {
		t.Fatal(err)
	}
	if r1 != nil {
		t.Error("開著開關時不該當場回戰報")
	}
	p := g1.PendingDefence()
	if p == nil {
		t.Fatal("開著開關卻沒有把戰役交出來")
	}
	if !p.Player {
		t.Error("交出來的那一場沒有標成玩家指揮")
	}
	if p.B.Over {
		t.Error("交出來的那一場已經打完了")
	}
	// 交出去之後接著打完，結果要與自動那一邊逐項相同——**整編是同一份**，
	// 開關只換誰下令。日迴圈的擲骰在 `settle` 裡照樣跑。
	r1b := g1.FinishDefence()
	if r1b == nil {
		t.Fatal("FinishDefence 沒有回戰報")
	}
	if r1b.From != r0.From || r1b.To != r0.To {
		t.Errorf("兩邊打的不是同一場：%d→%d 對 %d→%d", r0.From, r0.To, r1b.From, r1b.To)
	}
	if g1.PendingDefence() != nil {
		t.Error("收完了還留著那一場")
	}
}
