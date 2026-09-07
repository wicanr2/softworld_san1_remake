//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 購置武器的對拍（`docs/mechanics/70-ai` §2.13）。
//
// 六個等級全部 thunk 到同一支 `0xc168`（`0xc26c` 起，間隔 12），
// 所以這一支不分等級。走守軍清單，對每一位：
//
//	現有武器 ＝ trunc(武裝度 × 兵力 × 0.01)   0xc1df  AX ＝ 現有武器
//	缺口     ＝ 兵力 − 現有武器               0xc1e6  AX ＝ 缺口
//	缺口 ÷ 100 > 預算 → 缺口 ＝ 100 × 預算    0xc1f3  ES:0x3d16 ＝ 預算
//	缺口 <= 0 → 跳過
//	扣錢(缺口 ÷ 100)                          0xc211  堆疊頂 ＝ 金額
//	新武裝度 ＝ (武裝度 × 兵力 × 0.01 ＋ 缺口) ÷ 兵力 × 100
//	                                          0xc261  AX ＝ 新武裝度
//	                                          0xc18e  AL ＝ 寫回去的值
//
// ⚠ **這一支的 `0.01` 是 float64**（`fmull DS:0xa5c8`），與徵兵那條稀釋
// 的 float32 不同（`docs/mechanics/10-strategy` §5.2）。
//
// ⚠ **中間那個乘積沒有先截斷**：`fiadds` 把缺口加到**浮點**的乘積上，
// 除回去之前一直是小數。而且**沒有夾 100 的上限**，寫回去的是低位那個
// byte。remake 原本兩件事都做反了（先截斷、又夾上限）。

// TestArmsPurchaseMatchesTheOriginal 讓電腦諸侯去買武器，逐次核對。
func TestArmsPurchaseMatchesTheOriginal(t *testing.T) {
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
		slot, arms, men   int
		have, rawGap, gap int
		budget, paid      int
		newArms, wrote    int
	}
	var shots []shot
	var cur shot
	armed := false

	o.OnCall(addr(0x0c1df), func(o *oracle.Oracle) {
		si := uint32(o.BX())
		cur = shot{
			slot: int(si) / 30,
			arms: int(o.Byte(addr(genBase + si + 25))),
			men:  int(int16(o.Word(addr(genBase + si + 22)))),
			have: int(int16(o.AX())),
		}
		armed = true
	})
	o.OnCall(addr(0x0c1e6), func(o *oracle.Oracle) {
		if armed {
			cur.rawGap = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0c1f3), func(o *oracle.Oracle) {
		if armed {
			cur.budget = int(int16(o.Word(addr(uint32(o.ES())*16 + 0x3d16))))
		}
	})
	o.OnCall(addr(0x0c20d), func(o *oracle.Oracle) {
		if armed {
			cur.gap = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0c211), func(o *oracle.Oracle) {
		if armed {
			cur.paid = int(int16(o.StackWord(0)))
		}
	})
	o.OnCall(addr(0x0c261), func(o *oracle.Oracle) {
		if armed {
			cur.newArms = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0c18e), func(o *oracle.Oracle) {
		if !armed {
			return
		}
		cur.wrote = int(o.AX() & 0xFF)
		shots = append(shots, cur)
		armed = false
	})

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	const settle = 40_000_000
	for m := 0; m < 3; m++ {
		// 金拉滿、武裝度壓低，電腦才補得動。
		for p := 1; p <= state.PrefectureCount; p++ {
			o.SetWord(addr(staBase+uint32(p*176+18)), 20000)
		}
		for i := 0; i < 350; i++ {
			o.SetByte(addr(genBase+uint32(i*30+25)), uint8(20+i%60))
		}
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("三個月：亂數 %d 次（正對照）、買武器 %d 次", rnd, len(shots))
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if len(shots) == 0 {
		t.Fatal("三個月裡一次都沒買武器——盤面沒擺成功")
	}

	bad, bought := 0, 0
	for _, s := range shots {
		if s.men <= 0 {
			continue // 兵力 0 那一條走的是「武裝度歸零」，不比
		}
		if want := game.Weapons(s.arms, s.men); want != s.have {
			t.Errorf("槽 %d：武裝度 %d、兵 %d → 原版算出 %d 件武器，remake 算 %d",
				s.slot, s.arms, s.men, s.have, want)
			bad++
			continue
		}
		if want := s.men - s.have; want != s.rawGap {
			t.Errorf("槽 %d：缺口原版給 %d，兵 %d − 武器 %d ＝ %d",
				s.slot, s.rawGap, s.men, s.have, want)
			bad++
			continue
		}
		want := s.rawGap
		if want/game.ArmsPerGold > s.budget {
			want = game.ArmsPerGold * s.budget
		}
		if want != s.gap {
			t.Errorf("槽 %d：預算 %d、原始缺口 %d → 原版買 %d，照算式是 %d",
				s.slot, s.budget, s.rawGap, s.gap, want)
			bad++
			continue
		}
		if s.gap <= 0 {
			continue
		}
		bought++
		if w := s.gap / game.ArmsPerGold; w != s.paid {
			t.Errorf("槽 %d：買 %d 件原版付 %d 金，截斷除法給 %d",
				s.slot, s.gap, s.paid, w)
			bad++
		}
		if w := game.ArmsAfterPurchase(s.arms, s.men, s.gap); w != s.newArms {
			t.Errorf("槽 %d：武裝度 %d、兵 %d、買 %d → 原版算 %d，remake 算 %d",
				s.slot, s.arms, s.men, s.gap, s.newArms, w)
			bad++
		}
		if w := s.newArms & 0xFF; w != s.wrote {
			t.Errorf("槽 %d：原版算出 %d，寫回去卻是 %d", s.slot, s.newArms, s.wrote)
			bad++
		}
	}
	t.Logf("%d 次走到這支常式，其中 %d 次真的買了，%d 項對不上",
		len(shots), bought, bad)
}
