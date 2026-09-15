package game

import "testing"

// TestPrestigeDriftRoundsTowardZero 釘住 `0x16e20`–`0x16e2c` 那段的取整：
// MSC 的帶號 `÷ 32` 是向零，不是向下——負數那一側差一格。
func TestPrestigeDriftRoundsTowardZero(t *testing.T) {
	cases := []struct {
		charm, sum, land, want int
	}{
		{100, 100 * 3, 3, 3}, // 100 + 100 − 100 ＝ 100 → 3
		{50, 50 * 2, 2, 0},   // 50 + 50 − 100 ＝ 0 → 0
		{40, 40 * 2, 2, 0},   // −20 → 向零 ＝ 0（向下會是 −1）
		{20, 20 * 4, 4, -1},  // −60 → −1
		{0, 0, 1, -3},        // −100 → −3（−100 ÷ 32 ＝ −3.125，向零）
		{90, 0, 0, 0},        // 沒有領地不調
		{70, 60*2 + 7, 2, 1}, // 和 127 ÷ 2 ＝ 63（整數除）：70 + 63 − 100 ＝ 33 → 1
	}
	for _, c := range cases {
		if got := PrestigeDrift(c.charm, c.sum, c.land); got != c.want {
			t.Errorf("PrestigeDrift(%d, %d, %d) ＝ %d，應為 %d", c.charm, c.sum, c.land, got, c.want)
		}
	}
}
