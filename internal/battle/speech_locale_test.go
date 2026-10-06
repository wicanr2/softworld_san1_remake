package battle

import (
	"reflect"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func TestSpeechPersonLocaleSnapshot(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	keys := []string{"bub.duelChallenge", "bub.duelAccept", "bub.duelPraise", "bub.duelEqual",
		"bub.duelKill", "bub.duelFamed", "bub.duelSeize", "bub.duelDie", "bub.seen", "bub.captiveRefuse"}
	for _, key := range keys {
		for _, from := range i18n.Locales() {
			t.Run(key+"/"+string(from), func(t *testing.T) {
				i18n.Current = from
				b := arena(flat(Plain))
				rolls := useScript(b, 3)
				x := lead("呂布", 100, 50, 2500)
				x.Index = 6
				args := []any{speechPerson("關羽")}
				b.say(&x, BoxAttacker, true, key, args...)
				wantAsked(t, "對白", rolls, MessageLines)
				args[0] = speechPerson("陳宮")
				x.Name = "張飛"
				sp := b.TakeSpeeches()[0]
				if b.Speeches != nil {
					t.Fatal("移交後仍留在戰場")
				}
				before := sp
				old := from
				for _, to := range []i18n.Locale{i18n.En, i18n.Ja, i18n.ZhHant, i18n.En, from} {
					sp.Relocalize(old, to)
					name := map[i18n.Locale]string{i18n.ZhHant: "關羽", i18n.Ja: "関羽", i18n.En: "Guan Yu"}[to]
					if sp.Text != i18n.Tf(to, key, name) {
						t.Fatalf("%s→%s: %q", old, to, sp.Text)
					}
					before.Text = sp.Text
					if !reflect.DeepEqual(sp, before) {
						t.Fatal("換語言改變來源快照或非文字欄位")
					}
					old = to
				}
				if len(rolls.asked) != 1 {
					t.Fatal("換語言擲了新亂數")
				}
			})
		}
	}
}

func TestDuelSpeechesRelocalizeFromSource(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, edition := range []state.Edition{"base", "plus"} {
		for _, from := range i18n.Locales() {
			t.Run(string(edition)+"/"+string(from), func(t *testing.T) {
				i18n.Current = from
				b := arena(flat(Plain))
				b.Rules = RulesFor(edition, 5)
				ca, ct := lead("呂布", 100, 50, 2500), lead("陳宮", 50, 50, 301)
				b.duelLeaders(&ca, &ct, MainAttacker, MainDefender, func() bool { return true }, nil)
				beforeRNG, beforeA, beforeT := *b.rng, ca, ct
				old := from
				for _, to := range []i18n.Locale{i18n.En, i18n.Ja, i18n.ZhHant, from} {
					for i := range b.Speeches {
						b.Speeches[i].Relocalize(old, to)
					}
					chen := map[i18n.Locale]string{i18n.ZhHant: "陳宮", i18n.Ja: "陳宮", i18n.En: "Chen Gong"}[to]
					lu := map[i18n.Locale]string{i18n.ZhHant: "呂布", i18n.Ja: "呂布", i18n.En: "Lu Bu"}[to]
					if b.Speeches[1].Text != i18n.Tf(to, "bub.duelChallenge", chen) ||
						b.Speeches[2].Text != i18n.Tf(to, "bub.duelAccept", lu) {
						t.Fatal("實際單挑的姓名未換語言")
					}
					if *b.rng != beforeRNG || ca != beforeA || ct != beforeT {
						t.Fatal("換語言改變規則或亂數狀態")
					}
					old = to
				}
			})
		}
	}
}

func TestSpeechLocaleFallbackAndOrdinaryArguments(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	i18n.Current = i18n.ZhHant
	b := arena(flat(Plain))
	x := lead("呂布", 100, 50, 2500)
	for _, arg := range []any{speechPerson("名𠮷"), "陳宮"} {
		b.say(&x, BoxAttacker, true, "bub.duelChallenge", arg)
	}
	for i, name := range []string{"名𠮷", "陳宮"} {
		b.Speeches[i].Relocalize(i18n.ZhHant, i18n.En)
		if b.Speeches[i].Text != i18n.Tf(i18n.En, "bub.duelChallenge", name) {
			t.Fatal("未知姓名或普通字串被猜譯")
		}
	}
	for _, original := range []Speech{
		{Text: "自訂對白 AB-345", Speaker: 6, Color: 3},
		{Scene: 29, Style: 2},
		{LureFlash: true, At: FromOffset(2, 3)},
	} {
		sp := original
		sp.Relocalize(i18n.ZhHant, i18n.En)
		if !reflect.DeepEqual(sp, original) {
			t.Fatal("回退改變未知文字或特效")
		}
	}
	legacy := Speech{Text: i18n.T(i18n.ZhHant, "bub.kill")}
	legacy.Relocalize(i18n.ZhHant, i18n.En)
	if legacy.Text != i18n.T(i18n.En, "bub.kill") {
		t.Fatal("舊對白模板回退失效")
	}
	rolls := useScript(b)
	b.say(nil, BoxThird, false, "bub.kill")
	wantAsked(t, "無說話者", rolls, MessageLines)
	if len(b.Speeches) != 2 {
		t.Fatal("無說話者仍新增對白")
	}
}
