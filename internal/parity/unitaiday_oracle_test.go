//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
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
// **盤面每一次決策都從原版重拍**，所以比的是判斷式本身。
//
// `SAN1_NORESYNC=1` 多跑一段**不重拍**（Issue #24）：只拿第一條鏈的盤面，
// 之後 remake 自己走一整場——每支部隊的決策、後果常式（交戰結算、弓箭、
// 計謀、退兵、被擒處置）、回合結束的投敵判定與回填、玩家那幾支的休息、
// 日結算與天候——骰用 MSC 的 LCG 從原版的種子接，每一條鏈進來時比種子、
// 這支部隊的狀態與決策。原版讀鍵時會重新播種（`docs/re/03` §1.45），
// 那幾個點照原版的值接。
func TestZZUnitAIDayParity(t *testing.T) {
	// 四張盤面，讓九支都輪得到（選項 1 是評估前置，每次都跑）：
	//   甲 玩家兩萬兵對五支各一千五 → 移動、弓箭、策略、快戰、休息
	//   乙 玩家三萬兵對五支各一千   → 退兵（相鄰敵軍四倍以上、總兵力比 ≥ 3）
	//   丙 玩家六千對五支各兩萬七、敵將謀略戰力 5 → 死戰（目標兵力比
	//      ≤ 0.23）；敵將能力壓低是讓玩家那支多撐幾天，決策才夠多
	//   丁 玩家一千三對五支各六千、敵將 5 → 對戰（RND(16)==0 那一擲在
	//      這張盤面第二天就出現；dosgolem 是決定性的，每次都一樣）
	//   戊 玩家兩萬兵、智 10 對五支各一千五、敵將智武 99 → 計謀**成功**的
	//      那幾條路（火攻／水淹／陷阱／誘敵／燒糧／圍攻的效果與骰序，
	//      Issue #24）；前四張盤面玩家的智是 99，電腦的計謀一次都不會成
	boards := []struct {
		name                                             string
		soldiers, enemies, enemySoldiers, stats, myIntel int
	}{
		{"甲", 20000, 5, 1500, 0, 0}, {"乙", 30000, 5, 1000, 0, 0}, {"丙", 6000, 5, 27000, 5, 0},
		{"丁", 1300, 5, 6000, 5, 0}, {"戊", 20000, 5, 1500, 99, 10},
	}
	seen := map[int]int{}
	for _, bd := range boards {
		t.Run(bd.name, func(t *testing.T) {
			for k, v := range runUnitAIDayParity(t, bd.soldiers, bd.enemies, bd.enemySoldiers, bd.stats, bd.myIntel) {
				seen[k] += v
			}
		})
	}
	t.Logf("五張盤面合計，原版各選項定案：%v", seen)
	for opt := 2; opt <= 9; opt++ {
		if seen[opt] == 0 {
			t.Errorf("五張盤面裡選項 %d 一次都沒定案——盤面要再調", opt)
		}
	}
}

// compactDraws 把連續的 `srand=` 折成一格（讀鍵的迴圈一次會播種幾十萬次）。
func compactDraws(in []string) []string {
	var out []string
	run, last := 0, ""
	flush := func() {
		if run > 0 {
			out = append(out, fmt.Sprintf("srand×%d→%s", run, strings.TrimPrefix(last, "srand=")))
			run = 0
		}
	}
	for _, e := range in {
		if strings.HasPrefix(e, "srand=") {
			run++
			last = e
			continue
		}
		flush()
		out = append(out, e)
	}
	flush()
	return out
}

// runUnitAIDayParity 跑一張盤面，回傳原版各選項定案的次數。
func runUnitAIDayParity(t *testing.T, soldiers, enemies, enemySoldiers, enemyStats, myIntel int) map[int]int {
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
	if myIntel > 0 {
		// 把玩家這一邊的智壓低，電腦的計謀才過得了成功判定
		// （`RND(表) + 目標領隊的智 < 施法者領隊的智`，`docs/re/05` §4.1）。
		me := int(o.Byte(addr(staBase + uint32(at*176+30))))
		for i := 0; i < 350; i++ {
			rec := genBase + uint32(i*30)
			if int(o.Byte(addr(rec+18))) == me && int(o.Byte(addr(rec+19))) == at {
				o.SetByte(addr(rec+9), byte(myIntel))
			}
		}
	}

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
		// seed 是進決策鏈時原版的亂數種子，gap 是這條鏈結束到下一條
		// 鏈開始之間原版擲的骰（呼叫端），不重拍模式要接這些。
		// entry 是進鏈時這支部隊的樣子（複本，重拍那一段跑 DecideBase
		// 會改到 unit），fresh 是第一條鏈另拍的一份完整盤面，給不重拍
		// 模式從頭走。
		seed  uint32
		gap   []string
		entry *battle.Unit
		fresh *battle.Battle
	}
	cloneUnit := func(u *battle.Unit) *battle.Unit {
		if u == nil {
			return nil
		}
		c := *u
		c.Leaders = append([]battle.Leader(nil), u.Leaders...)
		return &c
	}
	var decisions []*decision
	var cur *decision
	noresync := envOr("SAN1_NORESYNC", "") != ""
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
		b := battle.New(battle.Setup{Field: field, Weather: weather, FixedWeather: true, Difficulty: w16(0x30fe)})
		b.Day = d.day
		b.Units = nil
		for a := 0; a < 4; a++ {
			arec := 0x175e + a*22
			b.Gold[toSide[a]] = w16(arec + 6)
			b.Rice[toSide[a]] = w16(arec + 8)
			// 統帥（軍力記錄 offset 0）：投敵判定跳過他。
			b.Commander[toSide[a]] = int(int16(w16(arec)))
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
					Quality: w16(rec + unitAbility),
					Cap:     w16(rec + unitCap),
				}
				for pos := 0; pos < 10; pos++ {
					idx := w16(rec + pos*2)
					if idx == 0xFFFF {
						continue
					}
					g := genBase + uint32(idx*30)
					l := battle.Leader{
						Index: idx, Intel: o.Byte(addr(g + 9)), War: o.Byte(addr(g + 10)),
						Soldiers: int(o.Word(addr(g + 22))), Training: o.Byte(addr(g + 24)),
						Arms: o.Byte(addr(g + 25)), Troop: battle.TroopKind(o.Byte(addr(g + 21))),
						Lord: o.Byte(addr(g+17)) == 0,
					}
					l.Loyalty = int(int8(o.Byte(addr(g + 16)))) // 在野的 0xFF 讀成 −1（cbw）
					if bond := int(o.Word(addr(g + 14))); bond != idx && bond < 350 {
						l.BondAlly = o.Byte(addr(genBase+uint32(bond*30)+18)) == o.Byte(addr(g+18))
					}
					u.Leaders = append(u.Leaders, l)
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
		// 無主或自己勢力的，扣掉對方助軍出兵的那一郡。四個軍力都算，
		// 不重拍模式要用；順便填誰是電腦、人望多少（被擒處置要看）。
		pref := w16(0x1bf8)
		for a := 0; a < 4; a++ {
			faction := w16(0x175e + a*22 + 16)
			if faction == 0xFFFF || faction >= 16 {
				continue
			}
			mrec := base + uint32(faction*72)
			b.Computer[toSide[a]] = o.Word(addr(mrec)) == 2
			b.Renown[toSide[a]] = int(o.Word(addr(mrec + 8)))
			other := 3
			if a >= 2 {
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
				b.Escapes[toSide[a]] = append(b.Escapes[toSide[a]],
					battle.Escape{Prefecture: n, Active: int(o.Byte(addr(staBase + uint32(n*176+22))))})
			}
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

	seedNow := func(o *oracle.Oracle) uint32 {
		ds := o.DSReg()
		return uint32(o.Word(oracle.Addr{Seg: ds, Off: 0xa3ae})) | uint32(o.Word(oracle.Addr{Seg: ds, Off: 0xa3b0}))<<16
	}
	// 鏈外的骰（玩家那支的命令、投敵判定、日結算、天候）記在上一條鏈
	// 上，值一樣從下一次讀到的種子回推。
	gapN, gapAt := 0, ""
	flushGap := func(o *oracle.Oracle) {
		if gapN > 0 && len(decisions) > 0 {
			last := decisions[len(decisions)-1]
			last.gap = append(last.gap, fmt.Sprintf("%d=%d@%s", gapN, int((seedNow(o)>>16)&0x7fff)%gapN, gapAt))
		}
		gapN = 0
	}
	o.OnCall(addr(0x29014), func(o *oracle.Oracle) {
		if dgroup == 0 {
			return
		}
		flushGap(o)
		cur = snapshot(int(int16(o.Arg(0))), int(int16(o.Arg(1))))
		cur.seed = seedNow(o)
		if noresync {
			cur.entry = cloneUnit(cur.unit)
			if len(decisions) == 0 {
				cur.fresh = snapshot(cur.army, cur.team).model
			}
		}
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
	flushRoll := func(o *oracle.Oracle) {
		if pendingN > 0 && cur != nil {
			cur.rolls = append(cur.rolls, int((seedNow(o)>>16)&0x7fff)%pendingN)
			cur.rollNs = append(cur.rollNs, pendingN)
		}
		pendingN = 0
	}
	// `srand()`（`0x5c4:0x2c9e`）：原版讀鍵的迴圈每等一輪就把計數器
	// `es:0x2172` 加一，鍵到了拿它重新播種（`0x10bb0`／`0x11832`／`0x11c2c`）。
	// 玩家每按一個鍵，種子就跳到那個計數值——記成 `srand=值@位址`，
	// 不重拍模式在同一個位置把 remake 的種子也跳過去。
	o.OnCall(oracle.Addr{Seg: 0x5c4, Off: 0x2c9e}, func(o *oracle.Oracle) {
		if dgroup == 0 {
			return
		}
		flushGap(o)
		flushRoll(o)
		at := fmt.Sprintf("srand=%d@%05x", o.Arg(0), o.Caller().Linear())
		if cur != nil {
			cur.callers = append(cur.callers, at)
		} else if len(decisions) > 0 {
			last := decisions[len(decisions)-1]
			last.gap = append(last.gap, at)
		}
	})
	// 對白常式（`0x3273e`）的呼叫端：記成 `msg@位址`，看每一道 `RND(8)`
	// 是誰印的。
	o.OnCall(addr(0x3273e), func(o *oracle.Oracle) {
		if dgroup == 0 {
			return
		}
		at := fmt.Sprintf("msg@%05x", o.Caller().Linear())
		if cur != nil {
			cur.callers = append(cur.callers, at)
		} else if len(decisions) > 0 {
			last := decisions[len(decisions)-1]
			last.gap = append(last.gap, at)
		}
	})
	// 直接叫 `rand()`（`0x5c4:0x2cb0`）而不經 `RND(n)` 的呼叫端：記成
	// `rand@位址`。有這種呼叫，從種子回推的骰值就會錯位。
	viaWrapper := false
	o.OnCall(oracle.Addr{Seg: 0x5c4, Off: 0x2cb0}, func(o *oracle.Oracle) {
		if viaWrapper {
			viaWrapper = false
			return
		}
		if dgroup == 0 {
			return
		}
		at := fmt.Sprintf("rand@%05x", o.Caller().Linear())
		if cur != nil {
			cur.callers = append(cur.callers, at)
		} else if len(decisions) > 0 {
			last := decisions[len(decisions)-1]
			last.gap = append(last.gap, at)
		}
	})
	o.OnCall(addr(0x10b0c), func(o *oracle.Oracle) {
		if n := int(int16(o.Arg(0))); n > 0 {
			viaWrapper = true
		}
		if cur == nil {
			flushGap(o)
			if n := int(int16(o.Arg(0))); n > 0 && dgroup != 0 {
				gapN, gapAt = n, fmt.Sprintf("%05x", o.Caller().Linear())
			}
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
	// 玩家那支是在第一天的電腦部隊都動完之後才被搬到守軍旁邊的
	// （紮寨那幾步已經讓守方走完第一天）；不重拍模式要在同一個時點
	// 把 remake 的那支也搬過去。
	meRec, _ := placeNextToDefender(t, o, dgroup)
	placedAt := battle.NoHex
	if meRec >= 0 {
		placedAt = battle.FromOffset(w16(meRec+unitCol), w16(meRec+unitRow))
	}

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
			t.Logf("✓ %s；RND 的呼叫端 %v", line, compactDraws(d.callers))
		} else {
			t.Errorf("✗ %s；RND 的呼叫端 %v；鄰格 %v；本隊兵 %d", line, compactDraws(d.callers), d.around, d.unit.Soldiers())
			bad++
		}
	}
	t.Logf("決策 %d 次，原版各選項定案：%v，不同 %d 次", len(decisions), byOpt, bad)
	if len(decisions) < 10 {
		t.Errorf("只比到 %d 次決策——樣本太少", len(decisions))
	}
	if !noresync || len(decisions) == 0 {
		return byOpt
	}

	// 不重拍模式（Issue #24）：只拿第一條鏈的盤面，之後 remake 自己走，
	// 骰用 MSC 的 LCG 從原版當時的種子接。每一條鏈進來時比三件事：
	// 種子（兩邊擲的次數一樣多）、這支部隊的狀態（兵、將領數、位置、
	// 移動力、箭、陷阱）、決策（選項／目標／落點）。種子岔開就把 remake
	// 的種子接回原版的，讓後面的鏈還比得下去，但算一次「岔開」。
	first := decisions[0]
	model := first.fresh
	seed := first.seed
	// 玩家那一邊：原版每天在電腦的部隊之後輪到它們，測試每一支都送「0」
	// 休息；休息印一句對白，回合結束一樣判投敵、回填移動力。投敵或招降
	// 的人進了空槽位會多出一支（後軍），所以照行動順序逐支來。
	playerUnits := func() []*battle.Unit {
		var out []*battle.Unit
		for _, f := range battle.ActionOrder() {
			for _, u := range model.Units {
				if u.Side == battle.MainAttacker && u.Formation == f && u.Alive() {
					out = append(out, u)
				}
			}
		}
		return out
	}
	// 中途生出來的部隊原版是問玩家紮在哪（`0x2731a`）；這裡拿下一條鏈
	// 拍到的位置當那個答案。
	placeNew := func(snap *battle.Battle) {
		for _, u := range model.Units {
			if !u.Unplaced {
				continue
			}
			for _, v := range snap.Units {
				if v.Side == u.Side && v.Formation == u.Formation {
					u.At = v.At
					u.Unplaced = false
					t.Logf("中途生出的 %s%s 照原版擺在 %v", u.Side, u.Formation, u.At)
				}
			}
		}
	}
	// 原版讀鍵與等待的迴圈會不斷 `srand(計數器)`（見上面的 hook），
	// 玩家每按一個鍵種子就跳一次。remake 擲骰時照原版同一段的紀錄
	// （鏈內 callers ＋ 鏈外 gap）走：擲第 k 次之前，先把排在原版第 k
	// 次擲骰前面的 `srand=` 套上去。骰的**順序與次數**還是 remake 自己的，
	// 種子相不相同由下一條鏈進來時的比對決定。
	var stream []string
	cursor := 0
	isDraw := func(e string) bool { return e != "" && e[0] >= '0' && e[0] <= '9' }
	applySrands := func() {
		for cursor < len(stream) && !isDraw(stream[cursor]) {
			if strings.HasPrefix(stream[cursor], "srand=") {
				var v uint32
				fmt.Sscanf(stream[cursor], "srand=%d@", &v)
				seed = v
			}
			cursor++
		}
	}
	var asked []string
	model.UseRoll(func(n int) int {
		applySrands()
		if cursor < len(stream) {
			cursor++
		}
		var out int
		seed, out = game.MSCRand(seed)
		asked = append(asked, fmt.Sprintf("%d=%d", n, out%n))
		return out % n
	})
	findUnit := func(b *battle.Battle, army, team int) *battle.Unit {
		for _, u := range b.Units {
			if u.Side == toSide[army] && u.Formation == teamForm[team] {
				return u
			}
		}
		return nil
	}
	unitState := func(u *battle.Unit) string {
		if u == nil {
			return "（沒有這支）"
		}
		x, y := battle.ToOffset(u.At)
		return fmt.Sprintf("兵 %d 將 %d 落點 (%d,%d) 移動 %d 箭 %d 陷阱 %d 綜合 %d 在場 %v",
			u.Soldiers(), u.LeaderCount(), x, y, u.Move, u.Arrows, u.Trapped, u.Quality, u.Alive())
	}
	layout := func(b *battle.Battle) string {
		var parts []string
		for _, u := range b.Units {
			x, y := battle.ToOffset(u.At)
			var idx []int
			for i := range u.Leaders {
				if u.Leaders[i].InUnit() {
					idx = append(idx, u.Leaders[i].Index)
				}
			}
			parts = append(parts, fmt.Sprintf("%s%s@(%d,%d)兵%d將%v移%d", u.Side, u.Formation, x, y, u.Soldiers(), idx, u.Move))
		}
		return fmt.Sprint(parts)
	}
	t.Logf("不重拍：起點 第 %d 天 種子 %08x 統帥 %v 天候 %v；%s", first.day, seed, model.Commander, model.Weather, layout(model))
	diverged, stateBad, decideBad := 0, 0, 0
	for i, d := range decisions {
		// 上一段尾巴的 `srand=`（等鍵的迴圈在下一條鏈之前又播了種）先套上。
		applySrands()
		stream, cursor = append(append([]string(nil), d.callers...), d.gap...), 0
		tag := fmt.Sprintf("第 %2d 天 %s 隊伍 %d", d.day, sideName(d.army), d.team)
		if seed != d.seed {
			steps := lcgStepsBetween(seed, d.seed, 64)
			t.Errorf("✗ %s：進鏈時骰岔開——remake 種子 %08x、原版 %08x（原版比 remake 多 %d 步）；上一條鏈之後原版鏈外的骰 %v，remake 問了 %v",
				tag, seed, d.seed, steps, compactDraws(decisions[i-1].gap), asked)
			diverged++
			seed = d.seed
		}
		asked = nil
		placeNew(d.model)
		u := findUnit(model, d.army, d.team)
		if u == nil || d.entry == nil {
			t.Errorf("✗ %s：remake 找不到這支部隊（原版 %s）", tag, unitState(d.entry))
			stateBad++
			continue
		}
		u.RefreshQuality()
		if got, want := unitState(u), unitState(d.entry); got != want {
			t.Errorf("✗ %s：進鏈時部隊狀態不同——remake %s；原版 %s", tag, got, want)
			stateBad++
		}
		if d.option == 7 {
			// 對戰子畫面（`0x2deb0`）還沒對齊：remake 的「對戰」是叫陣
			// 單挑，骰序不同（`docs/mechanics/40` §8）。不重拍走到這裡為止。
			t.Logf("%s：原版選了對戰（選項 7），子畫面的骰序還沒對齊，不重拍比到這一條為止（%d／%d 條）", tag, i, len(decisions))
			break
		}
		got := model.DecideBase(u)
		model.EndTurn(u)
		if i+1 < len(decisions) && decisions[i+1].day != d.day {
			// 換日：原版在最後一支電腦部隊之後輪到玩家那支（休息、
			// 判投敵、回填），再做日結算與天候；remake 在這裡做同一串。
			next := decisions[i+1]
			for day := d.day; day < next.day; day++ {
				if day == first.day && placedAt != battle.NoHex {
					if p := findUnit(model, 2, 0); p != nil {
						p.At = placedAt
					}
				}
				placeNew(next.model)
				for _, p := range playerUnits() {
					p.RefreshQuality()
					if p.Trapped > 0 {
						model.SkipTrappedTurn(p)
						continue
					}
					_ = model.Rest(p)
					model.EndTurn(p)
				}
				model.EndDay()
			}
			applySrands()
			if model.Weather != next.model.Weather {
				t.Errorf("第 %d 天：remake 擲出的天候是 %v，原版 %v", next.day, model.Weather, next.model.Weather)
				model.Weather = next.model.Weather
			}
		}
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
		x, y := battle.ToOffset(u.At)
		// 退了兵的部隊原版會走到出口才消失，記錄裡留的是那一格；remake
		// 不走那段路，落點不比。
		if d.option != 2 && (x != d.col || y != d.row) {
			ok = false
		}
		want += fmt.Sprintf(" 落點 (%d,%d)", d.col, d.row)
		have += fmt.Sprintf(" 落點 (%d,%d)", x, y)
		if ok {
			t.Logf("✓ %s：%s；種子 %08x；鏈內骰 %v；鏈外骰 %v；remake 問了 %v", tag, want, d.seed, compactDraws(d.callers), compactDraws(d.gap), asked)
		} else {
			t.Errorf("✗ %s：原版 %s；remake %s（%v）；鏈內骰 %v；鏈外骰 %v；remake 鏈內問了 %v；remake 盤面 %s；原版盤面 %s",
				tag, want, have, got.Why, compactDraws(d.callers), compactDraws(d.gap), asked, layout(model), layout(d.model))
			decideBad++
		}
		asked = nil
	}
	t.Logf("不重拍：%d 條鏈，骰岔開 %d 次、狀態不同 %d 次、決策不同 %d 次", len(decisions), diverged, stateBad, decideBad)
	return byOpt
}
