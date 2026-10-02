//go:build oracle

package parity

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

type dateImageCall struct {
	name       string
	x, y, w, h int
}

// watchMainDate 量原版自己的載圖名與實際繪圖參數，不從缺字的最終畫面猜座標。
// 兩個入口是 AA.EXE 的執行期線性位址，不跨版本套用（docs/spec/005 §2.2）。
func watchMainDate(t *testing.T, o *oracle.Oracle, c3 *assets.Container) *[]dateImageCall {
	t.Helper()
	var calls []dateImageCall
	pending := ""
	o.OnCall(addr(imgLoadFn), func(o *oracle.Oracle) {
		pending = ""
		name := cstr(o, uint32(o.Arg(1))*16+uint32(o.Arg(0)))
		if strings.HasPrefix(name, "CP") && strings.HasSuffix(name, ".IMG") {
			pending = name
		}
	})
	o.OnCall(addr(0x36c9*16+0x4c8), func(o *oracle.Oracle) {
		if pending == "" || o.Arg(0) != 24 {
			return
		}
		i, ok := c3.ByName(pending)
		if !ok {
			t.Fatalf("原版載了 %s，DATA3 沒有該項", pending)
		}
		im, err := assets.DecodeImage(c3.Data(i))
		if err != nil {
			t.Fatal(err)
		}
		calls = append(calls, dateImageCall{pending, int(o.Arg(0)), int(o.Arg(1)), im.W, im.H})
		pending = ""
	})
	return &calls
}

// TestZZMainDateMatchesTheOriginal：正常開新局的十次日期繪圖是主要收據；
// 年數及月份邊界由同一開局快照直接呼叫日期函式，只證明局部排版。
// 自建字模比有墨／空白及框外底圖，不宣稱原版日期像素完全相同。
func TestZZMainDateMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	c3 := openContainer(t, filepath.Join(root, "DATA3"))
	sc, err := state.LoadScenario(openContainer(t, filepath.Join(root, "DATA2")), state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	mas, _, _ := sc.Tables()
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	calls := watchMainDate(t, o, c3)
	bootToNewGame(t, o, caoCaoPick, mas)
	if len(*calls) != 10 {
		t.Fatalf("正常開新局日期繪圖命中 %d 次，預期 10 次", len(*calls))
	}
	snap := o.Save()
	art, err := ui.NewArtScreen(c3, openContainer(t, filepath.Join(root, "DATA1")))
	if err != nil {
		t.Fatal(err)
	}
	g, err := game.New(sc, 1, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	face := loadFace(t)
	for _, tc := range []struct {
		date  game.Date
		codes [10]int
	}{
		{game.Date{Year: 189, Month: 1}, [10]int{146, 151, 171, 134, 171, 139, 171, 169, 140, 142}},
		{game.Date{Year: 197, Month: 9}, [10]int{147, 148, 171, 130, 171, 139, 171, 137, 140, 144}},
		{game.Date{Year: 205, Month: 12}, [10]int{147, 148, 171, 138, 171, 139, 138, 130, 140, 145}},
		{game.Date{Year: 206, Month: 12}, [10]int{147, 148, 171, 138, 129, 139, 138, 130, 140, 145}},
		{game.Date{Year: 216, Month: 12}, [10]int{147, 148, 130, 138, 129, 139, 138, 130, 140, 145}},
	} {
		name := fmt.Sprintf("%03d-%02d", tc.date.Year, tc.date.Month)
		t.Run(name, func(t *testing.T) {
			if tc.date != (game.Date{Year: 189, Month: 1}) {
				o.Restore(snap)
				*calls = nil
				if _, err := o.CallBudget(50_000_000, addr(0x12720), uint16(tc.date.Year), uint16(tc.date.Month)); err != nil {
					t.Fatal(err)
				}
			}
			if len(*calls) != 10 {
				t.Fatalf("日期繪圖命中 %d 次，預期 10 次", len(*calls))
			}
			orig := screenOf(o)
			g.Date = tc.date
			cv := ui.NewCanvasPx(scrW, scrH, face)
			ui.DrawArtSession(cv, art, g, nil, ui.View{})
			var ink [10]int
			bad := 0
			for y := 36; y < 372; y++ {
				for x := 0; x < 72; x++ {
					inBox := false
					for i, b := range *calls {
						if x >= b.x && x < b.x+b.w && y >= b.y && y < b.y+b.h {
							inBox = true
							if cv.Img.RGBAAt(x, y) == assets.EGAPalette[0] {
								ink[i]++
							}
							break
						}
					}
					if !inBox && cv.Img.RGBAAt(x, y) != assets.EGAPalette[orig[y*scrW+x]&15] {
						bad++
					}
				}
			}
			for i, b := range *calls {
				want := dateImageCall{fmt.Sprintf("CP%03d.IMG", tc.codes[i]), 24, 65 + 28*i, 24, 24}
				if b != want {
					t.Errorf("日期槽 %d：原版 %+v，預期 %+v", i, b, want)
				}
				if (ink[i] > 0) != (tc.codes[i] != 171) {
					t.Errorf("日期槽 %d：原版 %s，remake 墨點 %d", i, b.name, ink[i])
				}
			}
			if bad != 0 {
				t.Errorf("日期字框外底圖有 %d 像素不同", bad)
			}
			dumpScreen(t, o, "date-base-"+name+"-original")
			if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
				savePNG(t, filepath.Join(dir, "date-base-"+name+"-remake.png"), cv)
			}
			t.Logf("原版十次呼叫=%+v；remake 墨點=%v；字框外差=%d；原版畫面 SHA-256=%x", *calls, ink, bad, sha256.Sum256(orig))
		})
	}
}
