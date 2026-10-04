package main

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

type windowBar struct {
	state      ui.WindowBarState
	canvas     *ui.Canvas
	image      *ebiten.Image
	focus      int
	mode       ai.Mode
	orders     int
	aiExplicit bool
}

func newWindowBar(face, small *font.Face) *windowBar {
	c := ui.NewCanvasPx(640, ui.WindowBarCanvasHeight, face)
	c.SetSmallFace(small)
	return &windowBar{state: ui.NewWindowBarState(), canvas: c, image: ebiten.NewImage(640, ui.WindowBarCanvasHeight)}
}

func (a *app) barVisible() bool { return a.windowBar != nil && a.windowBar.state.Visible }
func (a *app) displayScale() int {
	if a.hdTheme {
		return 4
	}
	return 1
}

func (a *app) resizeBar(old bool) {
	if old == a.barVisible() {
		return
	}
	w, h := ebiten.WindowSize()
	d := ui.WindowBarHeightDelta(w, h, a.canvas.Img.Bounds().Dy(), old)
	if a.barVisible() {
		h += d
	} else {
		h -= d
	}
	ebiten.SetWindowSize(w, max(h, 100))
}

func (a *app) barView() ui.WindowBarView {
	v := ui.WindowBarView{Menu: a.windowBar.state.Menu, Hover: a.windowBar.state.Hover, Focus: a.windowBar.focus}
	for _, l := range i18n.Locales() {
		v.Lists[0] = append(v.Lists[0], l.Name())
		if l == i18n.Current {
			v.Selected[0] = len(v.Lists[0]) - 1
		}
	}
	v.Lists[1] = []string{t("window.original"), t("window.hd")}
	v.Disabled[1] = []bool{false, a.canvas.HD == nil || a.canvas.HD.Count == 0}
	if a.hdTheme {
		v.Selected[1] = 1
	}
	v.Values[0], v.Values[1] = v.Lists[0][v.Selected[0]], v.Lists[1][v.Selected[1]]
	if v.Disabled[1][1] {
		v.Lists[1][1] = t("window.hdMissing")
	}
	faithful := ai.ModeBase
	if a.edition == "plus" {
		faithful = ai.ModePlus
	}
	v.Lists[2] = []string{ai.ModeName(faithful)}
	for n := 1; n <= game.AIOrdersMax; n++ {
		v.Lists[2] = append(v.Lists[2], tf("window.enhanced", n))
	}
	mode, orders := a.windowBar.mode, a.windowBar.orders
	if a.s != nil {
		mode, orders = a.s.Brain.Mode(), a.s.G.Options.AIOrders()
	}
	if mode == ai.ModeEnhanced {
		v.Selected[2] = max(1, orders)
	}
	v.Values[2] = v.Lists[2][v.Selected[2]]
	if a.fight != nil {
		v.Disabled[2] = make([]bool, len(v.Lists[2]))
		for i := range v.Disabled[2] {
			v.Disabled[2][i] = true
		}
		v.Values[2] = t("window.battle")
	}
	return v
}

// 在原版的任何輸入、轉場或任意鍵判斷前攔下外部選項列。
func (a *app) updateWindowBar() bool {
	if a.windowBar == nil {
		return false
	}
	b := a.windowBar
	x, y := ebiten.CursorPosition()
	p := image.Pt(x/a.displayScale(), y/a.displayScale())
	old := b.state.Visible
	esc := inpututil.IsKeyJustPressed(ebiten.KeyEscape)
	shift := ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	if esc && shift {
		b.state.Visible, b.state.Pinned, b.state.Menu = false, false, -1
		a.resizeBar(old)
		return false
	}
	v := a.barView()
	rows := 0
	if b.state.Menu >= 0 {
		rows = len(v.Lists[b.state.Menu])
	}
	changed := b.state.Reveal(esc, p, rows)
	a.resizeBar(old)
	if esc || changed {
		return true
	}
	if !b.state.Visible {
		return false
	}
	v = a.barView()
	f, row := ui.WindowBarHit(p, v)
	if f == v.Menu && row >= 0 {
		b.state.Hover = row
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		switch {
		case f == 3:
			b.state.Visible, b.state.Pinned, b.state.Menu = false, false, -1
			a.resizeBar(true)
		case row >= 0:
			a.chooseWindowOption(f, row)
			b.state.Menu = -1
		case f >= 0:
			b.focus = f
			b.state.Hover = v.Selected[f]
			if b.state.Menu == f {
				b.state.Menu = -1
			} else {
				b.state.Menu = f
			}
		default:
			b.state.Menu = -1
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		b.focus = (b.focus + 1) % 3
		b.state.Menu = -1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		if b.state.Menu < 0 {
			b.state.Menu = b.focus
			b.state.Hover = v.Selected[b.focus]
		} else {
			d := 1
			if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
				d = -1
			}
			n := len(v.Lists[b.state.Menu])
			b.state.Hover = (b.state.Hover + d + n) % n
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if b.state.Menu < 0 {
			b.state.Menu = b.focus
			b.state.Hover = v.Selected[b.focus]
		} else {
			a.chooseWindowOption(b.state.Menu, b.state.Hover)
			b.state.Menu = -1
		}
	}
	return true
}

func (a *app) chooseWindowOption(field, row int) {
	v := a.barView()
	if field < 0 || field >= 3 || row < 0 || row >= len(v.Lists[field]) {
		return
	}
	if row < len(v.Disabled[field]) && v.Disabled[field][row] {
		return
	}
	switch field {
	case 0:
		old := i18n.Current
		i18n.Current = i18n.Locales()[row]
		a.relocalizeWindow(old)
	case 1:
		a.hdTheme = row == 1
	case 2:
		mode := ai.ModeBase
		if a.edition == "plus" {
			mode = ai.ModePlus
		}
		if row > 0 {
			mode = ai.ModeEnhanced
		}
		brain, err := ai.New(mode)
		if err != nil {
			return
		}
		a.windowBar.mode = mode
		a.windowBar.aiExplicit = true
		if row > 0 {
			a.windowBar.orders = row
		}
		if a.s != nil {
			if a.s.Brain.Mode() != mode {
				a.s.SetBrain(brain)
			}
			a.s.G.Options.SetAIMode(string(mode))
			if row > 0 {
				_ = a.s.G.Options.SetAIOrders(row)
			}
		}
		if a.menuScreen != nil {
			_ = a.menuScreen.SetAI(mode, row)
		}
	}
	a.dirty = true
}

func (a *app) uploadGame() {
	im := a.canvas.Output(a.hdTheme)
	if a.screen == nil || a.screen.Bounds() != im.Bounds() {
		if a.screen != nil {
			a.screen.Dispose()
		}
		a.screen = ebiten.NewImage(im.Bounds().Dx(), im.Bounds().Dy())
	}
	a.screen.WritePixels(im.Pix)
}

func (a *app) drawWindow(dst *ebiten.Image) {
	op := &ebiten.DrawImageOptions{}
	if a.barVisible() {
		op.GeoM.Translate(0, float64(ui.WindowBarHeight*a.displayScale()))
	}
	dst.DrawImage(a.screen, op)
	if a.barVisible() {
		b := a.windowBar
		ui.DrawWindowBar(b.canvas, a.barView())
		b.image.WritePixels(b.canvas.Img.Pix)
		bo := &ebiten.DrawImageOptions{}
		bo.GeoM.Scale(float64(a.displayScale()), float64(a.displayScale()))
		dst.DrawImage(b.image, bo)
	}
}

func (a *app) relocalizeWindow(old i18n.Locale) {
	f := func(s string) string { return i18n.Relocalize(s, old, i18n.Current) }
	if n := a.numPrompt; a.num == nil && n != nil && a.view.Prompt == n.shown {
		n.title = f(n.title)
		n.shown = n.text()
		a.view.Prompt = n.shown
	} else {
		a.view.Prompt = f(a.view.Prompt)
		if a.num == nil {
			a.numPrompt = nil
		}
	}
	a.view.Menu = f(a.view.Menu)
	a.view.PageTitle = f(a.view.PageTitle)
	for i := range a.view.Items {
		a.view.Items[i].Name = f(a.view.Items[i].Name)
	}
	for i := range a.view.Page {
		a.view.Page[i] = f(a.view.Page[i])
	}
	for i := range a.pick {
		a.pick[i].label = f(a.pick[i].label)
	}
	if a.num != nil {
		a.num.title = f(a.num.title)
		a.num.hint = f(a.num.hint)
		a.showNumber()
	}
	if a.menuScreen != nil {
		a.menuScreen.Relocalize(old)
	}
	if battle := a.fight; battle != nil {
		command := ""
		if battle.acting != nil {
			command = a.commandWindow(battle.acting)
		}
		battle.view.Relocalize(old, command)
		battle.saved.Relocalize(old, command)
		for i := range battle.speeches {
			battle.speeches[i].Text = f(battle.speeches[i].Text)
		}
	}
	ebiten.SetWindowTitle(fmt.Sprintf("三國演義 remake (%s)", i18n.Current.Name()))
}
