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
	g, err := game.New(sc, f, 5)
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
		// 九張表的判斷式都讀出來了，但 AI 的回合預算、賞賜的增幅與
		// 排序鍵、原版的亂數都還沒量到。**結構對了不代表數值對了。**
		if b.Derived() {
			t.Errorf("%s 宣稱已經完整還原了——表底下還有沒量到的量", m)
		}
		done, total := b.Coverage()
		if total != 9 {
			t.Errorf("%s 的行為總數是 %d，原版的分派器是九張表", m, total)
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

	orders := armsPurchase(g, p, pref.Gold)
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
	for _, o := range armsPurchase(g, p, pref.Gold) {
		total += o.(game.ArmsOrder).Units
	}
	if total > 1*game.ArmsPerGold {
		t.Errorf("只有 1 金卻買了 %d 單位，上限是 %d", total, game.ArmsPerGold)
	}
}

// TestFaithfulPlansAllApply 釘住「AI 送出的命令套得上去」。
//
// `ApplyAll` 遇到擋下來的命令會**中斷同一輪後面全部的命令**，所以一道
// 送錯就少算一整個勢力的行動。而少算的結果長得跟「公式不準」一模一樣：
// 對拍那一邊只看得到欄位對不上，看不到有一整串命令根本沒跑。
//
// 這一條不需要原版素材以外的東西，三秒跑完——`internal/parity` 那個
// 三分鐘的對拍不該是第一個發現這件事的地方。
func TestFaithfulPlansAllApply(t *testing.T) {
	g := newGame(t, 0) // 劉備是玩家，其餘全是電腦
	b, err := New(ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range g.Factions() {
		if !f.Alive || f.ID == g.Player {
			continue
		}
		orders := b.Plan(g, f.ID)
		if n, err := g.ApplyAll(orders, f.ID); err != nil {
			t.Errorf("勢力 %d 的 %d 道命令裡有 %d 道成立，然後：%v",
				f.ID, len(orders), n, err)
		}
	}
}
