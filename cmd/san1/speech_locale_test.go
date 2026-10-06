package main

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

func TestWindowSpeechLocaleBothQueues(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, from := range i18n.Locales() {
		i18n.Current = from
		b := battle.New(battle.Setup{Field: battle.Generate(battle.Params{}), Seed: 1})
		x := battle.FromOffset(2, 2)
		a := &battle.Unit{Side: battle.MainAttacker, Move: 20, At: x,
			Leaders: []battle.Leader{{Name: "呂布", Index: 6, War: 100, Stamina: 100, Soldiers: 2500}}}
		d := &battle.Unit{Side: battle.MainDefender, Move: 20, At: x.Step(battle.DirDownRight),
			Leaders: []battle.Leader{{Name: "陳宮", Index: 112, War: 50, Stamina: 100, Soldiers: 301}}}
		b.Units = []*battle.Unit{a, d}
		if err := b.Duel(a, battle.DirDownRight, func() bool { return true }); err != nil {
			t.Fatal(err)
		}
		speeches := b.TakeSpeeches()
		if len(speeches) < 3 {
			t.Fatal("沒有實際叫陣與應戰")
		}
		f := &fight{pending: &game.Pending{B: b}, speeches: speeches[:2]}
		b.Speeches = speeches[2:]
		app := &app{fight: f}
		old := from
		for _, to := range []i18n.Locale{i18n.En, i18n.Ja, i18n.ZhHant, from} {
			i18n.Current = to
			app.relocalizeWindow(old)
			chen := map[i18n.Locale]string{i18n.ZhHant: "陳宮", i18n.Ja: "陳宮", i18n.En: "Chen Gong"}[to]
			lu := map[i18n.Locale]string{i18n.ZhHant: "呂布", i18n.Ja: "呂布", i18n.En: "Lu Bu"}[to]
			if f.speeches[1].Text != i18n.Tf(to, "bub.duelChallenge", chen) ||
				b.Speeches[0].Text != i18n.Tf(to, "bub.duelAccept", lu) {
				t.Fatalf("%s→%s 未更新兩個對白佇列", old, to)
			}
			if len(f.speeches) != 2 || len(b.Speeches) != len(speeches)-2 {
				t.Fatal("換語言消耗對白")
			}
			old = to
		}
		if sp := f.speech(true); sp == nil || sp.Scene != 29 || len(b.Speeches) != 0 || len(f.speeches) != len(speeches) {
			t.Fatal("對白移交或場景順序改變")
		}
	}
}
