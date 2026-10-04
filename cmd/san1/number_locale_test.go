package main

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func switchNumberLocale(a *app, locale i18n.Locale) {
	old := i18n.Current
	i18n.Current = locale
	a.relocalizeWindow(old)
}

func TestCompletedNumberPromptLocale(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, from := range i18n.Locales() {
		for _, digits := range []string{"2", "02", "42"} {
			i18n.Current = from
			a := &app{art: &ui.ArtScreen{}}
			calls := 0
			a.askRange(i18n.T(i18n.Current, "ask.pref"), 1, 42, func(id int) {
				calls++
				a.view.Sel, a.view.Status = id, true
			})
			for _, d := range []byte(digits) {
				a.numberKey(d)
			}
			n := a.num
			a.num = nil
			if n.typed {
				n.then(n.value)
			}
			selected, called := a.view.Sel, calls
			for _, to := range append(i18n.Locales(), from) {
				switchNumberLocale(a, to)
				want := i18n.T(to, "ask.pref") + i18n.Tf(to, "pick.range", 1, 42) + digits
				if a.view.Prompt != want || a.num != nil || a.view.Sel != selected || calls != called {
					t.Fatalf("%s→%s digits=%q: prompt=%q sel=%d calls=%d", from, to, digits, a.view.Prompt, a.view.Sel, calls)
				}
			}
		}
	}
}

func TestActiveNumberPromptLocaleKeepsInput(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, digits := range []string{"", "0", "02", "99"} {
		i18n.Current = i18n.ZhHant
		a := &app{art: &ui.ArtScreen{}}
		calls := 0
		a.askRange(i18n.T(i18n.Current, "ask.pref"), 1, 42, func(int) { calls++ })
		for _, d := range []byte(digits) {
			a.numberKey(d)
		}
		n, value, typed := a.num, a.num.value, a.num.typed
		for _, locale := range []i18n.Locale{i18n.En, i18n.Ja, i18n.ZhHant} {
			switchNumberLocale(a, locale)
			want := i18n.T(locale, "ask.pref") + i18n.Tf(locale, "pick.range", 1, 42) + digits
			if a.view.Prompt != want || a.num != n || n.digits != digits || n.value != value || n.typed != typed || n.lo != 1 || n.max != 42 || calls != 0 {
				t.Fatalf("%s digits=%q 切換改變輸入或回呼", locale, digits)
			}
		}
	}
}

func TestNumberPromptBareUnknownAndReplacement(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, tc := range []struct {
		name    string
		bare    bool
		unknown bool
		replace string
	}{
		{name: "bare", bare: true},
		{name: "unknown", unknown: true},
		{name: "known replacement", replace: "msg.cancel"},
		{name: "unknown replacement", replace: "自訂提示 AB-345"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i18n.Current = i18n.ZhHant
			a := &app{art: &ui.ArtScreen{}}
			title := i18n.T(i18n.Current, "ask.pref")
			if tc.unknown {
				title = "自訂提示 AB-345\n"
			}
			if tc.bare {
				title = tf("ask.gift", "X")
				a.askBare(title, 2, 5, func(int) {})
			} else {
				a.askRange(title, 1, 42, func(int) {})
			}
			a.numberKey('2')
			a.num = nil
			if tc.replace != "" {
				a.view.Prompt = i18n.T(i18n.ZhHant, tc.replace)
			}
			for _, locale := range []i18n.Locale{i18n.En, i18n.Ja, i18n.ZhHant} {
				switchNumberLocale(a, locale)
				want := i18n.T(locale, "ask.pref") + i18n.Tf(locale, "pick.range", 1, 42) + "2"
				switch {
				case tc.replace != "":
					want = i18n.T(locale, tc.replace)
					if a.numPrompt != nil {
						t.Fatal("被取代的提示仍保留數字快照")
					}
				case tc.bare:
					want = i18n.Tf(locale, "ask.gift", "X") + "2"
				case tc.unknown:
					want = title + i18n.Tf(locale, "pick.range", 1, 42) + "2"
				}
				if a.view.Prompt != want || a.num != nil {
					t.Fatalf("%s 提示=%q，預期=%q", locale, a.view.Prompt, want)
				}
			}
		})
	}
}

func TestTextNumberPromptLocale(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	i18n.Current = i18n.ZhHant
	a := &app{}
	a.askNumber(i18n.T(i18n.Current, "ask.delay"), "", 100, func(int) {})
	a.numberKey('5')
	if a.numPrompt != nil {
		t.Fatal("文字版面保留了美術版面的組合提示")
	}
	switchNumberLocale(a, i18n.En)
	if a.num == nil || a.num.value != 5 || a.num.digits != "5" || a.view.Menu != i18n.T(i18n.En, "ask.delay") {
		t.Fatal("文字版面切換改變數字或選單")
	}
}
