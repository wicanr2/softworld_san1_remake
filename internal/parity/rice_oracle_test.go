//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 買米的匯率對拍（`docs/mechanics/60-economy` §1.3，`0xc634`）。
//
// 電腦諸侯每個月都會買米（分派表 `0x55d4`），而**買賣共用同一條匯率**
// ——一金換 `(100 − 物價) ÷ 10` 單位米。原版這一段走浮點，remake 走整數，
// 兩邊會不會差一只有實跑說得準。
//
// 寫回去的順序是先金（`0xc759`）後米（`0xc767`），所以在第一個 hook 記
// 舊金與新金，在第二個 hook 記舊米與新米，兩者一配就得到「花了多少金、
// 換到多少米」。

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

	type buy struct{ pref, price, spent, got int }
	var pending struct {
		pref, price, spent int
		armed              bool
	}
	var buys []buy
	o.OnCall(addr(0x0c759), func(o *oracle.Oracle) {
		pref := int(o.BX()) / 176
		if pref < 1 || pref > state.PrefectureCount {
			return
		}
		old := int(o.Word(addr(staBase + uint32(pref*176+18))))
		pending.pref = pref
		pending.price = int(o.Byte(addr(staBase + uint32(pref*176+29))))
		pending.spent = old - int(o.AX())
		pending.armed = true
	})
	o.OnCall(addr(0x0c767), func(o *oracle.Oracle) {
		pref := int(o.BX()) / 176
		if !pending.armed || pref != pending.pref {
			return
		}
		old := int(o.Word(addr(staBase + uint32(pref*176+20))))
		buys = append(buys, buy{pending.pref, pending.price,
			pending.spent, int(o.AX()) - old})
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

	bad := 0
	for _, b := range buys {
		if b.spent <= 0 {
			continue // 沒花錢就沒得比
		}
		rate := game.RicePerGold(uint8(b.price))
		if want := b.spent * rate; want != b.got {
			t.Errorf("郡 %d 物價 %d：花 %d 金換到 %d 米，"+
				"remake 的匯率 %d 給的是 %d", b.pref, b.price, b.spent, b.got,
				rate, want)
			bad++
			continue
		}
	}
	t.Logf("%d 次買米，%d 次對不上", len(buys), bad)
}
