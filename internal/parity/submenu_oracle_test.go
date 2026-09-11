//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestZZSubMenuScreens 把原版九個子選單打開時的畫面各存一張。
//
// 為什麼要量：remake 接原版素材的主畫面（`ui.DrawArtSession`）**先前沒有
// 畫子選單也沒有畫分頁**——打開與沒打開畫出來差 0 個位元組。要補得先
// 知道原版把它們畫在哪、長什麼樣，而那只有原版的畫面答得出來
//（`docs/re/04` §2 只給字串，不給位置）。
//
// 這一支是探索用的：只存圖、印出每一次打開之後**哪一塊變了**（外框），
// 不斷言。存圖要設 `SAN1_SHOTS`。
func TestZZSubMenuScreens(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, _, _ := sc.Tables()
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	bootToMain(t, o, mas)
	snap := o.Save()
	const settle = 50_000_000
	base, mask := blinkMask(o, snap, settle, 3)
	dumpScreen(t, o, "sub-0-主畫面")

	bbox := func(a, b []uint8) string {
		x0, y0, x1, y1, n := scrW, scrH, -1, -1, 0
		for i := range a {
			if i >= len(b) || a[i] == b[i] || mask[i] {
				continue
			}
			n++
			x, y := i%scrW, i/scrW
			x0, y0 = min(x0, x), min(y0, y)
			x1, y1 = max(x1, x), max(y1, y)
		}
		if n == 0 {
			return "沒有變"
		}
		return fmt.Sprintf("差 %d 點，範圍 x %d–%d、y %d–%d", n, x0, x1, y0, y1)
	}
	for k := 1; k <= 9; k++ {
		o.Restore(snap)
		o.Drain()
		o.TypeBoth(fmt.Sprintf("%d\r", k))
		if err := o.Run(settle); err != nil {
			t.Fatalf("類別 %d：%v", k, err)
		}
		t.Logf("類別 %d：%s", k, bbox(base, screenOf(o)))
		dumpScreen(t, o, fmt.Sprintf("sub-%d", k))
	}
	// 一個分頁：查看 → 將軍列表。
	for _, step := range []struct{ name, keys string }{
		{"sub-1-2-將軍列表", "2\r"},
	} {
		o.Restore(snap)
		o.Drain()
		o.TypeBoth("1\r")
		_ = o.Run(settle)
		o.Drain()
		o.TypeBoth(step.keys)
		if err := o.Run(settle * 2); err != nil {
			t.Fatalf("%s：%v", step.name, err)
		}
		t.Logf("%s：%s", step.name, bbox(base, screenOf(o)))
		dumpScreen(t, o, step.name)
	}
}
