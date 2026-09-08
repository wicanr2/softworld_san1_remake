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
type enhanced struct{}

func (e *enhanced) Mode() Mode    { return ModeEnhanced }
func (e *enhanced) Name() string  { return "remake 強化 AI" }
func (e *enhanced) Derived() bool { return false }

// Coverage 對 remake 自己的 AI 沒有意義——它不是在還原什麼。
func (e *enhanced) Coverage() (int, int) { return 0, 0 }

// 門檻。**這些是 remake 自己的判斷，不是原版的數字。**
const (
	floodDanger = 60  // 洪水率到這裡就優先防洪
	landTarget  = 70  // 土地價值低於這裡就開墾
	loyaltyLow  = 70  // 民眾忠誠低於這裡就賑民
	goldReserve = 200 // 不動用的存底
	riceReserve = 800 // 不動用的存糧

	// TuneEnhancedRelief 是強化 AI 一次賑民撥出去的金。**remake 自選**：
	// 原版那一邊的量是分派器算出來的回合預算（`docs/mechanics/70-ai`
	// §2.14），強化 AI 沒有那條預算線。
	TuneEnhancedRelief = 100

	// attackEdge 是「戰力要領先多少倍才出兵」。守方有地利加成，
	// 平手出兵是送死。
	attackEdgeNum, attackEdgeDen = 3, 2

	// sellRiceAbove 是米多到這裡就賣一些換金。
	// conscriptShare：一次最多抽剩餘人口的幾分之一。
	conscriptShare = 20

	sellRiceAbove = 5000
	sellRiceBatch = 1000
)

// PlanPrefecture 只替一個郡挑一件事做。`enhanced` 不看 AI 等級
// （它是 remake 自己的 AI，不是還原），所以 `level` 收下不用。
func (e *enhanced) PlanPrefecture(g *game.State, f state.FactionID,
	prefectureID, level int) []game.Order {
	p := g.Prefecture(prefectureID)
	if p == nil || p.Commanded {
		return nil
	}
	if o := e.planOne(g, f, p); o != nil {
		return []game.Order{o}
	}
	return nil
}

// ActPrefecture 是逐郡的執行版。
func (e *enhanced) ActPrefecture(g *game.State, f state.FactionID,
	prefectureID, level int) ([]game.Order, int, error) {
	out := e.PlanPrefecture(g, f, prefectureID, level)
	n, err := g.ApplyAll(out, f)
	return out, n, err
}

// Act 是 `enhanced` 的執行版。它**每郡只下一道令**，所以「發一道套一道」
// 與「排完再一次套上」在同一個郡裡沒有差別——`enhanced` 是創作不是還原，
// 這裡照 `Plan` ＋ `ApplyAll` 走就好（`Brain.Act`）。
func (e *enhanced) Act(g *game.State, f state.FactionID) ([]game.Order, int, error) {
	out := e.Plan(g, f)
	n, err := g.ApplyAll(out, f)
	return out, n, err
}

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
	sort.Slice(out, func(i, j int) bool { return out[i].Prefecture() < out[j].Prefecture() })
	return out
}

// planOne 是單一郡的優先序。順序本身就是策略。
func (e *enhanced) planOne(g *game.State, f state.FactionID, p *game.Prefecture) game.Order {
	spendable := p.Gold - goldReserve

	// 1. 水患優先：洪水率高的時候收成與人口都保不住。
	if p.FloodRate >= floodDanger && spendable >= game.CostFloodControl {
		if x := e.wisest(g, f, p.ID); x != nil {
			return game.FloodControlOrder{At: p.ID, General: x.Index}
		}
	}

	// 2. 打得贏的鄰郡就打——擴張是唯一的勝利路徑。
	if o := e.attack(g, f, p); o != nil {
		return o
	}

	// 3. 民怨高就賑民：天災多因人怨引起（說明書 p.36）。
	// **賑民付的是金**（`game.Relief`，原版 `0xc8f6`）；撥多少由這裡決定，
	// 是 remake 自己挑的——原版的量來自分派器算的回合預算。
	if p.PublicLoyalty < loyaltyLow && spendable >= TuneEnhancedRelief {
		return game.ReliefOrder{At: p.ID, Gold: TuneEnhancedRelief}
	}

	// 4. 本地有在野人才就登用。
	if spendable >= game.CostRecruit {
		if t := e.freeTalent(g, p.ID); t != nil {
			return game.RecruitOrder{At: p.ID, Target: t.Index}
		}
	}

	// 5. 兵力不足就募兵。
	//
	// ⚠ **一次抽多少要有節制。** 徵兵是 1:1 減人口，抽到下限的話
	// 這個郡的生產力就毀了，而下一次還會再抽——整個世界會慢慢空掉。
	// 一次最多抽剩餘人口（扣掉下限）的 conscriptShare 分之一。
	if p.Population >= game.MinPopulationToConscript*2 {
		if x, room := e.weakestGarrison(g, f, p.ID); x != nil && room > 0 {
			n := room
			if n > spendable {
				n = spendable
			}
			if max := (p.Population - game.MinPopulationToConscript) / conscriptShare; n > max {
				n = max
			}
			if n > 0 {
				return game.ConscriptOrder{At: p.ID, General: x.Index, Count: n}
			}
		}
	}

	// 6. 兵多但訓練差就練兵。不花錢，所以放在募兵之後。
	if x := e.leastTrained(g, f, p.ID); x != nil && x.Training < 80 && x.Soldiers > 0 {
		return game.TrainOrder{At: p.ID}
	}

	// 7. 米太多就賣一些。
	if p.Rice > sellRiceAbove && p.Gold < game.MaxGold-1000 {
		return game.SellRiceOrder{At: p.ID, Units: sellRiceBatch}
	}

	// 8. 沒有急事就開墾。
	if p.LandValue < landTarget && spendable >= game.CostReclaim {
		if x := e.wisest(g, f, p.ID); x != nil {
			return game.ReclaimOrder{At: p.ID, General: x.Index}
		}
	}
	return nil
}

// attack 挑一個打得贏的鄰郡。
//
// 判準是**戰力比**而不是兵數比：守方有城池與城寨加成，
// 而且訓練度與武裝度的差距可以很大。
func (e *enhanced) attack(g *game.State, f state.FactionID, p *game.Prefecture) game.Order {
	var force []int
	mine := 0
	var keep *game.General
	for _, x := range g.Garrison(p.ID) {
		if x.Faction != f {
			continue
		}
		// 留一位治理——傾巢而出會被規則層擋下來。
		if keep == nil {
			keep = x
			continue
		}
		force = append(force, x.Index)
		mine += game.Power(x)
	}
	if len(force) == 0 {
		return nil
	}
	best, bestGain := 0, 0
	for _, n := range p.Neighbours {
		q := g.Prefecture(n)
		if q == nil || q.Owner == f {
			continue
		}
		theirs := g.DefencePower(n)
		if mine*attackEdgeDen <= theirs*attackEdgeNum {
			continue
		}
		// 打分：優先拿人口多、開發好的郡。
		gain := q.Population/1000 + int(q.LandValue)
		if gain > bestGain {
			best, bestGain = n, gain
		}
	}
	if best == 0 {
		return nil
	}
	return game.AttackOrder{At: p.ID, To: best, Force: force}
}

func (e *enhanced) wisest(g *game.State, f state.FactionID, id int) *game.General {
	var best *game.General
	for _, x := range g.Garrison(id) {
		if x.Faction != f {
			continue
		}
		if best == nil || x.Intel > best.Intel {
			best = x
		}
	}
	return best
}

func (e *enhanced) leastTrained(g *game.State, f state.FactionID, id int) *game.General {
	var best *game.General
	for _, x := range g.Garrison(id) {
		if x.Faction != f || x.Soldiers == 0 {
			continue
		}
		if best == nil || x.Training < best.Training {
			best = x
		}
	}
	return best
}

func (e *enhanced) freeTalent(g *game.State, id int) *game.General {
	var best *game.General
	for _, x := range g.Free(id) {
		if best == nil || x.War+x.Intel > best.War+best.Intel {
			best = x
		}
	}
	return best
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
