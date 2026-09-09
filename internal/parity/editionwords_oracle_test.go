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

// 誰讀 `DS:0x506a` 與 `DS:0x50e8`（`docs/mechanics/90-version-diff` 的最後一項）。
//
// 兩版的碼段裡都沒有絕對定址的參考指到它們（`docs/spec/004` §4 複驗過）。
// 但**掃到零筆只證明「沒有絕對定址的參考」**——用算出來的指標取的存取
// 在位元組層面看不見。要分清楚「沒有人讀」與「掃不到」，只能掛讀取監看。
//
// **一定要配正對照。** 監看本身沒響與「沒有人讀」印出來一模一樣，
// 所以同一輪先對一個確定會被讀的位址跑一次（`DS:0xa666`／`0xa668` 是
// 建名單常式每次都 `mov es,[…]` 的段變數）。
func TestWhoReadsTheEditionWords(t *testing.T) {
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

	bootToGame(t, o, seedMas)

	var dgroup uint16
	o.OnCall(addr(0xfc1e), func(o *oracle.Oracle) {
		if dgroup == 0 {
			dgroup = o.DSReg()
		}
	})
	turn := func(tag string) {
		for _, keys := range strings.Split(envOr("SAN1_TURNKEY", "4\r|4\r|Y"), "|") {
			o.Drain()
			o.PressScan(keys)
			if err := o.Run(200_000_000); err != nil {
				t.Fatalf("%s 送 %q 停止：%v", tag, keys, err)
			}
		}
	}
	turn("取 DGROUP")
	if dgroup == 0 {
		t.Fatal("沒抓到 DGROUP——建名單常式一次都沒跑到")
	}
	dg := uint32(dgroup) << 4
	t.Logf("DGROUP 段 %#06x（線性 %#07x）", dgroup, dg)

	snap := o.Save()
	run := func(name string, lo, hi uint16) map[string]int {
		o.Restore(snap)
		log := o.WatchReadsAt(dg+uint32(lo), dg+uint32(hi))
		turn(name)
		o.StopWatchingReads()
		by := map[string]int{}
		byOff := map[string]int{}
		for _, r := range *log {
			by[fmt.Sprintf("%#06x:%#06x", r.IP.Seg, r.IP.Off)]++
			byOff[fmt.Sprintf("%#06x", int(lo)+int(r.Off))]++
		}
		t.Logf("%s（DS:%#06x–%#06x）：讀了 %d 次，來自 %d 個指令位址 %v",
			name, lo, hi, len(*log), len(by), by)
		t.Logf("%s：碰到的位移 %v", name, byOff)
		return by
	}

	// 正對照先跑：監看沒響與「沒有人讀」長得一樣。
	ctrl := run("正對照 段變數", 0xa666, 0xa669)
	if len(ctrl) == 0 {
		t.Fatal("正對照一次都沒讀到——讀取監看沒有生效，這一輪的結論全部無效")
	}

	run("整段", 0x5040, 0x50f0)
	run("0x506a 那張表", 0x506a, 0x5077)
	got := run("0x50e8 那個 word", 0x50e8, 0x50e9)
	// 讀 `0x50e8` 的是 `0x03eb:0x034e`（線性 `0x41fe`），**在已倒出的
	// 碼段窗（`0xb000` 起）之外**——靜態掃描是零筆的原因就在這裡。
	// 攔那一道指令，把呼叫端記下來：那些位址落在遊戲自己的碼段裡，
	// 反組譯得到，就知道是哪一類動作會去讀它。
	o.Restore(snap)
	callers := map[string]int{}
	o.OnCall(addr(0x41fe), func(o *oracle.Oracle) {
		c := o.Caller()
		callers[fmt.Sprintf("%#06x:%#06x", c.Seg, c.Off)]++
	})
	turn("抓呼叫端")
	t.Logf("讀 DS:0x50e8 那一道指令（0x03eb:0x034e）的呼叫端：%v", callers)
	if len(callers) == 0 {
		t.Log("攔不到 0x41fe——那個位址不是指令邊界，或這一輪沒走到")
	}

	if len(got) == 0 {
		t.Logf("一個月裡沒有任何指令讀過 DS:0x50e8。"+
			"正對照同一輪讀到 %d 個位址，所以監看是活的。", len(ctrl))
	}

	// ── 它是不是延時？把值放大，量那一支跑掉多少指令 ──────────
	//
	// `0x19653` 的 `lcall 0x3eb:0x31a` **不帶參數、回傳值也丟掉**
	// （下一道就是 `sub ax,ax`），而它在逐郡的月循環裡每郡跑一次
	// ——不帶參數卻讀一個 32 位元全域，是延時常式的形狀。
	//
	// 判準不是「看起來像」：**把值乘上去，看那一支花掉的指令數跟不跟著走**。
	measure := func(tag string, mul uint32) (int, uint64) {
		o.Restore(snap)
		lo := oracle.Addr{Seg: dgroup, Off: 0x50e8}
		hi := oracle.Addr{Seg: dgroup, Off: 0x50ea}
		v := uint32(o.Word(lo)) | uint32(o.Word(hi))<<16
		if mul != 1 {
			nv := v * mul
			o.SetWord(lo, uint16(nv))
			o.SetWord(hi, uint16(nv>>16))
		}
		var enter uint64
		var total uint64
		n := 0
		// **要從常式入口量起**：`0x3eb:0x31a` ＝ 線性 `0x41ca`。
		// 從讀值那一刻（`0x41fe`）量到返回只有四條，那是尾巴不是整支。
		o.OnCall(addr(0x41ca), func(o *oracle.Oracle) { enter = o.Steps() })
		o.OnCall(addr(0x19658), func(o *oracle.Oracle) {
			if enter == 0 {
				return
			}
			total += o.Steps() - enter
			n++
			enter = 0
		})
		turn(tag)
		t.Logf("%s：DS:0x50e8 ＝ %d，那一支跑了 %d 次、共 %d 條指令"+
			"（平均 %d 條）", tag, v*mul, n, total, total/uint64(max(n, 1)))
		return n, total
	}
	n1, t1 := measure("原值", 1)
	n2, t2 := measure("放大十倍", 10)
	if n1 == 0 || n2 == 0 {
		t.Fatal("攔不到那一支的進出——位址不是指令邊界，或這一輪沒走到")
	}
	a1, a2 := t1/uint64(n1), t2/uint64(n2)
	t.Logf("平均每次：原值 %d 條、放大十倍 %d 條（%.1f 倍）",
		a1, a2, float64(a2)/float64(max64(a1, 1)))
	if a2 <= a1*2 {
		t.Logf("值放大十倍，那一支花的指令沒變（%d → %d）"+
			"——**它不是照這個值在數的忙碌等待**。", a1, a2)
	}

	// 那它讀這個值做什麼？看它有沒有碰 BIOS 的計時器
	// （`0040:006C`，線性 `0x46c`）——有的話這個值就是拿來比時間的。
	o.Restore(snap)
	ticks := o.WatchReadsAt(0x46c, 0x46f)
	turn("看誰讀 BIOS 計時器")
	o.StopWatchingReads()
	byIP := map[string]int{}
	for _, r := range *ticks {
		byIP[fmt.Sprintf("%#06x:%#06x", r.IP.Seg, r.IP.Off)]++
	}
	t.Logf("讀 BIOS 計時器（0040:006C）的指令：%v", byIP)
	lib := 0
	for k, n := range byIP {
		if strings.HasPrefix(k, "0x0003eb:") {
			lib += n
		}
	}
	t.Logf("其中出自 0x03eb 那個程式庫段的有 %d 次", lib)

	// **它會不會被寫？** 被讀又被寫的 32 位元格子是「執行期狀態」，
	// 不是設定值——那樣的話兩版數字不同只是跑到不同時刻的殘留，
	// 不能算版本差異（`docs/spec/004` §4）。
	o.Restore(snap)
	wr := o.WatchWritesAt(dg+0x50e8, dg+0x50eb)
	turn("看誰寫 0x50e8")
	o.StopWatchingWrites()
	wIP := map[string]int{}
	for _, w := range *wr {
		wIP[fmt.Sprintf("%#06x:%#06x", w.IP.Seg, w.IP.Off)]++
	}
	t.Logf("寫 DS:0x50e8–0x50eb 的指令：%d 次，來自 %v", len(*wr), wIP)
	o.Restore(snap)
	wr2 := o.WatchWritesAt(dg+0x506a, dg+0x5077)
	turn("看誰寫 0x506a")
	o.StopWatchingWrites()
	w2 := map[string]int{}
	for _, w := range *wr2 {
		w2[fmt.Sprintf("%#06x:%#06x", w.IP.Seg, w.IP.Off)]++
	}
	t.Logf("寫 DS:0x506a–0x5077 的指令：%d 次，來自 %v", len(*wr2), w2)

	if len(*wr) > 0 {
		t.Logf("**它是執行期會變的狀態**，兩版的數字不同不能當版本差異。")
	} else {
		t.Logf("一個月裡沒有人寫它——是唯讀的設定值或常數。")
	}
}

func max64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
