//go:build oracle

package parity

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// 對戰子畫面的畫面（Issue #56，`docs/spec/005` §8）：進主戰場之後直接呼叫
// `0x2deb0(主攻軍, 帥隊, 主守軍, 帥隊)`，停在玩家那一方第一位將領讀選單鍵
// 的那一刻存畫面。
var skirmishFn = oracle.Addr{Seg: 0x2deb, Off: 0x0}

func TestZZSkirmishScreenMatchesTheOriginal(t *testing.T) {
	s := newBattlePanelScene(t)
	o := s.o
	// remake 那一邊的兩支部隊與種子要在呼叫之前取。
	work := o.Word(oracle.Addr{Seg: s.dgroup, Off: battleWorkSeg})
	w16 := func(off int) int { return int(o.Word(oracle.Addr{Seg: work, Off: uint16(off)})) }
	unitFrom := func(army int, side battle.Side) *battle.Unit {
		rec := battleUnitBase + army*battleUnitPer*battleUnitSize
		u := &battle.Unit{Side: side, Formation: battle.DeployOrder()[0],
			At: battle.FromOffset(w16(rec+unitCol), w16(rec+unitRow))}
		for pos := 0; pos < w16(rec+unitLeaders); pos++ {
			x, l := s.general(t, w16(rec+pos*2))
			l.Troop = battle.TroopKind(x.Troop)
			l.Soldiers = int(o.Word(addr(skirmishGenBase(s) + uint32(l.Index*30+22))))
			u.Leaders = append(u.Leaders, l)
		}
		return u
	}
	att, def := unitFrom(2, battle.MainAttacker), unitFrom(0, battle.MainDefender)
	ds := o.DSReg()
	seed := uint32(o.Word(oracle.Addr{Seg: ds, Off: 0xa3ae})) | uint32(o.Word(oracle.Addr{Seg: ds, Off: 0xa3b0}))<<16
	difficulty := w16(0x30fe)

	o.StubValue(captiveWaitFn, 0x20)
	o.StubValue(captiveSayFn, 0)
	shot := []byte(nil)
	keys := 0
	o.Stub(captiveKeyFn, func(o *oracle.Oracle) uint32 {
		keys++
		if shot == nil {
			shot = append([]byte(nil), o.IndexedEGASize(scrW, scrH)...)
			dumpScreen(t, o, "skirmish-first-menu")
		}
		return '0'
	})
	_, err := o.CallBudget(300_000_000, skirmishFn, 2, 0, 0, 0)
	var be *oracle.BudgetError
	if err != nil && !errors.As(err, &be) {
		t.Fatalf("子畫面停止：%v", err)
	}
	if shot == nil {
		t.Fatal("沒有走到玩家的選單")
	}
	t.Logf("讀了 %d 次鍵；時刻 %d", keys, w16(0x31a6))
	for side := 0; side < 2; side++ {
		for slot := 0; slot < 10; slot++ {
			i := (side*10 + slot) * 2
			if idx := w16(0x1bc8 + i); idx != 0xFFFF {
				t.Logf("陣營 %d 槽 %d：人物 %d 在 (%d,%d)", side, slot, idx, w16(0x9a6+i), w16(0x15d8+i))
			}
		}
	}

	// remake：同一個種子從頭跑到玩家那一方第一次被問，當下畫一張。
	b := battle.New(battle.Setup{Field: s.fld, FixedWeather: true, Difficulty: difficulty,
		AI: battle.AIBase, Rules: battle.RulesFor("base", difficulty)})
	b.Units = []*battle.Unit{def, att}
	b.Computer[battle.MainDefender] = true
	rseed := seed
	b.UseRoll(func(n int) int {
		var out int
		rseed, out = game.MSCRand(rseed)
		return out % n
	})
	// 兩塊部隊面板：攻方陣營那一支、守方陣營那一支的第 0 槽（#55）。
	var panels [2]ui.UnitPanel
	for i, u := range []*battle.Unit{att, def} {
		x, _ := s.general(t, u.Leaders[0].Index)
		panels[i] = ui.UnitPanel{Unit: u, Portrait: int(x.Portrait)}
		if lord, err := s.sc.Lord(int(x.Faction)); err == nil {
			panels[i].Lord = lord.Name
		}
	}
	type stop struct{}
	var cv *ui.Canvas
	var acting *battle.SkirmishGeneral
	var running *battle.Skirmish
	b.PlayerSkirmish = func(sk *battle.Skirmish, g *battle.SkirmishGeneral) battle.SkirmishCommand {
		acting, running = g, sk
		cv = ui.NewCanvasPx(scrW, scrH, s.face)
		for y := 0; y < scrH; y++ {
			for x := 0; x < scrW; x++ {
				cv.Img.SetRGBA(x, y, assets.EGAPalette[shot[y*scrW+x]&15])
			}
		}
		info := ui.ArtBattleInfo{Field: s.pref.BattleField, Portrait: [2]int{-1, -1}, Skirmish: sk,
			Units: &panels}
		// 第三塊面板照 `cmd/san1` 問子畫面選單時擺的樣子（`0x2fb14`）。
		v := ui.BattleView{SkirmishActing: g, Menu: "對戰",
			Items:  []string{"1.行軍 2.單挑 3.攻擊", "7.查看 0.休息"},
			Prompt: fmt.Sprintf("%s(%d/%d)(0-4):", g.Leader.Name, g.Left, g.MoveCap)}
		ui.DrawArtBattle(cv, s.ab, b, v, info)
		panic(stop{})
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(stop); !ok {
					panic(r)
				}
			}
		}()
		b.NewSkirmish(att, def).Run()
	}()
	if cv == nil {
		t.Fatal("remake 沒有走到玩家的選單")
	}
	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		savePNG(t, filepath.Join(dir, "remake-skirmish-first-menu.png"), cv)
	}
	sk := acting
	t.Logf("remake 輪到 %s（陣營 %d 槽 %d）在 (%d,%d)", sk.Leader.Name, sk.Side, sk.Slot, sk.Col, sk.Row)

	// 場地逐像素，標記那幾格另外比（字模不接原版，`CLAUDE.md` §3.3）。
	var marks []image.Rectangle
	for _, gs := range running.Gens {
		for _, g := range gs {
			if g != nil && !g.Gone {
				x, y := assets.FieldCell(g.Col, g.Row)
				marks = append(marks, image.Rect(x, y, x+48, y+32))
			}
		}
	}
	inMark := func(x, y int) bool {
		for _, r := range marks {
			if image.Pt(x, y).In(r) {
				return true
			}
		}
		return false
	}
	bad, first := 0, ""
	for y := 36; y < 268; y++ { // 寬版面的面板從 y 268 起
		for x := 56; x < 56+48*12; x++ {
			if inMark(x, y) {
				continue
			}
			op := int(shot[y*scrW+x] & 15)
			mp := paletteIndex(cv.Img.RGBAAt(x, y))
			if op != mp {
				if bad == 0 {
					first = fmt.Sprintf("(%d,%d) 原版 %d remake %d", x, y, op, mp)
				}
				bad++
			}
		}
	}
	if bad != 0 {
		t.Errorf("標記以外的場地差 %d 個像素，第一個 %s", bad, first)
	} else {
		t.Logf("標記以外的場地逐像素相同（%d 個標記）", len(marks))
	}
	// 標記裡的每一個字格：底色（格內最多的那一色）相同、有沒有墨相同。
	// 字格是名字三格 16×16、「攻／守」一格 16×16、兵四格 8×16。
	cellCheck := func(name string, r image.Rectangle) {
		count := func(get func(x, y int) int) (bg, inked int) {
			hist := map[int]int{}
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					hist[get(x, y)]++
				}
			}
			best := -1
			for c, n := range hist {
				if best < 0 || n > hist[best] || (n == hist[best] && c < best) {
					best = c
				}
			}
			return best, r.Dx()*r.Dy() - hist[best]
		}
		ob, oi := count(func(x, y int) int { return int(shot[y*scrW+x] & 15) })
		mb, mi := count(func(x, y int) int { return paletteIndex(cv.Img.RGBAAt(x, y)) })
		if ob != mb || (oi > 0) != (mi > 0) {
			t.Errorf("%s %v：原版底 %d 墨 %d、remake 底 %d 墨 %d", name, r, ob, oi, mb, mi)
		}
	}
	for _, r := range marks {
		x, y := r.Min.X, r.Min.Y
		for k := 0; k < 3; k++ {
			cellCheck("名字", image.Rect(x+16*k, y, x+16*k+16, y+15))
		}
		cellCheck("攻守", image.Rect(x, y+16, x+16, y+31))
		for k := 0; k < 4; k++ {
			cellCheck("兵", image.Rect(x+16+8*k, y+16, x+24+8*k, y+31))
		}
		for yy := y; yy < y+32; yy++ {
			if op, mp := int(shot[yy*scrW+x+47]&15), paletteIndex(cv.Img.RGBAAt(x+47, yy)); op != mp {
				t.Errorf("標記 %v 右緣 (%d,%d)：原版 %d remake %d", r, x+47, yy, op, mp)
				break
			}
		}
	}
	// 兩塊部隊面板與第三塊的選單：字以外逐像素、每一行有沒有墨。
	l := assets.BattleWide
	for i := range panels {
		x0, y0, x1, y1 := l.Panel(i)
		nx, tx := l.NameX(i), l.TextX(i)
		nameBox := [4]int{nx, y0, nx + 31, y1}
		textBox := [4]int{tx, y0, tx + 63, y1}
		comparePanel(t, fmt.Sprintf("部隊面板 %d", i), shot, cv, x0, y0, x1, y1,
			[][4]int{nameBox, textBox},
			map[[4]int][]bool{textBox: {true, true, false, true, true, false}}, assets.BattlePanelPaper)
	}
	{
		x0, y0, x1, y1 := l.Panel(2)
		box := [4]int{x0, y0, x1, y1}
		comparePanel(t, "子畫面選單", shot, cv, x0, y0, x1, y1, [][4]int{box},
			map[[4]int][]bool{box: {true, true, true, false, false, false}}, assets.BattleOrderPaper)
	}
	// 左欄第四框：國字日數、日、時辰、時數，每一行有沒有墨兩邊相同。
	for row := 0; row < 6; row++ {
		y0 := 228 + row*16
		oi, mi := 0, 0
		for y := y0; y < y0+16; y++ {
			for x := assets.BattleLeftBoxX0; x <= assets.BattleLeftBoxX1; x++ {
				if int(shot[y*scrW+x]&15) != 3 {
					oi++
				}
				if paletteIndex(cv.Img.RGBAAt(x, y)) != 3 {
					mi++
				}
			}
		}
		if (oi > 0) != (mi > 0) {
			t.Errorf("左欄第四框第 %d 行（y %d）：原版有墨 %v、remake %v", row+1, y0, oi > 0, mi > 0)
		}
	}

	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		// 兩邊的標記放大四倍並排存起來看。
		for _, g := range []*battle.SkirmishGeneral{sk} {
			x0, y0 := assets.FieldCell(g.Col, g.Row)
			img := image.NewRGBA(image.Rect(0, 0, 48*4*2+8, 32*4))
			for y := 0; y < 32; y++ {
				for x := 0; x < 48; x++ {
					oc := assets.EGAPalette[shot[(y0+y)*scrW+x0+x]&15]
					mc := cv.Img.RGBAAt(x0+x, y0+y)
					for dy := 0; dy < 4; dy++ {
						for dx := 0; dx < 4; dx++ {
							img.Set(x*4+dx, y*4+dy, oc)
							img.Set(48*4+8+x*4+dx, y*4+dy, mc)
						}
					}
				}
			}
			f, err := os.Create(filepath.Join(dir, "marker-compare.png"))
			if err == nil {
				_ = png.Encode(f, img)
				f.Close()
			}
		}
	}
}

// skirmishGenBase 是原版人物表在記憶體裡的基底。
func skirmishGenBase(s *battlePanelScene) uint32 {
	return s.base + uint32(state.MasterTableSize+state.PrefectureTableSize)
}
