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
	g, err := New(sc, 0, 5, state.EditionBase)
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
// 兩種都塞得進 640×408：`x ＝ 48欄 + 56`、`y ＝ 32列 + 36`（奇數欄再 +16）。
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

// TestChiefMatchesTheMasterTable 釘住軍師的推導（`L0`）。
//
// remake 是從人物表的身分推的（身分 1 ＝ 軍師），原版另外把軍師的
// 人物槽號存在諸侯記錄 offset 6，而且拿它去查謀略、決定要不要勸諫
// （`0x175a7`）。**兩條路要得到同一個人**——推導錯了不會報錯，
// 只會讓計略與勸諫默默失效。
func TestChiefMatchesTheMasterTable(t *testing.T) {
	sc := loadScenario(t, state.Scenario1)
	g, err := New(sc, 0, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	mas, _, _ := sc.Tables()
	const masterSize = 72
	for _, f := range g.Factions() {
		i := int(f.ID) * masterSize
		if i+8 > len(mas) {
			t.Fatalf("諸侯表只有 %d 個位元組，讀不到槽 %d", len(mas), f.ID)
		}
		want := int(mas[i+6]) | int(mas[i+7])<<8
		if want == 0xFFFF {
			want = -1
		}
		if f.Chief != want {
			t.Errorf("勢力 %d 的軍師推成 %d，諸侯表寫的是 %d",
				f.ID, f.Chief, want)
		}
	}
}

// TestAdvisorWarns 釘住軍師勸諫的門檻（`0x18a90`，`L0`）：
// `RND(5) + 80 < 軍師的謀略`。
func TestAdvisorWarns(t *testing.T) {
	// 謀略 85 以上一定勸：RND(5) 最大 4，80+4 ＝ 84 < 85。
	for r := 0; r < AdvisorWarnSpread; r++ {
		if !AdvisorWarns(85, r) {
			t.Errorf("謀略 85、RND ＝ %d 沒有勸諫", r)
		}
	}
	// 謀略 80 以下一定不勸：80+0 ＝ 80 不小於 80。
	for r := 0; r < AdvisorWarnSpread; r++ {
		if AdvisorWarns(80, r) {
			t.Errorf("謀略 80、RND ＝ %d 卻勸諫了", r)
		}
	}
	// 中間是機率：謀略 83 只有 RND 0、1、2 會勸。
	n := 0
	for r := 0; r < AdvisorWarnSpread; r++ {
		if AdvisorWarns(83, r) {
			n++
		}
	}
	if n != 3 {
		t.Errorf("謀略 83 有 %d/5 會勸，應該是 3", n)
	}
}

// TestPrestigeMovesWithBattles 釘住戰役對人望的影響
// （`0x204b4`／`0x204e0`，`L0`）：勝方 +2、敗方 −2，夾在 0–100。
//
// **人望不是裝飾**：它每年決定部下忠誠的漲跌（`LoyaltyDrift`），
// 60 是分水嶺。所以連敗的諸侯會先掉人望，再一年一年掉忠誠。
func TestPrestigeMovesWithBattles(t *testing.T) {
	if PrestigeOnWin != 2 {
		t.Errorf("勝負的人望增減是 %d，原版是 2", PrestigeOnWin)
	}
	g := newGame(t)
	id := g.Factions()[0].ID
	f := g.Faction(id)

	f.Prestige = 50
	g.shiftPrestige(id, PrestigeOnWin)
	if f.Prestige != 52 {
		t.Errorf("打贏之後人望 %d，應該是 52", f.Prestige)
	}
	g.shiftPrestige(id, -PrestigeOnWin)
	if f.Prestige != 50 {
		t.Errorf("打輸之後人望 %d，應該回到 50", f.Prestige)
	}
	// 夾在 0–100。
	f.Prestige = 100
	g.shiftPrestige(id, PrestigeOnWin)
	if f.Prestige != 100 {
		t.Errorf("人望上限是 100，得到 %d", f.Prestige)
	}
	f.Prestige = 1
	g.shiftPrestige(id, -PrestigeOnWin)
	if f.Prestige != 0 {
		t.Errorf("人望下限是 0，得到 %d", f.Prestige)
	}
	// 無主的一方不算——空白郡沒有諸侯。
	g.shiftPrestige(state.NoFaction, PrestigeOnWin)
}
