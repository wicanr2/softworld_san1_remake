package game

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func TestExpandedBattleSuccessionAfterCaptiveFates(t *testing.T) {
	for _, ed := range []state.Edition{state.EditionBase, state.EditionPlus} {
		for _, hasHeir := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/surviving-heir-%t", ed, hasHeir), func(t *testing.T) {
				g, err := NewExpanded(expandedScenario(t, "001", ed), 16, 5, ed)
				if err != nil {
					t.Fatal(err)
				}
				lord := g.Lord(16)
				var captive, survivor *General
				for i := range g.generals {
					x := &g.generals[i]
					if x.Faction != 16 || x.Index == lord.Index {
						continue
					}
					if captive == nil {
						captive = x
					} else if hasHeir && survivor == nil {
						survivor = x
					} else {
						x.Faction, x.Status = state.NoFaction, state.StatusAvailable
					}
				}
				if captive == nil {
					t.Fatal("缺少俘虜測試人物")
				}
				captive.Charm = 100
				if survivor != nil {
					survivor.Charm, survivor.Location = 1, 35
				}
				attacker := g.Lord(5)
				p := g.prepare(15, 5, []*General{attacker}, []*General{lord, captive}, 5, HalfSupply(), Aid{})
				p.B.Over, p.B.AttackerWon = true, true
				for _, u := range p.B.Units {
					for i := range u.Leaders {
						l := &u.Leaders[i]
						switch l.Index {
						case lord.Index:
							l.Dead, l.Soldiers = true, 0
						case captive.Index:
							l.Captured, l.CapturedBy, l.Fate, l.ReleasedTo = true, battle.MainAttacker, battle.Released, -1
						}
					}
				}
				g.settle(p)
				if captive.Faction != state.NoFaction || captive.Status != state.StatusAvailable {
					t.Fatalf("被釋放的部下狀態不符：%+v", captive)
				}
				if hasHeir {
					if g.Lord(16) != survivor {
						t.Fatalf("繼承者應只包含戰後仍在本勢力的人：%+v", g.Lord(16))
					}
					_, list := g.NeedsHeir()
					if len(list) != 1 || list[0] != survivor.Index {
						t.Fatalf("戰後繼承清單不符：%v", list)
					}
				} else if g.Lord(16) != nil || g.Faction(16).Alive {
					t.Fatalf("部下已全數離開，勢力仍引用在野人物：%+v", g.Lord(16))
				}
			})
		}
	}
}

func expandedScenario(t *testing.T, slot state.Slot, ed state.Edition) *state.Scenario {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("需要兩版原始劇本素材")
	}
	dir := "三國演義"
	if ed == state.EditionPlus {
		dir = "三國演義1加強版"
	}
	read := func(ext string) []byte {
		b, err := os.ReadFile(filepath.Join(root, dir, "DATA2."+ext))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	c, err := assets.OpenContainer(read("NAM"), read("IDX"), read("GRP"))
	if err != nil {
		t.Fatal(err)
	}
	sc, err := state.LoadScenario(c, slot)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

func TestExpandedAllScenarios(t *testing.T) {
	counts := []int{220, 207, 198, 180, 175, 168}
	unborn := []int{146, 134, 124, 112, 99, 85}
	forces := []int{8, 5, 1, 0, 0, 1}
	for _, ed := range []state.Edition{state.EditionBase, state.EditionPlus} {
		for n := 1; n <= 6; n++ {
			t.Run(fmt.Sprintf("%s/%03d", ed, n), func(t *testing.T) {
				sc := expandedScenario(t, state.Slot(fmt.Sprintf("%03d", n)), ed)
				original, err := New(sc, 0, 5, ed)
				if err != nil {
					t.Fatal(err)
				}
				g, err := NewExpanded(sc, 0, 5, ed)
				if err != nil {
					t.Fatal(err)
				}
				if !g.Expanded() || len(g.AllGenerals()) != 350+counts[n-1] || len(g.Factions()) != len(original.Factions())+forces[n-1] {
					t.Fatal("擴充容量不符")
				}
				for i := 0; i < 346; i++ {
					a, b := original.General(i), g.General(i)
					if a.Name != b.Name || a.Age != b.Age || a.Stamina != b.Stamina || a.War != b.War || a.Intel != b.Intel || a.Charm != b.Charm || a.Lifespan != b.Lifespan || a.Portrait != b.Portrait {
						t.Fatalf("原人物 %d 姓名或能力改變", i)
					}
					if a.Status <= state.StatusOfficer && (a.Status != b.Status || a.Location != b.Location || a.Faction != b.Faction) {
						t.Fatalf("原駐軍 %d 改變", i)
					}
				}
				for _, p := range original.Prefectures() {
					after := g.Prefecture(p.ID)
					if p.Owned() && p.Owner != after.Owner {
						t.Fatalf("原領土 %d 改變", p.ID)
					}
					if p.Gold != after.Gold || p.Rice != after.Rice || p.Population != after.Population {
						t.Fatalf("原資源 %d 改變", p.ID)
					}
				}
				seen := map[string]bool{}
				young := 0
				for _, p := range g.AllGenerals()[350:] {
					if p.Source == nil || seen[p.Source.ID] || p.Source.Scenario != n || p.Stamina != 80 || p.Lifespan != 68 || p.Debut != 15 {
						t.Fatal("新增人物來源或預設不符")
					}
					seen[p.Source.ID] = true
					if p.SignedAge() < 15 {
						young++
						if p.Status != state.StatusUnborn || p.Employed() {
							t.Fatal("未成年已登場")
						}
					}
				}
				if young != unborn[n-1] {
					t.Fatalf("未登場 %d", young)
				}
				if n == 1 && (!seen["san5:222"] || !seen["san5:452"]) {
					t.Fatal("兩位馬忠被合併")
				}
				if n == 2 {
					if p := g.General(103); p.Name != "王朗" || p.Faction != 16 || p.Location != 23 || p.Status != state.StatusLord {
						t.Fatal("王朗例外不符")
					}
				}
				if _, err := g.CaptureSnapshot(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestExpandedNewFactionPlayerAndDebut(t *testing.T) {
	g, err := NewExpanded(expandedScenario(t, state.Scenario1, state.EditionBase), 16, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	if g.Lord(16).Name != "張角" || !g.IsHuman(16) || g.Lord(16).Location != 5 {
		t.Fatal("新勢力無法作玩家")
	}
	var child *General
	for i := 350; i < len(g.generals); i++ {
		if g.generals[i].Status == state.StatusUnborn {
			child = &g.generals[i]
			break
		}
	}
	if child == nil {
		t.Fatal("缺少登場候補")
	}
	child.Age = 14
	g.Date = Date{Year: 189, Month: 12}
	g.EndMonth()
	if child.SignedAge() != 15 || (child.Status != state.StatusAvailable && child.Status != state.StatusIdle) || child.Location != child.Origin || child.Employed() {
		t.Fatalf("十五歲未按原事件入口登場：%+v", child)
	}
}

// 可控局面抽測兩條主要戰鬥路徑；正常 GUI 另由 verify-expanded-roster-inner.py 驗證。
func TestExpandedSpecialOfficerRecruitAppointAndBattle(t *testing.T) {
	for _, tactical := range []bool{false, true} {
		t.Run(fmt.Sprintf("tactical=%v", tactical), func(t *testing.T) {
			g, err := NewExpanded(expandedScenario(t, state.Scenario1, state.EditionBase), 23, 5, state.EditionBase)
			if err != nil {
				t.Fatal(err)
			}
			g.SeedRand(0x13579bdf)
			var physician *General
			for _, x := range g.AllGenerals()[350:] {
				if x.Source.Portrait == 492 {
					physician = x
				}
			}
			if physician == nil || physician.Status != state.StatusAvailable || physician.Location != 12 {
				t.Fatal("華佗沒有以一般在野武將加入")
			}
			p := g.Prefecture(12)
			p.Gold, p.Rice, p.Population = 10000, 10000, 100000
			g.Faction(23).Prestige = 100
			if err = (RecruitOrder{At: 12, Target: physician.Index}).Apply(g, 23); err != nil {
				t.Fatal(err)
			}
			p.Commanded = false
			if err = (AppointChiefOrder{At: 12, Target: physician.Index}).Apply(g, 23); err != nil {
				t.Fatal(err)
			}
			p.Commanded = false
			if err = g.Conscript(12, physician.Index, 200, 23); err != nil {
				t.Fatal(err)
			}
			if !physician.Employed() || g.Faction(23).Chief != physician.Index || physician.Soldiers != 200 {
				t.Fatal("特殊人物無法任命或帶兵")
			}
			p.Commanded = false
			to := 0
			for _, id := range p.Neighbours {
				if g.Prefecture(id).Owner != 23 {
					to = id
					break
				}
			}
			if to == 0 {
				t.Fatal("缺少戰鬥鄰郡")
			}
			var result *BattleResult
			if tactical {
				pending, err := g.BeginAttack(12, to, []int{physician.Index}, 23, HalfSupply())
				if err != nil {
					t.Fatal(err)
				}
				if head := pending.B.Commander[0]; head != physician.Index {
					t.Fatal("新增人物索引未傳入戰術層")
				}
				pending.B.Auto()
				result = g.FinishAttack(pending)
			} else {
				g.Players, g.Player = nil, state.NoFaction
				result, err = g.Attack(12, to, []int{physician.Index}, 23)
				if err != nil {
					t.Fatal(err)
				}
			}
			if result == nil || result.From != 12 || result.To != to || physician.Source.ID != "san5:492" {
				t.Fatal("戰鬥結算未保留新增人物")
			}
			if _, err = g.CaptureSnapshot(); err != nil {
				t.Fatal("戰後快照：", err)
			}
		})
	}
}
