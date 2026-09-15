package battle

import "github.com/wicanr2/softworld_san1_remake/internal/state"

// 版本與難度決定的戰役規則（`docs/spec/004` §5、`docs/re/05` §8.1）。
//
// 原版與加強版在戰役層的差異全部是從加強版自己的碼讀出來的
//（`docs/re/05` §8、`docs/mechanics/90` §6.3／§6.7）：
//
//	0x2296b  難度 ≥ 11 時，「守方統帥全滅 → 攻方勝」那一段被跳過
//	0x229ba  加強版獨有的函式：總兵數為 0 者敗（**不看難度**）
//	DS:0x82e4／0x8304  交戰結算的攻／守地形表，淺水、深水、城池、關寨四格不同
//	DS:0xaba0          弓箭殺傷的尺度常數 5e-5（原版 `DS:0xa98e` ＝ 1e-4）
//	0x24703            綜合能力三項相加的順序：(武裝 ＋ 戰力) ＋ 訓練 × 0.05
//
// 前兩條是同一條規則的兩面：打光守方的統帥不再算贏，但打光守方的兵還是算。
// 後三條不看難度，加強版一律如此。電腦部隊九支判斷式的差異不在這裡，
// 那是 `AI`（`autobase.go`，`AIPlus`）。

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

	// MeleeTerrainPlus：交戰結算查加強版的兩張地形表（`DS:0x82e4` 攻／
	// `DS:0x8304` 守，`L0`）——淺水 20／18、深水 15／15、城池 35／45、
	// 關寨 30／35（原版 15／20、10／10、40／50、30／40）。
	MeleeTerrainPlus bool

	// ArrowHalfScale：弓箭殺傷的尺度常數是 5e-5（`DS:0xaba0`），原版的一半。
	ArrowHalfScale bool

	// QualityAddsWarFirst：每天重算綜合能力時三項的相加順序是
	// (武裝 × 5 ÷ 20 ＋ 戰力 × 14 ÷ 20) ＋ 訓練 × 0.05（`0x24703`），原版是
	// (武裝 ＋ 訓練 × 0.05) ＋ 戰力（`0x27045`）。整數部分先加完才加那個
	// 小數，x87 只捨入一次而不是兩次——差的是最後一位。
	QualityAddsWarFirst bool
}

// 加強版交戰結算的兩張地形表（`DS:0x82e4` 攻／`DS:0x8304` 守，`L0`）。
var (
	meleeAttackPlus = [terrainCount]int{
		Hill: 30, Shallow: 20, Deep: 15, City: 35,
		Fort: 30, Plain: 25, Forest: 20, Desert: 20, Mountain: 0,
	}
	meleeDefendPlus = [terrainCount]int{
		Hill: 30, Shallow: 18, Deep: 15, City: 45,
		Fort: 35, Plain: 20, Forest: 25, Desert: 20, Mountain: 0,
	}
)

// ArrowScalePlus 是加強版弓箭殺傷的尺度常數（`DS:0xaba0`）。
const ArrowScalePlus = 5e-5

// meleeAttackValue／meleeDefendValue 是這場戰役交戰結算用的地形值。
func (b *Battle) meleeAttackValue(t Terrain) int {
	if b.Rules.MeleeTerrainPlus {
		return meleeAttackPlus[t]
	}
	return meleeAttack[t]
}

func (b *Battle) meleeDefendValue(t Terrain) int {
	if b.Rules.MeleeTerrainPlus {
		return meleeDefendPlus[t]
	}
	return meleeDefend[t]
}

// arrowScale 是這場戰役弓箭殺傷的尺度常數。
func (b *Battle) arrowScale() float64 {
	if b.Rules.ArrowHalfScale {
		return ArrowScalePlus
	}
	return MeleeDefendScale
}

// RefreshQuality 照這場戰役的版本重算一支部隊的綜合能力（`Unit.RefreshQuality`
// 是原版的加法順序）。
func (b *Battle) RefreshQuality(u *Unit) { u.refreshQuality(b.Rules.QualityAddsWarFirst) }

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
	if ed != state.EditionPlus {
		return r
	}
	r.MeleeTerrainPlus = true
	r.ArrowHalfScale = true
	r.QualityAddsWarFirst = true
	if difficulty >= PlusHardDifficulty {
		r.DefenderCommanderLossIgnored = true
	}
	return r
}
