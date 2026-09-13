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

// 逼戰術層 AI 選出弓箭與計謀，看是哪一個選項（`docs/re/05` §12）。
//
// 九個選項裡目前有名字的是移動（`0x29344`）、會選出對戰的兩支
// （`0x29784`／`0x29ade`，目標軍力 2）與兜底的 `0x29e2e`。其餘五支在
// 「玩家貼著守軍」那個盤面上一次都沒定過案——**要換盤面才逼得出來**。
//
// 弓箭最好擺：部隊記錄 offset 20 是弓箭次數，把電腦那支補滿，
// 玩家那支放在**同一直線相隔一格**（弓箭的射程，§4）。
const unitArrows = 20

func TestUnitAIRangedAndPlotOptions(t *testing.T) {
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
		targetArmy int
	}
	var decisions []decision
	cur := -1
	targetArmy := func() int {
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
			army: int(o.Arg(0)), team: int(o.Arg(1)), targetArmy: -1})
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
			decisions[cur].targetArmy = targetArmy()
			cur = -1
		}
	})

	// **目標軍力 2 只表示選中了攻方主軍力。** 誘敵與圍攻也會跑交戰結算，
	// 而金的寫入端有一個在計謀模組（`0x2a22` 段）——所以光看交戰結算
	// 分不出電腦是在對戰還是在用計。六支效果常式與成功判定一起攔。
	plots := map[string]int{}
	for name, a := range map[string]uint32{
		"成功判定 0x2abb8": 0x2abb8, "火攻 0x2adec": 0x2adec,
		"水淹 0x2b15a": 0x2b15a, "陷阱 0x2b648": 0x2b648,
		"誘敵 0x2b6aa": 0x2b6aa, "圍攻 0x2bb00": 0x2bb00,
		"扣錢 0x2ada7": 0x2ada7,
	} {
		n, at := name, a
		o.OnCall(addr(at), func(*oracle.Oracle) { plots[n]++ })
	}

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
	// **計謀那條路也一起備齊**：守方軍團的金拉高、那一支的領隊智力拉滿。
	// 沒錢或智力不夠時六種計謀一律被擋在門檻（§4），那樣的零沒有意義。
	staBase := base + uint32(state.MasterTableSize)
	genBase := staBase + uint32(state.PrefectureTableSize)
	armyGold := 0x175e + 0*22 + 6 // 守方主軍團（軍力 0）的金
	setw(armyGold, 9000)
	if who := w16(foe + 38); who >= 0 && who < 350 {
		o.SetByte(addr(genBase+uint32(who)*30+9), 99)
		t.Logf("守方軍團的金拉到 %d；那一支的領隊是人物 %d，智力拉到 99",
			w16(armyGold), who)
	}
	// **這一輪改成貼身**：計謀與對戰的目標都只能是相鄰的格子（§4.0），
	// 上一輪擺在射程外，六種計謀連門都進不去。
	fc, fr := w16(foe+unitCol), w16(foe+unitRow)
	placed := false
	for dir := 0; dir < 6 && !placed; dir++ {
		dc, dr := off(fc, dir)
		c, r := fc+dc, fr+dr
		if c < 0 || c >= 12 || r < 0 || r >= 10 || o.Word(occ(c, r)) != 0xFFFF {
			continue
		}
		o.SetWord(occ(w16(me+unitCol), w16(me+unitRow)), 0xFFFF)
		setw(me+unitCol, c)
		setw(me+unitRow, r)
		o.SetWord(occ(c, r), 20)
		t.Logf("電腦 (%d,%d) 弓箭 %d 次；玩家貼到 (%d,%d)",
			fc, fr, w16(foe+unitArrows), c, r)
		placed = true
	}
	if !placed {
		t.Fatal("守軍旁邊沒有空格")
	}

	// **金被花光了要知道是誰花的。** 守方軍團的金欄位盯著。
	goldWr := o.WatchWritesAt(
		uint32(work)<<4+uint32(armyGold), uint32(work)<<4+uint32(armyGold)+1)
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
		byOpt[decider][dec.targetArmy]++
	}
	var keys []string
	for k := range byOpt {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	t.Logf("擺好盤面之後五天 %d 次決策：", len(decisions)-seen)
	for _, k := range keys {
		t.Logf("  %s → 目標軍力分布 %v（0xFFFF ＝ 65535）", k, byOpt[k])
	}
	o.StopWatchingWrites()
	gIP := map[string]int{}
	for _, w := range *goldWr {
		gIP[fmt.Sprintf("%#06x:%#06x", w.IP.Seg, w.IP.Off)]++
	}
	t.Logf("跑完之後：電腦那支剩下的弓箭次數 %d、守方軍團的金 %d",
		w16(foe+unitArrows), w16(armyGold))
	t.Logf("守方軍團的金被寫 %d 次，來自 %v", len(*goldWr), gIP)
	t.Logf("計謀那一組攔到的：%v", plots)
}
