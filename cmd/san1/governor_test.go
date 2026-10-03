package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func TestGovernorChoiceResumesHumanPrompt(t *testing.T) {
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("SAN1_ORIG 未設定，需要本機原版素材")
	}
	for _, edition := range []string{"base", "plus"} {
		for _, waiting := range []bool{true, false} {
			name := edition + "/computer"
			if waiting {
				name = edition + "/human"
			}
			t.Run(name, func(t *testing.T) {
				folder, ed, mode := "三國演義", state.EditionBase, ai.ModeBase
				if edition == "plus" {
					folder, ed, mode = "三國演義1加強版", state.EditionPlus, ai.ModePlus
				}
				var raw [3][]byte
				for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
					b, err := os.ReadFile(filepath.Join(root, folder, "DATA2"+ext))
					if err != nil {
						t.Fatal(err)
					}
					raw[i] = b
				}
				container, err := assets.OpenContainer(raw[0], raw[1], raw[2])
				if err != nil {
					t.Fatal(err)
				}
				scenario, err := state.LoadScenario(container, state.Scenario3)
				if err != nil {
					t.Fatal(err)
				}
				g, err := game.New(scenario, 1, 5, ed)
				if err != nil {
					t.Fatal(err)
				}
				brain, err := ai.New(mode)
				if err != nil {
					t.Fatal(err)
				}
				s := session.New(g, brain, 1)
				if waiting && s.AdvanceToHuman(1000) != 14 {
					t.Fatal("正常首次玩家停點不是弘農")
				}
				a := &app{s: s, art: &ui.ArtScreen{}}
				a.askNewGovernor(6)
				r := a.closeRoster()
				before := g.Date
				r.then(r.pick.List[0])
				if g.Date != before || s.Waiting() != map[bool]int{true: 14, false: 0}[waiting] {
					t.Fatal("太守選擇改變年月或玩家停點")
				}
				if waiting {
					if a.num == nil || a.view.Sel != 14 || !a.view.Status ||
						!strings.Contains(a.view.Prompt, "弘農") {
						t.Fatalf("補位後未恢復弘農下令：num=%v sel=%d prompt=%q", a.num, a.view.Sel, a.view.Prompt)
					}
				} else if a.num != nil || a.view.Sel != 6 {
					t.Fatal("電腦流程不應插入玩家下令")
				}
			})
		}
	}
}
