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
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// TestZZDemoCameraMatchesTheOriginal 對拍示範模式月底的鏡頭（`0x1e1fc`，Issue #73）：
// 原版開 0 人局，每次進 `0x1e1fc` 拍三張表、被看的郡與人（`DS:0x771c`／`0x771e`）、
// 亂數狀態與月份，出來（`0x1582c`）再拍一次，並記下它叫了郡資料面板
// （`0x32fb:0x70`）還是人物卡（`0xf17:0x704`）與參數。remake 從進去那一刻的
// 狀態跑 `RunDemoCamera`：換到的郡與人、排的那一格、之後的亂數狀態都要相同。
// 至少四個月，奇偶兩種都要走到。
func TestZZDemoCameraMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	type shot struct {
		tables                 []byte
		pref, person, month    int
		seed, seedAfter        uint32
		prefAfter, personAfter int
		panel, card            int
		screen                 []uint8
	}
	var shots []shot
	var cur *shot
	ds := func(o *oracle.Oracle) uint32 { return uint32(o.DSReg()) * 16 }
	seedOf := func(o *oracle.Oracle) uint32 {
		return uint32(o.Word(addr(ds(o)+unifySeedVar))) | uint32(o.Word(addr(ds(o)+unifySeedVar+2)))<<16
	}
	o.OnCall(addr(0x1e1fc), func(o *oracle.Oracle) {
		cur = &shot{
			tables: o.Bytes(addr(unifyTablesBase), state.MasterTableSize+state.PrefectureTableSize+state.GeneralTableSize),
			pref:   int(o.Word(addr(ds(o) + 0x771c))), person: int(o.Word(addr(ds(o) + 0x771e))),
			month: int(o.Word(addr(workSeg(o) + unifyMonthOff))), seed: seedOf(o),
		}
	})
	o.OnCall(addr(0x33020), func(o *oracle.Oracle) {
		if cur != nil {
			cur.panel = int(int16(o.Arg(0)))
		}
	})
	o.OnCall(addr(0xf874), func(o *oracle.Oracle) {
		if cur != nil {
			cur.card = int(int16(o.Arg(0)))
		}
	})
	o.OnCall(addr(0x1e33b), func(o *oracle.Oracle) { // 兩條路畫完都落在這裡
		if cur != nil {
			cur.screen = append([]uint8(nil), o.IndexedEGASize(scrW, scrH)...)
		}
	})
	o.OnCall(addr(0x1582c), func(o *oracle.Oracle) {
		if cur == nil {
			return
		}
		cur.prefAfter, cur.personAfter = int(o.Word(addr(ds(o)+0x771c))), int(o.Word(addr(ds(o)+0x771e)))
		cur.seedAfter = seedOf(o)
		shots = append(shots, *cur)
		cur = nil
	})

	s := bootToDemo(t, o, 5)
	handled := s.passwordAsk
	odd, even := 0, 0
	for i := 0; i < 400 && (len(shots) < 4 || odd == 0 || even == 0); i++ {
		stop := oracle.NewCond("鏡頭或密碼", func(*oracle.Oracle) bool {
			return len(shots) > odd+even || s.passwordAsk > handled
		})
		if err := o.RunUntil(stop, oracle.Budget(200_000_000)); err != nil && !isBudget(err) {
			t.Fatalf("原版停止：%v", err)
		}
		if s.passwordAsk > handled {
			handled = s.passwordAsk
			o.Drain()
			o.PressScan(passwordAnswer + "\r")
			beforeYN := s.passwordYN
			waitBoot(t, o, "示範中的密碼確認", 500_000_000, func() bool { return s.passwordYN > beforeYN })
			o.Drain()
			o.PressScan("Y")
		}
		for len(shots) > odd+even {
			if shots[odd+even].month%2 == 0 {
				even++
			} else {
				odd++
			}
		}
	}
	if len(shots) < 4 || odd == 0 || even == 0 {
		t.Fatalf("只拍到 %d 個月底（奇 %d 偶 %d）", len(shots), odd, even)
	}

	art, err := ui.NewArtScreen(openContainer(t, filepath.Join(root, "DATA3")),
		openContainer(t, filepath.Join(root, "DATA1")))
	if err != nil {
		t.Fatal(err)
	}
	face := loadFace(t)
	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	for k, sh := range shots {
		name := fmt.Sprintf("第 %d 次（%d 月）", k+1, sh.month)
		sc, err := state.DecodeTables(state.Slot("001"), sh.tables[:nMas], sh.tables[nMas:nMas+nSta], sh.tables[nMas+nSta:])
		if err != nil {
			t.Fatal(err)
		}
		g, err := game.NewPlayers(sc, nil, 5, state.EditionBase)
		if err != nil {
			t.Fatal(err)
		}
		g.SetDemoCamera(sh.pref, sh.person)
		g.SeedRand(sh.seed)
		g.Date.Month = sh.month
		events := g.RunDemoCamera()
		p, x := g.DemoCamera()
		t.Logf("%s：原版 郡 %d→%d 人 %d→%d 面板 %d 卡 %d；remake 郡 %d 人 %d", name,
			sh.pref, sh.prefAfter, sh.person, sh.personAfter, sh.panel, sh.card, p, x)
		if p != sh.prefAfter || x != sh.personAfter {
			t.Errorf("%s：換到的郡／人 原版 %d／%d，remake %d／%d", name, sh.prefAfter, sh.personAfter, p, x)
		}
		if g.RandSeed() != sh.seedAfter {
			t.Errorf("%s：之後的亂數狀態 原版 %#08x，remake %#08x", name, sh.seedAfter, g.RandSeed())
		}
		if len(events) != 1 || events[0].Bubble == nil {
			t.Fatalf("%s：remake 排了 %+v", name, events)
		}
		b := events[0].Bubble
		switch {
		case sh.panel != 0 && (b.Panel != sh.panel || b.Card):
			t.Errorf("%s：原版畫郡 %d 的資料面板，remake 排了 %+v", name, sh.panel, b)
		case sh.panel == 0 && (!b.Card || b.Speaker != sh.card):
			t.Errorf("%s：原版畫人物 %d 的卡，remake 排了 %+v", name, sh.card, b)
		}
		compareCameraPanel(t, name, sh.screen, art, face, g, b, k+1)
	}
}

// compareCameraPanel 比鏡頭畫完那一刻的右側面板：郡資料面板走
// `compareStatusPanel`（`statuspanel_oracle_test.go`）；人物卡比文字行（x 424–623、
// y 52 起，第一列 32 高、其餘每列 16）有沒有墨，其餘像素逐格相同。
func compareCameraPanel(t *testing.T, name string, orig []uint8, art *ui.ArtScreen, face *font.Face,
	g *game.State, b *game.Bubble, n int) {
	t.Helper()
	cv := ui.NewCanvasPx(scrW, scrH, face)
	if b.Card {
		ui.DrawPersonCard(cv, art, g, b.Speaker)
	} else {
		ui.DrawArtSession(cv, art, g, nil, ui.View{Status: true, Sel: b.Panel})
	}
	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		savePNG(t, filepath.Join(dir, fmt.Sprintf("remake-democamera-%d.png", n)), cv)
	}
	if !b.Card {
		if _, ok := compareStatusPanel(t, name, orig, cv); ok {
			t.Logf("%s：郡資料面板文字格以外逐像素相同、文字列有墨相同", name)
		}
		return
	}
	idx := func(x, y int) int {
		p := cv.Img.RGBAAt(x, y)
		for i, q := range assets.EGAPalette {
			if q == p {
				return i
			}
		}
		return -1
	}
	org := func(x, y int) int { return int(orig[y*scrW+x] & 15) }
	bad, first := 0, ""
	for y := 36; y <= 291; y++ {
		for x := 408; x <= 631; x++ {
			if x >= 424 && x <= 623 && y >= 52 && y < 228 {
				continue // 文字行另外比
			}
			if op, mp := org(x, y), idx(x, y); op != mp {
				bad++
				if first == "" {
					first = fmt.Sprintf("(%d,%d) 原版 %d remake %d", x, y, op, mp)
				}
			}
		}
	}
	rowsBad, rows := 0, 0
	for y0 := 52; y0 < 228; {
		h := 16
		if y0 == 52 {
			h = 32
		}
		inked := func(pix func(x, y int) int) bool {
			bg := pix(424, y0)
			for y := y0; y < y0+h; y++ {
				for x := 424; x <= 623; x++ {
					if pix(x, y) != bg {
						return true
					}
				}
			}
			return false
		}
		o, m := inked(org), inked(idx)
		rows++
		if o != m {
			rowsBad++
			t.Logf("%s：y %d 那一列 原版有墨 %v、remake %v", name, y0, o, m)
		}
		y0 += h
	}
	if bad != 0 || rowsBad != 0 {
		t.Errorf("%s：人物卡文字行以外 %d 點不同（第一個 %s），文字行有墨不同 %d／%d", name, bad, first, rowsBad, rows)
		return
	}
	t.Logf("%s：人物卡文字行以外逐像素相同，%d 列文字行有墨相同", name, rows)
}
