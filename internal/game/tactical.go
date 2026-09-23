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
//
// 被擒之後電腦捕獲方當場處置要看的三格（`battle.Leader` 的
// `Lord`／`Loyalty`／`BondAlly`）也在這裡填：牽絆對象是不是同一勢力的人
// 在出征那一刻算好——原版是被擒那一刻查（`0x25ebe`–`0x25ed6`），差別只在
// 同一場裡牽絆對象自己先改投了的情況。
func (g *State) toLeader(x *General) battle.Leader {
	l := battle.Leader{
		Index: x.Index, Name: x.Name,
		War: x.War, Intel: x.Intel, Stamina: x.Stamina, Charm: x.Charm,
		Soldiers: x.Soldiers, Training: x.Training, Arms: x.Arms,
		Troop: battle.TroopKind(x.Troop),
		Lord:  x.Status == state.StatusLord,
	}
	// 在野的忠誠是 0xFF 哨兵，原版當有號位元組讀成 −1（`cbw`）。
	l.Loyalty = int(int8(x.Loyalty))
	if x.Bond != x.Index {
		if y := g.General(x.Bond); y != nil && y.Faction == x.Faction {
			l.BondAlly = true
		}
	}
	return l
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

	// aidAtt／aidDef 是助攻軍與助守軍。玩家戰役勝方主軍尚存時，
	// 勝方援軍回來源郡；主軍全滅時由勝方援軍接管戰場（`docs/spec/020`）。
	aidAtt, aidDef []*General
	// aid 是兩支援軍各從哪一郡來（0 ＝ 沒有）。
	aid Aid

	// factions 是四種軍力各屬哪個勢力（`sideFactions`），收尾時把電腦
	// 捕獲方當場的處置搬回人物表要用。
	factions [4]state.FactionID

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

// SidePrefecture 是某一種軍力從哪一郡來（原版軍團記錄的 `0x1770`，紮寨提示
// 「(%2d%s)」那一格）：主攻軍是出兵郡、主守軍是戰場那一郡、兩支援軍是各自的
// 來源郡；沒有那支軍力回 0。
func (p *Pending) SidePrefecture(s battle.Side) int {
	switch s {
	case battle.MainAttacker:
		return p.from
	case battle.MainDefender:
		return p.to
	case battle.AidAttacker:
		return p.aid.Attacker
	case battle.AidDefender:
		return p.aid.Defender
	}
	return 0
}

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
	noPlayer := g.noPlayerIn(from, to)
	p := g.prepare(from, to, att, def, by, HalfSupply(), Aid{})
	if noPlayer {
		p.autoAI = true
		// 每天那一擲 `RND(11)`（`0x1f5d8`）走的是原版同一顆 `rand()`，
		// 與郡回合、收降、打殘同一條序列——不接上的話，這一場之後
		// 整個月的骰都會岔開（`TestZZMonthParityPlus`）。
		day := 0
		p.B.AutoResolveAIWithRoll(func(n int) int {
			day++
			return g.Roll(n, from, to, day, 0x1f5d8)
		})
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
// **這組公式只在電腦對電腦那條路上做**。玩家戰術層也會受損，
// 但使用 `ravagePlayerBattlefield` 的另一組範圍與固定值（`docs/spec/020`）。
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

// ravagePlayerBattlefield 對應玩家戰術層戰後的 `0x2674a`–`0x2685a`
// （加強版 `0x23f06`–`0x24002`），兩版都依 15／15／10／25 擲骰。
func (g *State) ravagePlayerBattlefield(prefecture int) {
	p := g.Prefecture(prefecture)
	if p == nil {
		return
	}
	down := func(v uint8, amount int) uint8 {
		if int(v) <= amount {
			return 0
		}
		return v - uint8(amount)
	}
	up := func(v uint8, amount, cap int) uint8 {
		return uint8(clampTo(int(v)+amount, cap))
	}
	p.PublicLoyalty = down(p.PublicLoyalty, g.Roll(15, prefecture, 0, 0x2674a)+3)
	p.LandValue = down(p.LandValue, g.Roll(15, prefecture, 1, 0x2678f)+2)
	p.FloodRate = up(p.FloodRate, g.Roll(10, prefecture, 2, 0x267d3)+4, RavageFloodCap)
	p.PriceLevel = up(p.PriceLevel, g.Roll(25, prefecture, 3, 0x26819)+5, RavagePriceCap)
}

// 戰後受損的兩個上限（`0x1f95e`／`0x1f9a3`）。
const (
	RavageFloodCap = 100 // 洪水率
	RavagePriceCap = 70  // 物價
)

// placeAfterAIBattle 是電腦對電腦戰役的安置（`0x1f9fe`–`0x1fb25` ＋
// `0x1fb26`，`L0`，`docs/re/05` §7.1）。
//
// 原版先把四個軍團每一位的所在郡清成 0，**勝方兩個軍團的生還者一律回到
// 戰場郡**（`0x1fab8`）。敗方的兩個軍團走另一條：
//
//	退路候選表 ＝ 戰場郡的鄰郡裡，所屬 == 敗方主軍的勢力 或 無主 的那些
//	逐軍團、逐部隊、逐槽（0..9）：
//	    挑 ＝ RND(候選數)                       ; 0x1fc32；候選數 0 時 RND 不抽
//	    候選數 <= 0                    → 進名單
//	    r ＝ RND(200)                            ; 0x1fc5d
//	    謀略 ＋ 戰力 ÷ 2 >= r          → 進名單   ; 0x1fc8a
//	    候選[挑] 的現役將 >= 50        → 進名單   ; 0x1fc9d
//	    否則所在郡 ← 候選[挑]，那一郡的現役將 ＋1  ; 逃掉了，不進名單
//
// 名單裡的每一位再落在戰場郡（`0x1fd4f`）跑一次收降（`0x1ff7c`）：收得
// 下來就改勢力、身分降成一般武將、忠誠等於勝方的人望；收不下來的
// **離開敗方**——變成戰場郡的在野（身分 10，`0x200f4`）或下野（身分 12，
// `0x20050`），沒有一條路讓人保留舊勢力（`CONTEXT.md` R63）。
// 郡的歸屬由人物表重算；**這裡不改郡的所屬**，原版也沒改（`0x1e394` 在
// 下一個郡回合入口重算）。
//
// **兵力歸零的人一樣要安置**：敗方的存活比例是 0 時整個軍團的兵都是
// 0，他們照樣逃或留下，原版沒有「沒兵就消失」這條。
//
// 玩家那條的敗軍逐隊走強制退兵常式，退不了才進被擒處置（`docs/spec/020`）。
func (g *State) placeAfterAIBattle(p *Pending, attackerWon bool) {
	dst := g.Prefecture(p.to)
	if dst == nil {
		return
	}
	b := p.B
	winSides := [2]battle.Side{battle.MainAttacker, battle.AidAttacker}
	loseSides := [2]battle.Side{battle.MainDefender, battle.AidDefender}
	if !attackerWon {
		winSides, loseSides = loseSides, winSides
	}
	winner, loser := p.factions[winSides[0]], p.factions[loseSides[0]]
	// 部隊記錄的順序就是原版跑迴圈的順序：軍團 → 部隊 → 槽。
	generalsOf := func(sides [2]battle.Side) []*General {
		var out []*General
		for _, side := range sides {
			for _, u := range b.Units {
				if u.Side != side {
					continue
				}
				for i := range u.Leaders {
					if x := g.General(u.Leaders[i].Index); x != nil {
						out = append(out, x)
					}
				}
			}
		}
		return out
	}
	// 勝方回戰場郡（`0x1fab8`），然後戰場郡重整一次（`0x1fb07` 的
	// `0x1949e` 與 `0x1fb18` 的 `0x1d638`）——這一刻敗方的所在郡還是 0
	//（主守軍整編時已經清掉，戰場郡在開打期間是無主、沒有主事者的），
	// 所以主事者在這裡換成勝方裡行動者鍵最大的那一位（`refreshGovernor`）。
	for _, x := range generalsOf(winSides) {
		x.Location = p.to
	}
	g.RefreshGarrison(p.to)
	g.refreshGovernor(p.to)

	// 退路候選表（`0x1fb42`–`0x1fbbf`）。
	retreatable := func(n int) bool {
		q := g.Prefecture(n)
		return q != nil && (!q.Owned() || q.Owner == loser)
	}
	var cands []int
	for _, n := range dst.Neighbours {
		if retreatable(n) {
			cands = append(cands, n)
		}
	}
	var roster []*General
	for _, x := range generalsOf(loseSides) {
		pick := g.Roll(len(cands), x.Index, p.to, 0x1fc32)
		if len(cands) <= 0 {
			roster = append(roster, x)
			continue
		}
		r := g.Roll(WarFleeSpread, x.Index, p.to, 0x1fc5d)
		if int(x.Intel)+int(x.War)/2 >= r {
			roster = append(roster, x)
			continue
		}
		q := g.Prefecture(cands[pick])
		if g.StoredActiveGenerals(q.ID) >= WarRecruitOfficerCap {
			roster = append(roster, x)
			continue
		}
		x.Location = q.ID
		q.activeGenerals++
	}

	prestige := 0
	if f := g.Faction(winner); f != nil {
		prestige = f.Prestige
	}
	// 名單逐人（`0x1fd00`–`0x1fe12`）。三條出路由旗標決定：
	//
	//	0  收不下來、不是君主、戰場郡的在野將沒滿 → 0x200f4：失去勢力
	//	   （身分 10、勢力 0xFF、所在郡 ← 戰場郡，郡的在野將 ＋1）
	//	1  君主，或戰場郡的在野將已滿 50 → 0x20050：身分 12、勢力與所在
	//	   都 0xFF——君主的話同時記下事件（0x14968 的繼承與 0x26c08 的寶物）
	//	2  收得下來 → 0x1feba 再判一次 0x1ff7c，過了才改四個欄位；
	//	   沒過（有牽絆的人第二擲）落到 0x20050(1, 他)，與旗標 1 同一支
	//
	// 「收不下來的人留在勝方的郡裡」是對的，但**他已經不是敗方的人**：
	// 身分 10 是可登用的在野（`state.StatusStranded`），勝方下一回合的
	// 登用表就會看到他。
	var fallenLords []*General
	for _, x := range roster {
		x.Location = p.to // 0x1fd4f
		flag := 0
		if x.Status == state.StatusLord {
			flag = 1
		}
		if g.FreeGenerals(p.to) >= WarRecruitOfficerCap && flag == 0 {
			flag = 1 // 0x1fd73：在野將（offset 23）
		}
		// 0x1ff7c：君主與郡裡在職將滿 50 都回 0，不擲。
		bonded := false
		if x.Status != state.StatusLord && g.StoredActiveGenerals(p.to) < WarRecruitOfficerCap {
			bonded = x.Bond != x.Index && x.Bond >= 0
			if bonded {
				mate := g.General(x.Bond)
				bonded = mate != nil && mate.Faction == x.Faction
			}
			roll := 0
			if bonded {
				roll = g.Roll(WarRecruitBondSpread, x.Index, p.to, 0x1ff7c)
			}
			if prestige > 0 && WarRecruited(prestige,
				WarRecruitResistance(int(x.Intel), int(x.War), bonded, roll)) {
				flag = 2
			}
		}
		switch flag {
		case 0:
			if x.Status == state.StatusChief {
				if f := g.Faction(x.Faction); f != nil {
					f.Chief = -1
				}
			}
			// 忠誠不動（`0x200f4` 只寫身分、勢力、所在郡）。
			x.Status = state.StatusStranded
			x.Faction = state.NoFaction
			x.Location = p.to
			continue
		case 2:
			// 收編那一支（`0x1feba`）進去**再判一次** `0x1ff7c`——有牽絆的人
			// 因此再擲一次 `RND(30)`。
			ok := true
			if bonded {
				roll := g.Roll(WarRecruitBondSpread, x.Index, p.to, 0x1feba)
				ok = WarRecruited(prestige,
					WarRecruitResistance(int(x.Intel), int(x.War), bonded, roll))
			}
			if ok {
				// 收編改的四個欄位（`0x1feba`）。原本是軍師的話，舊主的軍師
				// 位子跟著空出來。
				if x.Status == state.StatusChief {
					if f := g.Faction(x.Faction); f != nil {
						f.Chief = -1
					}
				}
				x.Loyalty = uint8(WarRecruitLoyalty(prestige))
				x.Faction = winner
				x.Status = state.StatusOfficer
				x.Location = p.to
				continue
			}
		}
		// 0x20050：身分 12、勢力與所在 0xFF。
		if x.Status == state.StatusLord {
			fallenLords = append(fallenLords, x)
			continue
		}
		if x.Status == state.StatusChief {
			if f := g.Faction(x.Faction); f != nil {
				f.Chief = -1
			}
		}
		x.Status = state.StatusFallen
		x.Faction = state.NoFaction
		x.Location = int(state.NoValue)
	}
	// 名單跑完只做重整（`0x1fe16`–`0x1fe94`）：戰場郡一次，再對候選表
	// 那些鄰郡（敗方的或無主的）各一次。
	g.RefreshGarrison(p.to)
	for _, n := range dst.Neighbours {
		if retreatable(n) {
			g.RefreshGarrison(n)
		}
	}
	// 君主落到旗標 1 的那條（`0x20050` 記事件，`0x1fead` 的 `0x26c08`
	// 收尾）：與戰死同一支繼承常式（`0x14968`），然後勝方分走敗方的寶物。
	for _, x := range fallenLords {
		lost := x.Faction
		g.retireBy(x, "battle")
		g.spoilsFromFallenLord(winner, lost)
	}
}

// spoilsFromFallenLord 是兩種戰役共用的退場君主分贓
// （原版 `0x26c08`、加強版 `0x24338`，`L0`；`docs/spec/020`）：
//
//	敗方的四件寶物之一（RND(4)）先 ＋2
//	每一件：拿走 半 ＋ RND(半)，半 ＝ 敗方的件數 ÷ 2
//	勝方 ← min(100, 勝方 ＋ 拿走)，敗方 −= 拿走
func (g *State) spoilsFromFallenLord(winner, loser state.FactionID) {
	w, l := g.Faction(winner), g.Faction(loser)
	if w == nil || l == nil {
		return
	}
	// 四件是諸侯 offset 15–18（兵書、寶刀、美女、駿馬）；offset 14 的玉璽
	// 不在這一支裡。
	const first = int(TreasureBook)
	k := g.Roll(4, int(winner), int(loser), 0x26c31)
	l.Treasury[first+k] += 2
	var take [4]int
	for i := range take {
		half := l.Treasury[first+i] / 2
		take[i] = half + g.Roll(half, int(winner), int(loser), i, 0x26c67)
	}
	for i := range take {
		if take[i] == 0 {
			continue
		}
		w.Treasury[first+i] = clampTo(w.Treasury[first+i]+take[i], TreasuryCap)
		l.Treasury[first+i] -= take[i]
	}
}

// WarFleeSpread 是敗軍逃散那一擲的範圍（`0x1fc59`：`RND(200)`，`L0`）：
// 謀略 ＋ 戰力 ÷ 2 不到那個數的人才逃得掉。
const WarFleeSpread = 200

// noPlayerIn 回報這幾個郡是不是一個玩家的都沒有。
//
// 原版判的是**諸侯記錄 offset 0 == 1（玩家控制）**，四個郡（主攻、助攻、
// 主守、助守）各判一次，郡編號不在 1..42 的那一格跳過——所以「沒有援軍」
// 的 `0xFFFF` 不會被誤判成玩家。
func (g *State) noPlayerIn(prefectures ...int) bool {
	for _, n := range prefectures {
		p := g.Prefecture(n)
		if p != nil && p.Owned() && g.IsHuman(p.Owner) {
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

	// 天候由戰術層自己擲（開場 `RND(3)`、每天 `RND(10) > 5` 才換，
	// `battle.New`／`EndDay`，`L0`），這裡不指定。
	setup := battle.Setup{
		Field:    g.fieldFor(to),
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
		setup.Attackers = append(setup.Attackers, g.toLeader(x))
	}
	for _, x := range def {
		setup.Defenders = append(setup.Defenders, g.toLeader(x))
	}
	aidAtt, aidDef := g.garrisonOf(aid.Attacker), g.garrisonOf(aid.Defender)
	for _, x := range aidAtt {
		setup.AidAttackers = append(setup.AidAttackers, g.toLeader(x))
	}
	for _, x := range aidDef {
		setup.AidDefenders = append(setup.AidDefenders, g.toLeader(x))
	}
	// 四方各是誰在操縱、人望多少、從哪一郡出兵（`battle.Battle` 的
	// `Computer`／`Renown`／`Origin`）。主守軍的出兵郡就是戰場，原版
	// 退兵時不看它（`0x23fd7`），留 0。
	factions := g.sideFactions(by, dst, aid)
	for side, id := range factions {
		setup.Computer[side] = id != state.NoFaction && !g.IsHuman(id)
		if f := g.Faction(id); f != nil {
			setup.Renown[side] = f.Prestige
		}
	}
	for side, at := range [...]int{battle.MainAttacker: from, battle.AidAttacker: aid.Attacker, battle.AidDefender: aid.Defender} {
		if at > 0 {
			setup.Origin[side] = battle.Escape{Prefecture: at, Active: g.ActiveGenerals(at)}
		}
	}

	// **四支軍團都在整編後離開原郡**（`0x20ce0`，`docs/spec/020` §1）。
	// 主守軍也走同一段，所以戰場郡在戰役期間可暫時無主；版本／玩家
	// 身分須在這一刻之前由 `factions` 保存，不可再讀當下郡所屬。
	for _, army := range [][]*General{att, aidAtt, def, aidDef} {
		for _, x := range army {
			if x != nil {
				x.Location = 0
			}
		}
	}
	seen := map[int]bool{}
	for _, n := range []int{from, to, aid.Attacker, aid.Defender} {
		if n > 0 && !seen[n] {
			g.refreshGovernor(n) // 原版整編後的 `0x1d638`
			seen[n] = true
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
	pb := &Pending{B: battle.New(setup), from: from, to: to, by: by,
		att: att, def: def, aidAtt: aidAtt, aidDef: aidDef, aid: aid, result: r, factions: factions}
	pb.B.Host = &captiveHost{g: g, at: to}
	return pb
}

// CaptiveHost 是 `at` 那一郡當戰場時的 `battle.CaptiveHost`。對拍直接呼叫
// 原版的處置常式時，remake 這一邊從這裡拿同一份。
func (g *State) CaptiveHost(at int) battle.CaptiveHost { return &captiveHost{g: g, at: at} }

// ApplyCaptiveFate 把一位被擒將領的處置（`battle.Leader.Fate`）寫回人物表，
// 戰場是 `at`、捕獲方是 `captor`。收尾的 `settle` 走同一段。
func (g *State) ApplyCaptiveFate(at int, l battle.Leader, captor state.FactionID) {
	if x := g.General(l.Index); x != nil {
		g.applyFate(&Pending{to: at}, x, l, captor)
	}
}

// captiveHost 是 `battle.CaptiveHost`（`docs/spec/018` R2）：戰場那一郡的
// 在野數與釋放的去處。這一場裡囚禁或釋放進去的人另外計，人物表要到
// 收尾才寫回。
type captiveHost struct {
	g     *State
	at    int
	idle  int
	moved map[int]int // 釋放到各郡的人數（現役數要算進去）
}

// CaptiveIdleCap 是一郡在野數的上限（`0x26118`／`0x262d7`：`>= 0x32` 就拒絕）。
const CaptiveIdleCap = 50

func (h *captiveHost) IdleRoom() bool { return h.g.FreeGenerals(h.at)+h.idle < CaptiveIdleCap }
func (h *captiveHost) AddIdle()       { h.idle++ }

// ReleaseTo 是 `0x265ac`：先列被釋放者的勢力裡現役未滿 50 的郡，沒有就列
// 無主的郡，兩張都扣掉戰場郡；清單不空才擲。
func (h *captiveHost) ReleaseTo(general int, roll func(int) int) int {
	x := h.g.General(general)
	if x == nil {
		return -1
	}
	var list []int
	for id := 1; id <= len(h.g.prefectures); id++ {
		q := h.g.Prefecture(id)
		if q == nil || id == h.at || q.Owner != x.Faction { // 在野者的勢力 0xFF 也照比（`cbw` 之後 −1 對 −1）
			continue
		}
		if h.g.ActiveGenerals(id)+h.moved[id] < MaxGeneralsPerPrefecture {
			list = append(list, id)
		}
	}
	if len(list) == 0 {
		for id := 1; id <= len(h.g.prefectures); id++ {
			if q := h.g.Prefecture(id); q != nil && id != h.at && !q.Owned() {
				list = append(list, id)
			}
		}
	}
	if len(list) == 0 {
		return -1
	}
	to := list[roll(len(list))]
	if h.moved == nil {
		h.moved = map[int]int{}
	}
	h.moved[to]++
	return to
}

// sideFactions 是四種軍力各屬哪個勢力：主攻是出兵的諸侯，主守是戰場
// 那一郡的主人，兩支援軍各是援郡的主人。沒出場的是 NoFaction。
func (g *State) sideFactions(by state.FactionID, dst *Prefecture, aid Aid) [4]state.FactionID {
	var out [4]state.FactionID
	for i := range out {
		out[i] = state.NoFaction
	}
	out[battle.MainAttacker] = by
	if dst != nil {
		out[battle.MainDefender] = dst.Owner
	}
	if p := g.Prefecture(aid.Attacker); p != nil && p.Owned() {
		out[battle.AidAttacker] = p.Owner
	}
	if p := g.Prefecture(aid.Defender); p != nil && p.Owned() {
		out[battle.AidDefender] = p.Owner
	}
	return out
}

// battleAI 把這一局的 AI 模式換成戰術層的旗標：`enhanced` 走 remake 自己
// 的自動作戰，其餘（含沒設）走原版的九支判斷式——**哪一版的九支由
// 這一局的版本決定**，不由模式字串決定（`ai.CheckEdition` 已經擋掉
// 「原版 AI 跑在加強版規則上」那種混搭，這裡不再分家）。
func (g *State) battleAI() battle.AI {
	if g.Options.AIMode == "enhanced" {
		return battle.AIEnhanced
	}
	if g.Edition == state.EditionPlus {
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
	winMain, winAid := battle.MainDefender, battle.AidDefender
	loseMain, loseAid := battle.MainAttacker, battle.AidAttacker
	if r.AttackerWon {
		winMain, winAid = battle.MainAttacker, battle.AidAttacker
		loseMain, loseAid = battle.MainDefender, battle.AidDefender
	}
	activeSide := func(side battle.Side) bool {
		for _, u := range b.Units {
			if u.Side == side && u.Alive() {
				return true
			}
		}
		return false
	}
	placeSide := winMain
	returnAid := false
	if !p.autoAI {
		// `0x25652`：主軍尚存時，勝方援軍回來源郡；否則援軍接戰場。
		returnAid = activeSide(winMain) && activeSide(winAid) && p.SidePrefecture(winAid) > 0
		if !activeSide(winMain) {
			placeSide = winAid
		}
		// `0x258f2`：兩支敗軍逐隊強制退兵，退不了的生還者當場被擒。
		// 原版的第三參數 0xffff 使玩家不能以空 Enter 取消。
		b.UseRoll(func(n int) int { return g.Roll(n, from, to, 0x258f2) })
		for _, side := range [...]battle.Side{loseMain, loseAid} {
			for _, u := range b.Units {
				if u.Side != side || !u.Alive() {
					continue
				}
				if err := b.ForceRetreat(u); err == nil {
					continue
				}
				for i := len(u.Leaders) - 1; i >= 0; i-- {
					if u.Leaders[i].InUnit() {
						b.Capture(placeSide, u, &u.Leaders[i])
					}
				}
			}
		}
	}
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
	factions := p.factions
	// 原版的 `0x26c08` 只在君主實際退場後執行。被擒名單本身不是
	// 分贓條件：釋放到其他郡的君主仍在位，不能搬他的寶庫。
	var fallenLord state.FactionID = state.NoFaction
	for _, u := range b.Units {
		for _, l := range u.Leaders {
			x := byIndex[l.Index]
			if x == nil {
				continue
			}
			if (l.Captured && l.Fate == battle.Defected) || l.Deserted {
				// 招降或投敵之後留在原部隊的佔位（`battle.enlist`）：
				// 人已經在對方的部隊裡，那一份才算數。
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
				if !p.autoAI && x.Status == state.StatusLord {
					fallenLord = x.Faction
				}
				g.retireBy(x, "battle")
			case l.Captured && l.Fate != battle.FateNone:
				// 在戰場上已經處置完（`battle.capture`），這裡只把下場搬回
				// 人物表。玩家當場處置的照樣列進被擒名單，君主與他的勢力先
				// 記下來——斬首之後就查不出他曾是誰的君主。
				if !b.Computer[l.CapturedBy] {
					r.Captives = append(r.Captives, Captive{General: l.Index, Name: l.Name,
						Lord: x.Status == state.StatusLord, Faction: x.Faction, Fate: l.Fate})
				}
				if !p.autoAI && x.Status == state.StatusLord &&
					(l.Fate == battle.Executed || l.Fate == battle.Released && l.ReleasedTo < 0) {
					fallenLord = x.Faction
				}
				g.applyFate(p, x, l, factions[l.CapturedBy])
			case l.Captured:
				r.Captives = append(r.Captives, Captive{General: l.Index, Name: l.Name})
			case l.Fate == battle.Defected:
				// 招降或陣前投敵之後在對方部隊裡的那一份：換勢力。
				g.applyFate(p, x, l, factions[u.Side])
			}
			if !p.autoAI && l.InUnit() {
				switch {
				case u.Retreated && u.RetreatTo > 0:
					x.Location = u.RetreatTo
				case u.Side == winAid && returnAid:
					x.Location = p.SidePrefecture(winAid)
				case u.Side == placeSide:
					x.Location = to
				}
			}
		}
	}
	sort.Slice(r.Captives, func(i, j int) bool {
		return r.Captives[i].General < r.Captives[j].General
	})

	if p.autoAI {
		// 電腦對電腦的安置在 `0x1E908` 裡面做完才回到 `0x20200` 加人望
		// （`0x1ec37` → `0x204b4`），所以收降看的是**加 2 之前**的人望。
		// 郡的所屬這裡不動——原版沒有「攻下」這個動作，歸屬由人物表在
		// 下一個郡回合入口重算（`RecomputeOwners`）；`PrefectureTook` 只是
		// 戰報上的字。
		g.placeAfterAIBattle(p, r.AttackerWon)
		r.PrefectureTook = r.AttackerWon
	} else {
		// `0x266a6`：戰場郡由勝方主軍（全滅時由援軍）佔據。
		// 原版把軍團統帥直接寫入州郡 offset 32，沒有魅力挑選。
		if dst != nil {
			dst.governor = b.Commander[placeSide]
			if chief := g.General(dst.governor); chief != nil &&
				chief.Location == to && chief.Status == state.StatusOfficer {
				chief.Status = state.StatusGovernor
			}
			g.RefreshGarrison(to)
			for _, n := range dst.Neighbours {
				g.RefreshGarrison(n)
			}
		}
		if returnAid {
			if home := g.Prefecture(p.SidePrefecture(winAid)); home != nil {
				home.Gold = clampTo(home.Gold+b.Gold[winAid], MaxGold)
				home.Rice = clampTo(home.Rice+b.Rice[winAid], MaxRice)
				g.RefreshGarrison(home.ID)
			}
			b.Gold[winAid], b.Rice[winAid] = 0, 0
		}
		r.PrefectureTook = r.AttackerWon && dst != nil && dst.Owner == by
		// 玩家戰術層在 `0x23d17` 將勝方勢力交給同一支分贓常式；
		// 電腦對電腦由 `placeAfterAIBattle` 內的退場事件執行，不能重複。
		winner := by
		if !r.AttackerWon {
			winner = p.factions[battle.MainDefender]
		}
		if fallenLord != state.NoFaction && fallenLord != winner &&
			g.Faction(fallenLord) != nil && g.Faction(winner) != nil {
			g.spoilsFromFallenLord(winner, fallenLord)
		}
	}

	// **人望：勝方 +2、敗方 −2**（`0x204b4`／`0x204e0`，`L0`），夾在 0–100。
	// 人望決定部下忠誠每年的漲跌（`Faction.Prestige`），所以打贏仗的
	// 諸侯不只多一個郡，麾下也更死心塌地。**守方是開打時記下的那個勢力**
	// （`0x20200` 入口存進 `-0xc(bp)` 的四格）——戰場郡的所屬這時已經
	// 被收尾的重整改過了。
	defender := p.factions[battle.MainDefender]
	winner, loser := by, defender
	if !r.AttackerWon {
		winner, loser = defender, by
	}
	g.shiftPrestige(winner, PrestigeOnWin)
	g.shiftPrestige(loser, -PrestigeOnWin)

	if p.autoAI {
		// 電腦對電腦（`0x1f82f`／`0x1f8ae`）：**四個軍團的隨軍錢糧全部
		// 收進守方那一郡**，不管誰贏。玩家路徑同樣把兩支主軍與
		// 敗方援軍收進戰場郡，但先送勝方援軍回來源郡（`docs/spec/020`）。
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
		// `0x266a6`／加強版 `0x23e70`：敗方援軍與雙方主軍的
		// 隨軍金米都進戰場郡，勝方援軍已在上面帶回來源郡。
		if dst != nil {
			dst.Gold = clampTo(b.Gold[loseAid]+b.Gold[battle.MainAttacker]+b.Gold[battle.MainDefender], MaxGold)
			dst.Rice = clampTo(b.Rice[loseAid]+b.Rice[battle.MainAttacker]+b.Rice[battle.MainDefender], MaxRice)
			g.ravagePlayerBattlefield(to)
		}
	}
	// 加強版打完（不分哪一條路）把四個參戰郡的守將清單各重整一次
	// （`0x1e503`–`0x1e52c`：`push 郡 / lcall 0x17fc8` 四次，郡不在 1..42
	// 的那一格在 `0x17fd4` 擋掉）；原版的 `0x20200` 尾巴沒有這四道。
	if g.Edition == state.EditionPlus {
		for _, n := range []int{from, to, p.aid.Attacker, p.aid.Defender} {
			if n >= 1 && n <= 42 {
				g.RefreshGarrison(n)
			}
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

// applyFate 把戰場上當場做的處置（`battle.capture`，`docs/spec/018` §2）
// 搬回人物表。
//
//   - 斬首（`0x25f6a`）：退場
//   - 囚禁（`0x260dc`）：身分 10、所屬 `0xFF`、所在是戰場；軍師先清軍師欄
//   - 釋放（`0x262b8`）：有去處就搬去、**所屬不變**，去處沒有現役就當主事者；
//     沒有去處是身分 8、所屬 `0xFF`、所在是戰場；軍師先清軍師欄
//   - 招降（`0x25b94` → `0x25cd2`）：忠誠 ＝ 判定算出來的值、身分部下、
//     所屬換成捕獲方、所在郡是戰場；原本是軍師的話舊主的軍師欄清空
//
// 囚禁與釋放不寫忠誠與兵（原版那幾支只寫身分、所屬、所在三格）。
func (g *State) applyFate(p *Pending, x *General, l battle.Leader, captor state.FactionID) {
	at := p.to
	dropChief := func() {
		if x.Status == state.StatusChief {
			if f := g.Faction(x.Faction); f != nil && f.Chief == x.Index {
				f.Chief = -1
			}
		}
	}
	switch l.Fate {
	case battle.Executed:
		g.retireBy(x, "beheaded")
	case battle.Jailed:
		dropChief()
		x.Status, x.Faction, x.Location = state.StatusStranded, state.NoFaction, at
	case battle.Released:
		if to := g.Prefecture(l.ReleasedTo); to != nil {
			if g.ActiveGenerals(to.ID) == 0 {
				to.governor = x.Index
			}
			x.Location = to.ID
			// 去處重整守將清單（`0x26442` → `0x1949e`）：君主回去就接主事者。
			g.RefreshGarrison(to.ID)
			break
		}
		dropChief()
		x.Status, x.Faction, x.Location = state.StatusAvailable, state.NoFaction, at
	case battle.Defected:
		dropChief()
		_ = g.DisposeCaptive(at, x.Index, Enlist, captor)
		x.Loyalty = uint8(l.Loyalty)
		// 招降來的兵是 0，陣前投敵的把兵一起帶過去（`0x25cd2`）。
		x.Soldiers = l.Soldiers
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
