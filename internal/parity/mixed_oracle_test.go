//go:build oracle

package parity

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 郡裡站著別的勢力的武將時，「調整兵力」算不算他（`0xc2c4`）。
//
// 原版的 `0xc2c4` 呼叫 `buildRoster(郡, 2)`，而模式 2 的過濾條件只有
// **所在郡（offset 19）與身分（offset 17） ∈ {0,1,2,3}**——`docs/re/07` §6
// 七個模式一個都沒有比對勢力（offset 18）。所以靜態讀出來的答案是
// 「算」，但那是從「過濾條件裡沒有那一項」推出來的，屬於**沒看到的東西
// 不算證據**那一類：真正該問的是「擺一個外人進去，原版動不動他的兵」。
//
// remake 這一邊 `game.Redistribute` 遇到 `勢力 != 下令的勢力` 就回
// `ErrUnknownUnit`，而 `ApplyAll` 一道失敗會中止該勢力後面全部的命令——
// 月度對拍固定卡在「勢力 4 的第 3 道命令」就是這樣來的。
//
// 盤面自己擺：每個有三位以上在職武將的郡，挑**槽號最小**的那一位
// （身分 3、不是君主）把勢力改掉。挑最小的是因為郡的所屬每回合由
// `0x1e394` 重算——掃全部 350 筆、後寫的蓋前寫的，改最後一位會把整個郡
// 送給外人，那就不是混編而是易主了。

// TestRedistributeCountsForeignGenerals 量原版在混編的郡裡算誰。
func TestRedistributeCountsForeignGenerals(t *testing.T) {
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

	// 每個郡的在職武將（身分 0–3），照槽號。
	byPref := map[int][]int{}
	for i := 0; i < genCount; i++ {
		if r := o.Byte(addr(rec(i) + 17)); r > 3 {
			continue
		}
		if o.Byte(addr(rec(i)+18)) == 0xFF {
			continue
		}
		p := int(o.Byte(addr(rec(i) + 19)))
		byPref[p] = append(byPref[p], i)
	}

	// 挑槽號最小、身分 3 的那一位換勢力。
	planted := map[int]int{} // 郡 → 被改的槽
	for p, who := range byPref {
		if len(who) < 3 || p < 1 || p > state.PrefectureCount {
			continue
		}
		for _, i := range who {
			if o.Byte(addr(rec(i)+17)) != 3 {
				continue
			}
			mine := o.Byte(addr(rec(i) + 18))
			var other uint8 = 0xFF
			for _, j := range who {
				if f := o.Byte(addr(rec(j) + 18)); f != mine && f != 0xFF {
					other = f
					break
				}
			}
			if other == 0xFF {
				// 同郡沒有第二個勢力可借，就借諸侯表裡活著的另一個。
				for k := 0; k < state.MasterTableSize/72; k++ {
					if o.Word(addr(base+uint32(k*72))) == 0xFFFF || uint8(k) == mine {
						continue
					}
					other = uint8(k)
					break
				}
			}
			if other == 0xFF {
				break
			}
			o.SetByte(addr(rec(i)+18), other)
			planted[p] = i
			break
		}
	}
	t.Logf("在 %d 個郡各種了一位外勢力的武將", len(planted))
	if len(planted) == 0 {
		t.Skip("盤面上湊不出可以種外人的郡")
	}

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	type shot struct {
		pref, mode   int
		want, got    []int
		foreign      int  // 種進去的槽（-1 ＝ 這個郡沒種）
		foreignInSet bool // 原版的名單裡有沒有他
	}
	var shots []shot
	var cur *shot

	o.OnCall(addr(0x0c2de), func(o *oracle.Oracle) {
		p := int(int16(o.StackWord(0)))
		s := shot{pref: p, mode: int(int16(o.StackWord(1))), foreign: -1}
		// 名單該有誰：所在郡相同、身分 0–3，**不看勢力**。
		for i := 0; i < genCount; i++ {
			if o.Byte(addr(rec(i)+17)) > 3 || int(o.Byte(addr(rec(i)+19))) != p {
				continue
			}
			s.want = append(s.want, i)
		}
		if j, ok := planted[p]; ok {
			s.foreign = j
		}
		shots = append(shots, s)
		cur = &shots[len(shots)-1]
	})
	o.OnCall(addr(0x0c4a6), func(o *oracle.Oracle) {
		if cur == nil {
			return
		}
		slot := int(o.BX()) / state.GeneralRecordSize
		cur.got = append(cur.got, slot)
		if slot == cur.foreign {
			cur.foreignInSet = true
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

	mixed, hit, mismatch := 0, 0, 0
	for _, s := range shots {
		if s.mode != 2 {
			t.Errorf("郡 %d：調整兵力用的模式是 %d，不是 2", s.pref, s.mode)
		}
		sort.Ints(s.got)
		if len(s.got) > 0 && !sameInts(s.want, s.got) {
			if mismatch < 6 {
				t.Errorf("郡 %d：原版動了 %v，「同郡 ＋ 身分 0–3」算出 %v",
					s.pref, s.got, s.want)
			}
			mismatch++
		}
		if s.foreign >= 0 && len(s.got) > 0 {
			mixed++
			if s.foreignInSet {
				hit++
			}
		}
	}
	t.Logf("兩個月：亂數 %d 次（正對照）、調整兵力 %d 郡次，其中 %d 郡次有外人；"+
		"原版把外人算進去 %d 次", rnd, len(shots), mixed, hit)
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if mixed == 0 {
		t.Skip("兩個月裡沒有一個混編的郡真的調整過兵力")
	}
	if hit != mixed {
		t.Errorf("%d 郡次有外人，只有 %d 次被算進去——過濾條件不只「同郡 ＋ 身分」",
			mixed, hit)
	}
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
