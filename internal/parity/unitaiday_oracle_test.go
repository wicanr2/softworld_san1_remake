//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestZZUnitAIDayParity 對拍原版部隊 AI 的九支判斷式（Issue #22）：玩家
// 親征，盤面直寫記憶體（`stageABattle`），每一支電腦部隊每一天的決策
// ——**哪一支定案、對誰、走到哪**——與 remake 的 `DecideBase` 逐次比。
//
// 比的方法：攔決策鏈入口 `0x29014`，把原版當下的盤面（四個軍力的部隊
// 記錄、軍力記錄、地圖、天候、難度）拍下來，並記下這條鏈裡每一次
// `RND(n)` 擲出的值；鏈結束（`0x29132`）時讀原版定案的選項、目標與
// 部隊的落點。remake 從同一份盤面出發、用同一串骰值走 `DecideBase`，
// 三件事逐一相同才算對。
//
// **盤面每一次決策都從原版重拍**，所以比的是判斷式本身：交戰結算與
// 計謀判定裡的骰序是不是與原版逐次相同，不在這一支的範圍
//（那是後果常式，各自有對拍）。
func TestZZUnitAIDayParity(t *testing.T) {
	// 四張盤面，讓九支都輪得到（選項 1 是評估前置，每次都跑）：
	//   甲 玩家兩萬兵對五支各一千五 → 移動、弓箭、策略、快戰、休息
	//   乙 玩家三萬兵對五支各一千   → 退兵（相鄰敵軍四倍以上、總兵力比 ≥ 3）
	//   丙 玩家六千對五支各兩萬七、敵將謀略戰力 5 → 死戰（目標兵力比
	//      ≤ 0.23）；敵將能力壓低是讓玩家那支多撐幾天，決策才夠多
	//   丁 玩家一千三對五支各六千、敵將 5 → 對戰（RND(16)==0 那一擲在
	//      這張盤面第二天就出現；dosgolem 是決定性的，每次都一樣）
	boards := []struct {
		name                                   string
		soldiers, enemies, enemySoldiers, stats int
	}{
		{"甲", 20000, 5, 1500, 0}, {"乙", 30000, 5, 1000, 0}, {"丙", 6000, 5, 27000, 5},
		{"丁", 1300, 5, 6000, 5},
	}
	seen := map[int]int{}
	for _, bd := range boards {
		t.Run(bd.name, func(t *testing.T) {
			for k, v := range runUnitAIDayParity(t, bd.soldiers, bd.enemies, bd.enemySoldiers, bd.stats) {
				seen[k] += v
			}
		})
	}
	t.Logf("四張盤面合計，原版各選項定案：%v", seen)
	for opt := 2; opt <= 9; opt++ {
		if seen[opt] == 0 {
			t.Errorf("四張盤面裡選項 %d 一次都沒定案——盤面要再調", opt)
		}
	}
}

// runUnitAIDayParity 跑一張盤面，回傳原版各選項定案的次數。
func runUnitAIDayParity(t *testing.T, soldiers, enemies, enemySoldiers, enemyStats int) map[int]int {
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
	// 玩家一支部隊帶兩萬兵撐場、敵方五位（五支部隊）——決策多、而且
	// 打得完一場（守方五支輪流快戰，三十天內分得出勝負）。
	at, to := stageABattleWith(t, o, base, soldiers, enemies, enemySoldiers, enemyStats)

	var dgroup uint16
	o.OnCall(addr(0x2053c), func(o *oracle.Oracle) {
		if dgroup == 0 {
			dgroup = o.DSReg()
		}
	})
	cmdReads := 0
	o.OnCall(addr(0x27a68), func(*oracle.Oracle) { cmdReads++ })

	work := func() uint16 { return o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg}) }
	w16 := func(off int) int { return int(o.Word(oracle.Addr{Seg: work(), Off: uint16(off)})) }

	// 一次決策的紀錄。
	type decision struct {
		day, army, team int
		option          int
		tArmy, tTeam    int
		col, row        int // 鏈結束時部隊的落點
		rolls           []int
		rollNs          []int
		callers         []string
		around          []string
		model           *battle.Battle
		unit            *battle.Unit
		escapes         int
	}
	var decisions []*decision
	var cur *decision
	curOpt := 0
	options := map[uint32]int{
		0x29e78: 1, 0x29138: 2, 0x29344: 3, 0x2985c: 4, 0x29784: 5,
		0x29c56: 6, 0x29b82: 7, 0x29ade: 8, 0x29e2e: 9,
	}
	toSide := [...]battle.Side{battle.MainDefender, battle.AidDefender, battle.MainAttacker, battle.AidAttacker}
	teamForm := battle.DeployOrder()

	snapshot := func(army, team int) *decision {
		d := &decision{army: army, team: team, day: w16(0x2100)}
		field, err := battle.Load(o.Bytes(oracle.Addr{Seg: work(), Off: 0x163a}, 120), nil)
		if err != nil {
			t.Fatalf("戰場地圖讀不出來：%v", err)
		}
		if field.CityAt == battle.NoHex {
			field.CityAt = battle.FromOffset(w16(0x584), w16(0x586))
		}
		weather := battle.Clear
		switch w16(0x17bc) {
		case 1:
			weather = battle.Rainy
		case 2:
			weather = battle.Windy
		}
		b := battle.New(battle.Setup{Field: field, Weather: weather, Difficulty: w16(0x30fe)})
		b.Day = d.day
		b.Units = nil
		for a := 0; a < 4; a++ {
			arec := 0x175e + a*22
			b.Gold[toSide[a]] = w16(arec + 6)
			b.Rice[toSide[a]] = w16(arec + 8)
			// 五個隊伍槽全掃，活著的判準是將領數 > 0：軍力記錄的部隊數
			// （offset 10）在一支被打光之後會少一，但槽號不會往前補。
			for tm := 0; tm < 5; tm++ {
				rec := battleUnitBase + (a*battleUnitPer+tm)*battleUnitSize
				if w16(rec+unitLeaders) <= 0 {
					continue
				}
				u := &battle.Unit{Side: toSide[a], Formation: teamForm[tm],
					At:      battle.FromOffset(w16(rec+unitCol), w16(rec+unitRow)),
					Move:    w16(rec + unitMove),
					Arrows:  w16(rec + 20),
					Trapped: w16(rec + unitTrapped),
				}
				for pos := 0; pos < 10; pos++ {
					idx := w16(rec + pos*2)
					if idx == 0xFFFF {
						continue
					}
					g := genBase + uint32(idx*30)
					u.Leaders = append(u.Leaders, battle.Leader{
						Index: idx, Intel: o.Byte(addr(g + 9)), War: o.Byte(addr(g + 10)),
						Soldiers: int(o.Word(addr(g + 22))), Training: o.Byte(addr(g + 24)),
						Arms: o.Byte(addr(g + 25)), Troop: battle.TroopKind(o.Byte(addr(g + 21))),
					})
				}
				u.Started = u.Soldiers()
				if got, want := u.Soldiers(), w16(rec+unitSoldiers); got != want {
					t.Errorf("第 %d 天 軍力 %d 隊伍 %d：將領兵力和 %d，部隊記錄的兵士數 %d", d.day, a, tm, got, want)
				}
				b.Units = append(b.Units, u)
				if a == army && tm == team {
					d.unit = u
				}
			}
		}
		// 退兵逃得去的鄰郡（`0x23e34`–`0x23ef2`）：戰場所在郡的鄰郡裡
		// 無主或自己勢力的，扣掉對方助軍出兵的那一郡。
		pref := w16(0x1bf8)
		faction := w16(0x175e + army*22 + 16)
		other := 3
		if army >= 2 {
			other = 1
		}
		exclude := w16(0x175e + other*22 + 18)
		for k := 45; k <= 54; k++ {
			n := int(o.Byte(addr(staBase + uint32(pref*176+k))))
			if n == 0xFF || n == 0 || n == exclude {
				continue
			}
			owner := int(o.Byte(addr(staBase + uint32(n*176+30))))
			if owner != 0xFF && owner != faction {
				continue
			}
			b.Escapes[toSide[army]] = append(b.Escapes[toSide[army]],
				battle.Escape{Prefecture: n, Active: int(o.Byte(addr(staBase + uint32(n*176+22))))})
		}
		d.escapes = len(b.Escapes[toSide[army]])
		d.model = b
		// 診斷：這支部隊六個鄰格的佔位（原版 `es:0x2532`）與那些部隊的兵士數。
		if d.unit != nil {
			rec := battleUnitBase + (army*battleUnitPer+team)*battleUnitSize
			c, r := w16(rec+unitCol), w16(rec+unitRow)
			for dir := 0; dir < 6; dir++ {
				i := ((c%2)*6 + dir) * 2
				dc := int(int16(o.Word(oracle.Addr{Seg: dgroup, Off: uint16(0x7c6a + i)})))
				dr := int(int16(o.Word(oracle.Addr{Seg: dgroup, Off: uint16(0x7c82 + i)})))
				nc, nr := c+dc, r+dr
				if nc < 0 || nc >= 12 || nr < 0 || nr >= 10 {
					continue
				}
				occ := w16(0x2532 + (nr*12+nc)*2)
				if occ == 0xFFFF {
					continue
				}
				orec := battleUnitBase + ((occ/10)*battleUnitPer+occ%10)*battleUnitSize
				d.around = append(d.around, fmt.Sprintf("(%d,%d)=%d/%d 兵 %d 將 %d", nc, nr, occ/10, occ%10, w16(orec+unitSoldiers), w16(orec+unitLeaders)))
			}
		}
		if d.unit == nil {
			rec := battleUnitBase + (army*battleUnitPer+team)*battleUnitSize
			t.Logf("第 %d 天 軍力 %d 隊伍 %d：軍力記錄部隊數 %d、將領數 %d、兵士 %d、槽 %04x %04x、位置 (%d,%d)",
				d.day, army, team, w16(0x175e+army*22+10), w16(rec+unitLeaders), w16(rec+unitSoldiers),
				w16(rec), w16(rec+2), w16(rec+unitCol), w16(rec+unitRow))
		}
		return d
	}

	o.OnCall(addr(0x29014), func(o *oracle.Oracle) {
		if dgroup == 0 {
			return
		}
		cur = snapshot(int(int16(o.Arg(0))), int(int16(o.Arg(1))))
		curOpt = 0
	})
	for a, n := range options {
		n := n
		o.OnCall(addr(a), func(*oracle.Oracle) {
			if cur != nil {
				curOpt = n
			}
		})
	}
	// `RND(n)` 擲出什麼：**從下一次讀到的種子回推**。`rand()` 的輸出就是
	// 更新後種子的第 16..30 位（`game.MSCRand`），而種子只有 `rand()` 會動，
	// 所以下一次進 `RND` 時（或鏈結束時）讀到的種子，就是上一擲的結果。
	// 不從進入時的種子往前算——那要假設這一份 `rand()` 的算式，回推不必。
	pendingN := 0
	seedNow := func(o *oracle.Oracle) uint32 {
		ds := o.DSReg()
		return uint32(o.Word(oracle.Addr{Seg: ds, Off: 0xa3ae})) | uint32(o.Word(oracle.Addr{Seg: ds, Off: 0xa3b0}))<<16
	}
	flushRoll := func(o *oracle.Oracle) {
		if pendingN > 0 && cur != nil {
			cur.rolls = append(cur.rolls, int((seedNow(o)>>16)&0x7fff)%pendingN)
			cur.rollNs = append(cur.rollNs, pendingN)
		}
		pendingN = 0
	}
	o.OnCall(addr(0x10b0c), func(o *oracle.Oracle) {
		if cur == nil {
			return
		}
		flushRoll(o)
		// `RND(0)`（移動那一支的 `push 0`）回 0 而且不動種子，remake 沒有
		// 對應的一擲，不記。
		if n := int(int16(o.Arg(0))); n > 0 {
			pendingN = n
			cur.callers = append(cur.callers, fmt.Sprintf("%d@%05x", n, o.Caller().Linear()))
		}
	})
	// 弓箭的目標不進 `es:0x31a8`，從射箭常式的參數讀。
	o.OnCall(addr(0x2a80a), func(o *oracle.Oracle) {
		if cur != nil && curOpt == 4 {
			cur.tArmy, cur.tTeam = int(int16(o.Arg(2))), int(int16(o.Arg(3)))
		}
	})
	o.OnCall(addr(0x29132), func(o *oracle.Oracle) {
		if cur == nil {
			return
		}
		flushRoll(o)
		seg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa92e})
		if o.Word(oracle.Addr{Seg: seg, Off: 0x31c0}) != 0 {
			cur.option = 0 // 沒有任何一支定案（不該發生：選項 9 無條件）
		} else {
			cur.option = curOpt
		}
		if cur.option >= 5 && cur.option <= 8 {
			cur.tArmy = int(int16(o.Word(oracle.Addr{Seg: seg, Off: 0x31a8})))
			cur.tTeam = int(int16(o.Word(oracle.Addr{Seg: seg, Off: 0x1604})))
		}
		rec := battleUnitBase + (cur.army*battleUnitPer+cur.team)*battleUnitSize
		cur.col, cur.row = w16(rec+unitCol), w16(rec+unitRow)
		decisions = append(decisions, cur)
		cur = nil
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
	_, _ = placeNextToDefender(t, o, dgroup)

	days := 32
	if v := envOr("SAN1_DAYS", ""); v != "" {
		fmt.Sscan(v, &days)
	}
	// 鍵不照天數送，照「還有沒有新決策」送：玩家那支每天「0」休息，
	// 「Y」答掉沿路的確認；電腦選了對戰（選項 7）會進對戰子畫面，
	// 那裡的每一位將領也吃「0」休息，子畫面打完才回到主戰場——所以
	// 連續十五輪沒有新決策才當作這一場結束。
	quiet := 0
	o.Drain()
	o.PressScan("Y")
	if err := o.Run(120_000_000); err != nil {
		t.Fatalf("開戰確認停止：%v", err)
	}
	for i := 0; i < days*12 && quiet < 15; i++ {
		before := len(decisions)
		for _, k := range []string{"0", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(60_000_000); err != nil {
				t.Fatalf("第 %d 輪送 %q 停止：%v", i+1, k, err)
			}
		}
		if len(decisions) == before {
			quiet++
			if quiet == 14 {
				dumpScreen(t, o, fmt.Sprintf("unitaiday-%d-quiet", soldiers))
			}
		} else {
			quiet = 0
		}
		if w16(0x2100) > days {
			break
		}
	}

	// remake 這一邊：同一份盤面、同一串骰值。
	sideName := func(a int) string { return toSide[a].String() }
	bad := 0
	byOpt := map[int]int{}
	for _, d := range decisions {
		if d.unit == nil {
			t.Errorf("第 %d 天 %s 隊伍 %d：原版在替一支 remake 認不出來的部隊決策", d.day, sideName(d.army), d.team)
			bad++
			continue
		}
		i := 0
		var asked []int
		d.model.UseRoll(func(n int) int {
			asked = append(asked, n)
			if i < len(d.rolls) {
				v := d.rolls[i]
				i++
				if n > 0 {
					return v % n
				}
				return 0
			}
			return 0
		})
		got := d.model.DecideBase(d.unit)
		byOpt[d.option]++
		ok := got.Option == d.option
		want := fmt.Sprintf("選項 %d", d.option)
		have := fmt.Sprintf("選項 %d", got.Option)
		if d.option >= 4 && d.option <= 8 {
			want += fmt.Sprintf(" 對 %s 隊伍 %d", sideName(d.tArmy), d.tTeam)
			if got.Target != nil {
				have += fmt.Sprintf(" 對 %s %s", got.Target.Side, got.Target.Formation)
				if got.Target.Side != toSide[d.tArmy] || got.Target.Formation != teamForm[d.tTeam] {
					ok = false
				}
			} else {
				ok = false
			}
		}
		if d.option == 3 || got.Option == 3 {
			x, y := battle.ToOffset(d.unit.At)
			want += fmt.Sprintf(" 落點 (%d,%d)", d.col, d.row)
			have += fmt.Sprintf(" 落點 (%d,%d)", x, y)
			if x != d.col || y != d.row {
				ok = false
			}
		}
		line := fmt.Sprintf("第 %2d 天 %s 隊伍 %d：原版 %s；remake %s；骰 %v（n=%v）remake 問了 %v；可逃鄰郡 %d",
			d.day, sideName(d.army), d.team, want, have, d.rolls, d.rollNs, asked, d.escapes)
		if ok {
			t.Log("✓ " + line)
		} else {
			t.Errorf("✗ %s；RND 的呼叫端 %v；鄰格 %v；本隊兵 %d", line, d.callers, d.around, d.unit.Soldiers())
			bad++
		}
	}
	t.Logf("決策 %d 次，原版各選項定案：%v，不同 %d 次", len(decisions), byOpt, bad)
	if len(decisions) < 10 {
		t.Errorf("只比到 %d 次決策——樣本太少", len(decisions))
	}
	return byOpt
}
