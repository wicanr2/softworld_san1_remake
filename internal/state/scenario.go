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

// Prefecture 是一個郡。
type Prefecture struct {
	// ID 是郡編號，**1..42**。原版的筆 0 是啞元，所以編號與筆號相同，
	// 也與手冊用 1–42 稱呼州郡一致。
	ID   int
	Name string // "遼東"
	Raw  [prefSize]byte
}

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

	// IsPerson 為真表示 Name 全部是漢字。這是目前唯一能把人物與填充槽
	// 分開的判準，而且是從資料本身推的，不是從槽號硬編的。
	IsPerson bool

	Raw [generalSize]byte
}

// Master 是一位諸侯。姓名不在 offset 0，這一版不解任何欄位。
type Master struct {
	Index int
	Raw   [masterSize]byte
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
		s.prefectures[i] = p
	}

	s.generals = make([]General, genCount)
	for i := range s.generals {
		rec := gen[i*generalSize:]
		g := General{Index: i}
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

// Masters 回傳 16 位諸侯。
func (s *Scenario) Masters() []Master { return s.masters }

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
