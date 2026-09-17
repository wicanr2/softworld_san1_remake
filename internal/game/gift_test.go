package game

import (
	"errors"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestGiftRoundFollowsTheOriginal 釘住玩家賞賜物品的兩層迴圈（`0x1cfd6`）：
// 一道命令可以連送好幾件，同一位只能收一次，送出過東西收掉時才算下過令；
// 忠誠看截到 90 之前的能力。
func TestGiftRoundFollowsTheOriginal(t *testing.T) {
	g := newGame(t)
	// 找一位君主身邊至少有兩位部將的勢力（劇本一開局就有）。
	var by state.FactionID
	var lord, a, b *General
	for id := state.FactionID(0); id < 16 && b == nil; id++ {
		l := g.Lord(id)
		if l == nil {
			continue
		}
		a, b = nil, nil
		for _, x := range g.GiftCandidates(l.Location) {
			if x.Index == l.Index {
				continue
			}
			if a == nil {
				a = x
			} else if b == nil {
				b = x
			}
		}
		by, lord = id, l
	}
	if b == nil {
		t.Fatal("劇本一沒有一位君主身邊有兩位部將——盤面換了，這支測試要跟著換")
	}
	at := lord.Location
	f := g.Faction(by)
	f.Treasury[TreasureBook] = 2
	f.Treasury[TreasureBlade] = 1

	// 一件都沒送就收掉：不算下過令（`0x17791` 回主選單再問）。
	r, err := g.OpenGift(at, by)
	if err != nil {
		t.Fatal(err)
	}
	if g.CloseGift(r) || g.Prefecture(at).Commanded {
		t.Fatal("一件都沒送就收掉，郡被記成下過令")
	}
	r, err = g.OpenGift(at, by)
	if err != nil {
		t.Fatal(err)
	}
	// 謀略 91 收兵書：先加成 93、擲忠誠看 93÷2＝46，之後截回 91
	// （看截斷後的算法會是 45）。
	a.Intel, a.Loyalty = 91, 0
	if err := g.Gift(r, a.Index, TreasureBook); err != nil {
		t.Fatal(err)
	}
	if a.Intel != 91 {
		t.Errorf("謀略 91 收兵書變成 %d，該留在 91", a.Intel)
	}
	if a.Loyalty < 46 || a.Loyalty >= 46+TreasureLoyaltySpread {
		t.Errorf("忠誠加了 %d，該在 46–%d（看截斷前的 93）", a.Loyalty, 46+TreasureLoyaltySpread-1)
	}
	if g.Prefecture(at).Commanded {
		t.Error("同一道命令裡送完第一件就記成下過令")
	}
	// 同一位再送：「%s已賞賜過了」，寶庫不動。
	if err := g.Gift(r, a.Index, TreasureBook); !errors.Is(err, ErrAlreadyGifted) {
		t.Errorf("同一道命令再賞同一位回 %v", err)
	}
	if f.Treasury[TreasureBook] != 1 {
		t.Errorf("兵書剩 %d，該是 1", f.Treasury[TreasureBook])
	}
	// 另一位照送；戰力 95 收寶刀不會被拉低，忠誠大於 100 寫 100。
	b.War, b.Loyalty = 95, 90
	if err := g.Gift(r, b.Index, TreasureBlade); err != nil {
		t.Fatal(err)
	}
	if b.War != 95 || b.Loyalty != 100 {
		t.Errorf("戰力 95、忠誠 90 收寶刀變成戰力 %d、忠誠 %d", b.War, b.Loyalty)
	}
	if !g.CloseGift(r) || !g.Prefecture(at).Commanded {
		t.Fatal("送出過東西，收掉時該記成下過令")
	}
	// 已賞名單每道命令重設，但這個月已經下過令了。
	if _, err := g.OpenGift(at, by); !errors.Is(err, ErrAlreadyMoved) {
		t.Errorf("下過令的郡再開賞賜物品回 %v", err)
	}
}
