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

// 六種戰場計謀在原版裡各走一次。
//
// `docs/mechanics/40-military.md` 一直只有陷阱是實跑量到的，其餘五種
// 靠反組譯推。這支把同一個盤面存成快照，六種各還原一次再送鍵，
// 量的是**原版自己的記憶體**：主攻軍的金扣了多少、目標那支的兵士數與
// 中陷阱天數怎麼變、交戰結算跑了幾次。
//
// 按鍵流程（`docs/re/05` §7.0）：命令提示收 `6` → **先問方向**
// （`0x28d7a` 查 `DS:0x7c6a`／`DS:0x7c82`，那一格沒敵人就直接取消）→
// 才是計謀選單（`DS:0x8113`：`1.火攻 2.水渰 3.陷阱 4.誘敵 5.燒糧 6.圍攻`）。
//
// 費用表在 `DS:0x7f62`；remake 這一邊是 `battle.Stratagem.Cost()`。
func TestStratagemsRunLive(t *testing.T) {
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
	o.OnCall(addr(0x27a68), func(o *oracle.Oracle) { cmdReads++ })
	exchanges := 0
	o.OnCall(addr(0x2a224), func(o *oracle.Oracle) { exchanges++ })
	// `0x28af5` 是計謀選單讀完鍵回來的那一刻（`docs/re/05` §7.0 的
	// OnCall 邊界那個坑）。它沒動就代表選單根本沒開。
	plotMenu := 0
	o.OnCall(addr(0x28af5), func(*oracle.Oracle) { plotMenu++ })

	driveIntoBattle(t, o, at, to)
	if dgroup == 0 {
		t.Fatal("沒有進到主戰場 0x2053c——按鍵序列或盤面不對")
	}
	work := o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg})
	// 天候與戰場地圖各自從 DGROUP 的段變數取（`docs/re/05` §4.0）。
	wxSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9d6})
	mapSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa99e})
	weatherAt := oracle.Addr{Seg: wxSeg, Off: 0x17bc}
	t.Logf("工作區段 %#06x｜天候段 %#06x（現值 %d）｜地圖段 %#06x",
		work, wxSeg, o.Word(weatherAt), mapSeg)
	w16 := func(off int) int {
		return int(o.Word(oracle.Addr{Seg: work, Off: uint16(off)}))
	}
	recOf := func(army, team int) int {
		return battleUnitBase + (army*battleUnitPer+team)*battleUnitSize
	}
	armyGold := func(army int) int { return w16(0x175e + army*22 + 6) }

	// 盤面上有哪幾支。
	type slot struct{ army, team, rec int }
	var slots []slot
	for army := 0; army < battleArmies; army++ {
		for team := 0; team < battleTeams; team++ {
			rec := recOf(army, team)
			if w16(rec) == 0xFFFF || w16(rec+unitLeaders) <= 0 {
				continue
			}
			slots = append(slots, slot{army, team, rec})
		}
	}
	if len(slots) == 0 {
		t.Fatal("一支部隊都沒讀到——工作區的段或部隊記錄的位置不對")
	}

	// 紮寨：1–6 移游標、`0` 紮下去，一支一支問。判準是命令提示有沒有
	// 讀走一個鍵（`0x27a68`），不是數按鍵。
	for step := 1; step <= 10 && cmdReads == 0; step++ {
		o.Drain()
		o.PressScan("0")
		if err := o.Run(60_000_000); err != nil {
			t.Fatalf("紮寨第 %d 次停止：%v", step, err)
		}
	}
	if cmdReads == 0 {
		t.Fatal("送了十個 `0` 都還沒問到命令——紮寨的按鍵序列不對")
	}
	// **紮寨的最後一個 `0` 已經是第 1 天的命令（休息）**，補一個 `Y`
	// 把那一天的 Y/N 確認答掉（`0x1538c`），天數才會推到 2。
	// 不補的話玩家那支在第 1 天已經用掉命令，後面按 `1` 進不了移動模式
	// ——量到過一次，部隊留在原地而按鍵一路吃下去，看起來像方向不對。
	o.Drain()
	o.PressScan("Y")
	if err := o.Run(60_000_000); err != nil {
		t.Fatalf("確認第 1 天的休息停止：%v", err)
	}
	if day := w16(0x2100); day != 2 {
		t.Fatalf("答完第 1 天的 Y/N，天數是 %d，應該是 2", day)
	}

	// 走到守軍旁邊。命令 `1` 進移動模式，之後 1–6 一律當方向，
	// **只有 Enter 離得開**（`0x27d63`）。
	me := -1
	for i, sl := range slots {
		if sl.army == 2 {
			me = i
		}
	}
	if me < 0 {
		t.Fatal("盤面上找不到主攻軍——玩家沒有部隊就用不了計謀")
	}
	startCol, startRow := w16(slots[me].rec+unitCol), w16(slots[me].rec+unitRow)
	o.Drain()
	o.PressScan("1")
	if err := o.Run(40_000_000); err != nil {
		t.Fatalf("進移動模式停止：%v", err)
	}
	for step := 1; step <= 2; step++ {
		o.Drain()
		o.PressScan("5") // 5 ＝ 往上
		if err := o.Run(40_000_000); err != nil {
			t.Fatalf("移動第 %d 步停止：%v", step, err)
		}
	}
	o.Drain()
	o.PressScan("\r")
	if err := o.Run(40_000_000); err != nil {
		t.Fatalf("離開移動模式停止：%v", err)
	}
	var sb strings.Builder
	for _, sl := range slots {
		fmt.Fprintf(&sb, "[%d-%d 格 %d,%d 兵 %d] ", sl.army, sl.team,
			w16(sl.rec+unitCol), w16(sl.rec+unitRow), w16(sl.rec+unitSoldiers))
	}
	t.Logf("要下計謀的盤面：天數 %d 金 %d｜%s",
		w16(0x2100), armyGold(2), sb.String())

	// **前置條件要在掃之前就斷言**：計謀先問方向，`0x28d7a` 算出來的
	// 那一格沒有敵人就直接取消、也不扣錢——六種全部「沒扣錢」與
	// 「六種的條件都不合」長得一模一樣。方向用 `5`（欄位移 0、
	// 列位移 −1），所以正上方那一格要站著守軍。
	myCol, myRow := w16(slots[me].rec+unitCol), w16(slots[me].rec+unitRow)
	if myCol == startCol && myRow == startRow {
		t.Fatalf("主攻軍還在 (%d,%d) 沒動——移動模式的按鍵不對，"+
			"不是計謀的問題", myCol, myRow)
	}
	adj := -1
	for i, sl := range slots {
		if sl.army >= 2 {
			continue // 0、1 是守方
		}
		if w16(sl.rec+unitCol) == myCol && w16(sl.rec+unitRow) == myRow-1 {
			adj = i
		}
	}
	if adj < 0 {
		t.Fatalf("主攻軍在 (%d,%d)，正上方 (%d,%d) 沒有守軍——"+
			"方向 `5` 會被判成無效目標，六種計謀都發不出去",
			myCol, myRow, myCol, myRow-1)
	}
	t.Logf("主攻軍在 (%d,%d)，方向 `5` 指到守軍 %d-%d",
		myCol, myRow, slots[adj].army, slots[adj].team)

	// **天候要在快照那一刻再記一次。** 進場時記到的是第 1 天的值，
	// 掃描發生在第 3 天——燒糧在下雨天會被擋（§4.0），拿第 1 天的值
	// 去解釋第 3 天的結果會得到一個假的矛盾。
	t.Logf("快照時的天候 ＝ %d（1 下雨、2 刮風）", o.Word(weatherAt))
	snap := o.Save()

	// 選單索引 → remake 的計謀。順序是 `DS:0x8113` 的字串表。
	// **條件自己擺**（`docs/re/05` §4.0）：火攻要天候 2（刮風），
	// 水淹要天候 1（下雨）**而且**目標的六個鄰格裡有一格淺水（地形碼 3）。
	// 兩者都在還原快照之後才寫，所以不影響其餘四種那幾次。
	menu := []struct {
		key   string
		s     battle.Stratagem
		setup func()
	}{
		{"3", battle.Trap, nil}, // 已經實跑過的那一支，當正對照排第一
		{"1", battle.Fire, func() { o.SetWord(weatherAt, 2) }},
		{"2", battle.Flood, func() {
			o.SetWord(weatherAt, 1)
			// 目標在 (欄, 列)；把它正上方那一格改成淺水。
			col := w16(slots[adj].rec + unitCol)
			row := w16(slots[adj].rec + unitRow)
			at := oracle.Addr{Seg: mapSeg, Off: uint16(0x163a + (row-1)*12 + col)}
			o.SetByte(at, o.Byte(at)&0xf0|3)
		}},
		{"4", battle.Lure, nil},
		{"5", battle.Burn, nil},
		{"6", battle.Siege, nil},
	}

	type shot struct {
		gold, move int
		sol        []int
		trapped    []int
	}
	take := func() shot {
		s := shot{gold: armyGold(2), move: w16(slots[me].rec + unitMove)}
		for _, sl := range slots {
			s.sol = append(s.sol, w16(sl.rec+unitSoldiers))
			s.trapped = append(s.trapped, w16(sl.rec+unitTrapped))
		}
		return s
	}

	ran, checked, bad := 0, 0, 0
	for _, m := range menu {
		o.Restore(snap)
		if m.setup != nil {
			m.setup()
		}
		before := take()
		wx := int(o.Word(weatherAt))
		wasEx := exchanges
		wasMenu := plotMenu

		// **要分得出 `6` 是被誰吃掉的。** 命令提示讀走一個鍵時
		// `0x27a68` 會跑一次；沒跑就代表這一鍵落到別的提示上，
		// 那時再送選項會被計謀選單當成選擇。
		chose := false
		for try := 1; try <= 4 && !chose; try++ {
			was := cmdReads
			o.Drain()
			o.PressScan("6")
			if err := o.Run(80_000_000); err != nil {
				t.Fatalf("%v：送 `6` 第 %d 次停止：%v", m.s, try, err)
			}
			if cmdReads == was {
				continue
			}
			// 方向 `5`（往上，索引 4：欄位移 0、列位移 −1）——
			// 玩家那支在守軍正下方。
			for _, k := range []string{"5", m.key} {
				o.Drain()
				o.PressScan(k)
				if err := o.Run(80_000_000); err != nil {
					t.Fatalf("%v：送 %q 停止：%v", m.s, k, err)
				}
			}
			chose = true
		}
		if !chose {
			t.Errorf("%v：送了四次 `6` 都不是命令提示收的——玩家那支這時沒輪到", m.s)
			continue
		}

		now := take()
		spent := before.gold - now.gold
		menuHits := plotMenu - wasMenu
		var eff strings.Builder
		for i, sl := range slots {
			if d := before.sol[i] - now.sol[i]; d != 0 {
				fmt.Fprintf(&eff, "[%d-%d 兵 %d→%d] ",
					sl.army, sl.team, before.sol[i], now.sol[i])
			}
			if now.trapped[i] != before.trapped[i] {
				fmt.Fprintf(&eff, "[%d-%d 陷阱 %d 天] ",
					sl.army, sl.team, now.trapped[i])
			}
		}
		t.Logf("%v（選單 %s，天候 %d）：金 %d→%d 扣 %d（remake 的表 %d）｜"+
			"選單讀鍵 %d 次｜交戰結算 %d 次｜%s",
			m.s, m.key, wx, before.gold, now.gold, spent, m.s.Cost(),
			menuHits, exchanges-wasEx, eff.String())

		if menuHits == 0 {
			t.Errorf("%v：計謀選單一次都沒讀鍵——`6` 或方向那一步就斷了，"+
				"這一次量不到計謀本身", m.s)
			continue
		}
		if spent == 0 {
			// 選單開過而金沒動，才輪得到「條件不合」這個解釋。
			t.Logf("%v：選單開了但沒扣錢——門檻／天候／地形其中一項不合", m.s)
			continue
		}
		ran++
		checked++
		if spent != m.s.Cost() {
			bad++
			t.Errorf("%v 的費用：原版扣 %d 金／remake 的表是 %d 金",
				m.s, spent, m.s.Cost())
		}
		// **移動力在這個觀測點量不到。** 玩家下完命令之後其他部隊還會
		// 動，天數也可能已經翻頁——翻頁時原版把剩下的補回上限
		// （`0x24ee1`），所以「還剩滿點」與「清成 0 之後又被補滿」
		// 印出來一樣。要驗這一欄得攔計謀常式回來的那一刻，不是等
		// 整輪跑完再讀。這裡不比。
	}
	t.Logf("六種計謀送了一輪，成立 %d 種；比了 %d 個欄位，對不上 %d 個",
		ran, checked, bad)
	if ran == 0 {
		t.Fatal("六種一種都沒成立——方向或按鍵流程不對，不是計謀本身的條件")
	}
}
