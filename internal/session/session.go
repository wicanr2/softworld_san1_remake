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

	// Over 為真表示已經有人一統天下並拿到玉璽。
	Over bool

	battles []*game.BattleResult
}

// New 開一局。
func New(g *game.State, brain ai.Brain, player state.FactionID) *Session {
	s := &Session{G: g, Brain: brain, Player: player, MaxLog: 200}
	s.note("%d 年 %d 月　開局", g.Date.Year, g.Date.Month)
	if !brain.Derived() {
		// ⚠ **這一行不能省。** 一個只做一部分行為的電腦諸侯，在畫面上
		// 看起來就只是「這個諸侯比較保守」——差別看不出來。
		if done, total := brain.Coverage(); total > 0 {
			s.note("⚠ %s 還原到 %d/%d 種行為，其餘的電腦諸侯不會做",
				brain.Name(), done, total)
		} else {
			s.note("⚠ %s 不是還原，是 remake 自己的 AI", brain.Name())
		}
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
		s.drainBattles()
		return err
	}
	s.note("%s", o.Describe(s.G))
	s.drainBattles()
	return nil
}

// Battles 是最近打完的戰役，新的在後面。畫戰報那一頁要用完整的逐日紀錄。
//
// 只留最近幾場：一場三十天的主戰場動輒上百行，全部留著會把記憶體
// 吃到跟局面本身一樣大。
func (s *Session) Battles() []*game.BattleResult { return s.battles }

// Drain 把剛打完的戰役記進紀錄。
//
// 玩家親自指揮的戰役由畫面那一層收尾（`game.FinishAttack`），
// 不經過 Do 也不經過 EndMonth——**沒有這個出口，親征的戰報就掉了**。
func (s *Session) Drain() { s.drainBattles() }

// drainBattles 把剛打完的戰役記進紀錄。
func (s *Session) drainBattles() {
	for _, r := range s.G.DrainReports() {
		s.note("%s", r.Summary(s.G))
		s.battles = append(s.battles, r)
	}
	if n := len(s.battles); n > MaxBattles {
		s.battles = append([]*game.BattleResult(nil), s.battles[n-MaxBattles:]...)
	}
}

// MaxBattles 是保留幾場戰役的逐日戰報。
const MaxBattles = 8

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
		s.drainBattles()
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
	wasAlive := s.PlayerAlive()
	events := s.G.EndMonth()
	s.drainBattles()
	if wasAlive && !s.PlayerAlive() {
		s.note("✗ 你的勢力已被消滅")
		s.Over = true
	}
	s.note("──── %d 年 %d 月 ────", s.G.Date.Year, s.G.Date.Month)
	for _, e := range events {
		s.note("%s", e.Text)
	}
	if f, seal, done := s.G.Winner(); done {
		lord := s.G.Lord(f)
		name := fmt.Sprintf("勢力 %d", f)
		if lord != nil {
			name = lord.Name
		}
		if seal {
			s.note("★ %s 一 統 天 下", name)
			s.Over = true
		} else {
			// 「在遊戲結束前一定要拿到玉璽，如果屆時玉璽尚未到手，
			// 便須再多等候數月才能看到君臨天下的結局」（說明書 p.37）。
			s.note("%s 已無敵手，但玉璽尚未到手", name)
		}
	}
}

// PlayerTerritory 是玩家的郡編號。
func (s *Session) PlayerTerritory() []int { return s.G.Territory(s.Player) }

// PlayerAlive 回報玩家還在不在。
//
// **沒有這一個的話，被消滅之後畫面只是變成空白**——玩家會以為是壞掉。
func (s *Session) PlayerAlive() bool {
	if s.Player == state.NoFaction {
		return true // 純觀戰
	}
	f := s.G.Faction(s.Player)
	return f != nil && f.Alive
}
