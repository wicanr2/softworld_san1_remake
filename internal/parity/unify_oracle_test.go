//go:build oracle

package parity

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// unifyRun 是原版示範模式跑一局的結果（Issue #9）。
type unifyRun struct {
	Seed       uint32 `json:"seed"`
	SeedAt     string `json:"seed_at"`     // 種子寫進去的那一刻（第一個郡回合入口）
	Scenario   string `json:"scenario"`    // 劇本
	Difficulty int    `json:"difficulty"`  // 難度
	StartYear  int    `json:"start_year"`  // 開局年
	Unified    bool   `json:"unified"`     // 有沒有在上限內統一
	Year       int    `json:"year"`        // 統一那一刻的西元年
	Month      int    `json:"month"`       // 月
	Winner     int    `json:"winner"`      // 勢力槽號（0 起）
	WinnerLord string `json:"winner_lord"` // 君主名（cp950 解碼）
	Months     int    `json:"months"`      // 走過的月底結算次數
	Steps      uint64 `json:"steps"`       // 原版指令數
	Seconds    int    `json:"seconds"`     // 實跑秒數
	Passwords  int    `json:"passwords"`   // 途中答了幾次防拷密碼
	Alive      []int  `json:"alive"`       // 每年元月還有幾個勢力持有郡
}

// 原版工作段裡的日期（`docs/re/08` §1）與三張表的位置。
const (
	unifyWorkSegVar  = 0xa726 // DS:[..] ＝ 工作段
	unifyYearOff     = 0x3140
	unifyMonthOff    = 0x3f08
	unifyTablesBase  = 0x399b0
	unifyMonthEndFn  = 0x1581c // 月底結算入口（`docs/re/06` §9.5）
	unifyTurnEntryFn = 0x1746e // 郡回合入口（`docs/re/03` §1.5）
	unifySeedVar     = 0xa3ae  // 亂數種子（`docs/re/03` §1.45）
)

// isBudget 判斷 RunUntil 是因為預算用完而不是條件成立回來的。
func isBudget(err error) bool {
	var b *oracle.BudgetError
	return errors.As(err, &b)
}

func workSeg(o *oracle.Oracle) uint32 {
	ds := uint32(o.DSReg()) * 16
	return uint32(o.Word(addr(ds+unifyWorkSegVar))) * 16
}

// ownersOf 讀 42 個郡的所屬（州郡 offset 30，`0xFF` ＝ 無主）。
func ownersOf(o *oracle.Oracle) map[int]int {
	owners := map[int]int{}
	sta := uint32(unifyTablesBase + state.MasterTableSize)
	for id := 1; id <= state.PrefectureCount; id++ {
		v := int(o.Bytes(addr(sta+uint32(id*state.PrefectureRecordSize)+30), 1)[0])
		if v != 0xFF {
			owners[v]++
		}
	}
	return owners
}

func lordNameOf(o *oracle.Oracle, faction int) string {
	slot := int(o.Word(addr(uint32(unifyTablesBase + faction*state.MasterRecordSize + 2))))
	gen := uint32(unifyTablesBase + state.MasterTableSize + state.PrefectureTableSize)
	raw := o.Bytes(addr(gen+uint32(slot*state.GeneralRecordSize)), 6)
	n := 0
	for n < len(raw) && raw[n] != 0 {
		n++
	}
	s, err := decodeBig5(raw[:n])
	if err != nil {
		return fmt.Sprintf("%x", raw)
	}
	return s
}

// bootToDemo 開新局、玩家數答 0（電腦自動示範模式）、難度 5，停在遊戲
// 開始之後。走的是 `bootToNewGame` 同一套行為停點，只是玩家數那一格送 0，
// 之後沒有選君主與就任對白。
func bootToDemo(t *testing.T, o *oracle.Oracle, difficulty int) *bootSignals {
	t.Helper()
	s := bootToMenu(t, o)

	var ascCallers []uint32
	var nums []struct{ lo, hi int }
	o.OnCall(addr(bootASCInputFn), func(o *oracle.Oracle) {
		ascCallers = append(ascCallers, o.Caller().Linear())
	})
	o.OnCall(addr(bootNumInputFn), func(o *oracle.Oracle) {
		nums = append(nums, struct{ lo, hi int }{int(o.Arg(0)), int(o.Arg(1))})
	})
	const eraCaller = 0x1058*16 + 0x17d7
	const playerCountCaller = 0x33d8*16 + 0x1fab
	waitCaller := func(name string, want uint32) {
		before := len(ascCallers)
		waitBoot(t, o, name, 500_000_000, func() bool {
			for _, c := range ascCallers[before:] {
				if c == want {
					return true
				}
			}
			return false
		})
	}
	o.Drain()
	o.PressScan("1")
	waitCaller("新局年代選擇輸入", eraCaller)
	waitBootScan(t, o, "新局年代畫面", 500_000_000)
	o.Drain()
	o.TypeBoth("1")
	waitCaller("新局玩家數輸入", playerCountCaller)
	waitBootScan(t, o, "新局玩家數畫面", 500_000_000)
	o.Drain()
	o.TypeBoth("0\r")

	// 「電腦自動示範模式」之後要任一鍵確認（`docs/re/02` §3.1），
	// 接著才問難度（0..9 之外的 1..10 那個數字欄位）。
	before := len(nums)
	waitNum := func() bool {
		for _, n := range nums[before:] {
			if n.lo == 1 && n.hi == 10 {
				return true
			}
		}
		return false
	}
	o.Drain()
	o.PressScan("\r")
	waitBoot(t, o, "示範模式的難度輸入", 500_000_000, waitNum)
	waitBootScan(t, o, "難度欄位", 5_000_000)
	o.Drain()
	o.TypeBoth(fmt.Sprintf("%d\r", difficulty))
	return s
}

// TestZZUnifyYearOriginal 讓原版在電腦自動示範模式下自己打完一局，
// 記下哪一年統一、誰統一（Issue #9）。
//
// 一次跑一個 seed（`SAN1_SEED`，預設 `0x13579BDF`）；分布由外面跑多次湊。
// 種子在**第一個郡回合入口**寫進 `DS:0xa3ae`——那時開局的抽樣（防拷
// 密碼的地支等）已經做完，之後的每一次 `RND()` 都從這顆種子出發。
//
// 判準是**盤面**：月底結算入口每次被呼叫時讀 42 個郡的所屬，只剩一個
// 勢力持有郡就算統一（原版自己的判定 `0x15852` 也是這個條件，
// `docs/mechanics/80` §1）。上限 `SAN1_UNIFY_YEARS` 年（預設 150），
// 到了沒統一也照樣記，`unified` 為假。
func TestZZUnifyYearOriginal(t *testing.T) {
	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	seed := uint32(0x13579BDF)
	if v := os.Getenv("SAN1_SEED"); v != "" {
		n, err := strconv.ParseUint(v, 0, 32)
		if err != nil {
			t.Fatal(err)
		}
		seed = uint32(n)
	}
	maxYears := 150
	if v, err := strconv.Atoi(os.Getenv("SAN1_UNIFY_YEARS")); err == nil && v > 0 {
		maxYears = v
	}
	const difficulty = 5

	started := time.Now()
	s := bootToDemo(t, o, difficulty)
	run := &unifyRun{Seed: seed, Scenario: "001", Difficulty: difficulty}

	seeded := false
	o.OnCall(addr(unifyTurnEntryFn), func(o *oracle.Oracle) {
		if seeded {
			return
		}
		seeded = true
		ds := uint32(o.DSReg()) * 16
		o.SetWord(addr(ds+unifySeedVar), uint16(seed))
		o.SetWord(addr(ds+unifySeedVar+2), uint16(seed>>16))
		run.SeedAt = o.IP().String()
		run.StartYear = int(o.Word(addr(workSeg(o) + unifyYearOff)))
	})

	var lastYear int
	done := false
	o.OnCall(addr(unifyMonthEndFn), func(o *oracle.Oracle) {
		if done {
			return
		}
		run.Months++
		ws := workSeg(o)
		year := int(o.Word(addr(ws + unifyYearOff)))
		month := int(o.Word(addr(ws + unifyMonthOff)))
		owners := ownersOf(o)
		if year != lastYear {
			run.Alive = append(run.Alive, len(owners))
			lastYear = year
		}
		if len(owners) == 1 {
			for f := range owners {
				run.Winner = f
			}
			run.Unified, run.Year, run.Month = true, year, month
			run.WinnerLord = lordNameOf(o, run.Winner)
			done = true
		}
	})

	// 示範模式沒有人下命令，原版自己一路跑。途中唯一會停下來等鍵的是
	// 防拷密碼（`RND(12) < 月份` 抽中就問，`docs/re/08` §2）；答了就繼續。
	const chunk = 200_000_000
	lastMonths, stall := -1, 0
	handledPw := s.passwordAsk
	nextLog := 60
	for !done && run.Months < maxYears*12 {
		// **停在密碼常式被呼叫的那一刻**再餵字元：跑過頭之後再 `Type`，
		// 字元進不了那個欄位（`bootToNewGame` 也是在 callback 上停的）。
		stopAt := oracle.NewCond("密碼或統一", func(*oracle.Oracle) bool {
			return done || s.passwordAsk > handledPw
		})
		if err := o.RunUntil(stopAt, oracle.Budget(chunk)); err != nil && !isBudget(err) {
			t.Fatalf("原版停止：%v", err)
		}
		// 示範模式一路放音樂，埠寫入紀錄每個月長二十幾萬筆（70 個月
		// 1,670 萬筆），不清掉會在四百個月左右把 4 GB 的容器撐爆。
		// `PortLog` 沒有清除 API，但 `Restore` 會把它歸零——倒回剛拍的快照
		// 等於原地不動、只清紀錄。
		o.ClearOPL()
		o.Restore(o.Save())
		// 統一判定（`0x15852`）在月底結算之後、下一次入口之前觸發，成立就
		// 進結局畫面等按鍵——月底結算的 hook 因此看不到最後那一步。
		// 每段跑完再看一次盤面：只剩一個勢力持郡就是統一了。
		if owners := ownersOf(o); !done && len(owners) == 1 {
			for f := range owners {
				run.Winner = f
			}
			ws := workSeg(o)
			run.Unified = true
			run.Year, run.Month = int(o.Word(addr(ws+unifyYearOff))), int(o.Word(addr(ws+unifyMonthOff)))
			run.WinnerLord = lordNameOf(o, run.Winner)
			done = true
		}
		if s.passwordAsk > handledPw {
			handledPw = s.passwordAsk
			run.Passwords++
			dumpScreen(t, o, fmt.Sprintf("unify-pw%d-ask", run.Passwords))
			o.Drain()
			// 載入路徑的密碼是用掃描碼送的（`bootToMainState`），這裡照做；
			// 字元佇列那條在這個欄位上進不去（試過：畫面上一個數字都沒出現）。
			o.PressScan(passwordAnswer + "\r")
			beforeYN := s.passwordYN
			cond := oracle.NewCond("示範中的密碼確認", func(*oracle.Oracle) bool {
				return s.passwordYN > beforeYN
			})
			if err := o.RunUntil(cond, oracle.Budget(500_000_000)); err != nil {
				dumpScreen(t, o, fmt.Sprintf("unify-pw%d-stuck", run.Passwords))
				t.Fatalf("答了密碼之後沒等到確認（月份 %d）：%v", run.Months, err)
			}
			o.Drain()
			o.PressScan("Y")
			dumpScreen(t, o, fmt.Sprintf("unify-pw%d-done", run.Passwords))
		}
		if run.Months == lastMonths {
			stall++
			if stall >= 10 {
				dumpScreen(t, o, fmt.Sprintf("unify-stall-%08x", seed))
				ops := o.FileOps()
				if len(ops) > 8 {
					ops = ops[len(ops)-8:]
				}
				writeUnifyRun(t, run, seed)
				t.Fatalf("連續 %d 段（%d 道指令）沒有月底結算——原版停在某個輸入上；"+
					"畫面存成 unify-stall-%08x，密碼 %d 次、月份 %d、停在 %s、最後幾次檔案操作 %v",
					stall, uint64(stall)*chunk, seed, run.Passwords, run.Months, o.IP(), ops)
			}
		} else {
			stall, lastMonths = 0, run.Months
		}
		if run.Months >= nextLog {
			nextLog += 60
			// 每六十個月就把目前的結果寫一次：跑到逾時被砍也留得下
			// 「走到哪一年、剩幾個勢力」，不會整局白跑。
			run.Steps, run.Seconds = o.Steps(), int(time.Since(started).Seconds())
			writeUnifyRun(t, run, seed)
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			t.Logf("seed %#x：%d 個月、%d 個勢力持郡、%s、%d 道指令、heap %d MB、檔案操作 %d 筆、喇叭樣本 %d 筆、OPL %d 筆",
				seed, run.Months, run.Alive[len(run.Alive)-1],
				time.Since(started).Round(time.Second), o.Steps(),
				ms.HeapAlloc>>20, len(o.FileOps()), len(o.Speaker()), len(o.PortWrites()))
		}
	}
	run.Steps = o.Steps()
	run.Seconds = int(time.Since(started).Seconds())
	if run.Unified {
		t.Logf("seed %#x：%d 年 %d 月由勢力 %d（%s）統一；%d 個月、%d 道指令、%s",
			seed, run.Year, run.Month, run.Winner, run.WinnerLord, run.Months,
			run.Steps, time.Since(started).Round(time.Second))
	} else {
		t.Logf("seed %#x：%d 個月沒統一，還剩 %d 個勢力；%s",
			seed, run.Months, run.Alive[len(run.Alive)-1], time.Since(started).Round(time.Second))
	}
	dumpScreen(t, o, fmt.Sprintf("unify-end-%08x", seed))
	writeUnifyRun(t, run, seed)
}

func writeUnifyRun(t *testing.T, run *unifyRun, seed uint32) {
	t.Helper()
	dir := os.Getenv("SAN1_DUMP")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(run, "", "  ")
	path := filepath.Join(dir, fmt.Sprintf("unify-orig-%08x.json", seed))
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("結果寫到 %s", path)
}
