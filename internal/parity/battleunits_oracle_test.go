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
	battleArmies   = 4 // 主守、助守、主攻、助攻
	battleTeams    = 5 // 一個軍團五個隊伍
	battleUnitPer  = 10 // 陣列跨距

	unitCol      = 22 // 欄
	unitRow      = 24 // 列
	unitLeaders  = 28 // 將領人數
	unitSoldiers = 30 // 兵士數
	unitAbility  = 32 // 綜合能力
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

	// **移動力（offset 36）是「這一天剩下的」，佈陣完還是 0**——
	// 位置也都在 (0,0)，紮寨還沒送。紮寨的提示是
	// `數字鍵選方向 / 4 5 6 / 1 2 3 / 0:紮寨`：**1–6 是移游標，`0` 才是
	// 紮下去**（`DS:0x7f9e`）。一支一支問，全部就位之後才進第一天。
	for step := 1; step <= 10; step++ {
		o.Drain()
		o.PressScan("0")
		if err := o.Run(40_000_000); err != nil {
			t.Fatalf("紮寨第 %d 步停止：%v", step, err)
		}
		var sb strings.Builder
		placed := 0
		for _, s := range slots {
			col, row, mov := w16(s.rec+unitCol), w16(s.rec+unitRow), w16(s.rec+unitMove)
			if col != 0 || row != 0 || mov != 0 {
				placed++
			}
			fmt.Fprintf(&sb, "[%d-%d 格 %d,%d 移 %d／remake %d] ",
				s.army, s.team, col, row, mov, s.u.MovePoints())
		}
		t.Logf("紮寨第 %2d 步：%s", step, sb.String())
		if placed == len(slots) {
			t.Logf("全部 %d 支都就位了", placed)
			break
		}
	}

	// **移動力要等紮完寨才比**：offset 36 是「這一天剩下的」，佈陣的
	// 時候還是 0。offset 34 是這支部隊一天的上限，開始新的一天時
	// 36 ← 34（`0x27200`）。
	for _, s := range slots {
		cap34, left36 := w16(s.rec+unitCap), w16(s.rec+unitMove)
		var who strings.Builder
		for _, l := range s.u.Leaders {
			fmt.Fprintf(&who, "槽 %d 訓 %d 武裝 %d 兵 %d；",
				l.Index, l.Training, l.Arms, l.Soldiers)
		}
		checked++
		if got := s.u.MovePoints(); got != cap34 {
			bad++
			t.Errorf("%d-%d 移動力上限：原版 %d／remake %d｜%s",
				s.army, s.team, cap34, got, who.String())
		}
		t.Logf("%d-%d 移動力上限 %d（剩 %d）｜%s",
			s.army, s.team, cap34, left36, who.String())
	}
	t.Logf("連移動力一起算：比了 %d 個欄位，對不上 %d 個", checked, bad)

	// 逐日對拍的下一步：把一天送完。主戰場的選單是
	// `1.移動 2.對戰 3.快戰 4.死戰 5.弓箭 6.策略 7.查看 8.退兵 0.休息`
	// （`DS:0x7f22`）。**全部休息**是最乾淨的一天：不動、不打，
	// 只看天數跳不跳、移動力怎麼被夾。
	//
	// 天數在 `es:[0x2100]`，而 `es` 在戰術層是從好幾格取的
	// （`ds:0xa872`／`0xa896`／`0xa89e`／`0xa8a8`／`0xa8ce`）——
	// 先把每一格當段去讀 0x2100，看哪一個像天數。
	segs := []uint16{0xa872, 0xa896, 0xa89e, 0xa8a8, 0xa8ce}
	dayOf := func() string {
		var sb strings.Builder
		for _, g := range segs {
			seg := o.Word(oracle.Addr{Seg: dgroup, Off: g})
			fmt.Fprintf(&sb, "ds:%#x→%#x:[0x2100]=%d ", g, seg,
				o.Word(oracle.Addr{Seg: seg, Off: 0x2100}))
		}
		return sb.String()
	}
	t.Logf("紮完寨：%s", dayOf())
	for step := 1; step <= 10; step++ {
		// **兩種輸入都試**：紮寨讀的是掃描碼（`PressScan` 有效），
		// 命令欄位可能讀字元（`Press`）。奇數步送掃描碼、偶數步送字元，
		// 哪一種讓天數動起來就是哪一種。
		send, kind := o.PressScan, "掃描碼"
		if step%2 == 0 {
			send, kind = o.Press, "字元"
		}
		for _, k := range []string{"0", "\r"} {
			o.Drain()
			send(k)
			if err := o.Run(60_000_000); err != nil {
				t.Fatalf("休息第 %d 步（%s %q）停止：%v", step, kind, k, err)
			}
		}
		var sb strings.Builder
		for _, sl := range slots {
			fmt.Fprintf(&sb, "[%d-%d 兵 %d 移 %d/%d] ", sl.army, sl.team,
				w16(sl.rec+unitSoldiers), w16(sl.rec+unitMove), w16(sl.rec+unitCap))
		}
		t.Logf("休息第 %2d 步（%s）：%s｜%s", step, kind, dayOf(), sb.String())
	}
	dumpScreen(t, o, "battle-rested")
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
		o.PressScan(strings.ReplaceAll(seg, enterMark, "\r"))
		if err := o.Run(settle); err != nil {
			t.Fatalf("送第 %d 段（%q）時停止：%v", i+1, seg, err)
		}
	}
	if err := o.Run(settle); err != nil {
		t.Fatalf("戰場畫面停止：%v", err)
	}
}
