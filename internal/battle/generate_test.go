package battle

import "testing"

// 地形生成器的性質。
//
// ⚠ 生成出來的版面是 **remake 自己畫的**，不是原版的郡地理誌
//（見 `docs/design/03-battle.md`）。所以這裡問的不是
// 「跟原版一不一樣」，而是「同一個郡永遠一樣」「打得起來」。

func params(id int) Params {
	return Params{
		Prefecture: id,
		Neighbours: []int{(id % 42) + 1, ((id + 7) % 42) + 1, ((id + 19) % 42) + 1},
		Forts:      id % 6,
		LandValue:  uint8(30 + id%50),
		FloodRate:  uint8(id % 100),
	}
}

// TestGenerateIsDeterministic 釘住同一個郡永遠得到同一張圖。
//
// 這是整個戰術層可重現的前提：對拍、存讀檔、事後重播都靠它。
func TestGenerateIsDeterministic(t *testing.T) {
	for id := 1; id <= 42; id++ {
		a, b := Generate(params(id)), Generate(params(id))
		if a.String() != b.String() {
			t.Fatalf("第 %d 郡生成兩次不一樣", id)
		}
		if len(a.Gates) != len(b.Gates) {
			t.Fatalf("第 %d 郡的通道數不一樣", id)
		}
		for n := range a.Gates {
			if a.Gate(n) != b.Gate(n) {
				t.Fatalf("第 %d 郡通往 %d 郡的通道位置不一樣", id, n)
			}
		}
		if a.CityAt != b.CityAt {
			t.Fatalf("第 %d 郡的城池位置不一樣", id)
		}
	}
}

// TestGenerateDiffersByPrefecture 釘住不同的郡不是同一張圖。
//
// 一個把種子接錯的生成器會讓四十二個郡長得一模一樣，
// 而那在畫面上只看得出「戰場好像有點眼熟」。
func TestGenerateDiffersByPrefecture(t *testing.T) {
	seen := map[string]int{}
	for id := 1; id <= 42; id++ {
		s := Generate(params(id)).String()
		if prev, dup := seen[s]; dup {
			t.Errorf("第 %d 郡與第 %d 郡的戰場一模一樣", id, prev)
		}
		seen[s] = id
	}
}

// TestGenerateGates 釘住每個鄰郡有一個入口，而且入口走得進去。
//
// 手冊 p.19：「圖中的數字位置代表前往鄰近州郡的通道，
// 也是鄰郡攻入時的發兵地點」。
func TestGenerateGates(t *testing.T) {
	for id := 1; id <= 42; id++ {
		p := params(id)
		f := Generate(p)
		if len(f.Gates) != len(p.Neighbours) {
			t.Errorf("第 %d 郡有 %d 個鄰郡卻開了 %d 個通道",
				id, len(p.Neighbours), len(f.Gates))
		}
		for n := range f.Gates {
			h := f.Gate(n)
			if !f.InBounds(h) {
				t.Errorf("第 %d 郡通往 %d 郡的通道 %v 在場外", id, n, h)
			}
			if !f.At(h).Passable() {
				t.Errorf("第 %d 郡通往 %d 郡的通道踩在%s上", id, n, f.At(h))
			}
		}
	}
}

// TestGenerateCityAndForts 釘住城池一定在場上，城寨不超過五座（說明書 p.21）。
func TestGenerateCityAndForts(t *testing.T) {
	for id := 1; id <= 42; id++ {
		f := Generate(params(id))
		if f.At(f.CityAt) != City {
			t.Errorf("第 %d 郡的 CityAt 踩在%s上", id, f.At(f.CityAt))
		}
		cities, forts := 0, 0
		f.Cells(func(_ Hex, tr Terrain) {
			switch tr {
			case City:
				cities++
			case Fort:
				forts++
			}
		})
		if cities != 1 {
			t.Errorf("第 %d 郡有 %d 座城池，應該只有一座", id, cities)
		}
		if forts > 5 {
			t.Errorf("第 %d 郡有 %d 座關寨，上限是五座", id, forts)
		}
	}
}

// TestGenerateFortsCapped 釘住就算傳進來的城寨數超標也不會蓋出第六座。
func TestGenerateFortsCapped(t *testing.T) {
	f := Generate(Params{Prefecture: 7, Neighbours: []int{1}, Forts: 99, LandValue: 50})
	n := 0
	f.Cells(func(_ Hex, tr Terrain) {
		if tr == Fort {
			n++
		}
	})
	if n > 5 {
		t.Errorf("蓋了 %d 座關寨，上限是五座", n)
	}
}

// TestGenerateCityReachableFromEveryGate 釘住從每個入口都走得到城池。
//
// **這一條是戰場能不能玩的底線。** 隨機山脈有機會把城池封死，
// 而那在畫面上只看得出「電腦的軍隊在原地繞圈」——攻方永遠打不下郡，
// 一局就這樣卡住。
func TestGenerateCityReachableFromEveryGate(t *testing.T) {
	for id := 1; id <= 42; id++ {
		f := Generate(params(id))
		for n := range f.Gates {
			gate := f.Gate(n)
			if !reachable(f, gate, f.CityAt) {
				t.Errorf("第 %d 郡：從通往 %d 郡的入口 %v 走不到城池\n%s",
					id, n, gate, f)
			}
		}
	}
}

// reachable 是走得到嗎（只走得過去的格子）。
func reachable(f *Field, from, to Hex) bool {
	seen := map[Hex]bool{from: true}
	queue := []Hex{from}
	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		if h == to {
			return true
		}
		for _, d := range Dirs() {
			nxt := h.Step(d)
			if seen[nxt] || !f.InBounds(nxt) || !f.At(nxt).Passable() {
				continue
			}
			seen[nxt] = true
			queue = append(queue, nxt)
		}
	}
	return false
}

// TestGenerateTerrainIsValid 釘住每一格都是合法地形。
func TestGenerateTerrainIsValid(t *testing.T) {
	f := Generate(params(11))
	f.Cells(func(h Hex, tr Terrain) {
		if tr >= terrainCount {
			t.Fatalf("%v 是地形 %d，超出範圍", h, tr)
		}
		if tr.String() == "?" {
			t.Fatalf("%v 的地形印不出名字", h)
		}
	})
}

// TestFloodRateMakesWater 釘住洪水率高的郡水域比較多。
//
// 手冊沒說戰場地貌怎麼來，這是 remake 的詮釋（`docs/design/03`）；
// 釘住它是為了讓「郡的屬性有影響」這件事不會在重構時悄悄消失。
func TestFloodRateMakesWater(t *testing.T) {
	count := func(rate uint8) int {
		n := 0
		for id := 1; id <= 42; id++ {
			p := params(id)
			p.FloodRate = rate
			Generate(p).Cells(func(_ Hex, tr Terrain) {
				if tr.Water() {
					n++
				}
			})
		}
		return n
	}
	dry, wet := count(0), count(100)
	if dry != 0 {
		t.Errorf("洪水率 0 的郡有 %d 格水域，應該沒有", dry)
	}
	if wet <= dry {
		t.Errorf("洪水率 100 有 %d 格水域，洪水率 0 有 %d 格——應該多得多", wet, dry)
	}
}
