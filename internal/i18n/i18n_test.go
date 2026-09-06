package i18n

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestEveryLocaleHasEveryKey 釘住每個語系都補齊了。
//
// **缺的字在畫面上會退回中文**，而一個中英混排的畫面看起來像
// 「這個地方還沒翻」與「這個地方壞了」的中間態——沒有人看得出是哪一種。
func TestEveryLocaleHasEveryKey(t *testing.T) {
	if len(Keys()) == 0 {
		t.Fatal("原文表是空的")
	}
	for _, l := range Locales() {
		if l == ZhHant {
			continue
		}
		if missing := Missing(l); len(missing) > 0 {
			t.Errorf("%s 少了 %d 個鍵：%v", l, len(missing), missing)
		}
		if extra := Extra(l); len(extra) > 0 {
			t.Errorf("%s 多了 %d 個原文沒有的鍵：%v", l, len(extra), extra)
		}
	}
}

// TestFormatVerbsMatch 釘住帶參數的句子在每個語系的動詞數相同。
//
// **`%s` 的數目對不上會在執行期印出 `%!s(MISSING)`**，
// 而那要跑到那一行才看得到。
func TestFormatVerbsMatch(t *testing.T) {
	for _, k := range Keys() {
		want := verbs(T(ZhHant, k))
		for _, l := range Locales() {
			if l == ZhHant {
				continue
			}
			if got := verbs(T(l, k)); got != want {
				t.Errorf("%s 的 %q：原文有 %d 個格式動詞，譯文有 %d 個\n  原文 %q\n  譯文 %q",
					l, k, want, got, T(ZhHant, k), T(l, k))
			}
		}
	}
}

// verbs 數一句話裡有幾個格式動詞（`%%` 不算）。
func verbs(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 < len(s) && s[i+1] == '%' {
			i++
			continue
		}
		n++
	}
	return n
}

// TestMissingKeyIsVisible 釘住查不到的鍵看得出來，不會變成空字串。
//
// 一個安靜地回空字串的翻譯層會讓缺字看起來像排版問題。
func TestMissingKeyIsVisible(t *testing.T) {
	got := T(En, "no.such.key")
	if got == "" {
		t.Fatal("查不到的鍵回了空字串")
	}
	if !strings.Contains(got, "no.such.key") {
		t.Errorf("查不到的鍵回 %q，應該看得出是哪一個鍵", got)
	}
}

// TestFallsBackToOriginal 釘住譯文缺一句時退回原文，不是退回空白。
func TestFallsBackToOriginal(t *testing.T) {
	// 借一個一定存在的鍵，臨時把譯文拿掉。
	const key = "cmd.status"
	saved := catalog[En][key]
	delete(catalog[En], key)
	defer func() { catalog[En][key] = saved }()
	if got := T(En, key); got != T(ZhHant, key) {
		t.Errorf("譯文缺這一句時回 %q，應該退回原文 %q", got, T(ZhHant, key))
	}
}

// TestParse 釘住語系名稱認得出來。
func TestParse(t *testing.T) {
	for _, c := range []struct {
		in   string
		want Locale
		ok   bool
	}{
		{"zh-Hant", ZhHant, true}, {"zh", ZhHant, true}, {"繁中", ZhHant, true},
		{"en", En, true}, {"english", En, true},
		{"ja", Ja, true}, {"jp", Ja, true}, {"日文", Ja, true},
		{"klingon", ZhHant, false}, {"", ZhHant, false},
	} {
		got, ok := Parse(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("Parse(%q) ＝ %v, %v；應該是 %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
	for _, l := range Locales() {
		if l.Name() == "" {
			t.Errorf("%s 沒有自己的名字", l)
		}
	}
}

// TestOriginalIsFirst 釘住原文排第一。
//
// **繁體中文是原文不是譯文**：這個專案還原的是 1991 年的繁中版。
func TestOriginalIsFirst(t *testing.T) {
	if Locales()[0] != ZhHant {
		t.Error("語系清單的第一個應該是原文")
	}
	if ZhHant.Name() != "繁體中文" {
		t.Errorf("原文的名字是 %q", ZhHant.Name())
	}
}

// TestCataloguesAreSortedJSON 釘住字串表是排好序的 JSON。
//
// **排序不是美觀問題**：兩份譯文並排比對要能逐行對照，
// 而未排序的 map 每次寫出來順序都不一樣，diff 會整份變紅。
func TestCataloguesAreSortedJSON(t *testing.T) {
	for _, l := range Locales() {
		b, err := lang.ReadFile("lang/" + string(l) + ".json")
		if err != nil {
			t.Fatalf("%s 的字串表讀不到：%v", l, err)
		}
		var m map[string]string
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("%s 的字串表解不開：%v", l, err)
		}
		want, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(b)) != strings.TrimSpace(string(want)) {
			t.Errorf("%s.json 不是排好序、縮排兩格的 JSON——請重新產生", l)
		}
	}
}

// TestNoLocaleFileIsOrphaned 釘住 lang/ 底下沒有多餘的檔案。
func TestNoLocaleFileIsOrphaned(t *testing.T) {
	entries, err := lang.ReadDir("lang")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(Locales()) {
		t.Errorf("lang/ 有 %d 個檔案，支援 %d 個語系", len(entries), len(Locales()))
	}
	for _, e := range entries {
		l := Locale(strings.TrimSuffix(e.Name(), ".json"))
		if !l.Valid() {
			t.Errorf("lang/%s 不對應任何支援的語系", e.Name())
		}
	}
}
