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
	"slices"

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
	PlayerCount // 「請問有幾人玩(0-%d)」（`docs/spec/019` §1）
	Demo        // 0 人：「電腦自動示範模式」，按任意鍵往下
	Lord
	CustomLord // 新君主的設定（`docs/spec/013`）
	LordBorn   // 「新君主出現!!」那一格：新君主的肖像與一句話，按任意鍵（`docs/spec/005` §9.5）
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

	// playerCount 是這一局幾位玩家；players 是已經選好的君主（依玩家序號）。
	playerCount int
	players     []int
	// difficulty 是設好的難度；pendingCustom 是被選走、還沒分配能力的新君主欄；
	// customDone 是分配好的新君主（開局時依序寫進劇本）。
	difficulty    int
	pendingCustom []int
	customDone    []customLordDone

	// g 是選君主那一層拿來列候選的局面（肖像、名字、地圖填色都從它取）。
	g *game.State

	// customs 是這個劇本還空著的新君主欄（諸侯槽號）。
	customs []int
	// lordItems 是選君主那一層的清單（文字版面用）。
	lordItems []string
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

// Lords 是選君主那一層的候選（諸侯槽號）；Game 是列出它們的那個局面。
// 從玩家人數到設難度那幾層都有意義。
func (s *Screen) Lords() []int { return s.lords }

// Players 是已經選好的君主（諸侯槽號，依玩家序號）；PlayerCount 是這一局幾位。
func (s *Screen) Players() []int    { return s.players }
func (s *Screen) PlayerCount() int  { return s.playerCount }
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
	s.players, s.playerCount, s.pendingCustom, s.customDone = nil, 0, nil, nil
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
	case PlayerCount:
		s.pickPlayers(i)
	case Demo:
		s.pickDifficulty()
	case Lord:
		s.pickPlayerLord(i)
	case CustomLord:
		s.confirmCustom(i)
	case LordBorn:
		if s.custom != nil {
			s.customDone = append(s.customDone, customLordDone{s.custom.faction, s.custom.lord})
			s.custom = nil
		}
		return s.nextCustomOrStart()
	case Difficulty:
		s.difficulty = i + 1
		// 被選走的新君主欄**設完難度才分配能力**（`0x123aa`，照槽號順序）。
		s.pendingCustom = nil
		for _, f := range s.lords {
			if s.isCustom(f) && slices.Contains(s.players, f) {
				s.pendingCustom = append(s.pendingCustom, f)
			}
		}
		slices.Sort(s.pendingCustom)
		return s.nextCustomOrStart()
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
		// 六個按鈕上的字照原版的字串（`DS:5ee1`…`DS:5e78`，含字間的空白，
		// `docs/spec/005` §6.4）。
		s.items = nil
		for k := 1; k <= 6; k++ {
			s.items = append(s.items, i18n.Sf(fmt.Sprintf("title.scenario%d", k)))
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
	s.items, s.lords, s.customs, s.g = nil, nil, nil, g
	s.players, s.customDone = nil, nil
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
	s.lordItems = s.items
	// 先問幾人玩（`0x12072`）：0 … 候選數，預設 1。
	s.stage, s.pick = PlayerCount, 1
	s.title = i18n.Sf("title.playerCount", len(s.lords))
	s.items = nil
	for k := 0; k <= len(s.lords); k++ {
		s.items = append(s.items, fmt.Sprintf("%d", k))
	}
}

// pickPlayers 收玩家人數：0 人進示範模式，否則逐位選君主。
func (s *Screen) pickPlayers(n int) {
	if n < 0 || n > len(s.lords) {
		return
	}
	s.playerCount, s.players = n, nil
	if n == 0 {
		s.stage, s.pick = Demo, 0
		s.title, s.items = i18n.S("title.demo"), []string{i18n.S("title.demo")}
		return
	}
	s.askLord()
}

// askLord 問下一位玩家選哪一位君主（「第%d位,請選擇(1-%d)」）。
func (s *Screen) askLord() {
	s.stage = Lord
	s.title = i18n.Sf("title.lordPrompt", len(s.players)+1, len(s.lords))
	s.items = s.lordItems
	if s.pick >= len(s.items) {
		s.pick = 0
	}
}

// pickPlayerLord 收一位玩家的君主。已經被選走的不收（原版重問）。
func (s *Screen) pickPlayerLord(i int) {
	if i < 0 || i >= len(s.lords) || slices.Contains(s.players, s.lords[i]) {
		return
	}
	s.players = append(s.players, s.lords[i])
	if len(s.players) < s.playerCount {
		s.askLord()
		return
	}
	s.pickDifficulty()
}

// customLordDone 是分配好能力的一位新君主。
type customLordDone struct {
	faction int
	lord    state.CustomLord
}

// nextCustomOrStart 分配下一位被選走的新君主欄；都分完就開局。
func (s *Screen) nextCustomOrStart() *session.Session {
	if len(s.pendingCustom) > 0 {
		f := s.pendingCustom[0]
		s.pendingCustom = s.pendingCustom[1:]
		s.pickCustomLord(f) // 沒有空白郡時停在那一句話
		return nil
	}
	return s.start()
}

// pickDifficulty 問難度。上限看版本（原版 10、加強版 20）。
func (s *Screen) pickDifficulty() {
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

func (s *Screen) start() *session.Session {
	sc, err := s.scenarioWithCustoms()
	if err != nil {
		s.note(i18n.S("title.newLord"), err.Error())
		return nil
	}
	players := make([]state.FactionID, len(s.players))
	for i, f := range s.players {
		players[i] = state.FactionID(f)
	}
	g, err := game.NewPlayers(sc, players, s.difficulty, s.edition)
	if err != nil {
		s.note(i18n.S("title.pickScenario"), err.Error())
		return nil
	}
	brain, err := ai.New(s.mode)
	if err != nil {
		s.note(i18n.S("title.pickScenario"), err.Error())
		return nil
	}
	if len(s.customDone) > 0 {
		// 名字的字模：人物表裡只有造字碼位，字模另外存
		//（`docs/spec/013` R4）。名字固定是「新君主」，而**原版出貨的
		// `BASEPRE` 內容正好就是它**（`docs/re/08` §3：六個進度位元組
		// 完全相同），所以直接把出貨的那一份帶進這一局。
		if x := shippedGlyphs(s.c2); x != nil {
			g.SetGlyphs(x)
		}
	}
	first := state.FactionID(state.NoFaction)
	if len(players) > 0 {
		first = players[0]
	}
	return session.New(g, brain, first)
}

// scenarioWithCustoms 是這個劇本加上已經分配好的新君主（依分配順序寫進去）。
func (s *Screen) scenarioWithCustoms() (*state.Scenario, error) {
	sc, err := state.LoadScenario(s.c2, s.slot)
	if err != nil {
		return nil, err
	}
	for _, c := range s.customDone {
		if sc, err = sc.WithCustomLord(c.faction, c.lord); err != nil {
			return nil, err
		}
	}
	return sc, nil
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
