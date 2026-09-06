// Package music 讀原版的配樂與音色資料。
//
// 資料在 `DATA1` 容器裡的四個項目：`MUS.IDX`／`MUS.GRP` 是五首配樂，
// `MUSV.IDX`／`MUSV.GRP` 是另外一首長曲。兩者都是**巢狀容器**——
// `.IDX` 每項四個位元組是結束位移，與外層容器同一套（`docs/formats/01`）。
//
// 項目成對出現：偶數項是曲子，奇數項是那首曲子用的音色庫。
// 開機主選單列的五首（思古／小徑／風雲／戰鼓／末路，`docs/re/04` §10）
// 就是 `MUS` 的五對。
//
// ⚠ **本套件不含任何原版資料**，也不散布解出來的內容。它讀的是
// 玩家自己那一份。
package music

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// 曲子的表頭版面。出處是把六首曲子的前 0x46 個位元組排在一起比對
// （`docs/formats/06`）：整批只有這幾個位移非零，而且 `dataLen`
// 對每一首都等於「項目長度 − 0x46」。
const (
	hdrVersion = 0x00 // uint16，全部是 1
	hdrMagic   = 0x24 // uint16，全部是 0x04F0
	hdrTicks   = 0x26 // uint32，全曲長度（tick）
	hdrDataLen = 0x2A // uint32，MIDI 資料的位元組數
	hdrEvents  = 0x2E // uint32，事件數
	hdrTempo   = 0x3C // uint8，速度（BPM）
	hdrSize    = 0x46 // MIDI 資料的起點

	// TicksPerBeat 是一拍幾個 tick。
	//
	// **這是推出來的**：事件流裡最常見的間隔是 0x78＝120，而 120 是
	// MIDI 檔常用的每四分音符 tick 數。用它換算出來的曲長（一分多鐘到
	// 五分半）也合理。要確認得聽原版。
	TicksPerBeat = 120

	// magic 是每一首都有的固定值。**對不上就不要硬解**——
	// 一個把別的資料當成曲子解的程式會吐出一堆看似合理的音符。
	magic = 0x04F0
)

// Song 是一首曲子。
type Song struct {
	// Tempo 是速度（BPM），Ticks 是全曲長度。
	Tempo int
	Ticks int

	// Events 是照時間排好的 MIDI 事件。
	Events []Event
}

// Event 是一個帶時間的 MIDI 事件。
type Event struct {
	// At 是絕對時間（tick）。
	At int

	// Status 是 MIDI 狀態位元組（含頻道）；Data 是它的參數。
	// SysEx（0xF0）的 Data 是 0xF7 之前的全部內容。
	Status byte
	Data   []byte
}

// Channel 是事件的 MIDI 頻道（0..15）。狀態位元組 ≥ 0xF0 時沒有頻道，回 −1。
func (e Event) Channel() int {
	if e.Status >= 0xF0 {
		return -1
	}
	return int(e.Status & 0x0F)
}

// Kind 是事件的種類（狀態位元組的高四位元）。
func (e Event) Kind() byte { return e.Status & 0xF0 }

// MIDI 事件種類。
const (
	NoteOff         = 0x80
	NoteOn          = 0x90
	KeyPressure     = 0xA0
	ControlChange   = 0xB0
	ProgramChange   = 0xC0
	ChannelPressure = 0xD0
	PitchBend       = 0xE0
	SysEx           = 0xF0
)

// Duration 是全曲長度（秒）。
func (s *Song) Duration() float64 {
	if s.Tempo <= 0 {
		return 0
	}
	return float64(s.Ticks) / TicksPerBeat * 60 / float64(s.Tempo)
}

// Instrument 是一個音色。
//
// 名稱是 AdLib Visual Composer 的標準音色名（`piano1`、`oboe1`、
// `bdrum1`…），參數是 28 個 16 位元欄位——那正是 AdLib 的
// 音色結構，每個位元組欄位存成一個 word。
type Instrument struct {
	Name string
	Raw  [InstrumentSize]byte
}

// InstrumentSize 是一個音色佔幾個位元組。
const InstrumentSize = 56

// Param 取第 i 個參數（0..27）。
func (in Instrument) Param(i int) int {
	if i < 0 || i*2+1 >= len(in.Raw) {
		return 0
	}
	return int(binary.LittleEndian.Uint16(in.Raw[i*2:]))
}

// Bank 是一首曲子用的音色庫。
type Bank struct {
	Instruments []Instrument
}

// Find 依名稱取一個音色。
func (b *Bank) Find(name string) (Instrument, bool) {
	for _, in := range b.Instruments {
		if in.Name == name {
			return in, true
		}
	}
	return Instrument{}, false
}

// Track 是一首曲子加上它的音色庫。
type Track struct {
	Song *Song
	Bank *Bank
}

// nameLen 是音色名稱欄位的長度。
const nameLen = 9

// bankHeader 是音色庫的表頭：版本、音色數、資料起點。
const (
	bankVersion = 0x00 // uint16
	bankCount   = 0x02 // uint16
	bankOffset  = 0x04 // uint16，等於 6 + 9×count
	bankHdrSize = 6
)

// ParseBank 解一個音色庫。
func ParseBank(b []byte) (*Bank, error) {
	if len(b) < bankHdrSize {
		return nil, fmt.Errorf("music: 音色庫只有 %d 個位元組", len(b))
	}
	n := int(binary.LittleEndian.Uint16(b[bankCount:]))
	off := int(binary.LittleEndian.Uint16(b[bankOffset:]))
	if n <= 0 || off != bankHdrSize+nameLen*n {
		return nil, fmt.Errorf("music: 音色庫的表頭對不上（%d 個音色、資料在 %d）", n, off)
	}
	if want := off + InstrumentSize*n; len(b) != want {
		return nil, fmt.Errorf("music: 音色庫有 %d 個位元組，%d 個音色應該是 %d",
			len(b), n, want)
	}
	bank := &Bank{Instruments: make([]Instrument, n)}
	for i := 0; i < n; i++ {
		in := &bank.Instruments[i]
		in.Name = strings.TrimRight(string(b[bankHdrSize+i*nameLen:bankHdrSize+(i+1)*nameLen]), "\x00")
		copy(in.Raw[:], b[off+i*InstrumentSize:])
	}
	return bank, nil
}

// ParseSong 解一首曲子。
//
// **事件數要與表頭吻合**才回傳成功：走完事件流卻數不對，表示版面
// 讀錯了——而一個讀錯版面的解析器照樣會吐出一串看似合理的音符。
func ParseSong(b []byte) (*Song, error) {
	if len(b) < hdrSize {
		return nil, fmt.Errorf("music: 曲子只有 %d 個位元組", len(b))
	}
	if m := binary.LittleEndian.Uint16(b[hdrMagic:]); m != magic {
		return nil, fmt.Errorf("music: 表頭的固定值是 %#04x，應該是 %#04x", m, magic)
	}
	dataLen := int(binary.LittleEndian.Uint32(b[hdrDataLen:]))
	if want := len(b) - hdrSize; dataLen != want {
		return nil, fmt.Errorf("music: 表頭說資料有 %d 個位元組，實際有 %d", dataLen, want)
	}
	s := &Song{
		Tempo: int(b[hdrTempo]),
		Ticks: int(binary.LittleEndian.Uint32(b[hdrTicks:])),
	}
	want := int(binary.LittleEndian.Uint32(b[hdrEvents:]))
	events, err := parseEvents(b[hdrSize:])
	if err != nil {
		return nil, err
	}
	if len(events) != want {
		return nil, fmt.Errorf("music: 表頭說有 %d 個事件，走出來 %d 個", want, len(events))
	}
	s.Events = events
	return s, nil
}

// parseEvents 走一段 MIDI 事件流。
//
// 每個事件是「可變長度的時間差 ＋ 狀態位元組 ＋ 參數」。
// **running status 要支援**：省略狀態位元組沿用上一個，是 MIDI 的常規，
// 不支援的話會把資料位元組當成狀態而整串走歪。
func parseEvents(b []byte) ([]Event, error) {
	var out []Event
	at, i := 0, 0
	var running byte
	for i < len(b) {
		delta, n, err := varLen(b[i:])
		if err != nil {
			return nil, fmt.Errorf("music: 位移 %d 的時間差：%w", i, err)
		}
		i += n
		at += delta
		if i >= len(b) {
			return nil, fmt.Errorf("music: 位移 %d 之後沒有事件了", i)
		}
		status := b[i]
		if status&0x80 != 0 {
			i++
			switch {
			case status < 0xF0:
				running = status
			case status >= 0xF8:
				// 即時訊息（0xF8 時脈、0xFC 停止…）**不影響 running status**，
				// 這是 MIDI 的規矩。清掉的話後面那個省略狀態的事件就沒得沿用。
			default:
				running = 0
			}
		} else {
			if running == 0 {
				return nil, fmt.Errorf("music: 位移 %d 是 %#02x，而且沒有可沿用的狀態", i, status)
			}
			status = running
		}
		size, err := eventSize(status)
		if err != nil {
			return nil, fmt.Errorf("music: 位移 %d：%w", i, err)
		}
		if status >= 0xF8 || status == 0xF6 || status == 0xF7 {
			// 單一位元組的訊息，沒有參數。
			out = append(out, Event{At: at, Status: status})
			continue
		}
		if status == SysEx {
			end := i
			for end < len(b) && b[end] != 0xF7 {
				end++
			}
			if end >= len(b) {
				return nil, fmt.Errorf("music: 位移 %d 起的 SysEx 沒有結尾", i)
			}
			out = append(out, Event{At: at, Status: status,
				Data: append([]byte(nil), b[i:end]...)})
			i = end + 1
			continue
		}
		if i+size > len(b) {
			return nil, fmt.Errorf("music: 位移 %d 的事件 %#02x 少了參數", i, status)
		}
		out = append(out, Event{At: at, Status: status,
			Data: append([]byte(nil), b[i:i+size]...)})
		i += size
	}
	return out, nil
}

// eventSize 是一個事件要幾個參數位元組。
func eventSize(status byte) (int, error) {
	if status >= 0xF0 {
		switch status {
		case SysEx:
			return 0, nil // 讀到 0xF7 為止，由呼叫端處理
		case 0xF1, 0xF3: // MTC quarter frame、song select
			return 1, nil
		case 0xF2: // song position
			return 2, nil
		}
		// 0xF6、0xF7 與 0xF8..0xFF（時脈、開始、停止…）都是單一位元組。
		return 0, nil
	}
	switch status & 0xF0 {
	case NoteOff, NoteOn, ControlChange, PitchBend:
		return 2, nil
	case ProgramChange, ChannelPressure, KeyPressure:
		// ⚠ **0xA0 在這個格式裡只帶一個參數。** 標準 MIDI 的
		// polyphonic key pressure 帶兩個；照標準讀會把下一個事件的
		// 狀態位元組吃掉，整串從那裡開始歪掉，而且不會報錯——
		// 走出來的音符看起來仍然像音樂。資料裡的樣子是
		// `00 A3 58 | 00 A4 58 | 00 A5 58`（`docs/formats/06`）。
		return 1, nil
	case SysEx:
		return 0, nil
	}
	return 0, fmt.Errorf("不認識的狀態位元組 %#02x", status)
}

// varLen 讀一個 MIDI 的可變長度數值。
func varLen(b []byte) (value, size int, err error) {
	for size < len(b) && size < 4 {
		c := b[size]
		value = value<<7 | int(c&0x7F)
		size++
		if c&0x80 == 0 {
			return value, size, nil
		}
	}
	return 0, 0, fmt.Errorf("可變長度數值沒有結尾")
}

// 容器裡的項目名。
const (
	SongIndex = "MUS.IDX"
	SongData  = "MUS.GRP"
	LongIndex = "MUSV.IDX"
	LongData  = "MUSV.GRP"
)

// TrackNames 是開機主選單列的五首曲名（`AA.EXE` `0x47bb9`–`0x47c0d`，
// `docs/re/04` §10）。順序與 `MUS` 裡的五對相同。
//
// ⚠ **對應是推的**：主選單由上而下是 5..1，而容器裡是 0..4；
// 兩邊都是五個，順序也只有這一種讀法合理。要確認得聽原版。
func TrackNames() []string { return []string{"思古", "小徑", "風雲", "戰鼓", "末路"} }

// ParseAll 把一組 `.IDX`／`.GRP` 解成曲子與音色庫的配對。
//
// 項目成對出現：偶數項是曲子，奇數項是它的音色庫。
func ParseAll(idx, grp []byte) ([]Track, error) {
	items, err := split(idx, grp)
	if err != nil {
		return nil, err
	}
	if len(items)%2 != 0 {
		return nil, fmt.Errorf("music: 有 %d 個項目，曲子與音色庫應該成對", len(items))
	}
	out := make([]Track, 0, len(items)/2)
	for i := 0; i+1 < len(items); i += 2 {
		s, err := ParseSong(items[i])
		if err != nil {
			return nil, fmt.Errorf("music: 第 %d 首：%w", i/2, err)
		}
		b, err := ParseBank(items[i+1])
		if err != nil {
			return nil, fmt.Errorf("music: 第 %d 首的音色庫：%w", i/2, err)
		}
		out = append(out, Track{Song: s, Bank: b})
	}
	return out, nil
}

// split 依 `.IDX` 的結束位移把 `.GRP` 切開。版面與外層容器相同。
func split(idx, grp []byte) ([][]byte, error) {
	if len(idx)%4 != 0 {
		return nil, fmt.Errorf("music: 索引有 %d 個位元組，不是 4 的倍數", len(idx))
	}
	var out [][]byte
	prev := uint32(0)
	for i := 0; i+4 <= len(idx); i += 4 {
		end := binary.LittleEndian.Uint32(idx[i:])
		if end < prev || int(end) > len(grp) {
			return nil, fmt.Errorf("music: 第 %d 項的結束位移 %d 不合理（上一項 %d，資料 %d）",
				i/4, end, prev, len(grp))
		}
		out = append(out, grp[prev:end])
		prev = end
	}
	if int(prev) != len(grp) {
		return nil, fmt.Errorf("music: 索引只蓋到 %d，資料有 %d 個位元組", prev, len(grp))
	}
	return out, nil
}
