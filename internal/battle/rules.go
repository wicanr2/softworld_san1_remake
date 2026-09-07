package battle

import "github.com/wicanr2/softworld_san1_remake/internal/state"

// 版本與難度決定的戰役規則（`docs/spec/004` §5、`docs/re/05` §8.1）。
//
// 原版與加強版在戰役層有兩處差異，兩處都是從加強版自己的碼讀出來的：
//
//	0x2296b  難度 ≥ 11 時，「守方統帥全滅 → 攻方勝」那一段被跳過
//	0x229ba  加強版獨有的函式：總兵數為 0 者敗（**不看難度**）
//
// 兩者是同一條規則的兩面：打光守方的統帥不再算贏，但打光守方的兵還是算。

// PlusHardDifficulty 是加強版「難度 11–20」那一段的下界（`0x2296b` 比 11）。
const PlusHardDifficulty = 11

// Rules 是一場戰役適用的版本規則。
//
// **欄位用「差異」命名，不用「行為」命名**，這樣零值就是原版：
// 沒設 Rules 的呼叫端拿到的是原版規則，不是「所有規則都關掉」。
// 反過來寫（`DefenderCommanderLossEnds bool`）的零值會是加強版難度
// 11–20 的行為，而那在測試裡看起來只是「戰役沒結束」。
type Rules struct {
	// DefenderCommanderLossIgnored：守方（主守＋助守）的統帥都不在場上
	// 時，**不**判攻方獲勝。
	//
	// 原版一律判；加強版難度 11–20 不判，戰役繼續打到卅天期滿，由城池
	// 歸屬決定（加強版 README 第 5 條，`0x2296b` `L0`）。
	DefenderCommanderLossIgnored bool
}

// 加強版另外多一個函式：**總兵數為 0 者敗**（`0x229ba`，原版沒有對應的
// 碼，README 第 6 條）。這裡**沒有對應的旗標**，因為 remake 的「全滅」
// 條件就是它——一支部隊的兵歸零就不算還在場上。原版的同樣局面由統帥
// 條件蓋掉（兵打光了統帥也就不在了），所以那兩個 case 只有在
// `DefenderCommanderLossIgnored` 開著時才走得到，也就是加強版難度 11–20。
//
// 加一個永遠不會生效的旗標比不加更糟：它看起來像「這條規則實作了」。

// RulesFor 依版本與難度給出規則。
//
// 版本空字串當原版。**難度越界不特別處理**：上限由 `state.Edition`
// 在開局時擋掉（`internal/game` `New`），這裡再擋一次只會多一份會分家的檢查。
func RulesFor(ed state.Edition, difficulty int) Rules {
	var r Rules
	if ed == state.EditionPlus && difficulty >= PlusHardDifficulty {
		r.DefenderCommanderLossIgnored = true
	}
	return r
}
