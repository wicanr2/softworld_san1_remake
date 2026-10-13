package game

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func TestAutomaticBattleQueuesOneMarchBeforeSettlement(t *testing.T) {
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		for _, computer := range []bool{false, true} {
			t.Run(string(edition)+map[bool]string{false: "/force", true: "/computer"}[computer], func(t *testing.T) {
				g, err := New(loadScenario(t, state.Scenario1), 0, 5, edition)
				if err != nil {
					t.Fatal(err)
				}
				g.SeedRand(0x13579bdf)
				g.Players = nil
				from, to := 15, 0
				for _, id := range g.Prefecture(from).Neighbours {
					if g.Prefecture(id).Owner != 5 {
						to = id
						break
					}
				}
				if to == 0 {
					t.Fatal("原版夾具沒有敵方鄰郡")
				}
				g.PendingEvents()
				var result *BattleResult
				if computer {
					result, err = g.ComputerAttack(from, to, 5, func(*State, int) int { return 10 })
				} else {
					result, err = g.Attack(from, to, attackersAt(g, from, 5, true), 5)
				}
				if err != nil || result == nil {
					t.Fatalf("自動戰爭失敗：%v", err)
				}
				count := 0
				for i, event := range g.PendingEvents() {
					if event.Bubble == nil || event.Bubble.MapBattle == nil {
						continue
					}
					count++
					mb := event.Bubble.MapBattle
					if i != 0 || mb.Attacker != result.From || mb.Defender != result.To || mb.Days != result.Days {
						t.Fatalf("動畫順序或郡／天數不符：%+v，戰果 %+v", mb, result)
					}
				}
				if count != 1 {
					t.Fatalf("行軍動畫數量 %d，預期 1", count)
				}
			})
		}
	}
}

func TestPlayerBattleDoesNotQueueMapMarch(t *testing.T) {
	g := newGame(t)
	att, def, err := g.musterAttack(8, 7, attackersAt(g, 8, 0, true), 0)
	if err != nil {
		t.Fatal(err)
	}
	p := g.prepare(8, 7, att, def, 0, Supply{}, Aid{})
	p.Player = true
	g.PendingEvents()
	g.settle(p)
	for _, event := range g.PendingEvents() {
		if event.Bubble != nil && event.Bubble.MapBattle != nil {
			t.Fatal("玩家親征又重播大地圖戰爭動畫")
		}
	}
}
