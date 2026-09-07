//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 戰役勝負判定的對拍（`docs/re/05` §8、§8.1）。
//
// 兩支常式共用同一個結果欄位 `es:[0x20dc]`（勝方的軍力編號：
// 主守軍 0、助守軍 1、主攻軍 2、助攻軍 3），`0xFFFF` ＝ 還沒分出勝負。
//
//	0x24f8c  統帥條件，每天判一次
//	0x250d4  三十天期滿，看此刻誰站在城池那一格
//
// **統帥條件的兩段有順序**：先寫「攻方統帥都不在 → 主守軍勝（0）」，
// 再寫「守方統帥都不在 → 主攻軍勝（2）」，後者蓋掉前者，所以兩邊統帥
// 都不在時判攻方勝。remake 這一邊是 `battle.checkOver` 的前兩個 case。
//
// 盤面自己擺：AI 等級輪流 3–5、各郡的金米拉滿，並且在 `0xb2b4` 的入口把
// 留守目標改成 1——不改的話電腦幾乎不出兵，也就打不起來
// （`internal/parity/sortie_oracle_test.go` 的同一套）。
//
// ⚠ **電腦對電腦不進戰術層。** 這一套盤面三個月打起來 5 次：`0xb644`
// （AI 進攻）5 次、`0x20253`（戰役開場，天數 ← 1）5 次，而
// `0x24f8c`（每天判一次的統帥條件）與 `0x250d4`（三十天期滿）**一次都
// 沒跑**。所以原版的電腦對電腦戰役只走到開場與畫面，日循環是玩家在場
// 才跑的。remake 這一邊 `game.fight` 走 `p.B.Auto()` 打滿三十天——
// 這是 remake 與原版在戰役層最大的一道結構差異，也很可能是月度對拍裡
// 兵力／訓練／武裝三欄差異的主要來源（`CONTEXT.md` worklist）。
//
// 所以這支測試現在會 skip：判準寫好了、正對照（戰役有沒有開場）也擋著，
// 等到有一條路徑真的走進日循環（玩家親征，或原版的電腦戰役另有結算）
// 就會自動生效。

// TestBattleOutcomeMatchesTheOriginal 釘住統帥條件與三十天期滿的判定。
func TestBattleOutcomeMatchesTheOriginal(t *testing.T) {
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

	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), uint16(3+alive%3))
		alive++
	}

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	// 讓電腦真的打起來：`es:[0x2e62]` 是留守目標，改小就出得了兵。
	var sg sortieGlobals
	sgOK := false
	o.OnCall(addr(0xb2b4), func(o *oracle.Oracle) {
		if !sgOK {
			sg, sgOK = resolveSortieGlobals(o), true
		}
		o.SetWord(addr(sg.want), 1)
	})

	// 戰役工作區的遠指標（DS 槽位直接從反組譯讀出來）。
	type warGlobals struct{ won, day, row, col, occ, force, unit uint32 }
	var w warGlobals
	wOK := false
	resolveWar := func(o *oracle.Oracle) {
		ds := uint32(o.DSReg()) * 16
		far := func(slot uint16, off uint32) uint32 {
			return uint32(o.Word(addr(ds+uint32(slot))))*16 + off
		}
		w = warGlobals{
			won:   far(0xa89e, 0x20dc), // 勝方的軍力編號
			day:   far(0xa8a2, 0x2100), // 天數
			row:   far(0xa89c, 0x0586), // 城池的列
			col:   far(0xa89a, 0x0584), // 城池的欄
			occ:   far(0xa8cc, 0x2532), // 佔位地圖
			force: far(0xa8b6, 0x175e), // 軍力記錄（22 byte 一筆，第 0 欄是統帥）
			unit:  far(0xa896, 0x3502), // 部隊記錄（一方 420 byte，第一位將領）
		}
		wOK = true
	}

	var fails []string
	fail := func(f string, a ...any) {
		if len(fails) < 16 {
			fails = append(fails, fmt.Sprintf(f, a...))
		}
	}

	// 純計數，不帶條件——分得出「常式沒被呼叫」與「判準沒armed」。
	enters := map[string]int{}
	for _, a := range []struct {
		at   uint32
		name string
	}{
		{0x0b644, "AI 進攻→戰役入口"},
		{0x20253, "戰役開場（天數=1）"},
		{0x24f8c, "統帥條件入口"},
		{0x250d4, "三十天判定入口"},
	} {
		name := a.name
		o.OnCall(addr(a.at), func(*oracle.Oracle) { enters[name]++ })
	}

	// ---- 軍團 offset 10 是「將領人數」還是「部隊數」（0x1ea67）------
	//
	// `0x1E908` 拿 offset 10 當 42 byte 部隊記錄的迴圈上界，而 `docs/re/05`
	// §3.3 把 offset 10 標成將領人數、offset 12 才是部隊數。三個數一起讀：
	// 上界、offset 12、以及實際非空（將領[0] != 0xFFFF）的部隊記錄數。
	shape := map[string]int{}
	shapeSeen := 0
	o.OnCall(addr(0x1ea67), func(o *oracle.Oracle) {
		if shapeSeen >= 40 {
			return
		}
		ds := uint32(o.DSReg()) * 16
		far := func(slot uint16, off uint32) uint32 {
			return uint32(o.Word(addr(ds+uint32(slot))))*16 + off
		}
		forceBase := far(0xa7f6, 0x175e) // 軍團記錄（22 byte 一筆）
		unitBase := far(0xa810, 0x384a)  // 軍團 2 的部隊記錄（42 byte，十格）
		bound := int(int16(o.Word(addr(forceBase + 2*22 + 10))))
		f12 := int(int16(o.Word(addr(forceBase + 2*22 + 12))))
		units, leaders := 0, 0
		for i := 0; i < 10; i++ {
			u := unitBase + uint32(i*42)
			if int(int16(o.Word(addr(u)))) == -1 {
				continue
			}
			units++
			for k := 0; k < 10; k++ {
				if int(int16(o.Word(addr(u+uint32(k*2))))) != -1 {
					leaders++
				}
			}
		}
		shapeSeen++
		switch {
		case bound == units && bound != leaders:
			shape["上界＝部隊數"]++
		case bound == leaders && bound != units:
			shape["上界＝將領人數"]++
		case bound == units && bound == leaders:
			shape["部隊數＝將領人數，分不開"]++
		default:
			shape[fmt.Sprintf("上界%d／部隊%d／將領%d／offset12=%d",
				bound, units, leaders, f12)]++
		}
	})

	// ---- 統帥條件（0x24f8c）----------------------------------------
	chiefRuns, chiefDecided := 0, map[int]int{}
	armedChief := false
	o.OnCall(addr(0x24f97), func(o *oracle.Oracle) {
		if !wOK {
			resolveWar(o)
		}
		armedChief = int(int16(o.Word(addr(w.won)))) == -1
	})
	o.OnCall(addr(0x2502e), func(o *oracle.Oracle) {
		if !armedChief {
			return
		}
		armedChief = false
		chiefRuns++
		here := func(side int) bool {
			cmd := int(int16(o.Word(addr(w.force + uint32(side*22)))))
			if cmd == -1 {
				return false
			}
			return cmd == int(int16(o.Word(addr(w.unit+uint32(side*420)))))
		}
		want := -1
		if !here(2) && !here(3) {
			want = 0 // 攻方統帥都不在 → 主守軍勝
		}
		if !here(0) && !here(1) {
			want = 2 // 守方那一段在後面，蓋掉前面寫的值
		}
		got := int(int16(o.Word(addr(w.won))))
		if got != want {
			fail("統帥條件：原版判 %d，照 §8.1 算是 %d（統帥在場 主守%v 助守%v 主攻%v 助攻%v）",
				got, want, here(0), here(1), here(2), here(3))
		}
		if want != -1 {
			chiefDecided[want]++
		}
	})

	// ---- 三十天期滿（0x250d4）--------------------------------------
	dayRuns, dayDecided := 0, map[int]int{}
	armedDay := false
	o.OnCall(addr(0x250f6), func(o *oracle.Oracle) {
		if !wOK {
			resolveWar(o)
		}
		armedDay = true
	})
	o.OnCall(addr(0x25146), func(o *oracle.Oracle) {
		if !armedDay {
			return
		}
		armedDay = false
		dayRuns++
		row := int(int16(o.Word(addr(w.row))))
		col := int(int16(o.Word(addr(w.col))))
		cell := int(int16(o.Word(addr(w.occ + uint32((12*row+col)*2)))))
		want := 0
		if cell != -1 {
			want = cell / 10
		}
		got := int(int16(o.Word(addr(w.won))))
		if got != want {
			fail("三十天期滿：城池在 (%d,%d)、那一格是 %d → 原版判 %d，照 §8 算是 %d",
				col, row, cell, got, want)
		}
		dayDecided[want]++
	})

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

	t.Logf("入口計數：%v", enters)
	t.Logf("軍團 offset 10 的形狀（%d 次取樣）：%v", shapeSeen, shape)
	t.Logf("三個月：亂數 %d 次（正對照）、統帥條件跑 %d 次（判出勝負 %v）、"+
		"三十天期滿跑 %d 次（結果分布 %v）",
		rnd, chiefRuns, chiefDecided, dayRuns, dayDecided)
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	for _, s := range fails {
		t.Error(s)
	}
	if enters["戰役開場（天數=1）"] == 0 {
		t.Fatal("一場戰役都沒開場：電腦沒出兵，或者留守目標沒被改小")
	}
	if chiefRuns == 0 && dayRuns == 0 {
		t.Skip("戰役開場了但日循環沒跑——電腦對電腦不進戰術層（見註解）")
	}
}
