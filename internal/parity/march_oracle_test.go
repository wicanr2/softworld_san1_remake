//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// TestZZMarchAnimationMatchesTheOriginal 對拍大地圖上的戰役動畫（`0x1ecfc`，
// Issue #62）。盤面與 TestAIvsAIBattleMatchesOriginal 同一套（AI 等級輪流、
// 留守目標改 1、金米拉滿），自然月份裡的電腦對電腦戰役逐場比：
//
//   - 入口讀兩郡的州郡座標，與劇本表的 MapX／MapY 相同
//   - 每一格搬回第一頁之後（原地踏步 `0x1f1cc`、日迴圈 `0x1f486`）整張
//     640×408 與 `ui.March` 的 Screen 逐格相同
//   - 收尾（`0x1f532`）存底搬回之後整張相同
//   - 整場 `speak(0, 速度)` 的速度序列與 `March.Speeds` 逐格接起來相同
//
// 第二頁在入口照原版的內容起頭（remake 播放時是全黑）；另外用全黑起頭再跑
// 一份，報告差幾格——第二頁的舊內容只在旗隊畫出那一塊之外才會碰到。
func TestZZMarchAnimationMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()
	art, err := ui.MarchArt(openContainer(t, filepath.Join(root, "DATA3")))
	if err != nil {
		t.Fatal(err)
	}

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToGame(t, o, seedMas)
	staBase := base + uint32(state.MasterTableSize)
	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), uint16(3+alive%3))
		alive++
	}
	var sg sortieGlobals
	sgOK := false
	o.OnCall(addr(0xb2b4), func(oo *oracle.Oracle) {
		if !sgOK {
			sg, sgOK = resolveSortieGlobals(oo), true
		}
		oo.SetWord(addr(sg.want), 1)
	})

	const wantBattles = 3
	var (
		m, blank     *ui.March
		battles      int
		frames       int
		days         int
		speeds       []int // 這一場原版叫過的速度
		failure      string
		blankDiffMax int
	)
	fail := func(f string, a ...any) {
		if failure == "" {
			failure = fmt.Sprintf("第 %d 場：", battles) + fmt.Sprintf(f, a...)
		}
	}
	page0 := func(oo *oracle.Oracle) []uint8 { return oo.IndexedEGASize(scrW, scrH) }
	diff := func(got []uint8, want []byte) (int, int) {
		bad, first := 0, -1
		for i := range want {
			if got[i]&15 != want[i]&15 {
				if first < 0 {
					first = i
				}
				bad++
			}
		}
		return bad, first
	}
	// frameCheck 在一格搬回第一頁之後：remake 畫同一格、比整張、比這一格的聲音。
	frameCheck := func(oo *oracle.Oracle, where string) {
		if m == nil || failure != "" {
			return
		}
		k := m.Frame()
		if !m.Step() {
			fail("%s：remake 已經收尾（第 %d 格）", where, k)
			return
		}
		blank.Step()
		got := page0(oo)
		if bad, first := diff(got, m.Screen.Pix); bad != 0 {
			fail("%s 第 %d 格差 %d 格，第一格 (%d,%d) 原版 %d remake %d", where, k, bad,
				first%scrW, first/scrW, got[first]&15, m.Screen.Pix[first]&15)
			saveIndexed(t, fmt.Sprintf("march-%d-orig", battles), got)
			saveIndexed(t, fmt.Sprintf("march-%d-remake", battles), m.Screen.Pix)
			return
		}
		if bad, _ := diff(got, blank.Screen.Pix); bad > blankDiffMax {
			blankDiffMax = bad
		}
		frames++
	}

	o.OnCall(addr(0x1ecfc), func(oo *oracle.Oracle) {
		if battles >= wantBattles || failure != "" {
			return
		}
		battles++
		at, to := int(oo.Arg(0)), int(oo.Arg(1))
		rd := func(p int, off uint32) int { return int(int16(oo.Word(addr(staBase + uint32(p*176) + off)))) }
		ax, ay, dx, dy := rd(at, 6), rd(at, 8), rd(to, 6), rd(to, 8)
		pa, errA := sc0.Prefecture(at)
		pd, errD := sc0.Prefecture(to)
		if errA != nil || errD != nil || int(pa.MapX) != ax || int(pa.MapY) != ay || int(pd.MapX) != dx || int(pd.MapY) != dy {
			fail("郡 %d→%d 的座標原版 (%d,%d)→(%d,%d) 與劇本表不同", at, to, ax, ay, dx, dy)
			return
		}
		l := ui.NewMarchLayout(ax, ay, dx, dy)
		scr := page0(oo)
		m = ui.NewMarch(l, art, 1<<20, indexedImage(scr), indexedImage(oo.IndexedEGAFrom(0x8000, scrW, scrH)))
		blank = ui.NewMarch(l, art, 1<<20, indexedImage(scr), nil)
		days, speeds = 0, nil
		t.Logf("第 %d 場：郡 %d (%d,%d) → 郡 %d (%d,%d)，那一塊 (%d,%d) %d×%d，圖 %d/%d/%d/%d",
			battles, at, ax, ay, to, dx, dy, l.X, l.Y, l.W, l.H, l.Att, l.AttMask, l.Def, l.DefMask)
	})
	o.OnCall(addr(0x5b80), func(oo *oracle.Oracle) {
		if m == nil {
			return
		}
		if c := oo.Caller(); c.Linear() >= 0x1ecfc && c.Linear() < 0x1f538 {
			speeds = append(speeds, int(oo.Arg(1)))
		}
	})
	o.OnCall(addr(0x1f538), func(*oracle.Oracle) {
		if m != nil {
			days++
		}
	})
	o.OnCall(addr(0x1f1cc), func(oo *oracle.Oracle) { frameCheck(oo, "原地踏步") })
	o.OnCall(addr(0x1f486), func(oo *oracle.Oracle) { frameCheck(oo, "日迴圈") })
	o.OnCall(addr(0x1f532), func(oo *oracle.Oracle) {
		if m == nil || failure != "" {
			return
		}
		m.Days, blank.Days = days, days
		if m.Frame() != m.Frames() {
			fail("原版 %d 天，remake 畫到第 %d 格（應該 %d 格）", days, m.Frame(), m.Frames())
			return
		}
		var want []int
		for k := 0; k <= m.Frames(); k++ {
			before, after := m.Speeds(k)
			want = append(append(want, before...), after...)
		}
		if m.Step() {
			fail("收尾時 remake 還在畫")
			return
		}
		blank.Step()
		got := page0(oo)
		if bad, first := diff(got, m.Screen.Pix); bad != 0 {
			fail("收尾之後差 %d 格，第一格 (%d,%d)", bad, first%scrW, first/scrW)
			return
		}
		if !slices.Equal(speeds, want) {
			fail("整場 speak 的速度：原版 %v remake %v", speeds, want)
		}
		t.Logf("第 %d 場：%d 天、%d 格，每格整張逐格相同，收尾還原相同，%d 聲速度序列相同", battles, days, frames, len(speeds))
		m, frames = nil, 0
	})

	const settle = 120_000_000
	for month := 1; month <= 3 && battles < wantBattles && failure == ""; month++ {
		for p := 1; p <= state.PrefectureCount; p++ {
			o.SetWord(addr(staBase+uint32(p*176+18)), 20000)
			o.SetWord(addr(staBase+uint32(p*176+20)), 30000)
		}
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", month, k, err)
			}
			if failure != "" {
				break
			}
		}
	}
	if failure != "" {
		t.Fatal(failure)
	}
	if battles == 0 {
		t.Fatal("三個月內沒有任何一場電腦對電腦戰役")
	}
	if m != nil {
		t.Fatalf("第 %d 場沒有跑到收尾", battles)
	}
	t.Logf("比了 %d 場；第二頁用全黑起頭時整張最多差 %d 格", battles, blankDiffMax)
}

// indexedImage 把 dosgolem 的索引畫面複製成 640×408 的圖。
func indexedImage(pix []uint8) *assets.Image {
	im := &assets.Image{W: scrW, H: scrH, Pix: make([]byte, scrW*scrH)}
	for i := range im.Pix {
		im.Pix[i] = pix[i] & 15
	}
	return im
}
