package battle

import "testing"

// 六方向、距離、地形適性——戰術層的座標系統。
//
// 這些看起來像「不會錯的東西」，但六方向的軸座標**錯了不會當掉**，
// 只會讓部隊走到奇怪的地方，而戰場上沒有人看得出來。

// TestDirsAreThreeOppositePairs 釘住六方向兩兩相反。
func TestDirsAreThreeOppositePairs(t *testing.T) {
	pairs := [][2]Dir{
		{DirUp, DirDown},
		{DirUpRight, DirDownLeft},
		{DirDownRight, DirUpLeft},
	}
	for _, p := range pairs {
		h := Hex{3, 4}
		if got := h.Step(p[0]).Step(p[1]); got != h {
			t.Errorf("%v 走 %d 再走 %d 到 %v，應該回到原點", h, p[0], p[1], got)
		}
	}
	if len(Dirs()) != 6 {
		t.Fatalf("方向有 %d 個，應該是 6", len(Dirs()))
	}
	seen := map[Hex]bool{}
	for _, d := range Dirs() {
		h := (Hex{}).Step(d)
		if seen[h] {
			t.Errorf("方向 %d 與另一個方向走到同一格 %v", d, h)
		}
		seen[h] = true
		if Distance(Hex{}, h) != 1 {
			t.Errorf("走一步到 %v，距離卻是 %d", h, Distance(Hex{}, h))
		}
	}
}

// TestDistanceGrowsAlongLine 釘住同方向連走 n 步距離就是 n。
func TestDistanceGrowsAlongLine(t *testing.T) {
	for _, d := range Dirs() {
		h := Hex{}
		for n := 1; n <= 5; n++ {
			h = h.Step(d)
			if got := Distance(Hex{}, h); got != n {
				t.Errorf("方向 %d 走 %d 步，距離算成 %d", d, n, got)
			}
		}
	}
}

// TestOffsetRoundTrip 釘住軸座標與矩形座標互換得回來。
func TestOffsetRoundTrip(t *testing.T) {
	f := &Field{W: FieldW, H: FieldH, cell: make([]Terrain, FieldW*FieldH)}
	for y := 0; y < FieldH; y++ {
		for x := 0; x < FieldW; x++ {
			gx, gy := f.offset(FromOffset(x, y))
			if gx != x || gy != y {
				t.Fatalf("(%d,%d) 換過去再換回來變成 (%d,%d)", x, y, gx, gy)
			}
		}
	}
}

// TestOutOfBoundsIsMountain 釘住界外當成不可通行。
//
// 邊界不另外判斷，靠的就是這個約定；改掉它會讓部隊走出場外。
func TestOutOfBoundsIsMountain(t *testing.T) {
	f := flat(Plain)
	for _, h := range []Hex{{-99, -99}, {999, 0}, FromOffset(-1, 3), FromOffset(FieldW, 3)} {
		if got := f.At(h); got != Mountain {
			t.Errorf("界外的 %v 是 %s，應該是大山", h, got)
		}
		if f.InBounds(h) {
			t.Errorf("%v 不該算在場內", h)
		}
	}
	if Mountain.Passable() {
		t.Error("大山應該無法穿越（說明書 p.29）")
	}
}

// TestTroopSuits 釘住兵種對地形的適性（說明書 p.18）。
func TestTroopSuits(t *testing.T) {
	cases := []struct {
		k    TroopKind
		suit []Terrain
	}{
		{TroopLand, []Terrain{Plain, Desert, Forest, City, Fort}},
		{TroopMountain, []Terrain{Hill, Mountain}},
		{TroopWaterOnly, []Terrain{Shallow, Deep}},
		{TroopMtnLand, []Terrain{Hill, Mountain, Plain, Desert, Forest, City, Fort}},
		{TroopWaterLand, []Terrain{Shallow, Deep, Plain, Desert, Forest, City, Fort}},
		{TroopMtnWater, []Terrain{Hill, Mountain, Shallow, Deep}},
	}
	for _, c := range cases {
		want := map[Terrain]bool{}
		for _, t2 := range c.suit {
			want[t2] = true
		}
		for tr := Terrain(0); tr < terrainCount; tr++ {
			if got := c.k.Suits(tr); got != want[tr] {
				t.Errorf("%s 軍對 %s 的適性是 %v，應該是 %v", c.k, tr, got, want[tr])
			}
		}
	}
	// 「強力軍是萬能兵種」。
	for tr := Terrain(0); tr < terrainCount; tr++ {
		if !TroopMighty.Suits(tr) {
			t.Errorf("強力軍應該適應 %s", tr)
		}
	}
}

// TestMoveCostIsTerrainOnly 釘住每一格的花費**只看地形**（`L0`）。
//
// 原版取完 `DS:0x7c42` 就直接跟部隊剩下的移動力比（`0x27e71`），
// 中間沒有按兵種的調整——所以水軍走深水一樣花 6。
// 這裡順便把整張表釘住：它與說明書 p.29 逐格相同。
func TestMoveCostIsTerrainOnly(t *testing.T) {
	for k := TroopLand; k <= TroopMighty; k++ {
		for _, tr := range []Terrain{Plain, Hill, Shallow, Deep, City} {
			if got, want := MoveCost(tr, k), moveCost[tr]; got != want {
				t.Errorf("%s軍走%s花 %d，應該跟地形一樣是 %d", k, tr, got, want)
			}
		}
	}
	for _, c := range []struct {
		t    Terrain
		want int
	}{
		{Plain, 2}, {Desert, 2}, {Hill, 3}, {Forest, 3}, {City, 3},
		{Fort, 3}, {Shallow, 4}, {Deep, 6}, {Mountain, 999},
	} {
		if got := moveCost[c.t]; got != c.want {
			t.Errorf("%s 的移動力消耗是 %d，原版是 %d", c.t, got, c.want)
		}
	}
}

// TestTerrainTablesMatchTheOriginal 釘住量到的兩張地形攻防表
// （`DS:0x85c2`／`DS:0x85e2`，`L0`），以及它們與說明書 p.31–32
// 的方向一致。
//
// **兩件事要各驗一次**：數字要是原版那一組，方向要對得起手冊。
// 只驗數字的話，抄錯一格不會被抓到；只驗方向的話，數字換成別的
// 也照樣綠。
func TestTerrainTablesMatchTheOriginal(t *testing.T) {
	for _, c := range []struct {
		t        Terrain
		atk, def int
	}{
		{Mountain, 0, 0}, {Hill, 20, 22}, {Shallow, 8, 9}, {Deep, 6, 8},
		{City, 27, 40}, {Fort, 25, 30}, {Plain, 20, 17}, {Forest, 16, 22},
		{Desert, 15, 16},
	} {
		if a, d := TerrainAttack(c.t), TerrainDefence(c.t); a != c.atk || d != c.def {
			t.Errorf("%s 的攻守值是 %d／%d，原版是 %d／%d", c.t, a, d, c.atk, c.def)
		}
	}
	// 方向照手冊 p.31–32。
	if TerrainAttack(Hill) < TerrainAttack(Plain) ||
		TerrainDefence(Hill) <= TerrainDefence(Plain) {
		t.Error("山丘應該強化攻擊力和防禦力")
	}
	if TerrainDefence(Forest) <= TerrainDefence(Plain) {
		t.Error("樹林應該提供不錯的防禦掩護")
	}
	if TerrainAttack(Forest) >= TerrainAttack(Plain) {
		t.Error("樹林只說防禦掩護，攻擊不該比平原好")
	}
	for _, tr := range []Terrain{Shallow, Deep} {
		if TerrainAttack(tr) >= TerrainAttack(Plain) ||
			TerrainDefence(tr) >= TerrainDefence(Plain) {
			t.Errorf("%s 應該不利攻擊與防禦", tr)
		}
	}
	if TerrainDefence(Deep) >= TerrainDefence(Shallow) {
		t.Error("深水應該比淺水更不利")
	}
	if TerrainAttack(City) <= TerrainAttack(Fort) ||
		TerrainDefence(City) <= TerrainDefence(Fort) {
		t.Error("城池是一流防禦工事，關寨是簡陋的——城池要高於關寨")
	}
	if TerrainDefence(City) <= TerrainDefence(Hill) {
		t.Error("城池的防禦應該高過山丘")
	}
	if TerrainAttack(Fort) <= TerrainAttack(Plain) {
		t.Error("關寨應該有少許攻擊優勢")
	}
}

// TestTroopTerrainTable 釘住兵種對地形的加成（`DS:0x8602`，`L0`），
// 以及它與說明書 p.18 那句話一致。
func TestTroopTerrainTable(t *testing.T) {
	for _, c := range []struct {
		k    TroopKind
		t    Terrain
		want int
	}{
		{TroopLand, Plain, 5}, {TroopLand, Forest, 5}, {TroopLand, Hill, 0},
		{TroopMountain, Hill, 10}, {TroopMountain, Plain, 0},
		{TroopWaterOnly, Shallow, 10}, {TroopWaterOnly, Deep, 10},
		{TroopMtnLand, Hill, 10}, {TroopMtnLand, Plain, 5},
		{TroopWaterLand, Deep, 10}, {TroopWaterLand, Plain, 5},
		{TroopMtnWater, Hill, 10}, {TroopMtnWater, Shallow, 10},
		{TroopMighty, Deep, 15}, {TroopMighty, City, 8}, {TroopMighty, Desert, 5},
	} {
		if got := TroopTerrainBonus(c.k, c.t); got != c.want {
			t.Errorf("%s軍在%s的加成是 %d，原版是 %d", c.k, c.t, got, c.want)
		}
	}
	// 「強力軍是萬能兵種……而且也更能發揮地形特性」（p.18）：
	// **每一種走得進去的地形都有加成，而且不低於專精那一種**。
	for _, tr := range []Terrain{Hill, Shallow, Deep, City, Fort, Plain, Forest, Desert} {
		if TroopTerrainBonus(TroopMighty, tr) <= 0 {
			t.Errorf("強力軍在%s沒有加成", tr)
		}
	}
	if TroopTerrainBonus(TroopMighty, Deep) <= TroopTerrainBonus(TroopWaterOnly, Deep) {
		t.Error("強力軍在深水應該比水軍還強")
	}
	// 大山誰都走不進去，所以誰都沒有加成。
	for k := TroopLand; k <= TroopMighty; k++ {
		if TroopTerrainBonus(k, Mountain) != 0 {
			t.Errorf("%s軍在大山有加成", k)
		}
	}
}

// TestLeaderPower 釘住一位將領的戰力值公式（`0x2e01a`／`0x2e13a`，`L0`）：
//
//	(戰力 × 7 + 3 × 武裝度) × (兵種適性 + 地形值) ÷ 1000
func TestLeaderPower(t *testing.T) {
	// 陸軍、平原、攻方：(80×7 + 3×60) × (5 + 20) / 1000 ＝ 740×25/1000 ＝ 18
	if got := LeaderPower(80, 60, TroopLand, Plain, true); got != 18 {
		t.Errorf("陸軍在平原進攻的戰力值是 %d，公式算出來是 18", got)
	}
	// 同一位在城池守：(740) × (0 + 40) / 1000 ＝ 29
	if got := LeaderPower(80, 60, TroopLand, City, false); got != 29 {
		t.Errorf("陸軍在城池防守的戰力值是 %d，公式算出來是 29", got)
	}
	// 兵種適性真的有進去：山軍在山丘 (0+20) → (10+20)。
	lo := LeaderPower(80, 60, TroopLand, Hill, true)
	hi := LeaderPower(80, 60, TroopMountain, Hill, true)
	if hi <= lo {
		t.Errorf("山軍在山丘 %d 應該高過陸軍的 %d", hi, lo)
	}
	// 大山的地形值是 0，所以誰站上去戰力值都是 0。
	if got := LeaderPower(100, 100, TroopMighty, Mountain, true); got != 0 {
		t.Errorf("大山上的戰力值是 %d，應該是 0", got)
	}
}

// TestMovePointsFormula 釘住移動力（`0x2e07a`，`L0`）：
//
//	min(15, (訓練度 − 武裝度 + 100) ÷ 10 + 1)
func TestMovePointsFormula(t *testing.T) {
	for _, c := range []struct {
		train, arms, want int
	}{
		{50, 50, 11}, {100, 0, 15}, {0, 100, 1}, {100, 100, 11}, {80, 20, 15},
		{60, 50, 12},
	} {
		u := &Unit{Leaders: []Leader{{
			Training: uint8(c.train), Arms: uint8(c.arms), Soldiers: 1000,
		}}}
		if got := u.MovePoints(); got != c.want {
			t.Errorf("訓練 %d 武裝 %d 的移動力是 %d，公式算出來是 %d",
				c.train, c.arms, got, c.want)
		}
	}
}
