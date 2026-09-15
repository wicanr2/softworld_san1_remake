package state

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// scenarioForTest 讀劇本 001；沒有素材就 skip。
func scenarioForTest(t *testing.T) *Scenario {
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
	sc, err := LoadScenario(c, Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

// 劇本 001 有兩個新君主欄——手冊 p.7 寫「16（含 2 個新君主欄）」。
func TestCustomLordSlotsMatchTheManual(t *testing.T) {
	sc := scenarioForTest(t)
	got := sc.CustomLordSlots()
	if len(got) != 2 {
		t.Fatalf("空的新君主欄有 %d 個 %v，手冊說 2 個", len(got), got)
	}
	// 那兩個槽的君主欄指著填充筆，而且一個郡都沒有。
	for _, f := range got {
		if n := len(sc.Territory(f)); n != 0 {
			t.Errorf("槽 %d 有 %d 個郡，那就不是空的新君主欄", f, n)
		}
	}
	if n := len(sc.ActiveFactions()); n != masterCount-len(got) {
		t.Errorf("在用的勢力有 %d 個，加上 %d 個空欄不等於 %d",
			n, len(got), masterCount)
	}
}

func TestCustomLordStatsStartFromTheTemplate(t *testing.T) {
	// 不加點就是範本的值（`0x3e44c`，`L0`）。
	st, in, mi, ch := CustomLord{}.Stats()
	if st != CustomLordStamina || in != CustomLordIntellect ||
		mi != CustomLordMight || ch != CustomLordCharm {
		t.Errorf("不加點 ＝ %d/%d/%d/%d，範本是 %d/%d/%d/%d",
			st, in, mi, ch, CustomLordStamina, CustomLordIntellect,
			CustomLordMight, CustomLordCharm)
	}
	// 加點會加上去，而且夾在上限。
	c := CustomLord{Stamina: 5, Intellect: 100, Might: 0, Charm: 0}
	st, in, _, _ = c.Stats()
	if st != CustomLordStamina+5 {
		t.Errorf("體能加 5 ＝ %d", st)
	}
	if in != CustomLordStatCap {
		t.Errorf("謀略加 100 ＝ %d，上限是 %d", in, CustomLordStatCap)
	}
	if c.Spent() != 105 {
		t.Errorf("用掉 %d 點", c.Spent())
	}
}

func TestCustomLordValidate(t *testing.T) {
	sc := scenarioForTest(t)
	name := [CustomLordNameChars]rune{'新', '君', '主'}
	// 劇本 001 的空白郡（`docs/mechanics/01`）：39 牂柯。
	ok := CustomLord{Name: name, Prefecture: 39, Stamina: 50, Charm: 50}
	if err := ok.Validate(sc); err != nil {
		t.Fatalf("合法的設定被擋：%v", err)
	}
	// 有主的郡要現找——**不要寫死郡號**：劇本 001 的十八個空白郡裡
	// 就有郡 1（遼東），寫死會讓「郡已經有主」那一格其實在驗別的事。
	owned := 0
	for _, p := range sc.Prefectures() {
		if p.Owned() {
			owned = p.ID
			break
		}
	}
	if owned == 0 {
		t.Fatal("劇本裡一個有主的郡都沒有")
	}
	bad := []struct {
		why string
		c   CustomLord
	}{
		{"點數超過", CustomLord{Name: name, Prefecture: 39, Stamina: 101}},
		{"點數是負的", CustomLord{Name: name, Prefecture: 39, Stamina: -1}},
		{"郡已經有主", CustomLord{Name: name, Prefecture: owned}},
		{"郡越界", CustomLord{Name: name, Prefecture: 0}},
		{"沒有名字", CustomLord{Prefecture: 39}},
	}
	for _, x := range bad {
		if err := x.c.Validate(sc); err == nil {
			t.Errorf("%s：沒有被擋", x.why)
		}
	}
}

// 寫進去之後讀得回來，而且原來那一份不動。
func TestWithCustomLordWritesTheTables(t *testing.T) {
	sc := scenarioForTest(t)
	slots := sc.CustomLordSlots()
	if len(slots) == 0 {
		t.Skip("這個劇本沒有空的新君主欄")
	}
	f := slots[0]
	c := CustomLord{
		Name:       [CustomLordNameChars]rune{'新', '君', '主'},
		Prefecture: 39, Stamina: 10, Intellect: 40, Might: 20, Charm: 30,
	}
	out, err := sc.WithCustomLord(f, c)
	if err != nil {
		t.Fatal(err)
	}
	// 原來那一份不動——**這是最容易寫錯的一條**：Tables() 回的是複本，
	// 改到原件的話「取消」就回不去了。
	if len(sc.Territory(f)) != 0 {
		t.Errorf("原來那一份被改到了：槽 %d 現在有 %d 個郡",
			f, len(sc.Territory(f)))
	}

	lord, err := out.Lord(f)
	if err != nil {
		t.Fatalf("讀不回新君主：%v", err)
	}
	st, in, mi, ch := c.Stats()
	if int(lord.Age) != CustomLordAge || int(lord.Stamina) != st ||
		int(lord.Intel) != in || int(lord.War) != mi ||
		int(lord.Charm) != ch {
		t.Errorf("能力值 ＝ 年齡 %d 體能 %d 謀略 %d 戰力 %d 魅力 %d，"+
			"想要 %d/%d/%d/%d/%d", lord.Age, lord.Stamina, lord.Intel,
			lord.War, lord.Charm, CustomLordAge, st, in, mi, ch)
	}
	if int(lord.Faction) != f {
		t.Errorf("勢力 ＝ %d，想要 %d", lord.Faction, f)
	}
	if int(lord.Location) != c.Prefecture {
		t.Errorf("所在郡 ＝ %d，想要 %d", lord.Location, c.Prefecture)
	}
	// 姓名是三個造字碼位，不是真的字——字模另外存。
	for i := 0; i < CustomLordNameChars; i++ {
		want := CustomGlyphBase + i
		got := int(lord.Raw[i*2])<<8 | int(lord.Raw[i*2+1])
		if got != want {
			t.Errorf("第 %d 個字的碼位 ＝ %04X，想要 %04X", i, got, want)
		}
	}
	// 領地換了主人，主事者是他。
	terr := out.Territory(f)
	if len(terr) != 1 || terr[0].ID != c.Prefecture {
		t.Fatalf("新君主的領地 ＝ %v，想要一個郡 %d", terr, c.Prefecture)
	}
	if gov, ok := out.Governor(c.Prefecture); !ok || gov.Raw != lord.Raw {
		t.Errorf("郡 %d 的主事者不是新君主", c.Prefecture)
	}
	// 現在它是「在用的勢力」了。
	live := false
	for _, x := range out.ActiveFactions() {
		if x == f {
			live = true
		}
	}
	if !live {
		t.Errorf("寫完之後槽 %d 還不算在用", f)
	}
	// 空欄少一個。
	if n := len(out.CustomLordSlots()); n != len(slots)-1 {
		t.Errorf("空的新君主欄剩 %d 個，想要 %d", n, len(slots)-1)
	}

	// 同一個槽不能再用一次。
	if _, err := out.WithCustomLord(f, c); err == nil {
		t.Error("同一個槽用了兩次沒被擋")
	}
}

// 六個劇本的新君主欄：判準是「君主欄指向範本（346 起）而且還沒進場」
// （原版 `0x123b6`），不是「不在 `ActiveFactions` 裡」——劇本三到六的
// 範本槽夾在中間、操縱方是 2，單看操縱方會把它們當成在用的勢力，
// 新君主欄一個都列不出來。
func TestCustomLordSlotsAcrossScenarios(t *testing.T) {
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
	want := map[Slot][]int{
		Scenario1: {14, 15}, Scenario2: {15}, Scenario3: {4, 5, 10, 11},
		Scenario4: {10, 11, 12, 13}, Scenario5: {4, 5}, Scenario6: {4, 5, 6, 7},
	}
	for slot, exp := range want {
		sc, err := LoadScenario(c, slot)
		if err != nil {
			t.Fatal(err)
		}
		got := sc.CustomLordSlots()
		if fmt.Sprint(got) != fmt.Sprint(exp) {
			t.Errorf("劇本 %s 的新君主欄 %v，要 %v", slot, got, exp)
		}
		live := map[int]bool{}
		for _, f := range sc.ActiveFactions() {
			live[f] = true
		}
		for _, f := range got {
			if live[f] {
				t.Errorf("劇本 %s 的槽 %d 同時是新君主欄與在用的勢力", slot, f)
			}
			if g, err := sc.Lord(f); err != nil || g.Index < CustomLordTemplateFrom {
				t.Errorf("劇本 %s 的槽 %d 君主欄沒有指向範本", slot, f)
			}
		}
		// 其餘的槽是劇本裡就沒在用的（操縱方 `0xFFFF`、君主欄 `0xFFFF`）。
		dead := 0
		for i := 0; i < masterCount; i++ {
			if sc.Controller(i) == ControlledByNobody {
				dead++
			}
		}
		if n := len(sc.ActiveFactions()) + len(got) + dead; n != masterCount {
			t.Errorf("劇本 %s：在用 %d ＋ 空欄 %d ＋ 沒在用 %d ≠ %d", slot, len(sc.ActiveFactions()), len(got), dead, masterCount)
		}
	}
}
