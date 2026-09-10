//go:build oracle

package parity

import (
	"encoding/binary"
	"fmt"
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

// 一個月的狀態轉移對拍。
//
// 原版與 remake 從**同一個局面**出發，各走一個月，比三張表。
// 局面是原版自己記憶體裡的那 19,220 個位元組，所以「同一個」不是假設
// ——remake 這一邊是拿那份位元組解出來的。
//
// 玩家兩邊都休息（不做任何事），所以差異只可能來自：
//
//   - 電腦諸侯的決策（`docs/mechanics/70-ai`，還沒解）
//   - 每月結算（收成、洪水、物價、忠誠……）
//
// 這一條**不是回歸測試**，是觀測工具：`base` AI 還沒解出來之前它必然
// 有差，差在哪裡就是接下來要解的東西。所以它只報告不判定。

// TestZZMonthParity 原版走一個月，remake 走同一個月，逐欄位比。
func TestZZMonthParity(t *testing.T) {
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
	// **從月初出發，不是從 `bootToGame` 停的地方。** 它停在玩家的郡
	//（原版自動跑完電腦那些才停等輸入），從那裡取視窗會漏掉開月那一整段，
	// 而且停在哪由指令預算決定、換執行器就漂（`CONTEXT.md` R49）。
	// 這一步用原版自己的路標把它推到下個月的月初。
	turnSeqKeys := strings.Split(envOr("SAN1_TURNKEY", "4\r|4\r|Y"), "|")
	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	total := nMas + nSta + nGen

	var atSettle []byte
	// 盤面在**月初那一刻**取（hook 裡拍的），不是在 `bootToGame` 停的地方。
	// **亂數固定**，否則玩家排到第幾格是抽出來的，視窗大小每輪都不同
	//（`SAN1_SEED` 可換；0 ＝ 不動，用原版自己抽的）。
	//
	// 這個值是 `TestZZSeedScan` 掃出來的：**判準是視窗大小，不是「哪個
	// 種子讓測試變綠」**。視窗是 `[游標, 玩家的郡)`，玩家排得越後面比得
	// 越多；`0x13579BDF` 讓玩家排第 38 格，是掃過的十個裡最大的
	//（舊 base 那一輪是 37 格，所以涵蓋相當）。
	//
	// ⚠ 掃出來的分布很分散：同一批種子裡有 2 格的、5 格的、7 格的。
	// **拿小視窗換綠是這裡最容易犯的錯**——`0x5A17C0DE`（7 格）一度讓
	// 這支測試 PASS，而 38 格的視窗下差異又回來了。
	seed := uint32(0x13579BDF)
	if v := os.Getenv("SAN1_SEED"); v != "" {
		if n, err := strconv.ParseUint(v, 0, 32); err == nil {
			seed = uint32(n)
		}
	}
	before := driveToMonthStart(t, o, turnSeqKeys, base, total, seed)
	// **出發盤面的幾個郡印出來。** 兩邊都從這一份走，所以它是共同的起點；
	// 郡回合開始時對不上時，要先分得出「起點就不同」與「走的過程不同」。
	for _, id := range []int{6, 10} {
		at := state.MasterTableSize + id*176
		t.Logf("出發盤面：郡 %d 金 %d 米 %d 兵(百) %d 人口(百) %d 主事者 %d",
			id,
			binary.LittleEndian.Uint16(before[at+18:]),
			binary.LittleEndian.Uint16(before[at+20:]),
			binary.LittleEndian.Uint16(before[at+16:]),
			binary.LittleEndian.Uint16(before[at+14:]),
			int16(binary.LittleEndian.Uint16(before[at+32:])))
	}
	dumpTables(t, before, "parity-00-出發")

	// remake 這一邊從同一份位元組建局面。
	sc, err := state.DecodeTables(state.Slot("001"),
		before[:nMas], before[nMas:nMas+nSta], before[nMas+nSta:])
	if err != nil {
		t.Fatalf("原版記憶體裡的三張表解不開：%v", err)
	}

	// 玩家是誰**盤面自己說**：諸侯記錄 offset 0，1 ＝ 玩家、2 ＝ 電腦、
	// 0xFFFF ＝ 沒在用（`docs/formats/03`）。先前是拿主畫面問的那個郡
	// 反推所屬——那只在「主畫面剛好停在玩家的郡」時成立。
	players := sc.Players()
	if len(players) == 0 {
		t.Skip("這個盤面沒有玩家控制的勢力（電腦自動示範模式）")
	}
	player := state.FactionID(players[0])
	t.Logf("玩家勢力槽號 %d（盤面上共 %d 個玩家）", player, len(players))

	g, err := game.New(sc, player, 5, state.EditionBase)
	if err != nil {
		t.Fatalf("remake 這邊開不了局：%v", err)
	}
	// **年月要對齊，否則季節事件會比錯。** `game.New` 用的是劇本的起始
	// 年月（中平六年元月），而這個存檔停在建安二年九月——年齡在元月加、
	// 人口在十月加，差一個月就足以讓整組年度事件對不上
	//（`docs/mechanics/50-events` §1）。
	//
	// 年月不在三張表裡（存檔的 `BASEPRO.SVn` 還沒解），所以先寫死，
	// 出處是原版主畫面左側直排的「建安二年九月秋」。
	g.Date = game.Date{Year: 197, Month: 9}
	t.Logf("兩邊都從 %d 年 %d 月出發", g.Date.Year, g.Date.Month)

	// **兩邊從同一個亂數狀態出發**（`DS:0xa3ae`，`docs/re/03` §1.45）。
	// 原版每呼叫一次 `rand()` 就抽一次；remake 這一邊 `RandDraws()` 數同
	// 一件事。**抽的次數對不上比位元組數好定位**：某一支常式的分支或
	// 迴圈次數與原版不同，次數就會差。
	randCalls := 0
	// 逐郡歸戶：`curDisp` 是「現在跑的是哪個郡的分派器」，−1 表示不在
	// 郡回合裡（開月、每月結算那些）。
	curDisp := -1
	// 第一個對不上的那一格要拆到「哪一張表」。要追哪一個郡用
	// `SAN1_WATCH` 指定（remake 那邊同一個環境變數），預設郡 1。
	firstGapAt := 1
	if v := os.Getenv("SAN1_WATCH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			firstGapAt = n
		}
	}
	firstTbl := map[string]int{}
	randBy := map[int]int{}
	// 再歸一次戶：十八張表各自抽了幾次。分派點是 `0xe926`–`0xec1b`
	// （`docs/re/03` §1.4），攔在 `lcall` 上（呼叫前），所以下一個攔截點
	// 之前的抽樣都算這一張的。
	tables := []struct {
		at   uint32
		name string
	}{
		{0xe926, "行動者"}, {0xe937, "指定軍師"}, {0xe948, "指定太守"},
		{0xe959, "尋訪"}, {0xe96a, "登用"}, {0xe97b, "訓練"},
		{0xe98c, "內政"}, {0xe99d, "賞賜物品"}, {0xe9fc, "武器"},
		{0xea5b, "徵兵"}, {0xeaba, "0x5514"}, {0xeb19, "賑民"},
		{0xeb78, "賞賜金帛"}, {0xebd7, "挖角"}, {0xebe8, "計略"},
		{0xebf9, "調整兵力"}, {0xec0a, "買米"}, {0xec1b, "出兵"},
	}
	// 賞賜物品的 `lcall`（`0xe99d`，4 bytes）回到 `0xe9a1`，之後到
	// `0xe9fc`（武器）之間是「呼叫前先算本回合預算」那一段。分成兩個桶
	// 才知道那 271 次是常式裡的迴圈還是預算計算。
	extra := []struct {
		at   uint32
		name string
	}{
		{0x1581c, "月底結算"}, {0x16e70, "冬季事件（人口成長）"},
		{0x1712a, "進貢"},
		{0x17371, "開月"}, {0x1740a, "開月洗牌"},
		// 郡回合的入口（`0x1746e`）到分派器之間（`0x174f3`）。
		// 這一段先前落在**前一張表**的桶裡：進貢之後緊接著就是第一個郡，
		// 所以進貢那 69 次裡可能有一次其實是這裡的。
		{0x1746e, "郡回合入口"},
		{0x1734f, "進貢之後"},
		// 月迴圈每前進一格的那一次 `RND(12)`（`0x15790`）。它排在
		// `0x1746e` 之前，不分出來的話會記在**前一個郡最後一張表**
		// （出兵）的帳上。
		{0x15790, "月迴圈每格"},
		// `0x1734f` 之後第一件事是**整張地圖重繪**（`0x32fb8`：郡 1..42
		// 逐一呼叫 `0x32fe6`，讀 offset 30 決定顏色，再走間接的
		// `lcall *es:[0x20ea]`）。那一段與月迴圈要分開數。
		{0x32fb8, "地圖重繪"},
		{0x1735c, "重繪之後到第一個郡"},
		{0xe9a1, "賞賜物品之後的預算計算"},
		{0xd962, "賞賜物品：兵書"}, {0xdac0, "賞賜物品：寶刀"},
		{0xdc1e, "賞賜物品：美女"}, {0xdd94, "賞賜物品：駿馬"},
	}
	curTable := "郡回合之外"
	randTbl := map[string]int{}
	// **州郡記錄直接從 `base` 讀**，和倒三張表用的是同一塊記憶體。
	// 自己算 DS 相對的段選擇子會拿到別的東西——同一個位址在不同的
	// 呼叫點對到不同的段。
	// 守軍的兵力／訓練／武裝，逐人列出來。徵兵、訓練、武器那三支動的
	// 就是這三欄，而它們佔了剩下位元組差的八成。
	gar := func(o *oracle.Oracle, p int) string {
		out := ""
		for i := 0; i < 350; i++ {
			at := base + uint32(nMas) + uint32(nSta) + uint32(i)*30
			if int(o.Byte(addr(at+19))) != p {
				continue
			}
			if st := int(o.Byte(addr(at + 17))); st > 3 {
				continue
			}
			out += fmt.Sprintf(" [%d 兵 %d 訓 %d 武裝 %d]", i,
				o.Word(addr(at+22)), o.Byte(addr(at+24)), o.Byte(addr(at+25)))
		}
		return out
	}
	sta := func(o *oracle.Oracle, p int) string {
		at := base + uint32(nMas) + uint32(p)*176
		return fmt.Sprintf("人口(百) %d 兵(百) %d 金 %d 米 %d",
			o.Word(addr(at+14)), o.Word(addr(at+16)),
			o.Word(addr(at+18)), o.Word(addr(at+20)))
	}
	// 逐表記下**進這一張表之前**郡的兵金米。remake 那邊記的是
	// 「跑完這一張表之後」，所以原版的第 n+1 筆對 remake 的第 n 筆。
	watch := -1
	if v := os.Getenv("SAN1_WATCH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			watch = n
		}
	}
	valLog := []string{}
	for _, tb := range tables {
		name := tb.name
		o.OnCall(addr(tb.at), func(o *oracle.Oracle) {
			curTable = name
			if watch >= 0 && curDisp == watch {
				valLog = append(valLog,
					fmt.Sprintf("進 %-10s 之前　%s", name, sta(o, watch)))
				if name == "賑民" || name == "武器" {
					valLog = append(valLog,
						fmt.Sprintf("　  守軍（進 %s 之前）%s", name, gar(o, watch)))
				}
				if name == "內政" {
					// 主事者（州郡 offset 32）與他的智（人物 offset 9）
					// ——開墾的量是 `(智 − 底) ÷ 12`，非正才多擲一次。
					at := base + uint32(nMas) + uint32(watch)*176
					who := int(o.Word(addr(at + 32)))
					intel := -1
					if who >= 0 && who < 350 {
						intel = int(o.Byte(addr(base + uint32(nMas) +
							uint32(nSta) + uint32(who)*30 + 9)))
					}
					owner := int(o.Byte(addr(at + 30)))
					lvl := -1
					if owner >= 0 && owner < 16 {
						lvl = int(o.Byte(addr(base + uint32(owner)*72 + 4)))
					}
					valLog = append(valLog,
						fmt.Sprintf("　  原版的主事者 %d 智 %d；所屬 %d "+
							"AI 等級 %d；自治欄(offset 12) %d",
							who, intel, owner, lvl,
							o.Word(addr(at+12))))
				}
			}
		})
	}
	// 這些攔截點不只要記抽了幾次，還要記**被呼叫幾次**——一個桶是 0
	// 有兩種意思：常式跑了但沒抽亂數，或者它在這個窗口裡根本沒跑。
	hits := map[string]int{}
	for _, tb := range extra {
		name := tb.name
		o.OnCall(addr(tb.at), func(*oracle.Oracle) {
			curTable = name
			hits[name]++
		})
	}
	o.OnCall(addr(0x174f3), func(*oracle.Oracle) { curTable = "郡回合之外" })
	o.OnCall(oracle.Addr{Seg: 0x5c4, Off: 0x2cb0}, func(*oracle.Oracle) {
		randCalls++
		randBy[curDisp]++
		if curDisp == firstGapAt {
			firstTbl[curTable]++
		}
		randTbl[curTable]++
	})
	// **從月底結算那一刻接上**，不是從快照那一刻。快照到月底之間是
	// **玩家自己的回合**（原版按了「內政 → 休息」，會抽亂數），而 remake
	// 這一邊的玩家什麼都不做——量到那一段原版抽了 38 次。
	// 從結算接上，比較的窗口才對齊。
	seedAtSettle := uint32(0)
	haveSettleSeed := false
	// 從**月底結算的入口**接（`0x1581c`）：量到那裡到開月之間原版
	// 一次亂數都沒抽，所以兩邊在這一點的狀態可以直接對齊，
	// 而接下來的開月（物價 ＋ 洗牌）兩邊都會跑。
	o.OnCall(addr(0x1581c), func(o *oracle.Oracle) {
		if haveSettleSeed {
			return
		}
		ds := uint32(o.DSReg()) * 16
		seedAtSettle = uint32(o.Word(addr(ds+0xa3ae))) | uint32(o.Word(addr(ds+0xa3b0)))<<16
		haveSettleSeed = true
		// **窗口的起點盤面也要驗。** remake 是從 `parity-00`（開機那一刻）
		// 建局面的，而原版在那之後還跑過玩家的回合才進結算——那一段
		// 動過的欄位，remake 這邊沒有。
		atSettle = o.Bytes(addr(base), total)
	})

	// 每個郡的回合（`0x1746e`）：走到幾個、其中幾個真的跑了分派器。
	// **`0x17471` 的最後一道閘門讀的是重算過的所屬**（`0x1e394` 在同一支
	// 常式裡先跑），所以「這個郡有沒有輪到」是動態的，靜態盤面看不出來。
	// **旗標陣列是存檔欄位**（`es:[0x24d8]`，43 格；`docs/re/08` §2）：
	// 0xFFFF ＝ 這個月還沒下令，0 ＝ 已經下過。載入的進度是月中存的，
	// 所以一開始就有幾格是 0——那幾個郡這個月**不會**輪到。
	// remake 那邊要照同一份旗標跳過，否則是拿 37 個郡的原版比 42 個郡的
	// remake，兩邊做的事情不一樣多。
	staBase := base + uint32(state.MasterTableSize)
	genBase := staBase + uint32(state.PrefectureTableSize)
	turnFlags := make([]bool, 43) // true ＝ 這個月還沒下令
	var turnSeq []int             // 原版這個月的郡順序
	winFrom, winTo := 0, 0        // 對拍視窗在順序表上的範圍
	flagsRead := false
	visited := map[int]int{}
	dispatched := map[int]bool{}
	gate := map[int]string{}
	curTurn := -1
	o.OnCall(addr(0x1746e), func(o *oracle.Oracle) {
		if !flagsRead {
			ds := uint32(o.DSReg()) * 16
			flags := uint32(o.Word(addr(ds+0xa730)))*16 + 0x24d8
			for i := range turnFlags {
				turnFlags[i] = o.Word(addr(flags+uint32(i*2))) == 0xFFFF
			}
			// 游標 `es:[0x20f4]` 與順序表 `es:[0x0e]`（`docs/re/08` §2）。
			cur := int(int16(o.Word(addr(uint32(o.Word(addr(ds+0xa726)))*16 + 0x20f4))))
			ord := uint32(o.Word(addr(ds+0xa72c)))*16 + 0x0e
			seq := make([]int, 43)
			for i := range seq {
				seq[i] = int(int16(o.Word(addr(ord + uint32(i*2)))))
			}
			t.Logf("第一個郡的回合時：游標 %d，順序表 %v", cur, seq)
			// **對拍的視窗到玩家那一格為止。** 月內迴圈走到玩家的郡就停下來
			// 等玩家下令（`0x17471` 的 `諸侯 offset0 == 2` 不成立，接著跳進
			// 主命令提示），游標留在那裡下次接著跑。所以這一段量到的是
			// 「玩家回合 → 排在玩家前面的郡 → 又輪到玩家」，**不是一整個月**
			// ——排在玩家後面的郡要等下一輪才動。remake 那邊要照同一個視窗。
			stop := len(seq)
			for i := cur; i < len(seq); i++ {
				p := seq[i]
				if p <= 0 || p >= 43 {
					continue
				}
				if state.FactionID(o.Byte(addr(staBase+uint32(p*176+30)))) == player {
					stop = i
					break
				}
			}
			for i := 0; i < len(seq); i++ {
				if i >= cur && i < stop {
					continue
				}
				if p := seq[i]; p >= 0 && p < len(turnFlags) {
					turnFlags[p] = false
				}
			}
			turnSeq, winFrom, winTo = seq, cur, stop
			t.Logf("對拍視窗：順序表 [%d, %d)，共 %d 個郡", cur, stop, stop-cur)
			flagsRead = true
		}
		curTurn = int(int16(o.Arg(0)))
		visited[curTurn]++
	})
	o.OnCall(addr(0x174e4), func(o *oracle.Oracle) {
		if curTurn < 0 || curTurn >= 43 {
			return
		}
		own := int(o.Byte(addr(base + uint32(state.MasterTableSize) + uint32(curTurn*176+30))))
		mode := -1
		if own < 16 {
			mode = int(int16(o.Word(addr(base + uint32(own*72)))))
		}
		gate[curTurn] = fmt.Sprintf("重算後所屬 %d、諸侯 offset0 %d", own, mode)
	})
	// 每個郡的回合**開始前**的亂數狀態，照順序表的順序記。
	origSeed := map[int]uint32{}
	o.OnCall(addr(0x174ec), func(o *oracle.Oracle) {
		if curTurn >= 0 && curTurn < 43 {
			ds := uint32(o.DSReg()) * 16
			origSeed[curTurn] = uint32(o.Word(addr(ds+0xa3ae))) |
				uint32(o.Word(addr(ds+0xa3b0)))<<16
		}
	})
	o.OnCall(addr(0x174ec), func(o *oracle.Oracle) {
		if curTurn >= 0 && curTurn < 43 {
			dispatched[curTurn] = true
			curDisp = curTurn
		}
	})
	o.OnCall(addr(0x174f3), func(*oracle.Oracle) { curDisp = -1 })

	// 兵書那一支的漏斗：進入 → 過了 RND(100) → 過了庫存。
	// 靜態讀出來的條件是「庫存 > r」而 r ∈ {2,3}，可是盤面上的庫存多半
	// 是 1–2——照那樣算迴圈跑不到，但抽樣次數說它跑得到。數一次就知道。
	var fIn, fRnd, fStock int
	o.OnCall(addr(0x0d962), func(*oracle.Oracle) { fIn++ })
	o.OnCall(addr(0x0d988), func(*oracle.Oracle) { fRnd++ })
	o.OnCall(addr(0x0d9bc), func(*oracle.Oracle) { fStock++ })

	// 進貢的漏斗：迴圈頭 → 過領地閘門（要 RND(10)）→ 過人才閘門（要抽四種）。
	var tbLoop, tbLand, tbTalent int
	tbDumped := false
	var origTributeSeed uint32
	o.OnCall(addr(0x1712a), func(o *oracle.Oracle) {
		tbLoop++
		if tbDumped {
			return
		}
		tbDumped = true
		ds := uint32(o.DSReg()) * 16
		land := uint32(o.Word(addr(ds+0xa73c)))*16 + 0x24b2
		tal := uint32(o.Word(addr(ds+0xa77e)))*16 + 0x2e36
		a, b2 := make([]int, 16), make([]int, 16)
		for i := 0; i < 16; i++ {
			a[i] = int(int16(o.Word(addr(land + uint32(i*2)))))
			b2[i] = int(int16(o.Word(addr(tal + uint32(i*2)))))
		}
		t.Logf("進貢的兩張表：0x24b2 %v", a)
		t.Logf("進貢的兩張表：0x2e36 %v", b2)
		origTributeSeed = uint32(o.Word(addr(ds+0xa3ae))) |
			uint32(o.Word(addr(ds+0xa3b0)))<<16
	})
	// 寶庫（諸侯 offset 14–18）在**接上的那一刻**與**進貢之後**各拍一份。
	// 賞賜物品那四支拿存量當閘門（`0xd9b4`），存量差一格就整支不跑。
	dumpTreasury := func(o *oracle.Oracle) [16][5]int {
		var out [16][5]int
		ds := uint32(o.DSReg()) * 16
		seg := uint32(o.Word(addr(ds+0xa734))) * 16
		for i := 0; i < 16; i++ {
			for k := 0; k < 5; k++ {
				out[i][k] = int(o.Byte(addr(seg + uint32(i*0x48+14+k))))
			}
		}
		return out
	}
	var tbBefore, tbAfter [16][5]int
	o.OnCall(addr(0x170a2), func(o *oracle.Oracle) { tbBefore = dumpTreasury(o) })
	o.OnCall(addr(0x1734f), func(o *oracle.Oracle) { tbAfter = dumpTreasury(o) })
	// 開墾讀的是**行動者**（`es:[0x4196]`），不是州郡 offset 32 那位。
	// `0xbd0d` 那一刻 AX ＝ 30 × 行動者，直接從暫存器拿，免得自己算
	// DS 相對的段選擇子。等級 5 那一支（`0xbcea`）。
	o.OnCall(addr(0xbd0d), func(o *oracle.Oracle) {
		if watch >= 0 && curDisp == watch {
			who := int(o.AX()) / 30
			intel := -1
			if who >= 0 && who < 350 {
				intel = int(o.Byte(addr(base + uint32(nMas) +
					uint32(nSta) + uint32(who)*30 + 9)))
			}
			valLog = append(valLog,
				fmt.Sprintf("　  原版開墾的行動者 %d 智 %d", who, intel))
		}
	})

	// 徵兵逐人配額（`0xc025` 那一刻 AX ＝ 這一位徵到的人數、
	// BX ＝ 30 × 人物槽號）。原版是走名單逐一補，remake 先前讓第一位
	// 吃光預算——量的差八成在這裡。
	csLog := []string{}
	// `0xbf63` 那一刻 ES 已經指向含 `es:[0x3d16]`（本回合預算）的段，
	// AX ＝ 這一位的空額。郡的金直接從 `base` 讀。
	o.OnCall(addr(0xbf63), func(o *oracle.Oracle) {
		if watch >= 0 && curDisp == watch {
			at := base + uint32(nMas) + uint32(watch)*176
			csLog = append(csLog, fmt.Sprintf("（空額 %d 預算 %d 金 %d）",
				int(o.AX()),
				int(o.Word(addr(uint32(o.ES())*16+0x3d16))),
				int(o.Word(addr(at+18)))))
		}
	})
	o.OnCall(addr(0xc025), func(o *oracle.Oracle) {
		if watch >= 0 && curDisp == watch {
			csLog = append(csLog,
				fmt.Sprintf("%d:+%d", int(o.BX())/30, int(o.AX())))
		}
	})

	// 出兵那三道閘門（`0xb47a`）：兵士(百) > 金、兵士(百)×15 > 米、
	// 清單長度 < 1。進到 `0xb47a` 表示編隊已經洗過了，所以這裡讀到的
	// 是**擋下來之前**的盤面。
	soLog := []string{}
	// 郡回合**開始時**的金與米（`0x174f3`，分派器之前）。出兵那一道
	// 比的是米，所以要知道差是回合開始就有、還是回合中某張表造成的。
	startLog := map[int]string{}
	var atFirstTurn []byte
	o.OnCall(addr(0x174f3), func(o *oracle.Oracle) {
		if curTurn >= 0 && curTurn <= 42 {
			startLog[curTurn] = sta(o, curTurn)
		}
		if atFirstTurn == nil {
			atFirstTurn = o.Bytes(addr(base), total)
		}
	})
	o.OnCall(addr(0xb47a), func(o *oracle.Oracle) {
		if curDisp >= 0 && curDisp <= 42 {
			soLog = append(soLog, fmt.Sprintf("郡 %d %s", curDisp, sta(o, curDisp)))
		}
	})
	o.OnCall(addr(0x1713e), func(*oracle.Oracle) { tbLand++ })
	o.OnCall(addr(0x17167), func(*oracle.Oracle) { tbTalent++ })
	// 四種寶物的「重抽」各一段（`c < v → v = RND(5)+8`）。
	// 進貢那 69 次拆成 13 次閘門 ＋ 6 勢力 × 9 ＋ 重抽，
	// 而 13＋54＋2 與 12＋54＋3 都湊得出來——要數才知道是哪一種。
	var tbAgain int
	for _, at := range []uint32{0x17188, 0x171be, 0x171f4, 0x1722a} {
		o.OnCall(addr(at), func(*oracle.Oracle) { tbAgain++ })
	}

	// 指定軍師（`0xd7ae`）逐次記下來：郡、所屬、舊軍師、門檻，以及
	// 寫完之後諸侯 offset 6 的值。remake 在郡 13 把勢力 4 的軍師換掉，
	// 而原版這個月一個身分都沒動——要知道原版在那個郡到底做了什麼。
	var sg sortieGlobals
	sgOK := false
	chiefLog := []string{}
	curOwner, curOld := 0, -1
	o.OnCall(addr(0x0d7ae), func(o *oracle.Oracle) {
		if !sgOK {
			sg, sgOK = resolveSortieGlobals(o), true
		}
		pref := int(int16(o.Word(addr(sg.pref))))
		curOwner = int(o.Byte(addr(staBase + uint32(pref*176+30))))
		curOld = int(int16(o.Word(addr(base + uint32(curOwner*72+6)))))
		floor := 79
		if curOld >= 0 && curOld < state.GeneralTableSize/state.GeneralRecordSize {
			floor = int(o.Byte(addr(genBase + uint32(curOld*state.GeneralRecordSize+9))))
		}
		chiefLog = append(chiefLog,
			fmt.Sprintf("郡 %d 所屬 %d 舊 %d 門檻 %d", pref, curOwner, curOld, floor))
	})
	// 挖角真的掃名單的次數（`0xe0bc`）——入口擲骰不算。
	hhAt := []int{}
	o.OnCall(addr(0x0e0bc), func(o *oracle.Oracle) {
		if sgOK {
			hhAt = append(hhAt, int(int16(o.Word(addr(sg.pref)))))
		}
	})
	o.OnCall(addr(0x0d8b0), func(o *oracle.Oracle) {
		if !sgOK || len(chiefLog) == 0 {
			return
		}
		now := int(int16(o.Word(addr(base + uint32(curOwner*72+6)))))
		if now != curOld {
			chiefLog[len(chiefLog)-1] += fmt.Sprintf(" → 換成 %d", now)
		}
	})

	// 原版：走一個月。玩家只有一個郡，所以一次「內政 → 休息 → Y」
	// 就把玩家的回合用掉，接著是電腦諸侯與每月結算。
	seq := turnSeqKeys
	turns := 1
	if v, err := strconv.Atoi(os.Getenv("SAN1_TURNS")); err == nil && v > 0 {
		turns = v
	}
	const settle = 40_000_000
	for i := 0; i < turns; i++ {
		for _, keys := range seq {
			o.Drain()
			o.PressScan(keys)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("原版停止：%v", err)
			}
		}
	}
	// **月內迴圈會中途停下來等按鍵。** `0x15758` 的迴圈本體看 `es:0x80`，
	// 被清掉就退出；游標 `es:0x20f4` 是存檔欄位，所以下一次進來會接著跑
	//（`docs/re/08` §2）。空轉沒有用——它在等輸入。不補按的話快照是在
	// **月中**拍的，拿沒跑完的原版比跑完的 remake。
	// `0x15772`：迴圈每一輪都輪詢鍵盤，**有鍵就退出**（游標留著，下次接著
	// 跑）。按鍵送完之後緩衝區還有殘留的話，電腦的回合會在半路被打斷——
	// 而半路停下來看起來只是「原版這個月做得少」。清空再空轉，讓它跑完。
	for i := 0; i < 20 && len(visited) < 42; i++ {
		n := len(visited)
		o.Drain()
		if err := o.Run(settle * 3); err != nil {
			t.Fatalf("續跑第 %d 輪時停止：%v", i+1, err)
		}
		if len(visited) == n {
			break
		}
	}
	t.Logf("清鍵盤續跑之後走到 %d 個郡", len(visited))

	done := []int{}
	for i, ok := range turnFlags {
		if !ok {
			done = append(done, i)
		}
	}
	t.Logf("出發時旗標已清（這個月不會輪到）的郡：%v（讀到旗標：%v）", done, flagsRead)
	t.Logf("每個郡的回合：走到 %d 個，其中 %d 個跑了分派器", len(visited), len(dispatched))
	for p := 0; p < 43; p++ {
		if visited[p] > 0 && !dispatched[p] {
			t.Logf("    郡 %d 走到了但沒跑分派器：%s", p, gate[p])
		}
	}

	t.Logf("原版的指定軍師走了 %d 次：", len(chiefLog))
	for _, l := range chiefLog {
		t.Log("    " + l)
	}

	after := o.Bytes(addr(base), total)
	dumpTables(t, after, "parity-01-原版走完")
	t.Logf("原版抽了 %d 次亂數", randCalls)
	t.Logf("原版：三張表動了 %d 個位元組%s",
		differs8(before, after), where(before, after, nMas, nSta))

	// remake：玩家休息，電腦諸侯各自出手，然後結算。
	mineBy := map[int]int{}
	mineTbl := map[string]int{}
	brain, err := ai.New(ai.ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	if done, total := brain.Coverage(); done < total {
		t.Logf("⚠ %s 解出 %d/%d 種行為——下面的差異包含「沒解出來的那幾種完全沒動」",
			brain.Name(), done, total)
	}
	// **窗口跨過月底。** 原版走的是：月 9 的尾巴（那 5 個郡的旗標早就
	// 清掉了）→ 月底結算 ＋ 冬季事件（**含進貢**）→ 開月洗牌 →
	// 月 10 的前 37 個郡 → 又輪到玩家。remake 這一邊要照同一個相位：
	// **先結算再跑郡回合**，否則郡回合看到的是進貢之前的庫存
	//（賞賜物品那一支因此少抽 271 次），而人口成長也會落在錯的一邊。
	if !haveSettleSeed {
		t.Fatal("沒攔到月底結算——亂數對不起來")
	}
	// 進貢的兩張表 remake 這一邊也印一份。進貢的抽樣次數只差「重抽」
	// 那一次，而重抽要 `c < v`、`v = RND(基數+1)`——次數相同表示迴圈
	// 結構一致，所以差別只能在**基數的分子**，也就是這兩張表。
	rl, rt := make([]int, 16), make([]int, 16)
	for i := 0; i <= 42; i++ {
		q := g.Prefecture(i)
		if q == nil || !q.Owned() || int(q.Owner) >= 16 {
			continue
		}
		rl[q.Owner]++
		rt[q.Owner] += int(q.PublicLoyalty)/4 + int(q.LandValue)/2
	}
	ids := []int{}
	for _, f := range g.Factions() {
		ids = append(ids, int(f.ID))
	}
	t.Logf("remake 的勢力槽號：%v（原版走的是 0..15 全部）", ids)
	ra := [16][5]int{}
	rb := [16][5]int{}
	for _, f := range g.Factions() {
		if int(f.ID) < 16 {
			rb[f.ID] = f.Treasury
		}
	}
	t.Logf("寶庫（進貢前）原版 %v", tbBefore)
	t.Logf("寶庫（進貢前）remake %v", rb)
	t.Logf("進貢的兩張表（remake）：領地 %v", rl)
	t.Logf("進貢的兩張表（remake）：人才 %v", rt)

	g.SeedRand(seedAtSettle)
	phase := map[string]int{}
	phaseSeed := map[string]uint32{}
	g.TracePhases(phase, phaseSeed)
	t.Logf("兩邊都從月底結算那一刻的亂數狀態 0x%08x 接上", seedAtSettle)
	g.EndMonth()
	t.Logf("起點：接上的是 0x%08x，remake 走完 0 步是 0x%08x（該相同）",
		seedAtSettle, phaseSeed["換月"])
	for _, f := range g.Factions() {
		if int(f.ID) < 16 {
			ra[f.ID] = f.Treasury
		}
	}
	t.Logf("進貢前的狀態：原版 0x%08x，remake 0x%08x（民亂判定之後）",
		origTributeSeed, phaseSeed["民亂判定"])
	for _, k := range []string{"換月", "物價", "洗牌", "人口成長", "民亂判定", "四季"} {
		t.Logf("remake 換月各段：%s %d 次", k, phase[k])
	}
	// **照原版的順序跑。** 郡的回合是一條全域的洗牌迴圈，不是「一個勢力
	// 跑完換下一個」——即使每一張表抽的次數都對，消耗的**順序**不同，
	// 亂數序列一開始就岔開。順序表直接用原版那一份（`turnSeq`）。
	pp, ok := brain.(ai.PrefecturePlanner)
	if !ok {
		t.Fatalf("%s 不支援逐郡執行", brain.Name())
	}
	pp.TraceDraws(mineTbl)
	firstMine := map[string]int{}
	firstGap := -1
	for i := winFrom; i < winTo && i < len(turnSeq); i++ {
		// 月迴圈每一格都先抽一次（`0x15790`），跳過的格子也算。
		g.TurnTick()
		at := turnSeq[i]
		q := g.Prefecture(at)
		if q == nil || !q.Owned() || q.Owner == player {
			continue
		}
		if s0, ok := origSeed[at]; ok && firstGap < 0 && s0 != g.RandSeed() {
			firstGap = i
			t.Logf("第一個岔開：順序表第 %d 格（郡 %d，勢力 %d）"+
				"開始前原版的狀態 0x%08x，remake 0x%08x"+
				"——前一個郡（%d）消耗的次數不一樣",
				i, at, q.Owner, s0, g.RandSeed(),
				func() int {
					if i > winFrom {
						return turnSeq[i-1]
					}
					return -1
				}())
		}
		if at == firstGapAt {
			t.Logf("郡 %d 回合開始：原版 %s", at, startLog[at])
			t.Logf("郡 %d 回合開始：remake 兵(百) %d 金 %d 米 %d",
				at, func() int {
					n := 0
					for _, x := range g.Garrison(at) {
						n += x.Soldiers
					}
					return n / 100
				}(), q.Gold, q.Rice)
		}
		d0 := g.RandDraws()
		if at == firstGapAt {
			for k, v := range mineTbl {
				firstMine[k] = -v
			}
		}
		if _, n, err := pp.ActPrefecture(g, q.Owner, at, g.AILevel(q.Owner)); err != nil {
			t.Errorf("郡 %d（勢力 %d）的命令有 %d 道成立，然後：%v", at, q.Owner, n, err)
		}
		mineBy[at] = g.RandDraws() - d0
		if at == firstGapAt {
			for k, v := range mineTbl {
				if firstMine[k] += v; firstMine[k] == 0 {
					delete(firstMine, k)
				}
			}
		}
	}

	rm, rs, rg, err := g.Tables()
	if err != nil {
		t.Fatalf("remake 的盤面寫不回三張表：%v", err)
	}
	mine := append(append(append([]byte{}, rm...), rs...), rg...)
	dumpTables(t, mine, "parity-02-remake走完")

	t.Logf("抽亂數：原版 %d 次、remake %d 次（郡回合之外原版抽了 %d 次）",
		randCalls, g.RandDraws(), randBy[-1])
	t.Log("原版逐表抽亂數的次數：")
	for _, tb := range tables {
		if n := randTbl[tb.name]; n > 0 {
			t.Logf("　  %-8s %4d", tb.name, n)
		}
	}
	t.Logf("　  %-8s %4d", "郡回合之外", randTbl["郡回合之外"])
	for _, tb := range extra {
		t.Logf("　  %-16s 抽 %4d 次（進去 %d 次）",
			tb.name, randTbl[tb.name], hits[tb.name])
	}
	t.Logf("兵書那一支的漏斗：進入 %d → 過 RND(100) %d → 過庫存 %d", fIn, fRnd, fStock)
	// 四支寶物常式的桶要併回賞賜物品，否則比較欄位對不起來。
	t.Log("四種寶物逐支（原版／remake）：")
	for _, k := range []string{"賞賜物品：兵書", "賞賜物品：寶刀",
		"賞賜物品：美女", "賞賜物品：駿馬"} {
		t.Logf("　  %s 原版 %4d／remake %4d（差 %+d）",
			k, randTbl[k], mineTbl[k], mineTbl[k]-randTbl[k])
		randTbl["賞賜物品"] += randTbl[k]
		mineTbl["賞賜物品"] += mineTbl[k]
	}
	t.Logf("原版挖角觸發 %d 次：郡 %v", len(hhAt), hhAt)
	for k, v := range mineTbl {
		if len(k) > 6 && k[:9] == "挖角：" {
			t.Logf("remake 挖角觸發：%s ×%d", k, v)
		}
	}
	t.Logf("進貢漏斗（原版）：迴圈 %d 次 → 過領地 %d → 過人才 %d → 重抽 %d 次"+
		"（合計 %d）", tbLoop, tbLand, tbTalent, tbAgain, tbLand+tbTalent*9+tbAgain)
	t.Log("逐表抽亂數：原版／remake")
	for _, tb := range tables {
		if randTbl[tb.name] > 0 || mineTbl[tb.name] > 0 {
			t.Logf("　  %-8s 原版 %4d／remake %4d（差 %+d）",
				tb.name, randTbl[tb.name], mineTbl[tb.name],
				mineTbl[tb.name]-randTbl[tb.name])
		}
	}

	type gap struct{ p, a, b int }
	var gaps []gap
	for p := 0; p < 43; p++ {
		if randBy[p] != 0 || mineBy[p] != 0 {
			gaps = append(gaps, gap{p, randBy[p], mineBy[p]})
		}
	}
	sort.Slice(gaps, func(i, j int) bool {
		return abs(gaps[i].a-gaps[i].b) > abs(gaps[j].a-gaps[j].b)
	})
	t.Log("逐郡抽亂數的次數（原版／remake，差最多的在前）：")
	for i, x := range gaps {
		if i >= 12 {
			break
		}
		t.Logf("    郡 %2d：原版 %3d／remake %3d（差 %+d）", x.p, x.a, x.b, x.b-x.a)
	}
	t.Logf("寶庫（進貢後）原版 %v", tbAfter)
	t.Logf("寶庫（進貢後）remake %v", ra)
	for k, v := range mineTbl {
		if strings.HasPrefix(k, "內政：郡 1 ") {
			t.Logf("remake 的內政：%s（%d 次）", k, v)
		}
	}
	if len(csLog) > 0 {
		t.Logf("原版徵兵逐人（順序即名單順序）：%s", strings.Join(csLog, " "))
	}
	for _, ln := range valLog {
		t.Logf("原版逐表的值：%s", ln)
	}
	for _, ln := range soLog {
		if strings.HasPrefix(ln, "郡 1 ") {
			t.Logf("原版的出兵閘門：%s", ln)
		}
	}
	for k, v := range mineTbl {
		if strings.HasPrefix(k, "出兵：郡 1 ") {
			t.Logf("remake 的出兵閘門：%s（%d 次）", k, v)
		}
	}
	if len(atFirstTurn) == total {
		t.Logf("月底結算 → 第一個郡的回合之間原版動了 %d 個位元組%s",
			differs8(atSettle, atFirstTurn),
			where(atSettle, atFirstTurn, nMas, nSta))
		t.Log(byPrefecture(atSettle, atFirstTurn, nMas, nSta))
	}
	if len(atSettle) == total {
		dumpTables(t, atSettle, "parity-005-結算前")
		t.Logf("快照 → 月底結算之間原版動了 %d 個位元組%s",
			differs8(before, atSettle),
			where(before, atSettle, nMas, nSta))
		t.Log(byPrefecture(before, atSettle, nMas, nSta))
	}
	t.Logf("郡 %d 逐表（原版）：%v", firstGapAt, firstTbl)
	t.Logf("郡 %d 逐表（remake）：%v", firstGapAt, firstMine)
	t.Log("照順序表的次序（第一個對不上的就是要追的）：")
	for i := winFrom; i < winTo && i < len(turnSeq); i++ {
		at := turnSeq[i]
		if randBy[at] == 0 && mineBy[at] == 0 {
			continue
		}
		flag := ""
		if randBy[at] != mineBy[at] {
			flag = "  ←"
		}
		t.Logf("    第 %2d 格 郡 %2d：原版 %3d／remake %3d%s",
			i, at, randBy[at], mineBy[at], flag)
	}
	t.Logf("原版 vs remake：差 %d 個位元組%s",
		differs8(after, mine), where(after, mine, nMas, nSta))
	t.Log(byPrefecture(after, mine, nMas, nSta))

	// 人物表逐人：兵力／訓練／武裝三欄佔了剩下位元組差的八成，
	// 而抽樣次數已經對上——列出來才知道是哪幾支算式還沒對。
	gb := after[nMas+nSta:]
	gm := mine[nMas+nSta:]
	shown := 0
	for i := 0; i+30 <= len(gb) && i+30 <= len(gm); i += 30 {
		a, b := gb[i:i+30], gm[i:i+30]
		if a[22] == b[22] && a[23] == b[23] && a[24] == b[24] && a[25] == b[25] &&
			a[9] == b[9] && a[10] == b[10] && a[11] == b[11] && a[16] == b[16] {
			continue
		}
		if shown++; shown > 12 {
			continue
		}
		t.Logf("    人物 %3d（郡 %d 勢力 %d 身分 %d）："+
			"兵 %5d／%5d　訓 %3d／%3d　武裝 %3d／%3d　"+
			"智 %3d／%3d　武 %3d／%3d　魅 %3d／%3d　忠 %3d／%3d",
			i/30, a[19], a[18], a[17],
			int(a[22])|int(a[23])<<8, int(b[22])|int(b[23])<<8,
			a[24], b[24], a[25], b[25],
			a[9], b[9], a[10], b[10], a[11], b[11], a[16], b[16])
	}
	t.Logf("人物表對不上的共 %d 位", shown)

	// **這是硬閘門。** 2026-09-08 起整個月逐位元組相同——43 個郡、
	// 350 位人物、16 個勢力，十張分派表的抽亂數次數也全部 ±0。
	// 差一個位元組就是有一條規則被改壞了，上面那幾行會指出是哪一欄。
	if n := differs8(after, mine); n != 0 {
		t.Errorf("月度對拍差 %d 個位元組（應該是 0）", n)
	}
}

// byPrefecture 把州郡表的差異逐郡列出來，欄位印名字。
//
// **總數看不出東西。** 「差 400 個位元組」與「40 個郡各差一個物價」
// 是同一個數字，而後者才是線索。
func byPrefecture(a, b []byte, nMas, nSta int) string {
	rec := state.PrefectureRecordSize
	var lines []string
	for i := 0; i*rec < nSta; i++ {
		lo := nMas + i*rec
		fields := map[string]bool{}
		for j := 0; j < rec && lo+j < len(b); j++ {
			if a[lo+j] != b[lo+j] {
				fields[fieldName(prefField, j)] = true
			}
		}
		if len(fields) == 0 {
			continue
		}
		names := make([]string, 0, len(fields))
		for n := range fields {
			names = append(names, n)
		}
		sort.Strings(names)
		// **所屬要一起印。** 「哪個欄位變了」不接上「那是誰的郡」，
		// 就分不出是電腦諸侯下的令還是每月結算。
		line := fmt.Sprintf("    郡 %2d（勢力 %d）：%s",
			i, b[lo+30], strings.Join(names, " "))
		// 主事者（offset 32）差的時候把兩邊的槽號印出來——欄位名說不出
		// 「差在哪一位」，而那正是唯一有用的線索。
		if fields[fieldName(prefField, 32)] || fields[fieldName(prefField, 33)] {
			line += fmt.Sprintf("｜主事者 原版 %d／remake %d",
				int(a[lo+32])|int(a[lo+33])<<8, int(b[lo+32])|int(b[lo+33])<<8)
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return "    州郡表逐郡相同"
	}
	if len(lines) > 20 {
		lines = append(lines[:20], fmt.Sprintf("    …（還有 %d 個郡）", len(lines)-20))
	}
	return "州郡表逐郡：\n" + strings.Join(lines, "\n")
}

// byGeneral 把人物表的差異逐人列出來。
//
// 訓練度、武裝度、忠誠、所在——電腦諸侯下了什麼令，多半是從這裡看出來的。
func byGeneral(a, b []byte, nMas, nSta int) string {
	rec := state.GeneralRecordSize
	lo0 := nMas + nSta
	var lines []string
	for i := 0; lo0+i*rec+rec <= len(a) && lo0+i*rec+rec <= len(b); i++ {
		lo := lo0 + i*rec
		var fields []string
		for j := 0; j < rec; j++ {
			if a[lo+j] != b[lo+j] {
				n := fieldName(genField, j)
				if len(fields) == 0 || fields[len(fields)-1] != n {
					fields = append(fields, n)
				}
			}
		}
		if len(fields) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("    人物 %3d（勢力 %d、所在 %d）：%s",
			i, b[lo+18], b[lo+19], strings.Join(fields, " ")))
	}
	if len(lines) == 0 {
		return "    人物表逐筆相同"
	}
	if len(lines) > 24 {
		lines = append(lines[:24], fmt.Sprintf("    …（還有 %d 人）", len(lines)-24))
	}
	return "人物表逐筆：\n" + strings.Join(lines, "\n")
}
