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

// 出兵：照原版當下在問什麼決定送什麼。
//
// **按鍵序列是原版的狀態機決定的，從外面推不出來。** 照秒數補鍵補到
// 「攜帶多少米」就停住，而多送一個空 Enter 會讓整編整個重來（五個隊歸零）
// ——連續兩輪卡在同一格之後就該換方法（`rulebook/40`）。
//
// 換的方法是把 `TestZZPlayerMenuPrompts` 那個儀器改成即時的：監看字串區的
// 讀取，每跑一小段就看原版**剛剛讀了哪些字串常數**，認出提示再送對應的鍵。
// 認不出來就送 Enter——對白與「請按任一鍵」都是這樣過的。
// ⚠ **這一支目前走不完，卡在整編。** 原因量出來了：
// **整編畫面在建立時就把自己所有的字串讀過一遍**，所以「這條字串被讀過」
// 在那個畫面裡不代表「現在正在問這一句」——第一輪之所以「認出分配完畢」
// 是誤判，那是畫面初始化時讀到的。監看字串讀取對**畫面上一句一句換的
// 提示**有效（主選單、子選單、從那一郡攻打、攻打那一郡都認得準），
// 對**一次畫完的整編版面**無效。
//
// 下一個槓桿不是字串而是**盤面狀態**：部隊記錄 42 bytes、每個軍團五個、
// 基底 `es:[0x3502]`，將領欄 `0xFFFF` ＝ 還沒分配（`docs/re/05` §3.3）。
// 分配一位將軍就會有一格從 `0xFFFF` 變成槽號——那是看得見、不會誤判的信號。
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

	// prompts 回傳「剛剛讀過的字串」。
	prompts := func() []string {
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
			a := lo + uint32(off)
			for i := 0; i < 64 && a > lo; i++ {
				if o.Byte(addr(a-1)) == 0 {
					break
				}
				a--
			}
			b := o.Bytes(addr(a), 64)
			if i := indexZero(b); i >= 0 {
				b = b[:i]
			}
			if s, err := dec.Bytes(b); err == nil {
				out = append(out, string(s))
			}
		}
		return out
	}

	// 腳本：認出提示就送對應的鍵。**順序固定，但什麼時候送由原版決定。**
	script := []struct{ want, keys string }{
		{"下您的命令", "2\r"},
		{"發動戰役", "2\r"},
		{"從那一郡攻打", fmt.Sprintf("%d\r", at)},
		{"攻打那一郡", fmt.Sprintf("%d\r", to)},
		{"分配那一位將軍", "1\r"},
		{"分到那一軍", "1\r"},
		{"分配完畢", "Y\r"},
		{"攜帶多少金", "100\r"},
		{"攜帶多少米", "100\r"},
	}
	// noEnterFrom：從這一步開始不再亂送 Enter（`分到那一軍` 之後全是
	// 有提示可認的欄位與 Y/N）。
	const noEnterFrom = 5
	// **每輪清、但記著最近幾輪認出的提示。**
	//
	// 兩種極端都不行：每輪清會漏掉「上一輪尾巴才畫出來」的提示；
	// 完全不清則會讓名單重繪把整段字串區讀過一遍，相鄰的字串併成同一叢，
	// 回推出來的開頭就不是那一條了。分群要在小窗上做，記憶另外留。
	var recent []string
	step, idle := 0, 0
	for round := 0; round < 120 && step < len(script); round++ {
		if err := o.Run(30_000_000); err != nil {
			t.Fatalf("第 %d 輪停止：%v", round, err)
		}
		recent = append(recent, prompts()...)
		*log = (*log)[:0]
		if len(recent) > 400 {
			recent = recent[len(recent)-400:]
		}
		hit := ""
		for _, p := range recent {
			if strings.Contains(p, script[step].want) {
				hit = p
				break
			}
		}
		if hit == "" {
			// **Enter 只能用在對白。** 進了整編之後，認不出提示時送
			// Enter 會把「分配完畢(Y/N)」當成 N 答掉，整編重來（五個隊
			// 歸零）——寧可多等幾輪，也不要亂送鍵。
			if step >= noEnterFrom {
				continue
			}
			// **紀錄不要每輪清掉。** 提示只被讀一次；它如果是在上一輪的
			// 尾巴畫出來的，這一輪就看不到——然後就會一直送 Enter，
			// 而那正是讓整編重來的動作。累積到認出來為止才清。
			//
			// 對白與「請按任一鍵」沒有可認的提示，靠 Enter 過；
			// 但要等幾輪確定不是「還沒畫出來」再送。
			if idle++; idle >= 3 {
				o.PressScan("\r")
				idle = 0
			}
			continue
		}
		idle = 0
		recent = recent[:0]
		t.Logf("第 %2d 輪 認出「%s」→ 送 %q", round,
			strings.ReplaceAll(hit, "\n", "\\n"), script[step].keys)
		for j, r := range script[step].keys {
			if j > 0 {
				if err := o.Run(keyGap); err != nil {
					t.Fatalf("送鍵時停止：%v", err)
				}
			}
			o.PressScan(string(r))
		}
		step++
	}
	if step < len(script) {
		dumpScreen(t, o, "sortie-driven")
		t.Fatalf("腳本只走到第 %d 步（等「%s」）", step, script[step].want)
	}
	// 最後的確認之後才扣錢糧，所以等郡庫動了再取樣。
	changed := oracle.NewCond("郡庫動了", func(o *oracle.Oracle) bool {
		return o.Word(addr(purse+18)) != 9000 || o.Word(addr(purse+20)) != 9000
	})
	o.PressScan("Y")
	if err := o.Run(keyGap); err == nil {
		o.PressScan("\r")
	}
	if err := o.RunUntil(changed, oracle.Budget(6*playerSettle)); err != nil {
		dumpScreen(t, o, "sortie-driven-end")
		t.Fatalf("整編走完了但郡庫沒動：%v", err)
	}
	after := board()

	g, err := game.New(sc, me, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	gi := -1
	for i := 0; i < nGen/state.GeneralRecordSize; i++ {
		r := before[nMas+nSta+i*state.GeneralRecordSize:]
		if r[17] <= 3 && int(r[19]) == at {
			gi = i
			break
		}
	}
	if _, err := g.BeginAttack(at, to, []int{gi}, me,
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
