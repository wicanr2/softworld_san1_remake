//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// `DS:0x506a` 那張表在**戰術層**有沒有人碰（`docs/mechanics/90-version-diff`
// 的最後一項）。
//
// `TestWhoReadsTheEditionWords` 量到它在一個內政月裡既沒被讀也沒被寫。
// 那只涵蓋戰略層——這一支把同一組監看掛在主戰場上再問一次。
//
// **正對照照樣要有**：同一輪先看部隊記錄（`es:[0x3502]`）有沒有被讀，
// 沒有的話就是監看沒生效，這一輪的零筆不能當結論。
func TestWhoTouchesTheEditionTable(t *testing.T) {
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
	at, to := stageABattle(t, o, base)

	var dgroup uint16
	o.OnCall(addr(0x2053c), func(o *oracle.Oracle) {
		if dgroup == 0 {
			dgroup = o.DSReg()
		}
	})
	cmdReads := 0
	o.OnCall(addr(0x27a68), func(*oracle.Oracle) { cmdReads++ })

	driveIntoBattle(t, o, at, to)
	if dgroup == 0 {
		t.Fatal("沒有進到主戰場")
	}
	dg := uint32(dgroup) << 4
	work := o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg})
	t.Logf("DGROUP 段 %#06x｜戰場工作區段 %#06x", dgroup, work)

	// 紮寨 ＋ 幾天的日循環，把戰術層走一遍。
	play := func(tag string) {
		for step := 1; step <= 12 && cmdReads == 0; step++ {
			o.Drain()
			o.PressScan("0")
			if err := o.Run(60_000_000); err != nil {
				t.Fatalf("%s 紮寨第 %d 步停止：%v", tag, step, err)
			}
		}
		for _, k := range []string{"Y", "0", "Y", "0", "Y", "0", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(80_000_000); err != nil {
				t.Fatalf("%s 送 %q 停止：%v", tag, k, err)
			}
		}
	}

	snap := o.Save()
	survey := func(name string, lo, hi uint32) int {
		o.Restore(snap)
		cmdReads = 0
		rd := o.WatchReadsAt(lo, hi)
		wr := o.WatchWritesAt(lo, hi)
		play(name)
		o.StopWatchingReads()
		o.StopWatchingWrites()
		byIP := map[string]int{}
		for _, r := range *rd {
			byIP[fmt.Sprintf("%#06x:%#06x", r.IP.Seg, r.IP.Off)]++
		}
		wIP := map[string]int{}
		for _, w := range *wr {
			wIP[fmt.Sprintf("%#06x:%#06x", w.IP.Seg, w.IP.Off)]++
		}
		t.Logf("%s：讀 %d 次（%d 個位址）、寫 %d 次（%d 個位址）",
			name, len(*rd), len(byIP), len(*wr), len(wIP))
		if len(byIP) > 0 && len(byIP) <= 8 {
			t.Logf("%s：讀的指令 %v", name, byIP)
		}
		if len(wIP) > 0 && len(wIP) <= 8 {
			t.Logf("%s：寫的指令 %v", name, wIP)
		}
		return len(*rd) + len(*wr)
	}

	// 正對照：部隊記錄一定會被碰。
	ctrl := survey("正對照 部隊記錄",
		uint32(work)<<4+battleUnitBase, uint32(work)<<4+battleUnitBase+41)
	if ctrl == 0 {
		t.Fatal("正對照一次都沒碰到——監看沒生效，這一輪的零筆不能當結論")
	}
	got := survey("0x506a 那張表", dg+0x506a, dg+0x5077)
	if got == 0 {
		t.Logf("**戰術層走完一輪也沒有人碰 DS:0x506a**（正對照同一輪碰了 %d 次）。",
			ctrl)
	}
}
