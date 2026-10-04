package i18n

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var formatSlot = regexp.MustCompile(`%[0-9]*[sd]`)

// Relocalize 只回譯能完整匹配字串表的 UI 提示；不猜歷史敘事。
func Relocalize(text string, from, to Locale) string {
	if from == to || text == "" {
		return text
	}
	keys := Keys()
	for _, key := range keys {
		if text == T(from, key) {
			return T(to, key)
		}
	}
	// 先比對具體的提示，避免通用「%s」模板吞掉整句。
	literalWeight := func(key string) int {
		s := T(from, key)
		for _, slot := range formatSlot.FindAllString(s, -1) {
			s = strings.Replace(s, slot, "", 1)
		}
		// 只有空白或標點的模板無法辨識提示，會把未知文字誤當成 UI。
		if !strings.ContainsFunc(s, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
			return 0
		}
		return len(s)
	}
	sort.SliceStable(keys, func(i, j int) bool { return literalWeight(keys[i]) > literalWeight(keys[j]) })
	for _, key := range keys {
		template := T(from, key)
		slots := formatSlot.FindAllStringIndex(template, -1)
		if len(slots) == 0 || literalWeight(key) == 0 {
			continue
		}
		var pattern strings.Builder
		pattern.WriteString("(?s)^")
		at := 0
		for _, slot := range slots {
			pattern.WriteString(regexp.QuoteMeta(template[at:slot[0]]))
			if template[slot[1]-1] == 'd' {
				pattern.WriteString(` *(-?[0-9]+)`)
			} else {
				pattern.WriteString(`(.*?)`)
			}
			at = slot[1]
		}
		pattern.WriteString(regexp.QuoteMeta(template[at:]))
		pattern.WriteString("$")
		match := regexp.MustCompile(pattern.String()).FindStringSubmatch(text)
		if match == nil {
			continue
		}
		args := make([]any, len(slots))
		valid := true
		for i, slot := range slots {
			if template[slot[1]-1] == 'd' {
				n, err := strconv.Atoi(match[i+1])
				if err != nil {
					valid = false
					break
				}
				args[i] = n
			} else {
				args[i] = match[i+1]
			}
		}
		if valid {
			return Tf(to, key, args...)
		}
	}
	return text
}
