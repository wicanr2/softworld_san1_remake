//go:build oracle

package parity

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 誰寫了這個欄位。
//
// **這是「決策程式碼在哪」最直接的答案。** 靜態的交叉參考只涵蓋直接
// 定址；`mov es:[si+0x2228], al` 這種以結構基底加位移的寫法掃不出來，
// 而三張表全部是這樣存取的（`docs/re/03` §1）。
//
// 做法是讓執行器在寫入落進表的範圍時記下當時的 `CS:IP`，跑一個月，
// 再按 IP 分組。**訓練度是誰改的、金是誰扣的**，答案就是那幾個位址。
//
// ⚠ **位址只到「這一次執行的線性位址」為止。** 現成的
// `workplace/ida/OVL.BIN`（從 `0110:0000` 取的）裡連一次 `28 22` 都沒有
// ——那是人物表訓練度欄的位移，所以含這些程式碼的那一層不在那份 dump
// 裡。要對到映像位移，得從**同一次執行**把碼段取出來（`dumpImage`）。

// dumpImage 把一段線性記憶體寫成檔，給 objdump 用。
//
// 寫進 `workplace/`（gitignore）。那是原版載入後的碼段，與原版執行檔
// 一樣不散布。
func dumpImage(t *testing.T, o *oracle.Oracle, lo, hi uint32, name string) {
	t.Helper()
	dir := os.Getenv("SAN1_DUMP")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Log(err)
		return
	}
	b := o.Bytes(addr(lo), int(hi-lo))
	path := filepath.Join(dir, fmt.Sprintf("%s-%06x.bin", name, lo))
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Log(err)
		return
	}
	t.Logf("碼段 %#x–%#x（%d 個位元組）寫到 %s；objdump 的 --adjust-vma ＝ %#x",
		lo, hi, len(b), path, lo)
}

// TestZZDumpCode 只做一件事：開機到遊戲裡，把碼段寫成檔。
//
// 拆開來是因為**取碼段不必跑一個月**，而跑一個月會讓整條測試超過十分鐘
// ——那個長度在這台機器上常常被記憶體守衛砍掉，砍掉就什麼都沒留下。
func TestZZDumpCode(t *testing.T) {
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
	dumpImage(t, o, 0x00b000, 0x01f000, "code")
}

// TestZZHookTraining 攔「訓練兵士」那支常式，讀它的參數與呼叫端。
//
// 常式在線性 `0xbd70`（`docs/re/03` §1.3）。它對郡裡每一位守將算
//
//	訓練度 = min(100, 訓練度 + (智/3 + 武/2) / 參數)
//
// **參數是唯一還不知道的東西**，而呼叫端就是電腦諸侯的決策位址——
// 一次攔截兩件事都拿得到。
func TestZZHookTraining(t *testing.T) {
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

	seen := map[string]int{}
	// `0xbd70` 是共用常式，`0xbe80`／`0xbe94` 是兩個 thunk——各自推一個
	// 常數（4 與 3）再呼叫它。**決策端是呼叫 thunk 的人**，所以三個都攔。
	for _, site := range []struct {
		name string
		lin  uint32
	}{{"共用常式 0xbd70", 0xbd70}, {"thunk÷4 0xbe80", 0xbe80}, {"thunk÷3 0xbe94", 0xbe94}} {
		name := site.name
		o.OnCall(addr(site.lin), func(o *oracle.Oracle) {
			c := o.Caller()
			seen[fmt.Sprintf("%-18s ← 呼叫端 %04x:%04x（線性 %#06x）",
				name, c.Seg, c.Off, uint32(c.Seg)*16+uint32(c.Off))]++
		})
	}
	for _, keys := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(keys)
		if err := o.Run(40_000_000 * 3); err != nil {
			t.Fatalf("原版停止：%v", err)
		}
	}
	if len(seen) == 0 {
		t.Log("一個月裡沒有人呼叫 0xbd70——位址可能不是常式的進入點")
		return
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("%s ×%d", k, seen[k])
	}
}

// TestZZWhoWritesTheTables 跑一個月，列出寫三張表的程式位址。
func TestZZWhoWritesTheTables(t *testing.T) {
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
	// **碼段和量到的位址要出自同一次執行**，否則對不上。
	dumpImage(t, o, 0x00b000, 0x01f000, "code")
	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	nGen := state.GeneralTableSize
	_ = state.MasterRecordSize

	// 分三段看，否則同一個 IP 寫哪一張表分不出來。
	for _, seg := range []struct {
		name string
		lo   uint32
		n    int
		rec  int
		fld  map[int]string
	}{
		{"州郡", base + uint32(nMas), nSta, state.PrefectureRecordSize, prefField},
		{"人物", base + uint32(nMas+nSta), nGen, state.GeneralRecordSize, genField},
	} {
		log := o.WatchWritesAt(seg.lo, seg.lo+uint32(seg.n)-1)
		for _, keys := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(keys)
			if err := o.Run(40_000_000 * 3); err != nil {
				t.Fatalf("%s：原版停止 %v", seg.name, err)
			}
		}
		o.StopWatchingWrites()

		type site struct {
			n      int
			fields map[string]bool
		}
		by := map[uint32]*site{}
		for _, w := range *log {
			lin := uint32(w.IP.Seg)*16 + uint32(w.IP.Off)
			s := by[lin]
			if s == nil {
				s = &site{fields: map[string]bool{}}
				by[lin] = s
			}
			s.n++
			s.fields[fieldName(seg.fld, int(w.Off)%seg.rec)] = true
		}
		ips := make([]uint32, 0, len(by))
		for a := range by {
			ips = append(ips, a)
		}
		sort.Slice(ips, func(i, j int) bool { return by[ips[i]].n > by[ips[j]].n })

		t.Logf("%s表：一個月裡有 %d 次寫入，來自 %d 個位址",
			seg.name, len(*log), len(ips))
		for i, a := range ips {
			if i >= 20 {
				t.Logf("    …（還有 %d 個位址）", len(ips)-20)
				break
			}
			var fs []string
			for f := range by[a].fields {
				fs = append(fs, f)
			}
			sort.Strings(fs)
			t.Logf("    線性 %#06x 寫了 %4d 次：%v", a, by[a].n, fs)
		}
	}
}
