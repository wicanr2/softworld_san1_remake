package main

// 主選單。
//
// 六個項目照原版（`ui.TitleItems`，畫面在 `docs/spec/005` §6）：
//
//	1. 開始新遊戲  2. 載入舊進度  3. 使用楷書字
//	4. 使用隸書字  5. 音樂欣賞    6. 回作業系統
//
// **選劇本、選君主、選難度那幾層的版面是 remake 自己排的**：原版那一段
// 跑在開機鏈第二層（`DATA0.GRP`），主程式的碼段 dump 涵蓋不到。
// 選項的內容則是原版的資料——劇本的起始年月、勢力的君主姓名、
// 難度上限都從解出來的表來。
//
// 字型那兩項（楷書／隸書）remake 做不到：**不內嵌任何原版字模**
// （`CLAUDE.md` §3.3），所以選了只會說明一句。

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

type titleStage int

const (
	titleMenu titleStage = iota
	titleScenario
	titleLord
	titleDifficulty
	titleLoad
	titleMusic
	titleNote
)

// titleState 是停在主選單時的狀態。playing 時是 nil。
type titleState struct {
	ts    *ui.TitleScreen
	stage titleStage

	sel  int // 主選單反白的項目
	pick int // 子清單反白的項目

	title string
	items []string

	slot  state.Slot
	lords []int // 子清單每一項對應的勢力槽號
	saves []save.Info
	orig  bool // 這一份清單是原版的進度
}

// startTitle 停到主選單。
func (a *app) startTitle(ts *ui.TitleScreen) {
	a.title = &titleState{ts: ts}
	a.dirty = true
}

// drawTitle 畫主選單那一層。
func (a *app) drawTitle() {
	st := a.title
	if st.stage == titleMenu {
		ui.DrawTitle(a.canvas, st.ts, st.sel)
		return
	}
	ui.DrawTitleList(a.canvas, st.ts, st.title, st.items, st.pick)
}

// updateTitle 收主選單的按鍵。
func (a *app) updateTitle() error {
	st := a.title
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if st.stage == titleMenu {
			return nil
		}
		st.stage, st.items, st.pick = titleMenu, nil, 0
		a.dirty = true
		return nil
	}
	if st.stage == titleNote {
		if anyKeyPressed() {
			st.stage = titleMenu
			a.dirty = true
		}
		return nil
	}
	n := len(st.items)
	if st.stage == titleMenu {
		n = len(ui.TitleItems())
	}
	if n == 0 {
		st.stage = titleMenu
		a.dirty = true
		return nil
	}
	cur := &st.pick
	if st.stage == titleMenu {
		cur = &st.sel
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyUp):
		*cur = (*cur + n - 1) % n
		a.dirty = true
		return nil
	case inpututil.IsKeyJustPressed(ebiten.KeyDown):
		*cur = (*cur + 1) % n
		a.dirty = true
		return nil
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter),
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter):
		a.titleConfirm(*cur)
		return nil
	}
	for k := ebiten.Key1; k <= ebiten.Key9; k++ {
		if inpututil.IsKeyJustPressed(k) {
			i := int(k - ebiten.Key1)
			if i < n {
				*cur = i
				a.titleConfirm(i)
			}
			return nil
		}
	}
	return nil
}

// anyKeyPressed 回報這一格有沒有按下任何鍵。
func anyKeyPressed() bool {
	return len(inpututil.AppendJustPressedKeys(nil)) > 0
}

// titleConfirm 處理「選了第 i 項」。
func (a *app) titleConfirm(i int) {
	st := a.title
	a.dirty = true
	switch st.stage {
	case titleMenu:
		a.titleMenuPick(i)
	case titleScenario:
		st.slot = state.Slot(fmt.Sprintf("%03d", i+1))
		a.titleLords()
	case titleLord:
		if i < len(st.lords) {
			a.titleDifficulty(st.lords[i])
		}
	case titleDifficulty:
		if len(st.lords) > 0 {
			a.startNewGame(st.slot, st.lords[0], i+1)
		}
	case titleLoad:
		a.titleLoadSlot(i)
	case titleMusic:
		if a.jb != nil {
			a.jb.Play(i)
		}
	}
}

func (a *app) titleMenuPick(i int) {
	st := a.title
	switch i {
	case 0:
		st.stage, st.pick = titleScenario, 0
		st.title = t("title.pickScenario")
		st.items = nil
		for k := 1; k <= 6; k++ {
			slot := state.Slot(fmt.Sprintf("%03d", k))
			d := game.ScenarioStart[slot]
			st.items = append(st.items, fmt.Sprintf("%d. %s", k,
				d.Format(game.ChineseEra)))
		}
	case 1:
		a.titleSaves()
	case 2, 3:
		st.stage = titleNote
		st.title = t("title.font")
		st.items = []string{t("title.fontNote")}
		a.title.stage = titleNote
	case 4:
		st.stage, st.pick = titleMusic, 0
		st.title = t("title.music")
		st.items = nil
		for k := 0; k < a.jb.Len(); k++ {
			st.items = append(st.items, tf("title.track", k+1))
		}
	case 5:
		a.quit = true
	}
}

// titleLords 列出這個劇本可以選的君主。
func (a *app) titleLords() {
	st := a.title
	sc, err := state.LoadScenario(a.c2, st.slot)
	if err != nil {
		st.stage, st.title, st.items = titleNote, t("title.pickLord"), []string{err.Error()}
		return
	}
	g, err := game.New(sc, 0, 5, a.edition)
	if err != nil {
		st.stage, st.title, st.items = titleNote, t("title.pickLord"), []string{err.Error()}
		return
	}
	st.stage, st.pick = titleLord, 0
	st.title = t("title.pickLord")
	st.items, st.lords = nil, nil
	for _, f := range g.Factions() {
		if !f.Alive {
			continue
		}
		who := g.Lord(f.ID)
		name := "—"
		if who != nil {
			name = who.Name
		}
		st.items = append(st.items, fmt.Sprintf("%d. %s（%d 郡）",
			len(st.lords)+1, name, len(g.Territory(f.ID))))
		st.lords = append(st.lords, int(f.ID))
	}
}

// titleDifficulty 問難度，然後開局。
func (a *app) titleDifficulty(faction int) {
	st := a.title
	max := 10
	if a.edition == state.EditionPlus {
		max = 20
	}
	st.stage, st.pick = titleDifficulty, 4
	st.title = t("title.pickDifficulty")
	st.items = nil
	for k := 1; k <= max; k++ {
		st.items = append(st.items, fmt.Sprintf("%d", k))
	}
	st.lords = []int{faction}
}

// titleSaves 列出可以讀的進度：remake 自己的六個，加上原版的六個。
func (a *app) titleSaves() {
	st := a.title
	st.stage, st.pick = titleLoad, 0
	st.title = t("title.pickSave")
	st.items, st.saves = nil, nil
	if a.saveDir != "" {
		for _, s := range session.Saves(a.saveDir) {
			if !s.Exists {
				continue
			}
			st.items = append(st.items, fmt.Sprintf("%d. %s", len(st.saves)+1, s.Describe()))
			st.saves = append(st.saves, s)
		}
	}
	st.orig = false
	if len(st.items) == 0 {
		st.items = append(st.items, t("title.noSave"))
	}
}

// titleLoadSlot 讀第 i 個進度。
func (a *app) titleLoadSlot(i int) {
	st := a.title
	if i >= len(st.saves) {
		return
	}
	s, err := session.Load(a.saveDir, st.saves[i].Slot, a.aiMode)
	if err != nil {
		st.stage, st.title, st.items = titleNote, t("title.pickSave"), []string{err.Error()}
		return
	}
	a.s = s
	a.view = ui.View{}
	a.title = nil
}

// startNewGame 用選好的劇本、君主、難度開一局。
func (a *app) startNewGame(slot state.Slot, faction, difficulty int) {
	st := a.title
	sc, err := state.LoadScenario(a.c2, slot)
	if err != nil {
		st.stage, st.title, st.items = titleNote, t("title.pickScenario"), []string{err.Error()}
		return
	}
	g, err := game.New(sc, state.FactionID(faction), difficulty, a.edition)
	if err != nil {
		st.stage, st.title, st.items = titleNote, t("title.pickScenario"), []string{err.Error()}
		return
	}
	brain, err := ai.New(a.aiMode)
	if err != nil {
		st.stage, st.title, st.items = titleNote, t("title.pickScenario"), []string{err.Error()}
		return
	}
	a.s = session.New(g, brain, state.FactionID(faction))
	a.view = ui.View{}
	a.title = nil
}
