package session

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
)

// TestNoInterfaceCapturesHappen 量「玩家那一方抓到人、卻沒有介面可問」
// 在無畫面的路上到底發生幾次（Issue #100 的先量再改）。
//
// **這不是驗收，是量測**：數字是 0 的話，改用電腦的判斷式不會動到
// 這條路的骰序；不是 0 的話就得先回報再決定。
func TestNoInterfaceCapturesHappen(t *testing.T) {
	battle.ResetNoInterfaceCaptives()
	s := newSession(t, ai.ModeEnhanced, 0) // 劇本一的劉備
	for i := 0; i < 36; i++ {
		s.EndMonth()
		if s.Over {
			t.Logf("第 %d 個月結束了這一局", i+1)
			break
		}
	}
	t.Logf("三十六個月裡「玩家捕獲而沒有介面」發生 %d 次；打過 %d 場",
		battle.NoInterfaceCaptives(), len(s.battles))
}
