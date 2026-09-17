//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 電腦諸侯用計的對拍（`docs/mechanics/70-ai` §2.13.7）。
//
// 整條鏈在 `0xe68e`：建候選郡表 → `RND(長度)` 抽一個 → `0xf17:0x0aae`
// ＋ `0xf17:0x03b0` 挑使者 → `0x2c21:0x1b56`（＝ `0x2dd66`）判成敗 →
// 回 0 就 `0x2c21:0x0fea`（＝ `0x2d1fa`，偽書使疑）。
//
// 五種計謀裡**電腦只用偽書使疑**。`0xe68e` 尾巴是 `sub ax,ax; lret`，
// 固定回 0，所以 `0xe616` 那道 `je` 永遠成立，`0xe618` 的 `call 0xe7aa`
// 一次都執行不到——`0xe7aa` 與 `0xe68e` 前 0x111 個位元組逐位元組相同，
// 只有效果那一個 far call 不同（`0x14d0` ＝ `0x2d6e0` 策反五刀）。
//
// 效果的公式（`0x2d1fa`，`internal/game.Forgery`）：
//
//	名單 ＝ 0xf17:0x0aae(目標郡, 2)      ; 筆數在段 [0xa9e6] 的 0x0c
//	忠誠(offset 16) >= 使者魅力 → 跳過
//	忠誠 ← 忠誠 × (250 − RND(魅力 ÷ 2) − 魅力) ÷ 250
//	算成負數 → 歸零
//
// 判準是「**存在**一個合法的擲值算得出原版寫回去的那個數」——魅力 ÷ 2
// 的值域小，直接列舉。原版寫回去的點是 `0x2d26d`，`AL` 是新值、
// `SI` 是 `30 × 槽號`，舊值還在記憶體裡沒被蓋掉。

// TestAIPlotMatchesTheOriginal 讓電腦諸侯去用計，核對偽書使疑那條乘法。
func TestAIPlotMatchesTheOriginal(t *testing.T) {
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

	// **盤面自己擺**，三道門一次備齊：
	//
	//   - 計略是等級 3 才開的四種行為之一（`docs/mechanics/70-ai` §2.1）
	//     → 電腦勢力全部拉到等級 5
	//   - 沒有軍師不能用計，而且**軍師本人要在該郡**（`0x56f4`）
	//     → 軍師欄直接指向君主本人，君主一定在某個郡
	//   - 成敗是雙方謀略的硬碰硬（`PlotScore`）
	//     → 一半當攻方（謀略 100）、一半當箭靶（謀略 30）。兩邊都 100
	//       就永遠打平，一次也不會得手。
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
		if armed%2 == 0 {
			o.SetByte(addr(genBase+uint32(lord*30+9)), 100)
		} else {
			o.SetByte(addr(genBase+uint32(lord*30+9)), 30)
		}
		armed++
	}
	t.Logf("替 %d 個電腦勢力備好等級與軍師，謀略一半 100 一半 30", armed)

	// **第四道門也要自己擺**：效果只動「忠誠 < 使者魅力」的人，而劇本
	// 001 的忠誠幾乎都在 80 以上、使者魅力多半更低——上一輪五次得手
	// 一個人都沒動到，輸出看起來與「效果常式沒被呼叫」很像。
	//
	// 把在職者的忠誠壓到 20–39（**跳過 0xFF 那個哨兵**，那是在野者的
	// 「沒有忠誠」，改掉會讓別的常式把他們當成在職）。留一段跨度是為了
	// 讓乘法的截斷有機會出現不同的餘數。
	//
	// **開局壓一次不夠，效果常式入口還要再壓一次。** 等級 5 的電腦每個月
	// 都賞賜金帛（`0xd409` 寫回忠誠，`docs/mechanics/20` §4.3，加成 40），
	// 一個月就把 20–39 賞回 90–100。偽書使疑哪個月得手取決於開機停點之後的
	// 亂數路徑：停點一換（`1418b51`），得手從第一個月移到第二個月，中間正好
	// 隔一次賞賜，效果常式就一個人都動不到（Issue #86）。入口那一刻名單還沒建、
	// 忠誠還沒讀，在這裡擺盤不影響要驗的那條乘法。
	lower := func() int {
		n := 0
		for i := 0; i < 350; i++ {
			at := addr(genBase + uint32(i*30+16))
			if o.Byte(at) == 0xFF {
				continue
			}
			o.SetByte(at, uint8(20+i%20))
			n++
		}
		return n
	}
	t.Logf("把 %d 個在職者的忠誠壓到 20–39", lower())

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	// 逐關計數：常式被選中、三道門、建表的兩條出口、成敗判定的回傳值。
	entered := 0
	for _, at := range []uint32{0x0e4a2, 0x0e522, 0x0e5a2} {
		o.OnCall(addr(at), func(*oracle.Oracle) { entered++ })
	}
	empty, picked := 0, 0
	o.OnCall(addr(0x0e712), func(*oracle.Oracle) { empty++ })
	o.OnCall(addr(0x0e71a), func(*oracle.Oracle) { picked++ })

	rollAX := map[uint16]int{}
	o.OnCall(addr(0x0e77e), func(o *oracle.Oracle) { rollAX[o.AX()]++ })

	// 效果常式的入口：目標郡與使者魅力。
	charm, target, calls := 0, 0, 0
	o.OnCall(addr(0x2d1fa), func(o *oracle.Oracle) {
		target, charm = int(o.Arg(0)), int(o.Arg(1))
		calls++
		lower()
	})

	// 寫回忠誠的那一刻：`AL` 是新值，舊值還在 `genBase + SI + 16`。
	bad, hits := 0, 0
	o.OnCall(addr(0x2d26d), func(o *oracle.Oracle) {
		if calls == 0 {
			return
		}
		hits++
		si := uint32(o.SI())
		old := int(int8(o.Byte(addr(genBase + si + 16))))
		got := int(int8(o.AX() & 0xFF))
		if old >= charm {
			t.Errorf("郡 %d 魅力 %d：忠誠 %d 不該被動到（門檻是 < 魅力）",
				target, charm, old)
			bad++
			return
		}
		n := max(charm/2, 1)
		ok := false
		for r := 0; r < n; r++ {
			if old*(game.ForgeryScale-r-charm)/game.ForgeryScale == got {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("郡 %d 魅力 %d：忠誠 %d → %d，"+
				"沒有一個 RND(%d) 算得出來（分母 %d）",
				target, charm, old, got, n, game.ForgeryScale)
			bad++
		}
	})

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

	t.Logf("六個月：亂數 %d 次、計略常式進去 %d 次、"+
		"候選表 空 %d 次／抽中 %d 次、成敗判定回傳分布 %v",
		rnd, entered, empty, picked, rollAX)
	t.Logf("得手 %d 次，動到 %d 個人的忠誠，%d 次對不上", calls, hits, bad)
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if picked == 0 {
		t.Fatal("六個月裡候選表一次都沒抽中：等級或軍師的盤面沒擺成功")
	}
	// **skip 不是綠**：盤面四道門都自己擺過了，模擬器又是決定性的，
	// 所以這裡沒有樣本就是壞了，不是運氣。
	if calls == 0 {
		t.Fatal("抽中了但成敗判定一次都沒過——謀略的盤面沒擺成功")
	}
	if hits == 0 {
		t.Fatal("得手了但一個人的忠誠都沒動到——忠誠沒壓下去，" +
			"或者名單常式取的不是這一郡的人")
	}
}
