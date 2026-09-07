//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 指定軍師的對拍（表 `0x5694`，常式 `0xd7ae`）。
//
// 六個等級全部 thunk 到同一支，不分等級：
//
//	諸侯 offset 0 == 1（玩家操縱）→ 不做
//	門檻 ＝ 現任軍師的謀略；沒有軍師就是 0x4f（79）
//	掃名單：謀略 > 門檻 且 身分 ∈ {2, 3} → 記下這個人
//	諸侯 offset 6 ← 記下的那個人                0xd8ab  AX ＝ 新軍師
//
// ⚠ **門檻在迴圈裡不更新**（`0xd85d` 比的一直是 `[-4]`），所以挑到的是
// **名單裡最後一個**超過門檻的人，不是謀略最高的那一個。
// 兩者在多數盤面上結果相同——只有「名單裡有兩個以上超過門檻」時才分得開，
// 而那正是這一支要驗的。
//
//	0xd824  AX ＝ 現任軍師的謀略（門檻）
//	0xd82a  沒有軍師 → 門檻 79
//	0xd875  AX ＝ **通過兩道門的**候選人槽號（依序）
//
// ⚠ **判準要在掃描當下收，不能事後回讀狀態**：常式在寫回諸侯 offset 6
// 之後會把新軍師的身分改掉（`0xd8c2` 一帶），事後再去看他的身分就不是
// 2 或 3 了——照事後狀態重算會把選中的那個人濾掉，而錯誤訊息看起來像
// 「原版挑錯人」。

// TestAppointChiefMatchesTheOriginal 核對電腦挑軍師挑的是誰。
func TestAppointChiefMatchesTheOriginal(t *testing.T) {
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
	staBase := base + uint32(state.MasterTableSize)
	genBase := staBase + uint32(state.PrefectureTableSize)

	type shot struct {
		bar, chosen int
		accepted    []int
	}
	var shots []shot
	var cur shot
	armed := false

	o.OnCall(addr(0x0d7ae), func(*oracle.Oracle) {
		cur = shot{bar: -1, chosen: -1}
		armed = true
	})
	o.OnCall(addr(0x0d824), func(o *oracle.Oracle) {
		if armed {
			cur.bar = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0d82a), func(*oracle.Oracle) {
		if armed {
			cur.bar = 0x4f
		}
	})
	o.OnCall(addr(0x0d875), func(o *oracle.Oracle) {
		if armed {
			cur.accepted = append(cur.accepted, int(int16(o.AX())))
		}
	})
	o.OnCall(addr(0x0d8ab), func(o *oracle.Oracle) {
		if !armed {
			return
		}
		cur.chosen = int(int16(o.AX()))
		shots = append(shots, cur)
		armed = false
	})

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	bad, checked, contested := 0, 0, 0
	check := func() {
		for _, s := range shots {
			if s.bar < 0 {
				continue
			}
			checked++
			if len(s.accepted) == 0 {
				continue // 沒人合格 → 維持原樣
			}
			if len(s.accepted) > 1 {
				contested++
			}
			if want := s.accepted[len(s.accepted)-1]; want != s.chosen {
				t.Errorf("門檻 %d、通過的有 %v → 原版選了槽 %d，"+
					"「最後一個」是槽 %d", s.bar, s.accepted, s.chosen, want)
				bad++
			}
		}
		shots = shots[:0]
	}

	const settle = 40_000_000
	for m := 0; m < 2; m++ {
		// **讓名單裡有兩個以上合格**，「最後一個」與「最高的」才分得開：
		// 在職者的謀略設在 80–99。
		for i := 0; i < 350; i++ {
			at := genBase + uint32(i*30)
			if r := o.Byte(addr(at + 17)); r != 2 && r != 3 {
				continue
			}
			o.SetByte(addr(at+9), uint8(80+i%20))
		}
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
			check()
		}
	}

	t.Logf("兩個月：亂數 %d 次（正對照）、指定軍師 %d 次（其中 %d 次名單裡"+
		"不只一個合格），%d 項對不上", rnd, checked, contested, bad)
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if checked == 0 {
		t.Fatal("兩個月裡一次都沒指定軍師")
	}
	if contested == 0 {
		t.Error("沒有一次名單裡有兩個以上合格——「最後一個」與「最高的」" +
			"分不開，這一支等於沒驗到")
	}
}
