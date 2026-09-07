package state

import (
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// 原版的第四、五、六個存檔項目：`BASEPRO.SVn`（256 B）、
// `BASEPRE.SVn`（384 B）與共用的 `SAVENAME.SVP`（126 B）。
//
// 三張主表（諸侯、州郡、人物）裝的是盤面；這三個裝的是**盤面以外的
// 一切**——年月、難度、選項、這個月走到哪一個郡，以及自創君主的字模。
// 位移全部從讀檔常式 `0x141ef`–`0x1437f` 讀出來（`docs/formats/05`）。

// ProgressSize／GlyphTableSize／SaveNameTableSize 是三個檔案該有的長度。
// 長度是最強的一項驗證，對不上就是讀錯東西。
const (
	ProgressSize      = 256
	GlyphTableSize    = 384
	SaveNameTableSize = 126
)

// proUsed 是 `BASEPRO` 真正被讀進去的長度。
//
// 讀檔常式只搬到位移 `0xC4`（`0x14375`），**後面 58 個位元組沒有人讀**
// ——出貨的六個進度裡，那一段有的是 `0xFF` 有的是 `0x00`，
// 那是寫檔時沒清乾淨的緩衝區殘值，不是欄位。
const proUsed = 0xC6

// 三段陣列與各個純量在 `BASEPRO` 裡的位移。
const (
	proPending = 0x00 // 43 × u16：這個月這個郡還沒下令（0xFFFF ＝ 還沒）
	proOrder   = 0x56 // 43 × u16：郡的處理順序（每個月洗牌五輪）
	proUnknown = 0xAC // u16，讀進 DS:0x5b00，全域找不到第二個引用
	proYear    = 0xAE // u16 → es:0x3140，西元年
	proMonth   = 0xB0 // u16 → es:0x3f08，月份 1–12
	proCursor  = 0xB2 // u16 → es:0x20f4，處理到順序表的第幾個
	proDiff    = 0xB4 // u16 → es:0x30fe，難度
	proInMonth = 0xB6 // u16 → es:0x0080，月內迴圈的「繼續」旗標
	proMusic   = 0xB8 // u16 → es:0x31ba，音樂狀態
	proSound   = 0xBA // u16 → es:0x31aa，音效狀態
	proSkipWar = 0xBC // u16 → es:0x2174，查看電腦戰役
	proVoice   = 0xBE // u16 → es:0x17c2，語音狀態
	proDelay   = 0xC0 // u16 → es:0x2f72，延遲時間
	proCal     = 0xC2 // u16 → es:0x3148，年號用中曆還是西曆
	proSeal    = 0xC4 // u16 → es:0x2f6c，玉璽持有勢力（0xFFFF ＝ 未現世）
)

// proSlots 是兩段陣列各有幾格。43 ＝ 州郡表的筆數（含第 0 筆啞元）。
const proSlots = prefCount

// Progress 是 `BASEPRO.SVn` 的內容。
//
// 欄位名照原版變數的用途取，不照猜測。**沒解出來的位元組原樣帶著走**
// （`Unknown0AC` 與 `tail`），存回去時不會被洗掉。
type Progress struct {
	// Pending 是「這個月這一郡還沒下令」。索引就是郡號，第 0 格是啞元。
	Pending [proSlots]bool
	// Order 是這個月處理郡的順序，內容是 0–42 的一個排列。
	// 原版在每個月開頭洗五輪（`0x1740a`），所以它是狀態不是常數。
	Order [proSlots]int
	// Cursor 是處理到 Order 的第幾格。
	Cursor int

	Year, Month int
	Difficulty  int

	// InMonth 是月內迴圈的「繼續」旗標（`0x15804` 每跑完一郡檢查一次）。
	InMonth bool

	MusicOff, SoundOff, VoiceOff, SkipAIWar bool
	Delay                                   int
	// Calendar 0 ＝ 中曆、1 ＝ 西曆（`DS:0x690c`／`0x6911`）。
	Calendar int

	// Seal 是持有玉璽的勢力，−1 ＝ 還沒現世。
	Seal int

	// Unknown0AC 與 tail 是還沒解出用途的位元組。
	Unknown0AC uint16
	tail       [ProgressSize - proUsed]byte
}

// DecodeProgress 解一份 `BASEPRO`。
func DecodeProgress(b []byte) (*Progress, error) {
	if len(b) != ProgressSize {
		return nil, fmt.Errorf("state: BASEPRO 長度 %d，想要 %d", len(b), ProgressSize)
	}
	u16 := func(off int) uint16 { return binary.LittleEndian.Uint16(b[off:]) }
	p := &Progress{
		Cursor:     int(u16(proCursor)),
		Year:       int(u16(proYear)),
		Month:      int(u16(proMonth)),
		Difficulty: int(u16(proDiff)),
		InMonth:    u16(proInMonth) == NoValue16,
		MusicOff:   u16(proMusic) != 0,
		SoundOff:   u16(proSound) != 0,
		SkipAIWar:  u16(proSkipWar) != 0,
		VoiceOff:   u16(proVoice) != 0,
		Delay:      int(u16(proDelay)),
		Calendar:   int(u16(proCal)),
		Seal:       -1,
		Unknown0AC: u16(proUnknown),
	}
	if v := u16(proSeal); v != NoValue16 {
		p.Seal = int(v)
	}
	for i := 0; i < proSlots; i++ {
		p.Pending[i] = u16(proPending+i*2) == NoValue16
		p.Order[i] = int(u16(proOrder + i*2))
	}
	copy(p.tail[:], b[proUsed:])
	if !isPermutation(p.Order[:]) {
		return nil, fmt.Errorf("state: BASEPRO 的順序表不是 0–%d 的排列", proSlots-1)
	}
	return p, nil
}

// Encode 寫回 `BASEPRO` 的版面。
func (p *Progress) Encode() []byte {
	b := make([]byte, ProgressSize)
	put := func(off int, v uint16) { binary.LittleEndian.PutUint16(b[off:], v) }
	flag := func(on bool) uint16 {
		if on {
			return 1
		}
		return 0
	}
	for i := 0; i < proSlots; i++ {
		if p.Pending[i] {
			put(proPending+i*2, NoValue16)
		}
		put(proOrder+i*2, uint16(p.Order[i]))
	}
	put(proUnknown, p.Unknown0AC)
	put(proYear, uint16(p.Year))
	put(proMonth, uint16(p.Month))
	put(proCursor, uint16(p.Cursor))
	put(proDiff, uint16(p.Difficulty))
	if p.InMonth {
		put(proInMonth, NoValue16)
	}
	put(proMusic, flag(p.MusicOff))
	put(proSound, flag(p.SoundOff))
	put(proSkipWar, flag(p.SkipAIWar))
	put(proVoice, flag(p.VoiceOff))
	put(proDelay, uint16(p.Delay))
	put(proCal, uint16(p.Calendar))
	put(proSeal, NoValue16)
	if p.Seal >= 0 {
		put(proSeal, uint16(p.Seal))
	}
	copy(b[proUsed:], p.tail[:])
	return b
}

func isPermutation(v []int) bool {
	seen := make([]bool, len(v))
	for _, x := range v {
		if x < 0 || x >= len(v) || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}

// 自創君主：四個名額，姓名各三個字，字碼是 Big5 的 `A141`–`A14C`
// （範本在 `0x3e44c`，四筆 30 byte 的人物記錄）。那十二個碼位在標準
// Big5 是全形標點，原版把它們當成**造字**用，字模存在 `BASEPRE.SVn`。
const (
	// CustomLords 是自創君主的名額。
	CustomLords = 4
	// CustomLordNameChars 是一個自創君主的姓名字數。
	CustomLordNameChars = 3
	// GlyphBytes 是一個 16×16 字模的位元組數。
	GlyphBytes = 32
	// CustomGlyphBase 是第一個造字的 Big5 碼位。
	CustomGlyphBase = 0xA141
)

// Glyphs 是 `BASEPRE.SVn`：十二個 16×16 單色字模，每列一個 `u16`，
// 高位在前（bit 15 ＝ 最左邊那一點）。
type Glyphs [CustomLords * CustomLordNameChars][GlyphBytes]byte

// DecodeGlyphs 解一份 `BASEPRE`。
func DecodeGlyphs(b []byte) (*Glyphs, error) {
	if len(b) != GlyphTableSize {
		return nil, fmt.Errorf("state: BASEPRE 長度 %d，想要 %d", len(b), GlyphTableSize)
	}
	var g Glyphs
	for i := range g {
		copy(g[i][:], b[i*GlyphBytes:])
	}
	return &g, nil
}

// Encode 寫回 `BASEPRE` 的版面。
func (g *Glyphs) Encode() []byte {
	b := make([]byte, GlyphTableSize)
	for i := range g {
		copy(b[i*GlyphBytes:], g[i][:])
	}
	return b
}

// Row 回傳第 i 個字模的第 r 列，bit 15 是最左邊那一點。
func (g *Glyphs) Row(i, r int) uint16 {
	return binary.BigEndian.Uint16(g[i][r*2:])
}

// Code 是第 i 個字模對應的 Big5 碼位。
func (g *Glyphs) Code(i int) int { return CustomGlyphBase + i }

// DecodeSaveNames 解 `SAVENAME.SVP`：六筆 21 byte 的 NUL 結尾 Big5 字串。
//
// 126 ÷ 6 ＝ 21，每筆二十個位元組的正文加一個結尾。造字碼位
// （`A141`–`A14C`）在這裡照樣出現，解碼成全形標點是**正常的**——
// 那是原版拿標點的碼位當造字用（見 CustomGlyphBase）。
func DecodeSaveNames(b []byte) ([]string, error) {
	if len(b) != SaveNameTableSize {
		return nil, fmt.Errorf("state: SAVENAME 長度 %d，想要 %d",
			len(b), SaveNameTableSize)
	}
	const stride = SaveNameTableSize / 6
	out := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		rec := b[i*stride : (i+1)*stride]
		if j := indexByte(rec, 0); j >= 0 {
			rec = rec[:j]
		}
		s, err := decodeBig5(rec)
		if err != nil {
			return nil, fmt.Errorf("state: SAVENAME 第 %d 筆解不開：%w", i+1, err)
		}
		out = append(out, s)
	}
	return out, nil
}

// EncodeSaveNames 寫回 `SAVENAME.SVP` 的版面。太長的名稱會被截掉，
// **而且是照位元組截**——切在 Big5 的第二個位元組上會生出半個字，
// 所以要成對地退。
func EncodeSaveNames(names []string) ([]byte, error) {
	if len(names) != 6 {
		return nil, fmt.Errorf("state: SAVENAME 要六筆，給了 %d 筆", len(names))
	}
	const stride = SaveNameTableSize / 6
	b := make([]byte, SaveNameTableSize)
	for i, s := range names {
		enc, err := encodeBig5(s)
		if err != nil {
			return nil, fmt.Errorf("state: 存檔名稱 %q 編不出 Big5：%w", s, err)
		}
		copy(b[i*stride:], trimBig5(enc, stride-1))
	}
	return b, nil
}

// trimBig5 把位元組序列截到最多 n 個位元組，不切開雙位元組字。
func trimBig5(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	cut := 0
	for i := 0; i < n; {
		if b[i] >= 0xA1 {
			if i+1 >= n {
				break
			}
			i += 2
		} else {
			i++
		}
		cut = i
	}
	return b[:cut]
}

func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}

// LoadProgress 從 DATA2 容器讀一個存檔槽的 `BASEPRO`。
func LoadProgress(c *assets.Container, slot Slot) (*Progress, error) {
	b, err := section(c, "BASEPRO."+string(slot), ProgressSize)
	if err != nil {
		return nil, err
	}
	return DecodeProgress(b)
}

// LoadGlyphs 從 DATA2 容器讀一個存檔槽的 `BASEPRE`。
func LoadGlyphs(c *assets.Container, slot Slot) (*Glyphs, error) {
	b, err := section(c, "BASEPRE."+string(slot), GlyphTableSize)
	if err != nil {
		return nil, err
	}
	return DecodeGlyphs(b)
}

// LoadSaveNames 從 DATA2 容器讀六個存檔的名稱。
func LoadSaveNames(c *assets.Container) ([]string, error) {
	b, err := section(c, "SAVENAME.SVP", SaveNameTableSize)
	if err != nil {
		return nil, err
	}
	return DecodeSaveNames(b)
}
