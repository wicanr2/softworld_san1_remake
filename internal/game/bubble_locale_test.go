package game

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func TestBubblePersonLocaleSnapshot(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	keys := []string{"bub.recruitAsk", "bub.recruitYes", "bub.chiefOrder", "bub.governorOrder",
		"bub.governorReply", "bub.autonomyOrder", "bub.autonomyReply", "bub.giftThanks",
		"bub.headhuntYes", "bub.found", "bub.warDeclare", "bub.warReply", "bub.debutBond",
		"bub.debut", "bub.adv.plotEasy", "bub.adv.plotHard"}
	for _, key := range keys {
		for _, from := range i18n.Locales() {
			t.Run(key+"/"+string(from), func(t *testing.T) {
				i18n.Current = from
				g := &State{}
				g.SeedRand(0x13579bdf)
				x := &General{Index: 7, Name: "呂布", Location: 8}
				g.sayName(x, true, false, key, &General{Name: "關羽", Index: -1}, 8, 417)
				x.Name = "張飛"
				b := g.PendingEvents()[0].Bubble
				name := map[i18n.Locale]string{i18n.ZhHant: "關羽", i18n.Ja: "関羽", i18n.En: "Guan Yu"}
				if b.Text != i18n.Tf(from, key, name[from]) || g.RandDraws() != 1 {
					t.Fatal("初次文字或字色抽樣改變")
				}
				before, seed := *b, g.RandSeed()
				old := from
				for _, to := range []i18n.Locale{i18n.En, i18n.Ja, i18n.ZhHant, i18n.En, from} {
					b.Relocalize(old, to)
					if b.Text != i18n.Tf(to, key, name[to]) {
						t.Fatalf("%s→%s: %q", old, to, b.Text)
					}
					before.Text = b.Text
					if !reflect.DeepEqual(*b, before) || g.RandSeed() != seed || g.RandDraws() != 1 {
						t.Fatal("切換改變快照、幾何或亂數")
					}
					old = to
				}
			})
		}
	}
}

func TestPendingBubbleLocaleKeepsTablesAndSuccessionColor(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		t.Run(string(edition), func(t *testing.T) {
			i18n.Current = i18n.ZhHant
			g, err := New(loadScenario(t, state.Scenario1), 0, 5, edition)
			if err != nil {
				t.Fatal(err)
			}
			g.SeedRand(0x13579bdf)
			g.SucceedLord(0)
			if len(g.heirAsks) != 1 {
				t.Fatal("沒有玩家繼承停點")
			}
			color := g.heirAsks[0].Color
			seed, draws := g.RandSeed(), g.RandDraws()
			if err := g.AssignHeir(g.heirAsks[0].List[0]); err != nil {
				t.Fatal(err)
			}
			if g.RandSeed() != seed || g.RandDraws() != draws || len(g.pending) != 2 || g.pending[1].Bubble.Color != color {
				t.Fatal("選定繼承人新增字色抽樣")
			}
			g.pending = append(g.pending, g.sceneEvent(8, assets.SceneAppoint, 2, 432, 80), Event{Text: "固定紀錄"})
			before := append([]Event(nil), g.pending...)
			mas, sta, gen, err := g.Tables()
			if err != nil {
				t.Fatal(err)
			}
			old := i18n.ZhHant
			for _, to := range []i18n.Locale{i18n.En, i18n.Ja, i18n.ZhHant} {
				g.RelocalizePendingBubbles(old, to)
				if g.pending[0].Bubble.Text != i18n.T(to, "bub.lordDeath") || g.pending[1].Bubble.Text != i18n.T(to, "bub.succeed") {
					t.Fatal("無參數繼承對白含錯誤格式或未換語言")
				}
				m, s, n, err := g.Tables()
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(m, mas) || !bytes.Equal(s, sta) || !bytes.Equal(n, gen) || g.RandSeed() != seed || g.RandDraws() != draws {
					t.Fatal("換語言改變資料表或亂數")
				}
				if !reflect.DeepEqual(before, g.pending) {
					t.Fatal("換語言改變事件指標、順序或固定紀錄")
				}
				old = to
			}
			if out := g.PendingEvents(); len(out) != len(before) || g.pending != nil {
				t.Fatal("事件移交改變")
			}
		})
	}
}

func TestBubbleLocaleFallback(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	i18n.Current = i18n.ZhHant
	var absent *Bubble
	absent.Relocalize(i18n.ZhHant, i18n.En)
	for _, b := range []Bubble{{Text: "自訂 AB-345"}, {Scene: 5, Style: 2}, {Card: true, Speaker: 6},
		{FaceOnly: true, WipeIn: true, Style: 3}, {Panel: 8}, {MapBattle: &MapBattle{Attacker: 8, Defender: 11, Days: 3}}} {
		before := b
		b.Relocalize(i18n.ZhHant, i18n.En)
		if !reflect.DeepEqual(before, b) {
			t.Fatal("未知文字或非對白事件改變")
		}
	}
	g := &State{}
	g.SeedRand(1)
	x := &General{Name: "呂布", Index: 6}
	g.sayName(nil, true, false, "bub.chiefOrder", &General{Name: "關羽", Index: -1})
	g.sayName(&General{}, true, false, "bub.chiefOrder", &General{Name: "關羽", Index: -1})
	if len(g.pending) != 0 || g.RandDraws() != 0 {
		t.Fatal("無說話者仍排入或擲骰")
	}
	e := g.nameBubbleEvent(x, true, false, "bub.chiefOrder", &General{Name: "名𠮷", Index: -1})
	e.Bubble.Relocalize(i18n.ZhHant, i18n.En)
	if e.Bubble.Text != i18n.Tf(i18n.En, "bub.chiefOrder", "名𠮷") {
		t.Fatal("未知姓名被猜譯")
	}
	for _, key := range []string{"bub.chiefReply", "bub.death", "bub.epilogue", "bub.lordDeath", "bub.succeed"} {
		b := &Bubble{Text: i18n.T(i18n.ZhHant, key)}
		b.Relocalize(i18n.ZhHant, i18n.En)
		if b.Text != i18n.T(i18n.En, key) {
			t.Fatalf("%s 靜態模板回譯失效", key)
		}
	}
}
