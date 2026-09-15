package session_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 兩種 AI 全電腦跑同樣的月份，量出來的局面差在哪。
//
// 這一支回答的是「**強化 AI 到底有沒有在動**」。`internal/ai` 現有的三支
// 測試只驗 `enhanced` 合法、決定性、一郡一令——那三件事在一個**什麼都不做**
// 的 AI 上也全部成立（`CLAUDE.md` §7 第 14 條的同一個形狀）。
//
// ⚠ **這不是「誰比較強」的裁決**：兩者的驗收標準本來就不同
//（`base` 要對得上原版，`enhanced` 只要玩起來好）。這裡量的是
// **局面有沒有被推動**，那是「有沒有作用」的最低標準。
func TestZZAIModesMoveTheBoard(t *testing.T) {
	const months = 36
	type result struct {
		alive     int
		biggest   int
		blank     int
		soldiers  int
		gold      int
		turnovers int
	}
	got := map[ai.Mode]result{}

	for _, mode := range []ai.Mode{ai.ModeBase, ai.ModeEnhanced} {
		sc := loadScenarioForAI(t)
		g, err := game.New(sc, state.NoFaction, 5, state.EditionBase)
		if err != nil {
			t.Fatal(err)
		}
		brain, err := ai.New(mode)
		if err != nil {
			t.Fatal(err)
		}
		s := session.New(g, brain, state.NoFaction)

		// 起點的歸屬，用來數易主。
		before := map[int]state.FactionID{}
		for _, p := range g.Prefectures() {
			before[p.ID] = p.Owner
		}
		var r result
		for m := 0; m < months && !s.Over; m++ {
			// **電腦的命令不進 Log**（那是玩家的訊息欄），所以數 Log
			// 的長度量不到 AI 做了多少——要看的是盤面本身。
			s.EndMonth()
		}
		for _, p := range g.Prefectures() {
			if p.ID == 0 {
				continue
			}
			if !p.Owned() {
				r.blank++
			}
			if before[p.ID] != p.Owner {
				r.turnovers++
			}
			r.soldiers += g.Soldiers(p.ID)
			r.gold += p.Gold
		}
		for _, f := range g.Factions() {
			if len(g.Territory(f.ID)) > 0 { // 持郡才算存活（`Alive` 是絕嗣旗標）
				r.alive++
			}
			if n := len(g.Territory(f.ID)); n > r.biggest {
				r.biggest = n
			}
		}
		got[mode] = r
		t.Logf("%-8s 跑 %d 個月：存活勢力 %d、最大勢力 %d 郡、"+
			"空白郡 %d、總兵 %d、總金 %d、易主 %d 郡",
			mode, months, r.alive, r.biggest, r.blank,
			r.soldiers, r.gold, r.turnovers)
	}

	b, e := got[ai.ModeBase], got[ai.ModeEnhanced]
	// **最低標準：局面要被推動。** 三十六個月下來連一個郡都沒易主、
	// 兵也沒變，那就是這個 AI 沒有在下有效的命令。
	for mode, r := range got {
		if r.turnovers == 0 && r.soldiers == 0 {
			t.Errorf("%s 跑了 %d 個月，一個郡都沒易主、一個兵都沒有"+
				"——這個 AI 沒有在動", mode, months)
		}
	}
	// 兩邊要**真的不同**：一樣的話表示旗標沒接上，或 `enhanced` 只是
	// 換了個名字的 `base`。
	if b == e {
		t.Errorf("兩種 AI 三十六個月之後局面完全相同——旗標沒接上？")
	}
	t.Logf("差異：易主 %+d 郡、總兵 %+d、總金 %+d、最大勢力 %+d 郡",
		e.turnovers-b.turnovers, e.soldiers-b.soldiers,
		e.gold-b.gold, e.biggest-b.biggest)
}

func loadScenarioForAI(t *testing.T) *state.Scenario {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過（本儲存庫不含原版檔案）")
	}
	dir := filepath.Join(root, "三國演義")
	read := func(ext string) []byte {
		b, err := os.ReadFile(filepath.Join(dir, "DATA2."+ext))
		if err != nil {
			t.Skipf("讀不到 DATA2.%s：%v", ext, err)
		}
		return b
	}
	c, err := assets.OpenContainer(read("NAM"), read("IDX"), read("GRP"))
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(c, state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

// 守備門檻掃一遍：這個數字決定「擴張」與「守得住」的平衡。
//
// **這不是在調參數讓某個結果過關**（`rulebook/42`），是在量一條曲線：
// 門檻越高兵留得越多、打得越少，而兩端都是壞的——門檻 0 是先前那一版
//（打下 28 個郡、總兵只剩 base 的一半），門檻太高則一動也不動。
func TestZZGarrisonRatioSweep(t *testing.T) {
	const months = 36
	// ⚠ **0 是「用預設」不是「不設防」**（`ai.NewEnhanced` 的約定）。
	// 要量不設防那一端就給 1。
	for _, ratio := range []int{1, 40, 60, 80, 100, 140} {
		sc := loadScenarioForAI(t)
		g, err := game.New(sc, state.NoFaction, 5, state.EditionBase)
		if err != nil {
			t.Fatal(err)
		}
		s := session.New(g, ai.NewEnhanced(ratio), state.NoFaction)
		before := map[int]state.FactionID{}
		for _, p := range g.Prefectures() {
			before[p.ID] = p.Owner
		}
		for m := 0; m < months && !s.Over; m++ {
			s.EndMonth()
		}
		alive, biggest, turnovers, soldiers, gold := 0, 0, 0, 0, 0
		for _, p := range g.Prefectures() {
			if p.ID == 0 {
				continue
			}
			if before[p.ID] != p.Owner {
				turnovers++
			}
			soldiers += g.Soldiers(p.ID)
			gold += p.Gold
		}
		for _, f := range g.Factions() {
			if len(g.Territory(f.ID)) > 0 { // 持郡才算存活（`Alive` 是絕嗣旗標）
				alive++
			}
			if n := len(g.Territory(f.ID)); n > biggest {
				biggest = n
			}
		}
		name := fmt.Sprintf("%d%%", ratio)
		if ratio == 1 {
			name = "幾乎不設防"
		}
		t.Logf("守備門檻 %-5s：存活 %2d、最大 %2d 郡、易主 %2d 郡、"+
			"總兵 %6d、總金 %6d", name, alive, biggest, turnovers, soldiers, gold)
	}
}

// 指令數掃一遍：「其他 → 電腦指令」那一格（1–5）到底改變了什麼。
//
// **這一格調的是強化 AI 有多強，不是規則**——原版的電腦本來就不受
// 「每郡每月一道令」管（`docs/mechanics/70-ai` §2.12），所以 1 是
// 「與玩家同一條線」而不是「照原版」。預設 1 的理由是那條線最好懂：
// 玩家一個月一道令，電腦也是。
//
// ⚠ **這不是在挑一個最好的數字。** 量的是「這一格有沒有作用、往哪個
// 方向作用」——一個按下去什麼都不會變的設定，與沒做這個功能在畫面上
// 長得一模一樣（`internal/game/options.go` 的同一句）。
func TestZZAIOrdersSweep(t *testing.T) {
	const months = 36
	for _, n := range []int{1, 2, 3, 5} {
		sc := loadScenarioForAI(t)
		g, err := game.New(sc, state.NoFaction, 5, state.EditionBase)
		if err != nil {
			t.Fatal(err)
		}
		if err := g.Options.SetAIOrders(n); err != nil {
			t.Fatal(err)
		}
		s := session.New(g, ai.NewEnhanced(0), state.NoFaction)
		before := map[int]state.FactionID{}
		for _, p := range g.Prefectures() {
			before[p.ID] = p.Owner
		}
		for m := 0; m < months && !s.Over; m++ {
			s.EndMonth()
		}
		alive, biggest, turnovers, soldiers, gold, rice, land := 0, 0, 0, 0, 0, 0, 0
		owned := 0
		for _, p := range g.Prefectures() {
			if p.ID == 0 {
				continue
			}
			if before[p.ID] != p.Owner {
				turnovers++
			}
			soldiers += g.Soldiers(p.ID)
			gold += p.Gold
			rice += p.Rice
			if p.Owned() {
				land += int(p.LandValue)
				owned++
			}
		}
		for _, f := range g.Factions() {
			if len(g.Territory(f.ID)) > 0 { // 持郡才算存活（`Alive` 是絕嗣旗標）
				alive++
			}
			if k := len(g.Territory(f.ID)); k > biggest {
				biggest = k
			}
		}
		avgLand := 0
		if owned > 0 {
			avgLand = land / owned
		}
		t.Logf("每郡每月 %d 道令：存活 %2d、最大 %2d 郡、易主 %2d 郡、"+
			"總兵 %6d、總金 %6d、總米 %7d、平均地力 %2d",
			n, alive, biggest, turnovers, soldiers, gold, rice, avgLand)
	}
}
