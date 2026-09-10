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

	// MonthOrder 是這個月郡的處理順序（43 格，洗過），MonthCursor 是
	// 走到哪一格。兩者都是原版的存檔欄位（`docs/re/08` §2）。
	MonthOrder  []int
	MonthCursor int

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

// MonthOrder 是這個月的郡順序（43 格），`MonthCursor` 是走到哪一格。
//
// 原版把這兩樣連同「這個月下過令沒有」的旗標一起存進進度檔
// （`docs/re/08` §2）：開月時順序表填成 0..42、**洗五輪**（每輪逐格與
// `RND(43)` 交換），旗標全設成 `0xFFFF`，第 0 格（啞元郡）單獨清掉。
//
// **順序有意義**：郡的回合是一條全域的迴圈，不是「一個勢力跑完換下一個」
// ——誰排在前面誰先花錢、先徵兵、先出兵。
// **洗牌本身在 `State.EndMonth` 裡**（開月常式 `0x17364`），這裡只是把
// 洗好的順序接過來。盤面上還沒有順序時（開局第一個月）才自己洗一次。
func (s *Session) shuffleMonth() {
	s.MonthOrder = s.G.TurnOrder()
	if len(s.MonthOrder) != 43 {
		s.MonthOrder = s.G.ShuffleTurnOrder()
	}
	s.MonthCursor = 0
}

// runPrefectureTurns 走完這個月的郡順序。
//
// 每一格照 `0x17471`：無主的郡跳過；重算所屬；**玩家的郡不跑分派器**
// （原版是在那裡停下來讓玩家下令，remake 這一邊玩家已經先下過了）；
// 自治的郡拿 offset 12 減一當等級跑同一個分派器（`0x17550`）。
func (s *Session) runPrefectureTurns() {
	planner, ok := s.Brain.(ai.PrefecturePlanner)
	if !ok {
		return
	}
	if len(s.MonthOrder) != 43 {
		s.shuffleMonth()
	}
	done := map[state.FactionID]int{}
	for ; s.MonthCursor < len(s.MonthOrder); s.MonthCursor++ {
		// 月迴圈每一格先抽一次（`0x15790`），跳過的格子也算。
		s.G.TurnTick()
		at := s.MonthOrder[s.MonthCursor]
		// **回合入口先重整這個郡的守將清單**（`0x17471` 的第一道
		// `call 0x1949e`），兵士那一欄跟著刷新。
		s.G.RefreshTroops(at)
		p := s.G.Prefecture(at)
		// **跳過的判斷用重算之前的值**，重算才在後面（`0x17471`：
		// 所屬 == 0xFF → 回 −1；接著才 `call 0x1e394`）。
		if p == nil || !p.Owned() {
			continue
		}
		s.G.RecomputeOwners()
		// 重算之後可能已經易主或變無主（別的郡搬空了它、或搬進來的人
		// 槽號較大蓋過原主）——分派器看的是重算之後的那一位。
		if p = s.G.Prefecture(at); p == nil || !p.Owned() {
			continue
		}
		id, level := p.Owner, s.G.AILevel(p.Owner)
		if id == s.Player {
			// 自治的郡是玩家的地盤，交給電腦按指定的性格經營
			// （`game.AutonomousFor`）；其餘的玩家已經自己下過令了。
			lv, auto := s.G.AutonomousFor(at)
			if !auto || p.Commanded {
				continue
			}
			level = lv
		}
		_, n, err := planner.ActPrefecture(s.G, id, at, level)
		s.drainBattles()
		if err != nil {
			s.note("⚠ %s 的命令被擋下：%v", prefectureName(s.G, at), err)
		}
		if n > 0 && id != s.Player {
			done[id] += n
		}
	}
	// **紀錄按勢力彙總。** 一郡一行會把紀錄淹掉，而玩家關心的是
	// 「這個月哪個諸侯動得多」。
	for _, f := range s.G.Factions() {
		n := done[f.ID]
		if n == 0 {
			continue
		}
		name := fmt.Sprintf("勢力 %d", f.ID)
		if lord := s.G.Lord(f.ID); lord != nil {
			name = lord.Name
		}
		s.note("%s 下了 %d 個命令", name, n)
	}
}

// MaxBattles 是保留幾場戰役的逐日戰報。
const MaxBattles = 8

// EndMonth 讓電腦諸侯行動，然後推進到下個月。
//
// 順序是**先電腦後推進**：玩家已經在這個月下過令了，電腦要在同一個
// 月份裡回應。推進之後才清掉各郡的下令旗標。
func (s *Session) EndMonth() {
	s.runPrefectureTurns()
	wasAlive := s.PlayerAlive()
	events := s.G.EndMonth()
	// 開月在結算裡跑完了，順序表換成新的一份。
	s.shuffleMonth()
	s.drainBattles()
	if wasAlive && !s.PlayerAlive() {
		s.note("✗ 你的勢力已被消滅")
		s.Over = true
	}
	s.note("──── %d 年 %d 月 ────", s.G.Date.Year, s.G.Date.Month)
	for _, e := range events {
		s.note("%s", e.Text)
	}
	// 統一判定在原版是每個月最後一件事，緊接在四季常式之後
	// （`0x1583d`，`docs/re/06` §9）。條件只有「所有有主的郡同屬一方」
	// ——**玉璽不在條件裡**。
	if f, done := s.G.Winner(); done {
		lord := s.G.Lord(f)
		name := fmt.Sprintf("勢力 %d", f)
		if lord != nil {
			name = lord.Name
		}
		s.note("★ %s 一 統 天 下", name)
		s.Over = true
	}
}

// PlayerTerritory 是玩家的郡編號。
func (s *Session) PlayerTerritory() []int { return s.G.Territory(s.Player) }

// PlayerAlive 回報玩家還在不在。
//
// **判準是「有沒有繼承人」不是「有沒有領地」**（原版 `0x15924`，`L0`）：
// 掃十六個諸侯槽，只要有一個是玩家操縱**而且君主欄不是 `0xFFFF`**
// 就繼續；一個都沒有才印「所有玩家皆無繼承人 遊戲結束」
// （`DS:0x97d4`）。所以**君主還活著、領地被打光的玩家，原版照樣讓他
// 繼續**——他還能靠麾下的武將翻身。
//
// **以領地判會在「有君主沒領地」這個狀態上分岔**：那樣會把還能翻身的
// 玩家判出局。判定要問君主欄。
//
// **沒有這一個的話，被消滅之後畫面只是變成空白**——玩家會以為是壞掉。
func (s *Session) PlayerAlive() bool {
	if s.Player == state.NoFaction {
		return true // 純觀戰
	}
	return s.G.Lord(s.Player) != nil
}


func prefectureName(g *game.State, at int) string {
	if p := g.Prefecture(at); p != nil {
		return p.Name
	}
	return fmt.Sprintf("郡 %d", at)
}
