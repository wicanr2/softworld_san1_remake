//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 這一支回答：**原版有沒有自己設 PIT 通道 0 的分頻？**
//
// 2026-09-10 換 base 到 dosgolem `main` 之後，四支對拍紅了（月度、玩家命令、
// 四季事件、策反）。差異的形狀是「每月會動的欄位差一點點、靜態屬性全對」
// ——不是流程斷了，是兩邊走的量不同。
//
// main 上有一筆 `287ed67 machine：認 PIT 通道 0 的分頻，IRQ0 間隔照比例走`，
// 而那條計算是**不需要旗標的**（`machine.RecalcIRQ0`）：
//
//	every := base * PITDiv / PITDefaultDivisor
//
// 所以原版只要寫過 `0x40`／`0x43`，IRQ0 的間隔就會從預設的 165,000 變成
// 別的值。對拍給的是**指令預算**（`o.Run(N)`），IRQ0 頻率一變，原版在同樣
// 的預算內做完的事就不一樣——而畫面照跑、不報錯。
//
// ⚠ **這一支只量事實，不下結論。** 「原版設了 PIT」與「那就是四支變紅的
// 原因」是兩件事；後者要拿改回舊行為之後對拍變綠來證明。
func TestZZPITProbe(t *testing.T) {
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

	t.Logf("開機前：分頻 %d、設過了嗎 %v、IRQ0 每 %d 道指令、%.2f Hz",
		o.TimerDivisor(), o.TimerProgrammed(), o.StepsPerTick(), o.TimerHz())

	bootToGame(t, o, seedMas)

	// PIT 的寫入序列：`0x43` 是控制字、`0x40` 是通道 0 的分頻（先低後高）。
	var pit []oracle.PortWrite
	for _, w := range o.PortWrites() {
		if w.Port == 0x40 || w.Port == 0x43 {
			pit = append(pit, w)
		}
	}
	t.Logf("原版對 PIT（0x40／0x43）寫了 %d 次", len(pit))
	for i, w := range pit {
		if i >= 24 {
			t.Logf("  …（還有 %d 次）", len(pit)-i)
			break
		}
		t.Logf("  [%d] 埠 %02X ← %02X", w.Step, w.Port, w.Val)
	}

	t.Logf("到主畫面時：分頻 %d、設過了嗎 %v、IRQ0 每 %d 道指令、%.2f Hz",
		o.TimerDivisor(), o.TimerProgrammed(), o.StepsPerTick(), o.TimerHz())

	// **這一格是判準**：預設是 165,000（`machine.DefaultIRQ0Every`）。
	// 不等於它就表示 IRQ0 的節奏被 PIT 分頻改過，而對拍的指令預算
	// 是照舊節奏調的。
	const wasEvery = 165_000
	if got := o.StepsPerTick(); got != wasEvery {
		t.Logf("⚠ IRQ0 間隔是 %d，不是舊 base 的 %d——**節奏變了**", got, wasEvery)
	} else {
		t.Logf("IRQ0 間隔仍是 %d，與舊 base 相同", got)
	}
}
