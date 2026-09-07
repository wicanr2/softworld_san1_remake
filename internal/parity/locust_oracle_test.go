//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 蝗害的對拍。
//
// **四季事件一年各只跑一次**（`docs/playtest/02`），所以照劇本跑十二個月
// 蝗害只有一次機會。這裡把分派器讀到的月份改成七月，每個月都跑秋季常式。
//
// ⚠ **段基底不要猜。** `mov es, [0xa72a]` 的那個 `[0xa72a]` 用
// `oracle.DS()` 取回來是垃圾（讀到的「月份」是 55435、63232）。
// 改掛在**讀取那道指令**上（`0x15c29`，`mov ax, es:[0x3f08]`）——
// 那一刻 ES 已經載好，直接用 `o.ES()` 就是對的段，而且 hook 在指令
// 執行前觸發，寫進去的值那道指令就讀得到。

// TestLocustMatchesTheOriginal 讓每個月都跑秋季常式，核對蝗害的兩條算式。
func TestLocustMatchesTheOriginal(t *testing.T) {
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

	months := map[uint16]int{}
	o.OnCall(addr(0x15c29), func(o *oracle.Oracle) {
		a := oracle.Addr{Seg: o.ES(), Off: 0x3f08}
		months[o.Word(a)]++
		o.SetWord(a, 7) // 七月 ＝ 秋季常式
	})
	autumn := 0
	o.OnCall(addr(0x169d6), func(*oracle.Oracle) { autumn++ })

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	// 判準是「**存在**一個合法的保留率算得出原版寫回去的那個數」。
	//
	// ⚠ 不要用反推區間。`987 ÷ 4937 = 19.99%` 反推出 `19..20`，
	// 而合法範圍是 `20..29`——邊界差一格就把對的判成錯的。
	// 直接列舉十來個候選最乾淨，而且原版走浮點、remake 走整數，
	// 兩者可能差一，所以允許 ±1。
	inKeep := func(old, got int, k game.Keep) bool {
		for r := k.Floor; r <= k.Floor+k.Spread; r++ {
			if n := old * r / 100; n == got || n == got-1 || n == got+1 {
				return true
			}
		}
		return false
	}

	land, rice, bad := 0, 0, 0
	o.OnCall(addr(0x16d26), func(o *oracle.Oracle) {
		pref := int(o.SI()) / 176
		if pref < 1 || pref > state.PrefectureCount {
			return
		}
		old := int(o.Byte(addr(staBase + uint32(pref*176+27))))
		got := int(o.AX() & 0xFF)
		land++
		if old == 0 {
			return
		}
		if !inKeep(old, got, game.LocustLandKeep) {
			t.Errorf("郡 %d 土地價值 %d→%d：沒有一個 %d..%d%% 的保留率算得出來",
				pref, old, got, game.LocustLandKeep.Floor,
				game.LocustLandKeep.Floor+game.LocustLandKeep.Spread)
			bad++
		}
	})
	// ⚠ 米的寫入點是 `0x16ce8`（用 SI）。`0x16bb8` 是**秋收**——
	// 先前掛在那裡收到 298 次「蝗害」，而新值恰好等於 `176 × 郡`，
	// 那是索引不是米。**一個與郡編號成正比的「觀測值」是掛錯位址的徵兆。**
	o.OnCall(addr(0x16ce8), func(o *oracle.Oracle) {
		pref := int(o.SI()) / 176
		if pref < 1 || pref > state.PrefectureCount {
			return
		}
		old := int(o.Word(addr(staBase + uint32(pref*176+20))))
		got := int(o.AX())
		rice++
		if old == 0 {
			return
		}
		if !inKeep(old, got, game.LocustRiceKeep) {
			t.Errorf("郡 %d 米 %d→%d：沒有一個 %d..%d%% 的保留率算得出來",
				pref, old, got, game.LocustRiceKeep.Floor,
				game.LocustRiceKeep.Floor+game.LocustRiceKeep.Spread)
			bad++
		}
	})

	const settle = 40_000_000
	for m := 0; m < 8; m++ {
		// 每個月把兩道門備好：忠誠低、土地價值高。
		for p := 1; p <= state.PrefectureCount; p++ {
			o.SetByte(addr(staBase+uint32(p*176+26)), 0)
			o.SetByte(addr(staBase+uint32(p*176+27)), 100)
		}
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("八個月：亂數 %d 次、分派器讀到的月份 %v、秋季常式 %d 次",
		rnd, months, autumn)
	t.Logf("蝗害改地力 %d 次、改米 %d 次，%d 次對不上", land, rice, bad)
	if autumn == 0 {
		t.Fatal("秋季常式一次都沒跑——月份沒改成功")
	}
	if land+rice < 5 {
		t.Errorf("只收到 %d 次蝗害——樣本太少，說不上量到", land+rice)
	}
}
