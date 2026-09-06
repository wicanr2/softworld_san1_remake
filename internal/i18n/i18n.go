// Package i18n 是介面文字的多語系層。
//
// **繁體中文是原文，不是譯文。** 這個專案還原的是 1991 年的繁中版，
// 中文那一份就是原版寫的字（`docs/re/04` 抽出來的字串表）；
// 英文與日文是譯出去的。所以 `ZhHant` 的內容不可以為了「跟英文對齊」
// 而改寫——要改的是譯文那一邊。
//
// 遊戲資料裡的專有名詞（郡名、人名）**不在這裡**：那是玩家自己那一份
// 原版檔案的內容，remake 不重製。郡名有一份羅馬拼音對照（`places.go`）
// 讓英文介面讀得下去，人名目前照原樣顯示。
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

// lang 是各語系的字串表。
//
// **文字放 JSON 不放 Go 原始碼**：譯者不應該為了改一句話而動程式碼，
// 而且 JSON 的 diff 一行一句，看得出改了什麼。檔案照鍵排序，
// 所以兩份譯文並排比對就是逐行對照。
//
// 用 `go:embed` 打包進執行檔，發行時不必另外帶一包檔案；
// 要改字仍然是改 `internal/i18n/lang/*.json` 再重編。
//
//go:embed lang/*.json
var lang embed.FS

// Locale 是語系。
type Locale string

const (
	// ZhHant 是原文。
	ZhHant Locale = "zh-Hant"
	En     Locale = "en"
	Ja     Locale = "ja"
)

// Locales 是支援的語系，原文排第一。
func Locales() []Locale { return []Locale{ZhHant, En, Ja} }

// Name 是語系自己的名字，給選單用。
func (l Locale) Name() string {
	switch l {
	case En:
		return "English"
	case Ja:
		return "日本語"
	}
	return "繁體中文"
}

// Valid 回報這是不是支援的語系。
func (l Locale) Valid() bool {
	for _, x := range Locales() {
		if x == l {
			return true
		}
	}
	return false
}

// Parse 把字串換成語系；不認識回原文與 false。
func Parse(s string) (Locale, bool) {
	l := Locale(s)
	if l.Valid() {
		return l, true
	}
	switch s {
	case "zh", "zh-TW", "cht", "中文", "繁中":
		return ZhHant, true
	case "english", "eng":
		return En, true
	case "jp", "japanese", "日文":
		return Ja, true
	}
	return ZhHant, false
}

// catalog 是各語系的字串表。鍵是穩定的識別碼，不是中文原文——
// **用原文當鍵的話，改一個字就會漏掉所有譯文**，而漏掉的地方
// 在畫面上是空白，看起來像排版問題。
var catalog = map[Locale]map[string]string{}

// init 把打包進來的 JSON 讀進字串表。
//
// **讀不進來就 panic**：一個少了字串表的執行檔，畫面上每一句話都會
// 變成 `{key}`——那不是可以邊跑邊發現的問題，是根本不該編得出來的東西。
func init() {
	entries, err := lang.ReadDir("lang")
	if err != nil {
		panic(fmt.Sprintf("i18n: 讀不到字串表：%v", err))
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		b, err := lang.ReadFile(path.Join("lang", name))
		if err != nil {
			panic(fmt.Sprintf("i18n: 讀 %s：%v", name, err))
		}
		var m map[string]string
		if err := json.Unmarshal(b, &m); err != nil {
			panic(fmt.Sprintf("i18n: %s 解不開：%v", name, err))
		}
		l := Locale(strings.TrimSuffix(name, ".json"))
		if !l.Valid() {
			panic(fmt.Sprintf("i18n: %s 不是支援的語系", name))
		}
		catalog[l] = m
	}
	if len(catalog[ZhHant]) == 0 {
		panic("i18n: 原文表是空的")
	}
}

// T 取一句話。
//
// 找不到就退回原文（`ZhHant`）；原文也沒有就回鍵本身，**而且看得出來**
// ——一個安靜地回空字串的翻譯層會讓缺字看起來像排版問題。
func T(l Locale, key string) string {
	if m, ok := catalog[l]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	if s, ok := catalog[ZhHant][key]; ok {
		return s
	}
	return "{" + key + "}"
}

// Tf 取一句帶參數的話。
func Tf(l Locale, key string, a ...any) string {
	return fmt.Sprintf(T(l, key), a...)
}

// Keys 回傳原文表裡的全部鍵，排好序。
func Keys() []string {
	out := make([]string, 0, len(catalog[ZhHant]))
	for k := range catalog[ZhHant] {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Missing 回傳某個語系少掉的鍵，排好序。測試用。
func Missing(l Locale) []string {
	var out []string
	for _, k := range Keys() {
		if _, ok := catalog[l][k]; !ok {
			out = append(out, k)
		}
	}
	return out
}

// Extra 回傳某個語系多出來的鍵（原文表裡沒有的）。測試用。
//
// **多出來的鍵是死字串**：改名或刪掉的時候譯文那一邊不會有人發現。
func Extra(l Locale) []string {
	var out []string
	for k := range catalog[l] {
		if _, ok := catalog[ZhHant][k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// All 回傳某個語系的全部字串，測試檢查字型涵蓋率用。
func All(l Locale) []string {
	var out []string
	for _, v := range catalog[l] {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
