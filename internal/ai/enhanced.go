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
func (e *enhanced) Name() string  { return ModeName(ModeEnhanced) }
func (e *enhanced) Derived() bool { return false }

// Coverage 對 remake 自己的 AI 沒有意義——它不是在還原什麼。
func (e *enhanced) Coverage() (int, int) { return 0, 0 }

// 門檻。**這些是 remake 自己的判斷，不是原版的數字。**
const (
	floodDanger = 60  // 洪水率到這裡就優先防洪
	landTarget  = 90  // 土地價值低於這裡就開墾
	loyaltyLow  = 70  // 民眾忠誠低於這裡就賑民
	goldReserve = 200 // 不動用的存底
	riceReserve = 800 // 不動用的存糧

	// 底下三個是**收入那一段**的目標值。急難那一段（`floodDanger`、
	// `loyaltyLow`）問的是「會不會出事」，這一段問的是「秋天收多少」。
	//
	// 槓桿全部來自秋收公式（`game.HarvestGold`／`HarvestRice`，`L0`）：
	//
	//	金 ＝ (太守魅力 + 土地價值×4 + 民忠×2) × 人口 ÷ 300
	//	米 ＝ (太守魅力 + 土地價值×3 + (100−洪水率) + 民忠×2) × 人口 ÷ 200
	//
	// 所以四個可以操作的量是**土地價值、民眾忠誠、洪水率、太守魅力**，
	// 而且權重差很多：土地價值一點抵金四點、忠誠一點抵兩點，洪水率
	// 只進米那一半。強化 AI 的內政順序就照這個權重排。
	//
	// **這一段是 remake 自己的策略不是原版行為**：原版的內政那張表
	// 只擲兩次亂數決定「開墾／防洪／閒著」（`docs/mechanics/70-ai` §2.11），
	// 它不看收入。
	floodIncome   = 20 // 洪水率壓到這裡（再低下去只換得到米）
	loyaltyTarget = 90 // 民眾忠誠做到這裡

	// TuneGovernorCharmGain 是「換太守至少要多幾點魅力才值得」。
	//
	// 換人會動到身分（舊的降回部將、新的升太守，`0xd6cd`），所以不為了
	// 一兩點翻來覆去。魅力在秋收公式裡是 ×1，與土地價值的 ×4 比是小項——
	// 但它**改一次就長期生效**，不像開墾每年被自然衰減吃掉。
	TuneGovernorCharmGain = 10

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
	// 三十六個月、每郡每月一道令）：
	//
	//	門檻      存活  最大  易主   總兵    總金
	//	幾乎不設防   3   33 郡  33   39,983  68,018
	//	40%        12   12 郡  16   93,685  58,672
	//	60%        13   10 郡  12  110,377  47,260
	//	80%        14    8 郡  10  134,704  26,824
	//	100%       14    8 郡   8  131,096  29,209
	//	140%       14    7 郡   8  135,721  27,765
	//
	// 兩端都是壞的：不設防那一格三十六個月就把世界打空（總兵只剩
	// 四萬，一片守不住的地），80% 以上曲線就平了——多留的兵換不到
	// 更安全，只換到不再擴張。60% 在「還打得動」與「守得住」之間。
	// 守方本來就吃地利加成，不必一比一。
	TuneGarrisonRatio = 60
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
	// **一個郡一個月下幾道令由玩家決定**（「其他 → 電腦指令」，
	// 1–5，預設 1）。電腦不受「每郡每月一道令」管
	//（`docs/mechanics/70-ai` §2.12，`L1`），所以這一格調的是
	// **強化 AI 有多強**不是規則。
	//
	// `rounds` 是防呆不是規則：不耗指令的命令不算進道數（見下），
	// 所以迴圈的次數不再由 `AIOrders()` 一個人決定。
	const maxRounds = game.AIOrdersMax + 4
	for i, rounds := 0, 0; i < g.Options.AIOrders() && rounds < maxRounds; rounds++ {
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
		// **「指定太守」不耗指令**（說明書 p.23，`installGovernor`
		// 也沒有碰 `Commanded`），所以不算進這個月的道數——否則一個郡
		// 想換太守就等於整個月什麼都沒做。
		if _, free := o.(game.AppointGovernorOrder); !free {
			i++
		}
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

	// ---- 收入 ----
	//
	// **這一段排在「兵力補到上限」前面。** 先前的順序是徵兵在前，
	// 而徵兵幾乎永遠有空額可以補，於是開墾排在最後一條等於從來輪不到：
	// 三十六個月跑下來平均地力只剩 7（`TestZZAIOrdersSweep` 改動前的
	// 數字），而地力正是秋收公式裡權重最大的那一項。
	//
	// 換句話說，那一版的電腦把每一塊錢都變成兵，卻沒有人在賺錢。
	// 收入這一段就是把「錢從哪來」接回去——秋天收進來的金，隔年
	// 才有得徵兵。

	// 6. 太守換成魅力最高的那一位（秋收的 `太守魅力` 那一項）。
	//    **改一次就長期生效**，所以排在要按月重做的三項前面。
	if o := e.betterGovernor(g, f, p); o != nil {
		return o
	}

	// 7. 開墾：土地價值是秋收權重最大的因子（金 ×4、米 ×3），
	//    而且每年都被自然衰減吃掉（`game.AnnualDecay`），要一直做。
	if p.LandValue < landTarget && spendable >= game.CostReclaim {
		// 電腦那一條不收錢（`0xba02`），`spendable` 的閘門只是保守。
		if x := e.wisest(g, f, p.ID); x != nil {
			return game.ReclaimOrder{At: p.ID, General: x.Index, Auto: true}
		}
	}

	// 8. 防洪：洪水率只進米那一半（`100 − 洪水率`），權重比地力小，
	//    所以排在開墾後面。急難那一段（門檻 60）已經先擋過一次。
	if p.FloodRate > floodIncome && spendable >= game.CostFloodControl {
		if x := e.wisest(g, f, p.ID); x != nil {
			return game.FloodControlOrder{At: p.ID, General: x.Index, Auto: true}
		}
	}

	// 9. 賑民把民眾忠誠推到高檔（金米都是 ×2）。**要留得起存底**
	//    ——這一項按月花錢，把郡庫掏空換來的收入隔年才進帳。
	if p.PublicLoyalty < loyaltyTarget && spendable >= TuneEnhancedRelief*2 {
		return game.ReliefOrder{At: p.ID, Gold: TuneEnhancedRelief}
	}

	// ---- 兵 ----

	// 10. 兵力補到上限。
	if o := e.conscript(g, f, p, spendable, conscriptShare); o != nil {
		return o
	}

	// 11. 兵多但訓練差就練兵。不花錢，所以放在募兵之後。
	if x := e.leastTrained(g, f, p.ID); x != nil && x.Training < 80 && x.Soldiers > 0 {
		return game.TrainOrder{At: p.ID}
	}

	// 12. 米太多就賣一些。
	if p.Rice > sellRiceAbove && p.Gold < game.MaxGold-1000 {
		return game.SellRiceOrder{At: p.ID, Units: sellRiceBatch}
	}
	return nil
}

// betterGovernor 把太守換成魅力最高的那一位。
//
// 秋收的三個因子裡（`game.HarvestGold`），太守魅力是唯一**改一次就
// 長期生效**的——土地價值與忠誠每年都會被自然衰減吃掉。
//
// 三道閘門：
//
//   - **主事者是君主就不換**（`state.StatusLord`，不是 `StatusChief`
//     ——那一個是軍師）。這一條與原版同向：原版判的也是「州郡
//     offset 32 指到的那一位的身分」，不是「君主人在這個郡」
//     （`docs/mechanics/70-ai` §2.6）。玩家那一條另外擋著
//     （`game.AppointGovernor`：「君主自己在這個郡，不需要指定太守」）。
//   - **只挑自己勢力的人——這一道是強化 AI 自己加的。** 原版的候選
//     名單是「站在這個郡裡的人」（`buildRoster` 模式 2：所在郡相同 ＋
//     身分 0–3），**範圍本來就限制在同一個郡**，只是不比對勢力
//     （`docs/re/07` §6，七個模式一個都沒有）。那不是漏掉：郡的所屬
//     每回合由駐軍重算、後寫的蓋前寫的，混編是表得出來的盤面，而
//     主事者換人郡就跟著改所屬（`0xd74d`）——整套是自洽的。
//     強化 AI 不想讓郡易主，所以多加這一道；**這是它與還原版分岔的
//     地方，不是在修正原版**。
//   - **至少要多 `TuneGovernorCharmGain` 點**才換，不為了一兩點翻來覆去。
func (e *enhanced) betterGovernor(g *game.State, f state.FactionID,
	p *game.Prefecture) game.Order {
	cur := g.Governor(p.ID)
	if cur != nil && cur.Status == state.StatusLord {
		return nil
	}
	best := e.charmiest(g, f, p.ID)
	if best == nil || (cur != nil && best.Index == cur.Index) {
		return nil
	}
	now := 0
	if cur != nil {
		now = int(cur.Charm)
	}
	if int(best.Charm) < now+TuneGovernorCharmGain {
		return nil
	}
	return game.AppointGovernorOrder{At: p.ID, Target: best.Index, Auto: true}
}

// charmiest 是這個郡裡自己勢力魅力最高的那一位。
func (e *enhanced) charmiest(g *game.State, f state.FactionID, id int) *game.General {
	var best *game.General
	for _, x := range g.Garrison(id) {
		if x.Faction != f {
			continue
		}
		if best == nil || x.Charm > best.Charm {
			best = x
		}
	}
	return best
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
