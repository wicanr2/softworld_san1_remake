//go:build oracle

package parity

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 加強版的行為觸發開機（Issue #20；原版的在 boot_behavior_oracle_test.go、
// `docs/spec/015`）。
//
// 路標分兩種：**畫面雜湊**（三英圖、主選單、年代、玩家數——這幾張兩版
// 用同一批圖，但停點各自量）與**原版自己的輸入常式**（數字欄位
// `plusNumInputFn`、Y/N `plusKeyInputFn`，位址是拿原版的指令形狀在加強版
// 碼段裡比出來的，`TestZZPlusBootProbe` 實跑驗過呼叫端與範圍）。
// 選單三步的字元輸入常式（原版 `1058:17D7`／`33D8:1FAB` 那些呼叫端）
// 在加強版對不出來——加強版用 286 指令編譯，形狀不同——所以用畫面
// 雜湊等；掃描碼等待點倒是同一套 runtime，形狀對得上（§下）。
//
// ⚠ 這些雜湊與位址**只對 `ASV.EXE`（SHA-256 `ad18a251…`）成立**。
//
// 掃描碼輸入常式（原版 `1058:0E24`，`docs/spec/015` §2）在加強版是
// `0x10b8e`（`0FE4:0D4E`），bytes 對讀如下（`workplace/dump/plus/code-plus-00b000.bin`）：
//
//	10b8e  33 c0 / 9a 1e 05 b9 05      xor ax,ax / call 05B9:051E   （原版 1058:0E24）
//	10b98  6a 00 / 9a 8e 37 b9 05      push 0 / call 05B9:378E      吃掉上一鍵（原版 0E2E）
//	10ba2  6a 01 / … / 0b c0 / 75 e8   有鍵就再吃，等放開          （原版 0E45–0E47）
//	10bb4  6a 63 / 0e e8 84 f8         call 0x1043e(0x63)：十六項分派器（原版 1058:065F）
//	10bbd  6a 01 / 9a 8e 37 b9 05      push 1 / call 05B9:378E      **等新鍵**（原版 1058:0E57）
//	10bc7  0b c0 / 74 e9               沒鍵就回 10bb4               （原版 0E63–0E65）
//	10bd4  6a 00 / … / 2a e4 / cb      讀鍵、回 AL                  （原版 0E7F）
//
// 差別只有 `push imm8`（286 才有）取代 `mov ax,imm / push ax`，
// 所以 `0x10bbd` 就是加強版「正要等新鍵」的同步點，送掃描碼要在這裡。
var (
	plusTitleScreen = [sha256.Size]byte{ // 三英圖（與原版 bootOpeningTitleScreen 相同的一張圖）
		0x8d, 0xc9, 0x28, 0xa0, 0x13, 0xf6, 0xb7, 0x44, 0x77, 0xbc, 0x12, 0x4c, 0xbd, 0xe7, 0x7e, 0x19,
		0x1a, 0x9a, 0x03, 0x3f, 0xaa, 0x19, 0xc4, 0x03, 0x34, 0x7d, 0x71, 0x0b, 0x6f, 0x11, 0xe9, 0xaa}
	plusMenuScreen = [sha256.Size]byte{
		0xd6, 0xcb, 0xf7, 0xe7, 0x6b, 0x6c, 0xce, 0x5e, 0x64, 0x54, 0x35, 0xd5, 0xf2, 0x55, 0x1d, 0xb7,
		0x9c, 0xd6, 0x11, 0x41, 0xfe, 0x2d, 0xaa, 0x80, 0xeb, 0x74, 0x06, 0x38, 0x79, 0x14, 0xb6, 0xa4}
	plusEraScreen = [sha256.Size]byte{
		0x55, 0xdc, 0x14, 0xe2, 0xa2, 0x76, 0x06, 0x04, 0xbb, 0x5a, 0xcb, 0x6b, 0x20, 0xa5, 0xf3, 0x52,
		0x22, 0x96, 0x6e, 0x20, 0x57, 0x1f, 0x62, 0xce, 0x2c, 0xcd, 0x3d, 0xc2, 0xd9, 0xd4, 0x5d, 0x27}
	plusPlayersScreen = [sha256.Size]byte{
		0x22, 0x59, 0x15, 0x3b, 0x49, 0x16, 0x62, 0x44, 0xae, 0xa4, 0xa0, 0x37, 0xf6, 0x9c, 0x82, 0xf7,
		0x01, 0x06, 0xf8, 0x02, 0xf3, 0x77, 0x41, 0xb3, 0x81, 0x7b, 0x82, 0xf3, 0xd0, 0x34, 0xd3, 0x5a}
)

const (
	plusLordPickCaller   = 0x11848 // 數字 1..6（原版 1538:2EB8 的對應）
	plusDifficultyCaller = 0x1192e // 數字 1..20（加強版的上限是 20，`docs/spec/004`）
	plusPasswordCaller   = 0x04100 // 數字 0..9999（原版 03EB:02C2）
	plusPasswordYNCaller = 0x0411b // Y/N（原版 03EB:02DE）
	plusMainCmdCaller    = 0x16493 // 數字 0..9（原版 1538:236B）
	plusTablesBase       = 0x36200 // 三張表的基底（`TestZZMasterRecordAccessCensusPlus` 搜到的）
	plusScanWaitAt       = 0x10bbd // 掃描碼輸入常式「等新鍵」那一道（原版 1058:0E57）
)

// waitPlusScan 等加強版走到掃描碼輸入常式的「等新鍵」那一道——送掃描碼
// 的同步點（原版的 waitBootScan）。早於這裡送，鍵會被 `0x10b98` 那段
// 「吃掉上一鍵」吞掉。
func waitPlusScan(t *testing.T, o *oracle.Oracle, name string, budget uint64) {
	t.Helper()
	if err := o.RunUntil(oracle.At(addr(plusScanWaitAt)), oracle.Budget(budget)); err != nil {
		t.Fatalf("等待%s的掃描碼讀取迴圈失敗（step=%d、畫面 SHA-256=%x）：%v",
			name, o.Steps(), sha256.Sum256(screenOf(o)), err)
	}
}

type plusSignals struct {
	lordAsk, difficultyAsk, passwordAsk, passwordYN, mainAsk, keyAsk int
}

func observePlus(o *oracle.Oracle) *plusSignals {
	s := &plusSignals{}
	o.OnCall(addr(plusNumInputFn), func(o *oracle.Oracle) {
		lo, hi := int(int16(o.Arg(0))), int(int16(o.Arg(1)))
		switch o.Caller().Linear() {
		case plusLordPickCaller:
			if lo == 1 && hi == 6 {
				s.lordAsk++
			}
		case plusDifficultyCaller:
			if lo == 1 && hi == 20 {
				s.difficultyAsk++
			}
		case plusPasswordCaller:
			if lo == 0 && hi == 9999 {
				s.passwordAsk++
			}
		case plusMainCmdCaller:
			if lo == 0 && hi == 9 {
				s.mainAsk++
			}
		}
	})
	o.OnCall(addr(plusKeyInputFn), func(o *oracle.Oracle) {
		s.keyAsk++
		if o.Caller().Linear() == plusPasswordYNCaller {
			s.passwordYN++
		}
	})
	return s
}

// bootToNewGamePlus 把加強版開到「開始新遊戲 → 年代 1 → 一人 → 選君主 →
// 難度」之後的第一個主命令輸入，回傳三張表的基底。
func bootToNewGamePlus(t *testing.T, o *oracle.Oracle, lord, difficulty int) (uint32, *plusSignals) {
	t.Helper()
	s := observePlus(o)
	o.Type(envOr("SAN1_PLUSKEY", "122"))
	waitBootScreen(t, o, "加強版三英圖", plusTitleScreen, 1_000_000_000)
	o.TypeBoth("\r")
	waitBootScreen(t, o, "加強版主選單", plusMenuScreen, 1_000_000_000)
	waitPlusScan(t, o, "加強版主選單", 50_000_000)
	o.Drain()
	o.PressScan("1")
	waitBootScreen(t, o, "加強版年代畫面", plusEraScreen, 500_000_000)
	waitPlusScan(t, o, "加強版年代畫面", 50_000_000)
	o.Drain()
	o.PressScan("1")
	waitBootScreen(t, o, "加強版玩家數畫面", plusPlayersScreen, 500_000_000)
	waitPlusScan(t, o, "加強版玩家數畫面", 50_000_000)
	o.Drain()
	o.TypeBoth("1\r")
	waitBoot(t, o, "加強版選君主輸入", 500_000_000, func() bool { return s.lordAsk > 0 })
	waitPlusScan(t, o, "加強版選君主輸入", 5_000_000)
	o.Drain()
	o.PressScan(fmt.Sprintf("%d\r", lord))
	waitBoot(t, o, "加強版難度輸入", 500_000_000, func() bool { return s.difficultyAsk > 0 })
	waitPlusScan(t, o, "加強版難度輸入", 5_000_000)
	o.Drain()
	o.TypeBoth(fmt.Sprintf("%d\r", difficulty))
	beforeMain := s.mainAsk
	waitBoot(t, o, "加強版防拷或主命令輸入", 800_000_000,
		func() bool { return s.passwordAsk > 0 || s.mainAsk > beforeMain })
	if s.passwordAsk > 0 {
		o.Drain()
		o.PressScan(passwordAnswer + "\r")
		waitBoot(t, o, "加強版密碼確認", 300_000_000, func() bool { return s.passwordYN > 0 })
		o.Drain()
		o.PressScan("Y")
	}
	// 就任對白：Enter 到主命令出現為止（上限 32 句是失敗即關閉的護欄）。
	for i := 0; i < 32 && s.mainAsk == beforeMain; i++ {
		before := s.keyAsk
		o.Drain()
		o.PressScan("\r")
		waitBoot(t, o, fmt.Sprintf("加強版就任對白第 %d 句", i+1), 200_000_000,
			func() bool { return s.mainAsk > beforeMain || s.keyAsk > before })
	}
	if s.mainAsk == beforeMain {
		t.Fatalf("加強版就任對白超過 32 句仍未到主命令輸入")
	}
	waitPlusScan(t, o, "加強版主命令", 5_000_000)
	return plusTablesBase, s
}

// TestZZNewGameBoardPlus：加強版開新局（劇本一、曹操、難度 5）之後的三張表
// 與 remake `game.New(…, state.EditionPlus)` 逐位元組比；差異只能落在
// `newGameRuntimeFields`（與原版那支 `TestZZNewGameBoardBase` 同一份遮罩）。
//
// 拍的時刻是**第一個郡回合的入口**（`plusTurnEntry`）：那時開局設定
// （操縱方、電腦 AI 等級、物價）已經寫好，而還沒有任何一個郡動過。
// 等到主命令再拍就晚了——玩家的郡 11 排在十個電腦郡後面，那十個郡的
// 內政與登用已經改掉幾百個位元組。
func TestZZNewGameBoardPlus(t *testing.T) {
	root := plusRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	o, err := oracle.Load(filepath.Join(root, "ASV.EXE"), root)
	if err != nil {
		t.Fatalf("加強版載不進來：%v", err)
	}
	defer o.Close()
	var board []byte
	o.OnCall(addr(plusTurnEntry), func(o *oracle.Oracle) {
		if board == nil {
			board = o.Bytes(addr(plusTablesBase), state.MasterTableSize+state.PrefectureTableSize+state.GeneralTableSize)
		}
	})
	bootToNewGamePlus(t, o, caoCaoPick, 5)
	if board == nil {
		t.Fatal("走到主命令了，郡回合入口卻一次都沒攔到（位址攔錯了？）")
	}
	checkNewGameBoard(t, state.EditionPlus, sc0, board)
}

// newGameRuntimeFields 是「開新局之後、第一個郡回合之前」原版自己會改寫
// 而 remake 的 `game.New` 不重現的欄位——兩版共用，量到才登記
// （`docs/spec/015` §5）。目前只有物價：那是**開月**重抽的（`0x17364`、
// `docs/mechanics/60` §1.2），remake 在月初才抽，開局的表裡還是劇本值；
// 這裡只驗它落在 30–68。其餘開局改寫（操縱方、填充槽、AI 等級）remake
// 都在 `game.New` 做了，要逐位元組相同。
var newGameRuntimeFields = map[string]bool{"州郡 offset 29": true}

// checkNewGameBoard 把原版第一個郡回合入口拍到的三張表與 remake 的
// `game.New` 比：玩家要是曹操；逐位元組差異歸類到（表、offset）。
func checkNewGameBoard(t *testing.T, ed state.Edition, sc0 *state.Scenario, board []byte) {
	t.Helper()
	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	sc, err := state.DecodeTables(state.Slot("001"), board[:nMas], board[nMas:nMas+nSta], board[nMas+nSta:])
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
	t.Logf("%s：玩家勢力 %d，君主 %s", ed, players[0], lord.Name)
	if lord.Name != "曹操" {
		t.Errorf("選君主送 %d 拿到的是 %s，不是曹操", caoCaoPick, lord.Name)
	}

	g, err := game.New(sc0, state.FactionID(players[0]), 5, ed)
	if err != nil {
		t.Fatal(err)
	}
	rm, rs, rg, err := g.Tables()
	if err != nil {
		t.Fatal(err)
	}
	mine := append(append(append([]byte{}, rm...), rs...), rg...)
	if len(mine) != len(board) {
		t.Fatalf("remake 的三張表 %d 個位元組，原版 %d", len(mine), len(board))
	}
	byField := map[string][]int{}
	for i := range mine {
		if mine[i] == board[i] {
			continue
		}
		byField[whichField(i, nMas, nSta)] = append(byField[whichField(i, nMas, nSta)], i)
	}
	keys := make([]string, 0, len(byField))
	for k := range byField {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		offs := byField[k]
		sample := ""
		for j, i := range offs {
			if j >= 6 {
				sample += " …"
				break
			}
			sample += fmt.Sprintf(" [%s %02x→%02x]", whichTable(i, nMas, nSta), mine[i], board[i])
		}
		t.Logf("%s：%s 差 %d 格：%s", ed, k, len(offs), sample)
		if !newGameRuntimeFields[k] {
			t.Errorf("%s 開局的三張表與 remake 不同：%s（%d 格）", ed, k, len(offs))
		}
	}
	// 物價：開月重抽的，範圍 30–68（`docs/mechanics/60` §1.2）。
	for i := 0; i < state.PrefectureCount+1; i++ {
		if v := int(board[nMas+i*state.PrefectureRecordSize+29]); v < 30 || v > 68 {
			t.Errorf("%s：郡 %d 開局的物價 %d 不在 30–68", ed, i, v)
		}
	}
	// 三件開局改寫的值——remake 做了才會逐位元組相同，這裡再各自點名一次，
	// 讓失敗訊息說得出是哪一件。
	if v := binary.LittleEndian.Uint16(board[players[0]*72:]); v != 1 {
		t.Errorf("%s：玩家勢力的操縱方是 %#x，不是 1", ed, v)
	}
	for f := 0; f < 16; f++ {
		lordSlot := int(binary.LittleEndian.Uint16(board[f*72+2:]))
		ctl := binary.LittleEndian.Uint16(board[f*72:])
		if sc0Lord := int(binary.LittleEndian.Uint16(sc0Mas(sc0)[f*72+2:])); sc0Lord >= 346 {
			gen := board[nMas+nSta+sc0Lord*30:]
			if ctl != 0xFFFF || lordSlot != 0xFFFF || gen[17] != 12 || gen[18] != 0xFF || gen[19] != 0xFF {
				t.Errorf("%s：填充槽 %d 沒清乾淨：操縱方 %#x、君主 %#x、範本身分 %d 勢力 %#x 領地 %#x",
					ed, f, ctl, lordSlot, gen[17], gen[18], gen[19])
			}
		}
	}
	if len(byField) == 0 {
		t.Logf("%s：三張表與 remake 逐位元組相同", ed)
	}
}

func sc0Mas(sc *state.Scenario) []byte {
	m, _, _ := sc.Tables()
	return m
}

func whichField(off, nMas, nSta int) string {
	switch {
	case off < nMas:
		return fmt.Sprintf("諸侯 offset %d", off%state.MasterRecordSize)
	case off < nMas+nSta:
		return fmt.Sprintf("州郡 offset %d", (off-nMas)%state.PrefectureRecordSize)
	default:
		return fmt.Sprintf("人物 offset %d", (off-nMas-nSta)%state.GeneralRecordSize)
	}
}

// TestZZNewGameAILevelByDifficulty 對拍「難度怎麼改寫十六個勢力的 AI 等級」
// （諸侯 offset 4）——兩版**不一樣**（`docs/mechanics/90` §6.4）：
//
//	原版 0x12317   d ≤ 2 → 逐勢力 AI > 2 就寫 2，**碰到第一個 AI ≤ 2 的就停**
//	               （0x1236f 跳去 0x12324：加 (d−2)÷2 ＝ 0 的迴圈，跑完 f 已是 16）
//	               d > 2 → 不加；只做「≥ 5 寫 5」
//	加強版 0x11941 d ≤ 2 → 逐勢力 AI > 2 就寫 2，十六個都看
//	               d > 2 → 每個勢力 += (d mod 11) ÷ 2，再「≥ 5 寫 5」
//
// 劇本一的等級是 [3 4 3 3 2 2 1 2 1 0…]，原版那個「碰到 ≤ 2 就停」在
// 劇本上看不出來（第一個 ≤ 2 之後沒有 > 2 的）。所以在難度欄位出現時把
// 十六格改成 `craftedLevels`——第一格就是 1，後面還有 4 與 5——再看
// 第一個郡回合入口的十六個值。remake 那邊是 `game.AILevelsAtStart`。
func TestZZNewGameAILevelByDifficulty(t *testing.T) {
	craftedLevels := [16]int{1, 4, 3, 0, 5, 2, 3, 1, 4, 0, 2, 5, 0, 3, 1, 4}
	type run struct {
		ed   state.Edition
		diff int
	}
	// 原版 d > 2 只走「≥ 5 壓 5」那一段（`0x1231a` 直接跳過去），3..10 同一
	// 條路，量 5 就夠；原版難度 10 的開機在密碼確認那一步等不到（兩位數
	// 的難度輸入之後鍵的時序不同），所以不列。
	runs := []run{
		{state.EditionBase, 1}, {state.EditionBase, 2}, {state.EditionBase, 5},
		{state.EditionPlus, 2}, {state.EditionPlus, 3}, {state.EditionPlus, 10},
		{state.EditionPlus, 11}, {state.EditionPlus, 12}, {state.EditionPlus, 20},
	}
	if v := envOr("SAN1_DIFF", ""); v != "" {
		var d int
		fmt.Sscan(v, &d)
		runs = []run{{state.EditionBase, d}, {state.EditionPlus, d}}
	}
	for _, r := range runs {
		t.Run(fmt.Sprintf("%s-難度%d", r.ed, r.diff), func(t *testing.T) {
			var root, exe string
			var numFn, turnAt, tables uint32
			var diffHi int
			if r.ed == state.EditionPlus {
				root, exe = plusRoot(t), "ASV.EXE"
				numFn, turnAt, tables, diffHi = plusNumInputFn, plusTurnEntry, plusTablesBase, 20
			} else {
				root, exe = origRoot(t), "AA.EXE"
				numFn, turnAt, tables, diffHi = bootNumInputFn, 0x1746e, 0x399b0, 10
			}
			c := openContainer(t, filepath.Join(root, "DATA2"))
			sc0, err := state.LoadScenario(c, state.Slot("001"))
			if err != nil {
				t.Fatal(err)
			}
			seedMas, _, _ := sc0.Tables()
			o, err := oracle.Load(filepath.Join(root, exe), root)
			if err != nil {
				t.Fatal(err)
			}
			defer o.Close()
			planted := false
			o.OnCall(addr(numFn), func(o *oracle.Oracle) {
				if planted || int(int16(o.Arg(0))) != 1 || int(int16(o.Arg(1))) != diffHi {
					return
				}
				planted = true
				for f := 0; f < 16; f++ {
					o.SetWord(addr(tables+uint32(f*72+4)), uint16(craftedLevels[f]))
				}
			})
			var got [16]int
			taken := false
			o.OnCall(addr(turnAt), func(o *oracle.Oracle) {
				if taken {
					return
				}
				taken = true
				for f := 0; f < 16; f++ {
					got[f] = int(o.Word(addr(tables + uint32(f*72+4))))
				}
			})
			if r.ed == state.EditionPlus {
				bootToNewGamePlus(t, o, caoCaoPick, r.diff)
			} else {
				bootToNewGameAt(t, o, caoCaoPick, r.diff, seedMas)
			}
			if !planted || !taken {
				t.Fatalf("種等級 %v、讀等級 %v", planted, taken)
			}
			want := game.AILevelsAtStart(craftedLevels[:], r.diff, r.ed)
			t.Logf("%s 難度 %2d：種 %v", r.ed, r.diff, craftedLevels)
			t.Logf("　　　　　　原版 %v", got)
			t.Logf("　　　　　　remake %v", want)
			for f := 0; f < 16; f++ {
				if got[f] != want[f] {
					t.Errorf("勢力 %d：原版 %d、remake %d", f, got[f], want[f])
				}
			}
		})
	}
}
