// Package state 讀原版的劇本與存檔資料表。
//
// 規格 `docs/spec/002-scenario-tables.md`（READY），證據
// `docs/formats/03-scenario-tables.md`。
//
// ⚠ **這一版只解姓名與郡名，其餘 bytes 一律原樣交出。** 手冊列了 15 項郡屬性
// 與 15 項將領屬性，那是該去對的清單，不是版面。照它猜欄位順序，猜出來的
// 結構會自洽、會通過型別檢查、而且不會報錯——錯誤要到「某個郡的糧草數字
// 很怪」才浮現，那時已經有一堆程式建在上面。
package state

import (
	"encoding/binary"
	"fmt"
	"strings"

	"golang.org/x/text/encoding/traditionalchinese"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// 記錄大小與筆數（`docs/formats/03` §2，L0）。
const (
	masterSize, masterCount = 72, 16
	prefSize, prefCount     = 176, 43 // 含筆 0 的啞元
	generalSize, genCount   = 30, 350

	// PrefectureCount 是實際的郡數。筆 0 是啞元，所以 43 − 1。
	PrefectureCount = prefCount - 1
)

// Slot 是劇本或存檔槽的編號，直接對應容器項目名的副檔名。
type Slot string

// 六個劇本，對應手冊的六個時期。
const (
	Scenario1 Slot = "001"
	Scenario2 Slot = "002"
	Scenario3 Slot = "003"
	Scenario4 Slot = "004"
	Scenario5 Slot = "005"
	Scenario6 Slot = "006"
)

// FactionID 是勢力的槽號（`BASEMAS` 的筆號），NoFaction ＝ 沒有勢力。
//
// 給它一個名字是為了讓「勢力編號」與「郡編號」在型別上分得開——
// 兩者都是 1 到 40 幾的小整數，混用不會編譯錯誤。
type FactionID uint8

// NoFaction 是「沒有勢力」的哨兵值，原版用 0xFF。
//
// ⚠ **不要讓 0xFF 當成數值流進規則層。** 它是哨兵不是編號
// （`CLAUDE.md` §7 第 11 條）；郡的 Owner 與人物的 Faction 都要在
// 這一層問過 Owned／Employed 再用。
const NoFaction = 0xFF

// NoValue 是「這一格對這筆記錄沒有意義」的哨兵，原版同樣用 0xFF。
//
// ⚠ **哨兵不是數值。** 忠誠欄在在野者身上是 0xFF；當成 255 算下去
// 不會報錯，只會讓在野武將變成全遊戲最忠誠的人。
const NoValue = 0xFF

// Rank 是人物的職位（`BASEGEN` offset 12）。原版用它索引一張九格的字串表。
type Rank uint8

// Status 是人物的身分（`BASEGEN` offset 17）。
//
// ⚠ **它不只是顯示用的分類，是規則**：郡的「在野武將數」只數
// StatusAvailable 的人（`docs/spec/003` §2.2，42 個郡全對；
// 不加這個條件只對得上 26 個）。
type Status uint8

// TroopType 是兵種（`BASEGEN` offset 21），索引一張七格的字串表。
type TroopType uint8

const (
	RankLord Rank = iota // 君主
	RankStrategist
	RankStaff
	RankClerk
	RankAdvisor
	RankGeneral
	RankViceGeneral
	RankSubGeneral
	RankJuniorGeneral
)

const (
	StatusLord     Status = 0 // 君主
	StatusChief    Status = 1 // 軍師
	StatusGovernor Status = 2 // 太守
	StatusOfficer  Status = 3 // 一般武將
	// StatusAvailable 是「在野而且在該郡露面」。**只有這一種算進
	// 郡的在野武將數。**
	StatusAvailable Status = 8
	StatusIdle      Status = 9  // 在野，不列入郡的在野數
	StatusUnborn    Status = 11 // 還沒登場（此時諸葛亮 8 歲）

	// StatusFallen 是**已故**。君主死掉時原版就是把身分寫成 12、
	// 勢力與領地都寫成 `0xFF`（`0x14a5d`–`0x14a6a`），接著才處理繼承。
	// 劇本 001 有一位這樣的人。
	StatusFallen Status = 12
)

// Governs 回報這個身分是不是郡的主事者。
//
// 君主與太守是常態；**軍師是代理**——`Governor` 才是完整的判斷。
func (s Status) Governs() bool { return s == StatusLord || s == StatusGovernor }

// Prefecture 是一個郡。
type Prefecture struct {
	// ID 是郡編號，**1..42**。原版的筆 0 是啞元，所以編號與筆號相同，
	// 也與手冊用 1–42 稱呼州郡一致。
	ID   int
	Name string // "遼東"

	// Owner 是所屬勢力的槽號（`BASEMAS` 的筆號），NoFaction ＝ 無主。
	// 版面出處 `docs/spec/003` §3。
	Owner uint8

	// MapX／MapY 是這個郡在大地圖上的座標（offset 6／8，`u16`，`L0`）。
	//
	// 原版畫郡名時直接拿它們加偏移當螢幕座標（`0x10ce6`：
	// `x + 80`、`y + 44`）。劇本 001 的範圍是 X 15–285、Y 23–288，
	// 而且與地理對得起來：遼東 (276, 23) 在東北角、酒泉 (15, 55) 最西、
	// 南海 (180, 284) 最南。
	MapX, MapY uint16

	// Governor 是這個郡的**主事者**在人物表的槽號（offset 32，`u16`，
	// `L0`），`NoValue16` ＝ 沒有人主事。
	//
	// 原版直接讀這一格，不是每次從人物表推：用計的前置條件
	// （`0x2c270`）拿它取出那個人再看身分是不是君主，五種謀略把它
	// 當參數往下傳（`0x2c82e`／`0x2ccc4`／`0x2d094`／`0x2d57a`）。
	// 開局之後有十三處會寫它（`0x0d705`、`0x132ee`、`0x14d36`、
	// `0x19500`…），所以它是**執行期維護的狀態**不是劇本裡的裝飾。
	//
	// 劇本 001 的 24 個有主的郡逐郡驗過：那個人的勢力與所在郡都與郡
	// 本身相符，身分只有君主或太守。
	Governor uint16

	// ⚠ **Population 與 Soldiers 存的是實際值 ÷ 100。**
	// 原版的格式字串是 `人口 %5d00`／`兵士%4d00`——把 `00` 直接接在
	// 數字後面。存 800 顯示 80000。照存的數字當人口用會差兩個數量級，
	// 而且不會報錯，所以這裡不做換算，由 People()／Troops() 給實際值。
	Population uint16
	Soldiers   uint16

	Gold uint16 // 金
	Rice uint16 // 米

	ActiveGenerals uint8 // 現役武將數
	FreeGenerals   uint8 // 在野武將數（只數身分為 StatusAvailable 的）

	// Forts 是這個郡的關寨數（offset 25，`L0`）。
	//
	// **它與戰場地圖是同一件事的兩份記錄**：42 個郡逐一數過，這一格
	// 永遠等於 offset 55–174 裡地形碼 6（關寨）的格數
	// （`TestFortCountMatchesTheField`）。建築關寨時原版讓玩家在地圖上
	// 挑位置，兩邊一起更新（`0x1ab9e`）。
	//
	// 六個劇本的值完全相同——與相鄰表一樣，這是**地圖幾何不是劇本狀態**。
	Forts uint8

	PublicLoyalty uint8 // 民眾忠誠
	LandValue     uint8 // 土地價值
	FloodRate     uint8 // 洪水率
	PriceLevel    uint8 // 物價

	// Neighbours 是相鄰的郡編號（原版 offset 45–54，`0xFF` 補齊）。
	//
	// **欄位是 10 格**：原版建出兵候選清單的常式（`0xeee8`／`0xef5c`）
	// 掃的是 `k < 10`。劇本 001 的第 9、10 格全部是 `0xFF`，鄰居最多的
	// 洛陽只用到 8 格——所以只讀 8 格在這份資料上得到一樣的結果，
	// 但那是資料剛好，不是欄位寬度。
	//
	// 這是**地圖幾何不是劇本狀態**：十二個槽位（六個劇本 ＋ 六個存檔）
	// 的相鄰表完全相同。整張圖對稱（95 條邊、零條單向）而且全連通。
	Neighbours []int

	// BattleField 是這個郡的**戰場地圖**（原版 offset 55–174，120 個位元組）。
	//
	// 12 欄 × 10 列，索引是 `列 × 12 + 欄`。每一格一個位元組：
	//
	//	低四位 = 地形（1 大山、2 山丘、3 淺水、4 深水、5 城池、
	//	                6 關寨、7 平原、8 樹林、9 沙漠）
	//	高四位 = 0–9 通往第幾個鄰郡（索引 Neighbours）、
	//	         10 城池、11–14 四個軍團的起點、15 沒有標記
	//	0xFF   = 圖外
	//
	// 出處是原版自己的碼：戰鬥入口 `0x20200` 進來就把這 120 個位元組
	// 複製到工作區 `es:[0x163a]`（`docs/re/05` §1）。
	BattleField []byte

	Raw [prefSize]byte
}

// Owned 回報這個郡有沒有主。
func (p Prefecture) Owned() bool { return p.Owner != NoFaction }

// Adjacent 回報兩個郡相不相鄰。
func (s *Scenario) Adjacent(a, b int) bool {
	pa, err := s.Prefecture(a)
	if err != nil {
		return false
	}
	for _, n := range pa.Neighbours {
		if n == b {
			return true
		}
	}
	return false
}

// People 是實際人口（存的值 × 100）。
func (p Prefecture) People() int { return int(p.Population) * 100 }

// Troops 是實際兵士數（存的值 × 100）。
func (p Prefecture) Troops() int { return int(p.Soldiers) * 100 }

// General 是人物表的一個槽。
//
// ⚠ **不是每個槽都是人。** 350 個槽裡有 346 個是人物，末尾 4 個
// （槽 346–349）的姓名欄裝的是**連續的 Big5 碼位** A141–A14C
// （全形標點，每槽三個），而且四個槽在 offset 6–13 的位元組完全相同——
// 真實人物那幾格各不相同。連續碼位不會是資料，是填充。
// 用 IsPerson 過濾，不要假設槽號就是人物編號。
type General struct {
	// Index 是原版的槽號 0..349。**填充槽也占一個號**——
	// 中間少一筆會讓後面全部錯位。
	Index int

	// Name 是姓名欄解出來的文字，原樣交出（填充槽會是標點）。
	Name string

	// 欄位版面出處 `docs/spec/003` §2——是從原版自己的格式字串讀出來的
	// （哪一個 byte 被推進 `printf`、配哪一個標籤），不是猜的。
	Age     uint8 // 年齡
	Stamina uint8 // 體能
	Intel   uint8 // 謀略
	War     uint8 // 戰力
	Charm   uint8 // 魅力
	Rank    Rank  // 職位
	Origin  uint8 // 出身郡（1..42）
	// Loyalty 是忠誠。**在野者是 0xFF（NoValue）不是 0**——沒有主子就
	// 沒有忠誠可言。實測 346 位裡 227 位是 0xFF，而且全部沒有勢力；
	// 有勢力者的值域是 52–100。當成數值算下去會得到「在野者忠誠 255」。
	Loyalty  uint8
	Status   Status
	Troop    TroopType // 兵種
	Soldiers uint16    // 兵士數
	Training uint8     // 訓練度
	Arms     uint8     // 武裝度

	// Bond 是 offset 14 的人物槽號（`u16`）。
	//
	// **原版拿它當登用的閘門**（`0xce8c`，`L0`）：被登用者的 Bond 指到
	// 誰，就看那個人現在效力於誰——效力於招募方就一定成功、在野就照
	// 能力值判定、效力於第三方就幾乎不可能。346 筆裡 176 筆指向自己，
	// 而指向自己的人在野時 Bond 那一格自然是在野，所以「沒有牽絆」。
	//
	// ⚠ 這一格代表什麼關係還沒定（`L2`）：舊主與親族都解釋得通，
	// 而曹純指向孫乾（`docs/spec/003` §2.5）。**功能是 `L0`，語意是 `L2`**。
	Bond int

	// Debut 是未登場者出頭的年齡（人物表 offset 26，`L0`）。
	//
	// **每個人自己一個**，不是全域常數：春季的元月處理拿
	// `年齡 > Debut` 當閘門（`0x1605a`）。
	Debut uint8

	// Faction 是效力的勢力槽號，NoFaction ＝ 在野。
	// Location 是所在郡的編號（1..42），原版的標籤是「領地」。
	//
	// 兩個一起驗過：346 位人物裡 108 位有勢力，**這 108 位的所在郡
	// 全部歸屬於自己的勢力，零例外**。兩張表由不同欄位獨立編碼
	// 同一件事而完全對得上，所以不是巧合。
	Faction  uint8
	Location uint8

	// IsPerson 為真表示 Name 全部是漢字。這是目前唯一能把人物與填充槽
	// 分開的判準，而且是從資料本身推的，不是從槽號硬編的。
	IsPerson bool

	Raw [generalSize]byte
}

// Master 是一位諸侯。
//
// ⚠ **諸侯記錄裡沒有姓名。** 名字要拿 LordIndex 去人物表查
// （`docs/spec/003` §2）。16 個槽裡有兩個指向姓名是全形標點的填充筆，
// 那是沒在用的槽——用 Active 過濾，不要硬編「前 14 個」。
type Master struct {
	Index int

	// LordIndex 是君主本人在人物表的槽號（`BASEMAS` offset 2）。
	LordIndex int

	Raw [masterSize]byte
}

// Scenario 是一個劇本或存檔槽的三張表。
type Scenario struct {
	Slot        Slot
	prefectures []Prefecture
	generals    []General
	masters     []Master

	// 三張表的原始位元組。存檔要用它保住還沒解出來的欄位，見 Tables。
	rawMas, rawSta, rawGen []byte
}

// LoadScenario 從 DATA2 容器讀一個槽。
//
// 定位一律用容器項目名，**不用絕對位移**——位移會隨版本漂移
// （`DATA2.GRP` 兩版等長但內容不同），項目名不會。
func LoadScenario(c *assets.Container, slot Slot) (*Scenario, error) {
	mas, err := section(c, "BASEMAS."+string(slot), masterSize*masterCount)
	if err != nil {
		return nil, err
	}
	sta, err := section(c, "BASESTA."+string(slot), prefSize*prefCount)
	if err != nil {
		return nil, err
	}
	gen, err := section(c, "BASEGEN."+string(slot), generalSize*genCount)
	if err != nil {
		return nil, err
	}
	return DecodeTables(slot, mas, sta, gen)
}

// TableSizes 是三張表各自該有的長度。存檔要照這個尺寸寫回去。
const (
	MasterTableSize     = masterSize * masterCount
	PrefectureTableSize = prefSize * prefCount
	GeneralTableSize    = generalSize * genCount
)

// 一筆記錄的長度。對拍要按記錄走，所以這幾個數字要拿得到。
const (
	MasterRecordSize     = masterSize
	PrefectureRecordSize = prefSize
	GeneralRecordSize    = generalSize
)

// DecodeTables 從三張表的原始位元組解出一個劇本或存檔。
//
// 與 LoadScenario 分開，是因為**存檔不從容器來**：remake 的存檔是
// 三個獨立檔案，但版面與原版完全相同（`docs/formats/05`），
// 所以解碼要走同一段程式碼——兩份解碼就會有兩個真相。
func DecodeTables(slot Slot, mas, sta, gen []byte) (*Scenario, error) {
	if len(mas) != MasterTableSize || len(sta) != PrefectureTableSize ||
		len(gen) != GeneralTableSize {
		return nil, fmt.Errorf("state: 三張表長度 %d/%d/%d，應該是 %d/%d/%d",
			len(mas), len(sta), len(gen),
			MasterTableSize, PrefectureTableSize, GeneralTableSize)
	}
	s := &Scenario{Slot: slot}
	s.rawMas = append([]byte(nil), mas...)
	s.rawSta = append([]byte(nil), sta...)
	s.rawGen = append([]byte(nil), gen...)

	s.masters = make([]Master, masterCount)
	for i := range s.masters {
		s.masters[i].Index = i
		copy(s.masters[i].Raw[:], mas[i*masterSize:])
		s.masters[i].LordIndex = int(binary.LittleEndian.Uint16(mas[i*masterSize+2:]))
	}

	// 郡：跳過筆 0 的啞元，ID 從 1 開始。
	s.prefectures = make([]Prefecture, PrefectureCount)
	for i := range s.prefectures {
		rec := sta[(i+1)*prefSize:]
		p := Prefecture{ID: i + 1}
		copy(p.Raw[:], rec)
		p.BattleField = append([]byte(nil), rec[55:175]...)
		// 郡名：offset 0，4 byte Big5 ＋ 1 byte NUL。
		name, err := decodeBig5(rec[:4])
		if err != nil {
			return nil, fmt.Errorf("state: 郡 %d 的名稱解不出來：%w", p.ID, err)
		}
		p.Name = name
		p.MapX = binary.LittleEndian.Uint16(rec[6:])
		p.MapY = binary.LittleEndian.Uint16(rec[8:])
		p.Population = binary.LittleEndian.Uint16(rec[14:])
		p.Soldiers = binary.LittleEndian.Uint16(rec[16:])
		p.Gold = binary.LittleEndian.Uint16(rec[18:])
		p.Rice = binary.LittleEndian.Uint16(rec[20:])
		p.ActiveGenerals = rec[22]
		p.Forts = rec[25]
		p.FreeGenerals = rec[23]
		p.PublicLoyalty = rec[26]
		p.LandValue = rec[27]
		p.FloodRate = rec[28]
		p.PriceLevel = rec[29]
		p.Owner = rec[30]
		p.Governor = binary.LittleEndian.Uint16(rec[32:])
		for _, b := range rec[45:55] {
			if b >= 1 && b <= PrefectureCount {
				p.Neighbours = append(p.Neighbours, int(b))
			}
		}
		s.prefectures[i] = p
	}

	s.generals = make([]General, genCount)
	for i := range s.generals {
		rec := gen[i*generalSize:]
		g := General{
			Index:    i,
			Age:      rec[7],
			Stamina:  rec[8],
			Intel:    rec[9],
			War:      rec[10],
			Charm:    rec[11],
			Rank:     Rank(rec[12]),
			Origin:   rec[13],
			Bond:     int(binary.LittleEndian.Uint16(rec[14:])),
			Debut:    rec[26],
			Loyalty:  rec[16],
			Status:   Status(rec[17]),
			Faction:  rec[18],
			Location: rec[19],
			Troop:    TroopType(rec[21]),
			Soldiers: binary.LittleEndian.Uint16(rec[22:]),
			Training: rec[24],
			Arms:     rec[25],
		}
		copy(g.Raw[:], rec)
		// 姓名：offset 0，6 byte **空白補齊**（2–3 個漢字）。
		// 兩字名前後各補一個空白，三字名剛好填滿。
		field := strings.Trim(string(rec[:6]), " \x00")
		if field != "" {
			if name, err := decodeBig5([]byte(field)); err == nil {
				g.Name = name
				g.IsPerson = allHan(name)
			}
			// **解不出來不當錯誤。** 原版可能有造字（Big5 使用者造字區）。
			// 留空名、IsPerson 為假，保住索引對齊，讓上層決定要不要在意。
		}
		s.generals[i] = g
	}
	return s, nil
}

// Tables 回傳三張表的原始位元組副本。
//
// **存檔要靠它保住還沒解出來的欄位**：州郡表 176 個位元組裡解出 24 個、
// 人物表 30 個裡解出 20 個、諸侯表 72 個裡解出 2 個。存檔時把已知欄位
// 蓋回去、其餘原封不動，未解的東西才不會在存讀一輪之後消失。
func (s *Scenario) Tables() (mas, sta, gen []byte) {
	return append([]byte(nil), s.rawMas...),
		append([]byte(nil), s.rawSta...),
		append([]byte(nil), s.rawGen...)
}

// Prefecture 用 1-based 的郡編號取一個郡。
//
// **0 與越界回錯誤**，不回啞元——把啞元當成一個郡交出去，
// 上層會拿到名字是 "...." 的東西然後照樣算下去。
func (s *Scenario) Prefecture(id int) (Prefecture, error) {
	if id < 1 || id > PrefectureCount {
		return Prefecture{}, fmt.Errorf("state: 郡編號 %d 越界（有效範圍 1..%d）",
			id, PrefectureCount)
	}
	return s.prefectures[id-1], nil
}

// Prefectures 回傳 42 個郡，依編號排序。
func (s *Scenario) Prefectures() []Prefecture { return s.prefectures }

// Employed 回報這位人物有沒有效力對象。
func (g General) Employed() bool { return g.Faction != NoFaction }

// HasLoyalty 回報忠誠欄有沒有意義（在野者沒有）。
func (g General) HasLoyalty() bool { return g.Loyalty != NoValue }

// Masters 回傳 16 個諸侯槽，**含沒在用的**。
func (s *Scenario) Masters() []Master { return s.masters }

// Lord 回傳某個勢力的君主本人。
func (s *Scenario) Lord(faction int) (General, error) {
	if faction < 0 || faction >= len(s.masters) {
		return General{}, fmt.Errorf("state: 勢力 %d 越界（有效範圍 0..%d）",
			faction, len(s.masters)-1)
	}
	idx := s.masters[faction].LordIndex
	if idx < 0 || idx >= len(s.generals) {
		return General{}, fmt.Errorf("state: 勢力 %d 的君主索引 %d 越界", faction, idx)
	}
	return s.generals[idx], nil
}

// ActiveFactions 回傳實際在用的勢力槽號。
//
// **判準從資料推，不硬編 14。** 沒在用的槽指向姓名是全形標點的填充筆，
// 所以「君主是不是人」就是判準——換劇本、換版本都成立。
func (s *Scenario) ActiveFactions() []int {
	owned := map[int]bool{}
	for _, p := range s.prefectures {
		if p.Owned() {
			owned[int(p.Owner)] = true
		}
	}
	var out []int
	for i := range s.masters {
		if g, err := s.Lord(i); err == nil && g.IsPerson {
			out = append(out, i)
			continue
		}
		// **自創君主的諸侯記錄指向人物表的填充筆**（名字是全形標點，
		// `docs/formats/03` §4）。拿「君主是不是人」當唯一判準的話，
		// 讀原版存檔時會把玩家自己排除掉——而錯誤訊息會說那個勢力
		// 「沒有在用」，看起來像存檔壞了。
		//
		// 有領地就是在用。劇本檔那兩個指向填充筆的槽本來就沒有領地，
		// 所以這一條不會讓它們混進來。
		if owned[i] {
			out = append(out, i)
		}
	}
	return out
}

// Territory 回傳某個勢力擁有的郡，依編號排序。
func (s *Scenario) Territory(faction int) []Prefecture {
	var out []Prefecture
	for _, p := range s.prefectures {
		if p.Owned() && int(p.Owner) == faction {
			out = append(out, p)
		}
	}
	return out
}

// Governor 回傳某個郡的主事者。
//
// 常態是身分為君主或太守的那一位；**兩者都不在時由軍師代理**。
// 六個劇本 × 193 個有主的郡全部剛好一位（原版資料裡代理出現兩次：
// 劇本 004 的建業是周瑜、劇本 006 的南陽是司馬懿）。
//
// 郡無主、或找不到唯一的主事者時回 false。**不要回「第一個找到的」**
// ——那會把資料異常變成一個看起來正常的答案。
func (s *Scenario) Governor(prefectureID int) (General, bool) {
	p, err := s.Prefecture(prefectureID)
	if err != nil || !p.Owned() {
		return General{}, false
	}
	var main, deputy []General
	for _, g := range s.generals {
		if !g.Employed() || int(g.Location) != prefectureID || g.Faction != p.Owner {
			continue
		}
		switch {
		case g.Status.Governs():
			main = append(main, g)
		case g.Status == StatusChief:
			deputy = append(deputy, g)
		}
	}
	if len(main) == 1 {
		return main[0], true
	}
	if len(main) == 0 && len(deputy) == 1 {
		return deputy[0], true
	}
	return General{}, false
}

// Retinue 回傳效力於某個勢力的人物，依槽號排序。
func (s *Scenario) Retinue(faction int) []General {
	var out []General
	for _, g := range s.generals {
		if g.Employed() && int(g.Faction) == faction && g.IsPerson {
			out = append(out, g)
		}
	}
	return out
}

// Generals 回傳 350 個槽，**含填充槽**。索引就是原版的槽號。
// 要人物請用 IsPerson 過濾，或用 People。
func (s *Scenario) Generals() []General { return s.generals }

// People 只回傳姓名是漢字的槽（實測 346 個）。
func (s *Scenario) People() []General {
	out := make([]General, 0, len(s.generals))
	for _, g := range s.generals {
		if g.IsPerson {
			out = append(out, g)
		}
	}
	return out
}

// allHan 判斷是不是整串都是漢字。
//
// 這是把人物與填充槽分開的判準。範圍取 CJK 統一表意文字本區
// （U+4E00–U+9FFF）——原版是 Big5，用不到擴充區。
func allHan(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 0x4E00 || r > 0x9FFF {
			return false
		}
	}
	return true
}

// section 取一個容器項目並檢查長度。
//
// 長度不符就回錯誤，不截斷也不補零：長度是這個格式最強的一項驗證
// （7,568 ＝ 43 × 176），對不上代表讀的不是想的那個東西。
func section(c *assets.Container, name string, want int) ([]byte, error) {
	i, ok := c.ByName(name)
	if !ok {
		return nil, fmt.Errorf("state: 容器裡沒有 %s", name)
	}
	b := c.Data(i)
	if len(b) != want {
		return nil, fmt.Errorf("state: %s 長度 %d，想要 %d——讀的不是想的那個東西",
			name, len(b), want)
	}
	return b, nil
}

// decodeBig5 用 cp950 解碼。
//
// **不要用 big5**：那個對照表缺常用擴充字與使用者造字區，
// 而智冠這個年代的遊戲常有造字。
func decodeBig5(b []byte) (string, error) {
	// 每次建一個 decoder。共用的 *encoding.Decoder 帶狀態、不是並行安全的，
	// 而這裡一個劇本只跑幾百次，省下來的沒有意義。
	out, err := traditionalchinese.Big5.NewDecoder().Bytes(b)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// AILevel 是勢力的電腦諸侯等級（`BASEMAS` offset 4，`L0`、`[base]`）。
//
// 原版用它當索引挑一整套行為：十八張指令分派表，每張八個 far pointer
// （`docs/re/03` §1.4）。**進分派器之前會被夾在 0–5**，所以有效範圍是
// 六級；表的第 7、8 格是空操作，只為了把表補成 2 的冪。
//
// 劇本 001 的存檔裡：槽 0–8 是 5，槽 9–14 是 4。
func (s *Scenario) AILevel(faction int) int {
	off := faction*masterSize + 4
	if faction < 0 || off+1 >= len(s.rawMas) {
		return 0
	}
	v := int(binary.LittleEndian.Uint16(s.rawMas[off:]))
	if v < 0 {
		return 0
	}
	if v > 5 {
		return 5
	}
	return v
}

// Prestige 是諸侯的人望（`BASEMAS` offset 8，`L1`、`[base]`）。
//
// 對照的是原版畫面：郡的資訊欄寫「君主〇〇〇　人望 87」，而該勢力的
// offset 8 就是 87。
func (s *Scenario) Prestige(faction int) int {
	off := faction*masterSize + 8
	if faction < 0 || off >= len(s.rawMas) {
		return 0
	}
	return int(s.rawMas[off])
}

// TreasuryOf 是諸侯寶庫裡五種寶物的數量（`BASEMAS` offset 14–18）。
//
// **順序是量到的**（`L0`）：進貢那一段（`0x17301`–`0x17341`）跑
// `i = 0..3`，把第 i 種的件數加進 `諸侯記錄[15 + i]`，而同一支常式印的
// 訊息是 `DS:0x6825`「兵書:%d 寶刀:%d 美女:%d 駿馬:%d」——**同一個
// 迴圈變數同時決定欄位與標籤**，所以對應關係不是推的。
//
// offset 14 是玉璽：它不在進貢的四種裡，而且在十六個槽裡只有一個是 1
// （`0x15175` 的玉璽現世把它設成 1，`docs/re/06` §7.1）。
//
// **每一種都夾在 100**（`0x1731d`）。
func (s *Scenario) TreasuryOf(faction int) [5]int {
	var out [5]int
	base := faction*masterSize + 14
	if faction < 0 || base+5 > len(s.rawMas) {
		return out
	}
	for i := range out {
		out[i] = int(s.rawMas[base+i])
	}
	return out
}

// Controller 說一個勢力由誰控制（`BASEMAS` offset 0，`L0`／`L1`）。
//
// 原版的電腦諸侯常式（線性 `0xd652`）第一件事就是
// `cmpw es:[bx+0x0], 1`，等於 1 就直接返回——**玩家的勢力不跑 AI**。
//
// 資料側對得上：存檔裡玩家那個槽是 1、其餘在用的勢力是 2、
// 沒有領地的三個槽是 `0xFFFF`。
const (
	ControlledByPlayer   = 1
	ControlledByComputer = 2
	ControlledByNobody   = 0xFFFF
)

// Controller 回傳勢力的控制者。
func (s *Scenario) Controller(faction int) int {
	off := faction * masterSize
	if faction < 0 || off+1 >= len(s.rawMas) {
		return ControlledByNobody
	}
	return int(binary.LittleEndian.Uint16(s.rawMas[off:]))
}

// Players 回傳由玩家控制的勢力槽號。
//
// 原版問「請問有幾人玩(0-16)」，所以**可以不只一個**；
// 零個就是電腦自動示範模式。
func (s *Scenario) Players() []int {
	var out []int
	for i := range s.masters {
		if s.Controller(i) == ControlledByPlayer {
			out = append(out, i)
		}
	}
	return out
}

// ChiefIndex 是勢力的軍師（`BASEMAS` offset 6，`u16`、`L0`）。
//
// 原版的「指定軍師」常式（線性 `0xd7ae`）讀它當門檻：現任軍師的智就是
// 換人的下限，**沒有軍師時（`0xFFFF`）門檻是 79**——那正是說明書
// 「受封軍師之人謀略不得低於 80」。
func (s *Scenario) ChiefIndex(faction int) int {
	off := faction*masterSize + 6
	if faction < 0 || off+1 >= len(s.rawMas) {
		return NoValue16
	}
	return int(binary.LittleEndian.Uint16(s.rawMas[off:]))
}

// ChiefIntelFloor 是「換軍師」的智力下限（`L0`、`[base]`）。
const ChiefIntelFloor = 79

// AICostNumerator／AICostDenominator 是電腦諸侯的花費係數（`L0`、`[base]`）。
//
// 原版的「扣錢」常式（線性 `0xec24`）把金額乘上一個以 AI 等級索引的
// 浮點係數再取整。表在記憶體 `0x048074` 起的六個 double：
//
//	等級 0–3 → 1、等級 4 → 0.9、等級 5 → 0.75
//
// 也就是**等級越高，同一個動作花的錢越少**。等級 5 量到 19 個樣本，
// 每一個都與「×0.75 截斷」相符（`internal/parity` 的 `TestZZSpend`）。
var (
	aiCostNum = [6]int{1, 1, 1, 1, 9, 3}
	aiCostDen = [6]int{1, 1, 1, 1, 10, 4}
)

// AICost 是等級 level 的電腦諸侯付一個 base 要多少（截斷取整）。
func AICost(base, level int) int {
	if level < 0 {
		level = 0
	}
	if level >= len(aiCostNum) {
		level = len(aiCostNum) - 1
	}
	return base * aiCostNum[level] / aiCostDen[level]
}

// GovernorIndex 是郡的**主事者**在人物表的槽號（`BASESTA` offset 32，
// `u16`、`L0`），`NoValue16` ＝ 沒有。
//
// 原版直接讀這一格，不從人物表推：用計的前置條件（`0x2c270`）取出
// 那個人再看身分是不是君主，五種謀略把它當參數往下傳。開局之後
// 十三處會寫它，所以它是執行期維護的狀態（`docs/spec/003` §3.4）。
func (s *Scenario) GovernorIndex(prefectureID int) int {
	i := prefectureID - 1 // Prefectures() 是 1..42，切片從 0 起
	if i < 0 || i >= len(s.prefectures) {
		return NoValue16
	}
	return int(s.prefectures[i].Governor)
}

// NoValue16 是 16 位元欄位的「沒有」。
const NoValue16 = 0xFFFF
