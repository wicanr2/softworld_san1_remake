//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 電腦諸侯的判斷式：一次只變一個數，看它改做什麼。
//
// 走到遊戲裡要七分鐘，但**從同一個快照展開一個變體只要二十幾秒**——
// 把盤面寫進去、讓原版走一個月、讀回來，然後倒帶。所以「換一個金額
// 再問一次」是便宜的，可以問幾十次。
//
// 盤面由對拍這一方寫進去，不靠原版的 `RND()` 湊：亂數的狀態在快照裡，
// 每個變體都一樣，所以變體之間的差異只可能來自我們改的那個數。
//
// 這一條是**探索用的**，它不斷言門檻在哪裡，只把「這個數是 N 的時候
// 它做了什麼」列出來。門檻要等看得懂那些反應再寫。

// probeTarget 是要觀察的勢力與它的郡。
//
// 挑只有一個郡的勢力：郡數多的話，同一個月裡好幾個郡各下一道令，
// 讀回來的變化分不出是哪一個郡的決定。
type probeTarget struct {
	faction int
	pref    int
}

// TestZZAIProbe 一次改一個欄位，看電腦諸侯的反應。
func TestZZAIProbe(t *testing.T) {
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
	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	total := nMas + nSta + state.GeneralTableSize
	board := o.Bytes(addr(base), total)
	dumpTables(t, board, "probe-00-底盤")

	// 找一個只有一個郡的勢力。
	count := map[int]int{}
	owner := map[int]int{}
	rec := state.PrefectureRecordSize
	for i := 1; i*rec < nSta; i++ {
		f := int(board[nMas+i*rec+30])
		if f == 0xFF {
			continue
		}
		count[f]++
		owner[f] = i
	}
	var target probeTarget
	for f, n := range count {
		if n == 1 {
			target = probeTarget{faction: f, pref: owner[f]}
			break
		}
	}
	if target.pref == 0 {
		t.Skip("這個盤面沒有只有一個郡的勢力，換一個存檔再問")
	}
	t.Logf("觀察勢力 %d，它唯一的郡是 %d", target.faction, target.pref)

	const (
		offSoldiers = 16
		offGold     = 18
		offRice     = 20
		offLand     = 27
		offFlood    = 28
	)
	at := nMas + target.pref*rec
	put16 := func(b []byte, off, v int) {
		b[at+off] = byte(v)
		b[at+off+1] = byte(v >> 8)
	}

	snap := o.Save()
	const settle = 40_000_000

	// 一次跑一個變體：蓋盤面 → 走一個月 → 讀回來 → 倒帶。
	run := func(name string, edit func([]byte)) {
		o.Restore(snap)
		b := append([]byte(nil), board...)
		edit(b)
		o.SetBytes(addr(base), b)
		got := o.Bytes(addr(base), total)
		for i := range b {
			if got[i] != b[i] {
				t.Fatalf("%s：盤面寫進去讀回來對不上（第 %d 個位元組）", name, i)
			}
		}
		for _, keys := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(keys)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("%s：原版停止 %v", name, err)
			}
		}
		after := o.Bytes(addr(base), total)
		t.Logf("%-24s → 郡 %d%s", name, target.pref,
			oneP(b, after, at, rec))
		dumpTables(t, after, "probe-"+name)
	}

	for _, gold := range []int{0, 50, 200, 1000, 5000, 20000} {
		g := gold
		run(fmt.Sprintf("金=%d", g), func(b []byte) { put16(b, offGold, g) })
	}
	for _, rice := range []int{0, 100, 1000, 20000} {
		r := rice
		run(fmt.Sprintf("米=%d", r), func(b []byte) { put16(b, offRice, r) })
	}
	for _, flood := range []int{0, 50, 100} {
		f := flood
		run(fmt.Sprintf("洪水率=%d", f), func(b []byte) { b[at+offFlood] = byte(f) })
	}
	for _, land := range []int{10, 50, 100} {
		l := land
		run(fmt.Sprintf("地力=%d", l), func(b []byte) { b[at+offLand] = byte(l) })
	}
}

// oneP 說一個郡在這一個月裡哪些欄位變了、變多少。
func oneP(a, b []byte, at, rec int) string {
	names := map[int]string{
		14: "人口", 16: "兵士", 18: "金", 20: "米",
		22: "在職將", 23: "在野將", 26: "民忠", 27: "地力",
		28: "水利", 29: "物價", 30: "所屬",
	}
	out := ""
	for _, off := range []int{14, 16, 18, 20} {
		x := int(a[at+off]) | int(a[at+off+1])<<8
		y := int(b[at+off]) | int(b[at+off+1])<<8
		if x != y {
			out += fmt.Sprintf(" %s %d→%d", names[off], x, y)
		}
	}
	for _, off := range []int{22, 23, 26, 27, 28, 29, 30} {
		if a[at+off] != b[at+off] {
			out += fmt.Sprintf(" %s %d→%d", names[off], a[at+off], b[at+off])
		}
	}
	if out == "" {
		return "：沒動"
	}
	return "：" + out
}
