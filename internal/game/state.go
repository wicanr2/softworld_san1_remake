package game

import (
	"fmt"
	"sort"

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

	// Province 是這個郡屬於哪一州（原版州郡 offset 5，0–13）。
	Province uint8

	// MapX／MapY 是這個郡在大地圖上的座標（原版州郡 offset 6／8）。
	// 畫面上的位置是 `MapX + 0x50`、`MapY + 0x2c`（`0x10ceb`／`0x10cf4`）；
	// 主畫面的州郡填色就是從那一點灌下去的。
	MapX, MapY int

	// governor 是主事者的人物槽號，−1 ＝ 沒有（原版州郡 offset 32，
	// `docs/spec/003` §3.4）。
	//
	// **它是狀態不是導出值**：君主與太守同一個郡時，誰主事只有當初的
	// 任命說得準，從駐軍名單推不出來——那條路推過兩個版本，兩個都被
	// 資料推翻。用 Governor() 讀，別直接碰。
	governor int

	// BattleField 是這個郡的戰場地圖，原版州郡記錄 offset 55–174
	// 的 120 個位元組原樣搬過來（`internal/battle`.Load 解讀）。
	BattleField []byte

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

	Age     uint8
	Stamina uint8
	Intel   uint8
	War     uint8
	Charm   uint8
	Rank    state.Rank
	Origin  int // 出身郡（1..42），未登場者從這裡登場

	// Bond 是人物表 offset 14 指到的人物槽號，登用判定拿它當閘門
	//（`state.General.Bond`）。指向自己表示沒有牽絆。
	Bond int

	// Debut 是出頭的年齡（人物表 offset 26，`L0`）。
	Debut uint8

	// Portrait 是肖像編號（原版人物 offset 27）；Lifespan 是壽命
	// （offset 28）——**幾歲開始走下坡，不是幾歲一定死**（`AgingDrop`）。
	Portrait uint8
	Lifespan uint8
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
	// **列舉順序就是諸侯記錄 offset 14–18 的順序**（`L0`，
	// `state.TreasuryOf`）：進貢的迴圈用同一個索引寫欄位與挑標籤。
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
	// **它挑的是一整套行為**：原版有十八張指令分派表，每一張八個項目，
	// 用這個等級當索引（`docs/re/03` §1.4）。已經解出來的是訓練兵士
	// 那一張——等級越高除數越小、練得越快（`AITrainDivisor`）。
	AILevel int

	// Prestige 是人望（`BASEMAS` offset 8，`u16`，範圍 0–100，`L0`）。
	//
	// **它每年決定部下忠誠的漲跌**（`0x15fc2`）：
	// `忠誠 += (人望 − 60) ÷ 2`，只對軍師／太守／一般武將，君主自己不算。
	// 所以 60 是分水嶺——人望不到 60 的諸侯，部下每年都在離心。
	//
	// 動它的地方：戰役勝方 +2、敗方 −2（`0x204b4`／`0x204e0`），
	// 以及春季的一個隨機事件 `+RND(30) + 40`（`0x1519a`，前置條件未解）。
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

	// ByComputer 是「這個勢力由電腦操作」。
	//
	// **來源不是盤面的 offset 0**（`state.Controller`）：那一格記的是
	// 開局選單當時誰選了誰，而 remake 這一邊由 `New` 的 `player`
	// 參數決定，其餘全部是電腦。
	//
	// **它不只是標籤，會改規則**：說明書 p.17 的「每郡每月只能下一道令」
	// 是玩家的限制，電腦諸侯不受它管——原版的分派器對每一個電腦的郡
	// 把十八張表**全部跑一遍**（掛了其中九個分派點量到一個月 288 次，
	// ÷ 32 個郡 ＝ 每郡九次，也就是掛幾個就量到幾次），
	// 而且同一個月裡同一個郡的地力、訓練度、身分、忠誠都動過
	// （`docs/re/03` §1.4，`L1`）。
	ByComputer bool
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

	// Edition 是原版還是加強版（`docs/spec/004`）。
	Edition state.Edition

	// Difficulty 是難度。上限看版本：原版問「請設定難度(1-10)」，
	// 加強版問「請設定難度(1-20)」。
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

	// turnOrder 是**下個月**的郡順序，由 `EndMonth` 的開月那一段洗出來
	// （`0x17364`）。原版把它連同旗標寫進進度檔，所以它是盤面的一部分，
	// 不是 session 的暫存。
	turnOrder []int

	// randSeed 是原版那條 LCG 的狀態（`MSCRand`）。`randOn` 為真時
	// `roll`／`Roll` 改抽這一條，抽幾次記在 `randDraws`。
	//
	// **預設不開**：對齊序列要求每一處消耗的次數與順序都跟原版一樣，
	// 那是逐支常式要驗的事（`CONTEXT.md` 的「亂數對齊」）。沒對齊就開，
	// 只會把「與局面綁定、可重播」換成「與原版無關、也不可讀」。
	randSeed  uint32
	randDraws int
	randOn    bool

	// phaseTrace 非 nil 時，換月的每一段各抽了幾次會記進去（對拍用）。
	phaseTrace map[string]int
	phaseSeed  map[string]uint32
	phaseLast  int
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
//
// ed 空字串當原版。難度的上限跟著版本走（`docs/spec/004`）：
// **拿加強版的難度 15 去開原版不會是「比較難」，是規則接錯了**——
// 原版的係數表只有十格，第十五格是表外的位元組。
func New(sc *state.Scenario, player state.FactionID, difficulty int, ed state.Edition) (*State, error) {
	start, ok := ScenarioStart[sc.Slot]
	if !ok {
		return nil, fmt.Errorf("game: 不知道槽位 %q 的起始年月", sc.Slot)
	}
	return newAt(sc, player, difficulty, ed, start)
}

// newAt 是 New 去掉「起始年月要查得到」那一條。
//
// **讀進度時年月來自進度本身**，起始年月當場就被蓋掉；而原版存的進度
// 不記得自己是從哪個劇本開始的（`docs/re/08`），拿劇本表去查一定落空。
// 落空就擋下來的話，玩家自己的原版存檔一份都讀不進來。
func newAt(sc *state.Scenario, player state.FactionID, difficulty int,
	ed state.Edition, start Date) (*State, error) {
	if ed == "" {
		ed = state.EditionBase
	}
	if !ed.Valid() {
		return nil, fmt.Errorf("game: 不認識的版本 %q", ed)
	}
	if difficulty < 1 || difficulty > ed.MaxDifficulty() {
		return nil, fmt.Errorf("game: 難度 %d 越界（%s 收 1..%d）",
			difficulty, ed, ed.MaxDifficulty())
	}

	g := &State{Slot: sc.Slot, Date: start, Player: player,
		Edition: ed, Difficulty: difficulty}
	g.rawMas, g.rawSta, g.rawGen = sc.Tables()

	for _, p := range sc.Prefectures() {
		g.prefectures = append(g.prefectures, Prefecture{
			ID: p.ID, Name: p.Name,
			Owner: state.FactionID(p.Owner),
			MapX:  int(p.MapX), MapY: int(p.MapY),
			Population: p.People(),
			Gold:       int(p.Gold), Rice: int(p.Rice),
			PublicLoyalty: p.PublicLoyalty, LandValue: p.LandValue,
			FloodRate: p.FloodRate, PriceLevel: p.PriceLevel,
			Forts:       int(p.Forts),
			Autonomy:    Autonomy(p.Autonomy),
			governor:    governorSlot(p.Governor),
			Neighbours:  append([]int(nil), p.Neighbours...),
			BattleField: append([]byte(nil), p.BattleField...),
		})
	}
	for _, s := range sc.Generals() {
		g.generals = append(g.generals, General{
			Index: s.Index, Name: s.Name,
			Age: s.Age, Stamina: s.Stamina, Intel: s.Intel, War: s.War, Charm: s.Charm,
			Rank: s.Rank, Origin: int(s.Origin), Bond: s.Bond, Debut: s.Debut,
			Portrait: s.Portrait, Lifespan: s.Lifespan,
			Loyalty: s.Loyalty, Status: s.Status,
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
			AILevel: sc.AILevel(f), Prestige: sc.Prestige(f),
			ByComputer: state.FactionID(f) != player}
		for i, n := range sc.TreasuryOf(f) {
			if i < len(fa.Treasury) {
				fa.Treasury[i] = n
			}
		}
		// **軍師讀諸侯 offset 6 的存值，不從人物表重推**（`0xd7ae` 讀的
		// 就是那一格）。掃 `Retinue` 找身分 1 會漏掉**別的勢力的軍師**
		// ——月度對拍量到勢力 4 的軍師槽指向徐庶，而徐庶是勢力 5 的人。
		// 漏掉的後果是換軍師的門檻由「現任的智」掉回 79，於是每個月都
		// 換一次人（`docs/re/07` §7 的同一個形狀：存值不重推）。
		if c := sc.ChiefIndex(f); c >= 0 && c < len(g.generals) {
			fa.Chief = c
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
// RecomputeOwners 把每個郡的所屬勢力從人物表重算一次（`L0`、`0x1e394`）。
//
// **郡的歸屬是導出值，不是獨立的狀態。** 原版在每一個郡的回合入口
// （`0x17471`）都跑一次這件事：先把全部的郡設成無主，再掃全部人物，
// 把每一位有主的武將所在的郡標成他的勢力。
//
// **同一個郡裡有兩方的人時，人物槽號較大的那位說了算**——後寫的蓋掉
// 先寫的。這就是原版電腦諸侯「出兵」之後郡易主的機制：搬進去的部隊
// 不打仗，下一輪重算歸屬就換人（`docs/mechanics/70-ai` §2.13.6）。
func (g *State) RecomputeOwners() {
	for i := range g.prefectures {
		g.prefectures[i].Owner = state.NoFaction
	}
	for i := range g.generals {
		x := &g.generals[i]
		if !x.Employed() {
			continue
		}
		if p := g.Prefecture(x.Location); p != nil {
			p.Owner = x.Faction
		}
	}
}

// AllGenerals 是整張人物表，照槽號順序。
//
// **原版有好幾支常式掃全表**（挖角挑人 `0xe0bc` 掃 350 筆），
// 而 remake 這一邊沒有別的路走得到「不在自己地盤上的人」。
func (g *State) AllGenerals() []*General {
	out := make([]*General, 0, len(g.generals))
	for i := range g.generals {
		out = append(out, &g.generals[i])
	}
	return out
}

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

// AILevel 回傳勢力的電腦諸侯等級（0–5）。查不到的勢力回 0。
func (g *State) AILevel(f state.FactionID) int {
	if x := g.Faction(f); x != nil {
		return x.AILevel
	}
	return 0
}

// Chief 回傳某個勢力現任的軍師；沒有回 nil。
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

// ShuffleTurnOrder 洗這個月的郡順序（`0x17371`–`0x1740a`，`L0`）。
//
// 順序表填成 0..42，接著**洗五輪**，每輪逐格與 `RND(43)` 交換。
//
// ⚠ **它會消耗亂數**（5 × 43 ＝ 215 次）。原版的序列裡有這一段，所以
// 即使呼叫端不打算用回傳值也要跑，否則接下來的每一次抽樣都錯開
// （`CONTEXT.md` 的亂數路線圖：原版「郡回合之外」414 次裡有 215 次是它）。
// TurnOrder 是開月時洗出來的郡順序（43 格）。還沒開過月時是空的。
func (g *State) TurnOrder() []int {
	return g.turnOrder
}

func (g *State) ShuffleTurnOrder() []int {
	// 43 格：州郡表的筆數，第 0 筆是啞元（`docs/formats/03`）。
	const slots = 43
	order := make([]int, slots)
	for i := range order {
		order[i] = i
	}
	for round := 0; round < 5; round++ {
		for i := range order {
			j := g.Roll(slots, round, i, 0x17371)
			order[i], order[j] = order[j], order[i]
		}
	}
	return order
}

// Governor 回傳某個郡的主事者：君主或太守，兩者都不在時由軍師代理
// （`docs/spec/003` §2.2）。找不到唯一的一位回 nil。
func (g *State) Governor(prefectureID int) *General {
	p := g.Prefecture(prefectureID)
	if p == nil || !p.Owned() {
		return nil
	}
	// **記著的那一位優先**：原版把主事者存在州郡 offset 32，開局之後
	// 十三處會改它。他只要還在職、還在這個郡，就是他主事——君主與太守
	// 同一個郡時，這是唯一問得出答案的地方。
	//
	// **不比對勢力**：原版的主事者可以是別的勢力的人。月度對拍量到的
	// 一例是郡 13（勢力 4）的主事者為荀彧（勢力 5）——原版整輪跑完
	// 不動它，混編的郡本來就是這樣（`docs/re/07` §6：名單不比對勢力）。
	// 易主由另外兩個條件擋：被俘、陣亡與撤走都會讓他落掉。
	if x := g.General(p.governor); x != nil && x.Employed() &&
		x.Location == prefectureID {
		return x
	}
	// 記著的那一位不在了（陣亡、被俘、調走、郡易主）就重新指派，
	// 並且**把結果記回去**——否則每次都要重推，而推論在君主與太守
	// 同郡時給不出唯一解。
	x := g.pickGovernor(prefectureID, p.Owner)
	if x == nil {
		p.governor = -1
		return nil
	}
	p.governor = x.Index
	return x
}

// pickGovernor 是主事者失聯之後的重新指派：君主優先，其次太守，
// 再其次軍師，**都沒有就從一般武將裡挑一位升任太守**。
//
// 最後那一段不能省：郡易主或主事者陣亡之後，留在郡裡的常常只剩
// 一般武將。少了它，那個郡有主卻沒有人管——而畫面上只看得出
// 「太守欄是空的」，不會有任何錯誤。
func (g *State) pickGovernor(prefectureID int, owner state.FactionID) *General {
	var lord, main, deputy, officer []int
	for i := range g.generals {
		x := &g.generals[i]
		if x.Faction != owner || x.Location != prefectureID || !x.Employed() {
			continue
		}
		switch {
		case x.Status == state.StatusLord:
			lord = append(lord, i)
		case x.Status.Governs():
			main = append(main, i)
		case x.Status == state.StatusChief:
			deputy = append(deputy, i)
		case x.Status == state.StatusOfficer:
			officer = append(officer, i)
		}
	}
	// 一般武將要照魅力挑，與原版指定太守那張表同一個排序鍵
	// （`docs/mechanics/70-ai` §2.6）。
	sort.SliceStable(officer, func(a, b int) bool {
		return g.generals[officer[a]].Charm > g.generals[officer[b]].Charm
	})
	// **君主在場就是君主主事**，郡裡同時有太守是正常的——君主會巡狩，
	// 也會親征經過自己的郡。
	for _, pool := range [][]int{lord, main, deputy} {
		if len(pool) > 0 {
			return &g.generals[pool[0]]
		}
	}
	if len(officer) > 0 {
		x := &g.generals[officer[0]]
		x.Status = state.StatusGovernor // 升任，否則郡照樣沒有人主事
		return x
	}
	return nil
}

// governorSlot 把原版的 0xFFFF 哨兵換成 −1（`CLAUDE.md` §7 第 11 條：
// 哨兵在唯一入口正規化，不讓它當成數值流進規則層）。
func governorSlot(v uint16) int {
	if v == state.NoValue16 {
		return -1
	}
	return int(v)
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
// Recruitable 是本郡可以被登用的人，**照槽號由小到大**。
//
// 原版的條件只有兩個（`0xd0ae`）：所在郡相符、身分 ∈ {8 在野露面,
// 10 失去勢力}。它不看名字也不看別的——`Free` 那一份多了
// 「算進郡的在野武將數」的限制，兩者不是同一個集合。
func (g *State) Recruitable(prefectureID int) []*General {
	var out []*General
	for i := range g.generals {
		x := &g.generals[i]
		if x.Location == prefectureID && x.Status.Recruitable() {
			out = append(out, x)
		}
	}
	return out
}

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
