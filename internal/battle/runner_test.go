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
	if u.Move != before+RestMove {
		t.Errorf("休息之後移動力 %d，應該是 %d", u.Move, before+RestMove)
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

// TestCampMovesWithoutSpendingMoves 釘住紮營不扣移動力、不看距離。
//
// 原版是開戰前逐隊指定位置（`(%2d%s)%s之%s請%s將軍紮寨`），
// 那是佈陣不是行軍。
func TestCampMovesWithoutSpendingMoves(t *testing.T) {
	f := flat(Plain)
	b := arena(f)
	u := place(b, MainAttacker, Centre, FromOffset(2, 2), lead("甲", 50, 50, 1000))
	other := place(b, MainAttacker, Left, FromOffset(3, 3), lead("乙", 50, 50, 1000))
	move := u.Move

	far := FromOffset(10, 8)
	if err := b.Camp(u, far); err != nil {
		t.Fatalf("紮營到遠處失敗：%v", err)
	}
	if u.At != far {
		t.Errorf("紮營之後在 %v，應該是 %v", u.At, far)
	}
	if u.Move != move {
		t.Errorf("紮營扣了移動力：%d → %d", move, u.Move)
	}
	// 有人的格子紮不了。
	if err := b.Camp(u, other.At); err == nil {
		t.Error("那一格有人卻紮得了營")
	}
	// 過不去的地形紮不了。
	f.Set(FromOffset(5, 5), Mountain)
	if err := b.Camp(u, FromOffset(5, 5)); err == nil {
		t.Error("大山上紮得了營")
	}
	// 出界紮不了。
	if err := b.Camp(u, FromOffset(-1, 0)); err == nil {
		t.Error("界外紮得了營")
	}
	// 開戰之後就不能再紮營了。
	place(b, MainDefender, Centre, FromOffset(10, 10), lead("守", 50, 50, 1000))
	b.Rice[MainAttacker] = 10000
	b.EndDay()
	if err := b.Camp(u, FromOffset(4, 4)); err == nil {
		t.Error("第二天還紮得了營")
	}
}

// TestCampAreaMatchesCamp 釘住畫面問的與規則答的是同一件事。
func TestCampAreaMatchesCamp(t *testing.T) {
	f := flat(Plain)
	f.Set(FromOffset(4, 4), Mountain)
	b := arena(f)
	u := place(b, MainAttacker, Centre, FromOffset(2, 2), lead("甲", 50, 50, 1000))
	place(b, MainAttacker, Left, FromOffset(3, 3), lead("乙", 50, 50, 1000))
	for _, at := range []Hex{FromOffset(4, 4), FromOffset(3, 3), FromOffset(-1, 0),
		FromOffset(6, 6), FromOffset(2, 2)} {
		want := b.Camp(u, at) == nil
		// Camp 成功會把部隊移過去，移回來再問。
		if want {
			u.At = FromOffset(2, 2)
		}
		if got := b.CampArea(u, at); got != want {
			t.Errorf("%v：畫面說 %v，規則說 %v", at, got, want)
		}
	}
}
