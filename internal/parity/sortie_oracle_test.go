//go:build oracle

package parity

import (
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 出兵／移防的對拍（`docs/mechanics/70-ai` §2.13.6，表 `0x54f4`）。
//
// 這是十八張分派表的最後一張，等級 3、4、5 三格的機器碼**逐位元組相同**
// （把 `call rel16` 的位移正規化之後），等級 0–2 那三格是空操作。三格都
// 走同一組 `0xb2b4`（編隊）＋`0xb47a`（挑目標）。
//
// 四個要釘的點：
//
//	出征兵力目標      `es:[0x2e62]`：5 → 守將裡最小的非零兵力（百）→ 敵鄰郡的最大兵士（百）
//	編隊               洗牌之後從尾端拿人，累到 `100 × 目標`；**留下來的頭段才是出征的部隊**
//	帶走的錢糧         `金 ÷ 兵士（百） × 出征兵力（百）`，走浮點
//	進攻的兵力門檻     `難度係數 × 出征兵力（百） >= 目標郡的兵士（百）`
//
// ⚠ **清單在編隊當中被洗過。** `0xb2b4` 先逐格與 `RND(n)` 交換，再從尾端
// 往前填 `0xFFFF`；所以進場時拍的那一份順序與出場時的清單不是同一個排列，
// 拿進場的前 `count` 筆去加總會得到一個看起來很接近、但是錯的數。要嘛在
// 出口讀真正留下來的清單，要嘛把每一次「填 `0xFFFF`」當場攔下來——這支
// 測試兩件都做，兩條路的差額就是留守的兵力。
//
// 盤面自己擺：等級輪流設成 3–5，每個月把各郡的金拉到 20000、米拉到
// 30000（`0xb47a` 的兩道錢糧門檻），並且在編隊的入口**把兵力目標改成 1**。
// 不改的話目標會是「最強敵鄰郡的兵士」，那個數通常大到把整郡的守將全部
// 吃掉，`count` 收成 0 就整次作廢——上一輪 91 次編隊有 79 次是這樣，
// 於是帶走的錢糧與進攻門檻一個樣本都取不到。改之前先把原值與自己算的
// 對過，兩件事因此都驗得到。

// DS 裡的遠指標：`mov es,[ds:0xa5xx]` 取段值，段值 × 16 就是線性位址。
// **同一個物件在每一支函式裡有各自的段值欄位**（MSC 對每支函式各放一份
// fixup），所以位址不同不代表指到不同的東西。
type sortieGlobals struct {
	want, gold, rice   uint32 // es:[0x2e62]／es:[0x58a]／es:[0x20de]
	count, list, force uint32 // es:[0xc]／es:[0x58c]／es:[0x3c96]
	pref, difficulty   uint32 // es:[0x30fc]／es:[0x30fe]
	foeCount, foeList  uint32 // es:[0x4182]／es:[0x3eb4]
	sta, gen, ds       uint32
}

func resolveSortieGlobals(o *oracle.Oracle) sortieGlobals {
	ds := uint32(o.DSReg()) * 16
	far := func(slot uint16, off uint32) uint32 {
		return uint32(o.Word(addr(ds+uint32(slot))))*16 + off
	}
	return sortieGlobals{
		want:       far(0xa572, 0x2e62),
		gold:       far(0xa574, 0x058a),
		rice:       far(0xa576, 0x20de),
		count:      far(0xa578, 0x000c),
		list:       far(0xa57a, 0x058c),
		force:      far(0xa57e, 0x3c96),
		pref:       far(0xa580, 0x30fc),
		difficulty: far(0xa590, 0x30fe),
		foeCount:   far(0xa58c, 0x4182),
		foeList:    far(0xa58e, 0x3eb4),
		sta:        far(0xa582, 0x0480),
		gen:        far(0xa57c, 0x2210),
		ds:         ds,
	}
}

// TestSortieMustersMatchTheOriginal 釘住兵力目標、編隊、帶走的錢糧與進攻門檻。
func TestSortieMustersMatchTheOriginal(t *testing.T) {
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

	// 盤面自己擺：只有等級 3 以上出兵，開局的電腦諸侯只有 4 與 5，
	// 三格都要走到才算把「三格逐位元組相同」驗過。
	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), uint16(3+alive%3))
		alive++
	}
	t.Logf("活著的勢力 %d 個，AI 等級輪流設成 3–5", alive)

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	var g sortieGlobals
	resolved := false

	// `push cs`（1 B）＋`call rel16`（3 B）＝ 呼叫點 + 4；三格的呼叫點
	// 分別是 `0xb68c`／`0xb6bc`／`0xb6ec`。
	callerLevel := map[uint32]int{0xb690: 3, 0xb6c0: 4, 0xb6f0: 5}

	const forcedWant = 1 // 覆蓋掉的兵力目標，讓編隊真的留得下人

	type gone struct{ index, who, men int }
	type shot struct {
		level, pref, count0, total int
		dropped                    []gone
	}
	var cur shot
	armed := false

	var fails []string
	fail := func(f string, a ...any) {
		if len(fails) < 20 {
			fails = append(fails, fmt.Sprintf(f, a...))
		}
	}

	men := func(o *oracle.Oracle, who int) int {
		if who < 0 || who >= state.GeneralTableSize/state.GeneralRecordSize {
			return 0
		}
		return int(int16(o.Word(addr(genBase + uint32(who*state.GeneralRecordSize+22)))))
	}

	musters, levels := 0, map[int]int{}
	rolls := map[int]int{}
	attacks, attacked, wantSeen, shares := 0, 0, 0, 0

	o.OnCall(addr(0xb2b4), func(o *oracle.Oracle) {
		lvl, ok := callerLevel[o.Caller().Linear()]
		armed = ok
		if !ok {
			return
		}
		if !resolved {
			g, resolved = resolveSortieGlobals(o), true
			if g.sta != staBase {
				fail("州郡表：DS 的遠指標指到 %#x，開機時量到的是 %#x", g.sta, staBase)
			}
			if g.gen != genBase {
				fail("人物表：DS 的遠指標指到 %#x，開機時量到的是 %#x", g.gen, genBase)
			}
		}
		n := int(int16(o.Word(addr(g.count))))
		cur = shot{level: lvl, pref: int(int16(o.Word(addr(g.pref)))), count0: n}

		// 兵力目標：5 → 守將裡最小的非零兵力（百）→ 敵鄰郡的最大兵士（百）。
		want := 5
		for i := 0; i < n; i++ {
			who := int(int16(o.Word(addr(g.list + uint32(i*2)))))
			m := men(o, who)
			cur.total += m
			if v := m / 100; v != 0 && v < want {
				want = v
			}
		}
		for i, k := 0, int(int16(o.Word(addr(g.foeCount)))); i < k; i++ {
			to := int(int16(o.Word(addr(g.foeList + uint32(i*2)))))
			if to < 1 || to > state.PrefectureCount {
				continue
			}
			if v := int(int16(o.Word(addr(staBase + uint32(to*176+16))))); v > want {
				want = v
			}
		}
		if got := int(int16(o.Word(addr(g.want)))); got != want {
			fail("郡 %d：兵力目標原版 %d，算出 %d（守將 %d 人、敵鄰郡 %d 個）",
				cur.pref, got, want, n, int(int16(o.Word(addr(g.foeCount)))))
		} else {
			wantSeen++
		}
		// 換成小的目標，讓「留下來的人」不會被吃光。
		o.SetWord(addr(g.want), forcedWant)
	})

	// 每一次「從清單裡拿掉一個人」都當場記下來（`0xb37f`）。
	o.OnCall(addr(0xb37f), func(o *oracle.Oracle) {
		if !armed {
			return
		}
		i := int(o.BX()) / 2
		who := int(int16(o.Word(addr(g.list + uint32(o.BX())))))
		cur.dropped = append(cur.dropped, gone{i, who, men(o, who)})
	})

	// `0xb474`（`pop si`）是 `0xb2b4` 唯一的出口，兩條路都收得到。
	o.OnCall(addr(0xb474), func(o *oracle.Oracle) {
		if !armed {
			return
		}
		armed = false
		musters++
		levels[cur.level]++

		left := int(int16(o.Word(addr(g.count))))
		force := int(int16(o.Word(addr(g.force))))
		gold := int(int16(o.Word(addr(g.gold))))
		rice := int(int16(o.Word(addr(g.rice))))

		// 拿掉的是尾端，而且是一個接一個往前。
		dropped := 0
		for k, d := range cur.dropped {
			if w := cur.count0 - 1 - k; d.index != w {
				fail("郡 %d：第 %d 個拿掉的是位置 %d，不是從尾端數來的 %d",
					cur.pref, k, d.index, w)
				return
			}
			dropped += d.men
		}
		if left != cur.count0-len(cur.dropped) {
			fail("郡 %d：拿掉 %d 個，清單長度卻從 %d 變成 %d",
				cur.pref, len(cur.dropped), cur.count0, left)
			return
		}

		// 留下來的頭段才是出征的部隊——在出口讀真正的清單，不是進場那一份。
		kept := 0
		for i := 0; i < left; i++ {
			kept += men(o, int(int16(o.Word(addr(g.list+uint32(i*2))))))
		}
		if kept+dropped != cur.total {
			fail("郡 %d：留下 %d ＋ 拿掉 %d ≠ 進場時的 %d，洗牌把人弄丟了",
				cur.pref, kept, dropped, cur.total)
		}
		wantForce := kept / 100
		if left <= 0 {
			wantForce = 0
		}
		if force != wantForce {
			fail("郡 %d 等級 %d：出征兵力（百）原版 %d，留下的 %d 人算出 %d（拿掉的 %d 人算出 %d）",
				cur.pref, cur.level, force, left, wantForce, len(cur.dropped), dropped/100)
			return
		}

		// 停下來的規則：拿到跨過門檻為止，少拿最後一個就跨不過去。
		budget := 100 * forcedWant
		if n := len(cur.dropped); n > 0 && left > 0 {
			if dropped < budget {
				fail("郡 %d：拿掉 %d 兵力，沒到門檻 %d 卻停了", cur.pref, dropped, budget)
			} else if dropped-cur.dropped[n-1].men >= budget {
				fail("郡 %d：少拿最後一個（%d）還是過門檻 %d，多拿了",
					cur.pref, cur.dropped[n-1].men, budget)
			}
		}

		st := staBase + uint32(cur.pref*176)
		units := int(int16(o.Word(addr(st + 16))))
		haveGold := int(int16(o.Word(addr(st + 18))))
		haveRice := int(int16(o.Word(addr(st + 20))))
		if units > 0 && force > 0 {
			shares++
		}
		share := func(amount int) int { return game.SortieShare(amount, units, force) }
		if w := share(haveGold); gold != w {
			fail("郡 %d：帶走的金原版 %d，%d ÷ %d × %d 算出 %d",
				cur.pref, gold, haveGold, units, force, w)
		}
		if w := share(haveRice); rice != w {
			fail("郡 %d：帶走的米原版 %d，%d ÷ %d × %d 算出 %d",
				cur.pref, rice, haveRice, units, force, w)
		}
	})

	// `RND(4)` 三選一：0 無主的鄰郡、1 自己的鄰郡、2 與 3 敵國的鄰郡。
	roll := -1
	o.OnCall(addr(0xb4e9), func(o *oracle.Oracle) {
		roll = int(int16(o.AX()))
		rolls[roll]++
	})
	o.OnCall(addr(0xb5e0), func(*oracle.Oracle) {
		if roll != 2 && roll != 3 {
			fail("打敵國那條分支的 RND(4) 是 %d，不是 2 或 3", roll)
		}
	})

	// 進攻的兵力門檻：`難度係數 × 出征兵力（百） >= 目標郡的兵士（百）`。
	o.OnCall(addr(0xb62c), func(o *oracle.Oracle) {
		attacks++
		diff := int(int16(o.Word(addr(g.difficulty))))
		coef := math.NaN()
		if diff >= 0 && diff < 32 {
			b := o.Bytes(addr(g.ds+0x5430+uint32(diff)*8), 8)
			coef = math.Float64frombits(binary.LittleEndian.Uint64(b))
		}
		force := int(int16(o.Word(addr(g.force))))
		got := int(int16(o.CX()))
		pct := ai.SortieOdds(state.EditionBase, diff)
		if int(math.Round(coef*100)) != pct {
			fail("難度 %d：原版係數 %g，remake 的表給 %d %%", diff, coef, pct)
		}
		if w := game.SortieThreshold(pct, force); got != w {
			fail("難度 %d：係數 %g × 出征兵力 %d 原版得 %d，算出 %d", diff, coef, force, got, w)
		}
	})
	o.OnCall(addr(0xb644), func(*oracle.Oracle) { attacked++ })

	const settle = 40_000_000
	for m := 0; m < 3; m++ {
		for p := 1; p <= state.PrefectureCount; p++ {
			o.SetWord(addr(staBase+uint32(p*176+18)), 20000) // 金：兵士（百）要比它小
			o.SetWord(addr(staBase+uint32(p*176+20)), 30000) // 米：要有 兵士（百）× 15
		}
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("三個月：亂數 %d 次（正對照）、編隊 %d 次（等級分布 %v、兵力目標對上 %d 次）、"+
		"三選一分布 %v、帶走錢糧 %d 次、進攻判定 %d 次（真的打 %d 次）",
		rnd, musters, levels, wantSeen, rolls, shares, attacks, attacked)
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	for _, s := range fails {
		t.Error(s)
	}
	if musters == 0 {
		t.Skip("三個月裡一次都沒走到出兵那一張表")
	}
	if len(levels) < 3 {
		t.Errorf("只走到 %d 個等級，三格都要驗到才算數", len(levels))
	}
	if shares == 0 {
		t.Error("一次都沒量到帶走的錢糧")
	}
}
