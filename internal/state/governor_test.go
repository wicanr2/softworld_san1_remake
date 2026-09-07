package state

import "testing"

// TestGovernorFieldMatchesTheGarrison 釘住州郡 offset 32 是主事者的槽號。
//
// 判準是**那個人與郡本身相符**：勢力等於郡的所屬、所在郡等於郡本身、
// 身分是君主或太守。無主的郡一律 NoGovernor。
func TestGovernorFieldMatchesTheGarrison(t *testing.T) {
	c := loadData2(t, "三國演義")
	sc, err := LoadScenario(c, Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	gens := sc.Generals()
	owned := 0
	for _, p := range sc.Prefectures() {
		if !p.Owned() {
			if p.Governor != NoGovernor {
				t.Errorf("郡 %d %s 無主，主事者卻是 %d", p.ID, p.Name, p.Governor)
			}
			continue
		}
		owned++
		if int(p.Governor) >= len(gens) {
			t.Fatalf("郡 %d %s 的主事者槽號 %d 越界", p.ID, p.Name, p.Governor)
		}
		g := gens[p.Governor]
		if g.Faction != p.Owner || int(g.Location) != p.ID {
			t.Errorf("郡 %d %s 的主事者 %s 勢力 %d 領地 %d，與郡的所屬 %d 對不上",
				p.ID, p.Name, g.Name, g.Faction, g.Location, p.Owner)
		}
		if g.Status != StatusLord && !g.Status.Governs() {
			t.Errorf("郡 %d %s 的主事者 %s 身分是 %d", p.ID, p.Name, g.Name, g.Status)
		}
	}
	if owned != 24 {
		t.Fatalf("劇本 001 有主的郡應該是 24 個，數到 %d", owned)
	}
}
