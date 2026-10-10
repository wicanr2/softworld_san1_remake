//go:build oracle

package parity

import (
	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"path/filepath"
	"testing"
)

// Observe the original normal command screen. Compare every map pixel;
// the input cursor is outside this region and is not selection evidence.
func TestOriginalCurrentPrefectureBlink(t *testing.T) {
	for _, edition := range []string{"base", "plus"} {
		t.Run(edition, func(t *testing.T) {
			root, exe := origRoot(t), "AA.EXE"
			if edition == "plus" {
				root, exe = plusRoot(t), "ASV.EXE"
			}
			c := openContainer(t, filepath.Join(root, "DATA2"))
			sc, err := state.LoadScenario(c, state.Scenario1)
			if err != nil {
				t.Fatal(err)
			}
			o, err := oracle.Load(filepath.Join(root, exe), root)
			if err != nil {
				t.Fatal(err)
			}
			defer o.Close()
			if edition == "plus" {
				bootToNewGamePlus(t, o, 2, 5)
			} else {
				mas, _, _ := sc.Tables()
				bootToNewGame(t, o, 2, mas)
			}
			first := screenOf(o)
			dumpScreen(t, o, "current-prefecture-"+edition+"-first")
			var next uint64
			changed := oracle.NewCond("current prefecture changes phase", func(o *oracle.Oracle) bool {
				if o.Steps() < next {
					return false
				}
				next = o.Steps() + 100_000
				current := screenOf(o)
				for y := 36; y < 372; y++ {
					for x := 72; x < 408; x++ {
						i := y*scrW + x
						if first[i] != current[i] {
							return true
						}
					}
				}
				return false
			})
			if err := o.RunUntil(changed, oracle.Budget(100_000_000)); err != nil {
				t.Fatal(err)
			}
			second := screenOf(o)
			dumpScreen(t, o, "current-prefecture-"+edition+"-second")
			matched := 0
			for _, p := range sc.Prefectures() {
				x0, y0 := int(p.MapX)+assets.MapOriginX, int(p.MapY)+assets.MapOriginY
				ok := true
				for y := 36; y < 372; y++ {
					for x := 72; x < 408; x++ {
						i := y*scrW + x
						want := first[i]
						if x >= x0 && x < x0+16 && y >= y0 && y < y0+9 {
							want ^= 15
						}
						if second[i] != want {
							ok = false
						}
					}
				}
				if ok {
					matched++
					t.Logf("%s: prefecture=%d rectangle=(%d,%d,16,9), complete indexed XOR 15", edition, p.ID, x0, y0)
				}
			}
			if matched != 1 {
				t.Fatalf("whole map must match one original 16x9 marker; matches=%d", matched)
			}
		})
	}
}
