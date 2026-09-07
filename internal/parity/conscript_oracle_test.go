//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 徵兵的對拍（`docs/mechanics/70-ai` §2.13.1，`0xbeb8`）。
//
// **不走玩家選單**：電腦諸侯每個月都在徵兵，而且與玩家共用同一條效果
// （`game.Conscript`）。常式走守軍清單，對每一位算出人數之後寫回四個欄位，
// 四個寫入點都在同一輪迴圈裡：
//
//	0xbfcc  mov cx, es:0x48e(bx)     ; 州郡 offset 14 ＝ 人口 ÷ 100
//	0xc025  add ax, es:0x2226(bx)    ; 人物 offset 22 ＝ 兵力，AX ＝ 徵到的人數
//	0xc03e  mov al, es:0x2228(bx)    ; 人物 offset 24 ＝ 訓練度
//	0xc057  mov al, es:0x2229(bx)    ; 人物 offset 25 ＝ 武裝度
//
// 判準是 remake 的公式算得出原版寫回去的那三個數：
//
//	兵力'   ＝ 兵力 ＋ 人數
//	訓練度' ＝ DiluteAfterRecruit(訓練度, 舊兵力, 新兵力)
//	武裝度' ＝ DiluteAfterRecruit(武裝度, 舊兵力, 新兵力)
//
// ⚠ **中間那一步先截斷成「武器數／受訓人數」再攤回百分比**，而且乘的是
// **float32 的 0.01**——整除的時候會少 1（`game.WeaponsF32`）。
// 拿 `舊值 × 舊兵力 ÷ 新兵力` 一步到底會系統性地偏高，
// 而偏差只有一兩格，看起來像浮點誤差而不像公式錯。
//
// ⚠ **人口存的是實際值 ÷ 100**（`docs/spec/003` §3），而人數是實際人頭。
// 兩邊單位不同，比之前要先換算。

// TestConscriptionMatchesTheOriginal 讓電腦諸侯去徵兵，逐次核對四個欄位。
func TestConscriptionMatchesTheOriginal(t *testing.T) {
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

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	type shot struct {
		slot                     int
		got, oldMen              int
		oldTrain, oldArms        int
		newTrain, newArms        int
		popBefore, popAfter, men int
	}
	var shots []shot
	var cur shot
	armed := false

	// 人口：這一輪最先寫，所以在這裡開一筆。
	o.OnCall(addr(0x0bfcc), func(o *oracle.Oracle) {
		pref := int(o.BX()) / 176
		if pref < 1 || pref > state.PrefectureCount {
			return
		}
		cur = shot{
			popBefore: int(o.Word(addr(staBase + uint32(pref*176+14)))),
			popAfter:  int(o.CX()),
		}
		armed = true
	})
	o.OnCall(addr(0x0c025), func(o *oracle.Oracle) {
		if !armed {
			return
		}
		si := uint32(o.BX())
		cur.slot = int(si) / 30
		cur.men = int(int16(o.AX()))
		cur.oldMen = int(o.Word(addr(genBase + si + 22)))
		cur.oldTrain = int(o.Byte(addr(genBase + si + 24)))
		cur.oldArms = int(o.Byte(addr(genBase + si + 25)))
	})
	o.OnCall(addr(0x0c03e), func(o *oracle.Oracle) {
		if armed {
			cur.newTrain = int(o.AX() & 0xFF)
		}
	})
	o.OnCall(addr(0x0c057), func(o *oracle.Oracle) {
		if !armed {
			return
		}
		cur.newArms = int(o.AX() & 0xFF)
		shots = append(shots, cur)
		armed = false
	})

	// **盤面自己擺**：金拉滿、人口拉滿、兵力壓低，電腦才徵得動。
	// 預算是郡的金的 30–50 %（§2.14），所以金是徵兵量的實際上限。
	const settle = 40_000_000
	for m := 0; m < 3; m++ {
		for p := 1; p <= state.PrefectureCount; p++ {
			o.SetWord(addr(staBase+uint32(p*176+18)), 30000) // 金
			o.SetWord(addr(staBase+uint32(p*176+14)), 900)   // 人口 ÷ 100
		}
		for i := 0; i < 350; i++ {
			if o.Word(addr(genBase+uint32(i*30+22))) > 20 {
				o.SetWord(addr(genBase+uint32(i*30+22)), 20)
			}
		}
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("三個月：亂數 %d 次（正對照）、徵兵 %d 次", rnd, len(shots))
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if len(shots) == 0 {
		t.Fatal("三個月裡一次都沒徵兵——金、人口或兵力的盤面沒擺成功")
	}

	bad := 0
	for _, s := range shots {
		if s.men <= 0 {
			continue
		}
		total := s.oldMen + s.men
		if want := game.DiluteAfterRecruit(s.oldTrain, s.oldMen, total); want != s.newTrain {
			t.Errorf("槽 %d：訓練度 %d、兵 %d → %d，原版寫 %d，remake 算 %d",
				s.slot, s.oldTrain, s.oldMen, total, s.newTrain, want)
			bad++
		}
		if want := game.DiluteAfterRecruit(s.oldArms, s.oldMen, total); want != s.newArms {
			t.Errorf("槽 %d：武裝度 %d、兵 %d → %d，原版寫 %d，remake 算 %d",
				s.slot, s.oldArms, s.oldMen, total, s.newArms, want)
			bad++
		}
		// 人口：存的是 ÷100，扣的是實際人頭。
		if w := (s.popBefore*100 - s.men) / 100; abs(w-s.popAfter) > 1 {
			t.Errorf("槽 %d：人口 %d00 徵 %d 人，原版寫 %d00，remake 算 %d00",
				s.slot, s.popBefore, s.men, s.popAfter, w)
			bad++
		}
	}
	t.Logf("%d 次徵兵，%d 項對不上", len(shots), bad)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
