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

// 賞賜金帛的對拍（`docs/mechanics/70-ai` §2.13.4）。
//
// 六個等級各有一支分派常式（`0xd41a` 起，間隔 `0x5c`），差別只有推進去的
// **加成**；效果那一支是共用的 `0xd302(人物槽, 加成)`：
//
//	金   ＝ min(本回合預算, 100)                  0xd327  AX ＝ 預算
//	效果 ＝ RND(加成 ÷ 2) ＋ 太守魅力 ÷ 3 ＋ 加成  0xd374  CX ＝ 效果
//	忠誠 ＝ min(忠誠 ＋ 效果 × 金 ÷ 100, 100)      0xd3b4  AX ＝ 還沒夾的新忠誠
//	花費 ＝ min(增幅 × 100 ÷ 效果, 100)            0xd3ed  AX ＝ 還沒夾的花費
//	                                              0xd409  AL ＝ 寫回去的忠誠
//
// 等級由**呼叫端**分辨（`push cs` ＋ `call rel16` ＝ 4 bytes，所以是
// 呼叫點 + 4）；加成本身是 `Arg(1)`，兩邊一起核對就同時驗到表與公式。
//
// **這一支的 0.01 是 float64**（`fmull DS:0xa604`），與徵兵那條的 float32
// 不同（`docs/mechanics/10-strategy` §5.2），所以整數除法就對得上。

// TestRewardGoldMatchesTheOriginal 讓電腦諸侯去賞金，逐次核對三條式子。
func TestRewardGoldMatchesTheOriginal(t *testing.T) {
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

	// **盤面自己擺**：開局的電腦諸侯 AI 等級只有 4 與 5，照劇本跑只驗得到
	// 六份常式裡的兩份。只數活著的槽（offset 0 ＝ 0xFFFF 的沒在用）。
	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), uint16(alive%6))
		alive++
	}
	t.Logf("活著的勢力 %d 個，AI 等級輪流設成 0–5", alive)

	// 六個呼叫端的返回位址 → 等級（`push cs` ＋ `call rel16` ＝ 4 bytes）。
	//
	// ⚠ **間隔不是固定的 `0x5c`**：等級 0–2 的加成是 0，推的是
	// `sub ax,ax; push ax`（3 bytes）；等級 3–5 推的是
	// `mov $imm,%ax; push ax`（4 bytes），所以後三支各往後移一格。
	// 照固定間隔算會讓等級 3–5 一次都對不上，而輸出看起來只是
	// 「這三個月剛好沒輪到那幾個等級」。
	callerLevel := map[uint32]int{
		0xd45c: 0, 0xd4b8: 1, 0xd514: 2,
		0xd571: 3, 0xd5cd: 4, 0xd629: 5,
	}

	type shot struct {
		level, bonus, gold, effect, charm  int
		oldLoyal, rawLoyal, rawCost, wrote int
	}
	var shots []shot
	var cur shot
	armed := false

	o.OnCall(addr(0x0d302), func(o *oracle.Oracle) {
		lvl, ok := callerLevel[o.Caller().Linear()]
		if !ok {
			armed = false
			return
		}
		cur = shot{level: lvl, bonus: int(int16(o.Arg(1))),
			oldLoyal: int(int8(o.Byte(addr(genBase + uint32(o.Arg(0))*30 + 16))))}
		armed = true
	})
	o.OnCall(addr(0x0d327), func(o *oracle.Oracle) {
		if !armed {
			return
		}
		cur.gold = int(int16(o.AX()))
		if cur.gold > game.MaxReward {
			cur.gold = game.MaxReward
		}
	})
	// ⚠ **太守的魅力要在 `0xd369` 收**：那一格的下一道指令是
	// `mov $0x3,%bl`，`BX` 從「30 × 太守槽」被改成除數 3。
	// 在 `0xd374` 才讀 BX 會讀到別人的欄位，而讀出來的數字看起來很正常。
	o.OnCall(addr(0x0d369), func(o *oracle.Oracle) {
		if armed {
			cur.charm = int(int8(o.AX() & 0xFF))
		}
	})
	o.OnCall(addr(0x0d374), func(o *oracle.Oracle) {
		if armed {
			cur.effect = int(int16(o.CX()))
		}
	})
	o.OnCall(addr(0x0d3b4), func(o *oracle.Oracle) {
		if armed {
			cur.rawLoyal = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0d3ed), func(o *oracle.Oracle) {
		if armed {
			cur.rawCost = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0d409), func(o *oracle.Oracle) {
		if !armed {
			return
		}
		cur.wrote = int(int8(o.AX() & 0xFF))
		shots = append(shots, cur)
		armed = false
	})

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	const settle = 40_000_000
	for m := 0; m < 3; m++ {
		// 金拉滿，電腦才有預算賞。
		for p := 1; p <= state.PrefectureCount; p++ {
			o.SetWord(addr(staBase+uint32(p*176+18)), 30000)
		}
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("三個月：亂數 %d 次（正對照）、賞金 %d 次", rnd, len(shots))
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if len(shots) == 0 {
		t.Fatal("三個月裡一次都沒賞金——金的盤面沒擺成功，或者呼叫端的返回位址對不上")
	}

	seen, bad := map[int]int{}, 0
	for _, s := range shots {
		seen[s.level]++
		if want := game.RewardBonus(s.level); want != s.bonus {
			t.Errorf("等級 %d：原版的加成是 %d，remake 的表說 %d",
				s.level, s.bonus, want)
			bad++
			continue
		}
		// 效果扣掉不擲骰的兩項，剩下的要落在 RND(加成 ÷ 2) 的值域裡。
		roll := s.effect - s.charm/3 - s.bonus
		n := s.bonus / 2
		if n < 1 {
			n = 1
		}
		if roll < 0 || roll >= n {
			t.Errorf("等級 %d：效果 %d、魅力 %d、加成 %d → 擲值 %d，"+
				"不在 RND(%d) 的值域裡", s.level, s.effect, s.charm, s.bonus, roll, n)
			bad++
			continue
		}
		if want := game.RewardEffect(s.charm, s.bonus, roll); want != s.effect {
			t.Errorf("等級 %d：原版的效果是 %d，remake 算 %d", s.level, s.effect, want)
			bad++
		}
		gain := game.RewardGain(s.effect, s.gold)
		if want := s.oldLoyal + gain; want != s.rawLoyal {
			t.Errorf("等級 %d：忠誠 %d、效果 %d、金 %d → 原版算出 %d，remake 算 %d",
				s.level, s.oldLoyal, s.effect, s.gold, s.rawLoyal, want)
			bad++
		}
		if want := clamp100(s.rawLoyal); want != s.wrote {
			t.Errorf("等級 %d：原版寫回 %d，夾上限之後應該是 %d", s.level, s.wrote, want)
			bad++
		}
		if want := game.RewardCost(s.effect, s.wrote-s.oldLoyal); want != clamp100(s.rawCost) {
			t.Errorf("等級 %d：增幅 %d、效果 %d → 原版收 %d，remake 算 %d",
				s.level, s.wrote-s.oldLoyal, s.effect, clamp100(s.rawCost), want)
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
	t.Logf("%d 次賞金，%d 項對不上", len(shots), bad)
	if len(seen) < 6 {
		t.Errorf("只走到 %d 個等級的常式，六個都要驗到才算數", len(seen))
	}
}

func clamp100(n int) int {
	if n > 100 {
		return 100
	}
	return n
}
