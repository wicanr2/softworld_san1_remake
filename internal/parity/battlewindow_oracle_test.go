//go:build oracle

package parity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"golang.org/x/text/encoding/traditionalchinese"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// TestZZBattleWindowsMatchTheOriginal 對拍主戰場第三塊面板的文字視窗（Issue #75）：
// 原版每一個問玩家的地方都是清視窗 → 訊息常式寫字 → 讀一個鍵。這支走南海打廬陵
// 那一場（`battlePanelRig`），存五格：第一支部隊紮寨、休息確認、每天的命令、
// 「設計那一軍」、移動方向。帶參數的兩句（紮寨 `0x22ae0`、命令 `0x27a39`）在原版
// sprintf 回來那一刻讀出組好的字串，與 remake 組的逐字比；五格畫面都比第三塊面板——
// 每 16 像素一列比有沒有墨（字模不接原版），其餘像素逐格相同。
func TestZZBattleWindowsMatchTheOriginal(t *testing.T) {
	i18n.Current = i18n.ZhHant
	var campText, orderText string
	var campShot []byte
	readText := func(o *oracle.Oracle) string {
		b := o.Bytes(addr(0x3e640+0x314e), 64)
		for i, c := range b {
			if c == 0 {
				b = b[:i]
				break
			}
		}
		out, err := traditionalchinese.Big5.NewDecoder().Bytes(b)
		if err != nil {
			return err.Error()
		}
		return string(out)
	}
	var campArmy, campTeam int
	s := newBattlePanelSceneWith(t, func(o *oracle.Oracle) {
		o.OnCall(addr(0x22af4), func(o *oracle.Oracle) { // 紮寨那一句 sprintf 回來
			if campText == "" {
				campText = readText(o)
				// 那時的 BP 框：[bp+6] 軍力、[bp−0xc] 隊伍（`0x22a55`／`0x22a52`）。
				bp := uint32(o.Regs().SS)*16 + uint32(o.Regs().BP)
				campArmy = int(o.Word(addr(bp + 6)))
				campTeam = int(int16(o.Word(addr(bp - 0xc))))
			}
		})
		o.OnCall(addr(0x21d52), func(o *oracle.Oracle) { // 紮寨那一格讀鍵之前
			if campShot == nil {
				campShot = append([]byte(nil), o.IndexedEGASize(scrW, scrH)...)
			}
		})
	})
	o := s.o
	o.OnCall(addr(0x27a3e), func(o *oracle.Oracle) { orderText = readText(o) })
	reads := 0
	o.OnCall(addr(uint32(captiveKeyFn.Seg)*16+uint32(captiveKeyFn.Off)), func(*oracle.Oracle) { reads++ })
	step := func(name, k string) []byte {
		before := reads
		o.Drain()
		o.PressScan(k)
		if err := o.RunUntil(oracle.NewCond(name, func(*oracle.Oracle) bool { return reads > before }), oracle.Budget(100_000_000)); err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		// 讀鍵常式剛進去；讓它跑到真的在等鍵再存畫面、送下一個鍵。
		if err := o.Run(3_000_000); err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		dumpScreen(t, o, "battlewin-"+name)
		return append([]byte(nil), o.IndexedEGASize(scrW, scrH)...)
	}
	restShot := append([]byte(nil), o.IndexedEGASize(scrW, scrH)...)
	commandShot := step("command", "N")
	snap := o.Save()
	plotShot := step("plotdir", "6")
	o.Restore(snap)
	moveShot := step("move", "1")

	// 原版那一支部隊（命令提示的參數：軍力、隊伍在 `0x27a40` 的 [bp+6]／[bp+8]）
	// 用主攻軍帥隊——rig 裡玩家只有那一支，第 0 槽是陳就。
	work := o.Word(oracle.Addr{Seg: s.dgroup, Off: battleWorkSeg})
	w16 := func(army, team, off int) int {
		rec := battleUnitBase + (army*battleUnitPer+team)*battleUnitSize
		return int(int16(o.Word(oracle.Addr{Seg: work, Off: uint16(rec + off)})))
	}
	head := w16(2, 0, 0)
	leader, _ := s.general(t, head)
	lord, err := s.sc.Lord(int(leader.Faction))
	if err != nil {
		t.Fatal(err)
	}
	move := w16(2, 0, 36)
	unit := &battle.Unit{Side: battle.MainAttacker, Formation: battle.Centre, Move: move,
		Leaders: []battle.Leader{{Index: head, Name: leader.Name}}}

	mineOrder := ui.BattleCommandWindow(lord.Name, battle.Centre, move, leader.Name)
	if want := i18n.S("bat.win.menu") + orderText; mineOrder != want {
		t.Errorf("命令提示：\n原版   %q\nremake %q", want, mineOrder)
	}
	if campText == "" || campShot == nil {
		t.Fatal("紮寨那一格沒攔到")
	}
	campLeaderIdx := w16(campArmy, campTeam, 0)
	campLeader, _ := s.general(t, campLeaderIdx)
	campPrefID := int(o.Word(oracle.Addr{Seg: work, Off: uint16(0x1770 + 0x16*campArmy)}))
	campPref, err := s.sc.Prefecture(campPrefID)
	if err != nil {
		t.Fatal(err)
	}
	sides := []battle.Side{battle.MainDefender, battle.AidDefender, battle.MainAttacker, battle.AidAttacker}
	forms := []battle.Formation{battle.Centre, battle.Vanguard, battle.Left, battle.Right, battle.Rear}
	mineCamp := ui.BattleCampWindow(campPrefID, campPref.Name, sides[campArmy], forms[campTeam], campLeader.Name)
	if want := campText + i18n.S("bat.win.camp")[len("(%2d%s)%s之%s請%s將軍紮寨"):]; mineCamp != want {
		t.Errorf("紮寨提示：\n原版   %q\nremake %q", want, mineCamp)
	}
	t.Logf("原版的兩句：紮寨 %q（軍力 %d 隊伍 %d）；命令 %q", campText, campArmy, campTeam, orderText)

	for _, c := range []struct {
		name   string
		shot   []byte
		window string
	}{
		{"紮寨", campShot, mineCamp},
		{"休息確認", restShot, i18n.S("bat.win.restYN")},
		{"命令", commandShot, mineOrder},
		{"設計那一軍", plotShot, i18n.S("bat.win.plotDir")},
		{"移動方向", moveShot, ui.BattleDirWindow(battle.CmdMove, unit)},
	} {
		cv := ui.NewCanvasPx(scrW, scrH, s.face)
		for y := 0; y < scrH; y++ {
			for x := 0; x < scrW; x++ {
				cv.Img.SetRGBA(x, y, assets.EGAPalette[c.shot[y*scrW+x]&15])
			}
		}
		b := battle.New(battle.Setup{Field: s.fld, Seed: 1})
		ui.DrawArtBattle(cv, s.ab, b, ui.BattleView{Window: c.window},
			ui.ArtBattleInfo{Field: s.pref.BattleField, Portrait: [2]int{-1, -1}})
		if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
			savePNG(t, filepath.Join(dir, "remake-battlewin-"+c.name+".png"), cv)
		}
		compareBattleWindow(t, c.name, c.shot, cv)
	}
}

// compareBattleWindow 比第三塊面板 (448,268)–(623,363) 的 22×6 個 8×16 字格有沒有
// 墨（青 3 以外都算，字模不接原版）。原版在提示後面畫一個閃爍的游標：remake
// 那一列最後一個有墨的格子之後、原版多出來的兩格以內不算錯。
func compareBattleWindow(t *testing.T, name string, orig []byte, cv *ui.Canvas) {
	t.Helper()
	x0, y0, _, _ := assets.BattleWide.Panel(2)
	cellInk := func(pix func(x, y int) int, col, row int) bool {
		for y := y0 + row*16; y < y0+row*16+16; y++ {
			for x := x0 + col*8; x < x0+col*8+8; x++ {
				if pix(x, y) != 3 {
					return true
				}
			}
		}
		return false
	}
	org := func(x, y int) int { return int(orig[y*scrW+x] & 15) }
	mine := func(x, y int) int { return paletteIndex(cv.Img.RGBAAt(x, y)) }
	bad, inked := 0, 0
	for row := 0; row < ui.BattleWindowRows; row++ {
		last := -1
		for col := 0; col < ui.BattleWindowCols; col++ {
			if cellInk(mine, col, row) {
				last = col
			}
		}
		for col := 0; col < ui.BattleWindowCols; col++ {
			o, m := cellInk(org, col, row), cellInk(mine, col, row)
			if o {
				inked++
			}
			if o && !m && col > last && col <= last+2 {
				continue // 游標
			}
			if o != m {
				bad++
				if bad <= 6 {
					t.Logf("%s：第 %d 列第 %d 格 原版有墨 %v、remake %v", name, row+1, col+1, o, m)
				}
			}
		}
	}
	if bad != 0 {
		t.Errorf("%s：第三塊面板 132 格有墨不同 %d", name, bad)
		return
	}
	t.Logf("%s：第三塊面板 132 格（原版 %d 格有墨）墨相同", name, inked)
}
