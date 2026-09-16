package game

import (
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 四季事件（說明書 p.36–37）。
//
// **四季由 `es:[0x3f08]` 選**，原版四支常式的分工與這裡相同
// （`docs/re/06`）：春天是土地價值衰減與地震、夏天是水災與瘟疫、
// 秋天是秋收與蝗害、冬天是人口成長與進貢。
//
// 四種天災的**機率與損失幅度都是量到的**，各自的形狀不一樣——
// 地震純機率、水災只看洪水率、瘟疫要忠誠低且田瘦、蝗害要忠誠低但田肥。
// 手冊 p.36 的「天災多因人怨引起」只對瘟疫與蝗害成立。
const (
	// PopulationCap 是每個郡的人口上限（原版 `0x16f0a` 夾在 10000，
	// 存的值 ×100，`L0`）。
	PopulationCap = 1_000_000
	// PopulationOwnerlessChance 是無主郡成長的機率（`0x16ebd`，`L0`）：
	// `RND(100) < 30` 才長。有主的郡每次都長。
	PopulationOwnerlessChance = 30
	// 進貢的上限（`RND(5) + 8`，`L0`）。
	TributeCapSpread = 5
	TributeCapFloor  = 8

	// 冬季民亂的三個常數（`0x16f48`／`0x16f6c`，`L0`）：門檻是
	// `RND(20) + 50`（民眾忠誠）與 `RND(20) + 30`（人望）。
	UnrestSpread        = 20
	UnrestLoyaltyFloor  = 50
	UnrestPrestigeFloor = 30

	// TributeDivisorFloor 是「人才量 ÷ (RND(10) + 80)」裡的 80
	// （`0x1714e` 的 `add $0x50,%cx`，`L0`）。
	TributeDivisorFloor = 80
	// TreasuryCap 是寶庫裡每一種寶物的上限（`0x1731d`，`L0`）。
	// 欄位是一個 byte，原版自己夾在 100。
	//
	// ⚠ 不要與 `TreasureCap` 混淆——那是賞賜能把**能力值**提到的上限
	// （說明書 p.24：90 點），兩者是不同的東西。
	TreasuryCap = 100
)

// Event 是一則發生過的事件，給訊息列與測試用。
type Event struct {
	Prefecture int // 0 表示不屬於特定郡
	Text       string

	// Bubble 非 nil 時這一則在原版是**訊息常式**畫的「肖像＋對白泡泡」
	// （`0x3273e`，`docs/spec/005` §訊息框）；Text 可以是空的（原版那一句
	// 是片語表拼的，remake 的譯文放在 Bubble.Text）。
	Bubble *Bubble
}

// Bubble 是原版訊息常式（`0x3273e`）畫的那一格：框、說話者的肖像與名字、
// 兩行對白（`docs/spec/005` §訊息框，`L0`、`[base]`）。
//
// 呼叫端給的是框的四個角、肖像在左還是右、肖像號與三段片語；字色是進去
// 就擲的 `RND(8)`（`0x32d4d`）——**原版的對白字色是隨機的**，remake 把同一擲
// 的值放進 Color。
type Bubble struct {
	X1, Y1, X2, Y2 int  // 框的範圍（含肖像那一塊），螢幕座標
	Left           bool // 肖像在左（原版 side ＝ 0xFFFF）；否則在右
	Speaker        int  // 人物槽：肖像與名字從這裡取
	Color          int  // 對白的字色 0–7（`RND(8)`）
	Text           string

	// FaceOnly 為真是「只亮一張肖像」的那一格：不畫名字、泡泡與字，
	// 肖像貼在 (X1, Y1)。玩家尋訪找到人時原版先把那一位的肖像亮在
	// (488,88)、等一下，再清掉畫對白（`0x1bb7e`，`docs/spec/005` §9.3）。
	FaceOnly bool
}

// 尋訪找到的那一位亮肖像的位置（`0x1bb76`／`0x1bb7a`）。
const (
	SearchFaceX = 488
	SearchFaceY = 88
)

// searchEvents 是玩家尋訪之後的畫面（`0x1bb58`–`0x1bc45`，`L0`、`[base]`）：
// 找到人先亮那一位的肖像，再由尋訪者在下格報「主公洪福  發現名士」加
// 名字（片語 383）；沒找到只報「屬下無能  沒有找到人才」（384）。
// 肖像都在右邊（side 0）。
func (g *State) searchEvents(searcher, found *General) []Event {
	if found != nil {
		return []Event{
			{Prefecture: searcher.Location, Bubble: &Bubble{X1: SearchFaceX, Y1: SearchFaceY, Speaker: found.Index, FaceOnly: true}},
			g.bubbleEvent(searcher, false, false, tf("bub.found", personName(found.Name)), searcher.Index),
		}
	}
	return []Event{g.bubbleEvent(searcher, false, false, t("bub.notFound"), searcher.Index)}
}

// 原版訊息框固定用的兩個位置（右側面板那一塊）：上格與下格。
const (
	BubbleX1, BubbleX2 = 424, 615
	BubbleUpperY1      = 80 // `0x14a38`／`0x161c4`：(424,80)–(615,175)
	BubbleUpperY2      = 175
	BubbleLowerY1      = 180 // `0x14848`／`0x148eb`／`0x16270`：(424,180)–(615,275)
	BubbleLowerY2      = 275
)

// bubbleEvent 擲對白那一擲（`RND(8)`）並造一則帶泡泡的事件。
// upper 選上格或下格，left 是肖像在左。
func (g *State) bubbleEvent(x *General, upper, left bool, text string, salt ...int) Event {
	b := &Bubble{X1: BubbleX1, X2: BubbleX2, Left: left, Speaker: x.Index, Text: text}
	if upper {
		b.Y1, b.Y2 = BubbleUpperY1, BubbleUpperY2
	} else {
		b.Y1, b.Y2 = BubbleLowerY1, BubbleLowerY2
	}
	b.Color = g.Roll(MessageLines, salt...)
	at := x.Location
	if at < 1 || at > len(g.prefectures) {
		at = 0
	}
	return Event{Prefecture: at, Bubble: b}
}

// PendingEvents 交出內層常式（繼承、戰役分贓）累積的泡泡事件，交出就清掉。
// 呼叫端在自己的事件序列裡把它們接在該接的位置。
func (g *State) PendingEvents() []Event {
	out := g.pending
	g.pending = nil
	return out
}

// SeasonMonths 是四季常式真正會跑的四個月（`L0`、`0x15c48`）。
//
// **一年各跑一次，不是每個月都跑。** 原版的分派器讀的是月份
// （`es:[0x3f08]`），跳躍表比的是 1、4、7、10：其餘八個月直接 `retf`。
// 這決定了天災的頻率——每個月都判的話，地震、水災、瘟疫、蝗害
// 一年各有十二次機會而不是一次，**十二倍**。
var SeasonMonths = map[int]Season{1: Spring, 4: Summer, 7: Autumn, 10: Winter}

// RunSeason 跑這個月的季節事件，回傳發生了什麼。
//
// 呼叫時機是**推進到新的月份之後**——事件屬於新的那個月。
// 不在 `SeasonMonths` 裡的月份什麼都不跑。
func (g *State) RunSeason() []Event {
	season, ok := SeasonMonths[g.Date.Month]
	if !ok {
		return nil
	}
	switch season {
	case Spring:
		return g.spring()
	case Summer:
		return g.summer()
	case Autumn:
		return g.autumn()
	default:
		return g.winter()
	}
}

// floodBase 是各郡的洪水基礎值（`DS:0x679c`，43 個 word，AA.EXE 檔內
// 位移 0x484d0、ASV.EXE 0x3e8a5，`L0`、`[both]`，總和 158）。
//
// **各郡不同**——有些郡天生就容易淹。索引 0 是啞元郡，索引 42 是
// 最後一郡（值 3），表要數滿 43 格。
var floodBase = [state.PrefectureCount + 1]int{
	0, 0, 5, 4, 3, 1, 2, 5,
	7, 3, 3, 7, 2, 2, 7, 7,
	3, 2, 1, 2, 0, 5, 5, 3,
	10, 3, 12, 2, 2, 5, 5, 10,
	3, 3, 3, 1, 2, 5, 4, 1,
	2, 3, 3,
}

// FloodBase 是某個郡的洪水基礎值。
func FloodBase(prefectureID int) int {
	if prefectureID < 0 || prefectureID >= len(floodBase) {
		return 0
	}
	return floodBase[prefectureID]
}

// FloodRise 是每年四月洪水率自己的變動（`0x16500`–`0x16543`，`L0`）：
//
//	新洪水率 ＝ min(100, RND(舊 ÷ 4) + 舊 + 基礎[郡])
//
// **這件事每年四月都發生，與淹不淹無關。** `mov %ax,%dx` 存的是除法
// 之前的洪水率、`mov -0xa(%bp),%al` 讀回來的也是它——加的是整個舊值，
// 不是 `舊 % 4`。所以洪水率只會往上（每年至少加基礎值），只有防洪
// （`FloodDrop` ＝ 謀略 ÷ 10）壓得下來，最後停在 100。roll 是
// `RND(舊 ÷ 4)`，舊 < 4 時原版不抽、當 0。
func FloodRise(rate, base, roll int) int {
	v := roll + rate + base
	if v > 100 {
		v = 100
	}
	if v < 0 {
		v = 0
	}
	return v
}

// 水災的兩道判定（`0x16574`／`0x16585`，`L0`）：先 `RND(新洪水率)`、再
// `RND(65)`，`RND(65) + 5 >= RND(新洪水率)` 就不淹；過了才擲 `RND(100)`，
// `<= 80` 也不淹——第二道是無條件的 19%，洪水率滿檔也不是每年都淹。
const (
	FloodRollSpread = 65 // RND(65) + 5
	FloodRollFloor  = 5
	FloodGateSpread = 100 // 再擲一次 RND(100)，<= 80 就不淹
	FloodGateBar    = 80
)

// 瘟疫的兩道門檻（`0x167df`–`0x1683d`，`L0`）。
const (
	PlagueLoyaltySpread = 45 // RND(45) + 25
	PlagueLoyaltyFloor  = 25
	PlagueLandSpread    = 40 // RND(40) + 20
	PlagueLandFloor     = 20
)

// PlagueStrikes 回報隨機挑中的那個郡鬧不鬧瘟疫。
//
// **瘟疫不逐郡掃**，原版每年只挑一個郡（`RND(42) + 1`），不看所屬。
// 兩道門檻都要過：忠誠 70 以上一定安全（門檻上限 69），
// 25 以下一定過第一關；土地價值 59 以上安全、20 以下必過。
// 兩擲是一道一道來的（忠誠那一道沒過就不擲土地那一道，`0x16814`），
// 所以呼叫端要自己先判第一道再抽 landRoll。
//
// **這才是說明書「天災多因人怨引起」的出處**——水災完全不看忠誠。
func PlagueStrikes(loyalty, landValue, loyaltyRoll, landRoll int) bool {
	if loyalty >= loyaltyRoll+PlagueLoyaltyFloor {
		return false
	}
	return landValue < landRoll+PlagueLandFloor
}

// spring 是春天：年齡增長、體能衰退、老死、地震。
//
// 年齡一年只加一次，在**元月**——原版連走十六個月的觀測裡，
// 人物表全部 350 筆的年齡欄只在第 4 與第 16 輪變動，而那兩輪的畫面
// 分別是建安三年元月與建安四年元月（`L1`、`[base]`）。
func (g *State) spring() []Event {
	var out []Event
	if g.Date.Month != agingMonth {
		return nil
	}
	// **順序照 `0x15c5c`**（骰序要同）：土地價值衰減 → 玉璽 → 老死那一圈
	// → 加歲／忠誠／衰減／出頭那一圈 → 地震。
	//
	// **土地價值每年自己掉**（`0x15c96`，`L0`）：`−RND(土地價值 ÷ 10)`，逐郡跑，
	// 無主的郡也跑；`÷ 10` 前先 `cbw`，蝗害推到 128 以上的值是負的，
	// `RND(負)` 不抽、不掉。這是「土地開發要一直做」的原因——
	// 少了它，一次開發到頂就永遠不用再管。
	for i := range g.prefectures {
		p := &g.prefectures[i]
		p.LandValue = uint8(LandValueDecay(int(p.LandValue),
			g.Roll(int(int8(p.LandValue))/10, int(Spring), p.ID, 8)))
	}
	// **玉璽現世**（`0x15cfd`–`0x1519a`，`L0`）：玉璽還沒出現的話，
	// 每年約半數機率落到隨機一個活著的勢力手上，那一家的人望大漲。
	out = append(out, g.sealEvent()...)
	// **原版分兩圈走**（`L0`）：第一圈 `0x15d40`–`0x15eb3` 是老死，
	// 第二圈 `0x15f64`–`0x16042` 才加歲、忠誠漂移、訓練與武裝衰減、
	// 出頭。**老死比的是加歲之前的年齡**——先加再比的話每個人
	// 都早一年走下坡（Issue #23）。
	for i := range g.generals {
		x := &g.generals[i]
		// 第一圈只跳過已故（`0x15d55`），未登場、填充槽也掃、也擲。
		if x.Status == state.StatusFallen {
			continue
		}
		// **年齡與壽命都是有號的**（`0x15d71`／`0x15d79` 的 `cbw`）：還沒出生
		// 的人年齡是負的（曹叡在 189 年是 −16，位元組 `0xF0`），負的年齡
		// 自然過不了壽命那一道。當成無號讀的話那些人 240 歲、當年就老死。
		age, lifespan := x.SignedAge(), int(int8(x.Lifespan))
		// **老死看壽命，不是每年掉一點**（`0x15d5d`，`L0`）：
		// 沒過壽命的人體能一點都不掉。
		if !AlreadyPastPrime(age, lifespan, g.Roll(LifespanRollSpread, int(Spring), x.Index, 43)) {
			continue
		}
		n := AgingDrop(int(x.Stamina), age, lifespan, g.Roll(DeathStaminaSpread, int(Spring), x.Index, 44))
		x.Stamina = uint8(n)
		if n != 0 {
			continue
		}
		// 在職的（身分 1–3）先印一句、播一次特效（`0x15e8e` → `0x32dfa`，
		// `RND(4)`）；之後的處置常式 `0x14792` 不分身分都印一則對白
		// （`0x14848`／`0x14a38` → `0x3273e`，`RND(8)`），槽 96（周瑜）再多
		// 一則（`0x14893`）。
		if x.Status == state.StatusChief || x.Status == state.StatusGovernor ||
			x.Status == state.StatusOfficer {
			g.Roll(EffectVariants, int(Spring), x.Index, 47)
		}
		out = append(out, Event{Prefecture: x.Location, Text: tf("ev.death", personName(x.Name))})
		if x.Status != state.StatusLord {
			// 君主那一則在繼承常式裡（`SucceedLord`）。下格、肖像在右
			// （`0x14848`：(424,180)–(615,275)，side 0）。
			out = append(out, g.bubbleEvent(x, false, false,
				tf("bub.death", personName(x.Name)), int(Spring), x.Index, 48))
		}
		g.retire(x)
		out = append(out, g.PendingEvents()...)
		if x.Index == DeathEpilogueSlot {
			out = append(out, g.bubbleEvent(x, false, false,
				tf("bub.epilogue", personName(x.Name)), int(Spring), x.Index, 49))
		}
	}
	for i := range g.generals {
		x := &g.generals[i]
		// 全部 350 筆都加歲（`0x15f7a`），已故與未登場也加。
		x.Age++
		// **只有在職的部下（身分 1–3）才有下面三項**（`0x15f7f`–`0x15fa2`）：
		// 君主不會對自己不忠，他的訓練度與武裝度也不衰減；在野與未登場
		// 的人凍在原值。
		if x.Faction != state.NoFaction &&
			(x.Status == state.StatusChief || x.Status == state.StatusGovernor ||
				x.Status == state.StatusOfficer) {
			// **忠誠每年跟著君主的人望漂移**（`0x15fc2`，`L0`）：
			// `忠誠 += (人望 − 60) ÷ 2`；算出負的才擲 `RND(5)`（`0x15fd8`）。
			prestige := 0
			if f := g.Faction(x.Faction); f != nil {
				prestige = f.Prestige
			}
			// 與 `LoyaltyDrift` 同一條式，只是那一擲要在算出負的之後才抽。
			n := int(int8(x.Loyalty)) + (prestige-LoyaltyPivot)/2
			if n < 0 {
				n = g.Roll(LoyaltyFloorSpread, int(Spring), x.Index, 42)
			} else if n > 100 {
				n = 100
			}
			// **加強版多一道門**（`0x14f8d`–`0x14fb9`，`L0`、`[plus]`）：
			// 忠誠**原本**就不低於 `(難度 − 1) mod 10 + 90` 的人不寫回——
			// 難度 5 是 94 以上不動。那一擲 `RND(5)` 在門之前，照抽。
			// 原版沒有這道門（`0x15fad`–`0x16001` 直接寫回）。
			if g.Edition != state.EditionPlus ||
				int(int8(x.Loyalty)) < PlusLoyaltyDriftCeiling(g.Difficulty) {
				x.Loyalty = uint8(n)
			}
			// **武裝度與訓練度每年各自掉**（`0x16006`／`0x16025`，`L0`）：
			// `−RND(值 ÷ 10)`，與土地價值同一個形狀，先武裝後訓練；
			// 值不到 10 的不抽。
			//
			// 這是「訓練兵士」與「購置武器」要一直做的原因——少了它，
			// 一次練到頂就永遠是精兵，而數值欄停在 100 看起來完全正常。
			x.Arms = uint8(AnnualDecay(int(x.Arms),
				g.Roll(int(int8(x.Arms))/10, int(Spring), x.Index, 40)))
			x.Training = uint8(AnnualDecay(int(x.Training),
				g.Roll(int(int8(x.Training))/10, int(Spring), x.Index, 41)))
		}
		out = append(out, g.debut(x)...)
	}
	// **地震：每年 1/3 的機率，落在隨機一個郡**（`0x162d2`，`L0`），
	// **不看所屬**（無主郡也震）、不看民眾忠誠也不看土地價值——那道
	// 「天災多因人怨」的門檻是瘟疫的。
	if g.Roll(QuakeChance, int(Spring), 0, 10) == 0 {
		id := g.Roll(state.PrefectureCount, int(Spring), 0, 11) + 1
		if p := g.Prefecture(id); p != nil {
			// 印字、特效一次（`0x1633b` → `0x32dfa`，`RND(4)`）。
			g.Roll(EffectVariants, int(Spring), id, 9)
			// 四份保留率各擲一次，**不是同一個百分比套四次**：人口、金、米
			// （`0x1638b`／`0x163d5`／`0x1640e`），然後**郡裡每一位人物的兵**
			// （`0x16444`–`0x1649c`：人物表順序、所在郡等於那個郡的，各擲一次
			// `RND(20)+40`%），最後重整守將清單（`0x164a2` → `0x1949e`）。
			p.Population = keepPopulation(p.Population, QuakePopKeep,
				g.Roll(QuakePopKeep.Spread, int(Spring), id, 20))
			if p.Population < QuakePopFloor {
				p.Population = QuakePopFloor
			}
			p.Gold = QuakeGoldKeep.Apply(p.Gold,
				g.Roll(QuakeGoldKeep.Spread, int(Spring), id, 21))
			p.Rice = QuakeRiceKeep.Apply(p.Rice,
				g.Roll(QuakeRiceKeep.Spread, int(Spring), id, 22))
			for j := range g.generals {
				x := &g.generals[j]
				if x.Location != id {
					continue
				}
				x.Soldiers = QuakeSoldiersKeep.Apply(x.Soldiers,
					g.Roll(QuakeSoldiersKeep.Spread, int(Spring), x.Index, 23))
			}
			g.RefreshGarrison(id)
			out = append(out, Event{Prefecture: p.ID, Text: tf("ev.quake", placeName(p.Name))})
		}
	}
	return out
}

// DeathEpilogueSlot 是死了會多印一則對白的那一位（`0x14893`：人物槽 96，
// 劇本 001 是周瑜，`L0`）。
const DeathEpilogueSlot = 96

// MessageLines 是對白常式 `0x3273e` 每次進去挑句子的那一擲 `RND(8)`
// （`0x32d4d`，`L0`；戰場那一份是 `battle.MessageLines`）。
const MessageLines = 8

// EffectVariants 是特效常式 `0x32dfa` 進去先擲的 `RND(4)`（`0x32e4f`，`L0`）。
const EffectVariants = 4

// 人望決定忠誠漲跌的分水嶺與下限（`0x15fc7`／`0x15fd8`，`L0`）。
const (
	LoyaltyPivot       = 60 // 人望高於它部下向心，低於它離心
	LoyaltyFloorSpread = 5  // 算出負的就換成 RND(5)
)

// PlusLoyaltyDriftCeiling 是加強版元月忠誠漂移的門（`0x14f8d`–`0x14fb9`，
// `L0`、`[plus]`）：`(難度 − 1) mod 10 + 90`，忠誠不低於它的人不動。
// 與加強版賞賜金帛的門同一個 `(難度 − 1) mod 10` 形狀。
func PlusLoyaltyDriftCeiling(difficulty int) int { return (difficulty-1)%10 + 90 }

// LoyaltyDrift 是元月的忠誠漂移（`0x15fad`–`0x16001`，`L0`）：
//
//	忠誠 ← 忠誠 + (人望 − 60) ÷ 2
//	< 0   → RND(5)
//	> 100 → 100
//
// **60 是分水嶺**：人望不到 60 的諸侯，部下每年都在離心。
// 人望靠打勝仗累積（`PrestigeOnWin`）。
func LoyaltyDrift(loyalty, prestige, floorRoll int) int {
	n := loyalty + (prestige-LoyaltyPivot)/2
	if n < 0 {
		return floorRoll
	}
	if n > 100 {
		return 100
	}
	return n
}

// AnnualDecay 是「值越高掉越多」的年度衰減（`L0`）：
//
//	新值 ＝ 值 − RND(值 ÷ 10)
//
// 土地價值（每個春月，`0x15c96`）、將領的訓練度與武裝度
// （元月，`0x16006`／`0x16025`）三處都是這個形狀。
//
// **收斂點在「補的速度 ＝ 掉的速度」**，不是上限——所以內政、訓練、
// 購置武器都是要一直做的事。
func AnnualDecay(value, roll int) int {
	v := value - roll
	if v < 0 {
		v = 0
	}
	return v
}

// QuakeChance 是地震的機率分母（`0x162d2`：`RND(3) == 0`，`L0`）。
const QuakeChance = 3

// 四種天災的損失（`L0`、`[base]`）。每一項都是同一個形狀：
//
//	新值 ＝ 舊值 × (RND(幅度) + 底) ÷ 100
//
// 也就是**保留率**，不是損失率。底越小掉得越多。
var (
	// 地震（`0x1638b`／`0x163d5`／`0x1640e`）：人口、金、米各一份。
	QuakePopKeep  = Keep{20, 60} // 60–79%
	QuakeGoldKeep = Keep{20, 50} // 50–69%
	QuakeRiceKeep = Keep{20, 40} // 40–59%
	// 地震之後**郡裡每一位人物的兵**（`0x16444`–`0x1649c`，人物表順序、
	// 所在郡等於那個郡的，各擲一次）。
	QuakeSoldiersKeep = Keep{20, 40} // 兵 40–59%
	// 水災（`0x1666d`／`0x166c1`／`0x16701`；人物那一段 `0x16748`–`0x167a5`，
	// `[both]`）：人口、土地價值、洪水率各一份，郡裡每一位人物的兵一份。
	FloodPopKeep      = Keep{10, 70}  // 70–79%
	FloodLandKeep     = Keep{20, 60}  // 60–79%
	FloodRateGain     = Keep{20, 120} // 洪水率 ×1.20–1.39，越界變 100
	FloodSoldiersKeep = Keep{10, 70}  // 兵 70–79%
	// 瘟疫（`0x168ce`；人物那一段 `0x1690d`–`0x169aa`，加強版 `0x157a8`–
	// `0x15878`，`[both]`）：人口一份，郡裡**每一位人物**（人物表順序、
	// 所在郡等於那個郡的，不分在職在野）兵一份、體能一份——手冊 p.36
	// 「將領體能下降」的出處。
	PlaguePopKeep      = Keep{20, 40} // 40–59%
	PlagueSoldiersKeep = Keep{20, 40} // 兵 40–59%
	PlagueStaminaKeep  = Keep{10, 70} // 體能 70–79%
	// 蝗害（`0x16cba`／`0x16cfa`）。
	LocustRiceKeep = Keep{10, 20} // 20–29%，米幾乎被吃光
	LocustLandKeep = Keep{90, 80} // 80–169% ⚠ 見下
)

// Keep 是「保留率」的兩個參數：`RND(Spread) + Floor`，單位是百分比。
type Keep struct {
	Spread int
	Floor  int
}

// Apply 把保留率套上去。roll 是 `RND(Spread)`。
func (k Keep) Apply(value, roll int) int { return value * (roll + k.Floor) / 100 }

// QuakePopFloor 是地震（`0x163bb`）與瘟疫（`0x16901`）之後人口的下限：
// 低於 50 就補到 50（`L0`）。存的值是實際值 ÷ 100，所以下限是五千人。
const QuakePopFloor = 50 * 100

// keepPopulation 把保留率套在**存的單位**（實際值 ÷ 100）上：原版
// `filds 人口` 讀的是那個字，乘完 `ftol` 截尾再存回去，零頭在那一刻就
// 沒了——remake 的人口是實際值，直接乘會留下零頭，下一次人口成長或
// 徵兵讀到的就不是原版存的那個數（`GrowPopulation` 那一段量到過）。
func keepPopulation(population int, k Keep, roll int) int {
	return k.Apply(population/100, roll) * 100
}

// LandValueDecay 是土地價值每個春月的自然衰減（`0x15c96`，`L0`）。
//
// **衰減與郡有沒有主無關。** 形狀與訓練度／武裝度相同（`AnnualDecay`）。
func LandValueDecay(landValue, roll int) int { return AnnualDecay(landValue, roll) }

// comeOfAge 是春天的「新血出現」（說明書 p.36）。
//
// 未登場的人物（身分 11）年齡到了就在**出身郡**露面，成為在野將領。
// 出身郡是 `BASEGEN` offset 13：劇本 001 的劉備是涿郡、孫堅是吳郡，
// 與史實相符。
//
// ⚠ **沒有這一段的話，武將只死不生。** 實測：不補新血的話，四十七年後
// 十四個勢力全部滅亡，天下無主——那不是「難度高」，是少了一條規則。
func (g *State) comeOfAge() []Event {
	var out []Event
	for i := range g.generals {
		out = append(out, g.debut(&g.generals[i])...)
	}
	return out
}

// debut 是第二圈每一筆最後的出頭判定（`0x16042`–`0x160a2`，加歲之後）。
func (g *State) debut(x *General) []Event {
	if x.Status != state.StatusUnborn {
		return nil
	}
	// **出頭年齡是每個人自己的**（人物表 offset 26，`0x1605a`）——
	// 不是一個全域常數。原版比的是「年齡 > Debut」，不是 >=，而且
	// 年齡是**有號**的（`cbw`）：還沒出生的人是負的，要等到真的長到
	// 出頭年齡才出現。當成無號讀的話 −16 歲的人 240 歲，開局第二年
	// 全部出頭、隔年全部老死（Issue #23）。
	if x.SignedAge() <= int(x.Debut) {
		return nil
	}
	// **先看牽絆對象**（`0x16064`）：他有勢力、而且他所在的郡還沒
	// 滿五十位現役武將的話，這個人直接投奔他，不是回出身郡當在野。
	//
	// 少了這一條，名將的子姪與舊部都會變成散落各地的在野人士，
	// 而「牽絆」在登用之外就沒有別的作用了。
	if at, id, ok := g.bondDebut(x); ok {
		b := &g.generals[x.Bond]
		// 印字、特效一次（`0x16152` → `0x32dfa`，`RND(4)`）；牽絆對象不是
		// 君主再一則對白（`0x161c4` → `0x3273e`，`RND(8)`），然後一定一則
		// （`0x16270`）。忠誠寫成牽絆對象的九成（`0x1629e`–`0x162b4`）。
		g.Roll(EffectVariants, int(Spring), x.Index, 47)
		var bubbles []Event
		if b.Status != state.StatusLord {
			// 上格、肖像在右，說話的是牽絆對象（`0x161c4`）。
			bubbles = append(bubbles, g.bubbleEvent(b, true, false,
				tf("bub.debutBond", personName(x.Name)), int(Spring), x.Index, 48))
		}
		// 下格、肖像在左，說話的是新人（`0x16270`）。
		bubbles = append(bubbles, g.bubbleEvent(x, false, true,
			tf("bub.debut", personName(x.Name)), int(Spring), x.Index, 49))
		x.Location = at
		x.Faction = id
		x.Status = state.StatusOfficer
		x.Loyalty = uint8(int(int8(b.Loyalty)) * 9 / 10)
		// 那一郡存的現役將 +1（`0x162ca`），兵士不動。
		if p := g.Prefecture(at); p != nil {
			p.activeGenerals++
		}
		return append([]Event{{Prefecture: at,
			Text: tf("ev.appear", personName(x.Name), placeName(g.Prefecture(at).Name))}},
			bubbles...)
	}
	at := int(x.Origin)
	if at < 1 || at > len(g.prefectures) {
		at = x.Location
	}
	if at < 1 || at > len(g.prefectures) {
		return nil
	}
	// 退路是**出身郡**、無勢力，身分先寫 **9**（`0x15eda`：在野但不列入
	// 郡的在野數，要被尋訪到才變成 8），接著看露不露面（`0x15ee6`–
	// `0x15f5c`）：**年紀到了（≥ RND(5)+32）直接露面；年輕的才看才能——
	// 謀略與戰力取大者，`RND(30)+30` 壓不過它就藏著。** 兩道門依序擲，
	// 第一道過了就不擲第二道。年輕的名將要被尋訪才出現，庸才一出頭就
	// 在名單上。
	x.Location = at
	x.Faction = state.NoFaction
	x.Status = state.StatusIdle
	show := x.SignedAge() >= g.Roll(DebutShowAgeSpread, int(Spring), x.Index, 45)+DebutShowAge
	if !show {
		show = g.Roll(DebutHideTalentSpread, int(Spring), x.Index, 46)+DebutHideTalent >
			max(int(x.Intel), int(x.War))
	}
	if show {
		x.Status = state.StatusAvailable
	}
	p := g.Prefecture(at)
	name := ""
	if p != nil {
		name = p.Name
	}
	return []Event{{Prefecture: at, Text: tf("ev.appear", personName(x.Name), placeName(name))}}
}

// 玉璽現世的兩個數（`0x15d11`／`0x1518b`，`L0`）。
const (
	SealChanceBar      = 50 // RND(100) > 50 才出現
	SealPrestigeSpread = 30 // 人望 += RND(30) + 40
	SealPrestigeFloor  = 40
)

// SealFound 回報玉璽已經現世了沒有。
//
// 原版用一個全域旗標（`es:0x2f6c`，`0xFFFF` ＝ 還沒現世）；remake 從
// 各家的寶庫推導，**因為滅亡勢力的寶物會被勝方接收**（`seizeTreasures`），
// 玉璽不會憑空消失。少一個要存進存檔的欄位。
func (g *State) SealFound() bool {
	for i := range g.factions {
		if g.factions[i].Treasury[TreasureSeal] > 0 {
			return true
		}
	}
	return false
}

// sealEvent 是春季的玉璽現世（`0x15cfd`–`0x1519a`，`L0`）。
//
// **玉璽在 remake 裡原本只被檢查、從來不會被發出來**——
// 勝利條件那一條因此永遠走不到，而畫面上完全看不出來。
func (g *State) sealEvent() []Event {
	if g.SealFound() {
		return nil
	}
	if g.roll(int(Spring), 0, 50)%100 <= SealChanceBar {
		return nil
	}
	var alive []*Faction
	for i := range g.factions {
		if g.factions[i].Alive {
			alive = append(alive, &g.factions[i])
		}
	}
	if len(alive) == 0 {
		return nil
	}
	f := alive[g.roll(int(Spring), 0, 51)%len(alive)]
	f.Treasury[TreasureSeal] = 1
	f.Prestige = clampTo(f.Prestige+
		g.roll(int(Spring), int(f.ID), 52)%SealPrestigeSpread+SealPrestigeFloor, 100)
	lord := g.Lord(f.ID)
	name := tf("fld.factionN", f.ID)
	if lord != nil {
		name = personName(lord.Name)
	}
	return []Event{{Prefecture: 0, Text: tf("ev.seal", name)}}
}

// DebutGarrisonCap 是「牽絆對象的郡收不收得下」的門檻
// （`0x16097`：現役武將數 < 50，`L0`）。與每郡五十位將軍的上限同一個數。
const DebutGarrisonCap = 50

// 出頭之後露不露面的兩道門（`0x15efe`／`0x15f29`，`L0`、`[base]`）。
const (
	// DebutShowAge 是「年紀到了就露面」的門檻：年齡 ≥ RND(5) + 32。
	DebutShowAge       = 32
	DebutShowAgeSpread = 5
	// DebutHideTalent 是年輕人的才能門：max(謀略, 戰力) ≥ RND(30) + 30 就藏著。
	DebutHideTalent       = 30
	DebutHideTalentSpread = 30
)

// DebutShowsUp 回報沒有牽絆對象可投奔的新人出頭時是直接露面（身分 8）
// 還是藏在郡裡等尋訪（身分 9）。
func DebutShowsUp(age, intel, war, ageRoll, talentRoll int) bool {
	if age >= ageRoll+DebutShowAge {
		return true
	}
	return talentRoll+DebutHideTalent > max(intel, war)
}

// bondDebut 回報未登場者要不要直接投奔牽絆對象，以及去哪個郡、投哪一家。
//
// 三道閘門（`0x16064`–`0x1609d`）：牽絆對象不是自己、他有勢力、
// 他所在的郡現役武將數 < 50。
func (g *State) bondDebut(x *General) (int, state.FactionID, bool) {
	if x.Bond == x.Index || x.Bond < 0 || x.Bond >= len(g.generals) {
		return 0, state.NoFaction, false
	}
	b := &g.generals[x.Bond]
	if b.Faction == state.NoFaction {
		return 0, state.NoFaction, false
	}
	at := b.Location
	p := g.Prefecture(at)
	if p == nil {
		return 0, state.NoFaction, false
	}
	// 比的是州郡**存的**現役將（`0x16097` 讀 offset 22），不是重算的。
	if g.StoredActiveGenerals(at) >= DebutGarrisonCap {
		return 0, state.NoFaction, false
	}
	return at, b.Faction, true
}

// summer 是夏天：洪水與瘟疫（`0x164c8`–`0x167df`，加強版 `0x15423`–，
// `L0`、`[both]`；對拍 `SAN1_MONTH=3 SAN1_PLAGUE=1 TestZZMonthParity`）。
//
// 水災分兩段。第一段**逐郡 1..42、不看所屬**：洪水率先自己長、再擲兩道
// 門，淹的郡記進清單；第二段照清單的順序演——人口、土地價值、洪水率、
// **郡裡每一位人物的兵**各一擲，再重整守將清單。每一道 `RND(n)` 在
// `n <= 0` 時不抽（`0x10b0c`）。
func (g *State) summer() []Event {
	var out []Event
	var flooded []int
	for id := 1; id <= state.PrefectureCount; id++ {
		p := g.Prefecture(id)
		if p == nil {
			continue
		}
		// 洪水率每年都會自己長（`0x16500`–`0x16552`）：
		// `min(100, RND(舊 ÷ 4) + 舊 + 基礎[郡])`，與淹不淹無關。
		rate := int(int8(p.FloodRate))
		rise := 0
		if q := rate / 4; q > 0 {
			rise = g.roll(int(Summer), id, 3) % q
		}
		p.FloodRate = uint8(FloodRise(rate, FloodBase(id), rise))
		// 淹不淹（`0x16559`–`0x16585`）：先 `RND(新洪水率)`、再 `RND(65)`，
		// `RND(65) + 5 >= RND(新洪水率)` 就不淹；過了才擲 `RND(100)`，
		// `<= 80` 也不淹。
		rateRoll := 0
		if n := int(p.FloodRate); n > 0 {
			rateRoll = g.roll(int(Summer), id, 4) % n
		}
		guard := g.roll(int(Summer), id, 1) % FloodRollSpread
		if guard+FloodRollFloor >= rateRoll {
			continue
		}
		if g.roll(int(Summer), id, 2)%FloodGateSpread <= FloodGateBar {
			continue
		}
		flooded = append(flooded, id)
	}
	if len(flooded) > 0 {
		// 有郡淹了才印字、放一次特效（`0x165f6` 叫 `0x32dfa`，進去就擲
		// `RND(4)` 挑動畫，`docs/re/05` §12.2）——整個夏天一次，不是每郡一次。
		g.roll(int(Summer), 0, 8)
	}
	for _, id := range flooded {
		p := g.Prefecture(id)
		// 效果（`0x1663a`–`0x167ae`）：人口 70–79%（低於五千補到五千）、
		// 土地價值 60–79%、洪水率 ×1.20–1.39（算出來超過 100 或變成負的
		// 就是 100——說明書 p.21 說「水災後洪水率立刻升到 100」，那是高
		// 洪水率的郡才成立）、郡裡每一位人物（人物表順序、所在郡等於
		// 那個郡，不看在不在職）兵 70–79%；然後重整守將清單（`0x1949e`）。
		p.Population = keepPopulation(p.Population, FloodPopKeep,
			g.roll(int(Summer), id, 20)%FloodPopKeep.Spread)
		if p.Population < QuakePopFloor {
			p.Population = QuakePopFloor
		}
		p.LandValue = uint8(FloodLandKeep.Apply(int(p.LandValue),
			g.roll(int(Summer), id, 21)%FloodLandKeep.Spread))
		rate := FloodRateGain.Apply(int(int8(p.FloodRate)),
			g.roll(int(Summer), id, 22)%FloodRateGain.Spread)
		if rate > 100 || rate < 0 {
			rate = 100
		}
		p.FloodRate = uint8(rate)
		for i := range g.generals {
			x := &g.generals[i]
			if x.Location != id {
				continue
			}
			x.Soldiers = FloodSoldiersKeep.Apply(x.Soldiers, g.roll(int(Summer), id, 30+i)%FloodSoldiersKeep.Spread)
		}
		g.RefreshGarrison(id)
		out = append(out, Event{Prefecture: p.ID, Text: tf("ev.flood", placeName(p.Name))})
	}
	// **瘟疫每年只挑一個郡**（`RND(42) + 1`，`0x167df`），不逐郡掃、也
	// **不看所屬**。兩道門一道一道擲：忠誠那一道沒過就不擲土地那一道
	// （`0x16814` 直接跳到結尾）。
	{
		id := g.roll(int(Summer), 0, 5)%state.PrefectureCount + 1
		p := g.Prefecture(id)
		loyaltyRoll := g.roll(int(Summer), id, 6) % PlagueLoyaltySpread
		if p != nil && int(p.PublicLoyalty) < loyaltyRoll+PlagueLoyaltyFloor &&
			PlagueStrikes(int(p.PublicLoyalty), int(p.LandValue), loyaltyRoll,
				g.roll(int(Summer), id, 7)%PlagueLandSpread) {
			g.plague(p)
			out = append(out, Event{Prefecture: p.ID, Text: tf("ev.plague", placeName(p.Name))})
		}
	}
	return out
}

// plague 是瘟疫落在一個郡上的損失（`0x168c4`–`0x169b4`，加強版
// `0x157a8`–`0x1587f`，`L0`、`[both]`）：人口一擲（`RND(20)+40`%，低於 50
// 補到 50），然後**郡裡每一位人物**——照人物表的順序、所在郡等於那個郡
// 的，不看在不在職——各擲兩次：兵 `RND(20)+40`%、體能 `RND(10)+70`%
// （都截尾）。最後重整那個郡的守將清單（`0x1949e`），存的兵士跟著變成
// Σ兵力 ÷ 100。手冊 p.36 的「將領體能下降」出處在這裡。
func (g *State) plague(p *Prefecture) {
	id := p.ID
	// 印字、特效一次（`0x16871` 叫 `0x32dfa`，`RND(4)` 挑動畫）。
	g.roll(int(Summer), id, 8)
	p.Population = keepPopulation(p.Population, PlaguePopKeep,
		g.roll(int(Summer), id, 20)%PlaguePopKeep.Spread)
	if p.Population < QuakePopFloor {
		p.Population = QuakePopFloor
	}
	for i := range g.generals {
		x := &g.generals[i]
		if x.Location != id {
			continue
		}
		x.Soldiers = PlagueSoldiersKeep.Apply(x.Soldiers, g.roll(int(Summer), id, 30+i)%PlagueSoldiersKeep.Spread)
		x.Stamina = uint8(PlagueStaminaKeep.Apply(int(x.Stamina), g.roll(int(Summer), id, 400+i)%PlagueStaminaKeep.Spread))
	}
	g.RefreshGarrison(id)
}

// 年度事件落在哪一個月。
//
// **agingMonth 與 growthMonth 是量出來的。** 原版連走十六個月的觀測裡，
// 全部 350 筆的年齡只在元月動；二三十個郡的人口只在十月一起動，
// 兩者都十二個月後再來一次，其他月份沒有（`docs/mechanics/70-ai` §2.2）。
//
// **四個月份現在都是量到的**（`L0`、`0x15c48`）。四季常式一年各跑一次，
// 分派器比的是月份：1 春、4 夏、7 秋、10 冬。秋收在秋季常式裡、
// 進貢與人口成長在冬季常式裡，所以它們的月份不是 remake 挑得動的——
// 原本挑的 9 月與 12 月落在四支常式都不跑的月份上。
// 物價的範圍。**量出來的**：十六個月 × 四十二個郡，值全部落在 30–68。
const (
	PriceMin    = 30
	PriceSpread = 19 // 兩個 0..19 相加 → 30..68
)

// priceSalt 讓物價的亂數與其他事件的亂數分開。
// 共用鹽的話「物價高的郡也比較容易鬧災」會憑空成立。
const priceSalt = 0x9E37

const (
	agingMonth   = 1  // 春季常式
	harvestMonth = 7  // 秋季常式
	tributeMonth = 10 // 冬季常式
	growthMonth  = 10 // 冬季常式，與進貢同一支
)

// repriceAll 每個月替每一個郡重抽物價（`0x17396`–`0x173ea`，`L0`）。
//
// 它在**開月**常式 `0x17364` 的迴圈裡，和「旗標全開、順序表填成 0..42」
// 同一趟走完 43 筆：
//
//	cx  = RND(20)                                 ; 0x17396
//	基  = 民眾忠誠 − 洪水率÷2 + 土地價值          ; 0x173a0–0x173cd
//	物價 = RND(基) ÷ 10 + cx + 30                 ; 0x173d2–0x173ea
//
// 兩個加數都落在 0–19，所以範圍是 30–68，與連走十六個月量到的相符；
// 分布在中間隆起也是兩個近似均勻量相加的形狀。**郡的狀態確實是輸入**
// ——民眾忠誠與土地價值抬高上限、洪水率壓低它——只是除以 10 之後
// 影響被壓到和 `RND(20)` 同一個量級，掃描單一欄位看不出單調性。
//
// 每郡兩次亂數，43 筆共 86 次，全部在四季常式之前。
func (g *State) repriceAll() {
	// **啞元那一筆也要重抽。** 原版的州郡表有 43 筆，第 0 筆是啞元
	// （`docs/formats/03`），而重抽物價的迴圈走完整張表——月度對拍的
	// 差異清單裡「郡 0：物價」一直都在。remake 的 `prefectures` 只有
	// 42 筆，少抽兩次，而**那是整條亂數序列第一個岔開的地方**。
	price := func(loyalty, land, flood, salt int) uint8 {
		a := g.Roll(PriceSpread+1, int(priceSalt), salt)
		// `RND(n)` 在 n <= 0 時直接回 0 **而且不抽**（`0x10b0c`），
		// `Roll` 同樣的行為，所以荒地那幾筆不會位移亂數序列。
		b := g.Roll(loyalty-flood/2+land, int(priceSalt), salt, 1) / 10
		return uint8(PriceMin + a + b)
	}
	if len(g.rawSta) > staPrice {
		g.rawSta[staPrice] = price(int(g.rawSta[staLoyalty]),
			int(g.rawSta[staLandValue]), int(g.rawSta[staFloodRate]), 0)
	}
	for i := range g.prefectures {
		p := &g.prefectures[i]
		p.PriceLevel = price(int(p.PublicLoyalty), int(p.LandValue),
			int(p.FloodRate), p.ID)
	}
}

// autumn 是秋天：收成與蝗害。收成一年一次。
func (g *State) autumn() []Event {
	var out []Event
	if g.Date.Month != harvestMonth {
		return nil
	}
	for i := range g.prefectures {
		p := &g.prefectures[i]
		if !p.Owned() {
			continue
		}
		// 秋收（`0x16a1b`–`0x16ac6`，`L0`）。
		charm := 0
		if x := g.Governor(p.ID); x != nil {
			charm = int(x.Charm)
		}
		gold := HarvestGold(charm, int(p.LandValue), int(p.PublicLoyalty),
			p.Population)
		rice := HarvestRice(charm, int(p.LandValue), int(p.FloodRate),
			int(p.PublicLoyalty), p.Population)
		// **收成之後是 0 就改抽亂數**（`0x16a87`／`0x16b60`，`L0`）：
		// 金抽 `RND(土地價值 + 50)`、米抽 `RND((土地價值 + 50) × 2)`，
		// 而且是**取代**不是相加。地力與忠誠都被打到 0 的郡才走得到
		// ——那是一個保底，不然那個郡永遠翻不了身。
		gotGold, gotRice := p.Gold+gold, p.Rice+rice
		if gotGold <= 0 {
			gotGold = g.Roll(int(p.LandValue)+HarvestFloorBase, p.ID, 30)
		}
		if gotRice <= 0 {
			gotRice = g.Roll((int(p.LandValue)+HarvestFloorBase)*2, p.ID, 31)
		}
		p.Gold = clampTo(gotGold, HarvestGoldCap)
		p.Rice = clampTo(gotRice, MaxRice)
		out = append(out, Event{Prefecture: p.ID,
			Text: tf("ev.harvest", placeName(p.Name), rice, gold)})
	}
	// **蝗害每年只挑一個郡**（`0x16bd5`，`L0`），不逐郡掃，**也不看所屬**
	// （`0x16bf6` 直接讀那個郡的民眾忠誠，沒有 `0x49e` 的比較；無主郡照樣
	// 鬧）。兩道門**依序擲**：忠誠那一道沒過就不擲土地那一道
	// （`0x16c00` 直接跳到人望調整）。發生時先播一次特效（`0x16c5b` →
	// `0x32dfa`，`RND(4)`），再擲米與土地的保留率。
	{
		id := g.roll(int(Autumn), 0, 12)%state.PrefectureCount + 1
		p := g.Prefecture(id)
		loyaltyRoll := g.roll(int(Autumn), id, 13) % LocustLoyaltySpread
		if p != nil && int(p.PublicLoyalty) < loyaltyRoll+LocustLoyaltyFloor &&
			LocustStrikes(int(p.PublicLoyalty), int(p.LandValue), loyaltyRoll,
				g.roll(int(Autumn), id, 14)%LocustLandSpread) {
			g.roll(int(Autumn), id, 8) // 特效 `RND(4)`
			p.Rice = LocustRiceKeep.Apply(p.Rice,
				g.roll(int(Autumn), id, 20)%LocustRiceKeep.Spread)
			// ⚠ **蝗害的土地價值保留率是 80–169%**（`0x16cfa`：`RND(90) + 80`），
			// 也就是平均會**上升**。碼就是這樣寫的——以碼為準，而且**不夾**：
			// `0x16d26` 把 `ftol` 的低位元組直接寫回（100 × 1.69 ＝ 169 裝得下），
			// 之後秋收、瘟疫讀到的就是那個超過 100 的值。記在 `docs/re/06` §6。
			p.LandValue = uint8(LocustLandKeep.Apply(int(p.LandValue),
				g.roll(int(Autumn), id, 21)%LocustLandKeep.Spread))
			// 之後重整那個郡的守將清單（`0x16d2e` → `0x1949e`），與水災、瘟疫同。
			g.RefreshGarrison(id)
			out = append(out, Event{Prefecture: p.ID, Text: tf("ev.locust", placeName(p.Name))})
		}
	}
	// **人望的年度調整**（`0x16d4f`–`0x16e6a`，`L0`）：蝗害之後、
	// 不管有沒有鬧蝗都跑（「不發生」的兩條分支直接跳到 `0x16d4f`）。
	// 這是人望除了戰役 ±2、春季玉璽、繼承折損之外**唯一的變動來源**，
	// 而且每年都動——它讓人望朝「君主魅力 ＋ 領地平均狀態」對 100 的差距漂，
	// 漏掉它的話人望永遠卡在開局值，部下忠誠年年照 `(人望 − 60) ÷ 2` 離心。
	g.adjustPrestige()
	return out
}

// PrestigeDriftDivisor 是年度人望調整的除數（`0x16e25`：`sar ax, 5`，
// 對稱取整，`L0`）。差距最大 ±100，所以一年最多動 ±3。
const PrestigeDriftDivisor = 32

// PrestigeDrift 是一個勢力這一年人望要動多少（`0x16dee`–`0x16e2c`，`L0`）：
//
//	d = 君主魅力 + (Σ(民眾忠誠 ÷ 2 + 土地價值 ÷ 2)) ÷ 領地數 − 100
//	d = d ÷ 32（向零取整）
//
// 民眾忠誠與土地價值各先 ÷ 2 再加（`0x16da6`–`0x16dbc`，`idiv cl`），
// 不是加完再除；領地數為 0 的勢力不調（`0x16de4`）。
func PrestigeDrift(lordCharm, loyaltyLandSum, land int) int {
	if land <= 0 {
		return 0
	}
	d := lordCharm + loyaltyLandSum/land - 100
	// `cwd / xor / sub / sar / xor / sub` 是 MSC 的「帶號除以 2^n 向零取整」。
	if d < 0 {
		return -((-d) / PrestigeDriftDivisor)
	}
	return d / PrestigeDriftDivisor
}

// adjustPrestige 每年秋季逐勢力調整人望（`0x16d4f`–`0x16e6a`，`L0`）。
//
// 兩張 16 格的表（領地數、和）與冬季進貢用的是同一對緩衝區
// （`es:0x24b2`／`es:0x2e36`，`docs/re/06` §9），但係數不同：
// 這裡是 ÷2、÷2，進貢是 ÷4、÷2。逐郡掃的是 1..42，所屬 `0xFF` 跳過。
func (g *State) adjustPrestige() {
	land := make([]int, len(g.factions)+16)
	sum := make([]int, len(land))
	for i := range g.prefectures {
		p := &g.prefectures[i]
		if !p.Owned() || int(p.Owner) >= len(land) {
			continue
		}
		land[p.Owner]++
		sum[p.Owner] += int(p.PublicLoyalty)/2 + int(p.LandValue)/2
	}
	for i := range g.factions {
		f := &g.factions[i]
		if int(f.ID) >= len(land) || land[f.ID] <= 0 {
			continue
		}
		charm := 0
		if x := g.General(f.Lord); x != nil {
			charm = int(x.Charm)
		}
		f.Prestige = clampTo(f.Prestige+PrestigeDrift(charm, sum[f.ID], land[f.ID]), 100)
	}
}

// HarvestGoldCap 是秋收之後金的上限（`0x16aa1`：與 30000.0 比，`L0`）。
const HarvestGoldCap = 30000

// HarvestGold 是秋收進帳的金（`0x16a1b`–`0x16ac6`，`L0`）：
//
//	收入 ＝ (太守魅力 + 土地價值 × 4 + 民眾忠誠 × 2) × 人口 ÷ 300
//
// 人口用的是**存的值**（實際值 ÷ 100）。太守空缺時魅力算 0。
//
// **三個因子都在裡面**：土地價值權重最大（×4），忠誠其次（×2），
// 太守的魅力直接加。remake 原本只看土地價值與人口，忠誠與太守都不算——
// 那讓「派誰當太守」對收入沒有影響。
func HarvestGold(governorCharm, landValue, loyalty, population int) int {
	term := governorCharm + landValue*HarvestLandWeight + loyalty*HarvestLoyaltyWeight
	return term * (population / 100) / HarvestDivisor
}

// HarvestRice 是秋收進倉的米（`0x16afa`–`0x16ba1`，`L0`）：
//
//	收入 ＝ (太守魅力 + 土地價值 × 3 + (100 − 洪水率) + 民眾忠誠 × 2)
//	        × 人口 × 0.005
//
// **與金那一半不是同一組權重**：土地價值在這裡是 ×3 不是 ×4，
// 而且**洪水率會進算式**（`100 − 洪水率`，`0x16b18`）——水利做得好
// 收成才多，那正是「防洪」這道指令的回報。金那一半完全不看洪水率。
//
// 人口用的是**存的值**（實際值 ÷ 100）。太守空缺時魅力算 0。
//
// 收成之後不大於 0 時**改抽亂數**（`0x16b60`，金那一半是 `0x16a87`）：
// 米抽 `RND((土地價值 + 50) × 2)`、金抽 `RND(土地價值 + 50)`，而且是
// **取代**不是相加。兩條都在 `RunSeason` 的秋收那一段，不在這裡
// ——這一支是純函式，抽亂數要在有 `State` 的地方做。
func HarvestRice(governorCharm, landValue, floodRate, loyalty, population int) int {
	term := governorCharm + landValue*HarvestRiceLandWeight +
		(HarvestFloodBase - floodRate) + loyalty*HarvestLoyaltyWeight
	return term * (population / 100) / HarvestRiceDivisor
}

// HarvestFloorBase 是收成保底那條的常數（`0x16a89`／`0x16b62` 的 `+50`）。
const HarvestFloorBase = 50

// 秋收的米那一半的常數（`DS:0xa76e` ＝ 3.0、`DS:0xa776` ＝ 0.005）。
const (
	HarvestRiceLandWeight = 3
	HarvestFloodBase      = 100
	HarvestRiceDivisor    = 200
)

// 秋收的三個權重（`DS:0xa74e` ＝ 4.0、`DS:0xa756` ＝ 1/300）。
const (
	HarvestLandWeight    = 4
	HarvestLoyaltyWeight = 2
	HarvestDivisor       = 300
)

// 蝗害的兩道門檻（`0x16bd9`／`0x16c03`，`L0`）。
const (
	LocustLoyaltySpread = 80 // RND(80) + 30
	LocustLoyaltyFloor  = 30
	LocustLandSpread    = 100 // RND(100) + 30
	LocustLandFloor     = 30
)

// LocustStrikes 回報隨機挑中的那個郡鬧不鬧蝗害。
//
//	民眾忠誠 >= RND(80) + 30  → 不發生
//	土地價值 <= RND(100) + 30 → 不發生
//
// ⚠ **土地價值越高越容易鬧蝗害**——田越肥蟲越多。這與其他天災的方向
// 相反（瘟疫是土地價值**低**才發生），照著「災害都因為窮」的直覺寫會寫反。
func LocustStrikes(loyalty, landValue, loyaltyRoll, landRoll int) bool {
	if loyalty >= loyaltyRoll+LocustLoyaltyFloor {
		return false
	}
	return landValue > landRoll+LocustLandFloor
}

// GrowPopulation 是一年一次的人口成長（`0x16ec2`–`0x16f16`，`L0`）：
//
//	人口 ← min(上限, 人口 × (土地價值 + 民眾忠誠 ÷ 2 + 1000) ÷ 1000)
//
// 倍率的上限是 `100 + 50 + 1000 = 1150`，也就是**最快 15%**，
// 而且要土地價值與忠誠都滿檔才到得了。
//
// ⚠ 原本這裡寫的是固定 15%，出處是十六個月的對拍觀測「每個郡都剛好
// ×1.15」。那個觀測沒錯，但它走的是**載入舊進度**，而那份出貨存檔的
// 土地價值是 100、忠誠 99–100（36 個有主的郡裡 30 個倍率到頂）——
// 每個郡都在上限，所以看起來像常數。劇本 001 的倍率只有 1019–1059
// （2–6%），差得很遠。詳見 `CONTEXT.md` R15。
func GrowPopulation(population, landValue, loyalty int) int {
	n := population * (landValue + loyalty/2 + 1000) / 1000
	if n > PopulationCap {
		n = PopulationCap
	}
	return n
}

// winter 是冬季：人口增加與進貢物品，兩者都一年一次。
//
// **人口成長不是每個冬月都來。** 原版十六個月的觀測裡，二三十個郡的
// 人口只在十月一起變動，其他月份頂多動到幾個郡——那幾個是徵兵與賑民
// 之類的個別動作，不是全境的成長。
func (g *State) winter() []Event {
	var out []Event
	if g.Date.Month == growthMonth {
		for i := range g.prefectures {
			p := &g.prefectures[i]
			if !p.Owned() &&
				g.roll(int(Winter), p.ID, 9)%100 >= PopulationOwnerlessChance {
				continue
			}
			// **人口存的是「百」**（州郡 offset 14，`實際值 ÷ 100`），
			// 所以成長之後的零頭在寫回去的那一刻就沒了——徵兵那一支
			// 讀的是存回去的值（`0xbed9` 的 `filds 0x48e`），不是內部的
			// 精確數。留著零頭會讓當月徵完兵之後的人口多出一百
			//（月度對拍量到四個郡）。
			p.Population = GrowPopulation(p.Population,
				int(p.LandValue), int(p.PublicLoyalty)) / 100 * 100
		}
	}
	g.markPhase("人口成長")
	out = append(out, g.winterUnrest()...)
	g.markPhase("民亂判定")
	if g.Date.Month != tributeMonth {
		return out
	}
	// 「各州郡每年進貢寶物給諸侯，領地越多，貢品越多」（說明書 p.37）。
	//
	// 原版（`0x170b2`–`0x17363`，`L0`＋`L1`）先掃一次州郡湊兩張 16 格的表，
	// 再逐勢力發：
	//
	//	for 郡 = 1..42：所屬 != 0xFF →
	//	    領地數[所屬]++
	//	    人才量[所屬] += 民眾忠誠/4 + 土地價值/2
	//	for 勢力 = 0..15：
	//	    領地數 == 0 → 跳過
	//	    基數 = 人才量 ÷ (RND(10) + 80)        ; 0x1714c–0x1715b
	//	    基數 == 0 → 跳過                       ; 0x17162
	//	    每種寶物：v = RND(基數 + 1)            ; 0x17167
	//	              c = RND(5) + 8
	//	              c < v → v = RND(5) + 8      ; 再抽一次覆蓋
	//	              庫存 = min(100, 庫存 + v)
	//
	// **只有一個郡的勢力拿不到東西**：人才量約 75，除以 80–89 之後是 0。
	// 玉璽不在裡面——它只能諸侯持有，而且是勝利條件（說明書 p.24、p.37）。
	//
	// ⚠ **`RND(10)` 對每個有領地的勢力都要抽**（基數是不是 0 是後面才判
	// 的），四種寶物的兩次抽樣也一樣是無條件的。
	land := make([]int, len(g.factions)+16)
	talent := make([]int, len(land))
	for i := range g.prefectures {
		p := &g.prefectures[i]
		if !p.Owned() || int(p.Owner) >= len(land) {
			continue
		}
		land[p.Owner]++
		talent[p.Owner] += int(p.PublicLoyalty)/4 + int(p.LandValue)/2
	}
	for i := range g.factions {
		f := &g.factions[i]
		if int(f.ID) >= len(land) || land[f.ID] == 0 {
			continue
		}
		base := talent[f.ID] / (g.Roll(10, int(f.ID), 30) + TributeDivisorFloor)
		if base <= 0 {
			continue
		}
		// 四種的件數先各自擲好放著（`0x17167`–`0x1723b`，四段展開的碼，
		// 不是迴圈），**擲完之後再 `RND(4)` 挑一種多給一件**
		// （`0x1723c`：`incw -0x8(%bp,%si)`，加在件數上而不是庫存上），
		// 最後才逐種進庫（`0x172f8`：`庫存 = min(100, 庫存 + 件數)`）。
		//
		// 那一次 `RND(4)` 是**無條件**的——四種都擲到 0 也照抽，
		// 所以它算進每個過閘門的勢力固定的 9 次裡。
		var got [treasureCount]int
		for t := TreasureBook; t < treasureCount; t++ {
			v := g.Roll(base+1, int(f.ID), int(t), 30)
			if cap := g.Roll(TributeCapSpread, int(f.ID), int(t), 31) + TributeCapFloor; cap < v {
				v = g.Roll(TributeCapSpread, int(f.ID), int(t), 32) + TributeCapFloor
			}
			got[t] = v
		}
		// **`RND(4)` 對四種**，不是對整個列舉——列舉的第 0 格是玉璽，
		// 而玉璽只能諸侯持有、不在進貢的四種裡（`0x1723c`）。
		got[int(TreasureBook)+g.Roll(4, int(f.ID), 33)]++
		n := 0
		for t := TreasureBook; t < treasureCount; t++ {
			f.Treasury[t] = clampTo(f.Treasury[t]+got[t], TreasuryCap)
			n += got[t]
		}
		if n > 0 {
			lord := g.Lord(f.ID)
			name := tf("fld.factionN", f.ID)
			if lord != nil {
				name = personName(lord.Name)
			}
			out = append(out, Event{Prefecture: 0, Text: tf("ev.tribute", name, n)})
		}
	}
	return out
}

// retire 把一位人物從舞台上移走（老死用）。
//
// ⚠ **主事者死掉不能讓郡就這樣沒人管。** 有人接手就接手，
// 沒有人接手就變成空白郡——「因任何事故所形成的空白郡均不屬任何諸侯」
// （說明書 p.19）。
func (g *State) retire(x *General) { g.retireBy(x, "aging") }

// retireBy 是 retire 加上死因，記在 DeathLog（對拍與普查用）。
func (g *State) retireBy(x *General, cause string) {
	if x.Status != state.StatusFallen {
		if g.DeathLog == nil {
			g.DeathLog = map[string]int{}
		}
		g.DeathLog[cause]++
	}
	at, faction := x.Location, x.Faction
	wasGoverning := x.Status.Governs()
	wasLord := x.Status == state.StatusLord
	x.Soldiers = 0
	x.Faction = state.NoFaction
	// **死掉的人身分是 12（已故），君主與部下都一樣**（`0x147f6`／`0x14881`：
	// 身分 ← 12、勢力 ← 0xFF、領地 ← 0xFF）。先前寫成在野（9）——
	// 原版 280 年時已故累積 342 人，remake 一直是 3（`docs/playtest/05` §5）。
	// **忠誠不動、所在寫 0xFF**：那三行只寫這三格（加強版七月視窗量到，
	// 死掉的君主忠誠還是 100、所在 255）。
	x.Status = state.StatusFallen
	x.Location = int(state.NoValue)
	if f := g.Faction(faction); f != nil && f.Chief == x.Index {
		f.Chief = -1
	}
	if !wasGoverning {
		// 軍師或一般武將是這個郡最後一位現役的話（主事者早就不在，
		// 軍師在代理），郡跟著變無主。原版沒有這一段——它的歸屬是
		// 下一次重算（`0x1e394`）才落掉的；remake 在這裡提前做，
		// 與上面主事者死掉的處理同一個形狀。
		if p := g.Prefecture(at); p != nil && p.Owner == faction &&
			g.leavesNobody(at, nil, faction) {
			p.Owner = state.NoFaction
		}
		return
	}
	if wasLord {
		// 君主的繼承不看郡：原版掃**整個勢力**（`0x14a84` 的 350 筆迴圈）
		// 依魅力挑，所以繼承者常常人在別的郡。
		if g.SucceedLord(faction) != nil {
			return
		}
		// **沒有繼承人就把君主欄清成哨兵**（原版 `0x14ba3`：操縱方 ← `0xFFFF`；
		// 君主欄指著已故的那一位）。輸的定義是「絕嗣」不是「沒領地」
		// （`0x15924`，`docs/mechanics/80` §1.1），這一格不清的話那個判定
		// 永遠成立。`Alive` 跟著變假——原版「活著的勢力」就是這一格
		// （`0x15cd4`：君主槽 != `0xFFFF`），不是有沒有領地。
		//
		// 郡**不必另外釋出**：絕嗣代表這個勢力一個武將都不剩，而郡的歸屬
		// 是從人物表重算的（`RecomputeOwners`，`0x1e394`），下一次重算
		// 自然全部變無主。原版也沒有另一段釋出的碼（`0x14b67`–`0x14c0e`
		// 只寫操縱方與玉璽）。
		if f := g.Faction(faction); f != nil {
			f.Lord, f.Alive = -1, false
		}
	} else if succ := g.successorFor(at, x.Index, faction); succ != nil {
		succ.Status = state.StatusGovernor
		return
	}
	if p := g.Prefecture(at); p != nil {
		p.Owner = state.NoFaction
	}
}

// Winner 回報有沒有人一統天下（`0x15852`，`L0`、`[base]`）。
//
// 條件只有一條：**所有有主的郡屬於同一個勢力**。原版逐郡掃兩遍——
// 先取一個有主的郡的所屬當基準，再確認其他有主的郡都是同一方——
// 然後直接印「%s 一統天下」（`DS:0x66d4`）並把月內迴圈的旗標清掉。
//
// **無主的郡不算。** 掃描碰到所屬 `0xFF` 就跳過，所以地圖上還有荒地
// 也照樣算統一。
//
// **玉璽不是條件。** 說明書 p.37 的「在遊戲結束前一定要拿到玉璽」在碼裡
// 沒有對應：那支常式除了郡的所屬與君主的姓名之外沒有讀任何東西。
// 玉璽的作用在別處——現世時給人望（`docs/re/06` §7.1）。
func (g *State) Winner() (f state.FactionID, done bool) {
	var only state.FactionID = state.NoFaction
	for i := range g.prefectures {
		p := &g.prefectures[i]
		if !p.Owned() {
			continue
		}
		if only == state.NoFaction {
			only = p.Owner
			continue
		}
		if p.Owner != only {
			return state.NoFaction, false
		}
	}
	if only == state.NoFaction {
		return state.NoFaction, false
	}
	return only, true
}

// SuccessionPrestigeScale 是繼承之後人望要乘的分母（原版是浮點的
// `× 0.01` 再 `+ 0.5` 取整，`DS:0xa6f8`／`DS:0xa700`，`L0`）。
const SuccessionPrestigeScale = 100

// SuccessionPrestige 是繼承之後這個勢力剩多少人望（`L0`、`[base]`）：
//
//	新人望 = 四捨五入(繼承者魅力 × 原人望 ÷ 100)
//
// **魅力就是繼承的代價**：魅力 100 的世子保住全部人望，魅力 50 的
// 一上任就折半。人望決定部下忠誠每年的漲跌（`Faction.Prestige`），
// 所以一次糟糕的繼承會讓整個勢力慢慢離心。
func SuccessionPrestige(charm, prestige int) int {
	return (charm*prestige + SuccessionPrestigeScale/2) / SuccessionPrestigeScale
}

// SucceedLord 讓一個勢力的君主之位由部下接手（原版 `0x14a40`–`0x14ce6`，
// `L0`、`[base]`）。找不到人回 nil，那就是「無人繼承」。
//
// 原版的順序：
//
//	死者身分 ← 12（已故）、勢力與領地 ← 0xFF
//	候選 ＝ 掃全人物表，勢力欄等於這一方的人
//	依**魅力**由高到低排序
//	電腦取排頭；玩家自己挑（諸侯 offset 0 == 1 ＝ 玩家操縱）
//	勢力人望 ← 四捨五入(繼承者魅力 × 原人望 ÷ 100)
//	繼承者原本是軍師 → 軍師欄清空（一個人不能同時是君主與軍師）
//	繼承者：職位 ← 0、兵種 ← 6、身分 ← 君主
//
// **候選不限於死者所在的郡**——掃的是整張人物表。只從同一個郡找的話，
// 君主戰死在外地時會找不到人，而勢力就這樣無聲地滅了。
func (g *State) SucceedLord(id state.FactionID) *General {
	f := g.Faction(id)
	if f == nil {
		return nil
	}
	// 進來先印一則對白（`0x14a38`／加強版 `0x13c6b` → 對白常式，`RND(8)`）；
	// 老死與戰死都走這一支。說話的是**死去的君主**（`es:0x24d6`，`0x147d9`
	// 寫進去的），上格、肖像在右。
	if dead := g.General(f.Lord); dead != nil {
		g.pending = append(g.pending, g.bubbleEvent(dead, true, false,
			tf("bub.lordDeath", personName(dead.Name)), int(id), 0x14a38))
	} else {
		g.Roll(MessageLines, int(id), 0x14a38)
	}
	var heir *General
	for i := range g.generals {
		x := &g.generals[i]
		if x.Faction != id || !x.Employed() {
			continue
		}
		if heir == nil || x.Charm > heir.Charm {
			heir = x
		}
	}
	if heir == nil {
		return nil
	}
	f.Prestige = SuccessionPrestige(int(heir.Charm), f.Prestige)
	if f.Chief == heir.Index {
		f.Chief = -1
	}
	// 繼承者：職位 ← 0、兵種 ← 6、身分 ← 君主（`0x14cce`–`0x14cda`）；他所在
	// 的郡：自治 ← 0、原本的太守降成一般武將、主事者 ← 他（`0x14cf1`–`0x14d36`）。
	heir.Rank = state.RankLord
	heir.Troop = state.TroopType(6)
	heir.Status = state.StatusLord
	f.Lord = heir.Index
	if p := g.Prefecture(heir.Location); p != nil {
		p.Autonomy = AutoNormal
		if old := g.General(p.governor); old != nil && old.Status == state.StatusGovernor {
			old.Status = state.StatusOfficer
		}
		p.governor = heir.Index
	}
	// 找到繼承者：先印一行（`0x14d61`／`0x14d71`，不擲），再一則對白
	// （原版 `0x14dd2`、加強版 `0x13fa7` → 對白常式，`RND(8)`，`[both]`；
	// 片語 465「主公寬心  某必光大主公之霸業」，下格、肖像在左，說話的是
	// 繼承者 `es:0x1eda`）。兩版的碼逐字相同；加強版七月視窗量到過，
	// 原版的視窗還沒剛好死過君主。
	g.pending = append(g.pending, g.bubbleEvent(heir, false, true,
		tf("bub.succeed", personName(heir.Name)), int(id), 0x14dd2))
	return heir
}

// 老死（原版 `0x15d5d`–`0x15dd7`，`L0`、`[base]`）。
//
// **壽命是「幾歲開始走下坡」不是「幾歲一定死」**：沒過壽命的人體能
// 一點都不掉，過了才開始扣，扣到 0 才是死。體能因此是壽命的緩衝——
// 體能 80 的人過壽一年掉到 5–55 還活著，過壽四年才必死。
const (
	// LifespanRollSpread 是那道門的浮動：`RND(3) + 壽命 >= 年齡` 就跳過。
	LifespanRollSpread = 3
	// OverAgeWeight 是每超過壽命一歲要多扣的體能。
	OverAgeWeight = 25
	// DeathStaminaSpread 是每年的隨機扣減 `RND(50)`。
	DeathStaminaSpread = 50
)

// AlreadyPastPrime 回報這個人今年要不要跑老死判定。
func AlreadyPastPrime(age, lifespan, roll int) bool { return roll+lifespan < age }

// AgingDrop 是過了壽命之後的新體能，夾到 0。
//
//	新體能 = 體能 + (壽命 − 年齡) × 25 − RND(50)
//
// `壽命 − 年齡` 在這裡一定是負的，所以那一項是扣分。
func AgingDrop(stamina, age, lifespan, roll int) int {
	n := stamina + (lifespan-age)*OverAgeWeight - roll
	if n < 0 {
		return 0
	}
	return n
}

// winterUnrest 是冬季常式在人口成長之後的那一段（`0x16f22`–`0x16f95`，`L0`）：
// 抽一個郡，民眾忠誠低而諸侯人望又低，那個郡就出事。
//
//	郡 = RND(42) + 1
//	所屬 == 0xFF → 結束                      ; 0x16f3d
//	門檻 = RND(20) + 50                      ; 0x16f48
//	民眾忠誠（offset 26）>= 門檻 → 結束      ; 0x16f65
//	c = RND(20) + 30                         ; 0x16f6c
//	諸侯[所屬] 的人望（offset 8）>= c → 結束 ; 0x16f8e
//	→ 出事
//
// ⚠ **出事之後做什麼還沒解**：`0x16f98` 顯示 `DS:0x680a` 的訊息，接著對
// 那個郡做一次間接呼叫（`lcall *es:[0x20ea]`，參數 28）。所以這裡只還原
// 判定與**抽樣的次數**——少了這三次，整條亂數序列就對不上
// （`CONTEXT.md` 的亂數路線圖）。
func (g *State) winterUnrest() []Event {
	at := g.Roll(len(g.prefectures), 40) + 1
	p := g.Prefecture(at)
	if p == nil || !p.Owned() {
		return nil
	}
	if int(p.PublicLoyalty) >= g.Roll(UnrestSpread, at, 41)+UnrestLoyaltyFloor {
		return nil
	}
	f := g.Faction(p.Owner)
	if f == nil || f.Prestige >= g.Roll(UnrestSpread, at, 42)+UnrestPrestigeFloor {
		return nil
	}
	return []Event{{Prefecture: p.ID, Text: tf("ev.unrest", p.Name)}}
}
