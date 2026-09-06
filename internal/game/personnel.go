package game

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 6. 人事與 7. 君主（說明書 p.22–24）。
//
// 成功率型的指令要**決定性**：同一個局面同一個決定永遠得到同一個結果。
// 理由與 AI 相同——帶系統亂數的話「這一次為什麼失敗」無法回答，
// 對拍也無從下手。這裡用局面本身當種子（年、月、郡、人物），
// 所以重跑同一局會重現，而不同的局面會不同。

// roll 是決定性的擲骰，回 0..99。
//
// ⚠ **這不是原版的亂數。** 原版的亂數產生器還沒反組譯到；
// 換掉它的時候，這一層的介面不用動。
func (g *State) roll(salt ...int) int {
	h := uint32(2166136261)
	mix := func(v int) {
		h ^= uint32(v)
		h *= 16777619
	}
	mix(g.Date.Year)
	mix(g.Date.Month)
	for _, v := range salt {
		mix(v)
	}
	// **收尾的雪崩不能省。** FNV-1a 的最後一步是乘一個奇數，而乘奇數
	// 保留低位元——所以 `roll(a, b)` 與 `roll(a, b, 1)` 的最低位元必然
	// 相反，兩者相加永遠是奇數。
	//
	// 症狀不會是「亂數看起來怪」：拿去跟門檻比大小的地方完全正常，
	// 只有把兩次抽樣加起來的地方會露出來——物價只出現奇數值
	//（`TestPriceMovesEveryMonth`）。
	h ^= h >> 16
	h *= 0x7feb352d
	h ^= h >> 15
	h *= 0x846ca68b
	h ^= h >> 16
	return int(h % 100)
}

// Roll 是給 `internal/ai` 用的決定性亂數，回 0..n−1。
//
// **和規則層用的是同一顆**（`roll`），所以 AI 的選擇與事件的判定
// 共用同一份決定性——存檔重播得出同一局。
func (g *State) Roll(n int, salt ...int) int {
	if n <= 0 {
		return 0
	}
	return g.roll(salt...) % n
}

// ---- 6. 人事 ------------------------------------------------------------

// Search 是「尋訪人才」（說明書 p.22）：每次 5 金，
// **謀略越高成功機率越大**；找到的人成為當地在野將領。
func (g *State) Search(prefectureID, generalIndex int, by state.FactionID) (found *General, err error) {
	p, e := g.canOrder(prefectureID, by)
	if e != nil {
		return nil, e
	}
	x := g.General(generalIndex)
	if x == nil || x.Faction != by || x.Location != prefectureID {
		return nil, ErrUnknownUnit
	}
	fee := g.price(by, CostSearch)
	if p.Gold < fee {
		return nil, ErrNoGold
	}
	p.Gold -= fee
	p.Commanded = true

	// 這個郡有沒有人可找：身分是「在野但不列入該郡在野數」的那些人。
	var hidden *General
	for i := range g.generals {
		c := &g.generals[i]
		if c.Location == prefectureID && c.Status == state.StatusIdle && c.Name != "" {
			hidden = c
			break
		}
	}
	if hidden == nil {
		return nil, nil // 這裡真的沒有人才——不是錯誤
	}
	chance := clampTo(int(x.Intel)/TuneSearchIntel, 95)
	if g.roll(prefectureID, x.Index, hidden.Index) >= chance {
		return nil, nil
	}
	hidden.Status = state.StatusAvailable
	return hidden, nil
}

// Recruit 是「登用人才」（說明書 p.22）：從本地在野將領中招用。
//
// 判定照原版的 `0xce8c`（公式與推論等級見 `rules.go` 的
// `RecruitPersuasion`／`RecruitDifficulty`）：說服力大於難度才成功，
// 而難度前面有一道**牽絆閘門**——被登用者的 `Bond` 指到誰，
// 就看那個人現在效力於誰。
//
// **「失敗」與「不能下令」是兩件事**：錢照付、命令照用掉，
// 回傳的錯誤只是說他不肯來。
func (g *State) Recruit(prefectureID, targetIndex int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	t := g.General(targetIndex)
	if t == nil || t.Employed() || t.Location != prefectureID ||
		t.Status != state.StatusAvailable {
		return ErrUnknownUnit
	}
	level := g.aiLevelOf(by)
	fee := g.price(by, RecruitFee(level))
	if p.Gold < fee {
		return ErrNoGold
	}
	if g.ActiveGenerals(prefectureID) >= MaxGeneralsPerPrefecture {
		return ErrTooManyGens
	}
	p.Gold -= fee
	p.Commanded = true

	bonus := RecruitBonus(level)
	charm := 50
	if gov := g.Governor(prefectureID); gov != nil {
		charm = int(gov.Charm)
	}
	prestige := 0
	if f := g.Faction(by); f != nil {
		prestige = f.Prestige
	}
	say := RecruitPersuasion(prestige, charm, bonus)
	if say <= g.recruitDifficulty(t, prefectureID, by) {
		return fmt.Errorf("%s：%w", tf("msg.declined", t.Name), ErrDeclined)
	}
	loyalty := RecruitLoyalty(prestige,
		g.Roll(max(prestige/2, 1), prefectureID, targetIndex, 0x5634), bonus)
	if loyalty <= 0 {
		return fmt.Errorf("%s：%w", tf("msg.declined", t.Name), ErrDeclined)
	}
	t.Faction = by
	t.Status = state.StatusOfficer
	t.Rank = state.RankJuniorGeneral
	t.Loyalty = uint8(clampTo(loyalty, 100))
	return nil
}

// recruitDifficulty 是登用判定裡「對方有多難請動」的那一半。
//
// 牽絆閘門蓋過能力值：`Bond` 指到的人效力於招募方就是零，
// 效力於第三方就是一道牆。指向自己的人（346 筆裡 176 筆）在野時
// 那一格自然是在野，所以走能力值那條。
func (g *State) recruitDifficulty(t *General, prefectureID int, by state.FactionID) int {
	if b := g.General(t.Bond); b != nil {
		switch {
		case b.Employed() && b.Faction == by:
			return RecruitBondFree
		case b.Employed():
			return RecruitBondWall + g.Roll(10, prefectureID, t.Index, 0x5634)
		}
	}
	return RecruitDifficulty(int(t.Intel), int(t.War),
		g.Roll(4, prefectureID, t.Index, 1),
		g.Roll(4, prefectureID, t.Index, 2))
}

// Reward 是「賞賜金帛」（說明書 p.23）：賞金上限 100，
// **各郡每月可賞每人一次**。
func (g *State) Reward(prefectureID, targetIndex, gold int, by state.FactionID) error {
	p := g.Prefecture(prefectureID)
	if p == nil || !p.Owned() || p.Owner != by {
		return ErrNotYours
	}
	if gold <= 0 || gold > MaxReward {
		return fmt.Errorf("game: 賞金要在 1..%d，拿到 %d", MaxReward, gold)
	}
	t := g.General(targetIndex)
	if t == nil || t.Faction != by || t.Location != prefectureID {
		return ErrUnknownUnit
	}
	if t.Rewarded {
		return ErrAlreadyPaid
	}
	if p.Gold < gold {
		return ErrNoGold
	}
	p.Gold -= gold
	t.Rewarded = true
	if t.HasLoyalty() {
		t.Loyalty = uint8(clampTo(int(t.Loyalty)+gold/TuneRewardLoyalty, 100))
	}
	return nil
}

// Dismiss 是「撤職」（說明書 p.23）：免去部將職務成為在野將領，
// 遣散費 10 金。**主事者不能撤**——撤了那個郡就沒人治理。
func (g *State) Dismiss(prefectureID, targetIndex int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	t := g.General(targetIndex)
	if t == nil || t.Faction != by || t.Location != prefectureID {
		return ErrUnknownUnit
	}
	if t.Status.Governs() {
		return ErrNoGovernor
	}
	fee := g.price(by, CostDismiss)
	if p.Gold < fee {
		return ErrNoGold
	}
	p.Gold -= fee
	t.Soldiers = 0
	t.Faction = state.NoFaction
	t.Status = state.StatusAvailable
	t.Loyalty = state.NoValue
	if f := g.Faction(by); f != nil && f.Chief == t.Index {
		f.Chief = -1
	}
	p.Commanded = true
	return nil
}

// ---- 7. 君主 ------------------------------------------------------------

// requireLordAt 檢查君主本人在不在這個郡。君主指令只有君主能下。
func (g *State) requireLordAt(prefectureID int, by state.FactionID) error {
	lord := g.Lord(by)
	if lord == nil || lord.Location != prefectureID {
		return ErrNotLord
	}
	return nil
}

// AppointChief 是「指定軍師」（說明書 p.23）：
// **謀略不得低於 80**、君主不得兼任、同時只能有一位。
func (g *State) AppointChief(prefectureID, targetIndex int, by state.FactionID) error {
	if err := g.requireLordAt(prefectureID, by); err != nil {
		return err
	}
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	t := g.General(targetIndex)
	if t == nil || t.Faction != by || t.Location != prefectureID {
		return ErrUnknownUnit
	}
	if t.Status == state.StatusLord {
		return ErrLordCantBe
	}
	if t.Intel < MinIntelForChief {
		return ErrNeedIntel80
	}
	f := g.Faction(by)
	if f == nil {
		return ErrNotYours
	}
	// 前任軍師回任現役將領（說明書 p.23）。
	if old := g.General(f.Chief); old != nil && old.Status == state.StatusChief {
		old.Status = state.StatusOfficer
	}
	t.Status = state.StatusChief
	f.Chief = t.Index
	p.Commanded = true
	return nil
}

// AppointGovernor 是「指定太守」（說明書 p.23）：
// 指定**自身所在之外**的領地部將擔任該郡太守。不耗指令。
func (g *State) AppointGovernor(prefectureID, targetIndex int, by state.FactionID) error {
	lord := g.Lord(by)
	if lord == nil {
		return ErrNotLord
	}
	if lord.Location == prefectureID {
		return fmt.Errorf("game: 君主自己在這個郡，不需要指定太守")
	}
	p := g.Prefecture(prefectureID)
	if p == nil || !p.Owned() || p.Owner != by {
		return ErrNotYours
	}
	t := g.General(targetIndex)
	if t == nil || t.Faction != by || t.Location != prefectureID {
		return ErrUnknownUnit
	}
	for _, x := range g.Garrison(prefectureID) {
		if x.Faction == by && x.Status == state.StatusGovernor {
			x.Status = state.StatusOfficer
		}
	}
	t.Status = state.StatusGovernor
	return nil
}

// SetAutonomy 是「郡縣自治」（說明書 p.23–24）。不耗指令。
func (g *State) SetAutonomy(prefectureID int, mode Autonomy, by state.FactionID) error {
	p := g.Prefecture(prefectureID)
	if p == nil || !p.Owned() || p.Owner != by {
		return ErrNotYours
	}
	if mode < AutoNormal || mode > AutoSelf {
		return fmt.Errorf("game: 沒有這種自治型態 %d", mode)
	}
	p.Autonomy = mode
	return nil
}

// GiftTreasure 是「賞賜物品」（說明書 p.24）：
// 玉璽不能送人；靠賞賜提升的能力上限是 90。
func (g *State) GiftTreasure(prefectureID, targetIndex int, t Treasure, by state.FactionID) error {
	if err := g.requireLordAt(prefectureID, by); err != nil {
		return err
	}
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if t == TreasureSeal {
		return ErrCantGift
	}
	if t < 0 || t >= treasureCount {
		return fmt.Errorf("game: 沒有這件寶物 %d", t)
	}
	f := g.Faction(by)
	if f == nil || f.Treasury[t] <= 0 {
		return ErrNoTreasure
	}
	x := g.General(targetIndex)
	if x == nil || x.Faction != by || x.Location != prefectureID {
		return ErrUnknownUnit
	}
	di, dw, dc := TreasureEffect(t)
	x.Intel = raiseTo(x.Intel, di, TreasureCap)
	x.War = raiseTo(x.War, dw, TreasureCap)
	x.Charm = raiseTo(x.Charm, dc, TreasureCap)
	f.Treasury[t]--
	if x.HasLoyalty() {
		x.Loyalty = uint8(clampTo(int(x.Loyalty)+1, 100))
	}
	p.Commanded = true
	return nil
}

// raiseTo 把數值加上 d，但**不超過 cap**；已經高過 cap 的不動。
//
// 手冊寫「利用賞賜增加能力上限是 90 點」——那是**賞賜的上限不是能力的上限**，
// 原本就 95 的人不會因為收到禮物而掉到 90。
func raiseTo(v uint8, d, cap int) uint8 {
	if d == 0 || int(v) >= cap {
		return v
	}
	return uint8(clampTo(int(v)+d, cap))
}

// Headhunt 是「登用他國人才」（說明書 p.24）：每次 100 金，
// 對象必須是**別國的現役將領**，諸侯不算；
// **太守被挖角的話全郡同時歸屬新諸侯**。
func (g *State) Headhunt(prefectureID, targetIndex int, by state.FactionID) error {
	if err := g.requireLordAt(prefectureID, by); err != nil {
		return err
	}
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	t := g.General(targetIndex)
	if t == nil || !t.Employed() || t.Faction == by {
		return ErrUnknownUnit
	}
	if t.Status == state.StatusLord {
		return fmt.Errorf("game: 諸侯挖不動")
	}
	fee := g.price(by, CostHeadhunt)
	if p.Gold < fee {
		return ErrNoGold
	}
	if g.ActiveGenerals(prefectureID) >= MaxGeneralsPerPrefecture {
		return ErrTooManyGens
	}
	p.Gold -= fee
	p.Commanded = true

	chance := clampTo(TuneHeadhuntBase-int(t.Loyalty)/2, 95)
	if g.roll(prefectureID, targetIndex, int(t.Loyalty)) >= chance {
		return fmt.Errorf("%s", tf("msg.unmoved", t.Name))
	}
	old := t.Faction
	wasGovernor := t.Status.Governs()
	at := t.Location
	t.Faction = by
	t.Status = state.StatusOfficer
	t.Loyalty = 60
	if wasGovernor {
		// 太守被挖角 → 全郡同時歸屬新諸侯。
		if q := g.Prefecture(at); q != nil {
			q.Owner = by
		}
		t.Status = state.StatusGovernor
		// 原本那一郡的其他人還效忠舊主，跟著失去駐地——先讓他們留下，
		// 由「主事者唯一」的規則決定後續。
		_ = old
	} else {
		// 人被挖走要離開原郡，回到挖角方的所在地。
		t.Location = prefectureID
	}
	return nil
}
