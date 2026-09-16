//go:build oracle

package parity

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 加強版的一個月狀態轉移對拍（Issue #26）。原版那支是
// `TestZZMonthParity`（`month_parity_test.go`），視窗與接法相同：
//
//	開新局 → 玩家「內政 → 休息 → Y」→ 電腦諸侯的郡 → **月底結算入口**
//	（拍盤面、讀亂數狀態）→ 開月洗牌 → 下個月的郡回合 … → 又輪到玩家
//
// remake 從月底結算那一刻的三張表與亂數狀態接上，走 `EndMonth` 再照
// 原版的順序表跑到玩家之前的每一個郡，比三張表。

const (
	// plusRndFn 是加強版的 `RND(n)` 包裝（原版 `0x10b0c`）：形狀
	// `push bp / mov bp,sp / xor ax,ax / call … / cmp [bp+6],0 / jg /
	// sub ax,ax / leave / retf / call rand / cwd / idiv [bp+6] / mov ax,dx`。
	plusRndFn = 0x103aa
	// plusRandFn 是 MSC 的 `rand()`（`05B9:2CB2`，原版 `05C4:2CB0`）。
	// 種子的兩格從它的碼裡讀（`push ds:[lo]` / `push ds:[hi]`）。
	plusRandFn = 0x5b9*16 + 0x2cb2
	// plusWorkSegPtr 是工作段的段值放在 DS 的哪一格（原版 `0xa726`）。
	plusWorkSegPtr = 0xa8f8
	// plusMonthCursor 是「這個月處理到第幾格」在工作段的位移（原版 `0x20f4`）。
	plusMonthCursor = 0x20f6
	// plusMonthOff 是工作段裡的月份（四季分派器 `0x14c19` 讀 `es:[0x3f14]`；
	// 原版 `0x3f08`）。
	plusMonthOff = 0x3f14
)

// plusDispatchTables 是加強版十八張分派表的呼叫端（形狀 `shl bx,2 /
// call dword [bx+表]`，原版 `0xe926`–`0xec1b` 的對應；表的 DS 位移一律
// 是原版 +0x82）。順序與原版那支的 `tables` 相同。
var plusDispatchTables = []struct {
	at   uint32
	name string
}{
	{0xe49c, "行動者"}, {0xe4ac, "指定軍師"}, {0xe4bc, "指定太守"},
	{0xe4cc, "尋訪"}, {0xe4dc, "登用"}, {0xe4ec, "訓練"},
	{0xe4fc, "內政"}, {0xe50c, "賞賜物品"}, {0xe564, "武器"},
	{0xe5bc, "徵兵"}, {0xe614, "0x5596"}, {0xe66c, "賑民"},
	{0xe6c4, "賞賜金帛"}, {0xe71e, "挖角"}, {0xe72e, "計略"},
	{0xe73e, "調整兵力"}, {0xe74e, "買米"}, {0xe75e, "出兵"},
}

// tableLine 把逐表的抽樣次數照分派表的順序印成一行（0 的略過）。
func tableLine(tables []struct {
	at   uint32
	name string
}, m map[string]int) string {
	out := ""
	for _, tb := range tables {
		if n := m[tb.name]; n != 0 {
			out += fmt.Sprintf(" %s=%d", tb.name, n)
		}
	}
	// 表以外的（入口、值的紀錄）
	for k, n := range m {
		known := false
		for _, tb := range tables {
			if tb.name == k {
				known = true
			}
		}
		if !known && n != 0 && !strings.ContainsAny(k, "|｜") {
			out += fmt.Sprintf(" %s=%d", k, n)
		}
	}
	return out
}

// lcgStepsBetween 數 MSC LCG 從 a 走到 b 要幾步（上限 limit，找不到回 −1）。
func lcgStepsBetween(a, b uint32, limit int) int {
	s := a
	for i := 0; i < limit; i++ {
		if s == b {
			return i
		}
		s = s*214013 + 2531011
	}
	return -1
}

// turnSeqNext 是順序表上第 i 格之後、原版真的會走到的下一格（郡 0 那格
// 不存在，原版跳過）。
func turnSeqNext(seq []int, i int) int {
	for j := i + 1; j < len(seq); j++ {
		if seq[j] != 0 {
			return seq[j]
		}
	}
	return -1
}

// plusSeedAddr 從 `rand()` 的碼讀出種子的 DS 位移：MSC 6.0 的 `rand()`
// 是 `push ds:[hi] / push ds:[lo]`（`FF 36 lo hi` 兩道）。
func plusSeedAddr(t *testing.T, o *oracle.Oracle) (lo, hi uint32) {
	t.Helper()
	code := o.Bytes(addr(plusRandFn), 0x30)
	var found []uint32
	for i := 0; i+4 <= len(code); i++ {
		if code[i] == 0xFF && code[i+1] == 0x36 {
			found = append(found, uint32(code[i+2])|uint32(code[i+3])<<8)
		}
	}
	if len(found) != 2 {
		t.Fatalf("加強版 rand()（%#x）的碼裡找到 %d 個 push ds:[…]，該是 2：% x",
			plusRandFn, len(found), code)
	}
	// 先 push 高位再 push 低位（原版 `0xa3b0` 再 `0xa3ae`）。
	return found[1], found[0]
}

func TestZZMonthParityPlus(t *testing.T) {
	root := plusRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	if _, err := state.LoadScenario(c, state.Slot("001")); err != nil {
		t.Fatal(err)
	}
	o, err := oracle.Load(filepath.Join(root, "ASV.EXE"), root)
	if err != nil {
		t.Fatalf("加強版載不進來：%v", err)
	}
	defer o.Close()

	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	total := nMas + nSta + state.GeneralTableSize
	base := uint32(plusTablesBase)

	// 種子的 DS 位移，開機之後從 rand() 的碼讀出（plusSeedAddr）。
	var seedLo, seedHi uint32
	// 月底結算那一刻的三張表與亂數狀態（hook 在下面）。
	var atSettle []byte
	seedAtSettle := uint32(0)
	settleHits := 0
	// 亂數：每次 `RND(n)` 記一次，歸到「現在跑的是哪個郡」名下。
	// `RND(n <= 0)` 不抽（`0x103b4`），不算。
	randCalls := 0
	curDisp := -1
	curTable := "郡回合之外"
	randBy := map[int]int{}
	watch := -1
	if v := os.Getenv("SAN1_WATCH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			watch = n
		}
	}
	rndLog := []string{}
	o.OnCall(addr(plusRndFn), func(o *oracle.Oracle) {
		n := int(int16(o.Arg(0)))
		if curDisp == watch && atSettle != nil && seedLo != 0 {
			ds := uint32(o.DSReg()) * 16
			seed := uint32(o.Word(addr(ds+seedLo))) | uint32(o.Word(addr(ds+seedHi)))<<16
			next := seed*214013 + 2531011
			out := int((next >> 16) & 0x7fff)
			r := 0
			if n > 0 {
				r = out % n
			}
			rndLog = append(rndLog, fmt.Sprintf("%s RND(%d)=%d@%x", curTable, n, r, o.Caller().Linear()))
			// 電腦對電腦的每日一擲（`0x1d757`）：順手把四個 double 印出來
			//（攻品質 es:0x78、守品質 es:0xbc、攻戰力 es:0x9ce、守戰力 es:0x1608，
			// 段位址各自從 DS 的指標讀）。
			if c := o.Caller().Linear(); c == 0x1d757 || c == 0x1d9e9 {
				rd := func(ptr, off uint32) float64 {
					seg := uint32(o.Word(addr(ds+ptr))) * 16
					var b [8]byte
					for i := range b {
						b[i] = o.Byte(addr(seg + off + uint32(i)))
					}
					return math.Float64frombits(binary.LittleEndian.Uint64(b[:]))
				}
				rndLog = append(rndLog, fmt.Sprintf("[攻品質 %.6f 守品質 %.6f 攻 %.1f 守 %.1f 攻0 %.4f 守0 %.4f]",
					rd(0xa9d4, 0x78), rd(0xa9d6, 0xbc), rd(0xa9d8, 0x9ce), rd(0xa9da, 0x1608),
					rd(0xa9d8, 0x570), rd(0xa9da, 0x870)))
			}
		}
		if n <= 0 {
			return
		}
		randCalls++
		randBy[curDisp]++
	})
	// 真正的 LCG 步數：`rand()` 本體每進一次就是一步。與上面的 RND 呼叫數
	// 分開數，兩者對不上就是包裝與本體的關係想錯了。
	lcgSteps := 0
	o.OnCall(addr(plusRandFn), func(*oracle.Oracle) { lcgSteps++ })
	// 十八張分派表的呼叫端（`docs/spec/015` §8.4）：原版 `0xe926`–`0xec1b`
	// 的對應，表在 DS 的位移一律 +0x82。
	// 逐表的種子：每到一張表的呼叫端記一次（表名、當時的種子）。
	// 兩張表之間走了幾步 LCG 就是前一張表抽了幾次——`RND(0)` 那種
	// 不抽的呼叫自然不算，比數 hook 的次數準。
	type tblMark struct {
		table string
		seed  uint32
	}
	tblSeeds := map[int][]tblMark{}
	readSeed := func(o *oracle.Oracle) uint32 {
		ds := uint32(o.DSReg()) * 16
		return uint32(o.Word(addr(ds+seedLo))) | uint32(o.Word(addr(ds+seedHi)))<<16
	}
	for _, tb := range plusDispatchTables {
		name := tb.name
		o.OnCall(addr(tb.at), func(o *oracle.Oracle) {
			curTable = name
			if seedLo != 0 {
				tblSeeds[curDisp] = append(tblSeeds[curDisp], tblMark{name, readSeed(o)})
			}
			if curDisp == watch && atSettle != nil && curDisp >= 0 {
				rec := base + uint32(nMas) + uint32(curDisp)*176
				ws := uint32(o.Word(addr(uint32(o.DSReg())*16+plusWorkSegPtr))) * 16
				// 留守目標 `es:[0x2e6a]`（原版 `0x2e62`）與清單長度 `es:0xc`。
				rndLog = append(rndLog, fmt.Sprintf("〔%s 兵(百) %d 金 %d 米 %d 留守 %d〕", name,
					o.Word(addr(rec+16)), o.Word(addr(rec+18)), o.Word(addr(rec+20)),
					int16(o.Word(addr(ws+0x2e6a)))))
			}
		})
	}

	// 對白常式（`0x2f366`，裡面那一擲 `RND(8)` 在 `0x2f912`）：追的郡把
	// 呼叫端記下來，才分得出是哪一句對白在擲。
	o.OnCall(addr(0x2f366), func(o *oracle.Oracle) {
		if curDisp == watch && atSettle != nil {
			rndLog = append(rndLog, fmt.Sprintf("msg@%05x", o.Caller().Linear()))
		}
	})
	// 月底結算入口：第一次到的時候讀種子、拍盤面。
	// 第一場電腦對電腦的戰役：原版從出兵那一支直接進戰役入口
	// （`0xb590` → `1E25:0002`）。這一格之後的骰序含戰役結算，那一段
	// 是另一條對拍（戰術層／`AutoResolveAI`）的事，這支只釘到它之前。
	const plusBattleEntry = 0x1e25*16 + 0x2
	firstBattleAt := -1
	o.OnCall(addr(plusBattleEntry), func(*oracle.Oracle) {
		if atSettle != nil && firstBattleAt < 0 {
			firstBattleAt = curDisp
		}
	})
	o.OnCall(addr(plusMonthEndFn), func(o *oracle.Oracle) {
		settleHits++
		if atSettle != nil {
			return
		}
		ds := uint32(o.DSReg()) * 16
		seedAtSettle = uint32(o.Word(addr(ds+seedLo))) | uint32(o.Word(addr(ds+seedHi)))<<16
		atSettle = o.Bytes(addr(base), total)
		curDisp = -1
		// **帳從這一刻起算。** 開新局那個月電腦的郡已經跑過一輪，
		// 不清掉的話每一張表都會算成兩個月的量。
		randCalls, lcgSteps = 0, 0
		for k := range randBy {
			delete(randBy, k)
		}
		for k := range tblSeeds {
			delete(tblSeeds, k)
		}
	})

	// 郡回合入口：順序表、旗標、游標都在工作段裡。**位移用找的**——
	// 加強版的工作段版面與原版差幾個位元組（游標 `0x20f6` 對 `0x20f4`），
	// 順序表是 43 個 0..42 的排列、旗標是 43 格 `0xFFFF`／0，在入口那一刻
	// 掃一次就認得出來。
	var turnSeq []int
	turnFlags := make([]bool, 43)
	orderOff, flagsOff := -1, -1
	visited := map[int]int{}
	origSeed := map[int]uint32{}
	var atFirstTurn []byte
	cursorLog := []int{}
	o.OnCall(addr(plusTurnEntry), func(o *oracle.Oracle) {
		ds := uint32(o.DSReg()) * 16
		ws := uint32(o.Word(addr(ds+plusWorkSegPtr))) * 16
		cur := int(int16(o.Word(addr(ws + plusMonthCursor))))
		if orderOff < 0 {
			words := make([]int, 0x1400)
			for i := range words {
				words[i] = int(o.Word(addr(ws + uint32(i*2))))
			}
			for i := 0; i+43 <= len(words) && orderOff < 0; i++ {
				seen := make([]bool, 43)
				ok := true
				for j := 0; j < 43; j++ {
					v := words[i+j]
					if v < 0 || v > 42 || seen[v] {
						ok = false
						break
					}
					seen[v] = true
				}
				if ok {
					orderOff = i * 2
				}
			}
			for i := 0x1000; i+43 <= len(words) && flagsOff < 0; i++ {
				ok := true
				for j := 0; j < 43; j++ {
					if v := words[i+j]; v != 0xFFFF && v != 0 {
						ok = false
						break
					}
				}
				if ok && words[i] == 0xFFFF {
					flagsOff = i * 2
				}
			}
		}
		if atSettle != nil && atFirstTurn == nil {
			// 結算之後第一個郡回合：這一刻讀順序表與旗標，視窗從這裡起。
			atFirstTurn = o.Bytes(addr(base), total)
			dseg := uint32(o.Word(addr(ds+0xa7c0))) * 16
			t.Logf("難度格 es:0x310a（DS:[0xa7c0] 段）＝ %d", int16(o.Word(addr(dseg+0x310a))))
			if orderOff >= 0 {
				turnSeq = turnSeq[:0]
				for i := 0; i < 43; i++ {
					turnSeq = append(turnSeq, int(int16(o.Word(addr(ws+uint32(orderOff+i*2))))))
				}
			}
			if flagsOff >= 0 {
				for i := range turnFlags {
					turnFlags[i] = o.Word(addr(ws+uint32(flagsOff+i*2))) == 0xFFFF
				}
			}
		}
		if orderOff >= 0 && cur >= 0 && cur < 43 {
			at := int(int16(o.Word(addr(ws + uint32(orderOff+cur*2)))))
			curDisp = at
			curTable = "郡回合入口"
			if seedLo != 0 {
				tblSeeds[at] = append(tblSeeds[at], tblMark{"郡回合入口", readSeed(o)})
			}
			if atSettle != nil {
				visited[at]++
				origSeed[at] = uint32(o.Word(addr(ds+seedLo))) | uint32(o.Word(addr(ds+seedHi)))<<16
				cursorLog = append(cursorLog, cur)
			}
		}
	})

	s := observePlus(o)
	bootToNewGamePlus(t, o, caoCaoPick, 5)
	seedLo, seedHi = plusSeedAddr(t, o)
	t.Logf("加強版的亂數種子在 DS:%#x／%#x（原版 0xa3ae／0xa3b0）", seedLo, seedHi)
	if orderOff < 0 || flagsOff < 0 {
		t.Fatalf("工作段裡找不到順序表（%d）或旗標（%d）", orderOff, flagsOff)
	}
	t.Logf("工作段：順序表 +%#x、旗標 +%#x、游標 +%#x", orderOff, flagsOff, plusMonthCursor)
	// `SAN1_MONTH=n`：把原版的月份改寫成 n，讓視窗跨進 n+1 月——四季常式
	// 一年各只跑一次（`0x14c38` 比 1、4、7、10：春 `0x14c4c`、夏 `0x153fe`、
	// 秋 `0x158a6`、冬 `0x15cd0`），開新局是元月，不改寫就只看得到二月。
	// 月份在工作段的 `0x3f14`（分派器 `0x14c19` 讀的；原版 `0x3f08`）。
	// `SAN1_PLAGUE=1` 再把每個郡的民眾忠誠與土地價值歸零，瘟疫的兩道門
	// 一定過（原版那支同一組旋鈕，Issue #44）。
	month := 1
	if v := os.Getenv("SAN1_MONTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 12 {
			ds := uint32(o.DSReg()) * 16
			// 分派器讀的段值在 DS:0xa8fc，游標那一格用的是 DS:0xa8f8
			// （原版 0xa72a／0xa726 同一回事）——兩格要是同一個段。
			work := uint32(o.Word(addr(ds+plusWorkSegPtr))) * 16
			if w2 := uint32(o.Word(addr(ds+0xa8fc))) * 16; w2 != work {
				t.Fatalf("DS:0xa8f8 與 DS:0xa8fc 指的段不同（%#x／%#x），月份的位移要重讀", work, w2)
			}
			o.SetWord(addr(work+plusMonthOff), uint16(n))
			month = n
			t.Logf("月份改寫成 %d（視窗跨進 %d 月）", n, n%12+1)
		}
	}
	if envOr("SAN1_PLAGUE", "") != "" {
		for id := 1; id <= state.PrefectureCount; id++ {
			rec := base + uint32(nMas+id*176)
			o.SetByte(addr(rec+26), 0)
			o.SetByte(addr(rec+27), 0)
		}
		t.Log("每個郡的民眾忠誠與土地價值歸零：瘟疫必發")
	}

	// 玩家：內政 → 休息 → Y（與原版那支同一串鍵）。每一步都等到
	// 「等新鍵」那一道再送，早送會被吃掉。
	keys := strings.Split(envOr("SAN1_TURNKEY", "4\r|4\r|Y"), "|")
	mainBefore := s.mainAsk
	for _, k := range keys {
		waitPlusScan(t, o, "加強版玩家命令", 50_000_000)
		o.Drain()
		o.PressScan(k)
	}
	// 一路跑到下一次問主命令（中間會過月底結算與下個月的電腦郡）。
	waitBoot(t, o, "加強版下一次主命令", 3_000_000_000,
		func() bool { return s.mainAsk > mainBefore && atSettle != nil })
	if atSettle == nil {
		t.Fatal("沒攔到月底結算——亂數對不起來")
	}
	after := o.Bytes(addr(base), total)
	t.Logf("月底結算入口攔到 %d 次；結算之後走到 %d 個郡；原版 RND 呼叫 %d 次、LCG %d 步",
		settleHits, len(visited), randCalls, lcgSteps)
	t.Logf("結算之後的游標序列：%v", cursorLog)
	if watch >= 0 {
		t.Logf("郡 %d 的原版 RND 逐次：%v", watch, rndLog)
	}
	t.Logf("順序表：%v", turnSeq)
	done := []int{}
	for i, ok := range turnFlags {
		if !ok {
			done = append(done, i)
		}
	}
	t.Logf("結算之後第一個郡回合時旗標已清的郡：%v", done)
	t.Logf("原版：結算 → 又輪到玩家之間動了 %d 個位元組%s",
		differs8(atSettle, after), where(atSettle, after, nMas, nSta))

	// remake：從結算那一刻的三張表接手。
	sc, err := state.DecodeTables(state.Slot("001"),
		atSettle[:nMas], atSettle[nMas:nMas+nSta], atSettle[nMas+nSta:])
	if err != nil {
		t.Fatalf("原版記憶體裡的三張表解不開：%v", err)
	}
	players := sc.Players()
	if len(players) != 1 {
		t.Fatalf("盤面上的玩家 %v，該只有曹操一個", players)
	}
	player := state.FactionID(players[0])
	// 開新局是 189 年元月；結算入口那一刻月份還沒推進。
	g, err := game.Continue(sc, player, 5, state.EditionPlus, game.Date{Year: 189, Month: month})
	if err != nil {
		t.Fatal(err)
	}
	brain, err := ai.New(ai.ModePlus)
	if err != nil {
		t.Fatal(err)
	}
	pp, ok := brain.(ai.PrefecturePlanner)
	if !ok {
		t.Fatalf("%s 不支援逐郡執行", brain.Name())
	}
	mineTbl := map[string]int{}
	pp.TraceDraws(mineTbl)
	g.SeedRand(seedAtSettle)
	phase := map[string]int{}
	phaseSeed := map[string]uint32{}
	g.TracePhases(phase, phaseSeed)
	t.Logf("兩邊都從月底結算那一刻的亂數狀態 0x%08x 接上", seedAtSettle)
	g.EndMonth()
	for _, k := range []string{"換月", "物價", "洗牌", "四季"} {
		t.Logf("remake 換月各段：%s %d 次（之後 0x%08x）", k, phase[k], phaseSeed[k])
	}
	// 順序表：remake 自己洗出來的要與原版那份相同，否則後面全歪。
	mineSeq := g.TurnOrder()
	if fmt.Sprint(mineSeq) != fmt.Sprint(turnSeq) {
		t.Errorf("開月洗牌的順序表不同：\n原版   %v\nremake %v", turnSeq, mineSeq)
	}
	// 視窗 [0, 玩家的郡)：原版在玩家的郡停下來問主命令。
	winTo := len(turnSeq)
	for i, at := range turnSeq {
		if q := g.Prefecture(at); q != nil && q.Owner == player {
			winTo = i
			break
		}
	}
	mineBy := map[int]int{}
	mineSeed := map[int]uint32{}
	mineTblBy := map[int]map[string]int{}
	firstGap := -1
	for i := 0; i < winTo && i < len(turnSeq); i++ {
		g.TurnTick()
		at := turnSeq[i]
		mineSeed[at] = g.RandSeed()
		g.RefreshGarrison(at)
		q := g.Prefecture(at)
		if q == nil || !q.Owned() {
			continue
		}
		g.RecomputeOwners()
		if q = g.Prefecture(at); q == nil || !q.Owned() || q.Owner == player {
			continue
		}
		if s0, ok := origSeed[at]; ok && firstGap < 0 && s0 != g.RandSeed() {
			firstGap = i
			t.Logf("第一個岔開：順序表第 %d 格（郡 %d，勢力 %d）開始前原版 0x%08x、remake 0x%08x",
				i, at, q.Owner, s0, g.RandSeed())
		}
		d0 := g.RandDraws()
		if at == watch {
			ids := []int{}
			for _, x := range g.Garrison(at) {
				ids = append(ids, x.Index)
			}
			t.Logf("郡 %d 回合前 remake 的守軍：%v", at, ids)
		}
		before := map[string]int{}
		for k, v := range mineTbl {
			before[k] = v
		}
		var mineRolls []string
		if at == watch {
			g.TraceRolls(func(n, out int, salt []int) {
				tag := ""
				if len(salt) > 0 {
					tag = fmt.Sprintf("@%x", salt[len(salt)-1])
				}
				mineRolls = append(mineRolls, fmt.Sprintf("RND(%d)=%d%s", n, out, tag))
			})
		}
		if _, n, err := pp.ActPrefecture(g, q.Owner, at, g.AILevel(q.Owner)); err != nil {
			t.Errorf("郡 %d（勢力 %d）的命令有 %d 道成立，然後：%v", at, q.Owner, n, err)
		}
		g.FinishTurn(at)
		if at == watch {
			g.TraceRolls(nil)
			t.Logf("郡 %d 的 remake Roll 逐次：%v", at, mineRolls)
		}
		mineBy[at] = g.RandDraws() - d0
		if at == watch {
			ids := []int{}
			for _, x := range g.Garrison(at) {
				ids = append(ids, x.Index)
			}
			t.Logf("郡 %d 回合後 remake 的守軍：%v", at, ids)
		}
		mineTblBy[at] = map[string]int{}
		for k, v := range mineTbl {
			if d := v - before[k]; d != 0 {
				mineTblBy[at][k] = d
			}
		}
	}
	rm, rs, rg, err := g.Tables()
	if err != nil {
		t.Fatal(err)
	}
	mine := append(append(append([]byte{}, rm...), rs...), rg...)
	if dir := os.Getenv("SAN1_DUMP"); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, "plus-month-settle.bin"), atSettle, 0o644)
		_ = os.WriteFile(filepath.Join(dir, "plus-month-orig.bin"), after, 0o644)
		_ = os.WriteFile(filepath.Join(dir, "plus-month-remake.bin"), mine, 0o644)
	}
	t.Log("照順序表的次序（抽亂數的次數，標 ← 的是第一個對不上的）：")
	for i := 0; i < winTo && i < len(turnSeq); i++ {
		at := turnSeq[i]
		// 原版這一格的「本體」抽樣：入口到下一個入口的 LCG 步數，扣掉
		// 中間每一格月迴圈的那一抽（`0x15790` 的對應，跳過的格子也抽）。
		next, cells := -1, 0
		for j := i + 1; j < len(turnSeq); j++ {
			cells++
			if turnSeq[j] != 0 {
				next = turnSeq[j]
				break
			}
		}
		origBody := -1
		if next >= 0 {
			if n := lcgStepsBetween(origSeed[at], origSeed[next], 2000); n >= 0 {
				origBody = n - cells
			}
		}
		randBy[at] = origBody
		flag := ""
		if origBody != mineBy[at] {
			flag = "  ←"
		}
		seedFlag := ""
		if origSeed[at] != mineSeed[at] {
			seedFlag = " ≠"
		}
		t.Logf("    第 %2d 格 郡 %2d：原版 %3d／remake %3d%s　入口種子 原版 %08x／remake %08x%s",
			i, at, randBy[at], mineBy[at], flag, origSeed[at], mineSeed[at], seedFlag)
		if origBody != mineBy[at] && mineBy[at] != 0 {
			// 種子法：這一張表的步數 ＝ 到下一張表（或下一個郡的入口）之間的 LCG 步數。
			marks := tblSeeds[at]
			next := origSeed[turnSeqNext(turnSeq, i)]
			line := ""
			for j, m := range marks {
				to := next
				if j+1 < len(marks) {
					to = marks[j+1].seed
				}
				if n := lcgStepsBetween(m.seed, to, 400); n != 0 {
					line += fmt.Sprintf(" %s=%d", m.table, n)
				}
			}
			t.Logf("        原版逐表（種子法）：%s", line)
			t.Logf("        remake 逐表：%s", tableLine(plusDispatchTables, mineTblBy[at]))
		}
	}
	for _, id := range []int{12, 22, 13} {
		a := nMas + id*state.PrefectureRecordSize
		rec := func(b []byte) string {
			return fmt.Sprintf("所屬 %d 主事者 %d 兵(百) %d 在職 %d 金 %d 米 %d",
				b[a+30], int16(uint16(b[a+32])|uint16(b[a+33])<<8),
				uint16(b[a+16])|uint16(b[a+17])<<8, b[a+22],
				uint16(b[a+18])|uint16(b[a+19])<<8, uint16(b[a+20])|uint16(b[a+21])<<8)
		}
		t.Logf("郡 %d：結算時 %s｜原版走完 %s｜remake 走完 %s", id, rec(atSettle), rec(after), rec(mine))
	}
	keys2 := make([]string, 0, len(mineTbl))
	for k := range mineTbl {
		keys2 = append(keys2, k)
	}
	sort.Strings(keys2)
	t.Logf("remake 逐表抽樣：%v", func() string {
		out := ""
		for _, k := range keys2 {
			out += fmt.Sprintf(" %s=%d", k, mineTbl[k])
		}
		return out
	}())
	t.Logf("原版 vs remake：差 %d 個位元組%s",
		differs8(after, mine), where(after, mine, nMas, nSta))
	t.Log(byPrefecture(after, mine, nMas, nSta))
	t.Log(byGeneral(after, mine, nMas, nSta))
	dumpTables(t, atSettle, "plusparity-005-結算前")
	dumpTables(t, after, "plusparity-01-原版走完")
	dumpTables(t, mine, "plusparity-02-remake走完")
	// **釘住的是整個月**（Issue #29）：換月之後的每一個郡，入口的亂數狀態
	// 與本體的抽樣次數兩邊都要相同——十八張表的骰序逐格對上，含電腦對
	// 電腦的戰役（原版在出兵裡就地結算：每天 `RND(11)`、打殘 `RND(10)`×4、
	// 收降有牽絆的人 `RND(30)`，remake 這幾擲從同一顆 `rand()` 接）。
	firstAt := -1
	for i, at := range turnSeq {
		if at == firstBattleAt {
			firstAt = i
			break
		}
	}
	t.Logf("第一場電腦戰役：郡 %d（順序表第 %d 格）", firstBattleAt, firstAt)
	for i := 0; i < len(turnSeq) && i < winTo; i++ {
		at := turnSeq[i]
		if at == 0 {
			continue
		}
		if origSeed[at] != mineSeed[at] {
			t.Errorf("順序表第 %d 格（郡 %d）入口的亂數狀態不同：原版 %08x、remake %08x",
				i, at, origSeed[at], mineSeed[at])
		}
		if randBy[at] >= 0 && randBy[at] != mineBy[at] {
			t.Errorf("順序表第 %d 格（郡 %d）本體抽樣不同：原版 %d、remake %d",
				i, at, randBy[at], mineBy[at])
		}
	}
	// **這是硬閘門**，與原版那支相同（`month_parity_test.go`）：整個月的
	// 三張表逐位元組相同。
	if n := differs8(after, mine); n != 0 {
		t.Errorf("月度對拍差 %d 個位元組（應該是 0）", n)
	}
}
