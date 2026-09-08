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
//
// 剩下的 `Tune*` 是碼裡找不到對應的那幾項（瘟疫的體能下降、秋收的米、
// 老化、進貢件數、出頭年齡）。

const (
	// TunePlagueStamina 是瘟疫讓將領體能下降多少。
	//
	// **只剩這一項是 remake 選的**：碼裡的瘟疫只動人口
	// （`0x168fc`），手冊 p.36 說的「將領體能下降」在碼裡找不到，
	// 但把它拿掉會讓瘟疫與「人口減少」完全同義。
	TunePlagueStamina = 5
	// TuneHarvestRicePerLand 是秋收的米產出：
	// 每一點土地價值換多少米／金，再乘人口規模。
	TuneHarvestRicePerLand = 2

	// PopulationCap 是每個郡的人口上限（原版 `0x16f0a` 夾在 10000，
	// 存的值 ×100，`L0`）。
	PopulationCap = 1_000_000
	// PopulationOwnerlessChance 是無主郡成長的機率（`0x16ebd`，`L0`）：
	// `RND(100) < 30` 才長。有主的郡每次都長。
	PopulationOwnerlessChance = 30
	// ⚠ TuneAgingStamina 已經被量到的公式取代（`AgingDrop`，`L0`）。
	// 留著只為讓 `docs/design/02` 的對照表讀得下去。
	TuneAgingStamina = 1
	// 進貢的上限（`RND(5) + 8`，`L0`）。
	TributeCapSpread = 5
	TributeCapFloor  = 8

	// 冬季民亂的三個常數（`0x16f48`／`0x16f6c`，`L0`）：門檻是
	// `RND(20) + 50`（民眾忠誠）與 `RND(20) + 30`（人望）。
	UnrestSpread         = 20
	UnrestLoyaltyFloor   = 50
	UnrestPrestigeFloor  = 30

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

// floodBase 是各郡的洪水基礎值（`DS:0x679c`，43 個 word，`L0`）。
//
// **各郡不同**——有些郡天生就容易淹。索引 0 是啞元郡。
var floodBase = [state.PrefectureCount + 1]int{
	0, 0, 5, 4, 3, 1, 2, 5,
	7, 3, 3, 7, 2, 2, 7, 7,
	3, 2, 1, 2, 0, 5, 5, 3,
	10, 3, 12, 2, 2, 5, 5, 10,
	3, 3, 3, 1, 2, 5, 4, 1,
	2, 3,
}

// FloodBase 是某個郡的洪水基礎值。
func FloodBase(prefectureID int) int {
	if prefectureID < 0 || prefectureID >= len(floodBase) {
		return 0
	}
	return floodBase[prefectureID]
}

// FloodRise 是每個月洪水率自己的變動（`0x16500`–`0x16543`，`L0`）：
//
//	新洪水率 ＝ min(100, RND(舊 ÷ 4) + 舊 % 4 + 基礎[郡])
//
// **這件事每個月都發生，與淹不淹無關。** 期望值大約
// `舊 × 0.625 + 基礎`，所以洪水率會往 `基礎 ÷ 0.375` 收斂——
// 防洪（`FloodDrop` ＝ 謀略 ÷ 10）壓下去之後還會爬回來。
func FloodRise(rate, base, roll int) int {
	v := roll + rate%4 + base
	if v > 100 {
		v = 100
	}
	if v < 0 {
		v = 0
	}
	return v
}

// 水災的兩道判定（`0x16574`／`0x16585`，`L0`）。
const (
	FloodRollSpread = 65 // RND(65) + 5
	FloodRollFloor  = 5
	FloodGateSpread = 100 // 再擲一次 RND(100)，<= 80 就不淹
	FloodGateBar    = 80
)

// FloodStrikes 回報這個郡這個月淹不淹。
//
// rateRoll ＝ `RND(新洪水率)`、guard ＝ `RND(65)`、gate ＝ `RND(100)`。
//
// **兩道都要過**：`RND(65) + 5 < RND(洪水率)` 而且 `RND(100) > 80`。
// 第二道是無條件的 19%，所以就算洪水率滿檔也不是每個月都淹。
func FloodStrikes(rateRoll, guard, gate int) bool {
	return guard+FloodRollFloor < rateRoll && gate > FloodGateBar
}

// 瘟疫的兩道門檻（`0x167df`–`0x1683d`，`L0`）。
const (
	PlagueLoyaltySpread = 45 // RND(45) + 25
	PlagueLoyaltyFloor  = 25
	PlagueLandSpread    = 40 // RND(40) + 20
	PlagueLandFloor     = 20
)

// PlagueStrikes 回報隨機挑中的那個郡鬧不鬧瘟疫。
//
// **瘟疫不逐郡掃**，原版每個月只挑一個郡（`RND(42) + 1`）。
// 兩道門檻都要過：忠誠 70 以上一定安全（門檻上限 69），
// 25 以下一定過第一關；土地價值 59 以上安全、20 以下必過。
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
	if g.Date.Month == agingMonth {
		for i := range g.generals {
			x := &g.generals[i]
			if x.Name == "" {
				continue
			}
			x.Age++
			// 未登場的人只長年紀，不受體能衰退與老死影響。
			if x.Status == state.StatusUnborn {
				continue
			}
			// **老死看壽命，不是每年掉一點**（`0x15d5d`，`L0`）：
			// 沒過壽命的人體能一點都不掉。
			if AlreadyPastPrime(int(x.Age), int(x.Lifespan),
				g.roll(int(Spring), x.Index, 43)%LifespanRollSpread) {
				n := AgingDrop(int(x.Stamina), int(x.Age), int(x.Lifespan),
					g.roll(int(Spring), x.Index, 44)%DeathStaminaSpread)
				x.Stamina = uint8(n)
				if n == 0 {
					out = append(out, Event{x.Location, tf("ev.death", personName(x.Name))})
					g.retire(x)
					continue
				}
			}
			// **忠誠每年跟著君主的人望漂移**（`0x15fc2`，`L0`）：
			// `忠誠 += (人望 − 60) ÷ 2`。**君主自己不算**——
			// 他不會對自己不忠。
			if x.Status != state.StatusLord && x.Faction != state.NoFaction {
				if f := g.Faction(x.Faction); f != nil {
					x.Loyalty = uint8(LoyaltyDrift(int(x.Loyalty), f.Prestige,
						g.roll(int(Spring), x.Index, 42)%LoyaltyFloorSpread))
				}
			}
			// **訓練度與武裝度每年各自掉**（`0x16006`／`0x16025`，`L0`）：
			// `−RND(值 ÷ 10)`，與土地價值同一個形狀。
			//
			// 這是「訓練兵士」與「購置武器」要一直做的原因——少了它，
			// 一次練到頂就永遠是精兵，而數值欄停在 100 看起來完全正常。
			x.Arms = uint8(AnnualDecay(int(x.Arms),
				g.roll(int(Spring), x.Index, 40)%max(1, int(x.Arms)/10+1)))
			x.Training = uint8(AnnualDecay(int(x.Training),
				g.roll(int(Spring), x.Index, 41)%max(1, int(x.Training)/10+1)))
		}
	}
	out = append(out, g.comeOfAge()...)
	// **土地價值每個春月自己掉**（`0x15c96`，`L0`）：`−RND(土地價值 ÷ 10)`。
	// 逐郡跑，無主的郡也跑。這是「土地開發要一直做」的原因——
	// 少了它，一次開發到頂就永遠不用再管。
	for i := range g.prefectures {
		p := &g.prefectures[i]
		p.LandValue = uint8(LandValueDecay(int(p.LandValue),
			g.roll(int(Spring), p.ID, 8)%max(1, int(p.LandValue)/10+1)))
	}
	// **玉璽現世**（`0x15cfd`–`0x1519a`，`L0`）：玉璽還沒出現的話，
	// 每個春月約半數機率落到隨機一個活著的勢力手上，那一家的人望
	// 大漲。玉璽是勝利條件之一（說明書 p.24）。
	out = append(out, g.sealEvent()...)
	// **地震：每個春月 1/3 的機率，落在隨機一個郡**（`0x162d2`，`L0`）。
	// 不看民眾忠誠也不看土地價值——那道「天災多因人怨」的門檻是瘟疫的。
	if g.roll(int(Spring), 0, 10)%QuakeChance == 0 {
		id := g.roll(int(Spring), 0, 11)%state.PrefectureCount + 1
		if p := g.Prefecture(id); p != nil && p.Owned() {
			// 三份保留率各擲一次，**不是同一個百分比套三次**。
			p.Population = QuakePopKeep.Apply(p.Population,
				g.roll(int(Spring), id, 20)%QuakePopKeep.Spread)
			if p.Population < QuakePopFloor {
				p.Population = QuakePopFloor
			}
			p.Gold = QuakeGoldKeep.Apply(p.Gold,
				g.roll(int(Spring), id, 21)%QuakeGoldKeep.Spread)
			p.Rice = QuakeRiceKeep.Apply(p.Rice,
				g.roll(int(Spring), id, 22)%QuakeRiceKeep.Spread)
			out = append(out, Event{p.ID, tf("ev.quake", placeName(p.Name))})
		}
	}
	return out
}

// 人望決定忠誠漲跌的分水嶺與下限（`0x15fc7`／`0x15fd8`，`L0`）。
const (
	LoyaltyPivot       = 60 // 人望高於它部下向心，低於它離心
	LoyaltyFloorSpread = 5  // 算出負的就換成 RND(5)
)

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
	// 水災（`0x1666d`／`0x166c1`／`0x16701`）。
	FloodPopKeep  = Keep{10, 70}  // 70–79%
	FloodLandKeep = Keep{20, 60}  // 60–79%
	FloodRateGain = Keep{20, 120} // 洪水率 ×1.20–1.39，越界變 100
	// 瘟疫（`0x168ce`）：只動人口。
	PlaguePopKeep = Keep{20, 40} // 40–59%
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

// QuakePopFloor 是地震之後人口的下限（`0x163bb`：低於 50 就補到 50，`L0`）。
//
// 存的值是實際值 ÷ 100，所以下限是五千人。
const QuakePopFloor = 50 * 100

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
		x := &g.generals[i]
		if x.Status != state.StatusUnborn || x.Name == "" {
			continue
		}
		// **出頭年齡是每個人自己的**（人物表 offset 26，`0x1605a`）——
		// 不是一個全域常數。原版比的是「年齡 > Debut」，不是 >=。
		if int(x.Age) <= int(x.Debut) {
			continue
		}
		// **先看牽絆對象**（`0x16064`）：他有勢力、而且他所在的郡還沒
		// 滿五十位現役武將的話，這個人直接投奔他，不是回出身郡當在野。
		//
		// 少了這一條，名將的子姪與舊部都會變成散落各地的在野人士，
		// 而「牽絆」在登用之外就沒有別的作用了。
		if at, id, ok := g.bondDebut(x); ok {
			x.Location = at
			x.Faction = id
			x.Status = state.StatusOfficer
			out = append(out, Event{at,
				tf("ev.appear", personName(x.Name), placeName(g.Prefecture(at).Name))})
			continue
		}
		at := x.Origin
		if at < 1 || at > len(g.prefectures) {
			at = x.Location
		}
		if at < 1 || at > len(g.prefectures) {
			continue
		}
		// 退路是**出身郡**，身分「在野而且露面」、無勢力（`0x15ec4`）。
		x.Location = at
		x.Faction = state.NoFaction
		x.Status = state.StatusAvailable
		p := g.Prefecture(at)
		name := ""
		if p != nil {
			name = p.Name
		}
		out = append(out, Event{at, tf("ev.appear", personName(x.Name), placeName(name))})
	}
	return out
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
	return []Event{{0, tf("ev.seal", name)}}
}

// DebutGarrisonCap 是「牽絆對象的郡收不收得下」的門檻
// （`0x16097`：現役武將數 < 50，`L0`）。與每郡五十位將軍的上限同一個數。
const DebutGarrisonCap = 50

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
	if len(g.Garrison(at)) >= DebutGarrisonCap {
		return 0, state.NoFaction, false
	}
	return at, b.Faction, true
}

// summer 是夏天：洪水與瘟疫。
func (g *State) summer() []Event {
	var out []Event
	for i := range g.prefectures {
		p := &g.prefectures[i]
		if !p.Owned() {
			continue
		}
		// **洪水率每個月都會自己動**，與淹不淹無關（`0x16500`）。
		p.FloodRate = uint8(FloodRise(int(p.FloodRate), FloodBase(p.ID),
			g.roll(int(Summer), p.ID, 3)%max(1, int(p.FloodRate)/4+1)))
		// 淹不淹：兩道擲骰都要過（`0x16574`／`0x16585`）。
		if FloodStrikes(g.roll(int(Summer), p.ID, 4)%max(1, int(p.FloodRate)+1),
			g.roll(int(Summer), p.ID, 1)%FloodRollSpread,
			g.roll(int(Summer), p.ID, 2)%FloodGateSpread) {
			p.Population = FloodPopKeep.Apply(p.Population,
				g.roll(int(Summer), p.ID, 20)%FloodPopKeep.Spread)
			p.LandValue = uint8(FloodLandKeep.Apply(int(p.LandValue),
				g.roll(int(Summer), p.ID, 21)%FloodLandKeep.Spread))
			// 洪水率**乘上去**（`0x16711`）：×1.20–1.39，算出來超過 100
			// 或變成負的就是 100。說明書 p.21 說「水災後洪水率立刻升到
			// 100」——那是高洪水率的郡才成立，低的只是往上推一截。
			rate := FloodRateGain.Apply(int(p.FloodRate),
				g.roll(int(Summer), p.ID, 22)%FloodRateGain.Spread)
			if rate > 100 || rate < 0 {
				rate = 100
			}
			p.FloodRate = uint8(rate)
			out = append(out, Event{p.ID, tf("ev.flood", placeName(p.Name))})
		}
	}
	// **瘟疫每個月只挑一個郡**（`RND(42) + 1`，`0x167df`），不逐郡掃。
	{
		id := g.roll(int(Summer), 0, 5)%state.PrefectureCount + 1
		p := g.Prefecture(id)
		if p != nil && p.Owned() &&
			PlagueStrikes(int(p.PublicLoyalty), int(p.LandValue),
				g.roll(int(Summer), id, 6)%PlagueLoyaltySpread,
				g.roll(int(Summer), id, 7)%PlagueLandSpread) {
			p.Population = PlaguePopKeep.Apply(p.Population,
				g.roll(int(Summer), id, 20)%PlaguePopKeep.Spread)
			for _, x := range g.Garrison(p.ID) {
				x.Stamina = uint8(clampTo(int(x.Stamina)-TunePlagueStamina, 100))
			}
			out = append(out, Event{p.ID, tf("ev.plague", placeName(p.Name))})
		}
	}
	return out
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

// repriceAll 每個月替每一個郡重抽物價。
//
// **原版每個月幾乎四十二個郡一起換**（連走十六個月，`L1`）。
// 量到的範圍是 30–68，而值的分布在 44–50 隆起、兩端收斂——不是平的，
// 所以不是單一個均勻亂數（`docs/mechanics/60-economy.md` §1.2）。
//
// 形狀與「兩個 0–19 的亂數相加」相符（峰值 49），這裡就照那個做。
//
// 它**不是**郡狀態的函數：把地力每十格掃一次，物價在 31–40 之間亂跳，
// 沒有單調性，而且民忠也在跳（民忠怎麼看都不該進物價）。
// 那是原版自己的亂數被災害判定位移造成的，不是因果
// （`docs/mechanics/60-economy` §1.2）。
func (g *State) repriceAll() {
	// **啞元那一筆也要重抽。** 原版的州郡表有 43 筆，第 0 筆是啞元
	// （`docs/formats/03`），而重抽物價的迴圈走完整張表——月度對拍的
	// 差異清單裡「郡 0：物價」一直都在。remake 的 `prefectures` 只有
	// 42 筆，少抽兩次，而**那是整條亂數序列第一個岔開的地方**。
	if len(g.rawSta) > staPrice {
		a := g.roll(int(priceSalt), 0) % (PriceSpread + 1)
		b := g.roll(int(priceSalt), 0, 1) % (PriceSpread + 1)
		g.rawSta[staPrice] = byte(PriceMin + a + b)
	}
	for i := range g.prefectures {
		p := &g.prefectures[i]
		a := g.roll(int(priceSalt), p.ID) % (PriceSpread + 1)
		b := g.roll(int(priceSalt), p.ID, 1) % (PriceSpread + 1)
		p.PriceLevel = uint8(PriceMin + a + b)
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
		rice := int(p.LandValue) * TuneHarvestRicePerLand * (p.Population / 1000) / 10
		p.Gold = clampTo(p.Gold+gold, HarvestGoldCap)
		p.Rice = clampTo(p.Rice+rice, MaxRice)
		out = append(out, Event{p.ID,
			tf("ev.harvest", placeName(p.Name), rice, gold)})
	}
	// **蝗害每年只挑一個郡**（`0x16bd5`，`L0`），不逐郡掃。
	{
		id := g.roll(int(Autumn), 0, 12)%state.PrefectureCount + 1
		p := g.Prefecture(id)
		if p != nil && p.Owned() &&
			LocustStrikes(int(p.PublicLoyalty), int(p.LandValue),
				g.roll(int(Autumn), id, 13)%LocustLoyaltySpread,
				g.roll(int(Autumn), id, 14)%LocustLandSpread) {
			p.Rice = LocustRiceKeep.Apply(p.Rice,
				g.roll(int(Autumn), id, 20)%LocustRiceKeep.Spread)
			// ⚠ **蝗害的土地價值保留率是 80–169%**（`0x16cfa`：`RND(90) + 80`），
			// 也就是平均會**上升**。碼就是這樣寫的——以碼為準，
			// 但夾在 100 以內（欄位是 u8，說明書的範圍是 0–100）。
			// 這一條與直覺相反到值得單獨對拍一次，記在 `docs/re/06` §6。
			p.LandValue = uint8(clampTo(LocustLandKeep.Apply(int(p.LandValue),
				g.roll(int(Autumn), id, 21)%LocustLandKeep.Spread), 100))
			out = append(out, Event{p.ID, tf("ev.locust", placeName(p.Name))})
		}
	}
	return out
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
			p.Population = GrowPopulation(p.Population,
				int(p.LandValue), int(p.PublicLoyalty))
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
		n := 0
		for t := TreasureBook; t < treasureCount; t++ {
			got := g.Roll(base+1, int(f.ID), int(t), 30)
			if cap := g.Roll(TributeCapSpread, int(f.ID), int(t), 31) + TributeCapFloor; cap < got {
				got = g.Roll(TributeCapSpread, int(f.ID), int(t), 32) + TributeCapFloor
			}
			f.Treasury[t] = clampTo(f.Treasury[t]+got, TreasuryCap)
			n += got
		}
		if n > 0 {
			lord := g.Lord(f.ID)
			name := tf("fld.factionN", f.ID)
			if lord != nil {
				name = personName(lord.Name)
			}
			out = append(out, Event{0, tf("ev.tribute", name, n)})
		}
	}
	return out
}

// retire 把一位人物從舞台上移走（老死用）。
//
// ⚠ **主事者死掉不能讓郡就這樣沒人管。** 有人接手就接手，
// 沒有人接手就變成空白郡——「因任何事故所形成的空白郡均不屬任何諸侯」
// （說明書 p.19）。
func (g *State) retire(x *General) {
	at, faction := x.Location, x.Faction
	wasGoverning := x.Status.Governs()
	wasLord := x.Status == state.StatusLord
	x.Soldiers = 0
	x.Faction = state.NoFaction
	x.Status = state.StatusIdle
	x.Location = 0
	x.Loyalty = state.NoValue
	if f := g.Faction(faction); f != nil && f.Chief == x.Index {
		f.Chief = -1
	}
	if !wasGoverning {
		return
	}
	if wasLord {
		// **死掉的君主身分是 12（已故）不是在野**——原版 `0x14a5d`
		// 直接寫 12，而在野（9）的人還會被登用。
		x.Status = state.StatusFallen
		// 君主的繼承不看郡：原版掃**整個勢力**（`0x14a84` 的 350 筆迴圈）
		// 依魅力挑，所以繼承者常常人在別的郡。
		if g.SucceedLord(faction) != nil {
			return
		}
	} else if succ := g.successorFor(at, x.Index, faction); succ != nil {
		succ.Status = state.StatusGovernor
		return
	}
	if p := g.Prefecture(at); p != nil {
		p.Owner = state.NoFaction
	}
	if f := g.Faction(faction); f != nil && len(g.Territory(faction)) == 0 {
		f.Alive = false
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
	heir.Status = state.StatusLord
	f.Lord = heir.Index
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
//（`CONTEXT.md` 的亂數路線圖）。
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
	return []Event{{p.ID, tf("ev.unrest", p.Name)}}
}
