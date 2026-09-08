//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 遠交近攻與聯合出兵走玩家的完整選單（`docs/mechanics/30-diplomacy` 的缺口）。
//
// 這兩支**我方要參戰**，所以戰役會進戰術層、按鍵要再接一整串整編。
// 但要驗的斷言只到 `0x20200` 的四個參數（`docs/re/07` §4）——那一支被
// 呼叫的那一刻參數就在堆疊上，整編還沒開始。所以測到那裡就夠，
// 不必把仗打完。
//
// 選郡的提示有幾個、順序如何，靠**候選表重建**當訊號：每一次要玩家選郡
// 之前，`0x2c3e7` 會把段 `[0xa9e4]` 的 `0x2102` 起 43 個 word 重寫一遍
// （`docs/re/07` §2）。表重建了就代表下一個提示是選郡，沒重建就是別的
// 提示（使者、Y/N）——**不靠數提示的個數**。
func TestBattlePlotsRunLive(t *testing.T) {
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
	at, _ := stageABattle(t, o, base)

	staBase := base + uint32(state.MasterTableSize)
	genBase := staBase + uint32(state.PrefectureTableSize)
	const staRec = state.PrefectureRecordSize

	mine := int(o.Byte(addr(staBase + uint32(at)*staRec + 30)))
	lord := o.Word(addr(base + uint32(mine)*72 + 2))
	o.SetWord(addr(base+uint32(mine)*72+6), lord)
	o.SetByte(addr(genBase+uint32(lord)*30+9), 100)
	for i := 0; i < 16; i++ {
		if i == mine {
			continue
		}
		for _, off := range []uint32{2, 6} {
			if who := o.Word(addr(base + uint32(i)*72 + off)); who < 350 {
				o.SetByte(addr(genBase+uint32(who)*30+9), 30)
			}
		}
	}
	t.Logf("我方勢力 %d、君主槽 %d 坐鎮郡 %d；其餘勢力的謀略壓到 30",
		mine, lord, at)

	// **聯合出兵至少要兩個我方的郡**：第三個提示問「聯合我方那一郡合攻」，
	// 條件是「攻打目標的鄰郡、屬我方、不是出兵的那一郡」，一個都挑不到時
	// 原版印 `DS:0x8543`「無法聯合出兵」退回選單（`docs/re/07` §2）。
	// `stageABattle` 只給玩家一個郡，所以再撥一個——挑目標郡的鄰郡，
	// 換旗並放一位在職將進去。
	tgt := 0
	for k := 45; k <= 54; k++ {
		n := int(o.Byte(addr(staBase + uint32(at)*staRec + uint32(k))))
		if n >= 1 && n <= 42 && int(o.Byte(addr(staBase+uint32(n)*staRec+30))) != mine {
			tgt = n
			break
		}
	}
	second := 0
	if tgt > 0 {
		for k := 45; k <= 54; k++ {
			n := int(o.Byte(addr(staBase + uint32(tgt)*staRec + uint32(k))))
			if n >= 1 && n <= 42 && n != at {
				second = n
				break
			}
		}
	}
	if second == 0 {
		t.Fatalf("郡 %d 的鄰郡 %d 旁邊找不到第二個郡可以撥給玩家", at, tgt)
	}
	o.SetByte(addr(staBase+uint32(second)*staRec+30), byte(mine))
	placed := 0
	for i := 0; i < 350 && placed < 1; i++ {
		g := genBase + uint32(i)*30
		if o.Byte(addr(g+18)) != 0xFF {
			continue
		}
		o.SetByte(addr(g+18), byte(mine))
		o.SetByte(addr(g+19), byte(second))
		o.SetByte(addr(g+17), 2) // 太守
		o.SetByte(addr(g+12), 2)
		o.SetWord(addr(g+22), 2000)
		o.SetByte(addr(g+24), 80)
		o.SetByte(addr(g+25), 80)
		o.SetByte(addr(g+9), 90)
		placed++
	}
	o.SetWord(addr(staBase+uint32(second)*staRec+16), 20)
	o.SetWord(addr(staBase+uint32(second)*staRec+18), 5000)
	o.SetWord(addr(staBase+uint32(second)*staRec+20), 8000)
	o.SetByte(addr(staBase+uint32(second)*staRec+22), byte(placed))
	t.Logf("再撥郡 %d 給玩家（%d 位守將），它與郡 %d 相鄰", second, placed, tgt)

	var dgroup uint16
	o.OnCall(addr(0x2c2ee), func(o *oracle.Oracle) {
		if dgroup == 0 {
			dgroup = o.DSReg()
		}
	})
	rebuilt := 0
	o.OnCall(addr(0x2c3e7), func(*oracle.Oracle) { rebuilt++ })
	type battleCall struct{ a, aHelp, d, dHelp int }
	var battles []battleCall
	o.OnCall(addr(0x20200), func(o *oracle.Oracle) {
		if len(battles) < 4 {
			battles = append(battles, battleCall{
				int(o.Arg(0)), int(o.Arg(1)), int(o.Arg(2)), int(o.Arg(3))})
		}
	})

	send := func(t *testing.T, tag, keys string) {
		t.Helper()
		o.Drain()
		o.PressScan(strings.ReplaceAll(keys, "⏎", "\r"))
		if err := o.Run(150_000_000); err != nil {
			t.Fatalf("%s 送 %q 停止：%v", tag, keys, err)
		}
	}
	candidates := func(t *testing.T) []int {
		t.Helper()
		if dgroup == 0 {
			t.Fatal("還沒進到分派器 0x2c2ee，取不到 DGROUP")
		}
		seg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9e4})
		var out []int
		for id := 1; id <= 42; id++ {
			if o.Word(oracle.Addr{Seg: seg, Off: uint16(0x2102 + id*2)}) == 0 {
				out = append(out, id)
			}
		}
		return out
	}
	owner := func(id int) int {
		return int(o.Byte(addr(staBase + uint32(id)*staRec + 30)))
	}

	atMenu := o.Save()

	for _, tc := range []struct {
		name, key string
		envoy     bool
		rounds    int
	}{
		{"遠交近攻", "2", true, 3},
		{"聯合出兵", "5", false, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o.Restore(atMenu)
			battles = battles[:0]
			send(t, tc.name, "8⏎")
			send(t, tc.name, tc.key+"⏎")

			// **選郡的提示個數照 `docs/re/07` §2**：遠交近攻三個
			// （出使／聯合攻打／聯合我方），聯合出兵三或四個（出兵／
			// 聯合攻打／我方合攻／守方合守，最後一個只在守方求得到
			// 援軍時才問）。
			//
			// ⚠ **不要拿 `0x2c3e7` 當「下一個提示是選郡」的訊號**：
			// 那支只服務第一張表，第二張以後各有各的迴圈位址，
			// 量到過一次——第一輪之後 `rebuilt` 就不再動，
			// 迴圈提早收掉而看起來像「原版沒有再問」。
			var picks []int
			for round := 1; round <= tc.rounds; round++ {
				if len(battles) > 0 {
					break
				}
				cand := candidates(t)
				if len(cand) == 0 {
					t.Logf("第 %d 個選郡提示的候選表是空的，停在這裡", round)
					break
				}
				// 挑一個之後下一張表不能是空的——空了就換下一個候選。
				mark := o.Save()
				picked := 0
				for _, id := range cand {
					o.Restore(mark)
					send(t, tc.name, fmt.Sprintf("%d⏎", id))
					if len(battles) > 0 || len(candidates(t)) > 0 {
						picked = id
						break
					}
				}
				if picked == 0 {
					t.Fatalf("第 %d 個提示的每一個候選都讓下一張表變空", round)
				}
				picks = append(picks, picked)
				t.Logf("第 %d 個選郡提示：候選 %d 個 %v，挑 %d（所屬勢力 %d）",
					round, len(cand), cand, picked, owner(picked))
			}
			t.Logf("%s 選的郡：%v（我方是勢力 %d）", tc.name, picks, mine)

			if tc.envoy {
				send(t, tc.name, "1⏎") // 使者
			}
			send(t, tc.name, "Y") // 軍師預測之後的確認／出兵確認

			if len(battles) == 0 {
				t.Fatalf("0x20200 沒跑到——選了 %v 之後沒有發動戰役", picks)
			}
			b := battles[0]
			t.Logf("%s：戰役 (%d, %#x, %d, %#x)", tc.name, b.a, b.aHelp, b.d, b.dHelp)

			// 兩支都要有我方的郡在場——這正是它們與驅虎吞狼的差別。
			ours := 0
			for _, id := range []int{b.a, b.aHelp, b.d, b.dHelp} {
				if id >= 1 && id <= 42 && owner(id) == mine {
					ours++
				}
			}
			if ours == 0 {
				t.Errorf("%s 我方要參戰，四個郡裡卻沒有一個是我方的", tc.name)
			}
			if len(picks) > 0 && b.d != picks[1] && b.d != picks[0] {
				t.Errorf("被打的郡是 %d，不在挑的 %v 裡", b.d, picks)
			}
		})
	}
}
