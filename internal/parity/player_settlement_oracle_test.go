//go:build oracle

package parity

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

type settlementUnit struct {
	army, team, col, row int
	ids                  []int
}

type settlementOriginal struct {
	before, after                      []byte
	seed                               uint32
	day, winner, fallen                int
	chief, gold, rice, origin, faction [4]int
	units                              []settlementUnit
}

// TestZZPlayerSettlementTables 在戰後結算的同一切點比較整張州郡與人物表。
// 諸侯表另外列出診斷：人望與君主退場在此切點前已變動，不能用這份
// 存活部隊快照重建諸侯表的完整同狀態轉移。
func TestZZPlayerSettlementTables(t *testing.T) {
	runPlayerSettlementTables(t, baseDayRig(), dayBoard{soldiers: 3000, enemies: 2, enemySoldiers: 1500, difficulty: 5}, 0x23d03, 0x23d1d)
}

func TestZZPlayerSettlementTablesPlus(t *testing.T) {
	runPlayerSettlementTables(t, plusDayRig(), dayBoard{soldiers: 3000, enemies: 2, enemySoldiers: 1500, difficulty: 5, clearTarget: true}, 0x217d9, 0x217f3)
}

func runPlayerSettlementTables(t *testing.T, rig dayRig, board dayBoard, settlementEntry, settlementReturn uint32) {
	root := rig.root(t)
	o, err := oracle.Load(root+"/"+rig.exe, root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	base := rig.boot(t, o, board)
	from, to := stageABattleWith(t, o, base, board.soldiers, board.enemies, board.enemySoldiers, board.stats)
	total := state.MasterTableSize + state.PrefectureTableSize + state.GeneralTableSize
	var ds uint16
	o.OnCall(addr(rig.enter), func(oo *oracle.Oracle) {
		if ds == 0 {
			ds = oo.DSReg()
		}
	})
	cmdReads := 0
	o.OnCall(addr(rig.cmdRead), func(*oracle.Oracle) { cmdReads++ })
	shot := settlementOriginal{}
	preCalls, postCalls := 0, 0
	o.OnCall(addr(settlementEntry), func(oo *oracle.Oracle) {
		preCalls++
		if preCalls != 1 {
			return
		}
		work := oo.Word(oracle.Addr{Seg: ds, Off: rig.workSegPtr})
		w := func(off int) int { return int(oo.Word(oracle.Addr{Seg: work, Off: uint16(off)})) }
		shot.before = oo.Bytes(addr(base), total)
		shot.seed = uint32(oo.Word(oracle.Addr{Seg: ds, Off: rig.seedLo})) |
			uint32(oo.Word(oracle.Addr{Seg: ds, Off: rig.seedHi}))<<16
		// 加強版移了天數欄，勝負欄仍在 0x20dc（docs/re/05 §8.2）。
		shot.day, shot.winner, shot.fallen = w(rig.day), w(0x20dc), -1
		if rig.edition == state.EditionBase {
			shot.fallen = w(0x20f0)
		}
		for army := 0; army < 4; army++ {
			f := 0x175e + army*22
			shot.chief[army], shot.gold[army], shot.rice[army] = w(f), w(f+6), w(f+8)
			shot.faction[army], shot.origin[army] = w(f+16), w(f+18)
			for team := 0; team < battleTeams; team++ {
				r := rig.unitBase + (army*battleUnitPer+team)*battleUnitSize
				if w(r+unitLeaders) == 0 {
					continue
				}
				u := settlementUnit{army: army, team: team, col: w(r + unitCol), row: w(r + unitRow)}
				for pos := 0; pos < 10; pos++ {
					if id := w(r + pos*2); id != 0xffff {
						u.ids = append(u.ids, id)
					}
				}
				shot.units = append(shot.units, u)
			}
		}
	})
	o.OnCall(addr(settlementReturn), func(oo *oracle.Oracle) {
		postCalls++
		if postCalls == 1 {
			shot.after = oo.Bytes(addr(base), total)
		}
	})
	driveIntoBattleGap(t, o, from, to, rig.keyGap)
	if ds == 0 {
		t.Fatal("主戰場入口未命中")
	}
	const fixedSeed uint32 = 0x13579bdf
	o.SetWord(oracle.Addr{Seg: ds, Off: rig.seedLo}, uint16(fixedSeed&0xffff))
	o.SetWord(oracle.Addr{Seg: ds, Off: rig.seedHi}, uint16(fixedSeed>>16))
	work := o.Word(oracle.Addr{Seg: ds, Off: rig.workSegPtr})
	w := func(off int) int { return int(o.Word(oracle.Addr{Seg: work, Off: uint16(off)})) }
	set := func(off, v int) { o.SetWord(oracle.Addr{Seg: work, Off: uint16(off)}, uint16(v)) }
	win := func() int { return w(0x20dc) }
	for step := 1; step <= 12 && cmdReads == 0; step++ {
		o.Drain()
		o.PressScan("0")
		if err := o.Run(60_000_000); err != nil {
			t.Fatalf("紮寨第 %d 步：%v", step, err)
		}
	}
	o.Drain()
	o.PressScan("Y")
	if err := o.Run(60_000_000); err != nil {
		t.Fatal(err)
	}
	me, foe := -1, -1
	for army := 0; army < battleArmies; army++ {
		for team := 0; team < battleTeams; team++ {
			r := rig.unitBase + (army*battleUnitPer+team)*battleUnitSize
			if w(r) == 0xffff || w(r+unitLeaders) <= 0 {
				continue
			}
			if army >= 2 && me < 0 {
				me = r
			}
			if army < 2 {
				set(r+unitSoldiers, 120)
				if foe < 0 {
					foe = r
				}
			}
		}
	}
	if me < 0 || foe < 0 {
		t.Fatal("攻守部隊不全")
	}
	fc, fr := w(foe+unitCol), w(foe+unitRow)
	occ := func(col, row int) oracle.Addr { return oracle.Addr{Seg: work, Off: uint16(rig.occ + (row*12+col)*2)} }
	placed := false
	for dir := 0; dir < 6; dir++ {
		i := uint16(((fc%2)*6 + dir) * 2)
		col := fc + int(int16(o.Word(oracle.Addr{Seg: ds, Off: rig.colTable + i})))
		row := fr + int(int16(o.Word(oracle.Addr{Seg: ds, Off: rig.rowTable + i})))
		if col < 0 || col >= 12 || row < 0 || row >= 10 || o.Word(occ(col, row)) != 0xffff {
			continue
		}
		o.SetWord(occ(w(me+unitCol), w(me+unitRow)), 0xffff)
		set(me+unitCol, col)
		set(me+unitRow, row)
		o.SetWord(occ(col, row), 20)
		placed = true
		break
	}
	if !placed {
		t.Fatal("攻軍無法貼近守軍")
	}
	for day := 1; day <= 35 && win() == 0xffff; day++ {
		before := w(rig.day)
		for _, key := range []string{"2", "Y"} {
			o.Drain()
			o.PressScan(key)
			if err := o.Run(150_000_000); err != nil {
				t.Fatalf("第 %d 日鍵 %q：%v", day, key, err)
			}
		}
		if w(rig.day) == before && win() == 0xffff {
			for _, key := range []string{"0", "Y"} {
				o.Drain()
				o.PressScan(key)
				if err := o.Run(120_000_000); err != nil {
					t.Fatalf("第 %d 日補休息：%v", day, err)
				}
			}
		}
	}
	for i := 0; i < 24 && postCalls == 0; i++ {
		o.Drain()
		o.PressScan("\r")
		if err := o.Run(150_000_000); err != nil {
			t.Fatalf("戰後對白 %d：%v", i, err)
		}
	}
	if preCalls != 1 || postCalls != 1 || len(shot.before) != total || len(shot.after) != total {
		t.Fatalf("結算入口／返回未各命中一次：%d／%d，表長 %d／%d", preCalls, postCalls, len(shot.before), len(shot.after))
	}
	if shot.winner > 3 || len(shot.units) == 0 {
		t.Fatalf("戰場快照無效：勝方 %d、部隊 %d", shot.winner, len(shot.units))
	}
	t.Logf("原版結算入口：SHA-256 %x、seed %#08x、日 %d、勝方 %d、部隊 %d", sha256.Sum256(shot.before), shot.seed, shot.day, shot.winner, len(shot.units))
	if shot.fallen >= 0 {
		t.Logf("原版結算入口退場旗標 %#x", shot.fallen)
	}
	t.Logf("四軍統帥 %v，勢力 %v，來源郡 %v，金 %v，米 %v", shot.chief, shot.faction, shot.origin, shot.gold, shot.rice)
	for _, u := range shot.units {
		t.Logf("結算入口軍力 %d 隊伍 %d 將領 %v", u.army, u.team, u.ids)
	}
	for _, id := range []int{shot.faction[0], shot.faction[2]} {
		if id < 0 || id >= 16 {
			continue
		}
		p := id * 72
		t.Logf("勢力 %d 入口操縱方 %x、人望 %d、君主 %d；戰後操縱方 %x、人望 %d",
			id, shot.before[p:p+2], int(shot.before[p+8])|int(shot.before[p+9])<<8,
			int(shot.before[p+2])|int(shot.before[p+3])<<8,
			shot.after[p:p+2], int(shot.after[p+8])|int(shot.after[p+9])<<8)
	}
	sc, err := state.DecodeTables(state.Slot("001"), shot.before[:state.MasterTableSize],
		shot.before[state.MasterTableSize:state.MasterTableSize+state.PrefectureTableSize],
		shot.before[state.MasterTableSize+state.PrefectureTableSize:])
	if err != nil {
		t.Fatal(err)
	}
	players := sc.Players()
	if len(players) == 0 {
		t.Fatal("結算入口沒有玩家勢力")
	}
	// 這是執行期表格而非新劇本。New 會再做一次難度對 AI 等級的
	// 改寫（加強版因此多出 12 個諸侯表差異），應由 Continue 接續。
	date := game.Date{Year: 197, Month: 9}
	if rig.edition == state.EditionPlus {
		date = game.Date{Year: 189, Month: 1}
	}
	g, err := game.Continue(sc, state.FactionID(players[0]), board.difficulty, rig.edition, date)
	if err != nil {
		t.Fatal(err)
	}
	gm, gs, gg, err := g.Tables()
	if err != nil {
		t.Fatal(err)
	}
	initial := append(append(gm, gs...), gg...)
	compareSettlementTables(t, "結算入口", shot.before, initial, true)
	b := battle.New(battle.Setup{Field: g.Field(to), FixedWeather: true, FromGate: from})
	b.Day = shot.day
	b.Over = true
	b.AttackerWon = shot.winner >= 2
	toSide := [...]battle.Side{battle.MainDefender, battle.AidDefender, battle.MainAttacker, battle.AidAttacker}
	var snap game.SettlementSnapshot
	snap.Battle, snap.From, snap.To = b, from, to
	for army, side := range toSide {
		b.Commander[side] = shot.chief[army]
		b.Gold[side], b.Rice[side] = shot.gold[army], shot.rice[army]
		snap.Factions[side] = state.FactionID(shot.faction[army])
		if shot.origin[army] > 0 && shot.origin[army] <= 42 {
			b.Origin[side] = battle.Escape{Prefecture: shot.origin[army], Active: g.ActiveGenerals(shot.origin[army])}
		}
		if army == 1 {
			snap.Aid.Defender = shot.origin[army]
		}
		if army == 3 {
			snap.Aid.Attacker = shot.origin[army]
		}
		if shot.faction[army] >= 0 && shot.faction[army] < 16 {
			b.Computer[side] = !g.IsHuman(state.FactionID(shot.faction[army]))
		}
	}
	for _, u := range shot.units {
		side := toSide[u.army]
		model := &battle.Unit{Side: side, Formation: battle.Formation(u.team), At: battle.FromOffset(u.col, u.row)}
		for _, id := range u.ids {
			x := g.General(id)
			if x == nil {
				t.Fatalf("原版部隊指向不存在的人物 %d", id)
			}
			model.Leaders = append(model.Leaders, battle.Leader{Index: id, Soldiers: x.Soldiers, Stamina: x.Stamina,
				War: x.War, Intel: x.Intel, Charm: x.Charm, Training: x.Training, Arms: x.Arms,
				Lord: x.Status == state.StatusLord, Loyalty: int(int8(x.Loyalty))})
			snap.Armies[u.army] = append(snap.Armies[u.army], id)
		}
		b.Units = append(b.Units, model)
	}
	g.SeedRand(shot.seed)
	if r := g.ReplayPlayerSettlement(snap); r == nil {
		t.Fatal("remake 結算沒有結果")
	}
	gm, gs, gg, err = g.Tables()
	if err != nil {
		t.Fatal(err)
	}
	after := append(append(gm, gs...), gg...)
	compareSettlementTables(t, "戰後", shot.after, after, false)
	t.Logf("戰後整張州郡表 %d 位元組與人物表 %d 位元組逐格相同；原版三表 SHA-256 %x",
		state.PrefectureTableSize, state.GeneralTableSize, sha256.Sum256(shot.after))
}

func compareSettlementTables(t *testing.T, stage string, want, got []byte, strictMaster bool) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s：表長 %d／%d", stage, len(want), len(got))
	}
	if bytes.Equal(want, got) {
		return
	}
	names := []struct {
		name               string
		start, end, record int
	}{
		{"諸侯", 0, state.MasterTableSize, 72},
		{"州郡", state.MasterTableSize, state.MasterTableSize + state.PrefectureTableSize, 176},
		{"人物", state.MasterTableSize + state.PrefectureTableSize, len(want), 30},
	}
	for _, table := range names {
		count := 0
		for i := table.start; i < table.end; i++ {
			if want[i] == got[i] {
				continue
			}
			count++
			if count <= 20 {
				t.Logf("%s %s 筆 %d 偏移 %d：原版 %#02x／remake %#02x", stage, table.name, (i-table.start)/table.record, (i-table.start)%table.record, want[i], got[i])
			}
		}
		if count > 0 {
			if table.name == "諸侯" && !strictMaster {
				t.Logf("%s 諸侯表差 %d 個位元組；此切點不驗諸侯表的人望與已退場君主", stage, count)
			} else {
				t.Errorf("%s %s 全表差 %d 個位元組", stage, table.name, count)
			}
		}
	}
}
