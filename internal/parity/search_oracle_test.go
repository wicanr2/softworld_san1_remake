//go:build oracle

package parity

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 尋訪人才的對拍（`docs/mechanics/70-ai`，`game.SearchTierFor`）。
//
// 六支分派常式在 `0xcd20` 起（間隔 `0x3a`），形狀相同而**三個立即數都
// 隨等級變**：
//
//	RND(10) <= Bar → 這回合不做          0xcd33 起的 `cmp $Bar,%ax`
//	門檻 ＝ RND(Spread) ＋ Floor
//	call 0xcc86(尋訪者, 門檻)
//	  謀略 > 門檻 → 找到一位身分 9 的人   0xccd2  AX ＝ 尋訪者的謀略
//	  0xccf9 把他的身分改成 8            ＝ 這一次成功
//
// **六個呼叫點逐支量**：`0xcd52`／`0xcd8c`／`0xcdc6`／`0xce00`／`0xce3a`／
// `0xce74`，返回位址 ＋3。

// TestSearchMatchesTheOriginal 讓電腦諸侯去尋訪，逐次核對三張表。
func TestSearchMatchesTheOriginal(t *testing.T) {
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
	staBase := base + uint32(state.MasterTableSize)
	genBase := staBase + uint32(state.PrefectureTableSize)

	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), uint16(alive%6))
		alive++
	}
	t.Logf("活著的勢力 %d 個，AI 等級輪流設成 0–5", alive)

	// **可找的人自己擺**：常式收的是身分 9（在野未露面），劇本裡不多，
	// 而且尋到一個就少一個。把未登場（11）的人改成 9、散到 42 個郡。
	seeded := 0
	for i := 0; i < 350; i++ {
		at := genBase + uint32(i*30)
		if o.Byte(addr(at+17)) != 11 {
			continue
		}
		o.SetByte(addr(at+17), 9)
		o.SetByte(addr(at+18), 0xFF)
		o.SetByte(addr(at+16), 0xFF)
		o.SetByte(addr(at+19), uint8(1+seeded%state.PrefectureCount))
		seeded++
	}
	t.Logf("把 %d 個未登場的人放成身分 9，散到 %d 個郡", seeded, state.PrefectureCount)

	callerLevel := map[uint32]int{
		0xcd55: 0, 0xcd8f: 1, 0xcdc9: 2, 0xce03: 3, 0xce3d: 4, 0xce77: 5,
	}

	type shot struct {
		level, bar, intel int
		ok                bool
	}
	var shots []shot
	var cur shot
	armed := false

	o.OnCall(addr(0x0cc86), func(o *oracle.Oracle) {
		lvl, ok := callerLevel[o.Caller().Linear()]
		armed = ok
		if ok {
			cur = shot{level: lvl, bar: int(int16(o.Arg(1))), intel: -1}
		}
	})
	// `0xccd2` 每個候選人都會走到一次；謀略是尋訪者的，每次都一樣。
	o.OnCall(addr(0x0ccd2), func(o *oracle.Oracle) {
		if armed {
			cur.intel = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0ccf9), func(*oracle.Oracle) {
		if armed {
			cur.ok = true
		}
	})
	for ret := range callerLevel {
		o.OnCall(addr(ret), func(*oracle.Oracle) {
			if armed {
				shots = append(shots, cur)
				armed = false
			}
		})
	}

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	const settle = 40_000_000
	for m := 0; m < 3; m++ {
		for p := 1; p <= state.PrefectureCount; p++ {
			o.SetWord(addr(staBase+uint32(p*176+18)), 20000)
		}
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("三個月：亂數 %d 次（正對照）、尋訪 %d 次", rnd, len(shots))
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if len(shots) == 0 {
		t.Fatal("三個月裡一次都沒尋訪——呼叫端的返回位址對不上")
	}

	seen, bad, found := map[int]int{}, 0, 0
	for _, s := range shots {
		seen[s.level]++
		tier := game.SearchTierFor(s.level)
		lo, hi := tier.Floor, tier.Floor+tier.Spread-1
		if s.bar < lo || s.bar > hi {
			t.Errorf("等級 %d：原版的門檻是 %d，不在 RND(%d)+%d ＝ %d–%d 裡",
				s.level, s.bar, tier.Spread, tier.Floor, lo, hi)
			bad++
			continue
		}
		if s.intel < 0 {
			continue // 這一郡沒有身分 9 的人，判定沒走到
		}
		if want := s.intel > s.bar; want != s.ok {
			t.Errorf("等級 %d：謀略 %d、門檻 %d → 原版%s，判準說%s",
				s.level, s.intel, s.bar,
				map[bool]string{true: "找到", false: "沒找到"}[s.ok],
				map[bool]string{true: "找到", false: "沒找到"}[want])
			bad++
		}
		if s.ok {
			found++
		}
	}
	levels := make([]int, 0, len(seen))
	for k := range seen {
		levels = append(levels, k)
	}
	sort.Ints(levels)
	for _, k := range levels {
		t.Logf("等級 %d：%d 次", k, seen[k])
	}
	t.Logf("%d 次尋訪（找到 %d 次），%d 項對不上", len(shots), found, bad)
	if len(seen) < 6 {
		t.Errorf("只走到 %d 個等級的常式，六個都要驗到才算數", len(seen))
	}
}
