package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

func TestBattleViewRelocalizeCommandAndResumeSnapshot(t *testing.T) {
	previous := i18n.Current
	defer func() { i18n.Current = previous }()
	for _, locale := range []i18n.Locale{i18n.En, i18n.Ja} {
		t.Run(string(locale), func(t *testing.T) {
			i18n.Current = i18n.ZhHant
			unit := &battle.Unit{Move: 17}
			originalCommand := BattleCommandWindow("曹操", battle.Centre, 17, "關羽")
			unknown := "自訂紀錄 AB-345\n"
			view := BattleView{
				Window: unknown + originalCommand, Menu: i18n.S("bat.captive"),
				Items: []string{i18n.S("bat.arrowKeys")}, Prompt: i18n.S("bat.win.restYN"),
				PageTitle: i18n.S("bat.inspect"), Page: []string{unknown}, PageTop: 3,
				Acting: unit, Input: InputCursor{On: true, Frame: 2}, Blink: true,
				Cursor: Hexer{At: battle.FromOffset(3, 4), Shown: true},
			}
			before, saved := view, view
			i18n.Current = locale
			command := BattleCommandWindow("曹操", battle.Centre, 17, "關羽")
			view.Relocalize(i18n.ZhHant, command)
			saved.Relocalize(i18n.ZhHant, command)
			if view.Window != unknown+command || !strings.Contains(view.Window, "17") {
				t.Fatalf("命令、參數或未知前綴改變：%q", view.Window)
			}
			if !reflect.DeepEqual(view, saved) || before.Items[0] != i18n.T(i18n.ZhHant, "bat.arrowKeys") {
				t.Fatal("保存快照或共享字串列受影響")
			}
			if view.Acting != unit || unit.Move != 17 || view.Input != before.Input ||
				view.Cursor != before.Cursor || view.PageTop != 3 || !view.Blink || view.Page[0] != unknown {
				t.Fatal("更新語言改變非文字狀態")
			}
			i18n.Current = i18n.ZhHant
			view.Relocalize(locale, originalCommand)
			if !reflect.DeepEqual(view, before) {
				t.Fatalf("回切未恢復：%+v", view)
			}
		})
	}
}

func TestBattleViewRelocalizeOtherPromptAndUnknownWindow(t *testing.T) {
	previous := i18n.Current
	defer func() { i18n.Current = previous }()
	for _, text := range []string{i18n.Tf(i18n.ZhHant, "bat.win.move", 23), "自訂提示 AB-345"} {
		view := BattleView{Window: text}
		i18n.Current = i18n.En
		view.Relocalize(i18n.ZhHant, "")
		if got := view.Window; got != i18n.Relocalize(text, i18n.ZhHant, i18n.En) {
			t.Fatalf("提示未對回字串表：%q", got)
		}
		i18n.Current = i18n.ZhHant
		view.Relocalize(i18n.En, "")
		if view.Window != text || view.Items != nil || view.Page != nil {
			t.Fatal("未知字串、數值或 nil 顯示資料改變")
		}
	}
}
