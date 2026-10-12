package game

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 規格 022：此檔只含已確認的 remake 擴充，不是原版劇本或對拍資料。
//
//go:embed expanded_roster.json
var expandedRosterJSON []byte

type rosterData struct {
	Schema       int               `json:"schema"`
	SourceSHA256 map[string]string `json:"source_sha256"`
	Scenarios    []rosterScenario  `json:"scenarios"`
}
type rosterScenario struct {
	Slot           state.Slot      `json:"slot"`
	SourceScenario int             `json:"source_scenario"`
	People         []rosterPerson  `json:"people"`
	Factions       []rosterFaction `json:"factions"`
}
type rosterPerson struct {
	Name         string          `json:"name"`
	Portrait     int             `json:"portrait"`
	Age          int             `json:"age"`
	Origin       int             `json:"origin"`
	Location     int             `json:"location"`
	Kind         string          `json:"kind"`
	Faction      state.FactionID `json:"faction"`
	War          uint8           `json:"war"`
	Intel        uint8           `json:"intel"`
	Politics     uint8           `json:"politics"`
	Charm        uint8           `json:"charm"`
	SourceFile   string          `json:"source_file"`
	SourceOffset int             `json:"source_offset"`
}
type rosterFaction struct {
	ID             state.FactionID `json:"id"`
	City           int             `json:"city"`
	LeaderIndex    int             `json:"leader_index"`
	LeaderPortrait int             `json:"leader_portrait"`
}

// GeneralSource 保存新增人物的來源，不需要玩家持有 San5 才能讀檔。
type GeneralSource struct {
	ID       string `json:"id"`
	Portrait int    `json:"portrait"`
	Scenario int    `json:"scenario"`
	File     string `json:"file"`
	Offset   int    `json:"offset"`
	SHA256   string `json:"sha256"`
}

// PortraitID 保留原肖像欄；新增人物使用獨立的虛擬肖像編號。
func (g *General) PortraitID() int {
	if g.Source != nil {
		return 1000 + g.Source.Portrait
	}
	return int(g.Portrait)
}

// Expanded 回報這一局是否含規格 022 的擴充。讀舊檔不會呼叫擴充入口。
func (g *State) Expanded() bool { return g.rosterVersion == 1 }

// NewExpanded 是正式新局入口。New/NewPlayers 保留給原版資料與 oracle。
func NewExpanded(sc *state.Scenario, player state.FactionID, difficulty int, ed state.Edition) (*State, error) {
	return NewExpandedPlayers(sc, playerList(player), difficulty, ed)
}

func NewExpandedPlayers(sc *state.Scenario, players []state.FactionID, difficulty int, ed state.Edition) (*State, error) {
	if len(players) > 16 {
		return nil, fmt.Errorf("game: 玩家不可超過 16 位")
	}
	basePlayers := []state.FactionID{}
	for _, id := range players {
		if id < 16 {
			basePlayers = append(basePlayers, id)
		}
	}
	g, err := NewPlayers(sc, basePlayers, difficulty, ed)
	if err != nil {
		return nil, err
	}
	if err = g.addExpandedRoster(); err != nil {
		return nil, err
	}
	seen := map[state.FactionID]bool{}
	for _, id := range players {
		f := g.Faction(id)
		if seen[id] || f == nil || !f.Alive {
			return nil, fmt.Errorf("game: 無效或重複的玩家勢力 %d", id)
		}
		seen[id] = true
	}
	g.Players = append([]state.FactionID(nil), players...)
	g.Player = state.NoFaction
	if len(players) > 0 {
		g.Player = players[0]
	}
	for i := range g.factions {
		g.factions[i].ByComputer = !seen[g.factions[i].ID]
	}
	return g, nil
}

func (g *State) addExpandedRoster() error {
	var data rosterData
	if err := json.Unmarshal(expandedRosterJSON, &data); err != nil {
		return fmt.Errorf("game: 擴充名單：%w", err)
	}
	if data.Schema != 1 {
		return fmt.Errorf("game: 不支援擴充名單版本 %d", data.Schema)
	}
	var sc *rosterScenario
	for i := range data.Scenarios {
		if data.Scenarios[i].Slot == g.Slot {
			sc = &data.Scenarios[i]
			break
		}
	}
	if sc == nil || len(g.generals) != 350 {
		return fmt.Errorf("game: 劇本 %s 無有效擴充配置", g.Slot)
	}
	byPortrait := map[int]int{}
	for _, p := range sc.People {
		if p.Age < -128 || p.Age > 127 || p.Portrait < 0 || p.Portrait > 999 || byPortrait[p.Portrait] != 0 || g.Prefecture(p.Origin) == nil || g.Prefecture(p.Location) == nil {
			return fmt.Errorf("game: 擴充人物 %s 配置不合法", p.Name)
		}
		idx := len(g.generals)
		byPortrait[p.Portrait] = idx
		x := General{Index: idx, Name: p.Name, Age: uint8(int8(p.Age)), Stamina: 80, War: p.War, Intel: p.Intel, Charm: p.Charm,
			Politics: p.Politics, Rank: state.RankJuniorGeneral, Origin: p.Origin, Bond: idx, Debut: 15, Lifespan: 68,
			Loyalty: state.NoValue, Status: state.StatusAvailable, Faction: state.NoFaction, Location: p.Location, Training: 10, Arms: 10,
			Source: &GeneralSource{ID: "san5:" + strconv.Itoa(p.Portrait), Portrait: p.Portrait, Scenario: sc.SourceScenario,
				File: p.SourceFile, Offset: p.SourceOffset, SHA256: data.SourceSHA256[p.SourceFile]}}
		switch p.Kind {
		case "not_debuted":
			x.Status = state.StatusUnborn
		case "free":
		case "existing", "new":
			x.Faction = p.Faction
			x.Status = state.StatusOfficer
			x.Loyalty = 100
		default:
			return fmt.Errorf("game: 擴充人物 %s 身分不合法", p.Name)
		}
		g.generals = append(g.generals, x)
	}
	changed := map[int]bool{}
	for _, f := range sc.Factions {
		p := g.Prefecture(f.City)
		if p == nil || p.Owned() || g.Faction(f.ID) != nil || f.ID < 16 || f.ID == state.NoFaction {
			return fmt.Errorf("game: 新增勢力 %d 領土衝突", f.ID)
		}
		idx := f.LeaderIndex
		if idx < 0 {
			var ok bool
			idx, ok = byPortrait[f.LeaderPortrait]
			if !ok {
				return fmt.Errorf("game: 新增君主不存在")
			}
		}
		x := g.General(idx)
		if x == nil || x.Status == state.StatusFallen || (idx < 350 && x.Status != state.StatusAvailable) {
			return fmt.Errorf("game: 新增君主身分不合法")
		}
		x.Status, x.Rank, x.Faction, x.Location = state.StatusLord, state.RankLord, f.ID, f.City
		x.Loyalty, x.Soldiers, x.Troop = 100, 1000, state.TroopType(6)
		level := AILevelsAtStart([]int{0}, g.Difficulty, g.Edition)[0]
		g.factions = append(g.factions, Faction{ID: f.ID, Lord: idx, Alive: true, Chief: -1, AILevel: level, Prestige: 50,
			Treasury: [treasureCount]int{0, 1, 1, 1, 1}, ByComputer: true})
		p.Owner, p.governor = f.ID, idx
		changed[f.City] = true
	}
	for _, x := range g.generals[350:] {
		if x.Employed() {
			if g.Faction(x.Faction) == nil || g.Prefecture(x.Location).Owner != x.Faction {
				return fmt.Errorf("game: %s 的勢力或領地不符", x.Name)
			}
			changed[x.Location] = true
		}
	}
	for id := range changed {
		g.RefreshGarrison(id)
	}
	g.rosterVersion = 1
	slices.SortFunc(g.factions, func(a, b Faction) int { return int(a.ID) - int(b.ID) })
	return nil
}
