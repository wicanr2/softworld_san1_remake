//go:build oracle

package parity

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

// 加強版的輸入路標（Issue #20）。原版的常數在 boot_behavior_oracle_test.go；
// 這幾個是拿原版的指令形狀序列在加強版碼段裡比出來的（唯一命中），
// 其餘（呼叫端）要用這支探針從執行期抓。
const (
	plusNumInputFn = 0x317e4 // 原版 33d8:115e
	plusKeyInputFn = 0x144ba // 原版 0x1538c
	plusMonthEndFn = 0x14878 // 原版 0x1581c
	plusTurnEntry  = 0x16256 // 原版 0x1746e
)

// TestZZPlusBootProbe 用 `bootLikePlus` 的固定預算配方走加強版，沿路記
// 數字輸入與 Y/N 輸入的呼叫端、參數、當時的畫面雜湊——這就是原版
// `docs/spec/015` §3 那張表的加強版。輸出是收據，不是驗收。
func TestZZPlusBootProbe(t *testing.T) {
	root := plusRoot(t)
	o, err := oracle.Load(filepath.Join(root, "ASV.EXE"), root)
	if err != nil {
		t.Fatalf("加強版載不進來：%v", err)
	}
	defer o.Close()

	type ask struct {
		step   uint64
		caller uint32
		lo, hi int
		kind   string
	}
	var asks []ask
	o.OnCall(addr(plusNumInputFn), func(o *oracle.Oracle) {
		asks = append(asks, ask{o.Steps(), o.Caller().Linear(), int(int16(o.Arg(0))), int(int16(o.Arg(1))), "num"})
	})
	o.OnCall(addr(plusKeyInputFn), func(o *oracle.Oracle) {
		asks = append(asks, ask{o.Steps(), o.Caller().Linear(), 0, 0, "key"})
	})
	months := 0
	o.OnCall(addr(plusMonthEndFn), func(*oracle.Oracle) { months++ })
	turns := 0
	o.OnCall(addr(plusTurnEntry), func(*oracle.Oracle) { turns++ })

	shot := func(name string) {
		h := sha256.Sum256(o.IndexedEGASize(scrW, scrH))
		t.Logf("%s：步 %d、畫面 %x", name, o.Steps(), h[:8])
		dumpScreen(t, o, "plusboot-"+name)
	}
	o.Type(envOr("SAN1_PLUSKEY", "122"))
	if err := o.Run(200_000_000); err != nil {
		t.Logf("開機段停止：%v", err)
	}
	shot("00-200M")
	send := map[int]string{4: "\r", 17: "2", 23: "1"}
	for i := 0; i < 27; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Logf("第 %d 段停止：%v", i, err)
			break
		}
		if k, ok := send[i]; ok {
			shot(fmt.Sprintf("%02d-before-%q", i+1, k))
			// 送鍵前先看它在哪裡等：等待迴圈所在的函式就是這一層的輸入常式
			//（原版 `1058:0E24`–`0E7F`）。取樣 200 次，印最常見的幾個 IP。
			hist := map[uint32]int{}
			for j := 0; j < 200; j++ {
				if err := o.Run(5_000); err != nil {
					break
				}
				hist[o.IP().Linear()]++
			}
			type kv struct {
				a uint32
				n int
			}
			var top []kv
			for a, n := range hist {
				top = append(top, kv{a, n})
			}
			for x := 0; x < len(top); x++ {
				for y := x + 1; y < len(top); y++ {
					if top[y].n > top[x].n {
						top[x], top[y] = top[y], top[x]
					}
				}
			}
			if len(top) > 6 {
				top = top[:6]
			}
			t.Logf("  等待中的 IP 取樣：%v", top)
			o.TypeBoth(k)
		}
	}
	shot("28-before-password")
	o.TypeBoth("1234\r")
	for i := 0; i < 6; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Logf("送密碼之後第 %d 段停止：%v", i, err)
			break
		}
	}
	shot("34-password-yn")
	// 密碼確認之後才是玩家選擇與主命令：答 Y，再跑一段記下後面的呼叫端。
	o.Drain()
	o.PressScan("Y")
	for i := 0; i < 8; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Logf("答 Y 之後第 %d 段停止：%v", i, err)
			break
		}
		shot(fmt.Sprintf("%02d-after-Y", 35+i))
		if n := len(asks); n > 0 && asks[n-1].kind == "num" && asks[n-1].lo == 0 && asks[n-1].hi == 9 {
			break
		}
	}
	t.Logf("月底結算 %d 次、郡回合入口 %d 次", months, turns)
	for _, a := range asks {
		t.Logf("步 %11d  %s  caller %#07x  (%d..%d)", a.step, a.kind, a.caller, a.lo, a.hi)
	}
}

// TestZZPlusNewGameProbe 用固定預算走加強版的「開始新遊戲」，記每一步的
// 畫面雜湊與數字輸入的呼叫端——給 `bootToNewGamePlus` 當路標。
func TestZZPlusNewGameProbe(t *testing.T) {
	root := plusRoot(t)
	o, err := oracle.Load(filepath.Join(root, "ASV.EXE"), root)
	if err != nil {
		t.Fatalf("加強版載不進來：%v", err)
	}
	defer o.Close()
	type ask struct {
		step   uint64
		caller uint32
		lo, hi int
		kind   string
	}
	var asks []ask
	o.OnCall(addr(plusNumInputFn), func(o *oracle.Oracle) {
		asks = append(asks, ask{o.Steps(), o.Caller().Linear(), int(int16(o.Arg(0))), int(int16(o.Arg(1))), "num"})
	})
	o.OnCall(addr(plusKeyInputFn), func(o *oracle.Oracle) {
		asks = append(asks, ask{o.Steps(), o.Caller().Linear(), 0, 0, "key"})
	})
	shot := func(name string) {
		h := sha256.Sum256(o.IndexedEGASize(scrW, scrH))
		t.Logf("%s：步 %d、畫面 %x", name, o.Steps(), h)
		dumpScreen(t, o, "plusnew-"+name)
	}
	o.Type(envOr("SAN1_PLUSKEY", "122"))
	if err := o.Run(200_000_000); err != nil {
		t.Logf("開機段停止：%v", err)
	}
	// 與 bootLikePlus 同一個節奏：第 5 段送 Enter（故事）、第 18 段在主選單。
	steps := []struct {
		seg  int
		key  string
		name string
	}{
		{4, "\r", "story"}, {17, "1", "menu"}, {20, "1", "era"}, {23, "1\r", "players"},
		{26, "2\r", "lord"}, {29, "5\r", "difficulty"},
	}
	next := 0
	for i := 0; i < 40; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Logf("第 %d 段停止：%v", i, err)
			break
		}
		if next < len(steps) && steps[next].seg == i {
			shot(fmt.Sprintf("%02d-before-%s", i+1, steps[next].name))
			o.Drain()
			o.TypeBoth(steps[next].key)
			next++
		}
	}
	shot("41-end")
	for _, a := range asks {
		t.Logf("步 %11d  %s  caller %#07x  (%d..%d)", a.step, a.kind, a.caller, a.lo, a.hi)
	}
}
