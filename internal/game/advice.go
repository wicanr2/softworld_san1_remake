package game

import "github.com/wicanr2/softworld_san1_remake/internal/state"

// 軍師勸諫（`docs/spec/005` §9.6、`docs/re/12` §3，`L0`＋`L2`、`[base]`）。
//
// 玩家下命令時原版先擲 `RND(5)`，`RND(5) + 80 < 軍師的謀略`（`es:[0x16f2]`）
// 才開口——沒有軍師那一格是 0xFFFF，帶號比較永遠不成立，**但那一擲照抽**。
// 開口的形式是下格的訊息框、肖像在左、說話者是軍師（`es:0x3b96`），
// 接著印「主公是否繼續呢(Y/N):」，按 N 取消整道命令。
//
// 大多數命令一句話固定；買賣、調動、尋訪、登用、挖角與計略要看情況挑句，
// 其中「看得準不準」的那幾支再擲一次 `RND(18) + 80`，大於軍師的謀略就改成
// 隨機挑（`0x1ba63`、`0x1beff`、`0x1d95d`、`0x2c7d3`）。

// AdviceKind 是哪一道命令的勸諫。
type AdviceKind int

const (
	AdviceNone      AdviceKind = iota
	AdviceAttack               // 發動戰役（`0x18b08`，361）
	AdviceMove                 // 調動軍隊（`0x18f3f`，358／359）
	AdviceTrain                // 訓練（`0x19797`，363）
	AdviceConscript            // 徵兵（`0x19967`，365）
	AdviceArms                 // 武裝（`0x19d07`，367）
	AdviceBalance              // 調整兵力（`0x19ff9`，369）
	AdviceRest                 // 休息（`0x1a5cf`，377）
	AdviceReclaim              // 開墾（`0x1a697`，371）
	AdviceFlood                // 治水（`0x1a8aa`，373）
	AdviceFort                 // 築關（`0x1aa53`，375）
	AdviceBuy                  // 買米（`0x1b187`，378／380）
	AdviceSell                 // 賣米（`0x1b415`，379／381）
	AdviceRelief               // 賑民（`0x1b6cb`，382）
	AdviceSearch               // 尋訪（`0x1bae9`，385–387）
	AdviceRecruit              // 登用（`0x1bf85`，388／389）
	AdviceReward               // 賞賜（`0x1c258`，395）
	AdviceDismiss              // 撤職（`0x1c624`，396）
	AdviceHeadhunt             // 挖角（`0x1d9ed`，406／407）
	AdvicePlot                 // 計略（`0x2c3b2` 等，412–416，再加一則預測 417／418 或 419／420）
)

// 勸諫的門檻（`0x18a90` 等十九處同一道門）與「準不準」那一擲。
const (
	AdviceGuessSpread = 18 // RND(18) + 80 > 軍師謀略 → 隨機挑句
	AdvicePriceFloor  = 48 // 買賣：物價 >= RND(5) + 48 算「高」
	AdvicePriceSpread = 5
)

// Advice 是一道命令的勸諫：要秀的訊息框（零到兩格）。
type Advice struct {
	Events []Event
}

// AdviceTarget 是勸諫要看的對象：登用／挖角的目標、計略的目的郡與種類。
type AdviceTarget struct {
	Target int
	To     int
	What   Plot
}

// Advise 擲軍師勸諫那一擲，軍師開口就回他要說的話；不開口回 nil。
// **每一道命令都會擲**，與原版同一個順序。
func (g *State) Advise(kind AdviceKind, at int, by state.FactionID, t AdviceTarget) *Advice {
	roll := g.Roll(AdvisorWarnSpread, at, int(kind), 0x18a90)
	chief := g.Chief(by)
	if chief == nil || chief.Name == "" || !AdvisorWarns(int(chief.Intel), roll) {
		return nil
	}
	intel := int(chief.Intel)
	p := g.Prefecture(at)
	say := func(text string, salt int) Event { return g.bubbleEvent(chief, false, true, text, at, salt) }
	var ev []Event
	switch kind {
	case AdviceAttack:
		ev = append(ev, say(t_("bub.adv.attack"), 361))
	case AdviceMove:
		// 郡裡的現役將就是要走的那一位 → 走了沒人守（`0x18ee6`）。
		if p != nil && g.ActiveGenerals(at) <= 1 {
			ev = append(ev, say(t_("bub.adv.moveLast"), 358))
		} else {
			ev = append(ev, say(t_("bub.adv.move"), 359))
		}
	case AdviceTrain:
		ev = append(ev, say(t_("bub.adv.train"), 363))
	case AdviceConscript:
		ev = append(ev, say(t_("bub.adv.conscript"), 365))
	case AdviceArms:
		ev = append(ev, say(t_("bub.adv.arms"), 367))
	case AdviceBalance:
		ev = append(ev, say(t_("bub.adv.balance"), 369))
	case AdviceRest:
		ev = append(ev, say(t_("bub.adv.rest"), 377))
	case AdviceReclaim:
		ev = append(ev, say(t_("bub.adv.reclaim"), 371))
	case AdviceFlood:
		ev = append(ev, say(t_("bub.adv.flood"), 373))
	case AdviceFort:
		ev = append(ev, say(t_("bub.adv.fort"), 375))
	case AdviceBuy, AdviceSell:
		// 軍師先評價行情（`0x1b112`／`0x1b3a0`）：物價 >= RND(5)+48 算高。
		high := p != nil && int(p.PriceLevel) >= AdvicePriceFloor+g.Roll(AdvicePriceSpread, at, 0x1b112)
		switch {
		case kind == AdviceBuy && high:
			ev = append(ev, say(t_("bub.adv.buyHigh"), 378))
		case kind == AdviceBuy:
			ev = append(ev, say(t_("bub.adv.buyLow"), 380))
		case high:
			ev = append(ev, say(t_("bub.adv.sellHigh"), 379))
		default:
			ev = append(ev, say(t_("bub.adv.sellLow"), 381))
		}
	case AdviceRelief:
		ev = append(ev, say(t_("bub.adv.relief"), 382))
	case AdviceSearch:
		// 郡裡有沒有可找的人（身分 9）決定 385 或 387；軍師不夠聰明就亂猜
		// 385–387（`0x1ba51`–`0x1ba8c`）。
		key := "bub.adv.searchNone"
		for i := range g.generals {
			c := &g.generals[i]
			if c.Location == at && c.Status == state.StatusIdle && c.Name != "" {
				key = "bub.adv.searchYes"
				break
			}
		}
		if AdviceGuessSpread > 0 && g.Roll(AdviceGuessSpread, at, 0x1ba63)+AdvisorWarnFloor > intel {
			key = []string{"bub.adv.searchYes", "bub.adv.searchMaybe", "bub.adv.searchNone"}[g.Roll(3, at, 0x1ba81)]
		}
		ev = append(ev, say(t_(key), 385))
	case AdviceRecruit:
		// 看得準的軍師照牽絆說（`0x1beed`）：目標的牽絆對象效力於我方就
		// 「必會前來」、效力於別人就「不會加入」；其餘照能力值那一半的
		// 平均猜（remake 差異，`L2`）。不夠聰明就亂猜。
		key := "bub.adv.recruitNo"
		if x := g.General(t.Target); x != nil && g.recruitLooksEasy(x, at, by) {
			key = "bub.adv.recruitYes"
		}
		if g.Roll(AdviceGuessSpread, at, 0x1beff)+AdvisorWarnFloor > intel {
			key = []string{"bub.adv.recruitYes", "bub.adv.recruitNo"}[g.Roll(2, at, 0x1bf1d)]
		}
		ev = append(ev, say(t_(key), 388))
	case AdviceReward:
		ev = append(ev, say(t_("bub.adv.reward"), 395))
	case AdviceDismiss:
		ev = append(ev, say(t_("bub.adv.dismiss"), 396))
	case AdviceHeadhunt:
		// `0x1d95d`：RND(18)+80 < 謀略才真的判（`0x1dc0a` > 0 → 必來投靠），
		// 否則亂猜。
		key := "bub.adv.headhuntNo"
		if g.Roll(AdviceGuessSpread, at, 0x1d95d)+AdvisorWarnFloor < intel {
			if x := g.General(t.Target); x != nil && g.headhuntLooksEasy(x, by) {
				key = "bub.adv.headhuntYes"
			}
		} else if g.Roll(2, at, 0x1d97b) == 0 {
			key = "bub.adv.headhuntYes"
		}
		ev = append(ev, say(t_(key), 406))
	case AdvicePlot:
		key := map[Plot]string{PlotTigerWolf: "bub.adv.plotTiger", PlotFarNear: "bub.adv.plotDistant",
			PlotForgery: "bub.adv.plotForge", PlotIncite: "bub.adv.plotIncite", PlotJointAttack: "bub.adv.plotJoint"}[t.What]
		if key == "" {
			return nil
		}
		ev = append(ev, say(t_(key), 412))
		// 再一則預測：目的郡的主事者「乃無用之輩 此計必成」或「深具謀略
		// 此計不易成功」（`0x2c79d`，判定 `0x2dd66` 不擲骰）；聯合出兵那一支
		// 預測的是守方求不求得到援軍（419／420）。不夠聰明就亂猜。
		if t.What == PlotJointAttack {
			key = "bub.adv.jointFast"
			if g.jointNeedsHelp(by, t.To) {
				key = "bub.adv.jointHelp"
			}
			ev = append(ev, say(t_(key), 419))
		} else if gov := g.Governor(t.To); gov != nil {
			envoy := g.General(t.Target)
			ok := envoy != nil && g.plotSucceeds(by, envoy, t.To)
			if g.Roll(AdviceGuessSpread, at, 0x2c7d3)+AdvisorWarnFloor > intel {
				ok = g.Roll(2, at, 0x2c7f1) == 0
			}
			key = "bub.adv.plotHard"
			if ok {
				key = "bub.adv.plotEasy"
			}
			ev = append(ev, say(tf(key, personName(gov.Name)), 417))
		}
	default:
		return nil
	}
	return &Advice{Events: ev}
}

// recruitLooksEasy 是勸諫用的登用預判：牽絆閘門照原版，能力值那一半拿
// 兩次 RND(4) 的中點代替（`L2`，不擲骰，免得動到真正登用那一串亂數）。
func (g *State) recruitLooksEasy(t *General, at int, by state.FactionID) bool {
	if b := g.General(t.Bond); b != nil && b.Index != t.Index && b.Employed() {
		return b.Faction == by
	}
	charm := 50
	if gov := g.Governor(at); gov != nil {
		charm = int(gov.Charm)
	}
	prestige := 0
	if f := g.Faction(by); f != nil {
		prestige = f.Prestige
	}
	return RecruitPersuasion(prestige, charm, 0) > RecruitDifficulty(int(t.Intel), int(t.War), 2, 2)
}

// headhuntLooksEasy 是勸諫用的挖角預判（`0x1dc0a` 的無骰版）：忠誠、
// 戰力、謀略與對方人望堆出來的抵抗，對上我方君主的魅力與人望。
func (g *State) headhuntLooksEasy(t *General, by state.FactionID) bool {
	if b := g.General(t.Bond); b != nil && b.Index != t.Index && b.Employed() && b.Faction == t.Faction {
		return false
	}
	lordCharm, mine, theirs := 50, 0, 0
	if l := g.Lord(by); l != nil {
		lordCharm = int(l.Charm)
	}
	if f := g.Faction(by); f != nil {
		mine = f.Prestige
	}
	if f := g.Faction(t.Faction); f != nil {
		theirs = f.Prestige
	}
	return HeadhuntOffer(lordCharm, mine, 0) > HeadhuntResistance(int(t.Loyalty), int(t.War), int(t.Intel), theirs)
}

// jointNeedsHelp 是聯合出兵的預測：守方求不求得到援軍（`0x2dc51`：
// `0x2dd66(我方郡, 目的郡, 70)` 回 −1 才找援軍；魅力填 70 是不扣分的值）。
func (g *State) jointNeedsHelp(by state.FactionID, to int) bool {
	p := g.Prefecture(to)
	if p == nil || !p.Owned() {
		return false
	}
	prestige := 0
	if f := g.Faction(by); f != nil {
		prestige = f.Prestige
	}
	mine := PlotScore(chiefIntelOf(g, by), lordIntelOf(g, by), prestige, JointHelpCharm)
	theirs := chiefIntelOf(g, p.Owner)
	if l := lordIntelOf(g, p.Owner); l > theirs {
		theirs = l
	}
	return mine <= theirs
}

// JointHelpCharm 是聯合出兵問守方求不求援時填的魅力（`0x2dc51` 的 0x46）。
const JointHelpCharm = 70
