package cells

import (
	"reflect"
	"testing"
)

func TestRuneWidth(t *testing.T) {
	for _, tc := range []struct {
		r    rune
		want int
	}{
		{'A', 1}, {'0', 1}, {' ', 1}, {'~', 1},
		{'遼', 2}, {'東', 2}, {'諸', 2},
		{'　', 2},           // 全形空白
		{'あ', 2}, {'ア', 2}, // 日文假名（多語系會用到）
		{'é', 1}, // 拉丁補充：一格，不是全形
		{'\n', 0}, {'\t', 0},
	} {
		if got := RuneWidth(tc.r); got != tc.want {
			t.Errorf("RuneWidth(%q) ＝ %d，想要 %d", tc.r, got, tc.want)
		}
	}
}

func TestWidth(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want int
	}{
		{"", 0},
		{"遼東", 4},
		{"Liaodong", 8},
		{"諸葛亮", 6},
		{"Zhuge Liang", 11},
	} {
		if got := Width(tc.s); got != tc.want {
			t.Errorf("Width(%q) ＝ %d，想要 %d", tc.s, got, tc.want)
		}
	}
}

// TestTruncateNeverSplitsWide 釘住「不切半全形字」。
//
// 切半的話在原版的固定格版面上會變成亂碼，而**測試看不到**——
// 那是要實跑才抓得到的錯（CLAUDE.md §7 第 13 條）。所以在這裡擋。
func TestTruncateNeverSplitsWide(t *testing.T) {
	for _, tc := range []struct {
		s    string
		cols int
		want string
	}{
		{"遼東", 4, "遼東"},
		{"遼東", 3, "遼"}, // 剩一格放不下全形，停住
		{"遼東", 2, "遼"},
		{"遼東", 1, ""}, // 一格都放不下
		{"遼東", 0, ""},
		{"AB遼", 3, "AB"}, // 剩一格
		{"AB遼", 4, "AB遼"},
	} {
		if got := Truncate(tc.s, tc.cols); got != tc.want {
			t.Errorf("Truncate(%q, %d) ＝ %q，想要 %q", tc.s, tc.cols, got, tc.want)
		}
		if w := Width(Truncate(tc.s, tc.cols)); w > tc.cols {
			t.Errorf("Truncate(%q, %d) 寬度 %d 超過上限", tc.s, tc.cols, w)
		}
	}
}

func TestPadAndCenter(t *testing.T) {
	if got := Pad("遼東", 6); got != "遼東  " {
		t.Errorf("Pad ＝ %q", got)
	}
	if w := Width(Pad("遼東", 6)); w != 6 {
		t.Errorf("Pad 之後寬度 ＝ %d，想要 6", w)
	}
	// 原版的兩字郡名放在六格槽：左右各一格空白。
	if got := Center("遼東", 6); got != " 遼東 " {
		t.Errorf("Center ＝ %q，想要 %q", got, " 遼東 ")
	}
	if w := Width(Center("諸葛亮", 6)); w != 6 {
		t.Errorf("Center 三字名寬度 ＝ %d，想要 6", w)
	}
	// 補到剛好，不多不少——短的沒補滿會露出上一次畫的內容。
	for _, s := range []string{"", "A", "遼", "遼東", "Zhuge"} {
		for _, c := range []int{0, 1, 4, 8, 16} {
			if w := Width(Pad(s, c)); w != c {
				t.Errorf("Pad(%q, %d) 寬度 ＝ %d", s, c, w)
			}
		}
	}
}

func TestWrap(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    string
		cols int
		want []string
	}{
		{"中文逐字斷", "遼東涿郡渤海", 4, []string{"遼東", "涿郡", "渤海"}},
		{"換行強制斷", "遼東\n涿郡", 10, []string{"遼東", "涿郡"}},
		{"英文不切單字", "Zhuge Liang", 8, []string{"Zhuge ", "Liang"}},
		{"單字超長照切", "Liaodongprefecture", 6, []string{"Liaodo", "ngpref", "ecture"}},
		{"剛好一行", "遼東", 4, []string{"遼東"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Wrap(tc.s, tc.cols)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Wrap(%q, %d) ＝ %q，想要 %q", tc.s, tc.cols, got, tc.want)
			}
			for _, l := range got {
				if Width(l) > tc.cols {
					t.Errorf("行 %q 寬度 %d 超過 %d", l, Width(l), tc.cols)
				}
			}
		})
	}
}

// TestMultilingualOverflow 是這一層存在的理由：
// 原文是繁中，英日譯文要塞回原版的槽位，而英文通常比中文長。
// 「裝不下」要在這裡量出來，不是等畫面破版才發現。
func TestMultilingualOverflow(t *testing.T) {
	slot := 6 // 原版一個三字郡名的槽位
	for _, tc := range []struct {
		lang, text string
		fits       bool
	}{
		{"zh-Hant", "遼東", true},
		{"zh-Hant", "諸葛亮", true},
		{"ja", "遼東", true},
		{"en", "Liaodong", false}, // 8 格 > 6
		{"en", "Wu", true},
	} {
		if got := Fits(tc.text, slot); got != tc.fits {
			t.Errorf("[%s] Fits(%q, %d) ＝ %v，想要 %v（寬度 %d）",
				tc.lang, tc.text, slot, got, tc.fits, Width(tc.text))
		}
	}
}

func TestWrapDegenerate(t *testing.T) {
	// cols <= 0 不能無窮迴圈。
	if got := Wrap("遼東", 0); !reflect.DeepEqual(got, []string{"遼東"}) {
		t.Errorf("Wrap(_, 0) ＝ %q", got)
	}
	// 全形字放不進一格寬的框：不能卡死，也不能讓回傳的行超過 cols。
	for _, l := range Wrap("遼東", 1) {
		if Width(l) > 1 {
			t.Errorf("寬度 1 的框回傳了 %d 格寬的行 %q", Width(l), l)
		}
	}
}

// TestMinWidth 是給版面測試用的擋牆：框比一個字還窄時 Wrap 只能把字丟掉，
// 而丟掉是安靜的，所以要在更早的地方擋。
func TestMinWidth(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want int
	}{
		{"", 0},
		{"ABC", 1},
		{"遼東", 2},
		{"Wu 吳", 2},
	} {
		if got := MinWidth(tc.s); got != tc.want {
			t.Errorf("MinWidth(%q) ＝ %d，想要 %d", tc.s, got, tc.want)
		}
	}
	// 所有 42 個郡名都是兩字，所以郡名槽至少要兩格。
	if MinWidth("遼東") > 2 {
		t.Error("郡名的最小寬度不該超過 2")
	}
}

// TestColumnsNeverTruncate 釘住表格的欄名不會被截、欄與欄不會黏在一起。
//
// 先前的表格欄寬是寫死的：英文「Governor」補到八格剛好沒有空白，
// 與「Gold」黏成「GovernorGold」；「Flood Risk」被截成「Floo」。
func TestColumnsNeverTruncate(t *testing.T) {
	got := Columns([][]string{
		{"No.", "Governor", "Flood Risk", "Loyal"},
		{"1", "陳就", "43", "36"},
		{"41", "Zhuge Liang", "7", "100"},
	})
	want := []string{
		"No. Governor    Flood Risk Loyal",
		"1   陳就        43         36",
		"41  Zhuge Liang 7          100",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 列：\n  排出來 %q\n  想要   %q", i, got[i], want[i])
		}
	}
}
