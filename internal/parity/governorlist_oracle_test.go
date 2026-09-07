//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 指定太守排的是哪一份名單（`0xd652` → `0xf600`）。
//
// `0xd652` **自己不呼叫 `buildRoster`**：它在 `0xd6c2` 直接
// `lcall 0xf17:0x490` 排當下那一份清單（`es:[0x58c]`／長度 `es:[0xc]`），
// 所以名單是分派器前面某一張表留下來的。`0xc2c4`（調整兵力）建的是模式 2
// ——**「指定太守排的也是那一份」是外推，不是證據**，所以在排序之前把
// 清單讀出來，跟四個候選條件比。
//
// 候選（`docs/re/07` §6 的七個模式收斂成四種形狀）：
//
//	A  所在郡相同 ＋ 身分 ∈ {0,1,2,3}          ; 模式 2
//	B  A ＋ 勢力 == 郡的所屬
//	C  所在郡相同 ＋ 身分 ∈ {1,2,3}            ; 模式 5
//	D  所在郡相同 ＋ 身分 ∈ {2,3}

// TestGovernorSortListShape 量指定太守排序前那一份名單的形狀。
func TestGovernorSortListShape(t *testing.T) {
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
	const genCount = state.GeneralTableSize / state.GeneralRecordSize
	rec := func(i int) uint32 { return genBase + uint32(i*state.GeneralRecordSize) }

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	var g sortieGlobals
	resolved := false

	calls := 0
	fit := map[string]int{}
	var notes []string

	o.OnCall(addr(0x0d6c2), func(o *oracle.Oracle) {
		if !resolved {
			g, resolved = resolveSortieGlobals(o), true
		}
		calls++
		p := int(int16(o.Word(addr(g.pref))))
		n := int(int16(o.Word(addr(g.count))))
		got := map[int]bool{}
		for i := 0; i < n; i++ {
			v := int(int16(o.Word(addr(g.list + uint32(i*2)))))
			if v >= 0 && v < genCount {
				got[v] = true
			}
		}
		owner := o.Byte(addr(staBase + uint32(p*176+30)))
		want := func(f func(rank, faction uint8) bool) map[int]bool {
			m := map[int]bool{}
			for i := 0; i < genCount; i++ {
				if int(o.Byte(addr(rec(i)+19))) != p {
					continue
				}
				if f(o.Byte(addr(rec(i)+17)), o.Byte(addr(rec(i)+18))) {
					m[i] = true
				}
			}
			return m
		}
		cand := map[string]map[int]bool{
			"A 身分0-3": want(func(r, _ uint8) bool { return r <= 3 }),
			"B A+同勢力": want(func(r, f uint8) bool { return r <= 3 && f == owner }),
			"C 身分1-3": want(func(r, _ uint8) bool { return r >= 1 && r <= 3 }),
			"D 身分2-3": want(func(r, _ uint8) bool { return r >= 2 && r <= 3 }),
		}
		hit := ""
		for k, m := range cand {
			if sameSet(m, got) {
				if hit != "" {
					hit += "／"
				}
				hit += k
			}
		}
		if hit == "" {
			hit = "都不是"
			if len(notes) < 5 {
				notes = append(notes, describeSet(t, p, got, cand))
			}
		}
		fit[hit]++
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

	t.Logf("兩個月：亂數 %d 次（正對照）、指定太守排序 %d 次；名單形狀 %v",
		rnd, calls, fit)
	for _, s := range notes {
		t.Log(s)
	}
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if calls == 0 {
		t.Skip("兩個月裡一次都沒走到指定太守的排序")
	}
}

func sameSet(a, b map[int]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func describeSet(t *testing.T, pref int, got map[int]bool, cand map[string]map[int]bool) string {
	t.Helper()
	s := ""
	for k, m := range cand {
		s += k + "=" + itoa(len(m)) + " "
	}
	return "郡 " + itoa(pref) + "：原版名單 " + itoa(len(got)) + " 筆，候選 " + s
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}
