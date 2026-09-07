package game

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 出兵型的三種計謀（驅虎吞狼、遠交近攻、聯合出兵）。
//
// 這三種的結局都是**發動一場戰役**：原版直接呼叫戰鬥子系統
// `battle(攻方郡, 攻方援郡, 守方郡, 守方援郡)`（`0x20200`），
// 三支各自填不同的格子（`L0`、`[base]`）：
//
//	驅虎吞狼 0x2ce5b   battle(出使郡, −1, 被驅使去打的郡, −1)
//	遠交近攻 0x2c9cd   battle(我方出兵郡, 出使郡, 聯合攻打的郡, −1)
//	聯合出兵 0x2dd57   battle(我方出兵郡, 我方另一郡, 聯合攻打的郡, 守方援郡)
//
// 「助攻軍必須運用謀略才能得到」（說明書 p.26）指的就是這件事。
// 驅虎吞狼**我方完全不參戰**——兩個格子都是別人的郡。

// PlotPlan 是一個計謀要指定的郡與人，對應原版每一支的那幾個選單。
//
// 哪幾格有意義隨計謀而定；用不到的格子留 0。
type PlotPlan struct {
	// Envoy 是去遊說的人（`<…>派那一位去遊說`）。聯合出兵不用使者。
	Envoy int

	// At 是計謀施行的對象郡：驅虎吞狼與遠交近攻的「出使那一郡」、
	// 偽書使疑與策反人民的「派細作到那一郡」。
	At int

	// Strike 是要被打的郡：驅虎吞狼的「驅使攻打那一郡」、
	// 遠交近攻與聯合出兵的「聯合攻打那一郡」。
	Strike int

	// Ours 是我方出兵的郡：遠交近攻的「聯合我方那一郡」、
	// 聯合出兵的「從我方那一郡出兵」。
	Ours int

	// OursAid 是聯合出兵的「聯合我方那一郡合攻」。
	OursAid int
}

// JointAttackCharm 是聯合出兵那一次判定用的魅力（原版寫死 0x46 ＝ 70，
// `0x2dc51`）。
//
// `PlotScore` 對魅力的規則是「低於 70 才扣分」，所以填 70 等於不扣——
// 聯合出兵沒有使者，這一格就用一個不影響結果的值頂著。
const JointAttackCharm = 70

// EnvoyTargets 是「出使那一郡」的候選（驅虎吞狼與遠交近攻，`0x2c401`）。
//
//	有主、而且不是我方的郡
//
// 遠交近攻另外要求**距我方兩步以內**（`FarNearEnvoyTargets`）。
func (g *State) EnvoyTargets(by state.FactionID) []int {
	var out []int
	for i := 1; i <= state.PrefectureCount; i++ {
		p := g.Prefecture(i)
		if p == nil || !p.Owned() || p.Owner == by {
			continue
		}
		out = append(out, i)
	}
	return out
}

// FarNearEnvoyTargets 是遠交近攻第一個選單的候選（原版 `0x2c453` 的第二輪）：
//
//	有主、非我方，而且**它的鄰郡的鄰郡裡有我方的郡**
//
// 也就是隔著一個郡就能碰到我方——遠交的「遠」只有兩步。
func (g *State) FarNearEnvoyTargets(by state.FactionID) []int {
	var out []int
	for _, i := range g.EnvoyTargets(by) {
		if g.withinTwoSteps(i, by) {
			out = append(out, i)
		}
	}
	return out
}

func (g *State) withinTwoSteps(from int, by state.FactionID) bool {
	p := g.Prefecture(from)
	if p == nil {
		return false
	}
	for _, n := range p.Neighbours {
		q := g.Prefecture(n)
		if q == nil {
			continue
		}
		for _, m := range q.Neighbours {
			if r := g.Prefecture(m); r != nil && r.Owned() && r.Owner == by {
				return true
			}
		}
	}
	return false
}

// TigerWolfStrikeTargets 是驅虎吞狼第二個選單的候選（原版 `0x2cb4c`）：
//
//	出使郡的鄰郡、有主、既不是我方也不是出使郡那一方
//
// **驅虎吞狼對出使郡沒有距離限制**（`0x2caa6` 只查「有主、非我方」），
// 我方也不出兵——教唆的是兩個別人的郡互打。
func (g *State) TigerWolfStrikeTargets(envoyAt int, by state.FactionID) []int {
	return g.strikeTargets(envoyAt, by, false)
}

// FarNearStrikeTargets 是遠交近攻第二個選單的候選（原版 `0x2c55c`）：
//
//	與出使郡相鄰、有主、既不是我方也不是出使郡那一方
//
// 再篩一輪還要**與我方相鄰**（`0x2c5d3`）——夾在我方與遠方盟友之間的
// 那一個郡，正是「近攻」的對象。
func (g *State) FarNearStrikeTargets(envoyAt int, by state.FactionID) []int {
	return g.strikeTargets(envoyAt, by, true)
}

func (g *State) strikeTargets(envoyAt int, by state.FactionID, nextToUs bool) []int {
	src := g.Prefecture(envoyAt)
	if src == nil || !src.Owned() {
		return nil
	}
	var out []int
	for _, n := range src.Neighbours {
		q := g.Prefecture(n)
		if q == nil || !q.Owned() || q.Owner == by || q.Owner == src.Owner {
			continue
		}
		if nextToUs && !g.adjacentToFaction(n, by) {
			continue
		}
		out = append(out, n)
	}
	return out
}

// FarNearOurTargets 是遠交近攻第三個選單的候選（原版 `0x2c6a7`）：
// 與攻打目標相鄰、屬於我方的郡——實際出兵的就是它。
func (g *State) FarNearOurTargets(strike int, by state.FactionID) []int {
	dst := g.Prefecture(strike)
	if dst == nil {
		return nil
	}
	var out []int
	for _, n := range dst.Neighbours {
		if q := g.Prefecture(n); q != nil && q.Owned() && q.Owner == by {
			out = append(out, n)
		}
	}
	return out
}

func (g *State) adjacentToFaction(prefectureID int, by state.FactionID) bool {
	p := g.Prefecture(prefectureID)
	if p == nil {
		return false
	}
	for _, n := range p.Neighbours {
		if q := g.Prefecture(n); q != nil && q.Owned() && q.Owner == by {
			return true
		}
	}
	return false
}

// DefenderAid 是守方求得到的援軍郡（原版 `0x2dc6e`）：
//
//	守方郡的鄰郡裡，與守方同一勢力的那些
//
// 原版電腦取掃到的**最後一個**；那個勢力若由玩家操縱，原版會讓玩家
// 自己挑（`<聯合出兵>聯合守方那一郡合守`）。0 ＝ 求不到援。
func (g *State) DefenderAid(strike int) int {
	dst := g.Prefecture(strike)
	if dst == nil || !dst.Owned() {
		return 0
	}
	last := 0
	for _, n := range dst.Neighbours {
		if q := g.Prefecture(n); q != nil && q.Owned() && q.Owner == dst.Owner {
			last = n
		}
	}
	return last
}

// launchCampaign 照原版的四個郡發動一場戰役。
//
// 攻方是 `from` 郡的**所屬勢力與全部駐軍**——計謀發動的戰役沒有
// 「指定哪幾位將領」這一步，原版只把郡編號傳給戰鬥子系統。
func (g *State) launchCampaign(from, to int, aid Aid) (*BattleResult, error) {
	src, dst := g.Prefecture(from), g.Prefecture(to)
	if src == nil || dst == nil {
		return nil, fmt.Errorf("game: 郡編號 %d／%d 越界", from, to)
	}
	if !src.Owned() {
		return nil, fmt.Errorf("game: %s 沒有主，出不了兵", src.Name)
	}
	att := g.garrisonOf(from)
	if len(att) == 0 {
		return nil, fmt.Errorf("game: %s 沒有可出征的將領", src.Name)
	}
	def := g.garrisonOf(to)
	p := g.prepare(from, to, att, def, src.Owner, HalfSupply(), aid)
	p.B.Auto()
	return g.settle(p), nil
}

// 聯合出兵的三個我方選單（原版 `0x2d95f`／`0x2d9f8`／`0x2dab0`）。

// JointAttackFromTargets 是「從我方那一郡出兵」的候選：我方的郡，全部。
func (g *State) JointAttackFromTargets(by state.FactionID) []int {
	var out []int
	for i := 1; i <= state.PrefectureCount; i++ {
		if p := g.Prefecture(i); p != nil && p.Owned() && p.Owner == by {
			out = append(out, i)
		}
	}
	return out
}

// JointAttackStrikeTargets 是「聯合攻打那一郡」的候選：
// 出兵郡的鄰郡、有主、不是我方的。
func (g *State) JointAttackStrikeTargets(from int, by state.FactionID) []int {
	src := g.Prefecture(from)
	if src == nil {
		return nil
	}
	var out []int
	for _, n := range src.Neighbours {
		if q := g.Prefecture(n); q != nil && q.Owned() && q.Owner != by {
			out = append(out, n)
		}
	}
	return out
}

// JointAttackAidTargets 是「聯合我方那一郡合攻」的候選：
// 攻打目標的鄰郡、屬我方，**而且不是出兵的那一郡**。
//
// 一個都沒有的話原版印「無法聯合出兵」（`DS:0x8543`）退回選單——
// 聯合出兵至少要兩個郡才叫聯合。
func (g *State) JointAttackAidTargets(strike, from int, by state.FactionID) []int {
	dst := g.Prefecture(strike)
	if dst == nil {
		return nil
	}
	var out []int
	for _, n := range dst.Neighbours {
		if n == from {
			continue
		}
		if q := g.Prefecture(n); q != nil && q.Owned() && q.Owner == by {
			out = append(out, n)
		}
	}
	return out
}
