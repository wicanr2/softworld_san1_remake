package battle

import "testing"

// TestArrowsMatchesManualExample 釘住手冊 p.32 自己算給我們看的那一題。
//
// 「部隊各單武裝度平均值 ÷ 20 後取整數」，
// 例：`(75 + 80 + 50 + 100) ÷ 4 = 76.25`，`76.25 ÷ 20 = 3.8125`，射三次。
//
// ⚠ 算例用的是**各單的算術平均**，不是兵數加權。這個測試就是防止
// 有人「順手改成加權平均」——那在多數情況下看起來更合理，但與手冊不同。
func TestArrowsMatchesManualExample(t *testing.T) {
	// 兵數刻意偏在武裝度最低的那一位身上：加權平均會落到 50 附近，
	// 只射得出兩次；算術平均是手冊的 76，射三次。兩條路的答案不同，
	// 這一題才問得出我們走的是哪一條。
	u := &Unit{Leaders: []Leader{
		{Name: "甲", Arms: 75, Soldiers: 10},
		{Name: "乙", Arms: 80, Soldiers: 10},
		{Name: "丙", Arms: 50, Soldiers: 10000},
		{Name: "丁", Arms: 100, Soldiers: 10},
	}}
	if got := u.Arrows(); got != 3 {
		t.Errorf("弓箭次數 %d，手冊的算例是 3 次", got)
	}
	if u.AvgArms()/20 == 3 {
		t.Fatal("這組數字沒有拉開加權與算術平均的差距，測不到想測的東西")
	}
}

// TestArrowsIgnoresLostLeaders 釘住死掉與被擒的人不算進平均。
func TestArrowsIgnoresLostLeaders(t *testing.T) {
	u := &Unit{Leaders: []Leader{
		{Arms: 100, Soldiers: 100},
		{Arms: 100, Soldiers: 100},
		{Arms: 0, Soldiers: 100, Dead: true},
		{Arms: 0, Soldiers: 100, Captured: true},
	}}
	if got := u.Arrows(); got != 5 {
		t.Errorf("弓箭次數 %d，兩位陣亡被擒的不算，應該是 100/20 = 5", got)
	}
	if u.Arrows() == 0 {
		t.Error("整隊都沒了才該射不出箭")
	}
	empty := &Unit{}
	if empty.Arrows() != 0 {
		t.Error("空隊伍應該射不出箭")
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

// TestMovePointsHasFloor 釘住移動力有下限。
//
// 訓練度 0、武裝度滿的部隊算出來會是負的；**一支動不了的部隊
// 在戰場上看起來像卡住的 bug**，所以要有下限。
func TestMovePointsHasFloor(t *testing.T) {
	u := &Unit{Leaders: []Leader{{Soldiers: 100, Training: 0, Arms: 255}}}
	if got := u.MovePoints(); got < TuneMoveMin {
		t.Errorf("移動力 %d 低於下限 %d", got, TuneMoveMin)
	}
	// 訓練度高的走得比較遠（說明書 p.30：「移動力來源是訓練度…」）。
	slow := &Unit{Leaders: []Leader{{Soldiers: 100, Training: 0, Arms: 0}}}
	fast := &Unit{Leaders: []Leader{{Soldiers: 100, Training: 100, Arms: 0}}}
	if fast.MovePoints() <= slow.MovePoints() {
		t.Errorf("訓練度 100 的移動力 %d 沒有多於訓練度 0 的 %d",
			fast.MovePoints(), slow.MovePoints())
	}
	// 「全副武裝將稍減移動力」（p.30）。
	light := &Unit{Leaders: []Leader{{Soldiers: 100, Training: 100, Arms: 0}}}
	heavy := &Unit{Leaders: []Leader{{Soldiers: 100, Training: 100, Arms: 100}}}
	if heavy.MovePoints() >= light.MovePoints() {
		t.Errorf("全副武裝的移動力 %d 沒有少於輕裝的 %d",
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
