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
	// **無主的郡也走得進去**（`0x18ce1`，`L0`）：原版建目的地清單時，
	// 鄰郡的所屬是 `0xFF` 就直接放行，其餘要與本郡同屬。郡的歸屬是
	// 從人物表導出來的，所以搬進去就等於占領——電腦諸侯的「出兵」
	// 有四分之一的機率走的正是這一條（`docs/mechanics/70-ai` §2.13.6）。
	if dst == nil || (dst.Owned() && dst.Owner != by) {
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

// Train 是「訓練兵士」（說明書 p.20）：提高訓練度，不花錢。
//
// **對整個守軍跑一遍，不是挑一位**（`L0`、`[base]`）。原版的常式
// （線性 `0xbd70`）走一份清單，長度存在清單自己的前面：
//
//	cmp es:[0xc], ax                ; i 超過長度就結束
//	cmp word es:[bx+0x2226], 0      ; 兵士數是不是零
//	je  → mov word [bp-2], 0        ; 是 → 訓練度歸零
//	否則 → 訓練度 += (智/3 + 武/2) / 除數，上限 100
//
// **沒有兵的人訓練度會被歸零**——一支沒有兵的部隊「操演」不出東西，
// 而它下次拿到兵時是從零開始。這一條說明書沒寫。
func (g *State) Train(prefectureID int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	div := AITrainDivisor(g.AILevel(by))
	for _, x := range g.Garrison(prefectureID) {
		if x.Soldiers == 0 {
			// 沒有兵的部隊訓練度與武裝度都歸零（`0xbd70`／`0xc168`，
			// 兩支常式是同一個形狀）。
			x.Training = 0
			x.Arms = 0
			continue
		}
		add := (int(x.Intel)/3 + int(x.War)/2) / div
		x.Training = uint8(clampTo(int(x.Training)+add, 100))
	}
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
		// **原版是逐人先除以 100 再累加**（`0xc35b` 的
		// `fmul qword ds:[0xa5c8]` ＝ ×0.01，然後轉整數才加進 32 位元的
		// 累計）。放到最後才除會得到差一兩點的平均值。
		wTrain += x.Soldiers * int(x.Training) / 100
		wArms += x.Soldiers * int(x.Arms) / 100
	}
	if total > capSum {
		return ErrNoRoom
	}
	train, arms := 0, 0
	if total > 0 {
		train, arms = wTrain*100/total, wArms*100/total
	}
	// **依各人的上限比例分配，四捨五入**（原版 `0xc2c4` 在轉回整數之前
	// 加 `ds:[0xa5de]` ＝ 0.5，`L0`）。除得不盡時總數會差幾個人，
	// 原版沒有補回去——郡的總兵力是導出值（駐軍加總），不會因此失衡。
	left := total
	for _, x := range units {
		n := 0
		if capSum > 0 {
			n = (total*x.TroopCap()*2/capSum + 1) / 2
		}
		if n > x.TroopCap() {
			n = x.TroopCap()
		}
		x.Soldiers = n
		x.Training = uint8(train)
		x.Arms = uint8(arms)
		left -= n
	}
	// 上限擋住的剩餘塞回還有空間的人。**只補不足，不削多餘**：
	// 四捨五入會讓總數比原本多幾個，而原版就是這樣——它不做校正，
	// 郡的總兵力是駐軍加總算出來的，不會因此失衡。
	for _, x := range units {
		if left <= 0 {
			break
		}
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
	cost := g.price(by, FortCost(p.PriceLevel))
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
	// **買米不套電腦諸侯的折扣**：原版買米那一支（`0xc634`）根本不經過
	// 折扣常式 `0xec24`，它是市場交易不是勢力支出。
	//
	// 一金買到 `RicePerGold` 單位，**買到的米按實際花掉的金重算**——
	// 除不盡的零頭拿不到（原版 `0xc73e` 是拿「花掉的金 × 量」回填米）。
	rate := RicePerGold(p.PriceLevel)
	cost := units / rate
	if p.Gold < cost {
		return ErrNoGold
	}
	p.Gold -= cost
	p.Rice = clampTo(p.Rice+cost*rate, MaxRice)
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
	// ⚠ **賣米的比率還沒從原版讀出來**（買米是 `RicePerGold`，`L0`）。
	// 這裡仍是 remake 自己的換算，`docs/design/02` 記著。
	p.Rice -= units
	p.Gold = clampTo(p.Gold+units*int(p.PriceLevel)/100, MaxGold)
	p.Commanded = true
	return nil
}

// Relief 是「開倉賑民」（說明書 p.22）：撥米賑濟百姓換民眾忠誠，
// **太守魅力越高效果越好**。
func (g *State) Relief(prefectureID, gold int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if gold <= 0 {
		return fmt.Errorf("game: 賑民要撥出正的金額，拿到 %d", gold)
	}
	fee := g.price(by, gold)
	if p.Gold < fee {
		return ErrNoGold
	}
	charm := 50
	if gov := g.Governor(prefectureID); gov != nil {
		charm = int(gov.Charm)
	}
	p.Gold -= fee
	add := ReliefGain(int(p.PriceLevel), p.Population, gold, charm, g.aiLevelOf(by))
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
