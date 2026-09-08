//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 賞賜物品**挑誰**的對拍（兵書那一支，`0xd962`）。
//
// 四支的形狀相同，差別只在排序常式與看的那一項能力：
//
//	兵書 0xd9bc → 0xf360（謀略）    寶刀 0xdb1a → 0xf440（戰力）
//	美女 0xdc78 → 0xf520（魅力）    駿馬 0xddee → 0xf440（戰力）
//
// 排序鍵是「該能力 ＋ 加權表[身分]」，加權表 `DS:0x5986` 就是選出行動者
// 那一張（2000／1600／1200／800／…）。排完之後從頭找第一個
// 「該能力 > `RND(20) + 60`，**或者是君主**」而且該能力 < 90 的人。
//
// ⚠ **門檻逐人重擲**：`RND` 的呼叫點 `0xd9f2` 在迴圈裡面，所以要在
// `0xd9f7` 逐次收，不能事後補算。
//
// 這支測試只驗兵書那一支——四支共用同一段碼，形狀對了其餘三支就對。

// TestTreasurePickMatchesTheOriginal 核對排序鍵與挑中的那一位。
func TestTreasurePickMatchesTheOriginal(t *testing.T) {
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
	rec := func(i int) uint32 { return genBase + uint32(i*state.GeneralRecordSize) }
	const genCount = state.GeneralTableSize / state.GeneralRecordSize

	// 盤面自己擺：庫存囤到 9（要超過 RND(2)+2 才發得出去），等級全設 5。
	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), 5)
		for off := 15; off <= 18; off++ {
			o.SetByte(addr(base+uint32(i*72+off)), 9)
		}
		alive++
	}
	t.Logf("%d 個勢力：等級設 5、四種寶物各囤 9 件", alive)

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	// 加權表 `DS:0x5986`（與 `ai.actorWeight` 同一張）。
	weight := [12]int{2000, 1600, 1200, 800, 2000, 1600, 1200, 800, 0, 0, 400, 0}

	var g sortieGlobals
	resolved := false
	type run struct {
		list  []int
		rolls []int
	}
	var cur run
	armed := false
	var fails []string
	fail := func(f string, a ...any) {
		if len(fails) < 12 {
			fails = append(fails, fmt.Sprintf(f, a...))
		}
	}
	runs, picks, lords := 0, 0, 0

	// 排序剛做完：讀那一份排好的清單。
	o.OnCall(addr(0x0d9c1), func(o *oracle.Oracle) {
		if !resolved {
			g, resolved = resolveSortieGlobals(o), true
		}
		cur = run{}
		n := int(int16(o.Word(addr(g.count))))
		for i := 0; i < n; i++ {
			cur.list = append(cur.list, int(int16(o.Word(addr(g.list+uint32(i*2))))))
		}
		armed = true
		runs++
		// 鍵是「謀略 ＋ 加權表[身分]」遞減。
		key := func(who int) int {
			if who < 0 || who >= genCount {
				return -1 << 30
			}
			r := int(o.Byte(addr(rec(who) + 17)))
			w := 0
			if r < len(weight) {
				w = weight[r]
			}
			return int(int8(o.Byte(addr(rec(who)+9)))) + w
		}
		for i := 1; i < len(cur.list); i++ {
			if key(cur.list[i-1]) < key(cur.list[i]) {
				fail("排序不是「謀略 ＋ 加權表[身分]」遞減：第 %d 位（槽 %d，鍵 %d）"+
					"排在第 %d 位（槽 %d，鍵 %d）前面",
					i-1, cur.list[i-1], key(cur.list[i-1]),
					i, cur.list[i], key(cur.list[i]))
				break
			}
		}
	})
	// 迴圈裡逐人重擲的 RND(20)。
	o.OnCall(addr(0x0d9f7), func(o *oracle.Oracle) {
		if armed {
			cur.rolls = append(cur.rolls, int(int16(o.AX())))
		}
	})
	// 挑中的那一位：`0xda44` 是兵書那一支寫謀略的地方。
	o.OnCall(addr(0x0da44), func(o *oracle.Oracle) {
		if !armed {
			return
		}
		armed = false
		picks++
		got := int(o.SI()) / state.GeneralRecordSize
		// ⚠ **每一筆都擲**：`RND` 的呼叫點在迴圈最前面（`0xd9f2` 緊接在
		// 讀 `list[i]` 之後），能力已滿 90 的那些也照擲。跳過不擲的話
		// 之後的擲值全部錯位——23 次裡就是這樣錯了 1 次。
		want, k := -1, 0
		for _, who := range cur.list {
			if k >= len(cur.rolls) {
				break
			}
			bar := cur.rolls[k] + 60
			k++
			if who < 0 || who >= genCount {
				continue
			}
			intel := int(int8(o.Byte(addr(rec(who) + 9))))
			if intel >= 90 {
				continue
			}
			if o.Byte(addr(rec(who)+17)) == 0 || intel > bar { // 君主跳過門檻
				want = who
				break
			}
		}
		if want >= 0 && o.Byte(addr(rec(want)+17)) == 0 {
			lords++
		}
		if got != want {
			fail("原版把兵書給了槽 %d，照排序與門檻算出的是槽 %d（清單 %v、擲 %v）",
				got, want, cur.list, cur.rolls)
		}
	})

	const settle = 40_000_000
	for m := 0; m < 2; m++ {
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("兩個月：亂數 %d 次（正對照）、兵書那一支跑 %d 次、送出 %d 次"+
		"（其中收禮的是君主 %d 次）", rnd, runs, picks, lords)
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	for _, s := range fails {
		t.Error(s)
	}
	if picks == 0 {
		t.Skip("兩個月裡兵書一次都沒送出去")
	}
}
