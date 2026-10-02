package i18n

import "testing"

func TestRelocalizeKeepsNumbersAndUnknownText(t *testing.T) {
	for _, key := range []string{"title.difficultyPrompt", "window.enhanced"} {
		original := Tf(ZhHant, key, 5)
		for _, l := range []Locale{En, Ja} {
			translated := Relocalize(original, ZhHant, l)
			if translated != Tf(l, key, 5) {
				t.Fatalf("%s %s: %q", key, l, translated)
			}
			if Relocalize(translated, l, ZhHant) != original {
				t.Fatal("回切改變參數")
			}
		}
	}
	unknown := "自訂文字 AB-345"
	if got := Relocalize(unknown, ZhHant, En); got != unknown {
		t.Fatalf("猜譯未知文字: %q", got)
	}
}
