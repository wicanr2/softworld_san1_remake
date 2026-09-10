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

// `0x281ee` 是誰、什麼時候被叫、對哪些軍團做？
//
// 前一輪（`docs/re/05` 末節）用攔寫入問出「郡的金／米／兵士在月底結算之後
// 被戰役的碼扣掉」，寫入者是 `0x282df`／`0x282f8`／`0x28553`，函式入口
// `0x281ee`。三個未知還沒解：
//
//  1. 呼叫端在月流程的哪一格
//  2. 哪些軍團會被處理
//  3. 條件是什麼
//
// 這一支同時攔函式本身與月流程的路標，把「誰在什麼時候叫了它」排成時間軸。
//
// ⚠ **返回位址要逐支量，不要從第一支外推**（`CONTEXT.md` 的 `[HARD]`）：
// 同一支常式可以有好幾個呼叫端，而它們在流程裡的位置不同。
func TestZZSortieSupplyTiming(t *testing.T) {
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
	seq := strings.Split(envOr("SAN1_TURNKEY", "4\r|4\r|Y"), "|")
	total := state.MasterTableSize + state.PrefectureTableSize + state.GeneralTableSize
	driveToMonthStart(t, o, seq, base, total, 0x13579BDF)

	// 月流程的路標（`docs/re/06` §9.5、`docs/re/03` §1.4）。
	marks := []struct {
		at   uint32
		name string
	}{
		{0x1581c, "月底結算入口"},
		{0x1e394, "重算所屬"},
		{0x17364, "開月"},
		{0x159de, "月份推進"},
		{0x15c1e, "四季分派器"},
		{0x16e70, "冬季事件"},
		{0x1712a, "進貢"},
		{0x1746e, "郡回合入口"},
		{0x174ec, "呼叫郡的分派器"},
		{0x174f3, "郡回合結束"},
		{0xec1b, "分派表：出兵"},
		{0x188ea, "發動戰役的入口"},
		{0x18bb9, "呼叫戰鬥"},
		{0x20471, "電腦對電腦的分岔"},
		{0x20a30, "整編"},
	}
	var line []string
	seen := map[string]int{}
	for _, m := range marks {
		name := m.name
		o.OnCall(addr(m.at), func(*oracle.Oracle) {
			seen[name]++
			if len(line) < 60 {
				line = append(line, name)
			}
		})
	}

	type call struct {
		from, to   int
		gold, rice int
		caller     uint32
		after      string
	}
	var calls []call
	// ⚠ **`o.ToIDA()` 回的是 IDA 位址，不是映像位移。** 兩者差 `0xEF00`
	// （量法：寫入者 IP `1538:405F` 的線性位址是 `0x193DF`，而 `ToIDA`
	// 給 `0x282DF`）。拿 IDA 位址去 `objdump --adjust-vma=0xb000` 會讀到
	// **完全不相干的碼**，而那段碼一樣讀得通——上一輪就這樣把「搬運金米」
	// 誤讀成「部隊記錄的迴圈」。攔截點一律用線性位址。
	// **攔函式入口 `0x1938a`，不是中間。** `o.Arg(n)` 讀的是
	// `[SS:SP + 4 + n*2]`——那假設「剛進入函式、bp 還沒 push」。攔在函式
	// 中間時 SP 早就變了，讀到的是垃圾（實測「郡 1864 → 2859」）。
	//
	// 這一支做兩件事（`docs/re/05`）：迴圈把名單裡每一位將領的所在郡
	//（人物 offset 19）改成目標郡，然後把金米從來源郡搬到目標郡
	// （州郡 offset 18／20，各有「不能變負就夾 0」）。
	o.OnCall(addr(0x1938a), func(o *oracle.Oracle) {
		last := "（還沒經過任何路標）"
		if len(line) > 0 {
			last = line[len(line)-1]
		}
		calls = append(calls, call{
			from: int(int16(o.Arg(0))), to: int(int16(o.Arg(1))),
			gold: int(int16(o.Arg(2))), rice: int(int16(o.Arg(3))),
			caller: o.Caller().Linear(), after: last,
		})
		if len(line) < 60 {
			line = append(line, fmt.Sprintf("★搬運(郡 %d → %d)",
				int(int16(o.Arg(0))), int(int16(o.Arg(1)))))
		}
	})

	// 跑到第一個郡的回合入口——問題發生在那之前。
	got := false
	o.OnCall(addr(0x1746e), func(*oracle.Oracle) { got = true })
	for i := 0; i < 12 && !got; i++ {
		if err := o.Run(40_000_000); err != nil {
			t.Fatalf("跑到第一個郡的回合入口時：%v", err)
		}
	}

	t.Logf("月流程的時間軸（結算 → 第一個郡）：")
	for i, s := range line {
		t.Logf("  %2d. %s", i+1, s)
	}
	t.Logf("路標各進去幾次：")
	for _, m := range marks {
		if n := seen[m.name]; n > 0 {
			t.Logf("  %-16s %d 次", m.name, n)
		}
	}
	t.Logf("搬運（`0x1938a`）被叫了 %d 次：", len(calls))
	for i, c := range calls {
		if i >= 20 {
			t.Logf("  …（還有 %d 次）", len(calls)-i)
			break
		}
		t.Logf("  郡 %2d → %2d　金 %5d　米 %5d　呼叫端 %05X　接在「%s」之後",
			c.from, c.to, c.gold, c.rice, c.caller, c.after)
	}
}
