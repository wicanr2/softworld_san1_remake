//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
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
// 才跑的。remake 這一邊照著分岔：`game.fight` 在四個郡都沒有玩家時走
// `battle.AutoResolveAI`（不進戰術層、只用兩個數），有玩家才走
// `p.B.Auto()` 打滿三十天。
//
// 這支測試因此只驗玩家戰術層的兩個結束判定不會誤入電腦戰役；真正的
// 電腦對電腦日迴圈與逐將領傷亡由 TestAIvsAIBattleMatchesOriginal 驗。
//
// **玩家親征那條路已經補上了**：`TestBattleFinishesWithPlayer` 把玩家
// 驅動進主戰場、打到分出勝負，統帥條件 `0x24f8c` 跑了 105 次、
// 三十天判定 `0x250d4` 跑了 5 次，勝負也比過了。

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
		t.Log("符合原版分岔：電腦對電腦不進玩家戰術層的兩個結束判定")
	}
}

// TestAIvsAIBattleMatchesOriginal 從原版第一場電腦對電腦戰役的每日結算
// 入口擷取完整四軍力，再由 remake 的 AutoResolveAI 以同一盤面跑完。
// 比較單位是勝方、結算日數與每一位參戰將領的兵力，不用合計掩蓋分配錯誤。
func TestAIvsAIBattleMatchesOriginal(t *testing.T) {
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

	// 六種電腦性格輪流配置；留守目標改成 1，讓自然月份內確實會出兵。
	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), uint16(3+alive%3))
		alive++
	}
	var sg sortieGlobals
	sgOK := false
	o.OnCall(addr(0xb2b4), func(oo *oracle.Oracle) {
		if !sgOK {
			sg, sgOK = resolveSortieGlobals(oo), true
		}
		oo.SetWord(addr(sg.want), 1)
	})

	type warMemory struct {
		won, day, force, unit uint32
	}
	var w warMemory
	wOK := false
	resolve := func(oo *oracle.Oracle) {
		ds := uint32(oo.DSReg()) * 16
		far := func(slot uint16, off uint32) uint32 {
			return uint32(oo.Word(addr(ds+uint32(slot))))*16 + off
		}
		w = warMemory{
			won:   far(0xa89e, 0x20dc),
			day:   far(0xa8a2, 0x2100),
			force: far(0xa8b6, 0x175e),
			unit:  far(0xa896, 0x3502),
		}
		wOK = true
	}

	const fixedSeed = uint32(0x13579BDF)
	var model *battle.Battle
	wantSoldiers := map[int]int{}
	dailyCalls := 0
	compared := false

	// 原版軍力編號 0/1 是守方、2/3 是攻方；remake 的 Side 列舉先排攻方。
	toSide := [...]battle.Side{
		battle.MainDefender, battle.AidDefender,
		battle.MainAttacker, battle.AidAttacker,
	}

	o.OnCall(addr(0x1f538), func(oo *oracle.Oracle) {
		dailyCalls++
		if model != nil {
			return
		}
		if !wOK {
			resolve(oo)
		}

		// 在第一天任何 RND(11) 之前固定原版狀態；remake 同樣以此數起跑。
		ds := oo.DSReg()
		oo.SetWord(oracle.Addr{Seg: ds, Off: 0xa3ae}, uint16(fixedSeed&0xffff))
		oo.SetWord(oracle.Addr{Seg: ds, Off: 0xa3b0}, uint16(fixedSeed>>16))

		var units []*battle.Unit
		for army := 0; army < 4; army++ {
			n := int(oo.Word(addr(w.force + uint32(army*22+10))))
			for team := 0; team < n; team++ {
				rec := w.unit + uint32((army*10+team)*42)
				var leaders []battle.Leader
				for pos := 0; pos < 10; pos++ {
					idx := int(int16(oo.Word(addr(rec + uint32(pos*2)))))
					if idx < 0 {
						continue
					}
					g := genBase + uint32(idx*30)
					leaders = append(leaders, battle.Leader{
						Index: idx, Intel: oo.Byte(addr(g + 9)), War: oo.Byte(addr(g + 10)),
						Soldiers: int(oo.Word(addr(g + 22))),
					})
				}
				u := &battle.Unit{Side: toSide[army], Formation: battle.Formation(team), Leaders: leaders}
				if got, want := u.Soldiers(), int(oo.Word(addr(rec+30))); got != want {
					t.Errorf("原版軍力 %d 隊伍 %d 的逐將領兵力和 %d，部隊欄是 %d", army, team, got, want)
				}
				if got, want := u.Ability(), int(oo.Word(addr(rec+32))); got != want {
					t.Errorf("原版軍力 %d 隊伍 %d 的重算綜合能力 %d，部隊欄是 %d", army, team, got, want)
				}
				units = append(units, u)
			}
		}

		model = battle.New(battle.Setup{
			Field: battle.Generate(battle.Params{Prefecture: 1, LandValue: 100}),
			Seed:  fixedSeed,
		})
		model.Units = units
		model.Rice[battle.MainDefender] = int(oo.Word(addr(w.force + 0*22 + 8)))
		model.Rice[battle.MainAttacker] = int(oo.Word(addr(w.force + 2*22 + 8)))
		// Battle 的一般亂數是 xorshift32，原版是 MSC LCG；相同 seed 數字
		// 不代表相同骰序。這裡明示供應已由 rand oracle 驗過的原版序列，
		// 只比較同一規則在同一受控亂數輸入下的狀態轉移。
		seed := fixedSeed
		model.AutoResolveAIWithRoll(func(n int) int {
			var out int
			seed, out = game.MSCRand(seed)
			return out % n
		})
		for _, u := range model.Units {
			for _, l := range u.Leaders {
				wantSoldiers[l.Index] = l.Soldiers
			}
		}
	})

	// `0x1fb26` 緊接在傷亡寫回 `0x1f6fe` 之後；此刻尚未開始戰後安置。
	o.OnCall(addr(0x1fb26), func(oo *oracle.Oracle) {
		if compared || model == nil || !wOK {
			return
		}
		compared = true
		winner := int(int16(oo.Word(addr(w.won))))
		if got, want := winner == 2, model.AttackerWon; got != want {
			t.Errorf("勝方不同：原版勝方軍力=%d，remake 攻方勝=%v", winner, want)
		}
		if dailyCalls != model.Day {
			t.Errorf("結算日數不同：原版呼叫每日結算 %d 次，remake 在第 %d 天結束", dailyCalls, model.Day)
		}
		if dailyCalls <= 20 || model.Day <= 20 {
			t.Errorf("第 21 天以前不可能進傷亡段：原版=%d remake=%d",
				dailyCalls, model.Day)
		}
		for idx, want := range wantSoldiers {
			got := int(oo.Word(addr(genBase + uint32(idx*30+22))))
			if got != want {
				t.Errorf("人物 %d 戰後兵力：原版 %d，remake %d", idx, got, want)
			}
		}
		t.Logf("固定 seed=%#08x；第 %d 天結束，勝方軍力=%d，逐將領比較 %d 筆",
			fixedSeed, dailyCalls, winner, len(wantSoldiers))
	})

	const settle = 120_000_000
	for month := 1; month <= 3 && !compared; month++ {
		for p := 1; p <= state.PrefectureCount; p++ {
			o.SetWord(addr(staBase+uint32(p*176+18)), 20000)
			o.SetWord(addr(staBase+uint32(p*176+20)), 30000)
		}
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", month, k, err)
			}
			if compared {
				break
			}
		}
	}
	if !compared {
		t.Fatal("三個月內沒有完成任何一場電腦對電腦戰役")
	}
}
