package game

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// Date 是遊戲內的時間。原版以年號顯示，可切西曆／中曆（說明書 p.26）。
//
// **內部一律存西元年**：年號是顯示層的事，而年號表跨越漢末到三國，
// 用它當內部時間會讓「下個月」這種基本運算變成查表。
type Date struct {
	Year  int // 西元
	Month int // 1..12
}

// Next 回傳下一個月。
func (d Date) Next() Date {
	if d.Month >= 12 {
		return Date{Year: d.Year + 1, Month: 1}
	}
	return Date{Year: d.Year, Month: d.Month + 1}
}

// Season 回傳季節。原版的特殊事件按季節發生（說明書 p.36–37）。
func (d Date) Season() Season {
	switch {
	case d.Month >= 3 && d.Month <= 5:
		return Spring
	case d.Month >= 6 && d.Month <= 8:
		return Summer
	case d.Month >= 9 && d.Month <= 11:
		return Autumn
	default:
		return Winter
	}
}

// Season 是季節。
type Season int

const (
	Spring Season = iota
	Summer
	Autumn
	Winter
)

// ScenarioStart 是六個劇本的起始年月。
//
// 出處是原版的「選擇年代」畫面：中平六年／興平二年／建安六年／
// 建安十三年／建安二十年／黃初元年，換算成西元 189／195／201／208／
// 215／220。原版主畫面左側直排寫「中平六年元月」，所以起始月是元月。
var ScenarioStart = map[state.Slot]Date{
	state.Scenario1: {Year: 189, Month: 1},
	state.Scenario2: {Year: 195, Month: 1},
	state.Scenario3: {Year: 201, Month: 1},
	state.Scenario4: {Year: 208, Month: 1},
	state.Scenario5: {Year: 215, Month: 1},
	state.Scenario6: {Year: 220, Month: 1},
}

// Prefecture 是一局裡的一個郡。欄位意義見 `docs/spec/003` §3。
type Prefecture struct {
	ID   int
	Name string

	Owner state.FactionID

	// Population 與 Soldiers 是**實際值**，不是原版存的 ÷100。
	// 換算在載入的唯一入口做完，規則層不再碰那個倍率。
	Population int
	Soldiers   int

	Gold int
	Rice int

	PublicLoyalty uint8
	LandValue     uint8
	FloodRate     uint8
	PriceLevel    uint8

	Neighbours []int

	// Commanded 記這個月下過令了沒。原版是每郡每月一次（說明書 p.17）。
	Commanded bool
}

// Owned 回報這個郡有沒有主。
func (p *Prefecture) Owned() bool { return p.Owner != state.NoFaction }

// General 是一局裡的一個人物。欄位意義見 `docs/spec/003` §2。
type General struct {
	Index int
	Name  string

	Age      uint8
	Stamina  uint8
	Intel    uint8
	War      uint8
	Charm    uint8
	Rank     state.Rank
	Loyalty  uint8
	Status   state.Status
	Faction  state.FactionID
	Location int
	Troop    state.TroopType
	Soldiers int
	Training uint8
	Arms     uint8
}

// Employed 回報這位人物有沒有效力對象。
func (g *General) Employed() bool { return g.Faction != state.NoFaction }

// TroopCap 是這位人物帶得動的最大兵力。
func (g *General) TroopCap() int { return TroopCap(g.Rank) }

// Faction 是一個勢力。
type Faction struct {
	ID state.FactionID

	// Lord 是君主在 Generals 裡的索引。
	Lord int

	// Alive 為假表示這個勢力已經沒有領地了。
	Alive bool
}

// State 是一局進行中的遊戲。
//
// ⚠ **郡用 1-based 編號**（與原版一致），所以不要直接對 Prefectures 取索引，
// 用 Prefecture(id)。原版的第 0 筆是啞元，把它當成一個郡會拿到名字是
// "...." 的東西然後照樣算下去。
type State struct {
	Slot state.Slot
	Date Date

	// Player 是玩家控制的勢力；NoFaction 表示純觀戰。
	Player state.FactionID

	// Difficulty 是難度 1–10（原版開局時問「請設定難度(1-10)」）。
	Difficulty int

	prefectures []Prefecture // 索引 0 是郡 1
	generals    []General
	factions    []Faction
}

// New 從一個劇本開一局。
//
// player 要是實際在用的勢力，否則回錯誤——**不要默默改成 0**：
// 「玩家其實在控制別人」這種錯誤在畫面上完全看不出來。
func New(sc *state.Scenario, player state.FactionID, difficulty int) (*State, error) {
	start, ok := ScenarioStart[sc.Slot]
	if !ok {
		return nil, fmt.Errorf("game: 不知道槽位 %q 的起始年月", sc.Slot)
	}
	if difficulty < 1 || difficulty > 10 {
		return nil, fmt.Errorf("game: 難度 %d 越界（原版收 1..10）", difficulty)
	}

	g := &State{Slot: sc.Slot, Date: start, Player: player, Difficulty: difficulty}

	for _, p := range sc.Prefectures() {
		g.prefectures = append(g.prefectures, Prefecture{
			ID: p.ID, Name: p.Name,
			Owner:      state.FactionID(p.Owner),
			Population: p.People(),
			Soldiers:   p.Troops(),
			Gold:       int(p.Gold), Rice: int(p.Rice),
			PublicLoyalty: p.PublicLoyalty, LandValue: p.LandValue,
			FloodRate: p.FloodRate, PriceLevel: p.PriceLevel,
			Neighbours: append([]int(nil), p.Neighbours...),
		})
	}
	for _, s := range sc.Generals() {
		g.generals = append(g.generals, General{
			Index: s.Index, Name: s.Name,
			Age: s.Age, Stamina: s.Stamina, Intel: s.Intel, War: s.War, Charm: s.Charm,
			Rank: s.Rank, Loyalty: s.Loyalty, Status: s.Status,
			Faction: state.FactionID(s.Faction), Location: int(s.Location),
			Troop: s.Troop, Soldiers: int(s.Soldiers),
			Training: s.Training, Arms: s.Arms,
		})
	}

	valid := false
	for _, f := range sc.ActiveFactions() {
		lord, err := sc.Lord(f)
		if err != nil {
			return nil, err
		}
		g.factions = append(g.factions, Faction{
			ID: state.FactionID(f), Lord: lord.Index, Alive: true,
		})
		if state.FactionID(f) == player {
			valid = true
		}
	}
	if player != state.NoFaction && !valid {
		return nil, fmt.Errorf("game: 勢力 %d 在劇本 %s 裡沒有在用", player, sc.Slot)
	}
	return g, nil
}

// Prefecture 用 1-based 編號取一個郡。越界回 nil。
func (g *State) Prefecture(id int) *Prefecture {
	if id < 1 || id > len(g.prefectures) {
		return nil
	}
	return &g.prefectures[id-1]
}

// Prefectures 回傳全部的郡。
func (g *State) Prefectures() []Prefecture { return g.prefectures }

// General 用槽號取一個人物。越界回 nil。
func (g *State) General(index int) *General {
	if index < 0 || index >= len(g.generals) {
		return nil
	}
	return &g.generals[index]
}

// Factions 回傳實際在用的勢力。
func (g *State) Factions() []Faction { return g.factions }

// Lord 回傳某個勢力的君主。
func (g *State) Lord(f state.FactionID) *General {
	for _, x := range g.factions {
		if x.ID == f {
			return g.General(x.Lord)
		}
	}
	return nil
}

// Territory 回傳某個勢力擁有的郡編號，依編號排序。
func (g *State) Territory(f state.FactionID) []int {
	var out []int
	for i := range g.prefectures {
		if g.prefectures[i].Owner == f {
			out = append(out, g.prefectures[i].ID)
		}
	}
	return out
}

// Governor 回傳某個郡的主事者：君主或太守，兩者都不在時由軍師代理
// （`docs/spec/003` §2.2）。找不到唯一的一位回 nil。
func (g *State) Governor(prefectureID int) *General {
	p := g.Prefecture(prefectureID)
	if p == nil || !p.Owned() {
		return nil
	}
	var main, deputy []int
	for i := range g.generals {
		x := &g.generals[i]
		if x.Faction != p.Owner || x.Location != prefectureID {
			continue
		}
		switch {
		case x.Status.Governs():
			main = append(main, i)
		case x.Status == state.StatusChief:
			deputy = append(deputy, i)
		}
	}
	if len(main) == 1 {
		return &g.generals[main[0]]
	}
	if len(main) == 0 && len(deputy) == 1 {
		return &g.generals[deputy[0]]
	}
	return nil
}

// Garrison 回傳駐在某個郡的現役武將（含主事者）。
func (g *State) Garrison(prefectureID int) []*General {
	var out []*General
	for i := range g.generals {
		x := &g.generals[i]
		if x.Employed() && x.Location == prefectureID {
			out = append(out, x)
		}
	}
	return out
}

// Adjacent 回報兩個郡相不相鄰。
func (g *State) Adjacent(a, b int) bool {
	p := g.Prefecture(a)
	if p == nil {
		return false
	}
	for _, n := range p.Neighbours {
		if n == b {
			return true
		}
	}
	return false
}
