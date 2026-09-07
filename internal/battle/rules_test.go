package battle

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestRulesFollowEdition 釘住版本規則的對照表（`docs/spec/004` §5）。
func TestRulesFollowEdition(t *testing.T) {
	for _, c := range []struct {
		ed     state.Edition
		diff   int
		ignore bool
	}{
		{state.EditionBase, 1, false}, {state.EditionBase, 10, false},
		{"", 5, false}, // 版本空字串當原版
		{state.EditionPlus, 1, false}, {state.EditionPlus, 10, false},
		{state.EditionPlus, 11, true}, {state.EditionPlus, 20, true},
	} {
		if got := RulesFor(c.ed, c.diff).DefenderCommanderLossIgnored; got != c.ignore {
			t.Errorf("%q 難度 %d：守方統帥全滅不算敗 ＝ %v，原版讀出來是 %v",
				c.ed, c.diff, got, c.ignore)
		}
	}
	// **零值必須是原版。** 沒設 Rules 的呼叫端拿到的要是原版規則。
	if (Rules{}) != RulesFor(state.EditionBase, 5) {
		t.Error("Rules 的零值不是原版")
	}
}

// killSide 把某一方的將領全部打成陣亡。
func killSide(b *Battle, s Side) {
	for _, u := range b.Units {
		if u.Side != s {
			continue
		}
		for i := range u.Leaders {
			u.Leaders[i].Dead = true
			u.Leaders[i].Soldiers = 0
		}
	}
}

// killChief 只打掉一方的統帥，其餘的人與兵力原封不動。
//
// **這是統帥條件與全滅條件的分界**：兵還在、統帥不在。
func killChief(b *Battle, s Side) {
	id := b.Commander[s]
	for _, u := range b.Units {
		if u.Side != s {
			continue
		}
		for i := range u.Leaders {
			if u.Leaders[i].Index == id {
				u.Leaders[i].Dead = true
				u.Leaders[i].Soldiers = 0
			}
		}
	}
}

// TestCommanderLossDecidesIt 釘住原版的統帥條件（`0x24f8c`）。
//
// **兵還在也照樣結束**：原版比的是軍力記錄的統帥與該方第一支部隊的
// 第一位將領，統帥死了就對不上。只驗「全滅才結束」會讓這一條完全
// 沒被測到，而兩者在多數戰役裡結果相同。
func TestCommanderLossDecidesIt(t *testing.T) {
	for _, c := range []struct {
		side Side
		won  bool // 攻方贏不贏
		name string
	}{
		{MainDefender, true, "守方統帥"},
		{MainAttacker, false, "攻方統帥"},
	} {
		b := New(setup(777))
		if !b.CommanderAlive(c.side) {
			t.Fatalf("%s 開場就不在陣中", c.name)
		}
		killChief(b, c.side)
		if b.CommanderAlive(c.side) {
			t.Fatalf("%s 打死了還算在陣中", c.name)
		}
		if !b.sideAlive(c.side) {
			t.Fatalf("%s 那一方應該還有兵——這一條測的是「兵還在但統帥不在」", c.name)
		}
		b.checkOver()
		if !b.Over {
			t.Errorf("%s 不在陣中，戰役應該結束", c.name)
		}
		if b.AttackerWon != c.won {
			t.Errorf("%s 不在陣中，攻方獲勝 ＝ %v，應該是 %v", c.name, b.AttackerWon, c.won)
		}
	}
}

// TestPlusHardIgnoresDefenderCommanderLoss 釘住加強版難度 11–20 的差異
// （`0x2296b`，README 第 5 條）。
//
// 兩件事各自要成立：
//
//	一、守方統帥不在了**不判攻方勝**，戰役繼續
//	二、守方的兵打光了**還是判攻方勝**（加強版的「總兵數為 0 者敗」，
//	    `0x229ba`，不看難度）
//
// **只驗第一件會讓「守方無敵」看起來是對的**——那是規則接錯，不是加強。
func TestPlusHardIgnoresDefenderCommanderLoss(t *testing.T) {
	hard := RulesFor(state.EditionPlus, 15)

	s := setup(777)
	s.Rules = hard
	b := New(s)
	killChief(b, MainDefender)
	b.checkOver()
	if b.Over {
		t.Errorf("加強版難度 15：守方統帥不在也不該判勝負，卻結束了（攻方勝 ＝ %v）", b.AttackerWon)
	}

	// 攻方統帥那一半沒有被關掉。
	s2 := setup(777)
	s2.Rules = hard
	b2 := New(s2)
	killChief(b2, MainAttacker)
	b2.checkOver()
	if !b2.Over || b2.AttackerWon {
		t.Errorf("加強版難度 15：攻方統帥不在仍該判守方勝，得到 Over=%v 攻方勝=%v",
			b2.Over, b2.AttackerWon)
	}

	// 兵打光還是敗。
	s3 := setup(777)
	s3.Rules = hard
	b3 := New(s3)
	killSide(b3, MainDefender)
	b3.checkOver()
	if !b3.Over || !b3.AttackerWon {
		t.Errorf("加強版難度 15：守方總兵數為零仍該判攻方勝，得到 Over=%v 攻方勝=%v",
			b3.Over, b3.AttackerWon)
	}

	// 原版同樣的局面在統帥那一關就結束了。
	s4 := setup(777)
	b4 := New(s4)
	killChief(b4, MainDefender)
	b4.checkOver()
	if !b4.Over || !b4.AttackerWon {
		t.Errorf("原版：守方統帥不在應該判攻方勝，得到 Over=%v 攻方勝=%v",
			b4.Over, b4.AttackerWon)
	}
}
