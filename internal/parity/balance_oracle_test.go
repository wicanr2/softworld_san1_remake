//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 調整兵力的對拍（`docs/mechanics/70-ai` §2.13.2，`0xc2c4`）。
//
// 六個等級全部 thunk 到同一支，所以不分等級。它把整郡的兵按帶兵上限
// 攤平，訓練度與武裝度拉到全郡的加權平均：
//
//	平均訓練 ＝ Σ trunc(訓練度 × 兵力 ÷ 100) × 100 ÷ 總兵力
//	份額     ＝ min(round(帶兵上限 × 總兵力 ÷ 總上限), 帶兵上限)
//
// 三個寫入點都在同一輪迴圈裡，而且**寫兵力是最後一個**，所以在
// `0xc4a6`（寫訓練度）那一刻讀到的兵力還是舊值——一輪迴圈跑完就收齊了
// 整組的舊資料，不必另外去讀原版的名單。
//
//	0xc4a6  AL ＝ 平均訓練，BX ＝ 30 × 槽
//	0xc4ae  AL ＝ 平均武裝
//	0xc4f4  AX ＝ 這一位分到的兵

// TestRedistributeMatchesTheOriginal 讓電腦諸侯調整兵力，逐郡核對。
func TestRedistributeMatchesTheOriginal(t *testing.T) {
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

	type one struct{ slot, oldMen, oldTrain, oldArms, rank, got int }
	type group struct {
		train, arms int
		who         []one
	}
	var groups []group
	var cur group

	flush := func() {
		if len(cur.who) > 0 {
			groups = append(groups, cur)
		}
		cur = group{}
	}
	o.OnCall(addr(0x0c2c4), func(*oracle.Oracle) { flush() })
	o.OnCall(addr(0x0c4a6), func(o *oracle.Oracle) {
		si := uint32(o.BX())
		cur.train = int(int8(o.AX() & 0xFF))
		cur.who = append(cur.who, one{
			slot:     int(si) / 30,
			oldMen:   int(int16(o.Word(addr(genBase + si + 22)))),
			oldTrain: int(int8(o.Byte(addr(genBase + si + 24)))),
			oldArms:  int(int8(o.Byte(addr(genBase + si + 25)))),
			rank:     int(o.Byte(addr(genBase + si + 12))),
		})
	})
	o.OnCall(addr(0x0c4ae), func(o *oracle.Oracle) {
		if len(cur.who) > 0 {
			cur.arms = int(int8(o.AX() & 0xFF))
		}
	})
	o.OnCall(addr(0x0c4f4), func(o *oracle.Oracle) {
		if len(cur.who) > 0 {
			cur.who[len(cur.who)-1].got = int(int16(o.AX()))
		}
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
	flush()

	t.Logf("兩個月：亂數 %d 次（正對照）、調整兵力 %d 郡次", rnd, len(groups))
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if len(groups) == 0 {
		t.Fatal("兩個月裡一次都沒調整兵力")
	}

	bad, people := 0, 0
	for _, g := range groups {
		total, capSum, wTrain, wArms := 0, 0, 0, 0
		for _, x := range g.who {
			cap := game.TroopCap(state.Rank(x.rank))
			total += x.oldMen
			capSum += cap
			wTrain += x.oldMen * x.oldTrain / 100
			wArms += x.oldMen * x.oldArms / 100
		}
		if total <= 0 || capSum <= 0 {
			continue
		}
		if want := wTrain * 100 / total; want != g.train {
			t.Errorf("%d 人一組：原版的平均訓練是 %d，remake 算 %d",
				len(g.who), g.train, want)
			bad++
		}
		if want := wArms * 100 / total; want != g.arms {
			t.Errorf("%d 人一組：原版的平均武裝是 %d，remake 算 %d",
				len(g.who), g.arms, want)
			bad++
		}
		for _, x := range g.who {
			people++
			cap := game.TroopCap(state.Rank(x.rank))
			n := game.TroopShare(cap, total, capSum)
			if n != x.got {
				t.Errorf("槽 %d（職位 %d，上限 %d）：總兵 %d、總上限 %d "+
					"→ 原版分到 %d，照算式是 %d",
					x.slot, x.rank, cap, total, capSum, x.got, n)
				bad++
			}
		}
	}
	t.Logf("%d 郡次、%d 人，%d 項對不上", len(groups), people, bad)
}
