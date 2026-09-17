package game

import (
	"fmt"
	"math/big"

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

// Move 是玩家的「調動軍隊」（`0x18bc8`，`L0`、`[base]`）：把挑好的一份名單調到
// 相鄰、無主或同一個主人的郡，金米可一併隨行。
//
//	名單：多選清單 `0x18286`（模式 2，最多 50 − 目標郡存的現役將數，`0x18d50`）
//	金／米上限：min(30000 − 目標郡的量, 來源郡的量)（`0x18dc1`／`0x18e37`）
//	搬運：`0x1938a`，與電腦的移防同一支（`relocateSupplies`）
//
// **碼上沒有「主事者要有人接手」的閘門**，君主或太守一起走、把郡搬空都照搬；
// 主事者由兩郡的重整守將清單（`0x1949e`）重選。
func (g *State) Move(from, to int, generals []int, gold, rice int, by state.FactionID) error {
	src, err := g.canOrder(from, by)
	if err != nil {
		return err
	}
	dst := g.Prefecture(to)
	// **無主的郡也走得進去**（`0x18ce1`，`L0`）：原版建目的地清單時，
	// 鄰郡的所屬是 `0xFF` 就直接放行，其餘要與本郡同屬。
	if dst == nil || (dst.Owned() && dst.Owner != src.Owner) {
		return ErrNotYours
	}
	if !g.Adjacent(from, to) {
		return ErrNotAdjacent
	}
	if len(generals) == 0 || len(generals) > MaxGeneralsPerPrefecture-g.StoredActiveGenerals(to) {
		return ErrUnknownUnit
	}
	for _, i := range generals {
		x := g.General(i)
		if x == nil || x.Location != from || int(x.Status) > 3 {
			return ErrUnknownUnit
		}
	}
	if gold < 0 || rice < 0 {
		return fmt.Errorf("game: 隨行的金米不能是負數")
	}
	if gold > min(RelocateCap-dst.Gold, src.Gold) {
		return ErrNoGold
	}
	if rice > min(RelocateCap-dst.Rice, src.Rice) {
		return ErrNoRice
	}
	relocateSupplies(g, src, dst, generals, to, gold, rice)
	g.RefreshGarrison(from)
	g.RefreshGarrison(to)
	g.endTurn(src)
	return nil
}

// MoveLimits 是調動軍隊那三問的上限：最多幾位、金、米（`0x18d50`／`0x18dc1`／`0x18e37`）。
func (g *State) MoveLimits(from, to int) (people, gold, rice int) {
	src, dst := g.Prefecture(from), g.Prefecture(to)
	if src == nil || dst == nil {
		return 0, 0, 0
	}
	return MaxGeneralsPerPrefecture - g.StoredActiveGenerals(to),
		min(RelocateCap-dst.Gold, src.Gold), min(RelocateCap-dst.Rice, src.Rice)
}

// RelocateCap 是移防之後目標郡的金／米上限（`0x19413`／`0x1942c` 的
// `movw $0x7530`，`L0`）。**它不是「加到這裡就停」的上限**——原版是先在
// 16 位元把兩個數加起來，只有加出來變成負數（超過 32767）時才整個換成
// 30000。所以 30001 到 32767 那一段留得住，超過就掉回 30000。
const RelocateCap = 30000

// Relocate 是**電腦諸侯**的移防（`docs/spec/007`，`0x1938a`，`L0`）。
//
// 與 `Move`（玩家的「調動軍隊」）搬運是同一支，閘門不同：玩家那一條要相鄰、
// 有人數與金米上限；這一支把整份出征名單搬過去，允許把來源郡搬空——郡的歸屬在
// 下一個郡回合由 `0x1e394` 從人物表重算，搬空的郡就此變成無主
// （`docs/mechanics/70-ai` §「搬進去之後」）。
//
// **不要合併成一條規則**：兩支是不同的碼，合併等於替其中一支發明規則。
func (g *State) Relocate(from, to int, force []int, gold, rice int, by state.FactionID) error {
	src, err := g.canOrder(from, by)
	if err != nil {
		return err
	}
	dst := g.Prefecture(to)
	if dst == nil || (dst.Owned() && dst.Owner != by) {
		return ErrNotYours
	}
	if !g.Adjacent(from, to) {
		return ErrNotAdjacent
	}
	if len(force) == 0 {
		return ErrUnknownUnit
	}
	for _, i := range force {
		x := g.General(i)
		if x == nil || x.Faction != by || x.Location != from {
			return ErrUnknownUnit
		}
	}
	if gold < 0 || rice < 0 {
		return fmt.Errorf("game: 隨行的金米不能是負數")
	}
	relocateSupplies(g, src, dst, force, to, gold, rice)
	// **來源郡與目標郡各重整一次守將清單**（`0x1949e`，`docs/mechanics/70-ai`
	// §「移動本身」），兵士與現役將兩欄跟著刷新。
	g.RefreshGarrison(from)
	g.RefreshGarrison(to)
	g.endTurn(src)
	return nil
}

// relocateSupplies 是 `0x1938a` 本身——**兩條路徑共用一份**，因為原版
// 就是共用的：電腦的移防從 `0xb9f3` 進來，玩家的「調動軍隊」從
// `0x18fbc` 進來（`docs/spec/007` §1）。差別全在呼叫端的閘門。
//
//	名單裡每一位：人物 offset 19（所在郡）← 目標郡    ; 0x1939c–0x193c9
//	來源郡的金／米 −= 帶走的量，變負夾 0              ; 0x193df／0x193f8
//	目標郡的金／米 += 帶走的量，溢位夾 30000          ; 0x19411／0x1942a
func relocateSupplies(g *State, src, dst *Prefecture, force []int, to, gold, rice int) {
	for _, i := range force {
		if x := g.General(i); x != nil {
			x.Location = to
		}
	}
	src.Gold = max(src.Gold-gold, 0)
	src.Rice = max(src.Rice-rice, 0)
	dst.Gold = relocateAdd(dst.Gold, gold)
	dst.Rice = relocateAdd(dst.Rice, rice)
}

// relocateAdd 是 `0x1940c`／`0x19425` 的加法：在 **16 位元有號數**裡加，
// 變負才夾成 30000。
func relocateAdd(a, b int) int {
	v := int16(uint16(a) + uint16(b))
	if v < 0 {
		return RelocateCap
	}
	return int(v)
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
	src.Gold -= gold
	src.Rice -= rice
	dst.Gold = transportReceive(dst.Gold, TransportArrives(gold, charm))
	dst.Rice = transportReceive(dst.Rice, TransportArrives(rice, charm))
	g.endTurn(src)
	return nil
}

// TransportArrives 是運送 n 金（或米）實際到達多少（`0x192c7`–`0x1930c`，`L0`）：
//
//	到達 ＝ n × (來源主事者的魅力 + 50) ÷ 150
//
// 原版走浮點（`fild` 魅力+50、`fimul` n、`fmul` 常數 `DS:0xa7aa` ＝ 1/150、
// `ftol` 截尾）；那個 double 在 1/150 之上 4.3e-19，整數倍不會被截掉，
// 所以整數除法逐格相同（`TestZZTransportLoss`）。魅力 100 一分不少，
// 魅力 0 只到三分之一。
func TransportArrives(n, charm int) int {
	return n * (charm + 50) / 150
}

// transportReceive 是目的郡收下之後的庫存（`0x1931a`／`0x19333`，`L0`）：
// 16 位元帶號相加，**溢位（結果為負）才寫 30000**，沒溢位就照加——
// 所以運送可以把庫存推過 30000（最高 32767），與秋收、買米的上限不同。
func transportReceive(have, arrived int) int {
	if sum := have + arrived; sum <= 32767 {
		return sum
	}
	return MaxGold
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
	return g.TrainUnits(prefectureID, nil, by)
}

// TrainUnits 是訓練的本體；`indices` 給了就只練名單上的人。
//
// 原版那一支（`0xbd48`／加強版 `0xbc48`）走的是**郡回合共用的守將清單**
// `es:0x5a0`，不是當下的守軍：這一輪剛登用進來的人不在清單上（登用
// 每試一位才重建一次，成功的那一位之後沒有人再建），所以他這回合
// 不練。加強版月度對拍量到郡 29：剛登用的 159 原版留在 23，remake
// 把他練到 31，調整兵力攤平之後五個人差 2。
func (g *State) TrainUnits(prefectureID int, indices []int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	div := TrainDivisorPlayer
	if g.byComputer(by) {
		div = AITrainDivisor(g.AILevel(by))
	}
	list := g.Garrison(prefectureID)
	if indices != nil {
		list = list[:0:0]
		for _, i := range indices {
			if x := g.General(i); x != nil && x.Location == prefectureID {
				list = append(list, x)
			}
		}
	}
	for _, x := range list {
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
	g.endTurn(p)
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
	return g.redistribute(prefectureID, indices, by, 2)
}

// redistribute 的 minUnits 把玩家介面的「至少兩支」與原版 AI 表分開。
// 原版 AI 即使清單只有一支仍會跑百分比重算；玩家命令維持至少兩支。
func (g *State) redistribute(prefectureID int, indices []int, by state.FactionID,
	minUnits int) error {
	return g.redistributeWith(prefectureID, indices, by, minUnits, 0)
}

// redistributeWith 的 capSum 非 0 時當「總上限」用（電腦那一張表：分派器
// 入口算好的 `es:[0x347e]`，見 `RedistributeOrder.CapSum`）；玩家的命令
// 照當下的部隊算，總兵力超過總上限就擋（原版玩家那一條會問）。
func (g *State) redistributeWith(prefectureID int, indices []int, by state.FactionID,
	minUnits, capSum int) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if len(indices) < minUnits {
		return fmt.Errorf("game: 調整兵力至少要兩支部隊")
	}
	var units []*General
	total, liveCap := 0, 0
	var wTrain, wArms int
	for _, i := range indices {
		x := g.General(i)
		if x == nil || !x.Employed() || x.Location != prefectureID {
			return ErrUnknownUnit
		}
		units = append(units, x)
		total += x.Soldiers
		liveCap += x.TroopCap()
		// **原版是逐人先除以 100 再累加**（`0xc35b` 的
		// `fmul qword ds:[0xa5c8]` ＝ ×0.01，然後轉整數才加進 32 位元的
		// 累計）。放到最後才除會得到差一兩點的平均值。
		wTrain += x.Soldiers * int(x.Training) / 100
		wArms += x.Soldiers * int(x.Arms) / 100
	}
	if capSum <= 0 {
		capSum = liveCap
		if total > capSum {
			return ErrNoRoom
		}
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
	g.endTurn(p)
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
	// 沒指定位置就自己挑一格——**這是 remake 差異**，原版是玩家在
	// 「數字鍵選方向」那個畫面用游標挑的。
	return g.BuildFortAt(prefectureID, generalIndex, -1, by)
}

// BuildFortAt 指定蓋在哪一格（戰場地圖的索引，`列 × 12 + 欄`）。
// `spot` 給負數就沿用 `fortSpotFor` 自己挑。
func (g *State) BuildFortAt(prefectureID, generalIndex, spot int,
	by state.FactionID) error {
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
	if spot < 0 {
		spot = fortSpotFor(p.BattleField)
	}
	if spot < 0 || spot >= len(p.BattleField) ||
		!CanBuildFortOn(p.BattleField[spot]) {
		return ErrTooManyForts // 地圖上沒有可以蓋的格子
	}
	// 原版寫的是 `(格 & 0xF6) | 0x06`（`0x1afae`）：低四位變 6、高四位留著。
	p.BattleField[spot] = p.BattleField[spot]&0xF6 | fortTerrain
	p.Gold -= cost
	p.Forts++
	g.endTurn(p)
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
	g.endTurn(p)
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
	g.endTurn(p)
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
// BuyRiceForGold 是玩家那一條：**原版問的是金不是米**。提示是
// 「1 金 = %d 米\n您想買多少金的米(0-%d):」（`docs/re/04` §2 的 `0x4999f`），
// 打進去的數字是要花掉的金。實測送 100：郡庫金 9000 → 8900、
// 米 9000 → 9500，當月匯率 1 金 5 米（`docs/playtest/04`）。
//
// `BuyRice` 收的是**米的數量**，兩者差一個匯率。少了這一層，介面照著
// 原版的提示收數字再送進 `BuyRice`，玩家會用 100 金的價錢買到 100 米
// ——畫面與原版一樣，數字差五倍。
func (g *State) BuyRiceForGold(prefectureID, gold int, by state.FactionID) error {
	p := g.Prefecture(prefectureID)
	if p == nil || !p.Owned() || p.Owner != by {
		return ErrNotYours
	}
	rate := RicePerGold(p.PriceLevel)
	if f := g.Faction(by); f != nil && f.ByComputer {
		rate = AIRicePerGold(p.PriceLevel, f.AILevel)
	}
	return g.BuyRice(prefectureID, gold*rate, by)
}

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
	g.endTurn(p)
	return nil
}

// TradeRiceTo 是電腦諸侯那一支的米糧買賣（`0xc634`，`L0`、`[base]`）。
//
// **它是雙向的**：存糧低於目標就買，高於目標就賣，同一條算式：
//
//	量   ＝ (100 − 物價) ÷ 除數[等級]        ; 一金換幾單位米
//	缺口 ＝ 目標 − 米
//	剩金 ＝ clamp(金 − 缺口 ÷ 量, 0, 30000)  ; 浮點
//	買到 ＝ (金 − 剩金) × 量                 ; 浮點
//	金 ← trunc(剩金)；米 ← 米 + 買到
//
// 缺口為負時 `剩金 > 金`、`買到` 為負——**米降到目標、金往上補**。
// 代數上 `買到 == 缺口`（除非 `剩金` 被夾在 0），所以**米一定落在目標上**，
// 而金的變化是 `trunc(缺口 ÷ 量)`：賣 116 單位、量 8 換到 14 金
// （零頭被截掉），米卻是整整少 116。
//
// ⚠ 用浮點算，**不要先把 `剩金` 截成整數再回推買到**——那會讓米停在
// 目標上方幾個單位（量 8、缺口 116 時差 4）。
func (g *State) TradeRiceTo(prefectureID, target int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if target > MaxRice {
		target = MaxRice
	}
	rate := RicePerGold(p.PriceLevel)
	if f := g.Faction(by); f != nil && f.ByComputer {
		rate = AIRicePerGold(p.PriceLevel, f.AILevel)
	}
	// **逐字照抄那一串 8087 指令**（原版 `0xc6c3`–`0xc767`、加強版
	// `0xc4c6`–`0xc5a0`），順序不能動：
	//
	//	剩金 ← 金 − 缺口 ÷ 量        ; filds/fldl/fidivs/fsubrp，量是整數
	//	fstpl 剩金                   ; **落地成 double**
	//	剩金 夾在 [0, 30000]
	//	新米 ← (金 − 剩金) × 量 + 米 ; fsubl/fimuls/**fiadds**
	//	fstpl 新米                   ; 落地成 double
	//	金 ← trunc(剩金)；米 ← trunc(新米)
	//
	// ⚠ **米要先加進去再截尾**（`fiadds` 在 `ftol` 之前）。先把「買到」
	// 截成整數再相加會差一單位——`金` 上萬時 `缺口÷量×量` 會抵消掉有效
	// 位數，算出 −169.999… 或 −705.000…1，截尾的方向剛好相反。
	// 代數上那個乘積等於缺口，但**原版的捨入誤差是它行為的一部分**：
	// 郡 22 落在目標上（3060），郡 30 落在目標下面一格（3054）。
	//
	// ⚠ **暫存器是 64 位元有效位數，只有 `fstpl` 那兩處捨成 double**
	// （`x87` 的說明）。整段用 Go 的 `float64` 算會在每一步都多捨一次：
	// 加強版七月郡 3 賣米 3805 → 目標 1482、量 7，原版算出 1481.999…
	// 截成 1481，`float64` 算出剛好 1482.0——差的就是那一次捨入。
	// 加強版在 `fstpl 新米` 之後多一道 [0, 30000] 的夾（`0xc54a`–`0xc588`），
	// 原版沒有；目標已經先夾在 30000，兩版走到這裡的值一樣。
	x := func(v int) *big.Float { return x87(int64(v)) }
	asDouble := func(f *big.Float) *big.Float {
		d, _ := f.Float64()
		return new(big.Float).SetPrec(64).SetMode(big.ToNearestEven).SetFloat64(d)
	}
	quo := new(big.Float).SetPrec(64).SetMode(big.ToNearestEven).Quo(x(target-p.Rice), x(rate))
	left := asDouble(new(big.Float).SetPrec(64).SetMode(big.ToNearestEven).Sub(x(p.Gold), quo))
	if left.Sign() < 0 {
		left = x(0)
	}
	if left.Cmp(x(MaxGold)) > 0 {
		left = x(MaxGold)
	}
	spent := new(big.Float).SetPrec(64).SetMode(big.ToNearestEven).Sub(x(p.Gold), left)
	spent.Mul(spent, x(rate))
	newRice := asDouble(spent.Add(spent, x(p.Rice)))
	leftN, _ := left.Int64()
	riceN, _ := newRice.Int64()
	p.Gold = int(leftN)
	p.Rice = clampTo(int(riceN), MaxRice)
	if p.Rice < 0 {
		p.Rice = 0
	}
	g.endTurn(p)
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
	charm := 50
	if gov := g.Governor(prefectureID); gov != nil {
		charm = int(gov.Charm)
	}
	if !g.byComputer(by) {
		// **玩家那條發下去的是米不是金。** 原版的提示是
		// 「您給多少米(0-%d):」（`docs/re/04` §2 的 `0x49a88`），
		// 實測送 100：郡庫米 9000 → 8900、金一毛不動、民心 36 → 50
		//（`docs/playtest/04`）。
		//
		// 也**沒有第二次扣錢**：那是電腦那條「照實際增幅回頭付帳」的
		// 形狀（`ReliefSecondCharge`，`0xc9d6`），玩家照打進去的數字收。
		if p.Rice < gold {
			return ErrNoRice
		}
		p.Rice -= gold
		add := ReliefGainPlayer(p.Population, gold, charm)
		p.PublicLoyalty = uint8(clampTo(int(p.PublicLoyalty)+add, 100))
		g.endTurn(p)
		return nil
	}
	fee := g.price(by, gold)
	if p.Gold < fee {
		return ErrNoGold
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
	g.endTurn(p)
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
