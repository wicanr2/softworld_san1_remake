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

	// AIMode 是電腦用哪一版 AI（`base`／`plus`／`enhanced`）。
	// 空字串表示「開局時挑的那一個」。
	//
	// **這一項與底下的 AIOrders 都是 remake 加的**：原版的「其他」只有
	// 八項（`docs/re/04` §3），沒有 AI 的設定——它只有一套 AI。
	// 型別用字串不用 `ai.Mode`，那一包依賴這一包。
	AIMode string

	// aiOrders 是**強化 AI** 一個郡一個月下幾道令（1–5，0 ＝ 預設）。
	//
	// 原版的電腦不受「每郡每月一道令」管（`docs/mechanics/70-ai` §2.12），
	// 所以這一格調的是**強化 AI 有多強**，不是規則：1 與玩家同一條線、
	// 5 讓它一個月裡又打仗又補兵。還原版（`base`／`plus`）不看這一格，
	// 它照原版把十八張表全部跑一遍。
	aiOrders int
}

// AIOrdersDefault／AIOrdersMax 是強化 AI 每郡每月的指令數範圍。
const (
	AIOrdersDefault = 1
	AIOrdersMax     = 5
)

// AIOrders 是強化 AI 一個郡一個月下幾道令。
func (o *Options) AIOrders() int {
	if o.aiOrders < AIOrdersDefault {
		return AIOrdersDefault
	}
	if o.aiOrders > AIOrdersMax {
		return AIOrdersMax
	}
	return o.aiOrders
}

// SetAIOrders 設定強化 AI 每郡每月的指令數。
func (o *Options) SetAIOrders(v int) error {
	if v < AIOrdersDefault || v > AIOrdersMax {
		return fmt.Errorf("game: 電腦指令數 %d 越界（收 %d..%d）",
			v, AIOrdersDefault, AIOrdersMax)
	}
	o.aiOrders = v
	return nil
}

// SetAIMode 記下這一局要用哪一版 AI。
//
// **這裡只存字串不做驗證**：能不能跑在這一版規則上是 `ai.CheckEdition`
// 的事，而那一包依賴這一包，反過來 import 會繞成環。呼叫端要先過那一關
// 再寫進來（`cmd/san1` 走 `ai.NextMode`，它只在合法的版本裡繞）。
func (o *Options) SetAIMode(mode string) { o.AIMode = mode }

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
		return t("oth.on")
	}
	return t("oth.off")
}

// ToggleMusic 等開關，回傳原版會顯示的那一行。
func (o *Options) ToggleMusic() string {
	o.MusicOff = !o.MusicOff
	return tf("oth.music.state", onOff(!o.MusicOff))
}

// ToggleSound 切換音效。
func (o *Options) ToggleSound() string {
	o.SoundOff = !o.SoundOff
	return tf("oth.sound.state", onOff(!o.SoundOff))
}

// ToggleVoice 切換語音。
func (o *Options) ToggleVoice() string {
	o.VoiceOff = !o.VoiceOff
	return tf("oth.voice.state", onOff(!o.VoiceOff))
}

// ToggleAIWar 切換「查看電腦戰役」。
func (o *Options) ToggleAIWar() string {
	o.SkipAIWar = !o.SkipAIWar
	return tf("oth.war.state", onOff(!o.SkipAIWar))
}

// ToggleCalendar 切換年號的表示方式。
func (o *Options) ToggleCalendar() string {
	if o.Calendar == Western {
		o.Calendar = ChineseEra
	} else {
		o.Calendar = Western
	}
	return tf("oth.era.state", o.Calendar.Name())
}
