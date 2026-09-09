//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// `DS:0x80d4` 那張布林表的內容與用途（`docs/re/05` §2.3）。
//
// 兩個使用處（`0x28a5d` 與 `0x2999a`）長得一樣：那一格有部隊、佔位除以 20
// 與自己不同陣營、**而且地形碼查 `DS:0x80d4` 非零**，才往下走。旗標為 0
// 的是碼 1（大山）、5（城池）、6（關寨）。
//
// 兩處各自的外層常式是 `0x28944` 與 `0x2985c`，開頭一模一樣：拿
// `(軍力, 隊伍)` 算出部隊記錄再掃地圖。
//
// ⚠ **位址相鄰只是提示不是證據。** `0x28944` 緊接在弓箭的讀鍵
// `0x2891f` 之後，看起來就是弓箭的目標掃描——實跑按下弓箭之後它
// **一次都沒跑**。真正會跑的是 `0x2985c`，呼叫端在 `0x2907e`，
// 那是戰術層電腦部隊的行動選擇鏈（`docs/re/05` §12）。
func TestTerrainFlagTableAndTargetScan(t *testing.T) {
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

	var dgroup uint16
	o.OnCall(addr(0x2053c), func(o *oracle.Oracle) {
		if dgroup == 0 {
			dgroup = o.DSReg()
		}
	})
	cmdReads := 0
	o.OnCall(addr(0x27a68), func(*oracle.Oracle) { cmdReads++ })
	type call struct{ army, team int }
	var scanA, scanB []call
	o.OnCall(addr(0x28944), func(o *oracle.Oracle) {
		scanA = append(scanA, call{int(o.Arg(0)), int(o.Arg(1))})
	})
	o.OnCall(addr(0x2985c), func(o *oracle.Oracle) {
		scanB = append(scanB, call{int(o.Arg(0)), int(o.Arg(1))})
	})
	// 旗標那一關過了沒：`0x28a64` 是 `0x28a5d` 的 je 沒跳時的落點。
	passed := 0
	o.OnCall(addr(0x28a64), func(*oracle.Oracle) { passed++ })

	driveIntoBattle(t, o, at, to)
	if dgroup == 0 {
		t.Fatal("沒有進到主戰場")
	}
	work := o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg})
	w16 := func(off int) int {
		return int(o.Word(oracle.Addr{Seg: work, Off: uint16(off)}))
	}
	// `DS:0x80d4` 整張表印出來，順帶釘住它的內容。
	var flags []int
	for i := 0; i < 10; i++ {
		flags = append(flags, int(o.Word(oracle.Addr{
			Seg: dgroup, Off: uint16(0x80d4 + i*2)})))
	}
	t.Logf("DS:0x80d4 的前十格（地形碼 0–9）：%v", flags)
	for _, code := range []int{1, 5, 6} {
		if flags[code] != 0 {
			t.Errorf("地形碼 %d 的旗標是 %d，先前記的是 0", code, flags[code])
		}
	}
	for _, code := range []int{2, 3, 4, 7, 8, 9} {
		if flags[code] == 0 {
			t.Errorf("地形碼 %d 的旗標是 0，先前記的是非零", code)
		}
	}

	for step := 1; step <= 12 && cmdReads == 0; step++ {
		o.Drain()
		o.PressScan("0")
		if err := o.Run(60_000_000); err != nil {
			t.Fatalf("紮寨第 %d 步停止：%v", step, err)
		}
	}
	o.Drain()
	o.PressScan("Y")
	if err := o.Run(60_000_000); err != nil {
		t.Fatalf("確認第 1 天停止：%v", err)
	}
	t.Logf("紮完寨、推到第 %d 天；此時 0x28944 跑過 %d 次、0x2985c 跑過 %d 次",
		w16(0x2100), len(scanA), len(scanB))

	// **按 `5` 之後還要送方向才會走到掃描**，而那一場玩家那支旁邊未必
	// 有射得到的目標。改用確定會跑的那一支：`0x2985c` 在日循環裡跑過，
	// 抓它的呼叫端就知道是什麼動作——呼叫端落在遊戲自己的碼段裡，
	// 對得回 `docs/re/05` §8 與 `70-ai` 已經解過的常式。
	callers := map[string]int{}
	o.OnCall(addr(0x2985c), func(o *oracle.Oracle) {
		c := o.Caller()
		callers[fmt.Sprintf("%#06x:%#06x", c.Seg, c.Off)]++
	})
	callersA := map[string]int{}
	o.OnCall(addr(0x28944), func(o *oracle.Oracle) {
		c := o.Caller()
		callersA[fmt.Sprintf("%#06x:%#06x", c.Seg, c.Off)]++
	})
	for d := 1; d <= 4; d++ {
		for _, k := range []string{"0", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(100_000_000); err != nil {
				t.Fatalf("第 %d 天送 %q 停止：%v", d, k, err)
			}
		}
	}
	t.Logf("跑完四天：0x2985c 共 %d 次，呼叫端 %v", len(scanB), callers)
	t.Logf("           0x28944 共 %d 次，呼叫端 %v", len(scanA), callersA)
	t.Logf("旗標那一關（0x28a64）過了 %d 次", passed)
	if len(callers) == 0 && len(callersA) == 0 {
		t.Fatal("兩支都沒抓到呼叫端")
	}
}
