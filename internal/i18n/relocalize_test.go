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

func TestRelocalizeUnknownTextRoundTrip(t *testing.T) {
	for _, text := range []string{"自訂提示 AB-345", "自訂紀錄 AB-345\n", "Custom text AB-345"} {
		for _, locale := range []Locale{En, Ja} {
			translated := Relocalize(text, ZhHant, locale)
			if translated != text || Relocalize(translated, locale, ZhHant) != text {
				t.Fatalf("空白模板誤匹配未知文字：%s %q", locale, text)
			}
		}
	}
}

func TestRelocalizePaddedInteger(t *testing.T) {
	for _, value := range []int{0, 6, 17, -6} {
		original := Tf(ZhHant, "bat.win.order", "A", "B", value, "C")
		for _, locale := range []Locale{En, Ja} {
			translated := Relocalize(original, ZhHant, locale)
			if translated != Tf(locale, "bat.win.order", "A", "B", value, "C") ||
				Relocalize(translated, locale, ZhHant) != original {
				t.Fatalf("%s 填寬數值 %d 未保持：%q", locale, value, translated)
			}
		}
	}
}
