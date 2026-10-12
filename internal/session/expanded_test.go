package session

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func TestExpandedAITurnsAndSaveAcrossScenarios(t *testing.T) {
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("需要 SAN1_ORIG 兩版素材")
	}
	for _, ed := range []struct {
		edition state.Edition
		folder  string
		mode    ai.Mode
	}{{state.EditionBase, "三國演義", ai.ModeBase}, {state.EditionPlus, "三國演義1加強版", ai.ModePlus}} {
		var data [3][]byte
		for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
			b, err := os.ReadFile(filepath.Join(root, ed.folder, "DATA2"+ext))
			if err != nil {
				t.Fatal(err)
			}
			data[i] = b
		}
		container, err := assets.OpenContainer(data[0], data[1], data[2])
		if err != nil {
			t.Fatal(err)
		}
		for n := 1; n <= 6; n++ {
			t.Run(fmt.Sprintf("%s/%03d", ed.edition, n), func(t *testing.T) {
				sc, err := state.LoadScenario(container, state.Slot(fmt.Sprintf("%03d", n)))
				if err != nil {
					t.Fatal(err)
				}
				g, err := game.NewExpanded(sc, state.NoFaction, 5, ed.edition)
				if err != nil {
					t.Fatal(err)
				}
				g.SeedRand(0x13579bdf)
				brain, err := ai.New(ed.mode)
				if err != nil {
					t.Fatal(err)
				}
				s := New(g, brain, state.NoFaction)
				start := g.Date
				for month := 0; month < 36; month++ {
					s.EndMonth()
					if _, err = g.CaptureSnapshot(); err != nil {
						t.Fatalf("第 %d 月完整快照：%v", month+1, err)
					}
				}
				if g.Date.Year != start.Year+3 || g.Date.Month != start.Month {
					t.Fatal("AI 回合未推進三年")
				}
				dir := t.TempDir()
				if err = s.Save(dir, 1, "擴充 AI 驗證"); err != nil {
					t.Fatal(err)
				}
				loaded, err := Load(dir, 1, ed.mode)
				if err != nil || !loaded.G.Expanded() || len(loaded.G.AllGenerals()) != len(g.AllGenerals()) {
					t.Fatalf("AI 三年後無法完整載入：%v", err)
				}
				if info := save.List(dir); !info[0].Exists {
					t.Fatal("格式 2 存檔未列入選單")
				}
			})
		}
	}
}

func TestExpandedNewLordFirstTurn(t *testing.T) {
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("需要 SAN1_ORIG 兩版素材")
	}
	for _, ed := range []struct {
		edition state.Edition
		folder  string
		mode    ai.Mode
	}{{state.EditionBase, "三國演義", ai.ModeBase}, {state.EditionPlus, "三國演義1加強版", ai.ModePlus}} {
		var data [3][]byte
		for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
			var err error
			data[i], err = os.ReadFile(filepath.Join(root, ed.folder, "DATA2"+ext))
			if err != nil {
				t.Fatal(err)
			}
		}
		c, err := assets.OpenContainer(data[0], data[1], data[2])
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			slot state.Slot
			city int
		}{{"001", 5}, {"002", 23}, {"006", 20}} {
			t.Run(string(ed.edition)+"/"+string(tc.slot), func(t *testing.T) {
				sc, err := state.LoadScenario(c, tc.slot)
				if err != nil {
					t.Fatal(err)
				}
				g, err := game.NewExpanded(sc, 16, 5, ed.edition)
				if err != nil {
					t.Fatal(err)
				}
				brain, err := ai.New(ed.mode)
				if err != nil {
					t.Fatal(err)
				}
				s := New(g, brain, 16)
				// 受控合法月順序先輪到首府，隔離 AI 在玩家前出兵的戰果。
				s.MonthOrder = []int{tc.city}
				for id := 0; id <= 42; id++ {
					if id != tc.city {
						s.MonthOrder = append(s.MonthOrder, id)
					}
				}
				at := s.AdvanceToHuman(43)
				if at != tc.city || s.Waiting() != tc.city {
					t.Fatalf("首月沒有停在玩家首府：at=%d waiting=%d date=%v lord=%+v pref=%+v", at, s.Waiting(), g.Date, g.Lord(16), g.Prefecture(tc.city))
				}
				if tc.slot == "001" {
					dead := g.Lord(16)
					dead.Faction, dead.Status = state.NoFaction, state.StatusFallen
					g.SucceedLord(16)
					_, heirs := g.NeedsHeir()
					if len(heirs) == 0 {
						t.Fatal("缺少繼承測試候選")
					}
					cursor, date := s.MonthCursor, g.Date
					if at := s.AdvanceToHuman(86); at != 0 || s.MonthCursor != cursor || g.Date != date {
						t.Fatal("未回答繼承選擇前，回合仍繼續推進")
					}
					if err := g.AssignHeir(heirs[0]); err != nil {
						t.Fatal(err)
					}
					g.Faction(16).Lord, g.Faction(16).Alive = -1, false
					s.MonthCursor = len(s.MonthOrder)
					s.AdvanceToHuman(1)
					if !s.Over {
						t.Fatal("玩家在本月回合中絕嗣，月底未結束遊戲")
					}
				}
				uncontrolled, err := game.NewExpanded(sc, 16, 5, ed.edition)
				if err != nil {
					t.Fatal(err)
				}
				us := New(uncontrolled, brain, 16)
				at = us.AdvanceToHuman(43 * 12)
				_, pendingHeirs := uncontrolled.NeedsHeir()
				if at == 0 && !us.Over && len(pendingHeirs) == 0 && uncontrolled.NeedsGovernor() == 0 {
					t.Fatal("未控制順序的開局未返回指令、選擇或遊戲結束")
				}
				for _, f := range uncontrolled.Factions() {
					if f.ID >= 16 && f.Alive {
						lord := uncontrolled.Lord(f.ID)
						if lord == nil || lord.Faction != f.ID || lord.Status != state.StatusLord {
							t.Fatalf("戰後新增勢力引用錯誤君主：%+v", lord)
						}
					}
				}
			})
		}
	}
}
