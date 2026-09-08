package battle

import "testing"

// TestArrowsMatchesManualExample 釘住手冊 p.32 自己算給我們看的那一題。
//
// 「部隊各單武裝度平均值 ÷ 20 後取整數」，
// 例：`(75 + 80 + 50 + 100) ÷ 4 = 76.25`，`76.25 ÷ 20 = 3.8125`，射三次。
//
// ⚠ **手冊的算例看起來像算術平均，碼裡是兵數加權**（`0x272eb`）。
// 算例沒提兵數，所以兩種算法在那一組數字上分不出來；以碼為準。
func TestArrowCountIsWeightedByTroops(t *testing.T) {
	// 兵數刻意偏在武裝度最低的那一位身上：算術平均是 76（射三次），
	// 兵數加權落在 50 附近（射兩次）。兩條路的答案不同，這一題才問得出
	// 我們走的是哪一條。
	ls := []Leader{
		{Name: "甲", Arms: 75, Soldiers: 10},
		{Name: "乙", Arms: 80, Soldiers: 10},
		{Name: "丙", Arms: 50, Soldiers: 10000},
		{Name: "丁", Arms: 100, Soldiers: 10},
	}
	if got := ArrowCount(ls); got != 2 {
		t.Errorf("弓箭次數 %d，兵數加權算出來是 2 次（算術平均會是 3）", got)
	}
	u := &Unit{Leaders: ls}
	if u.AvgArms()/20 != 2 {
		t.Fatal("這組數字沒有拉開加權與算術平均的差距，測不到想測的東西")
	}
}

// TestArrowCountIgnoresLostLeaders 釘住死掉、被擒、沒有兵的人不算。
func TestArrowCountIgnoresLostLeaders(t *testing.T) {
	ls := []Leader{
		{Arms: 100, Soldiers: 100},
		{Arms: 100, Soldiers: 100},
		{Arms: 0, Soldiers: 100, Dead: true},
		{Arms: 0, Soldiers: 100, Captured: true},
		{Arms: 0, Soldiers: 0},
	}
	if got := ArrowCount(ls); got != 5 {
		t.Errorf("弓箭次數 %d，只算還在的兩位，應該是 100/20 = 5", got)
	}
	if ArrowCount(nil) != 0 {
		t.Error("空隊伍應該射不出箭")
	}
	if ArrowCount([]Leader{{Arms: 100, Soldiers: 0}}) != 0 {
		t.Error("沒有兵就沒有箭")
	}
}

// TestArrowsAreSpent 釘住箭**會用完**。
//
// 原版把次數存在部隊記錄 offset 20，每射一次遞減（`0x2abae`）；
// 每次都重算的話一支部隊可以無限次射滿箭，而戰報上只看得出
// 「這一場箭特別多」。
func TestArrowsAreSpent(t *testing.T) {
	b := arena(flat(Plain))
	from := FromOffset(4, 6)
	a := place(b, MainAttacker, Vanguard, from, lead("射", 50, 50, 5000))
	tgt := place(b, MainDefender, Centre, from.Step(DirUpRight).Step(DirUpRight),
		lead("靶", 50, 50, 5000))
	if a.Arrows <= 0 {
		t.Fatalf("開場的箭是 %d，應該大於 0", a.Arrows)
	}
	if err := b.Archery(a, tgt.At); err != nil {
		t.Fatalf("射箭失敗：%v", err)
	}
	if a.Arrows != 0 {
		t.Errorf("射完剩 %d 支，remake 一次射完整壺", a.Arrows)
	}
	a.Move = a.MovePoints()
	if err := b.Archery(a, tgt.At); err == nil {
		t.Error("箭射完了還射得出來")
	}
}

// TestAvgIsWeightedBySoldiers 釘住訓練度與武裝度是**兵數加權**。
//
// 一支一千人的精兵配一支十人的新兵，算術平均會把整隊看成中等。
func TestAvgIsWeightedBySoldiers(t *testing.T) {
	u := &Unit{Leaders: []Leader{
		{Training: 100, Arms: 100, Soldiers: 1000},
		{Training: 0, Arms: 0, Soldiers: 10},
	}}
	if got := u.AvgTraining(); got < 95 {
		t.Errorf("加權訓練度 %d，一千人精兵配十人新兵應該接近 100", got)
	}
	if got := u.AvgArms(); got < 95 {
		t.Errorf("加權武裝度 %d，應該接近 100", got)
	}
}

// TestChiefAndSmartest 釘住領隊看戰力、用計看謀略，而且都跳過陣亡被擒的人。
func TestChiefAndSmartest(t *testing.T) {
	u := &Unit{Leaders: []Leader{
		lead("勇", 99, 10, 100),
		lead("智", 20, 99, 100),
		lead("已歿", 100, 100, 100),
	}}
	u.Leaders[2].Dead = true
	if c := u.Chief(); c == nil || c.Name != "勇" {
		t.Errorf("領隊是 %v，應該是戰力最高的活人「勇」", c)
	}
	if s := u.Smartest(); s == nil || s.Name != "智" {
		t.Errorf("軍師是 %v，應該是謀略最高的活人「智」", s)
	}
	if (&Unit{}).Chief() != nil {
		t.Error("空隊伍不該有領隊")
	}
}

// TestSoldiersExcludeLostLeaders 釘住被擒與陣亡的人不再貢獻兵力。
func TestSoldiersExcludeLostLeaders(t *testing.T) {
	u := &Unit{Leaders: []Leader{
		{Soldiers: 100},
		{Soldiers: 100, Dead: true},
		{Soldiers: 100, Captured: true},
	}}
	if got := u.Soldiers(); got != 100 {
		t.Errorf("兵力 %d，只有一位將領還在，應該是 100", got)
	}
}

// TestTroopIsMajorityBySoldiers 釘住部隊兵種取兵數最多的那一種。
func TestTroopIsMajorityBySoldiers(t *testing.T) {
	u := &Unit{Leaders: []Leader{
		{Soldiers: 100, Troop: TroopLand},
		{Soldiers: 900, Troop: TroopWaterOnly},
	}}
	if got := u.Troop(); got != TroopWaterOnly {
		t.Errorf("部隊兵種是 %s，九成是水軍應該算水軍", got)
	}
}

// TestMovePointsHasFloor 釘住移動力的下限與方向。
//
// 下限是 2：`0x271b8` 對算出來的值 `inc ax` 兩次，一個人都沒有的部隊
// 走 `0x271f9` 那條分支，直接寫 2。
//
// ⚠ **武裝度在這條公式裡是加分**（係數 ¼），與說明書 p.30
// 「全副武裝將稍減移動力」相反。**以碼為準**：`0x2716c` 是 `fiadds`，
// 而且原版量到訓 97／武裝 97 的部隊是 12、訓 80／武裝 80 是 10
// （`TestBattleUnitsMatchTheOriginal`），都高過同訓練度輕裝的值。
// 說明書那半句對應的是對戰子畫面裡的另一支（`0x2e07a`）。
func TestMovePointsHasFloor(t *testing.T) {
	empty := &Unit{}
	if got := empty.MovePoints(); got != MoveFloor {
		t.Errorf("空部隊的移動力是 %d，應該是下限 %d", got, MoveFloor)
	}
	u := &Unit{Leaders: []Leader{{Soldiers: 100, Training: 0, Arms: 255}}}
	if got := u.MovePoints(); got < MoveFloor {
		t.Errorf("移動力 %d 低於下限 %d", got, MoveFloor)
	}
	// 訓練度高的走得比較遠（說明書 p.30：「移動力來源是訓練度…」）。
	slow := &Unit{Leaders: []Leader{{Soldiers: 100, Training: 0, Arms: 0}}}
	fast := &Unit{Leaders: []Leader{{Soldiers: 100, Training: 100, Arms: 0}}}
	if fast.MovePoints() <= slow.MovePoints() {
		t.Errorf("訓練度 100 的移動力 %d 沒有多於訓練度 0 的 %d",
			fast.MovePoints(), slow.MovePoints())
	}
	light := &Unit{Leaders: []Leader{{Soldiers: 100, Training: 100, Arms: 0}}}
	heavy := &Unit{Leaders: []Leader{{Soldiers: 100, Training: 100, Arms: 100}}}
	if heavy.MovePoints() <= light.MovePoints() {
		t.Errorf("全副武裝的移動力 %d 沒有多於輕裝的 %d——"+
			"這條公式裡武裝是加分（0x2716c 的 fiadds）",
			heavy.MovePoints(), light.MovePoints())
	}
}

// TestOrdersFollowManual 釘住手冊 p.28 的四張順序表。
func TestOrdersFollowManual(t *testing.T) {
	want := func(name string, got, exp []Formation) {
		if len(got) != len(exp) {
			t.Fatalf("%s 有 %d 項，應該是 %d 項", name, len(got), len(exp))
		}
		for i := range exp {
			if got[i] != exp[i] {
				t.Errorf("%s 第 %d 項是 %s，應該是 %s", name, i+1, got[i], exp[i])
			}
		}
	}
	want("作戰順序", ActionOrder(), []Formation{Vanguard, Left, Right, Centre, Rear})
	want("紮營順序", DeployOrder(), []Formation{Centre, Vanguard, Left, Right, Rear})

	sw := func(name string, got, exp []Side) {
		for i := range exp {
			if got[i] != exp[i] {
				t.Errorf("%s 第 %d 項是 %s，應該是 %s", name, i+1, got[i], exp[i])
			}
		}
	}
	sw("軍力作戰順序", SideActionOrder(),
		[]Side{MainDefender, AidDefender, MainAttacker, AidAttacker})
	sw("佈置全軍順序", SideDeployOrder(),
		[]Side{MainAttacker, AidAttacker, AidDefender, MainDefender})

	if !MainAttacker.Attacking() || !AidAttacker.Attacking() {
		t.Error("主攻軍與助攻軍都是攻方")
	}
	if MainDefender.Attacking() || AidDefender.Attacking() {
		t.Error("主守軍與助守軍不是攻方")
	}
}

// TestOriginalIndex 釘住與原版部隊陣列的換算。
//
// 軍力照行動順序（主守、助守、主攻、助攻），隊伍照紮營順序
// （中軍、先鋒、左軍、右軍、後軍）——旗幟的組名表與旗面上的字
// 都是這個順序（`internal/assets/battlefield.go`）。
func TestOriginalIndex(t *testing.T) {
	for s, want := range map[Side]int{
		MainDefender: 0, AidDefender: 1, MainAttacker: 2, AidAttacker: 3,
	} {
		if got := s.OriginalIndex(); got != want {
			t.Errorf("%s 的軍力編號是 %d，想要 %d", s, got, want)
		}
	}
	for f, want := range map[Formation]int{
		Centre: 0, Vanguard: 1, Left: 2, Right: 3, Rear: 4,
	} {
		if got := f.OriginalIndex(); got != want {
			t.Errorf("%s 的隊伍編號是 %d，想要 %d", f, got, want)
		}
	}
	// 兩個編號合起來就是原版部隊記錄的索引 `軍力×10 + 隊伍`，
	// 四個軍力各佔十格不重疊。
	seen := map[int]bool{}
	for _, s := range SideActionOrder() {
		for _, f := range ActionOrder() {
			k := s.OriginalIndex()*10 + f.OriginalIndex()
			if seen[k] {
				t.Errorf("%s%s 的槽號 %d 撞號", s, f, k)
			}
			seen[k] = true
		}
	}
	if len(seen) != 20 {
		t.Errorf("只排出 %d 個槽號", len(seen))
	}
}

// TestWeatherOriginalIndex 釘住天氣的原版編號：晴 0、雨 1、風 2。
func TestWeatherOriginalIndex(t *testing.T) {
	for w, want := range map[Weather]int{Clear: 0, Rainy: 1, Windy: 2} {
		if got := w.OriginalIndex(); got != want {
			t.Errorf("%s 的原版編號是 %d，想要 %d", w, got, want)
		}
	}
}
