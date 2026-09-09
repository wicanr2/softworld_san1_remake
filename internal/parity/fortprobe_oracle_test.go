//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestZZFortPromptsAsCaoCao 問原版：在劇本 1 曹操的郡 11 按下建築關寨之後，
// 它到底依序問了什麼。
//
// 板 A（南海）那道兩邊都因為謀略不足 80 而拒絕，走不到後面；板 B 的曹操
// 謀略 95，三道門的前兩道都過得去，但實測**原版連金都沒扣**——所以卡在
// 第三道（位置限山丘／平原／樹林且不是通道格）或是游標的起始格不能蓋。
//
// 只記「哪一支常式、範圍多少、誰呼叫的」：範圍是原版自己傳的參數，
// 呼叫者的回返位址分得出這一次讀鍵是數字欄位在收字元、Y/N 判定自己叫的、
// 還是真正的請按任一鍵（`docs/re/03` §1.5）。
func TestZZFortPromptsAsCaoCao(t *testing.T) {
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
	base := bootToNewGame(t, o, caoCaoPick, seedMas)

	work := o.ES()
	at := int(o.Word(oracle.Addr{Seg: work, Off: curPrefOff}))
	rec := base + uint32(state.MasterTableSize+at*state.PrefectureRecordSize)
	// 條件要和 `playercmd` 那組一樣：錢糧墊到 9000，免得第一道門
	//（費用 ＝ 當月物價 × 100）先把它擋掉。
	o.SetWord(addr(rec+18), 9000)
	o.SetWord(addr(rec+20), 9000)
	t.Logf("郡 %d：金 %d、關寨 %d 座、物價 %d", at,
		o.Word(addr(rec+18)), o.Byte(addr(rec+25)), o.Byte(addr(rec+29)))

	// 進到建築關寨本體幾次（`fortProbe` 是算花費那一段）。
	body := 0
	o.OnCall(addr(fortProbe), func(*oracle.Oracle) { body++ })

	type ask struct {
		kind   string
		lo, hi int
		from   uint32
	}
	var asks []ask
	note := func(kind string, lo, hi int) {
		c := o.Caller()
		asks = append(asks, ask{kind, lo, hi,
			uint32(c.Seg)*16 + uint32(c.Off)})
	}
	o.OnCall(addr(0x33d8*16+0x115e), func(o *oracle.Oracle) {
		note("數字", int(o.Arg(0)), int(o.Arg(1)))
	})
	o.OnCall(addr(0x1538c), func(*oracle.Oracle) { note("按鍵", 0, 0) })
	o.OnCall(addr(0x1058*16+0xe24), func(*oracle.Oracle) { note("字元", 0, 0) })

	const settle = 40_000_000
	// 游標畫面是「數字鍵選方向 4 5 6 / 1 2 3，0 才是蓋下去」（`DS:0x70cc`）。
	// 先照既有序列走，再多按幾次方向鍵看看是不是起始格不能蓋。
	for i, k := range []string{"4\r", "3\r", "1\r", "0", "2", "0", "3", "0", "N"} {
		before := len(asks)
		o.Drain()
		o.PressScan(k)
		if err := o.Run(settle); err != nil {
			t.Fatalf("送 %q 時停止：%v", k, err)
		}
		for _, a := range asks[before:] {
			t.Logf("  第 %d 步送 %-4q → 本體 %d 次、金 %d、關寨 %d；"+
				"%s(%d-%d) 誰要的 %#07x", i+1, k, body,
				o.Word(addr(rec+18)), o.Byte(addr(rec+25)),
				a.kind, a.lo, a.hi, a.from)
		}
	}
	dumpScreen(t, o, "fort-prompts")
}
