package battle

// remake 自己定的數值。命名一律 `Tune` 開頭，
// `grep -rn Tune internal/battle` 就是全部的清單（`docs/design/02`）。
//
// **手冊給了方向與門檻，沒給係數。** 有出處的數字不在這裡：
// 六種計謀的智力門檻與費用、弓箭次數公式、地形效應的**方向**、
// 三十天的判定、休息 +2 移動力——那些都直接寫在用到的地方並標頁碼。

const (
	// 移動力（說明書 p.29–30 只說「來源是訓練度和兵種」「全副武裝稍減」）。

	// TuneRestMove 是休息增加的移動力。**這個有出處**：
	// 手冊 p.29、p.30 兩處都寫「每休息一次可增加移動力 2」。
	TuneRestMove = 2

	// 攻擊的傷害係數。手冊列了影響戰力的因素（訓練度、武裝度、兵數、
	// 地形、兵種、有無用計，p.31）但沒給公式。

	// TuneArrowDamage 是一次弓箭相對白刃相接的殺傷百分比。
	// 手冊只給了**次數**公式（`Unit.Arrows`），沒給單次的殺傷。
	TuneArrowDamage = 6

	// 計謀的殺傷（百分比）。手冊給的是**地形差異的排序**
	// （火攻：樹林最強 ＞ 平原沙漠 ＞ 山丘關寨城池 ＞ 水上最輕；
	// 水淹：樹林最強 ＞ 平原沙漠城池 ＞ 山上關寨 ＞ 水上最輕），
	// 幅度是 remake 選的。
	//（說明書 p.30：「若拒絕挑戰，麾下士兵將有部份逃跑」）。

	// TuneDeathBattleEdge 是自動作戰敢打死戰的攻防比門檻（百分比）。
	// 手冊只說死戰是「一決生死的激戰」，沒說什麼時候該用。
	TuneDeathBattleEdge = 140

	// TuneRetreatShare 是自動作戰決定退兵的兵力門檻（剩下開戰時的百分之幾）。
	TuneRetreatShare = 30

	// TuneDuelWarEdge 是自動作戰敢叫陣的戰力差。
	//
	// **要落在原版接受判定的窗裡**：對方接不接受看
	// `RND(10) + 對方戰力 − 5 > 我方戰力`（`DuelAccepted`），
	// 也就是我方最多強過對方四點還有機會被接受。差距開太大的話
	// 叫陣一定被拒，整條單挑就永遠打不起來。
	TuneDuelWarEdge = 3

	// TuneStratagemRange 是自動作戰考慮用計的距離上限（格）。
	TuneStratagemRange = 3
)

// BattleDays 是一場戰役最多打幾天。**這個有出處**：
// 手冊 p.35「守方能堅持抗戰滿卅天，且城池未被奪去就算衛郡成功」。
// BattleDays 是一場戰役打幾天。原版第 1 天開始（`0x20253`），
// **滿 30 就判勝負**（`0x250f6`：`天數 < 30` 才繼續）。
const BattleDays = 30

// RiceForCampaign 是這麼多兵打滿三十天要多少米。
//
// 原版出兵時會把這個數字算給玩家看：`30日須耗用%d米`
// （`AA.EXE` `0x4f60c`，`docs/re/04` §5）。
//
// **與 `EndDay` 的每日耗用出自同一條算式**——兩邊各寫一次的話，
// 畫面上說夠、打起來卻餓死，而那要打完三十天才發現。
func RiceForCampaign(soldiers int) int { return soldiers / 100 * BattleDays }
