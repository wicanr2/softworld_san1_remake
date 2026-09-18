package game

import (
	"encoding/binary"
	"fmt"
	"slices"
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

// Name 是季節在畫面上的那一個字（原版直排的最後一格）。
func (s Season) Name() string {
	switch s {
	case Spring:
		return t("season.spring")
	case Summer:
		return t("season.summer")
	case Autumn:
		return t("season.autumn")
	default:
		return t("season.winter")
	}
}

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

	// troops 是州郡 offset 16（兵士，實際值 ÷ 100）。**它是存值不是導出值**
	// （`L0`）：原版只在「重整守將清單」（`0x1949e` → `0x1964e`）時把
	// 名單裡每個人的兵力加總除以 100 寫進去，而重整只發生在**那個郡自己**
	// 被碰到的時候——郡回合入口、移防之後的來源與目標郡、戰役收尾。
	//
	// 所以搬進來的兵在原版那邊要等該郡被重整才算得進去。開局時它與駐軍
	// 加總相等（四十二個郡逐一驗過），但**那只是初始值**：`bootToGame`
	// 停在月中時量到郡 3 存著 39 而駐軍加總是 55。
	//
	// 要問「現在實際有多少兵」用 `State.Soldiers`；要寫回原版的表用這一格。
	troops int

	// activeGenerals 是州郡 offset 22（現役武將數）的存值。它與 troops
	// 由同一支「重整守將清單」常式刷新；人物離開之後到下一次重整之前，
	// 這一格可以故意比人物表舊。規則需要即時人數時用 ActiveGenerals，
	// 寫回原版表與顯示原版欄位時用 StoredActiveGenerals。
	activeGenerals int

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

// SignedAge 是原版眼裡的年齡：offset 7 當 `int8`（`0x15d71`／`0x1604f`
// 的 `cbw`）。未登場的人是負的（曹叡 189 年 −16），每年 +1 長到出頭年齡
// 才露面。
func (g *General) SignedAge() int { return int(int8(g.Age)) }

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

	// Alive 為假表示這個勢力**絕嗣**了——君主死而無人繼承（`Lord` ＝ −1）。
	//
	// 原版「活著的勢力」就是「君主槽 != 0xFFFF」（春季玉璽的候選清單
	// `0x15cd4`、玩家出局 `0x15924`），**不是有沒有領地**：領地歸零而君主
	// 還在的勢力照樣活著，能靠麾下翻身。要問「還持有郡嗎」用 `Territory`。
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

	// Players 是玩家控制的勢力，照玩家序號排（第 1 位在前，`docs/spec/019`）。
	// 空的是電腦自動示範模式。
	Players []state.FactionID
	// governorAsks 是還沒讓玩家挑新主事者的郡（`askGovernor`）。
	governorAsks []int
	// heirAsks 是還沒讓玩家挑繼承人的那幾次繼承（`askHeir`，Issue #65）。
	heirAsks []heirAsk
	// Player 是第一位玩家（Players[0]）；沒有玩家時是 NoFaction。只有一位
	// 玩家的呼叫端用它；規則裡「是不是玩家」一律問 IsHuman。
	Player state.FactionID

	// Edition 是原版還是加強版（`docs/spec/004`）。
	Edition state.Edition

	// pending 是內層常式（繼承、戰役分贓）累積的泡泡事件，`PendingEvents` 交出。
	pending []Event

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

	// DeathLog 數每一種死法（老死／斬首／戰死／單挑…）發生幾次。
	// **不進存檔**：這是普查與對拍要看的計數，不是局面。
	DeathLog map[string]int

	// turnOrder 是**下個月**的郡順序，由 `EndMonth` 的開月那一段洗出來
	// （`0x17364`）。原版把它連同旗標寫進進度檔，所以它是盤面的一部分，
	// 不是 session 的暫存。
	turnOrder []int

	// demoPref／demoPerson 是示範模式鏡頭被看的郡與人（`DS:0x771c`／`0x771e`，
	// `democamera.go`）。原版不存進度。
	demoPref, demoPerson int

	// randSeed 是原版那條 LCG 的狀態（`MSCRand`）。`randOn` 為真時
	// `roll`／`Roll` 改抽這一條，抽幾次記在 `randDraws`。
	//
	// **預設不開**：對齊序列要求每一處消耗的次數與順序都跟原版一樣，
	// 那是逐支常式要驗的事（`CONTEXT.md` 的「亂數對齊」）。沒對齊就開，
	// 只會把「與局面綁定、可重播」換成「與原版無關、也不可讀」。
	randSeed  uint32
	randDraws int
	randOn    bool

	// glyphs 是自創君主名字的字模（`docs/spec/013` R4）。
	//
	// 原版把那三個字當**造字**（Big5 `A141`–`A14C`），字模隨進度存在
	// `BASEPRE.SVn`——人物表裡放的只是碼位，沒有字模就是三個空白。
	// 開自創君主的局時從原版出貨的那一份帶進來；沒有就是 nil，
	// 存檔那一層會退回「讀上一次寫出去的」。
	glyphs *state.Glyphs

	// phaseTrace 非 nil 時，換月的每一段各抽了幾次會記進去（對拍用）。
	phaseTrace map[string]int
	phaseSeed  map[string]uint32
	phaseLast  int
	// rollTrace 非 nil 時，每一次 `Roll` 都會回報一次（對拍用）。
	rollTrace func(n, out int, salt []int)
}

// DrainReports 取走並清空累積的戰報。
func (g *State) DrainReports() []*BattleResult {
	out := g.Reports
	g.Reports = nil
	return out
}

// Glyphs 是自創君主名字的字模；沒有回 nil。
func (g *State) Glyphs() *state.Glyphs { return g.glyphs }

// SetGlyphs 換一份字模。
func (g *State) SetGlyphs(x *state.Glyphs) { g.glyphs = x }

// New 從一個劇本開一局。
//
// player 要是實際在用的勢力，否則回錯誤——**不要默默改成 0**：
// 「玩家其實在控制別人」這種錯誤在畫面上完全看不出來。
//
// ed 空字串當原版。難度的上限跟著版本走（`docs/spec/004`）：
// **拿加強版的難度 15 去開原版不會是「比較難」，是規則接錯了**——
// 原版的係數表只有十格，第十五格是表外的位元組。
func New(sc *state.Scenario, player state.FactionID, difficulty int, ed state.Edition) (*State, error) {
	return NewPlayers(sc, playerList(player), difficulty, ed)
}

// playerList 把單一玩家換成玩家序列（NoFaction 是零位）。
func playerList(player state.FactionID) []state.FactionID {
	if player == state.NoFaction {
		return nil
	}
	return []state.FactionID{player}
}

// NewPlayers 開新局，players 依玩家序號排（0–16 位，`docs/spec/019` §1）。
func NewPlayers(sc *state.Scenario, players []state.FactionID, difficulty int, ed state.Edition) (*State, error) {
	start, ok := ScenarioStart[sc.Slot]
	if !ok {
		return nil, fmt.Errorf("game: 不知道槽位 %q 的起始年月", sc.Slot)
	}
	g, err := newAt(sc, players, difficulty, ed, start)
	if err != nil {
		return nil, err
	}
	// 難度改寫電腦諸侯的等級——**只在開新局**（`Restore` 讀回來的進度
	// 已經是改寫過的）。兩版的算式不同，見 AILevelsAtStart。
	// 原版是照十六個槽掃的（沒在用的槽也算，它們的等級是 0），所以
	// 拿劇本的十六格算，再按槽號派回去。
	levels := make([]int, state.MasterTableSize/state.MasterRecordSize)
	for f := range levels {
		levels[f] = sc.AILevel(f)
	}
	adjusted := AILevelsAtStart(levels, difficulty, ed)
	for f, v := range adjusted {
		// 沒建模的槽（填充槽）也照原版一起改——它們不在 `factions` 裡，
		// 只能寫在原始位元組上，`Tables()` 會原封帶出去。
		put16(g.rawMas[f*masRecord+masAILevel:], v)
	}
	for i := range g.factions {
		if id := int(g.factions[i].ID); id >= 0 && id < len(adjusted) {
			g.factions[i].AILevel = adjusted[id]
		}
	}
	g.clearFillerSlots(players)
	return g, nil
}

// fillerLordFrom 是自創君主範本在人物表的第一筆（346，`docs/re/08` §6）；
// 劇本裡君主槽指到這裡以後的諸侯槽是填充槽。
const fillerLordFrom = 346

// clearFillerSlots 是開新局的收尾（同一支常式的後半，`L0`、`[both]`：
// 原版 `0x1241b`–`0x1248b`、加強版 `0x11a35`–）：君主槽指向填充筆而沒被
// 玩家選成自創君主的諸侯槽，操縱方／君主／軍師全寫 `0xFFFF`，那筆填充
// 君主寫成已故（身分 12、勢力 `0xFF`、領地 `0xFF`）——出貨進度裡範本
// 的身分 12 就是這裡寫的。「被選成自創君主」在 remake 就是玩家那一格
// （`state.Scenario.WithCustomLord` 寫進去的），只有它留著。
//
// 原版不看那個槽有沒有領地、操縱方是不是 2——劇本三到六的填充槽夾在
// 中間（槽 4、5、10、11…），`ActiveFactions` 會把它們當在用的勢力建進
// `factions`；這裡一併拿掉。位元組直接寫在 `rawMas` 上，`Tables()` 從
// 它起手會原封帶出去。
func (g *State) clearFillerSlots(players []state.FactionID) {
	isFiller := func(f int) bool {
		// 原版是有號比較（`jl`）：君主槽 `0xFFFF` 的槽算 −1，不碰。
		lord := int(int16(binary.LittleEndian.Uint16(g.rawMas[f*masRecord+masLord:])))
		return lord >= fillerLordFrom && !slices.Contains(players, state.FactionID(f))
	}
	kept := g.factions[:0]
	for _, f := range g.factions {
		if isFiller(int(f.ID)) {
			continue
		}
		kept = append(kept, f)
	}
	g.factions = kept
	for f := 0; f*masRecord+masRecord <= len(g.rawMas); f++ {
		if !isFiller(f) {
			continue
		}
		lord := int(binary.LittleEndian.Uint16(g.rawMas[f*masRecord+masLord:]))
		for _, off := range []int{masController, masLord, masChief} {
			put16(g.rawMas[f*masRecord+off:], state.NoValue16)
		}
		if x := g.General(lord); x != nil {
			x.Status, x.Faction, x.Location = state.StatusFallen, state.NoFaction, int(state.NoValue)
		}
	}
}

// AILevelsAtStart 是開新局時難度對十六個勢力 AI 等級（諸侯 offset 4）的
// 改寫（`L1`、`[both]`；原版 `0x12317`–`0x123a1`、加強版 `0x11941`–`0x119c3`，
// 對拍 `TestZZNewGameAILevelByDifficulty`，`docs/mechanics/90` §6.4）。
//
//	難度 ≤ 2   兩版都把 > 2 的等級壓到 2——但**原版碰到第一個 ≤ 2 的勢力
//	           就停**（`0x1236f` 跳出迴圈），後面的不壓；加強版十六個都看。
//	難度 > 2   原版不動；加強版每個勢力 += (難度 mod 11) ÷ 2。
//	最後       兩版都把 ≥ 5 的壓到 5。
//
// 傳入的順序就是勢力槽號的順序（0..15）；原版那個「碰到就停」的判斷
// 是照槽號掃的，所以順序不能亂。
func AILevelsAtStart(levels []int, difficulty int, ed state.Edition) []int {
	out := append([]int(nil), levels...)
	if difficulty <= 2 {
		for i, v := range out {
			if v > 2 {
				out[i] = 2
			} else if ed != state.EditionPlus {
				break
			}
		}
	} else if ed == state.EditionPlus {
		inc := (difficulty % 11) / 2
		for i := range out {
			out[i] += inc
		}
	}
	for i, v := range out {
		if v >= 5 {
			out[i] = 5
		}
	}
	return out
}

// newAt 是 New 去掉「起始年月要查得到」那一條。
//
// **讀進度時年月來自進度本身**，起始年月當場就被蓋掉；而原版存的進度
// 不記得自己是從哪個劇本開始的（`docs/re/08`），拿劇本表去查一定落空。
// 落空就擋下來的話，玩家自己的原版存檔一份都讀不進來。
// Continue 從**已經開過局**的三張表接手一局：不重寫電腦諸侯的等級、
// 不清填充槽（那兩件事 `New` 只在開新局做；加強版的等級改寫是 `+N`，
// 做兩次就疊上去）。對拍從原版執行期記憶體拍下來的盤面接手用這一支；
// 讀進度用 `Restore`（它多帶三張表放不下的欄位）。
func Continue(sc *state.Scenario, player state.FactionID, difficulty int,
	ed state.Edition, at Date) (*State, error) {
	return newAt(sc, playerList(player), difficulty, ed, at)
}

func newAt(sc *state.Scenario, players []state.FactionID, difficulty int,
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

	g := &State{Slot: sc.Slot, Date: start, Player: state.NoFaction,
		Players: append([]state.FactionID(nil), players...),
		Edition: ed, Difficulty: difficulty,
		demoPref: DemoCameraPrefecture, demoPerson: DemoCameraPerson}
	if len(players) > 0 {
		g.Player = players[0]
	}
	g.rawMas, g.rawSta, g.rawGen = sc.Tables()

	for _, p := range sc.Prefectures() {
		g.prefectures = append(g.prefectures, Prefecture{
			ID: p.ID, Name: p.Name,
			Owner:    state.FactionID(p.Owner),
			Province: p.Province,
			MapX:     int(p.MapX), MapY: int(p.MapY),
			Population: p.People(),
			Gold:       int(p.Gold), Rice: int(p.Rice),
			PublicLoyalty: p.PublicLoyalty, LandValue: p.LandValue,
			FloodRate: p.FloodRate, PriceLevel: p.PriceLevel,
			Forts:          int(p.Forts),
			Autonomy:       Autonomy(p.Autonomy),
			governor:       governorSlot(p.Governor),
			troops:         int(p.Soldiers),
			activeGenerals: int(p.ActiveGenerals),
			Neighbours:     append([]int(nil), p.Neighbours...),
			BattleField:    append([]byte(nil), p.BattleField...),
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

	valid := 0
	for _, f := range sc.ActiveFactions() {
		lord, err := sc.Lord(f)
		if err != nil {
			return nil, err
		}
		fa := Faction{ID: state.FactionID(f), Lord: lord.Index, Alive: true, Chief: -1,
			AILevel: sc.AILevel(f), Prestige: sc.Prestige(f),
			ByComputer: !slices.Contains(players, state.FactionID(f))}
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
		if slices.Contains(players, state.FactionID(f)) {
			valid++
		}
	}
	seen := map[state.FactionID]bool{}
	for _, p := range players {
		if seen[p] {
			return nil, fmt.Errorf("game: 勢力 %d 被選了兩次", p)
		}
		seen[p] = true
	}
	if valid != len(players) {
		return nil, fmt.Errorf("game: 玩家 %v 裡有勢力在劇本 %s 裡沒有在用", players, sc.Slot)
	}
	return g, nil
}

// IsHuman 回報這個勢力是不是玩家在操作（諸侯記錄 offset 0 ＝ 1）。
// 規則裡「是不是玩家」都問這一支——多人時玩家不只一位（`docs/spec/019`）。
func (g *State) IsHuman(id state.FactionID) bool {
	if id == state.NoFaction {
		return false
	}
	f := g.Faction(id)
	return f != nil && !f.ByComputer
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
// TurnTick 是月迴圈每走一格都要抽的那一次亂數（`0x15790`，`L0`）。
//
//	0x15790  RND(12)
//	0x1579c  cmp es:[0x3f08]         ; 月份
//	0x157a5  jge → 跳過
//	0x157a7  lcall 0x3eb:0x0         ; 畫面／音樂，remake 用不到
//
// **每一格都抽，跳過的格子也抽**——它排在旗標與所屬的檢查之前
// （`0x157d1`），所以無主的郡、旗標早就清掉的郡照樣消耗一次。
// 效果本身 remake 不需要，但**那一次抽樣要留著**，否則從第一個郡起
// 整條亂數序列就錯開（`docs/re/08` §2）。
func (g *State) TurnTick() {
	g.Roll(12, 0x15790)
}

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
// 只數身分為「在野而且在該郡露面」（8）與「剛失去勢力」（10）的人——
// 原版的郡欄位是逐筆加減的計數（尋訪露面 ＋1、登用 −1、戰後失去勢力
// ＋1 `0x201f5`），這兩種身分都算進去；沒露面的（9）不算，開局的四十二
// 個郡全對，不加這個條件只對得上二十六個。
func (g *State) FreeGenerals(prefectureID int) int {
	n := 0
	for i := range g.generals {
		x := &g.generals[i]
		if !x.Employed() && x.Location == prefectureID && x.Name != "" &&
			(x.Status == state.StatusAvailable || x.Status == state.StatusStranded) {
			n++
		}
	}
	return n
}

// endTurn 是一個郡這個月**下過令了**：下令旗標立起來（玩家那一條
// 「每郡每月一道令」看它，說明書 p.17）。電腦一個回合跑十八張表、
// 下好幾道令，所以它不是「回合結束」——回合結束是 FinishTurn。
func (g *State) endTurn(p *Prefecture) {
	p.Commanded = true
}

// FinishTurn 是一個郡這個月的回合走完：玩家下完那一道令、電腦的
// 分派器跑完。**加強版在這裡再重整一次守將清單**（`0x162dc`–`0x162e0`：
// 電腦與玩家的回合走完都 `call 0x17fc8`；原版 `0x1746e` 只在入口做，
// `L0`），兵士與現役將兩欄因此在回合結束那一刻就是新值——玩家命令的
// 對拍在訓練兵士那一道抓到（`TestPlayerCommandsPlus`）。
//
// **不能綁在每一道令上**：電腦的十八張表中間重整，出兵那一張整編
// 帶走的錢糧會拿到調整兵力之後的兵士（加強版月度對拍量到郡 15 差 28 金）。
func (g *State) FinishTurn(prefectureID int) {
	if g.Edition == state.EditionPlus {
		g.RefreshGarrison(prefectureID)
	}
}

// EndTurnAt 是「下過令 ＋ 回合走完」，給逐郡驅動的對拍用。
func (g *State) EndTurnAt(prefectureID int) {
	if p := g.Prefecture(prefectureID); p != nil {
		g.endTurn(p)
		g.FinishTurn(prefectureID)
	}
}

// RefreshGarrison 是原版的「重整守將清單」（`0x1949e`，`L0`）。
//
// **只在原版會重整的時候叫它**：郡回合入口、加強版的回合結束、移防之後
// 的來源與目標郡、戰役收尾。每次寫盤面都順手重算會讓那幾欄變成導出值，
// 而原版的它們會陳舊——差別在「兵搬進來之後、那個郡還沒輪到」的窗口裡
// 看得見。
//
// 做的事不只兩個計數：
//
//	清單 ＝ 郡裡身分 0–3 的每一位（不比對勢力），按「魅力 ＋ 加權表[身分]」
//	       交換排序（0xf520）
//	現役將 ← 清單長度                                          ; 0x194e5
//	清單空 → 所屬 ← 無主、主事者 ← 沒有、兵士 ← 0             ; 0x194fa
//	所屬 ← 清單第一位的勢力                                     ; 0x1952f
//	主事者：第一位是君主 → 他；否則存的那一位還在郡裡 → 不動；
//	        否則 0x1d638 重建清單、挑**行動者鍵最大**的那一位     ; 0x19538–0x1956d
//	主事者是一般武將 → 升太守                                   ; 0x195a3
//	清單裡其他的太守 → 降成一般武將                             ; 0x195d8
//	兵士 ← Σ 兵力 ÷ 100                                         ; 0x195fc
//
// 所屬那一格緊接著會被回合入口的 `RecomputeOwners`（`0x1e394`）蓋掉，
// 但戰役收尾與移防之後的那幾次沒有人蓋，要留到下一個郡回合入口。
func (g *State) RefreshGarrison(prefectureID int) {
	p := g.Prefecture(prefectureID)
	if p == nil {
		return
	}
	roster := g.Garrison(prefectureID)
	p.activeGenerals = len(roster)
	if len(roster) == 0 {
		p.Owner = state.NoFaction
		p.governor = -1
		p.troops = 0
		return
	}
	sorted := append([]*General(nil), roster...)
	key := func(x *General) int {
		w := 0
		if int(x.Status) < len(ActorWeight) {
			w = ActorWeight[x.Status]
		}
		return int(x.Charm) + w
	}
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if key(sorted[j]) > key(sorted[i]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	p.Owner = sorted[0].Faction
	gov := p.governor
	switch {
	case sorted[0].Status == state.StatusLord:
		gov = sorted[0].Index
	case g.General(gov) != nil && g.General(gov).Location == prefectureID:
		// 存的那一位還在，不動。
	default:
		// 0x1d638：清單重建（建表常式收尾一律按行動者鍵排，`0xfe3e`／
		// 加強版 `0xf872`）之後的第一位——智 ＋ 武 ＋ 加權表[身分]最大者。
		gov = g.ActorRoster(prefectureID)[0].Index
		// **玩家的郡由玩家自己挑**（`0x1d6ed`「選擇新任太守」，Issue #84）：
		// 先填原版電腦那一條的預設值（沒有畫面的路照舊能跑），同時排進待答
		// 佇列讓畫面層問；答完由 `AssignGovernor` 覆蓋。
		g.askGovernor(prefectureID, sorted[0].Faction)
	}
	p.governor = gov
	if x := g.General(gov); x != nil && x.Status == state.StatusOfficer {
		x.Status = state.StatusGovernor
	}
	for _, x := range roster {
		if x.Index != gov && x.Status == state.StatusGovernor {
			x.Status = state.StatusOfficer
		}
	}
	p.troops = g.Soldiers(prefectureID) / 100
}

// askGovernor 把「這個郡要挑新主事者」排給畫面層（`0x1d638` → `0x1d6ed`
// 「選擇新任太守」，Issue #84）。只有**人類控制的勢力**會問（`0x1d6c0`：諸侯記錄
// 的控制旗標），而且原版那一問**不能取消**——取消會再問一次。
//
// 排進來之前規則層已經填了原版電腦那一條的預設值，所以沒有畫面的路（測試、
// 存檔匯入、電腦的郡）照舊走得完；玩家答了才覆蓋。
func (g *State) askGovernor(prefectureID int, owner state.FactionID) {
	if !g.IsHuman(owner) {
		return
	}
	for _, id := range g.governorAsks {
		if id == prefectureID {
			return
		}
	}
	g.governorAsks = append(g.governorAsks, prefectureID)
}

// NeedsGovernor 回下一個要玩家挑主事者的郡；沒有就回 0。
func (g *State) NeedsGovernor() int {
	for len(g.governorAsks) > 0 {
		id := g.governorAsks[0]
		p := g.Prefecture(id)
		// 排進來之後郡可能已經沒了（無主）或又換了主人。
		if p != nil && p.Owned() && g.IsHuman(p.Owner) && len(g.ActorRoster(id)) > 0 {
			return id
		}
		g.governorAsks = g.governorAsks[1:]
	}
	return 0
}

// AssignGovernor 是玩家挑完之後那一步（`0x1d709`–`0x1d727`）：寫州郡 offset 32、
// 身分 3 升 2、同郡其他人的 2 降回 3。
func (g *State) AssignGovernor(prefectureID, index int) error {
	p := g.Prefecture(prefectureID)
	if p == nil || !p.Owned() {
		return ErrNotYours
	}
	x := g.General(index)
	if x == nil || !x.Employed() || x.Location != prefectureID {
		return ErrUnknownUnit
	}
	p.governor = index
	if x.Status == state.StatusOfficer {
		x.Status = state.StatusGovernor
	}
	for _, y := range g.Garrison(prefectureID) {
		if y.Index != index && y.Status == state.StatusGovernor {
			y.Status = state.StatusOfficer
		}
	}
	for i, id := range g.governorAsks {
		if id == prefectureID {
			g.governorAsks = append(g.governorAsks[:i], g.governorAsks[i+1:]...)
			break
		}
	}
	return nil
}

// refreshGovernor 是 `0x1d638`（加強版 `0x1ba90`，`L0`＋`L1`）：整編把出征
// 的人清出郡之後、戰役收尾把勝方放回戰場郡之後各叫一次。與 RefreshGarrison
// 差在**不重挑所屬**：清單空就把郡設成無主；主事者不在了就換清單第一位——
// 建表常式（`0xfc1e`／`0xf67c`）收尾一律按行動者鍵（智 ＋ 武 ＋ 加權表[身分]）
// 交換排序，所以第一位是鍵最大的那一位，不是槽號最小的。**身分的加權會
// 讓剛在別的郡當上太守的人領先**：加強版月度對拍量到郡 37 的 114 先在
// 指定太守被升成太守，帶兵打下郡 30 之後以 1327 對 11 的 941 接下新郡的
// 主事者；按槽號挑會給 11。一般武將升太守；然後刷新兩個計數。
func (g *State) refreshGovernor(prefectureID int) {
	p := g.Prefecture(prefectureID)
	if p == nil {
		return
	}
	roster := g.ActorRoster(prefectureID)
	if len(roster) == 0 {
		p.Owner = state.NoFaction
		p.governor = -1
	} else if x := g.General(p.governor); x == nil || x.Location != prefectureID {
		p.governor = roster[0].Index
		if roster[0].Status == state.StatusOfficer {
			roster[0].Status = state.StatusGovernor
		}
		g.askGovernor(prefectureID, p.Owner)
	}
	p.activeGenerals = len(roster)
	p.troops = g.Soldiers(prefectureID) / 100
}

// RefreshActiveGenerals 只刷新現役將那一欄（州郡 offset 22）：電腦挖角
// 挑人那一支開頭重建本郡的守將清單之後寫回人數（`0xe0d5`–`0xe0f9`），
// 兵士那一欄不動。
func (g *State) RefreshActiveGenerals(prefectureID int) {
	if p := g.Prefecture(prefectureID); p != nil {
		p.activeGenerals = g.ActiveGenerals(prefectureID)
	}
}

// RefreshTroops 只刷新兵士與現役將兩欄（分派器建清單那一段的收尾
// `0xefe8`–`0xf083`：`0xf072` 寫兵士、`0xf083` 寫現役將），調整兵力那一張
// 表重建清單時順手做的。不碰所屬與主事者。
func (g *State) RefreshTroops(prefectureID int) {
	if p := g.Prefecture(prefectureID); p != nil {
		p.troops = g.Soldiers(prefectureID) / 100
		p.activeGenerals = g.ActiveGenerals(prefectureID)
	}
}

// Troops 是州郡 offset 16 的存值（兵士，實際值 ÷ 100）。
func (g *State) Troops(prefectureID int) int {
	if p := g.Prefecture(prefectureID); p != nil {
		return p.troops
	}
	return 0
}

// StoredGovernor 是州郡 offset 32 存的那一位，**不管他還在不在郡裡**；
// 沒有（`0xFFFF`）回 nil。指定太守的閘門讀的是這一格（`0xd6b3`：存的
// 那一位身分是君主就整支不做，不看他人在哪）。
func (g *State) StoredGovernor(prefectureID int) *General {
	if p := g.Prefecture(prefectureID); p != nil {
		return g.General(p.governor)
	}
	return nil
}

// StoredActiveGenerals 是州郡 offset 22 的存值；只有原版會寫這一格的
// 常式才能刷新它，不能在 Tables 裡從人物表偷偷重算。
func (g *State) StoredActiveGenerals(prefectureID int) int {
	if p := g.Prefecture(prefectureID); p != nil {
		return p.activeGenerals
	}
	return 0
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
