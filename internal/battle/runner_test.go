package battle

import "testing"

// TestRunnerStopsOnHumanUnits 釘住輪到玩家的部隊會停下來，
// 電腦的部隊沿路自己打完。
func TestRunnerStopsOnHumanUnits(t *testing.T) {
	b := New(setup(2024))
	r := NewRunner(b, func(s Side) bool { return s.Attacking() })
	stops := 0
	for {
		u := r.Next()
		if u == nil {
			break
		}
		if !u.Side.Attacking() {
			t.Fatalf("停在守方的 %s 上，玩家指揮的是攻方", u.Name())
		}
		stops++
		if stops > 5000 {
			t.Fatal("停太多次，可能沒有前進")
		}
		// 玩家什麼都不做就結束這一支的回合。
		r.Done()
	}
	if stops == 0 {
		t.Error("一次都沒停下來讓玩家下令")
	}
	if !b.Over {
		t.Error("跑完了卻沒有分勝負")
	}
}

// TestRunnerWithoutHumanPlaysItself 釘住沒有玩家時整場自己打完。
func TestRunnerWithoutHumanPlaysItself(t *testing.T) {
	b := New(setup(555))
	r := NewRunner(b, nil)
	if u := r.Next(); u != nil {
		t.Fatalf("沒有玩家卻停在 %s 上", u.Name())
	}
	if !b.Over {
		t.Error("沒有分勝負")
	}
}

// TestRunnerLetsThePlayerAct 釘住停下來的那一支真的下得了令。
func TestRunnerLetsThePlayerAct(t *testing.T) {
	b := New(setup(31337))
	r := NewRunner(b, func(s Side) bool { return s == MainAttacker })
	u := r.Next()
	if u == nil {
		t.Fatal("開場就結束了")
	}
	if u.Side != MainAttacker {
		t.Fatalf("停在 %s 上", u.Side)
	}
	before := u.Move
	if err := b.Rest(u); err != nil {
		t.Fatalf("休息失敗：%v", err)
	}
	if u.Move != before+TuneRestMove {
		t.Errorf("休息之後移動力 %d，應該是 %d", u.Move, before+TuneRestMove)
	}
	r.Done()
	if r.Next() == u {
		t.Error("結束回合之後又輪到同一支部隊")
	}
}

// TestCommandNumbersMatchTheOriginal 釘住部隊層的指令編號與原版相同。
//
// 原版的選單是「1.移動 2.對戰 3.快戰／4.死戰 5.弓箭 6.策略／
// 7.查看 8.退兵 0.休息」（`AA.EXE` `0x46c02`，`docs/re/04` §4）。
func TestCommandNumbersMatchTheOriginal(t *testing.T) {
	want := map[Command]string{
		0: "休息", 1: "移動", 2: "對戰", 3: "快戰", 4: "死戰",
		5: "弓箭", 6: "策略", 7: "查看", 8: "退兵",
	}
	for n, name := range want {
		if n.String() != name {
			t.Errorf("第 %d 個指令是 %q，原版寫 %q", n, n.String(), name)
		}
	}
	if len(Commands()) != 9 {
		t.Errorf("部隊層有 %d 個指令，原版是 9 個", len(Commands()))
	}
}

// TestInspectCostsTenGold 釘住「查看敵軍須10金!」（原版 `0x46ccf`）。
func TestInspectCostsTenGold(t *testing.T) {
	b := arena(flat(Plain))
	spot := FromOffset(6, 6)
	u := place(b, MainAttacker, Centre, spot, lead("我", 50, 50, 1000))
	e := place(b, MainDefender, Centre, spot.Step(DirDown), lead("敵", 50, 50, 1000))
	friend := place(b, AidAttacker, Left, spot.Step(DirUp), lead("友", 50, 50, 1000))

	b.Gold[MainAttacker] = 9
	if _, err := b.Inspect(u, e.At); err == nil {
		t.Error("只有 9 金卻查看得了敵軍")
	}
	// 看自己人不用錢。
	if got, err := b.Inspect(u, friend.At); err != nil || got != friend {
		t.Errorf("查看友軍失敗：%v", err)
	}
	if b.Gold[MainAttacker] != 9 {
		t.Error("查看友軍不該扣錢")
	}

	b.Gold[MainAttacker] = 10
	got, err := b.Inspect(u, e.At)
	if err != nil {
		t.Fatalf("有 10 金卻查看不了：%v", err)
	}
	if got != e {
		t.Error("查看回傳的不是那一支敵軍")
	}
	if b.Gold[MainAttacker] != 0 {
		t.Errorf("查看之後剩 %d 金，應該扣掉 10", b.Gold[MainAttacker])
	}
	if _, err := b.Inspect(u, spot.Step(DirUpRight)); err == nil {
		t.Error("那裡沒有部隊，查看應該失敗")
	}
}
