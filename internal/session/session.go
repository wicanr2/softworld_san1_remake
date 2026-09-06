// Package session 把規則層與電腦 AI 接成一個回合迴圈。
//
// 分成獨立的一層是為了**無頭測得到**：回合推進、電腦諸侯的行動、
// 訊息紀錄都在這裡，Ebiten 那一層只負責畫與收鍵。
package session

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// Session 是一局遊戲：局面 ＋ 電腦 AI ＋ 訊息紀錄。
type Session struct {
	G      *game.State
	Brain  ai.Brain
	Player state.FactionID

	// Log 是最近發生的事，新的在後面。畫面顯示與測試都讀它。
	Log []string

	// MaxLog 是保留幾則；0 用預設。
	MaxLog int
}

// New 開一局。
func New(g *game.State, brain ai.Brain, player state.FactionID) *Session {
	s := &Session{G: g, Brain: brain, Player: player, MaxLog: 200}
	s.note("%d 年 %d 月　開局", g.Date.Year, g.Date.Month)
	if !brain.Derived() {
		// ⚠ **這一行不能省。** 一個安靜地什麼都不做的電腦諸侯，
		// 在畫面上看起來就只是「這個諸侯這回合沒動作」。
		s.note("⚠ %s 還沒從原版還原出來，電腦諸侯不會行動", brain.Name())
	}
	return s
}

func (s *Session) note(format string, a ...any) {
	s.Log = append(s.Log, fmt.Sprintf(format, a...))
	max := s.MaxLog
	if max <= 0 {
		max = 200
	}
	if len(s.Log) > max {
		s.Log = s.Log[len(s.Log)-max:]
	}
}

// Do 讓玩家下一個命令。失敗時把理由記進 Log 並回傳錯誤。
func (s *Session) Do(o game.Order) error {
	if err := o.Apply(s.G, s.Player); err != nil {
		s.note("✗ %s：%v", o.Describe(s.G), err)
		return err
	}
	s.note("%s", o.Describe(s.G))
	return nil
}

// EndMonth 讓電腦諸侯行動，然後推進到下個月。
//
// 順序是**先電腦後推進**：玩家已經在這個月下過令了，電腦要在同一個
// 月份裡回應。推進之後才清掉各郡的下令旗標。
func (s *Session) EndMonth() {
	for _, f := range s.G.Factions() {
		if f.ID == s.Player || !f.Alive {
			continue
		}
		orders := s.Brain.Plan(s.G, f.ID)
		n, err := s.G.ApplyAll(orders, f.ID)
		if err != nil {
			// AI 產出違規命令是 bug。**記下來不要吞掉**——
			// 吞掉會讓它看起來像「電腦這回合比較保守」。
			s.note("⚠ 電腦（勢力 %d）的命令被擋下：%v", f.ID, err)
		}
		if n > 0 {
			lord := s.G.Lord(f.ID)
			name := fmt.Sprintf("勢力 %d", f.ID)
			if lord != nil {
				name = lord.Name
			}
			s.note("%s 下了 %d 個命令", name, n)
		}
	}
	s.G.EndMonth()
	s.note("──── %d 年 %d 月 ────", s.G.Date.Year, s.G.Date.Month)
}

// PlayerTerritory 是玩家的郡編號。
func (s *Session) PlayerTerritory() []int { return s.G.Territory(s.Player) }
