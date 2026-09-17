package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func newSession(t *testing.T, mode ai.Mode, player state.FactionID) *Session {
	t.Helper()
	g, err := game.New(scenarioOne(t), player, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ai.New(mode)
	if err != nil {
		t.Fatal(err)
	}
	return New(g, b, player)
}

// scenarioOne 讀原版劇本一；沒有素材就 skip。
func scenarioOne(t *testing.T) *state.Scenario {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過需要原版素材的測試")
	}
	base := filepath.Join(root, "三國演義", "DATA2")
	rd := func(ext string) []byte {
		b, err := os.ReadFile(base + ext)
		if err != nil {
			t.Fatalf("讀 %s%s：%v", base, ext, err)
		}
		return b
	}
	c, err := assets.OpenContainer(rd(".NAM"), rd(".IDX"), rd(".GRP"))
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(c, state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

// TestTwelveMonths 跑一整年，確認迴圈轉得動而且局面有變化。
func TestTwelveMonths(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, 0) // 劉備
	start := s.G.Date
	before := *s.G.Prefecture(15) // 洛陽，董卓的地盤，由電腦經營
	troops := s.G.Soldiers(15)
	for i := 0; i < 12; i++ {
		s.EndMonth()
	}
	if s.G.Date.Year != start.Year+1 || s.G.Date.Month != start.Month {
		t.Errorf("跑十二個月之後是 %v，起點是 %v", s.G.Date, start)
	}
	after := *s.G.Prefecture(15)
	if after.Gold == before.Gold && after.LandValue == before.LandValue &&
		after.FloodRate == before.FloodRate && s.G.Soldiers(15) == troops {
		t.Error("洛陽跑了一年完全沒變——電腦諸侯沒有在做事")
	}
}

// TestFaithfulModeSaysSo 釘住「還沒還原的 AI 要在紀錄裡講出來」。
func TestFaithfulModeSaysSo(t *testing.T) {
	s := newSession(t, ai.ModeBase, 0)
	found := false
	for _, line := range s.Log {
		if strings.Contains(line, "還原到") && strings.Contains(line, "種行為") {
			found = true
		}
	}
	if !found {
		t.Error("用還沒還原完的 AI 開局，紀錄裡沒有把還原到幾種講出來")
	}
	// **不能用「局面沒變」當判準**——季節事件本來就會改變局面。
	//
	// 也不能用「有沒有下命令」：十八種行為解出一種之後，`base` 就會發那
	// 一種（內政）。要問的是**紀錄有沒有把「還沒還原完」講出來**，
	// 那件事在上面已經驗過了。這裡只確認它下的命令沒有暴衝——
	// 一種行為每個郡最多一道。
	for i := 0; i < 6; i++ {
		s.EndMonth()
	}
	n := 0
	for _, line := range s.Log {
		if strings.Contains(line, "下了") && strings.Contains(line, "個命令") {
			n++
		}
	}
	if n > 6*16 {
		t.Errorf("六個月裡有 %d 筆下令紀錄，比「每個勢力每月一批」多太多", n)
	}
}

// TestPlayerOrderLogged 釘住玩家的命令會進紀錄，失敗也會。
func TestPlayerOrderLogged(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, 0)
	if err := s.Do(game.ReclaimOrder{At: 8}); err != nil {
		t.Fatalf("對自己的郡開墾失敗：%v", err)
	}
	if last := s.Log[len(s.Log)-1]; !strings.Contains(last, "開墾") {
		t.Errorf("紀錄最後一行是 %q，應該提到開墾", last)
	}
	if err := s.Do(game.ReclaimOrder{At: 11}); err == nil {
		t.Error("對曹操的郡開墾竟然成功")
	}
	if last := s.Log[len(s.Log)-1]; !strings.HasPrefix(last, "✗") {
		t.Errorf("失敗的命令沒有記成失敗：%q", last)
	}
}

// TestLogIsBounded 釘住紀錄不會無限長大。
func TestLogIsBounded(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, 0)
	s.MaxLog = 10
	for i := 0; i < 60; i++ {
		s.EndMonth()
	}
	if len(s.Log) > 10 {
		t.Errorf("紀錄有 %d 行，上限是 10", len(s.Log))
	}
}

// TestAutonomyActuallyRuns 釘住授權自治的郡真的會有動作。
//
// **自治不是一個設定欄位**：原版的郡回合入口看到州郡 offset 12 不是 0
// 就把那個郡交給電腦跑（`0x17550`）。只存型態不接上分派器的話，
// 「郡縣自冶」這道指令在畫面上會完全沒有效果——而那看起來像
// 「這個月剛好沒事發生」。
func TestAutonomyActuallyRuns(t *testing.T) {
	s := newSession(t, ai.ModeBase, 5) // 董卓
	lord := s.G.Lord(5)
	at := 0
	for _, n := range s.G.Territory(5) {
		if n != lord.Location {
			at = n
			break
		}
	}
	if at == 0 {
		t.Fatal("董卓只有一個郡")
	}
	if err := s.G.SetAutonomy(at, game.AutoCivil, 5); err != nil {
		t.Fatal(err)
	}
	p := s.G.Prefecture(at)
	p.Gold = game.MaxGold
	// 讓它有事可做：地力與洪水率都留出空間。
	p.LandValue, p.FloodRate = 20, 80
	land, flood, gold := p.LandValue, p.FloodRate, p.Gold
	moved := false
	for i := 0; i < 12 && !moved; i++ {
		s.EndMonth()
		moved = p.LandValue != land || p.FloodRate != flood || p.Gold != gold
	}
	if !moved {
		t.Errorf("授權自治十二個月，%s 的地力／洪水率／庫銀一動也沒動", p.Name)
	}
}

// newPlayersSession 開一局多位玩家（劇本一、難度 5、原版規則）。
func newPlayersSession(t *testing.T, players []state.FactionID) *Session {
	t.Helper()
	g, err := game.NewPlayers(scenarioOne(t), players, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ai.New(ai.ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	first := state.FactionID(state.NoFaction)
	if len(players) > 0 {
		first = players[0]
	}
	return New(g, b, first)
}

// TestAdvanceToHumanTakesTurnsByPrefecture 釘住多人輪流的形狀（`docs/spec/019`
// §2）：照這個月的郡順序停在玩家的郡、Player 換成那個郡的主人、每郡每月
// 最多停一次、停的順序就是順序表的順序，兩位玩家都輪得到。
func TestAdvanceToHumanTakesTurnsByPrefecture(t *testing.T) {
	players := []state.FactionID{0, 1}
	s := newPlayersSession(t, players)
	month := s.G.Date
	seen := map[int]bool{}
	who := map[state.FactionID]int{}
	last := -1
	for s.G.Date == month {
		at := s.AdvanceToHuman(0)
		if at == 0 {
			t.Fatal("有玩家的局面卻沒停在任何郡")
		}
		if s.G.Date != month {
			break // 跑過月底才停在下個月的郡
		}
		p := s.G.Prefecture(at)
		if p.Owner != s.Player || !s.G.IsHuman(p.Owner) {
			t.Fatalf("停在郡 %d（主人 %d），Player 是 %d", at, p.Owner, s.Player)
		}
		if s.Waiting() != at {
			t.Fatalf("停在郡 %d，Waiting 回 %d", at, s.Waiting())
		}
		if seen[at] {
			t.Fatalf("郡 %d 同一個月停了兩次", at)
		}
		if s.MonthCursor <= last {
			t.Fatalf("游標從 %d 退回 %d", last, s.MonthCursor)
		}
		if s.MonthOrder[s.MonthCursor] != at {
			t.Fatalf("游標那一格是郡 %d，停的是 %d", s.MonthOrder[s.MonthCursor], at)
		}
		seen[at], last = true, s.MonthCursor
		who[p.Owner]++
		s.EndTurn()
		if s.Waiting() != 0 {
			t.Fatal("EndTurn 之後還停著")
		}
	}
	for _, f := range players {
		if who[f] == 0 {
			t.Errorf("勢力 %d 這個月一次都沒輪到：%v", f, who)
		}
	}
	t.Logf("%v 的月份停了 %d 個郡：%v", month, len(seen), who)
}

// TestDemoAdvancesOneCellAtATime 釘住 0 人：沒有玩家的郡可停，一次一格，
// 44 次之內跨過月底。
func TestDemoAdvancesOneCellAtATime(t *testing.T) {
	s := newPlayersSession(t, nil)
	month := s.G.Date
	for i := 0; i < 45 && s.G.Date == month; i++ {
		if at := s.AdvanceToHuman(1); at != 0 {
			t.Fatalf("示範模式停在郡 %d", at)
		}
		if s.G.Date == month && s.MonthCursor != i+1 {
			t.Fatalf("第 %d 次之後游標在 %d", i+1, s.MonthCursor)
		}
	}
	if s.G.Date == month {
		t.Fatal("45 格還沒跨過月底")
	}
}
