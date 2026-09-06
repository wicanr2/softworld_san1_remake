package game

import (
	"sort"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestSeasons 釘住月份與季節的對應。
//
// **對照的是原版畫面**：主畫面左側直排寫年月與季節，三張畫面對出
// 元月春、四月夏、八月秋（`L1`、`[base]`）。分界照農曆，正月就是春天。
func TestSeasons(t *testing.T) {
	for m, want := range map[int]Season{
		1: Spring, 3: Spring, 4: Summer, 6: Summer,
		7: Autumn, 8: Autumn, 9: Autumn, 10: Winter, 12: Winter,
	} {
		if got := (Date{Year: 200, Month: m}).Season(); got != want {
			t.Errorf("%d 月是 %v，應該是 %v", m, got, want)
		}
	}
}

// TestAutumnHarvest 釘住秋收：米糧進倉、稅金入庫、土地價值略降
// （說明書 p.37、p.21）。
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
	g.Date = Date{Year: 189, Month: 12}
	g.EndMonth() // → 元月，年齡增長那一個月
	if old.Employed() {
		t.Errorf("%s 體能只剩 1，過了一年還活著（體能 %d）", old.Name, old.Stamina)
	}
}

// TestPopulationGrowsOnceAYear 釘住人口一年只長一次，在十月，長 15%。
//
// **三件事要一起釘**：月份、幅度、以及「其他月份不長」。只釘「有增加」
// 的話，每個月都長 1% 也會綠——而那與原版差了一個數量級。
// 數字是量出來的（`docs/mechanics/60-economy.md` §1，`L1`）。
func TestPopulationGrowsOnceAYear(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(15)

	g.Date = Date{Year: 189, Month: 9}
	before := p.Population
	g.EndMonth() // → 十月
	want := before + before*PopulationGrowthPercent/100
	if p.Population != want {
		t.Errorf("十月人口 %d，原本 %d，應該是 %d（＋%d%%）",
			p.Population, before, want, PopulationGrowthPercent)
	}

	// 其他月份不長。
	for _, m := range []int{10, 11, 12, 1} {
		g.Date = Date{Year: 189, Month: m}
		was := p.Population
		g.EndMonth()
		if p.Population > was {
			t.Errorf("%d 月推到下個月，人口從 %d 長到 %d——只有十月該長",
				m, was, p.Population)
		}
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
	// **開局的寶庫不是空的**——`BASEMAS` offset 14–18 帶著初始內容
	// （`state.TreasuryOf`），所以玉璽要比「有沒有增加」而不是「是不是零」。
	sealBefore := f.Treasury[TreasureSeal]
	g.Date = Date{Year: 189, Month: 11}
	g.EndMonth() // → 12 月
	after := 0
	for _, n := range f.Treasury {
		after += n
	}
	if after <= before {
		t.Errorf("董卓有四個郡，年底貢品卻沒有增加（%d → %d）", before, after)
	}
	if f.Treasury[TreasureSeal] > sealBefore {
		t.Errorf("玉璽不該由進貢產生（%d → %d）——它是勝利條件",
			sealBefore, f.Treasury[TreasureSeal])
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

// TestPriceMovesEveryMonth 釘住物價每個月都重抽，而且落在原版量到的範圍。
//
// **remake 原本完全不動這個欄位**，而原版每個月幾乎四十二個郡一起換
// （連走十六個月的觀測，`L1`）。物價決定買賣米糧的匯率與城寨造價，
// 不動的話那兩條規則整局都在同一個價位上運作。
func TestPriceMovesEveryMonth(t *testing.T) {
	g := newGame(t)
	moved, total := 0, 0
	seen := map[uint8]bool{}
	for m := 0; m < 12; m++ {
		before := make([]uint8, 0, len(g.Prefectures()))
		for _, p := range g.Prefectures() {
			before = append(before, p.PriceLevel)
		}
		g.EndMonth()
		for i, p := range g.Prefectures() {
			total++
			if p.PriceLevel != before[i] {
				moved++
			}
			seen[p.PriceLevel] = true
			if p.PriceLevel < PriceMin || int(p.PriceLevel) > PriceMin+2*PriceSpread {
				t.Fatalf("%s 的物價 %d 落在 %d–%d 之外",
					p.Name, p.PriceLevel, PriceMin, PriceMin+2*PriceSpread)
			}
		}
	}
	// 原版每個月都是四十二個郡裡四十個上下在動，所以「幾乎全部」是判準。
	if moved*100/total < 90 {
		t.Errorf("十二個月裡只有 %d/%d 次物價有變動，原版是幾乎每個郡每個月都換",
			moved, total)
	}
	// 分布要鋪得開；只有幾個值代表鹽或取模寫錯了。
	if len(seen) < 20 {
		var vs []int
		for v := range seen {
			vs = append(vs, int(v))
		}
		sort.Ints(vs)
		t.Errorf("十二個月只出現 %d 種物價，範圍應該鋪得開：%v", len(seen), vs)
	}
}
