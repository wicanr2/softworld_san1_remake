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
		p.B.AutoResolveAI()
		g.warWeariness(from, to)
	} else {
		p.B.Auto()
	}
	return g.settle(p)
}

// warWeariness 是打過仗的郡民眾忠誠的下降（`0x1f8b2`–`0x1f8f0`，`L0`）。
//
//	民眾忠誠 ← max(0, 民眾忠誠 − RND(10) − 1)
//
// 原版對四個軍力的郡各擲一次（沒有援軍的那一格是 `0xFFFF`，跳過），
// 而且**只在電腦對電腦那條路上做**——它寫在 `0x1f6fe` 的收尾裡，
// 玩家在場那條走的是戰術層，不經過這一段。
func (g *State) warWeariness(prefectures ...int) {
	for i, n := range prefectures {
		p := g.Prefecture(n)
		if p == nil {
			continue
		}
		drop := g.Roll(10, n, i, 0x1f6fe) + 1
		if int(p.PublicLoyalty) <= drop {
			p.PublicLoyalty = 0
			continue
		}
		p.PublicLoyalty -= uint8(drop)
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

	return &Pending{B: battle.New(setup), from: from, to: to, by: by,
		att: att, def: def, aidAtt: aidAtt, aidDef: aidDef, result: r}
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
				g.retire(x)
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
