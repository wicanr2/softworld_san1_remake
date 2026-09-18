//go:build oracle

package parity

import (
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// 繼承人清單的三個位址（`docs/spec/014` §4.4 末段，Issue #65）。
const (
	heirBranchAt = 0x14c36 // 玩家那一條的 `push cs`：候選已經排好，三張表還沒動
	                       // （不用 `0x14c12`——那一行電腦的繼承也會走到）
	heirAskAt    = 0x150e9 // 清單裡的數字輸入（`0x115e`）
	heirDoneAt   = 0x14d61 // 繼承寫完之後那一行
)

// TestZZHeirMatchesTheOriginal 讓**玩家**的君主老死，對拍原版停下來問的那一份
// 繼承人清單，以及挑了第三位之後的三張表。
//
// 原版只有在操縱方不是電腦時才問（`0x14c24`）；`TestSuccessionMatchesTheOriginal`
// 刻意只擺電腦君主，就是為了避開這一問。這一支反過來，專門把它逼出來。
func TestZZHeirMatchesTheOriginal(t *testing.T) {
	// **盤面要用 `newPickBoard`**：對拍的開局玩家是自創君主，人物表那一筆
	// 是填充筆（身分 12、勢力與所在都是 `0xFF`，`docs/re/08` §6）——
	// 光 `bootToGame` 的話君主根本不在任何郡裡，老死判定不會帶走他
	// （探針：跑六十輪一次都沒死）。`plantLordCommandBoard` 會把他寫成
	// 活著的君主搬進郡裡當主事者。
	b := newPickBoard(t)
	o := b.o
	tr := b.tr
	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	total := nMas + nSta + nGen
	base := b.base
	genBase := base + uint32(nMas+nSta)
	me := int(b.me)
	lord := int(o.Word(addr(base + uint32(me*state.MasterRecordSize+2))))

	asks := 0
	o.OnCall(addr(heirAskAt), func(*oracle.Oracle) { asks++ })
	ds := uint32(o.DSReg()) * 16
	date := func() (int, int) {
		seg := uint32(o.Word(addr(ds + 0xa72e)))
		return int(o.Word(addr(seg*16 + 0x3140))), int(o.Word(addr(seg*16 + 0x3f08)))
	}

	// 壽命 30、年齡 60、體能 1：過壽三十年，元月的老死判定必定帶走他
	// （`0x15d5d`：`RND(3) + 壽命 >= 年齡` 才躲得過，體能扣到 0 就是死）。
	// **壽命也要自己擺**：自創君主的壽命欄是 `0xFF`。
	const life, age = 30, 60
	o.SetByte(addr(genBase+uint32(lord*state.GeneralRecordSize+28)), life)
	o.SetByte(addr(genBase+uint32(lord*state.GeneralRecordSize+7)), age)
	o.SetByte(addr(genBase+uint32(lord*state.GeneralRecordSize+8)), 1)

	var pre, post []byte
	o.OnCall(addr(heirBranchAt), func(o *oracle.Oracle) {
		if pre == nil {
			pre = o.Bytes(addr(base), total)
		}
	})
	o.OnCall(addr(heirDoneAt), func(o *oracle.Oracle) {
		if post == nil && pre != nil {
			post = o.Bytes(addr(base), total)
		}
	})
	o.OnCall(addr(heirAskAt), func(*oracle.Oracle) { asks++ })

	// 一個月是「內政 → 休息 → 確認」一輪；跑到原版停在繼承人清單為止。
	y0, m0 := date()
	months := 0
	for m := 0; m < 30 && asks == 0; m++ {
		for _, k := range []string{"4\r", "4\r", "Y"} {
			if asks > 0 {
				break
			}
			o.Drain()
			o.PressScan(k)
			if err := o.Run(120_000_000); err != nil {
				t.Fatalf("第 %d 輪送 %q 時停止：%v", m+1, k, err)
			}
		}
		months = m + 1
	}
	y1, m1 := date()
	if asks == 0 {
		t.Fatalf("從 %d 年 %d 月跑到 %d 年 %d 月（%d 輪），`0x150e9` 一次都沒攔到——"+
			"君主沒死（身分還是 %d），或者問的不是這一支",
			y0, m0, y1, m1, months,
			o.Byte(addr(genBase+uint32(lord*state.GeneralRecordSize+17))))
	}
	lastAsk := (*b.asks)[len(*b.asks)-1]
	if pre == nil {
		t.Fatal("`0x14c12` 沒攔到：繼承常式沒走到看操縱方那一步")
	}
	waitCursorShown(t, o, tr, "請選擇繼任的將軍")
	dumpScreen(t, o, "heir")
	shot := append([]uint8(nil), o.IndexedEGASize(scrW, scrH)...)

	// remake 這一邊：從原版「剛死、還沒繼承」那一刻的三張表接手。
	sc, err := state.DecodeTables(state.Slot("001"), pre[:nMas], pre[nMas:nMas+nSta], pre[nMas+nSta:])
	if err != nil {
		t.Fatal(err)
	}
	g, err := game.New(sc, state.FactionID(me), 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	if g.SucceedLord(state.FactionID(me)) == nil {
		t.Fatal("remake 這一邊找不到繼承人")
	}
	who, idx := g.NeedsHeir()
	if int(who) != me || len(idx) == 0 {
		t.Fatalf("remake 要問的是勢力 %d（%d 位候選），原版問的是 %d", who, len(idx), me)
	}
	rp := &ui.RosterPick{List: idx, Key: game.PickByCharm, Succession: true}
	if lo, hi := ui.RosterRange(rp); lastAsk != [2]int{lo, hi} {
		t.Errorf("原版問 %v，remake %d-%d", lastAsk, lo, hi)
	}
	i18n.Current = i18n.ZhHant
	art := b.art
	face := loadFace(t)
	v := ui.View{Roster: rp, Prompt: i18n.Sf("ask.heir", len(idx)), Input: tr.input()}
	cv := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(cv, art, g, nil, v)
	v.Input = ui.InputCursor{}
	plain := ui.NewCanvasPx(scrW, scrH, face)
	ui.DrawArtSession(plain, art, g, nil, v)
	comparePanels(t, "請選擇繼任的將軍", shot, cv, plain, *tr, 1)

	// **挑第三位**（不是預設的排頭），才看得出玩家的選擇真的算數。
	if len(idx) < 3 {
		t.Fatalf("候選只有 %d 位，挑不到第三位", len(idx))
	}
	o.Drain()
	o.TypeBoth("3\r")
	for i := 0; i < 40 && post == nil; i++ {
		if err := o.Run(20_000_000); err != nil {
			t.Fatal(err)
		}
	}
	if post == nil {
		t.Fatalf("挑完之後 `0x14d61` 沒攔到；最後一次提問是 %v", lastAsk)
	}
	if err := g.AssignHeir(idx[2]); err != nil {
		t.Fatal(err)
	}
	if _, left := g.NeedsHeir(); len(left) != 0 {
		t.Error("答完之後 remake 還在等挑繼承人")
	}
	rm, rs, rg, err := g.Tables()
	if err != nil {
		t.Fatal(err)
	}
	got := append(append(append([]byte{}, rm...), rs...), rg...)
	t.Logf("勢力 %d 的君主 %d 老死（跑了 %d 輪）；繼承人挑第 3 位（槽號 %d，預設是 %d）；原版動到的記錄：%s",
		me, lord, months, idx[2], idx[0], changedRecords(pre, post, nMas, nSta))
	if n := diffCount(post, got); n != 0 {
		t.Errorf("三張表兩邊差 %d 個位元組：%s", n, changedRecords(post, got, nMas, nSta))
	}

	// **反向對照**：改挑預設的排頭，同一份表就該對不上——不然這支測試
	// 對「玩家挑了誰」根本不敏感，綠燈什麼都證明不了。
	sc2, err := state.DecodeTables(state.Slot("001"), pre[:nMas], pre[nMas:nMas+nSta], pre[nMas+nSta:])
	if err != nil {
		t.Fatal(err)
	}
	g2, err := game.New(sc2, state.FactionID(me), 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	g2.SucceedLord(state.FactionID(me))
	if _, head := g2.NeedsHeir(); len(head) > 0 {
		if err := g2.AssignHeir(head[0]); err != nil {
			t.Fatal(err)
		}
	}
	hm, hs, hg, err := g2.Tables()
	if err != nil {
		t.Fatal(err)
	}
	head := append(append(append([]byte{}, hm...), hs...), hg...)
	if n := diffCount(post, head); n == 0 {
		t.Error("挑排頭與挑第三位得到同一份表——這支測試對玩家的選擇不敏感")
	} else {
		t.Logf("反向對照：改挑排頭（槽 %d）差 %d 個位元組：%s",
			idx[0], n, changedRecords(post, head, nMas, nSta))
	}
}
