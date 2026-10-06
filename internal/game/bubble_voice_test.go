package game

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

func TestBubbleVoiceUsesRawAddresseeSnapshot(t *testing.T) {
	old := i18n.Current
	defer func() { i18n.Current = old }()
	for _, locale := range i18n.Locales() {
		for _, row := range []struct {
			key, speaker, addressee string
			clips                   [3]int
		}{
			{"bub.warDeclare", "劉備", "孔融", [3]int{32, 456, 499}},
			{"bub.warReply", "孔融", "劉備", [3]int{0, 457, 499}},
		} {
			t.Run(string(locale)+"/"+row.key, func(t *testing.T) {
				i18n.Current = locale
				g := &State{}
				g.SeedRand(0x13579bdf)
				x := &General{Index: 99, Name: row.speaker}
				b := g.nameBubbleEvent(x, true, false, row.key, row.addressee).Bubble
				x.Name = "曹操"
				seed, draws := g.RandSeed(), g.RandDraws()
				for _, to := range i18n.Locales() {
					b.Relocalize(locale, to)
					got, ok := b.VoiceClips()
					if !ok || got != row.clips || g.RandSeed() != seed || g.RandDraws() != draws {
						t.Fatal("語音映射使用了顯示姓名、人物槽或額外亂數")
					}
				}
			})
		}
	}
}

func TestBubbleVoiceUnknownAndSpecialViewsStaySilent(t *testing.T) {
	g := &State{}
	for _, row := range []struct{ key, speaker, addressee string }{
		{"bub.warDeclare", "曹操", "孔融"},
		{"bub.warDeclare", "劉備", "曹操"},
		{"bub.warReply", "劉備", "孔融"},
		{"bub.chiefOrder", "劉備", "孔融"},
	} {
		b := g.nameBubbleEvent(&General{Name: row.speaker}, true, false, row.key, row.addressee).Bubble
		if _, ok := b.VoiceClips(); ok {
			t.Fatalf("未知對白被套入已知片段：%+v", row)
		}
	}
	known := g.nameBubbleEvent(&General{Name: "劉備"}, true, false, "bub.warDeclare", "孔融").Bubble
	for _, mutate := range []func(*Bubble){
		func(b *Bubble) { b.FaceOnly = true },
		func(b *Bubble) { b.Card = true },
		func(b *Bubble) { b.Scene = 5 },
		func(b *Bubble) { b.Panel = 8 },
		func(b *Bubble) { b.MapBattle = &MapBattle{} },
	} {
		b := *known
		mutate(&b)
		if _, ok := b.VoiceClips(); ok {
			t.Fatal("特殊畫面觸發了語音")
		}
	}
	if _, ok := (*Bubble)(nil).VoiceClips(); ok {
		t.Fatal("nil 對白觸發了語音")
	}
}
