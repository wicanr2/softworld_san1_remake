// Package cells 是排版的下層：算寬度、判斷裝不裝得下、折行。
//
// **這一層不依賴 Ebiten**，所以無頭環境測得到（`CLAUDE.md` §3.3）。
// 畫字是 `internal/ui` 的事。
//
// 單位是**半形格**：ASCII 一格（8×16），CJK 兩格（16×16）。
// 原版的版面是固定格寬的，槽位能放幾個字是版面決定的，不是字數決定的。
//
// 這一層存在的理由是多語系：原文是繁中，英日譯文要塞回原版的槽位，
// 而英文通常比中文長。**「裝不下」要在這裡量出來，不是等畫面破版才發現**
// （`CLAUDE.md` §7 第 13 條：畫面 bug 測試看不到）。
package cells

import "unicode"

// RuneWidth 是一個字元佔幾個半形格。
//
// 判準是**東亞寬度**不是語系：全形標點、日文假名、漢字都是兩格，
// ASCII 與半形符號是一格。
func RuneWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 0x80:
		// ASCII。控制字元不佔格。
		if r < 0x20 || r == 0x7F {
			return 0
		}
		return 1
	case isCombining(r):
		return 0
	case isWide(r):
		return 2
	default:
		return 1
	}
}

// Width 是整串佔幾個半形格。
func Width(s string) int {
	w := 0
	for _, r := range s {
		w += RuneWidth(r)
	}
	return w
}

// Fits 判斷這串裝不裝得進 cols 個半形格。
func Fits(s string, cols int) bool { return Width(s) <= cols }

// Truncate 把字串裁到最多 cols 格。
//
// **不會切半一個全形字**：剩一格而下一個字要兩格時就停在那裡，
// 寧可少一格也不吐出半個字（那在原版的固定格版面上會變成亂碼）。
func Truncate(s string, cols int) string {
	if cols <= 0 {
		return ""
	}
	w := 0
	for i, r := range s {
		rw := RuneWidth(r)
		if w+rw > cols {
			return s[:i]
		}
		w += rw
	}
	return s
}

// MinWidth 是這串文字**最少需要幾格**才不會有字被擠掉，
// 也就是裡面最寬的那個字的寬度。
//
// 用途是在版面測試裡擋掉「框比一個字還窄」：那是版面錯誤，
// 不是文字問題。Wrap 碰到這種框只能丟掉那個字（見下），
// 而丟掉是安靜的——所以要在更早的地方擋。
func MinWidth(s string) int {
	m := 0
	for _, r := range s {
		if w := RuneWidth(r); w > m {
			m = w
		}
	}
	return m
}

// Wrap 依 cols 折行。
//
// 折行規則刻意簡單，因為原版的版面是固定格的：
//
//   - 換行字元強制斷行
//   - CJK 可以在任何字之間斷（中文不用空白分詞）
//   - ASCII 單字盡量不切開；但**單字本身就超過一行時照切**——
//     切開比讓它溢出版面好，溢出在原版的格版面上會蓋掉隔壁的東西
//
// **不變式：回傳的每一行寬度都不超過 cols。** 為了守住它，
// 比 cols 還寬的單一字元會被丟掉（框比一個字還窄時）。
// 那是版面錯誤，用 MinWidth 在更早的地方擋。
//
// cols <= 0 時原樣回傳單行，不做無窮迴圈。
func Wrap(s string, cols int) []string {
	if cols <= 0 {
		return []string{s}
	}
	var out []string
	line := ""
	lineW := 0
	word := ""
	wordW := 0

	flushWord := func() {
		if word == "" {
			return
		}
		if lineW+wordW <= cols {
			line += word
			lineW += wordW
		} else {
			if line != "" {
				out = append(out, line)
			}
			// 單字本身超過一行：照切。
			for wordW > cols {
				head := Truncate(word, cols)
				if head == "" { // 一格都放不下，避免無窮迴圈
					break
				}
				out = append(out, head)
				word = word[len(head):]
				wordW = Width(word)
			}
			line, lineW = word, wordW
		}
		word, wordW = "", 0
	}

	for _, r := range s {
		if r == '\n' {
			flushWord()
			out = append(out, line)
			line, lineW = "", 0
			continue
		}
		rw := RuneWidth(r)
		switch {
		case r == ' ':
			flushWord()
			if lineW+1 <= cols {
				line += " "
				lineW++
			} else {
				out = append(out, line)
				line, lineW = "", 0
			}
		case isWide(r):
			// CJK 逐字斷行：先把待處理的 ASCII 單字結掉。
			flushWord()
			if rw > cols {
				// 這個字在空行上都放不下（框比一個字還窄）。
				// 加進去會讓回傳的行超過 cols，破壞不變式。
				// **這是版面錯誤不是文字問題**——呼叫端該用 MinWidth
				// 事先擋掉，不該讓它走到這裡。
				continue
			}
			if lineW+rw > cols {
				out = append(out, line)
				line, lineW = "", 0
			}
			line += string(r)
			lineW += rw
		default:
			word += string(r)
			wordW += rw
		}
	}
	flushWord()
	if line != "" || len(out) == 0 {
		out = append(out, line)
	}
	return out
}

// Pad 把字串補到剛好 cols 格（右邊補空白）。超過就裁掉。
//
// 原版的槽位是固定寬度的，短的要補滿否則會露出上一次畫的內容。
func Pad(s string, cols int) string {
	s = Truncate(s, cols)
	for w := Width(s); w < cols; w++ {
		s += " "
	}
	return s
}

// Center 把字串置中到 cols 格。
//
// 多出來的一格放在**右邊**——原版的兩字郡名放在四格槽裡是
// 「空白 ＋ 名 ＋ 空白」，偶數對齊，這裡只在奇數餘數時才有差別。
func Center(s string, cols int) string {
	s = Truncate(s, cols)
	pad := cols - Width(s)
	if pad <= 0 {
		return s
	}
	left := pad / 2
	out := ""
	for i := 0; i < left; i++ {
		out += " "
	}
	out += s
	for w := Width(out); w < cols; w++ {
		out += " "
	}
	return out
}

// isWide 判斷是不是佔兩格。
//
// 只列原版真的會用到的區段：Big5 能表示的漢字與全形符號，
// 加上多語系會用到的日文假名。**不要用「非 ASCII 即全形」**——
// 英日多語系會帶進拉丁補充區與變音符號，那些是一格。
func isWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F: // 韓文字母
		return true
	case r >= 0x2E80 && r <= 0x303E: // CJK 部首、日文標點
		return true
	case r >= 0x3041 && r <= 0x33FF: // 假名、注音、CJK 相容
		return true
	case r >= 0x3400 && r <= 0x4DBF: // CJK 擴充 A
		return true
	case r >= 0x4E00 && r <= 0x9FFF: // CJK 統一表意文字
		return true
	case r >= 0xA000 && r <= 0xA4CF: // 彝文
		return true
	case r >= 0xAC00 && r <= 0xD7A3: // 韓文音節
		return true
	case r >= 0xF900 && r <= 0xFAFF: // CJK 相容表意文字
		return true
	case r >= 0xFE30 && r <= 0xFE6F: // CJK 相容形式
		return true
	case r >= 0xFF00 && r <= 0xFF60: // 全形 ASCII
		return true
	case r >= 0xFFE0 && r <= 0xFFE6: // 全形符號
		return true
	case r >= 0x20000 && r <= 0x3FFFD: // CJK 擴充 B 以上
		return true
	}
	return false
}

// isCombining 判斷是不是不佔格的組合字元。多語系會用到（如越南文、變音）。
func isCombining(r rune) bool {
	return unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r)
}
