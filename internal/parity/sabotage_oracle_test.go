//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 「策反人民」走玩家的完整選單跑一次（`docs/mechanics/30-diplomacy` 的缺口）。
//
// 五種戰略層謀略裡只有偽書使疑對拍過（`TestAIPlotMatchesTheOriginal`），
// 而那一支走的是**電腦諸侯**那條鏈（`0xe68e`）。玩家那四支要從主選單
// 第 8 項進去才發得出來，這支補的是其中不會打起來的那一種——
// 驅虎吞狼、遠交近攻、聯合出兵三支的結局都是發動一場戰役，
// 要再接一整串整編的按鍵。
//
// 效果是對目標郡下的五刀（`0x2d6e0`，`internal/game.Sabotage`）：
//
//	民眾忠誠 −= RND(魅力 ÷ 10)
//	洪水率   += RND(魅力 ÷ 5)
//	土地價值 −= RND(魅力 ÷ 12)
//	米 −= 米 × 100 ÷ (RND(魅力) + 300)
//	金 −= 金 × 100 ÷ (RND(魅力) + 500)
//
// 判準與偽書使疑同一種：**存在一個合法的擲值算得出原版寫回去的那個數**。
// 擲值的值域小，直接列舉。
func TestSabotageRunsLive(t *testing.T) {
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
	at, to := stageABattle(t, o, base)

	staBase := base + uint32(state.MasterTableSize)
	const staRec = state.PrefectureRecordSize
	const (
		staGold    = 18 // u16
		staRice    = 20 // u16
		staLoyalty = 26 // u8 民眾忠誠
		staLand    = 27 // u8 土地價值
		staFlood   = 28 // u8 洪水率
	)

	// **軍師那道門要自己擺**（`0x2c225`）：沒有軍師連選單都進不去，
	// 而且軍師本人要在這個郡。軍師欄直接指向君主本人——君主一定在郡裡。
	//
	// 玩家的勢力是 `stageABattle` 擺的那一個；從君主槽反查勢力欄。
	var mine uint16
	for i := 0; i < state.MasterTableSize/72; i++ {
		if int(o.Word(addr(staBase+uint32(at)*staRec+30))) == i {
			mine = uint16(i)
		}
	}
	lord := o.Word(addr(base + uint32(mine)*72 + 2))
	o.SetWord(addr(base+uint32(mine)*72+6), lord) // 軍師 ← 君主
	t.Logf("勢力 %d 的軍師欄指向君主槽 %d（坐鎮郡 %d）", mine, lord, at)

	// **五刀要量得到就得有得扣。** 目標郡的五個欄位直接寫進去
	// （`CLAUDE.md`：對拍的盤面自己擺，不靠原版的 RND()）。
	tgt := staBase + uint32(to)*staRec
	o.SetWord(addr(tgt+staGold), 9000)
	o.SetWord(addr(tgt+staRice), 20000)
	o.SetByte(addr(tgt+staLoyalty), 90)
	o.SetByte(addr(tgt+staLand), 90)
	o.SetByte(addr(tgt+staFlood), 10)

	type snap struct{ gold, rice, loyal, land, flood int }
	read := func() snap {
		return snap{
			gold:  int(o.Word(addr(tgt + staGold))),
			rice:  int(o.Word(addr(tgt + staRice))),
			loyal: int(o.Byte(addr(tgt + staLoyalty))),
			land:  int(o.Byte(addr(tgt + staLand))),
			flood: int(o.Byte(addr(tgt + staFlood))),
		}
	}

	// 逐關計數，失敗時看得出停在哪一關。
	menuIn := 0
	o.OnCall(addr(0x2c2ee), func(*oracle.Oracle) { menuIn++ }) // 分派器
	inIncite := 0
	o.OnCall(addr(0x2d34c), func(*oracle.Oracle) { inIncite++ }) // 策反人民
	var charm, hitTarget, calls int
	var before snap
	o.OnCall(addr(0x2d6e0), func(o *oracle.Oracle) {
		hitTarget, charm = int(o.Arg(0)), int(o.Arg(1))
		before = read()
		calls++
	})

	// 主選單第 8 項是謀略，子選單第 4 項是策反人民
	// （`docs/mechanics/10-strategy` §2、`docs/re/07`）。
	// **每一個提示都要 Enter**，選單也一樣（`driveIntoBattle` 同一組慣例）。
	E := enterMark
	spell := func(n int) string {
		out := ""
		for _, ch := range fmt.Sprintf("%d", n) {
			out += string(ch) + "|"
		}
		return out + E
	}
	keys := envOr("SAN1_PLOTKEY", strings.Join([]string{
		"8|" + E, // 謀略
		"4|" + E, // 策反人民
		spell(to),
		"1|" + E, // 細作：名單第一位（魅力最高）
		"Y",
	}, "|"))
	for i, seg := range strings.Split(keys, "|") {
		o.Drain()
		o.PressScan(strings.ReplaceAll(seg, enterMark, "\r"))
		if err := o.Run(60_000_000); err != nil {
			t.Fatalf("送第 %d 段（%q）時停止：%v", i+1, seg, err)
		}
	}
	if err := o.Run(60_000_000); err != nil {
		t.Fatalf("收尾停止：%v", err)
	}
	t.Logf("分派器進去 %d 次、策反人民進去 %d 次、效果常式跑了 %d 次",
		menuIn, inIncite, calls)
	if calls == 0 {
		t.Fatalf("效果常式一次都沒跑（分派器 %d／策反人民 %d）"+
			"——按鍵序列沒走到底，用 SAN1_PLOTKEY 換一組再看",
			menuIn, inIncite)
	}
	if hitTarget != to {
		t.Fatalf("細作去的是郡 %d，不是擺好的郡 %d", hitTarget, to)
	}
	now := read()
	t.Logf("郡 %d 魅力 %d：民忠 %d→%d 洪水 %d→%d 地力 %d→%d 米 %d→%d 金 %d→%d",
		to, charm, before.loyal, now.loyal, before.flood, now.flood,
		before.land, now.land, before.rice, now.rice, before.gold, now.gold)

	// 五刀逐條列舉合法的擲值。
	checked, bad := 0, 0
	exists := func(name string, n int, want int, f func(r int) int) {
		checked++
		if n < 1 {
			n = 1
		}
		for r := 0; r < n; r++ {
			if f(r) == want {
				return
			}
		}
		bad++
		t.Errorf("%s：原版給 %d，RND(%d) 的 %d 種擲值一個都算不出來", name, want, n, n)
	}
	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		if v > 100 {
			return 100
		}
		return v
	}
	exists("民眾忠誠", charm/game.SabotageLoyaltyDiv, now.loyal,
		func(r int) int { return clamp(before.loyal - r) })
	exists("洪水率", charm/game.SabotageFloodDiv, now.flood,
		func(r int) int { return clamp(before.flood + r) })
	exists("土地價值", charm/game.SabotageLandDiv, now.land,
		func(r int) int { return clamp(before.land - r) })
	exists("米", charm, now.rice, func(r int) int {
		return before.rice - before.rice*game.SabotageScale/(r+game.SabotageRiceBase)
	})
	exists("金", charm, now.gold, func(r int) int {
		return before.gold - before.gold*game.SabotageScale/(r+game.SabotageGoldBase)
	})
	t.Logf("策反人民的五刀比了 %d 條，對不上 %d 條", checked, bad)
}
