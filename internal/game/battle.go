package game

import (
	"fmt"
	"sort"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 戰役（說明書 p.26–35）。
//
// 原版的戰役分三階段：**召集 → 對陣 → 決勝**，中間有一張六方向的
// 主戰場與一層戰術對戰。這一版做的是**戰略層的決勝**：
// 誰打誰、雙方戰力怎麼算、贏了誰接手、被擒的將領怎麼處置。
//
// ⚠ **戰術層（主戰場的格子、移動力、單挑、六種計謀）還沒做**，
// 設計在 `docs/design/03-battle.md`。這一層的介面（`Attack` 的參數與
// `BattleResult`）刻意留得住那一層：戰術層做出來之後，
// 換掉的是 `resolve` 而不是呼叫端。
//
// 戰力的**因素**是手冊列的（p.31）：訓練度、武裝度、兵數、地形、兵種、
// 有無用計。係數是 remake 選的（`Tune*`）。

const (
	// TuneTrainingWeight／TuneArmsWeight 是訓練度與武裝度對戰力的權重
	// （百分比，100 表示「滿值時戰力加倍」）。
	TuneTrainingWeight = 60
	TuneArmsWeight     = 40

	// TuneWarWeight 是將領戰力對部隊戰力的權重。
	TuneWarWeight = 50

	// TuneDefenceBonus 是主守軍的地利加成百分比——守方在自己的城池裡
	//（說明書 p.32：城池「能夠發揮部隊最大戰力，以及一流防禦工事」）。
	TuneDefenceBonus = 30

	// TuneFortBonus 是每座城寨給守方的加成百分比（p.32：關寨提供
	//「少許攻擊優勢，及簡陋的防禦工事」）。
	TuneFortBonus = 5

	// TuneCasualty 是敗方的兵力損失百分比，TuneWinnerCasualty 是勝方的。
	TuneCasualty       = 60
	TuneWinnerCasualty = 20

	// TuneCaptureChance 是敗方將領被擒的機率。
	TuneCaptureChance = 40
)

// Captive 是一位被擒的將領。
type Captive struct {
	General int
	Name    string
}

// BattleResult 是一場戰役的結果。
type BattleResult struct {
	From, To       int
	AttackerWon    bool
	AttackerPower  int
	DefenderPower  int
	Captives       []Captive
	PrefectureTook bool
	Log            []string
}

// Attack 是「發動戰役」（說明書 p.19）：由該州郡獨力進犯鄰郡。
//
// 進攻的部隊是指定的那幾位將領；**主守軍必須派出所有兵力**（p.27），
// 所以守方自動是該郡的全部駐軍。
func (g *State) Attack(from, to int, attackers []int, by state.FactionID) (*BattleResult, error) {
	src, err := g.canOrder(from, by)
	if err != nil {
		return nil, err
	}
	dst := g.Prefecture(to)
	if dst == nil {
		return nil, fmt.Errorf("game: 郡編號 %d 越界", to)
	}
	if dst.Owner == by {
		return nil, fmt.Errorf("game: %s 已經是你的了", dst.Name)
	}
	if !g.Adjacent(from, to) {
		return nil, ErrNotAdjacent
	}
	if len(attackers) == 0 {
		return nil, fmt.Errorf("game: 要派出至少一位將領")
	}
	var att []*General
	for _, i := range attackers {
		x := g.General(i)
		if x == nil || x.Faction != by || x.Location != from {
			return nil, ErrUnknownUnit
		}
		att = append(att, x)
	}
	// 主事者不能傾巢而出——留守的人要能治理（說明書 p.19）。
	if g.leavesNobody(from, attackers, by) {
		return nil, ErrNoGovernor
	}
	// **主事者親征的話，出發前要先把治理交出去。**
	// 不交的話原郡在他離開之後就沒有主事者了，而那件事在畫面上
	// 只看得出「這個郡的太守欄空了」——不會有任何錯誤。
	for _, x := range att {
		if !x.Status.Governs() {
			continue
		}
		succ := g.successorForGoing(from, attackers, by)
		if succ == nil {
			return nil, ErrNoGovernor
		}
		succ.Status = state.StatusGovernor
		if x.Status == state.StatusLord {
			// 君主親征不卸君主身分，只是那個郡另有太守。
			break
		}
		x.Status = state.StatusOfficer
		break
	}
	var def []*General
	for _, x := range g.Garrison(to) {
		if x.Faction == dst.Owner {
			def = append(def, x)
		}
	}
	src.Commanded = true
	return g.resolve(from, to, att, def, by), nil
}

// successorForGoing 從**留守的人**裡挑一位接手治理，魅力最高的優先。
func (g *State) successorForGoing(prefectureID int, going []int, by state.FactionID) *General {
	out := map[int]bool{}
	for _, i := range going {
		out[i] = true
	}
	var best *General
	for _, x := range g.Garrison(prefectureID) {
		if x.Faction != by || out[x.Index] {
			continue
		}
		if best == nil || x.Charm > best.Charm {
			best = x
		}
	}
	return best
}

// leavesNobody 回報「這批人全部出征之後，這個郡是不是沒人治理」。
func (g *State) leavesNobody(prefectureID int, going []int, by state.FactionID) bool {
	out := map[int]bool{}
	for _, i := range going {
		out[i] = true
	}
	for _, x := range g.Garrison(prefectureID) {
		if x.Faction == by && !out[x.Index] {
			return false
		}
	}
	return true
}

// unitPower 是一支部隊的戰力。
//
// 因素是手冊列的（p.31）：兵數、訓練度、武裝度、將領戰力。
// 地形與兵種在戰術層才有意義，這一層先不計。
func unitPower(x *General) int {
	base := x.Soldiers
	quality := 100 +
		int(x.Training)*TuneTrainingWeight/100 +
		int(x.Arms)*TuneArmsWeight/100 +
		int(x.War)*TuneWarWeight/100
	return base * quality / 100
}

// Power 是一支部隊的戰力，對外版本（AI 要用它估算勝算）。
func Power(x *General) int { return unitPower(x) }

// DefencePower 是某個郡的守方戰力，含城池與城寨的加成。
func (g *State) DefencePower(prefectureID int) int {
	p := g.Prefecture(prefectureID)
	if p == nil {
		return 0
	}
	n := 0
	for _, x := range g.Garrison(prefectureID) {
		if x.Faction == p.Owner {
			n += unitPower(x)
		}
	}
	if p.Owned() {
		n = n * (100 + TuneDefenceBonus + p.Forts*TuneFortBonus) / 100
	}
	return n
}

func sumPower(units []*General) int {
	n := 0
	for _, x := range units {
		n += unitPower(x)
	}
	return n
}

// resolve 打完一場，把結果套用到局面上。
func (g *State) resolve(from, to int, att, def []*General, by state.FactionID) *BattleResult {
	dst := g.Prefecture(to)
	r := &BattleResult{From: from, To: to}
	r.AttackerPower = sumPower(att)

	dp := sumPower(def)
	if dst.Owned() {
		// 守方的地利：城池 ＋ 城寨（說明書 p.32）。
		dp = dp * (100 + TuneDefenceBonus + dst.Forts*TuneFortBonus) / 100
	}
	r.DefenderPower = dp
	r.AttackerWon = r.AttackerPower > dp

	win, lose := att, def
	if !r.AttackerWon {
		win, lose = def, att
	}
	for _, x := range win {
		x.Soldiers = x.Soldiers * (100 - TuneWinnerCasualty) / 100
	}
	for _, x := range lose {
		x.Soldiers = x.Soldiers * (100 - TuneCasualty) / 100
	}
	// 敗方的將領可能被擒（說明書 p.35）。
	for _, x := range lose {
		if g.roll(from, to, x.Index) < TuneCaptureChance {
			r.Captives = append(r.Captives, Captive{General: x.Index, Name: x.Name})
		}
	}
	sort.Slice(r.Captives, func(i, j int) bool {
		return r.Captives[i].General < r.Captives[j].General
	})

	// 「獲勝軍若於戰後捉到敵軍君主，其寶物將全歸獲勝軍所有」（說明書 p.35）。
	for _, c := range r.Captives {
		x := g.General(c.General)
		if x == nil || x.Status != state.StatusLord {
			continue
		}
		loser, winner := g.Faction(x.Faction), g.Faction(by)
		if !r.AttackerWon {
			// 守方獲勝時，被擒的是攻方的君主。
			winner = g.Faction(g.Prefecture(to).Owner)
		}
		if loser == nil || winner == nil || loser == winner {
			continue
		}
		for i := range loser.Treasury {
			winner.Treasury[i] += loser.Treasury[i]
			loser.Treasury[i] = 0
		}
		r.Log = append(r.Log, fmt.Sprintf("%s 的寶物盡歸戰勝方", x.Name))
	}

	if r.AttackerWon {
		g.takePrefecture(from, to, att, by)
		r.PrefectureTook = true
		r.Log = append(r.Log, fmt.Sprintf("攻下 %s", dst.Name))
	} else {
		// 攻方退回原郡，兵力已經扣過。
		r.Log = append(r.Log, fmt.Sprintf("%s 守住了", dst.Name))
	}
	for _, c := range r.Captives {
		r.Log = append(r.Log, fmt.Sprintf("%s 被擒", c.Name))
	}
	return r
}

// takePrefecture 讓攻方接手一個郡：「若進攻順利，軍隊將駐進被攻下的
// 州郡」（說明書 p.19）。
func (g *State) takePrefecture(from, to int, att []*General, by state.FactionID) {
	dst := g.Prefecture(to)
	old := dst.Owner
	// 守軍潰散：還活著的變成當地在野將領。
	for _, x := range g.Garrison(to) {
		if x.Faction != old {
			continue
		}
		x.Faction = state.NoFaction
		x.Status = state.StatusAvailable
		x.Loyalty = state.NoValue
		x.Soldiers = 0
	}
	dst.Owner = by
	// **剛攻下的郡這個月不能再下令。** 不擋的話同一個月可以一路連鎖
	// 進攻，而每郡每月一次的限制就形同虛設（說明書 p.17）。
	dst.Commanded = true
	for i, x := range att {
		x.Location = to
		if i == 0 && !x.Status.Governs() {
			x.Status = state.StatusGovernor
		}
	}
	// 舊主沒地了就退場。
	if f := g.Faction(old); f != nil && len(g.Territory(old)) == 0 {
		f.Alive = false
	}
}

// DisposeCaptive 是決勝之後對被擒敵將的處置（說明書 p.35）。
type Disposal int

const (
	Behead  Disposal = iota // 斬首：即處死刑
	Imprison                // 囚禁：成為戰場所在郡的在野將領
	Release                 // 釋放：其人將逃至鄰郡
	Enlist                  // 招降：成功便立刻成為部下
)

// DisposeCaptive 處置一位被擒的將領。
//
// **諸侯被擒只能斬首或釋放**（說明書 p.35）。
func (g *State) DisposeCaptive(at, generalIndex int, d Disposal, by state.FactionID) error {
	x := g.General(generalIndex)
	if x == nil {
		return ErrUnknownUnit
	}
	wasLord := x.Status == state.StatusLord
	if wasLord && d != Behead && d != Release {
		return fmt.Errorf("game: 諸侯被擒只能斬首或釋放")
	}
	switch d {
	case Behead:
		g.retire(x)
	case Imprison:
		x.Faction = state.NoFaction
		x.Status = state.StatusAvailable
		x.Loyalty = state.NoValue
		x.Location = at
		x.Soldiers = 0
	case Release:
		// 逃至鄰郡：挑編號最小的鄰郡，讓結果是決定性的。
		to := at
		if p := g.Prefecture(at); p != nil && len(p.Neighbours) > 0 {
			to = p.Neighbours[0]
		}
		x.Faction = state.NoFaction
		x.Status = state.StatusIdle
		x.Loyalty = state.NoValue
		x.Location = to
		x.Soldiers = 0
	case Enlist:
		x.Faction = by
		x.Status = state.StatusOfficer
		x.Location = at
		x.Loyalty = 50
		x.Soldiers = 0
	default:
		return fmt.Errorf("game: 沒有這種處置 %d", d)
	}
	return nil
}
