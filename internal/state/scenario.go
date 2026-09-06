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
//（`CLAUDE.md` §7 第 11 條）；郡的 Owner 與人物的 Faction 都要在
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
	StatusLord     Status = 0  // 君主
	StatusChief    Status = 1  // 軍師
	StatusGovernor Status = 2  // 太守
	StatusOfficer  Status = 3  // 一般武將
	// StatusAvailable 是「在野而且在該郡露面」。**只有這一種算進
	// 郡的在野武將數。**
	StatusAvailable Status = 8
	StatusIdle      Status = 9  // 在野，不列入郡的在野數
	StatusUnborn    Status = 11 // 還沒登場（此時諸葛亮 8 歲）
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

	PublicLoyalty uint8 // 民眾忠誠
	LandValue     uint8 // 土地價值
	FloodRate     uint8 // 洪水率
	PriceLevel    uint8 // 物價

	// Neighbours 是相鄰的郡編號，最多 8 個（原版 offset 45–52，`0xFF` 補齊）。
	//
	// 這是**地圖幾何不是劇本狀態**：十二個槽位（六個劇本 ＋ 六個存檔）
	// 的相鄰表完全相同。整張圖對稱（95 條邊、零條單向）而且全連通。
	Neighbours []int

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
	Age      uint8 // 年齡
	Stamina  uint8 // 體能
	Intel    uint8 // 謀略
	War      uint8 // 戰力
	Charm    uint8 // 魅力
	Rank     Rank  // 職位
	Origin   uint8 // 出身郡（1..42）
	// Loyalty 是忠誠。**在野者是 0xFF（NoValue）不是 0**——沒有主子就
	// 沒有忠誠可言。實測 346 位裡 227 位是 0xFF，而且全部沒有勢力；
	// 有勢力者的值域是 52–100。當成數值算下去會得到「在野者忠誠 255」。
	Loyalty uint8
	Status   Status
	Troop    TroopType // 兵種
	Soldiers uint16    // 兵士數
	Training uint8     // 訓練度
	Arms     uint8     // 武裝度

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
//（`docs/spec/003` §2）。16 個槽裡有兩個指向姓名是全形標點的填充筆，
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
}

// LoadScenario 從 DATA2 容器讀一個槽。
//
// 定位一律用容器項目名，**不用絕對位移**——位移會隨版本漂移
// （`DATA2.GRP` 兩版等長但內容不同），項目名不會。
func LoadScenario(c *assets.Container, slot Slot) (*Scenario, error) {
	s := &Scenario{Slot: slot}

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
		// 郡名：offset 0，4 byte Big5 ＋ 1 byte NUL。
		name, err := decodeBig5(rec[:4])
		if err != nil {
			return nil, fmt.Errorf("state: 郡 %d 的名稱解不出來：%w", p.ID, err)
		}
		p.Name = name
		p.Population = binary.LittleEndian.Uint16(rec[14:])
		p.Soldiers = binary.LittleEndian.Uint16(rec[16:])
		p.Gold = binary.LittleEndian.Uint16(rec[18:])
		p.Rice = binary.LittleEndian.Uint16(rec[20:])
		p.ActiveGenerals = rec[22]
		p.FreeGenerals = rec[23]
		p.PublicLoyalty = rec[26]
		p.LandValue = rec[27]
		p.FloodRate = rec[28]
		p.PriceLevel = rec[29]
		p.Owner = rec[30]
		for _, b := range rec[45:53] {
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
	var out []int
	for i := range s.masters {
		if g, err := s.Lord(i); err == nil && g.IsPerson {
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
