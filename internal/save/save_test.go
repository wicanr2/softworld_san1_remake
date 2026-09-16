package save_test

import (
	"bytes"
	"encoding/binary"
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

// loadScenario 讀劇本 001；沒有原版素材就 skip。**本儲存庫不含原版檔案。**
func loadScenario(t *testing.T) *state.Scenario {
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
	return sc
}

// newGame 用原版開一局。
func newGame(t *testing.T) *game.State {
	t.Helper()
	g, err := game.New(loadScenario(t), 0, 5, state.EditionBase)
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
	// **改一格戰場地圖**（州郡 offset 55–174）。關寨蓋在哪一格是地圖上的
	// 事，郡表的「關寨數」對得上不代表地圖存下來了——兩者分家的時候
	// 畫面完全正常，只有真的打起來才看得出那一格沒有關寨。
	fortCell := -1
	for i, b := range p.BattleField {
		if b != 0xFF && b>>4 == 15 && b&0x0F == 7 { // 沒有標記的平原
			p.BattleField[i] = b&0xF6 | 6 // 原版的寫法（`0x1afae`）
			fortCell = i
			break
		}
	}
	if fortCell < 0 {
		t.Fatal("郡 15 的戰場地圖上找不到一格沒有標記的平原")
	}
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
	if h.Edition != g.Edition {
		t.Errorf("版本 %q，存的是 %q", h.Edition, g.Edition)
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
		// **主事者要跟著存檔走**（原版州郡 offset 32）。漏掉的話讀回來
		// 得重推，而君主與太守同郡時推出來的常常是另一個人——存檔前後
		// 的局面就從那一刻起分家，而畫面上看不出任何異狀。
		if a.Autonomy != b.Autonomy {
			t.Errorf("郡 %d 的自治型態存的是 %s，讀回來是 %s", id, a.Autonomy, b.Autonomy)
		}
		// **戰場地圖要逐格比**（offset 55–174）。`Tables()` 一度把郡表的
		// 欄位一個個寫回去卻漏掉這一段，於是建築關寨改的那一格在編碼時
		// 消失：關寨數與金都對得上，只有地圖那一格還是舊值。
		if len(a.BattleField) != len(b.BattleField) {
			t.Fatalf("郡 %d 的戰場地圖存的是 %d 格、讀回來 %d 格",
				id, len(a.BattleField), len(b.BattleField))
		}
		for i := range a.BattleField {
			if a.BattleField[i] != b.BattleField[i] {
				t.Fatalf("郡 %d 的戰場地圖第 %d 格存的是 %#02x、讀回來 %#02x",
					id, i, a.BattleField[i], b.BattleField[i])
			}
		}
		x, y := g.Governor(id), h.Governor(id)
		switch {
		case (x == nil) != (y == nil):
			t.Fatalf("郡 %d 的主事者一邊有一邊沒有", id)
		case x != nil && x.Index != y.Index:
			t.Fatalf("郡 %d 的主事者存的是 %s，讀回來是 %s", id, x.Name, y.Name)
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
	// **兵士是存值不是導出值**（`game.State.troops`，`0x1949e`）：原版只在
	// 重整守將清單時刷新那一欄，所以它可以比駐軍加總舊。這支測試釘的是
	// 「存的是實際值 ÷ 100」，不是刷新時機——先全部重整一次，兩件事才
	// 不會混在同一個斷言裡。
	for id := 1; id <= state.PrefectureCount; id++ {
		g.RefreshGarrison(id)
	}
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
		if f.Lord < 0 {
			// 絕嗣的勢力君主欄是哨兵（`0x14ba3`），讀回來就該是沒有人。
			if lord != nil {
				t.Errorf("勢力 %d 已經絕嗣，讀回來卻有君主 %v", f.ID, lord)
			}
			continue
		}
		if lord == nil || lord.Index != f.Lord {
			t.Errorf("勢力 %d 的君主讀回來是 %v，應該是槽號 %d", f.ID, lord, f.Lord)
		}
	}
}

// TestUntouchedSaveMatchesTheOriginal 釘住「什麼都沒做就存檔」寫出來的
// 三張表與原版的劇本**逐位元組相同**——只差原版開新局自己也會寫的
// 三件事（`TestZZNewGameBoardBase` 對拍過，`docs/spec/015` §5）：玩家那
// 格的操縱方 ← 1；填充槽（君主槽 ≥ 346）的操縱方／君主／軍師 ← `0xFFFF`、
// 那筆填充君主 ← 已故（身分 12、勢力 `0xFF`、領地 `0xFF`）；AI 等級依
// 難度改寫（難度 5 在原版不動）。
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
		g, err := game.New(sc, state.FactionID(sc.ActiveFactions()[0]), 5, state.EditionBase)
		if err != nil {
			t.Fatal(err)
		}
		wantMas, wantSta, wantGen := sc.Tables()
		wantMas, wantGen = append([]byte(nil), wantMas...), append([]byte(nil), wantGen...)
		binary.LittleEndian.PutUint16(wantMas[int(g.Player)*72:], 1)
		for f := 0; f < 16; f++ {
			lord := int(int16(binary.LittleEndian.Uint16(wantMas[f*72+2:]))) // 原版是有號比較
			if lord < 346 {
				continue
			}
			for _, off := range []int{0, 2, 6} {
				binary.LittleEndian.PutUint16(wantMas[f*72+off:], 0xFFFF)
			}
			wantGen[lord*30+17], wantGen[lord*30+18], wantGen[lord*30+19] = 12, 0xFF, 0xFF
		}
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

// TestOptionsSurviveRoundTrip 釘住「其他」底下的開關存得回來。
//
// 玩家關掉音樂之後每次讀檔又響起來，與沒存過設定是同一件事。
func TestOptionsSurviveRoundTrip(t *testing.T) {
	g := newGame(t)
	g.Options.ToggleMusic()
	g.Options.ToggleVoice()
	g.Options.ToggleCalendar()
	if err := g.Options.SetDelay(0); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := save.Write(root, 1, g, ""); err != nil {
		t.Fatal(err)
	}
	h, err := save.Read(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !h.Options.MusicOff || !h.Options.VoiceOff {
		t.Error("音樂與語音的開關沒有存回來")
	}
	if h.Options.SoundOff {
		t.Error("沒動過的音效開關被改掉了")
	}
	if h.Options.Calendar != game.Western {
		t.Error("年號的設定沒有存回來")
	}
	if h.Options.Delay() != 0 {
		t.Errorf("延時讀回來是 %d，存的是 0（等待按鍵）", h.Options.Delay())
	}
}

// TestPlusSaveRoundTrip 釘住加強版的存檔——**難度 15 只有加強版收得下**。
//
// 版本沒存進去的話，這種存檔讀回來會被難度檢查擋掉（原版上限 10），
// 而錯誤訊息會說「存檔的難度是 15」，指向存檔壞了，方向完全相反。
func TestPlusSaveRoundTrip(t *testing.T) {
	plus, err := game.New(loadScenario(t), 0, 15, state.EditionPlus)
	if err != nil {
		t.Fatalf("加強版難度 15 應該開得起來：%v", err)
	}
	root := t.TempDir()
	if err := save.Write(root, 1, plus, "加強版"); err != nil {
		t.Fatal(err)
	}
	h, err := save.Read(root, 1)
	if err != nil {
		t.Fatalf("加強版存檔讀不回來：%v", err)
	}
	if h.Edition != state.EditionPlus || h.Difficulty != 15 {
		t.Errorf("讀回來是 %q／難度 %d，存的是 plus／15", h.Edition, h.Difficulty)
	}
}

// TestProgressFileMatchesOriginalLayout 釘住新增的三個檔案：長度照原版、
// 內容解得回來、而且**年月與選項以 `BASEPRO` 為準**。
func TestProgressFileMatchesOriginalLayout(t *testing.T) {
	g := newGame(t)
	play(t, g, 3)
	g.Options.MusicOff = true
	g.Options.Calendar = game.Western
	if err := g.Options.SetDelay(37); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := save.Write(root, 2, g, "測試進度"); err != nil {
		t.Fatal(err)
	}

	sizes := map[string]int{
		"SV2/BASEPRO.SV2": state.ProgressSize,
		"SV2/BASEPRE.SV2": state.GlyphTableSize,
		"SAVENAME.SVP":    state.SaveNameTableSize,
	}
	for name, want := range sizes {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("讀 %s：%v", name, err)
		}
		if len(b) != want {
			t.Errorf("%s 長 %d，原版是 %d", name, len(b), want)
		}
	}

	b, err := os.ReadFile(filepath.Join(root, "SV2", "BASEPRO.SV2"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := state.DecodeProgress(b)
	if err != nil {
		t.Fatal(err)
	}
	if p.Year != g.Date.Year || p.Month != g.Date.Month {
		t.Errorf("BASEPRO 存的是 %d 年 %d 月，局面是 %d 年 %d 月",
			p.Year, p.Month, g.Date.Year, g.Date.Month)
	}
	if p.Difficulty != g.Difficulty {
		t.Errorf("BASEPRO 存的難度是 %d，局面是 %d", p.Difficulty, g.Difficulty)
	}
	if !p.MusicOff || p.Calendar != int(game.Western) || p.Delay != 37 {
		t.Errorf("BASEPRO 的選項是 音樂關=%v 曆=%d 延時=%d",
			p.MusicOff, p.Calendar, p.Delay)
	}

	back, err := save.Read(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	if back.Date != g.Date || back.Difficulty != g.Difficulty {
		t.Errorf("讀回來是 %v 難度 %d，存的是 %v 難度 %d",
			back.Date, back.Difficulty, g.Date, g.Difficulty)
	}
	if !back.Options.MusicOff || back.Options.Delay() != 37 {
		t.Errorf("讀回來的選項是 音樂關=%v 延時=%d",
			back.Options.MusicOff, back.Options.Delay())
	}
}

// TestSaveNamesShareOneFile 釘住名稱表是**六個槽共用一個檔**：
// 寫第二個槽不可以把第一個槽的名字洗掉。
func TestSaveNamesShareOneFile(t *testing.T) {
	g := newGame(t)
	root := t.TempDir()
	if err := save.Write(root, 1, g, "第一個"); err != nil {
		t.Fatal(err)
	}
	if err := save.Write(root, 3, g, "第三個"); err != nil {
		t.Fatal(err)
	}
	list := save.List(root)
	if list[0].Name != "第一個" {
		t.Errorf("第 1 槽的名稱是 %q", list[0].Name)
	}
	if list[2].Name != "第三個" {
		t.Errorf("第 3 槽的名稱是 %q", list[2].Name)
	}
	b, err := os.ReadFile(filepath.Join(root, "SAVENAME.SVP"))
	if err != nil {
		t.Fatal(err)
	}
	names, err := state.DecodeSaveNames(b)
	if err != nil {
		t.Fatal(err)
	}
	if names[0] != "第一個" || names[2] != "第三個" || names[1] != "" {
		t.Errorf("名稱表是 %q", names)
	}
}

// TestCommandedSurvivesInProgress 釘住「這個月下過令沒」搬進 `BASEPRO`
// 之後還原得回來。索引差一格的話整批會位移一個郡。
func TestCommandedSurvivesInProgress(t *testing.T) {
	g := newGame(t)
	var marked []int
	for _, id := range g.Territory(g.Player) {
		if len(marked) == 2 {
			break
		}
		g.Prefecture(id).Commanded = true
		marked = append(marked, id)
	}
	if len(marked) == 0 {
		t.Skip("玩家一個郡都沒有")
	}
	root := t.TempDir()
	if err := save.Write(root, 1, g, "x"); err != nil {
		t.Fatal(err)
	}
	back, err := save.Read(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range marked {
		if !back.Prefecture(id).Commanded {
			t.Errorf("郡 %d 的「已下令」沒帶過去", id)
		}
	}
	for _, id := range g.Territory(g.Player) {
		want := false
		for _, m := range marked {
			if m == id {
				want = true
			}
		}
		if got := back.Prefecture(id).Commanded; got != want {
			t.Errorf("郡 %d 的「已下令」是 %v，應該是 %v", id, got, want)
		}
	}
}

// 存檔要寫這一局自己帶的字模，讀回來還在。
//
// **不接的話自創君主的名字存讀一輪就變空白**——人物表裡只有造字碼位
// （`docs/spec/013` R4）。
func TestGlyphsSurviveASaveLoadRound(t *testing.T) {
	dir := t.TempDir()
	g := newGame(t)
	var want state.Glyphs
	for i := range want {
		for j := range want[i] {
			want[i][j] = byte(i*7 + j)
		}
	}
	g.SetGlyphs(&want)
	if err := save.Write(dir, 1, g, "測試"); err != nil {
		t.Fatal(err)
	}
	back, err := save.Read(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	got := back.Glyphs()
	if got == nil {
		t.Fatal("讀回來沒有字模")
	}
	if *got != want {
		t.Error("讀回來的字模與存進去的不同")
	}

	// 這一局沒帶字模時，**不要把上一次寫出去的洗掉**。
	g2 := newGame(t)
	if err := save.Write(dir, 1, g2, "測試"); err != nil {
		t.Fatal(err)
	}
	back2, err := save.Read(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if back2.Glyphs() == nil || *back2.Glyphs() != want {
		t.Error("沒帶字模的那一局把上一次的洗掉了")
	}
}

// TestAISettingsSurviveRoundTrip 釘住「其他」底下 remake 加的那兩項也存得住。
//
// **玩家在遊戲中換了 AI 又讀檔，回到旗標挑的那一個，看起來就是
// 「設定沒存到」**——而畫面上唯一的差別只是電腦諸侯下不同的命令，
// 玩家分不出是設定掉了還是 AI 本來就這樣。
func TestAISettingsSurviveRoundTrip(t *testing.T) {
	g := newGame(t)
	g.Options.SetAIMode("base")
	if err := g.Options.SetAIOrders(4); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := save.Write(root, 1, g, ""); err != nil {
		t.Fatal(err)
	}
	h, err := save.Read(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if h.Options.AIMode != "base" {
		t.Errorf("AI 版本讀回來是 %q，存的是 base", h.Options.AIMode)
	}
	if h.Options.AIOrders() != 4 {
		t.Errorf("電腦指令數讀回來是 %d，存的是 4", h.Options.AIOrders())
	}
}

// TestOldSavesGetTheDefaults 釘住舊存檔（沒有那兩個鍵）讀回來是預設值。
//
// `omitempty` 讓沒動過的局面也不寫這兩個鍵，所以「0 道令」與「舊存檔」
// 在 JSON 裡長得一模一樣。**0 要讀成預設不是讀成 0 道令**——一個
// 一道令都不下的電腦，在畫面上就只是「這些諸侯都不動」。
func TestOldSavesGetTheDefaults(t *testing.T) {
	g := newGame(t)
	root := t.TempDir()
	if err := save.Write(root, 1, g, ""); err != nil {
		t.Fatal(err)
	}
	blob, err := os.ReadFile(filepath.Join(root, "SV1", "REMAKE.JSON"))
	if err != nil {
		t.Fatal(err)
	}
	// 沒動過就不該寫進去——寫進去的話這個測試也沒有在測舊存檔。
	if bytes.Contains(blob, []byte("ai_mode")) {
		t.Error("沒動過 AI 版本卻寫了 ai_mode")
	}
	h, err := save.Read(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if h.Options.AIMode != "" {
		t.Errorf("AI 版本讀回來是 %q，應該是空字串", h.Options.AIMode)
	}
	if h.Options.AIOrders() != game.AIOrdersDefault {
		t.Errorf("電腦指令數讀回來是 %d，應該是預設 %d",
			h.Options.AIOrders(), game.AIOrdersDefault)
	}
}

// TestDescribeHasNoSlotNumber 釘住存檔的描述不帶槽號。
//
// 清單的編號由呼叫端照位置編（開局選單、「其他 → 儲存」的挑選清單）。
// 描述自己再帶一次就疊成「`1. 1. 新君主…`」，讀檔清單跳過空槽時還會
// 變成「`1. 3. …`」——兩個號碼對不上，玩家不知道該按哪一個。
func TestDescribeHasNoSlotNumber(t *testing.T) {
	for _, i := range []save.Info{
		{Slot: 3, Exists: true, Name: "新君主", Year: 189, Month: 3},
		{Slot: 5},
	} {
		d := i.Describe()
		if d == "" {
			t.Errorf("槽 %d 的描述是空的", i.Slot)
		}
		if len(d) > 0 && d[0] >= '0' && d[0] <= '9' {
			t.Errorf("槽 %d 的描述 %q 以數字開頭——槽號由呼叫端編", i.Slot, d)
		}
	}
}
