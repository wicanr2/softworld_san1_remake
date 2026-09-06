package game

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// loadScenario 開原版劇本；沒素材就 skip。**本儲存庫不含原版檔案。**
func loadScenario(t *testing.T, slot state.Slot) *state.Scenario {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過需要原版素材的測試")
	}
	base := filepath.Join(root, "三國演義", "DATA2")
	rd := func(ext string) []byte {
		b, err := os.ReadFile(base + ext)
		if err != nil {
			t.Fatalf("讀 %s%s：%v", base, ext, err)
		}
		return b
	}
	c, err := assets.OpenContainer(rd(".NAM"), rd(".IDX"), rd(".GRP"))
	if err != nil {
		t.Fatalf("OpenContainer：%v", err)
	}
	sc, err := state.LoadScenario(c, slot)
	if err != nil {
		t.Fatalf("LoadScenario：%v", err)
	}
	return sc
}

func TestNewFromScenario1(t *testing.T) {
	sc := loadScenario(t, state.Scenario1)
	g, err := New(sc, 0, 5) // 勢力 0 ＝ 劉備
	if err != nil {
		t.Fatal(err)
	}
	if g.Date != (Date{Year: 189, Month: 1}) {
		t.Errorf("起始年月是 %v，應該是 189 年 1 月（中平六年元月）", g.Date)
	}
	if n := len(g.Factions()); n != 14 {
		t.Errorf("勢力有 %d 個，應該是 14 個", n)
	}
	lord := g.Lord(0)
	if lord == nil || lord.Name != "劉備" {
		t.Fatalf("勢力 0 的君主是 %v，應該是劉備", lord)
	}
	if got := g.Territory(0); len(got) != 1 || got[0] != 8 {
		t.Errorf("劉備的領地是 %v，應該只有郡 8（齊郡）", got)
	}
	p := g.Prefecture(8)
	if p == nil || p.Name != "齊郡" {
		t.Fatalf("郡 8 是 %v，應該是齊郡", p)
	}
	// 人口與兵士要是**實際值**，不是原版存的 ÷100。
	if p.Population != 80000 {
		t.Errorf("齊郡人口 %d，應該是 80000（原版存 800）", p.Population)
	}
	if gov := g.Governor(8); gov == nil || gov.Name != "劉備" {
		t.Errorf("齊郡的主事者是 %v，應該是劉備", gov)
	}
}

// TestPlayerFactionMustExist 釘住「玩家勢力不在的時候要報錯」。
//
// **默默改成 0 的話，玩家會在控制別人而畫面上完全看不出來。**
func TestPlayerFactionMustExist(t *testing.T) {
	sc := loadScenario(t, state.Scenario1)
	if _, err := New(sc, 15, 5); err == nil {
		t.Error("勢力 15 沒在用，開局卻沒有報錯")
	}
	if _, err := New(sc, 0, 0); err == nil {
		t.Error("難度 0 越界，開局卻沒有報錯")
	}
	if _, err := New(sc, 0, 11); err == nil {
		t.Error("難度 11 越界，開局卻沒有報錯")
	}
}

// TestTroopCapMatchesData 釘住帶兵上限。
//
// 說明書 p.18 給的表與原版資料兩邊獨立，而且對得上：346 位人物零人超標，
// 九個職位裡八個的實際最大兵數正好等於上限。**兩個獨立來源同意同一組
// 數字**，這比任何一邊自己說了算都強。
func TestTroopCapMatchesData(t *testing.T) {
	sc := loadScenario(t, state.Scenario1)
	g, err := New(sc, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	hit := map[state.Rank]int{}
	for i := range g.generals {
		x := &g.generals[i]
		if x.Name == "" {
			continue
		}
		cap := x.TroopCap()
		if cap == 0 {
			t.Errorf("%s 的職位 %d 沒有上限", x.Name, x.Rank)
			continue
		}
		if x.Soldiers > cap {
			t.Errorf("%s（%d）帶 %d 兵，超過上限 %d", x.Name, x.Rank, x.Soldiers, cap)
		}
		if x.Soldiers > hit[x.Rank] {
			hit[x.Rank] = x.Soldiers
		}
	}
	// 八個職位要頂到上限；謀士那一格劇本 001 裡最高只有 200。
	atCap := 0
	for r, best := range hit {
		if best == TroopCap(r) {
			atCap++
		}
	}
	if atCap != 8 {
		t.Errorf("頂到上限的職位有 %d 個，應該是 8 個——上限表大概不對", atCap)
	}
}

// TestDateAdvance 釘住月份進位與季節。
func TestDateAdvance(t *testing.T) {
	d := Date{Year: 189, Month: 12}
	if n := d.Next(); n != (Date{Year: 190, Month: 1}) {
		t.Errorf("189 年 12 月的下個月是 %v，應該是 190 年 1 月", n)
	}
	for _, c := range []struct {
		m int
		s Season
	}{{1, Spring}, {2, Spring}, {3, Spring}, {4, Summer}, {6, Summer},
		{7, Autumn}, {9, Autumn}, {10, Winter}, {12, Winter}} {
		if got := (Date{Year: 189, Month: c.m}).Season(); got != c.s {
			t.Errorf("%d 月的季節是 %v，應該是 %v", c.m, got, c.s)
		}
	}
}

// TestGovernorEverywhere 釘住「每個有主的郡都有主事者」，六個劇本都要成立。
func TestGovernorEverywhere(t *testing.T) {
	for _, slot := range []state.Slot{state.Scenario1, state.Scenario2, state.Scenario3,
		state.Scenario4, state.Scenario5, state.Scenario6} {
		sc := loadScenario(t, slot)
		f := sc.ActiveFactions()[0]
		g, err := New(sc, state.FactionID(f), 5)
		if err != nil {
			t.Fatalf("%s：%v", slot, err)
		}
		for i := range g.prefectures {
			p := &g.prefectures[i]
			if !p.Owned() {
				continue
			}
			gov := g.Governor(p.ID)
			if gov == nil {
				t.Errorf("%s：郡 %d %s 有主卻沒有主事者", slot, p.ID, p.Name)
				continue
			}
			if gov.Faction != p.Owner {
				t.Errorf("%s：郡 %d %s 的主事者屬於別的勢力", slot, p.ID, p.Name)
			}
		}
	}
}

// TestSealHasExactlyOneHolder 釘住玉璽在開局時只有一個人拿著。
//
// **這是 `BASEMAS` offset 14 是玉璽的判準**：十六個諸侯槽裡只有一個
// 是 1，其餘全 0。六個劇本都要成立——只驗一個的話，那一格是別的東西
// 而剛好長得像的機率不低。
func TestSealHasExactlyOneHolder(t *testing.T) {
	for _, slot := range []state.Slot{
		state.Scenario1, state.Scenario2, state.Scenario3,
		state.Scenario4, state.Scenario5, state.Scenario6,
	} {
		sc := loadScenario(t, slot)
		holders, total := 0, 0
		for f := 0; f < 16; f++ {
			n := sc.TreasuryOf(f)[TreasureSeal]
			if n > 0 {
				holders++
			}
			total += n
		}
		if holders != 1 || total != 1 {
			t.Errorf("劇本 %s：玉璽有 %d 個持有者、共 %d 個，應該是一個人拿一個",
				slot, holders, total)
		}
	}
}

// TestGovernorFieldMatchesStatus 釘住兩條獨立的路徑對得上。
//
// 郡的太守有兩個來源：`BASESTA` offset 32 直接存人物槽號（`L2`），
// 以及掃人物表的身分欄算出來（`Governor()`，`docs/spec/003`）。
// **兩邊不一致就表示其中一條讀錯了**——而單獨看任何一條都不會露餡。
func TestGovernorFieldMatchesStatus(t *testing.T) {
	for _, slot := range []state.Slot{
		state.Scenario1, state.Scenario2, state.Scenario3,
		state.Scenario4, state.Scenario5, state.Scenario6,
	} {
		sc := loadScenario(t, slot)
		g, err := New(sc, state.FactionID(firstPlayable(sc)), 5)
		if err != nil {
			t.Fatalf("劇本 %s 開不了局：%v", slot, err)
		}
		bad := 0
		for _, p := range g.Prefectures() {
			if !p.Owned() {
				continue
			}
			want := sc.GovernorIndex(p.ID)
			got := g.Governor(p.ID)
			if want == state.NoValue16 {
				continue
			}
			if got == nil || got.Index != want {
				bad++
				if bad <= 3 {
					name := "（無）"
					if got != nil {
						name = got.Name
					}
					t.Errorf("劇本 %s 郡 %d：欄位說太守是槽 %d，掃身分算出來是 %s",
						slot, p.ID, want, name)
				}
			}
		}
		if bad > 3 {
			t.Errorf("劇本 %s：另有 %d 個郡不一致", slot, bad-3)
		}
	}
}

// firstPlayable 找一個能當玩家的勢力。
func firstPlayable(sc *state.Scenario) int {
	if ps := sc.Players(); len(ps) > 0 {
		return ps[0]
	}
	return sc.ActiveFactions()[0]
}
