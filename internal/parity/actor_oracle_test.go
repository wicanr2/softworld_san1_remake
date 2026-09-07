//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 選出行動者的對拍（表 `0x54d4`，常式 `0xec86`）。
//
// 六個等級都呼叫同一支 `0xec86(郡)`（等級 0–3 另外寫兩個全域，
// 與挑人無關）：
//
//	0xf17:0x0aae(郡, 2)   ; 建本郡在職者名單（身分 0–3）
//	0xf17:0x0000          ; 排序
//	行動者 ＝ 名單[0]      ; 寫進三個全域   0xecb2  AX ＝ 行動者
//
// 排序鍵是 **`謀略 ＋ 戰力 ＋ 加權表[身分]`**（`0xf1d7` 的
// `add ax,[bx+0x5986]`），權重是 2000／1600／1200／800——完全壓過能力值
// （謀略 ＋ 戰力 最多 200），所以實際上是「先看身分，同身分再比能力」。
//
// **後面八種行為讀的都是這一位**，所以挑錯人整輪都會偏掉。

// actorWeight 是身分的加權（`internal/ai` 的同名表）。
var actorWeight = [12]int{2000, 1600, 1200, 800, 2000, 1600, 1200, 800, 0, 0, 400, 0}

// TestActorPickMatchesTheOriginal 核對每個郡這回合誰行動。
func TestActorPickMatchesTheOriginal(t *testing.T) {
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

	// ⚠ **判準要在 hook 裡當場算**：同一輪後面的「指定軍師」會把人的身分
	// 改成 1（權重 1600），事後再回讀就會多出一個選行動者那一刻還不存在
	// 的軍師——40 次裡有 1 次會這樣，而錯誤訊息看起來像「原版挑錯人」。
	pref := -1
	bad, checked, contested := 0, 0, 0
	var errs []string

	o.OnCall(addr(0x0ec86), func(o *oracle.Oracle) {
		pref = int(int16(o.Arg(0)))
	})
	o.OnCall(addr(0x0ecb2), func(o *oracle.Oracle) {
		p := pref
		pref = -1
		if p < 1 || p > state.PrefectureCount {
			return
		}
		chosen := int(int16(o.AX()))
		if chosen < 0 || chosen >= 350 {
			return // 名單是空的
		}
		key := func(slot int) int {
			at := genBase + uint32(slot)*30
			r := int(o.Byte(addr(at + 17)))
			w := 0
			if r < len(actorWeight) {
				w = actorWeight[r]
			}
			return int(o.Byte(addr(at+9))) + int(o.Byte(addr(at+10))) + w
		}
		mine, ranks := key(chosen), map[int]bool{}
		for i := 0; i < 350; i++ {
			at := genBase + uint32(i)*30
			if int(o.Byte(addr(at+19))) != p {
				continue
			}
			r := int(o.Byte(addr(at + 17)))
			if r > 3 {
				continue // 名單只收身分 0–3
			}
			ranks[r] = true
			if key(i) > mine {
				errs = append(errs, fmt.Sprintf(
					"郡 %d：選了槽 %d（鍵 %d），槽 %d 的鍵是 %d",
					p, chosen, mine, i, key(i)))
				bad++
				break
			}
		}
		if len(ranks) > 1 {
			contested++
		}
		checked++
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

	for _, e := range errs {
		t.Error(e)
	}
	t.Logf("兩個月：亂數 %d 次（正對照）、選行動者 %d 次"+
		"（其中 %d 次郡裡有兩種以上身分），%d 項對不上",
		rnd, checked, contested, bad)
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if checked == 0 {
		t.Fatal("兩個月裡一次都沒選行動者")
	}
	if contested == 0 {
		t.Error("沒有一次郡裡有兩種以上身分——加權表等於沒驗到")
	}
}
