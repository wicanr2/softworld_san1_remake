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

// TestMoveCostSuitability 釘住「兵種能否適應地形」會影響移動花費
// （說明書 p.30）。
func TestMoveCostSuitability(t *testing.T) {
	if got := MoveCost(Hill, TroopLand); got != 3 {
		t.Errorf("陸軍走山丘花 %d，應該是 3", got)
	}
	if got := MoveCost(Hill, TroopMountain); got != 2 {
		t.Errorf("山軍走山丘花 %d，應該少一點（3-1）", got)
	}
	if got := MoveCost(Deep, TroopWaterOnly); got != 5 {
		t.Errorf("水軍走深水花 %d，應該是 5（6-1）", got)
	}
	// 適應也不會減到零：平原本來就是最便宜的 2。
	if got := MoveCost(Plain, TroopLand); got != 1 {
		t.Errorf("陸軍走平原花 %d，應該是 1（2-1）", got)
	}
	// 原版的表（`DS:0x7c42`）與說明書 p.29 逐格相同，這裡整張釘住。
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
	// 大山誰都過不去。
	for k := TroopLand; k <= TroopMighty; k++ {
		if MoveCost(Mountain, k) < 99 {
			t.Errorf("%s 軍居然走得進大山", k)
		}
	}
}

// TestTerrainModsFollowManual 釘住地形效應的**方向**（說明書 p.31–32）。
//
// 幅度是 remake 選的（`docs/design/02`），但誰高誰低是手冊寫死的：
// 城池發揮最大戰力與一流防禦、山丘強化攻防、樹林是不錯的防禦掩護、
// 水域攻防都不便、平原沙漠無特殊效果。
func TestTerrainModsFollowManual(t *testing.T) {
	for _, tr := range []Terrain{Plain, Desert} {
		if AttackMod(tr) != 0 || DefenceMod(tr) != 0 {
			t.Errorf("%s 應該無特殊效果", tr)
		}
	}
	if AttackMod(Hill) <= 0 || DefenceMod(Hill) <= 0 {
		t.Error("山丘應該強化攻擊力和防禦力")
	}
	if DefenceMod(Forest) <= 0 {
		t.Error("樹林應該提供不錯的防禦掩護")
	}
	if AttackMod(Forest) != 0 {
		t.Error("樹林手冊只說防禦掩護，攻擊不該有加成")
	}
	for _, tr := range []Terrain{Shallow, Deep} {
		if AttackMod(tr) >= 0 || DefenceMod(tr) >= 0 {
			t.Errorf("%s 應該不利攻擊與防禦", tr)
		}
	}
	if DefenceMod(Deep) >= DefenceMod(Shallow) {
		t.Error("深水應該比淺水更不利")
	}
	if AttackMod(City) <= AttackMod(Fort) || DefenceMod(City) <= DefenceMod(Fort) {
		t.Error("城池是一流防禦工事，關寨是簡陋的——城池要高於關寨")
	}
	if DefenceMod(City) <= DefenceMod(Hill) {
		t.Error("城池的防禦應該高過山丘")
	}
	if AttackMod(Fort) <= 0 {
		t.Error("關寨應該有少許攻擊優勢")
	}
}
