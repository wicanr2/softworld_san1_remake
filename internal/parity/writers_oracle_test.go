//go:build oracle

package parity

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// TestZZDispatch 讀電腦諸侯的指令分派表。
//
// `0xe926`–`0xe9fc` 有九個分派點，形狀都一樣：
//
//	mov es, [0xa63a]
//	mov bx, es:[0x20f6]      ; 索引
//	shl bx; shl bx           ; ×4（far pointer）
//	lcall far ptr [bx+0x55NN]
//
// 表的位址彼此相差 `0x20` ＝ 8 個 far pointer，所以**索引是 0–7**，
// 也就是每種行為有八個版本。這一條把索引與解出來的目標位址讀下來
// ——那份對應就是「哪一種電腦諸侯做哪一件事」。
func TestZZDispatch(t *testing.T) {
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
	t.Logf("三張表的基底 %#x，難度 %q，索引來源 %#06x 現在是 %d",
		base, envOr("SAN1_DIFFICULTY", "5"), 0x040736, o.Byte(addr(0x040736)))

	sites := map[uint32]uint16{
		0xe926: 0x54d4, 0xe937: 0x5694, 0xe948: 0x5674, 0xe959: 0x5614,
		0xe96a: 0x5634, 0xe97b: 0x5554, 0xe98c: 0x5534, 0xe99d: 0x56b4,
		0xe9fc: 0x5594,
	}
	// **表走 DS 不是 ES。** 那道 `ff 9f 54 55` 沒有 `26` 前綴，
	// 預設段就是 DS；讀成 ES 會拿到看起來像位址的垃圾。
	word := func(o *oracle.Oracle, lin uint32) uint32 {
		return uint32(o.Byte(addr(lin))) | uint32(o.Byte(addr(lin+1)))<<8
	}
	seen := map[string]int{}
	dumped := map[uint16]bool{}
	for lin, tbl := range sites {
		table := tbl
		at := lin
		o.OnCall(addr(lin), func(o *oracle.Oracle) {
			ds := uint32(o.DSReg())
			// 索引來自 `es:[0x20f6]`，而那個 ES 是前一道指令從變數載的。
			// 把它印出來才知道那一格落在哪張表的哪個欄位。
			ix := uint32(o.ES())*16 + 0x20f6
			seen[fmt.Sprintf("分派點 %#06x 表 %#04x 索引 %d（來源線性 %#06x，距基底 %+d）",
				at, table, o.BX()/4, ix, int(ix)-int(base))]++
			if dumped[table] {
				return
			}
			dumped[table] = true
			// 八個項目一次讀完：表彼此相差 0x20 ＝ 8 個 far pointer。
			for i := 0; i < 8; i++ {
				ent := ds*16 + uint32(table) + uint32(i)*4
				off := word(o, ent)
				seg := word(o, ent+2)
				tgt := seg*16 + off
				// thunk 的形狀是 `33 c0 9a .. .. .. .. b8 K K 50 0e e8`
				// ——推的常數在 `b8` 後面。
				extra := ""
				if o.Byte(addr(tgt)) == 0x33 && o.Byte(addr(tgt+7)) == 0xb8 {
					extra = fmt.Sprintf("　推的常數 %d", word(o, tgt+8))
				}
				seen[fmt.Sprintf("  表 %#04x[%d] → %04x:%04x（線性 %#06x）%s",
					table, i, seg, off, tgt, extra)] = 0
			}
		})
	}
	for _, keys := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(keys)
		if err := o.Run(40_000_000 * 3); err != nil {
			t.Fatalf("原版停止：%v", err)
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if n := seen[k]; n > 0 {
			t.Logf("%s ×%d", k, n)
		} else {
			t.Log(k)
		}
	}
}

// TestZZIndexSource 找出分派索引是誰寫的、寫的是什麼。
//
// 索引在線性 `0x040736`，九個分派點共用它，取值只見過 4 與 5。
// **難度不是它**（難度 5 與 8 得到相同的索引值）。所以直接看寫入端。
func TestZZIndexSource(t *testing.T) {
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

	// 分派常式的進入點在 0xe8d2（`push bp; mov bp,sp`）。
	// 參數就是等級，進去之後被夾在 0–5。
	lv := map[string]int{}
	o.OnCall(addr(0xe8d2), func(o *oracle.Oracle) {
		c := o.Caller()
		lv[fmt.Sprintf("等級 %d ← 呼叫端 %04x:%04x（線性 %#06x）",
			int16(o.Arg(0)), c.Seg, c.Off, uint32(c.Seg)*16+uint32(c.Off))]++
	})

	const ix = 0x040736
	log := o.WatchWritesAt(ix, ix+1)
	for _, keys := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(keys)
		if err := o.Run(40_000_000 * 3); err != nil {
			t.Fatalf("原版停止：%v", err)
		}
	}
	o.StopWatchingWrites()

	seen := map[string]int{}
	for _, w := range *log {
		lin := uint32(w.IP.Seg)*16 + uint32(w.IP.Off)
		seen[fmt.Sprintf("線性 %#06x 把 %#04x 位移的值寫成 %d（原本 %d）",
			lin, w.Off, w.New, w.Old)]++
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lk := make([]string, 0, len(lv))
	for k := range lv {
		lk = append(lk, k)
	}
	sort.Strings(lk)
	for _, k := range lk {
		t.Logf("分派常式 0xe8d2：%s ×%d", k, lv[k])
	}
	t.Logf("一個月裡 %#x 被寫了 %d 次，來自 %d 種寫法", ix, len(*log), len(seen))
	for i, k := range keys {
		if i >= 20 {
			t.Logf("    …（還有 %d 種）", len(keys)-20)
			break
		}
		t.Logf("    %s ×%d", k, seen[k])
	}
}

// TestZZTableMeaning 把九張分派表各自對應到哪些欄位。
//
// 做法是**時間軸歸屬**：分派點與盤面寫入都帶著執行到第幾道指令
// （`MemWrite.Step`／`Oracle.Steps`），所以每一次寫入都可以歸給它前面
// 最近的那一次分派。不必逐支反組譯。
//
// ⚠ 歸屬只在「分派之間不重疊」時成立。九個分派點是**順序**執行的
// （`0xe926` 到 `0xe9fc` 一路往下），所以這個前提在這一層成立；
// 但被呼叫的常式如果自己又轉呼叫別的東西，寫入還是算在它頭上——
// 那正是我們要的。
func TestZZTableMeaning(t *testing.T) {
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
	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	total := nMas + nSta + state.GeneralTableSize

	type ev struct {
		step  uint64
		table uint16
	}
	var evs []ev
	for lin, tbl := range map[uint32]uint16{
		0xe926: 0x54d4, 0xe937: 0x5694, 0xe948: 0x5674, 0xe959: 0x5614,
		0xe96a: 0x5634, 0xe97b: 0x5554, 0xe98c: 0x5534, 0xe99d: 0x56b4,
		0xe9fc: 0x5594,
	} {
		table := tbl
		o.OnCall(addr(lin), func(o *oracle.Oracle) {
			evs = append(evs, ev{o.Steps(), table})
		})
	}
	log := o.WatchWritesAt(base, base+uint32(total)-1)
	for _, keys := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(keys)
		if err := o.Run(40_000_000 * 3); err != nil {
			t.Fatalf("原版停止：%v", err)
		}
	}
	o.StopWatchingWrites()
	sort.Slice(evs, func(i, j int) bool { return evs[i].step < evs[j].step })

	fields := map[uint16]map[string]int{}
	orphan := 0
	for _, w := range *log {
		i := sort.Search(len(evs), func(k int) bool { return evs[k].step > w.Step }) - 1
		if i < 0 {
			orphan++
			continue
		}
		tbl := evs[i].table
		if fields[tbl] == nil {
			fields[tbl] = map[string]int{}
		}
		off := int(w.Off)
		var name string
		switch {
		case off < nMas:
			name = "諸侯." + fieldName(masField, off%state.MasterRecordSize)
		case off < nMas+nSta:
			name = "州郡." + fieldName(prefField, (off-nMas)%state.PrefectureRecordSize)
		default:
			name = "人物." + fieldName(genField, (off-nMas-nSta)%state.GeneralRecordSize)
		}
		fields[tbl][name]++
	}
	t.Logf("一個月：分派 %d 次、盤面寫入 %d 次（%d 次落在第一次分派之前）",
		len(evs), len(*log), orphan)
	tbls := make([]int, 0, len(fields))
	for tb := range fields {
		tbls = append(tbls, int(tb))
	}
	sort.Ints(tbls)
	for _, tb := range tbls {
		m := fields[uint16(tb)]
		ks := make([]string, 0, len(m))
		for k := range m {
			ks = append(ks, k)
		}
		sort.Slice(ks, func(i, j int) bool { return m[ks[i]] > m[ks[j]] })
		var parts []string
		for i, k := range ks {
			if i >= 8 {
				parts = append(parts, "…")
				break
			}
			parts = append(parts, fmt.Sprintf("%s×%d", k, m[k]))
		}
		t.Logf("表 %#04x → %s", tb, strings.Join(parts, " "))
	}
}
