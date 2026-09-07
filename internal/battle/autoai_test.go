package battle

import "testing"

// 電腦對電腦那條路的規則（`docs/re/05` §7.1）。

// TestAutoResolveAIStrongerAttackerWins 釘住「戰力被削到零就分勝負」。
func TestAutoResolveAIStrongerAttackerWins(t *testing.T) {
	b := arena(flat(Plain))
	b.Rice = [sideCount]int{MainAttacker: 30000, MainDefender: 30000}
	place(b, MainAttacker, Vanguard, FromOffset(2, 2), lead("攻", 90, 90, 8000))
	place(b, MainDefender, Vanguard, FromOffset(5, 5), lead("守", 30, 30, 500))
	b.AutoResolveAI()

	if !b.Over {
		t.Fatal("打完了卻沒有結果")
	}
	if !b.AttackerWon {
		t.Errorf("八千打五百，攻方應該贏；紀錄：%v", b.Log)
	}
	if n := b.Units[1].Soldiers(); n != 0 {
		t.Errorf("守方潰散之後還剩 %d 人，應該是 0", n)
	}
	// 贏的那一方也折損——存活比例是「結束時的戰力 ÷ 開場戰力」。
	if n := b.Units[0].Soldiers(); n >= 8000 {
		t.Errorf("攻方一個人都沒折損（%d），原版贏的那一方也要乘存活比例", n)
	}
}

// TestAutoResolveAIStarvation 釘住「每三天按戰力吃糧，糧盡即敗」。
func TestAutoResolveAIStarvation(t *testing.T) {
	// 兩邊勢均力敵、品質都是 0，戰力就不會互相削減，勝負只能由糧決定。
	b := arena(flat(Plain))
	b.Rice = [sideCount]int{MainAttacker: 1, MainDefender: 30000}
	place(b, MainAttacker, Vanguard, FromOffset(2, 2), lead("攻", 0, 0, 5000))
	place(b, MainDefender, Vanguard, FromOffset(5, 5), lead("守", 0, 0, 5000))
	b.AutoResolveAI()

	if !b.Over || b.AttackerWon {
		t.Fatalf("攻方只帶一石米，應該糧盡而敗；Over=%v 攻方勝=%v 紀錄=%v",
			b.Over, b.AttackerWon, b.Log)
	}
	if b.Day != 3 {
		t.Errorf("糧在第 %d 天用完，每三天吃一次應該是第 3 天", b.Day)
	}
}

// TestAutoResolveAITimeoutFavoursDefender 釘住「三十天期滿判守方勝」。
//
// **不看城池那一格，也不看統帥在不在**——那是玩家那條路的規則
// （§8、§8.1），電腦這條走不到。
func TestAutoResolveAITimeoutFavoursDefender(t *testing.T) {
	b := arena(flat(Plain))
	b.Rice = [sideCount]int{MainAttacker: 30000, MainDefender: 30000}
	place(b, MainAttacker, Vanguard, FromOffset(2, 2), lead("攻", 0, 0, 5000))
	place(b, MainDefender, Vanguard, FromOffset(5, 5), lead("守", 0, 0, 5000))
	b.CityHeld = MainAttacker // 攻方佔著城池也不算數
	b.AutoResolveAI()

	if !b.Over || b.AttackerWon {
		t.Fatalf("三十天期滿應該判守方勝；Over=%v 攻方勝=%v 紀錄=%v",
			b.Over, b.AttackerWon, b.Log)
	}
	if b.Day != BattleDays {
		t.Errorf("在第 %d 天結束，應該撐到第 %d 天", b.Day, BattleDays)
	}
}

// TestUnitAbilityMatchesTheOriginalFormula 釘住部隊 offset 32 的算式。
func TestUnitAbilityMatchesTheOriginalFormula(t *testing.T) {
	u := &Unit{Leaders: []Leader{
		lead("甲", 100, 100, 100), // 100×3÷5 ＋ 100×2÷5 ＝ 60 ＋ 40
		lead("乙", 50, 50, 100),   // 50×3÷5 ＋ 50×2÷5 ＝ 30 ＋ 20
	}}
	if got, want := u.Ability(), (100+50)/2; got != want {
		t.Errorf("綜合能力 %d，照 Σ(謀略×2÷5 ＋ 戰力×3÷5) ÷ 人數 是 %d", got, want)
	}
	if (&Unit{}).Ability() != 0 {
		t.Error("沒有將領的部隊應該回 0，不是除以零")
	}
}
