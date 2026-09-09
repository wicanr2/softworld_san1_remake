//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 戰場工作區裡那幾個函式指標指到哪（`docs/re/05` §12）。
//
// `es:[0x20ea]`、`es:[0x2e5e]`、`es:[0x314a]` 三支是透過指標呼叫的，
// 靜態掃不到目標。**指標的值執行期讀得到**：進到主戰場之後把那幾個
// 遠指標讀出來，解算成線性位址就能反組譯。
//
// `es:[0x2e78]` 已解（反白開關，§2.5），當正對照——它解出來的位址要落在
// 程式的碼段範圍裡，否則就是段取錯了，其餘三個的數字也不能信。
func TestBattleFunctionPointers(t *testing.T) {
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
	driveIntoBattle(t, o, at, to)
	if dgroup == 0 {
		t.Fatal("沒有進到主戰場")
	}
	work := o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg})
	t.Logf("DGROUP 段 %#06x｜戰場工作區段 %#06x", dgroup, work)

	read := func(seg uint16, off uint16) (uint16, uint16, uint32) {
		lo := o.Word(oracle.Addr{Seg: seg, Off: off})
		hi := o.Word(oracle.Addr{Seg: seg, Off: off + 2})
		return lo, hi, uint32(hi)<<4 + uint32(lo)
	}
	for _, cse := range []struct {
		name string
		off  uint16
	}{
		{"0x2e78（正對照：反白開關）", 0x2e78},
		{"0x20ea", 0x20ea},
		{"0x2e5e", 0x2e5e},
		{"0x314a", 0x314a},
	} {
		wo, ws, wl := read(work, cse.off)
		do, ds, dl := read(dgroup, cse.off)
		t.Logf("%s：工作區 (%#06x, seg %#06x) → 線性 %#07x｜"+
			"DGROUP (%#06x, seg %#06x) → 線性 %#07x",
			cse.name, wo, ws, wl, do, ds, dl)
	}
}
