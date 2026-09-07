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

// 開倉賑民的對拍（`docs/mechanics/70-ai` §2.13.3）。
//
// 六個等級各一支分派常式（`0xc9fa` 起，間隔 `0x6a`），差別是門檻底
// （80／80／70／60／80／80）與除數（10／10／10／10／9／7）；效果那一支
// 是共用的 `0xc8f6(預算, 量)`：
//
//	量 <= 0 → 量 ＝ 5                  0xc935
//	扣錢(整份預算)                     0xc93d
//	每格 ＝ (人口 ÷ 100) ÷ 12          0xc962  AX ＝ 每格
//	原始增幅 ＝ 量 × 預算 ÷ 每格        0xc970  AX ＝ 原始增幅
//	增幅 ＝ min(原始增幅, 太守魅力 ÷ 2)
//	忠誠 ＝ min(忠誠 ＋ 增幅, 100)      0xc9f0  AL ＝ 寫回去的忠誠
//	**再扣一次** int(實際增幅 ÷ 量 × 每格)  0xc9d6
//
// 那第二次扣錢原本記著「碼是這樣寫的，還沒實測確認」（`L3`）——
// 這一支就是來測它的。
//
// ⚠ **`量 × 預算` 是 16 位元有號乘法**（`0xc96a` 的 `imulw` 之後接
// `cwd`，高位字被蓋掉），郡的金拉到上限時真的會溢位：
// 量 6 × 預算 6000 ＝ 36000 → −29536。忠誠那一格也**沒有下限夾**，
// 只有上限 100，寫回去時再截成一個 byte——所以會出現「賑民之後民心
// 變成 119」這種畫面。測試把這兩件事都照做，不繞開。

// TestReliefMatchesTheOriginal 讓電腦諸侯去賑民，逐次核對增幅與兩次扣錢。
func TestReliefMatchesTheOriginal(t *testing.T) {
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

	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), uint16(alive%6))
		alive++
	}
	t.Logf("活著的勢力 %d 個，AI 等級輪流設成 0–5", alive)

	// 六個呼叫端的返回位址 → 等級（`push cs` ＋ `call rel16` ＝ 4 bytes，
	// 六支的間隔是固定的 `0x6a`——門檻底與除數都是同長度的立即數）。
	callerLevel := map[uint32]int{}
	for k := 0; k < 6; k++ {
		callerLevel[uint32(0xca5f+k*0x6a)] = k
	}

	type shot struct {
		level, budget, rate, pref int
		charm, pop, price, per    int
		oldLoyal, rawGain, wrote  int
		paidFirst, paidSecond     int
	}
	var shots []shot
	var cur shot
	armed := false

	o.OnCall(addr(0x0c8f6), func(o *oracle.Oracle) {
		lvl, ok := callerLevel[o.Caller().Linear()]
		if !ok {
			armed = false
			return
		}
		cur = shot{level: lvl, budget: int(int16(o.Arg(0))), rate: int(int16(o.Arg(1)))}
		armed = true
	})
	// `0xc90d` 之後 BX ＝ 176 × 目前的郡。
	o.OnCall(addr(0x0c90f), func(o *oracle.Oracle) {
		if !armed {
			return
		}
		p := int(o.BX()) / 176
		if p < 1 || p > state.PrefectureCount {
			armed = false
			return
		}
		cur.pref = p
		cur.pop = int(o.Word(addr(staBase + uint32(p*176+14))))
		cur.price = int(o.Byte(addr(staBase + uint32(p*176+29))))
		cur.oldLoyal = int(int8(o.Byte(addr(staBase + uint32(p*176+26)))))
	})
	// ⚠ 太守的魅力要在 `0xc926` 收：下一道指令 `mov $0x2,%cl` 之後
	// `idiv %cl` 會把 AX 換成商，AL 就不是魅力了。
	o.OnCall(addr(0x0c926), func(o *oracle.Oracle) {
		if armed {
			cur.charm = int(int8(o.AX() & 0xFF))
		}
	})
	o.OnCall(addr(0x0c93d), func(o *oracle.Oracle) {
		if armed {
			cur.paidFirst = int(int16(o.StackWord(0)))
		}
	})
	o.OnCall(addr(0x0c962), func(o *oracle.Oracle) {
		if armed {
			cur.per = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0c970), func(o *oracle.Oracle) {
		if armed {
			cur.rawGain = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0c9d6), func(o *oracle.Oracle) {
		if armed {
			cur.paidSecond = int(int16(o.StackWord(0)))
		}
	})
	o.OnCall(addr(0x0c9f0), func(o *oracle.Oracle) {
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
		// 金拉滿、民眾忠誠壓低，六個等級的門檻才過得去。
		for p := 1; p <= state.PrefectureCount; p++ {
			o.SetWord(addr(staBase+uint32(p*176+18)), 30000)
			o.SetByte(addr(staBase+uint32(p*176+26)), 40)
		}
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("三個月：亂數 %d 次（正對照）、賑民 %d 次", rnd, len(shots))
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if len(shots) == 0 {
		t.Fatal("三個月裡一次都沒賑民——盤面沒擺成功，或者呼叫端的返回位址對不上")
	}

	seen, bad, second := map[int]int{}, 0, 0
	for _, s := range shots {
		seen[s.level]++
		if want := (100 - s.price) / game.ReliefRate(s.level); want != s.rate {
			t.Errorf("等級 %d：物價 %d，原版的量是 %d，remake 算 %d",
				s.level, s.price, s.rate, want)
			bad++
			continue
		}
		rate := s.rate
		if rate <= 0 {
			rate = game.ReliefMinRate
		}
		if want := s.pop / 12; want != s.per {
			t.Errorf("郡 %d：人口 %d00，原版的每格是 %d，remake 算 %d",
				s.pref, s.pop, s.per, want)
			bad++
			continue
		}
		// **乘法是 16 位元有號的**：高位字在 `cwd` 那一步被蓋掉。
		if want := int(int16(rate*s.budget)) / s.per; want != s.rawGain {
			t.Errorf("等級 %d 郡 %d：量 %d × 預算 %d ÷ 每格 %d → 原版算出 %d，"+
				"照 16 位元的算式是 %d",
				s.level, s.pref, rate, s.budget, s.per, s.rawGain, want)
			bad++
			continue
		}
		gain := s.rawGain
		if cap := s.charm / 2; gain > cap {
			gain = cap
		}
		// 忠誠只有上限夾，沒有下限；寫回去的是低位那個 byte。
		full := s.oldLoyal + gain
		if full > 100 {
			full = 100
		}
		if want := int(int8(uint8(full))); want != s.wrote {
			t.Errorf("等級 %d 郡 %d：忠誠 %d、量 %d、預算 %d、每格 %d、魅力 %d"+
				" → 原版寫 %d，照算式是 %d",
				s.level, s.pref, s.oldLoyal, rate, s.budget, s.per, s.charm,
				s.wrote, want)
			bad++
		}
		if s.paidFirst != s.budget {
			t.Errorf("等級 %d：第一次扣的是 %d，不是整份預算 %d",
				s.level, s.paidFirst, s.budget)
			bad++
		}
		// 第二次扣錢：照實際增幅反算（浮點，最後截成 16 位元）。
		got := full - s.oldLoyal
		if want := int(int16(got * s.per / rate)); want != s.paidSecond {
			t.Errorf("等級 %d：增幅 %d、量 %d、每格 %d → 原版第二次扣 %d，"+
				"照算式是 %d", s.level, got, rate, s.per, s.paidSecond, want)
			bad++
		}
		if s.paidSecond != 0 {
			second++
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
	t.Logf("%d 次賑民，%d 項對不上；第二次扣錢實際發生 %d 次",
		len(shots), bad, second)
	if len(seen) < 6 {
		t.Errorf("只走到 %d 個等級的常式，六個都要驗到才算數", len(seen))
	}
}
