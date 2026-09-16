//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 指定太守的對拍（表 `0x5674`，`docs/mechanics/70-ai`）。
//
// 六個等級全部 thunk 到同一支 `0xd652`，不分等級：
//
//	諸侯 offset 0 == 1（玩家操縱）→ 不做
//	主事者是君主（身分 0）→ 不做
//	lcall 0xf17:0x490            ; **照魅力遞減的選擇排序**（`0xf600`）
//	舊主事者身分 2 → 改成 3
//	州郡.主事者(offset 32) ← 名單第一位      0xd705  AX ＝ 新太守
//	新太守身分 3 → 改成 2
//	州郡.所屬(offset 30) ← 人物[新太守].勢力  0xd74d  CL ＝ 勢力
//
// 判準：選出來的人**在本郡**，而且**魅力不低於同郡任何一位在職者**。
// 這一條直接對上 remake 的 `mostCharming`。

// TestAppointGovernorMatchesTheOriginal 核對電腦每月換太守挑的是誰。
func TestAppointGovernorMatchesTheOriginal(t *testing.T) {
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

	// 每一次換太守都要**當場**檢查——同一段執行裡後面的郡回合就會把人
	// 搬走（出兵、移防），等按鍵送完再讀所在郡會看到他已經在別的郡
	//（郡 6 挑的 81 在同一段裡走到郡 14）。所以在寫主事者那一道指令上
	// 直接讀盤面。
	bad, checked := 0, 0
	lastPref, lastChosen := -1, -1
	o.OnCall(addr(0x0d705), func(o *oracle.Oracle) {
		pref := int(o.BX()) / 176
		if pref < 1 || pref > state.PrefectureCount {
			return
		}
		chosen := int(int16(o.AX()))
		lastPref, lastChosen = pref, chosen
		if chosen < 0 || chosen >= 350 {
			t.Errorf("郡 %d：選出來的槽號是 %d", pref, chosen)
			bad++
			return
		}
		at := genBase + uint32(chosen)*30
		if got := int(o.Byte(addr(at + 19))); got != pref {
			t.Errorf("郡 %d：選了槽 %d，但他的所在郡是 %d", pref, chosen, got)
			bad++
			return
		}
		mine := int(o.Byte(addr(at + 11)))
		// 同郡任何一位在職者的魅力都不能高過他。
		for i := 0; i < 350; i++ {
			p := genBase + uint32(i)*30
			if int(o.Byte(addr(p+19))) != pref {
				continue
			}
			if r := o.Byte(addr(p + 17)); r > 3 {
				continue // 在野／未登場不在名單裡
			}
			if int(o.Byte(addr(p+11))) > mine {
				t.Errorf("郡 %d：選了槽 %d（魅力 %d），但槽 %d 的魅力是 %d",
					pref, chosen, mine, i, o.Byte(addr(p+11)))
				bad++
				break
			}
		}
		checked++
	})
	o.OnCall(addr(0x0d74d), func(o *oracle.Oracle) {
		if lastChosen < 0 || lastChosen >= 350 {
			return
		}
		owner := int(int8(o.CX() & 0xFF))
		faction := int(int8(o.Byte(addr(genBase + uint32(lastChosen)*30 + 18))))
		if owner != faction {
			t.Errorf("郡 %d：所屬寫成 %d，新太守的勢力卻是 %d", lastPref, owner, faction)
			bad++
		}
	})

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	const settle = 40_000_000
	for m := 0; m < 2; m++ {
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("兩個月：亂數 %d 次（正對照）、換太守 %d 次，%d 項對不上",
		rnd, checked, bad)
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if checked == 0 {
		t.Fatal("兩個月裡一次都沒換太守")
	}
}
