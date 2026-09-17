package battle

import (
	"reflect"
	"testing"
)

// fakeHost 是測試用的 `CaptiveHost`：在野空位與釋放的去處都由測試擺。
type fakeHost struct {
	room    bool
	added   int
	release []int // ReleaseTo 的清單；空就回 −1、不擲
}

func (h *fakeHost) IdleRoom() bool { return h.room }
func (h *fakeHost) AddIdle()       { h.added++ }
func (h *fakeHost) ReleaseTo(_ int, roll func(int) int) int {
	if len(h.release) == 0 {
		return -1
	}
	return h.release[roll(len(h.release))]
}

// captiveArena 擺一場玩家（攻方）抓到守將的戰役，記下每一次擲骰的範圍。
func captiveArena(t *testing.T) (*Battle, *Unit, *Leader, *[]int) {
	t.Helper()
	b := arena(flat(Plain))
	b.Computer[MainDefender] = true
	b.Renown[MainAttacker] = 100
	place(b, MainAttacker, Centre, FromOffset(1, 1), lead("甲", 80, 80, 1000))
	d := place(b, MainDefender, Centre, FromOffset(5, 5), lead("乙", 10, 10, 0))
	var ns []int
	b.UseRoll(func(n int) int {
		ns = append(ns, n)
		return 0
	})
	return b, d, &d.Leaders[0], &ns
}

// TestPlayerCaptiveAsksAgainAfterARefusal 釘住 `docs/spec/018` §1：原版回
// −1 的處置（君主不能囚禁、招降不從）要再問一次，而且拒絕的那幾條
// 擲骰的次數照原版——囚禁被拒一擲都沒有，不從先印一句（`RND(8)`）。
func TestPlayerCaptiveAsksAgainAfterARefusal(t *testing.T) {
	b, d, x, ns := captiveArena(t)
	x.Lord = true
	answers := []Fate{Jailed, Defected, Executed}
	asked := 0
	b.PlayerCaptive = func(captor Side, got *Leader) Fate {
		if captor != MainAttacker || got != x {
			t.Fatalf("問的是 %v 的 %s", captor, got.Name)
		}
		f := answers[asked]
		asked++
		return f
	}
	b.Capture(MainAttacker, d, x)
	if asked != 3 || x.Fate != Executed {
		t.Fatalf("問了 %d 次、結果 %v；想要 3 次、斬首", asked, x.Fate)
	}
	// 君主：囚禁被拒不擲；招降判定君主直接 0 不擲 RND(3)，不從一擲 RND(8)；
	// 斬首一擲 RND(8)。
	if want := []int{MessageLines, MessageLines}; !reflect.DeepEqual(*ns, want) {
		t.Errorf("擲骰 %v，想要 %v", *ns, want)
	}
}

// TestPlayerReleaseRollsTheDestinationFirst 釘住釋放的骰序（`0x262b8`）：
// 去處 `RND(清單長)` 在對白 `RND(8)` 之前、場景圖 `RND(4)` 最後；沒有
// 去處時不擲那一次，而且記一位在野。
func TestPlayerReleaseRollsTheDestinationFirst(t *testing.T) {
	for _, tc := range []struct {
		name    string
		release []int
		wantNs  []int
		wantTo  int
		wantAdd int
	}{
		{"有去處", []int{7, 9, 11}, []int{3, MessageLines, EffectVariants}, 7, 0},
		{"沒有去處", nil, []int{MessageLines, EffectVariants}, -1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, d, x, ns := captiveArena(t)
			h := &fakeHost{room: true, release: tc.release}
			b.Host = h
			b.PlayerCaptive = func(Side, *Leader) Fate { return Released }
			b.Capture(MainAttacker, d, x)
			if x.Fate != Released || x.ReleasedTo != tc.wantTo || h.added != tc.wantAdd {
				t.Errorf("結果 %v 去處 %d 在野 +%d；想要釋放、%d、+%d", x.Fate, x.ReleasedTo, h.added, tc.wantTo, tc.wantAdd)
			}
			if !reflect.DeepEqual(*ns, tc.wantNs) {
				t.Errorf("擲骰 %v，想要 %v", *ns, tc.wantNs)
			}
		})
	}
}

// TestPlayerJailNeedsIdleRoom 釘住囚禁的第二道拒絕（`0x26118`：戰場郡在野
// 滿 50）：被拒之後再問，換成斬首。
func TestPlayerJailNeedsIdleRoom(t *testing.T) {
	b, d, x, _ := captiveArena(t)
	b.Host = &fakeHost{room: false}
	answers := []Fate{Jailed, Released, Executed}
	asked := 0
	b.PlayerCaptive = func(Side, *Leader) Fate { asked++; return answers[asked-1] }
	b.Capture(MainAttacker, d, x)
	if asked != 3 || x.Fate != Executed {
		t.Fatalf("問了 %d 次、結果 %v；在野滿了囚禁與釋放都該被拒", asked, x.Fate)
	}
}

// TestPlayerCampAsksUntilTheCellFits 釘住中途紮寨（`0x2731a`）：招降來的人
// 進了空部隊，玩家那一方問 `PlayerCamp`，挑到紮得下的格子為止。
func TestPlayerCampAsksUntilTheCellFits(t *testing.T) {
	b, d, x, _ := captiveArena(t)
	x.Loyalty = 0
	b.Host = &fakeHost{room: true}
	b.PlayerCaptive = func(Side, *Leader) Fate { return Defected }
	taken := b.Units[0].At // 攻方中軍站的格子
	want := FromOffset(3, 3)
	tries := []Hex{taken, want}
	var asked []*Unit
	b.PlayerCamp = func(u *Unit) Hex {
		asked = append(asked, u)
		return tries[len(asked)-1]
	}
	b.Capture(MainAttacker, d, x)
	if x.Fate != Defected {
		t.Fatalf("結果 %v，想要招降", x.Fate)
	}
	if len(asked) != 2 {
		t.Fatalf("問了 %d 次紮寨，想要 2 次（第一格有人）", len(asked))
	}
	u := asked[1]
	if u.At != want || u.Unplaced || u.Side != MainAttacker || u.LeaderCount() != 1 {
		t.Errorf("新部隊在 %v（Unplaced %v、%v、%d 位），想要 %v", u.At, u.Unplaced, u.Side, u.LeaderCount(), want)
	}
}
