//go:build oracle

package parity

import (
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// TestZZLordPickScreenMatchesTheOriginal 把原版開到選君主那一格（劇本一、
// 第一頁六位），對 remake 的 `ui.DrawLordPick`：地圖、外框、提示框、六張
// 肖像、彩色空心框、勢力色塊**逐像素相同**；名字、編號與提示字只比
// 「有沒有墨」（字模不比）；左側直條的年月不比。
func TestZZLordPickScreenMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	bootToLordPick(t, o)
	orig := o.IndexedEGASize(scrW, scrH)
	dumpScreen(t, o, "lordpick")

	g, err := game.New(sc, 0, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	art, err := ui.NewArtScreen(openContainer(t, filepath.Join(root, "DATA3")),
		openContainer(t, filepath.Join(root, "DATA1")))
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
	var slots []ui.LordPickSlot
	for f := 0; f < ui.LordPickPerPage; f++ {
		slots = append(slots, ui.LordPickSlot{Number: f + 1, Faction: f, Lord: g.Lord(state.FactionID(f))})
	}
	cv := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawLordPick(cv, art, g, slots, -1, "第1位,請選擇(1-16):", 0)
	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		savePNG(t, filepath.Join(dir, "remake-lordpick.png"), cv)
	}

	// 文字格外圍多留一像素：白字黑邊的邊會壓出格子一點，字模不同壓出去的
	// 位置也不同。
	type box struct{ x0, y0, x1, y1 int }
	var text []box
	for i := 0; i < ui.LordPickPerPage; i++ {
		x, y := 420+68*(i%3), 56+128*(i/3)
		text = append(text, box{x + 15, y + 83, x + 64, y + 116}, box{x - 3, y + 99, x + 14, y + 116})
	}
	text = append(text, box{423, 331, 624, 348})
	inBox := func(x, y int) int {
		for i, b := range text {
			if x >= b.x0 && x <= b.x1 && y >= b.y0 && y <= b.y1 {
				return i
			}
		}
		return -1
	}
	idx := func(c color.RGBA) int {
		for i, p := range assets.EGAPalette {
			if p == c {
				return i
			}
		}
		return -1
	}
	bad, first := 0, ""
	inkOrig, inkMine := make([]int, len(text)), make([]int, len(text))
	for y := 0; y < scrH; y++ {
		for x := 72; x < scrW; x++ {
			op, mp := int(orig[y*scrW+x]&15), idx(cv.Img.RGBAAt(x, y))
			if b := inBox(x, y); b >= 0 {
				// 字模不比：只數白字（名字、編號）或淺綠字（提示）的墨。
				want := 15
				if b == len(text)-1 {
					want = 10
				}
				if op == want {
					inkOrig[b]++
				}
				if mp == want {
					inkMine[b]++
				}
				continue
			}
			if op != mp {
				bad++
				if first == "" {
					first = fmt.Sprintf("(%d,%d) 原版 %d remake %d", x, y, op, mp)
				}
			}
		}
	}
	if bad != 0 {
		t.Errorf("地圖／外框／肖像／色塊有 %d 個像素不同，第一個 %s", bad, first)
	}
	for i := range text {
		if (inkOrig[i] == 0) != (inkMine[i] == 0) {
			t.Errorf("文字格 %d：原版有墨 %d 點、remake %d 點——一邊沒字", i, inkOrig[i], inkMine[i])
		}
	}
	t.Logf("文字格墨點 原版 %v remake %v", inkOrig, inkMine)
}

// savePNG 把 remake 的畫布存成 PNG（與 dumpScreen 放同一個目錄，好並排看）。
func savePNG(t *testing.T, path string, cv *ui.Canvas) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Log(err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, cv.Img); err != nil {
		t.Log(err)
	}
}

// customDrawnAt 是新君主那一格六行字畫完、要進輸入常式 `0x13524` 之前的
// 那一道（`0x13150`）；lordPickFn 是畫一頁候選的 `0x124ca`。
const (
	customDrawnAt = 0x13150
	lordPickFn    = 0x124ca
)

// TestZZCustomLordScreenMatchesTheOriginal 把原版開到新君主那一格（劇本一
// 的第 15 位是空的新君主欄），對 remake 的 `ui.DrawCustomLord`：地圖、外框、
// 肖像與框**逐像素相同**；名字、六行字與提示框只比「有沒有墨」（停在
// `0x13150` 時提示框裡還是上一個難度提示，輸入常式之後才寫自己那兩行）。
func TestZZCustomLordScreenMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	customs := sc.CustomLordSlots()
	if len(customs) == 0 {
		t.Skip("劇本一沒有空的新君主欄")
	}
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	d := bootToLordPick(t, o)
	drawn, pages := 0, 0
	o.OnCall(addr(customDrawnAt), func(*oracle.Oracle) { drawn++ })
	o.OnCall(addr(lordPickFn), func(*oracle.Oracle) { pages++ })
	// 一頁六位；空白鍵翻頁（數字輸入常式 `0x34fdd`：空白且沒打數字 → 0xFFFE），
	// 翻到新君主欄那一頁再打編號。
	for customs[0]/ui.LordPickPerPage > 0 && pages < customs[0]/ui.LordPickPerPage {
		before := pages
		o.Drain()
		o.PressScan(" ")
		waitBoot(t, o, "選君主翻頁", 200_000_000, func() bool { return pages > before })
		waitBootScan(t, o, "選君主翻頁後", 5_000_000)
	}
	dumpScreen(t, o, "customlord-page")
	// 選了新君主欄之後先問難度，新君主那一格在難度之後才畫。
	o.Drain()
	o.PressScan(fmt.Sprintf("%d\r", customs[0]+1))
	d.waitNum("新局難度輸入", 1, 10)
	o.Drain()
	o.TypeBoth("5\r")
	waitBoot(t, o, "新君主那一格畫完", 300_000_000, func() bool { return drawn > 0 })
	dumpScreen(t, o, "customlord")
	orig := o.IndexedEGASize(scrW, scrH)

	g, err := game.New(sc, 0, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	art, err := ui.NewArtScreen(openContainer(t, filepath.Join(root, "DATA3")),
		openContainer(t, filepath.Join(root, "DATA1")))
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
	cv := ui.NewCanvasPx(scrW, scrH, face)
	lines := [6]string{"1.體能: 80", "2.謀略: 50", "3.戰力: 50", "4.魅力: 50", "5.領地:遼東", "6.完成 剩100"}
	ui.DrawCustomLord(cv, art, g, customs[0], state.CustomLordPortrait[0], "新君主", lines, -1,
		[2]string{"剩餘點數:100", "更改(1-7,0-結束):"}, 0)
	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		savePNG(t, filepath.Join(dir, "remake-customlord.png"), cv)
	}

	type box struct{ x0, y0, x1, y1 int }
	text := []box{{431, 55, 527, 196}, {523, 154, 623, 207}, {423, 331, 624, 364}}
	inBox := func(x, y int) int {
		for i, b := range text {
			if x >= b.x0 && x <= b.x1 && y >= b.y0 && y <= b.y1 {
				return i
			}
		}
		return -1
	}
	idx := func(c color.RGBA) int {
		for i, p := range assets.EGAPalette {
			if p == c {
				return i
			}
		}
		return -1
	}
	bad, first := 0, ""
	inkOrig, inkMine := make([]int, len(text)), make([]int, len(text))
	for y := 0; y < scrH; y++ {
		for x := 72; x < scrW; x++ {
			op, mp := int(orig[y*scrW+x]&15), idx(cv.Img.RGBAAt(x, y))
			if b := inBox(x, y); b >= 0 {
				// 字模不比：底圖是 14／7 的雜訊（提示框是 3），其他色都算墨。
				if op != 14 && op != 7 && op != 3 {
					inkOrig[b]++
				}
				if mp != 14 && mp != 7 && mp != 3 {
					inkMine[b]++
				}
				continue
			}
			if op != mp {
				bad++
				if first == "" {
					first = fmt.Sprintf("(%d,%d) 原版 %d remake %d", x, y, op, mp)
				}
			}
		}
	}
	if bad != 0 {
		t.Errorf("地圖／外框／肖像／框有 %d 個像素不同，第一個 %s", bad, first)
	}
	for i := range text {
		if (inkOrig[i] == 0) != (inkMine[i] == 0) {
			t.Errorf("文字區 %d：原版有墨 %d 點、remake %d 點——一邊沒字", i, inkOrig[i], inkMine[i])
		}
	}
	t.Logf("文字區墨點 原版 %v remake %v", inkOrig, inkMine)
}

// TestZZDifficultyScreenMatchesTheOriginal 把原版開到選君主那一格、選第 1 位，
// 停在「請設定難度(1-10)」的數字輸入（Issue #67）。原版畫面還是選君主那一頁，
// 只多了兩處：選中那一位的肖像下緣印玩家序號、提示框換字。比法：
//
//   - 名字、編號、提示框與序號那一塊以外：整張逐像素相同（左側年月不比）
//   - 提示框：淺綠 10 的墨兩邊都有
//   - 序號：原版「選君主 → 設難度」變了的像素，與 remake「沒序號 → 有序號」變了
//     的像素，兩邊都有、外框重疊
func TestZZDifficultyScreenMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	sc, err := state.LoadScenario(openContainer(t, filepath.Join(root, "DATA2")), state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	d := bootToLordPick(t, o)
	before := append([]uint8(nil), o.IndexedEGASize(scrW, scrH)...)
	o.Drain()
	o.TypeBoth("1\r")
	d.waitNum("設難度輸入", 1, 10)
	after := append([]uint8(nil), o.IndexedEGASize(scrW, scrH)...)
	dumpScreen(t, o, "difficulty")

	g, err := game.New(sc, 0, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	art, err := ui.NewArtScreen(openContainer(t, filepath.Join(root, "DATA3")),
		openContainer(t, filepath.Join(root, "DATA1")))
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
	render := func(player int, prompt string) *ui.Canvas {
		var slots []ui.LordPickSlot
		for f := 0; f < ui.LordPickPerPage; f++ {
			s := ui.LordPickSlot{Number: f + 1, Faction: f, Lord: g.Lord(state.FactionID(f))}
			if f == 0 {
				s.Player = player
			}
			slots = append(slots, s)
		}
		cv := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawLordPick(cv, art, g, slots, -1, prompt, 0)
		return cv
	}
	noMark := render(0, "第1位,請選擇(1-16):")
	cv := render(1, "請設定難度(1-10):")
	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		savePNG(t, filepath.Join(dir, "remake-difficulty.png"), cv)
	}
	idx := func(c color.RGBA) int {
		for i, p := range assets.EGAPalette {
			if p == c {
				return i
			}
		}
		return -1
	}
	type box struct{ x0, y0, x1, y1 int }
	var text []box
	for i := 0; i < ui.LordPickPerPage; i++ {
		x, y := 420+68*(i%3), 56+128*(i/3)
		text = append(text, box{x + 15, y + 83, x + 64, y + 116}, box{x - 3, y + 99, x + 14, y + 116})
	}
	prompt := box{423, 331, 624, 348}
	mark := box{420 + 15, 56 + 72, 420 + 16 + 64, 56 + 90}
	in := func(b box, x, y int) bool { return x >= b.x0 && x <= b.x1 && y >= b.y0 && y <= b.y1 }
	bad, first := 0, ""
	promptOrig, promptMine := 0, 0
	var origMark, mineMark []int
	for y := 0; y < scrH; y++ {
		for x := 72; x < scrW; x++ {
			i := y*scrW + x
			op, mp := int(after[i]&15), idx(cv.Img.RGBAAt(x, y))
			switch {
			case in(prompt, x, y):
				if op == 10 {
					promptOrig++
				}
				if mp == 10 {
					promptMine++
				}
				continue
			case in(mark, x, y):
				if after[i] != before[i] {
					origMark = append(origMark, x, y)
				}
				if cv.Img.RGBAAt(x, y) != noMark.Img.RGBAAt(x, y) {
					mineMark = append(mineMark, x, y)
				}
				continue
			}
			skip := false
			for _, b := range text {
				if in(b, x, y) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			if op != mp {
				bad++
				if first == "" {
					first = fmt.Sprintf("(%d,%d) 原版 %d remake %d", x, y, op, mp)
				}
			}
		}
	}
	if bad != 0 {
		t.Errorf("框外有 %d 個像素不同，第一個 %s", bad, first)
	}
	if promptOrig == 0 || promptMine == 0 {
		t.Errorf("提示框的墨：原版 %d 點、remake %d 點", promptOrig, promptMine)
	}
	bbox := func(p []int) box {
		b := box{scrW, scrH, -1, -1}
		for k := 0; k+1 < len(p); k += 2 {
			b.x0, b.y0 = min(b.x0, p[k]), min(b.y0, p[k+1])
			b.x1, b.y1 = max(b.x1, p[k]), max(b.y1, p[k+1])
		}
		return b
	}
	ob, mb := bbox(origMark), bbox(mineMark)
	if len(origMark) == 0 || len(mineMark) == 0 || ob.x1 < mb.x0 || mb.x1 < ob.x0 || ob.y1 < mb.y0 || mb.y1 < ob.y0 {
		t.Errorf("玩家序號：原版變了 %d 點 %+v，remake %d 點 %+v", len(origMark)/2, ob, len(mineMark)/2, mb)
	}
	t.Logf("框外逐像素相同；提示框墨 原版 %d remake %d；序號 原版 %+v remake %+v", promptOrig, promptMine, ob, mb)
}
