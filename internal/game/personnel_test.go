package game

import (
	"errors"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestSearchThreshold 釘住尋訪的門檻與命中的效果。
//
// 原版的常式（`0xcc86`）掃全部 350 人，找**所在郡是本郡且身分 9
// （在野未露面）**的；尋訪者的智要**大於 `RND(Spread)+Floor`** 才算
// 成功，成功的話那個人身分 9 → 8、勢力設成 `0xFF`。
//
// **三個常數都隨 AI 等級變**（`SearchTierFor`，`L1`）：等級 0 的門檻是
// 30–94，等級 5 是 15–34；出手的機率也從 20 % 升到 50 %。
// 說明書只說「謀略越高成功機率越大」。
func TestSearchThreshold(t *testing.T) {
	for _, c := range []struct{ level, bar, spread, floor int }{
		{0, 7, 65, 30}, {1, 7, 65, 30}, {2, 7, 65, 30},
		{3, 6, 45, 30}, {4, 5, 20, 30}, {5, 4, 20, 15},
	} {
		got := SearchTierFor(c.level)
		if got.Bar != c.bar || got.Spread != c.spread || got.Floor != c.floor {
			t.Errorf("等級 %d 的尋訪常數是 %+v，應該是 (%d, %d, %d)",
				c.level, got, c.bar, c.spread, c.floor)
		}
	}
	// 越界要夾住，不能索引出界。
	if SearchTierFor(-1) != SearchTierFor(0) || SearchTierFor(99) != SearchTierFor(5) {
		t.Error("等級越界沒有夾住")
	}
}

// TestSearchRevealsTheLastHiddenOne 釘住郡裡站著兩位以上在野未露面的人
// 時露面的是**槽號最大的那一位**（`0xcc9b`–`0xcce5`，`L0`、`[both]`）：
// 原版掃完 350 人不 break，最後一位相符的留下。取第一位在四月的月度
// 對拍量到郡 20 露面 187、原版露面 248，登用進來的人跟著錯。
func TestSearchRevealsTheLastHiddenOne(t *testing.T) {
	g := newGame(t)
	at := 0
	for _, p := range g.Prefectures() {
		if p.Owned() && p.Owner == 0 {
			at = p.ID
			break
		}
	}
	if at == 0 {
		t.Fatal("劉備一個郡都沒有")
	}
	// 局面自己擺：郡裡先清掉原本在野未露面的人，再放兩位進去。
	for i := range g.generals {
		x := &g.generals[i]
		if x.Location == at && x.Status == state.StatusIdle {
			x.Location = 0
		}
	}
	lo, hi := g.General(200), g.General(300)
	for _, x := range []*General{lo, hi} {
		if x == nil || x.Name == "" {
			t.Fatal("找不到人物 200／300")
		}
		x.Location, x.Status, x.Faction = at, state.StatusIdle, state.NoFaction
	}
	// 尋訪者：郡裡智最高的在職者，門檻壓到一定過（等級 0 的底 30，
	// 智 99 一定大於 30–94）。
	var by *General
	for _, x := range g.Garrison(at) {
		if by == nil || x.Intel > by.Intel {
			by = x
		}
	}
	if by == nil {
		t.Fatal("郡裡沒有在職的人")
	}
	by.Intel = 99
	found, err := g.search(at, by.Index, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.Index != hi.Index {
		t.Fatalf("露面的是 %v，應該是槽號大的 %d", found, hi.Index)
	}
	if hi.Status != state.StatusAvailable || lo.Status != state.StatusIdle {
		t.Errorf("身分：300 ＝ %d（應為 8）、200 ＝ %d（應為 9）", hi.Status, lo.Status)
	}
}

// TestRecruitBondGate 釘住登用的牽絆閘門（`0xce8c`，`L0`）。
//
// **三條路要各驗一次**：牽絆對象效力於招募方（一定成功）、在野
// （照能力值判定）、效力於第三方（幾乎不可能）。只驗成功那一條的話，
// 閘門整個沒接上也照樣綠——在野是多數情形，能力值那條會蓋過去。
func TestRecruitBondGate(t *testing.T) {
	// 說服力與難度的兩半各自先釘住，再看閘門怎麼蓋過難度。
	if got := RecruitPersuasion(50, 90, 0); got != 80 {
		t.Errorf("人望 50、太守魅力 90、無加成算出 %d，應該是 (50*3+90)/3 ＝ 80", got)
	}
	if got := RecruitPersuasion(50, 90, 40); got != 120 {
		t.Errorf("等級 5 的加成沒算進去：%d", got)
	}
	if got := RecruitDifficulty(90, 80, 0, 0); got != 56 {
		t.Errorf("謀略 90、戰力 80、兩次 RND 都是 0 算出 %d，應該是 30+26 ＝ 56", got)
	}
	if got := RecruitDifficulty(90, 80, 3, 3); got != 28 {
		t.Errorf("兩次 RND 都是 3 算出 %d，應該是 15+13 ＝ 28", got)
	}
	// 費用與加成成對：等級越高越便宜也越容易。
	for lvl, want := range map[int][2]int{
		0: {30, 0}, 1: {30, 0}, 2: {30, 0},
		3: {20, 10}, 4: {10, 20}, 5: {0, 40},
	} {
		if fee, bonus := RecruitFee(lvl), RecruitBonus(lvl); fee != want[0] || bonus != want[1] {
			t.Errorf("等級 %d 的參數是 (%d, %d)，量到的是 %v", lvl, fee, bonus, want)
		}
	}

	g := newGame(t)
	// **局面自己擺，不去盤面上找。** 開局有沒有剛好符合條件的在野人才
	// 是資料的事；用 skip 帶過就等於這一條測試在多數環境下不存在。
	at := 0
	for _, p := range g.Prefectures() {
		if p.Owned() && p.Owner == 0 {
			at = p.ID
			break
		}
	}
	if at == 0 {
		t.Fatal("劉備一個郡都沒有")
	}
	target := g.General(200)
	if target == nil || target.Name == "" {
		t.Fatal("找不到人物 200")
	}
	target.Faction = state.NoFaction
	target.Location = at
	target.Status = state.StatusAvailable
	target.Intel, target.War = 90, 80

	// **牽絆對象效力於第三方**：難度 160 以上，說服力最多一百出頭。
	rival := g.General(1)
	if rival == nil {
		t.Fatal("找不到人物 1")
	}
	rival.Faction, rival.Status = 1, state.StatusOfficer
	target.Bond = rival.Index
	if err := g.Recruit(at, target.Index, 0); !errors.Is(err, ErrDeclined) {
		t.Errorf("牽絆對象在敵營，登用卻回 %v，應該是 ErrDeclined", err)
	}
	if target.Employed() {
		t.Error("牽絆對象在敵營，人卻來了")
	}

	// **牽絆對象效力於招募方**：難度 0，一定成功。
	g.Prefecture(at).Commanded = false
	rival.Faction = 0
	activeBefore := g.ActiveGenerals(at)
	troopsBefore := g.Troops(at)
	if err := g.Recruit(at, target.Index, 0); err != nil {
		t.Fatalf("牽絆對象在自己麾下，登用卻失敗：%v", err)
	}
	if target.Faction != 0 || target.Status != state.StatusOfficer {
		t.Errorf("登用成功了但欄位沒改：勢力 %d、身分 %d", target.Faction, target.Status)
	}
	if target.Loyalty == 0 || target.Loyalty > 100 {
		t.Errorf("新進的忠誠是 %d，應該落在 1..100", target.Loyalty)
	}
	if got := g.StoredActiveGenerals(at); got != activeBefore+1 {
		t.Errorf("登用後現役將快照是 %d，應該是 %d", got, activeBefore+1)
	}
	if got := g.Troops(at); got != troopsBefore+target.Soldiers/100 {
		t.Errorf("登用後兵士快照是 %d，應該是 %d", got, troopsBefore+target.Soldiers/100)
	}
}

// TestHeadhuntGates 釘住挖角的兩道閘門（`L0`、`0xe0bc`／`0x1dc0a`）。
//
// **忠誠門檻與牽絆閘門要各驗一次**：只驗忠誠的話，牽絆整個沒接上
// 也照樣綠——多數人的 Bond 指向自己，那條路不會走到。
func TestHeadhuntGates(t *testing.T) {
	for _, c := range []struct{ roll, want int }{{0, 80}, {7, 87}, {14, 94}} {
		if got := HeadhuntLoyaltyBar(c.roll); got != c.want {
			t.Errorf("RND(15) ＝ %d：門檻 %d，應該是 %d", c.roll, got, c.want)
		}
	}

	g := newGame(t)
	at := 0
	for _, p := range g.Prefectures() {
		if p.Owned() && p.Owner == 0 {
			at = p.ID
			break
		}
	}
	if at == 0 {
		t.Fatal("劉備一個郡都沒有")
	}
	// 造一位敵方將領：忠誠低、牽絆指向自己 → 挖得動。
	x := g.General(200)
	if x == nil || x.Name == "" {
		t.Fatal("找不到人物 200")
	}
	x.Faction, x.Status, x.Loyalty, x.Bond = 1, state.StatusOfficer, 50, x.Index
	if !g.Headhuntable(x, at) {
		t.Error("忠誠 50、無牽絆的敵將應該挖得動")
	}
	// **忠誠閘門**：忠誠 95 高於任何一次的門檻（上限 94）。
	x.Loyalty = 95
	if g.Headhuntable(x, at) {
		t.Error("忠誠 95 超過門檻上限 94，不該挖得動")
	}
	// **牽絆閘門**：牽絆對象與他同一勢力 → 挖不動，跟忠誠無關。
	x.Loyalty = 1
	b := g.General(201)
	if b == nil {
		t.Fatal("找不到人物 201")
	}
	b.Faction, b.Status = 1, state.StatusOfficer
	x.Bond = b.Index
	if g.Headhuntable(x, at) {
		t.Error("牽絆對象還在他自己陣營，不該挖得動")
	}
	// 牽絆對象跳槽到別處就挖得動了。
	b.Faction = 2
	if !g.Headhuntable(x, at) {
		t.Error("牽絆對象已經不在他陣營，應該挖得動")
	}
}

// TestPlayerSearchQueuesTheScreens 釘住玩家尋訪之後排進 pending 的畫面
// （`0x1bb58`–`0x1bc45`）：找到人是「亮肖像 (488,88)」＋「尋訪者在下格
// 報名士的名字」兩格；沒找到只有尋訪者報「沒有找到人才」一格；電腦那一條
// 什麼都不排。
func TestPlayerSearchQueuesTheScreens(t *testing.T) {
	g := newGame(t)
	at := 0
	for _, p := range g.Prefectures() {
		if p.Owned() && p.Owner == 0 {
			at = p.ID
			break
		}
	}
	for i := range g.generals {
		x := &g.generals[i]
		if x.Location == at && x.Status == state.StatusIdle {
			x.Location = 0
		}
	}
	hidden := g.General(300)
	hidden.Location, hidden.Status, hidden.Faction = at, state.StatusIdle, state.NoFaction
	var by *General
	for _, x := range g.Garrison(at) {
		if x.Faction == 0 && (by == nil || x.Intel > by.Intel) {
			by = x
		}
	}
	by.Intel = 99
	g.Prefecture(at).Gold = 100
	if err := (SearchOrder{At: at, General: by.Index}).Apply(g, 0); err != nil {
		t.Fatal(err)
	}
	ev := g.PendingEvents()
	if len(ev) != 2 || ev[0].Bubble == nil || ev[1].Bubble == nil {
		t.Fatalf("找到人該排兩格，排了 %d：%+v", len(ev), ev)
	}
	face, say := ev[0].Bubble, ev[1].Bubble
	if !face.FaceOnly || !face.WipeIn || face.Speaker != hidden.Index || face.X1 != SearchFaceX || face.Y1 != SearchFaceY {
		t.Errorf("第一格該是亮 %d 的肖像在 (%d,%d)，是 %+v", hidden.Index, SearchFaceX, SearchFaceY, face)
	}
	if say.FaceOnly || say.Speaker != by.Index || say.Left || say.Y1 != BubbleLowerY1 {
		t.Errorf("第二格該是尋訪者 %d 在下格、肖像在右，是 %+v", by.Index, say)
	}
	if !strings.Contains(say.Text, personName(hidden.Name)) {
		t.Errorf("對白 %q 沒有名士的名字", say.Text)
	}

	// 沒找到：郡裡沒人可找（下令旗標放掉，這個月再下一道）。
	hidden.Location = 0
	g.Prefecture(at).Gold, g.Prefecture(at).Commanded = 100, false
	if err := (SearchOrder{At: at, General: by.Index}).Apply(g, 0); err != nil {
		t.Fatal(err)
	}
	ev = g.PendingEvents()
	if len(ev) != 1 || ev[0].Bubble == nil || ev[0].Bubble.FaceOnly || ev[0].Bubble.Speaker != by.Index {
		t.Fatalf("沒找到該只排尋訪者那一格，排了 %+v", ev)
	}

	// 電腦那一條不排畫面。
	g.Prefecture(at).Commanded = false
	if err := (SearchOrder{At: at, General: by.Index, Auto: true}).Apply(g, 0); err != nil {
		t.Fatal(err)
	}
	if ev = g.PendingEvents(); len(ev) != 0 {
		t.Errorf("電腦尋訪排了 %d 格畫面", len(ev))
	}
}

// TestAdviseRollsOnceThenSpeaks 釘住軍師勸諫（`docs/spec/005` §9.6）：
// 每一道命令都擲一次 `RND(5)`；沒有軍師就不開口（那一擲照抽）；謀略 99
// 的軍師每一道都開口，說話者是他、下格、肖像在左，字色 0–7。
func TestAdviseRollsOnceThenSpeaks(t *testing.T) {
	g := newGame(t)
	at := 0
	for _, p := range g.Prefectures() {
		if p.Owned() && p.Owner == 0 {
			at = p.ID
			break
		}
	}
	f := g.Faction(0)
	f.Chief = -1
	g.SeedRand(1)
	before := g.RandDraws()
	if adv := g.Advise(AdviceTrain, at, 0, AdviceTarget{}); adv != nil {
		t.Errorf("沒有軍師還開口：%+v", adv)
	}
	if g.RandDraws() != before+1 {
		t.Errorf("沒有軍師那一擲該照抽：抽了 %d 次", g.RandDraws()-before)
	}
	// 找一位在職的當軍師，謀略拉到 99。
	var chief *General
	for _, x := range g.Garrison(at) {
		if x.Faction == 0 && x.Status != state.StatusLord {
			chief = x
			break
		}
	}
	if chief == nil {
		t.Fatal("郡裡沒有在職的部將")
	}
	chief.Intel, chief.Status = 99, state.StatusChief
	f.Chief = chief.Index
	for _, kind := range []AdviceKind{AdviceAttack, AdviceMove, AdviceTrain, AdviceConscript, AdviceArms,
		AdviceBalance, AdviceRest, AdviceReclaim, AdviceFlood, AdviceFort, AdviceBuy, AdviceSell,
		AdviceRelief, AdviceSearch, AdviceReward, AdviceDismiss} {
		adv := g.Advise(kind, at, 0, AdviceTarget{})
		if adv == nil || len(adv.Events) != 1 || adv.Events[0].Bubble == nil {
			t.Fatalf("勸諫 %d：軍師沒開口或不是一格：%+v", kind, adv)
		}
		b := adv.Events[0].Bubble
		if b.Speaker != chief.Index || !b.Left || b.Y1 != BubbleLowerY1 || b.Text == "" || b.Color < 0 || b.Color > 7 {
			t.Errorf("勸諫 %d 的格子不對：%+v", kind, b)
		}
	}
	// 登用與挖角看目標；計略再加一則預測。
	target := g.General(151)
	target.Location, target.Status, target.Faction = at, state.StatusAvailable, state.NoFaction
	if adv := g.Advise(AdviceRecruit, at, 0, AdviceTarget{Target: 151}); adv == nil || len(adv.Events) != 1 {
		t.Errorf("登用的勸諫：%+v", adv)
	}
	enemy := 0
	for _, p := range g.Prefectures() {
		if p.Owned() && p.Owner != 0 && g.Adjacent(at, p.ID) {
			enemy = p.ID
			break
		}
	}
	if enemy == 0 {
		t.Skip("沒有相鄰的敵郡")
	}
	if adv := g.Advise(AdvicePlot, at, 0, AdviceTarget{Target: chief.Index, To: enemy, What: PlotForgery}); adv == nil || len(adv.Events) != 2 {
		t.Errorf("計略的勸諫該是兩格（一句評語＋一則預測）：%+v", adv)
	}
	if adv := g.Advise(AdvicePlot, at, 0, AdviceTarget{To: enemy, What: PlotJointAttack}); adv == nil || len(adv.Events) != 2 {
		t.Errorf("聯合出兵的勸諫該是兩格：%+v", adv)
	}
}

// TestPlayerOrdersQueueTheirDialogue 釘住玩家命令之後排進 pending 的對白
// （`docs/spec/005` §9.6）：每一道先一格場景圖（`docs/spec/010` §8），
// 賞賜再一格（受賞者、下格右）、指定軍師再兩格（君主上格右、新軍師
// 下格左）、電腦的同一道命令一格都沒有。
func TestPlayerOrdersQueueTheirDialogue(t *testing.T) {
	g := newGame(t)
	at := 0
	for _, p := range g.Prefectures() {
		if p.Owned() && p.Owner == 0 {
			at = p.ID
			break
		}
	}
	g.Prefecture(at).Gold = 500
	var officer *General
	for _, x := range g.Garrison(at) {
		if x.Faction == 0 && x.Status != state.StatusLord {
			officer = x
			break
		}
	}
	officer.Loyalty = 50
	if err := (RewardOrder{At: at, Target: officer.Index, Gold: 10}).Apply(g, 0); err != nil {
		t.Fatal(err)
	}
	scene := func(ev []Event, n int, what string) []Event {
		t.Helper()
		if len(ev) == 0 || ev[0].Bubble == nil || ev[0].Bubble.Scene != n ||
			ev[0].Bubble.X1 != assets.SceneMainX || ev[0].Bubble.Y1 != assets.SceneMainY {
			t.Fatalf("%s的第一格該是場景圖 SCG%02d 落在 (432,80)：%+v", what, n, ev)
		}
		return ev[1:]
	}
	ev := scene(g.PendingEvents(), assets.SceneReward, "賞賜")
	if len(ev) != 1 || ev[0].Bubble == nil || ev[0].Bubble.Speaker != officer.Index || ev[0].Bubble.Left || ev[0].Bubble.Y1 != BubbleLowerY1 {
		t.Fatalf("賞賜之後該是受賞者在下格右邊一格：%+v", ev)
	}
	g.Prefecture(at).Commanded = false
	if err := (AppointChiefOrder{At: at, Target: officer.Index}).Apply(g, 0); err != nil {
		t.Fatal(err)
	}
	ev = scene(g.PendingEvents(), assets.SceneAppoint, "指定軍師")
	if len(ev) != 2 || ev[0].Bubble.Speaker != g.Lord(0).Index || ev[0].Bubble.Left || ev[0].Bubble.Y1 != BubbleUpperY1 ||
		ev[1].Bubble.Speaker != officer.Index || !ev[1].Bubble.Left || ev[1].Bubble.Y1 != BubbleLowerY1 {
		t.Fatalf("指定軍師該是君主上格右、新軍師下格左：%+v", ev)
	}
	// 賜物（`0x1d005`）：受賜者在上格右邊道謝（`0x1d4c1`）之後，右側面板
	// 再換成他的人物資料卡（`0x1d4d1`）——卡那一格沒有字。
	g.Prefecture(at).Commanded, officer.Rewarded = false, false
	g.Faction(0).Treasury[TreasureBook] = 1
	if err := (GiftOrder{At: at, Target: officer.Index, What: TreasureBook}).Apply(g, 0); err != nil {
		t.Fatal(err)
	}
	ev = scene(g.PendingEvents(), assets.SceneReward, "賜物")
	if len(ev) != 2 || ev[0].Bubble.Speaker != officer.Index || ev[0].Bubble.Left || ev[0].Bubble.Y1 != BubbleUpperY1 || ev[0].Bubble.Card ||
		!ev[1].Bubble.Card || ev[1].Bubble.Speaker != officer.Index {
		t.Fatalf("賜物該是受賜者上格右道謝、再一格他的資料卡：%+v", ev)
	}
	// 電腦那一條沒有畫面。
	g.Prefecture(at).Commanded, officer.Rewarded = false, false
	g.Player = 5
	if err := (RewardOrder{At: at, Target: officer.Index, Gold: 10}).Apply(g, 0); err != nil {
		t.Fatal(err)
	}
	if ev = g.PendingEvents(); len(ev) != 0 {
		t.Errorf("電腦的賞賜排了 %d 格畫面", len(ev))
	}
}
