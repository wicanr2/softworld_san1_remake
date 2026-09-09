//go:build oracle

package parity

import (
	"bytes"
	"fmt"
	"path/filepath"
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
	name  string
	keys  []string
	apply func(g *game.State, at, gi int, me state.FactionID) error
}

const playerSettle = 40_000_000

// 目前的郡在工作段的位移（`docs/re/03`：`imul es:[0x30fc]` × 176）。
const curPrefOff = 0x30fc

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

	snap := o.Save()
	for _, tc := range playerCases() {
		t.Run(tc.name, func(t *testing.T) {
			o.Restore(snap)
			// **錢糧直接寫進去。** 這份存檔的郡庫不夠，防洪、建關寨、
			// 尋訪、登用、撤職全部會被原版當場擋掉（「須用 N 金」之後
			// 回主提示），那時兩邊都「什麼都沒做」而看起來相同——
			// 對拍到的是拒絕不是命令。
			purse := base + uint32(nMas+at*state.PrefectureRecordSize)
			o.SetWord(addr(purse+18), 9000) // 金
			o.SetWord(addr(purse+20), 9000) // 米
			before := board()
			seed := seedOf()
			for i, k := range tc.keys {
				o.Drain()
				o.PressScan(k)
				if err := o.Run(playerSettle); err != nil {
					t.Fatalf("送第 %d 段 %q 時停止：%v", i+1, k, err)
				}
			}
			after := board()

			// **正對照。** 盤面一個位元組都沒動時，remake 那一邊多半也
			// 「什麼都沒做」，兩邊就會「相同」——那是按鍵序列不對，
			// 不是對拍過了。這一條先把那種情況擋掉。
			if bytes.Equal(before, after) {
				dumpScreen(t, o, "cmd-"+tc.name)
				t.Fatalf("原版的盤面一個位元組都沒動——按鍵序列 %v 沒走到底"+
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
			if err := tc.apply(g, at, gi, me); err != nil {
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
			bad, first := 0, -1
			for i := range after {
				if after[i] != got[i] {
					bad++
					if first < 0 {
						first = i
					}
				}
			}
			t.Logf("亂數狀態 0x%08x 出發；原版動了 %d 個位元組，"+
				"兩邊差 %d 個（抽了 %d 次）",
				seed, diffCount(before, after), bad, g.RandDraws())
			if bad != 0 {
				t.Errorf("三張表差 %d 個位元組，第一個在 %s",
					bad, whichTable(first, nMas, nSta))
			}
		})
	}
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
			apply: func(g *game.State, at, gi int, me state.FactionID) error {
				return g.Rest(at, me)
			},
		},
		{
			name: "土地開墾",
			keys: []string{"4\r", "1\r", "1\r"},
			apply: func(g *game.State, at, gi int, me state.FactionID) error {
				return g.Reclaim(at, gi, me)
			},
		},
		{
			name: "洪水防冶",
			keys: []string{"4\r", "2\r", "1\r"},
			apply: func(g *game.State, at, gi int, me state.FactionID) error {
				return g.FloodControl(at, gi, me)
			},
		},
		{
			name: "建築關寨",
			keys: []string{"4\r", "3\r", "1\r"},
			apply: func(g *game.State, at, gi int, me state.FactionID) error {
				return g.BuildFort(at, gi, me)
			},
		},
		{
			name: "訓練兵士",
			keys: []string{"3\r", "1\r"},
			apply: func(g *game.State, at, gi int, me state.FactionID) error {
				return g.Train(at, me)
			},
		},
		{
			name: "徵兵",
			keys: []string{"3\r", "2\r", "1\r", "10\r"},
			apply: func(g *game.State, at, gi int, me state.FactionID) error {
				return g.Conscript(at, gi, 10, me)
			},
		},
		{
			name: "購買武器",
			keys: []string{"3\r", "3\r", "1\r", "10\r"},
			apply: func(g *game.State, at, gi int, me state.FactionID) error {
				return g.BuyArms(at, gi, 10, me)
			},
		},
		{
			name: "買入米糧",
			keys: []string{"5\r", "1\r", "100\r"},
			apply: func(g *game.State, at, gi int, me state.FactionID) error {
				return g.BuyRice(at, 100, me)
			},
		},
		{
			name: "賣出米糧",
			keys: []string{"5\r", "2\r", "100\r"},
			apply: func(g *game.State, at, gi int, me state.FactionID) error {
				return g.SellRice(at, 100, me)
			},
		},
		{
			name: "賞賜金帛",
			keys: []string{"6\r", "3\r", "1\r", "100\r"},
			apply: func(g *game.State, at, gi int, me state.FactionID) error {
				return g.Reward(at, gi, 100, me)
			},
		},
	}
}
