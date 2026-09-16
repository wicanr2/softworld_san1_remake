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
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// 原版的訊息常式與它用到的兩支（`docs/spec/005` §9）。
const (
	msgFn       = 0x3273e            // msg(x1, y1, x2, y2, side, 肖像, 名字 far*, 片語×3)
	msgRndAt    = 0x32d52            // `RND(8)` 回來的下一道：AX ＝ 字色
	msgDoneAt   = 0x32df1            // 畫完、等完，`lret` 之前
	clearRectFn = 0x1058*16 + 0x27e8 // 清一塊（呼叫端在對白之前清右側面板）
	panelClear  = 1                  // 面板底色（藍）
)

// TestZZBubbleMatchesTheOriginal 直接呼叫原版的訊息常式，拿它畫出來的
// 那一格對 remake 的 `ui.DrawBubble`。
//
// 兩格各一次：出頭那兩則（`0x161c4` 上格、肖像在右；`0x16270` 下格、
// 肖像在左），片語與原版同一組（459／460、461／462），說話者是劇本 001
// 的呂蒙（151，肖像 28）與魯肅（132）。判準：
//
//   - 肖像 64×80、泡泡（白底、四邊、尾巴）、名字的黑底：**逐像素相同**
//     （remake 的字型不接原版，兩行對白與名字的字模不比）。
//   - 對白那兩行的框裡：兩邊都只有白與擲出來的那一色，而且都有墨。
func TestZZBubbleMatchesTheOriginal(t *testing.T) {
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
	genBase := base + uint32(state.MasterTableSize+state.PrefectureTableSize)

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
	newcomer, bond := 151, 132
	for _, who := range []int{newcomer, bond} {
		if x := g.General(who); x == nil || x.Name == "" {
			t.Fatalf("劇本 001 沒有人物 %d", who)
		}
	}

	colour := -1
	o.OnCall(addr(msgRndAt), func(o *oracle.Oracle) { colour = int(o.Regs().AX) })
	done := 0
	o.OnCall(addr(msgDoneAt), func(*oracle.Oracle) { done++ })

	type tc struct {
		name    string
		b       game.Bubble
		speaker int
		phrases [3]uint16
		text    string
	}
	cases := []tc{
		{"上格肖像在右（牽絆對象）", game.Bubble{X1: 424, Y1: 80, X2: 615, Y2: 175, Left: false},
			bond, [3]uint16{459, uint16(newcomer), 460},
			i18n.Sf("bub.debutBond", i18n.PersonName(g.General(newcomer).Name))},
		{"下格肖像在左（新人）", game.Bubble{X1: 424, Y1: 180, X2: 615, Y2: 275, Left: true},
			newcomer, [3]uint16{461, uint16(newcomer), 462},
			i18n.Sf("bub.debut", i18n.PersonName(g.General(newcomer).Name))},
		// #51 的兩種：登用時主事者在上格、肖像在左（`0x1c088`）；婉拒的
		// 那一位在下格、肖像在右（`0x1c0cc`）。
		{"上格肖像在左（登用的主事者）", game.Bubble{X1: 424, Y1: 80, X2: 615, Y2: 175, Left: true},
			bond, [3]uint16{390, uint16(newcomer), 391},
			i18n.Sf("bub.recruitAsk", i18n.PersonName(g.General(newcomer).Name))},
		{"下格肖像在右（婉拒的那一位）", game.Bubble{X1: 424, Y1: 180, X2: 615, Y2: 275, Left: false},
			newcomer, [3]uint16{392, 499, 499},
			i18n.S("bub.recruitNo")},
	}
	for _, k := range cases {
		t.Run(k.name, func(t *testing.T) {
			// 呼叫端在對白之前把右側面板清成藍色（`0x14899`／`0x149e3`）。
			if _, err := o.Call(addr(clearRectFn), 408, 36, 631, 291, panelClear); err != nil {
				t.Fatal(err)
			}
			side := uint16(0)
			if k.b.Left {
				side = 0xFFFF
			}
			x := g.General(k.speaker)
			nameLin := genBase + uint32(k.speaker*state.GeneralRecordSize)
			colour, done = -1, 0
			if _, err := o.Call(addr(msgFn), uint16(k.b.X1), uint16(k.b.Y1), uint16(k.b.X2), uint16(k.b.Y2),
				side, uint16(x.Portrait), uint16(nameLin&0xF), uint16(nameLin>>4),
				k.phrases[0], k.phrases[1], k.phrases[2]); err != nil {
				t.Fatal(err)
			}
			if done != 1 || colour < 0 {
				t.Fatalf("訊息常式走完 %d 次、字色 %d——hook 沒攔到", done, colour)
			}
			orig := o.IndexedEGASize(scrW, scrH)
			dumpScreen(t, o, "bubble-"+fmt.Sprint(k.speaker))

			// remake：同一塊藍底、同一格泡泡、同一個字色。
			cv := ui.NewCanvasPx(scrW, scrH, face)
			cv.FillRect(408, 36, 632, 292, assets.EGAPalette[panelClear])
			b := k.b
			b.Speaker, b.Color, b.Text = k.speaker, colour, k.text
			ui.DrawBubble(cv, art, g, &b)

			// 對白那兩行的框：字模不比，只比顏色集合與有沒有墨。
			tx, right := b.X1+8, b.X2-70
			if b.Left {
				tx, right = b.X1+72, b.X2-5
			}
			var textBox [2][4]int
			for i := range textBox {
				textBox[i] = [4]int{tx, b.Y1 + 12 + i*40, right, b.Y1 + 12 + i*40 + 32}
			}
			// 名字那一格：字模不比，只比黑底與字色。
			nx := b.X1 + 8
			if !b.Left {
				nx = b.X2 - 55
			}
			nameBox := [4]int{nx, b.Y1 + 80, nx + 48, b.Y1 + 96}
			inBox := func(x, y int, bx [4]int) bool {
				return x >= bx[0] && x < bx[2] && y >= bx[1] && y < bx[3]
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
			inkOrig, inkMine := [2]int{}, [2]int{}
			for y := b.Y1; y <= b.Y2; y++ {
				for x := b.X1; x <= b.X2; x++ {
					op := int(orig[y*scrW+x] & 15)
					mp := idx(cv.Img.RGBAAt(x, y))
					boxed := -1
					for i := range textBox {
						if inBox(x, y, textBox[i]) {
							boxed = i
						}
					}
					switch {
					case boxed >= 0:
						if op != 15 && op != colour {
							t.Fatalf("原版對白框裡 (%d,%d) 是色 %d，該只有白與 %d", x, y, op, colour)
						}
						if mp != 15 && mp != colour {
							t.Fatalf("remake 對白框裡 (%d,%d) 是色 %d，該只有白與 %d", x, y, mp, colour)
						}
						if op == colour {
							inkOrig[boxed]++
						}
						if mp == colour {
							inkMine[boxed]++
						}
					case inBox(x, y, nameBox):
						want := 12
						if !b.Left {
							want = 10
						}
						if op != 0 && op != want {
							t.Fatalf("原版名字格 (%d,%d) 是色 %d，該只有黑與 %d", x, y, op, want)
						}
						if mp != 0 && mp != want {
							t.Fatalf("remake 名字格 (%d,%d) 是色 %d，該只有黑與 %d", x, y, mp, want)
						}
					default:
						if op != mp {
							bad++
							if first == "" {
								first = fmt.Sprintf("(%d,%d) 原版 %d remake %d", x, y, op, mp)
							}
						}
					}
				}
			}
			if bad != 0 {
				t.Errorf("肖像／泡泡／黑底有 %d 個像素不同，第一個 %s", bad, first)
			}
			for i := range textBox {
				if (inkOrig[i] == 0) != (inkMine[i] == 0) {
					t.Errorf("第 %d 行：原版有墨 %d 點、remake %d 點——一邊沒字", i+1, inkOrig[i], inkMine[i])
				}
			}
			t.Logf("%s：字色 %d，對白墨點 原版 %v remake %v", k.name, colour, inkOrig, inkMine)
		})
	}
}

// faceFn 是畫肖像常式 `0xf7b4(x, y, 肖像, 模式, 翻面)`，用它真正的段呼叫。
var faceFn = oracle.Addr{Seg: 0x0f17, Off: 0x644}

// TestZZSearchFaceMatchesTheOriginal 釘住玩家尋訪找到人時亮的那一張肖像
// （`0x1bb7e`：`0xf7b4(488, 88, 肖像, 0, 0)`）：清成藍的面板上，原版與
// remake 的 `FaceOnly` 那一格逐像素相同。
func TestZZSearchFaceMatchesTheOriginal(t *testing.T) {
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
	bootToGame(t, o, seedMas)
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
	who := 151 // 呂蒙
	x := g.General(who)
	if _, err := o.Call(addr(clearRectFn), 408, 36, 631, 291, panelClear); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Call(faceFn, uint16(game.SearchFaceX), uint16(game.SearchFaceY), uint16(x.Portrait), 0, 0); err != nil {
		t.Fatal(err)
	}
	orig := o.IndexedEGASize(scrW, scrH)
	dumpScreen(t, o, "search-face")

	// 原版清的是外框裡面；外框（`SIDEB`）本身照主畫面拼一次。
	cv := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(cv, art, g, nil, ui.View{Sel: 1})
	ui.ClearPanel(cv, 408, 36, 631, 291, assets.EGAPalette[panelClear])
	ui.DrawBubble(cv, art, g, &game.Bubble{X1: game.SearchFaceX, Y1: game.SearchFaceY, Speaker: who, FaceOnly: true})
	idx := func(c color.RGBA) int {
		for i, p := range assets.EGAPalette {
			if p == c {
				return i
			}
		}
		return -1
	}
	bad, first := 0, ""
	for y := 36; y <= 291; y++ {
		for x := 408; x <= 631; x++ {
			if op, mp := int(orig[y*scrW+x]&15), idx(cv.Img.RGBAAt(x, y)); op != mp {
				bad++
				if first == "" {
					first = fmt.Sprintf("(%d,%d) 原版 %d remake %d", x, y, op, mp)
				}
			}
		}
	}
	if bad != 0 {
		t.Errorf("面板有 %d 個像素不同，第一個 %s", bad, first)
	}
}

// TestZZBattleSpeechMatchesTheOriginal 釘住戰場上的三種訊息框
// （`docs/spec/005` §9.7）：第三塊面板 (448,268)–(623,363) 肖像在右、單挑
// 攻方那一塊 (64,268)–(239,363) 肖像在左、守方那一塊 (256,268)–(431,363)
// 肖像在右。底圖用原版當下的畫面（呼叫端的填藍另外算），直接呼叫原版的
// 常式，肖像、泡泡、名字的黑底逐像素相同；對白框裡只有白與擲出來的那一色。
func TestZZBattleSpeechMatchesTheOriginal(t *testing.T) {
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
	genBase := base + uint32(state.MasterTableSize+state.PrefectureTableSize)
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
	colour := -1
	o.OnCall(addr(msgRndAt), func(o *oracle.Oracle) { colour = int(o.Regs().AX) })
	idx := func(c color.RGBA) int {
		for i, p := range assets.EGAPalette {
			if p == c {
				return i
			}
		}
		return -1
	}
	who := 151
	x := g.General(who)
	nameLin := genBase + uint32(who*state.GeneralRecordSize)
	for _, k := range []struct {
		name   string
		sp     battle.Speech
		phrase uint16
	}{
		{"第三塊面板（殺）", battle.Speech{Speaker: who, Box: battle.BoxThird, Text: i18n.S("bub.kill")}, 432},
		{"單挑攻方那一塊（肖像在左）", battle.Speech{Speaker: who, Box: battle.BoxAttacker, Left: true, Text: i18n.S("bub.duelLater2")}, 438},
		{"單挑守方那一塊（肖像在右）", battle.Speech{Speaker: who, Box: battle.BoxDefender, Text: i18n.S("bub.duelLater1")}, 437},
		// Issue #60：用計三道門（`0x28d48`）與打完回郡（`0x256fa`）也是第三塊面板。
		{"第三塊面板（資金不足）", battle.Speech{Speaker: who, Box: battle.BoxThird, Text: i18n.S("bub.plotGold")}, 479},
		{"第三塊面板（回郡）", battle.Speech{Speaker: who, Box: battle.BoxThird, Text: i18n.S("bub.helperReturn")}, 478},
	} {
		t.Run(k.name, func(t *testing.T) {
			x1, y1, x2, y2 := assets.BattleWide.Panel(k.sp.Box.Panel())
			before := o.IndexedEGASize(scrW, scrH)
			cv := ui.NewCanvasPx(scrW, scrH, face)
			for y := 0; y < scrH; y++ {
				for xx := 0; xx < scrW; xx++ {
					cv.Img.SetRGBA(xx, y, assets.EGAPalette[before[y*scrW+xx]&15])
				}
			}
			side := uint16(0)
			if k.sp.Left {
				side = 0xFFFF
			}
			colour = -1
			if _, err := o.Call(addr(msgFn), uint16(x1), uint16(y1), uint16(x2), uint16(y2),
				side, uint16(x.Portrait), uint16(nameLin&0xF), uint16(nameLin>>4),
				k.phrase, 499, 499); err != nil {
				t.Fatal(err)
			}
			if colour < 0 {
				t.Fatal("字色那一擲沒攔到")
			}
			orig := o.IndexedEGASize(scrW, scrH)
			dumpScreen(t, o, "speech-"+fmt.Sprint(k.phrase))
			sp := k.sp
			sp.Color = colour
			ui.DrawBubble(cv, art, g, &game.Bubble{X1: x1, Y1: y1, X2: x2, Y2: y2, Left: sp.Left,
				Speaker: sp.Speaker, Color: sp.Color, Text: sp.Text})

			tx, right := x1+8, x2-70
			if sp.Left {
				tx, right = x1+72, x2-5
			}
			nx := x1 + 8
			if !sp.Left {
				nx = x2 - 55
			}
			inText := func(px, py int) bool {
				line1 := py >= y1+12 && py < y1+44
				line2 := py >= y1+52 && py < y1+84
				return px >= tx && px < right && (line1 || line2)
			}
			inName := func(px, py int) bool { return px >= nx && px < nx+48 && py >= y1+80 && py < y1+96 }
			bad, first, ink := 0, "", 0
			for py := y1; py <= y2; py++ {
				for px := x1; px <= x2; px++ {
					op, mp := int(orig[py*scrW+px]&15), idx(cv.Img.RGBAAt(px, py))
					switch {
					case inText(px, py):
						if (op != 15 && op != colour) || (mp != 15 && mp != colour) {
							t.Fatalf("對白框裡 (%d,%d) 原版 %d remake %d，該只有白與 %d", px, py, op, mp, colour)
						}
						if op == colour {
							ink++
						}
					case inName(px, py):
					default:
						if op != mp {
							bad++
							if first == "" {
								first = fmt.Sprintf("(%d,%d) 原版 %d remake %d", px, py, op, mp)
							}
						}
					}
				}
			}
			if bad != 0 {
				t.Errorf("肖像／泡泡／黑底有 %d 個像素不同，第一個 %s", bad, first)
			}
			if ink == 0 {
				t.Error("原版的對白框裡沒有字")
			}
			t.Logf("%s：字色 %d，對白墨點 %d", k.name, colour, ink)
		})
	}
}
