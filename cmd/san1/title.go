package main

// 主選單那一層的按鍵。狀態機在 `internal/menu`（無頭測得到），
// 畫面在 `internal/ui`；這一檔只把兩邊接起來。

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wicanr2/softworld_san1_remake/internal/menu"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// startTitle 停到主選單。
func (a *app) startTitle(ts *ui.TitleScreen, m *menu.Screen) {
	a.titleArt, a.menuScreen = ts, m
	a.dirty = true
}

// drawTitle 畫主選單那一層。
func (a *app) drawTitle() {
	if a.menuScreen.Stage() == menu.Menu {
		ui.DrawTitle(a.canvas, a.titleArt, a.menuScreen.Sel())
		return
	}
	ui.DrawTitleList(a.canvas, a.titleArt, a.menuScreen.Title(),
		a.menuScreen.Items(), a.menuScreen.Sel())
}

// updateTitle 收主選單的按鍵。
func (a *app) updateTitle() error {
	m := a.menuScreen
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		m.Back()
		a.dirty = true
		return nil
	case inpututil.IsKeyJustPressed(ebiten.KeyUp):
		m.Move(-1)
		a.dirty = true
		return nil
	case inpututil.IsKeyJustPressed(ebiten.KeyDown):
		m.Move(1)
		a.dirty = true
		return nil
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter),
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter):
		a.titleConfirm(m.Sel())
		return nil
	}
	for k := ebiten.Key1; k <= ebiten.Key9; k++ {
		if inpututil.IsKeyJustPressed(k) {
			if i := int(k - ebiten.Key1); i < m.Len() {
				a.titleConfirm(i)
			}
			return nil
		}
	}
	if m.Stage() == menu.Note && anyKeyPressed() {
		m.Back()
		a.dirty = true
	}
	return nil
}

// titleConfirm 把「選了第 i 項」交給狀態機，再處理它要的副作用。
func (a *app) titleConfirm(i int) {
	a.dirty = true
	if s := a.menuScreen.Confirm(i); s != nil {
		a.s = s
		a.view = ui.View{}
		a.menuScreen = nil
		return
	}
	if t := a.menuScreen.Track(); t >= 0 {
		a.jb.Play(t)
	}
	if a.menuScreen.Quit() {
		a.quit = true
	}
}

// anyKeyPressed 回報這一格有沒有按下任何鍵。
func anyKeyPressed() bool {
	return len(inpututil.AppendJustPressedKeys(nil)) > 0
}
