package battle

// remake 自己定的數值。命名一律 `Tune` 開頭，
// `grep -rn Tune internal/battle` 就是全部的清單（`docs/design/02`）。
//
// **手冊給了方向與門檻，沒給係數。** 有出處的數字不在這裡：
// 六種計謀的智力門檻與費用、弓箭次數公式、地形效應的**方向**、
// 三十天的判定、休息 +2 移動力——那些都直接寫在用到的地方並標頁碼。

const (
	// 移動力（說明書 p.29–30 只說「來源是訓練度和兵種」「全副武裝稍減」）。
	TuneMoveBase        = 4
	TuneMoveTraining    = 25
	TuneMoveArmsPenalty = 50
	TuneMoveMin         = 2

	// TuneRestMove 是休息增加的移動力。**這個有出處**：
	// 手冊 p.29、p.30 兩處都寫「每休息一次可增加移動力 2」。
	TuneRestMove = 2

	// 攻擊的傷害係數。手冊列了影響戰力的因素（訓練度、武裝度、兵數、
	// 地形、兵種、有無用計，p.31）但沒給公式。
	TuneHitTraining = 60
	TuneHitArms     = 40
	TuneHitWar      = 50
	TuneHitBase     = 12 // 基礎傷害百分比

	// TuneEnragedPenalty 是中了誘敵之後攻擊力下降的百分比。
	TuneEnragedPenalty = 30

	// TuneArrowDamage 是一次弓箭的傷害百分比。
	TuneArrowDamage = 6

	// 計謀的殺傷（百分比）。手冊給的是**地形差異的排序**
	// （火攻：樹林最強 ＞ 平原沙漠 ＞ 山丘關寨城池 ＞ 水上最輕；
	// 水淹：樹林最強 ＞ 平原沙漠城池 ＞ 山上關寨 ＞ 水上最輕），
	// 幅度是 remake 選的。
	TuneFireBase   = 30
	TuneFloodBase  = 30
	TuneBurnLoss   = 40 // 燒糧減少的金米百分比
	TuneSiegeBonus = 25 // 圍攻時每一支參與部隊的攻擊力加成

	// TuneTrapDays 是中陷阱之後不能活動的天數。**這個有出處**：
	// 手冊 p.33 寫「中計的部隊在九日內無法活動」。
	TuneTrapDays = 9

	// TuneDuelDamage 是單挑一回合對體能的傷害基準。
	TuneDuelDamage = 12

	// TuneRefuseDuelLoss 是拒絕單挑時逃跑的士兵百分比
	//（說明書 p.30：「若拒絕挑戰，麾下士兵將有部份逃跑」）。
	TuneRefuseDuelLoss = 10

	// TuneCaptureOnDuel 是單挑落敗被擒（而不是被斬）的機率。
	TuneCaptureOnDuel = 60

	// TuneDeathBattleEdge 是自動作戰敢打死戰的攻防比門檻（百分比）。
	// 手冊只說死戰是「一決生死的激戰」，沒說什麼時候該用。
	TuneDeathBattleEdge = 140

	// TuneRetreatShare 是自動作戰決定退兵的兵力門檻（剩下開戰時的百分之幾）。
	TuneRetreatShare = 30

	// TuneDuelWarEdge 是自動作戰敢叫陣的戰力差。
	TuneDuelWarEdge = 20

	// TuneStratagemRange 是自動作戰考慮用計的距離上限（格）。
	TuneStratagemRange = 3
)

// BattleDays 是一場戰役最多打幾天。**這個有出處**：
// 手冊 p.35「守方能堅持抗戰滿卅天，且城池未被奪去就算衛郡成功」。
const BattleDays = 30
