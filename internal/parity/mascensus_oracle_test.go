//go:build oracle

package parity

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// masAccessSite 是諸侯表的一個存取端：哪一道指令、讀或寫、碰過哪些位移。
type masAccessSite struct {
	IP     string `json:"ip"`     // 執行期 CS:IP
	Linear uint32 `json:"linear"` // 線性位址（objdump 的 --adjust-vma 系）
	Kind   string `json:"kind"`   // r／w
	Count  int    `json:"count"`
	Slots  []int  `json:"slots"`  // 碰過的諸侯槽號（0–15）
	Offs   []int  `json:"offs"`   // 碰過的記錄內位移（0–71）
	Bytes  string `json:"bytes"`  // 該指令起 8 個位元組，給 objdump 對讀
	First  uint64 `json:"first"`  // 第一次命中的指令計數
}

// masCensus 是一次執行的普查結果。
type masCensus struct {
	Edition   string                    `json:"edition"`
	Exe       string                    `json:"exe"`
	ExeSHA256 string                    `json:"exe_sha256"`
	Base      uint32                    `json:"tables_base"`
	Cycles    int                       `json:"cycles"`
	MonthEnds int                       `json:"month_ends"`
	Reads     int                       `json:"reads"`
	Writes    int                       `json:"writes"`
	ByOffset  map[string][]string       `json:"by_offset"` // 位移 → 存取端 key
	Sites     map[string]*masAccessSite `json:"sites"`
}

// masCensusRun 在已經站在遊戲主命令的 oracle 上做諸侯表存取普查。
//
// 監看的是**線性位址範圍**（`WatchReadsAt`／`WatchWritesAt`），所以以
// 算出來的指標存取也看得到——靜態 xref 對 `es:[bx+N]` 這種形狀是盲的。
// 每一段 Run 之後就把紀錄收進聚合表再清掉，不讓逐次紀錄在記憶體裡長。
func masCensusRun(t *testing.T, o *oracle.Oracle, base uint32, seq []string,
	cycles int, ed masEdition) *masCensus {
	t.Helper()
	const settle = 40_000_000
	lo, hi := base, base+uint32(state.MasterTableSize)-1
	c := &masCensus{
		Base:     base,
		Cycles:   cycles,
		ByOffset: map[string][]string{},
		Sites:    map[string]*masAccessSite{},
	}
	slotSet := map[string]map[int]bool{}
	offSet := map[string]map[int]bool{}
	hit := func(kind string, ip oracle.Addr, off uint16, step uint64) {
		lin := ip.Linear()
		key := fmt.Sprintf("%s@%05x", kind, lin)
		s := c.Sites[key]
		if s == nil {
			s = &masAccessSite{
				IP: ip.String(), Linear: lin, Kind: kind, First: step,
				Bytes: fmt.Sprintf("% x", o.Bytes(ip, 8)),
			}
			c.Sites[key] = s
			slotSet[key] = map[int]bool{}
			offSet[key] = map[int]bool{}
		}
		s.Count++
		slotSet[key][int(off)/state.MasterRecordSize] = true
		offSet[key][int(off)%state.MasterRecordSize] = true
	}

	// 月底結算的入口（`docs/re/06` §9.5）：數走過幾個月。**位址 per-binary。**
	o.OnCall(addr(ed.monthEnd), func(*oracle.Oracle) { c.MonthEnds++ })

	reads := o.WatchReadsAt(lo, hi)
	writes := o.WatchWritesAt(lo, hi)
	drain := func() {
		for _, r := range *reads {
			hit("r", r.IP, r.Off, r.Step)
		}
		c.Reads += len(*reads)
		*reads = (*reads)[:0]
		for _, w := range *writes {
			hit("w", w.IP, w.Off, w.Step)
		}
		c.Writes += len(*writes)
		*writes = (*writes)[:0]
	}
	for i := 1; i <= cycles; i++ {
		for j, keys := range seq {
			o.Drain()
			o.PressScan(keys)
			err := o.Run(settle * 3)
			drain()
			if err != nil {
				t.Logf("第 %d 輪第 %d 段停止：%v", i, j+1, err)
				i = cycles
				break
			}
		}
		if i%4 == 0 || i == cycles {
			t.Logf("第 %2d 輪：月底結算 %d 次、游標 %d、讀 %d 次、寫 %d 次、存取端 %d",
				i, c.MonthEnds, ed.cursor(o), c.Reads, c.Writes, len(c.Sites))
		}
	}
	o.StopWatchingReads()
	o.StopWatchingWrites()

	for key, s := range c.Sites {
		for k := range slotSet[key] {
			s.Slots = append(s.Slots, k)
		}
		sort.Ints(s.Slots)
		for k := range offSet[key] {
			s.Offs = append(s.Offs, k)
			ok := strconv.Itoa(k)
			c.ByOffset[ok] = append(c.ByOffset[ok], key)
		}
		sort.Ints(s.Offs)
	}
	for _, v := range c.ByOffset {
		sort.Strings(v)
	}
	return c
}

// masCensusReport 印出逐位移的摘要，並把整份寫進 SAN1_DUMP。
func masCensusReport(t *testing.T, c *masCensus, name string) {
	t.Helper()
	var touched, untouched []int
	for off := 0; off < state.MasterRecordSize; off++ {
		keys := c.ByOffset[strconv.Itoa(off)]
		if len(keys) == 0 {
			untouched = append(untouched, off)
			continue
		}
		touched = append(touched, off)
		nr, nw := 0, 0
		for _, k := range keys {
			if c.Sites[k].Kind == "r" {
				nr++
			} else {
				nw++
			}
		}
		t.Logf("offset %2d：讀端 %d、寫端 %d：%s", off, nr, nw, strings.Join(keys, " "))
	}
	t.Logf("%s：%d 個月、讀 %d 次、寫 %d 次；碰過的位移 %d 個 %v；沒碰過 %d 個 %v",
		name, c.MonthEnds, c.Reads, c.Writes, len(touched), touched,
		len(untouched), untouched)

	dir := os.Getenv("SAN1_DUMP")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Log(err)
		return
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name+".json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Log(err)
		return
	}
	t.Logf("普查寫到 %s", path)
}

// masEdition 是普查用到的 per-binary 位址：月底結算入口與「這個月處理到
// 第幾格」的游標。拿原版的套到加強版上不會報錯，只會一次都攔不到、
// 游標讀成垃圾（第一輪就是這樣：月底結算 0 次、游標 772）。
//
// 加強版的兩個位址是拿原版的指令形狀序列在加強版碼段裡比出來的
// （`docs/re/03` §1.6）：`0x1581c` → `0x14878`、`DS:0xa726` → `DS:0xa8f8`，
// 而游標在工作段裡也挪了兩個位元組（`0x20f4` → `0x20f6`）。
type masEdition struct {
	monthEnd uint32
	cursor   func(*oracle.Oracle) int
}

var (
	masBase = masEdition{monthEnd: 0x1581c, cursor: monthCursor}
	masPlus = masEdition{monthEnd: 0x14878, cursor: func(o *oracle.Oracle) int {
		ds := uint32(o.DSReg()) * 16
		return int(int16(o.Word(addr(uint32(o.Word(addr(ds+0xa8f8)))*16 + 0x20f6))))
	}}
)

func sha256File(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func censusCycles() int {
	if v, err := strconv.Atoi(os.Getenv("SAN1_TURNS")); err == nil && v > 0 {
		return v
	}
	return 24
}

// TestZZMasterRecordAccessCensus 量原版**跑遊戲時**碰過諸侯記錄的哪些位元組
// （Issue #6）。
//
// 判準是原版自己的存取，不是靜態 xref：三張表全部以 `es:[reg+位移]` 取用，
// 靜態掃描抓不到（`docs/re/03` §1.1）。這裡在整張諸侯表（1,152 個位元組）
// 上掛讀寫監看，讓玩家每個月「內政 → 休息」，電腦諸侯照常行動，
// 把每一次讀寫依「記錄內位移 × 指令位址」歸類。
//
// 一個位移**在這一次執行裡沒被碰過**，只證明「這條路徑沒有 consumer」，
// 不證明它是填充——覆蓋率只揭露執行過的碼（`docs/re/01` §3）。
// 碰過的位移則拿到 L1 的讀寫端位址，接著用同一次執行倒出的碼段對讀。
func TestZZMasterRecordAccessCensus(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, _, _ := sc.Tables()

	exe := filepath.Join(root, "AA.EXE")
	o, err := oracle.Load(exe, root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToGame(t, o, mas)
	dumpImage(t, o, 0x00b000, 0x050000, "mas-census-base-code")

	seq := strings.Split(envOr("SAN1_TURNKEY", "4\r|4\r|Y"), "|")
	res := masCensusRun(t, o, base, seq, censusCycles(), masBase)
	res.Edition, res.Exe, res.ExeSHA256 = "base", "AA.EXE", sha256File(t, exe)
	masCensusReport(t, res, "mas-census-base")

	// 正對照：已知欄位一定要被碰到，否則是監看沒掛上，不是欄位沒人用。
	for _, off := range []int{0, 2, 4, 6, 8} {
		if len(res.ByOffset[strconv.Itoa(off)]) == 0 {
			t.Errorf("已知欄位 offset %d 一次都沒被碰到——監看沒掛上", off)
		}
	}
}

// TestZZMasterRecordAccessCensusPlus 對加強版做同一件事。
//
// 加強版沒有行為觸發的開機配方，這裡沿用 `bootLikePlus`（指令數觸發，
// `docs/spec/015` 之前的做法）。三張表的基底用**它自己的** `DATA2` 進度 1
// 去搜——兩版的 DGROUP 位置不同，拿原版的常數套過來不會報錯，只會一次
// 都攔不到（`CLAUDE.md` §4.1）。
func TestZZMasterRecordAccessCensusPlus(t *testing.T) {
	root := plusRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("SV1"))
	if err != nil {
		t.Fatal(err)
	}
	mas, _, _ := sc.Tables()

	exe := filepath.Join(root, "ASV.EXE")
	o, err := oracle.Load(exe, root)
	if err != nil {
		t.Fatalf("加強版載不進來：%v", err)
	}
	defer o.Close()
	bootLikePlus(t, o)

	// 進度 1 的諸侯表在記憶體裡應該只出現一次。offset 0 是操縱方，載入時
	// 會被改寫，所以從 offset 2 起搜。
	hits := o.Search(mas[2:48])
	if len(hits) != 1 {
		t.Fatalf("加強版的諸侯表找到 %d 處：%x", len(hits), hits)
	}
	base := hits[0] - 2
	t.Logf("加強版三張表的基底 ＝ %#x", base)
	dumpImage(t, o, 0x00b000, 0x050000, "mas-census-plus-code")

	seq := strings.Split(envOr("SAN1_TURNKEY", "4\r|4\r|Y"), "|")
	res := masCensusRun(t, o, base, seq, censusCycles(), masPlus)
	res.Edition, res.Exe, res.ExeSHA256 = "plus", "ASV.EXE", sha256File(t, exe)
	if res.MonthEnds == 0 {
		t.Errorf("加強版一次月底結算都沒攔到——攔截點 %#x 不是它的入口", masPlus.monthEnd)
	}
	masCensusReport(t, res, "mas-census-plus")

	for _, off := range []int{0, 2, 4, 6, 8} {
		if len(res.ByOffset[strconv.Itoa(off)]) == 0 {
			t.Errorf("已知欄位 offset %d 一次都沒被碰到——監看沒掛上", off)
		}
	}
}
