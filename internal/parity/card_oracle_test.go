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
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// cardFn 是人物資料卡常式 `0xf874`。**要用它真正的段**呼叫：裡面有一個
// `push cs / call` 的近呼叫（`0xf8d9` → `0xf7b4`），用合成的段位址進去
// 會讓那一跳落到別的地方（`docs/spec/005` §9.2）。
var cardFn = oracle.Addr{Seg: 0x0f17, Off: 0x0704}

// TestZZPersonCardMatchesTheOriginal 直接呼叫原版的人物資料卡，對
// remake 的 `ui.DrawPersonCard`：灰底、肖像、肖像框、外框逐像素相同；每一行
// 字的文字區（x 424–623）兩邊都有墨或都沒墨（字模不比）。
//
// **remake 的盤面從原版的記憶體解出來**（`bootToGame` 載的是存檔，不是
// 劇本 001 的開局——那份存檔裡關羽已經死了），三位各走身分那一行的一個
// 分支：君主（「  現為君主  」）、在職的（`任%s%s` ＋ 忠心度）、
// 身分 8–12 的（只畫身分名、從 456 起）。
func TestZZPersonCardMatchesTheOriginal(t *testing.T) {
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
		t.Fatalf("原版記憶體裡的三張表解不開：%v", err)
	}
	players := sc.Players()
	if len(players) == 0 {
		t.Skip("這個盤面沒有玩家控制的勢力")
	}
	g, err := game.New(sc, state.FactionID(players[0]), 5, state.EditionBase)
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
	// 三個分支各挑第一個碰到的人。
	pick := map[string]int{"君主": -1, "在職": -1, "身分8–12": -1}
	for i, x := range g.AllGenerals() {
		if x == nil || x.Name == "" {
			continue
		}
		switch {
		case x.Status == state.StatusLord:
			if pick["君主"] < 0 {
				pick["君主"] = i
			}
		case x.Status >= 8 && x.Status <= 12:
			if pick["身分8–12"] < 0 {
				pick["身分8–12"] = i
			}
		case x.Employed():
			if pick["在職"] < 0 {
				pick["在職"] = i
			}
		}
	}
	idx := func(c color.RGBA) int {
		for i, p := range assets.EGAPalette {
			if p == c {
				return i
			}
		}
		return -1
	}
	// 文字行：(y, 高, 字級) 的清單，每一行的文字區是 x 424–623
	// （右邊那 8 格是外框的邊，照像素比）。
	type row struct {
		y, h, scale int
	}
	for _, kind := range []string{"君主", "在職", "身分8–12"} {
		who := pick[kind]
		if who < 0 {
			t.Errorf("盤面上沒有「%s」的人", kind)
			continue
		}
		x := g.General(who)
		t.Run(fmt.Sprintf("%s-%d-%s", kind, who, x.Name), func(t *testing.T) {
			if _, err := o.Call(cardFn, uint16(who)); err != nil {
				t.Fatal(err)
			}
			orig := o.IndexedEGASize(scrW, scrH)
			dumpScreen(t, o, fmt.Sprintf("card-%03d", who))

			cv := ui.NewCanvasPx(scrW, scrH, face)
			ui.DrawPersonCard(cv, art, g, who)
			if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
				savePNG(t, filepath.Join(dir, fmt.Sprintf("remake-card-%03d.png", who)), cv)
			}

			rows := []row{{52, 32, 2}, {84, 16, 1}, {100, 16, 1}, {116, 16, 1}, {132, 16, 1},
				{164, 16, 1}, {180, 16, 1}, {196, 16, 1}, {212, 16, 1}}
			inRow := func(y int) (row, bool) {
				for _, r := range rows {
					if y >= r.y && y < r.y+r.h {
						return r, true
					}
				}
				return row{}, false
			}
			faceBox := func(px, py int) bool { return px >= 528 && px < 608 && py >= 60 && py < 156 }
			bad, first := 0, ""
			inkOrig, inkMine := map[int]int{}, map[int]int{}
			for py := 36; py <= 291; py++ {
				for px := 408; px <= 631; px++ {
					op := int(orig[py*scrW+px] & 15)
					mp := idx(cv.Img.RGBAAt(px, py))
					r, textRow := inRow(py)
					switch {
					case textRow && px >= 424 && px < 624 && !faceBox(px, py):
						// 文字行：只數墨（不是灰底）。
						if op != 7 {
							inkOrig[r.y]++
						}
						if mp != 7 {
							inkMine[r.y]++
						}
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
				t.Errorf("灰底／肖像／框有 %d 個像素不同，第一個 %s", bad, first)
			}
			for _, r := range rows {
				if (inkOrig[r.y] == 0) != (inkMine[r.y] == 0) {
					t.Errorf("y %d 那一行：原版有墨 %d 點、remake %d 點——一邊沒字", r.y, inkOrig[r.y], inkMine[r.y])
				}
			}
			t.Logf("%s（身分 %d）：各行墨點 原版 %v remake %v", x.Name, x.Status, inkOrig, inkMine)
		})
	}
}
