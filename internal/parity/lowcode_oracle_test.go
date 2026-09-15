//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestZZDumpLowCode 把兩版**開到遊戲之後** `0x400`–`0xb000` 那一段倒出來。
//
// 主程式的碼段 dump 從 `0xb000` 起（`docs/re/05`），但行為觸發開機用到的
// 幾個路標在它下面：防拷模組 `0x3eb:0`（密碼欄位 `03EB:02C2`）、BIOS 鍵盤
// 輪詢 `0583:3792`、標題頁的清鍵 `0AD0:0104`。要把這些路標對到加強版
// （Issue #20）得先有兩版同一段的位元組。
func TestZZDumpLowCode(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, _, _ := sc0.Tables()
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	bootToGame(t, o, mas)
	dumpImage(t, o, 0x000400, 0x00b000, "low-base")
	o.Close()

	proot := plusRoot(t)
	p, err := oracle.Load(filepath.Join(proot, "ASV.EXE"), proot)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	bootLikePlus(t, p)
	dumpImage(t, p, 0x000400, 0x00b000, "low-plus")
	dumpImage(t, p, 0x00b000, 0x050000, "code-plus")
}
