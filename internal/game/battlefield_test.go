package game

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestEveryPrefectureHasTheOriginalField 拿劇本裡真的資料驗戰場地圖
// （`L0`、`[base]`）。
//
// **判準是 round-trip 逐位元組相同**，不是「看起來像地圖」。解錯一個
// 欄位不會報錯，只會讓多數格子碰巧對（`CLAUDE.md` §7 第 18 條）。
// 除此之外每個郡都要有：一座城池、四個軍團起點、以及每個鄰郡一個出口。
func TestEveryPrefectureHasTheOriginalField(t *testing.T) {
	g := newGame(t)
	for _, p := range g.Prefectures() {
		if len(p.BattleField) != battle.FieldBytes {
			t.Fatalf("郡 %d（%s）的戰場資料是 %d 個位元組",
				p.ID, p.Name, len(p.BattleField))
		}
		f, err := battle.Load(p.BattleField, p.Neighbours)
		if err != nil {
			t.Fatalf("郡 %d（%s）的戰場解不出來：%v", p.ID, p.Name, err)
		}
		back := f.Bytes()
		for i := range back {
			if back[i] != p.BattleField[i] {
				t.Fatalf("郡 %d（%s）第 %d 格寫回去是 %#02x，原本是 %#02x",
					p.ID, p.Name, i, back[i], p.BattleField[i])
			}
		}
		if f.CityAt == battle.NoHex {
			t.Errorf("郡 %d（%s）沒有城池", p.ID, p.Name)
		}
		if f.At(f.CityAt) != battle.City {
			t.Errorf("郡 %d（%s）城池那一格的地形是 %s", p.ID, p.Name, f.At(f.CityAt))
		}
		for i, h := range f.Starts {
			if h == battle.NoHex {
				t.Errorf("郡 %d（%s）沒有第 %d 軍的起點", p.ID, p.Name, i+1)
			}
		}
		for _, n := range p.Neighbours {
			if _, ok := f.Gates[n]; !ok {
				t.Errorf("郡 %d（%s）沒有通往鄰郡 %d 的出口", p.ID, p.Name, n)
			}
		}
	}
}

// TestFieldComesFromTheScenario 釘住 State.Field 拿的是劇本的地圖
// 而不是生成器的。
//
// **兩者都會回一張看起來正常的圖**，所以判準是與劇本的位元組相同。
func TestFieldComesFromTheScenario(t *testing.T) {
	sc := loadScenario(t, state.Scenario1)
	g, err := New(sc, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range g.Prefectures() {
		f := g.Field(p.ID)
		if f == nil {
			t.Fatalf("郡 %d 沒有戰場", p.ID)
		}
		back := f.Bytes()
		for i := range back {
			if back[i] != p.BattleField[i] {
				t.Fatalf("郡 %d（%s）的戰場不是劇本那一張：第 %d 格 %#02x ≠ %#02x",
					p.ID, p.Name, i, back[i], p.BattleField[i])
			}
		}
	}
}

// TestFieldSizesAreTheTwoOriginalShapes 記錄原版只有兩種版面
// （`L0`，劇本 001 的 42 個郡）：12 欄 × 7 列與 8 欄 × 10 列。
//
// 兩種都塞得進 640×350：`x ＝ 48欄 + 56`、`y ＝ 32列 + 36`（奇數欄再 +16）。
func TestFieldSizesAreTheTwoOriginalShapes(t *testing.T) {
	g := newGame(t)
	shapes := map[[2]int]int{}
	for _, p := range g.Prefectures() {
		f, err := battle.Load(p.BattleField, p.Neighbours)
		if err != nil {
			t.Fatal(err)
		}
		w, h := 0, 0
		for y := 0; y < battle.FieldH; y++ {
			for x := 0; x < battle.FieldW; x++ {
				if f.Outside(battle.FromOffset(x, y)) {
					continue
				}
				if x+1 > w {
					w = x + 1
				}
				if y+1 > h {
					h = y + 1
				}
			}
		}
		shapes[[2]int{w, h}]++
		if right := 48*(w-1) + 56; right > 640 {
			t.Errorf("郡 %d（%s）的圖寬到 %d 像素，畫面只有 640", p.ID, p.Name, right)
		}
		if bottom := 32*(h-1) + 36 + 16; bottom > 350 {
			t.Errorf("郡 %d（%s）的圖高到 %d 像素，畫面只有 350", p.ID, p.Name, bottom)
		}
	}
	want := map[[2]int]int{{12, 7}: 21, {8, 10}: 21}
	if len(shapes) != len(want) {
		t.Fatalf("量到 %d 種版面：%v", len(shapes), shapes)
	}
	for k, n := range want {
		if shapes[k] != n {
			t.Errorf("%d 欄 × %d 列有 %d 個郡，應該是 %d", k[0], k[1], shapes[k], n)
		}
	}
}
