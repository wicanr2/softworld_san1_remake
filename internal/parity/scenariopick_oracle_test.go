//go:build oracle

package parity

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// TestZZScenarioPickMatchesTheOriginal 對拍主選單按「1」之後的選擇年代那一層
// （Issue #66）：原版開到主選單、送「1」、停在那一層讀鍵的地方存畫面，
// remake 用 `ui.DrawTitleLayer` 畫同一層，整張比。
//
// 字模不接原版（`CLAUDE.md` §3.3），所以字的格子比「有沒有墨」：直牌四格
// 32×16、六條按鈕各 20 個 8×16 的半形格；其餘像素逐格相同，只扣掉小飾框
// 會動的那一格與最上面兩個角落（`docs/spec/005` §6）。
func TestZZScenarioPickMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	c1 := openContainer(t, filepath.Join(root, "DATA1"))
	c3 := openContainer(t, filepath.Join(root, "DATA3"))
	ts, err := ui.NewTitleScreen(c3, c1)
	if err != nil {
		t.Fatal(err)
	}
	fh, err := os.Open("../../fonts/unifont.hex.gz")
	if err != nil {
		t.Skipf("沒有字型檔：%v", err)
	}
	face, err := font.ParseHexGz(fh, 16)
	fh.Close()
	if err != nil {
		t.Fatal(err)
	}

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	asks := 0
	o.OnCall(addr(bootASCInputFn), func(o *oracle.Oracle) {
		if o.Caller().Linear() == eraCaller {
			asks++
		}
	})
	bootToMenu(t, o)
	o.Drain()
	o.PressScan("1")
	waitBoot(t, o, "選擇年代輸入", 200_000_000, func() bool { return asks > 0 })
	waitBootScan(t, o, "選擇年代", 5_000_000)
	orig := screenOf(o)
	dumpScreen(t, o, "scenario-pick-orig")

	i18n.Current = i18n.ZhHant
	items := make([]string, 6)
	for k := range items {
		items[k] = i18n.S(fmt.Sprintf("title.scenario%d", k+1))
	}
	c := ui.NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
	ui.DrawTitleLayer(c, ts, -1, i18n.S("title.pickScenario"), ui.ScenarioLabelInk, items, -1)

	pal := assets.EGAPalette
	remake := func(x, y int) byte {
		p := c.Img.RGBAAt(x, y)
		for i, q := range pal {
			if q == p {
				return byte(i)
			}
		}
		return 0xFF
	}
	type cell struct{ x, y, w, h int }
	var cellsLabel, cellsText []cell
	for k := 0; k < 4; k++ {
		cellsLabel = append(cellsLabel, cell{assets.MenuLabelX, assets.MenuLabelY + k*assets.MenuLabelPitch, 32, 16})
	}
	for _, b := range assets.MenuButtons() {
		for k := 0; k < ui.TitleLayerTextCells; k++ {
			cellsText = append(cellsText, cell{b[0] + ui.TitleLayerTextX + k*8, b[1] + ui.TitleLayerTextY, 8, 16})
		}
	}
	inCell := make([]bool, scrW*scrH)
	inkBad, inkN, inked := 0, 0, 0
	check := func(cs []cell, fg byte) {
		for _, ce := range cs {
			o, r := false, false
			for y := ce.y; y < ce.y+ce.h; y++ {
				for x := ce.x; x < ce.x+ce.w; x++ {
					inCell[y*scrW+x] = true
					if orig[y*scrW+x]&15 == fg {
						o = true
					}
					if remake(x, y) == fg {
						r = true
					}
				}
			}
			inkN++
			if o {
				inked++
			}
			if o != r {
				inkBad++
				if inkBad <= 8 {
					t.Logf("字格 (%d,%d) 原版有墨 %v、remake %v", ce.x, ce.y, o, r)
				}
			}
		}
	}
	check(cellsLabel, ui.ScenarioLabelInk)
	check(cellsText, ui.TitleLayerTextFG)

	bad, n := 0, 0
	for y := 0; y < scrH; y++ {
		for x := 0; x < scrW; x++ {
			if y == 0 && (x == 0 || x == scrW-1) {
				continue
			}
			if x >= assets.MenuOrnamentX && x < assets.MenuOrnamentX+8 &&
				y >= assets.MenuOrnamentY && y < assets.MenuOrnamentY+16 {
				continue
			}
			i := y*scrW + x
			fgCell := inCell[i]
			if fgCell && (orig[i]&15 == ui.TitleLayerTextFG || orig[i]&15 == ui.ScenarioLabelInk ||
				remake(x, y) == ui.TitleLayerTextFG || remake(x, y) == ui.ScenarioLabelInk) {
				continue // 墨的形狀不比，上面比過格
			}
			n++
			if orig[i]&15 != remake(x, y) {
				bad++
				if bad <= 8 {
					t.Logf("(%d,%d) 原版 %d remake %d", x, y, orig[i]&15, remake(x, y))
				}
			}
		}
	}
	if inkBad != 0 || bad != 0 {
		t.Fatalf("字格有墨不同 %d／%d，其餘像素不同 %d／%d", inkBad, inkN, bad, n)
	}
	if inked < 40 {
		t.Fatalf("原版只有 %d 個字格有墨——停的地方不是選擇年代", inked)
	}
	// 反對照：同一套比法拿主選單那一層的字去比，必須對不上。
	c2 := ui.NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
	mainItems := ui.TitleItems()
	ui.DrawTitleLayer(c2, ts, -1, "主選擇單", ui.ScenarioLabelInk, mainItems[:], -1)
	neg := 0
	for _, ce := range cellsText {
		o, r := false, false
		for y := ce.y; y < ce.y+ce.h; y++ {
			for x := ce.x; x < ce.x+ce.w; x++ {
				o = o || orig[y*scrW+x]&15 == ui.TitleLayerTextFG
				r = r || c2.Img.RGBAAt(x, y) == pal[ui.TitleLayerTextFG]
			}
		}
		if o != r {
			neg++
		}
	}
	if neg == 0 {
		t.Fatal("反對照：主選單的字也比得過——這個比法分不出兩層")
	}
	t.Logf("選擇年代：%d 個字格（%d 格有墨）墨相同，其餘 %d 個像素逐格相同；反對照差 %d 格", inkN, inked, n, neg)
}
