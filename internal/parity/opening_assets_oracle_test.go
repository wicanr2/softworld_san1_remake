//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

// 開場十格畫面各自載了哪些圖（`docs/formats/01` §4.1）。
//
// 三英那張圖在 `DATA1`／`DATA3` 裡逐張比過都不中，`DATA0.NAM` 列的
// 31 個名字（`ENDO*`／`UPR*`／`REC*`）在三個容器裡也都找不到。
// 與其繼續猜它在哪，**問原版自己**：載圖的常式 `0x36c9:0x2b0`
// 第一個參數就是檔名的 far 指標（`0x36f4b` 那道 `0 <= 槽 < 0x100`
// 是第三個參數），攔下來就知道它要的是什麼名字；`TraceFiles` 同時
// 記下它去哪個檔案讀。
//
// 這一支不判對錯，只把兩份清單印出來——**它回答的是「資料在哪」，
// 不是「畫得對不對」**。

// TestZZOpeningAssetNames 列出開場每一步載進來的圖與讀過的檔案。
func TestZZOpeningAssetNames(t *testing.T) {
	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	o.TraceFiles()

	// 載圖常式的入口：Arg(0)/Arg(1) 是檔名的 offset/segment。
	var names []string
	step := 0
	o.OnCall(addr(0x36c9*16+0x2b0), func(o *oracle.Oracle) {
		lin := uint32(o.Arg(1))*16 + uint32(o.Arg(0))
		var b strings.Builder
		for i := uint32(0); i < 16; i++ {
			c := o.Byte(addr(lin + i))
			if c == 0 {
				break
			}
			b.WriteByte(c)
		}
		names = append(names, fmt.Sprintf("%d\t%s\t槽 %d", step, b.String(), int16(o.Arg(2))))
	})

	o.TypeBoth("122")
	seen := 0
	for step = 0; step < 10; step++ {
		if err := o.Run(50_000_000); err != nil {
			t.Fatalf("第 %d 步停止：%v", step, err)
		}
		dumpScreen(t, o, fmt.Sprintf("open-%02d", step))
		if step == 4 {
			o.TypeBoth("\r")
		}
		if len(names) > seen {
			t.Logf("第 %d 步載的圖：\n\t%s", step,
				strings.Join(names[seen:], "\n\t"))
			seen = len(names)
		}
	}

	// 逐筆 (seek, read) 配對，並且把位移對回容器裡的項目名。
	entries := map[string]map[int64]string{}
	for _, slot := range []string{"DATA1", "DATA2", "DATA3"} {
		c := openContainer(t, filepath.Join(root, slot))
		m := map[int64]string{}
		for i := 0; i < c.Len(); i++ {
			e := c.Entry(i)
			m[int64(e.Start)] = fmt.Sprintf("%s（%d bytes）", e.Name, e.Size())
		}
		entries[slot+".GRP"] = m
	}
	last := map[uint16]int64{}
	for _, op := range o.FileOps() {
		switch op.Op {
		case "open":
			t.Logf("[%d] open %s（%d bytes）→ handle %d",
				op.Step, op.Name, op.Arg, op.Handle)
		case "seek":
			last[op.Handle] = op.Pos // seek 後的位置（舊欄位 `Result`）
		case "read":
			at, ok := last[op.Handle]
			if !ok {
				continue
			}
			name := ""
			if m := entries[strings.ToUpper(op.Name)]; m != nil {
				name = m[at]
			}
			t.Logf("[%d] %s seek %d 讀 %d → %s",
				op.Step, op.Name, at, op.Arg, name)
			delete(last, op.Handle)
		}
	}
	if len(names) == 0 {
		t.Fatal("一次都沒攔到載圖——位址或參數版面不對")
	}
}
