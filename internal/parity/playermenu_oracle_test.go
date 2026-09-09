//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/text/encoding/traditionalchinese"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 玩家選單每一項在問什麼——問原版自己，不用眼睛看畫面。
//
// 既有的二十幾支對拍**刻意繞開玩家選單**（`affairs_oracle_test.go` 開頭：
// 「玩家那條要一路按進子選單再選將領，按鍵序列每試一次就是一分鐘」），
// 改掛在電腦與玩家共用的寫回常式上。那證得到「公式一樣」，證不到
// **玩家按下去會發生什麼**——選單走的參數（誰去做、幾個單位、扣誰的錢）
// 是另一段碼。要逐道命令對拍，得先知道每一道在問什麼。
//
// 做法是**看原版讀了哪些字串常數**。截圖也行，但一個項目一張圖，
// 二十幾張要一張一張看；字串可以 grep。
//
// 開機一次、存快照，之後每一項還原重試——開機兩分半，每一項十秒。
func TestZZPlayerMenuPrompts(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	bootToGame(t, o, seedMas)

	enc := traditionalchinese.Big5.NewEncoder()
	dec := traditionalchinese.Big5.NewDecoder()
	big5 := func(s string) []byte {
		b, err := enc.Bytes([]byte(s))
		if err != nil {
			t.Fatalf("%q 編不成 Big5：%v", s, err)
		}
		return b
	}

	// 字串區的範圍用**原版自己的字串**定，不寫死：拿幾條已知的子選單
	// 去搜，取最小與最大。寫死的位移會隨版本漂，搜出來的不會。
	var lo, hi uint32
	for _, probe := range []string{
		"1.訓練兵士", "1.土地開發", "1.買入米糧", "1.尋訪人才",
		"1.調動軍隊", "1.指定軍師",
	} {
		hits := o.Search(big5(probe))
		if len(hits) == 0 {
			t.Fatalf("記憶體裡找不到 %q——盤面還沒進到遊戲中", probe)
		}
		for _, h := range hits {
			if lo == 0 || h < lo {
				lo = h
			}
			if h > hi {
				hi = h
			}
		}
	}
	// 兩邊各留一點，並確保不超過 64 KB（`MemRead.Off` 是 uint16）。
	lo -= 0x400
	hi += 0x1000
	if hi-lo > 0xF000 {
		hi = lo + 0xF000
	}
	t.Logf("字串區 %#x–%#x（%d bytes）", lo, hi, hi-lo)

	log := o.WatchReadsAt(lo, hi)
	defer o.StopWatchingReads()

	// cstrAt 從線性位址讀出以 0 結尾的字串，解成繁中。
	cstrAt := func(at uint32) string {
		b := o.Bytes(addr(at), 64)
		if i := indexZero(b); i >= 0 {
			b = b[:i]
		}
		s, err := dec.Bytes(b)
		if err != nil {
			return fmt.Sprintf("%q", b)
		}
		return strings.ReplaceAll(string(s), "\n", "\\n")
	}

	// touched 把這一段時間讀過的位移收斂成「幾條字串」：連號（間隔 ≤ 3）
	// 的算同一條，取最小的位移當起點。
	touched := func() []string {
		if len(*log) == 0 {
			return nil
		}
		seen := map[uint16]bool{}
		for _, r := range *log {
			seen[r.Off] = true
		}
		offs := make([]int, 0, len(seen))
		for off := range seen {
			offs = append(offs, int(off))
		}
		sort.Ints(offs)
		var out []string
		start := -1
		last := -1
		flush := func() {
			if start < 0 {
				return
			}
			// 往前退到字串的開頭：前一個位元組是 0 才算頭。
			at := lo + uint32(start)
			for i := 0; i < 48 && at > lo; i++ {
				if o.Byte(addr(at-1)) == 0 {
					break
				}
				at--
			}
			if s := cstrAt(at); s != "" {
				out = append(out, fmt.Sprintf("%#x %s", at, s))
			}
		}
		for _, off := range offs {
			if last >= 0 && off-last <= 3 {
				last = off
				continue
			}
			flush()
			start, last = off, off
		}
		flush()
		return out
	}

	const settle = 40_000_000
	snap := o.Save()

	type item struct{ cat, no int }
	items := []item{}
	for _, ci := range []struct{ cat, n int }{
		{2, 3}, {3, 4}, {4, 4}, {5, 3}, {6, 4}, {7, 5},
	} {
		for k := 1; k <= ci.n; k++ {
			items = append(items, item{ci.cat, k})
		}
	}

	for _, it := range items {
		o.Restore(snap)
		o.Drain()
		o.PressScan(fmt.Sprintf("%d\r", it.cat))
		if err := o.Run(settle); err != nil {
			t.Fatalf("送類別 %d 時停止：%v", it.cat, err)
		}
		*log = (*log)[:0]
		o.Drain()
		o.PressScan(fmt.Sprintf("%d\r", it.no))
		if err := o.Run(settle * 2); err != nil {
			t.Fatalf("送 %d-%d 時停止：%v", it.cat, it.no, err)
		}
		lines := touched()
		if len(lines) > 12 {
			lines = lines[:12]
		}
		t.Logf("%d-%d 讀到的字串：\n\t%s", it.cat, it.no,
			strings.Join(lines, "\n\t"))
	}
}

func indexZero(b []byte) int {
	for i, x := range b {
		if x == 0 {
			return i
		}
	}
	return -1
}
