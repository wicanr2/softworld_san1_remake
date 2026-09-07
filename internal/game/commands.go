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
//
// **名單不比對勢力**（`L0`＋`L1`）。原版的 `0xc2c4` 用
// `buildRoster(郡, 2)`，模式 2 的條件只有「所在郡相同」與
// 「身分 ∈ {0,1,2,3}」（`docs/re/07` §6）。同一個郡站得下兩個勢力的
// 武將——郡的所屬是每回合掃全部 350 筆人物重算的，後寫的蓋前寫的
// （`0x1e394`），所以有主的郡裡留著別人的敗兵是**表得出來的盤面**。
// 種了外人再量，28 個混編的郡次原版 28 次都把他一起攤平
// （`internal/parity/mixed_oracle_test.go`）。
//
// 多一道勢力比對的代價不是「比較安全」而是**整批命令中斷**：
// `ApplyAll` 一道失敗就不跑後面的，於是少算一整個勢力的行動。
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
		if x == nil || !x.Employed() || x.Location != prefectureID {
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
	// **依各人的上限比例分配**（`TroopShare`，`L1`）。除得不盡時總數會差
	// 幾個人，原版沒有補回去——郡的總兵力是導出值（駐軍加總），
	// 不會因此失衡。
	for _, x := range units {
		x.Soldiers = TroopShare(x.TroopCap(), total, capSum)
		x.Training = uint8(train)
		x.Arms = uint8(arms)
	}
	p.Commanded = true
	return nil
}

// ---- 4. 內政 ------------------------------------------------------------

// BuildFort 是「建築關寨」（原版 `0x1aa83`–`0x1abcf`，`L0`、`[base]`）。
//
// 三道門與說明書 p.21 一致，而且都在碼裡讀得到：
//
//   - 上限 5 座（`0x1aa95` 的 `cmp 5`，訊息「本郡已有%d個關寨\n不能再建了」）
//   - 費用 ＝ 當月物價 × 100（`0x1aae2` 的 `imul 100`）
//   - 監工將領**謀略大於 79**（提示字串 `DS:0x706e` 自己寫著）
//
// 原版接著讓玩家在該郡的戰場地圖上挑一格放關寨（`0x1acba` 開的是
// 戰術層的地圖畫面），然後 offset 25 加一、金扣掉。
// **關寨數與地圖是同一件事的兩份記錄**，所以這裡也要一起改；
// remake 自己挑格子（原版由玩家指），那是登記在案的差異。
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
	spot := fortSpotFor(p.BattleField)
	if spot < 0 {
		return ErrTooManyForts // 地圖上沒有可以蓋的格子
	}
	// 原版寫的是 `(格 & 0xF6) | 0x06`（`0x1afae`）：低四位變 6、高四位留著。
	p.BattleField[spot] = p.BattleField[spot]&0xF6 | fortTerrain
	p.Gold -= cost
	p.Forts++
	p.Commanded = true
	return nil
}

// fortTerrain 是關寨的地形碼（`docs/spec/003` §3.3）。
const fortTerrain = 6

// CanBuildFortOn 回報一格能不能蓋關寨（`0x1aeba`–`0x1aed1`，`L0`）。
//
// 原版兩道門：
//
//	DS:0x7134[地形碼] != 0        ; 只有山丘(2)、平原(7)、樹林(8) 是 1
//	(格 & 0xF0) >= 0xA0           ; 高四位 0–9 是通往鄰郡的通道
//
// 第二道擋掉的正是通道格——蓋在那裡會把鄰郡從地圖上封死，而畫面上
// 只看得出「敵軍再也沒有從那一邊來過」。城池標記(10)與四個軍團起點
// (11–14) 過得了第二道，但它們的地形碼過不了第一道。
func CanBuildFortOn(cell byte) bool {
	if cell == 0xFF { // 圖外
		return false
	}
	if !fortableTerrain[cell&0x0F] {
		return false
	}
	return cell&0xF0 >= 0xA0
}

// fortableTerrain 是 `DS:0x7134` 那張十六格的表（`L0`）。
var fortableTerrain = [16]bool{2: true, 7: true, 8: true, 10: true, 11: true, 12: true}

// fortSpotFor 挑一格拿來蓋關寨。
//
// 原版由玩家在地圖上指（`0x1acba` 的游標畫面，`DS:0x70cc`「數字鍵選方向」），
// remake 自己挑——**那是登記在案的差異**。挑的順序先平原後其他，
// 但收的範圍與原版一樣寬：只挑平原的話，地圖上平原都被佔滿而山丘、
// 樹林還空著時，remake 會說蓋不了而原版蓋得起來。
func fortSpotFor(field []byte) int {
	fallback := -1
	for i, b := range field {
		if !CanBuildFortOn(b) {
			continue
		}
		if b&0x0F == 7 { // 平原優先
			return i
		}
		if fallback < 0 {
			fallback = i
		}
	}
	return fallback
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
	// **電腦的匯率隨 AI 等級變**（`AIRicePerGold`，`0x0c7c0` 起六支）：
	// 等級 5 是除以 3，同一個物價下換到的米是玩家的三倍有餘。
	rate := RicePerGold(p.PriceLevel)
	if f := g.Faction(by); f != nil && f.ByComputer {
		rate = AIRicePerGold(p.PriceLevel, f.AILevel)
	}
	cost := units / rate
	if p.Gold < cost {
		return ErrNoGold
	}
	p.Gold -= cost
	p.Rice = clampTo(p.Rice+cost*rate, MaxRice)
	p.Commanded = true
	return nil
}

// SellRice 是「賣出米糧」（原版 `0x1b45e` 起，`L0`、`[base]`）：
// 財庫上限 30000。
//
// **比率與買米是同一條**（`RicePerGold`，`0x1b49c` 與 `0x1b20e` 算的
// 是同一個量）：買是「一金換 rate 米」，賣是「rate 米換一金」。
// 原版的提示字串把這件事寫在臉上——買米問「1 金 = %d 米」，
// 賣米問「%d 米 = 1 金」。
//
// 所以同一個月買進再賣出剛好不賺不賠，**零頭還會被整數除法吃掉**：
// 賣 rate−1 米一個銅板也拿不到。
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
	rate := RicePerGold(p.PriceLevel)
	gold := units / rate
	p.Rice -= gold * rate // 換不到一金的零頭留在倉裡
	p.Gold = clampTo(p.Gold+gold, MaxGold)
	p.Commanded = true
	return nil
}

// Relief 是「開倉賑民」（說明書 p.22）：撥米賑濟百姓換民眾忠誠，
// **太守魅力越高效果越好**。
//
// **收兩次錢**（`L1`、`0xc93d` 與 `0xc9d6`）：先扣整份撥款，寫回忠誠之前
// 再照**實際**漲到的幅度收一次（`ReliefSecondCharge`）。99 次實跑
// 每一次都收了第二筆（`docs/playtest/02`）。
//
// 忠誠**只有上限沒有下限**，而且寫回去的是低位那個 byte——
// 撥款大到讓 `量 × 金` 溢位時，民心會直接掉成一個看起來莫名其妙的數。
// 那是原版的行為（`ReliefGain` 的說明）。
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
	level := g.aiLevelOf(by)
	add := ReliefGain(int(p.PriceLevel), p.Population, gold, charm, level)
	full := int(p.PublicLoyalty) + add
	if full > 100 {
		full = 100
	}
	p.Gold -= g.price(by, ReliefSecondCharge(full-int(p.PublicLoyalty),
		ReliefRateOf(int(p.PriceLevel), level), ReliefPerStep(p.Population)))
	p.PublicLoyalty = uint8(full)
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
