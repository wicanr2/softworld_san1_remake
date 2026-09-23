//go:build oracle

package parity

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// 主戰場上把面板換掉的兩支常式（Issue #55，`docs/spec/005` §8）：
//
//	0x320a6(軍力, 隊伍)  對戰子畫面的部隊面板：第 0 槽那一位的肖像與名字、
//	                     君主、軍力名、隊伍名與將數、兵數（段 0x2deb）
//	0x284a2(軍力, 人物)  查看：那一位的肖像 (552,276)、框 FBRB、名字 32×32 在
//	                     x 512、六行資料從 (448,268) 起（段 0x2020）
//
// 兩支都只有遠呼叫，段值只要對得上線性位址；照 `oracle-call-needs-real-segment`
// 的規矩仍寫真正的段。
var (
	unitPanelFn = oracle.Addr{Seg: 0x2deb, Off: 0x41f6}
	inspectFn   = oracle.Addr{Seg: 0x2020, Off: 0x82a2}
	// inspectWaitFn 是查看畫完之後等一個鍵的 runtime 常式。
	inspectWaitFn = oracle.Addr{Seg: 0x5c4, Off: 0x378c}
	// battleEnterAt 是進主戰場時攔一次記 DGROUP 的位址；battleCmdRead 是
	// 玩家每日命令讀到鍵之後那一條（紮寨走完的判準）。
	battleEnterAt = uint32(0x2053c)
	battleCmdRead = uint32(0x27a68)
)

// battlePanelRig 把原版帶到主戰場的命令提示（同 `TestZZBattleKeySweep`
// 的那條路：南海打廬陵，紮完寨、休息幾天），回傳三張表的基底與 DGROUP。
func battlePanelRig(t *testing.T, o *oracle.Oracle, seedMas []byte) (base uint32, dgroup uint16) {
	t.Helper()
	base = bootToGame(t, o, seedMas)
	at, to := stageABattle(t, o, base)
	o.OnCall(addr(battleEnterAt), func(o *oracle.Oracle) {
		if dgroup == 0 {
			dgroup = o.DSReg()
		}
	})
	cmdReads := 0
	o.OnCall(addr(battleCmdRead), func(*oracle.Oracle) { cmdReads++ })
	driveIntoBattleGap(t, o, at, to, 0)
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
	if cmdReads == 0 {
		t.Fatal("紮完寨沒有走到命令提示")
	}
	return base, dgroup
}

// battlePanelScene 是對拍用的一套東西：原版、DGROUP、活的三張表、remake 的
// 素材與畫布。
type battlePanelScene struct {
	o      *oracle.Oracle
	base   uint32
	dgroup uint16
	sc     *state.Scenario
	ab     *ui.ArtBattle
	pref   state.Prefecture
	fld    *battle.Field
	face   *font.Face
}

func newBattlePanelScene(t *testing.T) *battlePanelScene {
	t.Helper()
	return newBattlePanelSceneWith(t, nil)
}

// newBattlePanelSceneWith 同 newBattlePanelScene，pre 在開機之後、走進戰場之前
// 掛鉤子（紮寨那幾格在 rig 裡面就走過了）。
func newBattlePanelSceneWith(t *testing.T, pre func(*oracle.Oracle)) *battlePanelScene {
	t.Helper()
	root := origRoot(t)
	c2 := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c2, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(o.Close)
	if pre != nil {
		pre(o)
	}
	base, dgroup := battlePanelRig(t, o, seedMas)

	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	live := o.Bytes(addr(base), nMas+nSta+nGen)
	sc, err := state.DecodeTables(state.Slot("001"), live[:nMas], live[nMas:nMas+nSta], live[nMas+nSta:])
	if err != nil {
		t.Fatalf("原版記憶體裡的三張表解不開：%v", err)
	}
	pref, err := sc.Prefecture(25)
	if err != nil {
		t.Fatal(err)
	}
	fld, err := battle.Load(pref.BattleField, pref.Neighbours)
	if err != nil {
		t.Fatal(err)
	}
	ab, err := ui.NewArtBattle(openContainer(t, filepath.Join(root, "DATA1")),
		openContainer(t, filepath.Join(root, "DATA3")))
	if err != nil {
		t.Fatal(err)
	}
	fh, err := os.Open("../../fonts/unifont.hex.gz")
	if err != nil {
		t.Skipf("沒有字型檔：%v", err)
	}
	face, err := font.ParseHexGz(fh, 16)
	fh.Close()
	if err != nil {
		t.Fatal(err)
	}
	return &battlePanelScene{o: o, base: base, dgroup: dgroup, sc: sc, ab: ab, pref: pref, fld: fld, face: face}
}

// unitRecord 讀工作區裡一支部隊的記錄：第 0 槽的人物、將數、兵數。
func (s *battlePanelScene) unitRecord(army, team int) (head, leaders, soldiers int) {
	work := s.o.Word(oracle.Addr{Seg: s.dgroup, Off: battleWorkSeg})
	rec := battleUnitBase + (army*battleUnitPer+team)*battleUnitSize
	w16 := func(off int) int { return int(int16(s.o.Word(oracle.Addr{Seg: work, Off: uint16(rec + off)}))) }
	return w16(0), w16(unitLeaders), w16(unitSoldiers)
}

// general 把原版活的人物表那一筆換成 remake 的將領。
func (s *battlePanelScene) general(t *testing.T, index int) (state.General, battle.Leader) {
	t.Helper()
	gens := s.sc.Generals()
	if index < 0 || index >= len(gens) {
		t.Fatalf("人物 %d 超出人物表", index)
	}
	x := gens[index]
	return x, battle.Leader{Index: index, Name: x.Name, War: x.War, Intel: x.Intel,
		Stamina: x.Stamina, Charm: x.Charm, Soldiers: int(x.Soldiers),
		Training: x.Training, Arms: x.Arms, Troop: battle.TroopLand}
}

// screenCanvas 把原版現在的畫面抄成 remake 的畫布。
func (s *battlePanelScene) screenCanvas() (*ui.Canvas, []byte) {
	before := s.o.IndexedEGASize(scrW, scrH)
	cv := ui.NewCanvasPx(scrW, scrH, s.face)
	for y := 0; y < scrH; y++ {
		for x := 0; x < scrW; x++ {
			cv.Img.SetRGBA(x, y, assets.EGAPalette[before[y*scrW+x]&15])
		}
	}
	return cv, before
}

func paletteIndex(c color.RGBA) int {
	for i, p := range assets.EGAPalette {
		if p == c {
			return i
		}
	}
	return -1
}

// comparePanel 拿一塊面板逐像素比：text 裡的矩形是字的落點（字模是 remake
// 自己的，只比「那一行兩邊都有墨／都沒有墨」），其餘要逐點相同。
// lines 是 text 區裡每 16 像素一行該不該有墨。
func comparePanel(t *testing.T, name string, orig []byte, cv *ui.Canvas, x0, y0, x1, y1 int,
	text [][4]int, lines map[[4]int][]bool, paper int) {
	t.Helper()
	inText := func(x, y int) bool {
		for _, r := range text {
			if x >= r[0] && x <= r[2] && y >= r[1] && y <= r[3] {
				return true
			}
		}
		return false
	}
	bad, first := 0, ""
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if inText(x, y) {
				continue
			}
			op, mp := int(orig[y*scrW+x]&15), paletteIndex(cv.Img.RGBAAt(x, y))
			if op != mp {
				if bad == 0 {
					first = fmt.Sprintf("(%d,%d) 原版 %d remake %d", x, y, op, mp)
				}
				bad++
			}
		}
	}
	if bad != 0 {
		t.Errorf("%s：字以外有 %d 個像素不同，第一個 %s", name, bad, first)
	} else {
		t.Logf("%s：字以外逐像素相同", name)
	}
	for r, want := range lines {
		for k, w := range want {
			ly0 := r[1] + k*16
			compareTextLineStart(t, fmt.Sprintf("%s 第 %d 行", name, k+1), orig, cv,
				r[0], ly0, r[2], min(ly0+15, r[3]), paper)
			oi, mi := 0, 0
			for y := ly0; y < ly0+16 && y <= r[3]; y++ {
				for x := r[0]; x <= r[2]; x++ {
					if int(orig[y*scrW+x]&15) != paper {
						oi++
					}
					if paletteIndex(cv.Img.RGBAAt(x, y)) != paper {
						mi++
					}
				}
			}
			if (oi > 0) != w {
				t.Errorf("%s 第 %d 行 (%d,%d)：原版有墨 %v，該是 %v", name, k+1, r[0], ly0, oi > 0, w)
			}
			if (mi > 0) != w {
				t.Errorf("%s 第 %d 行 (%d,%d)：remake 有墨 %v，該是 %v", name, k+1, r[0], ly0, mi > 0, w)
			}
		}
	}
}

// TestZZUnitPanelsMatchTheOriginal 直接呼叫 `0x320a6` 把兩塊軍力面板換成
// 主攻軍帥隊與主守軍帥隊的部隊面板，再拿 remake 對同兩支部隊畫的比：
// 肖像、框、藍底逐像素相同，六行字的有無逐行相同。
func TestZZUnitPanelsMatchTheOriginal(t *testing.T) {
	s := newBattlePanelScene(t)
	// 側 2 ＝ 主攻軍、側 0 ＝ 主守軍，各拿帥隊（隊伍 0）。
	var panels [2]ui.UnitPanel
	for i, army := range []int{2, 0} {
		head, leaders, soldiers := s.unitRecord(army, 0)
		if head < 0 {
			t.Fatalf("軍力 %d 帥隊第 0 槽是空的", army)
		}
		x, l := s.general(t, head)
		side := battle.MainAttacker
		if army == 0 {
			side = battle.MainDefender
		}
		u := &battle.Unit{Side: side, Formation: battle.Centre, Leaders: []battle.Leader{l}}
		for k := 1; k < leaders; k++ {
			u.Leaders = append(u.Leaders, battle.Leader{Index: -1, Name: "?", Troop: battle.TroopLand})
		}
		// 兵數以整支部隊的記錄為準（第 0 槽那一位以外的兵都堆進去）。
		u.Leaders[0].Soldiers = soldiers
		for k := 1; k < len(u.Leaders); k++ {
			u.Leaders[k].Soldiers = 0
		}
		panels[i] = ui.UnitPanel{Unit: u, Portrait: int(x.Portrait)}
		if lord, err := s.sc.Lord(int(x.Faction)); err == nil {
			panels[i].Lord = lord.Name
		}
		t.Logf("軍力 %d 帥隊：第 0 槽 %s（肖像 %d、勢力 %d 君主 %q）、%d 將、兵 %d",
			army, x.Name, x.Portrait, x.Faction, panels[i].Lord, leaders, soldiers)
		if _, err := s.o.Call(unitPanelFn, uint16(army), 0); err != nil {
			t.Fatal(err)
		}
	}
	cv, orig := s.screenCanvas()
	dumpScreen(t, s.o, "unit-panels")

	b := battle.New(battle.Setup{Field: s.fld, Seed: 1})
	info := ui.ArtBattleInfo{Field: s.pref.BattleField, Portrait: [2]int{-1, -1}, Units: &panels}
	ui.DrawArtBattle(cv, s.ab, b, ui.BattleView{}, info)
	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		savePNG(t, filepath.Join(dir, "remake-unit-panels.png"), cv)
	}

	l := assets.BattleWide
	for i := range panels {
		x0, y0, x1, y1 := l.Panel(i)
		nx, tx := l.NameX(i), l.TextX(i)
		nameBox := [4]int{nx, y0, nx + 31, y1}
		textBox := [4]int{tx, y0, tx + 63, y1}
		comparePanel(t, fmt.Sprintf("部隊面板 %d", i), orig, cv, x0, y0, x1, y1,
			[][4]int{nameBox, textBox},
			map[[4]int][]bool{textBox: {true, true, false, true, true, false}}, assets.BattlePanelPaper)
	}
}

// TestZZInspectPanelMatchesTheOriginal 直接呼叫 `0x284a2` 把第三塊面板換成
// 主攻軍帥隊第 0 槽那一位的查看面板，與 remake 的比。
func TestZZInspectPanelMatchesTheOriginal(t *testing.T) {
	s := newBattlePanelScene(t)
	head, _, _ := s.unitRecord(2, 0)
	if head < 0 {
		t.Fatal("主攻軍帥隊第 0 槽是空的")
	}
	x, l := s.general(t, head)
	t.Logf("查看主攻軍帥隊第 0 槽 %s（肖像 %d）：體能 %d 謀略 %d 戰力 %d 訓練 %d 武裝 %d 兵 %d",
		x.Name, x.Portrait, x.Stamina, x.Intel, x.War, x.Training, x.Arms, x.Soldiers)
	// 常式畫完就等鍵（`0x5c4:0x378c`），釘成「按過了」讓它直接回來。
	s.o.StubValue(inspectWaitFn, 0x20)
	defer s.o.Stub(inspectWaitFn, nil)
	if _, err := s.o.Call(inspectFn, 2, uint16(head)); err != nil {
		t.Fatal(err)
	}
	cv, orig := s.screenCanvas()
	dumpScreen(t, s.o, "inspect-panel")

	b := battle.New(battle.Setup{Field: s.fld, Seed: 1})
	info := ui.ArtBattleInfo{Field: s.pref.BattleField, Portrait: [2]int{-1, -1},
		Inspect: &ui.InspectPanel{Leader: &l, Side: battle.MainAttacker, Portrait: int(x.Portrait)}}
	ui.DrawArtBattle(cv, s.ab, b, ui.BattleView{}, info)
	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		savePNG(t, filepath.Join(dir, "remake-inspect-panel.png"), cv)
	}

	x0, y0, x1, y1 := assets.BattleWide.Panel(2)
	textBox := [4]int{x0, y0, x0 + 63, y1}
	nameBox := [4]int{x0 + 64, y0, x0 + 95, y1}
	comparePanel(t, "查看面板", orig, cv, x0, y0, x1, y1,
		[][4]int{textBox, nameBox},
		map[[4]int][]bool{textBox: {true, true, true, true, true, true}}, assets.BattlePanelPaper)
}
