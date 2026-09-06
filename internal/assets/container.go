// Package assets 讀原版的資料容器。
//
// 規格 `docs/spec/001-container-format.md`（READY），證據
// `docs/formats/01-grp-idx-nam.md`。**本套件只讀不寫**——這是保存專案。
package assets

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// namEntrySize／idxEntrySize 是兩張表的每項大小。
//
// 這兩個數字是量出來的：五個槽上 len(.NAM)/16 與 len(.IDX)/4 逐一相等
// （`docs/formats/01` §2）。
const (
	namEntrySize = 16
	idxEntrySize = 4
)

// Entry 是容器裡的一項。
type Entry struct {
	// Name 是正規化後的名稱，例 "ZHONG.COD"。
	Name string

	// RawName 是 .NAM 裡原始的 12 byte（8.3 空白補齊）。
	//
	// **保留它是為了能對回原版 bytes。** 正規化會丟掉補齊空白，
	// 而「這個名稱在檔案裡長什麼樣」是驗證時要問的問題。
	RawName [12]byte

	Start uint32 // 含
	End   uint32 // 不含
}

// Size 是這一項的位元組長度。
func (e Entry) Size() uint32 { return e.End - e.Start }

// Container 是一組 .NAM／.IDX／.GRP。
type Container struct {
	entries []Entry
	grp     []byte
	byName  map[string]int
}

// OpenContainer 解析一組容器。三個切片由呼叫端自備（原版檔案不隨本儲存庫散布）。
//
// ⚠ **驗證條件不過就回錯誤，不會盡力而為地讀下去。** 解錯一個位元組不會報錯，
// 只會讓多數項目碰巧是對的——那種錯誤要到很後面才會以「某張圖怪怪的」浮現。
func OpenContainer(nam, idx, grp []byte) (*Container, error) {
	// 1. .GRP 不得是執行檔。
	//
	// DATA0／DATA4／DATA5 的 .GRP 開頭是 MZ（`docs/formats/01` §4），
	// 那些槽放的是執行檔不是容器本體。硬切下去會切出一堆看起來像
	// 資料的垃圾，而且不會報錯。
	if len(grp) >= 2 && grp[0] == 'M' && grp[1] == 'Z' {
		return nil, fmt.Errorf("assets: .GRP 開頭是 MZ，這是執行檔不是容器本體"+
			"（長度 %d；見 docs/formats/01 §4）", len(grp))
	}

	if len(nam)%namEntrySize != 0 {
		return nil, fmt.Errorf("assets: .NAM 長度 %d 不是 %d 的倍數", len(nam), namEntrySize)
	}
	if len(idx)%idxEntrySize != 0 {
		return nil, fmt.Errorf("assets: .IDX 長度 %d 不是 %d 的倍數", len(idx), idxEntrySize)
	}

	// 2. 項數兩邊都算並比對。DATA0 就是靠這一步露餡的（43 vs 604）。
	nNam, nIdx := len(nam)/namEntrySize, len(idx)/idxEntrySize
	if nNam != nIdx {
		return nil, fmt.Errorf("assets: 項數對不上——.NAM 說 %d 項，.IDX 說 %d 項", nNam, nIdx)
	}
	if nNam == 0 {
		return nil, fmt.Errorf("assets: 容器是空的")
	}

	entries := make([]Entry, nNam)
	byName := make(map[string]int, nNam)
	var start uint32
	for i := range entries {
		end := binary.LittleEndian.Uint32(idx[i*idxEntrySize:])

		// 3. .IDX 必須非遞減。逆序代表它不是位移表。
		if end < start {
			return nil, fmt.Errorf("assets: .IDX 第 %d 項逆序（%d < %d），這不是位移表",
				i, end, start)
		}
		var raw [12]byte
		copy(raw[:], nam[i*namEntrySize:])
		e := Entry{Name: normalizeName(raw), RawName: raw, Start: start, End: end}
		entries[i] = e

		// 同名時**留第一個**，並且不覆蓋——原版有沒有重名還沒查過，
		// 悄悄覆蓋會讓 ByName 回傳一個沒人預期的項目。
		if _, dup := byName[e.Name]; !dup {
			byName[e.Name] = i
		}
		start = end
	}

	// 4. 末值必須等於 .GRP 長度。這是最強的一項驗證：
	// 三個獨立檔案同時滿足，不會是巧合（`docs/formats/01` §2）。
	if last := entries[len(entries)-1].End; last != uint32(len(grp)) {
		return nil, fmt.Errorf("assets: .IDX 末值 %d 與 .GRP 長度 %d 不符——"+
			"這三個檔不是一組，或版面不是這樣", last, len(grp))
	}

	return &Container{entries: entries, grp: grp, byName: byName}, nil
}

// Len 是項數。
func (c *Container) Len() int { return len(c.entries) }

// Entry 回傳第 i 項的中繼資料。
func (c *Container) Entry(i int) Entry { return c.entries[i] }

// Data 回傳第 i 項的位元組。
//
// ⚠ 這是 .GRP 的**子切片，不複製**。呼叫端不得寫入——寫進去會改到
// 相鄰項目的內容。
func (c *Container) Data(i int) []byte {
	e := c.entries[i]
	return c.grp[e.Start:e.End]
}

// ByName 用正規化後的名稱查，**大小寫敏感**。
//
// 原版的名稱全大寫。容忍大小寫會把「名字打錯」變成「安靜地拿到別的東西」，
// 那比查不到難除錯得多。
func (c *Container) ByName(name string) (int, bool) {
	i, ok := c.byName[name]
	return i, ok
}

// normalizeName 把 8.3 空白補齊的 12 byte 拼成 "NAME.EXT"。
//
// **不對名稱做 Big5 解碼**：Big5 是資料內容的編碼，不是檔名的。
// 實測名稱全部落在 0x20–0x7E。
func normalizeName(raw [12]byte) string {
	base := strings.TrimRight(string(raw[0:8]), " \x00")
	ext := strings.TrimRight(string(raw[9:12]), " \x00")
	if ext == "" {
		return base
	}
	return base + "." + ext
}
