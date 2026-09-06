package game

import "fmt"

// 「其他」底下的開關（說明書 p.25–26）。
//
// 原版的提示都在字串表裡（`docs/re/04` §3）：
//
//	音樂狀態%s      開啟／關閉
//	音效狀態%s      開啟／關閉
//	查看電腦戰役%s  開啟／關閉
//	使用%s年號      中曆／西曆
//	語音狀態%s      開啟／關閉
//	設定延遲時間(%d)  0:等待按鍵(0-100)
//
// **開關要有狀態，不能只是選單上的一行。** 一個按下去什麼都不會變的
// 選項，與「這個功能還沒做」在畫面上長得一模一樣。

// Options 是這一局的顯示與音效設定。
//
// 零值就是原版的預設：音樂音效開、看電腦戰役、中曆、延時 30。
type Options struct {
	MusicOff   bool
	SoundOff   bool
	VoiceOff   bool
	SkipAIWar  bool
	Calendar   Calendar
	delaySet   bool
	delayValue int
}

// TuneDefaultDelay 是訊息停留時間的預設值。
//
// 原版收 0–100，`0` 表示等待按鍵（`設定延遲時間(%d)\n0:等待按鍵(0-100):`）；
// 預設是多少手冊沒寫，這個數字是 remake 選的。
const TuneDefaultDelay = 30

// Delay 是訊息停留時間（0–100，0 ＝ 等待按鍵）。
func (o *Options) Delay() int {
	if !o.delaySet {
		return TuneDefaultDelay
	}
	return o.delayValue
}

// SetDelay 設定訊息停留時間。原版收 0–100。
func (o *Options) SetDelay(v int) error {
	if v < 0 || v > 100 {
		return fmt.Errorf("game: 延遲時間 %d 越界（原版收 0..100）", v)
	}
	o.delaySet, o.delayValue = true, v
	return nil
}

// onOff 是原版的兩個字。
func onOff(on bool) string {
	if on {
		return "開啟"
	}
	return "關閉"
}

// ToggleMusic 等開關，回傳原版會顯示的那一行。
func (o *Options) ToggleMusic() string {
	o.MusicOff = !o.MusicOff
	return fmt.Sprintf("音樂狀態%s", onOff(!o.MusicOff))
}

// ToggleSound 切換音效。
func (o *Options) ToggleSound() string {
	o.SoundOff = !o.SoundOff
	return fmt.Sprintf("音效狀態%s", onOff(!o.SoundOff))
}

// ToggleVoice 切換語音。
func (o *Options) ToggleVoice() string {
	o.VoiceOff = !o.VoiceOff
	return fmt.Sprintf("語音狀態%s", onOff(!o.VoiceOff))
}

// ToggleAIWar 切換「查看電腦戰役」。
func (o *Options) ToggleAIWar() string {
	o.SkipAIWar = !o.SkipAIWar
	return fmt.Sprintf("查看電腦戰役%s", onOff(!o.SkipAIWar))
}

// ToggleCalendar 切換年號的表示方式。
func (o *Options) ToggleCalendar() string {
	if o.Calendar == Western {
		o.Calendar = ChineseEra
	} else {
		o.Calendar = Western
	}
	return fmt.Sprintf("使用%s年號", o.Calendar.Name())
}
