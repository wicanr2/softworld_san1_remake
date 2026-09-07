//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 計略得手之後那五刀的對拍（`docs/mechanics/30-diplomacy` §1，`0x2d6e0`）。
//
// 五刀的量全部跟著使者的魅力走，而且每一刀各擲一次：
//
//	忠誠     −RND(魅力 ÷ 10)     0x2d70c  sub cl
//	洪水率   ＋RND(魅力 ÷ 5)      0x2d739  add cl
//	土地價值 −RND(魅力 ÷ 12)     0x2d76c  sub cl
//	米       −米 × 100 ÷ (RND(魅力) + 300)   0x2d7b4  sub ax
//	金       −金 × 100 ÷ (RND(魅力) + 500)   0x2d7ea  sub ax
//
// 判準是「**存在**一個合法的擲值算得出原版扣掉的那個量」——直接列舉。
// 前三刀的量就是 CL，後兩刀是 AX。魅力從常式入口的 `Arg(1)` 拿。

// TestSabotageMatchesTheOriginal 讓電腦諸侯去用計，核對那五刀。
func TestSabotageMatchesTheOriginal(t *testing.T) {
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

	// **盤面自己擺**，三道門一次備齊：
	//
	//   - 計略是等級 3 才開的四種行為之一（`docs/mechanics/70-ai` §2.1）
	//     → 電腦勢力全部拉到等級 5
	//   - 沒有軍師不能用計，而且**軍師本人要在該郡**（`0x56f4`）
	//     → 軍師欄直接指向君主本人，君主一定在某個郡
	//   - 成敗是雙方謀略的硬碰硬（`PlotScore`）
	//     → 君主的謀略拉到 100
	//
	// 開局多數勢力沒有軍師，所以第一輪六個月一次計略都沒得手。
	genBase := base + uint32(state.MasterTableSize) + uint32(state.PrefectureTableSize)
	armed := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) != 2 {
			continue
		}
		lord := int(o.Word(addr(base + uint32(i*72+2))))
		if lord < 0 || lord >= 350 {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), 5)            // AI 等級
		o.SetWord(addr(base+uint32(i*72+6)), uint16(lord)) // 軍師 ← 君主
		o.SetByte(addr(genBase+uint32(lord*30+9)), 100)    // 謀略
		armed++
	}
	t.Logf("替 %d 個電腦勢力備好等級、軍師與謀略", armed)

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	charm, target := 0, 0
	calls := 0
	o.OnCall(addr(0x2d6e0), func(o *oracle.Oracle) {
		target, charm = int(o.Arg(0)), int(o.Arg(1))
		calls++
	})

	// canRoll 回報「有沒有一個 0..n−1 的擲值等於 got」。
	canRoll := func(n, got int) bool {
		if n < 1 {
			n = 1
		}
		return got >= 0 && got < n
	}

	bad, cuts := 0, 0
	byteCut := func(name string, div int, field int, add bool) func(*oracle.Oracle) {
		return func(o *oracle.Oracle) {
			if calls == 0 {
				return
			}
			cuts++
			got := int(o.CX() & 0xFF)
			if !canRoll(charm/div, got) {
				t.Errorf("郡 %d 魅力 %d 的%s：扣 %d，RND(%d) 給不出來",
					target, charm, name, got, charm/div)
				bad++
			}
		}
	}
	o.OnCall(addr(0x2d70c), byteCut("忠誠", game.SabotageLoyaltyDiv, 26, false))
	o.OnCall(addr(0x2d739), byteCut("洪水率", game.SabotageFloodDiv, 28, true))
	o.OnCall(addr(0x2d76c), byteCut("土地價值", game.SabotageLandDiv, 27, false))

	wordCut := func(name string, field, base2 int) func(*oracle.Oracle) {
		return func(o *oracle.Oracle) {
			if calls == 0 || target < 1 || target > state.PrefectureCount {
				return
			}
			cuts++
			old := int(o.Word(addr(staBase + uint32(target*176+field))))
			got := int(o.AX())
			ok := false
			for r := 0; r < max(charm, 1); r++ {
				if old*game.SabotageScale/(r+base2) == got {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("郡 %d 魅力 %d 的%s：從 %d 扣掉 %d，"+
					"沒有一個 RND(%d)+%d 算得出來", target, charm, name, old, got,
					charm, base2)
				bad++
			}
		}
	}
	o.OnCall(addr(0x2d7b4), wordCut("米", 20, game.SabotageRiceBase))
	o.OnCall(addr(0x2d7ea), wordCut("金", 18, game.SabotageGoldBase))

	const settle = 40_000_000
	for m := 0; m < 6; m++ {
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("六個月：亂數 %d 次、計略得手 %d 次、五刀共 %d 次，%d 次對不上",
		rnd, calls, cuts, bad)
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if calls == 0 {
		t.Skip("六個月裡電腦一次計略都沒得手——等級或軍師的條件沒滿足")
	}
}
