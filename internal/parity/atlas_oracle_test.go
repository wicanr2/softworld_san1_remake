//go:build oracle

package parity

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// 郡地理誌（查看 5，`0x185a6`）與「新君主出現!!」（`0x133f2`）——訊息常式
// 剩下的兩張整頁畫面（Issue #60，`docs/spec/005` §9.5／§9.8）。
const (
	atlasFn    = 0x185a6 // 查看 5：整張畫在顯示記憶體的第二頁再切過去
	atlasMsgAt = 0x18663 // 主事者那一句的呼叫點
	atlasPage  = 0x8000  // 第二頁在每個平面的位移（640×408 一頁 0x7f80，對齊到 0x8000）
	readKeyFn  = 0x113a4 // `1058:0e24` 讀一個鍵的共用常式（等鍵的判準）
	newLordMsg = 0x133f2 // 自創君主收尾那一句
)

func inkIndex(c color.RGBA) int {
	for i, p := range assets.EGAPalette {
		if p == c {
			return i
		}
	}
	return -1
}

// compareFullScreen 逐像素比整張畫面（x 從 x0 起），text 裡的矩形只比
// 「兩邊都有墨／都沒有墨」（墨 ＝ 不是 paper 裡的顏色）。
func compareFullScreen(t *testing.T, name string, orig []byte, cv *ui.Canvas, x0 int,
	text [][4]int, paper map[int]bool) {
	t.Helper()
	inBox := func(x, y int) int {
		for i, r := range text {
			if x >= r[0] && x <= r[2] && y >= r[1] && y <= r[3] {
				return i
			}
		}
		return -1
	}
	bad, first := 0, ""
	inkOrig, inkMine := make([]int, len(text)), make([]int, len(text))
	for y := 0; y < scrH; y++ {
		for x := x0; x < scrW; x++ {
			op, mp := int(orig[y*scrW+x]&15), inkIndex(cv.Img.RGBAAt(x, y))
			if b := inBox(x, y); b >= 0 {
				if !paper[op] {
					inkOrig[b]++
				}
				if !paper[mp] {
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
		t.Errorf("%s：字以外有 %d 個像素不同，第一個 %s", name, bad, first)
	} else {
		t.Logf("%s：字以外逐像素相同", name)
	}
	for i := range text {
		if inkOrig[i] == 0 {
			t.Errorf("%s：原版的文字區 %d %v 沒有墨", name, i, text[i])
		}
		if inkMine[i] == 0 {
			t.Errorf("%s：remake 的文字區 %d %v 沒有墨", name, i, text[i])
		}
	}
}

func loadFace(t *testing.T) *font.Face {
	t.Helper()
	fh, err := os.Open("../../fonts/unifont.hex.gz")
	if err != nil {
		t.Skipf("沒有字型檔：%v", err)
	}
	defer fh.Close()
	face, err := font.ParseHexGz(fh, 16)
	if err != nil {
		t.Fatal(err)
	}
	return face
}

// TestZZAtlasMatchesTheOriginal 把原版的「查看→5 郡地理誌」畫在第二頁的
// 那一張讀出來，與 remake 的 `DrawArtAtlas` ＋ 主事者那一句比：底紋、花邊、
// 場地、第三塊面板、肖像、泡泡逐像素相同；通道編號與對白只比有沒有墨。
func TestZZAtlasMatchesTheOriginal(t *testing.T) {
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

	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	live := o.Bytes(addr(base), nMas+nSta+nGen)
	sc, err := state.DecodeTables(state.Slot("001"), live[:nMas], live[nMas:nMas+nSta], live[nMas+nSta:])
	if err != nil {
		t.Fatal(err)
	}
	players := sc.Players()
	if len(players) == 0 {
		t.Skip("這個盤面沒有玩家")
	}
	g, err := game.New(sc, state.FactionID(players[0]), 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}

	hits, colour := map[string]int{}, -1
	o.OnCall(addr(atlasFn), func(*oracle.Oracle) { hits["地理誌"]++ })
	o.OnCall(addr(atlasMsgAt), func(*oracle.Oracle) { hits["訊息"]++ })
	o.OnCall(addr(msgRndAt), func(o *oracle.Oracle) { colour = int(o.Regs().AX) })
	// 主畫面：1（查看）→ 查看選單 5（郡地理誌）；看的是游標所在的郡。
	for _, k := range []string{"1", "\r", "5", "\r"} {
		o.Drain()
		o.PressScan(k)
		if err := o.Run(30_000_000); err != nil {
			t.Fatal(err)
		}
	}
	if hits["訊息"] == 0 {
		t.Fatalf("沒走到地理誌那一句：%v", hits)
	}
	if colour < 0 {
		t.Fatal("字色那一擲沒攔到")
	}
	// 游標所在的郡：原版讀 `es:0x31be`；remake 這邊用玩家的第一個郡。
	at := 0
	for id := 1; id <= 42; id++ {
		if p := g.Prefecture(id); p != nil && int(p.Owner) == players[0] {
			at = id
			break
		}
	}
	if at == 0 {
		t.Fatal("玩家沒有郡")
	}
	orig := o.IndexedEGAFrom(atlasPage, scrW, scrH)
	dumpScreen(t, o, "atlas-page0")
	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		cv := ui.NewCanvasPx(scrW, scrH, loadFace(t))
		for y := 0; y < scrH; y++ {
			for x := 0; x < scrW; x++ {
				cv.Img.SetRGBA(x, y, assets.EGAPalette[orig[y*scrW+x]&15])
			}
		}
		savePNG(t, filepath.Join(dir, "orig-atlas.png"), cv)
	}

	ab, err := ui.NewArtBattle(openContainer(t, filepath.Join(root, "DATA1")),
		openContainer(t, filepath.Join(root, "DATA3")))
	if err != nil {
		t.Fatal(err)
	}
	art, err := ui.NewArtScreen(openContainer(t, filepath.Join(root, "DATA3")),
		openContainer(t, filepath.Join(root, "DATA1")))
	if err != nil {
		t.Fatal(err)
	}
	cv := ui.NewCanvasPx(scrW, scrH, loadFace(t))
	p := g.Prefecture(at)
	fld := g.Field(at)
	ui.DrawArtAtlas(cv, ab, p.BattleField, fld)
	b := g.AtlasBubble(at)
	if b == nil {
		t.Fatalf("郡 %d 沒有主事者", at)
	}
	b.Color = colour
	ui.DrawBubble(cv, art, g, b)
	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		savePNG(t, filepath.Join(dir, "remake-atlas.png"), cv)
	}
	t.Logf("郡 %d %s：主事者 %s，字色 %d，通道 %d 處", at, p.Name, g.General(b.Speaker).Name, colour, len(fld.Gates))

	// 字：通道編號那幾格（左上角加 (16,15) 的 16×16）、對白兩行、名字。
	var text [][4]int
	for _, hs := range fld.Gates {
		for _, h := range hs {
			col, row := battle.ToOffset(h)
			x, y := assets.FieldCell(col, row)
			text = append(text, [4]int{x + 16, y + 15, x + 31, y + 30})
		}
	}
	text = append(text,
		[4]int{game.AtlasBubbleX1 + 8, game.AtlasBubbleY1 + 12, game.AtlasBubbleX2 - 70, game.AtlasBubbleY1 + 84},
		[4]int{game.AtlasBubbleX2 - 55, game.AtlasBubbleY1 + 80, game.AtlasBubbleX2 - 8, game.AtlasBubbleY1 + 95})
	compareFullScreen(t, "郡地理誌", orig, cv, 0, text, map[int]bool{7: true, 15: true, 0: true})
}

// TestZZNewLordBornMatchesTheOriginal 把原版的自創君主走到「新君主出現!!」
// 那一格（分完能力送 0），與 remake 的 `DrawNewLordBorn` 比：地圖、外框、
// 泡泡、肖像逐像素相同；提示、名字、對白只比有沒有墨。
func TestZZNewLordBornMatchesTheOriginal(t *testing.T) {
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
	drawn, pages, said, colour := 0, 0, 0, -1
	o.OnCall(addr(customDrawnAt), func(*oracle.Oracle) { drawn++ })
	o.OnCall(addr(lordPickFn), func(*oracle.Oracle) { pages++ })
	o.OnCall(addr(newLordMsg), func(*oracle.Oracle) { said++ })
	o.OnCall(addr(msgRndAt), func(o *oracle.Oracle) { colour = int(o.Regs().AX) })
	for customs[0]/ui.LordPickPerPage > 0 && pages < customs[0]/ui.LordPickPerPage {
		before := pages
		o.Drain()
		o.PressScan(" ")
		waitBoot(t, o, "選君主翻頁", 200_000_000, func() bool { return pages > before })
		waitBootScan(t, o, "選君主翻頁後", 5_000_000)
	}
	o.Drain()
	o.PressScan(fmt.Sprintf("%d\r", customs[0]+1))
	d.waitNum("新局難度輸入", 1, 10)
	o.Drain()
	o.TypeBoth("5\r")
	waitBoot(t, o, "新君主那一格畫完", 300_000_000, func() bool { return drawn > 0 })
	// 「更改(1-7,0-結束):」——讀鍵常式 `1058:0e24` 進門先把緩衝裡的鍵
	// 全吃掉再等（`0x113ae`–`0x113c7`），六行字畫完就送的鍵會被吃掉；
	// 要等到數字輸入 `0x115e(0,7)` 真的在等鍵再送。**點數要用完 0 才收工**
	// （`0x13826`：剩餘點數 ≠ 0 → 回到提示重問，`0x1381e` 才是收工那一條）；
	// 每改一項就整張重畫再回到提示。上限：體能 95、其餘 90（`0x1358e`…），
	// 下限是傳給數字輸入的 50／5／5／20。100 點分成 15＋40＋40＋5。
	reads := 0
	o.OnCall(addr(readKeyFn), func(*oracle.Oracle) { reads++ })
	for _, step := range []struct {
		lo, hi int
		key    string
	}{
		{0, 7, "2\r"}, {50, 95, "95\r"},
		{0, 7, "3\r"}, {5, 90, "90\r"},
		{0, 7, "4\r"}, {5, 90, "90\r"},
		{0, 7, "5\r"}, {20, 55, "55\r"},
		{0, 7, "0\r"},
	} {
		d.waitNum(fmt.Sprintf("新君主輸入 %d-%d", step.lo, step.hi), step.lo, step.hi)
		o.Drain()
		o.TypeBoth(step.key)
	}
	waitBoot(t, o, "新君主出現那一句", 300_000_000, func() bool { return said > 0 && reads > 0 })
	waitBootScan(t, o, "畫完等鍵", 2_000_000)
	if colour < 0 {
		t.Fatal("字色那一擲沒攔到")
	}
	dumpScreen(t, o, "newlord-born")
	orig := o.IndexedEGASize(scrW, scrH)

	// 地圖上那一郡已經是新君主的顏色：領地照 remake 的預設（第一個空白郡，
	// 原版這一格的「6.領地」也是 1），點數照上面那四筆。
	lord := state.CustomLord{Stamina: 15, Intellect: 40, Might: 40, Charm: 5}
	for _, p := range sc.Prefectures() {
		if p.ID > 0 && !p.Owned() {
			lord.Prefecture = p.ID
			break
		}
	}
	copy(lord.Name[:], []rune("新君主"))
	sc, err = sc.WithCustomLord(customs[0], lord)
	if err != nil {
		t.Fatal(err)
	}
	g, err := game.New(sc, state.FactionID(customs[0]), 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	art, err := ui.NewArtScreen(openContainer(t, filepath.Join(root, "DATA3")),
		openContainer(t, filepath.Join(root, "DATA1")))
	if err != nil {
		t.Fatal(err)
	}
	// 名字從局面裡取（人物表裡是造字碼位，`state.ShowCustomGlyphs` 換成字模畫的字，
	// Issue #72），不是寫死的字串——畫面拿到的就是遊戲裡那一條路。
	who := g.Lord(state.FactionID(customs[0]))
	if who == nil {
		t.Fatal("新君主沒有上盤面")
	}
	cv := ui.NewCanvasPx(scrW, scrH, loadFace(t))
	ui.DrawNewLordBorn(cv, art, g, state.CustomLordPortrait[0], who.Name, colour, 0)
	// 名字那一格比「有墨的直欄數」：原版的字模與 remake 的字庫形狀不同，但三個
	// 全形字各占滿一格；畫成標點「，、。」的只有左下角幾欄。反對照必須對不上。
	punct := ui.NewCanvasPx(scrW, scrH, loadFace(t))
	ui.DrawNewLordBorn(punct, art, g, state.CustomLordPortrait[0], "，、。", colour, 0)
	nameBox := [4]int{game.BubbleX2 - 55, 66 + 80, game.BubbleX2 - 8, 66 + 95}
	inkCols := func(pix func(x, y int) int) int {
		n := 0
		for x := nameBox[0]; x <= nameBox[2]; x++ {
			for y := nameBox[1]; y <= nameBox[3]; y++ {
				if pix(x, y) != 0 {
					n++
					break
				}
			}
		}
		return n
	}
	canvasIdx := func(c *ui.Canvas) func(x, y int) int {
		return func(x, y int) int {
			p := c.Img.RGBAAt(x, y)
			for i, q := range assets.EGAPalette {
				if q == p {
					return i
				}
			}
			return -1
		}
	}
	oc := inkCols(func(x, y int) int { return int(orig[y*scrW+x] & 15) })
	mc, pc := inkCols(canvasIdx(cv)), inkCols(canvasIdx(punct))
	t.Logf("名字 %q：有墨的直欄 原版 %d、remake %d、標點反對照 %d（共 %d 欄）", who.Name, oc, mc, pc, nameBox[2]-nameBox[0]+1)
	near := func(a, b int) bool { return a*4 >= b*3 && b*4 >= a*3 }
	if !near(oc, mc) {
		t.Errorf("名字的有墨直欄 原版 %d、remake %d，差超過四分之一", oc, mc)
	}
	if near(oc, pc) {
		t.Errorf("反對照：畫成「，、。」也有 %d 欄，這個比法分不出來", pc)
	}
	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		savePNG(t, filepath.Join(dir, "remake-newlord-born.png"), cv)
	}
	text := [][4]int{
		{game.BubbleX1 + 8, 66 + 12, game.BubbleX2 - 70, 66 + 84}, // 對白兩行
		{game.BubbleX2 - 55, 66 + 80, game.BubbleX2 - 8, 66 + 95}, // 名字（黑底）
		{423, 331, 624, 364}, // 提示框
	}
	compareFullScreen(t, "新君主出現", orig, cv, 72, text, map[int]bool{3: true, 15: true, 0: true})
}
