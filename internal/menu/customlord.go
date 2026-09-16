package menu

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 新君主的設定（`docs/spec/013`）。
//
// 原版是七項：年齡、體能、謀略、戰力、魅力、領地、修改姓名（手冊 p.14）。
// remake 這一版做**四項能力 ＋ 領地**，年齡與姓名照範本（`0x3e44c` 的
// 年齡 20、`BASEPRE` 出貨內容「新君主」）——那兩項原版怎麼收沒解，
// 而範本是 `L0`（`docs/spec/013` §4）。
//
// 上下鍵換項目、左右鍵加減、在「完成」那一項按下去就進難度。

// customState 是設定到一半的那一位。
type customState struct {
	faction int
	lord    state.CustomLord
	// spare 是還沒分配的點數。
	spare int
	// blanks 是可以選的空白郡，pref 是選到第幾個。
	blanks []state.Prefecture
	pref   int
}

// customRows 是清單上的項數：四項能力 ＋ 領地 ＋ 完成。
const customRows = 6

// isCustom 回報這個諸侯槽是不是空的新君主欄。
func (s *Screen) isCustom(faction int) bool {
	for _, f := range s.customs {
		if f == faction {
			return true
		}
	}
	return false
}

// pickCustomLord 進新君主的設定。
func (s *Screen) pickCustomLord(faction int) {
	sc, err := state.LoadScenario(s.c2, s.slot)
	if err != nil {
		s.note(i18n.S("title.newLord"), err.Error())
		return
	}
	var blanks []state.Prefecture
	for _, p := range sc.Prefectures() {
		// 第 0 筆是原版的啞元，跳過。
		if p.ID > 0 && !p.Owned() {
			blanks = append(blanks, p)
		}
	}
	if len(blanks) == 0 {
		// **空白郡是硬條件**（手冊 p.14）。一個都沒有的時候要說出來，
		// 不能讓玩家設定完才發現開不了局。
		s.note(i18n.S("title.newLord"), i18n.S("title.noBlankPref"))
		return
	}
	s.custom = &customState{
		faction: faction,
		spare:   state.CustomLordPoints,
		blanks:  blanks,
	}
	s.custom.lord.Prefecture = blanks[0].ID
	for i := range s.custom.lord.Name {
		s.custom.lord.Name[i] = []rune(i18n.S("title.newLordName"))[i]
	}
	s.stage, s.pick = CustomLord, 0
	s.title = i18n.S("title.newLord")
	s.rebuildCustom()
}

// rebuildCustom 把目前的值畫成清單。**每次改值都要重畫**——
// 清單是靜態字串，不重畫的話按了左右鍵畫面不會動，看起來像沒反應。
func (s *Screen) rebuildCustom() {
	c := s.custom
	if c == nil {
		return
	}
	st, in, mi, ch := c.lord.Stats()
	name := ""
	for _, r := range c.lord.Name {
		name += string(r)
	}
	pref := "—"
	if c.pref < len(c.blanks) {
		pref = c.blanks[c.pref].Name
	}
	s.items = []string{
		i18n.Sf("title.newLordStat", 1, i18n.S("fld.stamina"), st, c.lord.Stamina),
		i18n.Sf("title.newLordStat", 2, i18n.S("fld.intel"), in, c.lord.Intellect),
		i18n.Sf("title.newLordStat", 3, i18n.S("fld.war"), mi, c.lord.Might),
		i18n.Sf("title.newLordStat", 4, i18n.S("fld.charm"), ch, c.lord.Charm),
		i18n.Sf("title.newLordPref", 5, pref),
		i18n.Sf("title.newLordDone", 6, c.spare, name),
	}
}

// Adjust 在新君主那一層加減目前這一項。d 是 +1 或 −1。
//
// 回傳「這一鍵被吃掉了」——其他層沒有左右鍵可調，呼叫端照舊。
func (s *Screen) Adjust(d int) bool {
	if s.stage != CustomLord || s.custom == nil || d == 0 {
		return false
	}
	c := s.custom
	switch s.pick {
	case 0, 1, 2, 3:
		add := []*int{&c.lord.Stamina, &c.lord.Intellect, &c.lord.Might, &c.lord.Charm}[s.pick]
		// 加要有點數可用，減不能減到負的（範本的底不退點）。
		if d > 0 && c.spare <= 0 {
			return true
		}
		if d < 0 && *add <= 0 {
			return true
		}
		*add += d
		c.spare -= d
	case 4:
		c.pref = (c.pref + d + len(c.blanks)) % len(c.blanks)
		c.lord.Prefecture = c.blanks[c.pref].ID
	}
	s.rebuildCustom()
	return true
}

// confirmCustom 收下清單上那一項的 Enter。
func (s *Screen) confirmCustom(i int) {
	c := s.custom
	if c == nil {
		return
	}
	if i != customRows-1 {
		// 不是「完成」就只是移動游標——**能力那幾項用左右鍵調**。
		s.pick = i
		return
	}
	// 開局之前先驗一次：點數、空白郡、姓名都在這裡擋下來。
	sc, err := state.LoadScenario(s.c2, s.slot)
	if err != nil {
		s.note(i18n.S("title.newLord"), err.Error())
		return
	}
	if err := c.lord.Validate(sc); err != nil {
		s.note(i18n.S("title.newLord"), err.Error())
		return
	}
	s.pickDifficulty(c.faction)
}

// CustomView 是原版素材畫面畫新君主那一格要的東西（`docs/spec/005` §9.5）：
// 諸侯槽、肖像編號、名字、六行字（與 Items 同序，字串是原版的短格式）、
// 目前這一位排第幾個新君主。沒有在設定就回 false。
type CustomView struct {
	Faction  int
	Portrait int
	Name     string
	Lines    [customRows]string
	Spare    int
}

// Custom 交出設定到一半那一位給原版素材畫面；沒有就回 nil。
func (s *Screen) Custom() *CustomView {
	c := s.custom
	if c == nil {
		return nil
	}
	nth := 0
	for i, f := range s.customs {
		if f == c.faction {
			nth = i
		}
	}
	portrait := 0
	if nth < len(state.CustomLordPortrait) {
		portrait = state.CustomLordPortrait[nth]
	}
	st, in, mi, ch := c.lord.Stats()
	name := ""
	for _, r := range c.lord.Name {
		name += string(r)
	}
	pref := "—"
	if c.pref < len(c.blanks) {
		pref = i18n.PlaceName(c.blanks[c.pref].Name)
	}
	return &CustomView{
		Faction: c.faction, Portrait: portrait, Name: name, Spare: c.spare,
		Lines: [customRows]string{
			i18n.Sf("title.artStamina", 1, st),
			i18n.Sf("title.artIntel", 2, in),
			i18n.Sf("title.artWar", 3, mi),
			i18n.Sf("title.artCharm", 4, ch),
			i18n.Sf("title.artPref", 5, pref),
			i18n.Sf("title.artDone", 6, c.spare),
		},
	}
}

// CustomSummary 是設定到一半那一位的摘要，給畫面用；沒有就回空字串。
func (s *Screen) CustomSummary() string {
	if s.custom == nil {
		return ""
	}
	st, in, mi, ch := s.custom.lord.Stats()
	return fmt.Sprintf("%d/%d/%d/%d", st, in, mi, ch)
}
