package menu

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func TestWindowAISettingReachesNewGameAndSave(t *testing.T) {
	s := newScreen(t)
	if err := s.SetAI(ai.ModePlus, 5); err == nil {
		t.Fatal("跨版 AI 被接受")
	}
	if err := s.SetAI(ai.ModeEnhanced, 6); err == nil {
		t.Fatal("強度越界被接受")
	}
	if err := s.SetAI(ai.ModeEnhanced, 5); err != nil {
		t.Fatal(err)
	}
	toLord(t, s, 0)
	s.Confirm(0)
	ss := s.Confirm(4)
	if ss == nil || ss.G.Options.AIOrders() != 5 || ss.G.Options.AIMode != "enhanced" || ss.G.Difficulty != 5 {
		t.Fatalf("設定未接入新局: %+v", ss)
	}
	dir := t.TempDir()
	if err := ss.Save(dir, 1, "HD"); err != nil {
		t.Fatal(err)
	}
	loaded, err := session.Load(dir, 1, ai.ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Brain.Mode() != ai.ModeEnhanced || loaded.G.Options.AIOrders() != 5 {
		t.Fatal("AI 強度未保存")
	}
	if loaded.G.Edition != state.EditionBase {
		t.Fatal("AI 選項改變版本")
	}
}

func TestWindowLocaleKeepsMenuSelection(t *testing.T) {
	old := i18n.Current
	defer func() { i18n.Current = old }()
	i18n.Current = i18n.ZhHant
	s := newScreen(t)
	s.Confirm(0)
	s.Move(2)
	i18n.Current = i18n.En
	s.Relocalize(i18n.ZhHant)
	if s.Stage() != Scenario || s.Sel() != 2 || s.Title() != i18n.S("title.pickScenario") || s.Items()[2] != i18n.S("title.scenario3") {
		t.Fatal("語言切換取消或重設選擇")
	}
}
