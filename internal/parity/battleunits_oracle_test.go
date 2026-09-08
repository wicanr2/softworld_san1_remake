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
			gotSol, gotAbi, gotMov := w16(rec+unitSoldiers), w16(rec+unitAbility), w16(rec+unitMove)
			line := fmt.Sprintf("軍力 %d 隊伍 %d（%d 位將，格 %d,%d）",
				army, team, n, w16(rec+unitCol), w16(rec+unitRow))
			for _, x := range []struct {
				name      string
				orig, ours int
			}{
				{"兵士數", gotSol, sum},
				{"綜合能力", gotAbi, u.Ability()},
				{"移動力", gotMov, u.MovePoints()},
			} {
				checked++
				if x.orig != x.ours {
					bad++
					t.Errorf("%s：%s 原版 %d／remake %d", line, x.name, x.orig, x.ours)
				}
			}
			t.Logf("%s 兵 %d 綜合能力 %d 移動力 %d", line, gotSol, gotAbi, gotMov)
		}
	}
	if units == 0 {
		t.Fatal("一支部隊都沒讀到——部隊記錄的位置或工作區的段不對")
	}
	t.Logf("整編出 %d 支部隊，比了 %d 個欄位，對不上 %d 個", units, checked, bad)
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
