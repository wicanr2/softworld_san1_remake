//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 戰役層的執行期對拍：原版整編完的部隊，remake 的公式算不算得出同一個數。
//
// 這是**逐日對拍的第一段**。逐日要比的是「同一支部隊在第 N 天的狀態」，
// 而那件事只有在「第 0 天兩邊是同一支部隊」的前提下才有意義——編隊是
// 原版擲骰擲出來的，所以這裡不重現編隊，而是**讀原版自己分好的隊**，
// 拿同一組將領餵給 remake 的公式。
//
// 電腦對電腦那條路不進戰術層（`docs/re/05` §7.1），所以要玩家在場：
// 盤面由 `stageABattle` 直接寫記憶體擺出來，按鍵序列走到主戰場
// （`docs/re/05` §7）。
//
// 部隊記錄 42 bytes，基底 `es:[0x3502]`，索引 `軍力×10 + 隊伍`
// （`docs/re/05` §3.3）。工作區在另一個段，段值存在 `ds:0xa872`。
const (
	battleWorkSeg  = 0xa872 // ds:0xa872 ＝ 戰場工作區的段
	battleUnitBase = 0x3502 // 部隊記錄的基底（工作區內）
	battleUnitSize = 42
	battleArmies   = 4  // 主守、助守、主攻、助攻
	battleTeams    = 5  // 一個軍團五個隊伍
	battleUnitPer  = 10 // 陣列跨距

	unitCol      = 22 // 欄
	unitRow      = 24 // 列
	unitLeaders  = 28 // 將領人數
	unitSoldiers = 30 // 兵士數
	unitAbility  = 32 // 綜合能力
	unitTrapped  = 26 // 中陷阱之後不能動的天數（`0x2b648`）
	unitCap      = 34 // 這支部隊一天的移動力上限（`0x271d7` 寫）
	unitMove     = 36 // 這一天剩下的移動力
)

// TestBattleUnitsMatchTheOriginal 對拍整編完的部隊：兵士數、綜合能力、
// 移動力三欄，逐支比。
func TestBattleUnitsMatchTheOriginal(t *testing.T) {
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
	fielded := 0
	o.OnCall(addr(0x22704), func(o *oracle.Oracle) { fielded++ })
	// `0x27a63` 是每天的命令提示讀鍵那一支（`lcall 1058:0e24`），
	// `0x27a68` 是它回來的那一刻——**攔回來的位址**，那是確定的指令
	// 邊界。它一跑就代表紮寨問完、命令提示已經把一個鍵吃掉了。
	// **紮寨要幾個鍵不固定**，用這個當判準比數按鍵可靠。
	cmdReads := 0
	o.OnCall(addr(0x27a68), func(o *oracle.Oracle) { cmdReads++ })
	// `0x28af5` 是計謀選單讀完鍵回來的那一刻——**攔回來的位址**才是
	// 確定的指令邊界（`docs/re/05` §7.0 的那個坑）。
	plotMenu := 0
	o.OnCall(addr(0x28af5), func(o *oracle.Oracle) { plotMenu++ })

	driveIntoBattle(t, o, at, to)
	if dgroup == 0 {
		t.Fatal("沒有進到主戰場 0x2053c——按鍵序列或盤面不對")
	}
	if fielded == 0 {
		t.Fatal("戰場沒有畫出來——整編的按鍵序列不對")
	}
	work := o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg})
	t.Logf("戰場工作區的段 ＝ %#06x（線性 %#07x）", work, uint32(work)<<4)

	// 人物表要在**進戰場之後**讀：整編會改兵力（部隊帶走的兵）。
	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	gen := o.Bytes(addr(base+uint32(nMas+nSta)), nGen)
	_ = nGen

	w16 := func(off int) int {
		v := o.Word(oracle.Addr{Seg: work, Off: uint16(off)})
		return int(v)
	}
	// ── 交戰的傷亡：鉤子要早早掛上 ─────────────────────────
	//
	// **交戰不必自己按 `2` 才會發生**：玩家那支一走到守軍旁邊，
	// 電腦就會打過來。攔截點掛晚了那幾次就漏掉了——量到的一次是
	// 第 8 天，等移動走完才掛就一次都沒收到。
	//
	// 每一次交戰結算（`0x2a224`）進去時把參與者、模式與雙方的兵、
	// 綜合能力、所在地形讀下來。**參與者不用從差值反推**，
	// `o.Arg` 直接讀得到五個參數（`docs/re/05` §3.6）。
	type bout struct {
		aArmy, aTeam, dArmy, dTeam, mode   int
		aSol, aAbi, aTer, dSol, dAbi, dTer int
	}
	var bouts []bout
	recOf := func(army, team int) int {
		return battleUnitBase + (army*battleUnitPer+team)*battleUnitSize
	}
	terrainAt := func(col, row int) byte {
		return byte(o.Word(oracle.Addr{
			Seg: work, Off: uint16(0x163a + row*12 + col)})) & 0x0f
	}
	terrainOfUnit := func(rec int) int {
		return int(terrainAt(w16(rec+unitCol), w16(rec+unitRow)))
	}
	o.OnCall(addr(0x2a224), func(o *oracle.Oracle) {
		aa, at2 := int(o.Arg(0)), int(o.Arg(1))
		da, dt := int(o.Arg(2)), int(o.Arg(3))
		ra, rd := recOf(aa, at2), recOf(da, dt)
		bouts = append(bouts, bout{
			aArmy: aa, aTeam: at2, dArmy: da, dTeam: dt, mode: int(o.Arg(4)),
			aSol: w16(ra + unitSoldiers), aAbi: w16(ra + unitAbility),
			aTer: terrainOfUnit(ra),
			dSol: w16(rd + unitSoldiers), dAbi: w16(rd + unitAbility),
			dTer: terrainOfUnit(rd),
		})
	})

	units, checked, bad := 0, 0, 0
	var slots []unitSlot
	for army := 0; army < battleArmies; army++ {
		for team := 0; team < battleTeams; team++ {
			rec := battleUnitBase + (army*battleUnitPer+team)*battleUnitSize
			if w16(rec) == 0xFFFF {
				continue // 沒有這支部隊
			}
			n := w16(rec + unitLeaders)
			if n <= 0 {
				continue
			}
			units++
			var leaders []battle.Leader
			for i := 0; i < n && i < 10; i++ {
				idx := w16(rec + i*2)
				if idx == 0xFFFF || idx*30+30 > len(gen) {
					continue
				}
				r := gen[idx*30:]
				leaders = append(leaders, battle.Leader{
					Index:    idx,
					War:      r[10],
					Intel:    r[9],
					Soldiers: int(r[22]) | int(r[23])<<8,
					Training: r[24],
					Arms:     r[25],
				})
			}
			u := &battle.Unit{Leaders: leaders}
			sum := 0
			for _, l := range leaders {
				sum += l.Soldiers
			}
			gotSol, gotAbi := w16(rec+unitSoldiers), w16(rec+unitAbility)
			line := fmt.Sprintf("軍力 %d 隊伍 %d（%d 位將）", army, team, n)
			for _, x := range []struct {
				name       string
				orig, ours int
			}{
				{"兵士數", gotSol, sum},
				{"綜合能力", gotAbi, u.Ability()},
			} {
				checked++
				if x.orig != x.ours {
					bad++
					t.Errorf("%s：%s 原版 %d／remake %d", line, x.name, x.orig, x.ours)
				}
			}
			t.Logf("%s 兵 %d 綜合能力 %d", line, gotSol, gotAbi)
			slots = append(slots, unitSlot{army, team, rec, u})
		}
	}
	if units == 0 {
		t.Fatal("一支部隊都沒讀到——部隊記錄的位置或工作區的段不對")
	}
	t.Logf("整編出 %d 支部隊，比了 %d 個欄位，對不上 %d 個", units, checked, bad)

	// 紮寨：提示是 `數字鍵選方向 / 4 5 6 / 1 2 3 / 0:紮寨`（`DS:0x7f9e`），
	// **1–6 移游標、`0` 才是紮下去**，一支一支問。要幾個鍵不固定，所以
	// 判準是**命令選單有沒有印出來**（`0x27a40`），不是數按鍵。
	// 提示出現之後就不能再送——那一鍵會被當成當天的命令吃掉。
	for step := 1; step <= 10 && cmdReads == 0; step++ {
		o.Drain()
		o.PressScan("0")
		if err := o.Run(40_000_000); err != nil {
			t.Fatalf("紮寨第 %d 步停止：%v", step, err)
		}
	}
	if cmdReads == 0 {
		t.Fatal("紮完寨沒走到每天的命令提示（0x27a68 沒被執行到）")
	}
	t.Logf("紮完寨，最後一個 `0` 已經被當成第 1 天的命令（休息）")

	// **移動力要等紮完寨才比**：offset 36 是「這一天剩下的」，佈陣的
	// 時候還是 0。offset 34 是這支部隊一天的上限。
	for _, sl := range slots {
		cap34 := w16(sl.rec + unitCap)
		var who strings.Builder
		for _, l := range sl.u.Leaders {
			fmt.Fprintf(&who, "槽 %d 訓 %d 武裝 %d 兵 %d；",
				l.Index, l.Training, l.Arms, l.Soldiers)
		}
		checked++
		if got := sl.u.MovePoints(); got != cap34 {
			bad++
			t.Errorf("%d-%d 移動力上限：原版 %d／remake %d｜%s",
				sl.army, sl.team, cap34, got, who.String())
		}
		t.Logf("%d-%d 移動力上限 %d（剩 %d）｜%s",
			sl.army, sl.team, cap34, w16(sl.rec+unitMove), who.String())
	}

	// ── 逐日對拍 ─────────────────────────────────────────────
	//
	// 玩家的部隊休息一天就是兩個鍵：`0`（選單 `DS:0x7f22` 的 0.休息）
	// 再 `Y`（`0x1538c` 的 Y/N 確認）。**命令是單一 ASCII 不是數字欄位**，
	// 送 Enter 答 Y/N 的話天數會一直停在 1（`docs/re/05` §7.0）。
	//
	// 每天結束時原版把剩下的移動力回填到上限：`剩下的 ← max(剩下的, 上限)`
	// （`0x24ee1`）。這裡逐日比那一欄——**只比沒動過的部隊**：兵、欄、列
	// 都沒變才代表它這一天沒走也沒打，那一支的轉移才是我們模型裡的那條。
	type dayState struct{ move, sol, col, row int }
	read := func(rec int) dayState {
		return dayState{w16(rec + unitMove), w16(rec + unitSoldiers),
			w16(rec + unitCol), w16(rec + unitRow)}
	}
	prev := make([]dayState, len(slots))
	for i, sl := range slots {
		prev[i] = read(sl.rec)
	}
	days, moved := 0, 0
	for d := 1; d <= 7; d++ {
		// 第 1 天的 `0` 在紮寨那一段就被吃掉了，只補確認。
		keys := []string{"0", "Y"}
		if d == 1 {
			keys = []string{"Y"}
		}
		for _, k := range keys {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(60_000_000); err != nil {
				t.Fatalf("第 %d 天送 %q 停止：%v", d, k, err)
			}
		}
		got := int(o.Word(oracle.Addr{Seg: work, Off: 0x2100}))
		if got != d+1 {
			t.Fatalf("送完第 %d 天的命令，天數是 %d，應該是 %d"+
				"——按鍵序列不對（`docs/re/05` §7.0）", d, got, d+1)
		}
		days++
		var sb strings.Builder
		for i, sl := range slots {
			cur := read(sl.rec)
			// remake 這一邊的日轉移。**每一支都休息**：玩家那一支是
			// 我們送的 `0`，電腦那幾支是它自己選的，量到的四支守軍
			// 每天也是 +2。休息夾在 15（`0x27c2d`），開新的一天再把
			// 不足上限的補到上限（`0x24ee1`）。
			want := prev[i].move + battle.RestMove
			if want > battle.MoveMax {
				want = battle.MoveMax
			}
			if capMove := sl.u.MovePoints(); want < capMove {
				want = capMove
			}
			same := cur.sol == prev[i].sol && cur.col == prev[i].col &&
				cur.row == prev[i].row
			if !same {
				moved++
				fmt.Fprintf(&sb, "[%d-%d 動了 兵 %d→%d 格 %d,%d→%d,%d] ",
					sl.army, sl.team, prev[i].sol, cur.sol,
					prev[i].col, prev[i].row, cur.col, cur.row)
				prev[i] = cur
				continue
			}
			checked++
			if cur.move != want {
				bad++
				t.Errorf("第 %d 天 %d-%d 剩下的移動力：原版 %d／remake %d"+
					"（前一天 %d，上限 %d）", d, sl.army, sl.team,
					cur.move, want, prev[i].move, sl.u.MovePoints())
			}
			fmt.Fprintf(&sb, "[%d-%d 移 %d] ", sl.army, sl.team, cur.move)
			prev[i] = cur
		}
		t.Logf("第 %d 天（天數 %d）：%s", d, got, sb.String())
	}
	t.Logf("逐日跑了 %d 天，其中 %d 支次動過不比；三欄總共比了 %d 個，對不上 %d 個",
		days, moved, checked, bad)

	// ── 移動的花費 ───────────────────────────────────────────
	//
	// 命令 `1` 進移動模式，之後 1–6 一律當方向（`DS:0x7f9e` 的
	// `4 5 6` 在上、`1 2 3` 在下），**只有 Enter 離得開**
	// （`0x27d63` 的 `cmpw $0xd`）。
	//
	// 戰場地圖在工作區的 `0x163a + 列×12 + 欄`，每格低四位是地形碼。
	// 走進一格扣掉的移動力應該等於 `DS:0x7c42` 那張表——remake 這一邊
	// 是 `battle.MoveCost`，地形碼的對照是 `battle.TerrainOfCode`。
	var me *unitSlot
	for i := range slots {
		if slots[i].army == 2 {
			me = &slots[i]
		}
	}
	if me == nil {
		t.Fatal("盤面上找不到主攻軍——玩家沒有部隊就走不了")
	}
	o.Drain()
	o.PressScan("1")
	if err := o.Run(40_000_000); err != nil {
		t.Fatalf("進移動模式停止：%v", err)
	}
	steps := 0
	for step := 1; step <= 2; step++ {
		was := read(me.rec)
		o.Drain()
		o.PressScan("5") // 5 ＝ 往上
		if err := o.Run(40_000_000); err != nil {
			t.Fatalf("移動第 %d 步停止：%v", step, err)
		}
		now := read(me.rec)
		if now.col == was.col && now.row == was.row {
			t.Logf("移動第 %d 步沒走成（格 %d,%d，剩 %d）——大概是走不進去",
				step, was.col, was.row, was.move)
			break
		}
		steps++
		code := terrainAt(now.col, now.row)
		want := battle.MoveCost(battle.TerrainOfCode(code), 0)
		got := was.move - now.move
		checked++
		if got != want {
			bad++
			t.Errorf("移動第 %d 步走進 (%d,%d)（地形碼 %d ＝ %v）："+
				"原版扣 %d 點／remake 的表是 %d 點",
				step, now.col, now.row, code,
				battle.TerrainOfCode(code), got, want)
		}
		t.Logf("移動第 %d 步：(%d,%d)→(%d,%d) 地形碼 %d（%v）扣 %d 點，剩 %d",
			step, was.col, was.row, now.col, now.row, code,
			battle.TerrainOfCode(code), got, now.move)
	}
	if steps == 0 {
		t.Error("一步都沒走成——移動模式的按鍵或方向不對")
	}
	o.Drain()
	o.PressScan("\r") // 離開移動模式
	if err := o.Run(40_000_000); err != nil {
		t.Fatalf("離開移動模式停止：%v", err)
	}
	t.Logf("走了 %d 步；三欄加移動花費總共比了 %d 個，對不上 %d 個",
		steps, checked, bad)

	// ── 交戰的傷亡：比對上面收到的每一次 ────────────────────
	board := func(tag string) {
		var sb strings.Builder
		for _, sl := range slots {
			c := read(sl.rec)
			fmt.Fprintf(&sb, "[%d-%d 格 %d,%d 兵 %d 移 %d] ",
				sl.army, sl.team, c.col, c.row, c.sol, c.move)
		}
		t.Logf("%s：天數 %d｜%s", tag,
			o.Word(oracle.Addr{Seg: work, Off: 0x2100}), sb.String())
	}
	board("離開移動模式之後")
	if len(bouts) == 0 {
		// 電腦沒打過來就自己按一次對戰。
		o.Drain()
		o.PressScan("2")
		if err := o.Run(120_000_000); err != nil {
			t.Fatalf("對戰停止：%v", err)
		}
		board("按了對戰之後")
	}
	if len(bouts) == 0 {
		t.Fatal("一次交戰結算都沒跑——玩家那支大概沒和守軍相鄰")
	}
	// 第 i 次的結果就是第 i+1 次進去時的兵；最後一次拿收工的盤面比。
	after := func(i, army, team int) int {
		if i+1 < len(bouts) {
			b := bouts[i+1]
			if b.aArmy == army && b.aTeam == team {
				return b.aSol
			}
			if b.dArmy == army && b.dTeam == team {
				return b.dSol
			}
		}
		return w16(recOf(army, team) + unitSoldiers)
	}
	for i, b := range bouts {
		da := battle.MeleeDamage(battle.MeleeAttackValue(battle.TerrainOfCode(byte(b.aTer))),
			b.aSol, b.aAbi, battle.StrikeMultiplier(b.mode), battle.MeleeAttackScale)
		dd := battle.MeleeDamage(battle.MeleeDefendValue(battle.TerrainOfCode(byte(b.dTer))),
			b.dSol, b.dAbi, 1, battle.MeleeDefendScale)
		wantA := battle.MeleeSurvivors(b.aSol, battle.MeleeRatio(dd, b.aSol))
		wantD := battle.MeleeSurvivors(b.dSol, battle.MeleeRatio(da, b.dSol))
		gotA := after(i, b.aArmy, b.aTeam)
		gotD := after(i, b.dArmy, b.dTeam)
		for _, x := range []struct {
			who       string
			got, want int
		}{
			{fmt.Sprintf("甲 %d-%d", b.aArmy, b.aTeam), gotA, wantA},
			{fmt.Sprintf("乙 %d-%d", b.dArmy, b.dTeam), gotD, wantD},
		} {
			checked++
			if x.got != x.want {
				bad++
				t.Errorf("交戰 %d（模式 %d）%s 的兵：原版 %d／remake %d",
					i+1, b.mode, x.who, x.got, x.want)
			}
		}
		t.Logf("交戰 %d：%d-%d（地形 %d 兵 %d 能力 %d）打 %d-%d"+
			"（地形 %d 兵 %d 能力 %d）模式 %d｜殺傷 %d／%d → 兵 %d／%d",
			i+1, b.aArmy, b.aTeam, b.aTer, b.aSol, b.aAbi,
			b.dArmy, b.dTeam, b.dTer, b.dSol, b.dAbi, b.mode, da, dd, gotA, gotD)
	}
	t.Logf("打了 %d 次交戰；整支對拍總共比了 %d 個欄位，對不上 %d 個",
		len(bouts), checked, bad)

	// ── 計謀 ─────────────────────────────────────────────────
	//
	// 命令 `6` 進計謀選單（`DS:0x8113`：`1.火攻 2.水渰 3.陷阱 4.誘敵
	// 5.燒糧 6.圍攻`），選單讀一個 ASCII（`0x28af5`），`鍵 − '1'` 落在
	// 0–5 之外就取消。接著 `0x28d7a` 用 `DS:0x7c82` 的位移表算目標格
	// ——**那一格沒有敵人就直接回 −1**，所以要先走到守軍旁邊。
	//
	// 這裡要問的是「這六支在實跑裡走得到嗎」，不是它們的亂數結果。
	// 判準是**費用有沒有照 `DS:0x7f62` 扣**（陷阱 100 金）與
	// 陷阱的天數落在 1..5（`0x2b648`：`RND(5) + 1`）。
	armyRec := func(army int) int { return 0x175e + army*22 }
	goldBefore := w16(armyRec(2) + 6)
	// **要分得出 `6` 是被誰吃掉的。** 命令提示讀走一個鍵時 `0x27a68`
	// 會跑一次；沒跑就代表這一鍵落到別的提示上，那時再送選項會被
	// 計謀選單當成選擇（第一版就是這樣選到圍攻的）。
	chose := false
	for try := 1; try <= 4 && !chose; try++ {
		was := cmdReads
		o.Drain()
		o.PressScan("6")
		if err := o.Run(80_000_000); err != nil {
			t.Fatalf("送計謀第 %d 次停止：%v", try, err)
		}
		if cmdReads == was {
			continue // 這一鍵不是命令提示收的，再試
		}
		// **計謀先問方向再問哪一計。** `0x28d7a` 把那個鍵當方向索引，
		// 查 `DS:0x7c6a`（欄位移）與 `DS:0x7c82`（列位移）——兩張各
		// 12 格，`(欄 % 2) × 6 + 方向`——算出目標格；**那一格沒有敵人
		// 就直接回 −1**。玩家那支在 (6,4)、守軍 0-2 在 (6,3)，
		// 所以方向是 `5`（往上，索引 4：欄位移 0、列位移 −1）。
		for _, k := range []string{"5", "3"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(80_000_000); err != nil {
				t.Fatalf("送計謀的 %q 停止：%v", k, err)
			}
		}
		chose = true
	}
	if !chose {
		t.Fatal("送了四次 `6` 都不是命令提示收的——玩家那支這時沒輪到")
	}
	goldAfter := w16(armyRec(2) + 6)
	t.Logf("計謀：選單進去 %d 次；主攻軍的金 %d → %d（陷阱的費用是 %d）",
		plotMenu, goldBefore, goldAfter, battle.Trap.Cost())
	trapped := 0
	for _, sl := range slots {
		if d := w16(sl.rec + unitTrapped); d > 0 {
			trapped++
			checked++
			// **智 98 起跳會加碼**（`0x2b648`：再加 `RND(5) + 2`），
			// 而 stageABattle 把玩家的智墊到 99，所以上限是 11 不是 5。
			hi := battle.TrapDays(99, battle.TrapSpread-1, battle.TrapSpread-1)
			if d < 1 || d > hi {
				bad++
				t.Errorf("%d-%d 中陷阱 %d 天，原版的範圍是 1..%d", sl.army, sl.team, d, hi)
			}
			t.Logf("%d-%d 中陷阱 %d 天", sl.army, sl.team, d)
		}
	}
	if trapped == 0 && goldBefore == goldAfter {
		t.Log("陷阱沒放成（費用沒扣、也沒有人中招）——四道門有一道擋下來了")
	}
	t.Logf("整支對拍最後：比了 %d 個欄位，對不上 %d 個", checked, bad)
	dumpScreen(t, o, "battle-day7")
}

// unitSlot 是一支部隊在原版記錄裡的位置，加上 remake 這一邊對應的物件。
type unitSlot struct {
	army, team int
	rec        int
	u          *battle.Unit
}

// driveIntoBattle 送「軍事 → 發動戰役 → 出兵郡 → 目標郡 → 整編」那一串鍵。
//
// 序列與 `TestZZDumpBattleCode` 是同一組（`docs/re/05` §7）：
// **每一個提示都要 Enter，選單也一樣**。
func driveIntoBattle(t *testing.T, o *oracle.Oracle, at, to int) {
	t.Helper()
	driveIntoBattleGap(t, o, at, to, 0)
}

// driveIntoBattleGap 是本體；gap > 0 時同一段裡的每個鍵之間跑這麼多條
// 指令再送下一個。加強版要這樣——連著灌的「2⏎」只收到 2，Enter 被
// 重繪吃掉（與 `playercmd` 量到的同一件事）；原版照舊一段一送。
func driveIntoBattleGap(t *testing.T, o *oracle.Oracle, at, to int, gap uint64) {
	t.Helper()
	const settle = 40_000_000
	spell := func(n int) string {
		out := ""
		for _, c := range fmt.Sprintf("%d", n) {
			out += string(c) + "|"
		}
		return out + enterMark
	}
	menu := "2|" + enterMark
	one := func(n int) string {
		return fmt.Sprintf("%d|%s|1|%s", n, enterMark, enterMark)
	}
	org := enterMark + "|" + one(1) + "|N|" + one(2) + "|Y" +
		"|" + enterMark + "|" + one(1) + "|Y|5000|" + enterMark +
		"|9000|" + enterMark + "|Y"
	keys := envOr("SAN1_BATTLEKEY",
		menu+"|"+menu+"|"+spell(at)+"|"+spell(to)+"|"+org)
	for i, seg := range strings.Split(keys, "|") {
		o.Drain()
		seg = strings.ReplaceAll(seg, enterMark, "\r")
		if gap <= 0 {
			o.PressScan(seg)
		} else {
			for j, r := range seg {
				if j > 0 {
					if err := o.Run(gap); err != nil {
						t.Fatalf("送第 %d 段的第 %d 個鍵時停止：%v", i+1, j+1, err)
					}
				}
				o.PressScan(string(r))
			}
		}
		if err := o.Run(settle); err != nil {
			t.Fatalf("送第 %d 段（%q）時停止：%v", i+1, seg, err)
		}
		// SAN1_TRACE 非空就每一段存一張畫面（配 SAN1_SHOTS），看序列在
		// 哪一步走偏。
		if envOr("SAN1_TRACE", "") != "" {
			dumpScreen(t, o, fmt.Sprintf("drive-%02d", i+1))
		}
	}
	if err := o.Run(settle); err != nil {
		t.Fatalf("戰場畫面停止：%v", err)
	}
}
