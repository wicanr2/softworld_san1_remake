// Package boot 是原版的啟動裝置選擇。
//
// 原版在進遊戲前問三題，每題只收 `1` 或 `2`（`docs/re/00` 第四輪，`L0`：
// 反組譯 `0x290bd` 的 `cmp al,0x31` ／ `cmp al,0x32`，並以 dosgolem
// 實測 98 個可打的鍵只有這兩個被接受）。
//
// remake 保留這個流程，因為**它決定後面讀哪一份資料**：
// 顯示卡的選擇決定用 `EGAFILL.PAL` 還是 `HERCFILL.PAL`
// （`docs/formats/01` §3），那是規則以外的真實分支。
package boot

import "fmt"

// Device 是要選的裝置種類。順序就是原版問的順序。
type Device int

const (
	Music Device = iota
	Graphic
	Disk
)

// Choice 是一個選項。Key 是原版收的按鍵。
type Choice struct {
	Key  byte
	Name string
}

// Prompt 是一題。
type Prompt struct {
	Device  Device
	Label   string
	Choices []Choice
}

// Prompts 是三題，順序與原版相同。
//
// 選項名稱用原版印出來的英文（`Music`／`AdLib`／`Hercules`／`EGA`…），
// **不自己翻譯**：這是裝置名不是遊戲文字，而且對拍要拿它跟原版的
// 主控台輸出比。要顯示中文是 UI 層的事，不是這裡。
func Prompts() []Prompt {
	return []Prompt{
		{Music, "Music", []Choice{{'1', "No"}, {'2', "AdLib"}}},
		{Graphic, "Graphic", []Choice{{'1', "Hercules"}, {'2', "EGA"}}},
		// ⚠ 原版把 Floppy 拼成 "Floopy"。**不要改正。** 這是保存專案，
		// 原版怎麼寫就怎麼寫；改正等於在還原品上留下一處與原版不同而
		// 沒人記得的地方。對拍測試（internal/parity）盯著這個字串，
		// 有人「順手修好」會當場紅。
		{Disk, "Disk", []Choice{{'1', "Floopy"}, {'2', "HardDisk"}}},
	}
}

// Config 是選完的結果。零值不是合法設定——三項都要選過。
type Config struct {
	choice map[Device]byte
}

// NewConfig 開一份還沒選的設定。
func NewConfig() *Config { return &Config{choice: map[Device]byte{}} }

// Set 記一個選擇。**只收原版收的按鍵**。
//
// 原版對其他按鍵的行為是「重問」，不是「用預設值繼續」——
// 這裡回錯誤讓呼叫端自己決定要不要重問，但不會悄悄接受。
func (c *Config) Set(d Device, key byte) error {
	for _, p := range Prompts() {
		if p.Device != d {
			continue
		}
		for _, ch := range p.Choices {
			if ch.Key == key {
				c.choice[d] = key
				return nil
			}
		}
		return fmt.Errorf("boot: %s 不收按鍵 %q（只收 1 或 2）", p.Label, key)
	}
	return fmt.Errorf("boot: 沒有這個裝置 %d", d)
}

// Get 回傳選了哪一項；還沒選回 false。
func (c *Config) Get(d Device) (Choice, bool) {
	key, ok := c.choice[d]
	if !ok {
		return Choice{}, false
	}
	for _, p := range Prompts() {
		if p.Device != d {
			continue
		}
		for _, ch := range p.Choices {
			if ch.Key == key {
				return ch, true
			}
		}
	}
	return Choice{}, false
}

// Complete 判斷三題是否都選過。
func (c *Config) Complete() bool { return len(c.choice) == len(Prompts()) }

// Palette 回傳這份設定要用的調色盤資源名。
//
// 這是裝置選擇**真正影響資料讀取**的地方：`docs/formats/01` §3 量到
// `DATA1` 裡同時有 `EGAFILL.PAL` 與 `HERCFILL.PAL`。
func (c *Config) Palette() (string, error) {
	ch, ok := c.Get(Graphic)
	if !ok {
		return "", fmt.Errorf("boot: 還沒選顯示卡")
	}
	switch ch.Name {
	case "EGA":
		return "EGAFILL.PAL", nil
	case "Hercules":
		return "HERCFILL.PAL", nil
	}
	return "", fmt.Errorf("boot: 不認識的顯示卡 %q", ch.Name)
}

// KeySequence 回傳「照這份設定把三題答完」要送的按鍵。
//
// 對拍要用它：把同一串鍵送進原版，兩邊才是同一個狀態。
func (c *Config) KeySequence() (string, error) {
	if !c.Complete() {
		return "", fmt.Errorf("boot: 還有題目沒選")
	}
	s := ""
	for _, p := range Prompts() {
		s += string(c.choice[p.Device])
	}
	return s, nil
}
