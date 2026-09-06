package game

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// Date 是遊戲內的時間。原版以年號顯示，可切西曆／中曆（說明書 p.26）。
//
// **內部一律存西元年**：年號是顯示層的事，而年號表跨越漢末到三國，
// 用它當內部時間會讓「下個月」這種基本運算變成查表。
type Date struct {
	Year  int // 西元
	Month int // 1..12
}

// Next 回傳下一個月。
func (d Date) Next() Date {
	if d.Month >= 12 {
		return Date{Year: d.Year + 1, Month: 1}
	}
	return Date{Year: d.Year, Month: d.Month + 1}
}

// Season 回傳季節。原版的特殊事件按季節發生（說明書 p.36–37）。
//
// **分界照農曆不照西曆**：正月就是春天。原版主畫面左側直排寫年月與季節，
// 三張畫面對出來的是元月春、四月夏、八月秋（`L1`、`[base]`，
// `docs/mechanics/50-events.md` §1）。
//
// 西曆式的「三月才入春」會讓每一個季節事件晚兩個月發生——秋收、洪水、
// 冬季人口成長全部錯位，而畫面上看起來只是「今年收成比較晚」。
func (d Date) Season() Season {
	switch {
	case d.Month <= 3:
		return Spring
	case d.Month <= 6:
		return Summer
	case d.Month <= 9:
		return Autumn
	default:
		return Winter
	}
}

// Season 是季節。
type Season int

const (
	Spring Season = iota
	Summer
	Autumn
	Winter
)

// ScenarioStart 是六個劇本的起始年月。
//
// 出處是原版的「選擇年代」畫面：中平六年／興平二年／建安六年／
// 建安十三年／建安二十年／黃初元年，換算成西元 189／195／201／208／
// 215／220。原版主畫面左側直排寫「中平六年元月」，所以起始月是元月。
var ScenarioStart = map[state.Slot]Date{
	state.Scenario1: {Year: 189, Month: 1},
	state.Scenario2: {Year: 195, Month: 1},
	state.Scenario3: {Year: 201, Month: 1},
	state.Scenario4: {Year: 208, Month: 1},
	state.Scenario5: {Year: 215, Month: 1},
	state.Scenario6: {Year: 220, Month: 1},
}

// Prefecture 是一局裡的一個郡。欄位意義見 `docs/spec/003` §3。
type Prefecture struct {
	ID   int
	Name string

	Owner state.FactionID

	// Population 是**實際值**，不是原版存的 ÷100。
	// 換算在載入的唯一入口做完，規則層不再碰那個倍率。
	Population int

	// ⚠ **總兵力不存在這裡。** 手冊說它是「所有現役將麾下的兵力總合」
	// （p.17），而原版的檔案也的確如此——四十二個郡逐一驗過，
	// 郡的兵士欄與駐軍加總完全相等。存一份副本就會有兩個真相，
	// 而災害的百分比縮放會讓它們慢慢分家。要數請用 State.Soldiers。

	Gold int
	Rice int

	PublicLoyalty uint8
	LandValue     uint8
	FloodRate     uint8
	PriceLevel    uint8

	Neighbours []int

	// ⚠ **現役／在野武將數不存在這裡。** 原版的檔案有那兩欄，
	// 而且我們驗過它與人物表逐郡吻合——但那是初始值。開始下令之後
	// 存一份副本就會有兩個真相。要數請用 State.ActiveGenerals／FreeGenerals。

	// Forts 是城寨數，每郡最多五座（說明書 p.21，不含城池）。
	Forts int

	// Autonomy 是郡縣自治的型態（說明書 p.23–24）。
	Autonomy Autonomy

	// Commanded 記這個月下過令了沒。原版是每郡每月一次（說明書 p.17）。
	Commanded bool
}

// Autonomy 是郡縣自治的型態（說明書 p.23–24）。
type Autonomy int

const (
	// AutoNormal 由諸侯下令，太守擇人施行。
	AutoNormal   Autonomy = iota
	AutoCivil             // 內政：太守專心處理州郡內政
	AutoMilitary          // 軍事：太守全力加強軍事力量
	AutoSelf              // 自治：依太守的能力決定型態
)

// String 讓型態印得出中文。
func (a Autonomy) String() string {
	switch a {
	case AutoCivil:
		return "內政"
	case AutoMilitary:
		return "軍事"
	case AutoSelf:
		return "自治"
	}
	return "正常"
}

// Owned 回報這個郡有沒有主。
func (p *Prefecture) Owned() bool { return p.Owner != state.NoFaction }

// General 是一局裡的一個人物。欄位意義見 `docs/spec/003` §2。
type General struct {
	Index int
	Name  string

	Age      uint8
	Stamina  uint8
	Intel    uint8
	War      uint8
	Charm    uint8
	Rank     state.Rank
	Origin   int // 出身郡（1..42），未登場者從這裡登場
	Loyalty  uint8
	Status   state.Status
	Faction  state.FactionID
	Location int
	Troop    state.TroopType
	Soldiers int
	Training uint8
	Arms     uint8

	// Rewarded 記這個月被賞賜過了沒。原版是每郡每月可賞每人一次
	// （說明書 p.23）。
	Rewarded bool
}

// Employed 回報這位人物有沒有效力對象。
func (g *General) Employed() bool { return g.Faction != state.NoFaction }

// HasLoyalty 回報忠誠欄有沒有意義（在野者沒有，原版存 0xFF 哨兵）。
func (g *General) HasLoyalty() bool { return g.Loyalty != state.NoValue }

// TroopCap 是這位人物帶得動的最大兵力。
func (g *General) TroopCap() int { return TroopCap(g.Rank) }

// Treasure 是君主寶庫裡的一件寶物（說明書 p.24）。
type Treasure int

const (
	TreasureSeal   Treasure = iota // 玉璽：只能諸侯持有，不能送人
	TreasureBook                   // 兵書：謀略 +2
	TreasureBlade                  // 寶刀：戰力 +3
	TreasureBeauty                 // 美女：魅力 +5
	TreasureHorse                  // 駿馬：戰力 +2、魅力 +3
	treasureCount
)

// String 讓寶物印得出中文。
func (t Treasure) String() string {
	switch t {
	case TreasureSeal:
		return "玉璽"
	case TreasureBook:
		return "兵書"
	case TreasureBlade:
		return "寶刀"
	case TreasureBeauty:
		return "美女"
	case TreasureHorse:
		return "駿馬"
	}
	return "?"
}

// Faction 是一個勢力。
type Faction struct {
	ID state.FactionID

	// Lord 是君主在 Generals 裡的索引。
	Lord int

	// Alive 為假表示這個勢力已經沒有領地了。
	Alive bool

	// AILevel 是原版的電腦諸侯等級，0–5（`BASEMAS` offset 4，`L0`）。
	//
	// **它挑的是一整套行為**：原版有九張指令分派表，每一張八個項目，
	// 用這個等級當索引（`docs/re/03` §1.4）。已經解出來的是訓練兵士
	// 那一張——等級越高除數越小、練得越快（`AITrainDivisor`）。
	AILevel int

	// Prestige 是人望（`BASEMAS` offset 8，`L1`）。原版的郡資訊欄
	// 顯示「君主〇〇〇　人望 %d」，用途還沒解。
	Prestige int

	// Treasury 是君主寶庫裡各種寶物的數量（說明書 p.19「君主物品」）。
	//
	// 開局內容從盤面讀：`BASEMAS` offset 14–18（`state.TreasuryOf`）。
	// offset 14 是玉璽——十六個槽裡只有一個是 1，其餘全 0。
	// **15–18 之間誰是誰還沒驗**（`L3`）。
	Treasury [treasureCount]int

	// Chief 是現任軍師的人物槽號，−1 表示沒有。
	// 同時只能有一位（說明書 p.23）。
	Chief int
}

// State 是一局進行中的遊戲。
//
// ⚠ **郡用 1-based 編號**（與原版一致），所以不要直接對 Prefectures 取索引，
// 用 Prefecture(id)。原版的第 0 筆是啞元，把它當成一個郡會拿到名字是
// "...." 的東西然後照樣算下去。
type State struct {
	Slot state.Slot
	Date Date

	// Player 是玩家控制的勢力；NoFaction 表示純觀戰。
	Player state.FactionID

	// Difficulty 是難度 1–10（原版開局時問「請設定難度(1-10)」）。
	Difficulty int

	// Options 是「其他」底下的開關（`options.go`）。
	Options Options

	// Reports 是還沒被讀走的戰報，舊的在前面。
	//
	// 戰役由命令層觸發（`AttackOrder.Apply` 只回錯誤），而電腦諸侯的
	// 戰役玩家根本沒經手——**沒有這個佇列，三十天的主戰場就只有
	// 「某某出兵攻某某」一行**。呼叫端用 `DrainReports` 取走。
	Reports []*BattleResult

	prefectures []Prefecture // 索引 0 是郡 1
	generals    []General
	factions    []Faction

	// 開局時三張表的原始位元組。存檔要用它保住還沒解出來的欄位（tables.go）。
	rawMas, rawSta, rawGen []byte
}

// DrainReports 取走並清空累積的戰報。
func (g *State) DrainReports() []*BattleResult {
	out := g.Reports
	g.Reports = nil
	return out
}

// New 從一個劇本開一局。
//
// player 要是實際在用的勢力，否則回錯誤——**不要默默改成 0**：
// 「玩家其實在控制別人」這種錯誤在畫面上完全看不出來。
func New(sc *state.Scenario, player state.FactionID, difficulty int) (*State, error) {
	start, ok := ScenarioStart[sc.Slot]
	if !ok {
		return nil, fmt.Errorf("game: 不知道槽位 %q 的起始年月", sc.Slot)
	}
	if difficulty < 1 || difficulty > 10 {
		return nil, fmt.Errorf("game: 難度 %d 越界（原版收 1..10）", difficulty)
	}

	g := &State{Slot: sc.Slot, Date: start, Player: player, Difficulty: difficulty}
	g.rawMas, g.rawSta, g.rawGen = sc.Tables()

	for _, p := range sc.Prefectures() {
		g.prefectures = append(g.prefectures, Prefecture{
			ID: p.ID, Name: p.Name,
			Owner:      state.FactionID(p.Owner),
			Population: p.People(),
			Gold:       int(p.Gold), Rice: int(p.Rice),
			PublicLoyalty: p.PublicLoyalty, LandValue: p.LandValue,
			FloodRate: p.FloodRate, PriceLevel: p.PriceLevel,
			Neighbours: append([]int(nil), p.Neighbours...),
		})
	}
	for _, s := range sc.Generals() {
		g.generals = append(g.generals, General{
			Index: s.Index, Name: s.Name,
			Age: s.Age, Stamina: s.Stamina, Intel: s.Intel, War: s.War, Charm: s.Charm,
			Rank: s.Rank, Origin: int(s.Origin), Loyalty: s.Loyalty, Status: s.Status,
			Faction: state.FactionID(s.Faction), Location: int(s.Location),
			Troop: s.Troop, Soldiers: int(s.Soldiers),
			Training: s.Training, Arms: s.Arms,
		})
	}

	valid := false
	for _, f := range sc.ActiveFactions() {
		lord, err := sc.Lord(f)
		if err != nil {
			return nil, err
		}
		fa := Faction{ID: state.FactionID(f), Lord: lord.Index, Alive: true, Chief: -1,
			AILevel: sc.AILevel(f), Prestige: sc.Prestige(f)}
		for i, n := range sc.TreasuryOf(f) {
			if i < len(fa.Treasury) {
				fa.Treasury[i] = n
			}
		}
		for _, x := range sc.Retinue(f) {
			if x.Status == state.StatusChief {
				fa.Chief = x.Index
			}
		}
		g.factions = append(g.factions, fa)
		if state.FactionID(f) == player {
			valid = true
		}
	}
	if player != state.NoFaction && !valid {
		return nil, fmt.Errorf("game: 勢力 %d 在劇本 %s 裡沒有在用", player, sc.Slot)
	}
	return g, nil
}

// Prefecture 用 1-based 編號取一個郡。越界回 nil。
func (g *State) Prefecture(id int) *Prefecture {
	if id < 1 || id > len(g.prefectures) {
		return nil
	}
	return &g.prefectures[id-1]
}

// Prefectures 回傳全部的郡。
func (g *State) Prefectures() []Prefecture { return g.prefectures }

// General 用槽號取一個人物。越界回 nil。
func (g *State) General(index int) *General {
	if index < 0 || index >= len(g.generals) {
		return nil
	}
	return &g.generals[index]
}

// Factions 回傳實際在用的勢力。
func (g *State) Factions() []Faction { return g.factions }

// Faction 用勢力槽號取一個勢力。找不到回 nil。
func (g *State) Faction(f state.FactionID) *Faction {
	for i := range g.factions {
		if g.factions[i].ID == f {
			return &g.factions[i]
		}
	}
	return nil
}

// Chief 回傳某個勢力現任的軍師；沒有回 nil。
// AILevel 回傳勢力的電腦諸侯等級（0–5）。查不到的勢力回 0。
func (g *State) AILevel(f state.FactionID) int {
	if x := g.Faction(f); x != nil {
		return x.AILevel
	}
	return 0
}

func (g *State) Chief(f state.FactionID) *General {
	x := g.Faction(f)
	if x == nil || x.Chief < 0 {
		return nil
	}
	return g.General(x.Chief)
}

// Lord 回傳某個勢力的君主。
func (g *State) Lord(f state.FactionID) *General {
	for _, x := range g.factions {
		if x.ID == f {
			return g.General(x.Lord)
		}
	}
	return nil
}

// Territory 回傳某個勢力擁有的郡編號，依編號排序。
func (g *State) Territory(f state.FactionID) []int {
	var out []int
	for i := range g.prefectures {
		if g.prefectures[i].Owner == f {
			out = append(out, g.prefectures[i].ID)
		}
	}
	return out
}

// Governor 回傳某個郡的主事者：君主或太守，兩者都不在時由軍師代理
// （`docs/spec/003` §2.2）。找不到唯一的一位回 nil。
func (g *State) Governor(prefectureID int) *General {
	p := g.Prefecture(prefectureID)
	if p == nil || !p.Owned() {
		return nil
	}
	var main, deputy []int
	for i := range g.generals {
		x := &g.generals[i]
		if x.Faction != p.Owner || x.Location != prefectureID {
			continue
		}
		switch {
		case x.Status.Governs():
			main = append(main, i)
		case x.Status == state.StatusChief:
			deputy = append(deputy, i)
		}
	}
	if len(main) == 1 {
		return &g.generals[main[0]]
	}
	if len(main) == 0 && len(deputy) == 1 {
		return &g.generals[deputy[0]]
	}
	return nil
}

// Garrison 回傳駐在某個郡的現役武將（含主事者）。
func (g *State) Garrison(prefectureID int) []*General {
	var out []*General
	for i := range g.generals {
		x := &g.generals[i]
		if x.Employed() && x.Location == prefectureID {
			out = append(out, x)
		}
	}
	return out
}

// ActiveGenerals 是駐在某個郡的現役武將數（含主事者）。
//
// **算出來不存起來。** 這個數字在原版的檔案裡有一欄，而且我們驗過
// 它與人物表逐郡吻合（`docs/spec/003` §3.1）——但那是**初始值**。
// 開始下令之後，存一份副本就會有兩個真相，而它們遲早會不一致。
func (g *State) ActiveGenerals(prefectureID int) int {
	n := 0
	for i := range g.generals {
		x := &g.generals[i]
		if x.Employed() && x.Location == prefectureID && x.Name != "" {
			n++
		}
	}
	return n
}

// FreeGenerals 是某個郡露面的在野武將數。
//
// 只數身分為「在野而且在該郡露面」的人——原版的郡欄位就是這樣數的
// （四十二個郡全對；不加這個條件只對得上二十六個）。
func (g *State) FreeGenerals(prefectureID int) int {
	n := 0
	for i := range g.generals {
		x := &g.generals[i]
		if !x.Employed() && x.Location == prefectureID &&
			x.Status == state.StatusAvailable && x.Name != "" {
			n++
		}
	}
	return n
}

// Soldiers 是某個郡的總兵力：駐軍麾下兵力的總合。
//
// 手冊定義如此（p.17），原版的檔案也是——四十二個郡逐一驗過，
// 郡的兵士欄與駐軍加總完全相等（`docs/spec/003` §3）。
func (g *State) Soldiers(prefectureID int) int {
	n := 0
	for i := range g.generals {
		x := &g.generals[i]
		if x.Employed() && x.Location == prefectureID {
			n += x.Soldiers
		}
	}
	return n
}

// Free 回傳某個郡露面的在野武將。
func (g *State) Free(prefectureID int) []*General {
	var out []*General
	for i := range g.generals {
		x := &g.generals[i]
		if !x.Employed() && x.Location == prefectureID &&
			x.Status == state.StatusAvailable && x.Name != "" {
			out = append(out, x)
		}
	}
	return out
}

// Adjacent 回報兩個郡相不相鄰。
func (g *State) Adjacent(a, b int) bool {
	p := g.Prefecture(a)
	if p == nil {
		return false
	}
	for _, n := range p.Neighbours {
		if n == b {
			return true
		}
	}
	return false
}
