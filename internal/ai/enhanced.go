package ai

import (
	"sort"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// enhanced 是 remake 自己的強化 AI。
//
// **它不宣稱與任何原版相同。** 判斷依據是 remake 已經解出來的規則
// （`docs/spec/003` 的欄位、說明書給的花費與上限），策略是我寫的。
//
// 決策是**決定性的**：同一個局面永遠得到同一串命令。理由是對拍與重現——
// 帶亂數的 AI 會讓「這一手為什麼不一樣」變成無法回答的問題。
// 要隨機性的話應該從局面推導（例如以回合數與勢力編號當種子），
// 而不是拿系統亂數。
type enhanced struct{}

func (e *enhanced) Mode() Mode    { return ModeEnhanced }
func (e *enhanced) Name() string  { return "remake 強化 AI" }
func (e *enhanced) Derived() bool { return false }

// 內政的門檻。**這些是 remake 自己的判斷，不是原版的數字。**
const (
	// floodDanger 以上就優先防洪。說明書說洪水率越低越不易罹水患，
	// 而且水災後立刻升到 100（p.36）。
	floodDanger = 60

	// landTarget 以下就開墾。土地價值影響收成與人口增長（p.21）。
	landTarget = 60

	// goldReserve 是不動用的存底，免得把庫銀花光而無法應變。
	goldReserve = 200
)

// Plan 對每一個自己的郡挑一件事做。每郡每月只能下一次令（說明書 p.17），
// 所以每個郡最多產出一個命令。
func (e *enhanced) Plan(g *game.State, f state.FactionID) []game.Order {
	var out []game.Order
	for _, id := range g.Territory(f) {
		p := g.Prefecture(id)
		if p == nil || p.Commanded {
			continue
		}
		if o := e.planOne(g, f, p); o != nil {
			out = append(out, o)
		}
	}
	// 依郡編號排序，讓輸出與遍歷順序無關。
	sort.Slice(out, func(i, j int) bool { return out[i].Prefecture() < out[j].Prefecture() })
	return out
}

func (e *enhanced) planOne(g *game.State, f state.FactionID, p *game.Prefecture) game.Order {
	spendable := p.Gold - goldReserve

	// 1. 水患優先：洪水率高的時候，收成與人口都保不住。
	if p.FloodRate >= floodDanger && spendable >= game.CostFloodControl {
		return game.FloodControlOrder{At: p.ID}
	}

	// 2. 兵力不足就募兵。判準是「這個郡的守將加起來離上限還差多少」，
	//    而不是絕對數字——上限是官階決定的（說明書 p.18）。
	if p.Population >= game.MinPopulationToConscript {
		if x, room := e.weakestGarrison(g, f, p.ID); x != nil && room > 0 {
			n := room
			if n > spendable {
				n = spendable
			}
			if n > 0 {
				return game.ConscriptOrder{At: p.ID, General: x.Index, Count: n}
			}
		}
	}

	// 3. 沒有急事就開墾。
	if p.LandValue < landTarget && spendable >= game.CostReclaim {
		return game.ReclaimOrder{At: p.ID}
	}
	return nil
}

// weakestGarrison 回傳這個郡裡「離帶兵上限最遠」的守將，以及還差多少。
//
// 挑最弱的而不是挑主事者，理由是**上限是逐人算的**：主事者滿了之後，
// 整個郡的兵力還可以靠別人往上加。
func (e *enhanced) weakestGarrison(g *game.State, f state.FactionID, id int) (*game.General, int) {
	var best *game.General
	bestRoom := 0
	for _, x := range g.Garrison(id) {
		if x.Faction != f {
			continue
		}
		room := x.TroopCap() - x.Soldiers
		if room > bestRoom {
			best, bestRoom = x, room
		}
	}
	return best, bestRoom
}
