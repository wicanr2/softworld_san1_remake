package game

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 8. 謀略（說明書 p.24–26）。
//
// 「諸侯拜封軍師後，軍師和諸侯所在地均可用計」——**沒有軍師就不能用計**。
//
// 成功率取決於四項（p.24–25）：我方軍師智力、派遣使者魅力、
// 我方君主人望、對方軍師智力。**四項是手冊列的，權重是 remake 選的。**

// Plot 是一種計謀。編號與手冊相同。
type Plot int

const (
	PlotTigerWolf   Plot = iota + 1 // 1 驅虎吞狼：教唆某郡發兵攻打他郡
	PlotFarNear                     // 2 遠交近攻：計誘他郡與我合攻鄰郡
	PlotForgery                     // 3 偽書使疑：造假離間他郡君臣，降低其部將忠誠
	PlotIncite                      // 4 策反人民：鼓動他郡人民反叛，減少米、金和人民忠誠
	PlotJointAttack                 // 5 聯合出兵：聯絡我方二郡合攻鄰郡
)

// String 讓計謀印得出中文。
func (p Plot) String() string {
	switch p {
	case PlotTigerWolf:
		return "驅虎吞狼"
	case PlotFarNear:
		return "遠交近攻"
	case PlotForgery:
		return "偽書使疑"
	case PlotIncite:
		return "策反人民"
	case PlotJointAttack:
		return "聯合出兵"
	}
	return "?"
}

const (
	// ⚠ **下面四項已經被原版的公式取代**（`PlotScore`，`L0`、`0x2dd66`）：
	// 原版比的是雙方「軍師與君主裡謀略較高的那位」，人望與使者魅力
	// 只扣分不加分，而且沒有擲骰。留著是為了讓 `docs/design/02`
	// 的對照表讀得下去。
	TuneChiefWeight     = 40 // 我方軍師智力
	TuneEnvoyWeight     = 25 // 派遣使者魅力
	TunePrestigeWeight  = 15 // 我方君主人望（這裡用君主魅力代表）
	TuneEnemyChiefBonus = 20 // 對方軍師智力（扣分）

	// TuneForgeryLoyalty 是偽書使疑降低的忠誠。
	TuneForgeryLoyalty = 15
	// TuneInciteLoss 是策反人民減少的金米與民眾忠誠百分比。
	// ⚠ TuneInciteLoss 已被 `Sabotage` 取代（`L0`、`0x2d6e0`）。
	TuneInciteLoss = 20
)

var ErrNoChief = fmt.Errorf("還沒拜封軍師，不能用計")

// PlotCost 是計謀的花費。
//
// 平時的五種計謀手冊**沒有給費用**（只有戰場上的六種計謀有：
// 火攻 600、水淹 500、誘敵 400、燒糧 300、圍攻 200、陷阱 100，p.32–34）。
// 這裡照戰場那一組的量級給，等對拍量到再換。
func PlotCost(p Plot) int {
	switch p {
	case PlotTigerWolf, PlotJointAttack:
		return 300
	case PlotFarNear:
		return 200
	case PlotForgery, PlotIncite:
		return 100
	}
	return 0
}

// plotChance 是成功率（0..95）。
// plotSucceeds 是「這一計成不成」（`L0`、`0x2dd66`，公式見 `PlotScore`）。
func (g *State) plotSucceeds(by state.FactionID, envoy *General, target int) bool {
	best := func(id state.FactionID) int {
		n := 0
		if c := g.Chief(id); c != nil {
			n = int(c.Intel)
		}
		if l := g.Lord(id); l != nil && int(l.Intel) > n {
			n = int(l.Intel)
		}
		return n
	}
	prestige := 0
	if f := g.Faction(by); f != nil {
		prestige = f.Prestige
	}
	mine := PlotScore(chiefIntelOf(g, by), lordIntelOf(g, by), prestige, int(envoy.Charm))
	p := g.Prefecture(target)
	if p == nil || !p.Owned() {
		return true // 無主的郡沒有人反制
	}
	return mine > best(p.Owner)
}

func chiefIntelOf(g *State, id state.FactionID) int {
	if c := g.Chief(id); c != nil {
		return int(c.Intel)
	}
	return 0
}

func lordIntelOf(g *State, id state.FactionID) int {
	if l := g.Lord(id); l != nil {
		return int(l.Intel)
	}
	return 0
}

// UsePlot 施行一個計謀。
//
// `[HARD]` **用計的地點只能是軍師或諸侯所在地**（說明書 p.24）。
// 目標必須是別人的郡。
func (g *State) UsePlot(from, target int, p Plot, envoyIndex int, by state.FactionID) (bool, error) {
	src, err := g.canOrder(from, by)
	if err != nil {
		return false, err
	}
	chief := g.Chief(by)
	if chief == nil {
		return false, ErrNoChief
	}
	lord := g.Lord(by)
	if !(chief.Location == from || (lord != nil && lord.Location == from)) {
		return false, fmt.Errorf("game: 只有軍師或諸侯所在地能用計")
	}
	dst := g.Prefecture(target)
	if dst == nil {
		return false, fmt.Errorf("game: 郡編號 %d 越界", target)
	}
	if dst.Owner == by {
		return false, fmt.Errorf("game: 不能對自己的郡用計")
	}
	envoy := g.General(envoyIndex)
	if envoy == nil || envoy.Faction != by || envoy.Location != from {
		return false, ErrUnknownUnit
	}
	cost := PlotCost(p)
	if src.Gold < cost {
		return false, ErrNoGold
	}
	src.Gold -= cost
	src.Commanded = true

	// **成敗照原版的分數對決**（`PlotScore`，`0x2dd66`）：兩邊各取
	// 「軍師與君主裡謀略較高的那位」，我方再依人望與使者魅力扣分。
	// 這一段沒有擲骰——原版就是硬碰硬。
	if !g.plotSucceeds(by, envoy, target) {
		return false, nil
	}
	switch p {
	case PlotForgery:
		// 降低其部將忠誠。
		for _, x := range g.Garrison(target) {
			if x.Faction == dst.Owner && x.HasLoyalty() &&
				x.Status != state.StatusLord {
				x.Loyalty = uint8(clampTo(int(x.Loyalty)-TuneForgeryLoyalty, 100))
			}
		}
	case PlotIncite:
		// 五刀一起下（`Sabotage`，`L0`）：民眾忠誠、洪水率、土地價值、
		// 米、金。原版沒有把它們拆成不同的計謀。
		g.Sabotage(target, int(envoy.Charm))
	case PlotTigerWolf, PlotFarNear, PlotJointAttack:
		// ⚠ **這三種要有「別人替我出兵」的機制才做得完整。**
		// 戰役的戰略層已經有了（`Attack`），但「教唆」與「合攻」牽涉
		// 第三方勢力的意願與助攻軍，那要等戰術層與外交狀態
		//（`docs/design/03-battle.md`）。這裡先只記成功，不產生出兵。
		return true, fmt.Errorf("game: %s 已成功，但出兵的部分還沒實作", p)
	}
	return true, nil
}

// ---- 原版電腦諸侯用的那一種計略（`L0`、`[base]`）------------------------

// PlotScore 是計略的成敗判定（原版 `0x2dd66`）。
//
//	我方 ＝ max(軍師的謀略, 君主的謀略)
//	人望 < 80     → 我方 += (人望 − 80) ÷ 10      ; 只扣不加
//	使者魅力 < 70 → 我方 += (魅力 − 70) ÷ 5       ; 只扣不加
//	對方 ＝ max(對方君主的謀略, 對方軍師的謀略)
//	我方 > 對方 → 得手
//
// **人望與使者魅力到了 80／70 就封頂**，所以主軸是雙方的謀略對決。
func PlotScore(chiefIntel, lordIntel, prestige, envoyCharm int) int {
	n := chiefIntel
	if lordIntel > n {
		n = lordIntel
	}
	if prestige < 80 {
		n += (prestige - 80) / 10
	}
	if envoyCharm < 70 {
		n += (envoyCharm - 70) / 5
	}
	return n
}

// SabotageCharmDiv 是五刀各自的除數（原版 `0x2d6e0`）。
const (
	SabotageLoyaltyDiv = 10  // 民眾忠誠：− RND(魅力 ÷ 10)
	SabotageFloodDiv   = 5   // 洪水率：  ＋ RND(魅力 ÷ 5)
	SabotageLandDiv    = 12  // 土地價值：− RND(魅力 ÷ 12)
	SabotageRiceBase   = 300 // 米：− 米 × 100 ÷ (RND(魅力) + 300)
	SabotageGoldBase   = 500 // 金：− 金 × 100 ÷ (RND(魅力) + 500)
	SabotageScale      = 100 // 上面兩式的係數，從記憶體讀出來是 100
)

// Sabotage 是計略得手之後對目標郡下的五刀（`L0`、`0x2d6e0`）。
//
// **每一刀的量都跟著使者的魅力走**，而且五刀一起下——原版沒有把它們
// 拆成不同的計謀。
//
// **掛在「策反人民」底下是量到的**（`L0`）：`0x2d6e0` 只有兩個呼叫端，
// `0x2d6d7` 在策反人民那支常式裡（`0x2d34c`–`0x2d88c`，選單字串
// `<策反人民>派細作到那一郡` 在 `0x2d46c`），`0x0e8ba` 是電腦諸侯的
// 計略。兩邊都傳人物 offset 11（魅力）。
func (g *State) Sabotage(target, envoyCharm int) {
	p := g.Prefecture(target)
	if p == nil {
		return
	}
	roll := func(n, salt int) int {
		if n < 1 {
			n = 1
		}
		return g.Roll(n, target, envoyCharm, salt)
	}
	p.PublicLoyalty = uint8(clampTo(
		int(p.PublicLoyalty)-roll(envoyCharm/SabotageLoyaltyDiv, 1), 100))
	p.FloodRate = uint8(clampTo(
		int(p.FloodRate)+roll(envoyCharm/SabotageFloodDiv, 2), 100))
	p.LandValue = uint8(clampTo(
		int(p.LandValue)-roll(envoyCharm/SabotageLandDiv, 3), 100))
	p.Rice -= p.Rice * SabotageScale / (roll(envoyCharm, 4) + SabotageRiceBase)
	p.Gold -= p.Gold * SabotageScale / (roll(envoyCharm, 5) + SabotageGoldBase)
}
