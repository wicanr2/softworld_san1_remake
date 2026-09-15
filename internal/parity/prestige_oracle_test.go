//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestZZPrestigeAutumnDrift 對拍人望的年度調整（`0x16d4f`–`0x16e6a`，Issue #18）。
//
// 判準是原版自己寫回去的人望：在迴圈入口（`0x16d4f`）把三張表拍下來，
// 迴圈結束（`0x16e6a`）再讀十六個勢力的人望，逐勢力與 remake 的
// `PrestigeDrift` 套在**同一份快照**上比。輸入是原版當下的盤面，不是
// remake 的，所以比的只有這條算式。
//
// 盤面不必自己擺：載入的進度在八月，走到隔年七月就會碰到秋季；
// 為了讓差距的正負兩側都有樣本，把幾個君主的魅力改成極端值。
func TestZZPrestigeAutumnDrift(t *testing.T) {
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

	// 把君主的魅力交錯拉到 100／10（玩家的也改，秋季那一段不問人）：
	// 差距要落在兩側，向零取整那一格才驗得到。負向要 魅力 ＋ 平均 ≤ 68，
	// 而電腦君主一年裡會被賞賜美女、駿馬把魅力拉回二十幾，所以順手把
	// 玩家那個郡（載入的進度只有一個郡、平均 19）的君主也壓到 10。
	k := 0
	for i := 0; i < 16; i++ {
		ctl := o.Word(addr(base + uint32(i*72)))
		if ctl != 1 && ctl != 2 {
			continue
		}
		lord := int(o.Word(addr(base + uint32(i*72+2))))
		if lord < 0 || lord >= 350 {
			continue
		}
		v := []uint8{100, 10}[k%2]
		if ctl == 1 {
			v = 10
		}
		o.SetByte(addr(genBase+uint32(lord*30+11)), v)
		k++
	}

	type snap struct {
		before [16]int
		land   [16]int
		sum    [16]int
		charm  [16]int
	}
	var snaps []snap
	var afters [][16]int
	o.OnCall(addr(0x16d4f), func(o *oracle.Oracle) {
		var s snap
		for f := 0; f < 16; f++ {
			s.before[f] = int(o.Word(addr(base + uint32(f*72+8))))
			lord := int(o.Word(addr(base + uint32(f*72+2))))
			if lord >= 0 && lord < 350 {
				s.charm[f] = int(o.Byte(addr(genBase + uint32(lord*30+11))))
			}
		}
		for id := 1; id <= state.PrefectureCount; id++ {
			rec := o.Bytes(addr(staBase+uint32(id*state.PrefectureRecordSize)), 32)
			owner := int(rec[30])
			if owner == 0xFF || owner >= 16 {
				continue
			}
			s.land[owner]++
			s.sum[owner] += int(rec[26])/2 + int(rec[27])/2
		}
		snaps = append(snaps, s)
	})
	o.OnCall(addr(0x16e6a), func(o *oracle.Oracle) {
		var a [16]int
		for f := 0; f < 16; f++ {
			a[f] = int(o.Word(addr(base + uint32(f*72+8))))
		}
		afters = append(afters, a)
	})

	const settle = 40_000_000
	for m := 0; m < 14 && len(afters) == 0; m++ {
		for _, key := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(key)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, key, err)
			}
		}
	}
	if len(snaps) == 0 || len(afters) == 0 {
		t.Fatalf("十四個月裡沒走到秋季的人望調整（入口 %d 次、出口 %d 次）", len(snaps), len(afters))
	}
	s, a := snaps[0], afters[0]
	checked, bad := 0, 0
	for f := 0; f < 16; f++ {
		if s.land[f] == 0 {
			if a[f] != s.before[f] {
				t.Errorf("勢力 %d 沒有領地，人望卻從 %d 變成 %d", f, s.before[f], a[f])
				bad++
			}
			continue
		}
		want := s.before[f] + game.PrestigeDrift(s.charm[f], s.sum[f], s.land[f])
		if want < 0 {
			want = 0
		}
		if want > 100 {
			want = 100
		}
		checked++
		if want != a[f] {
			t.Errorf("勢力 %2d：魅力 %3d、領地 %2d、和 %4d，人望 %3d → 原版 %3d、remake %3d",
				f, s.charm[f], s.land[f], s.sum[f], s.before[f], a[f], want)
			bad++
			continue
		}
		t.Logf("勢力 %2d：魅力 %3d、領地 %2d、平均 %3d，人望 %3d → %3d（d ＝ %+d）✓",
			f, s.charm[f], s.land[f], s.sum[f]/s.land[f], s.before[f], a[f], a[f]-s.before[f])
	}
	if checked < 8 {
		t.Errorf("只比到 %d 個勢力——樣本太少", checked)
	}
	if bad > 0 {
		t.Errorf("%d／%d 個勢力的人望對不上", bad, checked)
	}
}
