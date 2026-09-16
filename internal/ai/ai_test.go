package ai

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func newGame(t *testing.T, f state.FactionID) *game.State {
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
	g, err := game.New(sc, f, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestThreeModes 釘住「三個版本都造得出來，而且分得開」。
func TestThreeModes(t *testing.T) {
	seen := map[Mode]bool{}
	for _, m := range Modes() {
		b, err := New(m)
		if err != nil {
			t.Fatalf("造 %s 失敗：%v", m, err)
		}
		if b.Mode() != m {
			t.Errorf("%s 回報自己是 %s", m, b.Mode())
		}
		if b.Name() == "" {
			t.Errorf("%s 沒有名字", m)
		}
		seen[m] = true
	}
	if len(seen) != 3 {
		t.Errorf("版本有 %d 個，應該是 3 個", len(seen))
	}
	if _, err := New("nope"); err == nil {
		t.Error("不認識的版本沒有報錯")
	}
}

// TestFaithfulModesDoNotPretend 釘住「還原用的 AI 不假裝自己會下棋」。
//
// **這一條擋的是「先填一個差不多的策略進去」。** 一旦填了，
// 之後就再也分不出哪些行為是還原的、哪些是我編的——而兩者的輸出
// 都是合法的命令，看不出差別。
func TestFaithfulModesDoNotPretend(t *testing.T) {
	g := newGame(t, 1) // 曹操
	for _, m := range []Mode{ModeBase, ModePlus} {
		b, err := New(m)
		if err != nil {
			t.Fatal(err)
		}
		// 十八張表都讀過了（`0x5514` 六個等級全是空操作），但已讀的
		// 那幾張底下還有沒量到的量（賞賜的增幅與排序鍵、原版的亂數）。
		// **結構對了不代表數值對了。**
		if b.Derived() {
			t.Errorf("%s 宣稱已經完整還原了——表底下還有沒量到的量", m)
		}
		done, total := b.Coverage()
		if total != 18 {
			t.Errorf("%s 的行為總數是 %d，原版的分派器是十八張表", m, total)
		}
		// **只准發已經解出來的那幾種行為。** 判準不是「不准下命令」——
		// 解出一種就該發一種，否則還原了也用不上；而是「下的命令要在
		// 解出來的那幾種裡面」。
		orders := b.Plan(g, 1)
		if done == 0 && len(orders) != 0 {
			t.Errorf("%s 一種行為都還沒解，卻下了 %d 個命令", m, len(orders))
		}
		for _, o := range orders {
			switch o.(type) {
			case game.ReclaimOrder, game.FloodControlOrder: // 內政（0x5534），已解
			case game.TrainOrder: // 訓練兵士（0x5554），已解
			case game.AppointGovernorOrder: // 指定太守（0x5674），已解
			case game.AppointChiefOrder: // 指定軍師（0x5694），已解
			case game.GiftOrder: // 賞賜物品（0x56b4），已解
			case game.SearchOrder: // 尋訪人才（0x5614），已解
			case game.RecruitOrder: // 登用人才（0x5634），已解
			case game.ArmsOrder: // 購置武器（0x5594），已解
			case game.ConscriptOrder: // 徵兵（0x5574），已解
			case game.RedistributeOrder: // 調整兵力（0x55b4），已解
			case game.ReliefOrder: // 開倉賑民（0x55f4），已解
			case game.RewardOrder: // 賞賜金帛（0x5654），已解
			case game.BuyRiceOrder: // 買入米糧（0x55d4），已解
			// 同一支分派表項目（常式 `0xc634`）是**雙向**的：存糧低於
			// 目標就買、高於目標就賣，所以電腦諸侯下的是 RiceTradeOrder。
			case game.RiceTradeOrder:
			case game.HeadhuntOrder: // 挖角（0x56d4），已解
			case game.PlotOrder: // 計略（0x56f4），已解
			case game.AttackOrder, game.MoveOrder: // 出兵／移防（0x54f4），已解
			default:
				t.Errorf("%s 下了還沒解出來的命令：%T", m, o)
			}
		}
	}
}

// TestEnhancedIsDeterministic 釘住「同一個局面得到同一串命令」。
//
// 帶亂數的 AI 會讓「這一手為什麼不一樣」變成無法回答的問題，
// 而那正是對拍與重現的基礎。
func TestEnhancedIsDeterministic(t *testing.T) {
	b, err := New(ModeEnhanced)
	if err != nil {
		t.Fatal(err)
	}
	a := b.Plan(newGame(t, 5), 5) // 董卓，四個郡
	c := b.Plan(newGame(t, 5), 5)
	if len(a) != len(c) {
		t.Fatalf("兩次規劃的命令數不同：%d vs %d", len(a), len(c))
	}
	g := newGame(t, 5)
	for i := range a {
		if a[i].Describe(g) != c[i].Describe(g) {
			t.Errorf("第 %d 個命令不同：%q vs %q", i+1, a[i].Describe(g), c[i].Describe(g))
		}
	}
	if len(a) == 0 {
		t.Error("董卓有四個郡，強化 AI 卻一個命令都沒下")
	}
}

// TestEnhancedOrdersAreLegal 釘住「AI 產出的命令套得上去」。
//
// AI 產出違規命令是 bug；靜靜跳過會讓那個 bug 變成「AI 這回合比較保守」。
func TestEnhancedOrdersAreLegal(t *testing.T) {
	b, _ := New(ModeEnhanced)
	for _, f := range []state.FactionID{0, 1, 5, 6, 8} {
		g := newGame(t, f)
		orders := b.Plan(g, f)
		n, err := g.ApplyAll(orders, f)
		if err != nil {
			t.Errorf("勢力 %d：套用第 %d 個命令失敗：%v", f, n+1, err)
		}
	}
}

// TestEnhancedRespectsOnePerMonth 釘住「每郡每月一個命令」。
func TestEnhancedRespectsOnePerMonth(t *testing.T) {
	b, _ := New(ModeEnhanced)
	g := newGame(t, 5)
	orders := b.Plan(g, 5)
	seen := map[int]bool{}
	for _, o := range orders {
		if seen[o.Prefecture()] {
			t.Errorf("郡 %d 被下了兩個命令", o.Prefecture())
		}
		seen[o.Prefecture()] = true
	}
	if _, err := g.ApplyAll(orders, 5); err != nil {
		t.Fatal(err)
	}
	// 套用之後再規劃一次，同一個月不該再有命令。
	if again := b.Plan(g, 5); len(again) != 0 {
		t.Errorf("同一個月又規劃出 %d 個命令", len(again))
	}
}

// TestBaseAppointsTheMostCharming 釘住 AI 指的太守是魅力最高的那位。
//
// 原版在指定太守之前把守軍**按魅力由高到低排序**再取第一位
// （`0xf600`，`docs/re/03` §1.4）。**判準是「魅力最高」不是「有指定」**
// ——一個隨便指一位的 AI 在畫面上看起來一模一樣。
func TestBaseAppointsTheMostCharming(t *testing.T) {
	g := newGame(t, 1)
	b, err := New(ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range g.Factions() {
		if !f.Alive {
			continue
		}
		for _, o := range b.Plan(g, f.ID) {
			ap, ok := o.(game.AppointGovernorOrder)
			if !ok {
				continue
			}
			var best *game.General
			for _, x := range g.Garrison(ap.At) {
				if x.Faction == f.ID && (best == nil || x.Charm > best.Charm) {
					best = x
				}
			}
			if best == nil {
				t.Errorf("郡 %d 沒有守將卻指了太守", ap.At)
				continue
			}
			got := g.General(ap.Target)
			if got == nil || got.Charm != best.Charm {
				t.Errorf("郡 %d 指的太守魅力是 %v，該郡最高是 %d",
					ap.At, got, best.Charm)
			}
		}
	}
}

// TestBaseChiefNeedsEightyIntel 釘住軍師的智力門檻。
//
// 原版沒有軍師時門檻是 79（`智 > 79`，也就是說明書的「不得低於 80」）；
// 已經有軍師時門檻是**現任軍師的智**——換人一定要更好。
// **兩個門檻要分開釘**：只驗前者的話，一個「有人就換」的 AI 也會綠。
func TestBaseChiefNeedsEightyIntel(t *testing.T) {
	g := newGame(t, 1)
	b, err := New(ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range g.Factions() {
		if !f.Alive {
			continue
		}
		floor := state.ChiefIntelFloor
		if cur := g.Chief(f.ID); cur != nil {
			floor = int(cur.Intel)
		}
		for _, o := range b.Plan(g, f.ID) {
			ap, ok := o.(game.AppointChiefOrder)
			if !ok {
				continue
			}
			x := g.General(ap.Target)
			if x == nil {
				t.Errorf("勢力 %d 指了不存在的軍師 %d", f.ID, ap.Target)
				continue
			}
			if int(x.Intel) <= floor {
				t.Errorf("勢力 %d 指的軍師智力 %d，門檻是 %d",
					f.ID, x.Intel, floor)
			}
			if x.Status != state.StatusGovernor && x.Status != state.StatusOfficer {
				t.Errorf("勢力 %d 指的軍師身分是 %d，原版只收太守與一般武將",
					f.ID, x.Status)
			}
		}
	}
}

// TestBaseRewardsOnlyAtLevelThree 釘住等級 0–2 完全不賞賜。
//
// 原版的分派表 `0x56b4` **前三格是空操作**（`xor ax,ax; lret`），
// 所以低等級的電腦諸侯根本不做這件事。**這一條只有負面案例驗得到**
// ——一個「總是賞賜」的 AI 在高等級的盤面上看起來一模一樣。
func TestBaseRewardsOnlyAtLevelThree(t *testing.T) {
	b, err := New(ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []int{0, 1, 2, 3, 4, 5} {
		g := newGame(t, 1)
		for i := range g.Factions() {
			g.Factions()[i].AILevel = level
			// 給滿寶庫，排除「沒東西可送」這個混淆。
			for k := range g.Factions()[i].Treasury {
				g.Factions()[i].Treasury[k] = 9
			}
		}
		gifts := 0
		for _, f := range g.Factions() {
			if !f.Alive {
				continue
			}
			for _, o := range b.Plan(g, f.ID) {
				if _, ok := o.(game.GiftOrder); ok {
					gifts++
				}
			}
		}
		if level < 3 && gifts != 0 {
			t.Errorf("等級 %d 賞賜了 %d 次，原版那三格是空操作", level, gifts)
		}
		if level >= 3 && gifts == 0 {
			t.Logf("等級 %d 這一輪沒賞賜（41%% 的機率閘，可能只是沒擲中）", level)
		}
	}
}

// TestActorIsHighestRank 釘住行動者的挑法。
//
// 原版的排序鍵是 `智 + 武 + 加權表[身分]`，而加權表的差是 400 的倍數、
// 智 ＋ 武 最多 200——**權重壓過能力值**，所以身分高的一定排前面。
// 判準要同時驗兩件事：跨身分時身分贏，同身分時智 ＋ 武 贏。
func TestActorIsHighestRank(t *testing.T) {
	g := newGame(t, 1)
	for _, p := range g.Territory(1) {
		a := actor(g, 1, p)
		if a == nil {
			continue
		}
		for _, x := range g.Garrison(p) {
			if x.Faction != 1 || x == a {
				continue
			}
			wa := actorWeight[a.Status]
			wx := actorWeight[x.Status]
			if wx > wa {
				t.Errorf("郡 %d：選了身分 %d（權重 %d），但有身分 %d（權重 %d）",
					p, a.Status, wa, x.Status, wx)
			}
			if wx == wa && int(x.Intel)+int(x.War) > int(a.Intel)+int(a.War) {
				t.Errorf("郡 %d：同身分下選了智+武 %d，但有 %d",
					p, int(a.Intel)+int(a.War), int(x.Intel)+int(x.War))
			}
		}
	}
	// 權重表本身也釘住——它是從記憶體讀出來的，不是推的。
	want := [12]int{2000, 1600, 1200, 800, 2000, 1600, 1200, 800, 0, 0, 400, 0}
	if actorWeight != want {
		t.Errorf("加權表是 %v，原版讀出來是 %v", actorWeight, want)
	}
}

// TestArmsPurchaseFillsToFull 釘住原版買武器的兩件事：**買到滿編**、
// **預算用完就停**（表 `0x5594`，常式 `0xc168`）。
//
// 判準不是「有沒有下購武器的命令」——那個一個門檻寫錯也照樣通過；
// 是**買的量剛好等於缺口**，而缺口是「兵力 − 現有武器數」。
func TestArmsPurchaseFillsToFull(t *testing.T) {
	// **玩家不能是曹操**：這裡要看的是電腦諸侯的行為，而「每郡每月
	// 一道令」只擋玩家（`game.Faction.ByComputer`）。
	const ai = state.FactionID(1) // 曹操
	g := newGame(t, 2)
	terr := g.Territory(ai)
	if len(terr) == 0 {
		t.Fatal("曹操一個郡都沒有")
	}
	p := terr[0]
	pref := g.Prefecture(p)
	pref.Gold = 30000 // 預算不設限，先看「買到滿編」這一半

	orders := armsPurchase(g, g.ActorRoster(p), p, pref.Gold)
	if len(orders) == 0 {
		t.Fatalf("郡 %d 的守軍一個都不缺武器？", p)
	}
	for _, o := range orders {
		a, ok := o.(game.ArmsOrder)
		if !ok {
			t.Fatalf("下了 %T，應該是 game.ArmsOrder", o)
		}
		x := g.General(a.General)
		want := x.Soldiers - game.Weapons(int(x.Arms), x.Soldiers)
		if a.Units != want {
			t.Errorf("將 %d：買了 %d 單位，缺口是 %d", a.General, a.Units, want)
		}
		if err := a.Apply(g, ai); err != nil {
			t.Fatalf("套用失敗：%v", err)
		}
		if x.Arms != 100 {
			t.Errorf("將 %d：補滿之後武裝度是 %d，應該是 100", a.General, x.Arms)
		}
	}

	// **預算用完就停**：把金壓到只夠買 100 單位。
	pref.Gold = 1
	for _, x := range g.Garrison(p) {
		x.Arms = 0
	}
	total := 0
	for _, o := range armsPurchase(g, g.ActorRoster(p), p, pref.Gold) {
		total += o.(game.ArmsOrder).Units
	}
	if total > 1*game.ArmsPerGold {
		t.Errorf("只有 1 金卻買了 %d 單位，上限是 %d", total, game.ArmsPerGold)
	}
}

// TestFaithfulPlansAllApply 釘住「AI 送出的命令套得上去」。
//
// 一道被擋下來會**中斷同一個郡後面全部的命令**，所以一道送錯就少算
// 一整個郡的行動。而少算的結果長得跟「公式不準」一模一樣：對拍那一邊
// 只看得到欄位對不上，看不到有一整串命令根本沒跑。
//
// 走的是**逐郡的執行版**（`ActPrefecture`），也就是 `session` 實際在用
// 的那一條。原版的分派器是循序的：後面的表讀的是前面改過的盤面——
// 米糧買賣把郡裡的米賣掉之後，緊接著的出兵能帶走多少就跟著變。
// 「整個勢力先排完再一次套上」因此本來就對不上，那個模式只有
// `enhanced` 用得到（它每郡只下一道令）。
//
// 這一條不需要原版素材以外的東西，三秒跑完——`internal/parity` 那個
// 三分鐘的對拍不該是第一個發現這件事的地方。
func TestFaithfulPlansAllApply(t *testing.T) {
	g := newGame(t, 0) // 劉備是玩家，其餘全是電腦
	b, err := New(ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	planner, ok := b.(PrefecturePlanner)
	if !ok {
		t.Fatalf("%s 沒有逐郡的執行版", b.Name())
	}
	for _, f := range g.Factions() {
		if !f.Alive || f.ID == g.Player {
			continue
		}
		level := g.AILevel(f.ID)
		for _, at := range g.Territory(f.ID) {
			orders, n, err := planner.ActPrefecture(g, f.ID, at, level)
			if err != nil {
				t.Errorf("勢力 %d 的郡 %d：%d 道命令裡有 %d 道成立，然後：%v",
					f.ID, at, len(orders), n, err)
			}
		}
	}
}

// TestAIBudgetIsAPercentOfGold 釘住本回合預算的係數（`L0`、§2.14）。
//
// **判準是兩張表的比例差**：購置武器只拿 2 %，徵兵拿 30–50 %。
// 只驗其中一張的話，係數表整個接錯位（六個 word 的位移）也看不出來。
func TestAIBudgetIsAPercentOfGold(t *testing.T) {
	for _, c := range []struct{ gold, level, table, want int }{
		{1000, 0, tableArms, 20},
		{1000, 5, tableArms, 20}, // 武器的係數不隨等級變
		{1000, 0, tableConscript, 300},
		{1000, 1, tableConscript, 300},
		{1000, 2, tableConscript, 400},
		{1000, 3, tableConscript, 500},
		{1000, 5, tableConscript, 500},
		{0, 5, tableConscript, 0},
		{1000, 5, 0x54d4, 0}, // 不花錢的表沒有預算
	} {
		if got := aiBudget(c.gold, c.level, c.table); got != c.want {
			t.Errorf("金 %d、等級 %d、表 %#04x：預算 %d，應該是 %d",
				c.gold, c.level, c.table, got, c.want)
		}
	}
}

// TestConscriptFillsToCap 釘住徵兵徵到帶兵上限，並受預算與人口下限夾住。
func TestConscriptFillsToCap(t *testing.T) {
	const ai = state.FactionID(1) // 曹操
	g := newGame(t, 2)
	terr := g.Territory(ai)
	if len(terr) == 0 {
		t.Fatal("曹操一個郡都沒有")
	}
	p := terr[0]
	pref := g.Prefecture(p)

	// 預算與人口都不設限：每一位都該徵到滿編。
	pref.Population = 100000
	for _, o := range conscript(g, g.ActorRoster(p), p, 0, 1000000, nil) {
		c := o.(game.ConscriptOrder)
		x := g.General(c.General)
		if want := x.TroopCap() - x.Soldiers; c.Count != want {
			t.Errorf("將 %d：徵 %d 人，空額是 %d", c.General, c.Count, want)
		}
	}
	// **人口下限**：人口剛好在下限上，一個都徵不到。
	pref.Population = game.MinPopulationToConscript
	if n := conscript(g, g.ActorRoster(p), p, 0, 1000000, nil); len(n) != 0 {
		t.Errorf("人口只剩下限卻還徵了 %d 道", len(n))
	}
	// **預算扣的是花掉的金，不是人數**（`0xc061`）：等級 0 沒有折扣，
	// 每人 1 金，所以總人數還是不超過預算；但**扣完之後預算沒歸零的話
	// 後面的人還徵得到**，所以這裡只釘總量。
	pref.Population = 100000
	total := 0
	for _, o := range conscript(g, g.ActorRoster(p), p, 0, 250, nil) {
		total += o.(game.ConscriptOrder).Count
	}
	if total > 250 {
		t.Errorf("預算 250 金卻徵了 %d 人", total)
	}
	// 等級 5 有 0.75 折：同一份預算徵得到更多人，而且**不會第一位
	// 就吃光**——量到的一輪是 323 → 81 → 21 → 6 → 2 → 1…
	pref.Population = 100000
	got := conscript(g, g.ActorRoster(p), p, 5, 250, nil)
	if len(got) < 2 {
		t.Errorf("等級 5 的 250 金只徵了 %d 道，第一位就把預算吃光了", len(got))
	}
}

// TestSortieGates 釘住出兵的四道門檻與難度係數（表 `0x54f4`，`L0`）。
//
// **等級 3 以下一格都不做**（0–2 是空操作），而四道門檻任何一道不過
// 就整個不做——這一條擋的是「AI 傾巢而出把自己餓死」。
func TestSortieGates(t *testing.T) {
	// 難度係數表：越小越保守。原版難度 10 要帶到守軍的兩倍才動手。
	for _, c := range []struct{ diff, want int }{
		{1, 100}, {2, 90}, {3, 100}, {4, 80}, {5, 80},
		{6, 70}, {7, 70}, {8, 60}, {9, 60}, {10, 50},
	} {
		if got := SortieOdds(state.EditionBase, c.diff); got != c.want {
			t.Errorf("原版難度 %d 的係數是 %d，原版是 %d", c.diff, got, c.want)
		}
	}
	if SortieOdds(state.EditionBase, 0) != 100 || SortieOdds(state.EditionBase, 11) != 100 {
		t.Error("難度越界應該回 100（不加碼也不打折）")
	}
	// 版本空字串當原版——舊存檔沒有這個欄位。
	if SortieOdds("", 10) != SortieOdds(state.EditionBase, 10) {
		t.Error("沒指定版本時應該照原版的表")
	}

	// 等級 0–2 不出兵：把等級調低，命令裡不該出現出兵或移防。
	for lvl := 0; lvl < SortieMinLevel; lvl++ {
		g := newGame(t, 1)
		f := g.Faction(1)
		if f == nil {
			t.Fatal("找不到勢力 1")
		}
		f.AILevel = lvl
		b, err := New(ModeBase)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range b.Plan(g, 1) {
			switch o.(type) {
			case game.AttackOrder, game.MoveOrder:
				t.Errorf("等級 %d 不該出兵，卻下了 %T", lvl, o)
			}
		}
	}
}

// TestLowLevelsSkipFourBehaviours 釘住**等級 3 那道閘門是四種行為共用的**。
//
// 把十八張分派表的 144 個 far pointer 全部解開之後，第 0–2 格指向空函式
// 的有四張：出兵／移防（`0x54f4`）、賞賜物品（`0x56b4`）、挖角
// （`0x56d4`）、計略／破壞（`0x56f4`）。`L0`。
//
// **只擋出兵會讓低等級的電腦諸侯看起來「只是比較不愛打仗」**，
// 而它其實連寶物、挖角、用計都不會做——那在畫面上分不出來。
func TestLowLevelsSkipFourBehaviours(t *testing.T) {
	for lvl := 0; lvl < 3; lvl++ {
		g := newGame(t, 1)
		f := g.Faction(1)
		if f == nil {
			t.Fatal("找不到勢力 1")
		}
		f.AILevel = lvl
		b, err := New(ModeBase)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range b.Plan(g, 1) {
			switch o.(type) {
			case game.AttackOrder, game.MoveOrder, game.GiftOrder,
				game.HeadhuntOrder, game.PlotOrder:
				t.Errorf("等級 %d 不該做這一種，卻下了 %T", lvl, o)
			}
		}
	}
	// 反面：等級 5 至少要做得出其中一種，否則上面那一條在
	// 「AI 什麼都不做」的情況下也會綠。
	seen := false
	for p := 1; p <= 42 && !seen; p++ {
		g := newGame(t, 1)
		for _, fa := range g.Factions() {
			fa.AILevel = 5
		}
		b, err := New(ModeBase)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range b.Plan(g, state.FactionID(p%14+1)) {
			switch o.(type) {
			case game.AttackOrder, game.MoveOrder, game.GiftOrder,
				game.HeadhuntOrder, game.PlotOrder:
				seen = true
			}
		}
	}
	if !seen {
		t.Error("等級 5 一種都做不出來——這個測試測不到想測的東西")
	}
}

// TestCoverageCountsTheNoopTable 釘住十八張表都讀過了。
//
// **`0x5514` 算解出來的**：六個等級的 far pointer 全部指向
// `33 c0 9a 1c 05 c4 05 cb`（配 0 位元組堆疊之後直接 `retf`），
// 那張表在任何等級都不做事——「已解」的正確做法就是不發命令。
// **這一條同時擋住「把沒讀的表算進去」**：Derived() 還是假，
// 因為已讀的那幾張底下還有沒量到的量。
func TestCoverageCountsTheNoopTable(t *testing.T) {
	for _, m := range []Mode{ModeBase, ModePlus} {
		b, err := New(m)
		if err != nil {
			t.Fatal(err)
		}
		done, total := b.Coverage()
		if done != 18 || total != 18 {
			t.Errorf("%s 的覆蓋率是 %d/%d，十八張表都讀過了", m, done, total)
		}
		if b.Derived() {
			t.Errorf("%s 宣稱已經完整還原了——表底下還有沒量到的量", m)
		}
	}
}

// TestPlusDifficultyTable 釘住加強版的難度係數表（`DS:0x5430`，21 格 double，
// `L0`；`docs/spec/004` §3）。
//
// 兩件事各自要成立：
//
//	一、加強版的表與原版**不同**——難度 10 原版 0.5、加強版 0.6
//	二、加強版的 11–20 與 1–10 **逐格相同**
//
// 第二件是這張表最有訊息量的地方：多出來的十級不改變出兵的積極度，
// 所以 11–20 要有意義，改的一定是別的規則。**只驗第一件會讓「兩張表
// 都對」看起來已經測完**，而把 11–20 填成任何數字都照樣綠。
func TestPlusDifficultyTable(t *testing.T) {
	want := []int{100, 90, 90, 80, 70, 75, 70, 75, 70, 60}
	for i, w := range want {
		d := i + 1
		if got := SortieOdds(state.EditionPlus, d); got != w {
			t.Errorf("加強版難度 %d 的係數是 %d，原版讀出來是 %d", d, got, w)
		}
		if got := SortieOdds(state.EditionPlus, d+10); got != w {
			t.Errorf("加強版難度 %d 的係數是 %d，應該與難度 %d 相同（%d）", d+10, got, d, w)
		}
	}
	if SortieOdds(state.EditionPlus, 10) == SortieOdds(state.EditionBase, 10) {
		t.Error("兩版難度 10 的係數不該相同（原版 0.5、加強版 0.6）")
	}
	if SortieOdds(state.EditionPlus, 21) != 100 {
		t.Error("加強版難度 21 越界，應該回 100")
	}
	// 上限跟著版本走。**拿加強版的難度去開原版不是「比較難」，是接錯了。**
	if state.EditionBase.MaxDifficulty() != 10 || state.EditionPlus.MaxDifficulty() != 20 {
		t.Errorf("難度上限是 %d／%d，原版讀出來是 10／20",
			state.EditionBase.MaxDifficulty(), state.EditionPlus.MaxDifficulty())
	}
}

// TestFaithfulAINeedsItsOwnEdition 釘住「還原型 AI 要配同一版的規則」。
//
// **混搭不會報錯，只會安靜地算錯**：`base` 的係數表只有十格，配上加強版
// 收得下的難度 15，出兵判斷讀到的是表外的位元組。所以要擋在開局。
// `enhanced` 是 remake 自己的 AI，不宣稱還原哪一版，兩邊都能跑。
func TestFaithfulAINeedsItsOwnEdition(t *testing.T) {
	for _, c := range []struct {
		m  Mode
		ed state.Edition
		ok bool
	}{
		{ModeBase, state.EditionBase, true},
		{ModePlus, state.EditionPlus, true},
		{ModeBase, state.EditionPlus, false},
		{ModePlus, state.EditionBase, false},
		{ModeEnhanced, state.EditionBase, true},
		{ModeEnhanced, state.EditionPlus, true},
		// 版本空字串（舊存檔）不擋——擋了會讓讀得回來的存檔突然讀不回來。
		{ModePlus, "", true},
	} {
		err := CheckEdition(c.m, c.ed)
		if c.ok && err != nil {
			t.Errorf("%q ＋ %q 應該可以：%v", c.m, c.ed, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%q ＋ %q 應該擋下來", c.m, c.ed)
		}
	}
}

// TestHeadhuntFollowsTheSeason 釘住挖角的預算按季節開關
// （係數表 `DS:0x5714`，`L0`）。
//
// **`es:[0x3f08]` 是季節**（`docs/re/06`），所以那張表的「四個相位」
// 就是四季。某些（等級, 季節）組合的係數是 0——**那不是「機率低」是
// 「完全不做」**，因為預算 0 付不起 100 金的挖角費。
func TestHeadhuntFollowsTheSeason(t *testing.T) {
	want := map[int][4]bool{
		0: {false, false, false, false},
		1: {false, false, false, false},
		2: {false, false, false, false},
		3: {false, false, false, true},
		4: {false, true, false, true},
		5: {false, true, true, true},
	}
	for lvl, seasons := range want {
		for s := 0; s < 4; s++ {
			got := HeadhuntBudget(lvl, s) > 0
			if got != seasons[s] {
				t.Errorf("等級 %d 相位 %d：挖不挖角 ＝ %v，原版是 %v",
					lvl, s, got, seasons[s])
			}
		}
	}
	// 相位 0 任何等級都不挖角——這是表裡唯一一整欄都是 0 的。
	// **相位是「月 mod 4」**，不是季節事件那個 1／4／7／10 的季。
	for lvl := 0; lvl <= 5; lvl++ {
		if HeadhuntBudget(lvl, 0) != 0 {
			t.Errorf("等級 %d 在相位 0 不該挖角", lvl)
		}
	}
}

// TestNextModeOnlyOffersCompatibleOnes 釘住遊戲中切換 AI 只在**跑得動
// 這一版規則**的版本裡繞。
//
// 還原型的 AI 配另一版的規則會安靜地算錯（`CheckEdition`）。選單那一格
// 每按一下就換一個，如果清單裡混進一個不相容的，玩家按到它只會看到
// 一行錯誤訊息——那與「這個功能壞了」在畫面上沒有差別。
func TestNextModeOnlyOffersCompatibleOnes(t *testing.T) {
	for _, ed := range []state.Edition{state.EditionBase, state.EditionPlus} {
		list := ModesFor(ed)
		if len(list) == 0 {
			t.Fatalf("%s 一個 AI 都沒有", ed)
		}
		for _, m := range list {
			if err := CheckEdition(m, ed); err != nil {
				t.Errorf("%s 的清單裡有 %s，但它跑不動：%v", ed, m, err)
			}
		}
		// 繞一圈要回到起點，而且中途每一個都合法。
		start := list[0]
		cur := start
		for i := 0; i < len(list); i++ {
			cur = NextMode(cur, ed)
			if err := CheckEdition(cur, ed); err != nil {
				t.Errorf("%s 繞到 %s，但它跑不動：%v", ed, cur, err)
			}
		}
		if cur != start {
			t.Errorf("%s 繞 %d 次回到 %s，起點是 %s", ed, len(list), cur, start)
		}
		// 認不得的（含空字串，＝「還沒挑過」）要給得出第一個。
		if got := NextMode("", ed); got != list[0] {
			t.Errorf("%s 從空字串繞到 %s，應該是 %s", ed, got, list[0])
		}
	}
	// 強化 AI 兩版都收得下——它不宣稱在還原誰。
	for _, ed := range []state.Edition{state.EditionBase, state.EditionPlus} {
		var found bool
		for _, m := range ModesFor(ed) {
			if m == ModeEnhanced {
				found = true
			}
		}
		if !found {
			t.Errorf("%s 的清單裡沒有 %s", ed, ModeEnhanced)
		}
	}
}

// TestEnhancedNeverGivesAwayAPrefecture 釘住「指定太守」不會把郡送人。
//
// ⚠ **這不是在驗原版錯了。** 原版的候選名單本來就限制在同一個郡
//（`buildRoster` 模式 2：所在郡相同 ＋ 身分 0–3），只是不比對勢力
//（`docs/re/07` §6，七個模式一個都沒有），而主事者換人郡就跟著改所屬
//（`0xd74d`，`L0`）——那一整套是自洽的：郡的所屬每回合由駐軍重算，
// 混編是表得出來的盤面。
//
// 驗的是**強化 AI 自己多加的那一道**：它不想讓郡易主，所以只挑自己人。
// 少了這一道，混編的郡裡魅力最高的剛好是隔壁的人時，這個郡會當場易主
// ——盤面上看起來只是「這個郡突然變色了」。
func TestEnhancedNeverGivesAwayAPrefecture(t *testing.T) {
	g := newGame(t, state.NoFaction)
	e := NewEnhanced(0)
	before := map[int]state.FactionID{}
	for _, p := range g.Prefectures() {
		before[p.ID] = p.Owner
	}
	appointed := 0
	for _, f := range g.Factions() {
		if !f.Alive {
			continue
		}
		for _, o := range e.Plan(g, f.ID) {
			a, ok := o.(game.AppointGovernorOrder)
			if !ok {
				continue
			}
			appointed++
			x := g.General(a.Target)
			if x == nil {
				t.Errorf("郡 %d 指了一個不存在的人 %d", a.At, a.Target)
				continue
			}
			if x.Faction != f.ID {
				t.Errorf("勢力 %d 在郡 %d 指了勢力 %d 的 %s 當太守"+
					"——這個郡會當場易主（強化 AI 應該只挑自己人）",
					f.ID, a.At, x.Faction, x.Name)
			}
			if err := o.Apply(g, f.ID); err != nil {
				t.Errorf("郡 %d 的指定太守套不上去：%v", a.At, err)
				continue
			}
			if got := g.Prefecture(a.At).Owner; got != before[a.At] {
				t.Errorf("郡 %d 指完太守之後從勢力 %d 變成 %d",
					a.At, before[a.At], got)
			}
		}
	}
	t.Logf("開局那一輪指了 %d 次太守", appointed)
}

// TestEnhancedRaisesTheHarvestLevers 釘住強化 AI 真的在動秋收公式裡的量。
//
// 秋收（`game.HarvestGold`／`HarvestRice`，`L0`）只看四個可以操作的量：
// 土地價值、民眾忠誠、洪水率、太守魅力。先前的順序把開墾排在最後一條，
// 而徵兵幾乎永遠有空額可補，於是開墾從來輪不到——跑三十六個月下來
// 平均地力只剩 7。**判準是這四個量裡至少有一個被推上去**，不是
// 「AI 有沒有下命令」：一個只會徵兵的 AI 也一直在下命令。
func TestEnhancedRaisesTheHarvestLevers(t *testing.T) {
	g := newGame(t, state.NoFaction)
	e := NewEnhanced(0)
	kinds := map[string]int{}
	for _, f := range g.Factions() {
		if !f.Alive {
			continue
		}
		for _, o := range e.Plan(g, f.ID) {
			switch o.(type) {
			case game.ReclaimOrder:
				kinds["開墾"]++
			case game.FloodControlOrder:
				kinds["防洪"]++
			case game.ReliefOrder:
				kinds["賑民"]++
			case game.AppointGovernorOrder:
				kinds["指定太守"]++
			}
		}
	}
	total := 0
	for _, n := range kinds {
		total += n
	}
	if total == 0 {
		t.Error("開局那一輪一道內政都沒有——收入那一段沒有接上")
	}
	t.Logf("開局那一輪的內政：%v（共 %d 道）", kinds, total)
}

// TestZZEnhancedCommandMix 跑三十六個月，量強化 AI 的命令分佈。
//
// **判準是「收入那一段走得到」**，用的是命中次數不是「有沒有實作」
//（`CLAUDE.md` §7 第 21 條的同一個判準：一支從來沒被命中的路徑，
// 與沒寫在畫面上長得一樣）。
//
// ⚠ 「指定太守」開局那一輪是 0 次——**那不是沒接上**：劇本出貨的太守
// 本來就已經是郡裡魅力最高的那一位（24 個有主的郡：14 個君主親臨、
// 10 個現任就是最高）。要等到有人搬進來、戰死或被登用進來才換得到，
// 所以這一條只有跑過時間才量得到。
func TestZZEnhancedCommandMix(t *testing.T) {
	g := newGame(t, state.NoFaction)
	e := NewEnhanced(0)
	kinds := map[string]int{}
	for m := 0; m < 36; m++ {
		for _, f := range g.Factions() {
			if !f.Alive {
				continue
			}
			got, _, err := e.Act(g, f.ID)
			if err != nil {
				t.Fatalf("第 %d 月勢力 %d 的命令套不上去：%v", m, f.ID, err)
			}
			for _, o := range got {
				switch o.(type) {
				case game.AppointGovernorOrder:
					kinds["指定太守"]++
				case game.ReclaimOrder:
					kinds["開墾"]++
				case game.FloodControlOrder:
					kinds["防洪"]++
				case game.ReliefOrder:
					kinds["賑民"]++
				case game.ConscriptOrder:
					kinds["徵兵"]++
				case game.AttackOrder:
					kinds["出兵"]++
				case game.TrainOrder:
					kinds["練兵"]++
				case game.RecruitOrder:
					kinds["登用"]++
				case game.SellRiceOrder:
					kinds["賣米"]++
				default:
					kinds["其他"]++
				}
			}
		}
		g.EndMonth()
	}
	t.Logf("三十六個月的命令分佈：%v", kinds)
	for _, k := range []string{"開墾", "指定太守"} {
		if kinds[k] == 0 {
			t.Errorf("三十六個月一次「%s」都沒有——收入那一段走不到", k)
		}
	}
}

// TestPlusTurnConstants 釘住加強版電腦諸侯多出來的幾個數（Issue #26，
// `docs/mechanics/90` §6.5）：行動者開頭的策略值範圍、賞賜金帛的難度門
// 與百分比表、出兵的留守目標係數。
func TestPlusTurnConstants(t *testing.T) {
	// 策略值：((難度−1) mod 10) × 2 + 3——難度 1 與 11 都是 RND(3)、10 與 20 都是 RND(21)。
	for _, c := range []struct{ d, want int }{{1, 3}, {5, 11}, {10, 21}, {11, 3}, {20, 21}} {
		if got := PlusPlanRange(c.d); got != c.want {
			t.Errorf("難度 %d 的策略值範圍 %d，原版是 %d", c.d, got, c.want)
		}
	}
	// 難度門：((難度−1) mod 10 + 1) × 100，賞賜金帛與出兵挑鄰敵共用。
	for _, c := range []struct{ d, want int }{{1, 100}, {5, 500}, {10, 1000}, {11, 100}, {20, 1000}} {
		if got := PlusRewardGate(c.d); got != c.want {
			t.Errorf("難度 %d 的門 %d，原版是 %d", c.d, got, c.want)
		}
	}
	if PlusRewardGateRange != 1010 || PlusSortieRandomRange != 850 {
		t.Errorf("兩個骰子的範圍 %d／%d，原版是 1010／850", PlusRewardGateRange, PlusSortieRandomRange)
	}
	// 賞金的百分比表（DS:0x5502）：1–10 與 11–20 逐格相同。
	want := []int{35, 40, 45, 50, 55, 60, 65, 75, 92, 100}
	for i, w := range want {
		if got := PlusRewardPercent(i + 1); got != w {
			t.Errorf("難度 %d 的賞金百分比 %d，原版是 %d", i+1, got, w)
		}
		if got := PlusRewardPercent(i + 11); got != w {
			t.Errorf("難度 %d 的賞金百分比 %d，該與難度 %d 相同", i+11, got, i+1)
		}
	}
	if PlusRewardPercent(0) != 100 || PlusRewardPercent(21) != 100 {
		t.Error("難度越界該回 100")
	}
	// 留守目標的係數表（DS:0x5956）與策略值 1／2 的 0.95。
	for _, c := range []struct {
		d    int
		want float64
	}{{1, 1.0}, {5, 0.8}, {10, 0.6}, {11, 0.9}, {20, 0.5}} {
		if got := plusSortieKeepCoef[c.d]; got != c.want {
			t.Errorf("難度 %d 的留守係數 %v，原版是 %v", c.d, got, c.want)
		}
	}
	if PlusSortieKeepFactor != 0.95 {
		t.Errorf("策略值 1／2 的留守係數 %v，原版是 0.95", PlusSortieKeepFactor)
	}
}

// TestPlusSortieFollowsThePlan 釘住加強版出兵的類別由策略值決定，
// 不擲 `RND(4)`：策略值 2 走無主鄰郡（移防）、1 走自己的鄰郡。
func TestPlusSortieFollowsThePlan(t *testing.T) {
	g := newGame(t, 1)
	g.Edition = state.EditionPlus
	b, err := New(ModePlus)
	if err != nil {
		t.Fatal(err)
	}
	f := b.(*faithful)
	fa := g.Faction(1)
	if fa == nil {
		t.Fatal("找不到勢力 1")
	}
	fa.AILevel = 5
	// 找一個有無主鄰郡、兵力與錢糧都夠的郡；盤面自己擺。
	var at int
	for _, p := range g.Territory(1) {
		free, _, _ := neighbourLists(g, g.Prefecture(p), 1)
		if len(free) > 0 && len(g.Garrison(p)) >= 3 {
			at = p
			break
		}
	}
	if at == 0 {
		t.Skip("勢力 1 沒有一個郡同時有無主鄰郡與三位守將")
	}
	p := g.Prefecture(at)
	p.Gold, p.Rice = 9999, 99999
	for _, x := range g.Garrison(at) {
		x.Soldiers = 3000
	}
	free, _, _ := neighbourLists(g, p, 1)
	f.plan = 2
	o, ok := f.sortiePlus(g, at, 1)
	if !ok {
		t.Fatal("策略值 2、有無主鄰郡，該出兵（移防）")
	}
	r, isMove := o.(game.RelocateOrder)
	if !isMove {
		t.Fatalf("策略值 2 該是移防，得到 %T", o)
	}
	hit := false
	for _, n := range free {
		if n == r.To {
			hit = true
		}
	}
	if !hit {
		t.Errorf("移防目標 %d 不在無主鄰郡 %v 裡", r.To, free)
	}
}
