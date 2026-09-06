package game

import (
	"fmt"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 戰役（說明書 p.26–35）。
//
// 原版的戰役分三階段：**召集 → 對陣 → 決勝**。
//
// 這一檔是**戰略層**：誰打誰、贏了誰接手、被擒的將領怎麼處置。
// 實際的交戰交給 `internal/battle`（主戰場的六方向格子、移動力、
// 五種戰鬥隊伍、快戰死戰、弓箭、單挑、六種計謀、三十天判定）。
//
// 戰場的**地形版面是 remake 生成的**——原版的郡地理誌是美術素材，
// 與主畫面地圖同理不重製也不散布。生成器是決定性的，所以整場戰役
// 仍然可重現。其餘機制照手冊（`docs/design/03-battle.md`）。

// 以下的權重是**估算**用的：AI 要在出兵之前判斷打不打得贏，
// 而真正的勝負是主戰場打出來的（`internal/battle`）。
// 估得準不準只影響電腦諸侯的選擇，不影響戰役本身。
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
	Captives       []Captive
	PrefectureTook bool

	// AttackerLost／DefenderLost 是雙方折損的兵。
	AttackerLost, DefenderLost int

	// Days 是這場戰役打了幾天（最多三十天，說明書 p.35）。
	Days int

	// Log 是主戰場的逐日戰報。**完整保留**：戰役是遊戲裡最花時間的
	// 一件事，只給一行結果等於把三十天的過程丟掉。摘要在 Summary。
	Log []string
}

// Summary 是給紀錄用的一行結果。
func (r *BattleResult) Summary(g *State) string {
	side := t("rep.defHeld")
	if r.AttackerWon {
		side = t("rep.won")
	}
	s := tf("rep.line",
		prefName(g, r.From), prefName(g, r.To), side,
		r.Days, r.AttackerLost, r.DefenderLost)
	if len(r.Captives) > 0 {
		names := make([]string, 0, len(r.Captives))
		for _, c := range r.Captives {
			names = append(names, personName(c.Name))
		}
		s += t("rep.took") + strings.Join(names, "、")
	}
	return s
}

// Attack 是「發動戰役」（說明書 p.19）：由該州郡獨力進犯鄰郡。
//
// 進攻的部隊是指定的那幾位將領；**主守軍必須派出所有兵力**（p.27），
// 所以守方自動是該郡的全部駐軍。
func (g *State) Attack(from, to int, attackers []int, by state.FactionID) (*BattleResult, error) {
	att, def, err := g.musterAttack(from, to, attackers, by)
	if err != nil {
		return nil, err
	}
	return g.fight(from, to, att, def, by), nil
}

// musterAttack 檢查出兵的條件並點齊雙方（說明書 p.19、p.27）。
//
// 抽出來是因為**玩家親征與電腦出兵走的是同一組條件**：
// 相鄰、不打自己、至少一位將領、原郡留得下人治理、主事者親征要先交接。
// 兩份檢查會慢慢分家，而分家的那一天只有一邊擋得住。
func (g *State) musterAttack(from, to int, attackers []int, by state.FactionID) (att, def []*General, err error) {
	src, err := g.canOrder(from, by)
	if err != nil {
		return nil, nil, err
	}
	dst := g.Prefecture(to)
	if dst == nil {
		return nil, nil, fmt.Errorf("game: 郡編號 %d 越界", to)
	}
	if dst.Owner == by {
		return nil, nil, fmt.Errorf("game: %s 已經是你的了", dst.Name)
	}
	if !g.Adjacent(from, to) {
		return nil, nil, ErrNotAdjacent
	}
	if len(attackers) == 0 {
		return nil, nil, fmt.Errorf("game: 要派出至少一位將領")
	}
	for _, i := range attackers {
		x := g.General(i)
		if x == nil || x.Faction != by || x.Location != from {
			return nil, nil, ErrUnknownUnit
		}
		att = append(att, x)
	}
	// 主事者不能傾巢而出——留守的人要能治理（說明書 p.19）。
	if g.leavesNobody(from, attackers, by) {
		return nil, nil, ErrNoGovernor
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
			return nil, nil, ErrNoGovernor
		}
		succ.Status = state.StatusGovernor
		if x.Status == state.StatusLord {
			// 君主親征不卸君主身分，只是那個郡另有太守。
			break
		}
		x.Status = state.StatusOfficer
		break
	}
	for _, x := range g.Garrison(to) {
		if x.Faction == dst.Owner {
			def = append(def, x)
		}
	}
	src.Commanded = true
	return att, def, nil
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

// unitPower 是一支部隊的**估計**戰力。
//
// 因素是手冊列的（p.31）：兵數、訓練度、武裝度、將領戰力。
// 地形、兵種、天氣、用計都要等部隊真的站到格子上才算得出來，
// 那是 `internal/battle` 的事——所以這個數字只拿來比大小，
// 不決定任何一場戰役的結果。
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

	// ⚠ **只有活著而且還效忠的人搬得進去。** 戰死或被擒的人已經被
	// `retire` 或處置移出勢力了；把他們也算進來的話，剛攻下的郡會
	// 掛著一個不存在的太守——而那件事在畫面上只看得出太守欄空了。
	var alive []*General
	for _, x := range att {
		if x.Employed() && x.Faction == by {
			alive = append(alive, x)
		}
	}
	if len(alive) == 0 {
		// 全軍覆沒卻「打贏了」：那個郡變成空白郡
		//（「因任何事故所形成的空白郡均不屬任何諸侯」，說明書 p.19）。
		dst.Owner = state.NoFaction
		return
	}
	// 太守挑魅力最高的——「魅力高的人比較能勝任太守之職」（說明書 p.23）。
	best := 0
	for i, x := range alive {
		x.Location = to
		if x.Charm > alive[best].Charm {
			best = i
		}
	}
	if !alive[best].Status.Governs() {
		alive[best].Status = state.StatusGovernor
	}
	// 舊主沒地了就退場。
	if f := g.Faction(old); f != nil && len(g.Territory(old)) == 0 {
		f.Alive = false
	}
}

// DisposeCaptive 是決勝之後對被擒敵將的處置（說明書 p.35）。
type Disposal int

const (
	Behead   Disposal = iota // 斬首：即處死刑
	Imprison                 // 囚禁：成為戰場所在郡的在野將領
	Release                  // 釋放：其人將逃至鄰郡
	Enlist                   // 招降：成功便立刻成為部下
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
