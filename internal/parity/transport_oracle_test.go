//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 運送錢糧的結算（`0x19289`–`0x1933a`，`docs/re/03` §1.5）：
//
//	19298  來源.金 −= 金；負 → 0        192b1  來源.米 −= 米；負 → 0
//	192c7  c = 來源主事者的魅力 + 50      ; 州郡 +32 → 人物 +11
//	192e1  fild c / fimul 米 / fmul [0xa7aa]=1/150 / ftol   → 到達的米
//	192fa  fld c / fimul 金 / fmul 1/150 / ftol               → 到達的金
//	1931a  目的.金 += 到達；16 位元溢位（jns 不成立）→ 30000
//	19333  目的.米 += 到達；同上
//
// `0xa7aa` 的 double 在 1/150 之上 4.3e-19，整數倍不會被截掉，所以整數的
// `n × (魅力 + 50) ÷ 150` 與它逐格相同。
const transportSettleAt = 0x19341 // 結算完、正要清訊息框的那一道 call

// TestZZTransportLoss 對拍運送錢糧的損耗（Issue #21）。
//
// 從曹操的新局出發（郡 11，君主本人是主事者，所以會先問「從那一郡」）；
// 把一個鄰郡的所屬直接寫成玩家，讓目的清單有得選。每一組（魅力、金、米）
// 從同一個快照重來，鍵送完之後停在 `0x19341`（結算剛做完），讀目的郡的
// 金米——**不等命令結束**：這是帶 ＊ 的命令，結束時控制權已經交出去，
// 目的郡那一格會被別的郡回合重算所屬、被電腦改寫。
func TestZZTransportLoss(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	base := bootToNewGame(t, o, caoCaoPick, mas)
	staBase := base + uint32(state.MasterTableSize)
	genBase := staBase + uint32(state.PrefectureTableSize)

	// **先把防拷密碼問掉。** 新局開頭沒抽中盤問的話，它會延到第一道 ＊
	// 命令才問（`docs/re/02` §3.3），而運送就是 ＊ 命令——問在鍵序列中間
	// 會把後面的鍵吃進密碼欄。先休息一個月，密碼出現就答，停在下一個月
	// 的主命令再拍快照。
	s := observeBoot(o)
	beforeMain := s.mainAsk
	for _, k := range []string{"4\r", "4\r", "Y\r"} {
		o.Drain()
		o.PressScan(k)
		if err := o.Run(40_000_000); err != nil {
			t.Fatalf("休息送 %q 時停止：%v", k, err)
		}
	}
	waitBoot(t, o, "密碼或下一個月的主命令", 800_000_000,
		func() bool { return s.passwordAsk > 0 || s.mainAsk > beforeMain })
	if s.passwordAsk > 0 {
		o.Drain()
		o.PressScan(passwordAnswer + "\r")
		beforeYN := s.passwordYN
		waitBoot(t, o, "密碼確認", 200_000_000, func() bool { return s.passwordYN > beforeYN })
		o.Drain()
		o.PressScan("Y")
		waitBoot(t, o, "下一個月的主命令", 800_000_000, func() bool { return s.mainAsk > beforeMain })
	}
	waitBootScan(t, o, "下一個月的主命令", 5_000_000)

	work := o.ES()
	at := int(o.Word(oracle.Addr{Seg: work, Off: curPrefOff}))
	src := staBase + uint32(at*state.PrefectureRecordSize)
	me := o.Byte(addr(src + 30))
	to := int(o.Byte(addr(src + 45))) // 相鄰表第一格
	if to == 0xFF || to == 0 {
		t.Fatalf("郡 %d 的相鄰表第一格是 %d", at, to)
	}
	dst := staBase + uint32(to*state.PrefectureRecordSize)
	governor := int(o.Word(addr(src + 32)))
	t.Logf("從郡 %d（勢力 %d，主事者槽 %d）運到郡 %d", at, me, governor, to)
	snap := o.Save()

	type tc struct{ charm, gold, rice int }
	cases := []tc{
		{99, 100, 100}, {50, 150, 300}, {10, 999, 1234}, {0, 3000, 30}, {75, 1, 2},
	}
	checked := 0
	for _, x := range cases {
		o.Restore(snap)
		o.SetWord(addr(src+18), 9000)
		o.SetWord(addr(src+20), 9000)
		o.SetByte(addr(dst+30), me) // 目的郡先寫成自己的，清單才有得選
		o.SetWord(addr(dst+18), 1000)
		o.SetWord(addr(dst+20), 2000)
		o.SetByte(addr(genBase+uint32(governor*30+11)), uint8(x.charm))

		keys := []string{"2\r", "3\r", fmt.Sprint(at) + "\r", fmt.Sprint(to) + "\r",
			fmt.Sprint(x.gold) + "\r", fmt.Sprint(x.rice) + "\r"}
		for i, k := range keys {
			o.Drain()
			o.PressScan(k)
			if i == len(keys)-1 {
				break // 最後一個鍵（米的數量）一送，結算就在後面：要用條件停
			}
			if err := o.Run(40_000_000); err != nil {
				t.Fatalf("送 %q 時停止：%v", k, err)
			}
		}
		if err := o.RunUntil(oracle.At(addr(transportSettleAt)), oracle.Budget(200_000_000)); err != nil {
			dumpScreen(t, o, fmt.Sprintf("transport-stuck-%d-%d-%d", x.charm, x.gold, x.rice))
			t.Fatalf("魅力 %d 金 %d 米 %d：沒走到結算點：%v", x.charm, x.gold, x.rice, err)
		}
		gotGold := int(o.Word(addr(dst+18))) - 1000
		gotRice := int(o.Word(addr(dst+20))) - 2000
		leftGold := int(o.Word(addr(src + 18)))
		leftRice := int(o.Word(addr(src + 20)))
		wantGold := game.TransportArrives(x.gold, x.charm)
		wantRice := game.TransportArrives(x.rice, x.charm)
		checked++
		if gotGold != wantGold || gotRice != wantRice || leftGold != 9000-x.gold || leftRice != 9000-x.rice {
			t.Errorf("魅力 %d：送 %d 金 %d 米 → 原版到達 %d／%d（來源剩 %d／%d），remake %d／%d",
				x.charm, x.gold, x.rice, gotGold, gotRice, leftGold, leftRice, wantGold, wantRice)
			continue
		}
		t.Logf("魅力 %2d：送 %4d 金 %4d 米 → 到達 %4d／%4d ✓", x.charm, x.gold, x.rice, gotGold, gotRice)
	}
	if checked < len(cases) {
		t.Errorf("只比了 %d／%d 組", checked, len(cases))
	}
}
