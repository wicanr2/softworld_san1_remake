//go:build oracle

package parity

import (
	"fmt"
	"image"
	"image/png"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/opening"
)

// 片頭是 `DATA0.GRP` 那支 overlay 畫的，位址是**執行期**的（載在 0110:0000，
// `docs/re/02` §1）。主程式 `DATA5.GRP` 之後載在同一個位置，所以這些 hook
// 只在片頭函式進來之後才算數（呼叫端要是 `0ad0:08eb`）。
var (
	openingEntry     = oracle.Addr{Seg: 0x0ad0, Off: 0x011a}
	openingCaller    = oracle.Addr{Seg: 0x0ad0, Off: 0x08eb} // main（0ad0:06ea）叫片頭的返回位址
	openingSeconds   = oracle.Addr{Seg: 0x0ad0, Off: 0x0f9c} // 等秒數跳 n 次
	openingTicks     = oracle.Addr{Seg: 0x0ad0, Off: 0x1022} // 等百分秒跳 n 次
	openingKbhit     = oracle.Addr{Seg: 0x0583, Off: 0x3792}
	openingRibbonAt  = oracle.Addr{Seg: 0x0ad0, Off: 0x0acc} // 頭像橫幅那一步問鍵盤的返回位址
	openingScrollAt  = oracle.Addr{Seg: 0x0ad0, Off: 0x112a} // 三英圖捲入那一步
	openingKeyAt     = oracle.Addr{Seg: 0x0ad0, Off: 0x0104} // 三英圖等鍵的迴圈
	openingShowPage  = oracle.Addr{Seg: 0x0110, Off: 0x1deb} // int 10h AH=05h
	openingSetPal    = oracle.Addr{Seg: 0x0aaa, Off: 0x0232} // (暫存器, 值)
	openingRestorPal = oracle.Addr{Seg: 0x0aaa, Off: 0x0251}
)

// TestZZOpeningMatchesTheOriginal 在原版片頭的每一個等待點與每一步動畫
// （Issue #61），拉 remake `opening.Script` 的下一拍，比：
//
//   - 種類與次數（隨機的船隊等待只比種類，次數餵給 remake 的 Rand）
//   - 顯示中那一頁的 640×408 逐格（原版切顯示頁用 `int 10h AH=05h`，
//     dosgolem 不畫第二頁，所以照 hook 記下來的頁號去讀那一頁的平面）
//   - 16 個屬性暫存器
//
// 詞那一層用原版的 `TZUE`／`TZUE1`，比的是動畫怎麼搬；remake 平常用自己
// 字庫畫的那兩張，字的墨另外比（`TestOpeningFontPoemInkMatchesTheOriginal`）。
func TestZZOpeningMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	c1 := openContainer(t, filepath.Join(root, "DATA1"))
	art, err := opening.LoadArt(c1)
	if err != nil {
		t.Fatal(err)
	}
	if art.PoemInk, art.PoemMask, err = opening.OriginalPoem(c1); err != nil {
		t.Fatal(err)
	}
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	var wantArg int // 目前這個 hook 的次數參數，給 remake 的 Rand
	script := &opening.Script{Art: art, Rand: func(n int) int { return wantArg - 5 }}
	next, stop := iter.Pull(script.Beats())
	defer stop()

	armed, done, keyed := false, false, false
	show := 0
	pal := opening.DefaultPal
	beats := 0
	counts := map[opening.Site]int{}
	var failure string

	check := func(kind opening.Kind, n int) {
		if !armed || done || failure != "" {
			return
		}
		wantArg = n
		b, ok := next()
		if !ok {
			failure = fmt.Sprintf("原版第 %d 拍（step %d）還在走，remake 的片頭已經結束", beats, o.Steps())
			return
		}
		beats++
		counts[b.Site]++
		where := fmt.Sprintf("第 %d 拍［%s］step %d", beats, b.Site, o.Steps())
		if b.Kind != kind {
			failure = fmt.Sprintf("%s：原版是種類 %d，remake 是 %d", where, kind, b.Kind)
			return
		}
		if b.Site != opening.SiteBoat && (kind == opening.HoldSeconds || kind == opening.HoldTicks) && b.N != n {
			failure = fmt.Sprintf("%s：原版等 %d，remake 等 %d", where, n, b.N)
			return
		}
		if b.Pages.Pal != pal {
			failure = fmt.Sprintf("%s：調色盤 原版 %x remake %x", where, pal, b.Pages.Pal)
			return
		}
		got := o.IndexedEGAFrom(show*0x8000, scrW, scrH)
		want := b.Pages.Visible()
		bad, first := 0, -1
		for i := range want.Pix {
			if got[i]&15 != want.Pix[i]&15 {
				if first < 0 {
					first = i
				}
				bad++
			}
		}
		if bad != 0 {
			failure = fmt.Sprintf("%s：顯示第 %d 頁（remake 第 %d 頁），差 %d 格，第一格在 (%d,%d) 原版 %d remake %d",
				where, show, b.Pages.Show, bad, first%scrW, first/scrW, got[first]&15, want.Pix[first]&15)
			saveIndexed(t, "opening-orig", got)
			saveIndexed(t, "opening-remake", want.Pix)
		}
	}

	o.OnCall(openingEntry, func(o *oracle.Oracle) {
		if o.Caller() == openingCaller {
			armed = true
		}
	})
	o.OnCall(openingShowPage, func(o *oracle.Oracle) {
		if armed {
			show = int(o.Arg(0) & 1)
		}
	})
	o.OnCall(openingSetPal, func(o *oracle.Oracle) {
		if armed && o.Arg(0) < 16 {
			pal[o.Arg(0)] = byte(o.Arg(1) & 0x3f)
		}
	})
	o.OnCall(openingRestorPal, func(o *oracle.Oracle) {
		if armed {
			pal = opening.DefaultPal
		}
	})
	o.OnCall(openingSeconds, func(o *oracle.Oracle) { check(opening.HoldSeconds, int(o.Arg(0))) })
	o.OnCall(openingTicks, func(o *oracle.Oracle) { check(opening.HoldTicks, int(o.Arg(0))) })
	o.OnCall(openingKbhit, func(o *oracle.Oracle) {
		switch o.Caller() {
		case openingRibbonAt, openingScrollAt:
			check(opening.Step, 0)
		case openingKeyAt:
			if !keyed && armed && failure == "" {
				keyed = true
				check(opening.WaitKey, 0)
				if err := o.SendKeys("Return"); err != nil {
					failure = err.Error()
				}
			}
		}
	})
	o.OnCall(openingCaller, func(o *oracle.Oracle) {
		if armed && !done {
			check(opening.End, 0)
			done = true
		}
	})

	waitBootKey(t, o, "音樂裝置輸入", "1")
	waitBootKey(t, o, "繪圖裝置輸入", "2")
	waitBootKey(t, o, "磁碟裝置輸入", "2")
	cond := oracle.NewCond("片頭結束或對不上", func(*oracle.Oracle) bool { return done || failure != "" })
	if err := o.RunUntil(cond, oracle.Budget(600_000_000)); err != nil {
		t.Fatalf("片頭沒有跑完（armed=%v，已比 %d 拍）：%v", armed, beats, err)
	}
	if failure != "" {
		t.Fatal(failure)
	}
	if b, ok := next(); ok {
		t.Fatalf("原版片頭結束了，remake 還有下一拍［%s］", b.Site)
	}
	for s := opening.SiteTrademark; s <= opening.SiteLoading; s++ {
		t.Logf("%s：%d 拍", s, counts[s])
	}
	t.Logf("原版片頭 %d 拍，每一拍顯示中那一頁 %d×%d 逐格相同、調色盤相同", beats, scrW, scrH)
}

// saveIndexed 存一張索引色畫面（有設 SAN1_SHOTS 才存）。
func saveIndexed(t *testing.T, name string, pix []uint8) {
	dir := os.Getenv("SAN1_SHOTS")
	if dir == "" {
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, scrW, scrH))
	for y := 0; y < scrH; y++ {
		for x := 0; x < scrW; x++ {
			img.Set(x, y, assets.EGAPalette[pix[y*scrW+x]&15])
		}
	}
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Log(err)
		return
	}
	defer f.Close()
	_ = png.Encode(f, img)
}
