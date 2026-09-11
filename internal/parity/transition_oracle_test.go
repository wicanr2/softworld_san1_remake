//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 計謀得手之後那一段四選一的拉幕（`0x32e40`，`docs/spec/010`）。
//
// 規格卡在兩件事：搬運常式 `es:[0x3efc]` 是什麼、四個分支各往哪個方向。
// 兩件都不必等遊戲自己走到那裡——**直接呼叫**（`oracle.Call`）走的是
// 同一段機器碼與同一份畫面記憶體，而 `RND(4)` 用 stub 指定分支，
// 四條路一次跑完。
//
// 參數是從**唯一的呼叫端**讀出來的（`0x1bbb9`：`push 0x50; push 0x1b0`），
// 所以 `Arg(0) = 0x1b0 = 432`、`Arg(1) = 0x50 = 80`。配上迴圈界限
// （`0x5f` 與 `0xaf`）就是 **x 432..607（寬 176）、y 80..175（高 96）**。
const (
	transitionFn = 0x32e40 // 四選一的拉幕
	rndFn        = 0x10b0c // RND(n)
	blitTableSeg = 0xaa8e  // DS 裡放搬運常式那個模組的段
	blitTableOff = 0x3efc  // 該模組跳表裡的位移
)

func TestZZTransitionFrames(t *testing.T) {
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

	// 區域：從唯一的呼叫端與迴圈界限推出來的。
	const (
		wx0, wx1 = 0x1b0, 0x1b0 + 0xaf // 432..607
		wy0, wy1 = 0x50, 0x50 + 0x5f   // 80..175
	)
	branch := uint32(0)
	step := 0
	// hook 只**存快照**，比對留到跑完：要拿「終態」當參考，而終態
	// 在跑完之前不存在。邊跑邊比就只能跟起點比，那答得出「變了沒」，
	// 答不出「變成什麼」。
	var frames [][]uint8
	o.OnCall(addr(speechSpeakFn), func(o *oracle.Oracle) {
		step++
		if p := o.IndexedEGASize(scrW, scrH); len(p) >= scrW*scrH {
			frames = append(frames, append([]uint8(nil), p...))
		}
	})

	bootToMain(t, o, seedMas)
	base := o.IndexedEGASize(scrW, scrH)
	base = append([]uint8(nil), base...)
	dumpScreen(t, o, "wipe-base")

	o.Stub(addr(rndFn), func(*oracle.Oracle) uint32 { return branch })
	saved := o.Save()
	for branch = 0; branch < 4; branch++ {
		o.Restore(saved)
		step, frames = 0, nil
		if _, err := o.CallBudget(200_000_000, addr(transitionFn), wx0, wy0); err != nil {
			t.Fatalf("分支 %d 呼叫失敗：%v", branch, err)
		}
		dumpScreen(t, o, fmt.Sprintf("wipe-%d-end", branch))
		if len(frames) == 0 {
			t.Fatalf("分支 %d 一張畫格都沒抓到", branch)
		}
		end := frames[len(frames)-1]

		// 三個量一起看，才分得出「逐步揭露終態」與「逐步抹成一片」：
		//   ① 這一步與**前一步**差在哪（bounding box ＋ 像素數）
		//   ② 這一步新露出來的那一塊，顏色直方圖前三名
		//   ③ 這一步與**最後一步**在那一塊差幾個像素
		// 只比「像不像起點／終點」會得到「兩邊都不像」，那句話排除得掉
		// 一個模型，卻指不出下一個。
		axis, lo, hi := "列", wy0, wy1
		if branch >= 2 {
			axis, lo, hi = "行", wx0, wx1
		}
		t.Logf("── 分支 %d（%d 步，逐%s，起點差 %d 個像素）",
			branch, step, axis, pixDiff(base, end, wx0, wy0, wx1, wy1))
		_ = lo
		_ = hi
		prev := base
		for n, f := range frames {
			x0, y0, x1, y1, nd := diffBox(prev, f)
			hist := topColors(f, wx0, wy0, wx1, wy1)
			vs := pixDiff(f, end, wx0, wy0, wx1, wy1)
			if n%4 == 0 || n == len(frames)-1 {
				t.Logf("  第 %2d 步　與前一步差 %5d 點（x %d–%d、y %d–%d）　"+
					"顏色 %v　與最後一步差 %5d 點", n+1, nd, x0, x1, y0, y1, hist, vs)
			}
			prev = f
		}
	}
}

// diffBox 回兩張畫面差在哪：bounding box 與像素數。
func diffBox(a, b []uint8) (x0, y0, x1, y1, n int) {
	x0, y0, x1, y1 = -1, -1, -1, -1
	for y := 0; y < scrH; y++ {
		for x := 0; x < scrW; x++ {
			if a[y*scrW+x] == b[y*scrW+x] {
				continue
			}
			n++
			if x0 < 0 || x < x0 {
				x0 = x
			}
			if x > x1 {
				x1 = x
			}
			if y0 < 0 || y < y0 {
				y0 = y
			}
			if y > y1 {
				y1 = y
			}
		}
	}
	return
}

// pixDiff 數兩張畫面在一塊矩形裡差幾個像素。
func pixDiff(a, b []uint8, x0, y0, x1, y1 int) int {
	n := 0
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if a[y*scrW+x] != b[y*scrW+x] {
				n++
			}
		}
	}
	return n
}

// topColors 回一塊矩形裡最常見的三個色號與佔比。
func topColors(p []uint8, x0, y0, x1, y1 int) []string {
	cnt := map[uint8]int{}
	tot := 0
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			cnt[p[y*scrW+x]]++
			tot++
		}
	}
	type kv struct {
		c uint8
		n int
	}
	var all []kv
	for c, n := range cnt {
		all = append(all, kv{c, n})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].n > all[j].n })
	var out []string
	for i := 0; i < len(all) && i < 3; i++ {
		out = append(out, fmt.Sprintf("%d:%.0f%%", all[i].c,
			100*float64(all[i].n)/float64(tot)))
	}
	return out
}
