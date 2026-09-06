//go:build oracle

package parity

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 讓原版走完一個月，把三張表的每一次變化記下來。
//
// 盤面由對拍這一方寫進去（`tables_oracle_test.go`），所以比出來的差異
// 只可能來自決策，不會來自亂數擺出來的局面。
//
// 每郡每月一道指令（手冊 p.17）；不輸入數字直接按 Enter 就結束該郡的
// 指令（p.5）。所以**一路送 Enter 就會走完自己的回合**，接下來是電腦
// 諸侯的回合，那正是 `docs/mechanics/70-ai` 要看的東西。

// TestZZAdvanceMonth 一路送 Enter，記錄盤面每一次變化。
//
// 環境變數 `SAN1_TURNKEY` 可以換掉送的鍵（預設 `\r`），
// `SAN1_TURNS` 換次數（預設 40）。
func TestZZAdvanceMonth(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, sta, gen := sc.Tables()
	total := len(mas) + len(sta) + len(gen)

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToMain(t, o, mas)
	dumpScreen(t, o, "20-主畫面")

	keys := os.Getenv("SAN1_TURNKEY")
	if keys == "" {
		keys = "\r"
	}
	turns := 40
	if v, err := strconv.Atoi(os.Getenv("SAN1_TURNS")); err == nil && v > 0 {
		turns = v
	}

	snap := o.Save()
	const settle = 40_000_000
	prevScr, mask := blinkMask(o, snap, settle, 3)
	prev := o.Bytes(addr(base), total)

	for i := 1; i <= turns; i++ {
		o.Drain()
		o.PressScan(keys)
		if err := o.Run(settle * 3); err != nil {
			t.Logf("第 %d 次停止：%v", i, err)
			break
		}
		cur := o.Bytes(addr(base), total)
		scr := screenOf(o)
		d := differs8(prev, cur)
		px := pixelDiff(prevScr, scr, mask)
		if d > 0 || px > 0 {
			t.Logf("第 %2d 次送 %q：畫面差 %6d、盤面差 %4d%s",
				i, keys, px, d, where(prev, cur, len(mas), len(sta)))
			dumpScreen(t, o, fmt.Sprintf("21-第%02d次", i))
		} else {
			t.Logf("第 %2d 次送 %q：什麼都沒動", i, keys)
		}
		prev, prevScr = cur, scr
	}
}
