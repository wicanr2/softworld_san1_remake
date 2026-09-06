package ui

import (
	"image/color"
	"os"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
)

var (
	bg = color.RGBA{0, 0, 0, 255}
	fg = color.RGBA{255, 255, 255, 255}
)

func testFace(t *testing.T) *font.Face {
	t.Helper()
	fh, err := os.Open("../../fonts/unifont.hex.gz")
	if err != nil {
		t.Skipf("沒有字型檔：%v", err)
	}
	defer fh.Close()
	f, err := font.ParseHexGz(fh, 16)
	if err != nil {
		t.Fatalf("字型載入失敗：%v", err)
	}
	return f
}

func TestDrawTextInk(t *testing.T) {
	f := testFace(t)
	c := NewCanvas(20, 3, f)
	c.Fill(bg)

	// 沒畫之前每一格都是空的——先確認基準，否則「有墨水」證明不了什麼。
	if n := c.InkAt(0, 0, bg); n != 0 {
		t.Fatalf("還沒畫，(0,0) 就有 %d 個像素", n)
	}

	w := c.DrawText(0, 0, "遼東", fg)
	if w != 4 {
		t.Errorf("畫了 %d 格，想要 4（兩個全形字）", w)
	}
	for col := 0; col < 4; col++ {
		if c.InkAt(col, 0, bg) == 0 {
			t.Errorf("第 %d 格是空的——字沒畫出來", col)
		}
	}
	// 第 4 格之後不該有東西。
	if n := c.InkAt(4, 0, bg); n != 0 {
		t.Errorf("第 4 格有 %d 個像素，畫超出去了", n)
	}
	if len(c.Missing) != 0 {
		t.Errorf("有缺字：%v", c.Missing)
	}
}

// TestDrawTextClips 釘住「超出右緣不繞行」。
//
// 原版的版面是固定格的，繞行會蓋掉下一列的東西。
func TestDrawTextClips(t *testing.T) {
	f := testFace(t)
	c := NewCanvas(4, 2, f)
	c.Fill(bg)
	w := c.DrawText(0, 0, "遼東涿郡", fg) // 8 格，畫布只有 4 格
	if w != 4 {
		t.Errorf("畫了 %d 格，想要 4（其餘裁掉）", w)
	}
	// 第 1 列必須完全乾淨——繞行的話這裡會有東西。
	for col := 0; col < 4; col++ {
		if n := c.InkAt(col, 1, bg); n != 0 {
			t.Errorf("第 1 列第 %d 格有 %d 個像素——文字繞行了", col, n)
		}
	}
}

// TestMissingIsRecorded 釘住「缺字要記下來，不要靜靜畫成空白」。
//
// 用一份自造的小字型，不用真字型去找「它沒有的字」——
// 第一版那樣寫，挑的碼位 unifont 居然有（它給未指派區也附了字模），
// 測試就變成在賭字型的涵蓋範圍。自造的字型知道自己有什麼。
func TestMissingIsRecorded(t *testing.T) {
	f, err := font.ParseHex(strings.NewReader("0041:FF00\n"), 2) // 只有 'A'
	if err != nil {
		t.Fatal(err)
	}
	c := NewCanvas(8, 1, f)
	c.Fill(bg)
	c.DrawText(0, 0, "AB", fg)
	if c.Missing['B'] != 1 {
		t.Errorf("缺字沒被記下來：%v", c.Missing)
	}
	if c.Missing['A'] != 0 {
		t.Errorf("'A' 有字模，不該記成缺字：%v", c.Missing)
	}
	// 'A' 畫在第 0 格，'B' 缺字所以第 1 格必須是空的——
	// 而且游標**有前進**（缺字不會讓後面的字擠上來）。
	if c.InkAt(0, 0, bg) == 0 {
		t.Error("'A' 沒畫出來")
	}
	if n := c.InkAt(1, 0, bg); n != 0 {
		t.Error("缺字時不該畫東西（畫豆腐塊會讓缺字看起來像設計）")
	}
}

// TestPrefectureRowFits 是版面測試：42 個郡名排成六欄，
// 每個槽 6 格寬時所有名字都要畫得出來、而且不溢出。
func TestPrefectureRowFits(t *testing.T) {
	f := testFace(t)
	const slot = 6
	names := []string{"遼東", "涿郡", "渤海", "鄴郡", "太原", "上黨"}
	c := NewCanvas(slot*len(names), 1, f)
	c.Fill(bg)
	for i, n := range names {
		if !cells.Fits(n, slot) {
			t.Errorf("%q 放不進 %d 格", n, slot)
		}
		if got := c.DrawText(i*slot, 0, cells.Center(n, slot), fg); got != slot {
			t.Errorf("%q 畫了 %d 格，想要 %d", n, got, slot)
		}
	}
	if len(c.Missing) != 0 {
		t.Errorf("有缺字：%v", c.Missing)
	}
	// 每個槽的中間兩格要有墨水（Center 之後名字在中間）。
	for i := range names {
		mid := i*slot + slot/2 - 1
		if c.InkAt(mid, 0, bg) == 0 {
			t.Errorf("第 %d 個槽的中間是空的", i)
		}
	}
}

func TestDrawBox(t *testing.T) {
	f := testFace(t)
	c := NewCanvas(6, 3, f)
	c.Fill(bg)
	c.DrawBox(0, 0, 6, 3, fg)
	// 框內部（第 1 列中間的格）不該被框線碰到。
	if n := c.InkAt(2, 1, bg); n != 0 {
		t.Errorf("框內部有 %d 個像素，框線畫進去了", n)
	}
	// 角落要有。
	if c.InkAt(0, 0, bg) == 0 {
		t.Error("左上角沒有框線")
	}
}

// TestOtherSubMenuFollowsManual 釘住「其他」的八項（說明書 p.25）。
//
// **還沒做的也要列出來。** 選單少一項，玩家看不出是「還沒做」
// 還是「原版沒有」；列出來按下去會說還沒實作，那是可以理解的狀態。
func TestOtherSubMenuFollowsManual(t *testing.T) {
	title, items := SubMenu('9')
	if title != "其他" {
		t.Errorf("第 9 類叫 %q，應該是「其他」", title)
	}
	want := []string{"＊結束", "儲存", "音樂", "音效", "延時", "戰役", "年號", "語音"}
	if len(items) != len(want) {
		t.Fatalf("其他有 %d 項，手冊列八項", len(items))
	}
	for i, w := range want {
		if items[i].Name != w {
			t.Errorf("第 %d 項是 %q，應該是 %q", i+1, items[i].Name, w)
		}
		if items[i].Key != byte('1'+i) {
			t.Errorf("%q 的按鍵是 %q，應該是 %q", w, items[i].Key, byte('1'+i))
		}
	}
}

// TestEveryMainCommandHasASubMenu 釘住十個指令都有東西可按。
//
// 0 是「狀態」，原版直接顯示在畫面上不必展開。
func TestEveryMainCommandHasASubMenu(t *testing.T) {
	for _, c := range Commands() {
		if c.Key == '0' {
			continue
		}
		title, items := SubMenu(c.Key)
		if title == "" || len(items) == 0 {
			t.Errorf("「%s」（%q）沒有子選單", c.Name, c.Key)
		}
	}
}
