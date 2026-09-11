package game

import (
	"fmt"
	"strconv"

	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

// 年號（說明書 p.26：「其他 → 年號」，可用西曆或中曆）。
//
// 原版的切換提示是 `使用%s年號`，`%s` 取 `中曆` 或 `西曆`
//（`AA.EXE` `0x48634`／`0x48640`／`0x48645`，`docs/re/04` §3）。
//
// ⚠ **這張表是 remake 補的。** 原版的執行檔裡只有四個年號字樣，
// 而且都嵌在六個劇本的選單標題裡（` 1 中平 六 年 ` 之類，`0x475ef`），
// 沒有可供查表的年號表。所以原版開局之後怎麼數年號**還沒查證**：
// 有可能像這裡一樣換年號，也有可能一路數下去（中平七年、中平八年…）。
//
// 這裡選歷史上的年號，因為它在手冊給的六個時期上全部對得上：
// 中平六年 189、興平二年 195、建安六年 201、建安十三年 208、
// 建安二十年 215、黃初元年 220。要驗證得在 dosgolem 裡把原版推過一年。

// Era 是一個年號的起訖。Start 是它的元年（西元）。
type Era struct {
	Name  string
	Start int
	End   int // 最後一年（含）
}

// eras 是漢末到西晉初的年號。
//
// 改元多半在年中，一年可能橫跨兩個年號（例如西元 220 年先後有
// 建安、延康、黃初）。原版顯示的是「哪一年」不是「哪一天」，
// 所以這裡一年只給一個年號，取**該年結束時行用的那一個**。
var eras = []Era{
	{"中平", 184, 189},
	{"初平", 190, 193},
	{"興平", 194, 195},
	{"建安", 196, 219},
	{"黃初", 220, 226},
	{"太和", 227, 232},
	{"青龍", 233, 236},
	{"景初", 237, 239},
	{"正始", 240, 248},
	{"嘉平", 249, 253},
	{"正元", 254, 255},
	{"甘露", 256, 259},
	{"景元", 260, 263},
	{"咸熙", 264, 264},
	{"泰始", 265, 274},
	{"咸寧", 275, 279},
	{"太康", 280, 289},
}

// Eras 回傳年號表。
func Eras() []Era { return append([]Era(nil), eras...) }

// EraOf 回傳某一西元年的年號與那是該年號的第幾年。
//
// 表外的年份回 false——**不要外推**。一個「太康三十七年」看起來像
// 正常運作，只有懂的人才看得出那是編的。
func EraOf(year int) (name string, nth int, ok bool) {
	for _, e := range eras {
		if year >= e.Start && year <= e.End {
			return e.Name, year - e.Start + 1, true
		}
	}
	return "", 0, false
}

// 中文數字。年號的年份最多兩位數，月份一到十二。
var digits = [...]string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}

// Chinese 把 1..99 寫成中文數字（元年另計，見 Era 的用法）。
func Chinese(n int) string {
	switch {
	case n < 0 || n > 99:
		return fmt.Sprint(n)
	case n < 10:
		return digits[n]
	case n < 20:
		if n == 10 {
			return "十"
		}
		return "十" + digits[n%10]
	default:
		s := digits[n/10] + "十"
		if n%10 != 0 {
			s += digits[n%10]
		}
		return s
	}
}

// Calendar 是年月的表示方式。
type Calendar int

const (
	// ChineseEra 是中曆年號，原版的預設（手冊 p.26）。
	ChineseEra Calendar = iota
	// Western 是西曆。
	Western
)

// Name 是切換提示裡的那兩個字（原版的 `中曆`／`西曆`）。
func (c Calendar) Name() string {
	if c == Western {
		return "西曆"
	}
	return "中曆"
}

// Format 把一個年月寫成畫面上的樣子。
//
// 中曆：`中平六年元月`。**正月寫「元月」**——原版的劇本標題是
// 「黃初元年」，主畫面是「中平六年元月」，兩個「元」是不同的東西：
// 前者是年號的第一年，後者是一年的第一個月。
//
// 年號表涵蓋不到的年份自動退回西曆，不硬掰一個年號出來。
func (d Date) Format(c Calendar) string {
	if c == Western {
		return tf("date.western", d.Year, d.Month)
	}
	name, nth, ok := EraOf(d.Year)
	if !ok {
		return tf("date.western", d.Year, d.Month)
	}
	year := numeral(nth)
	if nth == 1 {
		year = t("date.first")
	}
	month := numeral(d.Month)
	if d.Month == 1 {
		month = t("date.first")
	}
	return tf("date.era", EraName(name), year, month)
}

// FormatWithSeason 是**原版寫在畫面上的日期**：年月之後再接一個季節字。
//
// 主畫面左側直排與主戰場下方花邊都是這樣寫的——原版三張畫面對出來的是
// 元月春、四月夏、八月秋（`docs/mechanics/50-events` §1，`L1`）。
// `Format` 不含季節，因為訊息裡的日期不寫季節。
//
// ⚠ **西曆那條路沒有樣本**：原版切成西曆時直排寫不寫季節沒量過。
// 這裡照寫——季節是月份算出來的，與曆法無關。
func (d Date) FormatWithSeason(c Calendar) string {
	return d.Format(c) + d.Season().Name()
}

// numeral 是年月在畫面上的寫法。
//
// 中曆是「中平六年元月」不是「中平6年1月」，所以中日文用中文數字；
// 英文的年號名已經是拼音，數字跟著用阿拉伯數字才讀得下去。
func numeral(n int) string {
	if i18n.Current == i18n.En {
		return strconv.Itoa(n)
	}
	return Chinese(n)
}
