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

// 逼戰術層 AI 選出弓箭，看是哪一個選項、動作碼是幾號（`docs/re/05` §12）。
//
// 九個選項裡目前有名字的是移動（`0x29344`）、會選出對戰的兩支
// （`0x29784`／`0x29ade`，動作碼 2）與兜底的 `0x29e2e`。其餘五支在
// 「玩家貼著守軍」那個盤面上一次都沒定過案——**要換盤面才逼得出來**。
//
// 弓箭最好擺：部隊記錄 offset 20 是弓箭次數，把電腦那支補滿，
// 玩家那支放在**同一直線相隔一格**（弓箭的射程，§4）。
const unitArrows = 20

func TestUnitAIArcheryOption(t *testing.T) {
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

	options := []uint32{
		0x29e78, 0x29138, 0x29344, 0x2985c,
		0x29784, 0x29c56, 0x29b82, 0x29ade, 0x29e2e,
	}
	type step struct {
		opt  uint32
		seen int
	}
	type decision struct {
		army, team int
		trace      []step
		act        int
	}
	var decisions []decision
	cur := -1
	act := func() int {
		if dgroup == 0 {
			return -1
		}
		seg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa92e})
		return int(o.Word(oracle.Addr{Seg: seg, Off: 0x31a8}))
	}
	slot := func() int {
		if dgroup == 0 {
			return -1
		}
		seg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa92e})
		return int(o.Word(oracle.Addr{Seg: seg, Off: 0x31c0}))
	}
	o.OnCall(addr(0x29014), func(o *oracle.Oracle) {
		decisions = append(decisions, decision{
			army: int(o.Arg(0)), team: int(o.Arg(1)), act: -1})
		cur = len(decisions) - 1
	})
	for _, a := range options {
		opt := a
		o.OnCall(addr(opt), func(o *oracle.Oracle) {
			if cur >= 0 {
				decisions[cur].trace = append(decisions[cur].trace, step{opt, slot()})
			}
		})
	}
	o.OnCall(addr(0x29132), func(o *oracle.Oracle) {
		if cur >= 0 {
			decisions[cur].act = act()
			cur = -1
		}
	})

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

	occSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9ca})
	colSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9c8})
	rowSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9c6})
	w16 := func(off int) int {
		return int(o.Word(oracle.Addr{Seg: work, Off: uint16(off)}))
	}
	setw := func(off, v int) {
		o.SetWord(oracle.Addr{Seg: work, Off: uint16(off)}, uint16(v))
	}
	occ := func(c, r int) oracle.Addr {
		return oracle.Addr{Seg: occSeg, Off: uint16(0x2532 + (r*12+c)*2)}
	}
	recOf := func(army, team int) int {
		return battleUnitBase + (army*battleUnitPer+team)*battleUnitSize
	}
	off := func(col, dir int) (int, int) {
		i := uint16(((col%2)*6 + dir) * 2)
		dc := int(int16(o.Word(oracle.Addr{Seg: colSeg, Off: 0x7c6a + i})))
		dr := int(int16(o.Word(oracle.Addr{Seg: rowSeg, Off: 0x7c82 + i})))
		return dc, dr
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
	// 電腦那支的弓箭次數補滿。
	setw(foe+unitArrows, 10)
	// 玩家那支擺到同一直線、相隔一格的位置。
	fc, fr := w16(foe+unitCol), w16(foe+unitRow)
	placed := false
	for dir := 0; dir < 6 && !placed; dir++ {
		dc, dr := off(fc, dir)
		mc, mr := fc+dc, fr+dr
		if mc < 0 || mc >= 12 || mr < 0 || mr >= 10 {
			continue
		}
		dc2, dr2 := off(mc, dir)
		c, r := mc+dc2, mr+dr2
		if c < 0 || c >= 12 || r < 0 || r >= 10 {
			continue
		}
		if o.Word(occ(c, r)) != 0xFFFF || o.Word(occ(mc, mr)) != 0xFFFF {
			continue
		}
		o.SetWord(occ(w16(me+unitCol), w16(me+unitRow)), 0xFFFF)
		setw(me+unitCol, c)
		setw(me+unitRow, r)
		o.SetWord(occ(c, r), 20)
		t.Logf("電腦 (%d,%d) 弓箭次數補到 %d；玩家擺到 (%d,%d)，中間 (%d,%d) 空著",
			fc, fr, w16(foe+unitArrows), c, r, mc, mr)
		placed = true
	}
	if !placed {
		t.Fatal("找不到同一直線相隔一格的空位")
	}

	seen := len(decisions)
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
	}

	byOpt := map[string]map[int]int{}
	for _, dec := range decisions[seen:] {
		decider := "（沒有選項定案）"
		for i, st := range dec.trace {
			if st.seen != 0xFFFF && st.seen >= 0 && i > 0 {
				decider = fmt.Sprintf("%#x", dec.trace[i-1].opt)
				break
			}
			if i == len(dec.trace)-1 {
				decider = fmt.Sprintf("%#x", st.opt)
			}
		}
		if byOpt[decider] == nil {
			byOpt[decider] = map[int]int{}
		}
		byOpt[decider][dec.act]++
	}
	var keys []string
	for k := range byOpt {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	t.Logf("擺好盤面之後五天 %d 次決策：", len(decisions)-seen)
	for _, k := range keys {
		t.Logf("  %s → 動作碼分布 %v（0xFFFF ＝ 65535）", k, byOpt[k])
	}
	t.Logf("電腦那支剩下的弓箭次數：%d", w16(foe+unitArrows))
}
