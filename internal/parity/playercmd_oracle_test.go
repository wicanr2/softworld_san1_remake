//go:build oracle

package parity

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 玩家自己下的命令，一道一道對拍。
//
// 既有的二十幾支對拍**繞開玩家選單**，掛在電腦與玩家共用的寫回常式上
// （`affairs_oracle_test.go` 開頭寫了理由：按鍵序列每試一次一分鐘）。
// 那證得到「公式一樣」，證不到**玩家按下去會發生什麼**——選單那一段
// 自己決定誰去做、做幾個單位、扣誰的錢，是另一段碼。
//
// **亂數兩邊對齊，不靠運氣。** 每一道命令送出去之前，把原版的種子
// （`DS:0xa3ae`，`docs/re/03` §1.45）讀出來灌進 remake（`game.SeedRand`），
// 兩邊接著抽同一串數。判準是三張表**逐位元組相同**，不是某幾個欄位。
//
// 對哪個郡下令也不用猜：原版記在 `es:0x30fc`（`docs/re/03`），讀它就好。
//
// 開機一次（兩分半）、存快照，之後每一道還原重試（十幾秒）。
// 每一道命令的按鍵序列由 `TestZZPlayerMenuPrompts` 問出來——那一支
// 印出原版讀了哪些字串常數，所以「它在問什麼」是量的不是猜的。

// playerCase 是一道命令：送進原版的鍵，以及 remake 這一邊的同一道。
//
// `at` 是目前的郡，`gi` 是駐紮名單第一位的人物表槽號——原版的
// 「那一位將軍」清單照槽號排，所以送 `1` 選到的就是它。
type playerCase struct {
	name string
	// plant 在送鍵之前直接寫原版的記憶體，用來把局面擺成要驗的樣子
	// （`CLAUDE.md`：對拍的盤面自己擺，不靠原版的 `RND()` 湊）。
	plant func(o *oracle.Oracle, genRec func(int) uint32, gi int)
	// keys 裡的 `@to` 會換成目標郡的編號（出兵用）。
	keys []string
	// stopAt 是這道命令的取樣點：那支常式被呼叫就表示命令做完了。
	// 0 表示用預設規則（換郡或進月迴圈）。
	stopAt uint32
	apply  func(g *game.State, at, to, gi int, me state.FactionID) error
}

const playerSettle = 40_000_000

// 目前的郡在工作段的位移（`docs/re/03`：`imul es:[0x30fc]` × 176）。
const curPrefOff = 0x30fc

// 月迴圈每一格都先抽一次亂數的那一支（`month_parity_test.go`）。
// 它被呼叫就表示玩家的回合結束、月結算開始了。
const monthLoopTick = 0x15790

// 忠誠在人物記錄的位移（`internal/state`：`Loyalty: rec[16]`）。
// **在野者是 0xFF 不是 0。**
const generalLoyaltyOff = 16

func TestPlayerCommandsMatchTheOriginal(t *testing.T) {
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

	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize,
		state.GeneralTableSize
	total := nMas + nSta + nGen
	board := func() []byte { return o.Bytes(addr(base), total) }

	ds := uint32(o.DSReg()) * 16
	seedOf := func() uint32 {
		return uint32(o.Word(addr(ds+0xa3ae))) |
			uint32(o.Word(addr(ds+0xa3b0)))<<16
	}
	work := o.ES()
	at := int(o.Word(oracle.Addr{Seg: work, Off: curPrefOff}))
	t.Logf("工作段 %04X，目前的郡 %d，亂數狀態 0x%08x", work, at, seedOf())
	if at < 1 || at > 42 {
		t.Fatalf("`es:0x30fc` 讀出來是 %d，不是 1..42 的郡編號——"+
			"工作段取錯了，後面全部不成立", at)
	}

	raw := board()
	sc, err := state.DecodeTables(state.Slot("001"),
		raw[:nMas], raw[nMas:nMas+nSta], raw[nMas+nSta:])
	if err != nil {
		t.Fatalf("原版記憶體裡的三張表解不開：%v", err)
	}
	players := sc.Players()
	if len(players) == 0 {
		t.Skip("這個盤面沒有玩家控制的勢力")
	}
	me := state.FactionID(players[0])
	if owner := int(raw[nMas+at*state.PrefectureRecordSize+30]); owner != int(me) {
		t.Fatalf("目前的郡 %d 屬於勢力 %d，不是玩家（%d）——"+
			"主畫面不在玩家的郡上，命令送不進去", at, owner, me)
	}

	// 駐紮名單：在職（身分 0–3）而且所在郡是 at，照槽號。原版的
	// 「那一位將軍」清單同一個順序，所以送 `1` 就是第一位。
	gi := -1
	var roster []int
	for i := 0; i < nGen/state.GeneralRecordSize; i++ {
		r := raw[nMas+nSta+i*state.GeneralRecordSize:]
		if r[17] <= 3 && int(r[19]) == at {
			roster = append(roster, i)
		}
	}
	if len(roster) == 0 {
		t.Fatalf("郡 %d 沒有在職武將，這一組命令全部下不了", at)
	}
	gi = roster[0]
	t.Logf("郡 %d 的駐紮名單 %v，第一位是槽號 %d", at, roster, gi)

	// 出兵的目標：`at` 的鄰郡（州郡記錄 offset 45–54，`0xFF` 補齊）
	// 裡屬於別的勢力的第一個。
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
		t.Logf("郡 %d 的鄰郡全是自己的，出兵那一道會 skip", at)
	} else {
		t.Logf("出兵的目標是郡 %d（勢力 %d）", to,
			raw[nMas+to*state.PrefectureRecordSize+30])
	}

	snap := o.Save()
	for _, tc := range playerCases() {
		t.Run(tc.name, func(t *testing.T) {
			if to == 0 && strings.Contains(strings.Join(tc.keys, ""), "@to") {
				t.Skip("沒有可以打的鄰郡")
			}
			o.Restore(snap)
			// **錢糧直接寫進去。** 這份存檔的郡庫不夠，防洪、建關寨、
			// 尋訪、登用、撤職全部會被原版當場擋掉（「須用 N 金」之後
			// 回主提示），那時兩邊都「什麼都沒做」而看起來相同——
			// 對拍到的是拒絕不是命令。
			purse := base + uint32(nMas+at*state.PrefectureRecordSize)
			o.SetWord(addr(purse+18), 9000) // 金
			o.SetWord(addr(purse+20), 9000) // 米
			genRec := func(i int) uint32 {
				return base + uint32(nMas+nSta+i*state.GeneralRecordSize)
			}
			if tc.plant != nil {
				tc.plant(o, genRec, gi)
			}
			before := board()
			seed := seedOf()

			// **取樣點是「目前的郡一變」，不是「送完鍵再沉澱」。**
			// 帶 ＊ 的命令（休息、開墾、買米……）一下完就轉移控制權
			//（說明書 p.42）；玩家只有一個郡時那一下會把整個月跑完，
			// 電腦諸侯全部動過一輪。等沉澱完再取，量到的是「原版多跑了
			// 一個月」不是這道命令——實測休息動了 249 個位元組，
			// 散在諸侯表、州郡表、人物表三張裡。
			// **月結算也要擋在取樣點之外。** 只等「換郡」還不夠：玩家
			// 只有一個郡時，那一下就直接進月迴圈，回來還是同一個郡。
			// 實測休息之後諸侯表 6 筆、州郡表 43 筆全動過，而郡 41
			// 只差兩格——位移 14 與位移 29（物價，原版開月時自己算，
			// `docs/mechanics/60-economy` §1.2）。那兩格是下個月的，
			// 不是這道命令的。
			monthBegun := false
			o.OnCall(addr(monthLoopTick), func(*oracle.Oracle) { monthBegun = true })
			hitStop := false
			if tc.stopAt != 0 {
				o.OnCall(addr(tc.stopAt), func(*oracle.Oracle) { hitStop = true })
			}
			moved := oracle.NewCond("命令結束（換郡或進月迴圈）",
				func(o *oracle.Oracle) bool {
					if tc.stopAt != 0 {
						return hitStop
					}
					return monthBegun ||
						int(o.Word(oracle.Addr{Seg: work, Off: curPrefOff})) != at
				})
			var be *oracle.BudgetError
			done := false
			for i, k := range tc.keys {
				k = strings.ReplaceAll(k, "@to", fmt.Sprint(to))
				// **同一道命令裡不要再 Drain。** 對白有顯示時間，
				// 送早了的那個鍵還在佇列裡等它來取；下一段開頭再 Drain
				// 就把它丟掉了——實測宣戰對白因此永遠停在原地，
				// 而畫面上有東西、程式也在跑，看起來不像卡住。
				if i == 0 {
					o.Drain()
				}
				o.PressScan(k)
				err := o.RunUntil(moved, oracle.Budget(playerSettle))
				if err == nil {
					done = true
					break
				}
				if !errors.As(err, &be) {
					t.Fatalf("送第 %d 段 %q 時停止：%v", i+1, k, err)
				}
			}
			if !done {
				// 不轉移控制權的命令（徵兵、購買武器）：再沉澱一次就好。
				if err := o.Run(playerSettle); err != nil {
					t.Fatalf("沉澱時停止：%v", err)
				}
			}
			after := board()

			// **正對照。** 盤面一個位元組都沒動時，remake 那一邊多半也
			// 「什麼都沒做」，兩邊就會「相同」——那是按鍵序列不對，
			// 不是對拍過了。這一條先把那種情況擋掉。
			if bytes.Equal(before, after) {
				// 原版沒動有兩種可能：按鍵序列沒走到底，或者**原版拒絕了
				// 這道命令**（錢不夠、沒有可蓋的地形……）。分得出來的
				// 方法是問 remake：它也拒絕就是兩邊一致。
				sc, err := state.DecodeTables(state.Slot("001"),
					before[:nMas], before[nMas:nMas+nSta], before[nMas+nSta:])
				if err != nil {
					t.Fatalf("盤面解不開：%v", err)
				}
				g, err := game.New(sc, me, 5, state.EditionBase)
				if err != nil {
					t.Fatalf("remake 開不了局：%v", err)
				}
				g.SeedRand(seed)
				if err := tc.apply(g, at, to, gi, me); err != nil {
					t.Logf("兩邊都不做這道命令（remake：%v）", err)
					return
				}
				dumpScreen(t, o, "cmd-"+tc.name)
				t.Fatalf("原版的盤面一個位元組都沒動，但 remake 做了——"+
					"按鍵序列 %v 沒走到底，或者原版拒絕的條件 remake 沒有"+
					"（畫面存到 SAN1_SHOTS）", tc.keys)
			}

			sc, err := state.DecodeTables(state.Slot("001"),
				before[:nMas], before[nMas:nMas+nSta], before[nMas+nSta:])
			if err != nil {
				t.Fatalf("盤面解不開：%v", err)
			}
			g, err := game.New(sc, me, 5, state.EditionBase)
			if err != nil {
				t.Fatalf("remake 開不了局：%v", err)
			}
			g.SeedRand(seed)
			if err := tc.apply(g, at, to, gi, me); err != nil {
				t.Fatalf("remake 這一邊：%v", err)
			}
			rm, rs, rg, err := g.Tables()
			if err != nil {
				t.Fatalf("remake 的盤面寫不回三張表：%v", err)
			}
			got := make([]byte, 0, total)
			got = append(append(append(got, rm...), rs...), rg...)

			if len(got) != len(after) {
				t.Fatalf("表長度不同：原版 %d、remake %d", len(after), len(got))
			}
			t.Logf("亂數狀態 0x%08x 出發；原版動了 %d 個位元組，"+
				"整份盤面兩邊差 %d 個（remake 抽了 %d 次）",
				seed, diffCount(before, after), diffCount(after, got),
				g.RandDraws())
			loy := nMas + nSta + gi*state.GeneralRecordSize + generalLoyaltyOff
			t.Logf("將領 %d 的忠誠：原本 %d → 原版 %d／remake %d",
				gi, before[loy], after[loy], got[loy])
			t.Logf("原版動到的記錄：%s", changedRecords(before, after, nMas, nSta))
			t.Logf("兩邊不同的記錄：%s", changedRecords(after, got, nMas, nSta))

			// **判準是這道命令該碰的範圍，不是整份盤面。**
			// 帶 ＊ 的命令（休息、開墾……）下完就轉移控制權；玩家只有
			// 一個郡時，那一下就把整個月跑完了，電腦諸侯全部動過一輪。
			// 拿整份盤面比，量到的是「原版多跑了一個月」不是這道命令。
			for _, r := range []struct {
				name     string
				off, n   int
			}{
				{fmt.Sprintf("州郡表第 %d 筆", at),
					nMas + at*state.PrefectureRecordSize, state.PrefectureRecordSize},
				{fmt.Sprintf("人物表第 %d 筆", gi),
					nMas + nSta + gi*state.GeneralRecordSize, state.GeneralRecordSize},
				{fmt.Sprintf("諸侯表第 %d 筆", int(me)),
					int(me) * state.MasterRecordSize, state.MasterRecordSize},
			} {
				bad, first := 0, -1
				for i := r.off; i < r.off+r.n; i++ {
					if after[i] != got[i] {
						bad++
						if first < 0 {
							first = i - r.off
						}
					}
				}
				if bad != 0 {
					var where []string
					for i := r.off; i < r.off+r.n && len(where) < 8; i++ {
						if after[i] != got[i] {
							where = append(where, fmt.Sprintf(
								"位移 %d：原版 %d／remake %d（原本 %d）",
								i-r.off, after[i], got[i], before[i]))
						}
					}
					t.Errorf("%s 差 %d 個位元組：%s",
						r.name, bad, strings.Join(where, "、"))
				}
			}
		})
	}
}

// changedRecords 把差異歸戶到「哪幾筆記錄」，方便一眼看出是這道命令
// 動的還是整個月跑過去了。
func changedRecords(a, b []byte, nMas, nSta int) string {
	type tbl struct {
		name string
		off  int
		size int
		n    int
	}
	ts := []tbl{
		{"諸侯", 0, state.MasterRecordSize, nMas / state.MasterRecordSize},
		{"州郡", nMas, state.PrefectureRecordSize, nSta / state.PrefectureRecordSize},
		{"人物", nMas + nSta, state.GeneralRecordSize,
			(len(a) - nMas - nSta) / state.GeneralRecordSize},
	}
	var out []string
	for _, t := range ts {
		var hit []int
		for i := 0; i < t.n; i++ {
			off := t.off + i*t.size
			for k := 0; k < t.size; k++ {
				if a[off+k] != b[off+k] {
					hit = append(hit, i)
					break
				}
			}
		}
		if len(hit) == 0 {
			continue
		}
		if len(hit) > 12 {
			out = append(out, fmt.Sprintf("%s表 %d 筆 %v…", t.name, len(hit), hit[:12]))
			continue
		}
		out = append(out, fmt.Sprintf("%s表 %v", t.name, hit))
	}
	if len(out) == 0 {
		return "（沒有）"
	}
	return strings.Join(out, "；")
}

func diffCount(a, b []byte) int {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return n
}

// whichTable 把整份盤面的位移拆回「哪一張表的第幾筆的第幾格」。
func whichTable(off, nMas, nSta int) string {
	switch {
	case off < nMas:
		return fmt.Sprintf("諸侯表第 %d 筆位移 %d",
			off/state.MasterRecordSize, off%state.MasterRecordSize)
	case off < nMas+nSta:
		off -= nMas
		return fmt.Sprintf("州郡表第 %d 筆位移 %d",
			off/state.PrefectureRecordSize, off%state.PrefectureRecordSize)
	default:
		off -= nMas + nSta
		return fmt.Sprintf("人物表第 %d 筆位移 %d",
			off/state.GeneralRecordSize, off%state.GeneralRecordSize)
	}
}

// playerCases 是要對拍的命令。按鍵序列出自 `TestZZPlayerMenuPrompts`
// 量到的提示，不是猜的：
//
//	4-1 <土地開墾>\n派那一位將軍   4-2 防洪須用10金
//	4-3 建關寨須%d金              4-4 休息 (Y/N):
//	3-1 -=%2d=-（不問，直接做）    3-2/3-3 號 姓名 → 兵士／武裝
//	3-4 調整兵力 確認(Y/N):
//	5-1 您想買多少金的米(0-%d):    5-2 您想賣多少米(0-%d):
//	5-3 您給多少米(0-%d):
//	6-3 <賞賜金帛>\n賞賜那一位將軍
func playerCases() []playerCase {
	return []playerCase{
		{
			name: "休息",
			keys: []string{"4\r", "4\r", "Y"},
			apply: func(g *game.State, at, to, gi int, me state.FactionID) error {
				return g.Rest(at, me)
			},
		},
		{
			name: "土地開墾",
			keys: []string{"4\r", "1\r", "1\r"},
			apply: func(g *game.State, at, to, gi int, me state.FactionID) error {
				return g.Reclaim(at, gi, me)
			},
		},
		{
			name: "洪水防冶",
			keys: []string{"4\r", "2\r", "1\r"},
			apply: func(g *game.State, at, to, gi int, me state.FactionID) error {
				return g.FloodControl(at, gi, me)
			},
		},
		{
			name: "建築關寨",
			keys: []string{"4\r", "3\r", "1\r"},
			apply: func(g *game.State, at, to, gi int, me state.FactionID) error {
				return g.BuildFort(at, gi, me)
			},
		},
		{
			name: "訓練兵士",
			keys: []string{"3\r", "1\r"},
			apply: func(g *game.State, at, to, gi int, me state.FactionID) error {
				return g.Train(at, me)
			},
		},
		{
			name: "徵兵",
			keys: []string{"3\r", "2\r", "1\r", "10\r"},
			apply: func(g *game.State, at, to, gi int, me state.FactionID) error {
				return g.Conscript(at, gi, 10, me)
			},
		},
		{
			name: "購買武器",
			keys: []string{"3\r", "3\r", "1\r", "10\r"},
			apply: func(g *game.State, at, to, gi int, me state.FactionID) error {
				return g.BuyArms(at, gi, 10, me)
			},
		},
		{
			// ⚠ **原版問的是金不是米**：「1 金 = %d 米\n您想買多少金的米」。
			// 送 100 是花 100 金；實測金 9000 → 8900、米 9000 → 9500，
			// 所以這個盤面的匯率是 1 金 5 米。remake 的 `BuyRice` 收的是
			// **米的數量**，所以要換算過再送。
			name: "買入米糧",
			keys: []string{"5\r", "1\r", "100\r"},
			apply: func(g *game.State, at, to, gi int, me state.FactionID) error {
				p := g.Prefecture(at)
				if p == nil {
					return fmt.Errorf("郡 %d 不在盤面上", at)
				}
				return g.BuyRice(at, 100*game.RicePerGold(p.PriceLevel), me)
			},
		},
		{
			// ⚠ **原版問的是米不是金**：「您給多少米(0-%d):」（`0x49a88`）。
			// remake 的 `Relief` 收的是金，所以這一道要看原版實際扣哪一格。
			name: "開倉賑民",
			keys: []string{"5\r", "3\r", "100\r"},
			apply: func(g *game.State, at, to, gi int, me state.FactionID) error {
				return g.Relief(at, 100, me)
			},
		},
		{
			name: "賣出米糧",
			keys: []string{"5\r", "2\r", "100\r"},
			apply: func(g *game.State, at, to, gi int, me state.FactionID) error {
				return g.SellRice(at, 100, me)
			},
		},
		{
			// **主動出兵攻擊。** 取樣點是進戰術層那一刻（`0x2053c`，
			// `docs/re/05` §1）——再往後就開始逐日交戰，那是另一組對拍
			//（`battleday`／`battlefinish`）的事。這裡要驗的是**編隊**：
			// 誰出去、帶走多少錢糧、郡裡剩下什麼。
			//
			// 按鍵序列出自 `workplace/rec10`：宣戰對白三個 Enter、
			// 「請按任一鍵」一個、分配將軍兩步一位、`Y` 收工，
			// 再問攜帶多少金、多少米。
			name: "出兵攻擊",
			keys: []string{
				"2\r", "2\r", "@to\r",
				"\r", "\r", "\r", "\r", "\r",
				"1\r", "1\r", "Y",
				"100\r", "100\r", "Y",
			},
			stopAt: 0x2053c,
			apply: func(g *game.State, at, to, gi int, me state.FactionID) error {
				_, err := g.BeginAttack(at, to, []int{gi}, me,
					game.Supply{Gold: 100, Rice: 100})
				return err
			},
		},
		{
			// 兩個賞賜的差別只有**忠誠的起點**，問的是同一件事：
			// 賞下去的錢是照打進去的數字收，還是照真的換到的忠誠反算。
			//
			// 忠誠滿的時候增幅是 0。照 AI 那條的式子
			// （`花費 = min(增幅 × 100 ÷ 效果, 100)`，`0xd3ed`，對拍過 605 次）
			// 花費也該是 0；實測原版**照樣扣了 100 金**。兩個起點各跑一次
			// 才分得出是「玩家那條一律收滿」還是「忠誠滿另有規則」。
			name: "賞賜金帛（忠誠滿）",
			keys: []string{"6\r", "3\r", "1\r", "100\r"},
			plant: func(o *oracle.Oracle, genRec func(int) uint32, gi int) {
				o.SetByte(addr(genRec(gi)+generalLoyaltyOff), 100)
			},
			apply: func(g *game.State, at, to, gi int, me state.FactionID) error {
				return g.Reward(at, gi, 100, me)
			},
		},
		{
			name: "賞賜金帛（忠誠 50）",
			keys: []string{"6\r", "3\r", "1\r", "100\r"},
			plant: func(o *oracle.Oracle, genRec func(int) uint32, gi int) {
				o.SetByte(addr(genRec(gi)+generalLoyaltyOff), 50)
			},
			apply: func(g *game.State, at, to, gi int, me state.FactionID) error {
				return g.Reward(at, gi, 100, me)
			},
		},
	}
}
