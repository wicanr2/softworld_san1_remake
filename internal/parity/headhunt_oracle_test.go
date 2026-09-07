//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 挖角的成敗判定（`docs/mechanics/20-personnel`，常式 `0x1dc0a`）。
//
// 三支分派常式（等級 3／4／5 在 `0xe290`／`0xe32a`／`0xe3d2`，等級 0–2
// 是空操作）各自擲 `RND(10) > 6／3／1`，過了就呼叫 `0x1de46`：
// 先扣 100 金，再呼叫 `0x1dc0a(目標槽, 目標郡, 加成)`。
//
// **`0x1dc0a` 回的不是成敗，是成功之後的忠誠**——算出來 <= 0 就當失敗
// （`0x1de34`）。掛四個點就把整條式子攤開：
//
//	0x1dcb8  AX ＝ 門檻 ＝ (我方君主魅力×3 ＋ 我方人望×4) ÷ 6 ＋ 加成
//	0x1dced  SI ＝ 抵抗的固定部分
//	0x1ddcc  AX ＝ 門檻（要與 [-2] 比的那個）
//	0x1de1d  CX ＝ 新忠誠（還沒夾）
//	0x1de40  AX ＝ 回傳值（0xFFFF ＝ 失敗）

// TestHeadhuntMatchesTheOriginal 讓電腦諸侯去挖角，逐次核對判定。
func TestHeadhuntMatchesTheOriginal(t *testing.T) {
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

	// **等級全部拉到 5**：挖角是等級 3 才開的行為，而且要君主在本郡，
	// 樣本本來就少——等級 5 的機率是 80 %。
	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), 5)
		alive++
	}
	t.Logf("%d 個活著的勢力全部設成等級 5", alive)

	// **三個條件式加項要各自走到**，照劇本跑一個都碰不到：
	//
	//   - 對方人望 >= RND(5) + 90  → 諸侯 offset 8（人望）全設 95–100
	//   - 目標忠誠 >= RND(7) + 87  → 在職者的忠誠設在 87–93
	//   - 對方諸侯持有玉璽         → 一半的勢力 offset 14 設 1
	//
	// ⚠ 忠誠有**兩道相反的門**：候選過濾要 `忠誠 < RND(15) + 80`
	// （最高 94），這一項要 `忠誠 >= RND(7) + 87`（最低 87）。
	// 只有 87–93 同時滿足——設 95 會讓候選過濾先擋掉，看起來像
	// 「這個加項不存在」。
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+8)), uint16(95+i%6))
		if i%2 == 0 {
			o.SetWord(addr(base+uint32(i*72+14)), 1)
		}
	}
	for i := 0; i < 350; i++ {
		at := genBase + uint32(i*30)
		if o.Byte(addr(at+16)) == 0xFF {
			continue
		}
		o.SetByte(addr(at+16), uint8(87+i%7))
	}
	t.Log("人望全設 95–100、一半的勢力給玉璽、在職者的忠誠設在 87–93")

	type shot struct {
		slot, bar, resist, newLoyal, ret int
		loyalty, war, intel              int
		luckBar, luckAdd                 int
		zealBar, zealAdd                 int
		seal                             int
		theirPrestige                    int
	}
	var shots []shot
	var cur shot
	armed := false

	o.OnCall(addr(0x1dc0a), func(o *oracle.Oracle) {
		slot := uint32(o.Arg(0)) * 30
		cur = shot{
			slot:    int(slot) / 30,
			loyalty: int(int8(o.Byte(addr(genBase + slot + 16)))),
			intel:   int(o.Byte(addr(genBase + slot + 9))),
			war:     int(o.Byte(addr(genBase + slot + 10))),
			ret:     -2,
		}
		armed = true
	})
	o.OnCall(addr(0x1dcb8), func(o *oracle.Oracle) {
		if armed {
			cur.bar = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x1dced), func(o *oracle.Oracle) {
		if armed {
			cur.resist = int(int16(o.SI()))
		}
	})
	// 三個條件式加項各自的擲值與觸發與否。
	o.OnCall(addr(0x1dd27), func(o *oracle.Oracle) {
		if armed {
			cur.luckBar = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x1dd42), func(o *oracle.Oracle) {
		if armed {
			cur.theirPrestige = int(int16(o.Word(addr(
				uint32(o.ES())*16 + uint32(o.SI()) + 8))))
		}
	})
	o.OnCall(addr(0x1dd6d), func(o *oracle.Oracle) {
		if armed {
			cur.luckAdd = int(int16(o.AX())) + 1 // +1 分辨「沒觸發」
		}
	})
	o.OnCall(addr(0x1dd7f), func(o *oracle.Oracle) {
		if armed {
			cur.zealBar = int(int16(o.AX()))
		}
	})
	o.OnCall(addr(0x1dda3), func(o *oracle.Oracle) {
		if armed {
			cur.zealAdd = int(int16(o.AX())) + 1
		}
	})
	o.OnCall(addr(0x1ddc7), func(*oracle.Oracle) {
		if armed {
			cur.seal = 1
		}
	})
	o.OnCall(addr(0x1de1d), func(o *oracle.Oracle) {
		if armed {
			cur.newLoyal = int(int16(o.CX()))
		}
	})
	o.OnCall(addr(0x1de40), func(o *oracle.Oracle) {
		if armed {
			cur.ret = int(int16(o.AX()))
			shots = append(shots, cur)
			armed = false
		}
	})
	o.OnCall(addr(0x1dc35), func(o *oracle.Oracle) {
		if armed {
			cur.ret = -1
			shots = append(shots, cur)
			armed = false
		}
	})

	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })

	const settle = 40_000_000
	for m := 0; m < 4; m++ {
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

	t.Logf("四個月：亂數 %d 次（正對照）、挖角判定 %d 次", rnd, len(shots))
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if len(shots) == 0 {
		t.Fatal("四個月裡一次都沒進挖角的判定——君主在本郡那道門沒過")
	}

	bad, won := 0, 0
	for _, s := range shots {
		if s.ret == -1 {
			continue // 前置閘門擋掉（君主、或牽絆同勢力）
		}
		// 抵抗的固定部分：忠誠 ＋ 戰力÷10 ＋ 謀略÷10 ＋ 對方人望的加項。
		// 對方人望沒有單獨的掛點，用「原版算出來的抵抗減掉能算的部分」
		// 反推，再檢查它落在合理的範圍裡。
		fixed := s.loyalty + s.war/game.HeadhuntAbilityDiv +
			s.intel/game.HeadhuntAbilityDiv
		if s.resist < fixed {
			t.Errorf("槽 %d：抵抗 %d 比「忠誠＋戰力÷10＋謀略÷10」＝ %d 還小",
				s.slot, s.resist, fixed)
			bad++
			continue
		}
		if extra := s.resist - fixed; extra > (100-game.HeadhuntPrestigeFloor)/
			game.HeadhuntPrestigeDiv {
			t.Errorf("槽 %d：抵抗比固定部分多 %d，人望的加項最多 %d",
				s.slot, extra, (100-game.HeadhuntPrestigeFloor)/game.HeadhuntPrestigeDiv)
			bad++
			continue
		}
		// 三個條件式加項：擲值要落在值域裡，觸發與否要與條件相符。
		if s.luckBar < game.HeadhuntLuckFloor ||
			s.luckBar >= game.HeadhuntLuckFloor+game.HeadhuntLuckSpread {
			t.Errorf("槽 %d：RND(5)+90 給了 %d", s.slot, s.luckBar)
			bad++
		}
		if fired := s.luckAdd > 0; fired != (s.theirPrestige >= s.luckBar) {
			t.Errorf("槽 %d：對方人望 %d、門檻 %d → 原版%s，判準說%s",
				s.slot, s.theirPrestige, s.luckBar,
				map[bool]string{true: "加了", false: "沒加"}[fired],
				map[bool]string{true: "要加", false: "不加"}[s.theirPrestige >= s.luckBar])
			bad++
		}
		if s.luckAdd > 0 && s.luckAdd-1 >= max(s.theirPrestige/2, 1) {
			t.Errorf("槽 %d：RND(對方人望÷2 ＝ %d) 給了 %d",
				s.slot, s.theirPrestige/2, s.luckAdd-1)
			bad++
		}
		if s.zealBar < game.HeadhuntZealFloor ||
			s.zealBar >= game.HeadhuntZealFloor+game.HeadhuntZealSpread {
			t.Errorf("槽 %d：RND(7)+87 給了 %d", s.slot, s.zealBar)
			bad++
		}
		if fired := s.zealAdd > 0; fired != (s.loyalty >= s.zealBar) {
			t.Errorf("槽 %d：目標忠誠 %d、門檻 %d → 原版%s，判準說%s",
				s.slot, s.loyalty, s.zealBar,
				map[bool]string{true: "加了", false: "沒加"}[fired],
				map[bool]string{true: "要加", false: "不加"}[s.loyalty >= s.zealBar])
			bad++
		}
		if s.zealAdd > 0 && s.zealAdd-1 >= game.HeadhuntZealBonus {
			t.Errorf("槽 %d：RND(30) 給了 %d", s.slot, s.zealAdd-1)
			bad++
		}
		if s.ret == 0xFFFF || s.ret < 0 {
			continue // 失敗；成功那一側的忠誠不用比
		}
		won++
		// 新忠誠 ＝ (100 − 舊忠誠) ÷ 2 ＋ 我方人望 ÷ 2，夾到 0..100。
		half := (100 - s.loyalty) / 2
		if s.newLoyal < half || s.newLoyal > half+50 {
			t.Errorf("槽 %d：舊忠誠 %d → 新忠誠 %d，"+
				"應該是 %d ＋ 我方人望÷2（0–50）", s.slot, s.loyalty, s.newLoyal, half)
			bad++
			continue
		}
		if want := clamp100(s.newLoyal); want != s.ret {
			t.Errorf("槽 %d：算出 %d，回傳卻是 %d（應該夾到 100）",
				s.slot, s.newLoyal, s.ret)
			bad++
		}
	}
	luck, zeal, seals := 0, 0, 0
	for _, s := range shots {
		if s.luckAdd > 0 {
			luck++
		}
		if s.zealAdd > 0 {
			zeal++
		}
		if s.seal == 1 {
			seals++
		}
	}
	t.Logf("三個條件式加項各走到：對方人望 %d 次、目標忠誠 %d 次、玉璽 %d 次",
		luck, zeal, seals)
	t.Logf("%d 次判定（成功 %d 次），%d 項對不上", len(shots), won, bad)
	if luck == 0 || zeal == 0 || seals == 0 {
		t.Errorf("有加項沒被取樣到（%d／%d／%d）——盤面沒擺成功", luck, zeal, seals)
	}
}
