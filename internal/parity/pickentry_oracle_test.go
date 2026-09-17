//go:build oracle

package parity

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// 挑人清單 `0x18024` 與數字輸入 `0x34ede` 的對拍（Issue #77，`docs/spec/014` §4.2／§4.3）。

// pickBoard 開到主命令、把玩家的郡擺成 14 位將軍（兩頁），回盤面、郡、字型與素材。
type pickBoard struct {
	o      *oracle.Oracle
	base   uint32
	at     int
	me     state.FactionID
	tr     *cursorTrack
	asks   *[][2]int
	art    *ui.ArtScreen
	people []int
}

func newPickBoard(t *testing.T) *pickBoard {
	root := origRoot(t)
	sc0, err := state.LoadScenario(openContainer(t, filepath.Join(root, "DATA2")), state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(o.Close)
	b := &pickBoard{o: o, tr: trackCursor(o), asks: new([][2]int)}
	o.OnCall(addr(bootNumInputFn), func(o *oracle.Oracle) {
		*b.asks = append(*b.asks, [2]int{int(int16(o.Arg(0))), int(int16(o.Arg(1)))})
	})
	b.base = bootToGame(t, o, seedMas)
	b.at, _ = plantLordCommandBoard(t, o, b.base)
	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	live := o.Bytes(addr(b.base), nMas+nSta+nGen)
	b.me = state.FactionID(live[nMas+b.at*state.PrefectureRecordSize+30])
	// 把槽號 200 起的在野與未登場者搬進來當一般武將，謀略、忠誠、兵士錯開，
	// 排序才看得出是照哪一項排的；湊滿 14 位就有第二頁。
	have := 0
	for i := 0; i < nGen/state.GeneralRecordSize; i++ {
		r := live[nMas+nSta+i*state.GeneralRecordSize:]
		if r[17] <= 3 && int(r[19]) == b.at {
			have++
		}
	}
	lord := int(o.Word(addr(b.base + uint32(int(b.me)*state.MasterRecordSize+2))))
	for i := 200; have < 14 && i < 350; i++ {
		rec := b.base + uint32(nMas+nSta+i*state.GeneralRecordSize)
		// 沒有名字的填充筆、君主本人（自創君主是填充筆，名字也是 0xA1 起）、已經在這一郡的人都不要動。
		if o.Byte(addr(rec)) < 0xa1 || i == lord || (o.Byte(addr(rec+17)) <= 3 && int(o.Byte(addr(rec+19))) == b.at) {
			continue
		}
		o.SetByte(addr(rec+9), uint8(40+(i*7)%55))  // 謀略
		o.SetByte(addr(rec+16), uint8(50+(i*3)%50)) // 忠誠
		o.SetWord(addr(rec+22), uint16(100+(i*37)%900))
		o.SetByte(addr(rec+17), uint8(state.StatusOfficer))
		o.SetByte(addr(rec+18), uint8(b.me))
		o.SetByte(addr(rec+19), uint8(b.at))
		have++
	}
	for i := 0; i < nGen/state.GeneralRecordSize; i++ {
		rec := b.base + uint32(nMas+nSta+i*state.GeneralRecordSize)
		if o.Byte(addr(rec+17)) <= 3 && int(o.Byte(addr(rec+19))) == b.at {
			b.people = append(b.people, i)
		}
	}
	art, err := ui.NewArtScreen(openContainer(t, filepath.Join(root, "DATA3")), openContainer(t, filepath.Join(root, "DATA1")))
	if err != nil {
		t.Fatal(err)
	}
	b.art = art
	i18n.Current = i18n.ZhHant
	return b
}

// game 是此刻原版記憶體裡的盤面。
func (b *pickBoard) game(t *testing.T) *game.State {
	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	live := b.o.Bytes(addr(b.base), nMas+nSta+nGen)
	sc, err := state.DecodeTables(state.Slot("001"), live[:nMas], live[nMas:nMas+nSta], live[nMas+nSta:])
	if err != nil {
		t.Fatal(err)
	}
	g, err := game.New(sc, b.me, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// press 送一段鍵，等原版下一次叫數字輸入，再等游標畫好一格，回那一次的上下限與畫面。
func (b *pickBoard) press(t *testing.T, name, keys string) ([2]int, []uint8, cursorTrack) {
	t.Helper()
	before := len(*b.asks)
	b.o.Drain()
	b.o.TypeBoth(keys)
	waitBoot(t, b.o, name, 300_000_000, func() bool { return len(*b.asks) > before })
	waitCursorShown(t, b.o, b.tr, name)
	dumpScreen(t, b.o, "pick-"+name)
	return (*b.asks)[len(*b.asks)-1], append([]uint8(nil), b.o.IndexedEGASize(scrW, scrH)...), *b.tr
}

// comparePanels 比右側面板與下面板：字格（8×16）比有沒有墨（字模不接原版），
// 其餘像素逐點相同；游標那一格逐像素另比。
// textArea 是一塊 8×16 字格：左上角、欄列數、紙色（−1 表示取下面板的紙色）。
type textArea struct{ x0, y0, cols, rows, bg int }

// rosterText 是挑人清單的字格：表頭、十二列、多選的「*」那一欄、下面板。
var rosterText = []textArea{
	{440, 62, 23, 1, 1},
	{440, 84, 23, 12, 1},
	{424, 84, 1, 12, 1},
	{424, 300, 24, 4, -1},
}

// prefText 是挑郡清單的字格：表頭、三欄 × 14 列、下面板。
var prefText = []textArea{
	{424, 44, 25, 1, 1},
	{432, 60, 24, 14, 1},
	{424, 300, 24, 4, -1},
}

func comparePanels(t *testing.T, name string, orig []uint8, cv, plain *ui.Canvas, tr cursorTrack, style int, text ...textArea) {
	t.Helper()
	org := func(x, y int) int { return int(orig[y*scrW+x] & 15) }
	mine := func(x, y int) int { return paletteIndex(cv.Img.RGBAAt(x, y)) }
	if len(text) == 0 {
		text = rosterText
	}
	inText := func(x, y int) bool {
		for _, a := range text {
			if x >= a.x0 && x < a.x0+a.cols*8 && y >= a.y0 && y < a.y0+a.rows*16 {
				return true
			}
		}
		return false
	}
	bad, first := 0, ""
	for y := 36; y <= 291; y++ {
		for x := 408; x <= 631; x++ {
			if inText(x, y) {
				continue
			}
			if org(x, y) != mine(x, y) {
				bad++
				if first == "" {
					first = fmt.Sprintf("(%d,%d) 原版 %d remake %d", x, y, org(x, y), mine(x, y))
				}
			}
		}
	}
	cellsBad, inked := 0, 0
	for _, a := range text {
		bg := a.bg
		if bg < 0 {
			bg = lowerPaper(org)
		}
		for row := 0; row < a.rows; row++ {
			for col := 0; col < a.cols; col++ {
				x0, y0 := a.x0+col*8, a.y0+row*16
				if tr.Shown && x0 == tr.X && y0 == tr.Y {
					continue // 游標另比
				}
				ink := func(pix func(x, y int) int) bool {
					for y := y0; y < y0+16; y++ {
						for x := x0; x < x0+8; x++ {
							if pix(x, y) != bg {
								return true
							}
						}
					}
					return false
				}
				om, mm := ink(org), ink(mine)
				if om {
					inked++
				}
				if om != mm {
					cellsBad++
					if cellsBad <= 8 {
						t.Logf("%s：(%d,%d) 那一格 原版有墨 %v、remake %v", name, x0, y0, om, mm)
					}
				}
			}
		}
	}
	if bad != 0 || cellsBad != 0 {
		t.Errorf("%s：右側面板字格以外 %d 點不同（第一個 %s），字格有墨不同 %d", name, bad, first, cellsBad)
	} else {
		t.Logf("%s：右側面板字格以外逐像素相同；字格 %d 格有墨，墨相同", name, inked)
	}
	compareCursorCell(t, name, tr, style, org, mine,
		func(x, y int) int { return paletteIndex(plain.Img.RGBAAt(x, y)) })
}

// comparePrefColors 逐郡比挑郡清單那一格第一個有墨點的顏色：收得下的郡是黃 14、其餘棕 6（`0x1d4ec`）。
// comparePanels 只比有沒有墨，字色要另外比。
func comparePrefColors(t *testing.T, name string, shot []uint8, cv *ui.Canvas) {
	t.Helper()
	bad := 0
	for id := 1; id <= 42; id++ {
		x0, y0 := 432+(id-1)/14*64, 60+(id-1)%14*16
		ink := func(pix func(x, y int) int) int {
			for y := y0; y < y0+16; y++ {
				for x := x0; x < x0+48; x++ {
					if v := pix(x, y); v != 1 {
						return v
					}
				}
			}
			return -1
		}
		if o, m := ink(func(x, y int) int { return int(shot[y*scrW+x] & 15) }), ink(func(x, y int) int { return paletteIndex(cv.Img.RGBAAt(x, y)) }); o != m {
			bad++
			if bad <= 4 {
				t.Logf("%s：郡 %d 原版字色 %d、remake %d", name, id, o, m)
			}
		}
	}
	if bad != 0 {
		t.Errorf("%s：%d 郡的字色不同", name, bad)
	} else {
		t.Logf("%s：42 郡字色逐郡相同", name)
	}
}

// TestZZPickListMatchesTheOriginal 走內政→土地開墾（模式 2、鍵 1 謀略）：清單兩頁的右側面板
// 與下面板、名單順序與原版相同。
func TestZZPickListMatchesTheOriginal(t *testing.T) {
	b := newPickBoard(t)
	face := loadFace(t)
	b.press(t, "內政", "4\r")
	ask1, shot1, tr1 := b.press(t, "土地開墾", "1\r")
	// 原版的名單（`0x18272`：段 `DS:[0xa79c]` 的 `0x58c`，筆數段 `DS:[0xa7a0]` 的 `0x0c`）。
	ds := uint32(b.o.DSReg()) * 16
	lst, cnt := b.o.Word(addr(ds+0xa79c)), b.o.Word(addr(ds+0xa7a0))
	n := int(b.o.Word(oracle.Addr{Seg: cnt, Off: 0x0c}))
	var orig []int
	for i := 0; i < n; i++ {
		orig = append(orig, int(b.o.Word(oracle.Addr{Seg: lst, Off: uint16(0x58c + 2*i)})))
	}
	g := b.game(t)
	var mine []int
	for _, x := range g.PickRoster(b.at, game.PickServing, game.PickByIntel) {
		mine = append(mine, x.Index)
	}
	if fmt.Sprint(orig) != fmt.Sprint(mine) {
		t.Fatalf("名單順序：原版 %v，remake %v", orig, mine)
	}
	if len(orig) <= ui.RosterPageRows {
		t.Fatalf("名單只有 %d 位，翻不到第二頁", len(orig))
	}
	t.Logf("名單 %d 位、順序相同：%v；第一頁問 %v", len(orig), orig, ask1)
	ask2, shot2, tr2 := b.press(t, "第二頁", " ")

	for _, c := range []struct {
		name string
		page int
		ask  [2]int
		shot []uint8
		tr   cursorTrack
	}{
		{"第一頁", 0, ask1, shot1, tr1},
		{"第二頁", ui.RosterPageRows, ask2, shot2, tr2},
	} {
		p := &ui.RosterPick{List: mine, Key: game.PickByIntel, Page: c.page}
		lo, hi := ui.RosterRange(p)
		if lo != c.ask[0] || hi != c.ask[1] {
			t.Errorf("%s：數字輸入原版問 %v，remake 算 %d-%d", c.name, c.ask, lo, hi)
		}
		v := ui.View{Sel: b.at, Roster: p, Prompt: i18n.S("ask.reclaim") + i18n.Sf("pick.range", 1, len(mine)), Input: c.tr.input()}
		cv := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(cv, b.art, g, nil, v)
		v.Input = ui.InputCursor{}
		plain := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(plain, b.art, g, nil, v)
		if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
			savePNG(t, filepath.Join(dir, "remake-pick-"+c.name+".png"), cv)
		}
		comparePanels(t, c.name, c.shot, cv, plain, c.tr, 1)
		// 反對照：不畫清單，右側面板一定不同。
		v.Roster = nil
		none := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(none, b.art, g, nil, v)
		diff := 0
		for y := 36; y <= 291; y++ {
			for x := 408; x <= 631; x++ {
				if int(c.shot[y*scrW+x]&15) != paletteIndex(none.Img.RGBAAt(x, y)) {
					diff++
				}
			}
		}
		if diff == 0 {
			t.Errorf("%s：反對照沒有差別", c.name)
		}
	}
}

// TestZZNumberEntryMatchesTheOriginal 走兵士→徵兵→第 1 位：「%s有%d兵士 徵多少兵士 (0-%d):」
// 那一格與打了「12」之後的回顯，下面板與游標相同。
func TestZZNumberEntryMatchesTheOriginal(t *testing.T) {
	b := newPickBoard(t)
	face := loadFace(t)
	b.press(t, "兵士", "3\r")
	b.press(t, "徵兵", "2\r")
	ask, shot, tr := b.press(t, "徵多少兵士", "1\r")
	g := b.game(t)
	list := g.PickRoster(b.at, game.PickServing, game.PickBySoldiers)
	x := list[0]
	title := i18n.Sf("ask.conscriptN", ui.NameField(i18n.PersonName(x.Name)), int(x.Soldiers))
	prompt := title + i18n.Sf("pick.range", ask[0], ask[1])
	t.Logf("原版問 %v；remake 的提示 %q", ask, prompt)
	render := func(p string, in ui.InputCursor) *ui.Canvas {
		cv := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(cv, b.art, g, nil, ui.View{Sel: b.at, Prompt: p, Input: in})
		return cv
	}
	compareLower(t, "徵多少兵士", shot, render(prompt, tr.input()), render(prompt, ui.InputCursor{}), tr)
	// 打兩個數字：原版回顯在「(0-%d):」後面，欄寬是上限的位數，滿了再打的丟掉。
	b.o.Drain()
	b.o.TypeBoth("12")
	waitCursorShown(t, b.o, b.tr, "回顯")
	if err := b.o.Run(2_000_000); err != nil {
		t.Fatal(err)
	}
	waitCursorShown(t, b.o, b.tr, "回顯")
	echo, trEcho := append([]uint8(nil), b.o.IndexedEGASize(scrW, scrH)...), *b.tr
	dumpScreen(t, b.o, "pick-回顯")
	typed := "12"[:min(2, len(fmt.Sprint(ask[1])))]
	if trEcho.X != tr.X+8*len(typed) || trEcho.Y != tr.Y {
		t.Errorf("打了「12」之後游標在 (%d,%d)，欄寬 %d 位該在 (%d,%d)", trEcho.X, trEcho.Y, len(typed), tr.X+8*len(typed), tr.Y)
	}
	compareLower(t, "回顯 "+typed, echo, render(prompt+typed, trEcho.input()), render(prompt+typed, ui.InputCursor{}), trEcho)
}

// compareLower 比下面板 24×4 個字格有沒有墨，游標那一格逐像素。
func compareLower(t *testing.T, name string, orig []uint8, cv, plain *ui.Canvas, tr cursorTrack) {
	t.Helper()
	org := func(x, y int) int { return int(orig[y*scrW+x] & 15) }
	mine := func(x, y int) int { return paletteIndex(cv.Img.RGBAAt(x, y)) }
	bg := lowerPaper(org)
	bad, inked := 0, 0
	for row := 0; row < 4; row++ {
		for col := 0; col < 24; col++ {
			x0, y0 := 424+col*8, 300+row*16
			if tr.Shown && x0 == tr.X && y0 == tr.Y {
				continue
			}
			ink := func(pix func(x, y int) int) bool {
				for y := y0; y < y0+16; y++ {
					for x := x0; x < x0+8; x++ {
						if pix(x, y) != bg {
							return true
						}
					}
				}
				return false
			}
			om, mm := ink(org), ink(mine)
			if om {
				inked++
			}
			if om != mm {
				bad++
				if bad <= 8 {
					t.Logf("%s：(%d,%d) 原版有墨 %v、remake %v", name, x0, y0, om, mm)
				}
			}
		}
	}
	if bad != 0 {
		t.Errorf("%s：下面板字格有墨不同 %d", name, bad)
	} else {
		t.Logf("%s：下面板 %d 格有墨，墨相同", name, inked)
	}
	compareCursorCell(t, name, tr, 1, org, mine, func(x, y int) int { return paletteIndex(plain.Img.RGBAAt(x, y)) })
}

// lowerPaper 是下面板字區（424–615 × 300–363）最常見的顏色，當成紙色。
func lowerPaper(pix func(x, y int) int) int {
	var n [16]int
	for y := 300; y < 364; y++ {
		for x := 424; x < 616; x++ {
			n[pix(x, y)&15]++
		}
	}
	best := 0
	for i := range n {
		if n[i] > n[best] {
			best = i
		}
	}
	return best
}

// TestZZCardPromptMatchesTheOriginal 走查看→檢視將軍→第 1 位與君主→賞賜物品→那一郡→第 1 位：
// 卡片之後原版在下面板寫提示再讀鍵（`0x17cd8`「請按任一鍵」、`0x1d190`「請按任一鍵\n查看物品表」），
// 游標接在後面。下面板字格有墨與游標逐像素相同（Issue #81）。
func TestZZCardPromptMatchesTheOriginal(t *testing.T) {
	b := newPickBoard(t)
	face := loadFace(t)
	cards := 0
	b.o.OnCall(cardFn, func(*oracle.Oracle) { cards++ })
	card := func(name, keys string) ([]uint8, cursorTrack) {
		t.Helper()
		before := cards
		b.o.Drain()
		b.o.TypeBoth(keys)
		waitBoot(t, b.o, name, 300_000_000, func() bool { return cards > before })
		waitCursorShown(t, b.o, b.tr, name)
		dumpScreen(t, b.o, "cardprompt-"+name)
		return append([]uint8(nil), b.o.IndexedEGASize(scrW, scrH)...), *b.tr
	}
	g := b.game(t)
	render := func(prompt string, in ui.InputCursor) *ui.Canvas {
		cv := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(cv, b.art, g, nil, ui.View{Sel: b.at, Prompt: prompt, Input: in})
		return cv
	}
	b.press(t, "君主", "7\r")
	b.press(t, "賞賜物品", "4\r")
	b.press(t, "賞賜那一郡", fmt.Sprintf("%d\r", b.at))
	shot, tr := card("賞賜那一位 1", "1\r")
	compareLower(t, "賞賜物品的卡片", shot, render(i18n.S("ask.giftItems"), tr.input()), render(i18n.S("ask.giftItems"), ui.InputCursor{}), tr)

	// 收掉賞賜物品：任意鍵看物品表 → 空 Enter 回那一位 → 空 Enter 回那一郡 → 空 Enter 回主命令。
	b.press(t, "看物品表", " ")
	b.press(t, "不選物品", "\r")
	b.press(t, "不選那一位", "\r")
	b.press(t, "不選那一郡", "\r")
	b.press(t, "查看", "1\r")
	b.press(t, "檢視將軍", "3\r")
	shot, tr = card("檢視那位 1", "1\r")
	compareLower(t, "查看的卡片", shot, render(i18n.S("msg.anyKey"), tr.input()), render(i18n.S("msg.anyKey"), ui.InputCursor{}), tr)
}

// TestZZMovePickMatchesTheOriginal 走軍事→調動軍隊（Issue #80）：從那一郡移出 → 調到那一郡
// （把鄰郡擺成無主）→ 多選清單選第 1、2 位 → 金 → 米。多選清單兩格（沒選／選了一位）
// 與金那一問的畫面、上限相同；改選第 2、3 位、送金 1、米 10，搬完之後三張表逐位元組相同。
func TestZZMovePickMatchesTheOriginal(t *testing.T) {
	b := newPickBoard(t)
	face := loadFace(t)
	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	rec := b.base + uint32(nMas+b.at*state.PrefectureRecordSize)
	to := 0
	for k := 45; k <= 54 && to == 0; k++ {
		if n := int(b.o.Byte(addr(rec + uint32(k)))); n >= 1 && n <= 42 {
			to = n
		}
	}
	if to == 0 {
		t.Fatal("南海沒有鄰郡")
	}
	b.o.SetByte(addr(b.base+uint32(nMas+to*state.PrefectureRecordSize+30)), 0xFF)
	done := 0
	b.o.OnCall(addr(0x18fc2), func(*oracle.Oracle) { done++ }) // `0x1938a` 搬完回來
	before := b.o.Bytes(addr(b.base), nMas+nSta+nGen)
	g := b.game(t)

	b.press(t, "軍事", "2\r")
	b.press(t, "調動軍隊", "1\r")
	b.press(t, "從那一郡移出", fmt.Sprintf("%d\r", b.at))
	ask0, shot0, tr0 := b.press(t, "調到那一郡", fmt.Sprintf("%d\r", to))
	list := g.PickRoster(b.at, game.PickServing, game.PickByStatus)
	ds := uint32(b.o.DSReg()) * 16
	lst := b.o.Word(addr(ds + 0xa79c))
	var mine []int
	for i, x := range list {
		mine = append(mine, x.Index)
		if o := int(b.o.Word(oracle.Addr{Seg: lst, Off: uint16(0x58c + 2*i)})); o != x.Index {
			t.Fatalf("多選名單第 %d 位：原版 %d，remake %d", i+1, o, x.Index)
		}
	}
	people, goldMax, riceMax := g.MoveLimits(b.at, to)
	ask1, shot1, tr1 := b.press(t, "選第 1 位", "1\r")
	// 再按一次 1 取消選取，改選第 2、3 位：搬走主事者（君主）會讓原版在重整守將清單時
	// 問「選擇新任太守」（`0x1d6ed`），那是另一條路（CONTEXT R82）。
	b.press(t, "取消第 1 位", "1\r")
	b.press(t, "選第 2 位", "2\r")
	b.press(t, "選第 3 位", "3\r")
	askGold, shotGold, trGold := b.press(t, "選好了", "\r")
	askRice, _, _ := b.press(t, "金", "1\r")
	t.Logf("清單 %d 位 %v（上限 %d 位）；原版問 %v %v 金 %v 米 %v；remake 金上限 %d 米上限 %d",
		len(mine), ask0, people, ask0, ask1, askGold, askRice, goldMax, riceMax)
	if askGold[1] != goldMax || askRice[1] != riceMax {
		t.Errorf("金米上限：原版 %d／%d，remake %d／%d", askGold[1], askRice[1], goldMax, riceMax)
	}
	b.o.Drain()
	b.o.TypeBoth("10\r")
	if err := b.o.RunUntil(oracle.NewCond("搬運", func(*oracle.Oracle) bool { return done > 0 }), oracle.Budget(600_000_000)); err != nil {
		dumpScreen(t, b.o, "move-stuck")
		t.Fatalf("等搬運：%v", err)
	}
	after := b.o.Bytes(addr(b.base), nMas+nSta+nGen)

	for _, c := range []struct {
		name   string
		marked []bool
		ask    [2]int
		shot   []uint8
		tr     cursorTrack
	}{
		{"多選清單", make([]bool, len(mine)), ask0, shot0, tr0},
		{"選了第 1 位", append([]bool{true}, make([]bool, len(mine)-1)...), ask1, shot1, tr1},
	} {
		p := &ui.RosterPick{List: mine, Key: game.PickBySoldiers, Multi: true, Marked: c.marked}
		if lo, hi := ui.RosterRange(p); lo != c.ask[0] || hi != c.ask[1] {
			t.Errorf("%s：原版問 %v，remake %d-%d", c.name, c.ask, lo, hi)
		}
		v := ui.View{Sel: b.at, Roster: p, Prompt: i18n.S("ask.moveWho") + i18n.Sf("pick.range", 1, len(mine)), Input: c.tr.input()}
		cv := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(cv, b.art, g, nil, v)
		v.Input = ui.InputCursor{}
		plain := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(plain, b.art, g, nil, v)
		comparePanels(t, c.name, c.shot, cv, plain, c.tr, 1)
	}
	goldPrompt := i18n.S("ask.moveWho") + i18n.Sf("pick.range", 1, len(mine)) + i18n.Sf("ask.moveCount", 2) +
		i18n.S("ask.sendGold") + i18n.Sf("pick.range", 0, askGold[1])
	render := func(p string, in ui.InputCursor) *ui.Canvas {
		cv := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(cv, b.art, g, nil, ui.View{Sel: b.at, Prompt: p, Input: in})
		return cv
	}
	compareLower(t, "金那一問", shotGold, render(goldPrompt, trGold.input()), render(goldPrompt, ui.InputCursor{}), trGold)

	// 搬完的盤面：remake 從同一個盤面把第 2、3 位、金 1、米 10 搬過去。
	sc, err := state.DecodeTables(state.Slot("001"), before[:nMas], before[nMas:nMas+nSta], before[nMas+nSta:])
	if err != nil {
		t.Fatal(err)
	}
	g2, err := game.New(sc, b.me, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	if err := g2.Move(b.at, to, mine[1:3], 1, 10, b.me); err != nil {
		t.Fatalf("remake 的調動軍隊：%v", err)
	}
	rm, rs, rg, err := g2.Tables()
	if err != nil {
		t.Fatal(err)
	}
	got := append(append(append([]byte{}, rm...), rs...), rg...)
	t.Logf("原版動到的記錄：%s", changedRecords(before, after, nMas, nSta))
	if n := diffCount(after, got); n != 0 {
		t.Fatalf("三張表兩邊差 %d 個位元組：%s", n, changedRecords(after, got, nMas, nSta))
	}
}

// TestZZPrefPickMatchesTheOriginal 查看→查看那一郡、軍事→發動戰役→攻打那一郡：下面板提示接
// 「(1-42):」與游標相同（Issue #80）。
func TestZZPrefPickMatchesTheOriginal(t *testing.T) {
	b := newPickBoard(t)
	face := loadFace(t)
	g := b.game(t)
	check := func(name, prompt string, valid func(int) bool, shot []uint8, tr cursorTrack) {
		pp := &ui.PrefPick{}
		for id := 1; id <= 42; id++ {
			pp.Valid[id] = valid(id)
		}
		p := prompt + i18n.Sf("pick.range", 1, 42)
		cv := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(cv, b.art, g, nil, ui.View{Sel: b.at, Prompt: p, PrefPick: pp, Input: tr.input()})
		plain := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(plain, b.art, g, nil, ui.View{Sel: b.at, Prompt: p, PrefPick: pp})
		if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
			savePNG(t, filepath.Join(dir, "remake-pref-"+name+".png"), cv)
		}
		comparePanels(t, name, shot, cv, plain, tr, 1, prefText...)
		comparePrefColors(t, name, shot, cv)
	}
	b.press(t, "查看", "1\r")
	ask, shot, tr := b.press(t, "查看那一郡", "1\r")
	if ask != [2]int{1, 42} {
		t.Errorf("查看那一郡原版問 %v", ask)
	}
	check("查看那一郡", i18n.S("ask.pref"), func(int) bool { return true }, shot, tr)
	b.press(t, "不看", "\r")
	b.press(t, "收掉查看", "\r")
	b.press(t, "軍事", "2\r")
	b.press(t, "發動戰役", "2\r")
	_, shot, tr = b.press(t, "從那一郡攻打", fmt.Sprintf("%d\r", b.at))
	check("攻打那一郡", i18n.S("ask.attack"), func(to int) bool {
		q, p := g.Prefecture(to), g.Prefecture(b.at)
		return q != nil && p != nil && g.Adjacent(b.at, to) && q.Owned() && q.Owner != p.Owner
	}, shot, tr)
}

// TestZZPlotPickMatchesTheOriginal 計略的問法（Issue #83）：偽書使疑「派細作到那一郡」→
// 「派那一位去遊說」，遠交近攻「出使那一郡」→「聯合攻打那一郡」→「聯合我方那一郡」（盤面走得到的
// 那幾問）。每一問的挑郡清單字色逐郡、面板字格以外逐像素、下面板與游標相同。
func TestZZPlotPickMatchesTheOriginal(t *testing.T) {
	b := newPickBoard(t)
	face := loadFace(t)
	// 計略選單要有軍師（`0x2c241`：諸侯記錄 offset 6），而且軍師或君主在這一郡。把一位部將擺成
	// 軍師、謀略壓到 50——勸諫要 RND(5)＋80 小於軍師謀略才出來，50 永遠不會。
	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	lord := int(b.o.Word(addr(b.base + uint32(int(b.me)*state.MasterRecordSize+2))))
	chief := -1
	for _, i := range b.people {
		if i != lord {
			chief = i
			break
		}
	}
	rec := b.base + uint32(nMas+nSta+chief*state.GeneralRecordSize)
	b.o.SetByte(addr(rec+17), uint8(state.StatusChief))
	b.o.SetByte(addr(rec+9), 50)
	b.o.SetWord(addr(b.base+uint32(int(b.me)*state.MasterRecordSize+6)), uint16(chief))
	g := b.game(t)
	home := g.Prefecture(b.at)
	mine := func(id int) bool { q := g.Prefecture(id); return q != nil && q.Owned() && q.Owner == home.Owner }
	enemy := func(id int) bool { q := g.Prefecture(id); return q != nil && q.Owned() && q.Owner != home.Owner }
	nb := func(id int, ok func(int) bool) bool {
		for _, n := range g.Prefecture(id).Neighbours {
			if ok(n) {
				return true
			}
		}
		return false
	}
	pref := func(name, prompt string, valid func(int) bool, shot []uint8, tr cursorTrack) {
		t.Helper()
		pp := &ui.PrefPick{}
		for id := 1; id <= 42; id++ {
			pp.Valid[id] = valid(id)
		}
		p := prompt + i18n.Sf("pick.range", 1, 42)
		cv := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(cv, b.art, g, nil, ui.View{Sel: b.at, Prompt: p, PrefPick: pp, Input: tr.input()})
		plain := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(plain, b.art, g, nil, ui.View{Sel: b.at, Prompt: p, PrefPick: pp})
		comparePanels(t, name, shot, cv, plain, tr, 1, prefText...)
		comparePrefColors(t, name, shot, cv)
	}
	pick := func(valid func(int) bool) int {
		for id := 1; id <= 42; id++ {
			if valid(id) {
				return id
			}
		}
		return 0
	}

	b.press(t, "計略", "8\r")
	_, shot, tr := b.press(t, "偽書使疑", "3\r")
	pref("派細作到那一郡", i18n.S("plot.forge.at"), enemy, shot, tr)
	at := pick(enemy)
	if at == 0 {
		t.Fatal("沒有別人的郡")
	}
	ask, shot, tr := b.press(t, "派細作", fmt.Sprintf("%d\r", at))
	list := g.PickRoster(b.at, game.PickServing, game.PickByCharm)
	var idx []int
	for _, x := range list {
		idx = append(idx, x.Index)
	}
	p := &ui.RosterPick{List: idx, Key: game.PickByCharm}
	if lo, hi := ui.RosterRange(p); ask != [2]int{lo, hi} {
		t.Errorf("使者清單：原版問 %v，remake %d-%d", ask, lo, hi)
	}
	v := ui.View{Sel: b.at, Roster: p, Prompt: i18n.S("plot.forge.envoy") + i18n.Sf("pick.range", 1, len(idx)), Input: tr.input()}
	cv := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(cv, b.art, g, nil, v)
	v.Input = ui.InputCursor{}
	plain := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(plain, b.art, g, nil, v)
	comparePanels(t, "派那一位去遊說", shot, cv, plain, tr, 1)

	// 空 Enter 取消（原版印「取消」、延遲、回主命令），再走遠交近攻。
	b.press(t, "取消使者", "\r")
	b.press(t, "計略", "8\r")
	farAt := func(id int) bool { return enemy(id) && nb(id, func(n int) bool { return nb(n, mine) }) }
	_, shot, tr = b.press(t, "遠交近攻", "2\r")
	pref("出使那一郡", i18n.S("plot.far.at"), farAt, shot, tr)
	at = pick(farAt)
	if at == 0 {
		t.Log("這個盤面沒有遠交近攻出使得到的郡，後兩問不比")
		return
	}
	strike := func(id int) bool {
		return enemy(id) && g.Adjacent(at, id) && g.Prefecture(id).Owner != g.Prefecture(at).Owner && nb(id, mine)
	}
	_, shot, tr = b.press(t, "出使", fmt.Sprintf("%d\r", at))
	pref("聯合攻打那一郡", i18n.S("plot.far.strike"), strike, shot, tr)
	st := pick(strike)
	if st == 0 {
		t.Log("出使郡沒有可以聯合攻打的鄰郡，第三問不比")
		return
	}
	ours := func(id int) bool { return mine(id) && g.Adjacent(st, id) }
	_, shot, tr = b.press(t, "聯合攻打", fmt.Sprintf("%d\r", st))
	pref("聯合我方那一郡", i18n.S("plot.far.ours"), ours, shot, tr)
}

// TestZZHeadhuntPickMatchesTheOriginal 君主→5.登用他國人才（Issue #85）：「登用那一郡的將軍」
// 收任何別人的郡（不限相鄰）、「<登用他國將軍>登用那一位將軍」是那一郡的挑人清單（模式 5、鍵 0）。
// 兩問的面板、字色逐郡（`comparePrefColors`）、下面板與游標相同。
func TestZZHeadhuntPickMatchesTheOriginal(t *testing.T) {
	b := newPickBoard(t)
	face := loadFace(t)
	nMas := state.MasterTableSize
	b.o.SetWord(addr(b.base+uint32(nMas+b.at*state.PrefectureRecordSize+18)), 9000) // 金要有 100
	g := b.game(t)
	home := g.Prefecture(b.at)
	enemy := func(id int) bool { q := g.Prefecture(id); return q != nil && q.Owned() && q.Owner != home.Owner }
	b.press(t, "君主", "7\r")
	_, shot, tr := b.press(t, "登用他國人才", "5\r")
	pp := &ui.PrefPick{}
	for id := 1; id <= 42; id++ {
		pp.Valid[id] = enemy(id)
	}
	p := i18n.S("ask.headhuntPref") + i18n.Sf("pick.range", 1, 42)
	cv := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(cv, b.art, g, nil, ui.View{Sel: b.at, Prompt: p, PrefPick: pp, Input: tr.input()})
	plain := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(plain, b.art, g, nil, ui.View{Sel: b.at, Prompt: p, PrefPick: pp})
	comparePanels(t, "登用那一郡的將軍", shot, cv, plain, tr, 1, prefText...)
	comparePrefColors(t, "登用那一郡的將軍", shot, cv)
	// 挑一個不相鄰、而且有模式 5 對象的別人的郡——remake 先前只列鄰郡，這一格擋得住退回。
	target := 0
	for id := 1; id <= 42 && target == 0; id++ {
		if enemy(id) && !g.Adjacent(b.at, id) && len(g.PickRoster(id, game.PickSubject, game.PickByStatus)) > 0 {
			target = id
		}
	}
	if target == 0 {
		t.Fatal("找不到不相鄰、有部將的別人的郡")
	}
	ask, shot, tr := b.press(t, "登用那一郡", fmt.Sprintf("%d\r", target))
	var idx []int
	for _, x := range g.PickRoster(target, game.PickSubject, game.PickByStatus) {
		idx = append(idx, x.Index)
	}
	rp := &ui.RosterPick{List: idx, Key: game.PickByStatus}
	if lo, hi := ui.RosterRange(rp); ask != [2]int{lo, hi} {
		t.Errorf("郡 %d 的名單：原版問 %v，remake %d-%d", target, ask, lo, hi)
	}
	v := ui.View{Sel: b.at, Roster: rp, Prompt: i18n.S("ask.headhunt") + i18n.Sf("pick.range", 1, len(idx)), Input: tr.input()}
	cv = ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(cv, b.art, g, nil, v)
	v.Input = ui.InputCursor{}
	plain = ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(plain, b.art, g, nil, v)
	comparePanels(t, "登用那一位將軍", shot, cv, plain, tr, 1)
}

// TestZZAutonomyAskMatchesTheOriginal 君主→3.郡縣自冶（Issue #87）：「授權自冶那一郡」收同一主人、主事者不是
// 君主的郡（字色逐郡）；挑到之後下面板「授權某郡／1.正常 2.內政／3.軍事 4.自冶／那一種:」、`0x115e(1, 4)`
// 讀一位數，下面板與游標相同；打「3」之後那一郡 offset 12 是 2（選項減一），接著回到挑郡再問。
func TestZZAutonomyAskMatchesTheOriginal(t *testing.T) {
	b := newPickBoard(t)
	face := loadFace(t)
	// 玩家只有一個郡，而且君主主事——清單會是空的。把一個鄰郡擺成自己的，派一位部將去主事。
	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	lord := int(b.o.Word(addr(b.base + uint32(int(b.me)*state.MasterRecordSize+2))))
	who := b.people[len(b.people)-1]
	if who == lord {
		who = b.people[0]
	}
	nb := b.game(t).Prefecture(b.at).Neighbours[0]
	b.o.SetByte(addr(b.base+uint32(nMas+nSta+who*state.GeneralRecordSize+19)), uint8(nb))
	b.o.SetByte(addr(b.base+uint32(nMas+nb*state.PrefectureRecordSize+30)), uint8(b.me))
	b.o.SetWord(addr(b.base+uint32(nMas+nb*state.PrefectureRecordSize+32)), uint16(who))
	g := b.game(t)
	b.press(t, "君主", "7\r")
	ask, shot, tr := b.press(t, "授權自冶那一郡", "3\r")
	if ask != [2]int{1, 42} {
		t.Fatalf("郡縣自冶先問的不是挑郡：%v", ask)
	}
	pp := &ui.PrefPick{}
	target := 0
	for id := 1; id <= 42; id++ {
		pp.Valid[id] = g.AutonomyTarget(b.at, id)
		if pp.Valid[id] && target == 0 {
			target = id
		}
	}
	if pp.Valid[b.at] {
		t.Error("君主主事的郡不該收")
	}
	if target == 0 {
		t.Fatal("盤面上沒有主事者不是君主的自己的郡")
	}
	p := i18n.S("ask.autonomyPref") + i18n.Sf("pick.range", 1, 42)
	cv := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(cv, b.art, g, nil, ui.View{Sel: b.at, Prompt: p, PrefPick: pp, Input: tr.input()})
	plain := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(plain, b.art, g, nil, ui.View{Sel: b.at, Prompt: p, PrefPick: pp})
	comparePanels(t, "授權自冶那一郡", shot, cv, plain, tr, 1, prefText...)
	comparePrefColors(t, "授權自冶那一郡", shot, cv)

	// 君主主事的郡不收：打進去原版重問。
	if again, _, _ := b.press(t, "君主的郡重問", fmt.Sprintf("%d\r", b.at)); again != [2]int{1, 42} {
		t.Errorf("打君主的郡 %d 之後原版問 %v", b.at, again)
	}
	ask, shot, tr = b.press(t, "那一種", fmt.Sprintf("%d\r", target))
	if ask != [2]int{1, 4} {
		t.Errorf("「那一種」原版問 %v，remake 1-4", ask)
	}
	q := g.Prefecture(target)
	prompt := i18n.Sf("ask.autonomy", q.Name, i18n.S("autoMode.normal"), i18n.S("autoMode.civil"),
		i18n.S("autoMode.military"), i18n.S("autoMode.self"))
	render := func(in ui.InputCursor) *ui.Canvas {
		cv := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(cv, b.art, g, nil, ui.View{Sel: target, Prompt: prompt, Input: in})
		return cv
	}
	compareLower(t, "那一種", shot, render(tr.input()), render(ui.InputCursor{}), tr)

	at := addr(b.base + uint32(state.MasterTableSize+target*state.PrefectureRecordSize+12))
	before := len(*b.asks)
	b.o.Drain()
	b.o.TypeBoth("3\r")
	waitBoot(t, b.o, "寫回自冶型態", 50_000_000, func() bool { return b.o.Word(at) == 2 })
	for i := 0; i < 20 && len(*b.asks) == before; i++ {
		b.o.Drain()
		b.o.TypeBoth(" ")
		if err := b.o.Run(20_000_000); err != nil {
			t.Fatal(err)
		}
	}
	if len(*b.asks) == before || (*b.asks)[len(*b.asks)-1] != [2]int{1, 42} {
		t.Errorf("設定完原版沒有回到挑郡：%v", (*b.asks)[before:])
	}
	o := game.AutonomyOrder{At: b.at, Pref: target, Mode: game.Autonomy(3 - 1)}
	if err := o.Apply(g, b.me); err != nil {
		t.Fatal(err)
	}
	if !o.KeepsTurn() {
		t.Error("郡縣自冶不耗回合")
	}
	if got, want := int(g.Prefecture(target).Autonomy), int(b.o.Word(at)); got != want {
		t.Errorf("型態：原版 %d，remake %d", want, got)
	}
}

// TestZZGiftItemAskMatchesTheOriginal 君主→4.賞賜物品挑完人之後（Issue #88）：原版等一鍵之後右側面板畫
// 君主物品表（`0x14de6`），下面板「賞賜某人／那一樣(2-5):」、`0x115e(2, 5)`。右側面板字格以外逐像素、
// 字格有墨、標題與列的字色、下面板與游標相同。查看→6.物品（`0x17b64`）共用同一張表，下面板「請按任一鍵」，
// 那一格同樣比過。
func TestZZGiftItemAskMatchesTheOriginal(t *testing.T) {
	b := newPickBoard(t)
	face := loadFace(t)
	// 玉璽一件（第 0 列才有字）、兵書一件、寶刀兩件；件數錯開，「%2d」那一欄才看得出來。
	master := b.base + uint32(int(b.me)*state.MasterRecordSize)
	b.o.SetByte(addr(master+14), 1)
	b.o.SetByte(addr(master+15), 1)
	b.o.SetByte(addr(master+16), 2)
	g := b.game(t)
	cards := 0
	b.o.OnCall(cardFn, func(*oracle.Oracle) { cards++ })
	b.press(t, "君主", "7\r")
	b.press(t, "賞賜那一郡的將軍", "4\r")
	b.press(t, "賞賜那一位", fmt.Sprintf("%d\r", b.at))
	b.o.Drain()
	b.o.TypeBoth("1\r")
	waitBoot(t, b.o, "人物卡", 200_000_000, func() bool { return cards > 0 })
	waitBootScan(t, b.o, "請按任一鍵查看物品表", 100_000_000)
	ask, shot, tr := b.press(t, "那一樣", " ")
	if ask != [2]int{2, 5} {
		t.Errorf("「那一樣」原版問 %v，remake 2-5", ask)
	}
	x := g.PickRoster(b.at, game.PickServing, game.PickByLoyalty)[0]
	v := ui.View{Sel: b.at, Treasury: &ui.TreasuryPanel{Faction: b.me},
		Prompt: i18n.Sf("ask.gift", ui.NameField(i18n.PersonName(x.Name))), Input: tr.input()}
	cv := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(cv, b.art, g, nil, v)
	v.Input = ui.InputCursor{}
	plain := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(plain, b.art, g, nil, v)
	treasuryText := []textArea{{432, 52, 24, 1, 5}, {416, 68, 26, 5, 5}, {424, 300, 24, 4, -1}}
	compareTreasury := func(name string, shot []uint8, cv, plain *ui.Canvas, tr cursorTrack) {
		t.Helper()
		comparePanels(t, name, shot, cv, plain, tr, 1, treasuryText...)
		// 標題洋紅 13、五列黃 14：比每一行第一個有墨點的顏色。
		for _, line := range []struct{ x, y, cols int }{{432, 52, 24}, {416, 68, 26}, {416, 84, 26}, {416, 100, 26}, {416, 116, 26}, {416, 132, 26}} {
			ink := func(pix func(x, y int) int) int {
				for y := line.y; y < line.y+16; y++ {
					for xx := line.x; xx < line.x+line.cols*8; xx++ {
						if v := pix(xx, y); v != 5 {
							return v
						}
					}
				}
				return -1
			}
			o := ink(func(x, y int) int { return int(shot[y*scrW+x] & 15) })
			m := ink(func(x, y int) int { return paletteIndex(cv.Img.RGBAAt(x, y)) })
			if o != m {
				t.Errorf("%s：物品表 (%d,%d) 那一行原版字色 %d、remake %d", name, line.x, line.y, o, m)
			}
		}
	}
	compareTreasury("那一樣", shot, cv, plain, tr)

	// 收掉賞賜物品，改走查看→6.物品。
	b.press(t, "不選物品", "\r")
	b.press(t, "不選那一位", "\r")
	b.press(t, "不選那一郡", "\r")
	b.press(t, "查看", "1\r")
	tables := 0
	b.o.OnCall(addr(itemTableFn), func(*oracle.Oracle) { tables++ })
	b.o.Drain()
	b.o.TypeBoth("6\r")
	waitBoot(t, b.o, "查看物品", 300_000_000, func() bool { return tables > 0 })
	waitCursorShown(t, b.o, b.tr, "查看物品")
	shot, tr = append([]uint8(nil), b.o.IndexedEGASize(scrW, scrH)...), *b.tr
	v = ui.View{Sel: b.at, Treasury: &ui.TreasuryPanel{Faction: b.me}, Prompt: i18n.S("msg.anyKey"), Input: tr.input()}
	cv = ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(cv, b.art, g, nil, v)
	v.Input = ui.InputCursor{}
	plain = ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(plain, b.art, g, nil, v)
	compareTreasury("查看物品", shot, cv, plain, tr)
}

// TestZZAppointGovernorMatchesTheOriginal 君主→2.指定太守（Issue #89）：「指定那一郡的太守」收自己的其他郡
// （字色逐郡，下令那一郡打進去重問）；挑到之後列那一郡的人（模式 2、鍵 3），面板、下面板、游標相同；
// 選一般武將之後舊主事者 2→3、新人 3→2、州郡 offset 32 換人，三張表逐位元組相同；原版回主選單、不結束回合。
func TestZZAppointGovernorMatchesTheOriginal(t *testing.T) {
	b := newPickBoard(t)
	face := loadFace(t)
	// 玩家只有一個郡：把一個鄰郡擺成自己的，派兩位部將過去，一位當主事者（身分 2）、一位一般武將。
	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	lord := int(b.o.Word(addr(b.base + uint32(int(b.me)*state.MasterRecordSize+2))))
	var movers []int
	for i := len(b.people) - 1; i >= 0 && len(movers) < 2; i-- {
		if b.people[i] != lord {
			movers = append(movers, b.people[i])
		}
	}
	oldGov, newGov := movers[0], movers[1]
	nb := b.game(t).Prefecture(b.at).Neighbours[0]
	gen := func(i int) uint32 { return b.base + uint32(nMas+nSta+i*state.GeneralRecordSize) }
	for _, i := range movers {
		b.o.SetByte(addr(gen(i)+19), uint8(nb))
		b.o.SetByte(addr(gen(i)+17), uint8(state.StatusOfficer))
	}
	b.o.SetByte(addr(gen(oldGov)+17), uint8(state.StatusGovernor))
	pref := b.base + uint32(nMas+nb*state.PrefectureRecordSize)
	b.o.SetByte(addr(pref+30), uint8(b.me))
	b.o.SetWord(addr(pref+32), uint16(oldGov))
	g := b.game(t)
	again, done := 0, 0
	b.o.OnCall(addr(mainAskAgainAt), func(*oracle.Oracle) { again++ })
	b.o.OnCall(addr(mainTurnDoneAt), func(*oracle.Oracle) { done++ })

	b.press(t, "君主", "7\r")
	ask, shot, tr := b.press(t, "指定那一郡的太守", "2\r")
	if ask != [2]int{1, 42} {
		t.Fatalf("指定太守先問的不是挑郡：%v", ask)
	}
	pp := &ui.PrefPick{}
	for id := 1; id <= 42; id++ {
		pp.Valid[id] = g.GovernorTarget(b.at, id)
	}
	if !pp.Valid[nb] || pp.Valid[b.at] {
		t.Fatalf("清單：郡 %d %v、下令的郡 %d %v", nb, pp.Valid[nb], b.at, pp.Valid[b.at])
	}
	p := i18n.S("ask.governorPref") + i18n.Sf("pick.range", 1, 42)
	cv := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(cv, b.art, g, nil, ui.View{Sel: b.at, Prompt: p, PrefPick: pp, Input: tr.input()})
	plain := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(plain, b.art, g, nil, ui.View{Sel: b.at, Prompt: p, PrefPick: pp})
	comparePanels(t, "指定那一郡的太守", shot, cv, plain, tr, 1, prefText...)
	comparePrefColors(t, "指定那一郡的太守", shot, cv)
	if re, _, _ := b.press(t, "下令的郡重問", fmt.Sprintf("%d\r", b.at)); re != [2]int{1, 42} {
		t.Errorf("打下令的郡 %d 之後原版問 %v", b.at, re)
	}

	ask, shot, tr = b.press(t, "指定那一位", fmt.Sprintf("%d\r", nb))
	var idx []int
	row := 0
	for i, x := range g.PickRoster(nb, game.PickServing, game.PickByCharm) {
		idx = append(idx, x.Index)
		if x.Index == newGov {
			row = i + 1
		}
	}
	rp := &ui.RosterPick{List: idx, Key: game.PickByCharm}
	if lo, hi := ui.RosterRange(rp); ask != [2]int{lo, hi} || row == 0 {
		t.Fatalf("郡 %d 的名單：原版問 %v，remake %d-%d，新人在第 %d 列", nb, ask, lo, hi, row)
	}
	v := ui.View{Sel: nb, Roster: rp, Prompt: i18n.S("ask.governor") + i18n.Sf("pick.range", 1, len(idx)), Input: tr.input()}
	cv = ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(cv, b.art, g, nil, v)
	v.Input = ui.InputCursor{}
	plain = ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(plain, b.art, g, nil, v)
	comparePanels(t, "指定那一位", shot, cv, plain, tr, 1)

	total := nMas + nSta + state.GeneralTableSize
	before := b.o.Bytes(addr(b.base), total)
	b.o.Drain()
	b.o.TypeBoth(fmt.Sprintf("%d\r", row))
	waitBoot(t, b.o, "換主事者", 100_000_000, func() bool { return int(b.o.Word(addr(pref+32))) == newGov })
	for i := 0; i < 40 && again == 0 && done == 0; i++ {
		b.o.Drain()
		b.o.TypeBoth(" ")
		if err := b.o.Run(20_000_000); err != nil {
			t.Fatal(err)
		}
	}
	if again == 0 || done != 0 {
		t.Errorf("指定完原版回主選單 %d 次、結束回合 %d 次；應該回主選單、不結束回合", again, done)
	}
	after := b.o.Bytes(addr(b.base), total)
	o := game.AppointGovernorOrder{At: b.at, Pref: nb, Target: newGov}
	if !o.KeepsTurn() {
		t.Error("remake 的指定太守會結束回合")
	}
	if err := o.Apply(g, b.me); err != nil {
		t.Fatal(err)
	}
	rm, rs, rg, err := g.Tables()
	if err != nil {
		t.Fatal(err)
	}
	got := append(append(append([]byte{}, rm...), rs...), rg...)
	t.Logf("原版動到的記錄：%s", changedRecords(before, after, nMas, nSta))
	if n := diffCount(after, got); n != 0 {
		t.Errorf("三張表兩邊差 %d 個位元組：%s", n, changedRecords(after, got, nMas, nSta))
	}
}

// TestZZAppointChiefMatchesTheOriginal 君主→1.指定軍師（Issue #90）：「<指定軍師>謀略須大於79／指定那一位」
// 是下令那一郡的清單（模式 6、鍵 1），面板、下面板、游標相同；擺一位不主事的舊軍師，指定謀略 90 的一般武將之後
// 三張表逐位元組相同（舊軍師 1→3、新人 3→1、諸侯 offset 6 換人），原版結束這個郡的回合。
func TestZZAppointChiefMatchesTheOriginal(t *testing.T) {
	b := newPickBoard(t)
	face := loadFace(t)
	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	master := b.base + uint32(int(b.me)*state.MasterRecordSize)
	lord := int(b.o.Word(addr(master + 2)))
	var two []int
	for _, i := range b.people {
		if i != lord && len(two) < 2 {
			two = append(two, i)
		}
	}
	oldChief, pick := two[0], two[1]
	gen := func(i int) uint32 { return b.base + uint32(nMas+nSta+i*state.GeneralRecordSize) }
	b.o.SetByte(addr(gen(oldChief)+17), uint8(state.StatusChief))
	b.o.SetByte(addr(gen(oldChief)+9), 85)
	b.o.SetByte(addr(gen(pick)+17), uint8(state.StatusOfficer))
	b.o.SetByte(addr(gen(pick)+9), 90)
	b.o.SetWord(addr(master+6), uint16(oldChief))
	g := b.game(t)
	total := nMas + nSta + state.GeneralTableSize
	again, done := 0, 0
	var after []byte
	b.o.OnCall(addr(mainAskAgainAt), func(*oracle.Oracle) { again++ })
	// 回合一結束原版就接著跑下一個郡，表要在「下完令」那一刻取。
	b.o.OnCall(addr(mainTurnDoneAt), func(o *oracle.Oracle) {
		if done++; after == nil {
			after = o.Bytes(addr(b.base), total)
		}
	})

	b.press(t, "君主", "7\r")
	ask, shot, tr := b.press(t, "指定軍師", "1\r")
	var idx []int
	row := 0
	for i, x := range g.PickRoster(b.at, game.PickWiseSub, game.PickByIntel) {
		idx = append(idx, x.Index)
		if x.Index == pick {
			row = i + 1
		}
	}
	rp := &ui.RosterPick{List: idx, Key: game.PickByIntel}
	if lo, hi := ui.RosterRange(rp); ask != [2]int{lo, hi} || row == 0 {
		t.Fatalf("名單：原版問 %v，remake %d-%d，新人在第 %d 列", ask, lo, hi, row)
	}
	v := ui.View{Sel: b.at, Roster: rp, Prompt: i18n.S("ask.chief") + i18n.Sf("pick.range", 1, len(idx)), Input: tr.input()}
	cv := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(cv, b.art, g, nil, v)
	v.Input = ui.InputCursor{}
	plain := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(plain, b.art, g, nil, v)
	comparePanels(t, "指定那一位", shot, cv, plain, tr, 1)

	before := b.o.Bytes(addr(b.base), total)
	b.o.Drain()
	b.o.TypeBoth(fmt.Sprintf("%d\r", row))
	waitBoot(t, b.o, "換軍師", 100_000_000, func() bool { return int(b.o.Word(addr(master+6))) == pick })
	for i := 0; i < 40 && again == 0 && done == 0; i++ {
		b.o.Drain()
		b.o.TypeBoth(" ")
		if err := b.o.Run(20_000_000); err != nil {
			t.Fatal(err)
		}
	}
	if done == 0 || again != 0 {
		t.Errorf("指定完原版回主選單 %d 次、結束回合 %d 次；應該結束回合", again, done)
	}
	if after == nil {
		t.Fatal("沒有攔到結束回合那一刻")
	}
	if err := (game.AppointChiefOrder{At: b.at, Target: pick}).Apply(g, b.me); err != nil {
		t.Fatal(err)
	}
	rm, rs, rg, err := g.Tables()
	if err != nil {
		t.Fatal(err)
	}
	got := append(append(append([]byte{}, rm...), rs...), rg...)
	t.Logf("原版動到的記錄：%s", changedRecords(before, after, nMas, nSta))
	if n := diffCount(after, got); n != 0 {
		t.Errorf("三張表兩邊差 %d 個位元組：%s", n, changedRecords(after, got, nMas, nSta))
	}
}
