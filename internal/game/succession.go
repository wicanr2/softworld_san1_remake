package game

import "github.com/wicanr2/softworld_san1_remake/internal/state"

// HeirCandidates 是繼承人的候選名單（原版 `0x14a84`–`0x14b5c`，`L0`、`[base]`）。
//
// 兩段：先掃全人物表把**勢力欄等於這一方**的槽號收進來（`0x14a90`：
// 人物 offset 0x12；死掉的君主在這之前已經被寫成 0xFF，所以不會收到自己），
// 再按魅力（offset 0x0b）做一次交換排序（`0x14ad5`–`0x14b41`）——
// 外圈 i、內圈 j＞i，`魅力[j] > 魅力[i]` 才交換。
//
// **交換排序不是穩定排序**：魅力相同的兩位誰在前面由被換走的那一位決定，
// 所以這裡照抄原版的兩層迴圈，不用 `sort.SliceStable`。
func (g *State) HeirCandidates(id state.FactionID) []int {
	var list []int
	for i := range g.generals {
		if g.generals[i].Faction == id {
			list = append(list, i)
		}
	}
	charm := func(n int) int { return int(g.generals[n].Charm) }
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if charm(list[j]) > charm(list[i]) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	return list
}

// heirAsk 是一次還沒讓玩家挑繼承人的繼承（Issue #65）。
//
// 原版在 `0x14c24` 看諸侯 offset 0：等於 2（電腦）就取排頭，否則開清單
// （`0x14f7c`）讓玩家自己挑。remake 的規則層不能停下來等鍵，所以**先把
// 排頭那一位扶上去**（狀態隨時是完整的，電腦與無人看的路徑一如既往），
// 把這一次排進佇列；玩家答了再用 `AssignHeir` 覆蓋。
//
// undo 那幾格是為了覆蓋：繼承動到的欄位就那幾個（人望、軍師欄、繼承者的
// 職位／兵種／身分、他那一郡的自冶與主事者、被降成一般武將的舊主事者），
// 換人時先照原樣復原再套到新的人身上。
type heirAsk struct {
	Faction state.FactionID
	List    []int // 候選，魅力由高到低（`HeirCandidates`）
	Applied int   // 現在坐在位子上的那一位

	Prestige   int // 繼承之前的人望
	Chief      int // 繼承之前的軍師欄
	Rank       state.Rank // 繼承者原本的職位
	Troop      state.TroopType
	Status     state.Status
	Prefecture int // 繼承者所在的郡（0 表示沒有）
	Governor   int // 那一郡原本的主事者
	Autonomy   Autonomy // 那一郡原本的自冶
	Demoted    int // 被降成一般武將的舊主事者（−1 表示沒有）

	// Color 是繼承對白預擲的字色（`RND(8)`，`0x14dd2`）。**擲在原版擲的
	// 那一刻**，對白本身等玩家答完才排進佇列——不預擲的話那一擲會晚幾步，
	// 骰序就不一樣了。
	Color int
}

// askHeir 把一次繼承排進佇列，並記下復原要用的欄位。
func (g *State) askHeir(id state.FactionID, list []int, heir *General,
	prestige, chief int, rank state.Rank, troop state.TroopType, status state.Status,
	at, governor int, autonomy Autonomy, demoted, color int) {
	g.heirAsks = append(g.heirAsks, heirAsk{
		Faction: id, List: list, Applied: heir.Index,
		Prestige: prestige, Chief: chief, Rank: rank, Troop: troop, Status: status,
		Prefecture: at, Governor: governor, Autonomy: autonomy, Demoted: demoted,
		Color: color,
	})
}

// NeedsHeir 回下一個要玩家挑繼承人的勢力與候選名單；沒有就回 `NoFaction`。
func (g *State) NeedsHeir() (state.FactionID, []int) {
	for len(g.heirAsks) > 0 {
		a := g.heirAsks[0]
		f := g.Faction(a.Faction)
		if f != nil && f.Alive && g.IsHuman(a.Faction) && len(a.List) > 0 {
			return a.Faction, a.List
		}
		g.heirAsks = g.heirAsks[1:]
	}
	return state.NoFaction, nil
}

// AssignHeir 是玩家挑完之後那一步：挑的就是現在坐著的那一位就什麼都不動，
// 換人就先把上一位復原、再把君主之位套到新的人身上（人望用**繼承之前**的
// 值重算，不是用上一位折過的值）。繼承那一則對白到這裡才排進佇列。
func (g *State) AssignHeir(index int) error {
	if len(g.heirAsks) == 0 {
		return ErrUnknownUnit
	}
	a := g.heirAsks[0]
	f := g.Faction(a.Faction)
	x := g.General(index)
	if f == nil || x == nil {
		return ErrUnknownUnit
	}
	found := false
	for _, n := range a.List {
		if n == index {
			found = true
			break
		}
	}
	if !found {
		return ErrUnknownUnit
	}
	if index != a.Applied {
		g.undoSuccession(&a)
		g.applySuccession(f, x, a.Prestige)
	}
	g.heirAsks = g.heirAsks[1:]
	g.pending = append(g.pending, g.bubbleAt(x, false, true,
		tf("bub.succeed", personName(x.Name)), a.Color))
	return nil
}

// undoSuccession 把先扶上去的那一位還原成繼承之前的樣子。
func (g *State) undoSuccession(a *heirAsk) {
	f := g.Faction(a.Faction)
	if f == nil {
		return
	}
	// `f.Lord` 不必還原：接著就被 `applySuccession` 寫成新的那一位。
	f.Prestige, f.Chief = a.Prestige, a.Chief
	if x := g.General(a.Applied); x != nil {
		x.Rank, x.Troop, x.Status = a.Rank, a.Troop, a.Status
	}
	if p := g.Prefecture(a.Prefecture); p != nil {
		p.Autonomy, p.governor = a.Autonomy, a.Governor
	}
	if y := g.General(a.Demoted); y != nil {
		y.Status = state.StatusGovernor
	}
}

// applySuccession 是繼承的後半段（`0x14c3a`–`0x14d36`）：人望折算、軍師欄、
// 繼承者的職位／兵種／身分，以及他那一郡的自冶與主事者。
func (g *State) applySuccession(f *Faction, heir *General, prestige int) {
	f.Prestige = SuccessionPrestige(int(heir.Charm), prestige)
	if f.Chief == heir.Index {
		f.Chief = -1
	}
	heir.Rank = state.RankLord
	heir.Troop = state.TroopType(6)
	heir.Status = state.StatusLord
	f.Lord = heir.Index
	if p := g.Prefecture(heir.Location); p != nil {
		p.Autonomy = AutoNormal
		if old := g.General(p.governor); old != nil && old.Status == state.StatusGovernor {
			old.Status = state.StatusOfficer
		}
		p.governor = heir.Index
	}
}
