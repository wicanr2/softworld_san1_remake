package main

import (
	"fmt"
	"image"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/menu"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

type pointerInput struct {
	used       bool
	keys       []ebiten.Key
	owner      any
	key        ebiten.Key
	pressed    bool
	titleOwner any
	digits     string
}

// 滑鼠只轉成原有輸入。每幀送一鍵；狀態換掉就丟棄剩餘鍵，避免穿透下一個問題。
func (a *app) inputOwner() any {
	switch {
	case a.opening != nil:
		return a.opening
	case a.menuScreen != nil:
		return struct {
			m *menu.Screen
			s menu.Stage
		}{a.menuScreen, a.menuScreen.Stage()}
	case a.confirm != nil:
		return a.confirm
	case a.roster != nil:
		return a.roster
	case a.saving != nil:
		return struct {
			s    *ui.SaveScreen
			slot int
		}{a.saving, a.saving.Slot}
	case a.num != nil:
		return a.num
	case a.fight != nil:
		return struct {
			f *fight
			w waitFor
			q *engineAsk
		}{a.fight, a.fight.waiting, a.fight.asking}
	}
	return a
}

func (a *app) keyPressed(k ebiten.Key) bool {
	return (a.pointer.pressed && a.pointer.key == k) || inpututil.IsKeyJustPressed(k)
}

func (a *app) anyKeyPressed() bool {
	return a.pointer.pressed || len(inpututil.AppendJustPressedKeys(nil)) > 0
}

func pointerKey(k int) (ebiten.Key, bool) {
	switch {
	case k >= '0' && k <= '9':
		return ebiten.Key0 + ebiten.Key(k-'0'), true
	case k >= 'A' && k <= 'Z':
		return ebiten.KeyA + ebiten.Key(k-'A'), true
	}
	switch k {
	case 13:
		return ebiten.KeyEnter, true
	case 27:
		return ebiten.KeyEscape, true
	case 8:
		return ebiten.KeyBackspace, true
	case 32:
		return ebiten.KeySpace, true
	case ui.PointerPrevious:
		return ebiten.KeyPageUp, true
	case ui.PointerNext:
		return ebiten.KeyPageDown, true
	case ui.PointerLess:
		return ebiten.KeyLeft, true
	case ui.PointerMore:
		return ebiten.KeyRight, true
	}
	return 0, false
}

func (a *app) queuePointer(keys ...int) {
	a.pointer.keys = nil
	a.pointer.owner = a.inputOwner()
	for _, k := range keys {
		if key, ok := pointerKey(k); ok {
			a.pointer.keys = append(a.pointer.keys, key)
		}
	}
}

func (a *app) titleNumber() bool {
	return a.menuScreen != nil && (a.menuScreen.Stage() == menu.PlayerCount || a.menuScreen.Stage() == menu.Difficulty)
}

func (a *app) pointerNumber(n int) {
	if a.num == nil {
		return
	}
	a.num.value, a.num.digits, a.num.typed = n, strconv.Itoa(n), true
	a.showNumber()
	a.queuePointer(13)
}

// gamePointer 換算 Ebiten 的邏輯畫布座標。高清倍率與選項列各扣一次。
func (a *app) gamePointer(x, y int) image.Point {
	scale := a.displayScale()
	x, y = x/scale, y/scale
	if a.barVisible() {
		y -= ui.WindowBarHeight
	}
	return image.Pt(x, y)
}

func (a *app) updatePointerInput() {
	a.pointer.pressed = false
	owner := a.inputOwner()
	if a.pointer.owner != owner || len(inpututil.AppendJustPressedKeys(nil)) > 0 {
		a.pointer.keys = nil
	}
	if a.pointer.titleOwner != owner {
		a.pointer.titleOwner = owner
		a.pointer.digits = ""
	}
	x, y := ebiten.CursorPosition()
	p := a.gamePointer(x, y)
	left := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	right := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	_, wheel := ebiten.Wheel()
	if (left || right || wheel != 0) && p.In(a.canvas.Img.Bounds()) {
		if !a.pointer.used {
			a.pointer.used = true
			a.dirty = true
		}
		if a.wipe != nil {
			return
		}
		switch {
		case right:
			if a.titleNumber() {
				a.menuScreen.Back()
				a.dirty = true
			} else {
				a.pointerBack()
			}
		case wheel != 0:
			a.pointerWheel(wheel)
		case a.pointerModal():
			a.queuePointer(13)
		case left:
			if h, ok := a.canvas.PointerAt(p); ok {
				a.pointerHit(h)
			} else {
				a.pointerMap(p)
			}
		}
	}
	if len(a.pointer.keys) > 0 {
		a.pointer.key = a.pointer.keys[0]
		a.pointer.keys = a.pointer.keys[1:]
		a.pointer.pressed = true
	}
}

func (a *app) pointerModal() bool {
	if a.opening != nil || a.credits != nil {
		return true
	}
	if a.menuScreen != nil {
		s := a.menuScreen.Stage()
		return s == menu.Note || s == menu.Demo || s == menu.LordBorn
	}
	if a.fight != nil {
		return a.fight.speech(a.artBattle != nil) != nil
	}
	return a.s != nil && (a.s.Bubble() != nil || a.view.HasCard || a.view.Atlas != 0 || a.treasuryWait)
}

func (a *app) pointerWheel(w float64) {
	d := 1
	if w > 0 {
		d = -1
	}
	switch {
	case a.menuScreen != nil:
		if !a.lordPickTurn(d) {
			a.menuScreen.Move(d)
		}
		a.dirty = true
	case a.roster != nil:
		r := a.roster
		pages := (len(r.pick.List) + ui.RosterPageRows - 1) / ui.RosterPageRows
		if pages > 0 {
			r.pick.Page = ((r.pick.Page/ui.RosterPageRows + d + pages) % pages) * ui.RosterPageRows
			r.typed = ""
			a.showRoster()
			a.dirty = true
		}
	case len(a.view.Page) > 0:
		_, rows := ui.PageSize(a.art != nil)
		a.view.ScrollPage(d*rows, a.art != nil)
		a.dirty = true
	case a.fight != nil && len(a.fight.view.Page) > 0:
		_, rows := ui.BattlePageSize(a.artBattle != nil)
		a.fight.view.ScrollPage(d*rows, a.artBattle != nil)
		a.dirty = true
	}
}

func (a *app) pointerHit(h ui.PointerTarget) {
	if h.Kind == ui.PointerKey && h.Value == ui.PointerCommands {
		a.view.Status = false
		a.dirty = true
		return
	}
	if a.confirm != nil || (a.fortSpot != nil && a.fortSpot.confirm) {
		if h.Kind == ui.PointerKey && (h.Value == 'Y' || h.Value == 'N' || h.Value == 27) {
			a.queuePointer(h.Value)
		}
		return
	}
	if a.titleNumber() {
		if h.Kind != ui.PointerKey {
			return
		}
		switch k := h.Value; {
		case k >= '0' && k <= '9':
			if len(a.pointer.digits) < 3 {
				a.pointer.digits += string(rune(k))
			}
		case k == 8:
			if len(a.pointer.digits) > 0 {
				a.pointer.digits = a.pointer.digits[:len(a.pointer.digits)-1]
			}
		case k == 13:
			n := a.menuScreen.Sel()
			if a.pointer.digits != "" {
				n, _ = strconv.Atoi(a.pointer.digits)
				if a.menuScreen.Stage() == menu.Difficulty {
					n--
				}
			}
			if n >= 0 && n < a.menuScreen.Len() {
				a.titleConfirm(n)
			}
		case k == 27:
			a.menuScreen.Back()
		case k == ui.PointerLess:
			a.menuScreen.Move(-1)
			a.pointer.digits = ""
		case k == ui.PointerMore:
			a.menuScreen.Move(1)
			a.pointer.digits = ""
		}
		a.dirty = true
		return
	}
	if a.menuScreen != nil {
		if h.Kind == ui.PointerTitle {
			i := h.Value
			if a.menuScreen.Stage() == menu.Lord && a.art != nil {
				i += a.menuScreen.Sel() / ui.LordPickPerPage * ui.LordPickPerPage
			}
			if i >= 0 && i < a.menuScreen.Len() {
				a.titleConfirm(i)
			}
		} else if h.Kind == ui.PointerKey {
			if h.Value == ui.PointerPrevious || h.Value == ui.PointerNext {
				d := 1
				if h.Value == ui.PointerPrevious {
					d = -1
				}
				if !a.lordPickTurn(d) {
					a.menuScreen.Move(d)
				}
				a.dirty = true
			} else {
				a.queuePointer(h.Value)
			}
		}
		return
	}
	if a.roster != nil {
		if h.Kind == ui.PointerRoster {
			a.roster.typed = strconv.Itoa(h.Value)
			a.showRoster()
			a.queuePointer(13)
		} else if h.Kind == ui.PointerKey {
			if h.Value == ui.PointerPrevious || h.Value == ui.PointerNext {
				w := -1.0
				if h.Value == ui.PointerPrevious {
					w = 1
				}
				a.pointerWheel(w)
			} else {
				a.queuePointer(h.Value)
			}
		}
		return
	}
	if h.Kind == ui.PointerPref {
		if a.view.PrefPick != nil && a.view.PrefPick.Valid[h.Value] {
			a.pointerNumber(h.Value)
		}
		return
	}
	if h.Kind == ui.PointerCommand {
		if a.num != nil && a.num.main {
			a.pointerNumber(h.Value - '0')
		} else if a.num == nil && a.menu == 0 && len(a.pick) == 0 && len(a.view.Page) == 0 {
			a.queuePointer(h.Value)
		}
		return
	}
	if h.Kind == ui.PointerNumber {
		if a.num != nil && !a.num.main {
			a.pointerNumber(h.Value)
		} else if h.Value >= 0 && h.Value <= 9 && (a.menu != 0 || len(a.pick) > 0 || a.fight != nil) {
			a.queuePointer('0' + h.Value)
		}
		return
	}
	if h.Kind == ui.PointerKey {
		if h.Value == 27 {
			a.pointerBack()
		} else {
			a.queuePointer(h.Value)
		}
	}
}

func (a *app) pointerMap(p image.Point) {
	if a.fortSpot != nil {
		if a.fortSpot.confirm {
			return
		}
		col, row, ok := pointerField(p)
		if !ok {
			return
		}
		if a.s != nil && a.s.G.Field(a.fortSpot.at).Narrow() && col >= 8 {
			return
		}
		a.fortSpot.col, a.fortSpot.row = col, row
		a.queuePointer('0')
		a.dirty = true
		return
	}
	if f := a.fight; f != nil {
		if a.pointerBattleYN() {
			return
		}
		col, row, ok := pointerField(p)
		if !ok || len(f.view.Page) > 0 {
			return
		}
		if f.runner != nil && f.runner.B.Field.Narrow() && col >= 8 {
			return // 窄圖右側是人物及指令面板，不是可點擊的場地。
		}
		h := battle.FromOffset(col, row)
		if q := f.asking; q != nil && q.skirmish != nil {
			if f.skm == skmMarch || f.skm == skmDuel || f.skm == skmAttack {
				for _, d := range battle.Dirs() {
					if battle.FromOffset(q.skirmish.Col, q.skirmish.Row).Step(d) == h {
						a.queuePointer(int('0' + d))
						return
					}
				}
			}
			return
		}
		if f.waiting == waitCamp || (f.asking != nil && f.asking.camp != nil) {
			f.view.Cursor.At = h
			a.queuePointer('0')
			a.dirty = true
			return
		}
		if f.acting != nil && (f.waiting == waitDir || f.waiting == waitPlotDir) {
			for _, d := range battle.Dirs() {
				if f.acting.At.Step(d) == h {
					a.queuePointer(int('0' + d))
					return
				}
			}
		}
		f.view.Cursor.At = h
		a.dirty = true
		return
	}
	if a.s == nil || a.art == nil || a.roster != nil || a.confirm != nil || a.menu != 0 || len(a.pick) > 0 || len(a.view.Page) > 0 {
		return
	}
	if a.num != nil && a.view.PrefPick == nil && !a.num.main {
		return
	}
	for _, pref := range a.s.G.Prefectures() {
		x, y := int(pref.MapX)+assets.MapOriginX, int(pref.MapY)+assets.MapOriginY
		if !p.In(image.Rect(x-3, y-3, x+20, y+19)) {
			continue
		}
		if a.view.PrefPick != nil {
			if a.view.PrefPick.Valid[pref.ID] {
				a.pointerNumber(pref.ID)
			}
			return
		}
		for _, id := range a.s.PlayerTerritory() {
			if id == pref.ID {
				a.view.Sel = id
				a.view.Status = true
				a.dirty = true
				return
			}
		}
	}
}

func pointerField(p image.Point) (col, row int, ok bool) {
	col = (p.X - assets.FieldOriginX) / assets.TileW
	if p.X < assets.FieldOriginX || col < 0 || col >= assets.FieldCols {
		return 0, 0, false
	}
	y := p.Y - assets.FieldOriginY
	if col&1 != 0 {
		y -= assets.FieldStagger
	}
	row = y / assets.TileH
	return col, row, y >= 0 && row >= 0 && row < 10
}

func (a *app) drawPointerControls() {
	if !a.pointer.used || a.pointerModal() || a.wipe != nil {
		return
	}
	c := a.canvas
	button := func(x, y, w int, label string, k int) { ui.DrawPointerButton(c, image.Rect(x, y, x+w, y+28), label, k) }
	if a.confirm != nil || (a.fortSpot != nil && a.fortSpot.confirm) || a.pointerBattleYN() {
		button(80, 344, 96, t("mouse.yes"), 'Y')
		button(180, 344, 96, t("mouse.no"), 'N')
		return
	}
	if a.menuScreen != nil && a.menuScreen.Stage() == menu.CustomLord {
		button(80, 344, 76, "-", ui.PointerLess)
		button(160, 344, 76, "+", ui.PointerMore)
	}
	pad := a.titleNumber() || (a.num != nil && !a.num.main && a.view.PrefPick == nil)
	if pad {
		if a.titleNumber() {
			n := a.menuScreen.Sel()
			if a.menuScreen.Stage() == menu.Difficulty {
				n++
			}
			s := strconv.Itoa(n)
			if a.pointer.digits != "" {
				s = a.pointer.digits
			}
			button(80, 180, 252, fmt.Sprintf("%s: %s", t("mouse.number"), s), 13)
		}
		keys := []int{'7', '8', '9', 8, '4', '5', '6', 27, '1', '2', '3', 13, '0', ui.PointerLess, ui.PointerMore, 13}
		for i, k := range keys {
			label := string(rune(k))
			switch k {
			case 8:
				label = t("mouse.delete")
			case 27:
				label = t("mouse.back")
			case 13:
				label = t("mouse.ok")
			case ui.PointerLess:
				label = "-"
			case ui.PointerMore:
				label = "+"
			}
			if !a.titleNumber() && (k == ui.PointerLess || k == ui.PointerMore) {
				continue
			}
			button(80+i%4*64, 212+i/4*32, 60, label, k)
		}
		return
	}
	if a.roster != nil || (a.menuScreen != nil && a.menuScreen.Stage() == menu.Lord) || len(a.view.Page) > 0 || (a.fight != nil && len(a.fight.view.Page) > 0) {
		button(80, 376, 76, t("mouse.prev"), ui.PointerPrevious)
		button(160, 376, 76, t("mouse.next"), ui.PointerNext)
	} else if a.s != nil && a.fight == nil && a.menu == 0 && a.saving == nil && a.fortSpot == nil &&
		a.view.Status && a.view.PrefPick == nil && (a.num == nil || a.num.main) {
		button(80, 376, 76, t("mouse.commands"), ui.PointerCommands)
	}
	button(240, 376, 76, t("mouse.back"), 27)
	ok := 13
	if len(a.view.Page) > 0 || (a.fight != nil && len(a.fight.view.Page) > 0) {
		ok = 27
	}
	label := t("mouse.ok")
	if f := a.fight; f != nil && len(f.view.Page) == 0 && f.asking == nil {
		if f.waiting == waitCamp {
			ok, label = '0', t("bat.camp")
		} else if f.waiting == waitDir {
			ok, label = '0', t("mouse.finish")
		}
	}
	button(320, 376, 76, label, ok)
}

func (a *app) pointerBack() {
	if f := a.fight; f != nil && len(f.view.Page) == 0 {
		if a.pointerBattleYN() {
			a.queuePointer('N')
			return
		}
		if q := f.asking; q != nil {
			if q.retreat != nil {
				a.queuePointer('0')
				return
			}
			if q.skirmish != nil && (f.skm == skmDuel || f.skm == skmAttack) {
				a.queuePointer(13)
				return
			}
		} else if f.waiting == waitPlot || f.waiting == waitPlotDir {
			a.queuePointer(13)
			return
		}
	}
	a.queuePointer(27)
}

func (a *app) pointerBattleYN() bool {
	f := a.fight
	if f == nil {
		return false
	}
	if q := f.asking; q != nil {
		return q.challenged != nil || (q.skirmish != nil && f.skm == skmRest)
	}
	return f.waiting == waitRestYN || f.waiting == waitRetreatYN
}
