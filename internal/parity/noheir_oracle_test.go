//go:build oracle

package parity

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 「所有玩家皆無繼承人」那個結束條件在原版裡走一次
// （`docs/mechanics/80-victory` §1.1）。
//
// `0x15924` 掃十六個諸侯槽，`es:[bx+0]`（`0x48` ＝ 72 跨距）等於 1
// 表示玩家操縱，再看 `es:[bx+2]`（君主欄）是不是 `0xFFFF`：
//
//	0x1595b  找到一個活著的玩家 → 回 0xFFFF，遊戲繼續
//	0x15962  一個都沒有 → 清畫面、印 DS:0x97d4、結束
//
// 兩個出口都攔，正反兩面各跑一次：**只驗「結束那一支跑到了」不夠**，
// 沒跑到與沒被呼叫長得一樣。正對照是同一個盤面不動君主欄時走 `0x1595b`。
func TestNoHeirEndsTheGame(t *testing.T) {
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

	var entered, cont, ended int
	o.OnCall(addr(0x15924), func(*oracle.Oracle) { entered++ })
	o.OnCall(addr(0x1595b), func(*oracle.Oracle) { cont++ })
	o.OnCall(addr(0x15962), func(*oracle.Oracle) { ended++ })

	// 一個月：內政 → 休息 → 確認（與 `TestZZMonthParity` 同一組）。
	turn := func(tag string) {
		for _, keys := range strings.Split(envOr("SAN1_TURNKEY", "4\r|4\r|Y"), "|") {
			o.Drain()
			o.PressScan(keys)
			if err := o.Run(120_000_000); err != nil {
				t.Fatalf("%s 送 %q 停止：%v", tag, keys, err)
			}
		}
	}

	snap := o.Save()

	// ── 正對照：君主都在 ─────────────────────────────────────
	players := []int{}
	for i := 0; i < 16; i++ {
		if o.Word(addr(base+uint32(i)*72)) == 1 {
			players = append(players, i)
		}
	}
	if len(players) == 0 {
		t.Fatal("盤面上沒有玩家操縱的勢力——這個劇本擺不出這個條件")
	}
	t.Logf("玩家操縱的勢力：%v", players)
	turn("正對照")
	t.Logf("正對照：進去 %d 次、判「還有玩家」%d 次、判「全部絕嗣」%d 次",
		entered, cont, ended)
	if entered == 0 {
		t.Fatal("0x15924 一次都沒跑到——一個月沒有推過去")
	}
	if cont == 0 {
		t.Error("君主都還在，卻沒有走「還有玩家」那個出口")
	}
	if ended != 0 {
		t.Error("君主都還在就判了絕嗣")
	}

	// ── 反面：把玩家的君主欄清成哨兵 ─────────────────────────
	o.Restore(snap)
	entered, cont, ended = 0, 0, 0
	for _, i := range players {
		o.SetWord(addr(base+uint32(i)*72+2), 0xFFFF)
	}
	t.Logf("把 %d 個玩家勢力的君主欄清成 0xFFFF", len(players))
	turn("絕嗣")
	t.Logf("絕嗣：進去 %d 次、判「還有玩家」%d 次、判「全部絕嗣」%d 次",
		entered, cont, ended)
	if entered == 0 {
		t.Fatal("0x15924 一次都沒跑到")
	}
	if ended == 0 {
		t.Error("玩家全部沒有君主了，原版卻沒有走「結束」那個出口")
	}

	// remake 這一邊問的是同一件事：`Session.PlayerAlive` ＝
	// 「這個勢力還有沒有君主」，不是「還有沒有領地」。
	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	raw := o.Bytes(addr(base), nMas+nSta+nGen)
	sc, err := state.DecodeTables(state.Slot("001"),
		raw[:nMas], raw[nMas:nMas+nSta], raw[nMas+nSta:])
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range sc.Players() {
		if _, err := sc.Lord(f); err == nil {
			t.Errorf("清成哨兵之後，remake 這一邊勢力 %d 還讀得到君主", f)
		}
	}
	t.Logf("remake 這一邊：%d 個玩家勢力都讀不到君主了", len(sc.Players()))
}
