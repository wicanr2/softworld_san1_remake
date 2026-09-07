package menu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
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
