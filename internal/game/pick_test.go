package game

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestPickRosterModesAndKeys 釘住挑人清單的收人條件與排序：模式 2 鍵 0 與分派器的
// 行動者名單同一個順序；鍵 4 按忠誠由大到小；模式 1 只收在野、模式 7 只收一般武將。
func TestPickRosterModesAndKeys(t *testing.T) {
	g := newGame(t)
	at := g.Lord(0).Location
	serving := g.PickRoster(at, PickServing, PickByStatus)
	actors := g.ActorRoster(at)
	if len(serving) != len(actors) {
		t.Fatalf("模式 2 收了 %d 位，行動者名單 %d 位", len(serving), len(actors))
	}
	for i := range serving {
		if serving[i] != actors[i] {
			t.Fatalf("第 %d 位：挑人清單 %s、行動者名單 %s", i, serving[i].Name, actors[i].Name)
		}
	}
	loyal := g.PickRoster(at, PickServing, PickByLoyalty)
	for i := 1; i < len(loyal); i++ {
		if int8(loyal[i].Loyalty) > int8(loyal[i-1].Loyalty) {
			t.Errorf("忠誠排序第 %d 位 %d 大於前一位 %d", i, loyal[i].Loyalty, loyal[i-1].Loyalty)
		}
	}
	for _, x := range g.PickRoster(at, PickOfficerOnly, PickByLoyalty) {
		if x.Status != state.StatusOfficer {
			t.Errorf("模式 7 收了身分 %d 的 %s", x.Status, x.Name)
		}
	}
	for id := 1; id <= state.PrefectureCount; id++ {
		for _, x := range g.PickRoster(id, PickFree, PickByIntel) {
			if x.Status != 8 && x.Status != 10 {
				t.Errorf("模式 1 收了身分 %d 的 %s", x.Status, x.Name)
			}
		}
	}
}
