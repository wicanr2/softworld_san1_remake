package game

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestCommandSceneRollsOnceForThePlayerOnly 釘住主畫面命令的場景圖
// （`docs/spec/010` §8）：玩家親自下令時擲一次 `RND(4)`、排一格場景圖在
// (432,80)；電腦的同一道命令與自治的郡不擲也不排——原版那幾支常式只在
// 玩家下令時跑，電腦走分派器。
func TestCommandSceneRollsOnceForThePlayerOnly(t *testing.T) {
	g := newGame(t)
	at := 0
	for _, p := range g.Prefectures() {
		if p.Owned() && p.Owner == 0 {
			at = p.ID
			break
		}
	}
	g.SeedRand(0x13579bdf)

	g.commandScene(at, assets.SceneTrain, 0)
	ev := g.PendingEvents()
	if g.RandDraws() != 1 || len(ev) != 1 || ev[0].Bubble == nil {
		t.Fatalf("玩家下令該擲一次、排一格：抽 %d 次、排 %+v", g.RandDraws(), ev)
	}
	b := ev[0].Bubble
	if b.Scene != assets.SceneTrain || b.X1 != assets.SceneMainX || b.Y1 != assets.SceneMainY ||
		b.Style < 0 || b.Style >= EffectVariants || !b.Wiped() {
		t.Errorf("場景圖那一格不對：%+v", b)
	}

	other := state.FactionID(0)
	for _, p := range g.Prefectures() {
		if p.Owned() && p.Owner != 0 {
			other = p.Owner
			break
		}
	}
	g.commandScene(at, assets.SceneTrain, other)
	if g.RandDraws() != 1 || len(g.PendingEvents()) != 0 {
		t.Errorf("電腦的命令不該擲也不該排：抽 %d 次", g.RandDraws())
	}

	// 自治要主事者不是君主的郡，劉備只有一個郡；找一家有這種郡的勢力
	// 另開一局當玩家。
	sc := loadScenario(t, state.Scenario1)
	auto, who := 0, state.FactionID(0)
	for _, q := range g.Prefectures() {
		if !q.Owned() {
			continue
		}
		if gov := g.Governor(q.ID); gov != nil && gov.Status != state.StatusLord {
			auto, who = q.ID, q.Owner
			break
		}
	}
	if auto == 0 {
		t.Fatal("劇本一沒有主事者不是君主的郡，自治那一條要換一個盤面")
	}
	g, err := New(sc, who, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	g.SeedRand(0x13579bdf)
	g.Prefecture(auto).Autonomy = AutoSelf
	if _, ok := g.AutonomousFor(auto); !ok {
		t.Fatalf("郡 %d 設成自冶型之後沒有交給電腦", auto)
	}
	g.commandScene(auto, assets.SceneTrain, who)
	if g.RandDraws() != 0 || len(g.PendingEvents()) != 0 {
		t.Errorf("自治的郡不該擲也不該排：抽 %d 次", g.RandDraws())
	}
}
