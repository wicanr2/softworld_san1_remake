package battle

import (
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"testing"
)

func TestBattleVoiceSnapshotSurvivesQueueGrowthAndLocale(t *testing.T) {
	old := i18n.Current
	defer func() { i18n.Current = old }()
	b := arena(flat(Plain))
	x := lead("將領", 100, 50, 2500)
	x.Index = 99
	b.say(&x, BoxAttacker, true, "bub.duelChallenge", namedSpeechPerson("同名武將", 32))
	sp := &b.Speeches[0]
	if got, ok := sp.ConsumeVoiceClips(); !ok || got != [3]int{32, 433, 499} {
		t.Fatalf("cue %v/%v", got, ok)
	}
	// Force storage to move; an address-based deduplication would replay the line.
	before := &b.Speeches[0]
	for &b.Speeches[0] == before {
		b.Speeches = append(b.Speeches, Speech{})
	}
	for _, locale := range i18n.Locales() {
		b.Speeches[0].Relocalize(i18n.ZhHant, locale)
		if got, ok := b.Speeches[0].VoiceClips(); !ok || got != [3]int{32, 433, 499} {
			t.Fatal("snapshot changed")
		}
		if _, ok := b.Speeches[0].ConsumeVoiceClips(); ok {
			t.Fatal("queue growth or locale replayed voice")
		}
	}
	for _, sp := range []*Speech{nil, {Scene: 5}, {LureFlash: true}} {
		if _, ok := sp.ConsumeVoiceClips(); ok {
			t.Fatal("non-dialogue voiced")
		}
	}
}

func TestBattleVoiceDoesNotGuessAnIndexFromAName(t *testing.T) {
	b := arena(flat(Plain))
	x := lead("將領", 100, 50, 2500)
	b.say(&x, BoxThird, false, "bub.seen", speechPerson("孔融"))
	if _, ok := b.Speeches[0].VoiceClips(); ok {
		t.Fatal("name-only argument gained an invented general index")
	}
	b.say(&x, BoxThird, false, "bub.seen", namedSpeechPerson("孔融", 349))
	if got, ok := b.Speeches[1].VoiceClips(); !ok || got != [3]int{349, 428, 499} {
		t.Fatal("original index was replaced with name identity")
	}
}
