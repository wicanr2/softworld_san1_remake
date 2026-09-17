package ui

import "testing"

// TestMarchLayoutFollowsTheOriginal 釘住 `0x1ecfc` 的版面與四種方位（`docs/spec/005`
// 「大地圖上的戰役」）。前兩列是對拍那一局量到的兩場。
func TestMarchLayoutFollowsTheOriginal(t *testing.T) {
	for _, c := range []struct {
		ax, ay, dx, dy             int
		x, y, w, h                 int
		att, attMask, def, defMask int
	}{
		{52, 92, 98, 107, 96, 100, 104, 72, 10, 18, 0, 16},    // 往右（郡 19 → 17）
		{161, 160, 161, 188, 208, 172, 64, 88, 14, 22, 4, 20}, // 往下（郡 27 → 29）
		{98, 107, 52, 92, 96, 100, 104, 72, 8, 16, 2, 18},     // 往左
		{161, 188, 161, 160, 208, 172, 64, 88, 12, 20, 6, 22}, // 往上
		{20, 10, 30, 12, 72, 28, 72, 64, 10, 18, 0, 16},       // 座標小於 32：往零截
	} {
		l := NewMarchLayout(c.ax, c.ay, c.dx, c.dy)
		if l.X != c.x || l.Y != c.y || l.W != c.w || l.H != c.h ||
			l.Att != c.att || l.AttMask != c.attMask || l.Def != c.def || l.DefMask != c.defMask {
			t.Errorf("(%d,%d)→(%d,%d)：得 %+v", c.ax, c.ay, c.dx, c.dy, l)
		}
	}
}
