package game

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
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

var ErrNoChief = fmt.Errorf("還沒拜封軍師，不能用計")

// PlotCost 是計謀的花費：**平時的五種計謀不花錢**（`L0`、`[base]`）。
//
// 手冊沒有給費用，戰場上的六種計謀則明列 600/500/400/300/200/100
// （p.32–34），所以原本照那一組的量級猜過一輪。原版的碼裡沒有那筆帳：
// 選單前置只查軍師（`0x2c225`–`0x2c281`），派工的 dispatcher
// （`0x2c2ee`）與五支效果常式都沒有碰州郡 offset 18（金）——
// 策反人民那支動到金，動的是**目標郡**的金，那是效果不是費用。
//
// 攔阻用計的是軍師：沒有軍師不能用計，而且只有軍師（或君主）所在的郡
// 能用（`UsePlot`）。
func PlotCost(Plot) int { return 0 }

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
	return g.UsePlotPlan(from, p, PlotPlan{Envoy: envoyIndex, At: target}, by)
}

// UsePlotPlan 是完整版：出兵型的三種計謀要指定的郡不只一個（`PlotPlan`）。
func (g *State) UsePlotPlan(from int, p Plot, plan PlotPlan, by state.FactionID) (bool, error) {
	target := plan.At
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
	// 聯合出兵是我方兩郡合攻，沒有出使的對象、也沒有使者。
	if p == PlotJointAttack {
		g.endTurn(src)
		return g.jointAttack(plan, by)
	}
	dst := g.Prefecture(target)
	if dst == nil {
		return false, fmt.Errorf("game: 郡編號 %d 越界", target)
	}
	if dst.Owner == by {
		return false, fmt.Errorf("game: 不能對自己的郡用計")
	}
	envoy := g.General(plan.Envoy)
	if envoy == nil || envoy.Faction != by || envoy.Location != from {
		return false, ErrUnknownUnit
	}
	g.endTurn(src)

	// 遠交近攻與驅虎吞狼**先播場景圖再判定**：玩家的選單常式填藍、載 `SCG18`／
	// `SCG23`、`0x32dfa` 擲 `RND(4)`，然後才叫 `0x2dd66`（`0x2c8de`→`0x2c926`、
	// `0x2cd6e`→`0x2cdb6`）。那一擲不看成敗。這兩處在玩家的選單常式裡，電腦
	// 諸侯不走（`L2`：電腦這兩計的路沒有量過）。
	if (p == PlotFarNear || p == PlotTigerWolf) && g.playerCommand(by) {
		g.plotScene(from, target, p, by)
	}

	// **成敗照原版的分數對決**（`PlotScore`，`0x2dd66`）：兩邊各取
	// 「軍師與君主裡謀略較高的那位」，我方再依人望與使者魅力扣分。
	// 這一段沒有擲骰——原版就是硬碰硬。
	if !g.plotSucceeds(by, envoy, target) {
		return false, nil
	}
	// 離間君臣與策反人民的場景圖在**得手之後的效果常式末尾**（`0x2d1fa` 的
	// `0x2d338`、`0x2d6e0` 的 `0x2d879`）：效果的骰擲完才擲 `RND(4)`，沒得手
	// 就不擲。效果常式電腦與玩家共用，電腦也擲（月度對拍：電腦策反人民沒得手
	// 的那個月，計略那一段原版抽 34 次、沒有 `RND(4)`）。
	switch p {
	case PlotForgery:
		g.Forgery(target, int(envoy.Charm))
		g.plotScene(from, target, p, by)
	case PlotIncite:
		// 五刀一起下（`Sabotage`，`L0`）：民眾忠誠、洪水率、土地價值、
		// 米、金。原版沒有把它們拆成不同的計謀。
		g.Sabotage(target, int(envoy.Charm))
		g.plotScene(from, target, p, by)
	case PlotTigerWolf:
		// 教唆出使郡去打它的鄰郡，我方不參戰（`0x2ce5b`）。
		if _, err := g.launchCampaign(target, plan.Strike, Aid{}); err != nil {
			return true, err
		}
	case PlotFarNear:
		// 我方從 Ours 出兵，出使郡當助攻軍（`0x2c9cd`）。
		if _, err := g.launchCampaign(plan.Ours, plan.Strike,
			Aid{Attacker: target}); err != nil {
			return true, err
		}
	}
	return true, nil
}

// plotScene 擲四種帶使者的計謀那一擲 `RND(4)`，玩家自己的命令再把場景圖
// 排進畫面（`docs/spec/010` §1.1）：遠交近攻 `SCG18`、驅虎吞狼 `SCG23` 在
// (432,80)，離間君臣 `SCG13`、策反人民 `SCG22` 在 (432,120)。
func (g *State) plotScene(from, target int, p Plot, by state.FactionID) {
	scene, x, y := 0, assets.SceneMainX, assets.SceneMainY
	switch p {
	case PlotFarNear:
		scene = assets.ScenePlotFarNear
	case PlotTigerWolf:
		scene = assets.ScenePlotTiger
	case PlotForgery:
		scene, x, y = assets.ScenePlotSow, assets.ScenePlotX, assets.ScenePlotY
	case PlotIncite:
		scene, x, y = assets.ScenePlotRevolt, assets.ScenePlotX, assets.ScenePlotY
	default:
		return
	}
	style := g.Roll(EffectVariants, from, target, int(p))
	if g.playerCommand(by) {
		g.showScene(from, scene, style, x, y)
	}
}

// jointAttack 是「聯合出兵」（原版 `0x2d88c`）。
//
// 它**不判計謀成不成**——我方兩個郡合攻，不需要說服誰。那一次
// `PlotScore` 判的是**守方求不求得到援軍**（`0x2dc51`，魅力填 70）：
// 我方壓過守方，守方就孤立無援；壓不過，守方的鄰郡會出一支助守軍。
func (g *State) jointAttack(plan PlotPlan, by state.FactionID) (bool, error) {
	dst := g.Prefecture(plan.Strike)
	if dst == nil {
		return false, fmt.Errorf("game: 郡編號 %d 越界", plan.Strike)
	}
	if dst.Owner == by {
		return false, fmt.Errorf("game: 不能打自己的郡")
	}
	if plan.OursAid == plan.Ours || plan.OursAid == 0 {
		// 「聯合」至少要兩個郡，原版一個都挑不到時印「無法聯合出兵」。
		return false, fmt.Errorf("game: 無法聯合出兵")
	}
	aid := Aid{Attacker: plan.OursAid}
	if !g.outwitsDefender(by, plan.Strike) {
		aid.Defender = g.DefenderAid(plan.Strike)
	}
	if _, err := g.launchCampaign(plan.Ours, plan.Strike, aid); err != nil {
		return false, err
	}
	return true, nil
}

// outwitsDefender 是聯合出兵那一次的分數對決（`0x2dc51`）。
func (g *State) outwitsDefender(by state.FactionID, target int) bool {
	prestige := 0
	if f := g.Faction(by); f != nil {
		prestige = f.Prestige
	}
	mine := PlotScore(chiefIntelOf(g, by), lordIntelOf(g, by), prestige, JointAttackCharm)
	p := g.Prefecture(target)
	if p == nil || !p.Owned() {
		return true
	}
	theirs := chiefIntelOf(g, p.Owner)
	if n := lordIntelOf(g, p.Owner); n > theirs {
		theirs = n
	}
	return mine > theirs
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
// **掛在「策反人民」底下是量到的**（`L0`）：`0x2d6e0` 的呼叫端
// `0x2d6d7` 在策反人民那支常式裡（`0x2d34c`–`0x2d88c`，選單字串
// `<策反人民>派細作到那一郡` 在 `0x2d46c`），傳的是人物 offset 11
// （魅力）。**這一刀只有玩家下得了**——電腦諸侯那條鏈用的是偽書使疑
// （`Forgery`，`docs/mechanics/70-ai` §2.13.7）。
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

// ForgeryScale 是偽書使疑那條乘法的分母（原版 `0xfa` ＝ 250，`0x2d263`）。
const ForgeryScale = 250

// Forgery 是「偽書使疑」得手之後對目標郡下的手（`L0`、`0x2d1fa`）。
//
//	忠誠 < 使者魅力 的人：
//	    忠誠 ← 忠誠 × (250 − RND(魅力 ÷ 2) − 魅力) ÷ 250
//	    算成負數就歸零
//
// **門檻與幅度都跟著使者的魅力走**：魅力愈高牽連的人愈多、掉得也愈多。
// 魅力 100 時係數落在 0.40–0.60，魅力 60 時是 0.64–0.76。
//
// 原版逐一掃該郡的武將名單（`0xf17:0x0aae` 取名單，`0x2d21e` 起的迴圈），
// **不挑身分**——君主在自己的郡裡也照算。
//
// **電腦諸侯的「計略」也是這一支**（`0x0e79e`），五種計謀裡只有它接在
// 電腦的決策表底下（`docs/mechanics/70-ai` §2.13.7）。
func (g *State) Forgery(target, envoyCharm int) {
	dst := g.Prefecture(target)
	if dst == nil {
		return
	}
	// 名單是 `buildRoster(目標郡, 模式 2)`（`0x2d20e`）——**不比對勢力**，
	// 混編的郡裡別家的人也會被離間。
	for _, x := range g.Garrison(target) {
		// **忠誠讀的是有號位元組**（`0x2d234` 的 `cbtw`）。哨兵值 `0xFF`
		// 因此是 −1 而不是 255——差別只在「哨兵會不會被當成全場最忠誠
		// 的人」。實務上碰不到：`0xFF` 是**在野者**的哨兵（`docs/spec/003`
		// §人物 offset 16，346 位裡 227 位），而在野者身分是 8／9，
		// `buildRoster` 模式 2 只收身分 ≤ 3，兩邊都進不了名單。
		// 照有號讀是為了與原版逐位元組一致，不是為了改變結果。
		loyal := int(int8(x.Loyalty))
		if loyal >= envoyCharm {
			continue
		}
		roll := g.Roll(max(envoyCharm/2, 1), target, envoyCharm, loyal)
		n := loyal * (ForgeryScale - roll - envoyCharm) / ForgeryScale
		if n < 0 {
			n = 0
		}
		x.Loyalty = uint8(n)
	}
}
