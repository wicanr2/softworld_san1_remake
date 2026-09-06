//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 找出原版把三張表放在記憶體的哪裡。
//
// 對拍要的是「同一個局面下原版怎麼決定」，所以局面得由對拍這一方擺出來
// ——靠原版自己的亂數把局面湊出來的話，比出來的差異分不出是「決策不同」
// 還是「盤面不同」。要擺盤面就得先知道盤面在哪。
//
// 找法是拿劇本檔的位元組去記憶體裡搜：原版把 `BASESTA.001` 整段讀進來，
// 那一段在記憶體裡與檔案內容相同。

// startDemo 讓原版跑到電腦自動示範模式。
//
//	122   無音樂／EGA／硬碟
//	\r    跳過標題
//	1     開始新遊戲
//	1     中平六年（劇本 001）
//	0     零人玩 ＝ 電腦自動示範
func startDemo(t *testing.T, budget uint64) *oracle.Oracle {
	t.Helper()
	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatalf("載入原版：%v", err)
	}
	o.Type("122\r110")
	if err := o.Run(budget); err != nil {
		t.Logf("原版停止：%v", err)
	}
	return o
}

func TestZZTables(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, sta, gen := sc.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	o.Press("122")
	send := map[int]string{4: "\r", 17: "1", 23: "1"}
	for i := 0; i < 25; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Fatalf("停止：%v", err)
		}
		if k, ok := send[i]; ok {
			o.Press(k)
		}
	}
	at := o.Search(mas[:48])
	if len(at) != 1 {
		t.Fatalf("諸侯表找到 %d 個位置 %v", len(at), at)
	}
	base := at[0]
	t.Logf("三張表的基底 %#x（%d）", base, base)

	addr := func(lin uint32) oracle.Addr {
		return oracle.Addr{Seg: uint16(lin >> 4), Off: uint16(lin & 0xF)}
	}
	off := uint32(0)
	for _, x := range []struct {
		name string
		b    []byte
	}{{"BASEMAS", mas}, {"BASESTA", sta}, {"BASEGEN", gen}} {
		got := o.Bytes(addr(base+off), len(x.b))
		diff, first := 0, -1
		for k := range x.b {
			if got[k] != x.b[k] {
				diff++
				if first < 0 {
					first = k
				}
			}
		}
		t.Logf("%s：%#x 起 %d 個位元組，與檔案差 %d 個（第一個差在 %d）",
			x.name, base+off, len(x.b), diff, first)
		if first >= 0 {
			lo := first - 8
			if lo < 0 {
				lo = 0
			}
			hi := first + 16
			if hi > len(x.b) {
				hi = len(x.b)
			}
			t.Logf("    檔案 %X", x.b[lo:hi])
			t.Logf("    記憶 %X", got[lo:hi])
		}
		off += uint32(len(x.b))
	}
}

func TestZZFindTables(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, sta, gen := sc.Tables()

	o := startDemo(t, 400_000_000)
	defer o.Close()
	t.Logf("跑了 %d 道指令，開過 %d 個檔", o.Steps(), len(o.Opened()))
	t.Logf("主控台尾巴：%q", tail(o.Console(), 200))

	for _, x := range []struct {
		name string
		b    []byte
	}{{"BASEMAS", mas}, {"BASESTA", sta}, {"BASEGEN", gen}} {
		// 用前 64 個位元組當指紋，避開整表比對的成本。
		hits := o.Search(x.b[:64])
		t.Logf("%s（%d 位元組）：%d 個位置 %v", x.name, len(x.b), len(hits), hits)
		for _, at := range hits {
			same := 0
			for i, v := range x.b {
				if o.Byte(oracle.Addr{Seg: uint16(at >> 4), Off: uint16(at & 0xF)}) == v {
					_ = i
				}
				_ = v
				_ = same
			}
		}
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
