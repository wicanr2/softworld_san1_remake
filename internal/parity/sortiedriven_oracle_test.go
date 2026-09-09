//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/text/encoding/traditionalchinese"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 出兵：攔原版自己的輸入常式，它問什麼就答什麼。
//
// **按鍵序列是原版的狀態機決定的，從外面推不出來。** 前兩種方法都否定過：
//
//  1. 照秒數補鍵——補到「攜帶多少米」就停住，而多送一個空 Enter 會讓整編
//     整個重來（五個隊歸零）。
//  2. 監看字串區的讀取，認出提示再送鍵——主選單、子選單、「從那一郡攻打」、
//     「攻打那一郡」都認得準，然後卡在整編。原因是**整編畫面在建立時就把
//     自己所有的字串讀過一遍**，所以在那個畫面裡「這條字串被讀過」不代表
//     「現在正在問這一句」。
//
// 第三種才對：原版有三支專用的輸入常式（`docs/re/03` §1.5、`docs/re/05` §7.0），
// **被呼叫的那一刻就是「它正在問」**，而且該不該送 Enter 由是哪一支決定：
//
//	33d8:115e（線性 0x34ede）數字欄位，收 0-9／退格，**要 Enter**；
//	                         空欄位按 Enter 回 0xFFFF ＝ 取消
//	0x1538c                  讀一個鍵，Y/N 確認用，**不要 Enter**
//	1058:0e24                讀一個 ASCII，戰場的每日命令用，**不要 Enter**
//
// 提示字串仍然有用，只是取樣時機改成「呼叫的當下往回拿最近讀到的那一條」
// ——而不是整輪的窗口。攔到的當下才清紀錄，窗口就正好是「上一個問題到
// 這一個問題之間畫出來的東西」。
//
// **預設答案是 `Y` 不是 Enter。** 對白的「請按任一鍵」吃 Y，軍師勸諫只有
// N 會取消（`0x18b26`），「分配完畢(Y/N)」的 Y 是收工——Enter 才是那個
// 會把整編答成 N 的危險鍵。認不出來的數字欄位一律停下來倒畫面，不亂猜。
// corpsRoster 回傳四個軍團五支部隊裡已經填上的將領槽號。
// 部隊記錄 42 bytes、每個軍團留十格只用前五格、基底 `es:[0x3502]`，
// 將領欄 `0xFFFF` ＝ 空（`docs/re/05` §3.3）。
func corpsRoster(o *oracle.Oracle, work uint16) [][]int {
	out := make([][]int, 4)
	for c := 0; c < 4; c++ {
		for u := 0; u < 5; u++ {
			rec := uint16(0x3502 + (c*10+u)*42)
			for k := 0; k < 10; k++ {
				v := o.Word(oracle.Addr{Seg: work, Off: rec + uint16(k*2)})
				if v != 0xFFFF {
					out[c] = append(out[c], int(v))
				}
			}
		}
	}
	return out
}

func TestZZPlayerSortieDriven(t *testing.T) {
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

	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize,
		state.GeneralTableSize
	total := nMas + nSta + nGen
	board := func() []byte { return o.Bytes(addr(base), total) }
	work := o.ES()
	at := int(o.Word(oracle.Addr{Seg: work, Off: curPrefOff}))

	raw := board()
	sc, err := state.DecodeTables(state.Slot("001"),
		raw[:nMas], raw[nMas:nMas+nSta], raw[nMas+nSta:])
	if err != nil {
		t.Fatal(err)
	}
	me := state.FactionID(sc.Players()[0])
	to := 0
	for k := 45; k <= 54; k++ {
		n := int(raw[nMas+at*state.PrefectureRecordSize+k])
		if n == 0xFF || n == 0 {
			continue
		}
		if int(raw[nMas+n*state.PrefectureRecordSize+30]) != int(me) {
			to = n
			break
		}
	}
	if to == 0 {
		t.Skip("沒有可以打的鄰郡")
	}
	// 錢糧擺足，免得被「錢不夠」擋掉而看不出是哪一種拒絕。
	purse := base + uint32(nMas+at*state.PrefectureRecordSize)
	o.SetWord(addr(purse+18), 9000)
	o.SetWord(addr(purse+20), 9000)
	before := board()
	t.Logf("郡 %d 出兵打郡 %d", at, to)

	enc := traditionalchinese.Big5.NewEncoder()
	dec := traditionalchinese.Big5.NewDecoder()
	big5 := func(s string) []byte {
		b, err := enc.Bytes([]byte(s))
		if err != nil {
			t.Fatalf("%q 編不成 Big5：%v", s, err)
		}
		return b
	}
	var lo, hi uint32
	for _, probe := range []string{"1.調動軍隊", "1.訓練兵士", "1.土地開發"} {
		for _, h := range o.Search(big5(probe)) {
			if lo == 0 || h < lo {
				lo = h
			}
			if h > hi {
				hi = h
			}
		}
	}
	if lo == 0 {
		t.Fatal("找不到字串區")
	}
	lo -= 0x800
	hi += 0x2000
	if hi-lo > 0xF000 {
		hi = lo + 0xF000
	}
	log := o.WatchReadsAt(lo, hi)
	defer o.StopWatchingReads()

	// stringAt：從一個被讀到的位移往回走到 NUL，回推整條字串。
	stringAt := func(off uint16) string {
		a := lo + uint32(off)
		for i := 0; i < 96 && a > lo; i++ {
			if o.Byte(addr(a-1)) == 0 {
				break
			}
			a--
		}
		b := o.Bytes(addr(a), 96)
		if i := indexZero(b); i >= 0 {
			b = b[:i]
		}
		s, err := dec.Bytes(b)
		if err != nil {
			return ""
		}
		return string(s)
	}
	// clusters：這一段窗口裡讀過的字串，分群後各回推一條。只當診斷用——
	// 整編畫面一次把自己所有的字串讀完，這份名單裡有一半不是現在在問的。
	clusters := func() []string {
		seen := map[uint16]bool{}
		for _, r := range *log {
			seen[r.Off] = true
		}
		offs := make([]int, 0, len(seen))
		for off := range seen {
			offs = append(offs, int(off))
		}
		sort.Ints(offs)
		var out []string
		last := -99
		for _, off := range offs {
			if off-last <= 3 {
				last = off
				continue
			}
			last = off
			if s := stringAt(uint16(off)); s != "" {
				out = append(out, s)
			}
		}
		return out
	}

	// 原版的三支輸入常式。**被呼叫 ＝ 它正在問**，這是狀態機自己說的，
	// 不是從外面推的。
	const (
		numInputFn = 0x33d8*16 + 0x115e // 數字欄位，要 Enter
		keyInputFn = 0x1538c            // 讀一個鍵
		ascInputFn = 0x1058*16 + 0xe24  // 讀一個 ASCII
	)
	type ask struct {
		kind   string
		lo, hi int
		prompt string // 呼叫當下**最後**讀到的那一條，就是提示本身
		seen   []string
		at     uint64
	}
	var asks []ask
	note := func(kind string, a, b int) {
		p := ""
		if n := len(*log); n > 0 {
			last := (*log)[0]
			for _, r := range *log {
				if r.Step >= last.Step {
					last = r
				}
			}
			p = stringAt(last.Off)
		}
		asks = append(asks, ask{kind: kind, lo: a, hi: b,
			prompt: p, seen: clusters(), at: o.Steps()})
		// 攔到的當下清掉，下一個問題的窗口就正好是「這一問到下一問之間」。
		*log = (*log)[:0]
	}
	o.OnCall(addr(numInputFn), func(o *oracle.Oracle) {
		note("數字", int(o.Arg(0)), int(o.Arg(1)))
	})
	o.OnCall(addr(keyInputFn), func(o *oracle.Oracle) { note("按鍵", 0, 0) })
	o.OnCall(addr(ascInputFn), func(o *oracle.Oracle) { note("字元", 0, 0) })

	// 腳本。順序固定，但**什麼時候問、問哪一種、範圍多少由原版說了算**
	// ——這三件事全部來自 hook，不是猜的。`kind`／`lo`／`hi` 是交叉檢查：
	// 對不上就停下來，不硬答。
	type reply struct {
		want, keys, kind string
		lo, hi           int // hi == 0 ＝ 不檢查
	}
	// ⚠ **整編是個迴圈，不是一串固定的步驟**：`0x20a30` 每個軍團一輪
	//（順序主守 → 助守 → 主攻 → 助攻，援軍格 `0xFFFF` 的跳過），一輪裡
	// 「分配那一位將軍 → 分到那一軍 → 分配完畢(Y/N)」照 `N 再分一位、
	// Y 收工` 反覆問（`docs/re/05` §7）。所以這幾句是**規則**不是步驟
	// ——問幾次答案都一樣。把它們寫成第幾步就會像前四種驅動法那樣，
	// 答完一輪就去等一句永遠不會來的「攜帶多少金」。
	//
	// 陣營碼 0（主守軍）的錢糧直接取郡的全部、不問（`0x20ac3`）。
	rules := []reply{
		{want: "發動戰役", keys: "2", kind: "數字", lo: 1, hi: 3},
		{want: "從那一郡攻打", keys: fmt.Sprintf("%d", at), kind: "數字", lo: 1, hi: 42},
		{want: "攻打那一郡", keys: fmt.Sprintf("%d", to), kind: "數字", lo: 1, hi: 42},
		{want: "分到那一軍", keys: "1", kind: "數字", lo: 1, hi: 5},
		{want: "分配那一位將軍", keys: "1", kind: "數字", lo: 1},
		{want: "分配完畢", keys: "Y"},
		{want: "攜帶多少金", keys: "100", kind: "數字"},
		{want: "攜帶多少米", keys: "100", kind: "數字"},
	}

	// answer 回傳 (要送的鍵, 認不認得)。
	//
	// **先用 hook 給的 `(kind, lo, hi)` 篩掉不可能的規則再比字串。**
	// 範圍是原版自己傳給輸入常式的參數，比字串可靠：整編畫面一次把自己
	// 所有的字串讀過一遍，光看「窗口裡出現過這一句」會提早答到還沒問的
	// 那一句，而 `(1-5)` 與 `(1-7)` 一眼就分得開。
	//
	// 字串仍然要比，而且**先比呼叫當下最後讀到的那一條**：「從那一郡攻打」
	// 印完之後還會畫一份郡名清單，最後一條讀取是清單的格式字串 `%2d%s`
	// 不是提示，這種才需要退回整個窗口。
	answer := func(a ask) (string, bool) {
		for _, seen := range append([]string{a.prompt}, a.seen...) {
			// 「請按任一鍵」吃任何鍵；每個軍團整編前各一次。
			if strings.Contains(seen, "請按任一鍵") {
				return "Y", true
			}
		}
		fits := func(r reply) bool {
			return (r.kind == "" || r.kind == a.kind) &&
				(r.lo == 0 || r.lo == a.lo) && (r.hi == 0 || r.hi == a.hi)
		}
		for _, r := range rules {
			if fits(r) && strings.Contains(a.prompt, r.want) {
				return r.keys, true
			}
		}
		for _, r := range rules {
			if !fits(r) {
				continue
			}
			for _, seen := range a.seen {
				if strings.Contains(seen, r.want) {
					return r.keys, true
				}
			}
		}
		// 認不出來：對白與 Y/N 送 Y（軍師勸諫只有 N 會取消，`0x18b26`；
		// 分配完畢的 Y 是收工），數字欄位一律停下來——猜一個數字進去是
		// 「進得去、回不來」，範圍檢查不回錯誤碼而是把欄位重畫再讀
		//（`0x3503e`）。
		if a.kind != "數字" {
			return "Y", true
		}
		return "", false
	}

	purseMoved := func() bool {
		return o.Word(addr(purse+18)) != 9000 || o.Word(addr(purse+20)) != 9000
	}
	dumpAsks := func() {
		for i, a := range asks {
			t.Logf("  #%02d %s(%d-%d) @%d 提示「%s」窗口 %q", i, a.kind,
				a.lo, a.hi, a.at,
				strings.ReplaceAll(a.prompt, "\n", "\\n"), a.seen)
		}
	}
	// **開機結束時原版已經在輸入常式裡面等了。** `OnCall` 看得到「呼叫」，
	// 看不到「已經在裡面」——所以第一個問題（主命令提示，`0x176eb` 的
	// 數字輸入）攔不到，得照既有的十三道那樣直接答掉，之後才交給 hook。
	o.Drain()
	for j, r := range "2\r" {
		if j > 0 {
			if err := o.Run(keyGap); err != nil {
				t.Fatalf("送第一個鍵時停止：%v", err)
			}
		}
		o.PressScan(string(r))
	}
	// 一輪跑多久、最多補幾次鍵。四千萬條是 `TestZZDumpBattleCode` 量出來
	// 夠一個畫面換完的值。
	const roundBudget, maxNudge = 40_000_000, 12
	answered, nudge, asked := 0, 0, 0
	for round := 0; round < 400 && !purseMoved(); round++ {
		next := oracle.NewCond("原版在問下一個問題", func(*oracle.Oracle) bool {
			return len(asks) > answered || purseMoved()
		})
		err := o.RunUntil(next, oracle.Budget(roundBudget))
		if len(asks) <= answered {
			if purseMoved() {
				break
			}
			// ⚠ **有些提問不經過那三支常式**，其中最要命的是整編的
			// 「分配完畢(Y/N)」——它只在窗口裡看得到字串，沒有自己的
			// 攔截點。所以沒有人在問的時候補一個鍵推它一下，**而且那個鍵
			// 一定要是 `Y`**：Enter 在「分配完畢」等於 `N`（再分一位），
			// 量到的就是「分配那一位將軍」問上千次的無限迴圈。
			// 補完仍然等真正的提問才作答——不像盲送那樣搶在問題出現前
			// 就把數字按掉（欄位空著吃到 Enter 是當場取消重問，`0x1d4ec`）。
			if nudge++; nudge <= maxNudge {
				o.PressScan("Y")
				continue
			}
			dumpScreen(t, o, "sortie-driven")
			dumpAsks()
			t.Fatalf("補了 %d 次鍵還是沒有人在問（已答 %d 題，指令數 %d）：%v",
				maxNudge, answered, o.Steps(), err)
		}
		nudge = 0
		// ⚠ **上一題之後什麼字都沒印，就不是新的問題。** 數字欄位是用
		// `1058:0e24` 一個字元一個字元讀的，所以送「1⏎」會多出兩筆
		// 空提示、空窗口的「字元」——那是同一題的內部讀鍵。
		// **千萬不能去回答它們**：回答前的 `o.Drain()` 會把還沒被取走的
		// 那個 Enter 洗掉，欄位永遠等不到 Enter 就重問，量到的就是
		// 「分配那一位將軍」一路問到底的無限迴圈。
		for answered < len(asks) && asks[answered].kind == "字元" &&
			asks[answered].prompt == "" && len(asks[answered].seen) == 0 {
			answered++
		}
		if answered >= len(asks) {
			continue
		}
		a := asks[answered]
		answered++
		keys, ok := answer(a)
		if !ok {
			dumpScreen(t, o, "sortie-driven")
			dumpAsks()
			t.Fatalf("認不出這個數字欄位（%d-%d），這一問窗口裡讀到的是 %q",
				a.lo, a.hi, a.seen)
		}
		// 數字欄位一定要 Enter，**一位數的也要**（`docs/re/03` §1.5）。
		// 「一位數欄位滿了會自己收工、多的 Enter 漏到下一個讀鍵點」量過，
		// 不成立：不送 Enter 的話 `(1-3)` 的軍事子選單當場就不動了。
		if a.kind == "數字" {
			keys += "\r"
		}
		t.Logf("#%02d %s(%d-%d)「%s」→ 送 %q　編成 %v", answered-1, a.kind,
			a.lo, a.hi, strings.ReplaceAll(a.prompt, "\n", "\\n"), keys,
			corpsRoster(o, work))
		if strings.Contains(a.prompt, "分配那一位將軍") {
			if asked++; asked > 12 {
				dumpAsks()
				t.Fatal("「分配那一位將軍」問了十二次還沒進到下一句——" +
					"看上面每一問的編成有沒有長，就知道是分配沒生效還是被退回")
			}
		}
		// **這一刻才是清鍵盤的安全點**：原版剛進輸入常式，該收的都收了，
		// 留在緩衝區的一定是我上一次多送的。中途清會吃掉對白還沒取走的鍵。
		o.Drain()
		for j, r := range keys {
			if j > 0 {
				if err := o.Run(keyGap); err != nil {
					t.Fatalf("送鍵時停止：%v", err)
				}
			}
			o.PressScan(string(r))
		}
		// 讓最後一個鍵被取走再往下走，否則它會活到下一次 `Drain`。
		if err := o.Run(keyGap); err != nil {
			t.Fatalf("送完鍵之後停止：%v", err)
		}
		// ⚠ **空提示的「字元」不能丟掉。** 它們看起來像是數字欄位自己的
		// 內部讀鍵（`33d8:115e` 用 `1058:0e24` 一個字元一個字元讀），
		// 丟掉不餵卻會讓後面的鍵被它們吃掉——量到的是「11⏎」送出去，
		// 原版把欄位重畫一次再問（`0x1d4ec` 的不在清單裡就重來，
		// 症狀正是「進得去、回不來」）。一律餵 Y：非數字會被欄位丟掉，
		// 對白與 Y/N 則剛好答對。
	}
	dumpAsks()
	// 最後的確認之後才扣錢糧，所以等郡庫動了再取樣。
	changed := oracle.NewCond("郡庫動了", func(o *oracle.Oracle) bool {
		return o.Word(addr(purse+18)) != 9000 || o.Word(addr(purse+20)) != 9000
	})
	if err := o.RunUntil(changed, oracle.Budget(6*playerSettle)); err != nil {
		dumpScreen(t, o, "sortie-driven-end")
		t.Fatalf("整編走完了但郡庫沒動：%v", err)
	}
	after := board()

	g, err := game.New(sc, me, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	// **編進去的是誰，問原版自己**——別假設它照槽號挑。部隊記錄 42 bytes、
	// 每個軍團 5 個、基底 `es:[0x3502]`，將領欄 `0xFFFF` ＝ 空；攻方郡是
	// 陣營碼 2，所以是第 2 個軍團（`docs/re/05` §1、§3.3）。
	const unitRecAt, unitRecSize, corpsUnits = 0x3502, 42, 10
	var picked []int
	for u := 0; u < 5; u++ {
		rec := uint16(unitRecAt + (2*corpsUnits+u)*unitRecSize)
		for k := 0; k < 10; k++ {
			v := o.Word(oracle.Addr{Seg: work, Off: rec + uint16(k*2)})
			if v != 0xFFFF && int(v) < nGen/state.GeneralRecordSize {
				picked = append(picked, int(v))
			}
		}
	}
	if len(picked) == 0 {
		t.Fatal("部隊記錄裡一個將領都沒有——整編沒走完")
	}
	t.Logf("原版編進攻方軍團的槽號 %v", picked)
	if _, err := g.BeginAttack(at, to, picked, me,
		game.Supply{Gold: 100, Rice: 100}); err != nil {
		t.Fatalf("remake 這一邊：%v", err)
	}
	rm, rs, rg, err := g.Tables()
	if err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 0, total)
	got = append(append(append(got, rm...), rs...), rg...)

	t.Logf("原版動了 %d 個位元組；動到的記錄：%s",
		diffCount(before, after), changedRecords(before, after, nMas, nSta))
	t.Logf("兩邊不同的記錄：%s", changedRecords(after, got, nMas, nSta))
	off := nMas + at*state.PrefectureRecordSize
	for i := off; i < off+state.PrefectureRecordSize; i++ {
		if after[i] != got[i] {
			t.Errorf("州郡表第 %d 筆位移 %d：原版 %d／remake %d（原本 %d）",
				at, i-off, after[i], got[i], before[i])
		}
	}
}
