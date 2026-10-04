package ui

import (
	"image"
	"reflect"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
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

func TestBattleViewRelocalizeSkirmishAndInspection(t *testing.T) {
	previous := i18n.Current
	defer func() { i18n.Current = previous }()
	for _, from := range i18n.Locales() {
		for _, to := range i18n.Locales() {
			i18n.Current = from
			u := &battle.Unit{Leaders: []battle.Leader{{Name: "呂布", Soldiers: 2392, War: 99, Stamina: 78}}, Move: 4}
			g := &battle.SkirmishGeneral{Leader: &u.Leaders[0], Unit: u, Left: 3, MoveCap: 4}
			title, page := BattleUnitPage(u)
			view := BattleView{Items: strings.Split(i18n.S("skm.menu"), "|"), Prompt: SkirmishPrompt("呂布", 3, 4),
				SkirmishActing: g, Inspecting: u, PageTitle: title, Page: page, PageTop: 2, Blink: true,
				Input: InputCursor{On: true, Frame: 4}, Cursor: Hexer{At: battle.FromOffset(3, 9), Shown: true}}
			before, saved := view, view
			unitBefore, generalBefore := *u, *g
			i18n.Current = to
			view.Relocalize(from, "")
			saved.Relocalize(from, "")
			wantTitle, wantPage := BattleUnitPage(u)
			if !reflect.DeepEqual(view.Items, strings.Split(i18n.S("skm.menu"), "|")) || view.Prompt != SkirmishPrompt("呂布", 3, 4) ||
				view.PageTitle != wantTitle || !reflect.DeepEqual(view.Page, wantPage) || !reflect.DeepEqual(view, saved) {
				t.Fatalf("%s→%s 子畫面顯示未更新：%+v", from, to, view)
			}
			if !reflect.DeepEqual(*u, unitBefore) || !reflect.DeepEqual(*g, generalBefore) || view.PageTop != 2 ||
				view.Input != before.Input || view.Cursor != before.Cursor || !view.Blink || view.SkirmishActing != g || view.Inspecting != u {
				t.Fatal("切換語言改變非文字狀態")
			}
			i18n.Current = from
			view.Relocalize(to, "")
			if !reflect.DeepEqual(view, before) {
				t.Fatalf("%s→%s 回切未恢復完整顯示", to, from)
			}
		}
	}
}

func TestBattleViewRelocalizeSkirmishUnknownText(t *testing.T) {
	previous := i18n.Current
	defer func() { i18n.Current = previous }()
	i18n.Current = i18n.En
	view := BattleView{Items: []string{"自訂 1", "自訂 2"}, Prompt: "自訂提示 AB-345",
		SkirmishActing: &battle.SkirmishGeneral{Leader: &battle.Leader{Name: "呂布"}, Left: 3, MoveCap: 4},
		Inspecting:     &battle.Unit{}, PageTitle: "自訂分頁", Page: []string{"自訂內容"}}
	before := view
	view.Relocalize(i18n.ZhHant, "")
	if !reflect.DeepEqual(view, before) {
		t.Fatal("未知子畫面字串被猜譯或重建")
	}
}

func TestSkirmishEnglishCommandsKeepCompleteLabels(t *testing.T) {
	previous := i18n.Current
	defer func() { i18n.Current = previous }()
	i18n.Current = i18n.En
	c := testCanvasPx(t, 640, 408)
	f := battle.Generate(battle.Params{Prefecture: 26})
	b := battle.New(battle.Setup{Field: f, Seed: 1})
	opts := strings.Split(i18n.S("skm.menu"), "|")
	prompt := SkirmishPrompt("呂布", 4, 4)
	v := BattleView{Items: opts, Prompt: prompt, SkirmishActing: &battle.SkirmishGeneral{}}
	DrawArtBattle(c, &ArtBattle{}, b, v, ArtBattleInfo{})
	want := testCanvasPx(t, 640, 408)
	x, y, _, _ := assets.BattleLayoutFor(f.Narrow()).Panel(2)
	want.FillRect(x, y, x+176, y+96, assets.EGAPalette[3])
	for row, line := range append(opts, prompt) {
		ink := assets.EGAPalette[14]
		if row == len(opts) {
			ink = assets.EGAPalette[15]
		}
		want.DrawSmallTextPx(x, y+row*SmallH, line, ink)
	}
	assertBattleLocaleRegion(t, c, want, image.Rect(x, y, x+176, y+96), "完整 March/Duel/Attack/View/Rest 與人物提示")
}
