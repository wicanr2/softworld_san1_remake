//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 電腦部隊的移動是誰處理的（`docs/re/05` §12）。
//
// 決策鏈 `0x29014` 的目標軍力 `es:[0x31a8]` 留在 `0xFFFF` 時部隊仍然會走
// ——量到過一次，第 1 天 0-3 從 (6,1) 走到 (6,2) 而目標軍力是 `0xFFFF`。
// 所以**移動不在那條鏈裡**。
//
// 盯部隊記錄的欄（offset 22）與列（offset 24），寫的人就現形。
func TestWhoMovesTheUnits(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToGame(t, o, seedMas)
	at, to := stageABattle(t, o, base)

	var dgroup uint16
	o.OnCall(addr(0x2053c), func(o *oracle.Oracle) {
		if dgroup == 0 {
			dgroup = o.DSReg()
		}
	})
	cmdReads := 0
	o.OnCall(addr(0x27a68), func(*oracle.Oracle) { cmdReads++ })

	driveIntoBattle(t, o, at, to)
	if dgroup == 0 {
		t.Fatal("沒有進到主戰場")
	}
	work := o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg})
	for step := 1; step <= 12 && cmdReads == 0; step++ {
		o.Drain()
		o.PressScan("0")
		if err := o.Run(60_000_000); err != nil {
			t.Fatalf("紮寨第 %d 步停止：%v", step, err)
		}
	}

	// **敵人不在附近時電腦不動。** 把玩家那支擺到守軍旁邊，移動才會發生
	// （盤面直接寫記憶體，`CLAUDE.md`）。
	occSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9ca})
	colSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9c8})
	rowSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9c6})
	w16 := func(off int) int {
		return int(o.Word(oracle.Addr{Seg: work, Off: uint16(off)}))
	}
	occ := func(c, r int) oracle.Addr {
		return oracle.Addr{Seg: occSeg, Off: uint16(0x2532 + (r*12+c)*2)}
	}
	recOf := func(army, team int) int {
		return battleUnitBase + (army*battleUnitPer+team)*battleUnitSize
	}
	me, foe := -1, -1
	for army := 0; army < battleArmies; army++ {
		for team := 0; team < battleTeams; team++ {
			rec := recOf(army, team)
			if w16(rec) == 0xFFFF || w16(rec+unitLeaders) <= 0 {
				continue
			}
			if army >= 2 && me < 0 {
				me = rec
			}
			if army < 2 && foe < 0 {
				foe = rec
			}
		}
	}
	if me < 0 || foe < 0 {
		t.Fatal("盤面上湊不出攻守各一支")
	}
	fc, fr := w16(foe+unitCol), w16(foe+unitRow)
	for dir := 0; dir < 6; dir++ {
		i := uint16(((fc%2)*6 + dir) * 2)
		dc := int(int16(o.Word(oracle.Addr{Seg: colSeg, Off: 0x7c6a + i})))
		dr := int(int16(o.Word(oracle.Addr{Seg: rowSeg, Off: 0x7c82 + i})))
		c, r := fc+dc, fr+dr
		if c < 0 || c >= 12 || r < 0 || r >= 10 || o.Word(occ(c, r)) != 0xFFFF {
			continue
		}
		o.SetWord(occ(w16(me+unitCol), w16(me+unitRow)), 0xFFFF)
		o.SetWord(oracle.Addr{Seg: work, Off: uint16(me + unitCol)}, uint16(c))
		o.SetWord(oracle.Addr{Seg: work, Off: uint16(me + unitRow)}, uint16(r))
		o.SetWord(occ(c, r), 20)
		t.Logf("把玩家那支擺到 (%d,%d)，貼著守軍的 (%d,%d)", c, r, fc, fr)
		break
	}

	// 四十個部隊記錄整片盯著，之後再挑欄／列那幾個位元組。
	lo := uint32(work)<<4 + uint32(battleUnitBase)
	wr := o.WatchWritesAt(lo, lo+uint32(battleUnitSize*battleArmies*battleUnitPer))
	for d := 1; d <= 4; d++ {
		keys := []string{"0", "Y"}
		if d == 1 {
			keys = []string{"Y"}
		}
		for _, k := range keys {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(120_000_000); err != nil {
				t.Fatalf("第 %d 天送 %q 停止：%v", d, k, err)
			}
		}
	}
	o.StopWatchingReads()
	o.StopWatchingWrites()

	byField := map[string]map[string]int{}
	for _, w := range *wr {
		off := int(w.Off) % battleUnitSize
		unit := int(w.Off) / battleUnitSize
		var field string
		switch off {
		case unitCol, unitCol + 1:
			field = "欄"
		case unitRow, unitRow + 1:
			field = "列"
		default:
			continue
		}
		k := fmt.Sprintf("%s（%d-%d）", field, unit/battleUnitPer, unit%battleUnitPer)
		if byField[k] == nil {
			byField[k] = map[string]int{}
		}
		byField[k][fmt.Sprintf("%#06x:%#06x", w.IP.Seg, w.IP.Off)]++
	}
	if len(byField) == 0 {
		t.Fatalf("四天裡沒有任何部隊的欄／列被寫過（整片共 %d 次寫入）", len(*wr))
	}
	var keys []string
	for k := range byField {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	t.Logf("部隊記錄整片被寫 %d 次；其中欄／列的寫入端：", len(*wr))
	for _, k := range keys {
		t.Logf("  %s ← %v", k, byField[k])
	}
}
