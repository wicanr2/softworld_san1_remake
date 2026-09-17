package game

import "github.com/wicanr2/softworld_san1_remake/internal/state"

// 示範模式月底的鏡頭（`0x1e1fc`，`docs/re/06` §9、`docs/spec/005` §9.2，
// `L0`＋`L1`、`[base]`、Issue #73）。
//
// 原版只在電腦自動示範模式（`DS:0x5b02` ＝ 0）叫它，排在月底序列 `0x1581c`
// 的第一件，比結算 `0x1e394` 與開月 `0x17364` 都早，所以那兩次 `RND` 在整月
// 骰序的最前面。被看的郡與人（`DS:0x771c`／`0x771e`）不存進度，開機是 15 與 1。

const (
	// DemoCameraPrefecture／DemoCameraPerson 是開機時被看的郡與人。
	DemoCameraPrefecture = 15
	DemoCameraPerson     = 1
)

// DemoCamera 回報現在被看的郡與人。
func (g *State) DemoCamera() (prefecture, person int) {
	return g.demoPref, g.demoPerson
}

// SetDemoCamera 設定被看的郡與人（對拍從原版記憶體接過來用）。
func (g *State) SetDemoCamera(prefecture, person int) {
	g.demoPref, g.demoPerson = prefecture, person
}

// RunDemoCamera 是 `0x1e1fc`：換一個被看的郡與人，偶數月在右側面板畫那一郡的資料
// （`0x32fb:0x70`），奇數月畫那一位的人物資料卡（`0xf17:0x704`）。月份是結算
// **之前**的月份。
//
// 候選郡是「有主、而且主人不是被看那一郡的主人」的郡（`0x1e23a` 相等就跳過）；
// 候選人是所有有勢力、而且不是現在那一位的人物（不限所在郡）。清單空的時候
// 不擲、不換。
//
// 後段的開發期檢查（金或米 < 0 就停下來等 `S`）remake 不做：remake 的金米
// 不會是負的，那一段只在原版自己的算術出錯時才走得到。
func (g *State) RunDemoCamera() []Event {
	watched := state.FactionID(state.NoFaction)
	if p := g.Prefecture(g.demoPref); p != nil && p.Owned() {
		watched = p.Owner
	}
	var prefs []int
	for id := 1; id <= len(g.prefectures); id++ {
		if p := g.Prefecture(id); p.Owned() && p.Owner != watched {
			prefs = append(prefs, id)
		}
	}
	if len(prefs) > 0 {
		g.demoPref = prefs[g.Roll(len(prefs), 0x1e27b)]
	}
	var people []int
	for i := range g.generals {
		if x := &g.generals[i]; x.Faction != state.NoFaction && x.Index != g.demoPerson {
			people = append(people, x.Index)
		}
	}
	if len(people) > 0 {
		g.demoPerson = people[g.Roll(len(people), 0x1e2f9)]
	}
	if g.Date.Month%2 == 0 {
		return []Event{{Prefecture: g.demoPref, Bubble: &Bubble{Panel: g.demoPref}}}
	}
	at := 0
	if x := g.General(g.demoPerson); x != nil && x.Location >= 1 && x.Location <= len(g.prefectures) {
		at = x.Location
	}
	return []Event{{Prefecture: at, Bubble: &Bubble{Speaker: g.demoPerson, Card: true}}}
}
