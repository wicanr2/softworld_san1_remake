//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 君主繼承的對拍（`docs/mechanics/80-victory` §2.5）。
//
// 判準是原版自己寫回去的兩個數：
//
//	0x14c84  mov es:[si+8], ax    新人望，SI ＝ 72 × 勢力
//	0x14cbe  mov es:[bx+2], ax    新君主的槽號，BX ＝ 72 × 勢力
//
// 進到 `0x14c84` 時 BX 還留著 `30 × 繼承者槽號`（`0x14c4a` 之後沒動過），
// 所以繼承者是誰、他的魅力多少都拿得到，不必猜段變數。
//
// 人望那條是**浮點**的（`× 0.01` 再 `+ 0.5` 取整），remake 用整數
// `(魅力 × 人望 + 50) ÷ 100`——兩邊會不會差一，只有實跑說得準。

// TestSuccessionMatchesTheOriginal 讓幾個電腦君主老死，核對繼承。
func TestSuccessionMatchesTheOriginal(t *testing.T) {
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

	// **盤面自己擺**：把電腦勢力的君主擺成過壽五年、體能 1，
	// 元月的老死判定必定帶走他們。玩家控制的勢力不動——那一條會
	// 跳出「自己挑繼承者」的選單，卡在等輸入。
	dying := map[int]bool{}
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) != 2 { // 2 ＝ 電腦操縱
			continue
		}
		lord := int(o.Word(addr(base + uint32(i*72+2))))
		if lord < 0 || lord >= 350 {
			continue
		}
		life := o.Byte(addr(genBase + uint32(lord*30+28)))
		if life == 0xFF || life == 0 || life > 250-5 {
			continue
		}
		o.SetByte(addr(genBase+uint32(lord*30+7)), life+5) // 年齡
		o.SetByte(addr(genBase+uint32(lord*30+8)), 1)      // 體能
		dying[i] = true
	}
	t.Logf("擺了 %d 個電腦君主過壽五年", len(dying))

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	type shot struct{ faction, heir, charm, before, after int }
	var shots []shot
	o.OnCall(addr(0x14c84), func(o *oracle.Oracle) {
		faction := int(o.SI()) / 72
		heir := int(o.BX()) / 30
		if faction < 0 || faction >= state.MasterTableSize/72 || heir >= 350 {
			return
		}
		shots = append(shots, shot{
			faction: faction, heir: heir,
			charm:  int(o.Byte(addr(genBase + uint32(heir*30+11)))),
			before: int(o.Word(addr(base + uint32(faction*72+8)))),
			after:  int(o.AX()),
		})
	})
	var enthroned []int
	o.OnCall(addr(0x14cbe), func(o *oracle.Oracle) {
		enthroned = append(enthroned, int(o.AX()))
	})

	const settle = 40_000_000
	for m := 0; m < 4; m++ {
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("亂數 %d 次（正對照）、繼承 %d 次、登基 %d 次",
		rnd, len(shots), len(enthroned))
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if len(shots) == 0 {
		t.Fatal("四個月裡一次繼承都沒有——盤面沒擺好，或者元月沒走到")
	}

	bad := 0
	for _, s := range shots {
		want := game.SuccessionPrestige(s.charm, s.before)
		if want != s.after {
			t.Errorf("勢力 %d：繼承者槽 %d 魅力 %d，人望 %d → 原版 %d、remake %d",
				s.faction, s.heir, s.charm, s.before, s.after, want)
			bad++
			continue
		}
		t.Logf("勢力 %2d 繼承者槽 %3d 魅力 %3d：人望 %3d → %3d ✓",
			s.faction, s.heir, s.charm, s.before, s.after)
	}
	if bad > 0 {
		t.Errorf("%d／%d 次繼承的人望對不上", bad, len(shots))
	}
}
