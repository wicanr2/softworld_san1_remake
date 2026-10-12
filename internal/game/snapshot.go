package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// Snapshot 是規格 022 的完整局面，與格式 2 的年月、選項及 BASEPRO 一起保存。
// 原三張表只保留固定長度的相容投影與未知 bytes，不能取代本快照。
type Snapshot struct {
	Schema        int                  `json:"schema"`
	RosterVersion int                  `json:"roster_version"`
	Generals      []General            `json:"generals"`
	Factions      []Faction            `json:"factions"`
	Prefectures   []SnapshotPrefecture `json:"prefectures"`
	SealAppeared  bool                 `json:"seal_appeared"`
	RandSeed      uint32               `json:"rand_seed"`
	RandDraws     int                  `json:"rand_draws"`
	RandOn        bool                 `json:"rand_on"`
	TurnOrder     []int                `json:"turn_order,omitempty"`
	GovernorAsks  []int                `json:"governor_asks,omitempty"`
	HeirAsks      []heirAsk            `json:"heir_asks,omitempty"`
	Pending       []Event              `json:"pending,omitempty"`
	Reports       []*BattleResult      `json:"reports,omitempty"`
}

type SnapshotPrefecture struct {
	Prefecture
	Governor       int `json:"governor"`
	Troops         int `json:"troops"`
	ActiveGenerals int `json:"active_generals"`
}

func (g *State) CaptureSnapshot() (*Snapshot, error) {
	if !g.Expanded() {
		return nil, nil
	}
	// 正常介面在戰役或求援選擇中不能儲存；避免把尚未完成的戰役當主畫面快照。
	if g.defence != nil || g.aidAsk != nil {
		return nil, fmt.Errorf("game: 請先完成目前的戰役或求援選擇")
	}
	x := &Snapshot{Schema: 1, RosterVersion: g.rosterVersion, Generals: g.generals, Factions: g.factions,
		SealAppeared: g.sealAppeared, RandSeed: g.randSeed, RandDraws: g.randDraws, RandOn: g.randOn,
		TurnOrder: g.turnOrder, GovernorAsks: g.governorAsks, HeirAsks: g.heirAsks, Pending: g.pending, Reports: g.Reports}
	for _, p := range g.prefectures {
		x.Prefectures = append(x.Prefectures, SnapshotPrefecture{p, p.governor, p.troops, p.activeGenerals})
	}
	if err := x.validate(g.Slot, g.Players); err != nil {
		return nil, err
	}
	return x, nil
}

func (x *Snapshot) validate(slot state.Slot, players []state.FactionID) error {
	bad := func(s string) error { return fmt.Errorf("game: 擴充快照：%s", s) }
	if x == nil || x.Schema != 1 || x.RosterVersion != 1 {
		return bad("版本或完整快照缺失")
	}
	var data rosterData
	if err := json.Unmarshal(expandedRosterJSON, &data); err != nil {
		return err
	}
	var cohort *rosterScenario
	for i := range data.Scenarios {
		if data.Scenarios[i].Slot == slot {
			cohort = &data.Scenarios[i]
			break
		}
	}
	if cohort == nil || len(x.Generals) != 350+len(cohort.People) || len(x.Generals) > 570 || len(x.Prefectures) != 42 || len(x.Factions) > 24 {
		return bad("人物、勢力或郡的數量不符")
	}
	indices := func(i int) bool { return i >= 0 && i < len(x.Generals) }
	optionalIndex := func(i int) bool { return i == -1 || indices(i) }
	factions := map[state.FactionID]bool{}
	for _, f := range x.Factions {
		if f.ID == state.NoFaction || int(f.ID) >= 16+len(cohort.Factions) || factions[f.ID] || !optionalIndex(f.Lord) || !optionalIndex(f.Chief) || f.AILevel < 0 || f.AILevel > 5 {
			return bad("勢力識別或人物引用不符")
		}
		if f.Alive && !indices(f.Lord) {
			return bad("在用勢力沒有君主")
		}
		if f.Alive {
			lord := x.Generals[f.Lord]
			if lord.Status != state.StatusLord || lord.Faction != f.ID {
				return bad("在用勢力的君主身分或所屬不符")
			}
		}
		factions[f.ID] = true
	}
	validFaction := func(id state.FactionID) bool { return id == state.NoFaction || factions[id] }
	seenPlayers := map[state.FactionID]bool{}
	if len(players) > 16 {
		return bad("玩家數量越界")
	}
	for _, id := range players {
		if id == state.NoFaction || !factions[id] || seenPlayers[id] {
			return bad("玩家勢力引用不符")
		}
		seenPlayers[id] = true
	}
	expected := map[string]rosterPerson{}
	for _, p := range cohort.People {
		expected["san5:"+strconv.Itoa(p.Portrait)] = p
	}
	seenSources := map[string]bool{}
	for i, p := range x.Generals {
		if p.Index != i || p.Name == "" || !validFaction(p.Faction) || (p.Bond != state.NoValue16 && p.Bond != -1 && !indices(p.Bond)) {
			return bad("人物索引、牽絆或勢力引用不符")
		}
		validPlace := func(n int) bool { return n == 255 || n >= 1 && n <= 42 }
		if !validPlace(p.Origin) || !validPlace(p.Location) {
			return bad("人物郡引用不符")
		}
		if i < 350 {
			if p.Source != nil {
				return bad("原有人物帶有新增來源")
			}
			continue
		}
		s := p.Source
		if s == nil {
			return bad("新增人物來源缺失")
		}
		original, ok := expected[s.ID]
		if !ok || p.Name != original.Name || seenSources[s.ID] || s.ID != "san5:"+strconv.Itoa(s.Portrait) || s.Scenario != cohort.SourceScenario || s.File != original.SourceFile || s.Offset != original.SourceOffset || data.SourceSHA256[s.File] == "" || s.SHA256 != data.SourceSHA256[s.File] {
			return bad("新增人物來源缺失、重複或不符")
		}
		seenSources[s.ID] = true
	}
	for i, p := range x.Prefectures {
		if p.ID != i+1 || !validFaction(p.Owner) || !optionalIndex(p.Governor) || len(p.BattleField) != 120 || p.Population < 0 || p.Troops < 0 || p.ActiveGenerals < 0 {
			return bad("州郡或主事者引用不符")
		}
		for _, id := range p.Neighbours {
			if id < 1 || id > 42 {
				return bad("相鄰郡越界")
			}
		}
	}
	if x.RandDraws < 0 {
		return bad("亂數狀態不符")
	}
	if len(x.TurnOrder) > 0 {
		if len(x.TurnOrder) != 43 {
			return bad("回合順序數量不符")
		}
		order := append([]int(nil), x.TurnOrder...)
		slices.Sort(order)
		for i, n := range order {
			if n != i {
				return bad("回合順序不是排列")
			}
		}
	}
	for _, id := range x.GovernorAsks {
		if id < 1 || id > 42 {
			return bad("主事者選擇郡越界")
		}
	}
	for _, ask := range x.HeirAsks {
		if !factions[ask.Faction] || !indices(ask.Applied) || !optionalIndex(ask.Chief) || !optionalIndex(ask.Governor) || !optionalIndex(ask.Demoted) {
			return bad("繼承選擇引用不符")
		}
		for _, id := range ask.List {
			if !indices(id) {
				return bad("繼承候選越界")
			}
		}
	}
	return nil
}

func RestoreSnapshot(sc *state.Scenario, e Extra, x *Snapshot) (*State, error) {
	if e.Edition == "" {
		e.Edition = state.EditionBase
	}
	if !e.Edition.Valid() || e.Month < 1 || e.Month > 12 || e.Year < 1 || e.Difficulty < 1 || e.Difficulty > e.Edition.MaxDifficulty() {
		return nil, fmt.Errorf("game: 擴充存檔年月、版本或難度不符")
	}
	players := append([]state.FactionID(nil), e.Players...)
	if len(players) == 0 && e.Player != state.NoFaction {
		players = []state.FactionID{e.Player}
	}
	if err := x.validate(sc.Slot, players); err != nil {
		return nil, err
	}
	g := &State{Slot: sc.Slot, Date: Date{e.Year, e.Month}, Edition: e.Edition, Difficulty: e.Difficulty, Options: e.Options,
		Player: state.NoFaction, Players: players, generals: x.Generals, factions: x.Factions, rosterVersion: x.RosterVersion,
		sealAppeared: x.SealAppeared, randSeed: x.RandSeed, randDraws: x.RandDraws, randOn: x.RandOn,
		turnOrder: x.TurnOrder, governorAsks: x.GovernorAsks, heirAsks: x.HeirAsks, pending: x.Pending, Reports: x.Reports,
		demoPref: DemoCameraPrefecture, demoPerson: DemoCameraPerson}
	if len(players) > 0 {
		g.Player = players[0]
	}
	g.rawMas, g.rawSta, g.rawGen = sc.Tables()
	for _, p := range x.Prefectures {
		v := p.Prefecture
		v.governor, v.troops, v.activeGenerals = p.Governor, p.Troops, p.ActiveGenerals
		g.prefectures = append(g.prefectures, v)
	}
	for i := range g.factions {
		g.factions[i].ByComputer = !slices.Contains(players, g.factions[i].ID)
	}
	return g, nil
}
