package game

// remake 自己定的數值。
//
// ⚠ **這一檔裡的每一個常數都不是原版的數字。** 說明書寫了方向
// （「謀略越高，土地價值增加越多」）卻沒給係數；原版的公式還沒反組譯到。
// 把它們集中在一個檔案，是為了讓「哪些是還原的、哪些是我們選的」
// **一眼看得出來**，而且換掉的時候不必翻遍整個規則層。
//
// 對照表與替換條件在 `docs/design/02-remake-owned-values.md`。
//
// 命名一律 `Tune` 開頭，用到的地方也就標示出來了。
const (
	// ⚠ **下面這六個已經被原版的公式取代，規則層不再用它們**
	// （`ReclaimGain`／`FloodDrop`／`TrainGain`，都是 `L0`）。
	// 留著是為了讓 `docs/design/02` 的對照表讀得下去——那張表記的是
	// 「哪些數字曾經是 remake 自己挑的」，刪掉就看不出替換發生過。
	TuneReclaimBase  = 1
	TuneReclaimIntel = 25
	TuneFloodBase    = 1
	TuneFloodIntel   = 25
	TuneTrainBase    = 2
	TuneTrainIntel   = 25

	// ⚠ TuneReliefRice／TuneReliefLoyalty 已被原版的公式取代
	// （`L0`、`0xc8f6`）：賑民撥的是**金**不是米，增幅的上限是
	// 太守魅力 ÷ 2，見 `ReliefGain`。
	TuneReliefRice    = 500
	TuneReliefLoyalty = 3

	// TuneTransportLoss：運送錢糧的基礎損耗百分比；太守魅力越高越少
	// （說明書 p.20）。實際損耗 ＝ base × (100 − 魅力) ÷ 100。
	TuneTransportLoss = 20

	// ⚠ TuneSearchIntel 已被原版的三張表取代（`L1`、`0xcd20` 起六支）：
	// 尋訪比的是「謀略 > RND(Spread) + Floor」，而三個常數都隨 AI 等級變，
	// 見 `SearchTierFor`。留著只為讓 `docs/design/02` 的對照表讀得懂。
	TuneSearchIntel = 1

	// ⚠ TuneRecruitCharm 已被原版的公式取代（`L0`、`0xce8c`）：
	// 登用比的是說服力與難度，見 `RecruitPersuasion`。
	TuneRecruitCharm = 1

	// ⚠ TuneRewardLoyalty 已被原版的公式取代（`L0`、`0xd302`）：
	// 增幅 ＝ (RND(加成/2) ＋ 太守魅力/3 ＋ 加成) × 金 ÷ 100，
	// 見 `RewardEffect`。

	// ⚠ TuneHeadhuntBase 已被原版的公式取代（`L0`、`0x1dc0a`）：
	// 挖角比的是「招募方開的條件」對上「目標的抵抗」，見 `HeadhuntOffer`。
	TuneHeadhuntBase = 60

	// TuneNewSoldierTraining：新兵沒受過訓，加入時把部隊的訓練度
	// 拉低（說明書 p.20）——當成「新兵的訓練度是 0，全隊取加權平均」。
	TuneNewSoldierTraining = 0

	// ⚠ TuneNewSoldierArms 已被 `ArmsOf`／`Weapons` 取代（`L0`、`0xc168`）。
	// **武裝度是百分比不是絕對數量**，新兵稀釋是換算的結果，不是另一條規則。
	TuneNewSoldierArms = 0
)

// TreasureEffect 是寶物加給能力的**下界**（`L1`、`[base]`，
// 分派表 `0x56b4` 底下四支：`0xd962`／`0xdac0`／`0xdc1e`／`0xdd94`）。
//
// 兵書 +2 謀略、寶刀 +3 戰力、美女 +5 魅力、駿馬 +2 戰力 +3 魅力——
// **說明書 p.24 寫的就是這一組，而它是下界不是定值**：原版在每一項上面
// 再加一次 `RND(2)`（`add $底,%al` 之前的 `RND(2)`）。186 次對拍
// 逐次落在 `底`–`底+1` 裡（`docs/playtest/02`）。
//
// 忠誠的增幅另見 `TreasureLoyaltyGain`。
func TreasureEffect(t Treasure) (intel, war, charm int) {
	switch t {
	case TreasureBook:
		return 2, 0, 0
	case TreasureBlade:
		return 0, 3, 0
	case TreasureBeauty:
		return 0, 0, 5
	case TreasureHorse:
		return 0, 2, 3
	}
	return 0, 0, 0
}

// TreasureCap 是靠賞賜能把能力提到的上限（說明書 p.24：90 點）。
const TreasureCap = 90

// TreasureLoyaltySpread 是賞賜物品之後忠誠那一擲的上限（`L1`）。
//
// 三種寶物是 `RND(30) + 提升後的能力 ÷ 2`，**美女是 `RND(50) + 50`**
// ——沒有能力那一項，而且底就有 50。
const (
	TreasureLoyaltySpread       = 30
	TreasureBeautyLoyaltySpread = 50
	TreasureBeautyLoyaltyFloor  = 50
)

// TreasureLoyaltyGain 是賞賜一件寶物換到的忠誠。
//
//	roll ＝ RND(30)，美女那一支是 RND(50)
//	ability ＝ **提升之後**的能力值
func TreasureLoyaltyGain(t Treasure, ability, roll int) int {
	if t == TreasureBeauty {
		return TreasureBeautyLoyaltyFloor + roll
	}
	return ability/2 + roll
}
