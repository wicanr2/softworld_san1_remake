package battle

import (
	"fmt"
	"testing"
)

// 後果常式的骰序（Issue #24，`docs/re/05` §12.2）：每一支常式問了幾次
// `RND(n)`、依什麼順序。原版的呼叫端列在各測試的註解裡；這裡用腳本骰
// 把「問了哪幾個 n」釘住，值本身由腳本給。

// dice 讓 `UseRoll` 照表回值，並記下每一次問的 n。
type dice struct {
	vals  []int
	asked []int
}

func (s *dice) roll(n int) int {
	s.asked = append(s.asked, n)
	if len(s.vals) == 0 {
		return 0
	}
	v := s.vals[0]
	s.vals = s.vals[1:]
	return v % n
}

func useScript(b *Battle, vals ...int) *dice {
	s := &dice{vals: vals}
	b.UseRoll(s.roll)
	return s
}

func wantAsked(t *testing.T, what string, s *dice, want ...int) {
	t.Helper()
	if fmt.Sprint(s.asked) != fmt.Sprint(want) {
		t.Errorf("%s 問的骰 %v，原版是 %v", what, s.asked, want)
	}
}

// TestMeleeDice：交戰結算 `0x2a224` 進來先印一句對白（`RND(8)`），
// 沒有人被俘就只有這一擲。
func TestMeleeDice(t *testing.T) {
	b := arena(flat(Plain))
	a := place(b, MainAttacker, Centre, FromOffset(6, 6), lead("攻", 50, 50, 5000))
	place(b, MainDefender, Centre, FromOffset(6, 7), lead("守", 50, 50, 5000))
	s := useScript(b)
	if err := b.QuickBattle(a, dirBetween(t, b, a, FromOffset(6, 7))); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "快戰", s, MessageLines)
}

// TestCaptiveDiceForComputerCaptor：被擒處置 `0x259fe`。電腦捕獲方先擲
// `RND(10)`，君主一律斬首（只印一句對白）；一般將領：招降判定裡
// `RND(3)` 一擲，判定不過就照 `RND(10)` 的結果——囚禁多播一段特效。
func TestCaptiveDiceForComputerCaptor(t *testing.T) {
	// 君主：RND(10) → 判定不擲（君主直接回 0）→ 斬首 → RND(8)。
	b := arena(flat(Plain))
	b.Computer[MainDefender] = true
	lord := lead("主", 50, 50, 10)
	lord.Lord = true
	a := place(b, MainAttacker, Centre, FromOffset(6, 6), lord)
	place(b, MainDefender, Centre, FromOffset(6, 7), lead("守", 90, 90, 20000))
	s := useScript(b, 0, 5, 0)
	if err := b.QuickBattle(a, dirBetween(t, b, a, FromOffset(6, 7))); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "君主被擒", s, MessageLines, CaptiveExecuteRange, MessageLines)
	if a.Leaders[0].Fate != Executed {
		t.Errorf("君主被電腦擒住的下場是 %v，原版一律斬首", a.Leaders[0].Fate)
	}

	// 一般將領、人望不夠招降：RND(10)＝5 → 囚禁；判定 RND(3)；囚禁 RND(8)、RND(4)。
	b = arena(flat(Plain))
	b.Computer[MainDefender] = true
	b.Renown[MainDefender] = 10
	x := lead("將", 60, 40, 10)
	x.Loyalty = 80
	a = place(b, MainAttacker, Centre, FromOffset(6, 6), x)
	place(b, MainDefender, Centre, FromOffset(6, 7), lead("守", 90, 90, 20000))
	s = useScript(b, 0, 5, 0, 0, 0)
	if err := b.QuickBattle(a, dirBetween(t, b, a, FromOffset(6, 7))); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "將領被擒、囚禁", s, MessageLines, CaptiveExecuteRange, SurrenderSpread, MessageLines, EffectVariants)
	if a.Leaders[0].Fate != Jailed {
		t.Errorf("下場是 %v，RND(10)=5 該是囚禁", a.Leaders[0].Fate)
	}

	// RND(10) < 2 → 斬首，判定照擲。
	b = arena(flat(Plain))
	b.Computer[MainDefender] = true
	a = place(b, MainAttacker, Centre, FromOffset(6, 6), x)
	place(b, MainDefender, Centre, FromOffset(6, 7), lead("守", 90, 90, 20000))
	s = useScript(b, 0, 1, 0, 0)
	if err := b.QuickBattle(a, dirBetween(t, b, a, FromOffset(6, 7))); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "將領被擒、斬首", s, MessageLines, CaptiveExecuteRange, SurrenderSpread, MessageLines)
	if a.Leaders[0].Fate != Executed {
		t.Errorf("下場是 %v，RND(10)=1 該是斬首", a.Leaders[0].Fate)
	}

	// 人望夠、判定過 → 招降常式再判定一次（又一擲 RND(3)），成了就
	// 對白＋特效，人進捕獲方的部隊（最後一支還有位子的），兵是 0。
	b = arena(flat(Plain))
	b.Computer[MainDefender] = true
	b.Renown[MainDefender] = 100
	a = place(b, MainAttacker, Centre, FromOffset(6, 6), x)
	d := place(b, MainDefender, Centre, FromOffset(6, 7), lead("守", 90, 90, 20000))
	s = useScript(b, 0, 5, 0, 0, 0, 0)
	if err := b.QuickBattle(a, dirBetween(t, b, a, FromOffset(6, 7))); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "將領被擒、招降", s, MessageLines, CaptiveExecuteRange, SurrenderSpread, SurrenderSpread, MessageLines, EffectVariants)
	if a.Leaders[0].Fate != Defected {
		t.Errorf("下場是 %v，人望 100 該招降得動", a.Leaders[0].Fate)
	}
	if got, want := a.Leaders[0].Loyalty, (100-80/2)*100/100; got != want {
		t.Errorf("招降後忠誠 %d，(100 − 80÷2) × 100 ÷ 100 ＝ %d", got, want)
	}
	// 五個槽位都算、空的也算，所以落在**後軍**——守方沒有後軍就生一支。
	rear := b.unitSlot(MainDefender, Rear)
	if d.LeaderCount() != 1 || rear == nil || rear.LeaderCount() != 1 || rear.Leaders[0].Name != "將" || rear.Leaders[0].Soldiers != 0 {
		t.Errorf("招降的人該進守方新生的後軍（中軍將領 %d，後軍 %v）", d.LeaderCount(), rear)
	}
	if rear != nil && (!rear.Unplaced || !rear.Alive() || rear.Move != rear.Cap || rear.Cap == 0) {
		t.Errorf("新生的後軍該標 Unplaced、在場、移動力回滿（%+v）", rear)
	}

	// 玩家捕獲而**沒有介面**：照電腦的判斷式處置（Issue #100）。
	// 先前是「不擲、不處置」，那一位留 `FateNone`——被抓了卻什麼都沒發生，
	// 原版沒有這個狀態（它在同一刻是問人）。
	b = arena(flat(Plain))
	a = place(b, MainAttacker, Centre, FromOffset(6, 6), x)
	place(b, MainDefender, Centre, FromOffset(6, 7), lead("守", 90, 90, 20000))
	s = useScript(b)
	if err := b.QuickBattle(a, dirBetween(t, b, a, FromOffset(6, 7))); err != nil {
		t.Fatal(err)
	}
	if !a.Leaders[0].Captured {
		t.Error("被擒的旗標沒標上")
	}
	if f := a.Leaders[0].Fate; f != Executed && f != Jailed && f != Defected && f != Released {
		t.Errorf("處置是 %v，沒有介面時該走電腦那一套的四種之一", f)
	}
	// 電腦那一套會多擲一次 `RND(10)`（`0x25a93`），那正是這一條與原版
	// 的差異所在——原版在這一刻是問人、不擲。
	saw := false
	for _, n := range s.asked {
		if n == CaptiveExecuteRange {
			saw = true
		}
	}
	if !saw {
		t.Errorf("問的骰是 %v，沒有 RND(%d)——那是電腦判斷式的第一道門",
			s.asked, CaptiveExecuteRange)
	}
}

// TestSurrenderChance 釘住招降判定 `0x25e50` 的幾道門。
func TestSurrenderChance(t *testing.T) {
	b := arena(flat(Plain))
	b.Renown[MainDefender] = 60
	x := lead("將", 70, 40, 0)
	x.Loyalty = 50
	// 門檻 max(70,40)=70 ÷ (RND(3)+1)：擲 0 → 70 > 人望 60 → 不招；擲 1 → 35 → 招。
	s := useScript(b, 0)
	if got := b.surrenderChance(MainDefender, &x); got != 0 {
		t.Errorf("門檻 70 > 人望 60 卻招降了（%d）", got)
	}
	s = useScript(b, 1)
	if got, want := b.surrenderChance(MainDefender, &x), (100-25)*60/100; got != want {
		t.Errorf("門檻 35 ≤ 人望 60，忠誠該是 %d，得 %d", want, got)
	}
	wantAsked(t, "判定", s, SurrenderSpread)

	// 牽絆對象同勢力：＋1000，怎麼除都招不動。
	y := x
	y.BondAlly = true
	s = useScript(b, 2)
	if got := b.surrenderChance(MainDefender, &y); got != 0 {
		t.Errorf("牽絆對象同勢力卻招降了（%d）", got)
	}
	// 君主不擲。
	z := x
	z.Lord = true
	s = useScript(b, 2)
	if got := b.surrenderChance(MainDefender, &z); got != 0 || len(s.asked) != 0 {
		t.Errorf("君主：回 %d、問了 %v，該是 0 而且不擲", got, s.asked)
	}
	// 捕獲方將領數到 50 也不擲。
	var many []Leader
	for i := 0; i < 50; i++ {
		many = append(many, lead(fmt.Sprintf("將%d", i), 50, 50, 100))
	}
	place(b, MainDefender, Centre, FromOffset(6, 7), many[:10]...)
	place(b, MainDefender, Vanguard, FromOffset(6, 8), many[10:20]...)
	place(b, MainDefender, Left, FromOffset(7, 7), many[20:30]...)
	place(b, MainDefender, Right, FromOffset(7, 8), many[30:40]...)
	place(b, MainDefender, Rear, FromOffset(5, 7), many[40:50]...)
	s = useScript(b, 2)
	if got := b.surrenderChance(MainDefender, &x); got != 0 || len(s.asked) != 0 {
		t.Errorf("將領 50 位：回 %d、問了 %v，該是 0 而且不擲", got, s.asked)
	}
}

// TestDefenderFirstLeaderSurvivesMutualWipe：承受方第 0 槽那一位在出手方
// 打光時留下（`0x2a684`–`0x2a68e`），兩支不會在同一次結算裡一起消失。
func TestDefenderFirstLeaderSurvivesMutualWipe(t *testing.T) {
	b := arena(flat(Plain))
	a := place(b, MainAttacker, Centre, FromOffset(6, 6), lead("攻", 99, 99, 1))
	d := place(b, MainDefender, Centre, FromOffset(6, 7), lead("守一", 99, 99, 1), lead("守二", 99, 99, 1))
	useScript(b)
	b.exchange(a, d, 1)
	if a.LeaderCount() != 0 {
		t.Fatalf("一兵的攻方該打光（將領 %d）", a.LeaderCount())
	}
	if d.LeaderCount() != 1 || d.Leaders[0].Captured || !d.Leaders[1].Captured {
		t.Errorf("守方第 0 槽該留下、第 1 槽被俘（將領 %d，俘 %v %v）", d.LeaderCount(), d.Leaders[0].Captured, d.Leaders[1].Captured)
	}
	if d.Leaders[0].Soldiers != 0 {
		t.Errorf("留下那一位的兵該是 0，得 %d", d.Leaders[0].Soldiers)
	}
}

// TestArcheryDice：弓箭 `0x2a80a` 每一箭一句對白、一段特效，殺傷不擲。
func TestArcheryDice(t *testing.T) {
	b := arena(flat(Plain))
	a := place(b, MainAttacker, Centre, FromOffset(6, 6), lead("攻", 50, 50, 5000))
	target := FromOffset(6, 8)
	place(b, MainDefender, Centre, target, lead("守", 50, 50, 5000))
	s := useScript(b)
	if err := b.ArcheryOnce(a, target); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "一箭", s, MessageLines, EffectVariants)
}

// TestStratagemDice：計謀判定 `0x2abb8` 那一擲 `RND(表)`，不成印一句
// 「被看穿」；成了各計謀先印自己那一句，再擲效果。
func TestStratagemDice(t *testing.T) {
	setup := func(intel uint8) (*Battle, *Unit, Hex) {
		b := arena(flat(Plain))
		b.Gold[MainAttacker] = 5000
		u := place(b, MainAttacker, Centre, FromOffset(6, 6), lead("攻", 50, intel, 5000))
		target := FromOffset(6, 7)
		place(b, MainDefender, Centre, target, lead("守", 50, 60, 5000))
		return b, u, target
	}
	// 失敗：施法者 60 對目標 60，RND(2)+60 < 60 不成立。
	b, u, target := setup(60)
	s := useScript(b)
	if err := b.UseStratagem(u, Trap, target); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "陷阱不成", s, Trap.Spread(), MessageLines)

	// 陷阱成功（一般）：RND(2) → 對白 → RND(5)。
	b, u, target = setup(90)
	s = useScript(b)
	if err := b.UseStratagem(u, Trap, target); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "陷阱成", s, Trap.Spread(), MessageLines, TrapSpread)

	// 陷阱成功（謀略 ≥ 98）：多一擲 RND(5)。
	b, u, target = setup(99)
	s = useScript(b)
	if err := b.UseStratagem(u, Trap, target); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "陷阱成、神算", s, Trap.Spread(), MessageLines, TrapSpread, TrapSpread)

	// 燒糧：RND(4) → 對白 → 特效 → RND(3)（一般）／RND(2)（神算）。
	b, u, target = setup(90)
	s = useScript(b)
	if err := b.UseStratagem(u, Burn, target); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "燒糧成", s, Burn.Spread(), MessageLines, EffectVariants, BurnSpread)
	b, u, target = setup(99)
	s = useScript(b)
	if err := b.UseStratagem(u, Burn, target); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "燒糧成、神算", s, Burn.Spread(), MessageLines, EffectVariants, BurnGeniusSpread)

	// 火攻：RND(10) 判定 → 對白 → 特效 → RND(10) 比率；沒人被打光就沒別的。
	b, u, target = setup(90)
	b.Weather = Windy
	s = useScript(b)
	if err := b.UseStratagem(u, Fire, target); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "火攻成", s, Fire.Spread(), MessageLines, EffectVariants, StratagemRollSpread)

	// 誘敵：RND(2) → 對白 → 交戰結算（自己的對白）。
	b, u, target = setup(90)
	s = useScript(b)
	if err := b.UseStratagem(u, Lure, target); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "誘敵成", s, Lure.Spread(), MessageLines, MessageLines)

	// 圍攻：RND(6) → 對白 → 每一支貼著目標的我方各一次交戰結算。
	b, u, target = setup(90)
	place(b, MainAttacker, Vanguard, FromOffset(7, 7), lead("攻二", 50, 50, 3000))
	s = useScript(b)
	if err := b.UseStratagem(u, Siege, target); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "圍攻成", s, Siege.Spread(), MessageLines, MessageLines, MessageLines)
}

// TestFireBurnsOrCaptures：火攻把人打光之後 `RND(100) > 20` 燒死，否則
// 交給施法方處置（電腦：`RND(10)` 起）。
func TestFireBurnsOrCaptures(t *testing.T) {
	b := arena(flat(Forest))
	b.Gold[MainAttacker] = 5000
	b.Computer[MainAttacker] = true
	b.Weather = Windy
	u := place(b, MainAttacker, Centre, FromOffset(6, 6), lead("攻", 50, 99, 5000))
	target := FromOffset(6, 7)
	d := place(b, MainDefender, Centre, target, lead("守一", 50, 60, 1), lead("守二", 50, 60, 1))
	// 判定 0、對白、特效、比率 0（樹林 → 0.9）；守一 RND(100)=50 燒死；
	// 守二 RND(100)=10 被俘 → RND(10)=5 囚禁 → RND(3) → 對白、特效。
	s := useScript(b, 0, 0, 0, 0, 50, 10, 5, 0, 0, 0)
	if err := b.UseStratagem(u, Fire, target); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "火攻打光", s, Fire.Spread(), MessageLines, EffectVariants, StratagemRollSpread,
		100, 100, CaptiveExecuteRange, SurrenderSpread, MessageLines, EffectVariants)
	if !d.Leaders[0].Dead || d.Leaders[1].Fate != Jailed {
		t.Errorf("守一該燒死（%v）、守二該囚禁（%v）", d.Leaders[0].Dead, d.Leaders[1].Fate)
	}
}

// TestFireRatioKeepsTheFraction：比率是 double，神算那一乘不截整數。
func TestFireRatioKeepsTheFraction(t *testing.T) {
	// 山丘基數 25 ＋ 擲 3 ＝ 0.28 × 1.6 ＝ 0.448（不是 0.44）。
	r := FireRatio(Hill, 99, 3)
	if r < 0.4479 || r > 0.4481 {
		t.Errorf("比率 %v，該是 0.448", r)
	}
	// 0.448 的 double 比 0.448 略大，1 − 它略小於 0.552，乘 1500 落在
	// 827.99…，`ftol` 截成 827——照 x87 算就是這樣，不是 828。
	if got := ArrowSurvivors(1500, r); got != 827 {
		t.Errorf("ftol(1500 × (1 − 0.448)) ＝ 827，得 %d", got)
	}
	if r := FireRatio(Forest, 50, 9); r != StratagemMaxRatio {
		t.Errorf("樹林要夾在 0.9，得 %v", r)
	}
}

// TestRetreatDice：退兵 `0x23dd4`——對白、電腦挑去處（主守軍一律
// `RND(鄰郡數)`；其他軍力原郡裝得下就不擲）、特效。玩家不擲去處。
func TestRetreatDice(t *testing.T) {
	escapes := []Escape{{Prefecture: 1}, {Prefecture: 2}, {Prefecture: 3}}

	b := arena(flat(Plain))
	b.Computer[MainDefender] = true
	b.Escapes[MainDefender] = escapes
	u := place(b, MainDefender, Centre, FromOffset(6, 6), lead("守", 50, 50, 1000))
	s := useScript(b)
	if err := b.Retreat(u); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "主守軍退兵", s, MessageLines, 3, EffectVariants)

	b = arena(flat(Plain))
	b.Computer[MainAttacker] = true
	b.Escapes[MainAttacker] = escapes
	b.Origin[MainAttacker] = Escape{Prefecture: 9, Active: 10}
	u = place(b, MainAttacker, Centre, FromOffset(6, 6), lead("攻", 50, 50, 1000))
	s = useScript(b)
	if err := b.Retreat(u); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "主攻軍退回原郡", s, MessageLines, EffectVariants)

	b = arena(flat(Plain))
	b.Computer[MainAttacker] = true
	b.Escapes[MainAttacker] = escapes
	b.Origin[MainAttacker] = Escape{Prefecture: 9, Active: 50}
	u = place(b, MainAttacker, Centre, FromOffset(6, 6), lead("攻", 50, 50, 1000))
	s = useScript(b)
	if err := b.Retreat(u); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "主攻軍原郡滿了", s, MessageLines, 3, EffectVariants)

	b = arena(flat(Plain))
	b.Escapes[MainAttacker] = escapes
	u = place(b, MainAttacker, Centre, FromOffset(6, 6), lead("攻", 50, 50, 1000))
	s = useScript(b)
	if err := b.Retreat(u); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "玩家退兵", s, MessageLines, EffectVariants)
}

// dirBetween 找 u 到某一格的方向。
func dirBetween(t *testing.T, b *Battle, u *Unit, to Hex) Dir {
	t.Helper()
	for _, d := range Dirs() {
		if u.At.Step(d) == to {
			return d
		}
	}
	t.Fatalf("%v 與 %v 不相鄰", u.At, to)
	return 0
}

// TestDefectionDice：回合結束的投敵判定 `0x27604`——統帥以外每一位擲
// `RND(5)`，擲到 0 而且忠誠低於對方主軍人望的一半才投；投了印一句對白，
// 帶著兵進對方主軍最後一支有位子的部隊，忠誠變成 100 − 原忠誠。
func TestDefectionDice(t *testing.T) {
	b := arena(flat(Plain))
	b.Renown[MainAttacker] = 80
	lord := lead("主", 50, 50, 1000)
	lord.Index = 7
	low := lead("低", 50, 50, 500)
	low.Loyalty = 10
	high := lead("高", 50, 50, 500)
	high.Loyalty = 90
	d := place(b, MainDefender, Centre, FromOffset(6, 7), lord, low, high)
	b.Commander[MainDefender] = 7
	a := place(b, MainAttacker, Centre, FromOffset(6, 6), lead("攻", 50, 50, 1000))
	// 從第 9 槽往第 0 槽：高（RND(5)=0，忠誠 90 ≥ 40 不投）、低（RND(5)=0，
	// 10 < 40 → 對白 → 投）、統帥不擲。
	s := useScript(b, 0, 0, 0)
	b.EndTurn(d)
	wantAsked(t, "投敵判定", s, DefectionSpread, DefectionSpread, MessageLines)
	if d.LeaderCount() != 2 || !d.Leaders[1].Deserted || d.Leaders[1].Fate != Defected {
		t.Errorf("低忠誠那一位該投敵（將領 %d，Deserted %v Fate %v）", d.LeaderCount(), d.Leaders[1].Deserted, d.Leaders[1].Fate)
	}
	rear := b.unitSlot(MainAttacker, Rear)
	if a.LeaderCount() != 1 || rear == nil || rear.LeaderCount() != 1 || rear.Leaders[0].Name != "低" || rear.Leaders[0].Soldiers != 500 || rear.Leaders[0].Loyalty != 90 {
		t.Errorf("投敵的人該帶兵進攻方新生的後軍（中軍將領 %d，後軍 %v）", a.LeaderCount(), rear)
	}
	if d.Soldiers() != 1500 {
		t.Errorf("守方剩 %d 兵，該是 1500", d.Soldiers())
	}

	// RND(5) != 0 就不往下判，一擲一位。
	b = arena(flat(Plain))
	b.Renown[MainAttacker] = 80
	d = place(b, MainDefender, Centre, FromOffset(6, 7), lord, low)
	b.Commander[MainDefender] = 7
	place(b, MainAttacker, Centre, FromOffset(6, 6), lead("攻", 50, 50, 1000))
	s = useScript(b, 3)
	b.EndTurn(d)
	wantAsked(t, "沒擲到 0", s, DefectionSpread)
	if d.LeaderCount() != 2 {
		t.Errorf("沒擲到 0 不該投敵（將領 %d）", d.LeaderCount())
	}

	// 在野的忠誠是 0xFF，讀成 −1：人望 ÷ 2 只要大於 −1 就投。
	b = arena(flat(Plain))
	b.Renown[MainAttacker] = 0
	stray := lead("野", 50, 50, 500)
	stray.Loyalty = -1
	d = place(b, MainDefender, Centre, FromOffset(6, 7), lord, stray)
	b.Commander[MainDefender] = 7
	place(b, MainAttacker, Centre, FromOffset(6, 6), lead("攻", 50, 50, 1000))
	s = useScript(b, 0, 0)
	b.EndTurn(d)
	wantAsked(t, "在野投敵", s, DefectionSpread, MessageLines)
	if !d.Leaders[1].Deserted {
		t.Error("忠誠 −1 的該投")
	}
}

// TestPlayerRestDice：玩家的休息印一句對白（`0x27cb0`），電腦的休息
// （選項 9）不印。
func TestPlayerRestDice(t *testing.T) {
	b := arena(flat(Plain))
	u := place(b, MainAttacker, Centre, FromOffset(6, 6), lead("攻", 50, 50, 1000))
	s := useScript(b)
	if err := b.Rest(u); err != nil {
		t.Fatal(err)
	}
	wantAsked(t, "玩家休息", s, MessageLines)
	s = useScript(b)
	b.baseRest(u)
	wantAsked(t, "電腦休息", s)
}

// TestWeatherDice：開場 `RND(3)`；每天結束 `RND(10) > 5` 才重擲 `RND(3)`。
func TestWeatherDice(t *testing.T) {
	f := flat(Plain)
	b := New(Setup{Field: f, Seed: 1, Attackers: []Leader{lead("攻", 50, 50, 1000)},
		Defenders: []Leader{lead("守", 50, 50, 1000)}})
	b.Rice[MainAttacker], b.Rice[MainDefender] = 10000, 10000
	s := useScript(b, 3, 2)
	b.EndDay()
	wantAsked(t, "換天", s, WeatherChangeRange)
	s = useScript(b, 7, 2)
	b.EndDay()
	wantAsked(t, "換天", s, WeatherChangeRange, WeatherKinds)
	if b.Weather != Windy {
		t.Errorf("擲 2 是原版的「風」，得 %v", b.Weather)
	}
	fixed := New(Setup{Field: f, Seed: 1, Weather: Rainy, FixedWeather: true,
		Attackers: []Leader{lead("攻", 50, 50, 1000)}, Defenders: []Leader{lead("守", 50, 50, 1000)}})
	if fixed.Weather != Rainy {
		t.Errorf("指定的天候該照用，得 %v", fixed.Weather)
	}
}

// TestRefreshQuality 釘住每天輪到之前重算的綜合能力（`0x26fc6`）。
func TestRefreshQuality(t *testing.T) {
	x := lead("甲", 80, 20, 1000) // 武裝 50 訓練 50：50×5÷20=12、50×0.05=2.5、80×14÷20=56
	u := &Unit{Leaders: []Leader{x}}
	u.RefreshQuality()
	// (12 + 2.5 + 56) × 1000 × 0.01 + 0.5 = 705.5 → 705；705 ÷ 1000 × 100 + 0.5 = 71
	if u.Quality != 71 {
		t.Errorf("綜合能力 %d，該是 71", u.Quality)
	}
	// 兵數加權：一千兵的 80 戰力與十兵的 0 戰力。
	y := lead("乙", 0, 0, 10)
	u = &Unit{Leaders: []Leader{x, y}}
	u.RefreshQuality()
	// 甲 705；乙 (12 + 2.5 + 0) × 10 × 0.01 + 0.5 = 1.95 → 1；(706 ÷ 1010) × 100 + 0.5 = 70.4 → 70
	if u.Quality != 70 {
		t.Errorf("加權綜合能力 %d，該是 70", u.Quality)
	}
	u = &Unit{}
	u.RefreshQuality()
	if u.Quality != 0 {
		t.Errorf("沒兵的部隊綜合能力該是 0，得 %d", u.Quality)
	}
	// 整編那一支看的是謀略與戰力，兩套不同。
	if got := (&Unit{Leaders: []Leader{x}}).Ability(); got != 20*2/5+80*3/5 {
		t.Errorf("整編的綜合能力 %d", got)
	}
}

// TestRefreshQualityOverflows：S 與 N 是 16 位元，兩位各兩萬七的部隊
// N 溢位成負的，綜合能力是 0（盤面丙）。
func TestRefreshQualityOverflows(t *testing.T) {
	u := &Unit{Leaders: []Leader{lead("甲", 80, 20, 27000), lead("乙", 80, 20, 27000)}}
	u.RefreshQuality()
	if u.Quality != 0 {
		t.Errorf("54000 兵的部隊綜合能力該溢位成 0，得 %d", u.Quality)
	}
}
