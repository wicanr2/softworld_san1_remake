package save_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// newGame 開一局；沒有原版素材就 skip。**本儲存庫不含原版檔案。**
func newGame(t *testing.T) *game.State {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過（本儲存庫不含原版檔案）")
	}
	dir := filepath.Join(root, "三國演義")
	read := func(ext string) []byte {
		b, err := os.ReadFile(filepath.Join(dir, "DATA2."+ext))
		if err != nil {
			t.Skipf("讀不到 DATA2.%s：%v", ext, err)
		}
		return b
	}
	c, err := assets.OpenContainer(read("NAM"), read("IDX"), read("GRP"))
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(c, state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	g, err := game.New(sc, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// play 讓局面往前跑幾個月，這樣存的就不是開局狀態。
func play(t *testing.T, g *game.State, months int) {
	t.Helper()
	brain, err := ai.New(ai.ModeEnhanced)
	if err != nil {
		t.Fatal(err)
	}
	s := session.New(g, brain, g.Player)
	for i := 0; i < months; i++ {
		s.EndMonth()
	}
}

// TestRoundTrip 釘住存了再讀回來，局面一模一樣。
//
// **存讀檔壞掉的方式幾乎都是安靜的**：少存一個欄位，讀回來的局面
// 看起來完全正常，只是某個郡的關寨不見了、某個人這個月又能被賞一次。
// 所以這裡逐欄比。
func TestRoundTrip(t *testing.T) {
	g := newGame(t)
	play(t, g, 30)
	// 動幾個三張表放不下的東西，確認它們真的有被存到。
	p := g.Prefecture(15)
	p.Forts = 3
	p.Autonomy = game.AutoMilitary
	p.Commanded = true
	g.General(0).Rewarded = true
	if f := g.Faction(g.Player); f != nil {
		f.Treasury[1] = 2
	}

	root := t.TempDir()
	if err := save.Write(root, 2, g, "測試存檔"); err != nil {
		t.Fatal(err)
	}
	h, err := save.Read(root, 2)
	if err != nil {
		t.Fatal(err)
	}

	if h.Date != g.Date {
		t.Errorf("年月 %v，存的是 %v", h.Date, g.Date)
	}
	if h.Player != g.Player || h.Difficulty != g.Difficulty || h.Slot != g.Slot {
		t.Errorf("玩家/難度/劇本 %v/%d/%s，存的是 %v/%d/%s",
			h.Player, h.Difficulty, h.Slot, g.Player, g.Difficulty, g.Slot)
	}

	for id := 1; id <= state.PrefectureCount; id++ {
		a, b := g.Prefecture(id), h.Prefecture(id)
		if a.Name != b.Name || a.Owner != b.Owner || a.Population != b.Population ||
			a.Gold != b.Gold || a.Rice != b.Rice ||
			a.PublicLoyalty != b.PublicLoyalty || a.LandValue != b.LandValue ||
			a.FloodRate != b.FloodRate || a.PriceLevel != b.PriceLevel ||
			a.Forts != b.Forts || a.Autonomy != b.Autonomy || a.Commanded != b.Commanded {
			t.Fatalf("郡 %d 讀回來不一樣：\n存 %+v\n讀 %+v", id, *a, *b)
		}
		if len(a.Neighbours) != len(b.Neighbours) {
			t.Fatalf("郡 %d 的鄰郡數不一樣", id)
		}
	}

	for i := 0; i < 350; i++ {
		a, b := g.General(i), h.General(i)
		if a.Name != b.Name || a.Age != b.Age || a.Stamina != b.Stamina ||
			a.Intel != b.Intel || a.War != b.War || a.Charm != b.Charm ||
			a.Rank != b.Rank || a.Origin != b.Origin || a.Loyalty != b.Loyalty ||
			a.Status != b.Status || a.Faction != b.Faction || a.Location != b.Location ||
			a.Troop != b.Troop || a.Training != b.Training || a.Arms != b.Arms ||
			a.Rewarded != b.Rewarded {
			t.Fatalf("人物 %d（%s）讀回來不一樣：\n存 %+v\n讀 %+v", i, a.Name, *a, *b)
		}
	}

	for _, f := range g.Factions() {
		var got *game.Faction
		for i := range h.Factions() {
			if h.Factions()[i].ID == f.ID {
				got = &h.Factions()[i]
			}
		}
		if got == nil {
			t.Fatalf("勢力 %d 讀不回來", f.ID)
		}
		if got.Alive != f.Alive || got.Chief != f.Chief || got.Lord != f.Lord ||
			got.Treasury != f.Treasury {
			t.Errorf("勢力 %d 讀回來不一樣：存 %+v，讀 %+v", f.ID, f, *got)
		}
	}
}

// TestSoldiersSurviveRoundTrip 釘住兵力存得回來。
//
// ⚠ 原版的兵士欄存的是實際值 ÷ 100，所以**兵力只精確到百位**。
// 這不是 bug 是原版的格式；釘住它是為了讓「讀回來少了幾十個兵」
// 不會被當成新的錯誤去追。
func TestSoldiersSurviveRoundTrip(t *testing.T) {
	g := newGame(t)
	play(t, g, 12)
	root := t.TempDir()
	if err := save.Write(root, 1, g, ""); err != nil {
		t.Fatal(err)
	}
	h, err := save.Read(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 350; i++ {
		a, b := g.General(i), h.General(i)
		if a.Soldiers != b.Soldiers {
			t.Fatalf("人物 %d（%s）的兵力 %d → %d", i, a.Name, a.Soldiers, b.Soldiers)
		}
	}
	for id := 1; id <= state.PrefectureCount; id++ {
		if g.Soldiers(id) != h.Soldiers(id) {
			t.Fatalf("郡 %d 的兵力 %d → %d", id, g.Soldiers(id), h.Soldiers(id))
		}
	}
}

// TestUnknownBytesSurvive 釘住還沒解出來的欄位原封不動帶著走。
//
// 州郡表 176 個位元組解出 24 個、人物表 30 個解出 20 個。
// **把沒解出來的位元組寫成零，等於在替將來的人銷毀證據。**
func TestUnknownBytesSurvive(t *testing.T) {
	g := newGame(t)
	root := t.TempDir()
	if err := save.Write(root, 3, g, ""); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(filepath.Join(root, "SV3", "BASESTA.SV3"))
	if err != nil {
		t.Fatal(err)
	}
	orig, _, _, err := func() ([]byte, []byte, []byte, error) {
		_, sta, gen, err := g.Tables()
		return sta, gen, nil, err
	}()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, orig) {
		t.Fatal("寫出來的州郡表與 Tables() 給的不一樣")
	}
	// 郡表第 0 筆是原版的啞元，整筆要原樣保留。
	if bytes.Equal(saved[:176], make([]byte, 176)) {
		t.Error("郡表的啞元被寫成全零")
	}
	// 抽查幾個未解的位移不是零（開局的資料本來就有值）。
	nonZero := 0
	for id := 1; id <= state.PrefectureCount; id++ {
		rec := saved[id*176:]
		for _, off := range []int{84, 86, 100, 120} {
			if rec[off] != 0 {
				nonZero++
			}
		}
	}
	if nonZero == 0 {
		t.Error("四十二個郡的未解欄位全是零——存檔可能把它們洗掉了")
	}
}

// TestSlotsAreSix 釘住六個進度（說明書 p.25）。
func TestSlotsAreSix(t *testing.T) {
	if save.Slots != 6 {
		t.Errorf("存檔槽有 %d 個，手冊寫六個", save.Slots)
	}
	g := newGame(t)
	root := t.TempDir()
	for _, bad := range []int{0, 7, -1} {
		if err := save.Write(root, bad, g, ""); err == nil {
			t.Errorf("存到第 %d 格竟然可以", bad)
		}
		if _, err := save.Read(root, bad); err == nil {
			t.Errorf("讀第 %d 格竟然可以", bad)
		}
	}
}

// TestListShowsEmptySlots 釘住讀檔選單看得到哪幾格是空的。
func TestListShowsEmptySlots(t *testing.T) {
	g := newGame(t)
	root := t.TempDir()
	if err := save.Write(root, 4, g, "孤軍"); err != nil {
		t.Fatal(err)
	}
	list := save.List(root)
	if len(list) != save.Slots {
		t.Fatalf("列出 %d 格，應該是 %d 格", len(list), save.Slots)
	}
	for _, i := range list {
		if i.Slot == 4 {
			if !i.Exists || i.Name != "孤軍" || i.Year != g.Date.Year {
				t.Errorf("第 4 格的概況不對：%+v", i)
			}
			continue
		}
		if i.Exists {
			t.Errorf("第 %d 格是空的卻說有存檔", i.Slot)
		}
		if i.Describe() == "" {
			t.Errorf("第 %d 格沒有說明文字", i.Slot)
		}
	}
}

// TestReadRejectsWrongVersion 釘住格式版本不同要報錯，不要盡力而為。
func TestReadRejectsWrongVersion(t *testing.T) {
	g := newGame(t)
	root := t.TempDir()
	if err := save.Write(root, 5, g, ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "SV5", "REMAKE.JSON")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b = bytes.Replace(b, []byte(`"version": 1`), []byte(`"version": 99`), 1)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := save.Read(root, 5); err == nil {
		t.Error("讀到未來版本的存檔竟然沒報錯")
	}
	if save.List(root)[4].Exists {
		t.Error("讀不懂的存檔不該在選單裡顯示成可讀")
	}
}

// TestReadMissingSlot 釘住讀空槽會報錯。
func TestReadMissingSlot(t *testing.T) {
	if _, err := save.Read(t.TempDir(), 1); err == nil {
		t.Error("讀不存在的存檔竟然成功")
	}
}

// TestWriteIsAtomic 釘住覆寫失敗不會毀掉原本的存檔。
//
// 存檔寫到一半當掉，玩家失去的是**上一次的進度**——比沒存到嚴重得多。
func TestWriteIsAtomic(t *testing.T) {
	g := newGame(t)
	root := t.TempDir()
	if err := save.Write(root, 6, g, "第一次"); err != nil {
		t.Fatal(err)
	}
	// 留一個殘留的暫存目錄，模擬上一次寫到一半。
	if err := os.MkdirAll(filepath.Join(root, "SV6.tmp", "junk"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := save.Write(root, 6, g, "第二次"); err != nil {
		t.Fatalf("有殘留暫存目錄時存檔失敗：%v", err)
	}
	if got := save.List(root)[5].Name; got != "第二次" {
		t.Errorf("存檔名稱是 %q，應該是「第二次」", got)
	}
	if _, err := os.Stat(filepath.Join(root, "SV6.tmp")); !os.IsNotExist(err) {
		t.Error("暫存目錄沒有被收乾淨")
	}
}

// TestOriginalTableKeepsHundreds 釘住寫出去的州郡表照原版的版面：
// 人口與兵士存的是實際值 ÷ 100。
//
// remake 的精確人口另外放在 REMAKE.JSON（`game.PrefectureExtra.Population`）。
// **兩者的分工不能倒過來**：把精確值塞進那個 16 位元欄位，
// 檔案就不再是原版的版面，對拍與將來的比對都失去基礎；
// 三萬人以上的郡還會靜靜地捲回去。
func TestOriginalTableKeepsHundreds(t *testing.T) {
	g := newGame(t)
	play(t, g, 24)
	root := t.TempDir()
	if err := save.Write(root, 1, g, ""); err != nil {
		t.Fatal(err)
	}
	sta, err := os.ReadFile(filepath.Join(root, "SV1", "BASESTA.SV1"))
	if err != nil {
		t.Fatal(err)
	}
	for id := 1; id <= state.PrefectureCount; id++ {
		rec := sta[id*176:]
		got := int(rec[14]) | int(rec[15])<<8
		want := g.Prefecture(id).Population / 100
		if got != want {
			t.Fatalf("郡 %d 的人口欄是 %d，實際人口 %d ÷ 100 應該是 %d",
				id, got, g.Prefecture(id).Population, want)
		}
		gotSol := int(rec[16]) | int(rec[17])<<8
		if wantSol := g.Soldiers(id) / 100; gotSol != wantSol {
			t.Fatalf("郡 %d 的兵士欄是 %d，應該是 %d", id, gotSol, wantSol)
		}
	}
}

// TestTablesMatchOriginalSizes 釘住三張表的長度與原版相同。
func TestTablesMatchOriginalSizes(t *testing.T) {
	g := newGame(t)
	root := t.TempDir()
	if err := save.Write(root, 1, g, ""); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"BASEMAS.SV1": state.MasterTableSize,     // 16 × 72
		"BASESTA.SV1": state.PrefectureTableSize, // 43 × 176
		"BASEGEN.SV1": state.GeneralTableSize,    // 350 × 30
	}
	for name, size := range want {
		b, err := os.ReadFile(filepath.Join(root, "SV1", name))
		if err != nil {
			t.Fatal(err)
		}
		if len(b) != size {
			t.Errorf("%s 長度 %d，原版是 %d", name, len(b), size)
		}
	}
}

// TestLordSuccessionSurvives 釘住君主換人之後存得回來。
//
// 君主老死由麾下接位（`events.go`）；諸侯表的君主欄不寫回去的話，
// 讀檔會拿回開局那位——**而那個人已經不在了**。
func TestLordSuccessionSurvives(t *testing.T) {
	g := newGame(t)
	before := map[int]int{}
	for _, f := range g.Factions() {
		before[int(f.ID)] = f.Lord
	}
	play(t, g, 40*12)
	changed := false
	for _, f := range g.Factions() {
		if f.Lord != before[int(f.ID)] {
			changed = true
		}
	}
	if !changed {
		t.Skip("四十年之內沒有任何一家換過君主")
	}
	root := t.TempDir()
	if err := save.Write(root, 1, g, ""); err != nil {
		t.Fatal(err)
	}
	h, err := save.Read(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range g.Factions() {
		lord := h.Lord(f.ID)
		if lord == nil || lord.Index != f.Lord {
			t.Errorf("勢力 %d 的君主讀回來是 %v，應該是槽號 %d", f.ID, lord, f.Lord)
		}
	}
}

// TestUntouchedSaveMatchesTheOriginal 釘住「什麼都沒做就存檔」寫出來的
// 三張表與原版的劇本**逐位元組相同**。
//
// 這是整個存檔格式最強的一次檢查：它同時問了三件事——編碼與解碼對著
// 同一張版面表、衍生欄位（兵士、現役／在野武將數）算得回原版存的數字、
// 未解的位元組真的原封不動。任何一項錯了都會在這裡變成一個位移。
func TestUntouchedSaveMatchesTheOriginal(t *testing.T) {
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過（本儲存庫不含原版檔案）")
	}
	for _, slot := range []state.Slot{
		state.Scenario1, state.Scenario2, state.Scenario3,
		state.Scenario4, state.Scenario5, state.Scenario6,
	} {
		sc := loadSlot(t, slot)
		g, err := game.New(sc, state.FactionID(sc.ActiveFactions()[0]), 5)
		if err != nil {
			t.Fatal(err)
		}
		wantMas, wantSta, wantGen := sc.Tables()
		gotMas, gotSta, gotGen, err := g.Tables()
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct {
			name       string
			got, want  []byte
			recordSize int
		}{
			{"BASEMAS", gotMas, wantMas, 72},
			{"BASESTA", gotSta, wantSta, 176},
			{"BASEGEN", gotGen, wantGen, 30},
		} {
			if bytes.Equal(c.got, c.want) {
				continue
			}
			// 報出第一個不同的位移，而且說清楚是第幾筆的第幾個位元組——
			// 「兩份 7568 bytes 不一樣」沒有人查得下去。
			for i := range c.want {
				if c.got[i] != c.want[i] {
					t.Errorf("劇本 %s 的 %s：第 %d 筆的位移 %d 是 %#02x，原版是 %#02x",
						slot, c.name, i/c.recordSize, i%c.recordSize, c.got[i], c.want[i])
					break
				}
			}
		}
	}
}

// loadSlot 讀一個劇本槽。
func loadSlot(t *testing.T, slot state.Slot) *state.Scenario {
	t.Helper()
	dir := filepath.Join(os.Getenv("SAN1_ORIG"), "三國演義")
	read := func(ext string) []byte {
		b, err := os.ReadFile(filepath.Join(dir, "DATA2."+ext))
		if err != nil {
			t.Skipf("讀不到 DATA2.%s：%v", ext, err)
		}
		return b
	}
	c, err := assets.OpenContainer(read("NAM"), read("IDX"), read("GRP"))
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(c, slot)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}
