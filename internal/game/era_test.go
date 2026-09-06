package game

import "testing"

// TestEraMatchesTheSixScenarios 釘住六個劇本的年號與原版的選單標題相同。
//
// 原版的選單標題是 ` 1 中平 六 年 `…` 6 黃初 元 年 `
// （`AA.EXE` `0x475ef`–`0x47586`，`docs/re/04` §10）。年號表要在這六個
// 已知點上全部對得上，否則它連手冊都沒對到。
func TestEraMatchesTheSixScenarios(t *testing.T) {
	cases := []struct {
		year int
		text string
	}{
		{189, "中平六年元月"},
		{195, "興平二年元月"},
		{201, "建安六年元月"},
		{208, "建安十三年元月"},
		{215, "建安二十年元月"},
		{220, "黃初元年元月"},
	}
	for _, c := range cases {
		d := Date{Year: c.year, Month: 1}
		if got := d.Format(ChineseEra); got != c.text {
			t.Errorf("西元 %d 年正月寫成 %q，原版的劇本標題是 %q", c.year, got, c.text)
		}
	}
}

// TestScenarioStartsMatchTheEraTable 釘住六個劇本的起始年與年號表一致。
func TestScenarioStartsMatchTheEraTable(t *testing.T) {
	for slot, d := range ScenarioStart {
		if _, _, ok := EraOf(d.Year); !ok {
			t.Errorf("劇本 %s 起始年 %d 不在年號表裡", slot, d.Year)
		}
	}
}

// TestWesternCalendar 釘住西曆那一邊。
func TestWesternCalendar(t *testing.T) {
	d := Date{Year: 189, Month: 1}
	if got := d.Format(Western); got != "189年1月" {
		t.Errorf("西曆寫成 %q", got)
	}
	if ChineseEra.Name() != "中曆" || Western.Name() != "西曆" {
		t.Error("切換提示的兩個字要是「中曆」「西曆」（原版 0x48640／0x48645）")
	}
}

// TestChineseNumerals 釘住中文數字。
func TestChineseNumerals(t *testing.T) {
	cases := map[int]string{
		1: "一", 6: "六", 9: "九", 10: "十", 11: "十一", 13: "十三",
		19: "十九", 20: "二十", 21: "二十一", 25: "二十五", 99: "九十九",
	}
	for n, want := range cases {
		if got := Chinese(n); got != want {
			t.Errorf("%d 寫成 %q，應該是 %q", n, got, want)
		}
	}
}

// TestFirstYearAndFirstMonthAreYuan 釘住兩個「元」是不同的東西。
//
// 年號的第一年寫「元年」，一年的第一個月寫「元月」。
// 原版的劇本標題是「黃初元年」，主畫面是「中平六年元月」。
func TestFirstYearAndFirstMonthAreYuan(t *testing.T) {
	if got := (Date{Year: 220, Month: 3}).Format(ChineseEra); got != "黃初元年三月" {
		t.Errorf("黃初的第一年三月寫成 %q", got)
	}
	if got := (Date{Year: 221, Month: 1}).Format(ChineseEra); got != "黃初二年元月" {
		t.Errorf("黃初第二年正月寫成 %q", got)
	}
}

// TestEraTableIsContinuous 釘住年號表沒有洞也沒有重疊。
//
// 有洞的話那一年會退回西曆，畫面上就是一年突然變成阿拉伯數字；
// 重疊的話 EraOf 拿到的是先出現的那一個，而那不見得是對的。
func TestEraTableIsContinuous(t *testing.T) {
	list := Eras()
	for i, e := range list {
		if e.End < e.Start {
			t.Errorf("%s 的起訖是 %d..%d", e.Name, e.Start, e.End)
		}
		if i == 0 {
			continue
		}
		if prev := list[i-1]; e.Start != prev.End+1 {
			t.Errorf("%s 結束於 %d，%s 起於 %d——中間有洞或重疊",
				prev.Name, prev.End, e.Name, e.Start)
		}
	}
	// 一局最長跑六十年（`session.TestGameReachesAConclusion`），
	// 從最早的劇本 189 年算起要蓋到 249 年。
	for y := 184; y <= 249; y++ {
		if _, _, ok := EraOf(y); !ok {
			t.Errorf("西元 %d 年查不到年號", y)
		}
	}
}

// TestOutOfRangeFallsBackToWestern 釘住表外的年份退回西曆，不硬掰。
//
// **一個「太康三十七年」看起來像正常運作**，只有懂的人才看得出那是編的。
func TestOutOfRangeFallsBackToWestern(t *testing.T) {
	if _, _, ok := EraOf(400); ok {
		t.Error("西元 400 年不該查得到年號")
	}
	if got := (Date{Year: 400, Month: 5}).Format(ChineseEra); got != "400年5月" {
		t.Errorf("表外的年份寫成 %q，應該退回西曆", got)
	}
}
