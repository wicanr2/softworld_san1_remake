//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 戰後收降的對拍（`docs/mechanics/20` 的「戰後收降」，`0x1ff7c`）。
//
// 電腦對電腦的戰役打完之後，安置那一支（`0x1fb26`）對名單裡的每一位呼叫
// `0x1ff7c(勝方的勢力, 郡, 人物槽)` 一次：
//
//	人物身分 == 0（君主）             → 回 0
//	州郡的現役將（offset 22） >= 50   → 回 0
//	抵抗 ＝ max(謀略, 戰力) ＋（牽絆對象同勢力 ? 60 − RND(30) : 0）
//	勝方的人望（諸侯 offset 8） >= 抵抗 ÷ 2 → 回 人望，否則回 0
//
// 盤面與 `battleover_oracle_test.go` 同一套：AI 等級 3–5、各郡錢糧拉滿、
// 在 `0xb2b4` 的入口把留守目標改成 1，電腦才打得起來。
//
// ⚠ `RND(30)` 只在牽絆那一條走到時才擲，所以要在**呼叫點之後**收
// （`0x20011`），不能事後補算——事後算不出原版當時抽到哪一個數。

// TestWarRecruitMatchesTheOriginal 對拍收降的抵抗值與成敗。
func TestWarRecruitMatchesTheOriginal(t *testing.T) {
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
	staBase := base + uint32(state.MasterTableSize)
	genBase := staBase + uint32(state.PrefectureTableSize)
	rec := func(i int) uint32 { return genBase + uint32(i*state.GeneralRecordSize) }

	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), uint16(3+alive%3))
		alive++
	}

	// 盤面自己擺：不擺的話 15 次判定全部落在「人望夠、沒有牽絆」那一格，
	// 判準的三個分支一個都沒被考驗到。
	//
	//	人望：活著的勢力輪流設成 5／50／95 → 收與不收都取得到
	//	牽絆：偶數槽指向同勢力的另一位、奇數槽指向自己 → 加成那一條走得到
	const genCount = state.GeneralTableSize / state.GeneralRecordSize
	lord := map[int]int{}
	for i := 0; i < genCount; i++ {
		if o.Byte(addr(rec(i)+17)) == 0 { // 身分 0 ＝ 君主
			lord[int(o.Byte(addr(rec(i)+18)))] = i
		}
	}
	n := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+8)), uint16([]int{5, 50, 95}[n%3]))
		n++
	}
	seeded := 0
	for i := 0; i < genCount; i++ {
		f := int(o.Byte(addr(rec(i) + 18)))
		if o.Byte(addr(rec(i)+17)) > 3 {
			continue
		}
		if i%2 == 1 || lord[f] == 0 || lord[f] == i {
			o.SetWord(addr(rec(i)+14), uint16(i)) // 指向自己 ＝ 沒有牽絆
			continue
		}
		o.SetWord(addr(rec(i)+14), uint16(lord[f]))
		seeded++
	}
	t.Logf("人望設成 5／50／95 輪流（%d 個勢力），%d 位的牽絆指向同勢力的君主",
		n, seeded)

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	var sg sortieGlobals
	sgOK := false
	o.OnCall(addr(0xb2b4), func(o *oracle.Oracle) {
		if !sgOK {
			sg, sgOK = resolveSortieGlobals(o), true
		}
		o.SetWord(addr(sg.want), 1)
	})

	type shot struct {
		faction, pref, who   int
		rank, intel, war     int
		bond, mine, prestige int
		officers             int
		roll                 int
		bonded               bool
	}
	var cur shot
	armed := false
	var fails []string
	fail := func(f string, a ...any) {
		if len(fails) < 16 {
			fails = append(fails, fmt.Sprintf(f, a...))
		}
	}
	calls, yes, no, bonded := 0, 0, 0, 0

	o.OnCall(addr(0x1ff7c), func(o *oracle.Oracle) {
		f, p, who := int(int16(o.Arg(0))), int(int16(o.Arg(1))), int(int16(o.Arg(2)))
		armed = who >= 0 && who < state.GeneralTableSize/state.GeneralRecordSize &&
			p >= 1 && p <= state.PrefectureCount && f >= 0 && f < state.MasterTableSize/72
		if !armed {
			return
		}
		cur = shot{faction: f, pref: p, who: who, roll: -1,
			rank:     int(o.Byte(addr(rec(who) + 17))),
			intel:    int(int8(o.Byte(addr(rec(who) + 9)))),
			war:      int(int8(o.Byte(addr(rec(who) + 10)))),
			bond:     int(int16(o.Word(addr(rec(who) + 14)))),
			mine:     int(o.Byte(addr(rec(who) + 18))),
			prestige: int(int16(o.Word(addr(base + uint32(f*72+8))))),
			officers: int(o.Byte(addr(staBase + uint32(p*176+22)))),
		}
		if b := cur.bond; b != who && b >= 0 &&
			b < state.GeneralTableSize/state.GeneralRecordSize {
			cur.bonded = int(o.Byte(addr(rec(b)+18))) == cur.mine
		}
	})
	// 牽絆那一條才擲的 RND(30)，在呼叫點之後收。
	o.OnCall(addr(0x20011), func(o *oracle.Oracle) {
		if armed {
			cur.roll = int(int16(o.AX()))
		}
	})

	check := func(got bool) {
		if !armed {
			return
		}
		armed = false
		calls++
		if got {
			yes++
		} else {
			no++
		}
		want := cur.rank != 0 && cur.officers < game.WarRecruitOfficerCap
		if want {
			roll := cur.roll
			if cur.bonded && roll < 0 {
				fail("槽 %d：牽絆同勢力卻沒攔到 RND(30)", cur.who)
				return
			}
			if !cur.bonded {
				roll = 0
			} else {
				bonded++
			}
			r := game.WarRecruitResistance(cur.intel, cur.war, cur.bonded, roll)
			want = game.WarRecruited(cur.prestige, r)
		}
		if got != want {
			fail("槽 %d（身分 %d 謀 %d 武 %d 牽絆 %v 擲 %d）郡 %d 現役將 %d "+
				"勢力 %d 人望 %d：原版%s，算出%s",
				cur.who, cur.rank, cur.intel, cur.war, cur.bonded, cur.roll,
				cur.pref, cur.officers, cur.faction, cur.prestige,
				map[bool]string{true: "收", false: "不收"}[got],
				map[bool]string{true: "收", false: "不收"}[want])
		}
	}
	o.OnCall(addr(0x1ff9b), func(*oracle.Oracle) { check(false) })
	o.OnCall(addr(0x2003e), func(*oracle.Oracle) { check(true) })

	const settle = 40_000_000
	for m := 0; m < 3; m++ {
		for p := 1; p <= state.PrefectureCount; p++ {
			o.SetWord(addr(staBase+uint32(p*176+18)), 20000)
			o.SetWord(addr(staBase+uint32(p*176+20)), 30000)
		}
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("三個月：亂數 %d 次（正對照）、收降判定 %d 次（收 %d／不收 %d，"+
		"其中牽絆同勢力 %d 次）", rnd, calls, yes, no, bonded)
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	for _, s := range fails {
		t.Error(s)
	}
	if calls == 0 {
		t.Skip("三個月裡一次收降判定都沒走到")
	}
	// 判準的三個分支都要被考驗到才算數。
	if yes == 0 || no == 0 {
		t.Errorf("收 %d 次、不收 %d 次——有一邊沒取樣到，人望門檻沒被驗過",
			yes, no)
	}
	if bonded == 0 {
		t.Error("牽絆同勢力一次都沒走到，那一條加成沒被驗過")
	}
}
