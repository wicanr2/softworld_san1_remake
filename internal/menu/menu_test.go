package menu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// newScreen 開一個接得到原版劇本的主選單；沒有素材就 skip。
func newScreen(t *testing.T) *Screen {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過（本儲存庫不含原版檔案）")
	}
	dir := filepath.Join(root, "三國演義")
	read := func(ext string) []byte {
		b, err := os.ReadFile(filepath.Join(dir, "DATA2."+ext))
		if err != nil {
			t.Skipf("讀不到 DATA2.%s：%v", ext, err)
		}
		return b
	}
	c, err := assets.OpenContainer(read("NAM"), read("IDX"), read("GRP"))
	if err != nil {
		t.Fatal(err)
	}
	return New(c, state.EditionBase, ai.ModeEnhanced, "", 6)
}

// TestStartsANewGame 走完「開始新遊戲」那三層。
func TestStartsANewGame(t *testing.T) {
	s := newScreen(t)
	if s.Confirm(0) != nil { // 1. 開始新遊戲
		t.Fatal("選年代那一層就開出一局")
	}
	if s.Stage() != Scenario || len(s.Items()) != 6 {
		t.Fatalf("選年代那一層有 %d 項（stage %d）", len(s.Items()), s.Stage())
	}
	s.Confirm(0) // 第一個劇本
	if s.Stage() != Lord {
		t.Fatalf("選君主那一層沒出來（stage %d）：%v", s.Stage(), s.Items())
	}
	if len(s.lords) < 2 {
		t.Fatalf("只列出 %d 個君主", len(s.lords))
	}
	want := s.lords[0]
	s.Confirm(0) // 第一位君主
	if s.Stage() != Difficulty || len(s.Items()) != 10 {
		t.Fatalf("難度那一層有 %d 項（stage %d）", len(s.Items()), s.Stage())
	}
	ss := s.Confirm(4) // 難度 5
	if ss == nil {
		t.Fatal("沒有開出一局")
	}
	if int(ss.Player) != want {
		t.Errorf("玩家是勢力 %d，選的是 %d", ss.Player, want)
	}
	if ss.G.Difficulty != 5 {
		t.Errorf("難度是 %d，選的是 5", ss.G.Difficulty)
	}
}

// TestPlusRaisesTheDifficultyCap 釘住難度上限跟著版本走。
func TestPlusRaisesTheDifficultyCap(t *testing.T) {
	s := newScreen(t)
	s.edition = state.EditionPlus
	s.Confirm(0)
	s.Confirm(0)
	s.Confirm(0)
	if s.Stage() != Difficulty || len(s.Items()) != 20 {
		t.Errorf("加強版的難度有 %d 項", len(s.Items()))
	}
}

// TestQuit 釘住「回作業系統」。
func TestQuit(t *testing.T) {
	s := newScreen(t)
	s.Confirm(5)
	if !s.Quit() {
		t.Error("選了回作業系統卻沒有要離開")
	}
}

// TestFontNote 釘住字型那兩項只說明一句。
func TestFontNote(t *testing.T) {
	for _, i := range []int{2, 3} {
		s := newScreen(t)
		s.Confirm(i)
		if s.Stage() != Note || len(s.Items()) != 1 {
			t.Errorf("第 %d 項應該只說明一句（stage %d）", i+1, s.Stage())
		}
		s.Confirm(0)
		if s.Stage() != Menu {
			t.Errorf("說明按完沒回主選單（stage %d）", s.Stage())
		}
	}
}

// TestMusicPicksATrack 釘住音樂欣賞選得到曲子，而且只回報一次。
func TestMusicPicksATrack(t *testing.T) {
	s := newScreen(t)
	s.Confirm(4)
	if s.Stage() != Music || len(s.Items()) != 6 {
		t.Fatalf("音樂欣賞有 %d 項（stage %d）", len(s.Items()), s.Stage())
	}
	s.Confirm(2)
	if got := s.Track(); got != 2 {
		t.Errorf("選的是第 3 曲，回報 %d", got)
	}
	if got := s.Track(); got != -1 {
		t.Errorf("同一次選擇回報了兩次：%d", got)
	}
}

// TestMoveWraps 釘住上下移動會繞回去。
func TestMoveWraps(t *testing.T) {
	s := newScreen(t)
	s.Move(-1)
	if s.Sel() != ItemCount-1 {
		t.Errorf("往上繞到 %d", s.Sel())
	}
	s.Move(1)
	if s.Sel() != 0 {
		t.Errorf("往下繞到 %d", s.Sel())
	}
}

// TestNoSavesSaysSo 釘住沒有進度時說一句而不是給空清單。
func TestNoSavesSaysSo(t *testing.T) {
	s := newScreen(t)
	s.Confirm(1)
	if s.Stage() != Note || len(s.Items()) != 1 {
		t.Errorf("沒有進度時 stage %d、%d 項", s.Stage(), len(s.Items()))
	}
}

// 新君主欄要列在「選角色」那一層的最後面，而且走得完整條流程。
func TestPicksACustomLord(t *testing.T) {
	s := newScreen(t)
	s.Confirm(0) // 開始新遊戲
	s.Confirm(0) // 第一個劇本
	if s.Stage() != Lord {
		t.Fatalf("沒到選角色那一層（stage %d）", s.Stage())
	}
	if len(s.customs) != 2 {
		t.Fatalf("新君主欄列了 %d 個，劇本 001 有 2 個", len(s.customs))
	}
	// 它們在最後面，而且項數對得上。
	if len(s.Items()) != len(s.lords) {
		t.Fatalf("%d 項對上 %d 個可選的", len(s.Items()), len(s.lords))
	}
	first := len(s.lords) - len(s.customs)
	if !s.isCustom(s.lords[first]) || s.isCustom(s.lords[first-1]) {
		t.Fatal("新君主欄不在清單的最後面")
	}
	// **先存起來**：`pickDifficulty` 會把 `lords` 換成只剩選中的那一個。
	want := s.lords[first]

	s.Confirm(first)
	if s.Stage() != CustomLord {
		t.Fatalf("沒進新君主的設定（stage %d）：%v", s.Stage(), s.Items())
	}
	if len(s.Items()) != customRows {
		t.Fatalf("設定那一層有 %d 項，想要 %d", len(s.Items()), customRows)
	}
	if s.custom.spare != state.CustomLordPoints {
		t.Errorf("一開始有 %d 點，想要 %d", s.custom.spare, state.CustomLordPoints)
	}

	// 左右鍵加減點數。**加到沒點數就不能再加**——不然 100 點是裝飾。
	s.pick = 0
	for i := 0; i < state.CustomLordPoints+5; i++ {
		s.Adjust(1)
	}
	if s.custom.spare != 0 {
		t.Errorf("加滿之後還剩 %d 點", s.custom.spare)
	}
	if s.custom.lord.Stamina != state.CustomLordPoints {
		t.Errorf("體能加了 %d 點，想要 %d", s.custom.lord.Stamina,
			state.CustomLordPoints)
	}
	// 退回去也要退點。
	s.Adjust(-1)
	if s.custom.spare != 1 || s.custom.lord.Stamina != state.CustomLordPoints-1 {
		t.Errorf("退一點之後剩 %d 點、體能 +%d", s.custom.spare, s.custom.lord.Stamina)
	}
	// **減不能減到負的**：範本的底不退點。
	for i := 0; i < 200; i++ {
		s.Adjust(-1)
	}
	if s.custom.lord.Stamina != 0 {
		t.Errorf("一直減之後體能 +%d，想要 0", s.custom.lord.Stamina)
	}
	if s.custom.spare != state.CustomLordPoints {
		t.Errorf("全退之後剩 %d 點", s.custom.spare)
	}

	// 領地那一項換得動，而且一定是空白郡。
	s.pick = 4
	before := s.custom.lord.Prefecture
	s.Adjust(1)
	if s.custom.lord.Prefecture == before {
		t.Error("換不動領地")
	}
	for _, p := range s.custom.blanks {
		if p.Owned() {
			t.Fatalf("郡 %d 有主卻列在可選的領地裡", p.ID)
		}
	}

	// 其他層按左右鍵不該有事。
	s.pick = 0
	s.Adjust(1)
	if s.Adjust(0) {
		t.Error("d=0 也被當成調整")
	}

	// 「完成」→ 「新君主出現!!」（地圖上那一郡已經是新君主的）→ 難度 → 開局。
	s.pick = customRows - 1
	s.Confirm(customRows - 1)
	if s.Stage() != LordBorn {
		t.Fatalf("完成之後沒進「新君主出現」（stage %d）：%v", s.Stage(), s.Items())
	}
	if g := s.Game(); g == nil || len(g.Territory(state.FactionID(s.custom.faction))) != 1 {
		t.Fatalf("「新君主出現」那一格底下的局面沒有新君主的領地：%v", g)
	}
	s.Confirm(0)
	if s.Stage() != Difficulty {
		t.Fatalf("「新君主出現」之後沒進難度（stage %d）：%v", s.Stage(), s.Items())
	}
	ss := s.Confirm(4)
	if ss == nil {
		t.Fatal("沒有開出一局")
	}
	if int(ss.Player) != want {
		t.Errorf("玩家是勢力 %d，選的是 %d", ss.Player, want)
	}
	// 新君主真的在盤面上：一個郡、身分是君主。
	g := ss.G
	if n := len(g.Territory(ss.Player)); n != 1 {
		t.Errorf("新君主有 %d 個郡，想要 1", n)
	}
	lord := g.Lord(ss.Player)
	if lord == nil {
		t.Fatal("盤面上沒有新君主")
	}
	if int(lord.Age) != state.CustomLordAge {
		t.Errorf("年齡 %d，範本是 %d", lord.Age, state.CustomLordAge)
	}
}

// 沒選新君主的時候不該把 custom 的狀態帶進去。
func TestOrdinaryLordDoesNotCarryCustomState(t *testing.T) {
	s := newScreen(t)
	s.Confirm(0)
	s.Confirm(0)
	s.Confirm(0) // 第一位真的君主
	if s.Stage() != Difficulty {
		t.Fatalf("stage %d", s.Stage())
	}
	if s.custom != nil {
		t.Error("選一般君主也建了 custom 狀態")
	}
}

// 自創君主的名字要帶著字模，否則存出去給原版讀是三個空白。
func TestCustomLordCarriesTheShippedGlyphs(t *testing.T) {
	s := newScreen(t)
	s.Confirm(0)
	s.Confirm(0)
	first := len(s.lords) - len(s.customs)
	s.Confirm(first)
	s.Confirm(customRows - 1) // 完成 → 「新君主出現!!」
	s.Confirm(0)              // 任意鍵 → 難度
	ss := s.Confirm(4)
	if ss == nil {
		t.Fatal("沒有開出一局")
	}
	gl := ss.G.Glyphs()
	if gl == nil {
		t.Fatal("自創君主的局沒有帶字模")
	}
	// 出貨的那一份是「新君主」三個字重複四次（`docs/re/08` §3），
	// 所以**十二格都不是空的**，而且前三格與後面三組相同。
	blank := 0
	for i := range gl {
		empty := true
		for _, b := range gl[i] {
			if b != 0 {
				empty = false
				break
			}
		}
		if empty {
			blank++
		}
	}
	if blank != 0 {
		t.Errorf("十二個字模裡有 %d 個是空的", blank)
	}
	for k := 1; k < state.CustomLords; k++ {
		for i := 0; i < state.CustomLordNameChars; i++ {
			if gl[k*state.CustomLordNameChars+i] != gl[i] {
				t.Errorf("第 %d 組的第 %d 個字模與第一組不同——"+
					"出貨的那一份應該是同三個字重複四次", k+1, i+1)
			}
		}
	}

	// 一般君主的局不帶字模：那一局沒有造字要畫。
	s2 := newScreen(t)
	s2.Confirm(0)
	s2.Confirm(0)
	s2.Confirm(0)
	if ss2 := s2.Confirm(4); ss2 == nil || ss2.G.Glyphs() != nil {
		t.Error("一般君主的局也帶了字模")
	}
}

// TestTitleListsFitEveryLanguage 釘住開局選單的年代與君主兩層，三個語系、
// 六個劇本都在一項 40 格以內（`ui.TitleListCols`）。
//
// 再長的會被 `ui.DrawTitleList` 截掉而不報錯——選君主那一層截掉的正好是
// 最後的「幾郡」，那是挑君主時最要緊的數字。
func TestTitleListsFitEveryLanguage(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		widest, lords := "", 0
		for k := 0; k < 6; k++ {
			s := newScreen(t)
			s.Confirm(0) // 開始新遊戲 → 年代
			check := func(stage string) {
				if stage == "君主" {
					if s.Stage() != Lord {
						t.Fatalf("%s 劇本 %d 沒走到選君主那一層（stage %d）", l, k+1, s.Stage())
					}
					lords += len(s.Items())
				}
				for _, it := range append([]string{s.Title()}, s.Items()...) {
					if cells.Width(it) > cells.Width(widest) {
						widest = it
					}
					if w := cells.Width(it); w > ui.TitleListCols {
						t.Errorf("%s 劇本 %d 的%s有一項 %d 格，選單只有 %d 格：%q",
							l, k+1, stage, w, ui.TitleListCols, it)
					}
				}
			}
			check("年代")
			s.Confirm(k)
			check("君主")
		}
		t.Logf("%s：六個劇本共 %d 個君主欄，最寬一項 %d 格 %q", l, lords, cells.Width(widest), widest)
	}
}

// 劇本三的新君主欄有四個（槽 4、5、10、11 指向範本），列在最後面。
// 先前「不在 ActiveFactions 裡」的判準在這個劇本一個都列不出來。
func TestNewLordSlotsInScenarioThree(t *testing.T) {
	s := newScreen(t)
	s.Confirm(0) // 開始新遊戲
	s.Confirm(2) // 第三個劇本
	if s.Stage() != Lord {
		t.Fatalf("沒到選角色那一層（stage %d）", s.Stage())
	}
	if len(s.customs) != 4 {
		t.Fatalf("新君主欄列了 %d 個 %v，劇本 003 有 4 個", len(s.customs), s.customs)
	}
	for _, f := range s.lords[:len(s.lords)-4] {
		if s.isCustom(f) {
			t.Errorf("槽 %d 是新君主欄卻排在一般君主中間", f)
		}
	}
}
