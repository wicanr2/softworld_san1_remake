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

// 戰場上那九個選項分別是什麼動作。
//
// `docs/re/05` §12 把結構解出來了（入口 `0x29014`，九個選項依序試，
// 目標軍力 `es:[0x31a8]`）。這支保存普通相鄰盤面的自然樣本；罕見且會被
// 前面分支蓋掉的選項 2、6、7 另由 TestZZUnitAITargetedRoles 定向驗證。
//
// 這一支換一個問法：**把每一支跑期間叫到的「後果常式」記下來**。
// 交戰結算、計謀判定、天候門、對話框、畫部隊標記各自是什麼早就解了
// （`docs/re/05` §3.6／§4.0／§4.1／§2.5），所以「這一支叫了誰」
// 直接說出它在做什麼——不必讀那幾支的浮點模擬碼。
//
// ⚠ **後果要歸屬到正在跑的那一支**：同一個常式在選項之外也會被叫到
// （玩家自己的動作）。這裡只在「決策鏈進行中而且正停在某一支裡」時記。
func TestZZUnitAIOptionRoles(t *testing.T) {
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

	// 九個選項，順序就是決策鏈裡試的順序。
	options := []struct {
		at   uint32
		name string
	}{
		{0x29e78, "選項1"}, {0x29138, "選項2"}, {0x29344, "選項3"},
		{0x2985c, "選項4"}, {0x29784, "選項5"}, {0x29c56, "選項6"},
		{0x29b82, "選項7"}, {0x29ade, "選項8"}, {0x29e2e, "選項9"},
	}
	// 後果常式：位址的語意全部是已經解出來的（`docs/re/05`）。
	effects := []struct {
		at   uint32
		name string
	}{
		{0x2a224, "交戰結算"}, {0x2abb8, "計謀判定"}, {0x2bee8, "天候門"},
		{0x2a80a, "對話框"}, {0x21b40, "畫部隊"}, {0x30c5b, "單挑接受"},
		{0x2b6aa, "誘敵"}, {0x2bb00, "圍攻"}, {0x2adec, "火攻殺傷"},
		{0x2b15a, "水淹殺傷"}, {0x29896, "弓箭走兩步"}, {0x10b0c, "RND"},
	}

	slot := func() int {
		if dgroup == 0 {
			return -1
		}
		seg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa92e})
		return int(o.Word(oracle.Addr{Seg: seg, Off: 0x31c0}))
	}
	targetArmy := func() int {
		if dgroup == 0 {
			return -1
		}
		seg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa930})
		return int(o.Word(oracle.Addr{Seg: seg, Off: 0x31a8}))
	}

	type stat struct {
		tried, settled int
		targetArmies   map[int]int
		calls          map[string]int
	}
	st := map[string]*stat{}
	for _, opt := range options {
		st[opt.name] = &stat{targetArmies: map[int]int{}, calls: map[string]int{}}
	}

	inChain := false
	curOpt := ""
	// 進到下一支（或決策結尾）時，回頭判前一支有沒有定案。
	closeOpt := func() {
		if curOpt == "" {
			return
		}
		if slot() != 0xFFFF {
			s := st[curOpt]
			s.settled++
			s.targetArmies[targetArmy()]++
		}
		curOpt = ""
	}
	o.OnCall(addr(0x29014), func(*oracle.Oracle) { inChain, curOpt = true, "" })
	for _, opt := range options {
		name := opt.name
		o.OnCall(addr(opt.at), func(*oracle.Oracle) {
			if !inChain {
				return
			}
			closeOpt()
			st[name].tried++
			curOpt = name
		})
	}
	o.OnCall(addr(0x29132), func(*oracle.Oracle) {
		closeOpt()
		inChain = false
	})
	for _, e := range effects {
		name := e.name
		o.OnCall(addr(e.at), func(*oracle.Oracle) {
			if !inChain || curOpt == "" {
				return
			}
			st[curOpt].calls[name]++
		})
	}

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
	// **敵人不在旁邊電腦只會休息**：把玩家那支貼到守軍旁邊，
	// 前面那幾支選項才試得到（`docs/re/05` §12）。
	_, _ = placeNextToDefender(t, o, dgroup)

	for d := 1; d <= 14; d++ {
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

	t.Logf("九個選項各自的角色（跑 14 天）：")
	for _, opt := range options {
		s := st[opt.name]
		var acts []string
		for v, n := range s.targetArmies {
			acts = append(acts, fmt.Sprintf("%04X×%d", v, n))
		}
		sort.Strings(acts)
		var calls []string
		for k, n := range s.calls {
			calls = append(calls, fmt.Sprintf("%s×%d", k, n))
		}
		sort.Strings(calls)
		t.Logf("  %s(%05X)　試 %3d 次、定案 %3d 次　目標軍力 %v　叫了 %v",
			opt.name, opt.at, s.tried, s.settled, acts, calls)
	}

	tried := 0
	for _, s := range st {
		tried += s.tried
	}
	if tried == 0 {
		t.Fatal("決策鏈一次都沒跑到——沒進戰場？")
	}
	// **每一支都要試到過**，否則這一輪答不出它是什麼。
	var never []string
	for _, opt := range options {
		if st[opt.name].tried == 0 {
			never = append(never, opt.name)
		}
	}
	if len(never) > 0 {
		t.Logf("⚠ 這一輪沒試到：%v——它們前面就定案了，要換盤面才問得到",
			never)
	}
}

// TestZZUnitAITargetedRoles 專門換成「無箭、無金、不能移動」的盤面，
// 讓一般盤面會先定案的移動、弓箭與計謀都失敗，實跑原先到不了的選項 6、7。
// 另用兵力懸殊的快照驗選項 2 確實會呼叫退兵常式。
func TestZZUnitAITargetedRoles(t *testing.T) {
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

	current := 0
	controlledMode := ""
	retreatCalls := 0
	deathExchanges := 0
	duelPreludes := 0
	o.OnCall(addr(0x29138), func(oo *oracle.Oracle) {
		current = 2
		if controlledMode == "退兵" {
			// 292FE–29319 要求 RND(8)+3 不大於評估值；100.0 的 IEEE 754
			// 小端表示讓此盤面明確通過該門。
			seg := oo.Word(oracle.Addr{Seg: dgroup, Off: 0xa93e})
			for off := uint16(0x1bf0); off < 0x1bf6; off += 2 {
				oo.SetWord(oracle.Addr{Seg: seg, Off: off}, 0)
			}
			oo.SetWord(oracle.Addr{Seg: seg, Off: 0x1bf6}, 0x4059)
		}
	})
	o.OnCall(addr(0x29344), func(*oracle.Oracle) { current = 3 })
	o.OnCall(addr(0x29c56), func(oo *oracle.Oracle) {
		current = 6
		if controlledMode == "死戰" {
			// 29C62 的浮點門：把本次比較用的評估值釘成 0（≤ 門檻）。
			seg := oo.Word(oracle.Addr{Seg: dgroup, Off: 0xa96c})
			for off := uint16(0x1610); off < 0x1618; off += 2 {
				oo.SetWord(oracle.Addr{Seg: seg, Off: off}, 0)
			}
		}
	})
	o.OnCall(addr(0x29b82), func(oo *oracle.Oracle) {
		current = 7
		if controlledMode == "對戰" {
			// 29C00 的旗標非 0 會直接通過後面的浮點評估門。
			seg := oo.Word(oracle.Addr{Seg: dgroup, Off: 0xa940})
			oo.SetWord(oracle.Addr{Seg: seg, Off: 0x00a6}, 1)
		}
	})
	o.OnCall(addr(0x29ade), func(*oracle.Oracle) { current = 8 })
	o.OnCall(addr(0x23dd4), func(*oracle.Oracle) {
		if current == 2 {
			retreatCalls++
		}
	})
	o.OnCall(addr(0x2a224), func(*oracle.Oracle) {
		if current == 6 {
			deathExchanges++
		}
	})
	o.OnCall(addr(0x2deb0), func(*oracle.Oracle) {
		if current == 7 {
			duelPreludes++
		}
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
	me, foe := placeNextToDefender(t, o, dgroup)
	if me < 0 || foe < 0 {
		t.Fatal("盤面上湊不出攻守各一支")
	}

	work := o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg})
	w16 := func(off int) int {
		return int(o.Word(oracle.Addr{Seg: work, Off: uint16(off)}))
	}
	setw := func(off, v int) {
		o.SetWord(oracle.Addr{Seg: work, Off: uint16(off)}, uint16(v))
	}
	setSeed := func(seed uint32) {
		o.SetWord(oracle.Addr{Seg: dgroup, Off: 0xa3ae}, uint16(seed))
		o.SetWord(oracle.Addr{Seg: dgroup, Off: 0xa3b0}, uint16(seed>>16))
	}

	// 先走到相鄰守軍自己的決策入口，再從這裡做毫秒級快照重播；若從玩家
	// 確認畫面逐 seed 重跑，會把整段日動畫也重跑數百次，既慢又模糊判準。
	setw(foe+unitMove, 0)
	setw(foe+unitArrows, 0)
	setw(0x175e+6, 0)
	foeIndex := (foe - battleUnitBase) / battleUnitSize
	foeArmy, foeTeam := foeIndex/battleUnitPer, foeIndex%battleUnitPer
	o.Drain()
	o.PressScan("Y")
	decisionEntry := addr(0x29014).Linear()
	wantFoe := oracle.NewCond("相鄰守軍進入 AI 決策鏈", func(oo *oracle.Oracle) bool {
		return oo.IP().Linear() == decisionEntry && int(oo.Arg(0)) == foeArmy && int(oo.Arg(1)) == foeTeam
	})
	if err := o.RunUntil(wantFoe, oracle.Budget(80_000_000)); err != nil {
		t.Fatalf("找不到相鄰守軍的決策入口：%v", err)
	}
	decisionStart := o.Save()

	const fixedSeed = uint32(0x13579BDF)
	runDecision := func(mode string, weak bool) {
		o.Restore(decisionStart)
		current = 0
		controlledMode = mode
		beforeRetreat := retreatCalls
		beforeDeath, beforeDuel := deathExchanges, duelPreludes
		setw(foe+unitMove, 0)
		setw(foe+unitArrows, 0)
		setw(0x175e+6, 0) // 主守軍隨軍金；無金就不能用計。
		// 把守軍周圍除玩家所在格外都改成大山。選項 3 因無合法落點而
		// 快速失敗；選項 6／7 仍可對相鄰玩家部隊交戰。
		colSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9c8})
		rowSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9c6})
		fc, fr := w16(foe+unitCol), w16(foe+unitRow)
		mc, mr := w16(me+unitCol), w16(me+unitRow)
		for dir := 0; dir < 6; dir++ {
			i := uint16(((fc%2)*6 + dir) * 2)
			c := fc + int(int16(o.Word(oracle.Addr{Seg: colSeg, Off: 0x7c6a + i})))
			r := fr + int(int16(o.Word(oracle.Addr{Seg: rowSeg, Off: 0x7c82 + i})))
			if c < 0 || c >= 12 || r < 0 || r >= 10 || (c == mc && r == mr) {
				continue
			}
			a := oracle.Addr{Seg: work, Off: uint16(0x163a + r*12 + c)}
			o.SetByte(a, o.Byte(a)&0xf0|1)
		}
		if weak {
			setw(foe+unitSoldiers, 100)
			setw(me+unitSoldiers, 5000)
		} else {
			setw(foe+unitSoldiers, 3000)
			setw(me+unitSoldiers, 1000)
		}
		// 選項 6 在 29CC3 明確拒絕「守軍站城池／關寨且目標是攻方」；
		// 把守軍腳下改成平原，才是在驗該動作而非反覆撞地形門。
		foeTile := oracle.Addr{Seg: work, Off: uint16(0x163a + w16(foe+unitRow)*12 + w16(foe+unitCol))}
		o.SetByte(foeTile, o.Byte(foeTile)&0xf0|7)
		setSeed(fixedSeed)
		// 此測試問的是分支角色，不是原版的骰序。固定起始 seed 之外，依
		// dosgolem 的受控亂數契約，把每一道門的結果釘死：跳過弓箭／用計，
		// 再分別打開選項 6 或 7；退兵盤面則全部回 0。
		o.Stub(addr(0x10b0c), func(p *oracle.Oracle) uint32 {
			n := int(p.Arg(0))
			switch mode {
			case "死戰":
				switch n {
				case 2:
					return 1
				case 3, 4:
					return 0
				}
			case "對戰":
				switch n {
				case 2, 4:
					return 1
				case 3, 16, 100:
					return 0
				}
			case "退兵":
				return 0
			}
			return 0
		})
		stop := oracle.NewCond("命中未解選項或走完這次決策", func(oo *oracle.Oracle) bool {
			return retreatCalls > beforeRetreat || deathExchanges > beforeDeath ||
				duelPreludes > beforeDuel || oo.IP().Linear() == addr(0x29132).Linear()
		})
		if err := o.RunUntil(stop, oracle.Budget(20_000_000)); err != nil {
			t.Fatalf("%s盤面停止：%v", mode, err)
		}
		o.Stub(addr(0x10b0c), nil)
	}

	// 強守軍盤面：選項 2 不會退，移動／弓箭／計謀則刻意封住。
	beforeDeath := deathExchanges
	runDecision("死戰", false)
	if deathExchanges == beforeDeath {
		t.Fatal("受控亂數打開選項 6 後仍未呼叫交戰結算")
	}
	beforeDuel := duelPreludes
	runDecision("對戰", false)
	if duelPreludes == beforeDuel {
		t.Fatal("受控亂數打開選項 7 後仍未呼叫對戰前置")
	}

	// 弱守軍盤面：選項 2 的門檻是本隊兵力不高於相鄰敵軍的約 1/4～1/5。
	beforeRetreat := retreatCalls
	runDecision("退兵", true)
	if retreatCalls == beforeRetreat {
		t.Fatal("兵力懸殊盤面未讓選項 2 呼叫退兵常式")
	}

	t.Logf("固定 seed=%#08x；受控亂數分別命中選項 2 退兵、選項 6 死戰、選項 7 對戰",
		fixedSeed)
}

// placeNextToDefender 把玩家那支部隊擺到守軍旁邊。
//
// 盤面直接寫記憶體：走正規流程要先解出移動那條按鍵路徑，而這一支要問的
// 不是移動。
func placeNextToDefender(t *testing.T, o *oracle.Oracle, dgroup uint16) (int, int) {
	t.Helper()
	return placeNextToDefenderRig(t, o, dgroup, baseDayRig())
}

// placeNextToDefenderRig 是本體，路標由 rig 給（加強版的佔位圖、部隊記錄
// 與方向表在別的位移）。佔位圖與部隊記錄在工作區，方向表在 DGROUP。
func placeNextToDefenderRig(t *testing.T, o *oracle.Oracle, dgroup uint16, rig dayRig) (int, int) {
	t.Helper()
	return placeNearDefenderRig(t, o, dgroup, rig, false, false)
}

// placeNearDefenderRig：apart 為真時擺到守軍**同一方向連走兩步**的落點
// （中間那格空著、地形不是大山／城池／關寨），弓箭那一支才有目標。
func placeNearDefenderRig(t *testing.T, o *oracle.Oracle, dgroup uint16, rig dayRig, apart, solid bool) (int, int) {
	t.Helper()
	work := o.Word(oracle.Addr{Seg: dgroup, Off: rig.workSegPtr})
	occSeg, colSeg, rowSeg := work, dgroup, dgroup
	w16 := func(off int) int {
		return int(o.Word(oracle.Addr{Seg: work, Off: uint16(off)}))
	}
	occ := func(c, r int) oracle.Addr {
		return oracle.Addr{Seg: occSeg, Off: uint16(rig.occ + (r*12+c)*2)}
	}
	recOf := func(army, team int) int {
		return rig.unitBase + (army*battleUnitPer+team)*battleUnitSize
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
		t.Log("擺不了位：找不到兩邊的部隊")
		return me, foe
	}
	fc, fr := w16(foe+unitCol), w16(foe+unitRow)
	step := func(c, r, dir int) (int, int) {
		i := uint16(((c%2)*6 + dir) * 2)
		dc := int(int16(o.Word(oracle.Addr{Seg: colSeg, Off: rig.colTable + i})))
		dr := int(int16(o.Word(oracle.Addr{Seg: rowSeg, Off: rig.rowTable + i})))
		return c + dc, r + dr
	}
	inside := func(c, r int) bool { return c >= 0 && c < 12 && r >= 0 && r < 10 }
	terrain := func(c, r int) byte { return o.Byte(oracle.Addr{Seg: work, Off: uint16(0x163a + r*12 + c)}) & 0xf }
	for dir := 0; dir < 6; dir++ {
		c, r := step(fc, fr, dir)
		if !inside(c, r) || o.Word(occ(c, r)) != 0xFFFF {
			continue
		}
		// solid：**不擺在大山（碼 1）上**。原版的部隊走不進大山、紮寨也紮
		// 不上去，那是遊戲裡取不到的格；對戰子畫面拿目標所在格的地形碼
		// 挑版型（`(碼 − 2) × 2`，`docs/re/05` §10），碼 1 會索引到版型表
		// 前面的記憶體——量到整張子地圖是垃圾、誰都走不動（Issue #30）。
		// 沒開的盤面照舊擺在帥隊左上那格（郡 25 是 (4,3) 的大山）：四張
		// 盤面的鏈都是在那裡拍的，換到 (6,2) 會同時貼著左右軍，快戰的
		// 反擊把守將打光、原版停在「請主公裁決」，鏈就沒了。
		if ter := terrain(c, r); solid && (ter < 2 || ter > 9) {
			continue
		}
		if apart {
			// 中間那格的地形碼查弓箭表（`DS:0x80d4`／`0x8242`）要是 1：
			// 山丘、淺水、深水、平原、樹林、沙漠。
			ter := terrain(c, r)
			switch ter {
			case 2, 3, 4, 7, 8, 9:
			default:
				continue
			}
			c, r = step(c, r, dir)
			if !inside(c, r) || o.Word(occ(c, r)) != 0xFFFF {
				continue
			}
		}
		o.SetWord(occ(w16(me+unitCol), w16(me+unitRow)), 0xFFFF)
		o.SetWord(oracle.Addr{Seg: work, Off: uint16(me + unitCol)}, uint16(c))
		o.SetWord(oracle.Addr{Seg: work, Off: uint16(me + unitRow)}, uint16(r))
		o.SetWord(occ(c, r), 20)
		t.Logf("把玩家那支擺到 (%d,%d)，貼著守軍的 (%d,%d)", c, r, fc, fr)
		return me, foe
	}
	t.Log("守軍旁邊沒有空格")
	return me, foe
}
