//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 買米的對拍（`docs/mechanics/60-economy` §1.3，`0xc634`）。
//
// 整支常式走浮點（`0xc655`–`0xc767`）：
//
//	目標米   = min(30000, 兵士 × (RND(10) + 12))
//	缺口     = 目標米 − 現有米
//	剩下的金 = clamp(金 − 缺口 ÷ 匯率, 0, 30000)
//	花掉的金 = 金 − 剩下的金
//	買到的米 = 花掉的金 × 匯率
//
// **匯率的除數隨 AI 等級變**：`[10,10,10,10,9,8]`，六個呼叫端各一份。
// 只讀第一個會得到「一律 ÷ 10」，而那只對等級 0–3 成立
// （`CONTEXT.md` §4 的 R17）。
//
// 等級由**呼叫點**分辨，不從諸侯記錄的 offset 4 讀。兩者絕大多數時候
// 一致，但郡在同一個月易主時就會分岔——買米的是舊主人的常式，
// 事後讀到的所屬卻是新主人。呼叫點是當場的事實。
//
// ⚠ **`Caller()` 只有在函式入口讀得準**：它取的是堆疊頂端那個返回位址，
// 而序幕（`push bp` 一路到配置區域變數）一跑過去，堆疊頂端就不是它了。
// 所以等級要掛在 `0x0c634` 的入口收，不能在函式中段的 `0x0c759` 收——
// 在中段讀出來的值一個都對不上表，而那看起來與「常式沒被呼叫」一模一樣。
//
// 入口同時拿得到原版自己算好的匯率（`Arg(0)`，呼叫端推進來的 `[bp+6]`），
// 所以這支測試核對兩件事：匯率表算不算得出那個數，以及花掉的金乘上匯率
// 是不是原版寫回去的米。
//
// 寫回去的順序是先金（`0xc759`）後米（`0xc767`），兩個 hook 一配就得到
// 「花了多少金、換到多少米」。**兩邊都是截斷過的整數**，而真值是浮點，
// 所以判準留一個匯率的容差——差一格是截斷不是公式錯。

// TestRicePurchaseMatchesTheOriginal 讓電腦去買米，核對匯率。
func TestRicePurchaseMatchesTheOriginal(t *testing.T) {
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

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	// **盤面自己擺**：開局的電腦諸侯 AI 等級只有 4 與 5，照劇本跑只驗得到
	// 六個除數裡的兩個。把等級（`BASEMAS` offset 4）輪流設成 0–5，
	// 只數活著的槽（offset 0 ＝ 0xFFFF 的兩個沒在用）。
	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), uint16(alive%6))
		alive++
	}
	t.Logf("活著的勢力 %d 個，AI 等級輪流設成 0–5", alive)

	// 六個呼叫點的**返回位址** → AI 等級。`push cs`（1 B）＋
	// `call rel16`（3 B），所以是呼叫點 + 4。
	callerLevel := map[uint32]int{
		0x0c7d1 + 4: 0, 0x0c805 + 4: 1, 0x0c839 + 4: 2,
		0x0c86d + 4: 3, 0x0c8a1 + 4: 4, 0x0c8dd + 4: 5,
	}

	type buy struct{ pref, price, spent, got, level, rate int }
	var pending struct {
		pref, price, spent, level, rate int
		armed                           bool
	}
	var buys []buy

	// 入口：記下這一次是哪一支呼叫的，以及呼叫端推進來的匯率。
	cur := struct{ level, rate int }{-1, 0}
	o.OnCall(addr(0x0c634), func(o *oracle.Oracle) {
		lvl, ok := callerLevel[o.Caller().Linear()]
		if !ok {
			cur.level = -1
			return
		}
		cur.level, cur.rate = lvl, int(int16(o.Arg(0)))
	})
	o.OnCall(addr(0x0c759), func(o *oracle.Oracle) {
		pref := int(o.BX()) / 176
		if pref < 1 || pref > state.PrefectureCount {
			return
		}
		if cur.level < 0 {
			return // 別的呼叫端
		}
		old := int(o.Word(addr(staBase + uint32(pref*176+18))))
		pending.pref = pref
		pending.price = int(o.Byte(addr(staBase + uint32(pref*176+29))))
		pending.spent = old - int(o.AX())
		pending.level, pending.rate = cur.level, cur.rate
		pending.armed = true
	})
	o.OnCall(addr(0x0c767), func(o *oracle.Oracle) {
		pref := int(o.BX()) / 176
		if !pending.armed || pref != pending.pref {
			return
		}
		old := int(o.Word(addr(staBase + uint32(pref*176+20))))
		buys = append(buys, buy{pending.pref, pending.price,
			pending.spent, int(o.AX()) - old, pending.level, pending.rate})
		pending.armed = false
	})

	const settle = 40_000_000
	for m := 0; m < 4; m++ {
		// 盤面自己擺：金拉滿、米壓低，電腦才有得買。
		for p := 1; p <= state.PrefectureCount; p++ {
			o.SetWord(addr(staBase+uint32(p*176+18)), 20000)
			o.SetWord(addr(staBase+uint32(p*176+20)), 500)
		}
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("四個月：亂數 %d 次（正對照）、買米 %d 次", rnd, len(buys))
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if len(buys) == 0 {
		t.Skip("四個月裡電腦一次米都沒買")
	}

	bad, levels := 0, map[int]int{}
	for _, b := range buys {
		if b.spent <= 0 {
			continue // 沒花錢就沒得比
		}
		levels[b.level]++
		rate := game.AIRicePerGold(uint8(b.price), b.level)
		if rate != b.rate {
			t.Errorf("郡 %d 等級 %d 物價 %d：原版的匯率是 %d，remake 算 %d",
				b.pref, b.level, b.price, b.rate, rate)
			bad++
			continue
		}
		want := b.spent * rate
		if d := want - b.got; d < -rate || d > rate {
			t.Errorf("郡 %d 等級 %d 物價 %d：花 %d 金換到 %d 米，"+
				"匯率 %d 給的是 %d（差 %d，容差 ±%d）",
				b.pref, b.level, b.price, b.spent, b.got, rate, want, d, rate)
			bad++
		}
	}
	t.Logf("%d 次買米（等級分布 %v），%d 次對不上", len(buys), levels, bad)
	if len(levels) == 0 {
		t.Skip("沒有一次買米看得到 AI 等級")
	}
	if len(levels) < 6 {
		t.Errorf("只走到 %d 個等級，六個除數都要驗到才算數", len(levels))
	}
}
