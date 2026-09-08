//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestZZBattleDaySweep 找出「在主戰場下一道命令」要送什麼鍵。
//
// **開機一次、紮完寨存快照，每個候選還原之後再送**——同一個問題連問
// 三次還要等三分半就該把迴圈變快（`~/.claude/CLAUDE.md` 的長工作紀律）。
// 直接重跑的話一組候選要三分半，這樣一組十秒。
//
// 判準不是畫面：攔 `0x113a4`（讀一個鍵的共用常式）記呼叫端，就知道
// 卡在哪一個提示；攔 `0x34f91` 看哪些鍵真的進得去；天數在
// 工作區的 `0x2100`。
func TestZZBattleDaySweep(t *testing.T) {
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
	driveIntoBattle(t, o, at, to)
	if dgroup == 0 {
		t.Fatal("沒有進到主戰場 0x2053c")
	}
	work := o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg})

	// 紮寨：`0` 紮下去，一支一支問（`docs/re/05` §7）。
	for step := 0; step < 8; step++ {
		o.Drain()
		o.PressScan("0")
		if err := o.Run(40_000_000); err != nil {
			t.Fatalf("紮寨停止：%v", err)
		}
		if o.Word(oracle.Addr{Seg: work, Off: uint16(battleUnitBase + unitCap)}) != 0 {
			break
		}
	}
	day := func() int {
		return int(o.Word(oracle.Addr{Seg: work, Off: 0x2100}))
	}
	t.Logf("紮完寨：天數 %d，工作區段 %#06x", day(), work)
	snap := o.Save()

	marks := []struct {
		at   uint32
		name string
	}{
		{0x21816, "戰場迴圈"}, {0x24ee1, "夾移動力"}, {0x24f8c, "統帥條件"},
		{0x250d4, "三十天"}, {0x27114, "算移動力上限"}, {0x2a224, "交戰結算"},
		{0x2eafc, "紮寨休息"},
	}
	hit := map[string]int{}
	for _, m := range marks {
		name := m.name
		o.OnCall(addr(m.at), func(o *oracle.Oracle) { hit[name]++ })
	}
	readers := map[uint32]int{}
	o.OnCall(addr(0x113a4), func(o *oracle.Oracle) { readers[o.Caller().Linear()]++ })
	// **命令提示讀的是 ASCII**：`0x27a40` 印選單 `DS:0x7f22`，
	// `0x27a63` 用 `1058:0e24` 讀一個鍵，回來 `sub $0x30; cmp $8; ja`
	// ——不在 `'0'..'8'` 就當不合法再問一次。攔它回來的那一刻看 AX，
	// 就知道我們送進去的鍵原版收到的是什麼。
	var cmdKeys []string
	o.OnCall(addr(0x27a68), func(o *oracle.Oracle) {
		cmdKeys = append(cmdKeys, fmt.Sprintf("%#04x", o.AX()))
	})
	var keys []string
	o.OnCall(addr(0x34f91), func(o *oracle.Oracle) {
		k := o.AX() & 0xff
		if k >= 0x20 && k < 0x7f {
			keys = append(keys, fmt.Sprintf("%q", rune(k)))
			return
		}
		keys = append(keys, fmt.Sprintf("%#02x", k))
	})

	// 主戰場的選單是 `1.移動 2.對戰 3.快戰 4.死戰 5.弓箭 6.策略
	// 7.查看 8.退兵 0.休息`（`DS:0x7f22`），提示是「XX 的命令(0-8)?」。
	// **`P:` 開頭的走 `int 21h` 的字元佇列**，其餘走硬體掃描碼——
	// 兩條一起餵會產生重複的字元，所以分開試。
	// 命令是**單一 ASCII**（`0x27aba` 的 `sub $0x30; cmp $8; ja`，
	// 跳表在 `cs:0x13a`），送 `0` 原版收到 `0x0030`，之後換成別的呼叫端
	// 在讀鍵——也就是 0（休息）確實被派出去了。剩下的問題是**一天要
	// 幾個鍵**：候選長度遞增，看天數在第幾個鍵跳。
	// 下完命令之後接的是 **Y/N 確認**（`0x1538c`：讀一個鍵，
	// `0x153a2` 寫 `'Y'`、`0x153aa` 寫 `'N'`），不是「請按任一鍵」——
	// 送 Enter 答不了它，所以先前怎麼送天數都不動。
	//
	// 休息（`0`）那條已經逐日對拍過了。這一輪要問的是**會打起來的
	// 那幾個命令**：每一個按下去之後接的是哪個提示、盤面動了什麼。
	// 判準是讀鍵的呼叫端與部隊記錄，不是畫面。
	// 量到的：`1`（移動）之後讀方向鍵（`0x27d3e`），
	// 六方向照 `DS:0x7f9e` 的 `4 5 6` 在上、`1 2 3` 在下。
	// 玩家那支在 (6,6)，`1|5` 走到 (6,5) 花 2 點、再一步到 (6,4) 花 3 點
	// ——與地形花費表 `DS:0x7c42` 對得上。守軍 0-2 在 (6,3)，
	// 走兩步就相鄰，接著 `2`（對戰）往上打。
	// **移動模式只有 Enter 離得開**（`0x27d63` 的 `cmpw $0xd`）：
	// 1–6 一律當方向，其餘的鍵忽略後繼續問。
	E := enterMark
	cands := []string{
		"1|5|5|" + E,
		"1|5|5|" + E + "|2",
		"1|5|5|" + E + "|2|5",
		"1|5|5|" + E + "|2|5|Y",
		"1|5|5|" + E + "|2|5|Y|Y",
		"1|5|5|" + E + "|2|5|" + E,
	}
	const settle = 40_000_000
	for ci, cand := range cands {
		o.Restore(snap)
		o.Drain()
		for k := range hit {
			delete(hit, k)
		}
		for k := range readers {
			delete(readers, k)
		}
		keys, cmdKeys = keys[:0], cmdKeys[:0]
		body := strings.TrimPrefix(cand, "P:")
		for _, seg := range strings.Split(body, "|") {
			k := strings.ReplaceAll(seg, enterMark, "\r")
			if strings.HasPrefix(cand, "P:") {
				o.Press(k)
			} else {
				o.PressScan(k)
			}
			if err := o.Run(settle); err != nil {
				t.Fatalf("候選 %q 停止：%v", cand, err)
			}
		}
		var route []string
		for _, m := range marks {
			if n := hit[m.name]; n > 0 {
				route = append(route, fmt.Sprintf("%s×%d", m.name, n))
			}
		}
		var who []string
		for a, n := range readers {
			who = append(who, fmt.Sprintf("%#07x×%d", a, n))
		}
		sort.Strings(who)
		var board strings.Builder
		for army := 0; army < battleArmies; army++ {
			for team := 0; team < battleTeams; team++ {
				rec := battleUnitBase + (army*battleUnitPer+team)*battleUnitSize
				w := func(off int) int {
					return int(o.Word(oracle.Addr{Seg: work, Off: uint16(rec + off)}))
				}
				if w(0) == 0xFFFF || w(unitLeaders) <= 0 {
					continue
				}
				fmt.Fprintf(&board, "[%d-%d 格 %d,%d 兵 %d 移 %d] ",
					army, team, w(unitCol), w(unitRow), w(unitSoldiers), w(unitMove))
			}
		}
		t.Logf("候選 %2d %-8q → 天數 %d；走到 %v；讀鍵的呼叫端 %v；"+
			"命令收到 %v；欄位收到 %v\n        盤面 %s",
			ci+1, cand, day(), route, who, cmdKeys, keys, board.String())
	}
}
