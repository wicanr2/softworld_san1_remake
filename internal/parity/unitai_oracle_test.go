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

// 戰術層每支電腦部隊怎麼決定要做什麼（`docs/re/05` §12）。
//
// `0x29014(軍力, 隊伍)` 是那一支：先把決策槽 `es:[0x31c0]` 清成 `0xFFFF`，
// 再依序試九個選項，槽一有值就不再往下。前四個是
//
//	0x29e78  無條件
//	0x29138  無條件
//	0x29344  槽空
//	0x2985c  槽空 ＋ RND
//
// 後面五個（`0x29784`、`0x29c56`、`0x29b82`、`0x29ade`、`0x29e2e`）
// 都是「槽空」，其中三個還要先過一次 `RND`。
//
// 要把選項對回動作，最短的路是**看誰把槽填起來、填成什麼**：每個選項
// 前後各讀一次槽，變了就是它定的案。
func TestUnitAIDecisionChain(t *testing.T) {
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

	// 選項依序，名字先用位址頂著。
	options := []uint32{
		0x29e78, 0x29138, 0x29344, 0x2985c,
		0x29784, 0x29c56, 0x29b82, 0x29ade, 0x29e2e,
	}
	slot := func() int {
		if dgroup == 0 {
			return -1
		}
		seg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa92e})
		return int(o.Word(oracle.Addr{Seg: seg, Off: 0x31c0}))
	}
	// 四個決策變數一起讀（寫入端見 `docs/re/05` §12 的表）。
	vars := func() [4]int {
		var out [4]int
		if dgroup == 0 {
			return out
		}
		seg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa92e})
		for i, off := range []uint16{0x31a8, 0x31ae, 0x31b8, 0x31c0} {
			out[i] = int(o.Word(oracle.Addr{Seg: seg, Off: off}))
		}
		return out
	}

	type step struct {
		opt  uint32
		seen int
	}
	type decision struct {
		army, team int
		trace      []step
		final      int
		vals       [4]int
	}
	var decisions []decision
	cur := -1
	o.OnCall(addr(0x29014), func(o *oracle.Oracle) {
		decisions = append(decisions, decision{
			army: int(o.Arg(0)), team: int(o.Arg(1)), final: -1})
		cur = len(decisions) - 1
	})
	for _, a := range options {
		opt := a
		o.OnCall(addr(opt), func(o *oracle.Oracle) {
			if cur < 0 {
				return
			}
			decisions[cur].trace = append(decisions[cur].trace,
				step{opt, slot()})
		})
	}
	o.OnCall(addr(0x29132), func(o *oracle.Oracle) {
		if cur < 0 {
			return
		}
		decisions[cur].final = slot()
		decisions[cur].vals = vars()
		cur = -1
	})

	// 交戰結算逐次記下來：`es:[0x31a8]` ＝ 2 的那幾次是不是真的在打，
	// 判準要是這一支有沒有跑，不是兵少了一兩個。
	type bout struct{ aArmy, aTeam, dArmy, dTeam, mode int }
	var bouts []bout
	o.OnCall(addr(0x2a224), func(o *oracle.Oracle) {
		bouts = append(bouts, bout{int(o.Arg(0)), int(o.Arg(1)),
			int(o.Arg(2)), int(o.Arg(3)), int(o.Arg(4))})
	})

	driveIntoBattle(t, o, at, to)
	if dgroup == 0 {
		t.Fatal("沒有進到主戰場")
	}
	for step := 1; step <= 12 && cmdReads == 0; step++ {
		o.Drain()
		o.PressScan("0")
		if err := o.Run(60_000_000); err != nil {
			t.Fatalf("紮寨第 %d 步停止：%v", step, err)
		}
	}
	// **電腦沒有敵人在旁邊就只會休息。** 紮完寨把玩家那支移到守軍旁邊，
	// 決策鏈才走得到後面的選項（盤面直接寫記憶體，`CLAUDE.md`）。
	work := o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg})
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
	var me, foe = -1, -1
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
	if me >= 0 && foe >= 0 {
		fc, fr := w16(foe+unitCol), w16(foe+unitRow)
		for dir := 0; dir < 6; dir++ {
			i := uint16(((fc%2)*6 + dir) * 2)
			dc := int(int16(o.Word(oracle.Addr{Seg: colSeg, Off: 0x7c6a + i})))
			dr := int(int16(o.Word(oracle.Addr{Seg: rowSeg, Off: 0x7c82 + i})))
			c, r := fc+dc, fr+dr
			if c < 0 || c >= 12 || r < 0 || r >= 10 {
				continue
			}
			if o.Word(occ(c, r)) != 0xFFFF {
				continue
			}
			o.SetWord(occ(w16(me+unitCol), w16(me+unitRow)), 0xFFFF)
			o.SetWord(oracle.Addr{Seg: work, Off: uint16(me + unitCol)}, uint16(c))
			o.SetWord(oracle.Addr{Seg: work, Off: uint16(me + unitRow)}, uint16(r))
			o.SetWord(occ(c, r), 20)
			t.Logf("把玩家那支擺到 (%d,%d)，貼著守軍的 (%d,%d)", c, r, fc, fr)
			break
		}
	}

	// **與其猜哪個 word 是動作，掃整個決策區。** `es:[0x31c0]` 量到永遠是
	// 0，所以它是參數不是動作碼；決策鏈寫過的其他格子才是線索。
	decSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa92e})
	wr := o.WatchWritesAt(uint32(decSeg)<<4+0x3180, uint32(decSeg)<<4+0x31d0)

	type snapshot struct{ col, row, sol, move int }
	unitsOf := func() map[string]snapshot {
		out := map[string]snapshot{}
		for army := 0; army < battleArmies; army++ {
			for team := 0; team < battleTeams; team++ {
				rec := recOf(army, team)
				if w16(rec) == 0xFFFF || w16(rec+unitLeaders) <= 0 {
					continue
				}
				out[fmt.Sprintf("%d-%d", army, team)] = snapshot{
					w16(rec + unitCol), w16(rec + unitRow),
					w16(rec + unitSoldiers), w16(rec + unitMove)}
			}
		}
		return out
	}
	prevUnits := unitsOf()
	seenDecisions := 0
	seenBouts := 0

	for d := 1; d <= 5; d++ {
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
		now := unitsOf()
		dayBouts := bouts[seenBouts:]
		seenBouts = len(bouts)
		var line string
		for _, dec := range decisions[seenDecisions:] {
			k := fmt.Sprintf("%d-%d", dec.army, dec.team)
			a, b := prevUnits[k], now[k]
			what := "沒動"
			if a.col != b.col || a.row != b.row {
				what = fmt.Sprintf("移動 (%d,%d)→(%d,%d)", a.col, a.row, b.col, b.row)
			}
			if a.sol != b.sol {
				what += fmt.Sprintf(" 兵 %d→%d", a.sol, b.sol)
			}
			decider := "?"
			for i, st := range dec.trace {
				if st.seen != 0xFFFF && st.seen >= 0 && i > 0 {
					decider = fmt.Sprintf("%#x", dec.trace[i-1].opt)
					break
				}
				if i == len(dec.trace)-1 {
					decider = fmt.Sprintf("%#x", st.opt)
				}
			}
			line += fmt.Sprintf("\n    %s 由 %s 定案，四個變數 %v｜%s",
				k, decider, dec.vals, what)
		}
		var bl string
		for _, b := range dayBouts {
			bl += fmt.Sprintf(" [%d-%d→%d-%d 模式 %d]",
				b.aArmy, b.aTeam, b.dArmy, b.dTeam, b.mode)
		}
		t.Logf("第 %d 天：交戰結算 %d 次%s%s", d, len(dayBouts), bl, line)
		seenDecisions = len(decisions)
		prevUnits = now
	}

	if len(decisions) == 0 {
		t.Fatal("五天裡一次決策都沒攔到——0x29014 沒跑或槽的位址不對")
	}
	// 定案的是「第一個看到槽已經有值的選項」的**前一個**；
	// 都沒看到就是最後一個進去的那一支定的。
	byOpt := map[string]map[int]int{}
	for _, d := range decisions {
		decider := "（沒有選項定案）"
		for i, st := range d.trace {
			if st.seen != 0xFFFF && st.seen >= 0 {
				if i > 0 {
					decider = fmt.Sprintf("%#x", d.trace[i-1].opt)
				}
				break
			}
			if i == len(d.trace)-1 && d.final != 0xFFFF {
				decider = fmt.Sprintf("%#x", st.opt)
			}
		}
		if byOpt[decider] == nil {
			byOpt[decider] = map[int]int{}
		}
		byOpt[decider][d.final]++
	}
	var keys []string
	for k := range byOpt {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	t.Logf("五天裡攔到 %d 次決策", len(decisions))
	for _, k := range keys {
		n := 0
		for _, c := range byOpt[k] {
			n += c
		}
		t.Logf("  %s 定案 %d 次，槽值分布 %v", k, n, byOpt[k])
	}
	// 走過的選項次數，看看哪幾個根本沒被試到。
	tried := map[string]int{}
	for _, d := range decisions {
		for _, st := range d.trace {
			tried[fmt.Sprintf("%#x", st.opt)]++
		}
	}
	t.Logf("各選項被試到的次數：%v", tried)

	o.StopWatchingWrites()
	byOff := map[string]map[string]int{}
	for _, w := range *wr {
		k := fmt.Sprintf("%#06x", 0x3180+int(w.Off))
		if byOff[k] == nil {
			byOff[k] = map[string]int{}
		}
		byOff[k][fmt.Sprintf("%#06x:%#06x", w.IP.Seg, w.IP.Off)]++
	}
	var offs []string
	for k := range byOff {
		offs = append(offs, k)
	}
	sort.Strings(offs)
	t.Logf("決策區 es:0x3180–0x31d0 被寫了 %d 次，落在 %d 個位址：",
		len(*wr), len(offs))
	for _, k := range offs {
		t.Logf("  %s ← %v", k, byOff[k])
	}
}

// prevOption 已經不需要了。
