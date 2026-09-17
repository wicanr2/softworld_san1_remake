//go:build oracle

package parity

import (
	"encoding/binary"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// 玩家人數 0–16（Issue #68，`docs/spec/019`）。
//
// 原版開兩局：2 人（第 1 位選曹操、第 2 位選劉備——**故意倒過來**，
// 玩家序號才分得出「照選的順序」與「照槽號」）與 0 人（示範模式）。
// 每一局在第一個郡回合入口（`0x1746e`）拍三張表與 `es:0x3120` 的
// 玩家序號表，與 `game.NewPlayers` 同樣的選擇逐位元組比。

// playersBoard 是第一個郡回合入口拍到的東西。
type playersBoard struct {
	tables []byte
	index  [16]uint16 // es:[0x3120 + 槽×2]：玩家序號，0xFFFF ＝ 沒人
}

// armPlayersBoard 掛第一個郡回合入口的鉤子。
func armPlayersBoard(o *oracle.Oracle) *playersBoard {
	b := &playersBoard{}
	o.OnCall(addr(0x1746e), func(o *oracle.Oracle) {
		if b.tables != nil {
			return
		}
		b.tables = o.Bytes(addr(0x399b0), state.MasterTableSize+state.PrefectureTableSize+state.GeneralTableSize)
		for f := range b.index {
			b.index[f] = o.Word(oracle.Addr{Seg: o.ES(), Off: uint16(0x3120 + 2*f)})
		}
	})
	return b
}

// finishNewGame 等難度的數字輸入（從 d.nums[since:] 起找）、送難度，
// 跑到第一個郡回合入口；中途抽中防拷盤問就答。
func finishNewGame(t *testing.T, d *newGameDrive, b *playersBoard, difficulty, since int) {
	t.Helper()
	o, s := d.o, d.s
	waitBoot(t, o, "新局難度輸入", 1_000_000_000, func() bool {
		for _, n := range d.nums[since:] {
			if n.lo == 1 && n.hi == 10 {
				return true
			}
		}
		return false
	})
	waitBootScan(t, o, "新局難度掃描碼", 5_000_000)
	beforePassword := s.passwordAsk
	o.Drain()
	o.TypeBoth(fmt.Sprintf("%d\r", difficulty))
	waitBoot(t, o, "第一個郡回合或防拷盤問", 500_000_000, func() bool {
		return b.tables != nil || s.passwordAsk > beforePassword
	})
	if b.tables == nil {
		o.Drain()
		o.Type(passwordAnswer + "\r")
		beforeYN := s.passwordYN
		waitBoot(t, o, "新局密碼確認", 500_000_000, func() bool { return s.passwordYN > beforeYN })
		o.Drain()
		o.PressScan("Y")
		waitBoot(t, o, "第一個郡回合", 500_000_000, func() bool { return b.tables != nil })
	}
}

// checkPlayersBoard 比 `game.NewPlayers(players)` 與原版拍到的表。
func checkPlayersBoard(t *testing.T, sc0 *state.Scenario, b *playersBoard, players []state.FactionID) {
	t.Helper()
	for f, v := range b.index {
		want := uint16(0xFFFF)
		for i, p := range players {
			if int(p) == f {
				want = uint16(i)
			}
		}
		if v != want {
			t.Errorf("es:0x3120 槽 %d 是 %#x，想要 %#x（整張 %x）", f, v, want, b.index)
		}
	}
	g, err := game.NewPlayers(sc0, players, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range players {
		if g.Players[i] != p {
			t.Errorf("remake 第 %d 位是勢力 %d，想要 %d", i+1, g.Players[i], p)
		}
	}
	rm, rs, rg, err := g.Tables()
	if err != nil {
		t.Fatal(err)
	}
	mine := append(append(append([]byte{}, rm...), rs...), rg...)
	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	byField := map[string]int{}
	for i := range mine {
		if mine[i] != b.tables[i] {
			byField[whichField(i, nMas, nSta)]++
		}
	}
	for k, n := range byField {
		t.Logf("%s 差 %d 格", k, n)
		if !newGameRuntimeFields[k] {
			t.Errorf("開局的三張表與 remake 不同：%s（%d 格）", k, n)
		}
	}
	for f := 0; f < 16; f++ {
		ctl := binary.LittleEndian.Uint16(b.tables[f*72:])
		human := false
		for _, p := range players {
			human = human || int(p) == f
		}
		if human && ctl != 1 {
			t.Errorf("玩家勢力 %d 的操縱方是 %#x，不是 1", f, ctl)
		}
		if !human && ctl == 1 {
			t.Errorf("沒被選的勢力 %d 操縱方是 1", f)
		}
	}
}

// TestZZTwoPlayersMatchTheOriginal 走 2 人：人數那一格、「第2位,請選擇」
// 那一格的畫面，再比開局的表。
func TestZZTwoPlayersMatchTheOriginal(t *testing.T) {
	root := origRoot(t)
	sc0, err := state.LoadScenario(openContainer(t, filepath.Join(root, "DATA2")), state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	board := armPlayersBoard(o)
	tr := trackCursor(o)
	d := bootToPlayerCount(t, o)
	waitCursorShown(t, o, tr, "人數")
	count := append([]uint8(nil), o.IndexedEGASize(scrW, scrH)...)
	countCursor := *tr
	dumpScreen(t, o, "players-count")
	o.Drain()
	o.TypeBoth("2\r")
	d.waitNum("第1位君主輸入", 1, 6)
	waitCursorShown(t, o, tr, "第1位")
	first := append([]uint8(nil), o.IndexedEGASize(scrW, scrH)...)
	o.Drain()
	o.TypeBoth(fmt.Sprintf("%d\r", caoCaoPick))
	d.waitNum("第2位君主輸入", 1, 6)
	waitCursorShown(t, o, tr, "第2位")
	second := append([]uint8(nil), o.IndexedEGASize(scrW, scrH)...)
	secondCursor := *tr
	dumpScreen(t, o, "players-second")
	o.Drain()
	since := len(d.nums)
	o.TypeBoth("1\r")
	finishNewGame(t, d, board, 5, since)
	checkPlayersBoard(t, sc0, board, []state.FactionID{1, 0})

	// 畫面：底都是第一頁六位；人數那一格提示是洋紅 13，第 2 位那一格
	// 曹操（槽 1）肖像下印著 1、提示框是綠 10。
	g, err := game.New(sc0, 0, 5, state.EditionBase)
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
	render := func(mark int, prompt, ask string, in ui.InputCursor) *ui.Canvas {
		var slots []ui.LordPickSlot
		for f := 0; f < ui.LordPickPerPage; f++ {
			s := ui.LordPickSlot{Number: f + 1, Faction: f, Lord: g.Lord(state.FactionID(f))}
			if f == mark {
				s.Player = 1
			}
			slots = append(slots, s)
		}
		cv := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawLordPick(cv, art, g, slots, -1, prompt, 0)
		ui.DrawLordPickCursor(cv, art, prompt, in)
		if ask != "" {
			ui.DrawLordPickAsk(cv, ask)
			ui.DrawLordPickAskCursor(cv, art, ask, in)
		}
		return cv
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
	in := func(b box, x, y int) bool { return x >= b.x0 && x <= b.x1 && y >= b.y0 && y <= b.y1 }
	var text []box
	for i := 0; i < ui.LordPickPerPage; i++ {
		x, y := 420+68*(i%3), 56+128*(i/3)
		text = append(text, box{x + 15, y + 83, x + 64, y + 116}, box{x - 3, y + 99, x + 14, y + 116})
	}
	// compare 比框外逐像素、提示框裡那個顏色的墨兩邊都有、序號那一格
	// 兩邊變動的範圍重疊（字模不接原版，`CLAUDE.md` §3.3）。
	compare := func(name string, orig []uint8, cv *ui.Canvas, prompt box, ink int,
		mark box, before []uint8, noMark *ui.Canvas) {
		bad, firstBad := 0, ""
		po, pm := 0, 0
		var om, mm []int
		for y := 0; y < scrH; y++ {
			for x := 72; x < scrW; x++ {
				i := y*scrW + x
				op, mp := int(orig[i]&15), idx(cv.Img.RGBAAt(x, y))
				if in(prompt, x, y) {
					if op == ink {
						po++
					}
					if mp == ink {
						pm++
					}
					continue
				}
				if before != nil && in(mark, x, y) {
					if orig[i] != before[i] {
						om = append(om, x, y)
					}
					if cv.Img.RGBAAt(x, y) != noMark.Img.RGBAAt(x, y) {
						mm = append(mm, x, y)
					}
					continue
				}
				skip := false
				for _, b := range text {
					skip = skip || in(b, x, y)
				}
				if !skip && op != mp {
					bad++
					if firstBad == "" {
						firstBad = fmt.Sprintf("(%d,%d) 原版 %d remake %d", x, y, op, mp)
					}
				}
			}
		}
		if bad != 0 {
			t.Errorf("%s：框外有 %d 個像素不同，第一個 %s", name, bad, firstBad)
			return
		}
		if po == 0 || pm == 0 {
			t.Errorf("%s：提示框顏色 %d 的墨 原版 %d 點、remake %d 點", name, ink, po, pm)
		}
		if before == nil {
			t.Logf("%s：框外逐像素相同；提示墨 原版 %d remake %d", name, po, pm)
			return
		}
		bb := func(p []int) box {
			b := box{scrW, scrH, -1, -1}
			for k := 0; k+1 < len(p); k += 2 {
				b.x0, b.y0 = min(b.x0, p[k]), min(b.y0, p[k+1])
				b.x1, b.y1 = max(b.x1, p[k]), max(b.y1, p[k+1])
			}
			return b
		}
		ob, mb := bb(om), bb(mm)
		if len(om) == 0 || len(mm) == 0 || ob.x1 < mb.x0 || mb.x1 < ob.x0 || ob.y1 < mb.y0 || mb.y1 < ob.y0 {
			t.Errorf("%s：玩家序號 原版變了 %d 點 %+v，remake %d 點 %+v", name, len(om)/2, ob, len(mm)/2, mb)
		}
		t.Logf("%s：框外逐像素相同；提示墨 原版 %d remake %d；序號 原版 %+v remake %+v", name, po, pm, ob, mb)
	}
	cvCount := render(-1, "", "請問有幾人玩(0-16):", countCursor.input())
	compare("人數", count, cvCount, box{423, 331, 624, 360}, 13, box{}, nil, nil)
	// 提示後面的輸入游標（開新局設定那一組 `CURD`）逐像素比，反對照是不畫游標的同一張。
	pix := func(cv *ui.Canvas) func(x, y int) int {
		return func(x, y int) int { return idx(cv.Img.RGBAAt(x, y)) }
	}
	shot := func(b []uint8) func(x, y int) int { return func(x, y int) int { return int(b[y*scrW+x] & 15) } }
	compareCursorCell(t, "人數", countCursor, 3, shot(count), pix(cvCount),
		pix(render(-1, "", "請問有幾人玩(0-16):", ui.InputCursor{})))

	noMark := render(-1, "第2位,請選擇(1-16):", "", secondCursor.input())
	cvSecond := render(caoCaoPick-1, "第2位,請選擇(1-16):", "", secondCursor.input())
	compareCursorCell(t, "第2位", secondCursor, 3, shot(second), pix(cvSecond),
		pix(render(caoCaoPick-1, "第2位,請選擇(1-16):", "", ui.InputCursor{})))
	markBox := box{420 + 68 + 15, 56 + 72, 420 + 68 + 16 + 64, 56 + 90}
	compare("第2位", second, cvSecond, box{423, 331, 624, 348}, 10, markBox, first, noMark)
	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		savePNG(t, filepath.Join(dir, "remake-players-count.png"), cvCount)
		savePNG(t, filepath.Join(dir, "remake-players-second.png"), cvSecond)
	}
}

// TestZZZeroPlayersMatchTheOriginal 走 0 人：「電腦自動示範模式」等一個鍵、
// 照樣問難度，開局的表裡沒有玩家。
func TestZZZeroPlayersMatchTheOriginal(t *testing.T) {
	root := origRoot(t)
	sc0, err := state.LoadScenario(openContainer(t, filepath.Join(root, "DATA2")), state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	board := armPlayersBoard(o)
	d := bootToPlayerCount(t, o)
	o.Drain()
	beforeNums := len(d.nums)
	o.TypeBoth("0\r")
	// 「電腦自動示範模式」之後等一個鍵；等鍵的迴圈不是君主／難度那一支，
	// 所以不猜停點：跑一段沒到難度就補一個鍵，最多補三次。
	asked := func() bool {
		for _, n := range d.nums[beforeNums:] {
			if n.lo == 1 && n.hi == 10 {
				return true
			}
		}
		return false
	}
	keys := 0
	for ; keys <= 3; keys++ {
		cond := oracle.NewCond("示範模式之後的難度", func(*oracle.Oracle) bool { return asked() })
		if err := o.RunUntil(cond, oracle.Budget(200_000_000)); err == nil {
			break
		}
		if keys == 0 {
			dumpScreen(t, o, "players-demo")
			c := o.Regs()
			t.Logf("示範模式等鍵停在 %04x:%04x", c.CS, c.IP)
		}
		o.Drain()
		o.PressScan("\r")
	}
	if !asked() {
		t.Fatalf("補了 %d 個鍵還沒問難度（nums=%v）", keys, d.nums[beforeNums:])
	}
	finishNewGame(t, d, board, 5, beforeNums)
	t.Logf("示範模式補鍵 %d 次", keys)
	checkPlayersBoard(t, sc0, board, nil)
}

// TestZZTwoPlayersTakeTurnsLikeTheOriginal 對拍兩位玩家的月迴圈形狀
// （`docs/spec/019` §2）：原版開 2 人局（曹操、劉備），第一個月每停一次主命令
// 就休息（內政 → 休息 → Y），記下停在哪個郡，一直到月底結算 `0x1581c`。
// remake 拿原版第一個郡回合那一刻的三張表、順序表與游標，`AdvanceToHuman`
// ＋`EndTurn` 走同一個月，停的郡、順序、那一格的主人要與原版相同。
//
// 比的是**停點序列**，不是整月的位元組：兩邊的電腦諸侯中間各自擲骰，
// 那一段由 `TestZZMonthParity` 管。
func TestZZTwoPlayersTakeTurnsLikeTheOriginal(t *testing.T) {
	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	board := armPlayersBoard(o)
	var order []int
	cursor, cur, monthEnd := -1, -1, false
	o.OnCall(addr(0x1746e), func(o *oracle.Oracle) {
		cur = int(int16(o.Arg(0)))
		if order == nil {
			ds := uint32(o.DSReg()) * 16
			cursor = int(int16(o.Word(addr(uint32(o.Word(addr(ds+0xa726)))*16 + 0x20f4))))
			ord := uint32(o.Word(addr(ds+0xa72c)))*16 + 0x0e
			order = make([]int, 43)
			for i := range order {
				order[i] = int(int16(o.Word(addr(ord + uint32(i*2)))))
			}
		}
	})
	o.OnCall(addr(0x1581c), func(*oracle.Oracle) { monthEnd = true })
	type stop struct{ at, owner int }
	var stops []stop
	o.OnCall(addr(bootNumInputFn), func(o *oracle.Oracle) {
		if o.Caller().Linear() != bootMainCmdCaller || monthEnd {
			return
		}
		if n := len(stops); n > 0 && stops[n-1].at == cur {
			return // 同一格回到主命令（子選單取消）不算新的一停
		}
		owner := int(o.Byte(addr(0x399b0 + uint32(state.MasterTableSize+cur*176+30))))
		stops = append(stops, stop{cur, owner})
	})

	d := bootToPlayerCount(t, o)
	s := d.s
	o.Drain()
	o.TypeBoth("2\r")
	d.waitNum("第1位君主輸入", 1, 6)
	o.Drain()
	o.TypeBoth(fmt.Sprintf("%d\r", caoCaoPick))
	d.waitNum("第2位君主輸入", 1, 6)
	o.Drain()
	since := len(d.nums)
	o.TypeBoth("1\r")
	finishNewGame(t, d, board, 5, since)

	rests := 0
	for i := 0; i < 400 && !monthEnd; i++ {
		bm, bk, bp := s.mainAsk, d.keyCalls, s.passwordAsk
		waitBoot(t, o, fmt.Sprintf("第 %d 次停點", i+1), 1_000_000_000, func() bool {
			return monthEnd || s.mainAsk > bm || d.keyCalls > bk || s.passwordAsk > bp
		})
		switch {
		case monthEnd:
		case s.passwordAsk > bp:
			// 月中的密碼欄位要用掃描碼送；字元佇列那條進不去（`TestZZUnifyYearOriginal` 同一個坑）。
			beforeYN := s.passwordYN
			o.Drain()
			o.PressScan(passwordAnswer + "\r")
			if err := o.RunUntil(oracle.NewCond("密碼確認", func(*oracle.Oracle) bool { return s.passwordYN > beforeYN }),
				oracle.Budget(500_000_000)); err != nil {
				dumpScreen(t, o, "turns-password-stuck")
				t.Fatalf("第 %d 次停點（停了 %v）的密碼確認：%v", i+1, stops, err)
			}
			o.Drain()
			o.PressScan("Y")
		case s.mainAsk > bm:
			waitBootScan(t, o, "主命令", 5_000_000)
			ba := s.affairsAsk
			o.Drain()
			o.PressScan("4\r")
			waitBoot(t, o, "內政選單", 500_000_000, func() bool { return s.affairsAsk > ba })
			waitBootScan(t, o, "內政選單掃描碼", 5_000_000)
			br := s.restYN
			o.Drain()
			o.PressScan("4\r")
			waitBoot(t, o, "休息確認", 500_000_000, func() bool { return s.restYN > br })
			o.Drain()
			o.PressScan("Y")
			rests++
		default: // 就任對白之類等一個鍵
			o.Drain()
			o.PressScan("\r")
		}
	}
	if !monthEnd {
		t.Fatalf("400 次停點還沒走到月底結算（停了 %v）", stops)
	}
	t.Logf("原版：游標 %d 起、順序表 %v；停了 %d 次 %v（休息 %d 次）", cursor, order, len(stops), stops, rests)
	if len(stops) == 0 {
		t.Fatal("原版一個月都沒停在玩家的郡——停點攔錯了")
	}
	owners := map[int]bool{}
	for _, st := range stops {
		owners[st.owner] = true
	}
	if !owners[0] || !owners[1] {
		t.Errorf("原版的停點沒有兩位玩家都輪到：%v", stops)
	}

	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	sc, err := state.DecodeTables(state.Slot("001"), board.tables[:nMas], board.tables[nMas:nMas+nSta], board.tables[nMas+nSta:])
	if err != nil {
		t.Fatal(err)
	}
	g, err := game.NewPlayers(sc, []state.FactionID{1, 0}, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	brain, err := ai.New(ai.ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	ss := session.New(g, brain, 1)
	ss.MonthOrder, ss.MonthCursor = order, cursor
	month := g.Date
	var mine []stop
	for g.Date == month && len(mine) <= len(stops)+4 {
		at := ss.AdvanceToHuman(0)
		if at == 0 || g.Date != month {
			break
		}
		mine = append(mine, stop{at, int(ss.Player)})
		ss.EndTurn()
	}
	if fmt.Sprint(mine) != fmt.Sprint(stops) {
		t.Errorf("停點不同：\n原版   %v\nremake %v", stops, mine)
	} else {
		t.Logf("remake 停點逐一相同：%v", mine)
	}
}
