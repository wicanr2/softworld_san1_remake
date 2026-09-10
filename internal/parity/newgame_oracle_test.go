//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 開一局新的，選指定的君主。
//
// `bootToGame` 走的是「載入舊進度」，落在建安二年、南海、一個只有一位
// 將領的郡——那個郡出兵會被原版擋掉（移出之後沒有人治理），編隊那一段
// 因此驗不到。開新局才選得到有兵有將、旁邊還有敵人的局面。
//
// **開新遊戲是一等驗收路徑**（`CLAUDE.md` §7 第 12 條）：從存檔載入
// 看不到的缺口，開局才會露出來。
//
// 劇本一的諸侯依槽號排，選君主那一格是 1 起算：
//
//	1 劉備（郡 8、3 人）    2 曹操（郡 11、7 人）   3 孫堅（郡 31、6 人）
//	4 袁紹（郡 3,4、19 人） 5 袁術（郡 13,27）      6 董卓（郡 6,14,15,16）
//	…
func bootToNewGame(t *testing.T, o *oracle.Oracle, lord int, mas []byte) uint32 {
	t.Helper()
	o.TypeBoth("122")
	// 4 跳標題、17 開始新遊戲、23 中平六年——與 `loadScenarioInOriginal`
	// 同一組數字，差別只在主選單送 1 不是 2。
	send := map[int]string{4: "\r", 17: "1", 23: "1"}
	var base uint32
	for i := 0; i < bootSteps/chunk; i++ {
		if err := o.Run(chunk); err != nil {
			t.Fatalf("開機停止：%v", err)
		}
		if k, ok := send[i]; ok {
			o.TypeBoth(k)
		}
		if base == 0 {
			if h := o.Search(mas[:48]); len(h) == 1 {
				base = h[0]
			}
		}
	}
	if base == 0 {
		t.Fatal("三張表還沒進記憶體——開機序列沒走到載盤面")
	}

	diff := "5"
	// 之後的提示讀掃描碼；防拷密碼那一關不吃掃描碼（`docs/re/02` §3）。
	for i, s := range []struct {
		keys  string
		chars bool
		what  string
	}{
		{"1\r", false, "有幾人玩"},
		{fmt.Sprintf("%d\r", lord), false, "第 1 位"},
		{diff + "\r", false, "難度"},
		{passwordAnswer + "\r", true, "防拷密碼"},
		{"Y", false, "請您一定要確定"},
	} {
		before := screenOf(o)
		o.Drain()
		if s.chars {
			o.TypeBoth(s.keys)
		} else {
			o.PressScan(s.keys)
		}
		moved := false
		for k := 0; k < 12; k++ {
			if err := o.Run(50_000_000); err != nil {
				t.Fatalf("送 %s 時停止：%v", s.what, err)
			}
			if pixelDiff(before, screenOf(o), nil) > 200 {
				moved = true
				break
			}
		}
		if !moved {
			dumpScreen(t, o, fmt.Sprintf("newgame-%d", i+1))
			t.Fatalf("開局第 %d 步（%s，送 %q）畫面沒動", i+1, s.what, s.keys)
		}
	}

	// 就任對白：每個現役將一句。**一句一句送，別在中間 Drain**——
	// 對白有顯示時間，送早的鍵還在佇列裡等它來取。
	for i := 0; i < 12; i++ {
		o.PressScan("\r")
		if err := o.Run(30_000_000); err != nil {
			t.Fatalf("就任對白第 %d 句停止：%v", i+1, err)
		}
	}
	return base
}

// TestPlayerCommandsAsCaoCao 拿劇本一的曹操再走一次逐道對拍。
//
// 曹操在郡 11、在職七人，鄰郡有別的勢力——**出兵那一道真的打得起來**，
// 而 `bootToGame` 那個局面（南海、一位將領）只驗得到「兩邊都拒絕」。
func TestPlayerCommandsAsCaoCao(t *testing.T) {
	runPlayerCommands(t, func(t *testing.T, o *oracle.Oracle, mas []byte) uint32 {
		return bootToNewGame(t, o, caoCaoPick, mas)
	})
}

// caoCaoPick 是選君主那一格要送的號碼（劇本一，1 起算）。
const caoCaoPick = 2

// TestZZNewGameBoardIsCaoCao 正對照：開局之後玩家真的是曹操。
//
// 選錯號碼不會報錯，只會讓後面每一道命令都對著別人的郡下——
// 而那看起來與「命令算錯了」一模一樣。
func TestZZNewGameBoardIsCaoCao(t *testing.T) {
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
	raw := o.Bytes(addr(base), nMas+nSta+nGen)
	sc, err := state.DecodeTables(state.Slot("001"),
		raw[:nMas], raw[nMas:nMas+nSta], raw[nMas+nSta:])
	if err != nil {
		t.Fatal(err)
	}
	players := sc.Players()
	if len(players) != 1 {
		t.Fatalf("玩家有 %v 個，開局選的是一個人", players)
	}
	lord, err := sc.Lord(players[0])
	if err != nil {
		t.Fatal(err)
	}
	work := o.ES()
	at := int(o.Word(oracle.Addr{Seg: work, Off: curPrefOff}))
	t.Logf("玩家勢力 %d，君主 %s，主畫面停在郡 %d", players[0], lord.Name, at)
	if lord.Name != "曹操" {
		t.Errorf("選君主送 %d 拿到的是 %s，不是曹操", caoCaoPick, lord.Name)
	}
}
