//go:build oracle

package parity

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/menu"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// TestZZSaveScreenMatchesTheOriginal 對拍「其他 → 儲存進度」（Issue #74）：
// 原版讀出貨進度、主命令送「9」「2」，停在 `0x33d8:0x115e(1, 6)`（返回位址
// `0x1e581`）存畫面，remake 用同一份 `SAVENAME.SVP` 畫存檔那一格比右側面板；
// 接著選 2、打備註「12」（備註只收 0x20–0x5A，小寫字母不收），在寫檔之前（`0x1e65b`）讀原版記憶體裡那一筆
// 21 byte，與 `session.SaveName` 組出來的名稱編回 Big5 逐位元組比。
func TestZZSaveScreenMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	names, err := state.LoadSaveNames(c)
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	const slotCaller, memoAt, writeAt = 0x1e581, 0x1e653, 0x1e65b
	slotAsks, memoAsks, subAsks := 0, 0, 0
	var record []byte
	at := -1
	o.OnCall(addr(bootNumInputFn), func(o *oracle.Oracle) {
		switch c := o.Caller().Linear(); {
		case c == slotCaller:
			slotAsks++
		case c != bootMainCmdCaller:
			subAsks++ // 「其他」子選單的數字輸入
		}
	})
	o.OnCall(addr(memoAt), func(*oracle.Oracle) { memoAsks++ })
	o.OnCall(addr(writeAt), func(o *oracle.Oracle) {
		if record == nil {
			record = o.Bytes(addr(0x3e640+0xc6+21*1), 21)
			ds := uint32(o.DSReg()) * 16
			work := uint32(o.Word(addr(ds+0xa72e))) * 16 // es:0x30fc 的段，存檔常式 `0x1e5b5`
			at = int(int16(o.Word(addr(work + 0x30fc))))
		}
	})

	base, s := bootToMainState(t, o, seedMas)
	before := subAsks
	o.Drain()
	o.PressScan("9\r")
	waitBoot(t, o, "其他選單輸入", 200_000_000, func() bool { return subAsks > before })
	waitBootScan(t, o, "其他選單", 5_000_000)
	o.Drain()
	o.PressScan("2\r")
	if err := o.RunUntil(oracle.NewCond("儲存進度(1-6)", func(*oracle.Oracle) bool { return slotAsks > 0 }),
		oracle.Budget(200_000_000)); err != nil {
		dumpScreen(t, o, "save-screen-stuck")
		t.Fatalf("送「9」「2」之後沒問 (1-6)：%v", err)
	}
	waitBootScan(t, o, "儲存進度", 5_000_000)
	orig := append([]uint8(nil), o.IndexedEGASize(scrW, scrH)...)
	dumpScreen(t, o, "save-screen-orig")
	_ = s

	// 畫面：右側面板六筆名稱。
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
	i18n.Current = i18n.ZhHant
	sv := &ui.SaveScreen{}
	for k, n := range names {
		sv.Names[k] = menu.LoadLine(save.Info{Slot: k + 1, Exists: true, Name: n})
	}
	cv := ui.NewCanvasPx(scrW, scrH, loadFace(t))
	ui.DrawArtSession(cv, art, g, nil, ui.View{Save: sv, Prompt: i18n.S("ask.saveOrig")})
	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		savePNG(t, filepath.Join(dir, "remake-save-screen.png"), cv)
	}
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
	bad, first := 0, ""
	for y := 36; y <= 291; y++ {
		for x := 408; x <= 631; x++ {
			if x >= 424 && x < 424+20*8 && y >= 52 && y < 52+6*16 {
				continue // 名稱六列另外比
			}
			if op, mp := org(x, y), idx(x, y); op != mp {
				bad++
				if first == "" {
					first = fmt.Sprintf("(%d,%d) 原版 %d remake %d", x, y, op, mp)
				}
			}
		}
	}
	cellsBad, inked := 0, 0
	for k := 0; k < 6; k++ {
		for col := 0; col < 20; col++ {
			x0, y0 := 424+col*8, 52+k*16
			ink := func(pix func(x, y int) int) bool {
				for y := y0; y < y0+16; y++ {
					for x := x0; x < x0+8; x++ {
						if pix(x, y) != 1 {
							return true
						}
					}
				}
				return false
			}
			om, mm := ink(org), ink(idx)
			if om {
				inked++
			}
			if om != mm {
				cellsBad++
				if cellsBad <= 6 {
					t.Logf("第 %d 列第 %d 格：原版有墨 %v、remake %v", k+1, col+1, om, mm)
				}
			}
		}
	}
	if bad != 0 || cellsBad != 0 {
		t.Errorf("存檔面板：名稱以外 %d 點不同（第一個 %s），名稱 120 格有墨不同 %d", bad, first, cellsBad)
	} else {
		t.Logf("存檔面板：名稱以外逐像素相同，120 格（%d 格有墨）墨相同", inked)
	}

	// 名稱：選 2、打「12」、Enter，寫檔之前讀那一筆。
	o.Drain()
	o.TypeBoth("2\r")
	waitBoot(t, o, "備註輸入", 200_000_000, func() bool { return memoAsks > 0 })
	// 輸入常式進去先清一次鍵盤；讓它跑到等鍵再送字。
	if err := o.Run(20_000_000); err != nil {
		t.Fatal(err)
	}
	o.Drain()
	o.TypeBoth("12\r")
	waitBoot(t, o, "寫檔之前", 200_000_000, func() bool { return record != nil })
	brain, _ := ai.New(ai.ModeBase)
	ss := session.New(g, brain, state.FactionID(players[0]))
	mine := ss.SaveName(2, at, "12")
	enc, err := state.EncodeSaveNames([]string{"", mine, "", "", "", ""})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("當下的郡 %d；原版那一筆 % x（%q）；remake %q", at, record, mustBig5(record), mine)
	if got := enc[21:42]; string(got) != string(record) {
		t.Errorf("名稱位元組不同：\n原版   % x\nremake % x", record, got)
	}
}

func mustBig5(b []byte) string {
	names, err := state.DecodeSaveNames(append(append([]byte{}, b...), make([]byte, 105)...))
	if err != nil {
		return err.Error()
	}
	return names[0]
}
