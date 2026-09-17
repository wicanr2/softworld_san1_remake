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
// ⚠ 這一檔曾經有二十來個常數；被原版的公式取代的那些已經刪掉，
// 「哪些數字曾經是 remake 自己挑的、被什麼取代」記在 `docs/design/02`
// 的替換表裡（含當初的值），不靠留著死常數來保存歷史。
// 規則層現在**沒有**任何 `Tune` 常數了：最後一個 `TuneTransportLoss`
// 也在 2026-09-15 換成原版的 `TransportArrives`（Issue #21）。
// 要加新的估計值時照舊用 `Tune` 前綴、登記到 `docs/design/02`。

// TreasureEffect 是寶物加給能力的**下界**（`L1`、`[base]`，
// 分派表 `0x56b4` 底下四支：`0xd962`／`0xdac0`／`0xdc1e`／`0xdd94`）。
//
// 兵書 +2 謀略、寶刀 +3 戰力、美女 +5 魅力、駿馬 +2 戰力 +3 魅力——
// **說明書 p.24 寫的就是這一組，而它是下界不是定值**：原版在每一項上面
// 再加一次 `RND(2)`（`add $底,%al` 之前的 `RND(2)`）。186 次對拍
// 逐次落在 `底`–`底+1` 裡（`docs/playtest/02`）。
//
// **只管電腦諸侯那一條。** 玩家的賞賜物品（`0x1d118`，`GiftRound.Gift`）
// 加的就是這組底、沒有那一擲，忠誠也看截斷前的能力（`docs/mechanics/20` §5.3）。
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
