package state

import (
	"bytes"
	"fmt"
	"testing"
)

// 原版出貨的六個進度（`docs/formats/05`）。這些數字是從 `DATA2` 讀出來的，
// 不是推的：年份對得上 `SAVENAME.SVP` 裡寫的「Y201」「Y208」「Y215」「Y220」。
var shippedProgress = []struct {
	year, month, cursor, diff, pending int
}{
	{197, 8, 16, 9, 26},
	{198, 5, 17, 15, 25},
	{201, 1, 38, 3, 5},
	{208, 1, 2, 3, 40},
	{215, 1, 6, 3, 36},
	{220, 1, 0, 3, 42},
}

// TestShippedProgressDecodes 對六個出貨進度逐欄核對。
//
// 判準裡最硬的一條把**三段獨立資料**綁在一起——旗標陣列、順序表與游標：
//
//	未下令的郡數 ＝ 42 − 游標 ＋ (順序表裡第 0 郡的位置 < 游標 ? 1 : 0)
//
// 開月時 43 格全設成「還沒」，接著第 0 筆（原版的啞元州郡）被單獨標成
// 已下令（`0x17401`），所以起點是 42。游標每走一格就清掉順序表指到的
// 那一郡；輪到第 0 郡時清的是已經清過的那一格，所以要補回來一個。
// 版面認錯一格，這條就會破。
func TestShippedProgressDecodes(t *testing.T) {
	c := loadData2(t, "三國演義")
	for i, exp := range shippedProgress {
		slot := Slot(fmt.Sprintf("SV%d", i+1))
		p, err := LoadProgress(c, slot)
		if err != nil {
			t.Fatalf("%s：%v", slot, err)
		}
		if p.Year != exp.year || p.Month != exp.month {
			t.Errorf("%s：%d 年 %d 月，想要 %d 年 %d 月",
				slot, p.Year, p.Month, exp.year, exp.month)
		}
		if p.Cursor != exp.cursor || p.Difficulty != exp.diff {
			t.Errorf("%s：游標 %d 難度 %d，想要 %d／%d",
				slot, p.Cursor, p.Difficulty, exp.cursor, exp.diff)
		}
		n := 0
		for _, pending := range p.Pending {
			if pending {
				n++
			}
		}
		if n != exp.pending {
			t.Errorf("%s：%d 個郡還沒下令，想要 %d", slot, n, exp.pending)
		}
		want := proSlots - 1 - p.Cursor
		for pos, pref := range p.Order {
			if pref == 0 && pos < p.Cursor {
				want++
			}
		}
		if n != want {
			t.Errorf("%s：還沒下令的 %d 個對不上游標 %d（應為 %d）",
				slot, n, p.Cursor, want)
		}
		if !p.InMonth {
			t.Errorf("%s：月內迴圈旗標不是「繼續」", slot)
		}
	}
}

// TestProgressRoundTrip 解完再編回去要逐位元組相同，**包含沒解出來的
// 那 58 個位元組**——存檔時把未知欄位寫成零，等於替將來解出它的人
// 銷毀證據。
func TestProgressRoundTrip(t *testing.T) {
	c := loadData2(t, "三國演義")
	for i := 1; i <= 6; i++ {
		slot := Slot(fmt.Sprintf("SV%d", i))
		raw, err := section(c, "BASEPRO."+string(slot), ProgressSize)
		if err != nil {
			t.Fatal(err)
		}
		p, err := DecodeProgress(raw)
		if err != nil {
			t.Fatalf("%s：%v", slot, err)
		}
		if got := p.Encode(); !bytes.Equal(got, raw) {
			for k := range got {
				if got[k] != raw[k] {
					t.Fatalf("%s：位移 0x%02x 編回去是 0x%02x，原版是 0x%02x",
						slot, k, got[k], raw[k])
				}
			}
		}
	}
}

// TestShippedGlyphsAreTheDefaultName 釘住 `BASEPRE` 的用途：
// 十二個 16×16 字模 ＝ 四個自創君主 × 三個字，出貨的六個進度全部相同，
// 內容是預設姓名重複四次。
func TestShippedGlyphsAreTheDefaultName(t *testing.T) {
	c := loadData2(t, "三國演義")
	var first *Glyphs
	for i := 1; i <= 6; i++ {
		slot := Slot(fmt.Sprintf("SV%d", i))
		g, err := LoadGlyphs(c, slot)
		if err != nil {
			t.Fatalf("%s：%v", slot, err)
		}
		if first == nil {
			first = g
			continue
		}
		if *g != *first {
			t.Errorf("%s 的字模與 SV1 不同", slot)
		}
	}
	// 四個名額共用同一組三個字模。
	for lord := 1; lord < CustomLords; lord++ {
		for ch := 0; ch < CustomLordNameChars; ch++ {
			a := first[ch]
			b := first[lord*CustomLordNameChars+ch]
			if a != b {
				t.Errorf("第 %d 個名額的第 %d 個字與第一個名額不同", lord+1, ch+1)
			}
		}
	}
	// 字模不是空的：三個字加起來要有相當數量的點。
	on := 0
	for i := 0; i < CustomLordNameChars; i++ {
		for r := 0; r < 16; r++ {
			v := first.Row(i, r)
			for b := 0; b < 16; b++ {
				on += int(v >> b & 1)
			}
		}
	}
	if on < 100 {
		t.Errorf("三個字模只有 %d 個點，看起來是空的", on)
	}
	if first.Code(0) != CustomGlyphBase {
		t.Errorf("第一個字模的碼位是 0x%04x", first.Code(0))
	}
}

// TestShippedSaveNames 釘住 `SAVENAME.SVP` 的版面：六筆 21 byte。
//
// 第一筆的名字是三個**造字碼位**（`A141`–`A143`），解成標準 Big5
// 就是三個全形標點——那不是亂碼，是原版拿標點的碼位當自創君主的名字。
func TestShippedSaveNames(t *testing.T) {
	c := loadData2(t, "三國演義")
	names, err := LoadSaveNames(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 6 {
		t.Fatalf("讀出 %d 筆名稱", len(names))
	}
	for i, n := range names {
		if n == "" {
			t.Errorf("第 %d 筆名稱是空的", i+1)
		}
	}
	for i, want := range []string{"Y201", "Y208", "Y215", "Y220"} {
		if got := names[i+2]; !bytes.Contains([]byte(got), []byte(want)) {
			t.Errorf("第 %d 筆 %q 裡沒有 %q", i+3, got, want)
		}
	}
	if got, err := EncodeSaveNames(names); err != nil {
		t.Fatal(err)
	} else if back, err := DecodeSaveNames(got); err != nil {
		t.Fatal(err)
	} else {
		for i := range names {
			if back[i] != names[i] {
				t.Errorf("第 %d 筆來回一趟變成 %q（原本 %q）", i+1, back[i], names[i])
			}
		}
	}
}

// TestShowCustomGlyphsMapsTheShippedName 釘住造字碼位 A141–A14C 畫成出貨字模的
// 「新君主」（每三格一組），其他字不動。
func TestShowCustomGlyphsMapsTheShippedName(t *testing.T) {
	var raw []byte
	for i := 0; i < CustomLords*CustomLordNameChars; i++ {
		code := CustomGlyphBase + i
		raw = append(raw, byte(code>>8), byte(code))
	}
	s, err := decodeBig5(append(raw, []byte("\xa6\x62")...)) // 結尾加一個「在」
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ShowCustomGlyphs(s), "新君主新君主新君主新君主在"; got != want {
		t.Errorf("ShowCustomGlyphs(%q) ＝ %q，想要 %q", s, got, want)
	}
}
