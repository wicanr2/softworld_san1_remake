package session

import (
	"testing"
	"unicode"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestSaveThenLoadContinues 釘住存了再讀回來還能繼續玩下去。
//
// 逐欄比對在 `internal/save`；這裡問的是另一件事——**讀回來的那一局
// 推得動嗎**。一份欄位都對但推一個月就當掉的存檔，逐欄比對看不出來。
func TestSaveThenLoadContinues(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	for i := 0; i < 24; i++ {
		s.EndMonth()
	}
	dir := t.TempDir()
	if err := s.Save(dir, 1, "續戰"); err != nil {
		t.Fatal(err)
	}
	list := Saves(dir)
	if len(list) != 6 || !list[0].Exists || list[0].Name != "續戰" {
		t.Fatalf("存檔清單不對：%+v", list)
	}

	t2, err := Load(dir, 1, ai.ModeEnhanced)
	if err != nil {
		t.Fatal(err)
	}
	if t2.G.Date != s.G.Date {
		t.Fatalf("讀回來是 %v，存的是 %v", t2.G.Date, s.G.Date)
	}
	before := t2.G.Date
	for i := 0; i < 12; i++ {
		t2.EndMonth()
	}
	if t2.G.Date == before {
		t.Error("讀回來的局面推不動")
	}
	// 讀回來之後再跑一年，兩邊應該走到同一個局面——**規則是決定性的**。
	for i := 0; i < 12; i++ {
		s.EndMonth()
	}
	if s.G.Date != t2.G.Date {
		t.Errorf("兩邊各跑一年後年月不同：%v vs %v", s.G.Date, t2.G.Date)
	}
	for id := 1; id <= state.PrefectureCount; id++ {
		a, b := s.G.Prefecture(id), t2.G.Prefecture(id)
		if a.Owner != b.Owner || a.Population != b.Population ||
			a.Gold != b.Gold || a.Rice != b.Rice {
			t.Fatalf("郡 %d 分家了：\n原局 %+v\n讀檔 %+v", id, *a, *b)
		}
	}
}

// TestSaveNeedsADirectory 釘住沒有存檔目錄要說出來。
//
// **不要偷偷選一個預設目錄**：玩家會在不知道的地方留下檔案，
// 而下一次「怎麼找不到我的進度」就沒人答得出來。
func TestSaveNeedsADirectory(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	if err := s.Save("", 1, "x"); err == nil {
		t.Error("沒有存檔目錄竟然存得成功")
	}
	if _, err := Load("", 1, ai.ModeEnhanced); err == nil {
		t.Error("沒有存檔目錄竟然讀得成功")
	}
	if Saves("") != nil {
		t.Error("沒有存檔目錄不該列得出存檔")
	}
}

// TestSaveNamesItselfAfterTheLord 釘住沒給名字時用君主的名字。
func TestSaveNamesItselfAfterTheLord(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, 0)
	dir := t.TempDir()
	if err := s.Save(dir, 3, ""); err != nil {
		t.Fatal(err)
	}
	lord := s.G.Lord(s.Player)
	if lord == nil {
		t.Skip("這一局沒有君主")
	}
	if got := Saves(dir)[2].Name; got != lord.Name {
		t.Errorf("存檔名稱是 %q，應該是君主的名字 %q", got, lord.Name)
	}
}

// TestLoadKeepsTheAIChosenInGame 釘住讀檔沿用**存檔裡**的 AI 版本。
//
// 玩家在遊戲中換過 AI（「其他 → 電腦AI」）之後存檔，讀回來卻套旗標的
// 版本，玩家看到的就是「設定沒存到」——而畫面上唯一的差別只是電腦
// 諸侯下不同的命令，看不出來。
func TestLoadKeepsTheAIChosenInGame(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	// 遊戲中換成還原版的 AI。
	next := ai.NextMode(s.Brain.Mode(), s.G.Edition)
	brain, err := ai.New(next)
	if err != nil {
		t.Fatal(err)
	}
	s.SetBrain(brain)
	s.G.Options.SetAIMode(string(next))
	if err := s.G.Options.SetAIOrders(3); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := s.Save(dir, 1, "換過AI"); err != nil {
		t.Fatal(err)
	}
	// **旗標故意給另一個**：存檔裡有就該聽存檔的。
	back, err := Load(dir, 1, ai.ModeEnhanced)
	if err != nil {
		t.Fatal(err)
	}
	if back.Brain.Mode() != next {
		t.Errorf("讀回來的 AI 是 %s，存的是 %s", back.Brain.Mode(), next)
	}
	if back.G.Options.AIOrders() != 3 {
		t.Errorf("電腦指令數讀回來是 %d，存的是 3", back.G.Options.AIOrders())
	}
	// 沒動過的那一局照樣聽旗標。
	plain := newSession(t, ai.ModeEnhanced, state.NoFaction)
	if err := plain.Save(dir, 2, "沒動過"); err != nil {
		t.Fatal(err)
	}
	b2, err := Load(dir, 2, ai.ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	if b2.Brain.Mode() != ai.ModeBase {
		t.Errorf("沒動過設定的存檔讀回來是 %s，旗標給的是 %s",
			b2.Brain.Mode(), ai.ModeBase)
	}
}

// TestLoadKeepsPlayerDefence 釘住「守城」那個開關進得了存檔（Issue #64）。
//
// **開關不進存檔與沒有這個開關是同一件事**：玩家開了親自守城，下一次
// 讀檔又變回自動打完，而畫面上唯一的差別是電腦來攻時有沒有停下來。
func TestLoadKeepsPlayerDefence(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	if s.G.Options.PlayerDefends {
		t.Fatal("預設應該是自動打完")
	}
	s.G.Options.TogglePlayerDefend()
	dir := t.TempDir()
	if err := s.Save(dir, 1, "親自守"); err != nil {
		t.Fatal(err)
	}
	back, err := Load(dir, 1, ai.ModeEnhanced)
	if err != nil {
		t.Fatal(err)
	}
	if !back.G.Options.PlayerDefends {
		t.Error("讀回來變成自動打完了")
	}
	// 沒開過的那一局讀回來仍然是自動。
	plain := newSession(t, ai.ModeEnhanced, state.NoFaction)
	if err := plain.Save(dir, 2, "沒開"); err != nil {
		t.Fatal(err)
	}
	b2, err := Load(dir, 2, ai.ModeEnhanced)
	if err != nil {
		t.Fatal(err)
	}
	if b2.G.Options.PlayerDefends {
		t.Error("沒開過卻讀成親自守")
	}
}

// TestSetBrainLeavesATrace 釘住換 AI 會在訊息紀錄裡留下痕跡。
//
// **AI 換了而畫面上沒有任何痕跡**，之後回頭問「這個諸侯為什麼突然
// 不動了」就查不出來。
func TestSetBrainLeavesATrace(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	n := len(s.Log)
	brain, err := ai.New(ai.ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	s.SetBrain(brain)
	if len(s.Log) == n {
		t.Fatal("換了 AI 卻沒有留下任何訊息")
	}
	if s.Brain.Mode() != ai.ModeBase {
		t.Errorf("換完是 %s", s.Brain.Mode())
	}
	// 換成同一個不留痕跡——那不是一次「換」。
	n = len(s.Log)
	s.SetBrain(s.Brain)
	if len(s.Log) != n {
		t.Error("換成同一個 AI 也寫了一則訊息")
	}
}

// TestEnglishLogHasNoChinese 釘住英文下的訊息紀錄沒有漢字。
//
// 訊息紀錄會出現在下面板上（原版素材畫面顯示最後一則），先前 session
// 自己寫的字（「已存入第 1 個進度」「電腦 AI 換成…」）寫死中文，
// AI 的名字也是。判準是**整局跑過**：開局、推兩年（事件、戰役、電腦
// 下令都會進紀錄）、存讀檔、換 AI。
func TestEnglishLogHasNoChinese(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	i18n.Current = i18n.En

	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	for i := 0; i < 24; i++ {
		s.EndMonth()
	}
	dir := t.TempDir()
	if err := s.Save(dir, 1, ""); err != nil {
		t.Fatal(err)
	}
	back, err := Load(dir, 1, ai.ModeEnhanced)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := ai.New(ai.ModeBase)
	back.SetBrain(b)
	bad := 0
	for _, log := range [][]string{s.Log, back.Log} {
		for _, line := range log {
			for _, r := range line {
				if unicode.Is(unicode.Han, r) {
					if bad < 10 {
						t.Errorf("英文的訊息紀錄有漢字 %q：%q", string(r), line)
					}
					bad++
					break
				}
			}
		}
	}
	if bad > 0 {
		t.Errorf("共 %d 行有漢字", bad)
	}
}

// TestSaveKeepsEveryPlayer 釘住多位玩家存讀檔之後每一位都還是玩家、
// 序號不變，其餘在用的諸侯還是電腦（`docs/spec/019` §3）。
func TestSaveKeepsEveryPlayer(t *testing.T) {
	players := []state.FactionID{1, 0}
	s := newPlayersSession(t, players)
	dir := t.TempDir()
	if err := s.Save(dir, 2, ""); err != nil {
		t.Fatal(err)
	}
	t2, err := Load(dir, 2, ai.ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	if len(t2.G.Players) != 2 || t2.G.Players[0] != 1 || t2.G.Players[1] != 0 {
		t.Fatalf("讀回來的玩家是 %v，存的是 %v", t2.G.Players, players)
	}
	for _, f := range s.G.Factions() {
		if s.G.IsHuman(f.ID) != t2.G.IsHuman(f.ID) {
			t.Errorf("勢力 %d：存之前 IsHuman %v、讀回來 %v", f.ID, s.G.IsHuman(f.ID), t2.G.IsHuman(f.ID))
		}
	}
	if at := t2.AdvanceToHuman(0); at == 0 || !t2.G.IsHuman(t2.G.Prefecture(at).Owner) {
		t.Errorf("讀回來之後停在郡 %d", at)
	}
}

// TestSaveNameFollowsTheOriginalLayout 釘住存檔名稱照原版組（`0x1e5b2`）：
// 「n.」＋姓名欄 6 byte（兩字名前後補空白）＋「在」＋郡名＋6 格備註，共 20 格；
// 寫出去再讀回來名稱原樣。
func TestSaveNameFollowsTheOriginalLayout(t *testing.T) {
	s := newSession(t, ai.ModeBase, 0) // 劉備
	own := s.G.Territory(0)
	if len(own) == 0 {
		t.Fatal("劉備沒有郡")
	}
	at := own[0]
	p := s.G.Prefecture(at)
	name := s.SaveName(3, at, "Y201")
	want := "3. 劉備 在" + p.Name + "Y201  "
	if name != want {
		t.Fatalf("SaveName ＝ %q，想要 %q", name, want)
	}
	if w := cells.Width(name); w != 20 {
		t.Errorf("名稱 %d 格，原版一筆是 20 格", w)
	}
	dir := t.TempDir()
	if err := s.Save(dir, 3, name); err != nil {
		t.Fatal(err)
	}
	if got := Saves(dir)[2].Name; got != name {
		t.Errorf("讀回來的名稱 %q，存的是 %q", got, name)
	}
}

// TestEndMonthFinishesEvenWhenPlayerDefends 釘住「守城」開著時無畫面的
// 月流程照樣走得完（Issue #64）。
//
// `ComputerAttack` 開著開關會把戰役交出來，月流程停在那一格等玩家；
// 而 `EndMonth` 是**沒有人可以指揮**的那一條（測試、批次跑）。
// 不就地打完的話，`runPrefectureTurns` 在那一格 break，月份照樣往前推——
// **剩下的郡整個月沒跑**，而且不會報錯。
//
// **交出去那一步要自己擺**：等電腦剛好打過來的話，沒打過來時這支測試
// 會安靜地變成空跑（`~/diagnosis-notes/docs/03-silence-is-not-success`）。
func TestEndMonthFinishesEvenWhenPlayerDefends(t *testing.T) {
	// **要有玩家**：`NoFaction` 的那一局 `IsHuman` 處處為假，
	// 交不出戰役，這支測試會安靜地變成空跑。劇本一的 0 是劉備。
	s := newSession(t, ai.ModeEnhanced, 0)
	g := s.G
	g.Options.PlayerDefends = true
	// 找一對「電腦郡挨著玩家郡」，讓電腦打過來。
	var from, to int
	var by state.FactionID
	for id := 1; id <= 42 && from == 0; id++ {
		p := g.Prefecture(id)
		if p == nil || !p.Owned() || !g.IsHuman(p.Owner) {
			continue
		}
		for _, n := range p.Neighbours {
			q := g.Prefecture(n)
			if q != nil && q.Owned() && !g.IsHuman(q.Owner) && len(g.ActorRoster(n)) > 1 {
				from, to, by = n, id, q.Owner
				break
			}
		}
	}
	if from == 0 {
		t.Fatal("劇本一裡找不到「電腦郡挨著玩家郡」的一對")
	}
	if _, err := g.ComputerAttack(from, to, by, func(*game.State, int) int { return 0 }); err != nil {
		t.Fatal(err)
	}
	if g.PendingDefence() == nil {
		t.Fatal("開關開著卻沒有把戰役交出來——這支測試什麼都沒驗到")
	}

	month := g.Date.Month
	s.EndMonth()
	if g.PendingDefence() != nil {
		t.Error("月流程跑完還留著一場沒打完的守城")
	}
	if g.Date.Month == month {
		t.Errorf("月份沒往前走，還是 %d 月", month)
	}
	if s.MonthCursor != 0 {
		t.Errorf("月底的游標是 %d，應該是開月的 0", s.MonthCursor)
	}
}
