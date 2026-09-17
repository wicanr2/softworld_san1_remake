package game

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestDemoCameraOnlyRunsWithoutPlayers 釘住鏡頭只在 0 人局擲：有玩家的局月底
// 骰數不變、被看的郡與人不動；0 人局換到「別的主人」的郡、換一位有勢力的人，
// 偶數月排郡資料面板、奇數月排人物卡。
func TestDemoCameraOnlyRunsWithoutPlayers(t *testing.T) {
	sc := loadScenario(t, state.Scenario1)
	withPlayer, err := New(sc, 0, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	withPlayer.EndMonth()
	if p, x := withPlayer.DemoCamera(); p != DemoCameraPrefecture || x != DemoCameraPerson {
		t.Errorf("有玩家的局鏡頭動了：郡 %d 人 %d", p, x)
	}

	for _, month := range []int{1, 2} {
		g, err := NewPlayers(sc, nil, 5, state.EditionBase)
		if err != nil {
			t.Fatal(err)
		}
		g.SeedRand(0x13579BDF)
		g.Date.Month = month
		oldOwner := g.Prefecture(DemoCameraPrefecture).Owner
		events := g.RunDemoCamera()
		p, x := g.DemoCamera()
		if q := g.Prefecture(p); q == nil || !q.Owned() || q.Owner == oldOwner {
			t.Errorf("%d 月換到郡 %d，主人要是別人（原本 %d）", month, p, oldOwner)
		}
		if who := g.General(x); who == nil || who.Faction == state.NoFaction || x == DemoCameraPerson {
			t.Errorf("%d 月換到人物 %d，要有勢力而且不是原本那一位", month, x)
		}
		if len(events) != 1 || events[0].Bubble == nil {
			t.Fatalf("%d 月的事件 %+v", month, events)
		}
		b := events[0].Bubble
		if month%2 == 0 && b.Panel != p {
			t.Errorf("偶數月要排郡 %d 的資料面板，排了 %+v", p, b)
		}
		if month%2 == 1 && (!b.Card || b.Speaker != x) {
			t.Errorf("奇數月要排人物 %d 的卡，排了 %+v", x, b)
		}
	}
}
