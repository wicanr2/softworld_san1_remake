package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/speaker"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func TestFullVoiceCatalogCanReadAndRenderBothOriginalBanks(t *testing.T) {
	raw, err := os.ReadFile("../../internal/speaker/voice_catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cues map[string][3]int `json:"cues"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cues) != 102 {
		t.Fatalf("dialogue coverage %d/102", len(catalog.Cues))
	}
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		t.Run(string(edition), func(t *testing.T) {
			_, v := bubbleVoiceFixture(t, edition)
			required := map[int]bool{}
			for key := range catalog.Cues {
				for person := 0; person < 350; person++ {
					indices, ok := speaker.VoiceClipsFor(key, person)
					if !ok {
						t.Fatalf("unresolved %s, general %d", key, person)
					}
					for _, index := range indices {
						required[index] = true
					}
				}
			}
			for index := range required {
				clip := v.clip(index)
				if len(clip) == 0 || len(clip) > speaker.MaxVoiceBytes {
					t.Fatalf("missing/invalid R%03d.OKR: %d bytes", index, len(clip))
				}
				pcm := speaker.Render(speaker.NewClip(clip), speaker.Rate(v.voiceDiv), audioRate)
				if len(pcm) == 0 {
					t.Fatalf("empty decoded R%03d.OKR", index)
				}
			}
			t.Logf("102 templates, 350 person slots, %d original clips fully rendered", len(required))
		})
	}
}

func TestWindowBattleVoiceAcrossGatesAndQueueGrowth(t *testing.T) {
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		for _, gates := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
			s, v := bubbleVoiceFixture(t, edition)
			v.SetGates(gates[0], gates[1])
			b := battle.New(battle.Setup{Field: &battle.Field{}, FixedWeather: true, Seed: 0x13579bdf})
			u := &battle.Unit{Leaders: []battle.Leader{{Index: 32, Name: "孔融", Soldiers: 1, Stamina: 100}}}
			if !b.SayPlotGate(u, battle.ErrPlotGold) {
				t.Fatal("plot gate did not create a speech")
			}
			f := &fight{speeches: b.TakeSpeeches()}
			if len(f.speeches) != 1 {
				t.Fatal("missing tactical speech")
			}
			a := &app{s: s, sfx: v, fight: f}
			want := voiceExpectedPCM(v, [3]int{479, 499, 499})
			a.playShownSpeechVoice()
			zero := make([]byte, 4096)
			v.mx.Read(zero)
			if !bytes.Equal(zero, make([]byte, len(zero))) {
				t.Fatal("tactical voice before draw")
			}
			a.shownSpeech = &f.speeches[0]
			a.playShownSpeechVoice()
			got := make([]byte, len(want))
			v.mx.Read(got)
			if gates[0] || gates[1] {
				want = make([]byte, len(want))
			}
			if !bytes.Equal(got, want) {
				t.Fatal("tactical voice clips or sound/voice gates differ")
			}
			f.speeches = append(append([]battle.Speech(nil), f.speeches...), battle.Speech{})
			a.shownSpeech = &f.speeches[0]
			v.SetGates(false, false)
			a.playShownSpeechVoice()
			v.mx.Read(zero)
			if !bytes.Equal(zero, make([]byte, len(zero))) {
				t.Fatal("queue growth or re-enabling replayed voice")
			}
		}
	}
}

func TestWindowAtlasVoicePlaysOnlyAfterShownAndOnlyOnce(t *testing.T) {
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		s, v := bubbleVoiceFixture(t, edition)
		b := s.G.AtlasBubble(8)
		if b == nil {
			t.Fatal("atlas has no governor dialogue")
		}
		a := &app{s: s, sfx: v}
		a.view.Atlas, a.view.AtlasBubble = 8, b
		want := voiceExpectedPCM(v, [3]int{356, 499, 499})
		zero := make([]byte, 4096)
		a.playShownBubbleVoice()
		v.mx.Read(zero)
		if !bytes.Equal(zero, make([]byte, len(zero))) {
			t.Fatal("atlas voice played before draw")
		}
		a.shownBubble = b
		a.playShownBubbleVoice()
		got := make([]byte, len(want))
		v.mx.Read(got)
		if !bytes.Equal(got, want) {
			t.Fatal("atlas did not play the complete original clips")
		}
		a.playShownBubbleVoice()
		v.mx.Read(zero)
		if !bytes.Equal(zero, make([]byte, len(zero))) {
			t.Fatal("atlas voice replayed")
		}
	}
}
