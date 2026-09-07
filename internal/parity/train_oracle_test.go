//go:build oracle

package parity

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 訓練兵士的對拍（`docs/mechanics/70-ai` §2.1、`game.TrainGain`）。
//
// 六個等級各一支 thunk（`0xbe30` 起，間隔 `0x14`），推一個除數再呼叫
// 共用常式 `0xbd70`。**這一支無條件做**，所以樣本很多。
//
//	增幅 ＝ (謀略 ÷ 3 ＋ 戰力 ÷ 2) ÷ 除數[等級]   ; 逐項截斷
//	訓練度 ＝ min(訓練度 ＋ 增幅, 100)             0xbded  AL ＝ 寫回去的值
//
// 除數 5／5／5／4／4／3，由呼叫端推進來（`Arg(0)`），等級由返回位址分辨。

// TestTrainingMatchesTheOriginal 讓電腦諸侯練兵，逐次核對增幅與除數表。
func TestTrainingMatchesTheOriginal(t *testing.T) {
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
	genBase := base + uint32(state.MasterTableSize) + uint32(state.PrefectureTableSize)

	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), uint16(alive%6))
		alive++
	}
	t.Logf("活著的勢力 %d 個，AI 等級輪流設成 0–5", alive)

	// 六個 thunk 的返回位址 → 等級（`push cs` ＋ `call rel16` ＝ 4 bytes）。
	callerLevel := map[uint32]int{}
	for k := 0; k < 6; k++ {
		callerLevel[uint32(0xbe3f+k*0x14)] = k
	}

	type shot struct{ level, div, intel, war, old, wrote int }
	var shots []shot
	cur := struct {
		level, div int
		ok         bool
	}{}

	o.OnCall(addr(0x0bd70), func(o *oracle.Oracle) {
		lvl, ok := callerLevel[o.Caller().Linear()]
		cur.ok = ok
		if !ok {
			return
		}
		cur.level, cur.div = lvl, int(int16(o.Arg(0)))
	})
	o.OnCall(addr(0x0bded), func(o *oracle.Oracle) {
		if !cur.ok {
			return
		}
		si := uint32(o.BX())
		shots = append(shots, shot{
			level: cur.level, div: cur.div,
			intel: int(o.Byte(addr(genBase + si + 9))),
			war:   int(o.Byte(addr(genBase + si + 10))),
			old:   int(int8(o.Byte(addr(genBase + si + 24)))),
			wrote: int(int8(o.AX() & 0xFF)),
		})
	})

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	const settle = 40_000_000
	for m := 0; m < 2; m++ {
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("兩個月：亂數 %d 次（正對照）、練兵 %d 次", rnd, len(shots))
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if len(shots) == 0 {
		t.Fatal("兩個月裡一次都沒練兵——呼叫端的返回位址對不上")
	}

	seen, bad := map[int]int{}, 0
	for _, s := range shots {
		seen[s.level]++
		if want := game.AITrainDivisor(s.level); want != s.div {
			t.Errorf("等級 %d：原版推的除數是 %d，remake 的表說 %d",
				s.level, s.div, want)
			bad++
			continue
		}
		want := s.old + game.TrainGain(s.intel, s.war, s.level)
		if want > 100 {
			want = 100
		}
		if want != s.wrote {
			t.Errorf("等級 %d：訓練 %d、謀略 %d、戰力 %d → 原版寫 %d，remake 算 %d",
				s.level, s.old, s.intel, s.war, s.wrote, want)
			bad++
		}
	}
	levels := make([]int, 0, len(seen))
	for k := range seen {
		levels = append(levels, k)
	}
	sort.Ints(levels)
	for _, k := range levels {
		t.Logf("等級 %d：%d 次", k, seen[k])
	}
	t.Logf("%d 次練兵，%d 項對不上", len(shots), bad)
	if len(seen) < 6 {
		t.Errorf("只走到 %d 個等級的 thunk，六個都要驗到才算數", len(seen))
	}
}
