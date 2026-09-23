//go:build oracle

package game

import (
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// SettlementSnapshot 是 dosgolem 戰場快照接到正式玩家結算路徑的測試橋梁。
// 三張表另由原版在同一切點的位元組解出；不重複執行出征整編。
type SettlementSnapshot struct {
	Battle   *battle.Battle
	From, To int
	Armies   [4][]int           // 原版順序：主守、助守、主攻、助攻
	Factions [4]state.FactionID // battle.Side 順序
	Aid      Aid
}

// ReplayPlayerSettlement 以原版戰後狀態執行正式結算。
// 僅在 oracle 建置存在，遊戲正式路徑無法呼叫。
func (g *State) ReplayPlayerSettlement(s SettlementSnapshot) *BattleResult {
	roster := func(ids []int) []*General {
		out := make([]*General, 0, len(ids))
		for _, id := range ids {
			if x := g.General(id); x != nil {
				out = append(out, x)
			}
		}
		return out
	}
	b := s.Battle
	by := s.Factions[battle.MainAttacker]
	if dst := g.Prefecture(s.To); dst != nil {
		b.Escapes[battle.MainAttacker] = g.escapesFor(dst, by, s.Aid.Defender)
		b.Escapes[battle.AidAttacker] = b.Escapes[battle.MainAttacker]
		b.Escapes[battle.MainDefender] = g.escapesFor(dst, s.Factions[battle.MainDefender], s.Aid.Attacker)
		b.Escapes[battle.AidDefender] = b.Escapes[battle.MainDefender]
	}
	b.Host = &captiveHost{g: g, at: s.To}
	p := &Pending{B: b, Player: true, from: s.From, to: s.To, by: by,
		att: roster(s.Armies[2]), def: roster(s.Armies[0]),
		aidAtt: roster(s.Armies[3]), aidDef: roster(s.Armies[1]),
		aid: s.Aid, factions: s.Factions,
		result: &BattleResult{From: s.From, To: s.To}}
	return g.settle(p)
}
