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
	for i := 200; have < 14 && i < 350; i++ {
		rec := b.base + uint32(nMas+nSta+i*state.GeneralRecordSize)
		if o.Byte(addr(rec)) < 0xa1 { // 沒有名字的填充筆不要
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
func comparePanels(t *testing.T, name string, orig []uint8, cv, plain *ui.Canvas, tr cursorTrack, style int) {
	t.Helper()
	org := func(x, y int) int { return int(orig[y*scrW+x] & 15) }
	mine := func(x, y int) int { return paletteIndex(cv.Img.RGBAAt(x, y)) }
	type area struct{ x0, y0, cols, rows, bg int }
	text := []area{
		{440, 62, 23, 1, 1},  // 表頭
		{440, 84, 23, 12, 1}, // 十二列
		{424, 300, 24, 4, -1},
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
