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
	CostConscriptPerSoldier = 1  // 徵兵，每人 1 金
	CostArmsPer100          = 1  // 武器，每 100 單位 1 金
	CostReclaim             = 10 // 開墾
	CostFloodControl        = 10 // 防洪
	CostSearch              = 5  // 尋訪
	CostRecruit             = 30 // 登用
	CostDismiss             = 10 // 撤職
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
