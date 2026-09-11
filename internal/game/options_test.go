package game

import "testing"

// TestOptionDefaults 釘住零值就是原版的預設。
func TestOptionDefaults(t *testing.T) {
	var o Options
	if o.MusicOff || o.SoundOff || o.VoiceOff || o.SkipAIWar {
		t.Error("預設應該是音樂、音效、語音都開，而且看電腦戰役")
	}
	if o.Calendar != ChineseEra {
		t.Error("預設應該是中曆年號（手冊 p.26）")
	}
	if o.Delay() != TuneDefaultDelay {
		t.Errorf("預設延時是 %d，應該是 %d", o.Delay(), TuneDefaultDelay)
	}
}

// TestTogglesSayWhatTheOriginalSays 釘住每個開關回的那一行與原版的
// 格式字串相同（`docs/re/04` §3）。
func TestTogglesSayWhatTheOriginalSays(t *testing.T) {
	var o Options
	cases := []struct {
		fn     func() string
		first  string
		second string
	}{
		{o.ToggleMusic, "音樂狀態關閉", "音樂狀態開啟"},
		{o.ToggleSound, "音效狀態關閉", "音效狀態開啟"},
		{o.ToggleVoice, "語音狀態關閉", "語音狀態開啟"},
		{o.ToggleAIWar, "查看電腦戰役關閉", "查看電腦戰役開啟"},
		{o.ToggleCalendar, "使用西曆年號", "使用中曆年號"},
	}
	for _, c := range cases {
		if got := c.fn(); got != c.first {
			t.Errorf("第一次切換回 %q，應該是 %q", got, c.first)
		}
		if got := c.fn(); got != c.second {
			t.Errorf("再切一次回 %q，應該是 %q", got, c.second)
		}
	}
}

// TestTogglesActuallyChangeState 釘住開關真的改到狀態。
//
// **一個按下去什麼都不會變的選項，與「這個功能還沒做」在畫面上
// 長得一模一樣。**
func TestTogglesActuallyChangeState(t *testing.T) {
	var o Options
	o.ToggleMusic()
	if !o.MusicOff {
		t.Error("切了音樂卻沒有關掉")
	}
	o.ToggleCalendar()
	if o.Calendar != Western {
		t.Error("切了年號卻沒有換成西曆")
	}
}

// TestDelayRange 釘住延時收 0–100（原版 `設定延遲時間(%d)\n0:等待按鍵(0-100):`）。
func TestDelayRange(t *testing.T) {
	var o Options
	for _, v := range []int{0, 1, 50, 100} {
		if err := o.SetDelay(v); err != nil {
			t.Errorf("延時 %d 應該收：%v", v, err)
		}
		if o.Delay() != v {
			t.Errorf("設成 %d 之後讀到 %d", v, o.Delay())
		}
	}
	for _, v := range []int{-1, 101, 1000} {
		if err := o.SetDelay(v); err == nil {
			t.Errorf("延時 %d 不該收", v)
		}
	}
	// 設過 0 之後不該掉回預設值——0 是「等待按鍵」，是有意義的設定。
	if err := o.SetDelay(0); err != nil {
		t.Fatal(err)
	}
	if o.Delay() != 0 {
		t.Errorf("設成 0 之後讀到 %d，0 是「等待按鍵」不是「沒設定」", o.Delay())
	}
}

// TestAIOrdersDefaultsToOne 釘住「電腦指令」的預設是一道。
//
// 預設 1 的理由是那條線最好懂：玩家一個月一道令，電腦也是。
// **這一格調的是強化 AI 有多強不是規則**——原版的電腦本來就不受
// 「每郡每月一道令」管（`docs/mechanics/70-ai` §2.12）。
func TestAIOrdersDefaultsToOne(t *testing.T) {
	var o Options
	if o.AIOrders() != AIOrdersDefault {
		t.Errorf("預設是 %d，應該是 %d", o.AIOrders(), AIOrdersDefault)
	}
	if AIOrdersDefault != 1 {
		t.Errorf("預設值改成 %d 了；使用者要的是一道", AIOrdersDefault)
	}
	for _, v := range []int{AIOrdersDefault, 3, AIOrdersMax} {
		if err := o.SetAIOrders(v); err != nil {
			t.Errorf("設 %d 應該收：%v", v, err)
		} else if o.AIOrders() != v {
			t.Errorf("設 %d 讀回 %d", v, o.AIOrders())
		}
	}
	// **越界要擋下來**，不要夾成一個看起來正常的數字：玩家多按一位
	// 卻換來「電腦一次下五道令」是查不出來的意外。
	for _, v := range []int{0, -1, AIOrdersMax + 1, 99} {
		if err := o.SetAIOrders(v); err == nil {
			t.Errorf("設 %d 竟然收下了", v)
		}
	}
	if o.AIOrders() != AIOrdersMax {
		t.Errorf("被擋下來的那幾次改到了值（現在是 %d）", o.AIOrders())
	}
}

// TestAIModeStartsEmpty 釘住 AI 版本預設是「開局挑的那一個」。
//
// 零值不能是 `base`——那會讓「沒動過設定」與「選了原版 AI」變成同一件事，
// 而兩者在加強版的規則上是不同的結局（`ai.CheckEdition`）。
func TestAIModeStartsEmpty(t *testing.T) {
	var o Options
	if o.AIMode != "" {
		t.Errorf("預設的 AI 版本是 %q，應該是空字串（＝開局挑的那一個）", o.AIMode)
	}
	o.SetAIMode("enhanced")
	if o.AIMode != "enhanced" {
		t.Errorf("設完是 %q", o.AIMode)
	}
}
