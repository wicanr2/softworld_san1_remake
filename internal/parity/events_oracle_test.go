//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 四季事件的對拍：老死與蝗害。
//
// 判準是**原版自己寫回去的那個數**：掛在寫入指令上，hook 在指令執行前
// 觸發，所以此刻記憶體裡還是舊值、AL 已經是新值。兩邊都有就比得出公式。
//
//	老死  0x15dd2  mov es:[bx+0x2218], al   體能，BX ＝ 30 × 槽號
//	蝗害  0x16d26  mov es:[si+0x49b], al    土地價值，SI ＝ 176 × 郡
//
// **蝗害那一條與直覺相反**（保留率 80–169%，平均往上），所以特別要
// 實跑一次——碼讀得再清楚，一條「災害讓田變好」的規則值得原版自己說。

// TestEventsMatchTheOriginal 讓原版跑半年，核對老死與蝗害的算式。
func TestEventsMatchTheOriginal(t *testing.T) {
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
	staBase := base + uint32(state.MasterTableSize)

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	aging, locust, bad := 0, 0, 0
	o.OnCall(addr(0x15dd2), func(o *oracle.Oracle) {
		slot := int(o.BX()) / 30
		if slot < 0 || slot >= 350 {
			return
		}
		rec := func(f int) int { return int(o.Byte(addr(genBase + uint32(slot*30+f)))) }
		age, stamina, life := rec(7), rec(8), rec(28)
		got := int(o.AX() & 0xFF)
		aging++
		// roll 反推：新體能 = 舊 + (壽命−年齡)×25 − RND(50)，夾到 0。
		roll := stamina + (life-age)*game.OverAgeWeight - got
		if got == 0 {
			// 夾住了，只能驗「不夾的話會是非正」。
			if want := game.AgingDrop(stamina, age, life, 0); want > game.DeathStaminaSpread {
				t.Errorf("槽 %d 體能 %d 年齡 %d 壽命 %d：原版歸零，"+
					"remake 連擲滿都還有 %d", slot, stamina, age, life, want)
				bad++
			}
			return
		}
		if roll < 0 || roll >= game.DeathStaminaSpread {
			t.Errorf("槽 %d 體能 %d→%d 年齡 %d 壽命 %d：反推的擲值 %d 不在 0..%d",
				slot, stamina, got, age, life, roll, game.DeathStaminaSpread-1)
			bad++
			return
		}
		if want := game.AgingDrop(stamina, age, life, roll); want != got {
			t.Errorf("槽 %d：原版體能寫 %d，remake 算 %d", slot, got, want)
			bad++
		}
		if !game.AlreadyPastPrime(age, life, 0) {
			t.Errorf("槽 %d 年齡 %d 壽命 %d：原版動了體能，remake 卻說還沒過壽",
				slot, age, life)
			bad++
		}
	})
	// 蝗害那一條鏈逐關計數：秋季常式 → 蝗害入口 → 真的改了地力。
	// `locust == 0` 有三種意思，分開數才知道是哪一種
	// （`~/diagnosis-notes/02`：查詢回空的四種形狀）。
	// 四季常式一年各只跑一次（月 1 春、4 夏、7 秋、10 冬，`docs/re/06`）。
	// 四個都數才知道十二輪按鍵到底走了幾個月——老死在春季，所以
	// 「春跑了秋沒跑」不是閘門問題，是月份沒走滿。
	season := map[string]int{}
	for at, name := range map[uint32]string{
		0x15c5c: "春", 0x164c8: "夏", 0x169d6: "秋", 0x16e70: "冬",
	} {
		n := name
		o.OnCall(addr(at), func(*oracle.Oracle) { season[n]++ })
	}
	autumn, locustIn := 0, 0
	o.OnCall(addr(0x169d6), func(*oracle.Oracle) { autumn++ })
	o.OnCall(addr(0x16bd5), func(*oracle.Oracle) { locustIn++ })
	o.OnCall(addr(0x16d26), func(o *oracle.Oracle) {
		pref := int(o.SI()) / 176
		if pref < 1 || pref > state.PrefectureCount {
			return
		}
		old := int(o.Byte(addr(staBase + uint32(pref*176+27))))
		got := int(o.AX() & 0xFF)
		locust++
		if old == 0 {
			return
		}
		// 保留率反推：新 = 舊 × r ÷ 100。
		lo := got * 100 / old
		hi := (got + 1) * 100 / old
		keep := game.LocustLandKeep
		if hi <= keep.Floor || lo >= keep.Floor+keep.Spread+1 {
			t.Errorf("郡 %d 土地價值 %d→%d：保留率落在 %d..%d%%，"+
				"remake 說是 %d..%d%%", pref, old, got, lo, hi,
				keep.Floor, keep.Floor+keep.Spread)
			bad++
		}
	})

	// **盤面自己擺**：照劇本跑半年只收到一次老死、蝗害零次——
	// 開局的郡忠誠都很高（蝗害要忠誠低），而多數人還沒到壽命。
	// 這裡每個月把兩個條件重新備好：
	//
	//   蝗害  忠誠 ← 0、土地價值 ← 100  → 判定的兩道門一道必過、
	//                                     一道有七成機率過
	//   老死  年齡 ← 壽命 + 1           → `RND(3) + 壽命 >= 年齡`
	//                                     有三分之一機率不成立
	//
	// 年齡只動前六十個槽：全表拉高會死掉一大片，勢力滅亡之後遊戲就
	// 不往下走了。
	plant := func() {
		for p := 1; p <= state.PrefectureCount; p++ {
			o.SetByte(addr(staBase+uint32(p*176+26)), 0)   // 民眾忠誠
			o.SetByte(addr(staBase+uint32(p*176+27)), 100) // 土地價值
		}
		for slot := 0; slot < 60; slot++ {
			life := o.Byte(addr(genBase + uint32(slot*30+28)))
			if life == 0xFF || life == 0 {
				continue
			}
			o.SetByte(addr(genBase+uint32(slot*30+7)), life+1)
		}
	}

	// **跑到四季都輪過為止，不是固定十二輪。** 四季常式一年各只跑一次
	// （月 1／4／7／10），而 `bootToGame` 停在月中——第一輪按鍵是把
	// 當下那個月走完，不是推進一個月。固定十二輪因此只推了約十個月，
	// 從 9 月出發剛好差在秋季（春 1 夏 1 冬 1 秋 0），而測試的訊息寫成
	// 「盤面沒擺好，或者秋季沒走到」，兩種病因混在同一行。
	//
	// 寫成「跑到四季齊了」直接表達要驗的東西，也不必跟著起點漂移改數字。
	const settle = 40_000_000
	months := 0
	for ; months < 18 && len(season) < 4; months++ {
		plant()
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", months+1, k, err)
			}
		}
	}

	t.Logf("走了 %d 個月：亂數 %d 次（正對照）、老死判定 %d 次、蝗害改地力 %d 次，%d 次對不上",
		months, rnd, aging, locust, bad)
	t.Logf("蝗害那一條鏈：秋季常式進去 %d 次、蝗害入口 %d 次、真的改地力 %d 次",
		autumn, locustIn, locust)
	t.Logf("四季常式各跑了幾次：春 %d 夏 %d 秋 %d 冬 %d（一年各該一次）",
		season["春"], season["夏"], season["秋"], season["冬"])
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if aging == 0 {
		t.Error("一年裡一次老死判定都沒有——盤面沒擺好，或者元月沒走到")
	}
	if season["秋"] == 0 {
		t.Fatalf("走了 %d 個月還沒輪到秋季（春 %d 夏 %d 冬 %d）"+
			"——按鍵序列沒推動月份，蝗害的結果不算數",
			months, season["春"], season["夏"], season["冬"])
	}
	if locust == 0 {
		// 秋季走到了才有資格說「沒發生」。兩道閘門都對擺盤有利
		// （民忠 0、地力 100，`docs/mechanics/50-events` §蝗害），
		// 所以這裡真的為 0 是規則對不上，不是盤面問題。
		t.Errorf("秋季走了 %d 次卻一次蝗害都沒有——擺盤是民忠 0、地力 100，"+
			"兩道閘門都最有利，這裡為 0 表示判定條件對不上", season["秋"])
	}
}
