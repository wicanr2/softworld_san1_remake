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
// SeedRand 接上原版的亂數狀態（`DS:0xa3ae`，`docs/re/03` §1.45）。
//
// 接上之後 `roll`／`Roll` 改抽同一條 LCG，與局面綁定的雜湊就不用了。
// 對拍要兩邊從同一個狀態出發，這是唯一的入口。
func (g *State) SeedRand(seed uint32) {
	g.randSeed, g.randOn, g.randDraws = seed, true, 0
}

// RandSeed 是目前的亂數狀態。對拍拿它逐郡比對——**第一個對不上的郡就是
// 第一個岔開的地方**，後面的差異全是它的連鎖。
func (g *State) RandSeed() uint32 { return g.randSeed }

// TracePhases 打開換月各段的抽樣計數，寫進 m。傳 nil 關掉。
func (g *State) TracePhases(m map[string]int, seeds map[string]uint32) {
	g.phaseTrace, g.phaseSeed, g.phaseLast = m, seeds, g.randDraws
}

// markPhase 把上一個標記點到現在的抽樣次數記到 name 名下。
func (g *State) markPhase(name string) {
	if g.phaseTrace == nil {
		return
	}
	g.phaseTrace[name] += g.randDraws - g.phaseLast
	g.phaseLast = g.randDraws
	if g.phaseSeed != nil {
		g.phaseSeed[name] = g.randSeed
	}
}

// RandDraws 是接上之後抽了幾次。
//
// **這比位元組數利**：抽的次數對不上，表示某一支常式的分支或迴圈次數
// 與原版不同——那是可以逐支定位的；位元組數只告訴你「有差」。
func (g *State) RandDraws() int { return g.randDraws }

// nextRand 抽一次原版的亂數（0..32767）。
func (g *State) nextRand() int {
	seed, out := MSCRand(g.randSeed)
	g.randSeed = seed
	g.randDraws++
	return out
}

func (g *State) roll(salt ...int) int {
	if g.randOn {
		return g.nextRand() % 100
	}
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

// MSCRand 是 MSC 6.0 的 `rand()`（原版的 `0x5c4:0x2cb0`，`L1`）：
// 線性同餘 `seed = seed*214013 + 2531011`，輸出取第 16..30 位元。
//
// 原版的 `RND(n)` 是 `0x10b0c`（`L0`）：`n <= 0` 回 0，否則
// `rand()` 對 n 取 **idiv 的餘數**。
//
// **這是亂數對齊的地基**：remake 現在用的是與局面綁定的雜湊
//（`roll`），跟原版的序列無關，所以月度對拍剩下的差異多半來自
// 「哪個郡擲到了什麼」。要收斂就得從原版的狀態接著抽，而且每一處
// 消耗的次數都要一樣（`internal/parity/rand_oracle_test.go`）。
func MSCRand(seed uint32) (uint32, int) {
	seed = seed*214013 + 2531011
	return seed, int((seed >> 16) & 0x7fff)
}

// Roll 是給 `internal/ai` 用的決定性亂數，回 0..n−1。
//
// **和規則層用的是同一顆**（`roll`），所以 AI 的選擇與事件的判定
// 共用同一份決定性——存檔重播得出同一局。
func (g *State) Roll(n int, salt ...int) int {
	if n <= 0 {
		return 0
	}
	if g.randOn {
		// 原版是 `RND(n)`：`rand()` 對 n 取餘數（`0x10b0c`），
		// **不是先取 100 再取 n**——那會多一層取模偏差。
		return g.nextRand() % n
	}
	return g.roll(salt...) % n
}

// ---- 6. 人事 ------------------------------------------------------------

// Search 是「尋訪人才」（說明書 p.22）：每次 5 金，
// **謀略越高成功機率越大**；找到的人成為當地在野將領。
func (g *State) Search(prefectureID, generalIndex int, by state.FactionID) (found *General, err error) {
	return g.search(prefectureID, generalIndex, by, true)
}

// search 是本體。`charge` 分開玩家與電腦兩條——電腦那一條不收錢。
func (g *State) search(prefectureID, generalIndex int, by state.FactionID,
	charge bool) (found *General, err error) {
	p, e := g.canOrder(prefectureID, by)
	if e != nil {
		return nil, e
	}
	x := g.General(generalIndex)
	if x == nil || x.Faction != by || x.Location != prefectureID {
		return nil, ErrUnknownUnit
	}
	// **收不收錢分兩條**：玩家一次 5 金（說明書 p.22），電腦那一條
	// （`0xcd20` 起的六支 → `0xcc86`）**不收**——那一支從頭到尾沒有寫
	// 州郡 offset 18，只把一位身分 9 的人改成 8、offset 23 加一。
	if charge {
		fee := g.price(by, CostSearch)
		if p.Gold < fee {
			return nil, ErrNoGold
		}
		p.Gold -= fee
	}
	p.Commanded = true

	// **門檻照原版**（`SearchTierFor`，`L1`、`0xcc86`）：
	// 尋訪者的謀略要**大於** `RND(Spread) + Floor`。三個常數隨 AI 等級變，
	// 玩家這一邊照等級 0 那一組（`L2`：玩家的常式沒有單獨讀過）。
	//
	// **先擲再找人。** 原版這一次亂數是無條件抽的——這個郡有沒有人可找
	// 是後面才判的。先找人再擲，沒人的郡就不會抽，整條亂數序列跟著錯開
	// （月度對拍量到尋訪少抽 13 次，正好等於閘門通過的次數）。
	tier := SearchTierFor(g.aiLevelOf(by))
	bar := g.roll(prefectureID, x.Index)%tier.Spread + tier.Floor

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
	if int(x.Intel) <= bar {
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
		!t.Status.Recruitable() {
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
	// **兩次能力值的抽樣無條件先抽**（`0xce8c`：難度先算，牽絆的兩種
	// 例外是抽完之後才蓋上去的）。寫成先判牽絆再算，那兩次就不會發生，
	// 整條亂數序列跟著錯開——同尋訪、計略、賑民那一族。
	hard := RecruitDifficulty(int(t.Intel), int(t.War),
		g.Roll(4, prefectureID, t.Index, 1),
		g.Roll(4, prefectureID, t.Index, 2))
	if b := g.General(t.Bond); b != nil {
		switch {
		case b.Employed() && b.Faction == by:
			return RecruitBondFree
		case b.Employed():
			return RecruitBondWall + g.Roll(10, prefectureID, t.Index, 0x5634)
		}
	}
	return hard
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
	charm := 50
	if gov := g.Governor(prefectureID); gov != nil {
		charm = int(gov.Charm)
	}
	bonus := RewardBonus(g.aiLevelOf(by))
	effect := RewardEffect(charm, bonus, g.Roll(max(bonus/2, 1), prefectureID, targetIndex, 0x5654))
	t.Rewarded = true
	if t.HasLoyalty() {
		was := int(t.Loyalty)
		t.Loyalty = uint8(clampTo(was+RewardGain(effect, gold), 100))
		// **只付真的換到的那一段**（原版 `0xd40e` 照增幅反算，`L0`）：
		// 忠誠已經接近 100 時賞下去的錢跟著變少。
		gold = RewardCost(effect, int(t.Loyalty)-was)
	}
	if gold > p.Gold {
		gold = p.Gold
	}
	p.Gold -= g.price(by, gold)
	return nil
}

// headhuntable 是「這個人挖不挖得動」（`L0`、`0xe0bc`／`0x1dc0a`）。
//
// 兩道閘門：**忠誠不低於 `RND(15) + 80` 就挖不動**，以及**牽絆對象
// 還在他自己陣營裡就挖不動**——後者是登用那道閘門的鏡像。
func (g *State) Headhuntable(t *General, prefectureID int) bool {
	bar := HeadhuntLoyaltyBar(g.Roll(HeadhuntLoyaltySpread, prefectureID, t.Index, 0x56d4))
	if int(t.Loyalty) >= bar {
		return false
	}
	if b := g.General(t.Bond); b != nil && b.Index != t.Index &&
		b.Employed() && b.Faction == t.Faction {
		return false
	}
	return true
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
	g.installChief(f, t)
	p.Commanded = true
	return nil
}

// appointChief 是電腦諸侯那一條（`0xd7ae`）。原版三件事與玩家那條不同：
// **君主不必在場**（整支常式沒有這個檢查）、**候選不比對勢力**
// （名單就是指定太守用的那一份，`docs/re/07` §6），以及智力門檻是
// 現任軍師的智而不是固定的 80。
func (g *State) appointChief(prefectureID, targetIndex int, by state.FactionID) error {
	p := g.Prefecture(prefectureID)
	if p == nil || !p.Owned() || p.Owner != by {
		return ErrNotYours
	}
	t := g.General(targetIndex)
	if t == nil || !t.Employed() || t.Location != prefectureID {
		return ErrUnknownUnit
	}
	// `0xd862`：候選的身分限太守或一般武將。
	if t.Status != state.StatusGovernor && t.Status != state.StatusOfficer {
		return ErrUnknownUnit
	}
	f := g.Faction(by)
	if f == nil {
		return ErrNotYours
	}
	g.installChief(f, t)
	return nil
}

// installChief 換軍師：前任回任，新任上任（`0xd8b0`–`0xd905`）。
func (g *State) installChief(f *Faction, t *General) {
	if old := g.General(f.Chief); old != nil && old.Status == state.StatusChief {
		// 前任回任現役將領（說明書 p.23），**但他若是所在郡的主事者就是
		// 太守**（`0xd8dc` 比的是州郡 offset 32 的存值）。少了這個例外，
		// 那一格會指到一個身分 3 的人，而原版保證它指到的是 0、1 或 2
		//（`docs/re/07` §7）。這裡讀 `p.governor` 而不是 `Governor()`
		// ——後者在存著的那位失聯時會重新指派並寫回去，比的就不是存值了。
		old.Status = state.StatusOfficer
		if q := g.Prefecture(old.Location); q != nil && q.governor == old.Index {
			old.Status = state.StatusGovernor
		}
	}
	t.Status = state.StatusChief
	f.Chief = t.Index
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
	return g.installGovernor(prefectureID, t, by)
}

// appointGovernor 是指定太守的本體。`sameFaction` 分開玩家與電腦兩條：
// 原版的電腦那一支挑名單時**不比對勢力**（`docs/re/07` §6，28 次量過），
// 所以目標可能是站在郡裡的外勢力武將。
func (g *State) appointGovernor(prefectureID, targetIndex int,
	by state.FactionID, sameFaction bool) error {
	p := g.Prefecture(prefectureID)
	if p == nil || !p.Owned() || p.Owner != by {
		return ErrNotYours
	}
	t := g.General(targetIndex)
	if t == nil || !t.Employed() || t.Location != prefectureID ||
		(sameFaction && t.Faction != by) {
		return ErrUnknownUnit
	}
	return g.installGovernor(prefectureID, t, by)
}

func (g *State) installGovernor(prefectureID int, t *General, by state.FactionID) error {
	for _, x := range g.Garrison(prefectureID) {
		if x.Faction == by && x.Status == state.StatusGovernor {
			x.Status = state.StatusOfficer
		}
	}
	// **只把身分 3 升成 2**（`0xd715`：`新的身分 == 3 → 新的身分 = 2`）。
	// 挑中的是軍師（身分 1）時原版不動他的身分——他成為主事者，但還是
	// 軍師。無條件寫 `StatusGovernor` 會順手廢掉一位軍師，而畫面上只
	// 看得到「這個勢力的軍師欄空了」。
	if t.Status == state.StatusOfficer {
		t.Status = state.StatusGovernor
	}
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
	return g.giftTreasure(prefectureID, targetIndex, t, by, true)
}

// giftTreasure 是賞賜物品的本體。`needLord` 分開玩家與電腦兩條：
// 說明書 p.24 的「君主賞賜」是**玩家**的規則，原版電腦那四支
// （`0xd961` 起）從 `RND(100)` 直接判到存量，**沒有君主在場的檢查**。
func (g *State) giftTreasure(prefectureID, targetIndex int, t Treasure,
	by state.FactionID, needLord bool) error {
	if needLord {
		if err := g.requireLordAt(prefectureID, by); err != nil {
			return err
		}
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
	// **電腦那一條不比對勢力**（名單是 `buildRoster` 模式 2 建的，
	// 混編的郡裡別的勢力的人也在名單上）；玩家那一條照舊。
	if x == nil || x.Location != prefectureID ||
		(needLord && x.Faction != by) {
		return ErrUnknownUnit
	}
	// **說明書的數字是下界**：原版在每一項上面再加一次 `RND(2)`
	// （`TreasureEffect` 的說明，`L1`）。
	di, dw, dc := TreasureEffect(t)
	bump := func(d, salt int) int {
		if d == 0 {
			return 0
		}
		return d + g.roll(prefectureID, targetIndex, int(t), salt)%2
	}
	di, dw, dc = bump(di, 1), bump(dw, 2), bump(dc, 3)
	x.Intel = raiseTo(x.Intel, di, TreasureCap)
	x.War = raiseTo(x.War, dw, TreasureCap)
	x.Charm = raiseTo(x.Charm, dc, TreasureCap)
	f.Treasury[t]--
	if x.HasLoyalty() {
		// 忠誠跟著提升**之後**的那個能力走（美女那一支不看能力）。
		ability := int(x.Intel)
		spread := TreasureLoyaltySpread
		switch t {
		case TreasureBlade, TreasureHorse:
			ability = int(x.War)
		case TreasureBeauty:
			ability, spread = 0, TreasureBeautyLoyaltySpread
		}
		roll := g.roll(prefectureID, targetIndex, int(t), 4) % spread
		x.Loyalty = uint8(clampTo(int(x.Loyalty)+
			TreasureLoyaltyGain(t, ability, roll), 100))
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
	// **挖角的 100 金是直接扣的**（原版 `0x1de7a` 的
	// `subw es:[bx+0x492], 100`），不經過電腦諸侯的折扣常式。
	if p.Gold < CostHeadhunt {
		return ErrNoGold
	}
	if g.ActiveGenerals(prefectureID) >= MaxGeneralsPerPrefecture {
		return ErrTooManyGens
	}
	if !g.Headhuntable(t, prefectureID) {
		return fmt.Errorf("%s：%w", tf("msg.unmoved", t.Name), ErrDeclined)
	}
	p.Gold -= CostHeadhunt
	p.Commanded = true

	// **成敗照原版**（`0x1dc0a`，`L0`）：招募方開的條件對上目標的抵抗，
	// 而回傳的不是布林是「成功之後的忠誠」——算出來 <= 0 就當失敗。
	lordCharm, mine := 50, 0
	if l := g.Lord(by); l != nil {
		lordCharm = int(l.Charm)
	}
	if f := g.Faction(by); f != nil {
		mine = f.Prestige
	}
	theirs := 0
	if f := g.Faction(t.Faction); f != nil {
		theirs = f.Prestige
	}
	resist := HeadhuntResistance(int(t.Loyalty), int(t.War), int(t.Intel), theirs)
	if theirs >= HeadhuntLuckFloor+
		g.roll(prefectureID, targetIndex, 1)%HeadhuntLuckSpread {
		resist += g.roll(prefectureID, targetIndex, 2) % max(theirs/2, 1)
	}
	if int(t.Loyalty) >= HeadhuntZealFloor+
		g.roll(prefectureID, targetIndex, 3)%HeadhuntZealSpread {
		resist += g.roll(prefectureID, targetIndex, 4) % HeadhuntZealBonus
	}
	if f := g.Faction(t.Faction); f != nil && f.Treasury[TreasureSeal] > 0 {
		resist += HeadhuntSealPenalty
	}
	won := HeadhuntNewLoyalty(int(t.Loyalty), mine)
	if resist >= HeadhuntOffer(lordCharm, mine, 0) || won <= 0 {
		return fmt.Errorf("%s：%w", tf("msg.unmoved", t.Name), ErrDeclined)
	}
	old := t.Faction
	wasGovernor := t.Status.Governs()
	at := t.Location
	t.Faction = by
	t.Status = state.StatusOfficer
	t.Loyalty = uint8(won)
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

// AutonomyAILevel 是自治的郡跑電腦分派器時用的 AI 等級
// （原版 `0x17572`，`L0`、`[base]`）。
//
// 郡的回合入口讀州郡 offset 12，非零就拿 `值 − 1` 當等級去跑
// `0xe8d2`。所以**內政型走等級 0、軍事型走等級 1、自冶型走等級 2**
// ——那三個等級的內政係數正好符合各自的名字：等級 0 開墾的底 50、
// 防洪除數 10（最勤於內政），等級 1 是 60／15（最懶），
// 等級 2 也是 60／15 但擲的範圍一樣（`docs/mechanics/70-ai` §2.5）。
//
// 第二個回傳值是「這個郡要不要交給電腦跑」。
func AutonomyAILevel(a Autonomy) (int, bool) {
	if a <= AutoNormal || a > AutoSelf {
		return 0, false
	}
	return int(a) - 1, true
}

// AutonomousFor 回報這個郡這個月要不要由電腦代管。
//
// **君主在的郡不自治**（原版 `0x17566`：主事者身分是 0 就跳過）——
// 主公親自坐鎮的地方輪不到太守自作主張。
func (g *State) AutonomousFor(prefectureID int) (int, bool) {
	p := g.Prefecture(prefectureID)
	if p == nil || !p.Owned() {
		return 0, false
	}
	level, ok := AutonomyAILevel(p.Autonomy)
	if !ok {
		return 0, false
	}
	if x := g.Governor(prefectureID); x == nil || x.Status == state.StatusLord {
		return 0, false
	}
	return level, true
}
