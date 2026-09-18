// Package session 把規則層與電腦 AI 接成一個回合迴圈。
//
// 分成獨立的一層是為了**無頭測得到**：回合推進、電腦諸侯的行動、
// 訊息紀錄都在這裡，Ebiten 那一層只負責畫與收鍵。
package session

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
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

	// entered 表示游標那一格已經跑過回合入口（抽樣、重整、重算所屬），
	// 停在玩家的郡時留著，玩家下完令才往下走（`AdvanceToHuman`）。
	entered bool
	// done 是這個月電腦諸侯各下了幾道令（紀錄彙總用）。
	done map[state.FactionID]int

	// Bubbles 是還沒給玩家看的訊息框（原版的訊息常式畫的「肖像＋對白」，
	// `game.Bubble`），新的在後面。畫面層一次秀一格、按鍵收一格
	// （`PopBubble`）；沒有原版素材的文字版面直接把它們寫進 Log。
	Bubbles []*game.Bubble
}

// New 開一局。
func New(g *game.State, brain ai.Brain, player state.FactionID) *Session {
	s := &Session{G: g, Brain: brain, Player: player, MaxLog: 200}
	s.say("sess.start", g.Date.Year, g.Date.Month)
	s.noteBrain()
	return s
}

// noteBrain 把「現在用的是哪一種 AI」記進 Log。
func (s *Session) noteBrain() {
	b := s.Brain
	if b.Derived() {
		return
	}
	// ⚠ **這一行不能省。** 一個只做一部分行為的電腦諸侯，在畫面上
	// 看起來就只是「這個諸侯比較保守」——差別看不出來。
	if done, total := b.Coverage(); total > 0 {
		s.say("sess.partial",
			b.Name(), done, total)
	} else {
		s.say("sess.notFaithful", b.Name())
	}
}

// SetBrain 在遊戲進行中換掉電腦 AI（「其他 → 電腦AI」）。
//
// **這是 remake 加的**（`docs/design/02` §5）：原版只有一套 AI。
// 換的時候要記進 Log——AI 換了而畫面上沒留下痕跡，之後回頭問
// 「這個諸侯為什麼突然不動了」就查不出來。
//
// 月中換也安全：AI 不帶跨郡的狀態，`runPrefectureTurns` 下一格就
// 改問新的那一個。已經發出去的命令不會回頭。
func (s *Session) SetBrain(b ai.Brain) {
	if b == nil || b == s.Brain {
		return
	}
	s.Brain = b
	s.say("sess.aiSwitched", b.Name())
	s.noteBrain()
}

// say 記一則走譯文的訊息（`sess.*`）；note 收的是已經成句的字串
//（事件、命令描述、戰報摘要都是別的套件譯好的）。
//
// **訊息紀錄會出現在下面板上**（原版素材畫面顯示最後一則），先前這些
// 字寫死中文，英日文玩家看到「已存入第 1 個進度」。
func (s *Session) say(key string, a ...any) { s.note("%s", i18n.Sf(key, a...)) }

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
		s.say("sess.failed", o.Describe(s.G), err)
		s.drainBattles()
		return err
	}
	s.note("%s", o.Describe(s.G))
	s.drainBattles()
	// 一道命令裡送好幾件的（賞賜物品）等 CloseGift 才算下完。
	if k, ok := o.(interface{ KeepsTurn() bool }); ok && k.KeepsTurn() {
		return nil
	}
	// 玩家一郡一道令，下完就是這個郡的回合走完（加強版在這裡重整守將清單）。
	s.G.FinishTurn(o.Prefecture())
	return nil
}

// CloseGift 收掉一道賞賜物品（`0x1cfd6` 回到主命令迴圈）：賞出過東西
// 就是這個郡的回合走完；一件都沒送回主選單再問（`0x17791`）。
func (s *Session) CloseGift(r *game.GiftRound) bool {
	if !s.G.CloseGift(r) {
		return false
	}
	s.G.FinishTurn(r.At)
	return true
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
	// 君主戰死的繼承在戰役的分贓裡跑（`0x26c28` → `0x14968`），那兩則
	// 對白從 `PendingEvents` 收。
	s.collect(s.G.PendingEvents())
	if n := len(s.battles); n > MaxBattles {
		s.battles = append([]*game.BattleResult(nil), s.battles[n-MaxBattles:]...)
	}
}

// collect 把一串事件收進 Log 與訊息框佇列：有字的進 Log，帶泡泡的排隊。
func (s *Session) collect(events []game.Event) {
	for _, e := range events {
		if e.Text != "" {
			s.note("%s", e.Text)
		}
		if e.Bubble != nil {
			s.Bubbles = append(s.Bubbles, e.Bubble)
		}
	}
}

// Queue 把一串事件（勸諫、宣戰）排進 Log 與訊息框佇列。
func (s *Session) Queue(events []game.Event) { s.collect(events) }

// Bubble 是現在該秀的那一格訊息框；沒有就是 nil。
func (s *Session) Bubble() *game.Bubble {
	if len(s.Bubbles) == 0 {
		return nil
	}
	return s.Bubbles[0]
}

// PopBubble 收掉現在這一格。
func (s *Session) PopBubble() {
	if len(s.Bubbles) > 0 {
		s.Bubbles = s.Bubbles[1:]
	}
}

// FlushBubbles 把佇列裡的訊息框全部改寫成 Log 的一行（文字版面用：
// 「名字：對白」），佇列清空。
func (s *Session) FlushBubbles() {
	for _, b := range s.Bubbles {
		if b.FaceOnly || b.Card || b.Scene > 0 || b.Panel != 0 || b.MapBattle != nil {
			continue // 只亮肖像／資料卡／場景圖的那一格沒有字
		}
		name := ""
		if x := s.G.General(b.Speaker); x != nil {
			name = i18n.PersonName(x.Name)
		}
		s.note("%s：「%s」", name, b.Text)
	}
	s.Bubbles = nil
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

// runPrefectureTurns 走完這個月剩下的郡順序，玩家的郡不停（`EndMonth` 用：
// 玩家在這之前已經自己下過令）。
func (s *Session) runPrefectureTurns() {
	for s.MonthCursor < len(s.MonthOrder) || len(s.MonthOrder) != 43 {
		if _, stop := s.turnCell(false); stop {
			break
		}
	}
	s.reportOrders()
}

// turnCell 走順序表的一格（`0x15763` 的一輪 ＋ `0x17471`）。
//
// 每一格照 `0x17471`：無主的郡跳過；重算所屬；**玩家的郡不跑分派器**
// ——stopAtHuman 為真時停在那裡（游標不動）回傳那個郡，由玩家下令之後
// `EndTurn` 往下走；為假時跳過（`EndMonth` 那一條）。自治的郡拿 offset 12
// 減一當等級跑同一個分派器（`0x17550`）。
func (s *Session) turnCell(stopAtHuman bool) (at int, stop bool) {
	planner, ok := s.Brain.(ai.PrefecturePlanner)
	if !ok {
		s.MonthCursor = len(s.MonthOrder)
		return 0, false
	}
	if len(s.MonthOrder) != 43 {
		s.shuffleMonth()
	}
	if s.MonthCursor >= len(s.MonthOrder) {
		return 0, false
	}
	at = s.MonthOrder[s.MonthCursor]
	if !s.entered {
		s.entered = true
		// 月迴圈每一格先抽一次（`0x15790`），跳過的格子也算。
		s.G.TurnTick()
		// **回合入口先重整這個郡的守將清單**（`0x17471` 的第一道
		// `call 0x1949e`），兵士與現役將兩欄跟著刷新。
		s.G.RefreshGarrison(at)
		p := s.G.Prefecture(at)
		// **跳過的判斷用重算之前的值**，重算才在後面（`0x17471`：
		// 所屬 == 0xFF → 回 −1；接著才 `call 0x1e394`）。
		if p == nil || !p.Owned() {
			s.nextCell()
			return at, false
		}
		s.G.RecomputeOwners()
	}
	// 重算之後可能已經易主或變無主（別的郡搬空了它、或搬進來的人
	// 槽號較大蓋過原主）——分派器看的是重算之後的那一位。
	p := s.G.Prefecture(at)
	if p == nil || !p.Owned() {
		s.nextCell()
		return at, false
	}
	id, level := p.Owner, s.G.AILevel(p.Owner)
	if s.G.IsHuman(id) {
		// 自治的郡是玩家的地盤，交給電腦按指定的性格經營
		// （`game.AutonomousFor`）；其餘的由那個郡的主人下令。
		lv, auto := s.G.AutonomousFor(at)
		if !auto || p.Commanded {
			if stopAtHuman && !p.Commanded && !auto {
				s.Player = id
				return at, true
			}
			s.nextCell()
			return at, false
		}
		level = lv
	}
	_, n, err := planner.ActPrefecture(s.G, id, at, level)
	s.drainBattles()
	s.G.FinishTurn(at)
	if err != nil {
		s.say("sess.blocked", prefectureName(s.G, at), err)
	}
	if n > 0 && !s.G.IsHuman(id) {
		if s.done == nil {
			s.done = map[state.FactionID]int{}
		}
		s.done[id] += n
	}
	// 電腦打過來、玩家要親自守的那一場（Issue #64）：回合停在這裡，
	// `cmd/san1` 打完叫 `FinishDefence` 再往下走。**游標留在原地**——
	// 這一格已經 `FinishTurn` 過了，推游標的是 `FinishDefence`。
	if s.G.PendingDefence() != nil {
		return at, true
	}
	s.nextCell()
	return at, false
}

// nextCell 把游標推到下一格。
func (s *Session) nextCell() {
	s.MonthCursor++
	s.entered = false
}

// reportOrders 把這個月電腦諸侯下的令按勢力彙總進紀錄。
//
// **紀錄按勢力彙總。** 一郡一行會把紀錄淹掉，而玩家關心的是
// 「這個月哪個諸侯動得多」。
func (s *Session) reportOrders() {
	for _, f := range s.G.Factions() {
		n := s.done[f.ID]
		if n == 0 {
			continue
		}
		name := i18n.Sf("fld.factionN", f.ID)
		if lord := s.G.Lord(f.ID); lord != nil {
			name = i18n.PersonName(lord.Name)
		}
		s.say("sess.orders", name, n)
	}
	s.done = nil
}

// AdvanceToHuman 照原版的順序往下跑（`docs/spec/019` §2）：停在「玩家的郡、
// 這個月還沒下令、不是自治」回傳那個郡，並把 Player 換成那個郡的主人；
// 游標跑完就月底結算、開月、接著跑。maxCells > 0 時最多走那麼多格就回 0
// （0 人的示範模式一幀推一格）。遊戲結束也回 0。
func (s *Session) AdvanceToHuman(maxCells int) int {
	for n := 0; maxCells <= 0 || n < maxCells; n++ {
		if s.Over {
			return 0
		}
		if len(s.MonthOrder) == 43 && s.MonthCursor >= len(s.MonthOrder) {
			s.reportOrders()
			s.finishMonth()
			continue
		}
		if at, stop := s.turnCell(true); stop {
			return at
		}
		if maxCells <= 0 && len(s.G.Players) == 0 {
			// 沒有玩家又沒給上限會永遠跑下去：一次最多一個月。
			if s.MonthCursor >= len(s.MonthOrder) {
				return 0
			}
		}
	}
	return 0
}

// FinishDefence 把玩家守完的那一場搬回局面，游標往下走（Issue #64）。
// 沒有那一場就什麼都不做。
func (s *Session) FinishDefence() {
	if s.G.PendingDefence() == nil {
		return
	}
	if r := s.G.FinishDefence(); r != nil {
		s.note("%s", r.Summary(s.G))
		s.battles = append(s.battles, r)
	}
	s.drainBattles()
	s.nextCell()
}

// EndTurn 結束現在停著的那個玩家郡的回合（下完令或休息），游標往下走。
func (s *Session) EndTurn() {
	if s.MonthCursor < len(s.MonthOrder) {
		s.G.FinishTurn(s.MonthOrder[s.MonthCursor])
		s.nextCell()
	}
}

// Waiting 回停著等玩家下令的那個郡；0 表示沒有停著。
func (s *Session) Waiting() int {
	if !s.entered || s.MonthCursor >= len(s.MonthOrder) {
		return 0
	}
	at := s.MonthOrder[s.MonthCursor]
	p := s.G.Prefecture(at)
	if p == nil || !p.Owned() || !s.G.IsHuman(p.Owner) || p.Commanded {
		return 0
	}
	if _, auto := s.G.AutonomousFor(at); auto {
		return 0
	}
	return at
}

// MaxBattles 是保留幾場戰役的逐日戰報。
const MaxBattles = 8

// EndMonth 讓電腦諸侯行動，然後推進到下個月。
//
// 順序是**先電腦後推進**：玩家已經在這個月下過令了，電腦要在同一個
// 月份裡回應。推進之後才清掉各郡的下令旗標。
func (s *Session) EndMonth() {
	// **這一條是無畫面的路**（測試、批次跑）：沒有人可以指揮守方，
	// 所以「守城」開著時交出來的那一場就地自動打完再往下走
	// （`FinishDefence` → `settle`，還沒打完的在那裡 `Auto()`）。
	// 不接這一段的話 `runPrefectureTurns` 會在交出去那一格 break，
	// 而月份照樣往前推——**剩下的郡整個月沒跑**，而且不會報錯。
	for {
		s.runPrefectureTurns()
		if s.G.PendingDefence() == nil {
			break
		}
		s.FinishDefence()
	}
	s.finishMonth()
}

// finishMonth 是月底結算到開月（游標跑完之後）。
func (s *Session) finishMonth() {
	wasAlive := s.PlayerAlive()
	events := s.G.EndMonth()
	// 開月在結算裡跑完了，順序表換成新的一份。
	s.shuffleMonth()
	s.drainBattles()
	if wasAlive && !s.PlayerAlive() {
		s.collect(s.G.GameOverScene())
		s.say("sess.destroyed")
		s.Over = true
	}
	s.say("sess.month", s.G.Date.Year, s.G.Date.Month)
	s.collect(events)
	// 統一判定在原版是每個月最後一件事，緊接在四季常式之後
	// （`0x1583d`，`docs/re/06` §9）。條件只有「所有有主的郡同屬一方」
	// ——**玉璽不在條件裡**。
	if f, done := s.G.Winner(); done {
		lord := s.G.Lord(f)
		name := i18n.Sf("fld.factionN", f)
		if lord != nil {
			name = i18n.PersonName(lord.Name)
		}
		s.say("sess.unified", name)
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
	if len(s.G.Players) == 0 && s.Player == state.NoFaction {
		return true // 純觀戰
	}
	for _, p := range s.players() {
		if s.G.Lord(p) != nil {
			return true
		}
	}
	return false
}

// players 是全部玩家；舊的單一玩家局面只有 Player。
func (s *Session) players() []state.FactionID {
	if len(s.G.Players) > 0 {
		return s.G.Players
	}
	if s.Player != state.NoFaction {
		return []state.FactionID{s.Player}
	}
	return nil
}

func prefectureName(g *game.State, at int) string {
	if p := g.Prefecture(at); p != nil {
		return i18n.PlaceName(p.Name)
	}
	return i18n.Sf("sess.prefN", at)
}
