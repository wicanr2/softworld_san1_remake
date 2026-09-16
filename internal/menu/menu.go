// Package menu 是主選單的狀態機。
//
// 六個項目照原版（`ui.TitleItems`，畫面在 `docs/spec/005` §6）：
//
//  1. 開始新遊戲  2. 載入舊進度  3. 使用楷書字
//  4. 使用隸書字  5. 音樂欣賞    6. 回作業系統
//
// **這一包不碰畫面也不碰 Ebiten**，所以無頭測得到——`cmd/san1` 只把
// 按鍵接進來、把 `Title()`／`Items()`／`Sel()` 交給 `internal/ui` 畫。
//
// 選劇本、選君主、選難度那幾層的**版面**是 remake 自己排的：原版那一段
// 跑在開機鏈第二層（`DATA0.GRP`），主程式的碼段 dump 涵蓋不到。
// 選項的**內容**是原版的資料——劇本的起始年月、勢力的君主姓名與郡數、
// 難度上限都從解出來的表來。
package menu

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// Stage 是停在哪一層。
type Stage int

const (
	Menu Stage = iota // 六個項目
	Scenario
	Lord
	CustomLord // 新君主的設定（`docs/spec/013`）
	Difficulty
	Load
	Music
	Note // 只有一句話，按任意鍵回主選單
)

// ItemCount 是主選單的項數。
const ItemCount = 6

// Screen 是主選單的狀態。
type Screen struct {
	c2      *assets.Container
	edition state.Edition
	mode    ai.Mode
	saveDir string
	tracks  int

	stage Stage
	sel   int // 主選單反白的項目
	pick  int // 子清單反白的項目

	title string
	items []string

	slot  state.Slot
	lords []int
	saves []save.Info

	// g 是選君主那一層拿來列候選的局面（肖像、名字、地圖填色都從它取）。
	g *game.State

	// customs 是這個劇本還空著的新君主欄（諸侯槽號）。
	customs []int
	// custom 非 nil 表示玩家選了其中一個，正在設定那一位。
	custom *customState

	// track 是最近一次在「音樂欣賞」選的曲子；−1 表示沒有。
	track int

	quit bool
}

// New 開一個停在主選單的狀態機。tracks 是配樂有幾首（沒有就給 0）。
func New(c2 *assets.Container, ed state.Edition, mode ai.Mode, saveDir string, tracks int) *Screen {
	return &Screen{c2: c2, edition: ed, mode: mode, saveDir: saveDir,
		tracks: tracks, track: -1}
}

// Stage／Title／Items／Sel 是畫面要的東西。
func (s *Screen) Stage() Stage    { return s.stage }
func (s *Screen) Title() string   { return s.title }
func (s *Screen) Items() []string { return s.items }

// Sel 是目前反白的項次。
func (s *Screen) Sel() int {
	if s.stage == Menu {
		return s.sel
	}
	return s.pick
}

// Len 是這一層有幾項。
func (s *Screen) Len() int {
	if s.stage == Menu {
		return ItemCount
	}
	return len(s.items)
}

// Lords 是選君主那一層的候選（諸侯槽號，與 Items 同序）；Game 是列出
// 它們的那個局面。兩者只在 Lord 那一層有意義。
func (s *Screen) Lords() []int      { return s.lords }
func (s *Screen) Game() *game.State { return s.g }

// IsCustom 回報候選 f 是不是空的新君主欄；CustomIndex 是它排第幾個新君主
// （0 起；不是新君主欄回 −1）。
func (s *Screen) IsCustom(f int) bool { return s.isCustom(f) }
func (s *Screen) CustomIndex(f int) int {
	for i, c := range s.customs {
		if c == f {
			return i
		}
	}
	return -1
}

// Quit 回報玩家有沒有選「回作業系統」。
func (s *Screen) Quit() bool { return s.quit }

// Track 取出並清掉「音樂欣賞」選的曲子；−1 表示沒有新的選擇。
func (s *Screen) Track() int {
	t := s.track
	s.track = -1
	return t
}

// Move 移動反白。
func (s *Screen) Move(d int) {
	n := s.Len()
	if n == 0 {
		return
	}
	cur := &s.pick
	if s.stage == Menu {
		cur = &s.sel
	}
	*cur = ((*cur+d)%n + n) % n
}

// Back 回上一層。已經在主選單就不動。
func (s *Screen) Back() {
	if s.stage == Menu {
		return
	}
	s.stage, s.items, s.pick, s.title = Menu, nil, 0, ""
	s.custom = nil
}

// Confirm 選下去。回傳非 nil 表示這一局開好了（或讀好了）。
func (s *Screen) Confirm(i int) *session.Session {
	switch s.stage {
	case Menu:
		s.sel = i
		s.menuPick(i)
	case Scenario:
		s.slot = state.Slot(fmt.Sprintf("%03d", i+1))
		s.pickLord()
	case Lord:
		if i < len(s.lords) {
			if s.isCustom(s.lords[i]) {
				s.pickCustomLord(s.lords[i])
			} else {
				s.pickDifficulty(s.lords[i])
			}
		}
	case CustomLord:
		s.confirmCustom(i)
	case Difficulty:
		if len(s.lords) > 0 {
			return s.start(s.slot, s.lords[0], i+1)
		}
	case Load:
		return s.load(i)
	case Music:
		if i < s.tracks {
			s.track = i
		}
	case Note:
		s.Back()
	}
	return nil
}

func (s *Screen) menuPick(i int) {
	switch i {
	case 0:
		s.stage, s.pick = Scenario, 0
		s.title = i18n.S("title.pickScenario")
		s.items = nil
		for k := 1; k <= 6; k++ {
			d := game.ScenarioStart[state.Slot(fmt.Sprintf("%03d", k))]
			s.items = append(s.items, fmt.Sprintf("%d. %s", k, d.Format(game.ChineseEra)))
		}
	case 1:
		s.pickSave()
	case 2, 3:
		s.note(i18n.S("title.font"), i18n.S("title.fontNote"))
	case 4:
		s.stage, s.pick = Music, 0
		s.title = i18n.S("title.music")
		s.items = nil
		for k := 0; k < s.tracks; k++ {
			s.items = append(s.items, i18n.Sf("title.track", k+1))
		}
		if s.tracks == 0 {
			s.note(i18n.S("title.music"), i18n.S("title.noMusic"))
		}
	case 5:
		s.quit = true
	}
}

// note 停在「只有一句話」那一層。
func (s *Screen) note(title, msg string) {
	s.stage, s.pick = Note, 0
	s.title, s.items = title, []string{msg}
}

// pickLord 列出這個劇本可以選的君主。
func (s *Screen) pickLord() {
	sc, err := state.LoadScenario(s.c2, s.slot)
	if err != nil {
		s.note(i18n.S("title.pickLord"), err.Error())
		return
	}
	g, err := game.New(sc, 0, 5, s.edition)
	if err != nil {
		s.note(i18n.S("title.pickLord"), err.Error())
		return
	}
	s.stage, s.pick = Lord, 0
	s.title = i18n.S("title.pickLord")
	s.items, s.lords, s.customs, s.g = nil, nil, nil, g
	for _, f := range g.Factions() {
		if !f.Alive {
			continue
		}
		name := "—"
		if who := g.Lord(f.ID); who != nil {
			name = who.Name
		}
		s.items = append(s.items, i18n.Sf("title.lordLine",
			len(s.lords)+1, i18n.PersonName(name), len(g.Territory(f.ID))))
		s.lords = append(s.lords, int(f.ID))
	}
	// **空的新君主欄也要列出來**：原版的「選角色」那一層就有它們
	//（劇本 001 是十六個槽裡的兩個，手冊 p.7 寫「16（含 2 個新君主欄）」）。
	// 不列的話玩家永遠選不到自創君主。
	for _, f := range sc.CustomLordSlots() {
		s.items = append(s.items, i18n.Sf("title.newLordLine", len(s.lords)+1))
		s.lords = append(s.lords, f)
		s.customs = append(s.customs, f)
	}
}

// pickDifficulty 問難度。上限看版本（原版 10、加強版 20）。
func (s *Screen) pickDifficulty(faction int) {
	max := 10
	if s.edition == state.EditionPlus {
		max = 20
	}
	s.stage, s.pick = Difficulty, 4
	s.title = i18n.S("title.pickDifficulty")
	s.items = nil
	for k := 1; k <= max; k++ {
		s.items = append(s.items, fmt.Sprintf("%d", k))
	}
	s.lords = []int{faction}
}

// pickSave 列出可以讀的進度。
func (s *Screen) pickSave() {
	s.stage, s.pick = Load, 0
	s.title = i18n.S("title.pickSave")
	s.items, s.saves = nil, nil
	if s.saveDir != "" {
		for _, info := range session.Saves(s.saveDir) {
			if !info.Exists {
				continue
			}
			s.items = append(s.items, fmt.Sprintf("%d. %s", len(s.saves)+1, info.Describe()))
			s.saves = append(s.saves, info)
		}
	}
	if len(s.items) == 0 {
		s.note(i18n.S("title.pickSave"), i18n.S("title.noSave"))
	}
}

func (s *Screen) load(i int) *session.Session {
	if i >= len(s.saves) {
		return nil
	}
	ss, err := session.Load(s.saveDir, s.saves[i].Slot, s.mode)
	if err != nil {
		s.note(i18n.S("title.pickSave"), err.Error())
		return nil
	}
	return ss
}

func (s *Screen) start(slot state.Slot, faction, difficulty int) *session.Session {
	sc, err := state.LoadScenario(s.c2, slot)
	if err != nil {
		s.note(i18n.S("title.pickScenario"), err.Error())
		return nil
	}
	// 選了新君主欄就先把那一位寫進劇本，再開局。
	custom := s.custom != nil && s.custom.faction == faction
	if custom {
		sc, err = sc.WithCustomLord(faction, s.custom.lord)
		if err != nil {
			s.note(i18n.S("title.newLord"), err.Error())
			return nil
		}
	}
	g, err := game.New(sc, state.FactionID(faction), difficulty, s.edition)
	if err != nil {
		s.note(i18n.S("title.pickScenario"), err.Error())
		return nil
	}
	brain, err := ai.New(s.mode)
	if err != nil {
		s.note(i18n.S("title.pickScenario"), err.Error())
		return nil
	}
	if custom {
		// 名字的字模：人物表裡只有造字碼位，字模另外存
		//（`docs/spec/013` R4）。名字固定是「新君主」，而**原版出貨的
		// `BASEPRE` 內容正好就是它**（`docs/re/08` §3：六個進度位元組
		// 完全相同），所以直接把出貨的那一份帶進這一局。
		if x := shippedGlyphs(s.c2); x != nil {
			g.SetGlyphs(x)
		}
	}
	return session.New(g, brain, state.FactionID(faction))
}

// shippedGlyphs 取原版出貨的字模；讀不到回 nil。
//
// **讀不到不是錯誤**：沒有字模只是名字畫不出來，不該擋著開局。
func shippedGlyphs(c *assets.Container) *state.Glyphs {
	if c == nil {
		return nil
	}
	i, ok := c.ByName("BASEPRE.SV1")
	if !ok {
		return nil
	}
	x, err := state.DecodeGlyphs(c.Data(i))
	if err != nil {
		return nil
	}
	return x
}
