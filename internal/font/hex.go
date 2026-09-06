// Package font 讀 GNU Unifont 的 HEX 點陣字型。
//
// **這裡讀的不是原版的字模。** 原版的中文字模在 `DATA1`／`DATA5` 容器裡
// （`ZHONG.PAT`／`ZHONG.COD`），那是原版素材，不散布也不內嵌
// （`CLAUDE.md` §3.3）。remake 的字模由 `fonts/` 底下的自由授權字型提供。
//
// 這一層**不依賴 Ebiten**，所以無頭環境測得到。畫到畫面上是 `internal/ui` 的事。
package font

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Glyph 是一個字的點陣。
//
// Rows 由上而下，每列的第 0 位在**最左邊**（bit 7 of byte 0）。
type Glyph struct {
	W, H int
	Rows [][]byte // len(Rows) == H，每列 (W+7)/8 個 byte
}

// At 回傳 (x, y) 這一點是不是實心。越界回 false。
func (g Glyph) At(x, y int) bool {
	if x < 0 || y < 0 || x >= g.W || y >= g.H {
		return false
	}
	return g.Rows[y][x/8]&(0x80>>(uint(x)%8)) != 0
}

// Face 是一整份字型。
type Face struct {
	// H 是所有字的共同高度。HEX 格式一份字型只有一種高度。
	H      int
	glyphs map[rune]Glyph
}

// Glyph 取一個字的點陣；沒有就回 false。
//
// **找不到不要自己畫豆腐塊。** 缺字是「字型沒涵蓋這個碼位」，
// 是要被看見的事實；悄悄畫一個方框會讓缺字看起來像設計。
// 要畫替代符號是 `internal/ui` 的決定，不是這一層的。
func (f *Face) Glyph(r rune) (Glyph, bool) {
	g, ok := f.glyphs[r]
	return g, ok
}

// Len 是收了幾個字。
func (f *Face) Len() int { return len(f.glyphs) }

// Covers 回傳這串文字裡**沒有**字模的字元。
//
// 用途是在測試裡擋掉「譯文用了字型沒有的字」——那在畫面上是空白，
// 而空白看起來像排版問題，不像缺字。
func (f *Face) Covers(s string) []rune {
	var missing []rune
	seen := map[rune]bool{}
	for _, r := range s {
		if r == ' ' || r == '\n' || seen[r] {
			continue
		}
		seen[r] = true
		if _, ok := f.glyphs[r]; !ok {
			missing = append(missing, r)
		}
	}
	return missing
}

// ParseHex 讀 HEX 格式。
//
// 一行一個字：`<碼位十六進位>:<點陣十六進位>`。`#` 開頭與空行是註解。
// 點陣的十六進位字元數決定寬度：H 列 × 每列 (W+7)/8 個 byte × 2 個字元。
//
// 高度由呼叫端給，因為 HEX 格式本身不記高度——同樣 64 個字元
// 可以是 16×16，也可以是 8×32。**猜錯不會報錯**，只會畫出拉長或壓扁的字，
// 所以這裡要求明講。
func ParseHex(r io.Reader, height int) (*Face, error) {
	if height <= 0 {
		return nil, fmt.Errorf("font: 高度要是正數，拿到 %d", height)
	}
	f := &Face{H: height, glyphs: make(map[rune]Glyph)}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		t := strings.TrimSpace(sc.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		colon := strings.IndexByte(t, ':')
		if colon < 0 {
			return nil, fmt.Errorf("font: 第 %d 行沒有冒號", line)
		}
		cp, err := strconv.ParseUint(t[:colon], 16, 32)
		if err != nil {
			return nil, fmt.Errorf("font: 第 %d 行的碼位 %q 解不出來：%w", line, t[:colon], err)
		}
		bits := t[colon+1:]
		if len(bits)%2 != 0 {
			return nil, fmt.Errorf("font: 第 %d 行的點陣有 %d 個字元，不是偶數", line, len(bits))
		}
		nbytes := len(bits) / 2
		if nbytes%height != 0 {
			return nil, fmt.Errorf("font: 第 %d 行有 %d bytes，除不盡高度 %d", line, nbytes, height)
		}
		bpr := nbytes / height // bytes per row
		rows := make([][]byte, height)
		for y := 0; y < height; y++ {
			row := make([]byte, bpr)
			for x := 0; x < bpr; x++ {
				v, err := strconv.ParseUint(bits[(y*bpr+x)*2:(y*bpr+x)*2+2], 16, 8)
				if err != nil {
					return nil, fmt.Errorf("font: 第 %d 行第 %d byte 解不出來：%w", line, y*bpr+x, err)
				}
				row[x] = byte(v)
			}
			rows[y] = row
		}
		f.glyphs[rune(cp)] = Glyph{W: bpr * 8, H: height, Rows: rows}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(f.glyphs) == 0 {
		// **空字型要報錯不要靜靜回傳。** 一份載不到東西的字型會讓整個
		// 畫面變成空白，而空白看起來像「還沒畫」不像「載錯檔」。
		return nil, fmt.Errorf("font: 一個字都沒讀到")
	}
	return f, nil
}

// ParseHexGz 讀 gzip 壓過的 HEX。
func ParseHexGz(r io.Reader, height int) (*Face, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("font: gzip：%w", err)
	}
	defer zr.Close()
	return ParseHex(zr, height)
}
