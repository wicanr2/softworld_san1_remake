//go:build oracle

package parity

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// statusPanelFn 是主畫面「郡的資料」面板（線性 `0x33020`，25 個呼叫端）。
var statusPanelFn = oracle.Addr{Seg: 0x32fb, Off: 0x70}

// TestZZStatusPanelMatchesTheOriginal 對拍郡的資料面板（Issue #76）：讀出貨進度，
// 對每一個郡直接呼叫原版 `0x33020(郡)`，remake 同一盤面畫同一郡。文字格
// （`docs/spec/005` §2.1 的表，每 16 像素一列）比有沒有墨，面板裡其餘像素
// （外框、底色、肖像與 `FBRD`）逐格相同。軍師那一行要同時碰到「在這一郡」
// 與「不在這一郡」兩種。
func TestZZStatusPanelMatchesTheOriginal(t *testing.T) {
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
	face := loadFace(t)

	chiefHere, chiefAway, compared := 0, 0, 0
	for id := 1; id <= state.PrefectureCount; id++ {
		p := g.Prefecture(id)
		if !p.Owned() {
			continue
		}
		name := fmt.Sprintf("郡 %d %s", id, p.Name)
		if _, err := o.Call(statusPanelFn, uint16(id)); err != nil {
			t.Fatal(err)
		}
		orig := append([]uint8(nil), o.IndexedEGASize(scrW, scrH)...)
		cv := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(cv, art, g, nil, ui.View{Status: true, Sel: id})
		chiefInk, ok := compareStatusPanel(t, name, orig, cv)
		if chiefInk {
			chiefHere++
		} else {
			chiefAway++
		}
		compared++
		if !ok {
			if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
				dumpScreen(t, o, fmt.Sprintf("statuspanel-%02d-orig", id))
				savePNG(t, filepath.Join(dir, fmt.Sprintf("remake-statuspanel-%02d.png", id)), cv)
			}
		}
	}
	t.Logf("比了 %d 個有主的郡：軍師在郡裡 %d 個、不在 %d 個", compared, chiefHere, chiefAway)
	if chiefHere == 0 || chiefAway == 0 {
		t.Errorf("軍師那一行兩種情形沒有都碰到（在 %d、不在 %d）", chiefHere, chiefAway)
	}
}

// statusPanelText 是郡的資料面板上的文字格 [x0, y0, x1, y1]（含）；多列的格子
// 逐 16 像素一列比（`docs/spec/005` §2.1 的表）。主事者姓名欄從 520 起
// （6 byte 兩倍寬，三字名填滿）。
var statusPanelText = [][4]int{
	{424, 52, 487, 83},   // 郡名 32×32
	{488, 52, 535, 83},   // 州名、編號
	{536, 52, 623, 99},   // 君主、人望、軍師
	{424, 100, 527, 195}, // 自治狀態、五個欄位
	{424, 212, 519, 243}, // 金、米
	{424, 260, 527, 275}, // 在野武將
	{520, 212, 623, 243}, // 主事者 32×32
	{536, 244, 623, 275}, // 現役將、兵士
}

// compareStatusPanel 比右側面板 (408,36)–(631,291)：文字格逐列比有沒有墨（底色 3
// 以外都算墨），其餘像素逐格相同。回報原版軍師那一列有沒有墨、這一張有沒有對上。
func compareStatusPanel(t *testing.T, name string, orig []uint8, cv *ui.Canvas) (chiefInk, ok bool) {
	t.Helper()
	idx := func(x, y int) int {
		q := cv.Img.RGBAAt(x, y)
		for i, e := range assets.EGAPalette {
			if e == q {
				return i
			}
		}
		return -1
	}
	org := func(x, y int) int { return int(orig[y*scrW+x] & 15) }
	inText := func(x, y int) bool {
		for _, b := range statusPanelText {
			if x >= b[0] && x <= b[2] && y >= b[1] && y <= b[3] {
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
			if op, mp := org(x, y), idx(x, y); op != mp {
				bad++
				if first == "" {
					first = fmt.Sprintf("(%d,%d) 原版 %d remake %d", x, y, op, mp)
				}
			}
		}
	}
	rowsBad := 0
	for _, b := range statusPanelText {
		for y0 := b[1]; y0 <= b[3]; y0 += 16 {
			inked := func(pix func(x, y int) int) bool {
				for y := y0; y < y0+16; y++ {
					for x := b[0]; x <= b[2]; x++ {
						if pix(x, y) != 3 {
							return true
						}
					}
				}
				return false
			}
			om, mm := inked(org), inked(idx)
			if b[0] == 536 && y0 == 84 {
				chiefInk = om
			}
			if om != mm {
				rowsBad++
				t.Logf("%s：(%d,%d) 那一列 原版有墨 %v、remake %v", name, b[0], y0, om, mm)
			}
		}
	}
	if bad != 0 || rowsBad != 0 {
		t.Errorf("%s：文字格以外 %d 點不同（第一個 %s），文字列有墨不同 %d", name, bad, first, rowsBad)
		return chiefInk, false
	}
	return chiefInk, true
}
