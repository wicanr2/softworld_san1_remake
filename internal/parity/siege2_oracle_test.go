//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 圍攻用**兩支真的部隊**跑一次（`docs/mechanics/40-military` §3.3b 的缺口）。
//
// `TestStratagemsRunLive` 那一場攻方只有一支部隊，是把它的編號多寫進一個
// 空鄰格湊出門檻的——那驗得了「條件怎麼算、費用扣多少、每個鄰格叫一次
// 結算」，驗不了「兩支各自帶自己的兵與綜合能力進 `0x2a224`」。
//
// 這裡多給玩家一位守將，整編就會分出兩支；**位置直接寫記憶體**擺到同一個
// 守軍的兩個鄰格（部隊記錄的欄／列 ＋ 佔位圖 `es:[0x2532]`），比驅動移動鍵
// 可靠，也不必猜地圖長什麼樣（`CLAUDE.md`：對拍的盤面自己擺）。
func TestSiegeWithTwoUnits(t *testing.T) {
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

	staBase := base + uint32(state.MasterTableSize)
	genBase := staBase + uint32(state.PrefectureTableSize)
	const staRec = state.PrefectureRecordSize
	mine := int(o.Byte(addr(staBase + uint32(at)*staRec + 30)))

	_ = mine

	var dgroup uint16
	o.OnCall(addr(0x2053c), func(o *oracle.Oracle) {
		if dgroup == 0 {
			dgroup = o.DSReg()
		}
	})
	cmdReads := 0
	o.OnCall(addr(0x27a68), func(*oracle.Oracle) { cmdReads++ })
	type bout struct{ aArmy, aTeam, dArmy, dTeam, mode int }
	var bouts []bout
	o.OnCall(addr(0x2a224), func(o *oracle.Oracle) {
		bouts = append(bouts, bout{
			int(o.Arg(0)), int(o.Arg(1)), int(o.Arg(2)), int(o.Arg(3)), int(o.Arg(4))})
	})

	driveIntoBattle(t, o, at, to)
	if dgroup == 0 {
		t.Fatal("沒有進到主戰場——兩位將的整編按鍵序列不對")
	}
	work := o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg})
	w16 := func(off int) int {
		return int(o.Word(oracle.Addr{Seg: work, Off: uint16(off)}))
	}
	setw := func(off, v int) {
		o.SetWord(oracle.Addr{Seg: work, Off: uint16(off)}, uint16(v))
	}
	recOf := func(army, team int) int {
		return battleUnitBase + (army*battleUnitPer+team)*battleUnitSize
	}
	// **段變數要各取各的**（`docs/re/05` §4.0）：佔位圖在 `[0xa9ca]`、
	// 方向位移表在 `[0xa9c8]`（欄）與 `[0xa9c6]`（列）。拿部隊記錄那個
	// 段（`[0xa872]`）去讀佔位圖不會報錯，只會每一格都看起來有人。
	occSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9ca})
	colSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9c8})
	rowSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9c6})
	t.Logf("段：部隊 %#06x｜佔位 %#06x｜欄位移 %#06x｜列位移 %#06x",
		work, occSeg, colSeg, rowSeg)
	occ := func(col, row int) oracle.Addr {
		return oracle.Addr{Seg: occSeg, Off: uint16(0x2532 + (row*12+col)*2)}
	}

	type slot struct{ army, team, rec int }
	var mineUnits, foes []slot
	for army := 0; army < battleArmies; army++ {
		for team := 0; team < battleTeams; team++ {
			rec := recOf(army, team)
			if w16(rec) == 0xFFFF || w16(rec+unitLeaders) <= 0 {
				continue
			}
			s := slot{army, team, rec}
			if army >= 2 {
				mineUnits = append(mineUnits, s)
			} else {
				foes = append(foes, s)
			}
		}
	}
	t.Logf("攻方 %d 支、守方 %d 支", len(mineUnits), len(foes))
	if len(mineUnits) == 0 || len(foes) == 0 {
		t.Fatalf("盤面不對：攻方 %d 支、守方 %d 支", len(mineUnits), len(foes))
	}

	// **第二支部隊直接複製記錄擺出來。** 攻方整編只出一支（郡裡只有
	// 一位守將），而 `0x2c140` 與 `0x2bb00` 讀的是佔位圖與部隊記錄。
	// 把 `2-0` 整份 42 byte 複製到 `2-1`，再把兵與綜合能力改成不一樣的
	// 數——**兩支的數字不同，「各自帶自己的編號進結算」才可證偽**。
	//
	// ⚠ 這一支是擺出來的：欄位形狀與值都是從真記錄來的，但它不是原版
	// 自己整編出來的部隊。驗得了「每個鄰格各叫一次結算、帶的是那一格
	// 自己的編號」，驗不了整編會不會這樣分。
	if len(mineUnits) == 1 {
		src := mineUnits[0]
		dst := slot{2, 1, recOf(2, 1)}
		wa := func(off int) oracle.Addr {
			return oracle.Addr{Seg: work, Off: uint16(off)}
		}
		o.SetBytes(wa(dst.rec), o.Bytes(wa(src.rec), battleUnitSize))
		setw(dst.rec+unitSoldiers, 1700)
		setw(dst.rec+unitAbility, 61)
		mineUnits = append(mineUnits, dst)
		t.Logf("把 %d-%d（兵 %d 綜合能力 %d）複製成 2-1，改成兵 1700 綜合能力 61",
			src.army, src.team, w16(src.rec+unitSoldiers), w16(src.rec+unitAbility))
	}

	// 紮寨 → 補第 1 天的確認。
	for step := 1; step <= 12 && cmdReads == 0; step++ {
		o.Drain()
		o.PressScan("0")
		if err := o.Run(60_000_000); err != nil {
			t.Fatalf("紮寨第 %d 步停止：%v", step, err)
		}
	}
	if cmdReads == 0 {
		t.Fatal("紮完寨沒問到命令")
	}
	o.Drain()
	o.PressScan("Y")
	if err := o.Run(60_000_000); err != nil {
		t.Fatalf("確認第 1 天停止：%v", err)
	}

	// 盤面自己擺：挑一支旁邊有兩個空格的守軍，把我方兩支放進去。
	place := func(s slot, col, row int) {
		o.SetWord(occ(w16(s.rec+unitCol), w16(s.rec+unitRow)), 0xFFFF)
		setw(s.rec+unitCol, col)
		setw(s.rec+unitRow, row)
		o.SetWord(occ(col, row), uint16(s.army*battleUnitPer+s.team))
	}
	// 位移表在工作區的 `0x7c6a`（欄）／`0x7c82`（列），
	// 索引 `(那一格的欄 % 2) × 6 + 方向`。
	off := func(col, dir int) (int, int) {
		i := uint16(((col%2)*6 + dir) * 2)
		dc := int(int16(o.Word(oracle.Addr{Seg: colSeg, Off: 0x7c6a + i})))
		dr := int(int16(o.Word(oracle.Addr{Seg: rowSeg, Off: 0x7c82 + i})))
		return dc, dr
	}
	var tgt slot
	var tc, tr int
	var spots [][2]int
	for _, f := range foes {
		c0, r0 := w16(f.rec+unitCol), w16(f.rec+unitRow)
		var free [][2]int
		for dir := 0; dir < 6; dir++ {
			dc, dr := off(c0, dir)
			c, r := c0+dc, r0+dr
			if c < 0 || c >= 12 || r < 0 || r >= 10 {
				continue
			}
			if o.Word(occ(c, r)) != 0xFFFF {
				continue
			}
			free = append(free, [2]int{c, r})
		}
		t.Logf("守軍 %d-%d 在 (%d,%d)：空鄰格 %d 個 %v",
			f.army, f.team, c0, r0, len(free), free)
		if len(free) >= 2 && len(spots) == 0 {
			tgt, tc, tr, spots = f, c0, r0, free[:2]
		}
	}
	if len(spots) < 2 {
		t.Fatal("每一支守軍旁邊都湊不出兩個空格")
	}
	place(mineUnits[0], spots[0][0], spots[0][1])
	place(mineUnits[1], spots[1][0], spots[1][1])
	t.Logf("目標 %d-%d 在 (%d,%d)；我方 %d-%d → (%d,%d)、%d-%d → (%d,%d)",
		tgt.army, tgt.team, tc, tr,
		mineUnits[0].army, mineUnits[0].team, spots[0][0], spots[0][1],
		mineUnits[1].army, mineUnits[1].team, spots[1][0], spots[1][1])

	// 找出從第一支指到目標的方向。
	dir := -1
	c0, r0 := spots[0][0], spots[0][1]
	for d := 0; d < 6; d++ {
		dc, dr := off(c0, d)
		if c0+dc == tc && r0+dr == tr {
			dir = d
		}
	}
	if dir < 0 {
		t.Fatalf("從 (%d,%d) 指不到 (%d,%d)", c0, r0, tc, tr)
	}

	// **成功判定要先擺平**（`docs/re/05` §4.1）：`RND(上限) + 目標領隊的
	// 謀略 < 施法者領隊的謀略` 才成功，而**錢是先付再賭**——被識破時金
	// 照扣、效果不跑，印出來與「效果常式沒被呼叫」一模一樣。把目標那支
	// 的謀略最高者（部隊記錄 offset 38）壓到 10。
	const unitWisest = 38
	if who := w16(tgt.rec + unitWisest); who >= 0 && who < 350 {
		o.SetByte(addr(genBase+uint32(who)*30+9), 10)
		t.Logf("目標 %d-%d 的領隊是人物 %d，謀略壓到 10", tgt.army, tgt.team, who)
	} else {
		t.Logf("目標 %d-%d 的 offset 38 是 %d，沒壓謀略", tgt.army, tgt.team, who)
	}

	// **倍率格是算出來的**（`docs/re/05` §4.5）：目標六個鄰格裡與它不同
	// 陣營的部隊數，加上「施法者的領隊謀略 ≥ 98」那一分。
	//
	// ⚠ **要在下計謀之前算。** 打完之後被清空的部隊佔位會跟著清掉，
	// 事後再數會少——量到過一次，兩支圍攻打完剩一支，算出來的模式
	// 比原版少 1。
	around := 0
	for dir := 0; dir < 6; dir++ {
		dc, dr := off(tc, dir)
		c, r := tc+dc, tr+dr
		if c < 0 || c >= 12 || r < 0 || r >= 10 {
			continue
		}
		v := int(o.Word(occ(c, r)))
		if v == 0xFFFF || v/20 == tgt.army/2 {
			continue
		}
		around++
	}
	wantMode := around + 1 // 施法者的領隊謀略是 99
	t.Logf("下計謀之前：圍著目標的我方部隊 %d 支，模式應該是 %d",
		around, wantMode)

	goldBefore := w16(0x175e + 2*22 + 6)
	before := map[string]int{}
	for _, s := range append(append([]slot{}, mineUnits...), foes...) {
		before[fmt.Sprintf("%d-%d", s.army, s.team)] = w16(s.rec + unitSoldiers)
	}
	bouts = bouts[:0]

	// `6`（策略）→ 方向 → `6`（圍攻）。命令提示要收得到那個 `6`。
	chose := false
	for try := 1; try <= 6 && !chose; try++ {
		was := cmdReads
		o.Drain()
		o.PressScan("6")
		if err := o.Run(80_000_000); err != nil {
			t.Fatalf("送 `6` 第 %d 次停止：%v", try, err)
		}
		if cmdReads == was {
			continue
		}
		for _, k := range []string{fmt.Sprintf("%d", dir+1), "6"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(80_000_000); err != nil {
				t.Fatalf("送 %q 停止：%v", k, err)
			}
		}
		chose = true
	}
	if !chose {
		t.Fatal("送了六次 `6` 都不是命令提示收的")
	}

	goldAfter := w16(0x175e + 2*22 + 6)
	t.Logf("圍攻：金 %d → %d（扣 %d）", goldBefore, goldAfter, goldBefore-goldAfter)
	for _, b := range bouts {
		t.Logf("交戰結算：攻 %d-%d 守 %d-%d 模式 %d",
			b.aArmy, b.aTeam, b.dArmy, b.dTeam, b.mode)
	}
	for _, s := range append(append([]slot{}, mineUnits...), foes...) {
		k := fmt.Sprintf("%d-%d", s.army, s.team)
		if now := w16(s.rec + unitSoldiers); now != before[k] {
			t.Logf("%s 兵 %d → %d", k, before[k], now)
		}
	}

	if goldBefore-goldAfter != 200 {
		t.Errorf("圍攻的費用是 %d，表上是 200", goldBefore-goldAfter)
	}
	// **兩支各自進結算**：兩支不同的部隊各對目標打一次。
	seen := map[string]int{}
	for _, b := range bouts {
		if b.dArmy == tgt.army && b.dTeam == tgt.team {
			seen[fmt.Sprintf("%d-%d", b.aArmy, b.aTeam)] = b.mode
		}
	}
	for _, s := range mineUnits[:2] {
		k := fmt.Sprintf("%d-%d", s.army, s.team)
		if _, ok := seen[k]; !ok {
			t.Errorf("我方 %s 貼著目標，卻沒有對它跑過交戰結算（收到 %v）",
				k, seen)
		}
	}
	for k, m := range seen {
		if m != wantMode {
			t.Errorf("%s 那一次的模式是 %d，算出來應該是 %d"+
				"（圍著目標的敵方部隊 %d 支 ＋ 謀略加碼 1）",
				k, m, wantMode, around)
		}
	}
	t.Logf("模式量到 %v", seen)
}
