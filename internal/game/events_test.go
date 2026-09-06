package game

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestSeasons 釘住月份與季節的對應。
func TestSeasons(t *testing.T) {
	for m, want := range map[int]Season{1: Winter, 3: Spring, 6: Summer, 9: Autumn, 12: Winter} {
		if got := (Date{Year: 200, Month: m}).Season(); got != want {
			t.Errorf("%d 月是 %v，應該是 %v", m, got, want)
		}
	}
}

// TestAutumnHarvest 釘住秋收：米糧進倉、稅金入庫、土地價值略降
//（說明書 p.37、p.21）。
func TestAutumnHarvest(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(15) // 洛陽，人口最多
	g.Date = Date{Year: 189, Month: 8}
	rice, gold, land := p.Rice, p.Gold, p.LandValue
	events := g.EndMonth() // → 9 月，秋收
	if g.Date.Month != 9 {
		t.Fatalf("推到 %d 月", g.Date.Month)
	}
	if p.Rice <= rice && p.Gold <= gold {
		t.Errorf("秋收之後米 %d（原 %d）金 %d（原 %d）——都沒有增加",
			p.Rice, rice, p.Gold, gold)
	}
	if p.LandValue >= land {
		t.Errorf("秋收之後土地價值 %d，原本 %d——應該略降", p.LandValue, land)
	}
	if len(events) == 0 {
		t.Error("秋收沒有產生任何事件訊息")
	}
}

// TestHarvestOncePerYear 釘住秋收一年一次。
func TestHarvestOncePerYear(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(15)
	g.Date = Date{Year: 189, Month: 9}
	before := p.Rice
	g.EndMonth() // → 10 月，還是秋天但不是秋收月
	if p.Rice != before {
		t.Errorf("十月又收了一次：米從 %d 變成 %d", before, p.Rice)
	}
}

// TestAgingKillsEventually 釘住「體能逐年降到 0 時將領死亡」（說明書 p.36）。
func TestAgingKillsEventually(t *testing.T) {
	g := newGame(t)
	var old *General
	for _, x := range g.Garrison(15) {
		if x.Faction == 5 {
			old = x
			break
		}
	}
	if old == nil {
		t.Skip("洛陽沒有守將")
	}
	old.Stamina = 1
	old.Status = state.StatusOfficer
	g.Date = Date{Year: 189, Month: 2}
	g.EndMonth() // → 3 月，春天
	if old.Employed() {
		t.Errorf("%s 體能只剩 1，過了一年還活著（體能 %d）", old.Name, old.Stamina)
	}
}

// TestWinterGrowsPopulation 釘住冬季人口增加（說明書 p.37）。
func TestWinterGrowsPopulation(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(15)
	g.Date = Date{Year: 189, Month: 11}
	before := p.Population
	g.EndMonth() // → 12 月，冬季
	if p.Population <= before {
		t.Errorf("冬季人口 %d，原本 %d——應該增加", p.Population, before)
	}
}

// TestTributeYearly 釘住進貢：領地越多貢品越多（說明書 p.37）。
func TestTributeYearly(t *testing.T) {
	g := newGame(t)
	f := g.Faction(5) // 董卓四個郡
	before := 0
	for _, n := range f.Treasury {
		before += n
	}
	g.Date = Date{Year: 189, Month: 11}
	g.EndMonth() // → 12 月
	after := 0
	for _, n := range f.Treasury {
		after += n
	}
	if after <= before {
		t.Errorf("董卓有四個郡，年底貢品卻沒有增加（%d → %d）", before, after)
	}
	if f.Treasury[TreasureSeal] > 0 {
		t.Error("玉璽不該由進貢產生——它是勝利條件")
	}
}

// TestWinnerNeedsSeal 釘住勝利條件（說明書 p.37）。
func TestWinnerNeedsSeal(t *testing.T) {
	g := newGame(t)
	for i := range g.prefectures {
		if g.prefectures[i].Owned() {
			g.prefectures[i].Owner = 0
		}
	}
	f, seal, done := g.Winner()
	if !done || f != 0 {
		t.Fatalf("全部歸劉備之後 Winner 回 %v／%v", f, done)
	}
	if seal {
		t.Error("還沒拿到玉璽卻說贏了")
	}
	g.Faction(0).Treasury[TreasureSeal] = 1
	if _, seal, _ = g.Winner(); !seal {
		t.Error("拿到玉璽卻還是沒贏")
	}
}

// TestNewBloodAppears 釘住春天的「新血出現」（說明書 p.36）。
//
// **沒有這一段的話武將只死不生。** 實測：不補新血的話四十七年後
// 十四個勢力全部滅亡，天下無主——那不是難度高，是少了一條規則。
func TestNewBloodAppears(t *testing.T) {
	g := newGame(t)
	unborn := 0
	for i := range g.generals {
		if g.generals[i].Status == state.StatusUnborn {
			unborn++
		}
	}
	if unborn == 0 {
		t.Fatal("劇本 001 應該有未登場的人物")
	}
	// 跑到諸葛亮那一輩該出頭的年份。
	for g.Date.Year < 215 {
		g.EndMonth()
	}
	left := 0
	for i := range g.generals {
		if g.generals[i].Status == state.StatusUnborn {
			left++
		}
	}
	if left >= unborn {
		t.Errorf("跑到 %d 年還有 %d 位未登場（原本 %d）——新血沒有出現",
			g.Date.Year, left, unborn)
	}
	// 出身郡要對得上：登場的人應該在自己的出身郡。
	for i := range g.generals {
		x := &g.generals[i]
		if x.Status == state.StatusAvailable && x.Origin >= 1 && x.Location != x.Origin {
			continue // 登場後可能被登用而移動，只檢查沒被動過的
		}
	}
}

// TestZhugeLiangAppears 釘住一個具體的人：諸葛亮在劇本 001 是八歲，
// 到了二十歲該露面。這比「有人登場」硬——**它會抓到出身郡讀錯**。
func TestZhugeLiangAppears(t *testing.T) {
	g := newGame(t)
	var zgl *General
	for i := range g.generals {
		if g.generals[i].Name == "諸葛亮" {
			zgl = &g.generals[i]
		}
	}
	if zgl == nil {
		t.Skip("劇本 001 找不到諸葛亮")
	}
	if zgl.Status != state.StatusUnborn {
		t.Fatalf("諸葛亮開局的身分是 %d，應該是未登場", zgl.Status)
	}
	born := zgl.Age
	for g.Date.Year < 189+int(TuneComingOfAge-born)+2 {
		g.EndMonth()
	}
	if zgl.Status == state.StatusUnborn {
		t.Errorf("跑到 %d 年（諸葛亮 %d 歲）還沒登場", g.Date.Year, zgl.Age)
	}
	if zgl.Location < 1 || zgl.Location > state.PrefectureCount {
		t.Errorf("諸葛亮登場在郡 %d，越界了", zgl.Location)
	}
}
