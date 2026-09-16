package main

// 主選單那一層的按鍵。狀態機在 `internal/menu`（無頭測得到），
// 畫面在 `internal/ui`；這一檔只把兩邊接起來。

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/menu"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// startTitle 停到主選單。
func (a *app) startTitle(ts *ui.TitleScreen, m *menu.Screen) {
	a.titleArt, a.menuScreen = ts, m
	a.titleAnimTick, a.titleAnimFrame = 0, 0
	a.dirty = true
}

// drawTitle 畫主選單那一層。
func (a *app) drawTitle() {
	m := a.menuScreen
	if m.Stage() == menu.Menu {
		ui.DrawTitleFrame(a.canvas, a.titleArt, m.Sel(), a.titleAnimFrame)
		return
	}
	// 選君主：有原版素材就照原版畫——主畫面的地圖加右側一頁六位君主的
	// 肖像（`docs/spec/005` §9.4）；沒有素材退回文字清單。
	if m.Stage() == menu.Lord && a.art != nil && m.Game() != nil {
		ui.DrawLordPick(a.canvas, a.art, m.Game(), a.lordPickPage(), m.Sel()%ui.LordPickPerPage,
			tf("title.lordPrompt", len(m.Lords())), a.view.Calendar)
		return
	}
	// 新君主：同一塊面板，肖像加框、名字、六行能力（`docs/spec/005` §9.5）。
	if cv := m.Custom(); m.Stage() == menu.CustomLord && a.art != nil && m.Game() != nil && cv != nil {
		ui.DrawCustomLord(a.canvas, a.art, m.Game(), cv.Faction, cv.Portrait, cv.Name, cv.Lines, m.Sel(),
			[2]string{tf("title.newLordPoints", cv.Spare), t("title.newLordHint")}, a.view.Calendar)
		return
	}
	// 「新君主出現!!」：面板還原成底圖，新君主的肖像在上格說一句（§9.5）。
	if cv := m.Custom(); m.Stage() == menu.LordBorn && a.art != nil && m.Game() != nil && cv != nil {
		ui.DrawNewLordBorn(a.canvas, a.art, m.Game(), cv.Portrait, cv.Name, cv.Color, a.view.Calendar)
		return
	}
	ui.DrawTitleList(a.canvas, a.titleArt, m.Title(), m.Items(), m.Sel())
}

// lordPickPage 是選君主那一格現在這一頁的候選（反白那一位所在的那一頁）。
func (a *app) lordPickPage() []ui.LordPickSlot {
	m := a.menuScreen
	g, lords := m.Game(), m.Lords()
	first := m.Sel() / ui.LordPickPerPage * ui.LordPickPerPage
	var out []ui.LordPickSlot
	for i := first; i < len(lords) && i < first+ui.LordPickPerPage; i++ {
		f := lords[i]
		slot := ui.LordPickSlot{Number: f + 1, Faction: f}
		if m.IsCustom(f) {
			slot.Label = t("title.newLord")
			if nth := m.CustomIndex(f); nth >= 0 && nth < len(state.CustomLordPortrait) {
				slot.Portrait = state.CustomLordPortrait[nth]
			}
		} else {
			slot.Lord = g.Lord(state.FactionID(f))
		}
		out = append(out, slot)
	}
	return out
}

// updateTitle 收主選單的按鍵。
func (a *app) updateTitle() error {
	m := a.menuScreen
	if m.Stage() == menu.Menu {
		a.titleAnimTick++
		if a.titleAnimTick%ui.TitleOrnamentTicksPerFrame == 0 {
			a.titleAnimFrame = (a.titleAnimFrame + 1) % assets.MenuOrnamentFrameCount
			a.dirty = true
		}
	}
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
	case inpututil.IsKeyJustPressed(ebiten.KeyLeft):
		// 左右鍵：新君主那一層加減點數、換領地；選君主那一格（原版素材）翻頁。
		if m.Adjust(-1) || a.lordPickTurn(-1) {
			a.dirty = true
		}
		return nil
	case inpututil.IsKeyJustPressed(ebiten.KeyRight):
		if m.Adjust(1) || a.lordPickTurn(1) {
			a.dirty = true
		}
		return nil
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter),
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter):
		a.titleConfirm(m.Sel())
		return nil
	}
	for k := ebiten.Key1; k <= ebiten.Key9; k++ {
		if inpututil.IsKeyJustPressed(k) {
			i := int(k - ebiten.Key1)
			// 選君主那一格（原版素材）的數字鍵是這一頁的第幾位。
			if m.Stage() == menu.Lord && a.art != nil {
				if i >= ui.LordPickPerPage {
					return nil
				}
				i += m.Sel() / ui.LordPickPerPage * ui.LordPickPerPage
			}
			if i < m.Len() {
				a.titleConfirm(i)
			}
			return nil
		}
	}
	if m.Stage() == menu.Note && anyKeyPressed() {
		m.Back()
		a.dirty = true
	}
	// 「新君主出現!!」按任意鍵往下（原版等一個鍵）。
	if m.Stage() == menu.LordBorn && anyKeyPressed() {
		a.titleConfirm(0)
		a.dirty = true
	}
	return nil
}

// lordPickTurn 把選君主那一格翻一頁（只在原版素材畫面）。
func (a *app) lordPickTurn(d int) bool {
	m := a.menuScreen
	if m.Stage() != menu.Lord || a.art == nil {
		return false
	}
	n, per := m.Len(), ui.LordPickPerPage
	if n <= per {
		return false
	}
	page := (m.Sel()/per + d + (n+per-1)/per) % ((n + per - 1) / per)
	target := page * per
	if target >= n {
		target = n - 1
	}
	m.Move(target - m.Sel())
	return true
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
