//go:build oracle

package parity

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 君主→4.賞賜物品的兩層迴圈（`0x1cfd6`，Issue #79，`docs/spec/005` §9.2）。
const (
	giftCommandFn = 0x1cfd6 // 賞賜物品整道命令
	giftOneFn     = 0x1d118 // 送一件（人, 勢力）
	// 主命令迴圈看君主那一類的回傳值：0xFFFF 回主選單再問（`0x17797`），
	// 否則這個郡的回合結束（`0x1779a`）。
	mainAskAgainAt = 0x17797
	mainTurnDoneAt = 0x1779a
)

// TestZZGiftLoopMatchesTheOriginal 在原版上走一道命令送兩件、中間重送同一位，
// 比每一問停下來時的提示，再比三張表：remake 從同一個盤面、同一個亂數
// 狀態送同樣兩件，整份盤面逐位元組相同。
func TestZZGiftLoopMatchesTheOriginal(t *testing.T) {
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
	// 兵書與寶刀各一件。
	at, roster := plantLordCommandBoard(t, o, base, 15, 16)

	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	total := nMas + nSta + nGen
	board := func() []byte { return o.Bytes(addr(base), total) }
	genRec := func(i int) uint32 { return base + uint32(nMas+nSta+i*state.GeneralRecordSize) }
	raw := board()
	me := state.FactionID(raw[nMas+at*state.PrefectureRecordSize+30])
	var people []int
	for i := 0; i < nGen/state.GeneralRecordSize; i++ {
		r := raw[nMas+nSta+i*state.GeneralRecordSize:]
		if r[17] <= 3 && int(r[19]) == at {
			people = append(people, i)
		}
	}
	if len(people) != 2 || roster != 2 {
		t.Fatalf("郡 %d 的名單是 %v（擺盤算的是 %d 位），這支測試要剛好兩位", at, people, roster)
	}
	// 忠誠壓低，才看得到加了多少（100 會被截住，錯的算法也對得上）；
	// 謀略 91 收兵書先變 93、擲忠誠看 93÷2＝46，之後截回 91——看截斷後
	// 的算法會是 45，差一點就分得出來（89 分不出：91÷2 與 90÷2 都是 45）。
	book, blade := people[0], people[1]
	o.SetByte(addr(genRec(book)+9), 91)
	o.SetByte(addr(genRec(book)+16), 20)
	o.SetByte(addr(genRec(blade)+10), 60)
	o.SetByte(addr(genRec(blade)+16), 20)
	nameOf := func(i int) string {
		b := o.Bytes(addr(genRec(i)), 6)
		if n := bytes.IndexByte(b, 0); n >= 0 {
			b = b[:n]
		}
		return decodeBig5Loose(b)
	}

	ds := uint32(o.DSReg()) * 16
	seedOf := func() uint32 {
		return uint32(o.Word(addr(ds+0xa3ae))) | uint32(o.Word(addr(ds+0xa3b0)))<<16
	}
	var seq []string
	inGift := false
	mark := func(at uint32, name string) {
		o.OnCall(addr(at), func(*oracle.Oracle) { seq = append(seq, name) })
	}
	o.OnCall(addr(giftCommandFn), func(*oracle.Oracle) { inGift = true })
	o.OnCall(cardFn, func(*oracle.Oracle) { seq = append(seq, "卡") })
	mark(pickGeneralFn, "挑人")
	mark(itemTableFn, "物品表")
	mark(giftPictureAt, "寶物圖")
	mark(giftThanksAt, "道謝")
	mark(mainAskAgainAt, "回主選單")
	mark(mainTurnDoneAt, "下完令")
	o.OnCall(addr(bootNumInputFn), func(o *oracle.Oracle) {
		seq = append(seq, fmt.Sprintf("問%d-%d", int16(o.Arg(0)), int16(o.Arg(1))))
	})
	o.OnCall(addr(0x33d8*16+0x0cc0), func(o *oracle.Oracle) {
		if !inGift {
			return
		}
		off, seg := uint32(o.Arg(0)), uint32(o.Arg(1))
		b := o.Bytes(addr(seg*16+off), 48)
		if n := bytes.IndexByte(b, 0); n >= 0 {
			b = b[:n]
		}
		seq = append(seq, "「"+strings.ReplaceAll(decodeBig5Loose(b), "\n", "")+"」")
	})
	// 送一件：誰（`0x1d118` 的第一個參數），以及效果那一擲之前的亂數狀態
	// （原版在等鍵與延遲時會重設種子，所以每一件各取一次）。
	type shot struct {
		person int
		seed   uint32
	}
	var shots []shot
	o.OnCall(addr(giftOneFn), func(o *oracle.Oracle) {
		seq = append(seq, "送")
		shots = append(shots, shot{person: int(o.Arg(0))})
	})
	for _, at := range []uint32{0x1d2eb, 0x1d333, 0x1d36d} {
		o.OnCall(addr(at), func(*oracle.Oracle) {
			if n := len(shots); n > 0 {
				shots[n-1].seed = seedOf()
			}
		})
	}
	// 名單是原版自己排的（賞賜那一類的排序鍵，`0x18024` 第三個參數 4），
	// 賞過一件忠誠就變了，順序可能跟著換——送鍵前先問原版誰在第幾位。
	// `0x18024` 讀的是段 `DS:[0xa79c]` 的 `0x58c` 起、筆數在段
	// `DS:[0xa7a0]` 的 `0x0c`（`0x18272`／`0x18153`）。
	keyFor := func(person int) string {
		lst := o.Word(addr(ds + 0xa79c))
		cnt := o.Word(addr(ds + 0xa7a0))
		n := int(o.Word(oracle.Addr{Seg: cnt, Off: 0x0c}))
		for i := 0; i < n; i++ {
			if int(o.Word(oracle.Addr{Seg: lst, Off: uint16(0x58c + 2*i)})) == person {
				return fmt.Sprintf("%d\r", i+1)
			}
		}
		t.Fatalf("原版的名單裡沒有槽號 %d", person)
		return ""
	}

	before := board()
	press := func(name, keys string, scan bool, want string) {
		t.Helper()
		from := len(seq)
		last := want[strings.LastIndex(want, " ")+1:]
		o.Drain()
		if scan {
			o.PressScan(keys)
		} else {
			o.TypeBoth(keys)
		}
		waitBoot(t, o, name, 400_000_000, func() bool {
			return len(seq) > from && seq[len(seq)-1] == last
		})
		if last != "下完令" && last != "回主選單" {
			waitBootScan(t, o, name, 100_000_000)
		}
		if got := strings.Join(seq[from:], " "); got != want {
			dumpScreen(t, o, "gift-"+name)
			t.Fatalf("%s 之後原版走的是「%s」，該是「%s」", name, got, want)
		}
	}
	who := fmt.Sprintf("挑人 「賞賜那一位」 「(1-%d):」 問1-%d", roster, roster)
	press("君主", "7\r", true, "問1-5")
	press("賞賜物品", "4\r", true, "「賞賜那一郡的將軍」 「(1-42):」 問1-42")
	press("賞賜那一郡", fmt.Sprintf("%d\r", at), false, who)
	press("賞賜那一位 兵書", keyFor(book), false, "送 卡 「請按任一鍵查看物品表」")
	press("看物品表 兵書", " ", true, fmt.Sprintf("物品表 「賞賜%s那一樣(2-5):」 問2-5", nameOf(book)))
	press("那一樣 兵書", "2\r", false, "寶物圖 「謀略加2點」 道謝 卡 "+who)
	press("同一位再賞", keyFor(book), false,
		fmt.Sprintf("送 「%s已賞賜過了」 「取消」 %s", nameOf(book), who))
	press("賞賜那一位 寶刀", keyFor(blade), false, "送 卡 「請按任一鍵查看物品表」")
	press("看物品表 寶刀", " ", true, fmt.Sprintf("物品表 「賞賜%s那一樣(2-5):」 問2-5", nameOf(blade)))
	press("那一樣 寶刀", "3\r", false, "寶物圖 「戰力加3點」 道謝 卡 "+who)
	press("空 Enter 回那一郡", "\r", false, "「取消」 「賞賜那一郡的將軍」 「(1-42):」 問1-42")
	press("空 Enter 收掉", "\r", false, "下完令")
	after := board()

	if len(shots) != 3 || shots[0].person != book || shots[1].person != book || shots[2].person != blade {
		t.Fatalf("原版送的是 %+v，該是槽號 %d、%d（已賞賜過）、%d", shots, book, book, blade)
	}
	if shots[0].seed == 0 || shots[2].seed == 0 || shots[1].seed != 0 {
		t.Fatalf("效果那一擲的路標沒對上：%+v", shots)
	}

	sc, err := state.DecodeTables(state.Slot("001"), before[:nMas], before[nMas:nMas+nSta], before[nMas+nSta:])
	if err != nil {
		t.Fatal(err)
	}
	g, err := game.New(sc, me, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	r, err := g.OpenGift(at, me)
	if err != nil {
		t.Fatalf("remake 開不了賞賜物品：%v", err)
	}
	g.SeedRand(shots[0].seed)
	if err := g.Gift(r, book, game.TreasureBook); err != nil {
		t.Fatal(err)
	}
	if err := g.Gift(r, book, game.TreasureBook); err != game.ErrAlreadyGifted {
		t.Fatalf("remake 同一道命令再賞同一位回 %v", err)
	}
	g.SeedRand(shots[2].seed)
	if err := g.Gift(r, blade, game.TreasureBlade); err != nil {
		t.Fatal(err)
	}
	if !g.CloseGift(r) {
		t.Fatal("remake 送了兩件，收掉時不算下過令")
	}
	rm, rs, rg, err := g.Tables()
	if err != nil {
		t.Fatal(err)
	}
	got := append(append(append([]byte{}, rm...), rs...), rg...)
	t.Logf("原版動到的記錄：%s", changedRecords(before, after, nMas, nSta))
	for _, p := range []int{book, blade} {
		off := nMas + nSta + p*state.GeneralRecordSize
		t.Logf("槽號 %d：謀略 %d→%d、戰力 %d→%d、忠誠 %d→%d（remake 忠誠 %d）", p,
			before[off+9], after[off+9], before[off+10], after[off+10],
			before[off+16], after[off+16], got[off+16])
	}
	if bytes.Equal(before, after) {
		t.Fatal("原版的盤面一個位元組都沒動——送鍵沒走到底")
	}
	// 反對照：謀略 91 收兵書，截斷之後還是 91。
	if v := after[nMas+nSta+book*state.GeneralRecordSize+9]; v != 91 {
		t.Errorf("謀略 91 收兵書之後原版是 %d，擺盤沒生效", v)
	}
	if n := diffCount(after, got); n != 0 {
		t.Fatalf("三張表兩邊差 %d 個位元組：%s", n, changedRecords(after, got, nMas, nSta))
	}
}
