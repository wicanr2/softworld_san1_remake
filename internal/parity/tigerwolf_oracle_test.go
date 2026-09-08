//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 「驅虎吞狼」走玩家的完整選單跑一次（`docs/mechanics/30-diplomacy` 的缺口）。
//
// 戰略層剩下三支沒實跑的計謀結局都是發動一場戰役，但**這一支我方完全
// 不參戰**——`0x2ce5b` 呼叫的是 `battle(出使郡, −1, 被驅使去打的郡, −1)`，
// 兩個格子都是別人的郡。照 `docs/re/05` §7.1，四個郡都不是玩家操縱時
// 戰役走自動結算（`0x1E908`），不進戰術層，所以按鍵不必再接一整串整編。
//
// 兩個目標郡**從原版自己算出來的候選表挑**（`docs/re/07` §2）：段
// `[0xa9e4]` 的 `0x2102` 起 43 個 word，`0` ＝ 可選、`0xFFFF` ＝ 不可選，
// 郡編號 1–42 直接當索引。這樣不必猜盤面，順帶把那張表驗過一次。
func TestTigerWolfRunsLive(t *testing.T) {
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
	at, _ := stageABattle(t, o, base)

	staBase := base + uint32(state.MasterTableSize)
	genBase := staBase + uint32(state.PrefectureTableSize)
	const staRec = state.PrefectureRecordSize

	// 我方的勢力：從坐鎮郡的所屬反查。
	mine := int(o.Byte(addr(staBase + uint32(at)*staRec + 30)))
	lord := o.Word(addr(base + uint32(mine)*72 + 2))
	o.SetWord(addr(base+uint32(mine)*72+6), lord) // 軍師 ← 君主（`0x2c225`）
	o.SetByte(addr(genBase+uint32(lord)*30+9), 100)
	// 成敗是雙方謀略的硬碰硬（`PlotScore`）。目標是執行期才挑的，
	// 所以**其餘勢力的君主與軍師一律壓到 30**，把這個變數移出去。
	lowered := 0
	for i := 0; i < 16; i++ {
		if i == mine {
			continue
		}
		for _, off := range []uint32{2, 6} {
			who := o.Word(addr(base + uint32(i)*72 + off))
			if who < 350 {
				o.SetByte(addr(genBase+uint32(who)*30+9), 30)
				lowered++
			}
		}
	}
	t.Logf("勢力 %d 的軍師欄指向君主槽 %d（謀略 100）；其餘 %d 位壓到 30",
		mine, lord, lowered)

	// 候選表所在的段。
	candSeg := o.Word(addr(base)) // 佔位，稍後從 DGROUP 取
	_ = candSeg
	var dgroup uint16
	o.OnCall(addr(0x2c2ee), func(o *oracle.Oracle) {
		if dgroup == 0 {
			dgroup = o.DSReg()
		}
	})

	inPlot := 0
	o.OnCall(addr(0x2c9d8), func(*oracle.Oracle) { inPlot++ })
	type judgeCall struct{ from, at, charm int }
	var judges []judgeCall
	o.OnCall(addr(0x2dd66), func(o *oracle.Oracle) {
		if len(judges) < 8 {
			judges = append(judges, judgeCall{
				int(o.Arg(0)), int(o.Arg(1)), int(o.Arg(2))})
		}
	})
	type battleCall struct{ a, aHelp, d, dHelp int }
	var battles []battleCall
	o.OnCall(addr(0x20200), func(o *oracle.Oracle) {
		if len(battles) < 4 {
			battles = append(battles, battleCall{
				int(o.Arg(0)), int(o.Arg(1)), int(o.Arg(2)), int(o.Arg(3))})
		}
	})

	send := func(tag, keys string) {
		o.Drain()
		o.PressScan(strings.ReplaceAll(keys, "⏎", "\r"))
		if err := o.Run(120_000_000); err != nil {
			t.Fatalf("%s 送 %q 停止：%v", tag, keys, err)
		}
	}
	// 候選表：43 個 word，0 ＝ 可選。
	candidates := func() []int {
		if dgroup == 0 {
			t.Fatal("還沒進到分派器 0x2c2ee，取不到 DGROUP")
		}
		seg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9e4})
		var out []int
		for id := 1; id <= 42; id++ {
			v := o.Word(oracle.Addr{Seg: seg, Off: uint16(0x2102 + id*2)})
			if v == 0 {
				out = append(out, id)
			}
		}
		return out
	}
	spell := func(n int) string { return fmt.Sprintf("%d⏎", n) }

	send("謀略選單", "8⏎")
	send("驅虎吞狼", "1⏎")
	if inPlot == 0 {
		t.Fatalf("沒有進到 0x2c9d8——主選單第 8 項或子選單第 1 項不對"+
			"（分派器進去 %d 次）", inPlot)
	}
	c1 := candidates()
	t.Logf("出使的候選郡（%d 個）：%v", len(c1), c1)
	if len(c1) == 0 {
		t.Fatal("出使的候選郡是空的——這個盤面挑不出「有主、非我方」的郡")
	}

	// **第二張表不是每個出使郡都湊得出來**：它要「出使郡的鄰郡、有主、
	// 非我方、非出使郡那一方」（`docs/re/07` §2）。先自己照這條算一輪
	// 排出優先序，再用快照逐一試——猜錯了原版會給空表，那時換下一個。
	owner := func(id int) int {
		return int(o.Byte(addr(staBase + uint32(id)*staRec + 30)))
	}
	promising, rest := []int{}, []int{}
	for _, id := range c1 {
		ok := false
		for k := 45; k <= 54; k++ {
			n := int(o.Byte(addr(staBase + uint32(id)*staRec + uint32(k))))
			if n == 0xFF || n == 0 || n > 42 {
				continue
			}
			ow := owner(n)
			if ow != 0xFF && ow != mine && ow != owner(id) {
				ok = true
				break
			}
		}
		if ok {
			promising = append(promising, id)
		} else {
			rest = append(rest, id)
		}
	}
	t.Logf("自己算的：%d 個郡的鄰郡湊得出第二個目標，%d 個湊不出",
		len(promising), len(rest))

	atPrompt := o.Save()
	pick1, c2 := 0, []int(nil)
	for _, id := range append(promising, rest...) {
		o.Restore(atPrompt)
		send("出使那一郡", spell(id))
		if got := candidates(); len(got) > 0 {
			pick1, c2 = id, got
			break
		}
	}
	if pick1 == 0 {
		t.Fatal("每一個出使候選的第二張表都是空的——條件的讀法不對")
	}
	t.Logf("出使郡挑 %d；驅使攻打的候選郡（%d 個）：%v", pick1, len(c2), c2)
	c1 = []int{pick1}
	send("驅使攻打那一郡", spell(c2[0]))
	send("使者", "1⏎")
	send("繼續嗎", "Y")

	t.Logf("成敗判定 %v｜戰役 %v", judges, battles)
	if len(judges) == 0 {
		t.Fatal("0x2dd66 沒跑到——按鍵沒走到判定那一步")
	}
	if len(battles) == 0 {
		t.Fatal("0x20200 沒跑到——計謀沒有發動戰役")
	}
	b := battles[0]
	if b.a != c1[0] || b.d != c2[0] {
		t.Errorf("戰役的兩個郡是 (%d, %d)，挑的是 (%d, %d)",
			b.a, b.d, c1[0], c2[0])
	}
	if b.aHelp != 0xFFFF || b.dHelp != 0xFFFF {
		t.Errorf("驅虎吞狼我方不參戰，兩個援郡都該是 0xFFFF，量到 (%#x, %#x)",
			b.aHelp, b.dHelp)
	}
	// 兩個郡都不是我方——「驅虎吞狼」教唆的是兩個別人的郡互打。
	for _, id := range []int{b.a, b.d} {
		if owner := int(o.Byte(addr(staBase + uint32(id)*staRec + 30))); owner == mine {
			t.Errorf("郡 %d 是我方的（勢力 %d），不該出現在驅虎吞狼裡", id, owner)
		}
	}
}
