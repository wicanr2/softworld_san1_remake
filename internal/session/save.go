package session

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
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
			name = "觀戰"
		}
	}
	if err := save.Write(dir, slot, s.G, name); err != nil {
		s.note("✗ 存檔失敗：%v", err)
		return err
	}
	s.note("已存入第 %d 個進度（%s）", slot, name)
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
	brain, err := ai.New(mode)
	if err != nil {
		return nil, err
	}
	s := New(g, brain, g.Player)
	s.note("讀入第 %d 個進度", slot)
	return s, nil
}

// Saves 回傳六個存檔槽的概況。
func Saves(dir string) []save.Info {
	if dir == "" {
		return nil
	}
	return save.List(dir)
}
