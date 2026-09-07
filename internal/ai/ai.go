// Package ai 是電腦諸侯的決策。
//
// **三個版本並列**，由旗標選：
//
//	base      三國演義（原版）的 AI —— 以還原原版行為為目標
//	plus      三國演義1加強版的 AI —— 同上，加強版另有一份
//	enhanced  remake 強化 AI —— remake 自己的，不宣稱與任何原版相同
//
// ⚠ **前兩個是還原，第三個是創作。** 兩者的驗收標準完全不同：
// `base`／`plus` 要與原版對拍（同一個局面下同一串命令），
// `enhanced` 只要「玩起來好」。把它們混在一起——例如在 `base` 裡
// 「順手改好一點」——會讓還原失去意義，而且**沒有人看得出來**，
// 因為兩者的輸出都是合法的命令。
//
// 還原用的 AI 目前**還沒解**（原版的決策程式碼還沒反組譯到）。
// 它們不會假裝自己會下棋：`Derived()` 回 false，`Plan` 回空，
// 呼叫端必須把這件事顯示出來。
package ai

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// Mode 是 AI 的版本。
type Mode string

const (
	ModeBase     Mode = "base"
	ModePlus     Mode = "plus"
	ModeEnhanced Mode = "enhanced"
)

// Modes 是全部三個版本，順序固定（旗標說明與選單都用它）。
func Modes() []Mode { return []Mode{ModeBase, ModePlus, ModeEnhanced} }

// Brain 是一個 AI。
type Brain interface {
	Mode() Mode

	// Name 是給人看的名字。
	Name() string

	// Derived 回報這個 AI 是不是已經**完整**從原版還原出來的。
	//
	// **false 表示它還不會下完整的棋**，不是「它比較弱」。呼叫端要把
	// 這件事顯示出來——一個安靜地什麼都不做的電腦諸侯，在畫面上看起來
	// 就只是「這個諸侯這回合沒動作」。
	//
	// ⚠ **`Coverage()` 的分母是十八，不是九。** 分派器是一條直線，
	// `0xe926`–`0xec1b` 連續十八個 `lcall far [bx+表]` 之後 `lret`，
	// 中間沒有分支（`docs/re/03` §1.4）。而且**滿分也不等於
	// `Derived()` 為真**：判斷式讀出來之後，表底下還有沒量到的量
	// （賞賜的忠誠增幅與挑人的排序鍵、原版的亂數產生器）。
	Derived() bool

	// Coverage 回報十八種行為裡解出了幾種。
	//
	// 原版的電腦諸侯每個郡每回合把**十八種行為都跑一遍**，每種各有
	// 自己的條件（`docs/mechanics/70-ai`）——不是「從十道命令裡挑一道」。
	// 所以進度是「十八分之幾」，不是布林值。
	Coverage() (done, total int)

	// Plan 回傳某個勢力這個月要下的命令。**不改變局面**。
	Plan(g *game.State, f state.FactionID) []game.Order
}

// New 依版本造一個 AI。
func New(m Mode) (Brain, error) {
	switch m {
	case ModeBase:
		return &faithful{mode: ModeBase, name: "三國演義（原版）"}, nil
	case ModePlus:
		return &faithful{mode: ModePlus, name: "三國演義1加強版"}, nil
	case ModeEnhanced:
		return &enhanced{}, nil
	}
	return nil, fmt.Errorf("ai: 不認識的版本 %q（有 %v）", m, Modes())
}

// faithful 是「以還原原版為目標」的 AI 的共同外殼。
//
// **只做已經從原版讀出來的行為。** 十八種行為裡解出三種
// （內政、訓練兵士、指定太守、指定軍師、賞賜物品、尋訪人才、登用人才）。
// 沒解出來的一律不做——填一個「差不多的」策略進去，之後就再也分不出
// 哪些行為是還原的、哪些是我編的。
type faithful struct {
	mode Mode
	name string
}

func (f *faithful) Mode() Mode           { return f.mode }
func (f *faithful) Name() string         { return f.name }
func (f *faithful) Derived() bool        { return false }
func (f *faithful) Coverage() (int, int) { return 16, 18 }

// Plan 只發出已經解出來的那一種行為。
//
// 內政（原版的分派表 `0x5534`，`docs/re/03` §1.4）：
//
//	r = RND(K)      K ＝ [4,4,4,3,3,2]，由勢力的 AI 等級選
//	r == 0 → 土地開發
//	r == 1 → 洪水防治
//	否則   → 這回合不做
//
// **等級越高範圍越小、動手的機率越大**：等級 5 是 `RND(2)`，兩件事
// 各半、從不閒著；等級 0 是 `RND(4)`，一半的回合什麼都不做。
//
// ⚠ **這裡只還原了「選哪一道」，沒有還原「做多少」。** 原版的量是
// 地力 `+= (智 − 50)/12`、洪水率 `-= 智/10`，而 remake 的 `Reclaim`／
// `FloodControl` 用的是自己的係數（`docs/design/02`）。要對拍得先把
// 那兩個量也接過來。
func (f *faithful) Plan(g *game.State, id state.FactionID) []game.Order {
	var out []game.Order
	k := internalAffairsRange(g.AILevel(id))
	for _, p := range g.Territory(id) {
		// **錢包要跟著這一輪扣。** 原版每一支常式開頭都看一次本回合的
		// 預算（`es:[0x3d16]`）；remake 這一邊沒有那個數，用郡的金頂著。
		// 對著開局餘額規劃的話，後面幾道會被 `ErrNoGold` 擋下來，
		// 而 `ApplyAll` 會連同再後面的命令一起作廢。
		purse := 0
		if x := g.Prefecture(p); x != nil {
			purse = x.Gold
		}
		afford := func(cost int) bool {
			if purse < cost {
				return false
			}
			purse -= cost
			return true
		}
		// **行動者不是太守**：表 `0x54d4` 按「智 ＋ 武 ＋ 加權表[身分]」
		// 排序，取第一位——君主優先，其次軍師、太守、一般武將。
		act := actor(g, id, p)
		if act == nil {
			continue
		}
		gov := act
		// 內政（表 `0x5534`）
		switch g.Roll(k, int(id), p, 0x5534) {
		case 0:
			// 開墾不會因為錢不夠而失敗（「若財庫已空則徒手開墾」）。
			purse -= min(purse, game.CostReclaim)
			out = append(out, game.ReclaimOrder{At: p, General: gov.Index})
		case 1:
			if afford(game.CostFloodControl) {
				out = append(out, game.FloodControlOrder{At: p, General: gov.Index})
			}
		}
		// 訓練兵士（表 `0x5554`）：分派器每回合都跑，常式自己對整個
		// 守軍算，沒有額外的條件。
		out = append(out, game.TrainOrder{At: p})
		// 指定軍師（表 `0x5694`）：跑在指定太守之前。
		if x := betterChief(g, id, p); x != nil {
			out = append(out, game.AppointChiefOrder{At: p, Target: x.Index})
		}
		// 指定太守（表 `0x5674`）：守軍按魅力由高到低排序，第一位當
		// 太守。**已經是他就不必再指一次**——原版那一段是直接寫欄位，
		// remake 這一邊走命令，重複指定會白費一道紀錄。
		if best := mostCharming(g, id, p); best != nil && best.Index != gov.Index {
			out = append(out, game.AppointGovernorOrder{At: p, Target: best.Index})
		}
		// 尋訪人才（表 `0x5614`）：`RND(10) > 7`，也就是 20 %。
		// **機率不隨等級變**（六個項目都推 10）。
		if g.Roll(10, int(id), p, 0x5614) > 7 && afford(game.CostSearch) {
			out = append(out, game.SearchOrder{At: p, General: gov.Index})
		}
		// 登用人才（表 `0x5634`）：掃本郡身分 8（在野露面）的人。
		// **每郡最多 50 位將軍**（`0xced2` 的 `cmpw es:[0xc],50`）。
		// 判定在 `game.Recruit`（`0xce8c`，`docs/re/03` §1.4）；
		// 等級參數 (30,0)/(20,10)/(10,20)/(0,40) 是**費用與加成**。
		// ⚠ 原版掃的是身分 8 **與 10**，而 10 是什麼還沒解；
		// 有好幾位可選時它挑誰也還沒讀。
		if len(g.Garrison(p)) < game.MaxGeneralsPerPrefecture &&
			afford(game.RecruitFee(g.AILevel(id))) {
			if who := f.recruitTarget(g, p); who != nil {
				out = append(out, game.RecruitOrder{At: p, Target: who.Index})
			}
		}
		// 賞賜物品（表 `0x56b4`）：**等級 0–2 完全不做**（那三格是空操作）。
		out = append(out, f.rewards(g, id, p)...)
		// 購置武器（表 `0x5594`）：預算是郡的金的 2 %。
		bought := armsPurchase(g, p, aiBudget(purse, g.AILevel(id), tableArms))
		out = append(out, bought...)
		// **扣的是真的花掉的，不是配下去的額度**：原版每一支常式都重讀
		// 一次郡的金，而金只被實際的支出扣減。
		for _, o := range bought {
			purse -= o.(game.ArmsOrder).Units / game.ArmsPerGold
		}
		// 徵兵（表 `0x5574`）：預算是**剩下的**金的 30–50 %。
		// 原版每一支常式都重讀一次郡的金，所以後面的表看到的是
		// 前面花剩的（`docs/mechanics/70-ai` §2.14）。
		out = append(out, conscript(g, p, aiBudget(purse, g.AILevel(id), tableConscript))...)
		// 調整兵力（表 `0x55b4`）：**不花錢，也不隨等級變**——六格全部
		// thunk 到同一支 `0xc2c4`。它把整郡的兵按帶兵上限重新攤平，
		// 訓練度與武裝度拉到全郡的加權平均。
		if who := garrisonIndices(g, p); len(who) >= 2 {
			out = append(out, game.RedistributeOrder{At: p, Units: who})
		}
		// 開倉賑民（表 `0x55f4`）：民眾忠誠低於「底 ＋ RND(20)」才做，
		// 撥的是**整份預算**（郡的金的 10–20 %）。
		if o, ok := relief(g, p, id, aiBudget(purse, g.AILevel(id), tableRelief)); ok {
			out = append(out, o)
			purse -= o.Gold
		}
		// 賞賜金帛（表 `0x5654`）：走守軍清單，君主自己不受賞，
		// 每人上限 100 金，發到預算用完為止。
		paid := rewardGold(g, p, id, aiBudget(purse, g.AILevel(id), tableReward))
		out = append(out, paid...)
		for _, o := range paid {
			purse -= o.(game.RewardOrder).Gold
		}
		// 買入米糧（表 `0x55d4`）：**不走回合預算也不打折**，
		// 它是市場交易。存糧目標跟著兵力走，不夠就用郡的金補到滿。
		if o, ok := buyRice(g, p, id, purse); ok {
			out = append(out, o)
			purse -= o.Units / game.RicePerGold(g.Prefecture(p).PriceLevel)
		}
		// 挖角（表 `0x56d4`）：**君主要在本郡**，機率隨等級 30／60／80 %，
		// 預算要 ≥ 100，費用是直接扣的 100 金。
		if o, ok := headhunt(g, p, id, purse); ok {
			out = append(out, o)
			purse -= game.CostHeadhunt
		}
		// 計略（表 `0x56f4`）：**軍師本人要在這個郡**，機率隨等級
		// 10／12.5／20 %；目標是**全圖**任何一個敵郡，使者取本郡魅力
		// 最高的人。
		if o, ok := plot(g, p, id); ok {
			out = append(out, o)
		}
	}
	return out
}

// 分派表的位址，當識別碼用。
const (
	tableArms      = 0x5594 // 購置武器
	tableConscript = 0x5574 // 徵兵
	tableRelief    = 0x55f4 // 開倉賑民
	tableReward    = 0x5654 // 賞賜金帛
	tableRice      = 0x55d4 // 買入米糧
	tableHeadhunt  = 0x56d4 // 挖角
	tablePlot      = 0x56f4 // 計略
)

// aiBudgetPercent 是「本回合預算佔郡的金的百分之幾」（`L0`、`[base]`）。
//
// 分派器在呼叫這幾支之前先算一次 `es:[0x3d16]`：
// `預算 ＝ 郡的金 × 係數 ÷ 100`，係數表在 `DS:0x5714`，
// 24 筆 ＝ 6 個等級 × 4 個相位（`docs/mechanics/70-ai` §2.14）。
//
// **這兩張表的係數不隨相位變**，所以「相位是什麼」（`es:[0x3f08] mod 4`，
// 還沒解）不影響這裡。會隨相位變的是 `0x5514` 與 `0x56d4`，
// 而 `0x5514` 八格全是空操作。
var aiBudgetPercent = map[int][6]int{
	tableArms:      {2, 2, 2, 2, 2, 2},
	tableConscript: {30, 30, 40, 50, 50, 50},
	tableRelief:    {20, 20, 10, 10, 20, 20},
	tableReward:    {20, 20, 20, 15, 15, 15},
}

func aiBudget(gold, level, table int) int {
	pct, ok := aiBudgetPercent[table]
	if !ok || gold <= 0 {
		return 0
	}
	if level < 0 {
		level = 0
	}
	if level >= len(pct) {
		level = len(pct) - 1
	}
	return gold * pct[level] / 100
}

// conscript 是「徵兵」（表 `0x5574`，常式 `0xbeb8`，`L0`、`[base]`）。
//
// 走守軍清單，對每一位徵到帶兵上限為止：
//
//	空額 ＝ 帶兵上限[職位] − 兵力
//	人數 ＝ min(空額, 預算)                 ; 每人 1 金
//	人數 ＝ min(人數, max(人口 − 3000, 0))  ; 人口下限
//	人數 ≤ 0 → 這一位跳過
//
// 徵完人口等量減少，訓練度與武裝度都按新的兵力重算——**新兵沒受訓
// 也沒武器**，兩個欄位走的是同一個加權平均（`game.ArmsOf`）。
func conscript(g *game.State, prefecture, budget int) []game.Order {
	p := g.Prefecture(prefecture)
	if p == nil {
		return nil
	}
	people := p.Population
	var out []game.Order
	for _, x := range g.Garrison(prefecture) {
		n := x.TroopCap() - x.Soldiers
		if n > budget {
			n = budget
		}
		if room := people - game.MinPopulationToConscript; n > room {
			n = room
		}
		if n <= 0 {
			continue
		}
		budget -= n
		people -= n
		out = append(out, game.ConscriptOrder{At: prefecture, General: x.Index, Count: n})
	}
	return out
}

// relief 是「開倉賑民」（表 `0x55f4`，常式 `0xc8f6`，`L0`、`[base]`）。
//
//	門檻 ＝ 門檻底[等級] ＋ RND(20)      ; 底 ＝ 80,80,70,60,80,80
//	民眾忠誠 >= 門檻 → 這回合不做
//	撥出整份預算，效果見 `game.ReliefGain`
//
// **等級 2、3 的門檻反而低**（70、60），做得比等級 0、1 少；
// 等級 4、5 門檻回到 80，但每一分錢換到的忠誠多（除數 9 與 7）。
func relief(g *game.State, prefecture int, id state.FactionID, budget int) (game.ReliefOrder, bool) {
	p := g.Prefecture(prefecture)
	if p == nil || budget <= 0 {
		return game.ReliefOrder{}, false
	}
	bar := game.ReliefThreshold(g.AILevel(id)) + g.Roll(20, int(id), prefecture, tableRelief)
	if int(p.PublicLoyalty) >= bar {
		return game.ReliefOrder{}, false
	}
	return game.ReliefOrder{At: prefecture, Gold: budget}, true
}

// rewardGold 是「賞賜金帛」（表 `0x5654`，常式 `0xd302`，`L0`、`[base]`）。
//
// 走守軍清單，**跳過身分 0（君主自己）**，每一位賞 `min(剩下的預算, 100)`
// ——說明書 p.23 的賞金上限 100 就是常式裡的 `cmp ax, 100`。
// 效果與反算回來的花費在 `game.Reward`。
func rewardGold(g *game.State, prefecture int, id state.FactionID, budget int) []game.Order {
	var out []game.Order
	for _, x := range g.Garrison(prefecture) {
		if budget <= 0 {
			break
		}
		if x.Faction != id || x.Status == state.StatusLord || x.Rewarded {
			continue
		}
		gold := budget
		if gold > game.MaxReward {
			gold = game.MaxReward
		}
		budget -= gold
		out = append(out, game.RewardOrder{At: prefecture, Target: x.Index, Gold: gold})
	}
	return out
}

// buyRice 是「買入米糧」（表 `0x55d4`，常式 `0xc634`，`L0`、`[base]`）。
//
//	量   ＝ (100 − 物價) ÷ 10                      ; 一金買到幾單位
//	目標 ＝ min(兵士（百）× (RND(10) + 12), 30000)  ; 存糧跟著兵力走
//	缺口 ＝ 目標 − 米
//	剩金 ＝ clamp(金 − 缺口 ÷ 量, 0, 30000)
//	買到 ＝ (金 − 剩金) × 量
//
// **下限是 0**——缺口夠大就把郡的金全部花光（`ds:[0xa5f2]` 讀出來是 0）。
// 這一支不經過折扣常式 `0xec24`，所以電腦諸侯買米沒有折扣。
func buyRice(g *game.State, prefecture int, id state.FactionID, purse int) (game.BuyRiceOrder, bool) {
	p := g.Prefecture(prefecture)
	if p == nil || purse <= 0 {
		return game.BuyRiceOrder{}, false
	}
	troops := 0
	for _, x := range g.Garrison(prefecture) {
		troops += x.Soldiers
	}
	want := troops / 100 * (g.Roll(10, int(id), prefecture, tableRice) + 12)
	if want > game.MaxRice {
		want = game.MaxRice
	}
	gap := want - p.Rice
	if gap <= 0 {
		return game.BuyRiceOrder{}, false
	}
	rate := game.RicePerGold(p.PriceLevel)
	spend := gap / rate
	if spend > purse {
		spend = purse
	}
	if spend <= 0 {
		return game.BuyRiceOrder{}, false
	}
	return game.BuyRiceOrder{At: prefecture, Units: spend * rate}, true
}

// headhunt 是「挖角」（表 `0x56d4`，`L0`、`[base]`）。
//
//	等級 0–2 不做（三格空操作）
//	君主的所在郡 != 本郡 → 不做
//	RND(10) <= K → 不做            ; K ＝ 6／3／1，也就是 30 % / 60 % / 80 %
//	本回合的錢 < 100 → 不做
//	掃全部人物挑候選（`game.Headhunt` 的 `headhuntable`），取第一位
//
// 費用 100 金是**直接扣的**，不經過等級折扣。
func headhunt(g *game.State, prefecture int, id state.FactionID, purse int) (game.HeadhuntOrder, bool) {
	level := g.AILevel(id)
	if level < 3 || purse < game.CostHeadhunt {
		return game.HeadhuntOrder{}, false
	}
	if lord := g.Lord(id); lord == nil || lord.Location != prefecture {
		return game.HeadhuntOrder{}, false
	}
	if g.Roll(10, int(id), prefecture, tableHeadhunt) <= headhuntBar(level) {
		return game.HeadhuntOrder{}, false
	}
	for _, x := range g.AllGenerals() {
		if !x.Employed() || x.Faction == id || x.Status == state.StatusLord {
			continue
		}
		if g.Headhuntable(x, prefecture) {
			return game.HeadhuntOrder{At: prefecture, Target: x.Index}, true
		}
	}
	return game.HeadhuntOrder{}, false
}

// headhuntBar 是 `RND(10) > K` 裡的 K：等級 3／4／5 ＝ 6／3／1。
func headhuntBar(level int) int {
	switch level {
	case 3:
		return 6
	case 4:
		return 3
	}
	return 1
}

// plot 是「計略」（表 `0x56f4`，`L0`、`[base]`）。
//
//	等級 0–2 不做（三格空操作）
//	RND(10 / 8 / 5) != 0 → 不做      ; 等級 3／4／5 ＝ 10 % / 12.5 % / 20 %
//	沒有軍師、或軍師不在這個郡 → 不做
//	掃全部 42 個郡，取「有主而且不是自己」的，隨機挑一個
//	使者 ＝ 本郡守軍裡魅力最高的
//
// 成敗與效果在 `game.PlotScore`／`game.Sabotage`。
func plot(g *game.State, prefecture int, id state.FactionID) (game.PlotOrder, bool) {
	level := g.AILevel(id)
	if level < 3 {
		return game.PlotOrder{}, false
	}
	chief := g.Chief(id)
	if chief == nil || chief.Location != prefecture {
		return game.PlotOrder{}, false
	}
	if g.Roll(plotRange(level), int(id), prefecture, tablePlot) != 0 {
		return game.PlotOrder{}, false
	}
	envoy := mostCharmingHere(g, id, prefecture)
	if envoy == nil {
		return game.PlotOrder{}, false
	}
	var targets []int
	for _, q := range g.Prefectures() {
		if q.Owned() && q.Owner != id {
			targets = append(targets, q.ID)
		}
	}
	if len(targets) == 0 {
		return game.PlotOrder{}, false
	}
	to := targets[g.Roll(len(targets), int(id), prefecture, tablePlot, 1)]
	return game.PlotOrder{
		At: prefecture, To: to, What: game.PlotIncite, Envoy: envoy.Index,
	}, true
}

// plotRange 是 `RND(n) == 0` 裡的 n：等級 3／4／5 ＝ 10／8／5。
func plotRange(level int) int {
	switch level {
	case 3:
		return 10
	case 4:
		return 8
	}
	return 5
}

// mostCharmingHere 是本郡守軍裡魅力最高的一位（**不看君主在不在**，
// 與 `mostCharming` 那個「指定太守」用的不同）。
func mostCharmingHere(g *game.State, id state.FactionID, prefecture int) *game.General {
	var best *game.General
	for _, x := range g.Garrison(prefecture) {
		if x.Faction != id {
			continue
		}
		if best == nil || x.Charm > best.Charm {
			best = x
		}
	}
	return best
}

// garrisonIndices 是這一郡守軍的槽號，照清單順序。
func garrisonIndices(g *game.State, prefecture int) []int {
	var out []int
	for _, x := range g.Garrison(prefecture) {
		out = append(out, x.Index)
	}
	return out
}

// armsPurchase 是「購置武器」（表 `0x5594`，常式 `0xc168`，`L0`、`[base]`）。
//
// 走守軍清單，對每一位算：
//
//	現有武器 ＝ 武裝度 × 兵力 ÷ 100
//	缺口     ＝ 兵力 − 現有武器           ; 目標是人人有武器
//	花費     ＝ 缺口 ÷ 100                ; 截斷
//	花費 > 本回合預算 → 缺口 ＝ 預算 × 100 ; 買到錢用完為止
//	缺口 <= 0 → 這一位跳過
//
// **目標一律是滿編**，沒有門檻也沒有機率——原版這一支不擲骰。
// 缺口為零才跳過，所以武裝度已經 100 的人不會被重買。
//
// ⚠ **原版的預算是勢力層級的**（`es:[0x3d16]`，§2.12），它怎麼算出來
// 還沒解（`L3`）。這裡拿郡的金當上限，因為那是 remake 這一邊唯一
// 擋得住的東西——`ApplyAll` 遇到買不起會整串中斷，不是少買一點。
func armsPurchase(g *game.State, prefecture, budget int) []game.Order {
	var out []game.Order
	for _, x := range g.Garrison(prefecture) {
		gap := x.Soldiers - game.Weapons(int(x.Arms), x.Soldiers)
		if cost := gap / game.ArmsPerGold; cost > budget {
			gap = budget * game.ArmsPerGold
		}
		if gap <= 0 {
			continue
		}
		budget -= gap / game.ArmsPerGold
		out = append(out, game.ArmsOrder{At: prefecture, General: x.Index, Units: gap})
	}
	return out
}

// rewards 是「賞賜物品」（表 `0x56b4`，`L0`、`[base]`）。
//
// 三種寶物各跑一次（諸侯 offset 16／17／18），每一次：
//
//	RND(100) > 40 → 跳過          ; 41 % 才進行
//	存量 <= RND(2) + 2 → 跳過      ; 手上要夠多才送得出去
//	排序守軍、挑一位**非君主**的
//	該人忠誠上升，寶物存量 −1
//
// ⚠ **忠誠上升多少還沒解**（`L3`）——常式裡看得到 0–100 的夾取，
// 增幅那一段在讀到的範圍之外。`game.GiftTreasure` 用的還是 remake
// 自己的幅度。
func (f *faithful) rewards(g *game.State, id state.FactionID, prefecture int) []game.Order {
	if g.AILevel(id) < 3 {
		return nil // 等級 0–2 那三格是空操作
	}
	fa := g.Faction(id)
	if fa == nil {
		return nil
	}
	var out []game.Order
	// 只有 offset 16／17／18 那三格會被送出去；14 是玉璽（不能送人）。
	for i, t := range []game.Treasure{
		game.TreasureBlade, game.TreasureBeauty, game.TreasureHorse,
	} {
		if g.Roll(100, int(id), prefecture, i, 0x56b4) > 40 {
			continue
		}
		if fa.Treasury[t] <= g.Roll(2, int(id), prefecture, i)+2 {
			continue
		}
		who := f.rewardTarget(g, id, prefecture)
		if who == nil {
			continue
		}
		out = append(out, game.GiftOrder{At: prefecture, Target: who.Index, What: t})
	}
	return out
}

// recruitTarget 是登用的對象：本郡身分 8（在野露面）的人。
//
// 原版還收身分 10——**那個編碼 remake 沒有**，還沒解出是什麼
// （`docs/mechanics/20-personnel`）。
func (f *faithful) recruitTarget(g *game.State, prefecture int) *game.General {
	for _, x := range g.Free(prefecture) {
		if x.Status == state.StatusAvailable {
			return x
		}
	}
	return nil
}

// rewardTarget 是賞賜的對象：守軍裡忠誠最低的非君主。
//
// 原版在挑人之前先排序清單並跳過身分 0（君主）。**排序的鍵還沒解**
// （`L3`），這裡用「忠誠最低」——那是最合理的猜測，而且標了出來。
func (f *faithful) rewardTarget(g *game.State, id state.FactionID, prefecture int) *game.General {
	var pick *game.General
	for _, x := range g.Garrison(prefecture) {
		if x.Faction != id || x.Status == state.StatusLord {
			continue
		}
		if pick == nil || x.Loyalty < pick.Loyalty {
			pick = x
		}
	}
	return pick
}

// betterChief 是守軍裡可以接任軍師的人（`L0`、`[base]`）。
//
// 原版的常式（`0xd7ae`）走守軍清單，條件是
//
//	智 > 門檻   且   身分 ∈ {太守, 一般武將}
//
// 門檻是**現任軍師的智**，沒有軍師時是 79——那正是說明書「受封軍師之人
// 謀略不得低於 80」。門檻在迴圈裡**不更新**，所以原版取的是清單順序中
// 最後一位合格者，不是智力最高的那位。
//
// ⚠ **「最後一位」跟著清單順序走，而清單順序還沒解**（`L3`）。
// 這裡取 remake 自己的守軍順序中的最後一位，形狀對、人選不保證相同。
func betterChief(g *game.State, id state.FactionID, prefecture int) *game.General {
	// **君主不在就拜不了軍師**（`game.AppointChief` 的 `requireLordAt`）。
	// 送出去只會被擋，然後同一輪後面的命令全部作廢。
	if lord := g.Lord(id); lord == nil || lord.Location != prefecture {
		return nil
	}
	floor := state.ChiefIntelFloor
	if cur := g.Chief(id); cur != nil {
		floor = int(cur.Intel)
	}
	var pick *game.General
	for _, x := range g.Garrison(prefecture) {
		if x.Faction != id || int(x.Intel) <= floor {
			continue
		}
		if x.Status != state.StatusGovernor && x.Status != state.StatusOfficer {
			continue
		}
		pick = x
	}
	return pick
}

// actorWeight 是「這回合誰行動」的身分加權（`L0`、`[base]`）。
//
// 原版的排序鍵是 **`智 + 武 + 加權表[身分]`**（`0xf1d7` 的
// `add ax, [bx+0x5986]`），表的內容從記憶體讀出來：
//
//	身分 0 君主 2000、1 軍師 1600、2 太守 1200、3 一般武將 800
//	身分 4–7 重複 2000／1600／1200／800
//	身分 8、9（在野）與 11（未登場）0、身分 10 是 400
//
// **權重完全壓過能力值**（智 ＋ 武 最多 200，而權重差是 400 的倍數），
// 所以排序實際上是「先看身分，同身分再比智 ＋ 武」。
var actorWeight = [12]int{2000, 1600, 1200, 800, 2000, 1600, 1200, 800, 0, 0, 400, 0}

// actor 是這個郡這回合的行動者（表 `0x54d4` 選出來的那一位）。
//
// 分派器的第一個呼叫把它寫進全域，**後面八種行為讀的都是它**
// （`docs/re/03` §1.4）。
func actor(g *game.State, id state.FactionID, prefecture int) *game.General {
	var best *game.General
	bestKey := -1
	for _, x := range g.Garrison(prefecture) {
		if x.Faction != id {
			continue
		}
		w := 0
		if int(x.Status) < len(actorWeight) {
			w = actorWeight[x.Status]
		}
		if k := int(x.Intel) + int(x.War) + w; k > bestKey {
			best, bestKey = x, k
		}
	}
	return best
}

// mostCharming 是守軍裡魅力最高的一位。
//
// 原版在指定太守之前先把守軍清單**按魅力由高到低排序**
// （`0xf600` 起的交換排序，比的是人物 offset 11），然後取第一位。
// 說明書只說「太守魅力越高，登用與賑民的效果越好」——這裡是 AI 實際
// 用的判準。
func mostCharming(g *game.State, id state.FactionID, prefecture int) *game.General {
	// **君主在的郡不指太守**：`game.AppointGovernor` 擋這一種，而
	// `ApplyAll` 把擋下來的命令當成違規、中斷同一輪後面全部的命令。
	// AI 不該送出套不上去的命令（`order.go` 的 `ApplyAll`）。
	if lord := g.Lord(id); lord != nil && lord.Location == prefecture {
		return nil
	}
	var best *game.General
	for _, x := range g.Garrison(prefecture) {
		if x.Faction != id {
			continue
		}
		if best == nil || x.Charm > best.Charm {
			best = x
		}
	}
	return best
}

// internalAffairsRange 是內政那張分派表的亂數範圍（`L0`、`[base]`）。
//
// 六份常式是同一段碼，只有 `mov ax,K` 的常數不同：
// 等級 0–2 是 `RND(4)`、3–4 是 `RND(3)`、5 是 `RND(2)`。
func internalAffairsRange(level int) int {
	switch {
	case level <= 2:
		return 4
	case level <= 4:
		return 3
	default:
		return 2
	}
}
