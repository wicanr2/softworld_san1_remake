package game

import "testing"

// 玩家那條的賞賜與賑民，把量到的點釘住。
//
// 出處是 `internal/parity` 的 `TestZZPlayerCharmSweep`：開機一次、存快照，
// 把主事者的魅力／賞金／給的米／人口各擺成一串值，每一格還原重試
// （`docs/playtest/04`）。**這一支不需要原版素材**，所以每次 `go test`
// 都會跑到——公式被改壞時當場紅，不必等對拍那一輪。

func TestRewardGainPlayerMatchesTheMeasurements(t *testing.T) {
	// 賞 100 金，掃主事者的魅力。
	for _, c := range []struct{ charm, want int }{
		{6, 3}, {12, 7}, {14, 8}, {24, 15}, {28, 17}, {30, 19},
		{42, 26}, {43, 27}, {51, 32}, {60, 38}, {75, 48},
		{89, 57}, {90, 57}, {99, 63},
	} {
		if got := RewardGainPlayer(c.charm, 100); got != c.want {
			t.Errorf("魅力 %d 賞 100 金：原版 %d，remake %d", c.charm, c.want, got)
		}
	}
	// 魅力 99，掃賞金。**乘完才截斷**：先把效果截成 63 再乘的話，
	// 金 30／60／90 會算出 18／37／56。
	for _, c := range []struct{ gold, want int }{
		{10, 6}, {20, 12}, {30, 19}, {40, 25}, {50, 31},
		{60, 38}, {70, 44}, {80, 50}, {90, 57}, {100, 63},
	} {
		if got := RewardGainPlayer(99, c.gold); got != c.want {
			t.Errorf("魅力 99 賞 %d 金：原版 %d，remake %d", c.gold, c.want, got)
		}
	}
}

func TestReliefGainPlayerMatchesTheMeasurements(t *testing.T) {
	// 人口 7000、給 300 米（原始增幅 42，全部被上限咬到），掃魅力。
	// 量到的就是上限本身：**魅力 ÷ 3**，不是電腦那條的 ÷ 2。
	for _, c := range []struct{ charm, want int }{
		{6, 2}, {12, 4}, {14, 4}, {24, 8}, {28, 9}, {30, 10},
		{42, 14}, {43, 14}, {51, 17}, {60, 20}, {75, 25},
		{89, 29}, {90, 30}, {99, 33},
	} {
		if got := ReliefGainPlayer(7000, 300, c.charm); got != c.want {
			t.Errorf("魅力 %d 給 300 米：原版 %d，remake %d", c.charm, c.want, got)
		}
	}
	// 人口 7000、魅力 99（上限 33），掃米：每 7 個米一格。
	for _, c := range []struct{ rice, want int }{
		{10, 1}, {25, 3}, {50, 7}, {75, 10}, {100, 14},
		{125, 17}, {150, 21}, {175, 25}, {200, 28}, {300, 33},
	} {
		if got := ReliefGainPlayer(7000, c.rice, 99); got != c.want {
			t.Errorf("給 %d 米：原版 %d，remake %d", c.rice, c.want, got)
		}
	}
	// 魅力 99、給 100 米，掃人口：每格是 (人口 ÷ 100) ÷ **10**，
	// 電腦那條是 ÷ 12。
	for _, c := range []struct{ pop, want int }{
		{3000, 33}, {5000, 20}, {8400, 12}, {10000, 10},
		{12000, 8}, {14400, 7}, {20000, 5},
	} {
		if got := ReliefGainPlayer(c.pop, 100, 99); got != c.want {
			t.Errorf("人口 %d 給 100 米：原版 %d，remake %d", c.pop, c.want, got)
		}
	}
}

func TestTrainGainPlayerMatchesTheMeasurements(t *testing.T) {
	// 訓練度從 0 起，掃智與武。**除數固定 5，不隨難度變**——
	// 電腦那條才是 `AITrainDivisor`（等級 ≤2 給 5、≤4 給 4、其餘 3）。
	for _, c := range []struct{ intel, war, want int }{
		{95, 82, 14}, {0, 0, 0}, {30, 0, 2}, {60, 0, 4}, {90, 0, 6},
		{99, 0, 6}, {0, 30, 3}, {0, 60, 6}, {0, 90, 9}, {0, 99, 9},
		{30, 30, 5}, {60, 60, 10}, {99, 99, 16}, {12, 88, 9}, {88, 12, 7},
	} {
		got := (c.intel/3 + c.war/2) / TrainDivisorPlayer
		if got != c.want {
			t.Errorf("智 %d 武 %d：原版 %d，remake %d", c.intel, c.war, c.want, got)
		}
	}
}
