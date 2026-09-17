//go:build oracle

package parity

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 人物資料卡 `0xf874` 的五個呼叫端裡兩條玩家走得到的路（Issue #59，
// `docs/spec/005` §9.2）：查看→3.檢視將軍是「檢視那位 → 卡 → 請按任一鍵
// → 再問檢視那位」的迴圈；君主→4.賞賜物品是「賞賜那一位 → 卡 → 請按任一鍵
// 查看物品表 → 物品表 → 那一樣(2-5) → 寶物圖 → 道謝 → 卡 → 再問那一位」。
const (
	pickGeneralFn = 0x18024 // 「%s(1-%d):」挑當地一位將領的共用提示
	itemTableFn   = 0x14de6 // `1479:0656` 君主物品表
	giftPictureAt = 0x1d264 // 選了那一樣之後畫 `SCG24.IMG` 的那一道
	giftThanksAt  = 0x1d4c1 // 受賜者道謝（405）
)

// TestZZCardTimingsMatchTheOriginal 記下原版兩條路上卡片、提示、物品表與
// 道謝的先後，釘住 remake 照抄的順序。
func TestZZCardTimingsMatchTheOriginal(t *testing.T) {
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
	at, roster := plantLordCommandBoard(t, o, base, 15)
	var seq []string
	mark := func(at uint32, name string) {
		o.OnCall(addr(at), func(*oracle.Oracle) { seq = append(seq, name) })
	}
	o.OnCall(cardFn, func(*oracle.Oracle) { seq = append(seq, "卡") })
	mark(pickGeneralFn, "挑人")
	mark(itemTableFn, "物品表")
	mark(giftPictureAt, "寶物圖")
	mark(giftThanksAt, "道謝")
	o.OnCall(addr(bootNumInputFn), func(o *oracle.Oracle) {
		seq = append(seq, fmt.Sprintf("問%d-%d", int16(o.Arg(0)), int16(o.Arg(1))))
	})
	// 每一鍵送出之後等到下一個提示或畫面（seq 多了東西），再等到讀鍵在等
	// （`1058:0e57`）——讀鍵常式進門會清緩衝，太早送的鍵會被丟掉。
	press := func(name, keys string, scan bool, want string) {
		before := len(seq)
		o.Drain()
		if scan {
			o.PressScan(keys)
		} else {
			o.TypeBoth(keys)
		}
		waitBoot(t, o, name, 200_000_000, func() bool { return len(seq) > before })
		waitBootScan(t, o, name, 100_000_000)
		dumpScreen(t, o, "card-"+name)
		if got := strings.Join(seq[before:], " "); got != want {
			t.Fatalf("%s 之後原版走的是「%s」，該是「%s」（全程 %v）", name, got, want, seq)
		}
	}
	one := fmt.Sprintf("挑人 問1-%d", roster)
	// 查看→3：檢視那位 → 卡 → 請按任一鍵 → 再問檢視那位；空 Enter 收兩層。
	press("查看", "1\r", true, "問1-6")
	press("檢視將軍", "3\r", true, one)
	press("檢視那位 1", "1\r", false, "卡")
	press("請按任一鍵", " ", true, one)
	press("收掉檢視那位", "\r", false, "問1-6")
	press("收掉查看", "\r", false, "問0-9")
	// 君主→4.賞賜物品：那一郡 → 那一位 → 卡 → 任意鍵 → 物品表、那一樣(2-5)
	// → 寶物圖 → 道謝 → 卡 → 再問那一位。寶物圖、道謝與卡之間是
	// `0x1058:0xe80`（延遲或按鍵，dosgolem 裡延遲自己走完），不是等鍵。
	press("君主", "7\r", true, "問1-5")
	press("賞賜物品", "4\r", true, "問1-42")
	press("賞賜那一郡", fmt.Sprintf("%d\r", at), false, one)
	press("賞賜那一位", "1\r", false, "卡")
	press("請按任一鍵 查看物品表", " ", true, "物品表 問2-5")
	press("那一樣 兵書", "2\r", false, "寶物圖 道謝 卡 "+one)
	t.Logf("兩條路的順序：%v", seq)
}

// plantLordCommandBoard 把 `bootToGame` 的盤面擺成君主那一類下得了令：
// 玩家的第一個郡、君主本人當主事者，`treasures` 列的諸侯記錄位移
// （15 兵書、16 寶刀、17 美女、18 駿馬）各寫一件。回郡與駐紮人數。
//
// 對拍的開局是新君主在南海（一位將領），寶庫是空的——賞賜物品那一條
// 要有東西可送。
func plantLordCommandBoard(t *testing.T, o *oracle.Oracle, base uint32,
	treasures ...int) (at, roster int) {
	t.Helper()
	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	live := o.Bytes(addr(base), nMas+nSta+nGen)
	sc, err := state.DecodeTables(state.Slot("001"), live[:nMas], live[nMas:nMas+nSta], live[nMas+nSta:])
	if err != nil {
		t.Fatal(err)
	}
	players := sc.Players()
	if len(players) == 0 {
		t.Fatal("這個盤面沒有玩家")
	}
	player := players[0]
	at = 0
	for _, p := range sc.Prefectures() {
		if p.ID > 0 && p.Owned() && int(p.Owner) == player {
			at = p.ID
			break
		}
	}
	if at == 0 {
		t.Fatal("玩家沒有郡")
	}
	for _, off := range treasures {
		o.SetByte(addr(base+uint32(player*state.MasterRecordSize+off)), 1)
	}
	// 君主那一類要**君主本人是這一郡的主事者**（`0x1c7d2`：主事者身分 ≠ 0
	// 就印「…才能用此功能」）；對拍的開局玩家是新君主槽，君主欄指著人物表
	// 的填充筆（身分 12、勢力與所在都是 0xFF，`docs/re/08` §6），把那一筆
	// 寫成活著的君主搬進來當主事者（人物記錄 offset 17 身分、18 勢力、
	// 19 所在郡；州郡記錄 offset 0x20 主事者）。
	roster = 0
	for _, x := range sc.Generals() {
		if int(x.Location) == at && int(x.Faction) == player && x.Status <= 3 {
			roster++
		}
	}
	// 君主是諸侯記錄 offset 2 指的那一位。
	lord := int(binary.LittleEndian.Uint16(live[player*state.MasterRecordSize+2:]))
	if lord < 0 || lord >= len(sc.Generals()) {
		t.Fatalf("玩家的君主欄是 %d", lord)
	}
	x := sc.Generals()[lord]
	if int(x.Location) != at || x.Status != state.StatusLord || int(x.Faction) != player {
		rec := base + uint32(nMas+nSta+lord*state.GeneralRecordSize)
		o.SetByte(addr(rec+17), 0)
		o.SetByte(addr(rec+18), uint8(player))
		o.SetByte(addr(rec+19), uint8(at))
		roster++
	}
	o.SetWord(addr(base+uint32(nMas+at*state.PrefectureRecordSize+0x20)), uint16(lord))
	return at, roster
}
