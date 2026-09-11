// Package save 是存讀檔（說明書 p.25：「其他 → 儲存」，六個進度）。
//
// 一個存檔槽是一個目錄，裡面四個檔案：
//
//	SV1/BASEMAS.SV1   諸侯表 16 × 72
//	SV1/BASESTA.SV1   州郡表 43 × 176
//	SV1/BASEGEN.SV1   人物表 350 × 30
//	SV1/BASEPRO.SV1   年月、難度、選項、月內進度 256 B
//	SV1/BASEPRE.SV1   自創君主的十二個字模 384 B
//	SV1/REMAKE.JSON   前五個放不下的東西
//	SAVENAME.SVP      六個進度共用的名稱 126 B
//
// 前五個**與原版的版面逐位元組相同**（`docs/formats/03`、`docs/formats/05`）。這樣做的
// 理由有兩個：解出來的欄位可以直接與原版的存檔對拍；還沒解出來的
// 欄位原封不動帶著走，不會因為存過一次檔就被我們洗掉。
//
// ⚠ **不寫回原版的容器。** 原版的存檔是 `DATA2.GRP` 裡的項目，
// 要存就得改寫玩家自己的原版檔案。那是別人的東西，而且一旦寫壞
// 就沒有第二份。remake 的存檔一律放自己的目錄。
package save

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// Slots 是幾個存檔槽。**這個有出處**：說明書 p.25「儲存進度數 6 個」。
const Slots = 6

// meta 是 REMAKE.JSON 的內容。
//
// 版本號是為了讓將來讀得出「這份存檔是哪一版寫的」。**讀不懂要報錯，
// 不要盡力而為**——一份被半懂的程式讀進來的存檔，玩下去才會出事。
type meta struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	Slot    string `json:"slot"`
	SavedAt string `json:"saved_at"`

	Year       int             `json:"year"`
	Month      int             `json:"month"`
	Player     state.FactionID `json:"player"`
	Edition    state.Edition   `json:"edition,omitempty"`
	Difficulty int             `json:"difficulty"`

	Prefectures []prefMeta          `json:"prefectures"`
	Rewarded    []int               `json:"rewarded"`
	Factions    map[string]factMeta `json:"factions"`
	Options     optMeta             `json:"options"`
}

// optMeta 是「其他」底下的開關。
//
// **要存**：玩家關掉音樂之後每次讀檔又響起來，那與沒存過設定是同一件事。
type optMeta struct {
	MusicOff  bool `json:"music_off"`
	SoundOff  bool `json:"sound_off"`
	VoiceOff  bool `json:"voice_off"`
	SkipAIWar bool `json:"skip_ai_war"`
	Calendar  int  `json:"calendar"`
	Delay     int  `json:"delay"`
}

type prefMeta struct {
	// Forts 留著只為讀得懂舊存檔；新的一律以 BASESTA offset 25 為準。
	Forts int `json:"forts,omitempty"`
	// Autonomy 留著只為讀得懂舊存檔；新的一律以 BASESTA offset 12 為準。
	Autonomy  int  `json:"autonomy,omitempty"`
	Commanded bool `json:"commanded"`

	// Population 是精確人口。原版的州郡表只放得下百位，見
	// `game.PrefectureExtra.Population`。
	Population int `json:"population"`
}

type factMeta struct {
	Alive    bool  `json:"alive"`
	Chief    int   `json:"chief"`
	Treasury []int `json:"treasury"`
}

// version 是目前寫出來的格式版本。
const version = 1

// Info 是一個存檔槽的概況，給讀檔選單用。
type Info struct {
	Slot   int
	Exists bool

	// Name 是存檔名稱（原版的 `SAVENAME.SVP`，remake 放在 JSON 裡）。
	Name    string
	Year    int
	Month   int
	SavedAt string
}

// dirFor 是一個槽的目錄名。
func dirFor(root string, slot int) string {
	return filepath.Join(root, fmt.Sprintf("SV%d", slot))
}

func checkSlot(slot int) error {
	if slot < 1 || slot > Slots {
		return fmt.Errorf("save: 存檔槽 %d 越界（原版有 %d 個）", slot, Slots)
	}
	return nil
}

// Write 把一局存進某個槽。
//
// 先寫到暫存檔再改名：**存檔寫到一半當掉，玩家會失去的是上一次的進度**，
// 那比沒存到嚴重得多。
func Write(root string, slot int, g *game.State, name string) error {
	if err := checkSlot(slot); err != nil {
		return err
	}
	mas, sta, gen, err := g.Tables()
	if err != nil {
		return err
	}
	e := g.CaptureExtra()
	m := meta{
		Version: version, Name: name, Slot: string(g.Slot),
		SavedAt:    time.Now().Format(time.RFC3339),
		Year:       e.Year,
		Month:      e.Month,
		Player:     e.Player,
		Edition:    e.Edition,
		Difficulty: e.Difficulty,
		Factions:   map[string]factMeta{},
	}
	for _, p := range e.Prefectures {
		m.Prefectures = append(m.Prefectures, prefMeta{
			Commanded:  p.Commanded,
			Population: p.Population,
		})
	}
	m.Rewarded = append([]int(nil), e.Rewarded...)
	sort.Ints(m.Rewarded)
	m.Options = optMeta{
		MusicOff: e.Options.MusicOff, SoundOff: e.Options.SoundOff,
		VoiceOff: e.Options.VoiceOff, SkipAIWar: e.Options.SkipAIWar,
		Calendar: int(e.Options.Calendar), Delay: e.Options.Delay(),
	}
	for id, f := range e.Factions {
		m.Factions[fmt.Sprint(id)] = factMeta{f.Alive, f.Chief,
			append([]int(nil), f.Treasury[:]...)}
	}
	blob, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	dir := dirFor(root, slot)
	tmp := dir + ".tmp"
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	suffix := fmt.Sprintf("SV%d", slot)
	files := []struct {
		name string
		data []byte
	}{
		{"BASEMAS." + suffix, mas},
		{"BASESTA." + suffix, sta},
		{"BASEGEN." + suffix, gen},
		{"BASEPRO." + suffix, buildProgress(e, readProgress(dirFor(root, slot), slot)).Encode()},
		{"BASEPRE." + suffix, glyphsFor(dirFor(root, slot), slot).Encode()},
		{"REMAKE.JSON", blob},
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmp, f.name), f.data, 0o644); err != nil {
			return fmt.Errorf("save: 寫 %s：%w", f.name, err)
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.Rename(tmp, dir); err != nil {
		return err
	}
	return writeName(root, slot, name)
}

// writeName 把名稱寫進共用的 `SAVENAME.SVP`。
//
// **六個進度共用一個檔**，所以要先讀回來再改一格；整份重寫會把另外
// 五個槽的名稱清掉，而那在畫面上只會表現成「別的存檔忽然沒有名字」。
func writeName(root string, slot int, name string) error {
	names := readNames(root)
	names[slot-1] = name
	b, err := state.EncodeSaveNames(names)
	if err != nil {
		return err
	}
	tmp := filepath.Join(root, "SAVENAME.tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(root, saveNameFile))
}

// saveNameFile 是名稱表的檔名，照原版。
const saveNameFile = "SAVENAME.SVP"

// readNames 讀共用的名稱表；讀不到或壞掉就回六個空字串。
//
// **壞掉不當錯誤**：名稱是給人看的，讀不出來頂多顯示「無名」，
// 不該讓整局存不進去。
func readNames(root string) []string {
	b, err := os.ReadFile(filepath.Join(root, saveNameFile))
	if err == nil {
		if names, err := state.DecodeSaveNames(b); err == nil {
			return names
		}
	}
	return make([]string, Slots)
}

// readProgress 讀某個槽上一次寫出去的 `BASEPRO`；沒有就回 nil。
func readProgress(dir string, slot int) *state.Progress {
	b, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("BASEPRO.SV%d", slot)))
	if err != nil {
		return nil
	}
	p, err := state.DecodeProgress(b)
	if err != nil {
		return nil
	}
	return p
}

// glyphsFor 讀某個槽上一次寫出去的字模；沒有就回一組空的。
//
// **自創君主的字模還沒接**（`docs/spec/013` R4）：remake 做得出自創君主
// 了，但名字的字模還沒寫進來，所以這裡多半是空的——**空的照樣要寫**，
// 否則存檔目錄與原版的項目對不齊，將來要比對就少一份。
func glyphsFor(dir string, slot int) *state.Glyphs {
	b, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("BASEPRE.SV%d", slot)))
	if err == nil {
		if g, err := state.DecodeGlyphs(b); err == nil {
			return g
		}
	}
	return &state.Glyphs{}
}

// Read 讀一個槽。
func Read(root string, slot int) (*game.State, error) {
	if err := checkSlot(slot); err != nil {
		return nil, err
	}
	dir := dirFor(root, slot)
	blob, err := os.ReadFile(filepath.Join(dir, "REMAKE.JSON"))
	if err != nil {
		return nil, fmt.Errorf("save: 讀存檔 %d：%w", slot, err)
	}
	var m meta
	if err := json.Unmarshal(blob, &m); err != nil {
		return nil, fmt.Errorf("save: 存檔 %d 的 REMAKE.JSON 解不開：%w", slot, err)
	}
	if m.Version != version {
		return nil, fmt.Errorf("save: 存檔 %d 是格式第 %d 版，這個程式讀第 %d 版",
			slot, m.Version, version)
	}
	suffix := fmt.Sprintf("SV%d", slot)
	mas, err := os.ReadFile(filepath.Join(dir, "BASEMAS."+suffix))
	if err != nil {
		return nil, err
	}
	sta, err := os.ReadFile(filepath.Join(dir, "BASESTA."+suffix))
	if err != nil {
		return nil, err
	}
	gen, err := os.ReadFile(filepath.Join(dir, "BASEGEN."+suffix))
	if err != nil {
		return nil, err
	}
	sc, err := state.DecodeTables(state.Slot(m.Slot), mas, sta, gen)
	if err != nil {
		return nil, err
	}

	e := game.Extra{
		Year: m.Year, Month: m.Month,
		Player: m.Player, Edition: m.Edition, Difficulty: m.Difficulty,
		Rewarded: append([]int(nil), m.Rewarded...),
		Factions: map[state.FactionID]game.FactionExtra{},
	}
	for _, p := range m.Prefectures {
		e.Prefectures = append(e.Prefectures, game.PrefectureExtra{
			Autonomy: game.Autonomy(p.Autonomy), Commanded: p.Commanded,
			Population: p.Population,
		})
	}
	e.Options = game.Options{
		MusicOff: m.Options.MusicOff, SoundOff: m.Options.SoundOff,
		VoiceOff: m.Options.VoiceOff, SkipAIWar: m.Options.SkipAIWar,
		Calendar: game.Calendar(m.Options.Calendar),
	}
	if err := e.Options.SetDelay(m.Options.Delay); err != nil {
		return nil, fmt.Errorf("save: 存檔 %d 的延時：%w", slot, err)
	}
	// `BASEPRO` 有的欄位以它為準——**同一個值不要有兩個真相**。
	// 舊存檔沒有這個檔，那時 JSON 就是唯一來源。
	if p := readProgress(dir, slot); p != nil {
		if err := applyProgress(&e, p); err != nil {
			return nil, fmt.Errorf("save: 存檔 %d：%w", slot, err)
		}
	}
	for k, f := range m.Factions {
		var id int
		if _, err := fmt.Sscanf(k, "%d", &id); err != nil {
			return nil, fmt.Errorf("save: 存檔 %d 的勢力鍵 %q 不是數字", slot, k)
		}
		fx := game.FactionExtra{Alive: f.Alive, Chief: f.Chief}
		copy(fx.Treasury[:], f.Treasury)
		e.Factions[state.FactionID(id)] = fx
	}
	return game.Restore(sc, e)
}

// List 回傳六個槽的概況，空槽也列出來。
//
// **空槽要列出來**：原版的儲存畫面就是六格，看得到哪幾格是空的。
func List(root string) []Info {
	out := make([]Info, 0, Slots)
	names := readNames(root)
	for slot := 1; slot <= Slots; slot++ {
		info := Info{Slot: slot}
		blob, err := os.ReadFile(filepath.Join(dirFor(root, slot), "REMAKE.JSON"))
		if err == nil {
			var m meta
			if json.Unmarshal(blob, &m) == nil && m.Version == version {
				info.Exists = true
				info.Name = m.Name
				info.Year, info.Month = m.Year, m.Month
				info.SavedAt = m.SavedAt
				if p := readProgress(dirFor(root, slot), slot); p != nil {
					info.Year, info.Month = p.Year, p.Month
				}
				if n := names[slot-1]; n != "" {
					info.Name = n
				}
			}
		}
		out = append(out, info)
	}
	return out
}

// Describe 是一行給選單看的說明。
func (i Info) Describe() string {
	if !i.Exists {
		return fmt.Sprintf("%d. （空）", i.Slot)
	}
	name := strings.TrimSpace(i.Name)
	if name == "" {
		name = "無名"
	}
	return fmt.Sprintf("%d. %s　%d 年 %d 月", i.Slot, name, i.Year, i.Month)
}
