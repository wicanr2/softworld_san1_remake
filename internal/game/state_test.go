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
	g, err := New(sc, 0, 5, state.EditionBase) // 勢力 0 ＝ 劉備
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
	if _, err := New(sc, 15, 5, state.EditionBase); err == nil {
		t.Error("勢力 15 沒在用，開局卻沒有報錯")
	}
	if _, err := New(sc, 0, 0, state.EditionBase); err == nil {
		t.Error("難度 0 越界，開局卻沒有報錯")
	}
	if _, err := New(sc, 0, 11, state.EditionBase); err == nil {
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
	g, err := New(sc, 0, 5, state.EditionBase)
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
		g, err := New(sc, state.FactionID(f), 5, state.EditionBase)
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

// TestGovernorFieldIsConsistent 釘住主事者那一格（`BASESTA` offset 32）
// 在六個劇本裡都自洽：那個人屬於郡的所屬勢力、就在這個郡、
// 身分是君主、軍師或太守——**軍師也算**，劇本 004 的周瑜與劇本 006 的
// 司馬懿就是這樣，那正是「只認身分 0 或 2」當初漏掉的兩例。
//
// **判準不是「與掃身分算出來的一致」**——掃身分那條路推過兩個版本、
// 兩個都被資料推翻（`docs/spec/003` §5），現在 `Governor()` 讀的就是
// 這一格，拿它去比自己不會發現任何事。
func TestGovernorFieldIsConsistent(t *testing.T) {
	for _, slot := range []state.Slot{
		state.Scenario1, state.Scenario2, state.Scenario3,
		state.Scenario4, state.Scenario5, state.Scenario6,
	} {
		sc := loadScenario(t, slot)
		g, err := New(sc, state.FactionID(firstPlayable(sc)), 5, state.EditionBase)
		if err != nil {
			t.Fatalf("劇本 %s 開不了局：%v", slot, err)
		}
		bad := 0
		for _, p := range g.Prefectures() {
			if !p.Owned() {
				continue
			}
			want := sc.GovernorIndex(p.ID)
			if want == state.NoValue16 {
				continue
			}
			x := g.General(want)
			switch {
			case x == nil:
				bad++
				t.Errorf("劇本 %s 郡 %d：主事者槽號 %d 越界", slot, p.ID, want)
			case x.Faction != p.Owner || x.Location != p.ID:
				bad++
				if bad <= 3 {
					t.Errorf("劇本 %s 郡 %d：主事者 %s 的勢力 %d／領地 %d 與郡的所屬 %d 對不上",
						slot, p.ID, x.Name, x.Faction, x.Location, p.Owner)
				}
			case x.Status != state.StatusLord && x.Status != state.StatusChief &&
				!x.Status.Governs():
				bad++
				if bad <= 3 {
					t.Errorf("劇本 %s 郡 %d：主事者 %s 的身分是 %d",
						slot, p.ID, x.Name, x.Status)
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

// TestOwnersAreDerivedFromTheGeneralTable 釘住「郡的歸屬是導出值」。
//
// **兩條路對同一份資料要逐格相同**：劇本檔存的所屬勢力，與從人物表
// 重算出來的，開局就該一致。不一致表示其中一條讀錯了——而兩者都是
// 合法的值，看不出差別（`CLAUDE.md` §7 第 18 條）。
func TestOwnersAreDerivedFromTheGeneralTable(t *testing.T) {
	g := newGame(t)
	was := make([]state.FactionID, 0, 42)
	for _, p := range g.Prefectures() {
		was = append(was, p.Owner)
	}
	g.RecomputeOwners()
	for i, p := range g.Prefectures() {
		if p.Owner != was[i] {
			t.Errorf("郡 %d（%s）：劇本存的是勢力 %d，從人物表算出來是 %d",
				p.ID, p.Name, was[i], p.Owner)
		}
	}
}

// TestRecomputeGivesThePrefectureToTheLastSlot 釘住「槽號較大的那位說了算」。
//
// 這不是實作細節：原版電腦諸侯的出兵**不打仗**，只把部隊搬進目標郡，
// 郡易主完全靠這次重算（`docs/mechanics/70-ai` §2.13.6）。
func TestRecomputeGivesThePrefectureToTheLastSlot(t *testing.T) {
	g := newGame(t)
	at := 0
	for _, p := range g.Prefectures() {
		if p.Owned() {
			at = p.ID
			break
		}
	}
	if at == 0 {
		t.Fatal("找不到有主的郡")
	}
	// 把兩位不同勢力的人放進同一個郡，槽號大的在後面。
	lo, hi := g.General(100), g.General(300)
	if lo == nil || hi == nil {
		t.Fatal("找不到人物 100／300")
	}
	lo.Faction, lo.Status, lo.Location = 1, state.StatusOfficer, at
	hi.Faction, hi.Status, hi.Location = 2, state.StatusOfficer, at
	g.RecomputeOwners()
	if got := g.Prefecture(at).Owner; got != 2 {
		t.Errorf("同郡兩方：槽號 300 是勢力 2，郡卻算成勢力 %d", got)
	}
	// 反過來換勢力，結論跟著換——確認靠的是槽號不是勢力編號。
	lo.Faction, hi.Faction = 2, 1
	g.RecomputeOwners()
	if got := g.Prefecture(at).Owner; got != 1 {
		t.Errorf("勢力對調之後郡算成 %d，應該跟著槽號大的那位變成 1", got)
	}
}

// TestDifficultyBoundsFollowEdition 釘住難度上限跟著版本走
// （`docs/spec/004`：原版 `請設定難度(1-10)`、加強版 `(1-20)`，`L0`）。
//
// **拿加強版的難度 15 去開原版不是「比較難」，是規則接錯了**——
// 原版的係數表只有十格，第十五格是表外的位元組。所以要擋在開局，
// 不是等到電腦諸侯出兵時讀到垃圾。
func TestDifficultyBoundsFollowEdition(t *testing.T) {
	sc := loadScenario(t, state.Scenario1)
	f := state.FactionID(sc.ActiveFactions()[0])
	for _, c := range []struct {
		ed   state.Edition
		diff int
		ok   bool
	}{
		{state.EditionBase, 1, true}, {state.EditionBase, 10, true},
		{state.EditionBase, 11, false}, {state.EditionBase, 20, false},
		{state.EditionPlus, 10, true}, {state.EditionPlus, 20, true},
		{state.EditionPlus, 21, false},
		{state.EditionBase, 0, false}, {state.EditionPlus, 0, false},
		// 空字串當原版：舊存檔沒有這個欄位。
		{"", 10, true}, {"", 11, false},
	} {
		g, err := New(sc, f, c.diff, c.ed)
		if c.ok && err != nil {
			t.Errorf("%q 難度 %d 應該收：%v", c.ed, c.diff, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%q 難度 %d 應該擋下來，卻開起來了", c.ed, c.diff)
		}
		if c.ok && g != nil && g.Edition == "" {
			t.Errorf("%q 難度 %d 開起來了，但局面的版本是空的", c.ed, c.diff)
		}
	}
	if _, err := New(sc, f, 5, "enhanced"); err == nil {
		t.Error("不認識的版本應該擋下來")
	}
}

// TestPrefectureKeepsItsProvince 釘住州別欄有被帶進來。
//
// **漏一個欄位不會報錯**：`Province` 的零值是 0，而 0 是合法的州（幽州），
// 所以每一個郡都會顯示成幽州而不是空白——畫面上看起來只是「州名不對」，
// 不像資料沒載進來。齊郡在原版是青州（2）。
func TestPrefectureKeepsItsProvince(t *testing.T) {
	g := newGame(t)
	for _, tc := range []struct{ id, want int }{
		{1, 0},  // 遼東 幽州
		{7, 2},  // 北海 青州
		{8, 2},  // 齊郡 青州
		{41, 13}, // 南海 交州
	} {
		p := g.Prefecture(tc.id)
		if p == nil {
			t.Fatalf("沒有郡 %d", tc.id)
		}
		if int(p.Province) != tc.want {
			t.Errorf("郡 %d %s 的州別是 %d（%s），應該是 %d（%s）",
				tc.id, p.Name, p.Province, state.ProvinceName(int(p.Province)),
				tc.want, state.ProvinceName(tc.want))
		}
	}
}
