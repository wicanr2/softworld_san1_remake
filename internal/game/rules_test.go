package game

import "testing"

// TestWarRecruitFollowsTheOriginal 釘住戰後收降的兩條式子
// （原版 `0x1ffbc`–`0x2003b`）。
func TestWarRecruitFollowsTheOriginal(t *testing.T) {
	// 抵抗值取謀略與戰力的較大者。
	if got := WarRecruitResistance(80, 40, false, 0); got != 80 {
		t.Errorf("謀略 80、戰力 40 的抵抗值是 %d，應該取較大的 80", got)
	}
	if got := WarRecruitResistance(40, 90, false, 0); got != 90 {
		t.Errorf("謀略 40、戰力 90 的抵抗值是 %d，應該取較大的 90", got)
	}
	// 牽絆同勢力時加 60 − RND(30)，也就是 31–60。
	for r := 0; r < WarRecruitBondSpread; r++ {
		add := WarRecruitResistance(50, 50, true, r) - 50
		if add < 31 || add > 60 {
			t.Fatalf("RND(30) 擲出 %d 時牽絆加了 %d，應該落在 31–60", r, add)
		}
	}
	if WarRecruitResistance(50, 50, false, 0) != 50 {
		t.Error("牽絆的對象不同勢力就不該加")
	}
	// 人望 >= 抵抗值 ÷ 2 才收得下來（整數除法）。
	if !WarRecruited(40, 80) {
		t.Error("人望 40 對抵抗值 80（門檻 40）應該收得下來")
	}
	if WarRecruited(39, 80) {
		t.Error("人望 39 對抵抗值 80（門檻 40）不該收得下來")
	}
	if !WarRecruited(40, 81) {
		t.Error("抵抗值 81 的門檻是 40（整數除法），人望 40 應該收得下來")
	}
}

// TestWarRecruitLoyalty 釘住「收編之後的忠誠 ＝ min(100, 勝方的人望)」
// （原版 `0x1fee6`／`0x1ff27`）。
func TestWarRecruitLoyalty(t *testing.T) {
	for _, c := range []struct{ prestige, want int }{
		{0, 0}, {55, 55}, {100, 100}, {120, 100},
	} {
		if got := WarRecruitLoyalty(c.prestige); got != c.want {
			t.Errorf("人望 %d 收編之後的忠誠是 %d，應該是 %d",
				c.prestige, got, c.want)
		}
	}
}
