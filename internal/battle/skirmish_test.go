package battle

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// 對戰子畫面（`skirmish.go`）的單測：版型、佈局、電腦的判斷式與骰序、
// 玩家的命令、結束的處置。骰序的對照是 `TestZZUnitAIDayParity` 盤面丁
// 拍到的那一場（`docs/re/05` §10.4）。

// TestSkirmishTemplatesMatchTheExe：十六張版型與 `AA.EXE` 檔案位移
// `0x47c22` 起的 1920 個位元組逐一相同。沒掛素材就跳過（本儲存庫不含
// 原版檔案）。
func TestSkirmishTemplatesMatchTheExe(t *testing.T) {
	d := os.Getenv("SAN1_ORIG")
	if d == "" {
		t.Skip("沒設 SAN1_ORIG，跳過（本儲存庫不含原版檔案）")
	}
	b, err := os.ReadFile(filepath.Join(d, "三國演義", "AA.EXE"))
	if err != nil {
		t.Skip(err)
	}
	const off = 0x47c22
	if len(b) < off+16*SkirmishRows*SkirmishCols {
		t.Fatalf("AA.EXE 只有 %d 個位元組", len(b))
	}
	bad := 0
	for i := 0; i < 16; i++ {
		for r := 0; r < SkirmishRows; r++ {
			for c := 0; c < SkirmishCols; c++ {
				want := int(b[off+(i*SkirmishRows+r)*SkirmishCols+c])
				if got := templateByte(i, r, c); got != want {
					bad++
					if bad < 10 {
						t.Errorf("版型 %d (%d,%d)：remake %02x，AA.EXE %02x", i, c, r, got, want)
					}
				}
			}
		}
	}
	if bad > 0 {
		t.Errorf("共 %d 格不同", bad)
	}
}

// skirmishPair 擺一場對戰：攻方在 (6,6)、守方在 (6,7)，都在同一種地形上。
func skirmishPair(t Terrain, att, def []Leader) (*Battle, *Unit, *Unit) {
	b := arena(flat(t))
	b.Difficulty = 5
	a := place(b, MainDefender, Left, FromOffset(6, 6), att...)
	d := place(b, MainAttacker, Centre, FromOffset(6, 7), def...)
	b.Computer[MainDefender] = true
	return b, a, d
}

// weakLead 是盤面丁那種將領：戰力 1、訓練與武裝 0，戰力值 0、移動力 11。
func weakLead(name string, soldiers int, stamina uint8) Leader {
	l := lead(name, 1, 1, soldiers)
	l.Training, l.Arms, l.Stamina = 0, 0, stamina
	return l
}

// TestSkirmishSetupFollowsTheOriginal：版型照守方所在格的地形挑，攻方的
// 起點寫死，守方的起點是版型的高四位；戰力值與移動力進來時算一次。
func TestSkirmishSetupFollowsTheOriginal(t *testing.T) {
	b, a, d := skirmishPair(Shallow, []Leader{weakLead("攻", 4978, 85), lead("攻二", 50, 50, 1000)}, []Leader{weakLead("守", 2334, 82)})
	s := b.NewSkirmish(a, d)
	if s.Layout != 2 || s.Narrow {
		t.Fatalf("淺水的寬圖應該是版型 2，是 %d（窄 %v）", s.Layout, s.Narrow)
	}
	g := s.Gens[SkirmishAttacker][0]
	if g == nil || g.Col != 0 || g.Row != 3 || g.MoveCap != 11 || g.Left != 11 || g.Power != 0 || g.StaminaCap != 85 {
		t.Errorf("攻方第 0 位：%+v", g)
	}
	if g2 := s.Gens[SkirmishAttacker][1]; g2 == nil || g2.Col != 1 || g2.Row != 3 || g2.MoveCap != 11 {
		t.Errorf("攻方第 1 位：%+v", g2)
	} else if want := LeaderPower(50, 50, TroopLand, Shallow, true); g2.Power != want {
		t.Errorf("攻方第 1 位戰力值 %d，應該是 %d", g2.Power, want)
	}
	if g := s.Gens[SkirmishDefender][0]; g == nil || g.Col != 11 || g.Row != 2 {
		t.Errorf("守方第 0 位：%+v", g)
	}
	if s.Occ[3][0] != 10 || s.Occ[3][1] != 11 || s.Occ[2][11] != 0 {
		t.Errorf("佔位：(0,3)=%d (1,3)=%d (11,2)=%d", s.Occ[3][0], s.Occ[3][1], s.Occ[2][11])
	}
	if s.Map[0][2] != 0xf4 || s.Map[7][0] != 0xff || s.terrainAt(2, 0) != 4 {
		t.Errorf("子地圖：(2,0)=%02x (0,7)=%02x", s.Map[0][2], s.Map[7][0])
	}
	// 移動力的公式與上限。
	if SkirmishMove(97, 97) != 11 || SkirmishMove(100, 0) != 15 || SkirmishMove(0, 100) != 1 {
		t.Errorf("移動力：%d %d %d", SkirmishMove(97, 97), SkirmishMove(100, 0), SkirmishMove(0, 100))
	}
}

// TestSkirmishNarrowLayout：主戰場 (8,0) 在圖外就是窄圖，版型加一、攻方
// 用另一組起點。
func TestSkirmishNarrowLayout(t *testing.T) {
	data := make([]byte, FieldBytes)
	for i := range data {
		x := i % FieldW
		data[i] = 0xf7
		if x >= 8 {
			data[i] = 0xff
		}
	}
	f, err := Load(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	b := arena(f)
	a := place(b, MainDefender, Left, FromOffset(3, 3), weakLead("攻", 100, 50))
	d := place(b, MainAttacker, Centre, FromOffset(3, 4), weakLead("守", 100, 50))
	s := b.NewSkirmish(a, d)
	if s.Layout != 11 || !s.Narrow {
		t.Fatalf("平原的窄圖應該是版型 11，是 %d（窄 %v）", s.Layout, s.Narrow)
	}
	if g := s.Gens[SkirmishAttacker][0]; g.Col != 3 || g.Row != 9 {
		t.Errorf("窄圖攻方第 0 位在 (%d,%d)，應該是 (3,9)", g.Col, g.Row)
	}
	if g := s.Gens[SkirmishDefender][0]; g.Col != 4 || g.Row != 0 {
		t.Errorf("窄圖守方第 0 位在 (%d,%d)，應該是 (4,0)", g.Col, g.Row)
	}
}

// TestSkirmishMatchesTheTracedFight 是盤面丁拍到的那一場（`docs/re/05`
// §10.4）：攻方一位（兵 4978、體 85）從 (0,3) 出發，每一時刻擲
// `RND(30)`、`RND(3)`、`RND(16)`，一路往 (11,2) 的守方走（淺水一格 4 步，
// 一時刻走兩格），第 11 時貼上之後每一時刻 `RND(20)` 不成單挑就攻擊
//（對白 `RND(8)`），戰力值 0 打不掉兵；守方（玩家）每一時刻休息，剩餘
// 行動力 11→13→15→17 之後夾在 15 再加 2。結束時體能寫回進來時的值。
func TestSkirmishMatchesTheTracedFight(t *testing.T) {
	b, a, d := skirmishPair(Shallow, []Leader{weakLead("攻", 4978, 85)}, []Leader{weakLead("守", 2334, 82)})
	b.PlayerSkirmish = func(*Skirmish, *SkirmishGeneral) SkirmishCommand { return SkirmishCommand{Kind: SkirmishRest} }
	dice := useScript(b, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1)
	s := b.NewSkirmish(a, d)
	g, p := s.Gens[SkirmishAttacker][0], s.Gens[SkirmishDefender][0]
	want := [][2]int{{2, 4}, {4, 4}, {6, 4}, {8, 4}, {10, 3}}
	s.Run()
	var asked []int
	for hour := SkirmishFirstHour; hour <= 10; hour++ {
		asked = append(asked, skirmishActRange, skirmishFleeSpread, skirmishEngageGate)
	}
	for hour := 11; hour <= SkirmishLastHour; hour++ {
		asked = append(asked, skirmishActRange, skirmishFleeSpread, skirmishDuelSpread, MessageLines)
	}
	wantAsked(t, "對戰", dice, asked...)
	if g.Col != 10 || g.Row != 3 {
		t.Errorf("攻方最後在 (%d,%d)，原版是 (10,3)", g.Col, g.Row)
	}
	// 每一時刻走到哪（從 Trace 讀）。
	got := [][2]int{}
	for _, l := range s.Trace {
		var c, r, cost int
		if n, _ := fmt.Sscanf(l, "    走到 (%d,%d) 花 %d", &c, &r, &cost); n == 3 {
			got = append(got, [2]int{c, r})
		}
	}
	if len(got) != 10 {
		t.Fatalf("走了 %d 格：%v", len(got), got)
	}
	for i, w := range want {
		if got[i*2+1] != w {
			t.Errorf("第 %d 時走到 %v，原版是 %v；全部 %v", 6+i, got[i*2+1], w, got)
		}
	}
	if s.Hour != SkirmishLastHour+1 {
		t.Errorf("結束在時刻 %d，應該是 19", s.Hour)
	}
	if g.Leader.Soldiers != 4978 || p.Leader.Soldiers != 2334 {
		t.Errorf("戰力值 0 不該掉兵：攻 %d 守 %d", g.Leader.Soldiers, p.Leader.Soldiers)
	}
	if g.Leader.Stamina != 85 || p.Leader.Stamina != 82 {
		t.Errorf("結束時體能要寫回：攻 %d 守 %d", g.Leader.Stamina, p.Leader.Stamina)
	}
	if p.Left != 17 {
		t.Errorf("守方休息累積的剩餘行動力是 %d，原版是 17", p.Left)
	}
	if g.Leader.Captured || p.Leader.Captured {
		t.Errorf("沒有人該被抓")
	}
}

// TestSkirmishRestsWhenTheActRollFails：難度 0 時 `RND(30)` 要 ≤ 10 才動；
// 沒過就休息（體能加 戰力÷20＋1 不超過進來時的值、剩餘行動力加 2），
// 這一步不再擲骰。
func TestSkirmishRestsWhenTheActRollFails(t *testing.T) {
	b, a, d := skirmishPair(Plain, []Leader{weakLead("攻", 1000, 40)}, []Leader{weakLead("守", 1000, 40)})
	b.Difficulty = 0
	b.PlayerSkirmish = func(*Skirmish, *SkirmishGeneral) SkirmishCommand { return SkirmishCommand{Kind: SkirmishRest} }
	dice := useScript(b, 11)
	s := b.NewSkirmish(a, d)
	g := s.Gens[SkirmishAttacker][0]
	s.step(g)
	wantAsked(t, "沒過的一步", dice, skirmishActRange)
	if g.Left != g.MoveCap+2 || g.Leader.Stamina != 40 {
		t.Errorf("休息之後 步 %d 體 %d", g.Left, g.Leader.Stamina)
	}
	// 體能不到 10 的自動休息連 RND(30) 都不擲。
	g.Leader.Stamina = 5
	g.StaminaCap = 30
	dice = useScript(b)
	s.step(g)
	wantAsked(t, "體能不足", dice)
	if g.Leader.Stamina != 6 {
		t.Errorf("自動休息後體能 %d，應該是 5 + 1÷20 + 1 = 6", g.Leader.Stamina)
	}
}

// TestSkirmishFleesFromAStrongerNeighbour：兵不到最強鄰敵的 1/(RND(3)+1)
// 就往「最強鄰敵方向 + RND(3) + 2」走一格；走得成這一步就結束。
func TestSkirmishFleesFromAStrongerNeighbour(t *testing.T) {
	b, a, d := skirmishPair(Plain, []Leader{weakLead("攻", 500, 50)}, []Leader{weakLead("守", 3000, 50)})
	s := b.NewSkirmish(a, d)
	g, p := s.Gens[SkirmishAttacker][0], s.Gens[SkirmishDefender][0]
	// 把攻方搬到守方旁邊：守方在 (11,2)，(10,3) 的方向 5 是它。
	s.Occ[g.Row][g.Col] = -1
	g.Col, g.Row = 10, 3
	s.Occ[3][10] = g.index()
	// RND(30)=0 行動；RND(3)=0 → 500 < 3000÷1 想逃；RND(3)=0 → 方向
	// (0 + 5 + 2) mod 6 = 1（往下）→ (10,4) 空著就走。
	dice := useScript(b, 0, 0, 0)
	s.step(g)
	wantAsked(t, "逃", dice, skirmishActRange, skirmishFleeSpread, skirmishFleeSpread)
	if g.Col != 10 || g.Row != 4 {
		t.Errorf("逃到 (%d,%d)，應該是 (10,4)", g.Col, g.Row)
	}
	if p.Leader.Soldiers != 3000 {
		t.Errorf("逃的那一步不該交手")
	}
}

// TestSkirmishDuelsWhenStronger：交手時 `我方戰力 > 對方戰力 + RND(20)`
// 就單挑；對方是電腦就走接不接受的判定，落敗被擒的記進捕獲方的名單，
// 結束時才處置（玩家捕獲的留著不處置）。
func TestSkirmishDuelsWhenStronger(t *testing.T) {
	att := lead("猛將", 99, 50, 3000)
	att.Training, att.Arms = 0, 0
	def := lead("庸將", 10, 50, 3000)
	def.Training, def.Arms = 0, 0
	b, a, d := skirmishPair(Plain, []Leader{att}, []Leader{def})
	b.Computer[MainAttacker] = true // 守方（被對戰的）也是電腦
	s := b.NewSkirmish(a, d)
	g := s.Gens[SkirmishAttacker][0]
	s.Occ[g.Row][g.Col] = -1
	g.Col, g.Row = 10, 3
	s.Occ[3][10] = g.index()
	// RND(30)=0；RND(3)=0（3000 < 3000÷1 不成立）；敵帥相鄰 → 交手；
	// RND(20)=0 → 99 > 10 + 0 單挑。叫陣：特效 RND(4)、兩句對白各 RND(8)；
	// 接受判定 RND(10)=9 → 9+10−5 > 99 不成立、兵相同第二道不擲、第三道
	// 不過；還沒接受再 RND(5)：10 ≥ r+90 不成立 → 拒絕：一句對白、
	// RND(10)（謀略 50 不到 81）、RND(5)（0+10 < 99 → 25 − RND(10÷20)，
	// 上限 0 不抽）→ 掉 3000÷25 ＝ 120。
	dice := useScript(b, 0, 0, 0, 0, 0, 0, 9, 0, 0, 0, 0)
	s.step(g)
	wantAsked(t, "單挑被拒", dice, skirmishActRange, skirmishFleeSpread, skirmishDuelSpread,
		EffectVariants, MessageLines, MessageLines, DuelWarSpread, DuelBraveSpread,
		MessageLines, RefuseIntelSpread, RefuseWarSpread)
	if got := s.Gens[SkirmishDefender][0].Leader.Soldiers; got != 3000-120 {
		t.Errorf("拒絕單挑的一方該掉 120 兵，剩 %d", got)
	}
	// 再來一次，這次接受：接受判定比的是**挑戰者**的兵——把守方兵壓到
	// 300、攻方 3000：3000÷2 > 300 → RND(20)=0：0+10 > 99 不成立；
	// 3000÷5 = 600 > 300 → 接受。
	s.Gens[SkirmishDefender][0].Leader.Soldiers = 300
	g.Leader.Soldiers = 3000
	g.Leader.Stamina, s.Gens[SkirmishDefender][0].Leader.Stamina = 100, 100
	// RND(30)=0；RND(3)=0（3000 < 300÷1 不成立）；敵帥相鄰 → 交手：
	// RND(20)=0 單挑；特效、兩句對白；接受：RND(10)=0、RND(20)=0、第三道
	// 過；接受的那一句對白；回合數 RND(99÷2+10÷2=54)=0 → 0 + 14 + 1 =
	// 15 回合；第 1 回合守方打攻方：RND(3)=0（回合÷2=0 是 10 的倍數）、
	// RND(5) RND(6) RND(5)＝0 0 0 → 0+(10−99)−0−0−4 < 0 → 0；第 2 回合
	// 攻方打守方：0+89−4 = 85 → 守方體能 100 → 15；第 3 回合 0；第 4
	// 回合 85 → 0；回合數加成 5 → 第 5 回合那一句對白照印（先印再判
	// 結束）；落敗 RND(7)=1 被擒：特效、兩句對白。
	dice = useScript(b, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1)
	s.step(g)
	wantAsked(t, "單挑得勝", dice, skirmishActRange, skirmishFleeSpread, skirmishDuelSpread,
		EffectVariants, MessageLines, MessageLines, DuelWarSpread, DuelOddsSpread, MessageLines,
		duelRoundSpread(99, 10), DuelBonusSpread,
		DuelBlowSpread, DuelBlowWide, DuelBlowSpread,
		DuelBlowSpread, DuelBlowWide, DuelBlowSpread,
		DuelBlowSpread, DuelBlowWide, DuelBlowSpread,
		DuelBlowSpread, DuelBlowWide, DuelBlowSpread, MessageLines,
		DuelDeathRoll, EffectVariants, MessageLines, MessageLines)
	p := s.Gens[SkirmishDefender][0]
	if !p.Gone || !p.Leader.Captured || s.Captured[SkirmishAttacker][0] != p {
		t.Errorf("守方該被抓進攻方的名單：%+v", p)
	}
	if s.Occ[2][11] != -1 {
		t.Errorf("被抓的人要離開子地圖")
	}
	if !s.over() {
		t.Errorf("守方第 0 位不在了，對戰要結束")
	}
	// 結束：處置由捕獲方（電腦）當場擲——RND(10) 斬首／囚禁、招降判定
	// RND(3)、對白。
	dice = useScript(b, 5, 0, 0, 0)
	s.finish()
	if got := dice.asked; len(got) == 0 || got[0] != CaptiveExecuteRange {
		t.Errorf("結束時要交給捕獲方處置（先擲 RND(10)），問了 %v", got)
	}
	if p.Leader.Fate == FateNone {
		t.Errorf("電腦捕獲的要有處置")
	}
}

// TestSkirmishAttackCapturesTheEmptied：攻擊的殺傷是 兵 × 戰力值 ÷ 100，
// 同時結算；兵扣到 0 的被對方抓走；超過 5000 的也當 0（原版的夾法）。
func TestSkirmishAttackCapturesTheEmptied(t *testing.T) {
	if SkirmishDamage(4978, 0) != 0 || SkirmishDamage(5000, 5) != 250 || SkirmishDamage(199, 1) != 1 {
		t.Errorf("殺傷：%d %d %d", SkirmishDamage(4978, 0), SkirmishDamage(5000, 5), SkirmishDamage(199, 1))
	}
	att := lead("攻", 80, 50, 5000)
	def := lead("守", 20, 50, 100)
	b, a, d := skirmishPair(Plain, []Leader{att, lead("攻二", 50, 50, 1000)}, []Leader{def})
	s := b.NewSkirmish(a, d)
	g, p := s.Gens[SkirmishAttacker][0], s.Gens[SkirmishDefender][0]
	dice := useScript(b, 3)
	s.attack(g, p)
	wantAsked(t, "攻擊", dice, MessageLines)
	da, dd := SkirmishDamage(5000, g.Power), SkirmishDamage(100, p.Power)
	if g.Leader.Soldiers != 5000-dd || p.Leader.Soldiers != 0 {
		t.Errorf("攻擊後 攻 %d（殺傷 %d）守 %d（殺傷 %d）", g.Leader.Soldiers, da, p.Leader.Soldiers, dd)
	}
	if !p.Gone || s.Captured[SkirmishAttacker][0] != p {
		t.Errorf("兵打光的守方該被抓")
	}
	if g.Leader.Stamina != 100-skirmishAttackStamina {
		t.Errorf("攻擊扣攻方 5 體能，剩 %d", g.Leader.Stamina)
	}
	// 八千兵的將領：扣完還大於 5000 → 0 → 被抓（原版的夾法）。
	b2, a2, d2 := skirmishPair(Plain, []Leader{weakLead("八千", 8000, 50)}, []Leader{weakLead("守", 100, 50)})
	s2 := b2.NewSkirmish(a2, d2)
	useScript(b2, 0)
	s2.attack(s2.Gens[SkirmishAttacker][0], s2.Gens[SkirmishDefender][0])
	if x := s2.Gens[SkirmishAttacker][0]; x.Leader.Soldiers != 0 || !x.Gone {
		t.Errorf("八千兵扣完仍大於五千，原版當成 0 而被抓：兵 %d", x.Leader.Soldiers)
	}
}

// TestSkirmishPlayerCommands：玩家的行軍一格一問、Enter 才離開；攻擊與
// 單挑把剩餘行動力歸零；沒人的方向回到選單。
func TestSkirmishPlayerCommands(t *testing.T) {
	b, a, d := skirmishPair(Plain, []Leader{weakLead("攻", 1000, 50)}, []Leader{weakLead("守", 1000, 50)})
	b.Computer[MainDefender] = false
	s := b.NewSkirmish(a, d)
	g := s.Gens[SkirmishAttacker][0]
	script := []SkirmishCommand{
		{Kind: SkirmishMarch, Dir: DirDownRight}, // (0,3) → (1,3)
		{Kind: SkirmishMarch, Dir: DirUpLeft},    // (1,3) → (0,3)
		{Kind: SkirmishMarchDone},
	}
	i := 0
	s.Player = func(*Skirmish, *SkirmishGeneral) SkirmishCommand {
		c := script[i]
		i++
		return c
	}
	dice := useScript(b)
	s.step(g)
	wantAsked(t, "行軍", dice)
	// 走兩格花 4 步剩 7，步結束時補回移動力 11（`0x2eb6d`）。
	if g.Col != 0 || g.Row != 3 || g.Left != 11 || i != 3 {
		t.Errorf("走兩格回到原點：(%d,%d) 剩 %d 步，問了 %d 次", g.Col, g.Row, g.Left, i)
	}
	if g.Leader.Stamina != 48 {
		t.Errorf("走兩格扣 2 體能，剩 %d", g.Leader.Stamina)
	}
	// 攻擊：先指一個沒人的方向（回到選單），再指守方。
	s.Occ[g.Row][g.Col] = -1
	g.Col, g.Row = 10, 3
	s.Occ[3][10] = g.index()
	g.Left = 11
	script = []SkirmishCommand{{Kind: SkirmishAttack, Dir: DirDown}, {Kind: SkirmishAttack, Dir: DirUpRight}}
	i = 0
	dice = useScript(b, 0)
	s.step(g)
	wantAsked(t, "攻擊", dice, MessageLines)
	if g.Left != 11 || i != 2 {
		// 步結束時剩餘行動力補回移動力（`0x2eb6d`）。
		t.Errorf("攻擊後剩 %d 步（歸零再補回 11），問了 %d 次", g.Left, i)
	}
}
