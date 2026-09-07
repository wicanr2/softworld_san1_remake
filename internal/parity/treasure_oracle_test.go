//go:build oracle

package parity

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 賞賜物品的對拍（表 `0x56b4`，`docs/mechanics/20-personnel` §5.2）。
//
// 等級 0–2 是空操作；等級 3–5 各自把四支常式跑一遍，每一支管一種寶物：
//
//	0xd962 兵書（諸侯 offset 15）→ 謀略
//	0xdac0 寶刀（offset 16）      → 戰力
//	0xdc1e 美女（offset 17）      → 魅力
//	0xdd94 駿馬（offset 18）      → 戰力 ＋ 魅力
//
// 每一支的形狀相同：
//
//	RND(100) > 40 → 不做
//	庫存 <= RND(2) + 2 → 不做            ; 囤到 3–4 件以上才發
//	名單裡第一個「該能力 > RND(20) + 60，或者是君主」而且該能力 < 90 的人
//	該能力 += RND(2) + 底  （上限 90）    ; 底 ＝ 2／3／5／(2 與 3)
//	忠誠   += RND(30) + 新能力 ÷ 2       ; 美女那一支是 RND(50) + 50
//	忠誠上限 100
//
// **說明書 p.24 的數字是下界不是定值**：兵書 +2 謀略、寶刀 +3 戰力、
// 美女 +5 魅力、駿馬 +2 戰力 +3 魅力——碼裡在每一項上面再加 `RND(2)`
// （`0xda44`／`0xdba2`／`0xdd09`／`0xde7f`／`0xde96` 的 `add $底,%al`）。
// 所以手冊沒寫錯，只是沒提那一擲。

// TestTreasureGiftMatchesTheOriginal 核對四種寶物的增幅與忠誠。
func TestTreasureGiftMatchesTheOriginal(t *testing.T) {
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

	// **庫存自己擺**：要囤到超過 RND(2)+2 才發得出去，開局多數勢力沒有。
	// 等級全設 5（等級 0–2 這張表是空操作）。
	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), 5)
		for k := 15; k <= 18; k++ {
			o.SetWord(addr(base+uint32(i*72+k)), 9)
		}
		alive++
	}
	t.Logf("%d 個勢力全設等級 5，四種寶物各囤 9 件", alive)

	names := [4]string{"兵書", "寶刀", "美女", "駿馬"}
	// 每一種寶物的增幅底（`add $底,%al`）；實際增幅是底 ＋ RND(2)。
	floors := [4]int{2, 3, 5, 2}
	type shot struct {
		kind, ability, delta, loyalDelta int
	}
	var shots []shot
	var cur shot
	kind := -1

	for k, at := range []uint32{0xd962, 0xdac0, 0xdc1e, 0xdd94} {
		k := k
		o.OnCall(addr(at), func(*oracle.Oracle) { kind = k })
	}
	// 能力的加項：AL ＝ 增幅，SI ＝ 30 × 槽（駿馬取第一項）。
	for k, at := range []uint32{0xda4a, 0xdba8, 0xdd0f, 0xde85} {
		k, off := k, []uint32{9, 10, 11, 10}[k]
		o.OnCall(addr(at), func(o *oracle.Oracle) {
			if kind != k {
				return
			}
			cur = shot{kind: k,
				ability: int(int8(o.Byte(addr(genBase + uint32(o.SI()) + off)))),
				delta:   int(o.AX() & 0xFF)}
		})
	}
	// 忠誠的加項。
	for k, at := range []uint32{0xda6d, 0xdbcb, 0xdd26, 0xdebf} {
		k, useAL := k, k == 2
		o.OnCall(addr(at), func(o *oracle.Oracle) {
			if kind != k || cur.kind != k {
				return
			}
			if useAL {
				cur.loyalDelta = int(o.AX() & 0xFF)
			} else {
				cur.loyalDelta = int(o.CX() & 0xFF)
			}
			shots = append(shots, cur)
			cur = shot{kind: -1}
		})
	}

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	const settle = 40_000_000
	for m := 0; m < 3; m++ {
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("三個月：亂數 %d 次（正對照）、賞賜物品 %d 次", rnd, len(shots))
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if len(shots) == 0 {
		t.Fatal("三個月裡一次都沒賞賜物品——庫存或等級的盤面沒擺成功")
	}

	seen, bad := map[int]int{}, 0
	for _, s := range shots {
		seen[s.kind]++
		if lo := floors[s.kind]; s.delta < lo || s.delta > lo+1 {
			t.Errorf("%s：能力增幅 %d，原版是 RND(2) + %d",
				names[s.kind], s.delta, lo)
			bad++
			continue
		}
		// 忠誠：美女是 RND(50) + 50，其餘是 RND(30) + 新能力 ÷ 2。
		lo, hi := (s.ability+s.delta)/2, (s.ability+s.delta)/2+29
		if s.kind == 2 {
			lo, hi = 50, 99
		}
		if s.loyalDelta < lo || s.loyalDelta > hi {
			t.Errorf("%s：能力 %d ＋%d，忠誠增幅 %d 不在 %d–%d 裡",
				names[s.kind], s.ability, s.delta, s.loyalDelta, lo, hi)
			bad++
		}
	}
	kinds := make([]int, 0, len(seen))
	for k := range seen {
		kinds = append(kinds, k)
	}
	sort.Ints(kinds)
	for _, k := range kinds {
		t.Logf("%s：%d 次", names[k], seen[k])
	}
	t.Logf("%d 次賞賜，%d 項對不上", len(shots), bad)
	if len(seen) < 4 {
		t.Errorf("只走到 %d 種寶物，四種都要驗到才算數", len(seen))
	}
}
