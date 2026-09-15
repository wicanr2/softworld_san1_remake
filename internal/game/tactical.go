package game

import (
	"sort"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 戰略層與戰術層的接縫。
//
// `Attack` 把雙方的將領交給 `internal/battle` 打完，再把結果
//（傷亡、被擒、誰佔了城池）搬回局面上。

// toLeader 把一位人物換成戰場上的將領。
func toLeader(x *General) battle.Leader {
	return battle.Leader{
		Index: x.Index, Name: x.Name,
		War: x.War, Intel: x.Intel, Stamina: x.Stamina, Charm: x.Charm,
		Soldiers: x.Soldiers, Training: x.Training, Arms: x.Arms,
		Troop: battle.TroopKind(x.Troop),
	}
}

// weatherFor 是這一場的天氣。
//
// 手冊沒說天氣怎麼決定，只說火攻要刮風、水淹要下雨（p.32–33）。
// 這裡從年月與郡編號推——**決定性**，所以同一場戰役重跑天氣一樣。
// 夏天多雨、秋天多風，與四季事件的取向一致。
func (g *State) weatherFor(at int) battle.Weather {
	r := g.roll(at, int(g.Date.Season()), 0x77ea)
	switch g.Date.Season() {
	case Summer:
		if r < 45 {
			return battle.Rainy
		}
	case Autumn:
		if r < 40 {
			return battle.Windy
		}
	default:
		if r < 20 {
			return battle.Windy
		}
		if r < 35 {
			return battle.Rainy
		}
	}
	return battle.Clear
}

// Field 是某個郡的主戰場地形（「郡地理誌」，說明書 p.19）。
//
// **地圖是原版的資料**：州郡記錄 offset 55–174 的 120 個位元組，
// 42 個郡各一張，沒有兩張相同。所以查看到的與真的打起來用的、
// 以及原版畫出來的，是同一張。
func (g *State) Field(at int) *battle.Field { return g.fieldFor(at) }

// fieldFor 取某個郡的戰場。
//
// 劇本沒帶地圖時（自組的測試局面）退回生成器——它是決定性的，
// 同一個郡永遠得到同一張圖，所以那條路徑也仍然可重現。
func (g *State) fieldFor(at int) *battle.Field {
	p := g.Prefecture(at)
	if p == nil {
		return battle.Generate(battle.Params{Prefecture: at})
	}
	if len(p.BattleField) == battle.FieldBytes {
		if f, err := battle.Load(p.BattleField, p.Neighbours); err == nil {
			return f
		}
	}
	return battle.Generate(battle.Params{
		Prefecture: at, Neighbours: p.Neighbours, Forts: p.Forts,
		LandValue: p.LandValue, FloodRate: p.FloodRate,
	})
}

// Pending 是一場已經開打、還沒收尾的戰役。
//
// 玩家親自指揮時，開打與收尾之間隔著幾十次按鍵；那段時間裡
// **戰場的狀態在 `internal/battle`，局面的狀態還沒動**。
// 把中間需要記住的東西放這裡，收尾時一起搬回去。
type Pending struct {
	B *battle.Battle

	// Player 為真表示這一場由玩家指揮。
	Player bool

	from, to int
	by       state.FactionID
	att, def []*General

	// aidAtt／aidDef 是助攻軍與助守軍。**它們不是主攻軍的一部分**：
	// 打贏了進駐的只有主攻軍，援軍的生還者留在自己的郡裡。
	aidAtt, aidDef []*General

	result *BattleResult

	// autoAI 為真表示這一場走的是電腦對電腦那條（`battle.AutoResolveAI`）。
	// 收尾的錢糧規則跟著換，見 settle。
	autoAI bool
}

// Battle 是這一場的戰術層戰役。
func (p *Pending) Battle() *battle.Battle { return p.B }

// Where 是這一場打在哪個郡（守方那一邊），From 是攻方從哪裡來。
func (p *Pending) Where() int { return p.to }
func (p *Pending) From() int  { return p.from }

// Chiefs 是攻方與守方的統帥；沒有就是 nil。
//
// 取的是各方名單的排頭——原版的統帥就是編隊時排在最前面的那一位
// （`internal/battle` 的 `Commander`）。
func (p *Pending) Chiefs() (att, def *General) {
	if len(p.att) > 0 {
		att = p.att[0]
	}
	if len(p.def) > 0 {
		def = p.def[0]
	}
	return
}

// fight 把一場戰役打完，並把結果搬回局面。
//
// **兩條路，照原版的分岔選**（`0x20471`，`docs/re/05` §7.1）：郡裡有玩家
// 就進戰術層（`Auto()` 是「不想看就自動打完」的那一種），四個郡都沒有玩家
// 就走 `AutoResolveAI()`——原版在那種情況下**根本不進戰術層**，整場只用
// 兩邊的兵士數與綜合能力。
func (g *State) fight(from, to int, att, def []*General, by state.FactionID) *BattleResult {
	p := g.prepare(from, to, att, def, by, HalfSupply(), Aid{})
	if g.noPlayerIn(from, to) {
		p.autoAI = true
		p.B.AutoResolveAI()
		g.ravageBattlefield(to)
	} else {
		p.B.Auto()
	}
	return g.settle(p)
}

// ravageBattlefield 是**戰場那個郡被打殘**（`0x1f8b2`–`0x1f9bd`，`L0`）。
//
//	民眾忠誠（州郡 offset 26） ← max(0, 忠誠 − RND(10) − 1)
//	土地價值（offset 27）      ← max(0, 地價 − RND(10) − 1)
//	洪水率（offset 28）        ← min(100, 洪水率 + RND(7) + 2)
//	物價（offset 29）          ← min(70, 物價 + RND(18) + 2)
//
// 四個欄位都打在**守方那一郡**（原版存在 `es:[0x1bf8]`，`0x1E908` 從
// 主守郡填進去的），不是四個軍力各一個郡。
//
// **只在電腦對電腦那條路上做**：它寫在 `0x1f6fe` 的收尾裡，玩家在場的
// 戰役走戰術層，不經過這一段（`docs/re/05` §7.1）。
func (g *State) ravageBattlefield(prefecture int) {
	p := g.Prefecture(prefecture)
	if p == nil {
		return
	}
	down := func(v uint8, drop int) uint8 {
		if int(v) <= drop {
			return 0
		}
		return v - uint8(drop)
	}
	up := func(v uint8, add, cap int) uint8 {
		n := int(v) + add
		if n > cap {
			n = cap
		}
		return uint8(n)
	}
	p.PublicLoyalty = down(p.PublicLoyalty, g.Roll(10, prefecture, 0, 0x1f6fe)+1)
	p.LandValue = down(p.LandValue, g.Roll(10, prefecture, 1, 0x1f6fe)+1)
	p.FloodRate = up(p.FloodRate, g.Roll(7, prefecture, 2, 0x1f6fe)+2, RavageFloodCap)
	p.PriceLevel = up(p.PriceLevel, g.Roll(18, prefecture, 3, 0x1f6fe)+2, RavagePriceCap)
}

// 戰後受損的兩個上限（`0x1f95e`／`0x1f9a3`）。
const (
	RavageFloodCap = 100 // 洪水率
	RavagePriceCap = 70  // 物價
)

// placeAfterAIBattle 是電腦對電腦戰役的安置（`0x1fb26`，`L0`）。
//
// **所有生還者的所在郡都設成戰場郡**（`0x1fd4f`），接著對敗方的每一位跑
// 一次收降（`0x1ff7c`）：收得下來就改勢力、身分降成一般武將、忠誠等於
// 勝方的人望；**收不下來的維持原本的勢力留在那裡**。
//
// 最後那一句是重點：一場敗仗會在勝方的郡裡留下一批敵方武將，原版的混編
// 郡就是這樣產生的——也因此原版挑名單時一律不比對勢力（`docs/re/07` §6）。
// 郡的歸屬由人物表重算，所以留下來的人會參與下一次的歸屬判定。
//
// 與玩家那條不同：玩家打輸時攻軍退回原郡（`settle` 的另一半）。
func (g *State) placeAfterAIBattle(p *Pending, winner, loser state.FactionID) {
	dst := g.Prefecture(p.to)
	if dst == nil {
		return
	}
	var all []*General
	for _, group := range [][]*General{p.att, p.def, p.aidAtt, p.aidDef} {
		for _, x := range group {
			if x != nil && x.Employed() && x.Soldiers > 0 {
				all = append(all, x)
			}
		}
	}
	for _, x := range all {
		x.Location = p.to
	}
	prestige := 0
	if f := g.Faction(winner); f != nil {
		prestige = f.Prestige
	}
	for _, x := range all {
		if x.Faction != loser || x.Status == state.StatusLord {
			continue // 君主不被收編（`0x1ff93`）
		}
		if len(g.Garrison(p.to)) >= WarRecruitOfficerCap {
			continue // 郡裡的在職將滿了（`0x1ffb4`）
		}
		bonded := x.Bond != x.Index && x.Bond >= 0
		if bonded {
			b := g.General(x.Bond)
			bonded = b != nil && b.Faction == x.Faction
		}
		roll := 0
		if bonded {
			roll = g.Roll(WarRecruitBondSpread, x.Index, p.to, 0x1ff7c)
		}
		if !WarRecruited(prestige,
			WarRecruitResistance(int(x.Intel), int(x.War), bonded, roll)) {
			continue
		}
		// 收編改的四個欄位（`0x1feba`）。原本是軍師的話，舊主的軍師位子
		// 跟著空出來。
		if x.Status == state.StatusChief {
			if f := g.Faction(x.Faction); f != nil {
				f.Chief = -1
			}
		}
		x.Loyalty = uint8(WarRecruitLoyalty(prestige))
		x.Faction = winner
		x.Status = state.StatusOfficer
		x.Location = p.to
	}
}

// noPlayerIn 回報這幾個郡是不是一個玩家的都沒有。
//
// 原版判的是**諸侯記錄 offset 0 == 1（玩家控制）**，四個郡（主攻、助攻、
// 主守、助守）各判一次，郡編號不在 1..42 的那一格跳過——所以「沒有援軍」
// 的 `0xFFFF` 不會被誤判成玩家。
func (g *State) noPlayerIn(prefectures ...int) bool {
	if g.Player == state.NoFaction {
		return true // 純觀戰：一個玩家都沒有
	}
	for _, n := range prefectures {
		p := g.Prefecture(n)
		if p != nil && p.Owned() && p.Owner == g.Player {
			return false
		}
	}
	return true
}

// Supply 是出兵時攜帶的錢糧。
//
// 原版會問：`攜帶多少金`、`攜帶多少米`，而且把三十天要多少米算給玩家看
// （`docs/re/04` §5）。負數或超過原郡庫存的部分會被夾住。
type Supply struct {
	Gold, Rice int

	// Auto 為真表示照舊帶原郡的一半——電腦諸侯用這個。
	Auto bool
}

// HalfSupply 是「帶一半」，電腦諸侯出兵時用的預設。
func HalfSupply() Supply { return Supply{Auto: true} }

// prepare 把雙方擺上戰場，扣掉隨軍帶走的錢糧，但**不打**。
// Aid 是一場戰役的兩支援軍（原版 `battle(攻方郡, 攻方援郡, 守方郡, 守方援郡)`
// 的第二與第四個參數，`0x20200`）。
//
// 援郡是**郡不是將領名單**：原版只把郡編號傳下去，援軍就是那個郡的
// 全部駐軍——與主守軍同一個規矩（說明書 p.27）。0 ＝ 沒有援軍。
type Aid struct {
	Attacker int // 助攻軍的來源郡
	Defender int // 助守軍的來源郡
}

func (g *State) prepare(from, to int, att, def []*General, by state.FactionID, sup Supply, aid Aid) *Pending {
	dst := g.Prefecture(to)
	r := &BattleResult{From: from, To: to}

	setup := battle.Setup{
		Field:    g.fieldFor(to),
		Weather:  g.weatherFor(to),
		Seed:     uint32(g.Date.Year*13 + g.Date.Month*7 + from*31 + to),
		FromGate: from,
		// 版本與難度決定的戰役規則（`docs/spec/004` §5）。
		Rules: battle.RulesFor(g.Edition, g.Difficulty),
	}
	src := g.Prefecture(from)
	if src != nil {
		// 「除了主守軍之外的軍隊都必須從己郡攜帶金、米」（說明書 p.28）。
		gold, rice := sup.Gold, sup.Rice
		if sup.Auto {
			// 帶一半，留一半給郡治理。
			gold, rice = src.Gold/2, src.Rice/2
		}
		gold = clampTo(gold, src.Gold)
		rice = clampTo(rice, src.Rice)
		if gold < 0 {
			gold = 0
		}
		if rice < 0 {
			rice = 0
		}
		setup.AttackerGold, setup.AttackerRice = gold, rice
		src.Gold -= gold
		src.Rice -= rice
	}
	if dst != nil {
		setup.DefenderGold, setup.DefenderRice = dst.Gold, dst.Rice
	}
	for _, x := range att {
		setup.Attackers = append(setup.Attackers, toLeader(x))
	}
	for _, x := range def {
		setup.Defenders = append(setup.Defenders, toLeader(x))
	}
	aidAtt, aidDef := g.garrisonOf(aid.Attacker), g.garrisonOf(aid.Defender)
	for _, x := range aidAtt {
		setup.AidAttackers = append(setup.AidAttackers, toLeader(x))
	}
	for _, x := range aidDef {
		setup.AidDefenders = append(setup.AidDefenders, toLeader(x))
	}

	// **出征的將領離開原本的郡**：原版在整編收尾走完該軍團的五支部隊，
	// 把每一位將領的人物記錄 offset 19（所在郡）寫 0
	//（`0x20ce0: movb $0x0, es:0x2223(%bx)`，基底 0x2210）。主守軍走的
	// `0x20ac3` 那條也 `jmp 0x20c5e` 進同一段，所以四個軍團都會清——
	// 只是主守軍本來就在戰場那個郡，清完打完再寫回 `p.to` 等於沒動。
	// 郡的歸屬是從人物表導出來的（`docs/re/03` §1.5），所以這一格會讓
	// 出征中的部隊不再替原郡撐著旗。打完由 `p.to` 補回去。
	for _, x := range append(append([]*General{}, att...), aidAtt...) {
		if x != nil {
			x.Location = 0
		}
	}
	// 電腦部隊用哪一套判斷式（`docs/design/01`）、交戰結算的模式要的難度，
	// 以及退兵逃得去的鄰郡（原版 `0x23dd4`：戰場所在郡的鄰郡裡無主或
	// 自己勢力的，扣掉對方助軍出兵的那一郡）。
	setup.AI = g.battleAI()
	setup.Difficulty = g.Difficulty
	if dst != nil {
		setup.Escapes[battle.MainAttacker] = g.escapesFor(dst, by, aid.Defender)
		setup.Escapes[battle.AidAttacker] = setup.Escapes[battle.MainAttacker]
		setup.Escapes[battle.MainDefender] = g.escapesFor(dst, dst.Owner, aid.Attacker)
		setup.Escapes[battle.AidDefender] = setup.Escapes[battle.MainDefender]
	}
	return &Pending{B: battle.New(setup), from: from, to: to, by: by,
		att: att, def: def, aidAtt: aidAtt, aidDef: aidDef, result: r}
}

// battleAI 把這一局的 AI 模式換成戰術層的旗標：`enhanced` 走 remake 自己
// 的自動作戰，其餘（含沒設）走原版的九支判斷式。
func (g *State) battleAI() battle.AI {
	switch g.Options.AIMode {
	case "enhanced":
		return battle.AIEnhanced
	case "plus":
		return battle.AIPlus
	}
	return battle.AIBase
}

// escapesFor 列出 `f` 這一方從 `at` 這個戰場退兵時逃得去的鄰郡
// （原版 `0x23e34`–`0x23ef2`，`L0`）：無主或 `f` 自己的鄰郡，扣掉對方
// 助軍出兵的那一郡 `exclude`。每一郡帶著現役武將數，容量判定在戰術層做。
func (g *State) escapesFor(at *Prefecture, f state.FactionID, exclude int) []battle.Escape {
	var out []battle.Escape
	for _, n := range at.Neighbours {
		q := g.Prefecture(n)
		if q == nil || n == exclude {
			continue
		}
		if q.Owned() && q.Owner != f {
			continue
		}
		out = append(out, battle.Escape{Prefecture: n, Active: g.ActiveGenerals(n)})
	}
	return out
}

// garrisonOf 是一個郡的全部駐軍（只算所屬勢力的人）。援軍用這個點齊。
func (g *State) garrisonOf(prefectureID int) []*General {
	p := g.Prefecture(prefectureID)
	if p == nil || !p.Owned() {
		return nil
	}
	var out []*General
	for _, x := range g.Garrison(prefectureID) {
		if x.Faction == p.Owner {
			out = append(out, x)
		}
	}
	return out
}

// settle 把打完的戰役搬回局面。
//
// **收尾只能做一次。** 做兩次的話傷亡會被重複套用，而那在畫面上
// 只看得出「這一場死得特別多」。
func (g *State) settle(p *Pending) *BattleResult {
	b, r := p.B, p.result
	from, to, by := p.from, p.to, p.by
	att, def := p.att, p.def
	dst := g.Prefecture(to)
	if !b.Over {
		b.Auto()
	}
	r.Days = b.Day
	r.AttackerWon = b.AttackerWon
	r.Log = append(r.Log, b.Log...)

	// 傷亡與生死搬回局面。
	byIndex := map[int]*General{}
	for _, x := range att {
		byIndex[x.Index] = x
	}
	for _, x := range def {
		byIndex[x.Index] = x
	}
	// **援軍的傷亡一樣要搬回去**：漏掉的話助攻軍打完毫髮無傷，
	// 而戰報上的數字看起來完全正常。
	for _, x := range p.aidAtt {
		byIndex[x.Index] = x
	}
	for _, x := range p.aidDef {
		byIndex[x.Index] = x
	}
	for _, u := range b.Units {
		for _, l := range u.Leaders {
			x := byIndex[l.Index]
			if x == nil {
				continue
			}
			lost := x.Soldiers - l.Soldiers
			if lost < 0 {
				lost = 0
			}
			if u.Side.Attacking() {
				r.AttackerLost += lost
			} else {
				r.DefenderLost += lost
			}
			x.Soldiers = l.Soldiers
			x.Stamina = l.Stamina
			switch {
			case l.Dead:
				g.retireBy(x, "battle")
			case l.Captured:
				r.Captives = append(r.Captives, Captive{General: l.Index, Name: l.Name})
			}
		}
	}
	sort.Slice(r.Captives, func(i, j int) bool {
		return r.Captives[i].General < r.Captives[j].General
	})
	g.seizeTreasures(r, by)

	// **人望：勝方 +2、敗方 −2**（`0x204b4`／`0x204e0`，`L0`），夾在 0–100。
	// 人望決定部下忠誠每年的漲跌（`Faction.Prestige`），所以打贏仗的
	// 諸侯不只多一個郡，麾下也更死心塌地。
	var defender state.FactionID = state.NoFaction
	if dst != nil {
		defender = dst.Owner
	}
	winner, loser := by, defender
	if !r.AttackerWon {
		winner, loser = defender, by
	}
	g.shiftPrestige(winner, PrestigeOnWin)
	g.shiftPrestige(loser, -PrestigeOnWin)

	// 撤退或全滅的攻方回原郡；沒被擒沒死的守方留在原地。
	if r.AttackerWon {
		g.takePrefecture(from, to, att, by)
		r.PrefectureTook = true
	}
	if p.autoAI {
		g.placeAfterAIBattle(p, winner, loser)
		// 電腦對電腦（`0x1f82f`／`0x1f8ae`）：**四個軍團的隨軍錢糧全部
		// 收進守方那一郡**，不管誰贏。攻方打輸時補給等於送給守方——
		// 與玩家那條「補給跟著自己走」不一樣，這是原版的規則。
		if dst != nil {
			gold, rice := 0, 0
			for s := range b.Gold {
				gold += b.Gold[s]
				rice += b.Rice[s]
			}
			dst.Gold = clampTo(gold, MaxGold)
			dst.Rice = clampTo(rice, MaxRice)
		}
	} else {
		// 隨軍剩下的錢糧回到落腳的郡。
		back := from
		if r.AttackerWon {
			back = to
		}
		if p := g.Prefecture(back); p != nil {
			p.Gold = clampTo(p.Gold+b.Gold[battle.MainAttacker], MaxGold)
			p.Rice = clampTo(p.Rice+b.Rice[battle.MainAttacker], MaxRice)
		}
		if dst != nil {
			dst.Gold = clampTo(b.Gold[battle.MainDefender], MaxGold)
			dst.Rice = clampTo(b.Rice[battle.MainDefender], MaxRice)
		}
	}
	g.Reports = append(g.Reports, r)
	return r
}

// BeginAttack 與 Attack 收同樣的條件，但**不打**：回傳一場擺好的戰役，
// 讓呼叫端一步一步指揮（`battle.Runner`）。打完之後要叫 FinishAttack。
//
// ⚠ **開打就已經動到局面**：攻方帶走的錢糧當場從原郡扣掉，
// 主事者親征也已經交接。中途放棄不會回到開打前——原版也是這樣，
// 出兵是不能反悔的。
func (g *State) BeginAttack(from, to int, attackers []int, by state.FactionID, sup Supply) (*Pending, error) {
	att, def, err := g.musterAttack(from, to, attackers, by)
	if err != nil {
		return nil, err
	}
	p := g.prepare(from, to, att, def, by, sup, Aid{})
	p.Player = true
	return p, nil
}

// CampaignForce 是這批將領帶出去的總兵力，用來算三十天要多少米。
func (g *State) CampaignForce(attackers []int) int {
	n := 0
	for _, i := range attackers {
		if x := g.General(i); x != nil {
			n += x.Soldiers
		}
	}
	return n
}

// RiceForCampaign 是這批兵打滿三十天要多少米（原版 `30日須耗用%d米`）。
func RiceForCampaign(soldiers int) int { return battle.RiceForCampaign(soldiers) }

// FinishAttack 把打完的戰役搬回局面。
func (g *State) FinishAttack(p *Pending) *BattleResult {
	if p == nil || p.result == nil {
		return nil
	}
	return g.settle(p)
}

// seizeTreasures 是「獲勝軍若於戰後捉到敵軍君主，其寶物將全歸獲勝軍所有」
// （說明書 p.35）。
//
// 要在 takePrefecture 之前叫：郡易主之後就查不出守方原本是誰了。
func (g *State) seizeTreasures(r *BattleResult, by state.FactionID) {
	winner := g.Faction(by)
	if !r.AttackerWon {
		winner = g.Faction(g.Prefecture(r.To).Owner)
	}
	if winner == nil {
		return
	}
	for _, c := range r.Captives {
		x := g.General(c.General)
		if x == nil || x.Status != state.StatusLord {
			continue
		}
		loser := g.Faction(x.Faction)
		if loser == nil || loser == winner {
			continue
		}
		moved := false
		for i := range loser.Treasury {
			if loser.Treasury[i] > 0 {
				moved = true
			}
			winner.Treasury[i] = clampTo(winner.Treasury[i]+loser.Treasury[i], TreasuryCap)
			loser.Treasury[i] = 0
		}
		if moved {
			r.Log = append(r.Log, tf("msg.spoils", x.Name))
		}
	}
}

// PrestigeOnWin 是打贏一場戰役的人望增減（`0x204b4`：`+2`／`−2`，`L0`）。
const PrestigeOnWin = 2

// shiftPrestige 調整一個勢力的人望，夾在 0–100。
//
// **無主的一方不算**：空白郡沒有諸侯，攻下它不加人望。
func (g *State) shiftPrestige(id state.FactionID, delta int) {
	f := g.Faction(id)
	if f == nil {
		return
	}
	f.Prestige = clampTo(f.Prestige+delta, 100)
	if f.Prestige < 0 {
		f.Prestige = 0
	}
}
