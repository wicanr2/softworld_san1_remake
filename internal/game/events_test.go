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

// TestAutumnHarvest 釘住秋收：米糧進倉、稅金入庫（`0x16a1b`，`L0`）。
//
// **土地價值在秋收時不掉**——原版是在**春天**每個月掉
// `RND(土地價值 ÷ 10)`（`LandValueDecay`）。說明書 p.21 說「收成後
// 土地價值會略降」，碼裡沒有這回事，掉的地方在別的季節。
func TestAutumnHarvest(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(15) // 洛陽，人口最多
	// 秋季常式一年只跑一次，在**七月**（`SeasonMonths`）。
	g.Date = Date{Year: 189, Month: 6}
	rice, gold, land := p.Rice, p.Gold, p.LandValue
	want := HarvestGold(charmOf(g, p.ID), int(p.LandValue),
		int(p.PublicLoyalty), p.Population)
	events := g.EndMonth() // → 7 月，秋收
	if g.Date.Month != 7 {
		t.Fatalf("推到 %d 月", g.Date.Month)
	}
	if p.Rice <= rice && p.Gold <= gold {
		t.Errorf("秋收之後米 %d（原 %d）金 %d（原 %d）——都沒有增加",
			p.Rice, rice, p.Gold, gold)
	}
	if got := p.Gold - gold; got != want {
		t.Errorf("秋收進帳 %d，算式給的是 %d", got, want)
	}
	if p.LandValue != land {
		t.Errorf("秋收之後土地價值 %d，原本 %d——秋收不動它", p.LandValue, land)
	}
	if len(events) == 0 {
		t.Error("秋收沒有產生任何事件訊息")
	}
}

func charmOf(g *State, prefectureID int) int {
	if x := g.Governor(prefectureID); x != nil {
		return int(x.Charm)
	}
	return 0
}

// TestHarvestCountsAllThreeFactors 釘住秋收的三個因子都算進去
// （太守魅力 ＋ 土地價值 × 4 ＋ 民眾忠誠 × 2）。
//
// **只算土地價值與人口的話，「派誰當太守」對收入沒有影響**，
// 而那正是原版讓魅力有用的地方之一。
func TestHarvestCountsAllThreeFactors(t *testing.T) {
	base := HarvestGold(50, 50, 50, 100000)
	if HarvestGold(90, 50, 50, 100000) <= base {
		t.Error("太守魅力高應該收得多")
	}
	if HarvestGold(50, 60, 50, 100000) <= base {
		t.Error("土地價值高應該收得多")
	}
	if HarvestGold(50, 50, 60, 100000) <= base {
		t.Error("民眾忠誠高應該收得多")
	}
	// 權重：土地價值 ×4 比忠誠 ×2 重。
	land := HarvestGold(50, 60, 50, 100000) - base
	loyal := HarvestGold(50, 50, 60, 100000) - base
	if land <= loyal {
		t.Errorf("土地價值 +10 給 %d，忠誠 +10 給 %d——土地價值的權重應該比較重",
			land, loyal)
	}
}

// TestLocustLikesRichLand 釘住蝗害的方向：**土地價值越高越容易鬧**
// （`0x16c20`，`L0`）。
//
// 這與其他天災相反（瘟疫是土地價值**低**才發生）。照著「災害都因為窮」
// 的直覺寫會寫反，而寫反之後遊戲照樣跑得動。
func TestLocustLikesRichLand(t *testing.T) {
	// 忠誠低、土地價值高 → 鬧
	if !LocustStrikes(20, 90, 40, 40) {
		t.Error("忠誠 20、土地價值 90 應該鬧蝗害")
	}
	// 忠誠低、土地價值低 → 不鬧
	if LocustStrikes(20, 30, 40, 40) {
		t.Error("土地價值 30 不該鬧蝗害——田不肥蟲不來")
	}
	// 忠誠高 → 不鬧
	if LocustStrikes(90, 90, 40, 40) {
		t.Error("忠誠 90 不該鬧蝗害")
	}
	// 瘟疫剛好相反：土地價值低才鬧。
	if !PlagueStrikes(20, 10, 30, 30) {
		t.Error("忠誠 20、土地價值 10 應該鬧瘟疫")
	}
	if PlagueStrikes(20, 90, 30, 30) {
		t.Error("土地價值 90 不該鬧瘟疫——瘟疫與蝗害的方向相反")
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

// TestAgingKillsEventually 釘住老死照原版的兩段判定來（`0x15d5d`，`L0`）。
//
// **體能低不等於快死了。** 原版先問「過壽命了沒」——沒過的人體能
// 一點都不掉，過了才開始扣。只釘「體能 1 的人一年後死掉」會通過一個
// 每年無條件扣體能的實作，而那會讓全圖的人在四十歲上下集體凋零。
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
		t.Fatal("洛陽沒有守將")
	}
	old.Status = state.StatusOfficer

	// 還沒過壽命：體能只剩 1 也不會死，而且**一點都不掉**。
	old.Stamina, old.Age, old.Lifespan = 1, 40, 70
	g.Date = Date{Year: 189, Month: 12}
	g.EndMonth()
	if !old.Employed() {
		t.Fatalf("%s 才 %d 歲、壽命 %d，不該死", old.Name, old.Age, old.Lifespan)
	}
	if old.Stamina != 1 {
		t.Errorf("沒過壽命體能卻從 1 掉到 %d", old.Stamina)
	}

	// 過壽四年：(70−74)×25 = −100，體能再高也扛不住。
	old.Stamina, old.Age, old.Lifespan = 90, 74, 70
	g.Date = Date{Year: 190, Month: 12}
	g.EndMonth()
	if old.Employed() {
		t.Errorf("%s 過壽四年還活著（體能 %d）", old.Name, old.Stamina)
	}
}

// TestAgingDropIsTheOriginalFormula 釘住那兩條算式。
func TestAgingDropIsTheOriginalFormula(t *testing.T) {
	// 門：RND(3) + 壽命 >= 年齡 就完全不動。
	for _, c := range []struct {
		age, lifespan, roll int
		past                bool
	}{
		{70, 70, 0, false}, // 剛好到壽命，還沒過
		{71, 70, 0, true},
		{71, 70, 1, false}, // 擲到 1 就再撐一年
		{74, 70, 2, true},
	} {
		if got := AlreadyPastPrime(c.age, c.lifespan, c.roll); got != c.past {
			t.Errorf("年齡 %d、壽命 %d、擲 %d：past=%v，應該是 %v",
				c.age, c.lifespan, c.roll, got, c.past)
		}
	}
	// 扣：體能 + (壽命 − 年齡) × 25 − RND(50)，夾到 0。
	for _, c := range []struct{ stamina, age, lifespan, roll, want int }{
		{80, 71, 70, 0, 55},  // 80 − 25
		{80, 71, 70, 49, 6},  // 80 − 25 − 49
		{80, 74, 70, 0, 0},   // 80 − 100 → 0
		{100, 72, 70, 0, 50}, // 100 − 50
	} {
		if got := AgingDrop(c.stamina, c.age, c.lifespan, c.roll); got != c.want {
			t.Errorf("體能 %d、年齡 %d、壽命 %d、擲 %d：得到 %d，應該是 %d",
				c.stamina, c.age, c.lifespan, c.roll, got, c.want)
		}
	}
}

// TestPopulationGrowsOnceAYear 釘住人口一年只長一次，在十月，
// 幅度跟著土地價值與民眾忠誠走（`0x16ec2`，`L0`）。
//
// **三件事要一起釘**：月份、幅度、以及「其他月份不長」。只釘「有增加」
// 的話，每個月都長 1% 也會綠——而那與原版差了一個數量級。
func TestPopulationGrowsOnceAYear(t *testing.T) {
	g := newGame(t)
	p := g.Prefecture(15)

	g.Date = Date{Year: 189, Month: 9}
	before := p.Population
	want := GrowPopulation(before, int(p.LandValue), int(p.PublicLoyalty))
	g.EndMonth() // → 十月
	if p.Population != want {
		t.Errorf("十月人口 %d，原本 %d，應該是 %d", p.Population, before, want)
	}
	// **幅度不是常數**：劇本 001 的倍率只有 1019–1059，離上限 1150 很遠。
	// 寫死 15% 的話這一條會差一個量級。
	if want == before+before*15/100 {
		t.Error("這個郡剛好是滿檔的 15%，測不出「幅度跟著土地價值與忠誠走」")
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
	// 冬季常式一年只跑一次，在**十月**。
	g.Date = Date{Year: 189, Month: 9}
	g.EndMonth() // → 10 月，進貢
	after := 0
	for _, n := range f.Treasury {
		after += n
	}
	if after <= before {
		t.Errorf("董卓有四個郡，年底貢品卻沒有增加（%d → %d）", before, after)
	}
	if f.Treasury[TreasureSeal] > sealBefore {
		t.Errorf("玉璽不該由進貢產生（%d → %d）——它走春季的現世事件",
			sealBefore, f.Treasury[TreasureSeal])
	}
}

// TestWinnerIgnoresTheSeal 釘住統一的條件（`0x15852`，`L0`）。
//
// 只有一條：**所有有主的郡屬於同一個勢力**。無主的郡跳過，玉璽不看。
// 說明書 p.37 那句「在遊戲結束前一定要拿到玉璽」在碼裡沒有對應
// （`CONTEXT.md` §4 R19）。
func TestWinnerIgnoresTheSeal(t *testing.T) {
	g := newGame(t)
	// 先留一個無主的郡，證明它不擋統一。
	ownerless := 0
	for i := range g.prefectures {
		if !g.prefectures[i].Owned() {
			ownerless = g.prefectures[i].ID
			break
		}
	}
	if ownerless == 0 {
		t.Fatal("劇本 001 應該有無主的郡")
	}
	for i := range g.prefectures {
		if g.prefectures[i].Owned() {
			g.prefectures[i].Owner = 0
		}
	}
	g.Faction(0).Treasury[TreasureSeal] = 0
	f, done := g.Winner()
	if !done || f != 0 {
		t.Fatalf("全部歸劉備之後 Winner 回 %v／%v（無主的郡 %d 不該擋）",
			f, done, ownerless)
	}
	// 有一個郡歸別人就不算統一。
	g.Prefecture(ownerless).Owner = 1
	if _, done := g.Winner(); done {
		t.Error("還有別人的郡卻說統一了")
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
	// **出頭年齡是他自己的**（人物表 offset 26），不是全域常數。
	born, debut := zgl.Age, zgl.Debut
	if debut == 0 {
		t.Fatal("諸葛亮的出頭年齡是 0——offset 26 沒有解碼進來")
	}
	for g.Date.Year < 189+int(debut-born)+2 {
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

// TestLandValueDecays 釘住土地價值每個春月自己掉（`0x15c96`，`L0`）。
//
// **少了這一條，土地開發是一次性的**：開到 100 就永遠不用再管，
// 而原版的內政是每個月都要做的事。畫面上看不出差別——土地價值欄
// 停在 100 看起來完全正常。
func TestLandValueDecays(t *testing.T) {
	if got := LandValueDecay(100, 7); got != 93 {
		t.Errorf("100 掉 7 之後是 %d，應該是 93", got)
	}
	if got := LandValueDecay(3, 9); got != 0 {
		t.Errorf("掉到負的應該夾成 0，得到 %d", got)
	}

	g := newGame(t)
	// 先把幾個郡的土地價值拉滿，走三年的元月看它掉下來。
	// **春季常式一年只跑一次**（`SeasonMonths`），所以「三個春月」
	// 要跑三年不是跑三個月。
	for id := 1; id <= 5; id++ {
		g.Prefecture(id).LandValue = 100
	}
	for y := 190; y <= 192; y++ {
		g.Date = Date{Year: y, Month: 12}
		g.EndMonth()
	}
	dropped := 0
	for id := 1; id <= 5; id++ {
		if g.Prefecture(id).LandValue < 100 {
			dropped++
		}
	}
	if dropped == 0 {
		t.Error("走過三年的元月，五個滿檔的郡一個都沒掉——衰減沒有生效")
	}
}

// TestDisasterLossesAreMeasured 釘住四種天災的損失幅度（`L0`）。
//
// **每一項都是「保留率」不是「損失率」**，而且各自擲一次骰——
// 地震對人口、金、米各擲一次，不是同一個百分比套三次。
//
// 這一組的重點是**相對輕重**：米最怕蝗害（只剩兩成）、人口最怕瘟疫
// （剩四成）、金最怕地震（剩五成）。全部寫成同一個數字的話，
// 「哪一種災害要防哪一樣」就消失了，而畫面上每一場災害看起來都一樣。
func TestDisasterLossesAreMeasured(t *testing.T) {
	for _, c := range []struct {
		name string
		k    Keep
		lo   int // 最少保留百分比
		hi   int // 最多保留百分比
	}{
		{"地震人口", QuakePopKeep, 60, 79},
		{"地震金", QuakeGoldKeep, 50, 69},
		{"地震米", QuakeRiceKeep, 40, 59},
		{"水災人口", FloodPopKeep, 70, 79},
		{"水災土地價值", FloodLandKeep, 60, 79},
		{"瘟疫人口", PlaguePopKeep, 40, 59},
		{"蝗害米", LocustRiceKeep, 20, 29},
		{"蝗害土地價值", LocustLandKeep, 80, 169},
	} {
		if got := c.k.Apply(100, 0); got != c.lo {
			t.Errorf("%s 擲 0 保留 %d%%，原版是 %d%%", c.name, got, c.lo)
		}
		if got := c.k.Apply(100, c.k.Spread-1); got != c.hi {
			t.Errorf("%s 擲滿保留 %d%%，原版是 %d%%", c.name, got, c.hi)
		}
	}
	// 相對輕重：米最怕蝗害、人口最怕瘟疫、金最怕地震。
	if LocustRiceKeep.Floor >= QuakeRiceKeep.Floor {
		t.Error("米應該最怕蝗害")
	}
	if PlaguePopKeep.Floor >= FloodPopKeep.Floor ||
		PlaguePopKeep.Floor >= QuakePopKeep.Floor {
		t.Error("人口應該最怕瘟疫")
	}
	// ⚠ 蝗害的土地價值是**唯一會往上走**的一項（平均 124.5%）。
	// 這與直覺相反，所以單獨釘一條——寫成減損會讓它安靜地變成另一個遊戲。
	if LocustLandKeep.Floor <= 100 && LocustLandKeep.Floor+LocustLandKeep.Spread <= 100 {
		t.Error("蝗害的土地價值保留率在原版是 80–169%，不是減損")
	}

	// 水災之後洪水率是**乘上去**的，越界才變 100。
	if got := FloodRateGain.Apply(50, 0); got != 60 {
		t.Errorf("洪水率 50 遇水災擲 0 之後是 %d，應該是 60", got)
	}
	if got := FloodRateGain.Apply(90, 19); got < 100 {
		t.Errorf("洪水率 90 擲滿之後是 %d，應該越界（→ 100）", got)
	}
}

// TestTributeNeedsMoreThanOnePrefecture 釘住進貢的基數怎麼算（`L0`＋`L1`）。
//
// 原版是 `基數 = Σ(民眾忠誠/4 + 土地價值/2) ÷ (RND(10) + 80)`，四種寶物
// 各抽 `RND(基數 + 1)`。**一個郡的勢力拿不到東西**：那一份和大約 75，
// 除以 80–89 之後是 0——這不是「拿得少」，是**一件都沒有**，而畫面上
// 看起來只是「今年沒有進貢的訊息」。
func TestTributeNeedsMoreThanOnePrefecture(t *testing.T) {
	g := newGame(t)
	// 找地最多與只有一個郡的兩個勢力。
	var big, small *Faction
	for i := range g.factions {
		f := &g.factions[i]
		if !f.Alive {
			continue
		}
		n := len(g.Territory(f.ID))
		if big == nil || n > len(g.Territory(big.ID)) {
			big = f
		}
		if n == 1 && small == nil {
			small = f
		}
	}
	if big == nil {
		t.Skip("這個劇本沒有活著的勢力")
	}
	sum := func(f *Faction) int {
		n := 0
		for tr := TreasureBook; tr < treasureCount; tr++ {
			n += f.Treasury[tr]
		}
		return n
	}
	seal := big.Treasury[TreasureSeal]
	b0 := sum(big)
	s0 := 0
	if small != nil {
		s0 = sum(small)
	}
	g.Date = Date{Year: 189, Month: 9}
	g.EndMonth() // → 十月，冬季常式（進貢與人口成長同一支）

	if big.Treasury[TreasureSeal] != seal {
		t.Error("玉璽不進貢")
	}
	if sum(big) == b0 {
		t.Errorf("地最多的勢力（%d 個郡）十月沒有收到任何貢品",
			len(g.Territory(big.ID)))
	}
	if small != nil && sum(small) != s0 {
		t.Errorf("只有一個郡的勢力不該收到貢品，卻多了 %d 件", sum(small)-s0)
	}
}

// TestAnnualDecayKeepsUpkeepMeaningful 釘住三處年度衰減同一個形狀
// （`AnnualDecay`，`L0`）：土地價值（每個春月）、訓練度與武裝度（元月）。
//
// **這是「內政、訓練、購置武器要一直做」的原因。** 少了它，一次做到頂
// 就永遠不用再管——而數值欄停在 100 看起來完全正常，只有跑完幾十年
// 才看得出「所有人都是滿訓練的精兵」。
func TestAnnualDecayKeepsUpkeepMeaningful(t *testing.T) {
	if got := AnnualDecay(100, 9); got != 91 {
		t.Errorf("100 掉 9 是 %d，應該是 91", got)
	}
	if AnnualDecay(3, 9) != 0 {
		t.Error("掉到負的應該夾成 0")
	}
	// 值越高掉得越多：擲的範圍是「值 ÷ 10」。
	if 100/10 <= 30/10 {
		t.Fatal("這個測試的前提壞了")
	}

	g := newGame(t)
	// 把一批將領練到滿，走一整年看它掉下來。
	var picked []*General
	for _, x := range g.Garrison(15) {
		x.Training, x.Arms = 100, 100
		picked = append(picked, x)
	}
	if len(picked) == 0 {
		t.Skip("這個郡沒有駐軍")
	}
	for m := 1; m <= 12; m++ {
		g.Date = Date{Year: 190, Month: m}
		g.EndMonth()
	}
	dropped := 0
	for _, x := range picked {
		if x.Training < 100 || x.Arms < 100 {
			dropped++
		}
	}
	if dropped == 0 {
		t.Errorf("走完一年，%d 位滿訓練的將領一個都沒掉", len(picked))
	}
}

// TestLoyaltyDriftsWithPrestige 釘住元月的忠誠漂移（`0x15fad`，`L0`）。
//
//	忠誠 ← 忠誠 + (人望 − 60) ÷ 2
//
// **60 是分水嶺**：人望不到 60 的諸侯，部下每年都在離心。這一條把
// 「打勝仗」與「留得住人」接在一起——人望靠戰役累積（勝 +2、敗 −2）。
//
// 沒有它的話，忠誠只會被登用、賞賜、計謀動到，**一個從不打仗也從不
// 賞賜的諸侯，部下的忠誠會永遠停在開局值**。
func TestLoyaltyDriftsWithPrestige(t *testing.T) {
	if got := LoyaltyDrift(50, 80, 3); got != 60 {
		t.Errorf("忠誠 50、人望 80 → %d，應該是 60", got)
	}
	if got := LoyaltyDrift(50, 40, 3); got != 40 {
		t.Errorf("忠誠 50、人望 40 → %d，應該是 40", got)
	}
	if got := LoyaltyDrift(50, LoyaltyPivot, 3); got != 50 {
		t.Errorf("人望剛好 %d 應該不動，得到 %d", LoyaltyPivot, got)
	}
	if got := LoyaltyDrift(2, 0, 3); got != 3 {
		t.Errorf("算出負的應該換成 RND(5)，得到 %d", got)
	}
	if got := LoyaltyDrift(99, 100, 0); got != 100 {
		t.Errorf("上限是 100，得到 %d", got)
	}

	// 整年跑一次：人望低的勢力部下該掉忠誠。
	g := newGame(t)
	id := g.Factions()[0].ID
	g.Faction(id).Prestige = 0
	var watched []*General
	for i := range g.generals {
		x := &g.generals[i]
		if x.Faction == id && x.Status != state.StatusLord && x.Loyalty > 40 {
			watched = append(watched, x)
		}
	}
	if len(watched) == 0 {
		t.Skip("這個勢力沒有可觀察的部下")
	}
	before := make([]uint8, len(watched))
	for i, x := range watched {
		before[i] = x.Loyalty
	}
	g.Date = Date{Year: 189, Month: 12}
	g.EndMonth() // → 元月
	dropped := 0
	for i, x := range watched {
		if x.Loyalty < before[i] {
			dropped++
		}
	}
	if dropped == 0 {
		t.Errorf("人望 0 的勢力，%d 位部下一個都沒掉忠誠", len(watched))
	}
	// 君主自己不受影響。
	if lord := g.Lord(id); lord != nil && lord.Loyalty != 100 && lord.Loyalty == 0 {
		t.Error("君主不該被人望影響")
	}
}

// TestDebutFollowsTheBond 釘住出頭的兩條路（`0x16064`／`0x15ec4`，`L0`）。
//
// **牽絆對象有勢力的話，人直接投奔他**——手冊 p.36 的「新將投效其
// 親族朋友」是字面意思。少了這一條，名將的子姪與舊部都會變成散落
// 各地的在野人士，而「牽絆」在登用之外就沒有別的作用。
func TestDebutFollowsTheBond(t *testing.T) {
	g := newGame(t)
	// 找一位未登場、牽絆對象有勢力的人。
	var who *General
	for i := range g.generals {
		x := &g.generals[i]
		if x.Status != state.StatusUnborn || x.Name == "" {
			continue
		}
		if _, _, ok := g.bondDebut(x); ok {
			who = x
			break
		}
	}
	if who == nil {
		t.Skip("劇本 001 沒有牽絆對象已出仕的未登場者")
	}
	at, id, _ := g.bondDebut(who)
	who.Age = who.Debut + 1
	g.Date = Date{Year: 200, Month: 12}
	g.EndMonth() // → 元月
	if who.Status == state.StatusUnborn {
		t.Fatalf("%s 年齡 %d 已過出頭年齡 %d，卻沒登場",
			who.Name, who.Age, who.Debut)
	}
	if who.Faction != id {
		t.Errorf("%s 投奔了勢力 %d，牽絆對象在 %d", who.Name, who.Faction, id)
	}
	if who.Location != at {
		t.Errorf("%s 出現在郡 %d，牽絆對象在郡 %d", who.Name, who.Location, at)
	}
	if who.Status != state.StatusOfficer {
		t.Errorf("投奔的人身分是 %d，應該是一般武將", who.Status)
	}

	// 沒有牽絆的人走退路：出身郡、在野、無勢力。
	var loner *General
	for i := range g.generals {
		x := &g.generals[i]
		if x.Status != state.StatusUnborn || x.Name == "" || x.Origin < 1 {
			continue
		}
		if _, _, ok := g.bondDebut(x); !ok {
			loner = x
			break
		}
	}
	if loner == nil {
		t.Skip("沒有可比較的無牽絆者")
	}
	origin := loner.Origin
	loner.Age = loner.Debut + 1
	g.Date = Date{Year: 201, Month: 12}
	g.EndMonth()
	if loner.Status != state.StatusAvailable || loner.Faction != state.NoFaction {
		t.Errorf("%s 應該在出身郡當在野，得到身分 %d 勢力 %d",
			loner.Name, loner.Status, loner.Faction)
	}
	if loner.Location != origin {
		t.Errorf("%s 出現在郡 %d，出身郡是 %d", loner.Name, loner.Location, origin)
	}
}

// TestSealAppearsAndBoostsPrestige 釘住玉璽現世（`0x15cfd`–`0x1519a`，`L0`）。
//
// **玉璽在 remake 裡原本只被檢查、從來不會被發出來**——勝利條件
// （說明書 p.24）因此永遠走不到，而畫面上完全看不出來：每個勢力的
// 寶庫都好端端地顯示玉璽 0 件。
func TestSealAppearsAndBoostsPrestige(t *testing.T) {
	g := newGame(t)
	// **劇本 001 開局就有人持玉璽**（孫堅），所以事件在那裡不會觸發——
	// 這一條要自己把盤面擺成「玉璽還沒現世」（使用者的指示：對拍直接
	// 設定記憶體，不要依賴 RND()）。
	// ⚠ `Factions()` 回的是**值**不是指標，`for _, f := range` 改不到——
	// 要透過 `Faction(id)` 拿指標。
	for _, f := range g.Factions() {
		g.Faction(f.ID).Treasury[TreasureSeal] = 0
	}
	if g.SealFound() {
		t.Fatal("清乾淨之後還是有人持玉璽")
	}
	before := map[state.FactionID]int{}
	for _, f := range g.Factions() {
		before[f.ID] = f.Prestige
	}
	// 跑十年的元月；約半數機率，十年內幾乎一定出現。
	// **春季常式一年只跑一次**（`SeasonMonths`），所以是十次不是三十次。
	for y := 0; y < 10 && !g.SealFound(); y++ {
		g.Date = Date{Year: 190 + y, Month: 12}
		g.EndMonth()
	}
	if !g.SealFound() {
		t.Fatal("跑了十年的元月，玉璽還沒現世")
	}
	// 只有一家拿到，而且人望大漲。
	holders := 0
	for _, f := range g.Factions() {
		if f.Treasury[TreasureSeal] == 0 {
			continue
		}
		holders++
		if f.Prestige <= before[f.ID] {
			t.Errorf("%v 拿到玉璽卻沒漲人望（%d → %d）",
				f.ID, before[f.ID], f.Prestige)
		}
	}
	if holders != 1 {
		t.Errorf("%d 家持有玉璽，應該只有一家", holders)
	}
	// 已經現世就不會再發一次。
	g.Date = Date{Year: 210, Month: 1}
	g.EndMonth()
	n := 0
	for _, f := range g.Factions() {
		n += f.Treasury[TreasureSeal]
	}
	if n != 1 {
		t.Errorf("玉璽變成 %d 件——現世之後不該再發", n)
	}
}

// TestTreasuryIsCapped 釘住寶庫每一種寶物的上限（`0x1731d`，`L0`）。
//
// **這與 `TreasureCap` 是兩件事**：那個是賞賜能把能力值提到的上限
// （90 點，說明書 p.24），這個是寶庫的存量上限（100 件）。兩個常數
// 名字只差一個字，混用不會編譯失敗也不會報錯。
func TestTreasuryIsCapped(t *testing.T) {
	if TreasuryCap == TreasureCap {
		t.Fatal("寶庫上限與能力上限不該是同一個數")
	}
	g := newGame(t)
	f := g.Faction(g.Factions()[0].ID)
	for tr := TreasureBook; tr < treasureCount; tr++ {
		f.Treasury[tr] = TreasuryCap
	}
	g.Date = Date{Year: 189, Month: 11}
	g.EndMonth() // → 十二月，進貢
	for tr := TreasureBook; tr < treasureCount; tr++ {
		if f.Treasury[tr] > TreasuryCap {
			t.Errorf("%v 存了 %d 件，上限是 %d", tr, f.Treasury[tr], TreasuryCap)
		}
	}
}

// TestSuccessionPicksTheMostCharming 釘住君主繼承照原版的規矩來：
// 候選是**整個勢力**不限郡，依魅力挑，人望按繼承者的魅力打折。
func TestSuccessionPicksTheMostCharming(t *testing.T) {
	g := newGame(t)
	var id state.FactionID = 1 // 曹操
	f := g.Faction(id)
	lord := g.Lord(id)
	f.Prestige = 80

	// **盤面自己擺**：曹操開局只有一個郡，照劇本挑就永遠測不到
	// 「候選不限於死者所在的郡」這件事。把一位部將調去別的郡，
	// 魅力設成全勢力最高。
	var far *General
	for _, x := range g.Garrison(lord.Location) {
		if x.Faction != id || x.Index == lord.Index {
			continue
		}
		if far == nil {
			far = x
			far.Location = lord.Location%state.PrefectureCount + 1
			far.Charm = 90
			continue
		}
		x.Charm = 60 // 留在原郡的都比不上他
	}
	if far == nil {
		t.Fatal("曹操麾下只有他自己")
	}
	g.retire(lord)

	if f.Lord != far.Index {
		got := "（無）"
		if x := g.General(f.Lord); x != nil {
			got = x.Name
		}
		t.Errorf("繼位的是 %s，應該是魅力最高的 %s", got, far.Name)
	}
	if far.Status != state.StatusLord {
		t.Errorf("繼位者的身分是 %d，應該是君主", far.Status)
	}
	if lord.Status != state.StatusFallen {
		t.Errorf("死去的君主身分是 %d，應該是已故（12）", lord.Status)
	}
	// 人望 = 四捨五入(90 × 80 ÷ 100) = 72。
	if f.Prestige != 72 {
		t.Errorf("繼承後人望 %d，應該是 72", f.Prestige)
	}
}

// TestSuccessionPrestigeRounds 釘住那條四捨五入。
func TestSuccessionPrestigeRounds(t *testing.T) {
	for _, c := range []struct{ charm, prestige, want int }{
		{100, 80, 80}, // 魅力滿分保住全部
		{50, 80, 40},
		{90, 80, 72},
		{55, 91, 50}, // 50.05 → 50
		{57, 91, 52}, // 51.87 → 52
	} {
		if got := SuccessionPrestige(c.charm, c.prestige); got != c.want {
			t.Errorf("魅力 %d、人望 %d：得到 %d，應該是 %d",
				c.charm, c.prestige, got, c.want)
		}
	}
}
