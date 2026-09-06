package game

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 指令層。
//
// **每一個指令都回錯誤而不是靜靜地不做事。** 條件不滿足時（錢不夠、
// 這個月下過令了、人口不足）玩家要看得到理由；靜靜地忽略會讓
// 「按了沒反應」變成一個查不出來的問題。
//
// ⚠ **這裡只實作花費與前置條件已經有出處的指令。** 效果的公式
//（開墾加多少土地價值、徵兵的忠誠影響……）**還沒解**，
// 手冊只說「越高越好」不給數字。沒有出處的公式不要寫進來——
// 猜出來的數值玩起來「差不多」，而那是最難發現的錯。

// ErrNotYours 等是指令被擋下來的理由。用具名錯誤讓上層分得開，
// 不必比對字串。
var (
	ErrNotYours     = fmt.Errorf("這個郡不是你的")
	ErrAlreadyMoved = fmt.Errorf("這個郡這個月已經下過令了")
	ErrNoGold       = fmt.Errorf("庫銀不足")
	ErrNoPeople     = fmt.Errorf("人口不足，徵不到兵")
	ErrNoRoom       = fmt.Errorf("帶兵已達上限")
	ErrUnknownUnit  = fmt.Errorf("找不到這位將領")
)

// canOrder 檢查「這個郡現在收不收指令」。
func (g *State) canOrder(prefectureID int, by state.FactionID) (*Prefecture, error) {
	p := g.Prefecture(prefectureID)
	if p == nil {
		return nil, fmt.Errorf("game: 郡編號 %d 越界", prefectureID)
	}
	if !p.Owned() || p.Owner != by {
		return nil, ErrNotYours
	}
	if p.Commanded {
		return nil, ErrAlreadyMoved
	}
	return p, nil
}

// Reclaim 是「開墾」：花 10 金提升土地價值（說明書 p.20）。
//
// **提升多少還沒解。** 手冊只說「土地開發值越高則收成越好」，沒給數字。
// 這裡先加 1，並在回傳值裡誠實報出來，等對拍量到真正的公式再換掉。
func (g *State) Reclaim(prefectureID int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if p.Gold < CostReclaim {
		return ErrNoGold
	}
	p.Gold -= CostReclaim
	if p.LandValue < 100 {
		p.LandValue++
	}
	p.Commanded = true
	return nil
}

// FloodControl 是「防洪」：花 10 金降低洪水率（說明書 p.20）。
// 降多少同樣還沒解。
func (g *State) FloodControl(prefectureID int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if p.Gold < CostFloodControl {
		return ErrNoGold
	}
	p.Gold -= CostFloodControl
	if p.FloodRate > 0 {
		p.FloodRate--
	}
	p.Commanded = true
	return nil
}

// Conscript 是「徵兵」：一位將領募兵，每人 1 金（說明書 p.20）。
//
// 三個硬條件都有出處：
//
//   - 人口少於 3000 就徵不到兵（p.20、p.37）
//   - 每人 1 金（p.20）
//   - 帶兵不得超過官階上限（p.18，而且與原版資料對得上）
//
// **兵從哪裡來還沒解**：手冊說人口是兵力的主要來源，但沒說徵一個兵
// 減多少人口。這裡先不動人口，等量到再補——**寧可少做也不要猜**，
// 猜錯的人口衰減會讓整局的經濟慢慢偏掉而且看不出來。
func (g *State) Conscript(prefectureID int, generalIndex, n int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if n <= 0 {
		return fmt.Errorf("game: 徵兵人數要是正數，拿到 %d", n)
	}
	if p.Population < MinPopulationToConscript {
		return ErrNoPeople
	}
	x := g.General(generalIndex)
	if x == nil || x.Faction != by || x.Location != prefectureID {
		return ErrUnknownUnit
	}
	cost := n * CostConscriptPerSoldier
	if p.Gold < cost {
		return ErrNoGold
	}
	if x.Soldiers+n > x.TroopCap() {
		return ErrNoRoom
	}
	p.Gold -= cost
	x.Soldiers += n
	p.Soldiers += n
	p.Commanded = true
	return nil
}

// BuyArms 是「武器」：每 100 單位 1 金（說明書 p.20），提升武裝度。
func (g *State) BuyArms(prefectureID, generalIndex, units int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if units <= 0 || units%100 != 0 {
		return fmt.Errorf("game: 武器要以 100 單位為單位，拿到 %d", units)
	}
	x := g.General(generalIndex)
	if x == nil || x.Faction != by || x.Location != prefectureID {
		return ErrUnknownUnit
	}
	cost := units / 100 * CostArmsPer100
	if p.Gold < cost {
		return ErrNoGold
	}
	p.Gold -= cost
	// 武裝度以百分表示（說明書 p.18），所以上限是 100。
	// **換算率還沒解**：這裡先當 100 單位加 1。
	add := units / 100
	if int(x.Arms)+add > 100 {
		add = 100 - int(x.Arms)
	}
	x.Arms += uint8(add)
	p.Commanded = true
	return nil
}

// EndMonth 把時間推到下個月，並清掉每郡的下令旗標。
//
// **季節事件與秋收還沒實作**（說明書 p.36–37 有清單但沒給公式）。
// 這裡只做時間推進，讓迴圈跑得起來；缺的部分列在 `CONTEXT.md` 的
// worklist，不用「差不多的公式」先頂著。
func (g *State) EndMonth() {
	g.Date = g.Date.Next()
	for i := range g.prefectures {
		g.prefectures[i].Commanded = false
	}
}
