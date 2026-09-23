//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 玩家親征，把一場戰役打到分出勝負，比勝負判定（`docs/re/05` §8、§8.1）。
//
// `TestBattleOutcomeMatchesTheOriginal` 的判準早就寫好，但它走的是電腦
// 對電腦那條路——那條**不進戰術層**，日循環一次都不跑，所以它一直 skip。
// 這一支補上缺的那條路徑：玩家親征。
//
// 勝負寫在 `es:[0x20dc]`（段取自 `ds:[0xa89e]`）：`0` 主守軍、`1` 助守軍、
// `2` 主攻軍、`3` 助攻軍，`0xFFFF` ＝ 還沒分出勝負。判定有兩支：
// `0x24f8c` 每天判一次的統帥條件、`0x250d4` 三十天期滿。
//
// remake 這一邊的同一條規則在 `battle.checkOver`：**守方統帥那一條蓋過
// 攻方那一條**，兩邊統帥都不在時判攻方勝。這裡不重跑一場，而是**拿原版
// 自己分出勝負那一刻的盤面**去套 remake 的規則，比兩邊算出來的勝方。
func TestBattleFinishesWithPlayer(t *testing.T) {
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
	chiefRuns, dayRuns := 0, 0
	// **盤面要在判定發生的那一刻取。** 戰役結束之後軍團與部隊記錄會被
	// 拆掉，事後再讀會拿到一整排 `0xFFFF`——量到過一次，比對因此拿垃圾
	// 去算。這裡在統帥條件每次進去時把四個軍團的統帥與四支第一部隊的
	// 排頭記下來，最後一筆就是判定當下的盤面。
	var lastChief, lastHead [4]int
	var haveBoard bool
	snapBoard := func() {}
	o.OnCall(addr(0x24f8c), func(*oracle.Oracle) { chiefRuns++; snapBoard() })
	o.OnCall(addr(0x250d4), func(*oracle.Oracle) { dayRuns++; snapBoard() })

	const fixedSeed uint32 = 0x13579bdf
	driveIntoBattle(t, o, at, to)
	if dgroup == 0 {
		t.Fatal("沒有進到主戰場")
	}
	o.SetWord(oracle.Addr{Seg: dgroup, Off: 0xa3ae}, uint16(fixedSeed&0xffff))
	o.SetWord(oracle.Addr{Seg: dgroup, Off: 0xa3b0}, uint16(fixedSeed>>16))
	t.Logf("原版進入主戰場後固定 MSC LCG seed %#x（DS:0xa3ae／0xa3b0）", fixedSeed)
	work := o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg})
	forceSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa8b6})
	wonSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa89e})
	occSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9ca})
	colSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9c8})
	rowSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9c6})
	w16 := func(off int) int {
		return int(o.Word(oracle.Addr{Seg: work, Off: uint16(off)}))
	}
	setw := func(off, v int) {
		o.SetWord(oracle.Addr{Seg: work, Off: uint16(off)}, uint16(v))
	}
	won := func() int {
		return int(o.Word(oracle.Addr{Seg: wonSeg, Off: 0x20dc}))
	}
	occ := func(c, r int) oracle.Addr {
		return oracle.Addr{Seg: occSeg, Off: uint16(0x2532 + (r*12+c)*2)}
	}
	recOf := func(army, team int) int {
		return battleUnitBase + (army*battleUnitPer+team)*battleUnitSize
	}
	day := func() int { return w16(0x2100) }
	// 正常玩家戰役也必須實際跑進收尾常式；零次不能用來證明「沒有」。
	var settleCalls [3]int
	var beforePlace, afterPlace []byte
	fallenLord := -1
	winnerFaction := -1
	var winnerTreasury, loserTreasury []byte
	postSpoils := 0
	spoilRollCalls := 0
	province := addr(base + uint32(state.MasterTableSize+to*state.PrefectureRecordSize))
	treasury := func(faction int) oracle.Addr { return addr(base + uint32(faction*72+14)) }
	o.OnCall(addr(0x25652), func(o *oracle.Oracle) {
		settleCalls[0]++
		winnerFaction = int(o.Word(oracle.Addr{Seg: forceSeg, Off: uint16(0x176e + won()*22)}))
	})
	o.OnCall(addr(0x266a6), func(o *oracle.Oracle) {
		settleCalls[1]++
		beforePlace = o.Bytes(province, state.PrefectureRecordSize)
	})
	o.OnCall(addr(0x26c08), func(o *oracle.Oracle) {
		settleCalls[2]++
		afterPlace = o.Bytes(province, state.PrefectureRecordSize)
		fallenLord = w16(0x20f0)
		if fallenLord == 0xFFFF {
			return
		}
		if winnerFaction < 0 || winnerFaction >= 16 || fallenLord < 0 || fallenLord >= 16 {
			t.Fatalf("戰後讀到非法勢力：勝方 %d、敗方 %d", winnerFaction, fallenLord)
		}
	})
	// 0x14968 繼承可能清掉退場勢力的玉璽。改在繼承返回後、
	// 0x26c31 第一擲之前注入，才是單獨測 0x26c08 的分贓。
	o.OnCall(addr(0x26c31), func(o *oracle.Oracle) {
		spoilRollCalls++
		o.SetBytes(treasury(fallenLord), []byte{1, 10, 9, 8, 7})
		o.SetBytes(treasury(winnerFaction), []byte{2, 1, 1, 1, 1})
	})
	o.OnCall(addr(0x23d1d), func(o *oracle.Oracle) {
		postSpoils++
		if winnerFaction >= 0 {
			winnerTreasury = o.Bytes(treasury(winnerFaction), 5)
			loserTreasury = o.Bytes(treasury(fallenLord), 5)
		}
	})

	snapBoard = func() {
		for i := 0; i < 4; i++ {
			lastChief[i] = w16(0x175e + i*22)
			lastHead[i] = w16(battleUnitBase + (i*battleUnitPer)*battleUnitSize)
		}
		haveBoard = true
	}

	// **不要對初值下斷言。** 進到主戰場那一刻 `0x20dc` 讀到的不是
	// `0xFFFF`——與其猜段取錯了還是時點不對，直接掛寫入監看看誰寫它
	// （全碼段有十一處寫這一格，`docs/re/05` §8）。
	t.Logf("進場時勝方欄位 %#x（段 %#06x）、天數 %d", won(), wonSeg, day())
	wonWr := o.WatchWritesAt(uint32(wonSeg)<<4+0x20dc, uint32(wonSeg)<<4+0x20dd)

	// 紮寨 ＋ 第 1 天的確認。
	for step := 1; step <= 12 && cmdReads == 0; step++ {
		o.Drain()
		o.PressScan("0")
		if err := o.Run(60_000_000); err != nil {
			t.Fatalf("紮寨第 %d 步停止：%v", step, err)
		}
	}
	o.Drain()
	o.PressScan("Y")
	if err := o.Run(60_000_000); err != nil {
		t.Fatalf("確認第 1 天停止：%v", err)
	}

	// **盤面自己擺**：守軍的兵壓低、玩家貼上去，戰役才會在幾天內結束
	// （`CLAUDE.md`：對拍要直接設定記憶體，不靠 RND 慢慢磨）。
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
			if army < 2 {
				setw(rec+unitSoldiers, 120) // 守軍壓到很薄
				if foe < 0 {
					foe = rec
				}
			}
		}
	}
	if me < 0 || foe < 0 {
		t.Fatal("盤面上湊不出攻守各一支")
	}
	fc, fr := w16(foe+unitCol), w16(foe+unitRow)
	for dir := 0; dir < 6; dir++ {
		i := uint16(((fc%2)*6 + dir) * 2)
		dc := int(int16(o.Word(oracle.Addr{Seg: colSeg, Off: 0x7c6a + i})))
		dr := int(int16(o.Word(oracle.Addr{Seg: rowSeg, Off: 0x7c82 + i})))
		c, r := fc+dc, fr+dr
		if c < 0 || c >= 12 || r < 0 || r >= 10 || o.Word(occ(c, r)) != 0xFFFF {
			continue
		}
		o.SetWord(occ(w16(me+unitCol), w16(me+unitRow)), 0xFFFF)
		setw(me+unitCol, c)
		setw(me+unitRow, r)
		o.SetWord(occ(c, r), 20)
		t.Logf("守軍的兵全部壓到 120；玩家貼到 (%d,%d)，守軍在 (%d,%d)", c, r, fc, fr)
		break
	}

	// 每天「對戰 → 確認」，打到分出勝負為止。
	days := 0
	for d := 1; d <= 35 && won() == 0xFFFF; d++ {
		before := day()
		for _, k := range []string{"2", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(150_000_000); err != nil {
				t.Fatalf("第 %d 天送 %q 停止：%v", d, k, err)
			}
		}
		days++
		if day() == before && won() == 0xFFFF {
			// 天數沒推進表示那一鍵沒被當成命令，補一個休息把這一天用掉。
			for _, k := range []string{"0", "Y"} {
				o.Drain()
				o.PressScan(k)
				if err := o.Run(120_000_000); err != nil {
					t.Fatalf("第 %d 天補休息停止：%v", d, err)
				}
			}
		}
	}
	// 勝負欄位在戰後對白前就寫好；繼承與分贓還會等玩家按鍵。
	// 以呼叫後的 0x23d1d 為終點，不能在 won() != 0xffff 時提早讀庫存。
	for i := 0; i < 24 && postSpoils == 0; i++ {
		o.Drain()
		o.PressScan("\r")
		if err := o.Run(150_000_000); err != nil {
			t.Fatalf("戰後對白第 %d 步：%v", i, err)
		}
	}
	o.StopWatchingWrites()
	t.Logf("玩家戰後：勝方援軍回郡 %d、戰場郡安置 %d、君主寶物收尾 %d；退場君主旗標 %d",
		settleCalls[0], settleCalls[1], settleCalls[2], fallenLord)
	if len(beforePlace) > 30 && len(afterPlace) > 30 {
		t.Logf("戰場郡 %d 收尾前後：金 %d→%d、米 %d→%d、民忠 %d→%d、地力 %d→%d、洪水 %d→%d、物價 %d→%d、所屬 %d→%d",
			to, int(beforePlace[18])|int(beforePlace[19])<<8, int(afterPlace[18])|int(afterPlace[19])<<8,
			int(beforePlace[20])|int(beforePlace[21])<<8, int(afterPlace[20])|int(afterPlace[21])<<8,
			beforePlace[26], afterPlace[26], beforePlace[27], afterPlace[27],
			beforePlace[28], afterPlace[28], beforePlace[29], afterPlace[29],
			beforePlace[30], afterPlace[30])
	}
	if settleCalls[1] == 0 || settleCalls[2] == 0 {
		t.Fatalf("玩家戰後安置或君主寶物收尾沒有實際命中：%v", settleCalls)
	}
	if fallenLord == 0xFFFF || winnerFaction < 0 || spoilRollCalls == 0 || postSpoils == 0 {
		t.Fatalf("這份固定盤面沒有完成君主退場分贓：旗標 %d、勝方 %d、分贓擲骰 %d、返回 %d", fallenLord, winnerFaction, spoilRollCalls, postSpoils)
	}
	if fallenLord != 0xFFFF {
		if len(winnerTreasury) != 5 || len(loserTreasury) != 5 {
			t.Fatal("結算後無法讀取兩側的寶物")
		}
		t.Logf("退場勢力 %d、勝方勢力 %d；五格寶物：勝方 %v，敗方 %v",
			fallenLord, winnerFaction, winnerTreasury, loserTreasury)
		if winnerTreasury[0] != 2 || loserTreasury[0] != 1 {
			t.Fatal("原版玩家戰役的分贓改動了玉璽")
		}
		total := 0
		for i := 1; i < 5; i++ {
			total += int(winnerTreasury[i]) + int(loserTreasury[i])
		}
		if total != 40 { // 起始總數 4+10+9+8+7，再由 RND(4) 選一格 +2
			t.Errorf("四件寶物總數 %d，原版應為 40", total)
		}
	}
	t.Logf("打了 %d 輪，天數 %d；統帥條件跑 %d 次、三十天判定跑 %d 次；勝方欄位 %#x",
		days, day(), chiefRuns, dayRuns, won())
	seq := ""
	for i, w := range *wonWr {
		if i >= 16 {
			seq += " …"
			break
		}
		seq += fmt.Sprintf(" [%#06x:%#06x %d→%d]", w.IP.Seg, w.IP.Off, w.Old, w.New)
	}
	t.Logf("勝方欄位被寫 %d 次：%s", len(*wonWr), seq)

	if chiefRuns == 0 && dayRuns == 0 {
		t.Fatal("兩支判定都沒跑——日循環沒走起來，這條路徑還是沒進戰術層")
	}
	v := won()
	if len(*wonWr) == 0 {
		t.Skipf("打了 %d 輪還沒分出勝負（天數 %d）——統帥條件跑了 %d 次，"+
			"判定有在跑，只是這一場還沒結束", days, day(), chiefRuns)
	}

	// ── remake 的規則套在原版分出勝負那一刻的盤面上 ──────────
	//
	// 統帥條件（§8.1）：一方的（主＋助）統帥**都不在其第一支部隊的排頭**
	// 就算失去統帥。統帥在軍團記錄 offset 0，排頭是部隊記錄 (軍力,0) 的
	// 將領槽 0。
	if !haveBoard {
		t.Fatal("判定一次都沒跑到，取不到當下的盤面")
	}
	commanderAlive := func(army int) bool {
		chief := lastChief[army]
		if chief == 0xFFFF {
			return false
		}
		head := lastHead[army]
		return head != 0xFFFF && head == chief
	}
	defChief := commanderAlive(0) || commanderAlive(1)
	atkChief := commanderAlive(2) || commanderAlive(3)
	want := -1
	switch {
	case !defChief:
		want = 2 // 守方統帥不在 → 攻方勝（這一條蓋過下面那一條）
	case !atkChief:
		want = 0
	}
	t.Logf("分出勝負那一刻：守方統帥在不在 %v／攻方 %v；"+
		"四個軍團的統帥 %d %d %d %d，四支第一部隊的排頭 %d %d %d %d",
		defChief, atkChief,
		lastChief[0], lastChief[1], lastChief[2], lastChief[3],
		lastHead[0], lastHead[1], lastHead[2], lastHead[3])
	if want < 0 {
		t.Logf("兩邊的統帥都還在，所以原版判的 %d 不是統帥條件"+
			"——那就是三十天期滿（跑了 %d 次）", v, dayRuns)
		if dayRuns == 0 {
			t.Errorf("兩邊統帥都在、三十天判定也沒跑，勝方卻是 %d", v)
		}
		return
	}
	if v != want {
		t.Errorf("勝方：原版 %d／remake 的規則算出來是 %d"+
			"（守方統帥 %v、攻方統帥 %v）", v, want, defChief, atkChief)
	} else {
		t.Logf("勝方一致：原版與 remake 的規則都算出 %d（%s）", v,
			map[int]string{0: "主守軍", 1: "助守軍", 2: "主攻軍", 3: "助攻軍"}[v])
	}
	_ = fmt.Sprint
}
