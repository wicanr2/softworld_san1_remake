package game

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestHeirCandidatesFollowTheOriginalSort 釘住候選名單照原版的兩段做
// （`0x14a84` 收人、`0x14ad5` 交換排序）：整個勢力的人都算，魅力由高到低。
func TestHeirCandidatesFollowTheOriginalSort(t *testing.T) {
	g := newGame(t)
	list := g.HeirCandidates(g.Player)
	if len(list) < 2 {
		t.Fatalf("劇本一的玩家勢力只有 %d 位，排不出順序", len(list))
	}
	for i := 1; i < len(list); i++ {
		a, b := g.General(list[i-1]), g.General(list[i])
		if a.Charm < b.Charm {
			t.Errorf("第 %d 位魅力 %d 排在第 %d 位的 %d 前面", i, a.Charm, i+1, b.Charm)
		}
	}
	// **掃的是勢力欄，不是某一個郡**：名單裡的人可以散在不同的郡。
	for _, n := range list {
		if x := g.General(n); x.Faction != g.Player {
			t.Errorf("槽 %d 不是這一方的人", n)
		}
	}
	seen := map[int]bool{}
	for i := range g.generals {
		if g.generals[i].Faction == g.Player {
			seen[i] = true
		}
	}
	if len(seen) != len(list) {
		t.Errorf("名單 %d 位，勢力欄等於這一方的有 %d 位", len(list), len(seen))
	}
}

// TestPlayerPicksTheHeir 釘住「玩家自己挑繼承人」那一條（Issue #65）：
// 先扶排頭上去、問玩家、答了改套到挑中的那一位，**人望用繼承之前的值重算**。
//
// 反向對照在最後一段：挑排頭本人時什麼都不動。
func TestPlayerPicksTheHeir(t *testing.T) {
	run := func(pick int) (*State, []int, int) {
		g := newGame(t)
		f := g.Faction(g.Player)
		before := f.Prestige
		lord := g.General(f.Lord)
		if lord == nil {
			t.Fatal("劇本一的玩家沒有君主")
		}
		// 死者照原版先被寫成已故（`0x14a5d`），繼承常式才掃候選。
		g.retire(lord)
		list := g.HeirCandidates(g.Player)
		if len(list) < 2 {
			t.Fatalf("候選只有 %d 位，挑不出「不是排頭」的那一位", len(list))
		}
		who, ask := g.NeedsHeir()
		if who != g.Player || len(ask) != len(list) {
			t.Fatalf("沒有排進佇列：勢力 %d、%d 位", who, len(ask))
		}
		if pick < 0 {
			pick = len(list) - 1
		}
		if err := g.AssignHeir(list[pick]); err != nil {
			t.Fatal(err)
		}
		return g, list, before
	}

	// 挑**最後一位**（不是預設的排頭），才看得出玩家的選擇真的算數。
	g, list, before := run(-1)
	f := g.Faction(g.Player)
	last := list[len(list)-1]
	heir := g.General(last)
	if f.Lord != last {
		t.Errorf("君主是槽 %d，玩家挑的是 %d", f.Lord, last)
	}
	if heir.Status != state.StatusLord {
		t.Errorf("挑中的那一位身分是 %d，應該是君主", heir.Status)
	}
	if x := g.General(list[0]); x.Status == state.StatusLord {
		t.Error("先扶上去的排頭沒有被復原")
	}
	// 人望要拿**繼承之前**的值折，不是拿排頭折過的再折一次。
	if want := SuccessionPrestige(int(heir.Charm), before); f.Prestige != want {
		t.Errorf("人望 %d，用繼承前的 %d 折出來應該是 %d", f.Prestige, before, want)
	}
	if _, left := g.NeedsHeir(); len(left) != 0 {
		t.Error("答完了還在等")
	}

	// 挑排頭本人：等於沒換，人望與身分都是先扶上去那一次的結果。
	g2, list2, before2 := run(0)
	f2 := g2.Faction(g2.Player)
	head := g2.General(list2[0])
	if f2.Lord != list2[0] {
		t.Errorf("君主是槽 %d，應該是排頭 %d", f2.Lord, list2[0])
	}
	if want := SuccessionPrestige(int(head.Charm), before2); f2.Prestige != want {
		t.Errorf("人望 %d，應該是 %d", f2.Prestige, want)
	}
}
