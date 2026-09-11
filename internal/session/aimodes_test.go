package session_test

import (
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
			if f.Alive {
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
