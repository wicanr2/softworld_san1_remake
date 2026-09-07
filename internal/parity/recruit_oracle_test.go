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

// 登用人才的對拍（`docs/mechanics/70-ai` §2.1、`game.RecruitPersuasion`）。
//
// 六個等級各一支分派常式（`0xd0ae` 起，六支的起點是
// `0xd0ae`／`0xd10e`／`0xd16e`／`0xd1ce`／`0xd230`／`0xd292`），掃全部 350
// 個人物槽，挑在本郡而且身分是 8 或 10（在野）的，呼叫共用常式
// `0xce8c(人物槽, 費用, 加成)`。**呼叫點在每支的 `+0x4c`**，所以返回
// 位址是起點 `+0x4f`——六支的長度不一樣（等級 3–5 推的是四位元組的
// 立即數），拿固定間隔算會少掉後三個。
//
//	0xcf10  SI ＝ 第一次 RND(4)
//	0xcf18  AX ＝ 第二次 RND(4)
//	0xcf44  SI ＝ 難度的能力值部分 ＝ 謀略÷(r1+3) ＋ 戰力÷(r2+3)
//	0xcf75  牽絆閘門 A：對象效力於招募方 → 難度 0
//	0xcfc5  牽絆閘門 B：對象效力於第三方 → 難度 ＝ 160 ＋ RND(10)
//	0xcff2  AX ＝ 太守的魅力
//	0xd006  AX ＝ 諸侯的人望
//	0xd019  AX ＝ 說服力
//	0xd030  AX ＝ RND(人望÷2)，SI ＝ 人望÷2
//	0xd058  AL ＝ 寫回去的忠誠（＝ 這一次登用成功）

// TestRecruitMatchesTheOriginal 讓電腦諸侯去登用在野武將，逐次核對。
func TestRecruitMatchesTheOriginal(t *testing.T) {
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

	// **在野的人自己擺**：劇本 001 只有 12 個身分 8（在該郡露面的在野），
	// 而且登用一次就少一個——照劇本跑兩個月只收得到三個樣本，六個等級
	// 走不完。把 197 個「未登場」（身分 11）改成在野、散到 42 個郡，
	// 勢力與忠誠都設成 0xFF 的哨兵。
	//
	// ⚠ **身分 9 不算**：常式收的是 8 或 10（`0xd0df`／`0xd0e7`），
	// 所以身分 9（在野但不列入該郡的在野數）一個都不會被挑到——
	// 這裡把 9 與 11 一起改成 8 才湊得出六個等級的樣本。
	seeded := 0
	for i := 0; i < 350; i++ {
		at := genBase + uint32(i*30)
		if r := o.Byte(addr(at + 17)); r != 11 && r != 9 {
			continue
		}
		o.SetByte(addr(at+17), 8)
		o.SetByte(addr(at+18), 0xFF)
		o.SetByte(addr(at+16), 0xFF)
		o.SetByte(addr(at+19), uint8(1+seeded%state.PrefectureCount))
		seeded++
	}
	t.Logf("把 %d 個未登場的人放成在野，散到 %d 個郡", seeded, state.PrefectureCount)

	// **六個返回位址逐支量出來，不要從第一支外推。** 呼叫點是
	// `0xd0fa`／`0xd15a`／`0xd1ba`／`0xd21b`／`0xd27d`／`0xd2de`——
	// 前三支的間隔是 `0x60`，後三支因為推的立即數變長而多一個位元組。
	// 照固定間隔算會讓等級 3 與 4 一次都對不上，而輸出看起來只是
	// 「那兩個等級剛好沒輪到」。
	callerLevel := map[uint32]int{
		0xd0fd: 0, 0xd15d: 1, 0xd1bd: 2, 0xd21e: 3, 0xd280: 4, 0xd2e1: 5,
	}

	type shot struct {
		level, fee, bonus        int
		intel, war, r1, r2, hard int
		charm, prestige, force   int
		bondFree, bondWall       int
		loyalRoll, half, wrote   int
		ok                       bool
	}
	var shots []shot
	var cur shot
	armed := false

	o.OnCall(addr(0x0ce8c), func(o *oracle.Oracle) {
		lvl, ok := callerLevel[o.Caller().Linear()]
		armed = ok
		if !ok {
			return
		}
		slot := uint32(o.Arg(0)) * 30
		cur = shot{level: lvl, fee: int(int16(o.Arg(1))), bonus: int(int16(o.Arg(2))),
			intel:    int(o.Byte(addr(genBase + slot + 9))),
			war:      int(o.Byte(addr(genBase + slot + 10))),
			bondWall: -1}
	})
	o.OnCall(addr(0x0cf10), func(o *oracle.Oracle) {
		if armed {
			cur.r1 = int(int16(o.SI()))
		}
	})
	o.OnCall(addr(0x0cf18), func(o *oracle.Oracle) {
		if armed {
			cur.r2 = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0cf44), func(o *oracle.Oracle) {
		if armed {
			cur.hard = int(int16(o.SI()))
		}
	})
	o.OnCall(addr(0x0cf75), func(*oracle.Oracle) {
		if armed {
			cur.bondFree = 1
		}
	})
	o.OnCall(addr(0x0cfc5), func(o *oracle.Oracle) {
		if armed {
			cur.bondWall = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0cff2), func(o *oracle.Oracle) {
		if armed {
			cur.charm = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0d006), func(o *oracle.Oracle) {
		if armed {
			cur.prestige = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0d019), func(o *oracle.Oracle) {
		if armed {
			cur.force = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x0d030), func(o *oracle.Oracle) {
		if armed {
			cur.loyalRoll, cur.half = int(int16(o.AX())), int(int16(o.SI()))
		}
	})
	o.OnCall(addr(0x0d058), func(o *oracle.Oracle) {
		if armed {
			cur.wrote, cur.ok = int(int8(o.AX()&0xFF)), true
		}
	})
	// 常式的三個出口都在 `0xd0a9` 之後；用下一次進入來收尾太晚，
	// 所以在呼叫端的返回位址上收——每一支都是 `add sp,6`。
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
	for m := 0; m < 4; m++ {
		for p := 1; p <= state.PrefectureCount; p++ {
			o.SetWord(addr(staBase+uint32(p*176+18)), 20000) // 金：登用要付費
		}
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("四個月：亂數 %d 次（正對照）、登用 %d 次", rnd, len(shots))
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if len(shots) == 0 {
		t.Fatal("四個月裡一次都沒登用——呼叫端的返回位址對不上")
	}

	seen, bad, ok := map[int]int{}, 0, 0
	for _, s := range shots {
		seen[s.level]++
		if w := game.RecruitFee(s.level); w != s.fee {
			t.Errorf("等級 %d：原版推的費用是 %d，remake 的表說 %d", s.level, s.fee, w)
			bad++
			continue
		}
		if w := game.RecruitBonus(s.level); w != s.bonus {
			t.Errorf("等級 %d：原版推的加成是 %d，remake 的表說 %d", s.level, s.bonus, w)
			bad++
			continue
		}
		if s.r1 < 0 || s.r1 > 3 || s.r2 < 0 || s.r2 > 3 {
			continue // 這一次連能力值都沒算到（人滿了或沒預算）
		}
		if w := game.RecruitDifficulty(s.intel, s.war, s.r1, s.r2); w != s.hard {
			t.Errorf("等級 %d：謀略 %d、戰力 %d、擲 %d/%d → 原版的難度是 %d，"+
				"remake 算 %d", s.level, s.intel, s.war, s.r1, s.r2, s.hard, w)
			bad++
			continue
		}
		if s.force == 0 && s.prestige == 0 {
			continue // 沒走到說服力那一段
		}
		if w := game.RecruitPersuasion(s.prestige, s.charm, s.bonus); w != s.force {
			t.Errorf("等級 %d：人望 %d、魅力 %d、加成 %d → 原版的說服力是 %d，"+
				"remake 算 %d", s.level, s.prestige, s.charm, s.bonus, s.force, w)
			bad++
			continue
		}
		hard := s.hard
		if s.bondFree == 1 {
			hard = game.RecruitBondFree
		} else if s.bondWall >= 0 {
			hard = s.bondWall
		}
		if want := s.force > hard; want != s.ok {
			t.Errorf("等級 %d：說服力 %d、難度 %d → 原版%s，判準說%s",
				s.level, s.force, hard,
				map[bool]string{true: "成功", false: "失敗"}[s.ok],
				map[bool]string{true: "成功", false: "失敗"}[want])
			bad++
			continue
		}
		if !s.ok {
			continue
		}
		ok++
		want := s.half + s.loyalRoll + s.bonus
		if want > 100 {
			want = 100
		}
		if want != s.wrote {
			t.Errorf("等級 %d：人望 %d（半 %d）、擲 %d、加成 %d → 原版寫忠誠 %d，"+
				"照算式是 %d", s.level, s.prestige, s.half, s.loyalRoll,
				s.bonus, s.wrote, want)
			bad++
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
	t.Logf("%d 次登用（成功 %d 次），%d 項對不上", len(shots), ok, bad)
	if len(seen) < 6 {
		t.Errorf("只走到 %d 個等級的常式，六個都要驗到才算數", len(seen))
	}
}
