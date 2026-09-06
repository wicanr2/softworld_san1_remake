//go:build oracle

package parity

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 誰寫了這個欄位。
//
// **這是「決策程式碼在哪」最直接的答案。** 靜態的交叉參考只涵蓋直接
// 定址；`mov es:[si+0x2228], al` 這種以結構基底加位移的寫法掃不出來，
// 而三張表全部是這樣存取的（`docs/re/03` §1）。
//
// 做法是讓執行器在寫入落進表的範圍時記下當時的 `CS:IP`，跑一個月，
// 再按 IP 分組。**訓練度是誰改的、金是誰扣的**，答案就是那幾個位址。
//
// 位址換算：主程式映像載在 `0110:0000`，所以映像位移 ＝ 線性位址 − 0x1100
// （`docs/re/01`）。這個位移是穩定的，可以直接拿去對 `objdump` 的輸出。

const imageBase = 0x1100 // DATA5.GRP 映像在記憶體裡的線性起點

// TestZZWhoWritesTheTables 跑一個月，列出寫三張表的程式位址。
func TestZZWhoWritesTheTables(t *testing.T) {
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
	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	nGen := state.GeneralTableSize

	// 分三段看，否則同一個 IP 寫哪一張表分不出來。
	for _, seg := range []struct {
		name string
		lo   uint32
		n    int
		rec  int
		fld  map[int]string
	}{
		{"諸侯", base, nMas, state.MasterRecordSize, masField},
		{"州郡", base + uint32(nMas), nSta, state.PrefectureRecordSize, prefField},
		{"人物", base + uint32(nMas+nSta), nGen, state.GeneralRecordSize, genField},
	} {
		log := o.WatchWritesAt(seg.lo, seg.lo+uint32(seg.n)-1)
		for _, keys := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(keys)
			if err := o.Run(40_000_000 * 3); err != nil {
				t.Fatalf("%s：原版停止 %v", seg.name, err)
			}
		}
		o.StopWatchingWrites()

		type site struct {
			n      int
			fields map[string]bool
		}
		by := map[uint32]*site{}
		for _, w := range *log {
			lin := uint32(w.IP.Seg)*16 + uint32(w.IP.Off)
			s := by[lin]
			if s == nil {
				s = &site{fields: map[string]bool{}}
				by[lin] = s
			}
			s.n++
			s.fields[fieldName(seg.fld, int(w.Off)%seg.rec)] = true
		}
		ips := make([]uint32, 0, len(by))
		for a := range by {
			ips = append(ips, a)
		}
		sort.Slice(ips, func(i, j int) bool { return by[ips[i]].n > by[ips[j]].n })

		t.Logf("%s表：一個月裡有 %d 次寫入，來自 %d 個位址",
			seg.name, len(*log), len(ips))
		for i, a := range ips {
			if i >= 20 {
				t.Logf("    …（還有 %d 個位址）", len(ips)-20)
				break
			}
			var fs []string
			for f := range by[a].fields {
				fs = append(fs, f)
			}
			sort.Strings(fs)
			t.Logf("    映像 %#06x（線性 %#06x）寫了 %4d 次：%v",
				a-imageBase, a, by[a].n, fs)
		}
	}
}
