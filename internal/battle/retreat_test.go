package battle

import "testing"

// 這一支檔案釘住「退兵的去處」那一問（Issue #101，原版 `0x23dd4`–`0x24460`）。
//
// 原版 `0x24318` 會把退去的郡寫入人物 offset 19。這裡驗選擇與
// 強制退卻；戰略層寫回由 `internal/game` 的玩家結算測試驗。

// retreatBoard 擺一支孤軍，給它三個逃得去的鄰郡。
func retreatBoard(side Side) (*Battle, *Unit) {
	b := arena(flat(Plain))
	u := place(b, side, Centre, FromOffset(6, 6), lead("退", 50, 50, 1000))
	b.Escapes[side] = []Escape{{Prefecture: 11}, {Prefecture: 22}, {Prefecture: 33}}
	return b, u
}

// TestPlayerRetreatAsksWhichPrefecture 釘住玩家那一條會問，而且收下答案
// （`0x2408e`「%s逃向那一郡」）。**反向對照**：答第二個郡就要記到第二個郡，
// 挑錯一個（例如退回清單第一項或擲骰）這裡就紅。
func TestPlayerRetreatAsksWhichPrefecture(t *testing.T) {
	b, u := retreatBoard(MainAttacker)
	var sawUnit *Unit
	var sawCands []Escape
	b.PlayerRetreat = func(x *Unit, cands []Escape) int {
		sawUnit, sawCands = x, cands
		return 22
	}
	s := useScript(b)
	if err := b.Retreat(u); err != nil {
		t.Fatalf("退不了：%v", err)
	}
	if sawUnit != u {
		t.Error("問的不是要退的那一支")
	}
	if len(sawCands) != 3 {
		t.Errorf("候選給了 %d 個，盤面上有 3 個", len(sawCands))
	}
	if u.RetreatTo != 22 {
		t.Errorf("記下的去處是 %d，玩家答的是 22", u.RetreatTo)
	}
	if !u.Retreated {
		t.Error("答完了卻沒退")
	}
	// 玩家那一條**不擲去處**（原版出選單問人）：只有對白與特效兩擲。
	wantAsked(t, "玩家退兵", s, MessageLines, EffectVariants)
}

// TestPlayerRetreatCancelKeepsTheUnitOnTheField 釘住空 Enter 取消整個退兵
// （`0x2416a`：那一支留在戰場）。
func TestPlayerRetreatCancelKeepsTheUnitOnTheField(t *testing.T) {
	ResetRetreats()
	b, u := retreatBoard(MainAttacker)
	b.PlayerRetreat = func(*Unit, []Escape) int { return 0 }
	err := b.Retreat(u)
	if !RetreatCancelled(err) {
		t.Fatalf("取消該回 errRetreatCancelled，得 %v", err)
	}
	if u.Retreated || !u.Alive() {
		t.Error("取消了卻把人退掉了")
	}
	if u.RetreatTo != 0 {
		t.Errorf("取消了卻記下去處 %d", u.RetreatTo)
	}
	if got := Retreats(); got != 0 {
		t.Errorf("取消也數了 %d 次退兵", got)
	}
	// 取消之後還能再退一次，而且這次算數。
	b.PlayerRetreat = func(*Unit, []Escape) int { return 33 }
	if err := b.Retreat(u); err != nil {
		t.Fatalf("取消之後退不了：%v", err)
	}
	if u.RetreatTo != 33 {
		t.Errorf("第二次的去處是 %d，該是 33", u.RetreatTo)
	}
}

// TestForcedRetreatAfterBattleCannotCancel 釘住 `0x258f2` 的第三參數
// `0xffff`：戰役已結束仍要退，空 Enter 不能取消敗軍退卻。
func TestForcedRetreatAfterBattleCannotCancel(t *testing.T) {
	b, u := retreatBoard(MainAttacker)
	b.Over = true
	asks := 0
	b.PlayerRetreat = func(*Unit, []Escape) int {
		asks++
		if asks == 1 {
			return 0
		}
		return 22
	}
	if err := b.ForceRetreat(u); err != nil {
		t.Fatalf("戰後強制退兵失敗：%v", err)
	}
	if asks != 2 || !u.Retreated || u.RetreatTo != 22 {
		t.Errorf("強制退兵：詢問 %d 次，退兵 %v，去處 %d；應為 2／true／22",
			asks, u.Retreated, u.RetreatTo)
	}
}

// TestPlayerRetreatRejectsAPrefectureThatIsNotAdjacent 釘住答了不在候選表裡的
// 郡要退回去（原版收的是**郡編號**不是清單序號，所以打錯號碼是可能的）。
func TestPlayerRetreatRejectsAPrefectureThatIsNotAdjacent(t *testing.T) {
	b, u := retreatBoard(MainAttacker)
	b.PlayerRetreat = func(*Unit, []Escape) int { return 44 }
	if err := b.Retreat(u); err == nil {
		t.Fatal("逃到一個不相鄰的郡卻成功了")
	} else if RetreatCancelled(err) {
		t.Error("那不是取消，是答錯")
	}
	if u.Retreated {
		t.Error("答錯卻把人退掉了")
	}
}

// TestNoEscapeIsNotTheSameAsSurrounded 釘住兩種「逃不掉」分開（Issue #101）：
// 「無郡可逃」是鄰郡的問題（`0x23f03`），「被完全包圍」是格子的問題。
// 訊息混在一起的話，玩家看到的理由與實際擋住他的條件不同。
func TestNoEscapeIsNotTheSameAsSurrounded(t *testing.T) {
	// 有活路、沒有鄰郡。
	b := arena(flat(Plain))
	u := place(b, MainAttacker, Centre, FromOffset(6, 6), lead("退", 50, 50, 1000))
	asked := false
	b.PlayerRetreat = func(*Unit, []Escape) int { asked = true; return 11 }
	err := b.Retreat(u)
	if err == nil {
		t.Fatal("沒有鄰郡卻退成功了")
	}
	if got := err.Error(); got != "battle: 無郡可逃" {
		t.Errorf("理由是 %q，該是「無郡可逃」", got)
	}
	if asked {
		t.Error("沒有候選就不該問人（原版直接印「無郡可逃」）")
	}

	// 有鄰郡、六面封死。
	f := flat(Plain)
	spot := FromOffset(6, 6)
	b2 := arena(f)
	u2 := place(b2, MainAttacker, Centre, spot, lead("退", 50, 50, 1000))
	b2.Escapes[MainAttacker] = []Escape{{Prefecture: 11}}
	for _, d := range Dirs() {
		f.Set(spot.Step(d), Mountain)
	}
	err = b2.Retreat(u2)
	if err == nil {
		t.Fatal("被封死卻退成功了")
	}
	if got := err.Error(); got != "battle: 被完全包圍，逃不掉" {
		t.Errorf("理由是 %q，該是「被完全包圍」", got)
	}
}

// TestComputerRetreatIgnoresThePlayerHook 是**反向對照**：電腦那一條照舊，
// 不因為介面掛著就改走問人。骰序 #24 已經對齊，這裡不准動。
func TestComputerRetreatIgnoresThePlayerHook(t *testing.T) {
	// 主攻軍、原郡裝得下 → 回原郡，不擲。
	b, u := retreatBoard(MainAttacker)
	b.Computer[MainAttacker] = true
	b.Origin[MainAttacker] = Escape{Prefecture: 9, Active: 10}
	b.PlayerRetreat = func(*Unit, []Escape) int { t.Error("電腦那一條不該問人"); return 11 }
	s := useScript(b)
	if err := b.Retreat(u); err != nil {
		t.Fatalf("退不了：%v", err)
	}
	if u.RetreatTo != 9 {
		t.Errorf("回原郡該是 9，得 %d", u.RetreatTo)
	}
	wantAsked(t, "電腦退回原郡", s, MessageLines, EffectVariants)

	// 主守軍一律 `RND(鄰郡數)`。
	b2, u2 := retreatBoard(MainDefender)
	b2.Computer[MainDefender] = true
	s2 := useScript(b2, 0, 2, 0) // 對白、去處、特效
	if err := b2.Retreat(u2); err != nil {
		t.Fatalf("退不了：%v", err)
	}
	if u2.RetreatTo != 33 {
		t.Errorf("骰到第 2 個候選該是 33，得 %d", u2.RetreatTo)
	}
	wantAsked(t, "主守軍退兵", s2, MessageLines, 3, EffectVariants)
}

// TestRetreatWithoutAnInterfaceTakesTheFirstAndDoesNotRoll 釘住沒有介面的路
// （月度對拍、示範模式、批次跑）：沒有人可以挑，取候選表第一個，**不擲**。
// 這是 registered remake 差異，理由與 #64／#98／#99／#100 同一個。
func TestRetreatWithoutAnInterfaceTakesTheFirstAndDoesNotRoll(t *testing.T) {
	b, u := retreatBoard(MainAttacker)
	s := useScript(b)
	if err := b.Retreat(u); err != nil {
		t.Fatalf("退不了：%v", err)
	}
	if u.RetreatTo != 11 {
		t.Errorf("沒有介面該取第一個（11），得 %d", u.RetreatTo)
	}
	wantAsked(t, "沒有介面退兵", s, MessageLines, EffectVariants)
}
