package battle

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 加強版九支判斷式的單元測試（`docs/re/05` §12.5）：骰子用腳本餵，
// 每一條釘一個門或一個目標。與原版逐支對拍的在 `internal/parity` 的
// `TestZZUnitAIDayParityPlus`。

// plusArena 開一場加強版的戰役。
func plusArena(difficulty int) *Battle {
	b := arena(flat(Plain))
	b.AI = AIPlus
	b.Difficulty = difficulty
	b.Rules = RulesFor(state.EditionPlus, difficulty)
	b.Gold[MainAttacker], b.Gold[MainDefender] = 0, 0
	return b
}

// 移動那一道門：天數要到表一，再擲 RND(表二) 要是 0。帥隊第十天以前
// 連骰都不擲；前軍第一天起每天擲一次 RND(1)。
func TestPlusChainMoveGateByDay(t *testing.T) {
	b := plusArena(5)
	att := unit(MainAttacker, Centre, FromOffset(0, 5), 3000, "攻")
	def := unit(MainDefender, Centre, b.Field.CityAt, 3000, "守")
	b.Units = []*Unit{att, def}

	// 第 1 天的帥隊：門沒開 → 弓箭 RND(2)、沒有目標 → 休息。
	s := useScript(b, 0)
	got := b.DecideBase(att)
	if got.Option != 9 {
		t.Errorf("第 1 天帥隊該休息，選了 %d", got.Option)
	}
	wantAsked(t, "第 1 天帥隊", s, 2)

	// 第 10 天的帥隊：RND(3)==0 才動。
	b.Day = 10
	att.Move = 2
	s = useScript(b, 1, 0)
	got = b.DecideBase(att)
	if got.Option != 9 {
		t.Errorf("RND(3)=1 的帥隊該休息，選了 %d", got.Option)
	}
	wantAsked(t, "第 10 天帥隊 RND(3)=1", s, 3, 2)
	s = useScript(b, 0, 0)
	got = b.DecideBase(att)
	if got.Option != 3 {
		t.Errorf("RND(3)=0 的帥隊該移動，選了 %d（%v）", got.Option, got.Why)
	}
	wantAsked(t, "第 10 天帥隊 RND(3)=0", s, 3)

	// 前軍第 1 天就擲 RND(1)。
	b.Day = 1
	van := unit(MainAttacker, Vanguard, FromOffset(0, 3), 3000, "前")
	b.Units = append(b.Units, van)
	van.Move = 2
	s = useScript(b, 0, 0)
	got = b.DecideBase(van)
	if got.Option != 3 {
		t.Errorf("第 1 天前軍該移動，選了 %d", got.Option)
	}
	wantAsked(t, "第 1 天前軍", s, 1)
}

// 行軍目標：難度 10 以下的守方（守著城池、主帥沒被貼身）圍著自己的
// 帥隊，帥隊旁邊沒有敵軍就不動；敵軍貼上帥隊而且帥隊壓不住（兵力（百）
// ≤ 2 × 鄰格敵軍）才往那支敵軍走。
func TestPlusChainGathersAroundTheChief(t *testing.T) {
	b := plusArena(5)
	b.Day = 3
	chief := unit(MainDefender, Centre, b.Field.CityAt, 1500, "守帥")
	left := unit(MainDefender, Left, FromOffset(2, 2), 1500, "守左")
	att := unit(MainAttacker, Centre, FromOffset(0, 8), 20000, "攻")
	b.Units = []*Unit{chief, left, att}

	// 敵軍沒貼上帥隊：左軍不動（RND(1) 照擲、弓箭門 RND(2)）。
	s := useScript(b, 0, 0)
	got := b.DecideBase(left)
	if got.Option != 9 || left.At != FromOffset(2, 2) {
		t.Errorf("帥隊旁邊沒有敵軍，左軍該原地休息，選了 %d 在 %v", got.Option, left.At)
	}
	wantAsked(t, "沒貼上", s, 1, 2)

	// 敵軍貼上帥隊：20000 兵（百）× 2 壓過帥隊的 15 → 往敵軍那一格走。
	att.At = b.Field.CityAt.Step(Dirs()[0])
	before := Distance(left.At, att.At)
	useScript(b, 0, 0)
	got = b.DecideBase(left)
	if got.Option != 3 {
		t.Fatalf("敵軍貼上帥隊，左軍該移動，選了 %d", got.Option)
	}
	if Distance(left.At, att.At) >= before {
		t.Errorf("左軍沒有往貼上帥隊的敵軍走：%v → 距離 %d（原 %d）", left.At, Distance(left.At, att.At), before)
	}
}

// 守著城池而總兵力比 ≤ 0.2（我方壓倒性多）：帥隊以外的部隊直撲對方帥隊。
func TestPlusChainChargesTheFoeChiefWhenOverwhelming(t *testing.T) {
	b := plusArena(5)
	b.Day = 3
	chief := unit(MainDefender, Centre, b.Field.CityAt, 30000, "守帥")
	left := unit(MainDefender, Left, FromOffset(2, 2), 30000, "守左")
	att := unit(MainAttacker, Centre, FromOffset(9, 8), 1000, "攻")
	b.Units = []*Unit{chief, left, att}
	before := Distance(left.At, att.At)
	useScript(b, 0, 0)
	got := b.DecideBase(left)
	if got.Option != 3 {
		t.Fatalf("該直撲對方帥隊，選了 %d", got.Option)
	}
	if Distance(left.At, att.At) >= before {
		t.Errorf("左軍沒有往對方帥隊走：%v", left.At)
	}
}

// 沒守城的一方（攻方）：帥隊旁邊沒有敵軍就往對方帥隊走；難度 ≥ 11 而
// 對方帥隊那一位不是君主時改往城池。守方帥隊擺在城外，兩個目標才分得開。
func TestPlusChainHardDifficultyGoesForTheCity(t *testing.T) {
	b := plusArena(15)
	b.Day = 3
	def := unit(MainDefender, Centre, FromOffset(10, 1), 3000, "守")
	att := unit(MainAttacker, Centre, FromOffset(0, 8), 3000, "攻")
	van := unit(MainAttacker, Vanguard, FromOffset(2, 8), 3000, "前")
	b.Units = []*Unit{def, att, van}
	van.Move = 2
	toCity, toDef := Distance(van.At, b.Field.CityAt), Distance(van.At, def.At)
	useScript(b, 0, 0)
	got := b.DecideBase(van)
	if got.Option != 3 {
		t.Fatalf("該往城池走，選了 %d", got.Option)
	}
	if Distance(van.At, b.Field.CityAt) >= toCity {
		t.Errorf("前軍沒有往城池走：%v", van.At)
	}

	// 對方帥隊那一位是君主：目標換成對方帥隊那一格。
	def.Leaders[0].Lord = true
	van.At = FromOffset(2, 8)
	van.Move = 2
	useScript(b, 0, 0)
	got = b.DecideBase(van)
	if got.Option != 3 {
		t.Fatalf("該往對方帥隊走，選了 %d", got.Option)
	}
	if Distance(van.At, def.At) >= toDef {
		t.Errorf("前軍沒有往對方帥隊走：%v", van.At)
	}

	// 難度 5：不看身分，一律往對方帥隊。
	b.Difficulty = 5
	def.Leaders[0].Lord = false
	van.At = FromOffset(2, 8)
	van.Move = 2
	useScript(b, 0, 0)
	b.DecideBase(van)
	if Distance(van.At, def.At) >= toDef {
		t.Errorf("難度 5 的前軍沒有往對方帥隊走：%v", van.At)
	}
}

// 退兵少一道門：只剩 RND(3)+3 < 總兵力比（嚴格），M 照算不比。
func TestPlusChainRetreatDice(t *testing.T) {
	b := plusArena(5)
	b.Escapes[MainAttacker] = []Escape{{Prefecture: 2}}
	att := unit(MainAttacker, Centre, FromOffset(6, 6), 1000, "攻")
	def := unit(MainDefender, Centre, FromOffset(6, 7), 4000, "守")
	b.Units = []*Unit{att, def}
	// 總兵力比 ＝ (100×40＋1) ÷ (100×10＋1) ≈ 4.0：RND(3)=0 → 3 < 4.0 退；
	// RND(3)=1 → 4 < 4.0 不成立。
	s := useScript(b, 0, 0)
	got := b.DecideBase(att)
	if got.Option != 2 {
		t.Errorf("RND(3)=0 該退兵，選了 %d", got.Option)
	}
	// 玩家那一邊的退兵不擲「逃向哪一郡」：對白 RND(8)、特效 RND(4)。
	wantAsked(t, "退兵", s, 1, 3, 8, 4)

	b = plusArena(5)
	b.Escapes[MainAttacker] = []Escape{{Prefecture: 2}}
	att = unit(MainAttacker, Centre, FromOffset(6, 6), 1000, "攻")
	def = unit(MainDefender, Centre, FromOffset(6, 7), 4000, "守")
	b.Units = []*Unit{att, def}
	s = useScript(b, 0, 1)
	got = b.DecideBase(att)
	if got.Option == 2 {
		t.Error("RND(3)=1 時 4 < 4.0 不成立，不該退兵")
	}
	if s.asked[1] != 3 {
		t.Errorf("退兵那一擲該是 RND(3)，問了 %v", s.asked)
	}
}

// 快戰／對戰看的是目標兵力比：守著城池時 0.5／0.4，否則 5.0／2.0 或
// RND(3)==1；死戰的門檻 0.4；模式用 難度 mod 11。
func TestPlusChainMeleeThresholds(t *testing.T) {
	// 攻方沒守城（es:0xa6 != 0）：目標兵力比 6.0 > 5.0 → RND(3)==1 才快戰。
	b := plusArena(5)
	att := unit(MainAttacker, Centre, FromOffset(6, 6), 1000, "攻")
	def := unit(MainDefender, Centre, FromOffset(6, 7), 6000, "守")
	b.Units = []*Unit{att, def}
	// 骰序：選項 1 RND(1)、退兵 RND(3)、弓箭 RND(2)、策略 RND(4)、
	// 死戰 RND(4)、對戰 RND(8)、快戰的 RND(3)。
	s := useScript(b, 0, 2, 0, 0, 1, 1, 2)
	got := b.DecideBase(att)
	if got.Option != 9 {
		t.Errorf("目標六倍強、RND(3)=2：該休息，選了 %d", got.Option)
	}
	wantAsked(t, "快戰不成立", s, 1, 3, 2, 4, 4, 8, 3)

	b = plusArena(5)
	att = unit(MainAttacker, Centre, FromOffset(6, 6), 1000, "攻")
	def = unit(MainDefender, Centre, FromOffset(6, 7), 6000, "守")
	b.Units = []*Unit{att, def}
	s = useScript(b, 0, 2, 0, 0, 1, 1, 1)
	got = b.DecideBase(att)
	if got.Option != 8 {
		t.Errorf("RND(3)=1：該快戰，選了 %d", got.Option)
	}
	wantAsked(t, "快戰成立", s, 1, 3, 2, 4, 4, 8, 3, MessageLines)

	// 死戰：目標兵力比 0.3 在原版的 0.23 之外、加強版的 0.4 之內。
	b = plusArena(5)
	att = unit(MainAttacker, Centre, FromOffset(6, 6), 10000, "攻")
	def = unit(MainDefender, Centre, FromOffset(6, 7), 3000, "守")
	b.Units = []*Unit{att, def}
	useScript(b, 0, 2, 0, 0, 0)
	got = b.DecideBase(att)
	if got.Option != 6 {
		t.Errorf("目標兵力比 0.3、RND(4)=0：該死戰，選了 %d（%v）", got.Option, got.Why)
	}
}

// 加強版的規則旗標：交戰的地形表、弓箭的尺度、綜合能力的加法順序。
func TestPlusRulesTables(t *testing.T) {
	r := RulesFor(state.EditionPlus, 5)
	if !r.MeleeTerrainPlus || !r.ArrowHalfScale || !r.QualityAddsWarFirst {
		t.Errorf("加強版的三條規則沒全開：%+v", r)
	}
	if r.DefenderCommanderLossIgnored {
		t.Error("難度 5 不該有難度 11–20 那一條")
	}
	b := plusArena(5)
	for _, c := range []struct {
		t        Terrain
		att, def int
	}{{Shallow, 20, 18}, {Deep, 15, 15}, {City, 35, 45}, {Fort, 30, 35}, {Plain, 25, 20}} {
		if got := b.meleeAttackValue(c.t); got != c.att {
			t.Errorf("%v 的攻值 %d，加強版是 %d", c.t, got, c.att)
		}
		if got := b.meleeDefendValue(c.t); got != c.def {
			t.Errorf("%v 的守值 %d，加強版是 %d", c.t, got, c.def)
		}
	}
	if b.arrowScale() != ArrowScalePlus {
		t.Errorf("弓箭尺度 %g，加強版是 %g", b.arrowScale(), ArrowScalePlus)
	}
	base := arena(flat(Plain))
	if base.arrowScale() != MeleeDefendScale || base.meleeAttackValue(City) != 40 {
		t.Error("原版的表被改到了")
	}
}
