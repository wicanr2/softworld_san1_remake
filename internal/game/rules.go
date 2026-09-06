// Package game 是規則層：一局進行中的遊戲，以及改變它的規則。
//
// **這一層不依賴 Ebiten 也不依賴檔案格式**。輸入是 `internal/state`
// 解出來的劇本，輸出是可以被測試逐項檢查的狀態轉移。
//
// 規則的來源分兩種，每一條都標出處：
//
//   - `說明書` —— 手冊寫明的數字（`docs/reference/01-manual-20-mechanics.md`）。
//     手冊是繁中原文的權威來源，但**它寫的是玩家看得到的規則，不是實作**；
//     沒有第二個來源佐證的數字只當假設。
//   - `對拍` —— 用 dosgolem 跑原版量到的。
//
// ⚠ **沒有出處的數字不要寫進這一層。** 猜出來的公式會自洽、會通過測試、
// 而且玩起來「差不多」——那是最難發現的錯（`CLAUDE.md` §7 第 18 條）。
package game

import "github.com/wicanr2/softworld_san1_remake/internal/state"

// 資源上限（說明書 p.22）。
const (
	MaxGold = 30000
	MaxRice = 30000
)

// MinPopulationToConscript 是徵兵需要的人口下限（說明書 p.20、p.37）：
// 「少於 3000 人就無法徵兵」。
const MinPopulationToConscript = 3000

// MaxForts 是一個郡最多幾座城寨（說明書 p.21，不含城池）。
const MaxForts = 5

// MinIntelForChief 是受封軍師的謀略下限（說明書 p.23）。
const MinIntelForChief = 80

// 各項花費，單位是金（說明書 p.20–34）。
const (
	CostConscriptPerSoldier = 1   // 徵兵，每人 1 金
	CostArmsPer100          = 1   // 武器，每 100 單位 1 金
	CostReclaim             = 10  // 開墾
	CostFloodControl        = 10  // 防洪
	CostSearch              = 5   // 尋訪
	CostRecruit             = 30  // 登用
	CostDismiss             = 10  // 撤職
	CostHeadhunt            = 100 // 挖角
	CostScoutEnemy          = 10  // 查看敵軍
	MaxReward               = 100 // 賞賜上限
)

// FortCost 是建一座城寨的花費：當月物價的 100 倍（說明書 p.21）。
func FortCost(priceLevel uint8) int { return int(priceLevel) * 100 }

// TroopCap 是一個職位帶得動的最大兵力。
//
// 出處是說明書 p.18 的「官階與帶兵上限」，而且**與原版資料對得上**：
// 劇本 001 的 346 位人物零人超標，九個職位裡有八個的實際最大兵數
// 正好等於上限（謀士那一格劇本裡最高只有 200，沒有頂到）。
//
// 職位的編號是原版字串表的順序，文官與武官成對：
// 軍師·大將 3000、參軍·副將 2500、主簿·裨將 2000、謀士·牙將 1500。
func TroopCap(r state.Rank) int {
	switch r {
	case state.RankLord:
		return 5000
	case state.RankStrategist, state.RankGeneral:
		return 3000
	case state.RankStaff, state.RankViceGeneral:
		return 2500
	case state.RankClerk, state.RankSubGeneral:
		return 2000
	case state.RankAdvisor, state.RankJuniorGeneral:
		return 1500
	}
	return 0
}

// TrainGain 是「訓練兵士」一次提升多少訓練度（`L0`、`[base]`）。
//
// 從原版的碼讀出來的。共用常式在線性 `0xbd70`，對清單裡的每一位算：
//
//	mov al, es:[bx+0x2219]   ; 智
//	idiv 3
//	mov al, es:[bx+0x221a]   ; 武
//	idiv 2
//	add                      ; 智/3 + 武/2
//	idiv word [bp+6]         ; 除以呼叫端給的常數
//	add 原本的訓練度
//	上限 100
//
// **逐項截斷**：智與武各自先整數除，再相加，再除。
//
// 那個常數由勢力的 AI 等級選（`AITrainDivisor`），不是固定的
// （`docs/re/03` §1.3–1.4）。說明書只說「各將的能力影響其麾下的訓練度
// 提升」——方向對，係數是碼裡才有的。
func TrainGain(intel, war, aiLevel int) int {
	return (intel/3 + war/2) / AITrainDivisor(aiLevel)
}

// AITrainDivisor 是訓練提升的除數，由勢力的 AI 等級選（`L0`、`[base]`）。
//
// 原版的指令分派表 `0x5554` 有八個 far pointer，每一個是一個 thunk，
// 推一個常數再呼叫共用常式：
//
//	等級 0 1 2 → 5
//	等級 3 4   → 4
//	等級 5     → 3
//	等級 6 7   → 空操作（`xor ax,ax; lret`）
//
// **等級 6 與 7 到不了**：分派前 `[bp+6]` 被夾在 0–5，那兩格只是把表
// 補成 2 的冪。所以除數越小（等級越高）練得越快。
func AITrainDivisor(level int) int {
	switch {
	case level <= 2:
		return 5
	case level <= 4:
		return 4
	default:
		return 3
	}
}

// ReclaimGain 是「土地開發」一次增加多少地力（`L0`、`[base]`）。
//
// 原版的常式在線性 `0xba02`：
//
//	cmp word [bp+6], 0
//	jg  用它                       ; 量 > 0 就照用
//	mov ax,2; lcall RND            ; 否則改用 RND(2)，也就是 0 或 1
//	add es:[bx+0x49b], al          ; 地力 += 量
//	cmp es:[bx+0x49b], 100; 超過就設 100
//
// 呼叫端算的量是 `(智 − 50) / 12`（`docs/re/03` §1.4）。
// **智力低的不會讓地力下降**——常式先擋掉非正的量，改成擲 0 或 1。
// 說明書只寫「謀略越高，土地價值增加越多」。
func ReclaimGain(intel, roll int) int {
	n := (intel - 50) / 12
	if n > 0 {
		return n
	}
	return roll // RND(2)：0 或 1
}

// FloodDrop 是「洪水防治」一次降多少洪水率（`L0`、`[base]`）。
//
// 原版的常式在線性 `0xba4c`：讀州郡 offset 28（洪水率），減掉呼叫端
// 給的量，**負的夾到 0**，寫回去。呼叫端算的量是 `智 / 10`。
func FloodDrop(intel int) int { return intel / 10 }

// 尋訪人才的門檻（`L0`、`[base]`）。
//
// 原版：`門檻 = RND(65) + 30`，尋訪者的智要**大於**它才算成功
// （`0xcd38`／`0xccd2`）。所以智 95 一定成功、智 30 一定失敗。
//
// 說明書只說「負責尋訪的將領謀略越高，成功的機率越大」。
const (
	SearchIntelFloor  = 30
	SearchIntelSpread = 65
)

// Weapons／ArmsOf 是武裝度與武器數之間的換算。
//
// **武裝度是「有武器的兵佔多少百分比」**，不是武器的絕對數量。
// 原版把它存成百分比，實際運算時先還原成武器數、加減、再換回百分比——
// 所以兵力一變，武裝度就跟著動。說明書「人員損耗後，其持有的軍械也隨同失去」
// 講的就是這件事。
// ArmsPerGold 是一金買得到幾單位武器（說明書 p.20；原版 `0xc168`
// 用 `idiv 100` 把缺口換成金額，`L0`）。
const ArmsPerGold = 100

func Weapons(arms, soldiers int) int { return arms * soldiers / 100 }

func ArmsOf(weapons, soldiers int) int {
	if soldiers <= 0 {
		return 0
	}
	return clampTo(weapons*100/soldiers, 100)
}

// ArmsAfterPurchase 是買武器之後的武裝度（`L0`、`[base]`）。
//
// 原版的常式（線性 `0xc168`）用浮點算，而 MSC 的浮點模擬器把 x87 指令
// 編碼成 `INT 34h`–`3Bh` ＋ 原本的 modrm（對應 `D8`–`DF`，`3Ch` 是帶段
// 前綴的形式）。還原出來的序列是
//
//	fild 武裝度 ; fild 兵力 ; fst qword [bp-12]
//	fmulp                      ; 武裝度 × 兵力
//	fmul qword ds:[0xa5c8]     ; × 0.01
//	fiadd word [bp-2]          ; + 新增的武器
//	fdiv qword [bp-12]         ; ÷ 兵力
//	fmul qword ds:[0xa5a8]     ; × 100
//
// 兩個常數從執行期記憶體讀出來是 `0.01` 與 `100`（`TestZZDispatch`），
// 所以整段就是「換成武器數 → 加上新買的 → 換回百分比」。
// 兵力為零時武裝度歸零，那是同一支常式開頭的 `cmpw es:[bx+0x2226], 0`。
//
// ⚠ **原版的「兵力」與「新增的武器」各自的單位還沒量**（`L3`）：
// 這裡照 remake 自己的單位算，兩邊的比例對得上才有意義。
func ArmsAfterPurchase(arms, soldiers, bought int) int {
	return ArmsOf(Weapons(arms, soldiers)+bought, soldiers)
}
