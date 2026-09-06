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
	PlotTigerWolf Plot = iota + 1 // 1 驅虎吞狼：教唆某郡發兵攻打他郡
	PlotFarNear                   // 2 遠交近攻：計誘他郡與我合攻鄰郡
	PlotForgery                   // 3 偽書使疑：造假離間他郡君臣，降低其部將忠誠
	PlotIncite                    // 4 策反人民：鼓動他郡人民反叛，減少米、金和人民忠誠
	PlotJointAttack               // 5 聯合出兵：聯絡我方二郡合攻鄰郡
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
	// 成功率的四項權重（百分比，加起來 100）。
	TuneChiefWeight     = 40 // 我方軍師智力
	TuneEnvoyWeight     = 25 // 派遣使者魅力
	TunePrestigeWeight  = 15 // 我方君主人望（這裡用君主魅力代表）
	TuneEnemyChiefBonus = 20 // 對方軍師智力（扣分）

	// TuneForgeryLoyalty 是偽書使疑降低的忠誠。
	TuneForgeryLoyalty = 15
	// TuneInciteLoss 是策反人民減少的金米與民眾忠誠百分比。
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
func (g *State) plotChance(by state.FactionID, envoy *General, target int) int {
	chief := g.Chief(by)
	if chief == nil {
		return 0
	}
	score := int(chief.Intel)*TuneChiefWeight +
		int(envoy.Charm)*TuneEnvoyWeight
	if lord := g.Lord(by); lord != nil {
		score += int(lord.Charm) * TunePrestigeWeight
	}
	// 對方軍師智力扣分。
	if p := g.Prefecture(target); p != nil && p.Owned() {
		if ec := g.Chief(p.Owner); ec != nil {
			score -= int(ec.Intel) * TuneEnemyChiefBonus
		}
	}
	return clampTo(score/100, 95)
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

	if g.roll(from, target, int(p), envoyIndex) >= g.plotChance(by, envoy, target) {
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
		dst.Gold = dst.Gold * (100 - TuneInciteLoss) / 100
		dst.Rice = dst.Rice * (100 - TuneInciteLoss) / 100
		dst.PublicLoyalty = uint8(clampTo(
			int(dst.PublicLoyalty)*(100-TuneInciteLoss)/100, 100))
	case PlotTigerWolf, PlotFarNear, PlotJointAttack:
		// ⚠ **這三種要有「別人替我出兵」的機制才做得完整。**
		// 戰役的戰略層已經有了（`Attack`），但「教唆」與「合攻」牽涉
		// 第三方勢力的意願與助攻軍，那要等戰術層與外交狀態
		//（`docs/design/03-battle.md`）。這裡先只記成功，不產生出兵。
		return true, fmt.Errorf("game: %s 已成功，但出兵的部分還沒實作", p)
	}
	return true, nil
}
