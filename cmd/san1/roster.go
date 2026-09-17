package main

import (
	"fmt"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// rosterEntry 是「挑一位將軍」那一格（`0x18024`，`docs/spec/014` §4.2）。
type rosterEntry struct {
	pick   ui.RosterPick
	prompt string
	typed  string
	then   func(gi int)
	cancel func()

	// thenMulti 非 nil 是多選清單（`0x18286`）：max 是最多幾位。
	thenMulti func(list []int)
	max       int
}

// askRoster 開一份原版的挑人清單：那一郡（mode 決定收誰、key 決定第三欄與排序），
// 右側面板一頁十二列，下面板「提示(1-筆數):」後面回顯數字。沒有原版素材的文字
// 版面退回挑選清單。cancel 可以是 nil（空 Enter 就回主選單）。
func (a *app) askRoster(prompt string, pref int, mode game.PickMode, key game.PickKey,
	then func(gi int), cancel func()) {
	list := a.s.G.PickRoster(pref, mode, key)
	if a.art == nil {
		var items []pickItem
		for _, x := range list {
			items = append(items, pickItem{x.Name, x.Index, then})
		}
		a.pickFrom(prompt, items)
		a.cancel = cancel
		return
	}
	r := &rosterEntry{prompt: prompt, then: then, cancel: cancel}
	r.pick.Key = key
	for _, x := range list {
		r.pick.List = append(r.pick.List, x.Index)
	}
	// 上一次停的那一頁（`DS:0x66b2`）；超出這份清單就從頭（`0x18187`–`0x1819e`）。
	if a.rosterPage >= 0 && a.rosterPage < len(r.pick.List) {
		r.pick.Page = a.rosterPage
	}
	a.pick, a.num, a.menu, a.view.Menu, a.view.Items = nil, nil, 0, "", nil
	a.roster = r
	a.showRoster()
}

// askRosterMulti 是多選清單（`0x18286(提示, 鍵, 郡, 模式, 上限)`）：名單只按行動者鍵排、
// 鍵只決定第三欄；數字選一位就切換「*」（已選滿上限時不收），空 Enter 選過人就交出
// 那一份（照名單順序），一位都沒選是取消。頁數另記一格（`DS:0x66b4`）。
func (a *app) askRosterMulti(prompt string, pref int, mode game.PickMode, key game.PickKey, max int,
	then func(list []int), cancel func()) {
	list := a.s.G.PickRoster(pref, mode, game.PickByStatus)
	r := &rosterEntry{prompt: prompt, cancel: cancel, thenMulti: then, max: max}
	r.pick.Key, r.pick.Multi = key, true
	for _, x := range list {
		r.pick.List = append(r.pick.List, x.Index)
	}
	r.pick.Marked = make([]bool, len(r.pick.List))
	if a.rosterPageMulti >= 0 && a.rosterPageMulti < len(r.pick.List) {
		r.pick.Page = a.rosterPageMulti
	}
	a.pick, a.num, a.menu, a.view.Menu, a.view.Items = nil, nil, 0, "", nil
	a.roster = r
	a.showRoster()
}

// showRoster 把清單與下面板的提示放進畫面。
func (a *app) showRoster() {
	r := a.roster
	a.view.Roster = &r.pick
	if len(r.pick.List) == 0 {
		a.view.Prompt = r.prompt + t("pick.anyKey")
		return
	}
	a.view.Prompt = r.prompt + tf("pick.range", 1, len(r.pick.List)) + r.typed
}

// closeRoster 收掉清單。
func (a *app) closeRoster() *rosterEntry {
	r := a.roster
	a.roster, a.view.Roster = nil, nil
	a.view.Prompt = ""
	return r
}

// updateRoster 收清單的按鍵（`0x34ede` 的規則）：數字最多上限那麼多位、Backspace 刪、
// 空欄位的空白鍵換下一頁、空欄位 Enter 取消、超出這一頁的範圍就清掉重問。
func (a *app) updateRoster() {
	r := a.roster
	a.dirty = true
	if len(r.pick.List) == 0 {
		// 「沒有任何將軍」＋「請按任一鍵」，按了就取消（`0x180a9`）。
		if anyKeyPressed() {
			a.closeRoster()
			if r.cancel != nil {
				r.cancel()
			}
		}
		return
	}
	lo, hi := ui.RosterRange(&r.pick)
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		a.closeRoster()
		if r.cancel != nil {
			r.cancel()
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyBackspace):
		if n := len(r.typed); n > 0 {
			r.typed = r.typed[:n-1]
		}
		a.showRoster()
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		if r.typed == "" {
			r.pick.Page += ui.RosterPageRows
			if r.pick.Page >= len(r.pick.List) {
				r.pick.Page = 0
			}
		}
		a.showRoster()
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter):
		if r.typed == "" {
			if r.thenMulti != nil {
				var chosen []int
				for i, m := range r.pick.Marked {
					if m {
						chosen = append(chosen, r.pick.List[i])
					}
				}
				if len(chosen) > 0 {
					a.closeRoster()
					r.thenMulti(chosen)
					return
				}
			}
			a.closeRoster()
			if r.cancel != nil {
				r.cancel()
			}
			return
		}
		n, _ := strconv.Atoi(r.typed)
		r.typed = ""
		if n < lo || n > hi {
			a.showRoster()
			return
		}
		if r.thenMulti != nil {
			// `0x184fb`：已選的人數到上限就不切換（連取消選取也不收）。
			a.rosterPageMulti = r.pick.Page
			count := 0
			for _, m := range r.pick.Marked {
				if m {
					count++
				}
			}
			if count < r.max {
				r.pick.Marked[n-1] = !r.pick.Marked[n-1]
			}
			a.showRoster()
			return
		}
		a.rosterPage = r.pick.Page
		gi := r.pick.List[n-1]
		a.closeRoster()
		r.then(gi)
	default:
		width := len(fmt.Sprint(hi))
		for k := ebiten.Key0; k <= ebiten.Key9; k++ {
			if inpututil.IsKeyJustPressed(k) && len(r.typed) < width {
				r.typed += string(rune('0' + (k - ebiten.Key0)))
				a.showRoster()
			}
		}
	}
}

// askPref 是挑郡的數字輸入（`0x1d4ec`：提示接「(1-42):」，不在 ok 裡就重問）。
func (a *app) askPref(prompt string, ok func(pref int) bool, then func(pref int), cancel func()) {
	pp := &ui.PrefPick{}
	for id := 1; id <= 42; id++ {
		pp.Valid[id] = ok(id)
	}
	a.askRange(prompt, 1, 42, func(pref int) {
		a.view.PrefPick = nil
		if !ok(pref) {
			a.askPref(prompt, ok, then, cancel)
			return
		}
		then(pref)
	})
	a.view.PrefPick = pp
	a.cancel = func() {
		a.view.PrefPick = nil
		if cancel != nil {
			cancel()
		}
	}
}
