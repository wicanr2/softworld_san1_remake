package battle

import "testing"

// 原版九支判斷式的單元測試：骰子用腳本餵（`UseRoll`），每一條測一個門。
// 與原版逐支對拍的在 `internal/parity` 的 `TestZZUnitAIDayParity`。

// script 依序回傳給定的骰值；用完就回 0。
func script(vals ...int) func(int) int {
	i := 0
	return func(n int) int {
		if i < len(vals) {
			v := vals[i]
			i++
			return v % n
		}
		return 0
	}
}

func unit(side Side, f Formation, at Hex, soldiers int, prefix string) *Unit {
	u := &Unit{Side: side, Formation: f, At: at, Move: 10, Arrows: 3}
	l := lead(prefix, 80, 80, soldiers)
	if side.Attacking() {
		l.Index = 1 + int(f)
	} else {
		l.Index = 101 + int(f)
	}
	u.Leaders = []Leader{l}
	u.Started = soldiers
	u.Quality = u.Ability()
	return u
}

// 沒有相鄰敵軍、路通到城池：攻方走過去（選項 3），守著城池且城裡的
// 部隊比城外敵軍強的守方不動（選項 3 的門），休息（選項 9）。
func TestBaseChainMovesTowardTheCityOrHolds(t *testing.T) {
	b := arena(flat(Plain))
	b.Gold[MainAttacker], b.Gold[MainDefender] = 1000, 1000
	att := unit(MainAttacker, Centre, FromOffset(0, 5), 3000, "攻")
	def := unit(MainDefender, Centre, b.Field.CityAt, 3000, "守")
	b.Units = []*Unit{att, def}

	b.UseRoll(script())
	before := att.At
	att.Move = 2 // 只走得動一步，免得走到城池旁邊把守方的門改掉
	b.autoTurnBase(att)
	if att.At == before {
		t.Fatal("攻方沒有往城池走")
	}
	if Distance(att.At, b.Field.CityAt) >= Distance(before, b.Field.CityAt) {
		t.Errorf("攻方走遠了：%v → %v", before, att.At)
	}

	// 選項 4 的門 RND(2)=1：不射箭，守方沒有相鄰敵軍就只剩休息。
	b.UseRoll(script(1))
	def.Move = 10
	b.autoTurnBase(def)
	if def.At != b.Field.CityAt {
		t.Errorf("守方離開了城池：%v", def.At)
	}
	if def.Move != 12 {
		t.Errorf("守方該休息（移動力 +2 → 12），得到 %d", def.Move)
	}
}

// 貼身而且目標弱（兵力比 ≤ 0.23）：RND(4)==0 時死戰，佔進對方那一格。
func TestBaseChainDeathBattleTakesTheCell(t *testing.T) {
	b := arena(flat(Plain))
	b.Gold[MainAttacker], b.Gold[MainDefender] = 0, 0 // 沒錢，策略一定不成立
	me := FromOffset(3, 5)
	att := unit(MainAttacker, Vanguard, me, 5000, "攻")
	def := unit(MainDefender, Vanguard, me.Step(DirDownRight), 800, "守")
	hold := unit(MainDefender, Centre, b.Field.CityAt, 3000, "守")
	b.Units = []*Unit{att, def, hold}
	// 選項 1 RND(1)=0；選項 2 的門看兵力（不擲）；選項 3 路被守軍擋住
	// 走得動就走——這裡把攻方移動力歸零讓它走不了；選項 4 RND(2)=1
	// 不射；選項 5 RND(3)=0 不用計；選項 6 RND(4)=0 → 死戰。
	att.Move = 0
	b.UseRoll(script(0, 1, 0, 0))
	b.autoTurnBase(att)
	if def.Alive() {
		t.Fatalf("死戰沒打到底：守方還有 %d 兵", def.Soldiers())
	}
	if att.At != me.Step(DirDownRight) {
		t.Errorf("死戰贏了該佔進對方那一格，攻方在 %v", att.At)
	}
}

// 貼身、目標不弱、其餘的門都沒開：快戰（選項 8）打一次。
func TestBaseChainQuickBattleIsTheFallback(t *testing.T) {
	b := arena(flat(Plain))
	me := FromOffset(3, 5)
	att := unit(MainAttacker, Vanguard, me, 3000, "攻")
	def := unit(MainDefender, Vanguard, me.Step(DirDownRight), 3000, "守")
	b.Units = []*Unit{att, def}
	att.Move = 0
	// 選項 1 RND(1)；選項 4 RND(2)=1；選項 5 RND(3)=0；選項 6 RND(4)=1；
	// 選項 7 RND(16)=1；→ 快戰。
	b.UseRoll(script(0, 1, 0, 1, 1))
	was := def.Soldiers()
	b.autoTurnBase(att)
	if def.Soldiers() >= was {
		t.Errorf("快戰沒打：守方兵力 %d → %d", was, def.Soldiers())
	}
	if !def.Alive() {
		t.Error("快戰只結算一次，不該打光對方")
	}
}

// 退兵：相鄰敵軍是自己的四倍以上、敵我總兵力比夠大、有鄰郡可逃。
func TestBaseChainRetreatNeedsAnEscape(t *testing.T) {
	mk := func(escape bool) (*Battle, *Unit) {
		b := arena(flat(Plain))
		me := FromOffset(3, 5)
		att := unit(MainAttacker, Vanguard, me, 500, "攻")
		def := unit(MainDefender, Vanguard, me.Step(DirDownRight), 9000, "守")
		b.Units = []*Unit{att, def}
		if escape {
			b.Escapes[MainAttacker] = []Escape{{Prefecture: 7, Active: 10}}
		}
		return b, att
	}
	// 選項 1 RND(1)=0；選項 2：RND(2)=0 → 除數 4，500 ≤ 9000÷4；
	// RND(8)=0 → 3 ≤ 90 → 退。
	b, att := mk(true)
	b.UseRoll(script(0, 0, 0))
	b.autoTurnBase(att)
	if !att.Retreated {
		t.Error("該退兵而沒退")
	}
	b, att = mk(false)
	b.UseRoll(script(0, 0, 0))
	b.autoTurnBase(att)
	if att.Retreated {
		t.Error("沒有鄰郡可逃卻退兵了")
	}
}

// 弓箭：同方向連走兩步的落點有敵軍、中間不是城池／關寨／大山。
func TestBaseChainArcheryTwoStepsInLine(t *testing.T) {
	b := arena(flat(Plain))
	me := FromOffset(3, 5)
	att := unit(MainAttacker, Vanguard, me, 3000, "攻")
	far := me.Step(DirDownRight).Step(DirDownRight)
	def := unit(MainDefender, Vanguard, far, 3000, "守")
	b.Units = []*Unit{att, def}
	att.Move = 0 // 走不動，才輪得到弓箭
	// 選項 1：沒有相鄰敵軍不擲；選項 4：RND(2)=0 進，RND(1)=0 挑。
	b.UseRoll(script(0, 0))
	was := def.Soldiers()
	b.autoTurnBase(att)
	if def.Soldiers() >= was {
		t.Errorf("沒射到：守方兵力 %d → %d", was, def.Soldiers())
	}
	if att.Arrows != 2 {
		t.Errorf("箭該少一次，剩 %d", att.Arrows)
	}
	// 中間隔著關寨就不射，改休息。
	b.Field.Set(me.Step(DirDownRight), Fort)
	att.Arrows, att.Move = 3, 0
	b.UseRoll(script(0, 0))
	b.autoTurnBase(att)
	if att.Arrows != 3 || att.Move != 2 {
		t.Errorf("隔著關寨該休息：箭 %d、移動力 %d", att.Arrows, att.Move)
	}
}

// 預設就是原版那條鏈；enhanced 要明說。
func TestAutoTurnDispatchesByAI(t *testing.T) {
	if (Setup{}).AI != AIBase {
		t.Error("Setup 的零值該是原版的判斷式")
	}
}
