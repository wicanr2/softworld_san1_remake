package game

import (
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 四季事件（說明書 p.36–37）。
//
// 手冊給了**清單與方向**（「洪水造成郡內洪水率激升 100」「秋收過後
// 土地價值會略降，稅金入庫、米糧進倉」），沒給係數。方向照做，
// 係數走 `Tune*`，一眼看得出哪些是 remake 選的。
//
// 「天災多因人怨引起，民眾忠誠最好不要太低」——所以災害機率與
// 民眾忠誠掛鉤，這是手冊明講的因果。

const (
	// TuneQuakeLoss 是地震的損失百分比（人口／金／米／兵）。
	TuneQuakeLoss = 10
	// TuneFloodPopLoss／TuneFloodLandLoss 是洪水的人口與土地價值損失。
	TuneFloodPopLoss  = 8
	TuneFloodLandLoss = 5
	// TunePlagueLoss 是瘟疫的人口與兵力損失，TunePlagueStamina 是體能下降。
	TunePlagueLoss    = 12
	TunePlagueStamina = 5
	// TuneHarvestRicePerLand／TuneHarvestGoldPerLand 是秋收的產出：
	// 每一點土地價值換多少米／金，再乘人口規模。
	TuneHarvestRicePerLand = 2
	TuneHarvestGoldPerLand = 1
	// TuneHarvestLandDrop 是秋收後土地價值的下降。
	TuneHarvestLandDrop = 2
	// TuneLocustRiceLoss／TuneLocustLandLoss 是蝗害。
	TuneLocustRiceLoss = 30
	TuneLocustLandLoss = 5
	// PopulationCap 是每個郡的人口上限（原版 `0x16f0a` 夾在 10000，
	// 存的值 ×100，`L0`）。
	PopulationCap = 1_000_000
	// PopulationOwnerlessChance 是無主郡成長的機率（`0x16ebd`，`L0`）：
	// `RND(100) < 30` 才長。有主的郡每次都長。
	PopulationOwnerlessChance = 30
	// TuneAgingStamina 是每年體能的衰減基準；年紀越大掉越多。
	TuneAgingStamina = 1
	// TuneDisasterBase 是災害的**每月**基礎機率；民眾忠誠越低越高。
	// 夏天有三個月，所以一年的水患機率遠高於這個數字。
	TuneDisasterBase = 8
	// TuneFloodWeight 是洪水率對水患機率的權重（百分比）。
	TuneFloodWeight = 50
	// TuneTributePerPrefecture 是每幾個郡一年進貢一件寶物。
	TuneTributePerPrefecture = 3

	// TuneComingOfAge 是未登場的人物幾歲出頭。
	//
	// 手冊只說「新血出現：新將投效其親族朋友」（p.36），沒給年齡。
	// 用二十歲的話，劇本 001 裡八歲的諸葛亮會在西元 201 年前後登場，
	// 十四歲的孫策在 195 年——與史實的量級相符。
	TuneComingOfAge = 20
)

// Event 是一則發生過的事件，給訊息列與測試用。
type Event struct {
	Prefecture int // 0 表示不屬於特定郡
	Text       string
}

// RunSeason 跑這個月的季節事件，回傳發生了什麼。
//
// 呼叫時機是**推進到新的月份之後**——事件屬於新的那個月。
func (g *State) RunSeason() []Event {
	switch g.Date.Season() {
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

// disasterChance 是某個郡這個月的災害機率。
//
// 「天災多因人怨引起，民眾忠誠最好不要太低」（說明書 p.36）——
// 忠誠 100 時降到基礎值的一半，忠誠 0 時是基礎值的兩倍。
//
// ⚠ **水災與瘟疫不走這一條**：兩者的判定都從碼讀出來了，而且形狀
// 完全不同（`FloodBase`／`FloodStrikes`、`PlagueStrikes`）。這裡只剩
// 地震與蝗害還在用它。
func (g *State) disasterChance(p *Prefecture) int {
	return TuneDisasterBase * (200 - int(p.PublicLoyalty)) / 200
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
			// 「越大體能越差」（說明書 p.18）：四十歲以後每年多掉一點。
			drop := TuneAgingStamina
			if x.Age > 40 {
				drop += int(x.Age-40) / 10
			}
			if int(x.Stamina) <= drop {
				x.Stamina = 0
				out = append(out, Event{x.Location, tf("ev.death", personName(x.Name))})
				g.retire(x)
				continue
			}
			x.Stamina -= uint8(drop)
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
	// **地震：每個春月 1/3 的機率，落在隨機一個郡**（`0x162d2`，`L0`）。
	// 不看民眾忠誠也不看土地價值——那道「天災多因人怨」的門檻是瘟疫的。
	if g.roll(int(Spring), 0, 10)%QuakeChance == 0 {
		id := g.roll(int(Spring), 0, 11)%state.PrefectureCount + 1
		if p := g.Prefecture(id); p != nil && p.Owned() {
			g.scale(p, 100-TuneQuakeLoss)
			out = append(out, Event{p.ID, tf("ev.quake", placeName(p.Name))})
		}
	}
	return out
}

// QuakeChance 是地震的機率分母（`0x162d2`：`RND(3) == 0`，`L0`）。
const QuakeChance = 3

// LandValueDecay 是土地價值每個春月的自然衰減（`0x15c96`，`L0`）：
//
//	土地價值 ← 土地價值 − RND(土地價值 ÷ 10)
//
// roll 是 `RND(土地價值 ÷ 10)`。**衰減與郡有沒有主無關**，
// 而且值越高掉越多——所以土地價值會停在「開發速度 ＝ 衰減速度」的地方，
// 不會一路到 100 就不動。
func LandValueDecay(landValue, roll int) int {
	v := landValue - roll
	if v < 0 {
		v = 0
	}
	return v
}

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
		if int(x.Age) < TuneComingOfAge {
			continue
		}
		at := x.Origin
		if at < 1 || at > len(g.prefectures) {
			at = x.Location
		}
		if at < 1 || at > len(g.prefectures) {
			continue
		}
		x.Location = at
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
			p.Population = p.Population * (100 - TuneFloodPopLoss) / 100
			g.scaleTroops(p.ID, 100-TuneFloodPopLoss)
			p.LandValue = uint8(clampTo(int(p.LandValue)-TuneFloodLandLoss, 100))
			// 「意外產生水災後，洪水率會立刻升到 100」（說明書 p.21）。
			p.FloodRate = 100
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
			p.Population = p.Population * (100 - TunePlagueLoss) / 100
			g.scaleTroops(p.ID, 100-TunePlagueLoss)
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
// harvestMonth 與 tributeMonth 是 **remake 挑的**：說明書只說秋收在秋天、
// 進貢每年一次，沒說是哪個月，原版那一邊也還沒量到——米糧每個月都被
// 電腦諸侯買賣，年度收成的尖峰埋在裡面看不出來
// （`docs/design/02-remake-owned-values.md`）。
// 物價的範圍。**量出來的**：十六個月 × 四十二個郡，值全部落在 30–68。
const (
	PriceMin    = 30
	PriceSpread = 19 // 兩個 0..19 相加 → 30..68
)

// priceSalt 讓物價的亂數與其他事件的亂數分開。
// 共用鹽的話「物價高的郡也比較容易鬧災」會憑空成立。
const priceSalt = 0x9E37

const (
	agingMonth   = 1
	harvestMonth = 9
	tributeMonth = 12
	growthMonth  = 10
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
		if g.roll(int(Autumn), p.ID) < g.disasterChance(p)/2 {
			p.Rice = p.Rice * (100 - TuneLocustRiceLoss) / 100
			p.LandValue = uint8(clampTo(int(p.LandValue)-TuneLocustLandLoss, 100))
			out = append(out, Event{p.ID, tf("ev.locust", placeName(p.Name))})
			continue
		}
		// 收成規模同時看土地價值與人口——人口是生產力的來源（說明書 p.20）。
		scale := p.Population / 1000
		rice := int(p.LandValue) * TuneHarvestRicePerLand * scale / 10
		gold := int(p.LandValue) * TuneHarvestGoldPerLand * scale / 10
		p.Rice = clampTo(p.Rice+rice, MaxRice)
		p.Gold = clampTo(p.Gold+gold, MaxGold)
		p.LandValue = uint8(clampTo(int(p.LandValue)-TuneHarvestLandDrop, 100))
		out = append(out, Event{p.ID,
			tf("ev.harvest", placeName(p.Name), rice, gold)})
	}
	return out
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
	if g.Date.Month != tributeMonth {
		return out
	}
	// 「各州郡每年進貢寶物給諸侯，領地越多，貢品越多」（說明書 p.37）。
	for i := range g.factions {
		f := &g.factions[i]
		if !f.Alive {
			continue
		}
		n := len(g.Territory(f.ID)) / TuneTributePerPrefecture
		for k := 0; k < n; k++ {
			// 玉璽不進貢——它只能諸侯持有，而且是勝利條件（說明書 p.24、p.37）。
			t := Treasure(1 + g.roll(int(f.ID), k)%int(treasureCount-1))
			f.Treasury[t]++
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

// scaleTroops 把一個郡所有駐軍的兵力按百分比縮放。
//
// **只動人不動郡**：郡的總兵力是導出值，動兩邊會讓它們分家。
func (g *State) scaleTroops(prefectureID, pct int) {
	for _, x := range g.Garrison(prefectureID) {
		x.Soldiers = x.Soldiers * pct / 100
	}
}

// scale 把一個郡的人口、金、米、兵按百分比縮放（災害用）。
func (g *State) scale(p *Prefecture, pct int) {
	p.Population = p.Population * pct / 100
	p.Gold = p.Gold * pct / 100
	p.Rice = p.Rice * pct / 100
	g.scaleTroops(p.ID, pct)
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
	if succ := g.successorFor(at, x.Index, faction); succ != nil {
		succ.Status = state.StatusGovernor
		if wasLord {
			// 君主死了由繼承者接位——「只有在繼承君主時才會改變等級」
			//（說明書 p.18）。
			succ.Status = state.StatusLord
			if f := g.Faction(faction); f != nil {
				f.Lord = succ.Index
			}
		}
		return
	}
	if p := g.Prefecture(at); p != nil {
		p.Owner = state.NoFaction
	}
	if f := g.Faction(faction); f != nil && len(g.Territory(faction)) == 0 {
		f.Alive = false
	}
}

// Winner 回報有沒有人一統天下。
//
// 條件是**擁有全部有主的郡**，而且**手上有玉璽**——
// 「在遊戲結束前一定要拿到玉璽」（說明書 p.37）。
// 沒有玉璽的話回傳 false 與那個勢力，讓呼叫端顯示「還要再等數月」。
func (g *State) Winner() (f state.FactionID, hasSeal bool, done bool) {
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
			return state.NoFaction, false, false
		}
	}
	if only == state.NoFaction {
		return state.NoFaction, false, false
	}
	x := g.Faction(only)
	return only, x != nil && x.Treasury[TreasureSeal] > 0, true
}
