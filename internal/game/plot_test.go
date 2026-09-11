package game

import "testing"


// 轉場的方向是「取走就沒了」。
//
// **哨兵不是 0**：0 是一個真的方向（由上往下），拿零值當「沒有」的話
// 每一次新局都會多播一次轉場。
func TestTakeWipeClearsAndUsesSentinel(t *testing.T) {
	g := newGame(t)
	if g.PendingWipe != NoWipe {
		t.Fatalf("新局的 PendingWipe ＝ %d，想要 %d（沒有要轉場）",
			g.PendingWipe, NoWipe)
	}
	if got := g.TakeWipe(); got != NoWipe {
		t.Errorf("沒有轉場時取到 %d", got)
	}
	for _, k := range []int{0, 1, 2, 3} {
		g.PendingWipe = k
		if got := g.TakeWipe(); got != k {
			t.Errorf("放 %d 取到 %d", k, got)
		}
		if got := g.TakeWipe(); got != NoWipe {
			t.Errorf("取過一次之後又取到 %d——同一次轉場會播兩遍", got)
		}
	}
}
