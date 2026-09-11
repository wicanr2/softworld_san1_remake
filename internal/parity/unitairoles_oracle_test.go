//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 戰場上那九個選項分別是什麼動作。
//
// `docs/re/05` §12 把結構解出來了（入口 `0x29014`，九個選項依序試，
// 動作碼 `es:[0x31a8]`），但**各支的角色只確認了三個**——敵人不在旁邊時
// 十三次決策全走到最後一個選項，前面那幾支連試都沒試到。
//
// 這一支換一個問法：**把每一支跑期間叫到的「後果常式」記下來**。
// 交戰結算、計謀判定、天候門、對話框、畫部隊標記各自是什麼早就解了
// （`docs/re/05` §3.6／§4.0／§4.1／§2.5），所以「這一支叫了誰」
// 直接說出它在做什麼——不必讀那幾支的浮點模擬碼。
//
// ⚠ **後果要歸屬到正在跑的那一支**：同一個常式在選項之外也會被叫到
//（玩家自己的動作）。這裡只在「決策鏈進行中而且正停在某一支裡」時記。
func TestZZUnitAIOptionRoles(t *testing.T) {
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
	at, to := stageABattle(t, o, base)

	var dgroup uint16
	o.OnCall(addr(0x2053c), func(o *oracle.Oracle) {
		if dgroup == 0 {
			dgroup = o.DSReg()
		}
	})
	cmdReads := 0
	o.OnCall(addr(0x27a68), func(*oracle.Oracle) { cmdReads++ })

	// 九個選項，順序就是決策鏈裡試的順序。
	options := []struct {
		at   uint32
		name string
	}{
		{0x29e78, "選項1"}, {0x29138, "選項2"}, {0x29344, "選項3"},
		{0x2985c, "選項4"}, {0x29784, "選項5"}, {0x29c56, "選項6"},
		{0x29b82, "選項7"}, {0x29ade, "選項8"}, {0x29e2e, "選項9"},
	}
	// 後果常式：位址的語意全部是已經解出來的（`docs/re/05`）。
	effects := []struct {
		at   uint32
		name string
	}{
		{0x2a224, "交戰結算"}, {0x2abb8, "計謀判定"}, {0x2bee8, "天候門"},
		{0x2a80a, "對話框"}, {0x21b40, "畫部隊"}, {0x30c5b, "單挑接受"},
		{0x2b6aa, "誘敵"}, {0x2bb00, "圍攻"}, {0x2adec, "火攻殺傷"},
		{0x2b15a, "水淹殺傷"}, {0x29896, "弓箭走兩步"}, {0x10b0c, "RND"},
	}

	slot := func() int {
		if dgroup == 0 {
			return -1
		}
		seg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa92e})
		return int(o.Word(oracle.Addr{Seg: seg, Off: 0x31c0}))
	}
	action := func() int {
		if dgroup == 0 {
			return -1
		}
		seg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa930})
		return int(o.Word(oracle.Addr{Seg: seg, Off: 0x31a8}))
	}

	type stat struct {
		tried, settled int
		actions        map[int]int
		calls          map[string]int
	}
	st := map[string]*stat{}
	for _, opt := range options {
		st[opt.name] = &stat{actions: map[int]int{}, calls: map[string]int{}}
	}

	inChain := false
	curOpt := ""
	// 進到下一支（或決策結尾）時，回頭判前一支有沒有定案。
	closeOpt := func() {
		if curOpt == "" {
			return
		}
		if slot() != 0xFFFF {
			s := st[curOpt]
			s.settled++
			s.actions[action()]++
		}
		curOpt = ""
	}
	o.OnCall(addr(0x29014), func(*oracle.Oracle) { inChain, curOpt = true, "" })
	for _, opt := range options {
		name := opt.name
		o.OnCall(addr(opt.at), func(*oracle.Oracle) {
			if !inChain {
				return
			}
			closeOpt()
			st[name].tried++
			curOpt = name
		})
	}
	o.OnCall(addr(0x29132), func(*oracle.Oracle) {
		closeOpt()
		inChain = false
	})
	for _, e := range effects {
		name := e.name
		o.OnCall(addr(e.at), func(*oracle.Oracle) {
			if !inChain || curOpt == "" {
				return
			}
			st[curOpt].calls[name]++
		})
	}

	driveIntoBattle(t, o, at, to)
	if dgroup == 0 {
		t.Fatal("沒有進到主戰場")
	}
	for step := 1; step <= 12 && cmdReads == 0; step++ {
		o.Drain()
		o.PressScan("0")
		if err := o.Run(60_000_000); err != nil {
			t.Fatalf("紮寨第 %d 步停止：%v", step, err)
		}
	}
	// **敵人不在旁邊電腦只會休息**：把玩家那支貼到守軍旁邊，
	// 前面那幾支選項才試得到（`docs/re/05` §12）。
	placeNextToDefender(t, o, dgroup)

	for d := 1; d <= 14; d++ {
		keys := []string{"0", "Y"}
		if d == 1 {
			keys = []string{"Y"}
		}
		for _, k := range keys {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(120_000_000); err != nil {
				t.Fatalf("第 %d 天送 %q 停止：%v", d, k, err)
			}
		}
	}

	t.Logf("九個選項各自的角色（跑 14 天）：")
	for _, opt := range options {
		s := st[opt.name]
		var acts []string
		for v, n := range s.actions {
			acts = append(acts, fmt.Sprintf("%04X×%d", v, n))
		}
		sort.Strings(acts)
		var calls []string
		for k, n := range s.calls {
			calls = append(calls, fmt.Sprintf("%s×%d", k, n))
		}
		sort.Strings(calls)
		t.Logf("  %s(%05X)　試 %3d 次、定案 %3d 次　動作碼 %v　叫了 %v",
			opt.name, opt.at, s.tried, s.settled, acts, calls)
	}

	tried := 0
	for _, s := range st {
		tried += s.tried
	}
	if tried == 0 {
		t.Fatal("決策鏈一次都沒跑到——沒進戰場？")
	}
	// **每一支都要試到過**，否則這一輪答不出它是什麼。
	var never []string
	for _, opt := range options {
		if st[opt.name].tried == 0 {
			never = append(never, opt.name)
		}
	}
	if len(never) > 0 {
		t.Logf("⚠ 這一輪沒試到：%v——它們前面就定案了，要換盤面才問得到",
			never)
	}
}

// placeNextToDefender 把玩家那支部隊擺到守軍旁邊。
//
// 盤面直接寫記憶體：走正規流程要先解出移動那條按鍵路徑，而這一支要問的
// 不是移動。
func placeNextToDefender(t *testing.T, o *oracle.Oracle, dgroup uint16) {
	t.Helper()
	work := o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg})
	occSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9ca})
	colSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9c8})
	rowSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa9c6})
	w16 := func(off int) int {
		return int(o.Word(oracle.Addr{Seg: work, Off: uint16(off)}))
	}
	occ := func(c, r int) oracle.Addr {
		return oracle.Addr{Seg: occSeg, Off: uint16(0x2532 + (r*12+c)*2)}
	}
	recOf := func(army, team int) int {
		return battleUnitBase + (army*battleUnitPer+team)*battleUnitSize
	}
	me, foe := -1, -1
	for army := 0; army < battleArmies; army++ {
		for team := 0; team < battleTeams; team++ {
			rec := recOf(army, team)
			if w16(rec) == 0xFFFF || w16(rec+unitLeaders) <= 0 {
				continue
			}
			if army >= 2 && me < 0 {
				me = rec
			}
			if army < 2 && foe < 0 {
				foe = rec
			}
		}
	}
	if me < 0 || foe < 0 {
		t.Log("擺不了位：找不到兩邊的部隊")
		return
	}
	fc, fr := w16(foe+unitCol), w16(foe+unitRow)
	for dir := 0; dir < 6; dir++ {
		i := uint16(((fc%2)*6 + dir) * 2)
		dc := int(int16(o.Word(oracle.Addr{Seg: colSeg, Off: 0x7c6a + i})))
		dr := int(int16(o.Word(oracle.Addr{Seg: rowSeg, Off: 0x7c82 + i})))
		c, r := fc+dc, fr+dr
		if c < 0 || c >= 12 || r < 0 || r >= 10 {
			continue
		}
		if o.Word(occ(c, r)) != 0xFFFF {
			continue
		}
		o.SetWord(occ(w16(me+unitCol), w16(me+unitRow)), 0xFFFF)
		o.SetWord(oracle.Addr{Seg: work, Off: uint16(me + unitCol)}, uint16(c))
		o.SetWord(oracle.Addr{Seg: work, Off: uint16(me + unitRow)}, uint16(r))
		o.SetWord(occ(c, r), 20)
		t.Logf("把玩家那支擺到 (%d,%d)，貼著守軍的 (%d,%d)", c, r, fc, fr)
		return
	}
	t.Log("守軍旁邊沒有空格")
}
