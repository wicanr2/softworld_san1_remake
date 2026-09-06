package session

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestSaveThenLoadContinues 釘住存了再讀回來還能繼續玩下去。
//
// 逐欄比對在 `internal/save`；這裡問的是另一件事——**讀回來的那一局
// 推得動嗎**。一份欄位都對但推一個月就當掉的存檔，逐欄比對看不出來。
func TestSaveThenLoadContinues(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	for i := 0; i < 24; i++ {
		s.EndMonth()
	}
	dir := t.TempDir()
	if err := s.Save(dir, 1, "續戰"); err != nil {
		t.Fatal(err)
	}
	list := Saves(dir)
	if len(list) != 6 || !list[0].Exists || list[0].Name != "續戰" {
		t.Fatalf("存檔清單不對：%+v", list)
	}

	t2, err := Load(dir, 1, ai.ModeEnhanced)
	if err != nil {
		t.Fatal(err)
	}
	if t2.G.Date != s.G.Date {
		t.Fatalf("讀回來是 %v，存的是 %v", t2.G.Date, s.G.Date)
	}
	before := t2.G.Date
	for i := 0; i < 12; i++ {
		t2.EndMonth()
	}
	if t2.G.Date == before {
		t.Error("讀回來的局面推不動")
	}
	// 讀回來之後再跑一年，兩邊應該走到同一個局面——**規則是決定性的**。
	for i := 0; i < 12; i++ {
		s.EndMonth()
	}
	if s.G.Date != t2.G.Date {
		t.Errorf("兩邊各跑一年後年月不同：%v vs %v", s.G.Date, t2.G.Date)
	}
	for id := 1; id <= state.PrefectureCount; id++ {
		a, b := s.G.Prefecture(id), t2.G.Prefecture(id)
		if a.Owner != b.Owner || a.Population != b.Population ||
			a.Gold != b.Gold || a.Rice != b.Rice {
			t.Fatalf("郡 %d 分家了：\n原局 %+v\n讀檔 %+v", id, *a, *b)
		}
	}
}

// TestSaveNeedsADirectory 釘住沒有存檔目錄要說出來。
//
// **不要偷偷選一個預設目錄**：玩家會在不知道的地方留下檔案，
// 而下一次「怎麼找不到我的進度」就沒人答得出來。
func TestSaveNeedsADirectory(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	if err := s.Save("", 1, "x"); err == nil {
		t.Error("沒有存檔目錄竟然存得成功")
	}
	if _, err := Load("", 1, ai.ModeEnhanced); err == nil {
		t.Error("沒有存檔目錄竟然讀得成功")
	}
	if Saves("") != nil {
		t.Error("沒有存檔目錄不該列得出存檔")
	}
}

// TestSaveNamesItselfAfterTheLord 釘住沒給名字時用君主的名字。
func TestSaveNamesItselfAfterTheLord(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, 0)
	dir := t.TempDir()
	if err := s.Save(dir, 3, ""); err != nil {
		t.Fatal(err)
	}
	lord := s.G.Lord(s.Player)
	if lord == nil {
		t.Skip("這一局沒有君主")
	}
	if got := Saves(dir)[2].Name; got != lord.Name {
		t.Errorf("存檔名稱是 %q，應該是君主的名字 %q", got, lord.Name)
	}
}
