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
	"github.com/wicanr2/softworld_san1_remake/internal/menu"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
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
	mainItems := ui.TitleItems()
	compareTitleLayer(t, "選擇年代", orig, ts, face, i18n.S("title.pickScenario"), items, "主選擇單", mainItems[:], 62)
}

// compareTitleLayer 拿原版停在主選單某一層的畫面 orig 與 remake 的
// `ui.DrawTitleLayer(label, items)` 比：直牌四格與六條按鈕的每一個半形格
// 比有沒有墨，其餘像素逐格相同（扣掉小飾框與最上面兩個角落）；反對照拿
// negLabel／negItems 畫的那一層比按鈕的墨，必須對不上。minInk 是原版至少要有墨的
// 字格數（停錯地方的正對照：直牌 4 格加上按鈕字的格數）。
func compareTitleLayer(t *testing.T, name string, orig []uint8, ts *ui.TitleScreen, face *font.Face,
	label string, items []string, negLabel string, negItems []string, minInk int) {
	t.Helper()
	c := ui.NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
	ui.DrawTitleLayer(c, ts, -1, label, ui.ScenarioLabelInk, items, -1)
	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		savePNG(t, filepath.Join(dir, "remake-"+name+".png"), c)
	}
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
		t.Fatalf("%s：字格有墨不同 %d／%d，其餘像素不同 %d／%d", name, inkBad, inkN, bad, n)
	}
	if inked < minInk {
		t.Fatalf("%s：原版只有 %d 個字格有墨——停的地方不對", name, inked)
	}
	// 反對照：同一套比法拿主選單那一層的字去比，必須對不上。
	c2 := ui.NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
	ui.DrawTitleLayer(c2, ts, -1, negLabel, ui.ScenarioLabelInk, negItems, -1)
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
		t.Fatalf("%s：反對照的字也比得過——這個比法分不出兩層", name)
	}
	t.Logf("%s：%d 個字格（%d 格有墨）墨相同，其餘 %d 個像素逐格相同；反對照差 %d 格", name, inkN, inked, n, neg)
}

// TestZZLoadPickMatchesTheOriginal 對拍主選單按「2」之後的載入進度那一層
// （Issue #69）：原版停在 `0x14218` 讀鍵存畫面，remake 用原版 `DATA2` 的
// `SAVENAME.SVP` 六筆當按鈕字畫同一層（`docs/spec/005` §6.5）。
func TestZZLoadPickMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	c1 := openContainer(t, filepath.Join(root, "DATA1"))
	c2 := openContainer(t, filepath.Join(root, "DATA2"))
	c3 := openContainer(t, filepath.Join(root, "DATA3"))
	ts, err := ui.NewTitleScreen(c3, c1)
	if err != nil {
		t.Fatal(err)
	}
	names, err := state.LoadSaveNames(c2)
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
	const loadCaller = 0x1421b // `0x14218` call 0x113a4 的返回位址
	asks := 0
	o.OnCall(addr(bootASCInputFn), func(o *oracle.Oracle) {
		if o.Caller().Linear() == loadCaller {
			asks++
		}
	})
	bootToMenu(t, o)
	o.Drain()
	o.PressScan("2")
	waitBoot(t, o, "載入進度輸入", 200_000_000, func() bool { return asks > 0 })
	waitBootScan(t, o, "載入進度", 5_000_000)
	orig := screenOf(o)
	dumpScreen(t, o, "load-pick-orig")

	i18n.Current = i18n.ZhHant
	items := make([]string, len(names))
	for k, n := range names {
		items[k] = menu.LoadLine(save.Info{Slot: k + 1, Exists: true, Name: n})
	}
	t.Logf("原版的六筆名稱：%q", items)
	scen := make([]string, 6)
	for k := range scen {
		scen[k] = i18n.S(fmt.Sprintf("title.scenario%d", k+1))
	}
	compareTitleLayer(t, "載入進度", orig, ts, face, i18n.S("title.loadPlate"), items, i18n.S("title.pickScenario"), scen, 94)
}

// TestZZMusicPickMatchesTheOriginal 對拍主選單按「5」之後的音樂欣賞那一層
// （Issue #70）：原版停在 `0x1468b` 讀鍵存畫面，remake 畫同一層比；接著送「3」，
// 原版要以 2 呼叫 `0x4fb:0x12a`（播第 3 首）並回到主選單讀鍵
// （`docs/spec/005` §6.6）。
func TestZZMusicPickMatchesTheOriginal(t *testing.T) {
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
	const musicCaller = 0x1468e // `0x1468b` call 0x113a4 的返回位址
	asks, menuAsks := 0, 0
	o.OnCall(addr(bootASCInputFn), func(o *oracle.Oracle) {
		switch o.Caller().Linear() {
		case musicCaller:
			asks++
		case 0x11b77: // 主選單那一支的返回位址（`bootMainMenuCaller`）
			menuAsks++
		}
	})
	var played []int
	o.OnCall(oracle.Addr{Seg: 0x4fb, Off: 0x12a}, func(o *oracle.Oracle) {
		played = append(played, int(int16(o.Arg(0))))
	})
	bootToMenu(t, o)
	o.Drain()
	o.PressScan("5")
	waitBoot(t, o, "音樂欣賞輸入", 200_000_000, func() bool { return asks > 0 })
	waitBootScan(t, o, "音樂欣賞", 5_000_000)
	orig := screenOf(o)
	dumpScreen(t, o, "music-pick-orig")

	i18n.Current = i18n.ZhHant
	var items []string
	for k := 1; k <= menu.MusicTracks; k++ {
		items = append(items, i18n.S(fmt.Sprintf("title.song%d", k)))
	}
	items = append(items, "")
	scen := make([]string, 6)
	for k := range scen {
		scen[k] = i18n.S(fmt.Sprintf("title.scenario%d", k+1))
	}
	compareTitleLayer(t, "音樂欣賞", orig, ts, face, i18n.S("title.musicPlate"), items, i18n.S("title.pickScenario"), scen, 34)

	// 開機到主選單時原版已經以 0 叫過一次（主選單的配樂），只看送鍵之後的。
	before, boot := menuAsks, len(played)
	o.Drain()
	o.PressScan("3")
	waitBoot(t, o, "選曲之後回主選單", 200_000_000, func() bool { return menuAsks > before })
	if got := played[boot:]; len(got) != 1 || got[0] != 2 {
		t.Fatalf("送「3」之後 `0x4fb:0x12a` 的參數是 %v（開機時 %v），想要 [2]", got, played[:boot])
	}
	t.Logf("開機時 `0x4fb:0x12a%v`；送「3」之後 `0x4fb:0x12a(%d)`，接著回到主選單讀鍵", played[:boot], played[boot])
}
