package speaker

import "testing"

func TestVoiceCueOriginalSlotOrderAndBounds(t *testing.T) {
	for _, row := range []struct {
		key    string
		person int
		want   [3]int
	}{
		{"bub.warDeclare", 32, [3]int{32, 456, 499}},
		{"bub.warReply", 0, [3]int{0, 457, 499}},
		{"bub.recruitAsk", 349, [3]int{390, 349, 391}},
		{"bub.recruitYes", 32, [3]int{393, 32, 394}},
		{"bub.rewardThanks", -1, [3]int{397, 499, 499}},
		{"bub.camp", -1, [3]int{430, 499, 499}},
	} {
		got, ok := VoiceClipsFor(row.key, row.person)
		if !ok || got != row.want {
			t.Fatalf("%s: %v/%v, want %v", row.key, got, ok, row.want)
		}
	}
	for _, key := range []string{"bub.missing", "bat.sideLine", "date.withSeason", "tre.sealRow", "unit.nameBare"} {
		if _, ok := VoiceClipsFor(key, 32); ok {
			t.Fatalf("non-dialogue key voiced: %s", key)
		}
	}
	for _, index := range []int{-1, 350, 499, 999} {
		if _, ok := VoiceClipsFor("bub.warDeclare", index); ok {
			t.Fatalf("invalid person index accepted: %d", index)
		}
	}
	got, _ := VoiceClipsFor("bub.warDeclare", 32)
	got[0] = 999
	if next, _ := VoiceClipsFor("bub.warDeclare", 32); next[0] != 32 {
		t.Fatal("caller mutated the catalog")
	}
}
