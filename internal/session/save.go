package session

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 存讀檔（說明書 p.25：「其他 → 儲存」，六個進度）。
//
// 檔案怎麼放是 `internal/save` 的事；這一層只管「存了什麼、讀完是哪一局」。

// SaveDir 是存檔要放哪裡。空字串表示這一局不能存——
// **不要偷偷選一個預設目錄**：玩家會在不知道的地方留下檔案。
func (s *Session) Save(dir string, slot int, name string) error {
	if dir == "" {
		return fmt.Errorf("session: 沒有指定存檔目錄")
	}
	if name == "" {
		if lord := s.G.Lord(s.Player); lord != nil {
			name = lord.Name
		} else {
			name = i18n.S("sess.spectator")
		}
	}
	if err := save.Write(dir, slot, s.G, name); err != nil {
		s.say("sess.saveFailed", err)
		return err
	}
	s.say("sess.saved", slot, name)
	return nil
}

// Load 讀一個進度，回傳一個新的 Session。
//
// **回新的而不是就地改**：讀檔失敗時原本那一局要毫髮無傷，
// 玩家按錯格子不該把正在玩的東西弄壞。
func Load(dir string, slot int, mode ai.Mode) (*Session, error) {
	if dir == "" {
		return nil, fmt.Errorf("session: 沒有指定存檔目錄")
	}
	g, err := save.Read(dir, slot)
	if err != nil {
		return nil, err
	}
	// 版本來自存檔，AI 版本來自旗標——**兩者可能不同版**。
	// 還原型的 AI 配另一版的規則會安靜地算錯（`internal/ai` `CheckEdition`）。
	brain, err := brainFor(g, mode)
	if err != nil {
		return nil, err
	}
	s := New(g, brain, g.Player)
	s.say("sess.loaded", slot)
	return s, nil
}

// brainFor 決定這一局用哪一個 AI：**存檔裡有就聽存檔的**。
//
// 玩家在遊戲中換過 AI（「其他 → 電腦AI」）之後存檔，那一項會寫進
// `Options.AIMode`。讀回來卻套旗標的版本，玩家看到的就是「設定沒存到」
// ——而畫面上唯一的差別只是電腦諸侯下不同的命令，看不出來。
// 沒有那一項（舊存檔、或從頭到尾沒動過）才用旗標。
func brainFor(g *game.State, mode ai.Mode) (ai.Brain, error) {
	if m := ai.Mode(g.Options.AIMode); m != "" {
		mode = m
	}
	if err := ai.CheckEdition(mode, g.Edition); err != nil {
		return nil, err
	}
	return ai.New(mode)
}

// LoadOriginal 讀玩家自己的**原版**進度（`DATA2.GRP` 裡的六個），
// 回傳一個新的 Session。
//
// ⚠ **只讀不寫。** 之後要存還是存進 remake 自己的目錄；原版的容器
// 一個位元組都不動（`internal/save` 的說明）。
func LoadOriginal(c *assets.Container, slot int, ed state.Edition, mode ai.Mode) (*Session, error) {
	g, err := save.ReadOriginal(c, slot, ed)
	if err != nil {
		return nil, err
	}
	brain, err := brainFor(g, mode)
	if err != nil {
		return nil, err
	}
	s := New(g, brain, g.Player)
	s.say("sess.loadedOrig", slot)
	return s, nil
}

// OriginalSaves 回傳原版六個進度的概況。
func OriginalSaves(c *assets.Container) []save.Info { return save.ListOriginal(c) }

// Saves 回傳六個存檔槽的概況。
func Saves(dir string) []save.Info {
	if dir == "" {
		return nil
	}
	return save.List(dir)
}
