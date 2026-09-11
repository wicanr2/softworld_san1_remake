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
type enhanced struct {
	// garrisonRatio 是「守備戰力要有鄰郡最強敵人的百分之幾」。
	// 0 表示用預設 `TuneGarrisonRatio`。
	//
	// 做成欄位是為了**掃得動**：這個數字直接決定「擴張」與「守得住」
	// 的平衡，而那個平衡只有跑過才知道（`TestZZGarrisonRatioSweep`）。
	garrisonRatio int
}

// NewEnhanced 開一個強化 AI。garrisonRatio ≤ 0 表示用預設
// （`TuneGarrisonRatio`）。
//
// 會收這個參數是因為它**要掃**：守備門檻直接決定「擴張」與「守得住」
// 的平衡，而那個平衡只有跑過才知道。
func NewEnhanced(garrisonRatio int) Brain {
	return &enhanced{garrisonRatio: garrisonRatio}
}

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

	// conscriptUrgentShare 是**守備不足時**一次抽剩餘人口的幾分之一。
	// 抽得比平時多，但仍然有節制：抽到人口下限這個郡的生產力就毀了。
	conscriptUrgentShare = 8

	// TuneGarrisonRatio 是「守備戰力要有鄰郡最強敵人的百分之幾」。
	//
	// **remake 自選。** 原版那一邊沒有這個概念——它的十八張表裡沒有
	// 「守不守得住」的判斷，出兵那一張只比自己與目標的兵力
	// （`docs/mechanics/70-ai` §2.13.6）。
	// 強化 AI 要的是「打得下也守得住」，所以多這一道。
	//
	// **60 是掃出來的**（`TestZZGarrisonRatioSweep`，劇本 001 全電腦
	// 三十六個月）：門檻越高兵留得越多、打得越少，而 60% 那一格
	// 兵力幾乎不比 100% 少（149,155 vs 151,455）而擴張多一半
	// （最大 12 郡 vs 8 郡）。守方本來就吃地利加成，不必一比一。
	TuneGarrisonRatio = 60

	// maxOrdersPerPrefecture 是一個郡一個月最多下幾道令。
	//
	// **電腦不受「每郡每月一道令」管**（`docs/mechanics/70-ai` §2.12，
	// `L1`）：原版的分派器對每個電腦的郡把十八張表全部跑一遍，量到
	// 每郡每月九次。強化 AI 先前自己只下一道——那不是規則，是自我設限，
	// 而代價是「要嘛打仗要嘛補兵，永遠二選一」。
	maxOrdersPerPrefecture = 8
)

// TraceDraws 對 `enhanced` 沒有意義（它不是還原），收下不用。
func (e *enhanced) TraceDraws(map[string]int) {}

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

// ActPrefecture 是逐郡的執行版：**發一道套一道**。
//
// 電腦不受「每郡每月一道令」管（`docs/mechanics/70-ai` §2.12），
// 所以這裡一直問到沒事可做為止——先前只下一道，結果是「要嘛打仗
// 要嘛補兵，永遠二選一」，打下來的郡守不住。
//
// **一定要套上去再問下一道**：`planOne` 讀的是盤面現況，不套用就會
// 一直挑中同一件事。
func (e *enhanced) ActPrefecture(g *game.State, f state.FactionID,
	prefectureID, level int) ([]game.Order, int, error) {
	var out []game.Order
	for i := 0; i < maxOrdersPerPrefecture; i++ {
		p := g.Prefecture(prefectureID)
		if p == nil {
			break
		}
		// 自治的郡屬於玩家，玩家自己下過令就不再插手。
		if p.Commanded && !g.Faction(f).ByComputer {
			break
		}
		o := e.planOne(g, f, p)
		if o == nil {
			break
		}
		if err := o.Apply(g, f); err != nil {
			// 被擋下來就停：同一道再試一次還是會被擋。
			return out, len(out), err
		}
		out = append(out, o)
		// 出兵之後這個郡的守軍全變了，重新評估。
		if _, ok := o.(game.AttackOrder); ok {
			break
		}
	}
	return out, len(out), nil
}

// Act 是 `enhanced` 的執行版：逐郡走 `ActPrefecture`（發一道套一道）。
func (e *enhanced) Act(g *game.State, f state.FactionID) ([]game.Order, int, error) {
	var out []game.Order
	for _, id := range g.Territory(f) {
		got, _, err := e.ActPrefecture(g, f, id, 0)
		out = append(out, got...)
		if err != nil {
			return out, len(out), err
		}
	}
	return out, len(out), nil
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
			// 電腦那一條不收錢（`0xbd39`），`spendable` 的閘門只是保守。
			return game.FloodControlOrder{At: p.ID, General: x.Index, Auto: true}
		}
	}

	// 2. **守不住就先補兵。**
	//
	// 沒有足夠的兵力防守，再多的土地與金子都是白搭：郡易主的時候
	// 錢糧與將領一起換主人。所以這一條排在出兵**前面**——先前的順序
	// 是「打得贏就打」，結果是打下來一片守不住的地。
	if need := e.garrisonNeed(g, f, p); e.garrisonPower(g, f, p.ID) < need {
		if o := e.conscript(g, f, p, spendable, conscriptUrgentShare); o != nil {
			return o
		}
	}

	// 3. 打得贏的鄰郡就打——擴張是唯一的勝利路徑。
	//    出兵會把守軍帶走，所以 `attack` 自己會留下守得住的兵力。
	if o := e.attack(g, f, p); o != nil {
		return o
	}

	// 4. 民怨高就賑民：天災多因人怨引起（說明書 p.36）。
	// **賑民付的是金**（`game.Relief`，原版 `0xc8f6`）；撥多少由這裡決定，
	// 是 remake 自己挑的——原版的量來自分派器算的回合預算。
	if p.PublicLoyalty < loyaltyLow && spendable >= TuneEnhancedRelief {
		return game.ReliefOrder{At: p.ID, Gold: TuneEnhancedRelief}
	}

	// 5. 本地有在野人才就登用。
	if spendable >= game.CostRecruit {
		if t := e.freeTalent(g, p.ID); t != nil {
			return game.RecruitOrder{At: p.ID, Target: t.Index}
		}
	}

	// 6. 兵力補到上限。
	if o := e.conscript(g, f, p, spendable, conscriptShare); o != nil {
		return o
	}

	// 7. 兵多但訓練差就練兵。不花錢，所以放在募兵之後。
	if x := e.leastTrained(g, f, p.ID); x != nil && x.Training < 80 && x.Soldiers > 0 {
		return game.TrainOrder{At: p.ID}
	}

	// 8. 米太多就賣一些。
	if p.Rice > sellRiceAbove && p.Gold < game.MaxGold-1000 {
		return game.SellRiceOrder{At: p.ID, Units: sellRiceBatch}
	}

	// 9. 沒有急事就開墾。
	if p.LandValue < landTarget && spendable >= game.CostReclaim {
		if x := e.wisest(g, f, p.ID); x != nil {
			// 同上（`0xba02`）。
			return game.ReclaimOrder{At: p.ID, General: x.Index, Auto: true}
		}
	}
	return nil
}

// conscript 是募兵。share 是「一次最多抽剩餘人口的幾分之一」。
//
// ⚠ **一次抽多少要有節制。** 徵兵是 1:1 減人口，抽到下限的話這個郡的
// 生產力就毀了，而下一次還會再抽——整個世界會慢慢空掉。
func (e *enhanced) conscript(g *game.State, f state.FactionID,
	p *game.Prefecture, spendable, share int) game.Order {
	if p.Population < game.MinPopulationToConscript*2 || share <= 0 {
		return nil
	}
	x, room := e.weakestGarrison(g, f, p.ID)
	if x == nil || room <= 0 {
		return nil
	}
	n := room
	if n > spendable {
		n = spendable
	}
	if max := (p.Population - game.MinPopulationToConscript) / share; n > max {
		n = max
	}
	if n <= 0 {
		return nil
	}
	return game.ConscriptOrder{At: p.ID, General: x.Index, Count: n}
}

// garrisonPower 是自己在這個郡的戰力總和。
func (e *enhanced) garrisonPower(g *game.State, f state.FactionID, id int) int {
	n := 0
	for _, x := range g.Garrison(id) {
		if x.Faction == f {
			n += game.Power(x)
		}
	}
	return n
}

// garrisonNeed 是這個郡該留多少戰力。
//
// **對得起隔壁最強的那一個**：鄰郡裡敵方守備戰力的最大值，乘上
// `TuneGarrisonRatio`。沒有敵鄰就不需要守備——內地的郡把兵留著是浪費。
func (e *enhanced) garrisonNeed(g *game.State, f state.FactionID, p *game.Prefecture) int {
	worst := 0
	for _, n := range p.Neighbours {
		q := g.Prefecture(n)
		if q == nil || !q.Owned() || q.Owner == f {
			continue
		}
		if v := g.DefencePower(n); v > worst {
			worst = v
		}
	}
	ratio := e.garrisonRatio
	if ratio <= 0 {
		ratio = TuneGarrisonRatio
	}
	return worst * ratio / 100
}

// attack 挑一個打得贏的鄰郡。
//
// 判準是**戰力比**而不是兵數比：守方有城池與城寨加成，
// 而且訓練度與武裝度的差距可以很大。
//
// **出兵要留得住家**：從最強的開始編隊，但留下來的戰力不能低於
// `garrisonNeed`——先前是「除了一位全部帶走」，於是打下新郡的同時
// 老家空了，下一個月換別人來拿。
func (e *enhanced) attack(g *game.State, f state.FactionID, p *game.Prefecture) game.Order {
	var mine []*game.General
	total := 0
	for _, x := range g.Garrison(p.ID) {
		if x.Faction != f {
			continue
		}
		mine = append(mine, x)
		total += game.Power(x)
	}
	if len(mine) < 2 {
		return nil
	}
	// 強的先出征：留下來的是守家的，守家吃地利加成。
	sort.Slice(mine, func(i, j int) bool {
		return game.Power(mine[i]) > game.Power(mine[j])
	})
	need := e.garrisonNeed(g, f, p)
	var force []int
	taken := 0
	for _, x := range mine {
		// 至少留一位治理——傾巢而出會被規則層擋下來。
		if len(mine)-len(force) <= 1 {
			break
		}
		if total-taken-game.Power(x) < need {
			break
		}
		force = append(force, x.Index)
		taken += game.Power(x)
	}
	if len(force) == 0 {
		return nil
	}
	mineForce := taken
	best, bestGain := 0, 0
	for _, n := range p.Neighbours {
		q := g.Prefecture(n)
		if q == nil || q.Owner == f {
			continue
		}
		theirs := g.DefencePower(n)
		if mineForce*attackEdgeDen <= theirs*attackEdgeNum {
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
