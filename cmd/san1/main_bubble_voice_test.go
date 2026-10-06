package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/speaker"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func bubbleVoiceFixture(t *testing.T, edition state.Edition) (*session.Session, *voicebox) {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("SAN1_ORIG 未設定，需要本機原版素材")
	}
	folder, mode := "三國演義", ai.ModeBase
	if edition == state.EditionPlus {
		folder, mode = "三國演義1加強版", ai.ModePlus
	}
	c2, err := openContainer(filepath.Join(root, folder), "DATA2")
	if err != nil {
		t.Fatal(err)
	}
	c3, err := openContainer(filepath.Join(root, folder), "DATA3")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(c2, state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	g, err := game.New(sc, 0, 5, edition)
	if err != nil {
		t.Fatal(err)
	}
	g.SeedRand(0x13579bdf)
	brain, err := ai.New(mode)
	if err != nil {
		t.Fatal(err)
	}
	s := session.New(g, brain, 0)
	s.Queue(g.WarDeclaration(8, 7, 0))
	if len(s.Bubbles) != 2 {
		t.Fatal("正常宣戰沒有兩則對白")
	}
	bank := &speaker.Bank{}
	v := &voicebox{bank: bank, mx: speaker.NewMixer(bank, audioRate), voice: []*assets.Container{c2, c3}, voiceDiv: speaker.VoiceDivisor}
	return s, v
}

func voiceExpectedPCM(v *voicebox, clips [3]int) []byte {
	var result []byte
	for _, n := range clips {
		for _, x := range speaker.Render(speaker.NewClip(v.clip(n)), speaker.Rate(v.voiceDiv), audioRate) {
			var frame [4]byte
			binary.LittleEndian.PutUint16(frame[:2], uint16(x))
			binary.LittleEndian.PutUint16(frame[2:], uint16(x))
			result = append(result, frame[:]...)
		}
	}
	return result
}

func TestWindowBubbleVoiceOnceAcrossLocalesAndGates(t *testing.T) {
	old := i18n.Current
	defer func() { i18n.Current = old }()
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		for _, locale := range i18n.Locales() {
			for _, gates := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
				t.Run(string(edition)+"/"+string(locale)+"/"+string(rune('0'+boolInt(gates[0])*2+boolInt(gates[1]))), func(t *testing.T) {
					i18n.Current = locale
					s, v := bubbleVoiceFixture(t, edition)
					v.SetGates(gates[0], gates[1])
					a := &app{s: s, sfx: v}
					seed, draws := s.G.RandSeed(), s.G.RandDraws()
					a.playShownBubbleVoice()
					if v.mx.Busy() {
						t.Fatal("畫面尚未顯示就播語音")
					}
					for _, clips := range [][3]int{{32, 456, 499}, {0, 457, 499}} {
						b := s.Bubble()
						a.shownBubble = b
						a.playShownBubbleVoice()
						if v.mx.Busy() != (!gates[0] && !gates[1]) {
							t.Fatal("語音開關未共同生效")
						}
						for _, to := range i18n.Locales() {
							b.Relocalize(locale, to)
							a.playShownBubbleVoice()
						}
						want := voiceExpectedPCM(v, clips)
						if gates[0] || gates[1] {
							clear(want)
						}
						got := make([]byte, len(want)+4096)
						if n, err := v.mx.Read(got); err != nil || n != len(got) {
							t.Fatal("語音來源讀取失敗")
						}
						if !bytes.Equal(got[:len(want)], want) || !bytes.Equal(got[len(want):], make([]byte, 4096)) {
							t.Fatal("完整三段波形、順序或重播控制不符")
						}
						v.SetGates(false, false)
						a.playShownBubbleVoice()
						if v.mx.Busy() {
							t.Fatal("開關開啟補播了舊對白")
						}
						s.PopBubble()
						a.playShownBubbleVoice()
						if v.mx.Busy() {
							t.Fatal("已收掉的對白被播出")
						}
						v.SetGates(gates[0], gates[1])
					}
					if s.G.RandSeed() != seed || s.G.RandDraws() != draws {
						t.Fatal("語音觸發改變遊戲亂數")
					}
				})
			}
		}
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestWindowBubbleVoiceLeavesSaveBytesUnchanged(t *testing.T) {
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		t.Run(string(edition), func(t *testing.T) {
			s, v := bubbleVoiceFixture(t, edition)
			dir := t.TempDir()
			name := s.SaveName(1, 8, "")
			if err := s.Save(dir, 1, name); err != nil {
				t.Fatal(err)
			}
			before := mainBubbleSaveFiles(t, dir)
			a := &app{s: s, sfx: v, shownBubble: s.Bubble()}
			a.playShownBubbleVoice()
			if err := s.Save(dir, 1, name); err != nil {
				t.Fatal(err)
			}
			for file, after := range mainBubbleSaveFiles(t, dir) {
				if !bytes.Equal(before[file], after) {
					t.Fatalf("語音改寫存檔 %s", file)
				}
			}
		})
	}
}

func TestVoiceboxRejectsIncompleteAndInvalidCue(t *testing.T) {
	_, v := bubbleVoiceFixture(t, state.EditionBase)
	for _, cue := range [][3]int{{32, 0, -1}, {32, 0, 500}} {
		v.Say(cue)
		if v.mx.Busy() {
			t.Fatal("越界索引被夾成別的語音")
		}
	}
	v.voice = v.voice[:1] // DATA2 有前兩段，DATA3 的 R499 缺漏。
	v.Say([3]int{32, 0, 499})
	if v.mx.Busy() || v.bank.Clip(1).Len() != 0 || v.bank.Clip(2).Len() != 0 {
		t.Fatal("缺第三段時播出了半句或更新了舊槽")
	}
	(*voicebox)(nil).Say([3]int{32, 456, 499})
}
