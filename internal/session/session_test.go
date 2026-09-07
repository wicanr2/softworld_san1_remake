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
	g, err := game.New(sc, player, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ai.New(mode)
	if err != nil {
		t.Fatal(err)
	}
	return New(g, b, player)
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
