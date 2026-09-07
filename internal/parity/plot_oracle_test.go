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
// ⚠ **這一支目前跑不出樣本，會 skip。** 四輪都沒收到一次得手，
// 分層計數把範圍縮到這裡：
//
//	等級 3–5 的計略常式被選中          182 次
//	  RND(10/8/5) != 0                 擋掉大部分
//	  諸侯 offset 6 ＝ 0xFFFF          （測試已備好軍師）
//	  人物[軍師].領地 != 目前的郡      （軍師 ← 君主，一定在某個郡）
//	走到 0xe608 的那道門                11 次，AX 恆為 6 → **過得去**
//	得手（0x2d6e0）                     0 次
//
// 所以卡在 `0xe610` 之後、`0x2d6e0` 之前：挑目標（`0xe7aa`）、
// 選使者、成敗判定（`0x2dd66`）那一段還沒逐支追。
//
// 走過的兩條岔路留在這裡免得重走：`0xe672` 是空殼沒錯，但它的 `AX`
// 是 6 不是 0，那道門不是關卡；把**所有**電腦君主的謀略設成 100 也不行
// ——成敗是 `我方 > 對方` 的硬碰硬，雙方都 100 就永遠打平。
//
// **效果那一側不受影響**：五刀的公式是 `L0`，`0x2d6e0` 的兩個呼叫端
// （玩家的策反人民、電腦的計略）都傳人物 offset 11。這一支要驗的是
// 「原版當場算出來的量」，不是公式本身有沒有讀對。
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
		// ⚠ **不能全部設成 100。** 成敗是 `我方 > 對方` 的硬碰硬，
		// 而目標是全圖隨機挑的——雙方都 100 就永遠打平，一次也不會得手。
		// 一半當攻方（謀略 100）、一半當箭靶（謀略 30）。
		if armed%2 == 0 {
			o.SetByte(addr(genBase+uint32(lord*30+9)), 100)
		} else {
			o.SetByte(addr(genBase+uint32(lord*30+9)), 30)
		}
		armed++
	}
	t.Logf("替 %d 個電腦勢力備好等級與軍師，謀略一半 100 一半 30", armed)

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	// 分層診斷：常式有沒有被選中、三道門各擋掉多少、空殼那一道回什麼。
	entered, gateAX := 0, map[uint16]int{}
	for k, at := range map[int]uint32{3: 0x0e4a2, 4: 0x0e522, 5: 0x0e5a2} {
		_ = k
		o.OnCall(addr(at), func(*oracle.Oracle) { entered++ })
	}
	o.OnCall(addr(0x0e608), func(o *oracle.Oracle) { gateAX[o.AX()]++ })

	// **逐關計數**：前四輪只數頭尾，看不出斷在哪。整條鏈是
	//
	//	0xe5a9 RND(5)==0        → 0xe5dd 有軍師 → 0xe5fd 軍師在本郡
	//	0xe605 空殼（AX=6）     → 0xe610 建候選表 → 0xe618 挑目標並執行
	//	  0xe836 抽中目標 → 0xe853 選使者 → 0xe892 成敗判定
	//	  0xe89a AX==0 才動手 → 0xe8ba 五刀（0x2d6e0）
	picked, rolled := 0, 0
	rollAX := map[uint16]int{}
	var lastMine, lastTarget, lastCharm int
	o.OnCall(addr(0x0e836), func(*oracle.Oracle) { picked++ })
	o.OnCall(addr(0x0e892), func(o *oracle.Oracle) {
		rolled++
		lastMine, lastTarget, lastCharm =
			int(o.StackWord(0)), int(o.StackWord(1)), int(o.StackWord(2))
	})
	o.OnCall(addr(0x0e89a), func(o *oracle.Oracle) {
		rollAX[o.AX()]++
		if o.AX() != 0 {
			t.Logf("成敗判定：我方郡 %d 打目標 %d，使者魅力 %d，回 %d（非 0 ＝ 沒得手）",
				lastMine, lastTarget, lastCharm, o.AX())
		}
	})

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

	t.Logf("逐關：抽中目標 %d 次、成敗判定 %d 次（回傳分布 %v）",
		picked, rolled, rollAX)
	t.Logf("六個月：亂數 %d 次、計略常式進去 %d 次、"+
		"空殼那道門的 AX 分布 %v、計略得手 %d 次、五刀共 %d 次，%d 次對不上",
		rnd, entered, gateAX, calls, cuts, bad)
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if calls == 0 {
		t.Skip("六個月裡電腦一次計略都沒得手——見檔頭，卡在 0xe610 之後那一段")
	}
}
