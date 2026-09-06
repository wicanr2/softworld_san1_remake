package game

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 指令層。
//
// **每一個指令都回錯誤而不是靜靜地不做事。** 條件不滿足時（錢不夠、
// 這個月下過令了、人口不足）玩家要看得到理由；靜靜地忽略會讓
// 「按了沒反應」變成一個查不出來的問題。
//
// ⚠ **這裡只實作花費與前置條件已經有出處的指令。** 效果的公式
//（開墾加多少土地價值、徵兵的忠誠影響……）**還沒解**，
// 手冊只說「越高越好」不給數字。沒有出處的公式不要寫進來——
// 猜出來的數值玩起來「差不多」，而那是最難發現的錯。

// ErrNotYours 等是指令被擋下來的理由。用具名錯誤讓上層分得開，
// 不必比對字串。
var (
	ErrNotYours     = fmt.Errorf("這個郡不是你的")
	ErrAlreadyMoved = fmt.Errorf("這個郡這個月已經下過令了")
	ErrNoGold       = fmt.Errorf("庫銀不足")
	ErrNoPeople     = fmt.Errorf("人口不足，徵不到兵")
	ErrNoRoom       = fmt.Errorf("帶兵已達上限")
	ErrUnknownUnit  = fmt.Errorf("找不到這位將領")

	// ErrDeclined 是**成功執行了、但對方不從**。
	//
	// 它與上面那幾個不同類：那幾個是「這道命令不該下」，這一個是
	// 「命令下出去了，判定沒過」——錢照付、命令照用掉。
	// `ApplyAll` 因此不把它當成中斷的理由（見那裡的說明）。
	ErrDeclined = fmt.Errorf("對方婉拒了")
)

// canOrder 檢查「這個郡現在收不收指令」。
func (g *State) canOrder(prefectureID int, by state.FactionID) (*Prefecture, error) {
	p := g.Prefecture(prefectureID)
	if p == nil {
		return nil, fmt.Errorf("game: 郡編號 %d 越界", prefectureID)
	}
	if !p.Owned() || p.Owner != by {
		return nil, ErrNotYours
	}
	// **「每郡每月一道令」只管玩家**（說明書 p.17）。原版的分派器對
	// 每一個電腦的郡把十八張表全部跑一遍，同一個月同一個郡的地力、
	// 訓練度、身分、忠誠都會動（`Faction.ByComputer`）。
	if p.Commanded && !g.byComputer(by) {
		return nil, ErrAlreadyMoved
	}
	return p, nil
}

func (g *State) byComputer(id state.FactionID) bool {
	f := g.Faction(id)
	return f != nil && f.ByComputer
}

// aiLevelOf 是這個勢力的電腦等級；**玩家一律回 0**。
//
// 原版的等級參數表（登用的費用與加成、內政的範圍、訓練的除數）等級
// 0–2 是同一組，而那一組的登用費用 30 金正是說明書給玩家的價目——
// 所以玩家照 0 級那一組算（`L2`：玩家的常式沒有單獨讀過）。
func (g *State) aiLevelOf(id state.FactionID) int {
	f := g.Faction(id)
	if f == nil || !f.ByComputer {
		return 0
	}
	return f.AILevel
}

// price 是「這個勢力做這件事實際付多少」。
//
// **電腦諸侯有折扣**：原版所有 AI 的花費都走同一支常式（線性
// `0xec24`），金額 ＝ base × 係數[AI 等級]，係數表 `[1,1,1,1,0.9,0.75]`
// 是從記憶體讀出來的（`docs/mechanics/70-ai` §2.12，`L0`）。等級 5
// 量到 19 個樣本，每一個都與「×0.75 截斷」相符。
//
// 玩家不打折——原版那支常式只有分派器底下的行為會呼叫。
func (g *State) price(by state.FactionID, base int) int {
	f := g.Faction(by)
	if f == nil || !f.ByComputer {
		return base
	}
	return state.AICost(base, f.AILevel)
}

// Reclaim 是「土地開發」（說明書 p.21）：每次 10 金，
// **負責開墾的將領謀略越高，土地價值增加越多**。
//
// 「若財庫已空則徒手開墾」——所以錢不夠不是錯誤，只是效果減半。
func (g *State) Reclaim(prefectureID, generalIndex int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	fee := g.price(by, CostReclaim)
	add := ReclaimGain(0, g.Roll(2, prefectureID, 0xba02))
	if x := g.General(generalIndex); x != nil && x.Faction == by &&
		x.Location == prefectureID {
		add = ReclaimGain(int(x.Intel), g.Roll(2, prefectureID, 0xba02))
	}
	if p.Gold >= fee {
		p.Gold -= fee
	} else {
		add = (add + 1) / 2 // 徒手開墾
	}
	p.LandValue = uint8(clampTo(int(p.LandValue)+add, 100))
	p.Commanded = true
	return nil
}

// FloodControl 是「洪水防治」（說明書 p.21）：每次 10 金，
// **負責治水的將領謀略越高，洪水發生機率下降越多**。
// 「沒錢就不能修浚」——與開墾不同，這一項錢不夠就是失敗。
func (g *State) FloodControl(prefectureID, generalIndex int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	fee := g.price(by, CostFloodControl)
	if p.Gold < fee {
		return ErrNoGold
	}
	drop := 0
	if x := g.General(generalIndex); x != nil && x.Faction == by &&
		x.Location == prefectureID {
		drop = FloodDrop(int(x.Intel))
	}
	p.Gold -= fee
	p.FloodRate = uint8(clampTo(int(p.FloodRate)-drop, 100))
	p.Commanded = true
	return nil
}

// Conscript 是「徵兵」（說明書 p.20）：一位將領募兵，每人 1 金。
//
// 條件都有出處：
//
//   - 每人 1 金
//   - 帶兵不得超過官階上限（p.18，而且與原版資料對得上）
//   - **每州郡不得因徵兵而導致人口少於 3000 人**——手冊這樣寫，
//     所以徵兵**會減少人口**，減的量就是募到的人數
//   - 新兵毫無訓練，加入時會把部隊的訓練度拉低，武裝度也會降低
//
// ⚠ **人口在原版檔案裡是以百為單位存的**（`docs/spec/003` §3）。
// 局面裡存的是實際值，所以徵一百五十人減得掉；要存回原版格式時
// 才需要處理進位，那是存檔的事。
func (g *State) Conscript(prefectureID, generalIndex, n int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if n <= 0 {
		return fmt.Errorf("game: 徵兵人數要是正數，拿到 %d", n)
	}
	if p.Population-n < MinPopulationToConscript {
		return ErrNoPeople
	}
	x := g.General(generalIndex)
	if x == nil || x.Faction != by || x.Location != prefectureID {
		return ErrUnknownUnit
	}
	cost := g.price(by, n*CostConscriptPerSoldier)
	if p.Gold < cost {
		return ErrNoGold
	}
	if x.Soldiers+n > x.TroopCap() {
		return ErrNoRoom
	}
	// 新兵沒有武器也沒受過訓，加入之後**同一批武器攤在更多人頭上**，
	// 武裝度自然下降（`ArmsOf`／`Weapons`，`rules.go`）。訓練度同理。
	total := x.Soldiers + n
	x.Training = uint8((int(x.Training)*x.Soldiers + TuneNewSoldierTraining*n) / total)
	x.Arms = uint8(ArmsOf(Weapons(int(x.Arms), x.Soldiers), total))

	p.Gold -= cost
	p.Population -= n
	x.Soldiers = total
	p.Commanded = true
	return nil
}

// BuyArms 是「武器」：每 100 單位 1 金（說明書 p.20），提升武裝度。
//
// **價格是截斷除法**（`L0`、`0xc168` ＋ `0xec24`）：買 250 單位收 2 金，
// 買 50 單位不用錢。原版的電腦諸侯就是這樣買的——它每次補到滿編，
// 補的量不是 100 的倍數。這裡不擋不足 100 的零頭，是照原版的算術；
// 玩家介面另外只給 100 的倍數選。
func (g *State) BuyArms(prefectureID, generalIndex, units int, by state.FactionID) error {
	p, err := g.canOrder(prefectureID, by)
	if err != nil {
		return err
	}
	if units <= 0 {
		return fmt.Errorf("game: 武器數要是正數，拿到 %d", units)
	}
	x := g.General(generalIndex)
	if x == nil || x.Faction != by || x.Location != prefectureID {
		return ErrUnknownUnit
	}
	cost := g.price(by, units/100*CostArmsPer100)
	if p.Gold < cost {
		return ErrNoGold
	}
	p.Gold -= cost
	x.Arms = uint8(ArmsAfterPurchase(int(x.Arms), x.Soldiers, units))
	p.Commanded = true
	return nil
}

// EndMonth 把時間推到下個月，清掉每月一次的旗標，然後跑季節事件。
// 回傳這個月發生了什麼（`events.go`）。
func (g *State) EndMonth() []Event {
	g.Date = g.Date.Next()
	for i := range g.prefectures {
		g.prefectures[i].Commanded = false
	}
	// 賞賜是每月每人一次（說明書 p.23）。
	for i := range g.generals {
		g.generals[i].Rewarded = false
	}
	g.repriceAll()
	return g.RunSeason()
}
