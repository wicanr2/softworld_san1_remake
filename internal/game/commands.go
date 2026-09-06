package game

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 說明書十類指令裡的平時指令。編號與手冊相同（`docs/reference/01-manual-10`）。
//
// 條件與花費有出處（手冊頁碼標在各函式上），**效果的量沒有**——
// 那些走 `tuning.go` 的 `Tune*` 常數，一眼看得出是 remake 自己選的。

var (
	ErrNoRice       = fmt.Errorf("糧倉不足")
	ErrNotAdjacent  = fmt.Errorf("兩郡不相鄰")
	ErrNeedIntel80  = fmt.Errorf("謀略不足 80")
	ErrTooManyForts = fmt.Errorf("城寨已達五座")
	ErrNotLord      = fmt.Errorf("只有君主能下這個命令")
	ErrLordCantBe   = fmt.Errorf("君主不得兼任")
	ErrAlreadyPaid  = fmt.Errorf("這個月已經賞賜過了")
	ErrNoTreasure   = fmt.Errorf("寶庫裡沒有這件寶物")
	ErrCantGift     = fmt.Errorf("這件寶物不能送人")
	ErrNoGovernor   = fmt.Errorf("移出之後這個郡沒有人治理")
	ErrNoOne        = fmt.Errorf("找不到合適的人選")
	ErrTooManyGens  = fmt.Errorf("本郡的將軍已達 50 人")
)

// MaxGeneralsPerPrefecture 是一個郡最多幾位現役將軍。
//
// **手冊沒寫這條，是原版自己說的**：`AA.EXE` 有 `本郡已有50位將軍`
// （`0x4907f`，登用人才）與 `本郡將軍已達50人`（`0x49390`，登用他國人才）
// 兩條訊息，另有開發時期留下的 `War Gen <1 or >50`
// （`docs/re/04` §9、§15）。
//
// ⚠ **只在那兩個指令上擋。** 調動軍隊與戰役之後駐進來的部隊，
// 原版有沒有一樣擋還沒有證據；沒有證據的地方不自己加規則。
const MaxGeneralsPerPrefecture = 50

// ---- 2. 軍事 ------------------------------------------------------------

// Move 是「調動軍隊」（說明書 p.19）：把一位將領調到相鄰的己方州郡，
// 金米可一併隨行。
//
// `[HARD]` **主事者移出之前要先有接手的人。** 手冊寫「諸侯或太守移動後，
// 需指派新太守治理」——這裡的做法是**擋下來**而不是留下一個沒人治理的郡：
// 一個安靜地變成無主的郡，在畫面上只看得出顏色變了。
func (g *State) Move(from, to, generalIndex, gold, rice int, by state.FactionID) error {
	src, err := g.canOrder(from, by)
	if err != nil {
		return err
	}
	dst := g.Prefecture(to)
	if dst == nil || !dst.Owned() || dst.Owner != by {
		return ErrNotYours
	}
	if !g.Adjacent(from, to) {
		return ErrNotAdjacent
	}
	x := g.General(generalIndex)
	if x == nil || x.Faction != by || x.Location != from {
		return ErrUnknownUnit
	}
	if gold < 0 || rice < 0 {
		return fmt.Errorf("game: 隨行的金米不能是負數")
	}
	if src.Gold < gold {
		return ErrNoGold
	}
	if src.Rice < rice {
		return ErrNoRice
	}
	if x.Status.Governs() && g.successorFor(from, x.Index, by) == nil {
		return ErrNoGovernor
	}
	if x.Status.Governs() {
		succ := g.successorFor(from, x.Index, by)
		succ.Status = state.StatusGovernor
		x.Status = state.StatusOfficer
	}
	src.Gold -= gold
	dst.Gold += gold
	src.Rice -= rice
	dst.Rice += rice
	x.Location = to
	src.Commanded = true
	return nil
}

// successorFor 找一位可以接手治理的守將（不含 exclude 本人）。
// 魅力高的比較能勝任太守之職（說明書 p.23）。
func (g *State) successorFor(prefectureID, exclude int, by state.FactionID) *General {
	var best *General
	for _, x := range g.Garrison(prefectureID) {
		if x.Faction != by || x.Index == exclude || x.Status == state.StatusLord {
			continue
		}
		if best == nil || x.Charm > best.Charm {
			best = x
		}
	}
	return best
}

// Transport 是「運送錢糧」（說明書 p.20）：送到我軍其他州郡，
// **太守魅力值越高，途中損耗越少**。不限相鄰。
func (g *State) Transport(from, to, gold, rice int, by state.FactionID) error {
	src, err := g.canOrder(from, by)
	if err != nil {
		return err
	}
	dst := g.Prefecture(to)
	if dst == nil || !dst.Owned() || dst.Owner != by || to == from {
		return ErrNotYours
	}
	if gold < 0 || rice < 0 {
		return fmt.Errorf("game: 運送的金米不能是負數")
	}
	if src.Gold < gold {
		return ErrNoGold
	}
	if src.Rice < rice {
		return ErrNoRice
	}
	charm := 0
	if gov := g.Governor(from); gov != nil {
		charm = int(gov.Charm)
	}
	keep := func(n int) int {
		loss := n * TuneTransportLoss * (100 - charm) / 10000
		return n - loss
	}
	src.Gold -= gold
	src.Rice -= rice
	dst.Gold = clampTo(dst.Gold+keep(gold), MaxGold)
	dst.Rice = clampTo(dst.Rice+keep(rice), MaxRice)
	src.Commanded = true
	return nil
}

// ---- 3. 兵士 ------------------------------------------------------------

// Train 是「訓練兵士」（說明書 p.20）：提高訓練度，
// **各將的能力影響其麾下的訓練度提升**。不花錢。
func (g *State) Train(prefectureID, generalIndex int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	x := g.General(generalIndex)
	if x == nil || x.Faction != by || x.Location != prefectureID {
		return ErrUnknownUnit
	}
	add := TuneTrainBase + int(x.Intel)/TuneTrainIntel
	x.Training = uint8(clampTo(int(x.Training)+add, 100))
	p.Commanded = true
	return nil
}

// Redistribute 是「調整兵力」（說明書 p.21）：把幾支部隊打散重編，
// **全軍的訓練度成為平均值**，兵員必須完全分配下去。
//
// 平均是**以兵數加權**的：一支一千人的精兵與一支十人的新兵混編，
// 結果不該是兩者的算術平均。
func (g *State) Redistribute(prefectureID int, indices []int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if len(indices) < 2 {
		return fmt.Errorf("game: 調整兵力至少要兩支部隊")
	}
	var units []*General
	total, capSum := 0, 0
	var wTrain, wArms int
	for _, i := range indices {
		x := g.General(i)
		if x == nil || x.Faction != by || x.Location != prefectureID {
			return ErrUnknownUnit
		}
		units = append(units, x)
		total += x.Soldiers
		capSum += x.TroopCap()
		wTrain += x.Soldiers * int(x.Training)
		wArms += x.Soldiers * int(x.Arms)
	}
	if total > capSum {
		return ErrNoRoom
	}
	train, arms := 0, 0
	if total > 0 {
		train, arms = wTrain/total, wArms/total
	}
	// 依各人的上限比例分配，餘數給第一位——**兵員必須完全分配下去**。
	left := total
	for i, x := range units {
		n := 0
		if capSum > 0 {
			n = total * x.TroopCap() / capSum
		}
		if i == len(units)-1 {
			n = left
		}
		if n > x.TroopCap() {
			n = x.TroopCap()
		}
		x.Soldiers = n
		x.Training = uint8(train)
		x.Arms = uint8(arms)
		left -= n
	}
	if left > 0 { // 上限擋住了剩餘，塞回還有空間的人
		for _, x := range units {
			room := x.TroopCap() - x.Soldiers
			if room <= 0 {
				continue
			}
			take := left
			if take > room {
				take = room
			}
			x.Soldiers += take
			left -= take
			if left == 0 {
				break
			}
		}
	}
	if left != 0 {
		return fmt.Errorf("game: 有 %d 兵分配不出去", left)
	}
	p.Commanded = true
	return nil
}

// ---- 4. 內政 ------------------------------------------------------------

// BuildFort 是「建築關寨」（說明書 p.21）：
// 監工將領**謀略不得少於 80**、每郡最多五座、每寨費用是當月物價的 100 倍。
func (g *State) BuildFort(prefectureID, generalIndex int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	x := g.General(generalIndex)
	if x == nil || x.Faction != by || x.Location != prefectureID {
		return ErrUnknownUnit
	}
	if x.Intel < MinIntelForChief {
		return ErrNeedIntel80
	}
	if p.Forts >= MaxForts {
		return ErrTooManyForts
	}
	cost := FortCost(p.PriceLevel)
	if p.Gold < cost {
		return ErrNoGold
	}
	p.Gold -= cost
	p.Forts++
	p.Commanded = true
	return nil
}

// Rest 是「休息」（說明書 p.21）：不做任何事，但耗掉這個月的指令。
func (g *State) Rest(prefectureID int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	p.Commanded = true
	return nil
}

// ---- 5. 商業 ------------------------------------------------------------

// BuyRice 是「買入米糧」（說明書 p.22）：依物價購米入倉，糧倉上限 30000。
//
// 物價是每單位米的金價基準。**換算率還沒解**：這裡當成
// 「一單位米 ＝ 物價 ÷ 100 金」，讓物價 50 時買一單位約半金。
func (g *State) BuyRice(prefectureID, units int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if units <= 0 {
		return fmt.Errorf("game: 買入的米要是正數")
	}
	if p.Rice+units > MaxRice {
		return fmt.Errorf("game: 糧倉上限 %d，買不下 %d", MaxRice, units)
	}
	cost := units * int(p.PriceLevel) / 100
	if p.Gold < cost {
		return ErrNoGold
	}
	p.Gold -= cost
	p.Rice += units
	p.Commanded = true
	return nil
}

// SellRice 是「賣出米糧」（說明書 p.22）：依物價以米換金，財庫上限 30000。
func (g *State) SellRice(prefectureID, units int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if units <= 0 {
		return fmt.Errorf("game: 賣出的米要是正數")
	}
	if p.Rice < units {
		return ErrNoRice
	}
	p.Rice -= units
	p.Gold = clampTo(p.Gold+units*int(p.PriceLevel)/100, MaxGold)
	p.Commanded = true
	return nil
}

// Relief 是「開倉賑民」（說明書 p.22）：撥米賑濟百姓換民眾忠誠，
// **太守魅力越高效果越好**。
func (g *State) Relief(prefectureID int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if p.Rice < TuneReliefRice {
		return ErrNoRice
	}
	charm := 50
	if gov := g.Governor(prefectureID); gov != nil {
		charm = int(gov.Charm)
	}
	p.Rice -= TuneReliefRice
	add := TuneReliefLoyalty * charm / 50
	p.PublicLoyalty = uint8(clampTo(int(p.PublicLoyalty)+add, 100))
	p.Commanded = true
	return nil
}

func clampTo(v, max int) int {
	if v > max {
		return max
	}
	if v < 0 {
		return 0
	}
	return v
}
