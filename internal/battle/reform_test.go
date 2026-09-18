package battle

import "testing"

// TestReformFollowsThePlayersAssignment 釘住玩家的整編（Issue #98）：
// 分到第幾軍照玩家給的，**沒有被分配的人不出征**。
func TestReformFollowsThePlayersAssignment(t *testing.T) {
	b := arena(flat(Plain))
	var pool []Leader
	for i := 0; i < 6; i++ {
		l := lead("將", uint8(10+i), 50, 100)
		l.Index = 100 + i // `lead` 不填槽號，這支測試要靠它分辨誰是誰
		pool = append(pool, l)
	}
	b.Units = b.formUp(MainAttacker, pool, FromOffset(2, 2))
	b.base[MainAttacker] = FromOffset(2, 2)

	// 六位：第 1、3 位進中軍，第 2 位進先鋒，第 5 位進左軍，第 4、6 位不出征。
	if err := b.Reform(MainAttacker, []int{1, 5, 1, 0, 2, 0}); err != nil {
		t.Fatal(err)
	}
	got := map[Formation][]int{}
	for _, u := range b.Units {
		if u.Side != MainAttacker {
			continue
		}
		for _, l := range u.Leaders {
			got[u.Formation] = append(got[u.Formation], l.Index)
		}
	}
	order := DeployOrder()
	want := map[Formation][]int{
		order[0]: {pool[0].Index, pool[2].Index},
		order[4]: {pool[1].Index},
		order[1]: {pool[4].Index},
	}
	if len(got) != len(want) {
		t.Fatalf("分成 %d 隊，應該是 %d 隊：%v", len(got), len(want), got)
	}
	for f, w := range want {
		g := got[f]
		if len(g) != len(w) {
			t.Errorf("%s 有 %d 位，應該是 %d 位", f, len(g), len(w))
			continue
		}
		for i := range w {
			if g[i] != w[i] {
				t.Errorf("%s 第 %d 位是槽 %d，應該是 %d", f, i+1, g[i], w[i])
			}
		}
	}
	// 沒分配的兩位不在場上——出征名單是整編決定的（`0x20c9a` 只把編進
	// 部隊的人的所在郡寫 0）。
	for _, u := range b.Units {
		for _, l := range u.Leaders {
			if l.Index == pool[3].Index || l.Index == pool[5].Index {
				t.Errorf("槽 %d 沒有被分配卻上了戰場", l.Index)
			}
		}
	}
	// 統帥跟著換：勝負判定拿它與第一支部隊的第一位比（`0x24f8c`）。
	if b.Commander[MainAttacker] != pool[0].Index {
		t.Errorf("統帥是槽 %d，應該是第一支部隊的第一位 %d",
			b.Commander[MainAttacker], pool[0].Index)
	}
}

// TestReformRejectsBadAssignments 是反向對照：擋得住的才算規則。
func TestReformRejectsBadAssignments(t *testing.T) {
	newBoard := func(n int) *Battle {
		b := arena(flat(Plain))
		var pool []Leader
		for i := 0; i < n; i++ {
			l := lead("將", uint8(10+i), 50, 100)
			l.Index = 100 + i
			pool = append(pool, l)
		}
		b.Units = b.formUp(MainAttacker, pool, FromOffset(2, 2))
		b.base[MainAttacker] = FromOffset(2, 2)
		return b
	}
	cases := []struct {
		name   string
		n      int
		groups []int
	}{
		{"位置數對不上", 3, []int{1, 1}},
		{"軍的號碼越界", 3, []int{1, 6, 1}},
		{"一位都沒分配", 3, []int{0, 0, 0}},
	}
	for _, c := range cases {
		if err := newBoard(c.n).Reform(MainAttacker, c.groups); err == nil {
			t.Errorf("%s：應該被擋下來", c.name)
		}
	}
	// 一軍最多 10 位（說明書 p.27）。
	b := newBoard(11)
	all := make([]int, 11)
	for i := range all {
		all[i] = 1
	}
	if err := b.Reform(MainAttacker, all); err == nil {
		t.Error("十一位塞進同一軍應該被擋下來")
	}
	// 開打之後不准重編。
	b2 := newBoard(3)
	b2.Day = 2
	if err := b2.Reform(MainAttacker, []int{1, 2, 3}); err == nil {
		t.Error("開打之後還能重編")
	}
}

// TestArmyNumbersMatchTheOriginal 釘住「將%s分到那一軍(1-5)」的編號
// 對到哪一支隊伍（Issue #98，`L1`、`[base]`）。
//
// **證據是原版自己印出來的軍名**：`TestZZPlayerSortieDriven` 在整編時
// 逐位送 1–5，原版在下一問的視窗裡印出那一位分到的軍——
// 1 中軍、2 先鋒、3 左軍、4 右軍、5 後軍。與 `DeployOrder` 逐項相同。
//
// ⚠ **不能拿部隊記錄的索引反推**：那個索引是**分配的順序**，不是軍別
// （量到過三位分別分到先鋒、左軍、右軍，卻落在記錄 0、1、2）。
func TestArmyNumbersMatchTheOriginal(t *testing.T) {
	want := []Formation{Centre, Vanguard, Left, Right, Rear}
	got := DeployOrder()
	if len(got) != len(want) {
		t.Fatalf("有 %d 種隊伍，應該是 %d 種", len(got), len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("第 %d 軍是 %s，原版印的是 %s", i+1, got[i], w)
		}
	}
}

// TestNoInterfaceCaptiveCounterFires 是上面那個量測的**正對照**
// （Issue #100）：先證明計數器數得到，`internal/session` 量到的 0 才有意義。
//
// 沒有這一段的話，0 相容於兩個世界：真的沒發生，或者計數器根本沒接上。
func TestNoInterfaceCaptiveCounterFires(t *testing.T) {
	ResetNoInterfaceCaptives()
	b := arena(flat(Plain))
	b.Computer[MainAttacker] = false // 玩家那一方
	b.PlayerCaptive = nil            // 沒有介面
	x := lead("被擒", 50, 50, 100)
	x.Index = 7
	u := &Unit{Side: MainDefender, Formation: Centre, Leaders: []Leader{x}}
	b.Units = append(b.Units, u)
	b.capture(MainAttacker, u, &u.Leaders[0])
	if got := NoInterfaceCaptives(); got != 1 {
		t.Errorf("數到 %d 次，應該是 1 次——計數器沒接上", got)
	}
	if u.Leaders[0].Fate != FateNone {
		t.Errorf("處置是 %v，現況應該是留著不處置", u.Leaders[0].Fate)
	}
	// 有介面時不算：那一條是玩家自己答。
	ResetNoInterfaceCaptives()
	b.PlayerCaptive = func(Side, *Leader) Fate { return Executed }
	b.capture(MainAttacker, u, &u.Leaders[0])
	if got := NoInterfaceCaptives(); got != 0 {
		t.Errorf("有介面卻數了 %d 次", got)
	}
}
