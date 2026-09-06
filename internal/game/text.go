package game

import (
	"errors"

	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

// 給玩家看的字。
//
// 規則層自己的錯誤（`game:` 開頭的那些）是**程式錯誤**，留原文；
// 會出現在畫面上的——被擋下來的理由、事件紀錄、戰報——走這裡。
//
// 領域型別的 `String()` 一律回繁體中文：那是遊戲的原文，測試與文件
// 都靠它。畫面要的是譯文，所以另外按編號查一次表——編號是資料的
// 一部分，不會因為語系而變。

func t(key string) string            { return i18n.S(key) }
func tf(key string, a ...any) string { return i18n.Sf(key, a...) }

// personName／placeName 把遊戲資料裡的專有名詞換成目前語系的寫法。
//
// **原文欄位不動。** `General.Name` 與 `Prefecture.Name` 一律保持原版的
// 位元組——存檔要照原版版面寫回去，測試也靠它。翻譯只發生在要顯示的
// 那一刻，與 `ErrorText` 同一個道理。
func personName(s string) string { return i18n.PersonName(s) }
func placeName(s string) string  { return i18n.PlaceName(s) }

// PlotName 是五種謀略的名稱（說明書 p.24–25）。
func PlotName(p Plot) string {
	keys := []string{"", "plot.tiger", "plot.distant", "plot.forge",
		"plot.incite", "plot.joint"}
	if int(p) < len(keys) && keys[p] != "" {
		return t(keys[p])
	}
	return "?"
}

// AutonomyName 是四種自治型態的名稱（說明書 p.23–24）。
func AutonomyName(a Autonomy) string {
	keys := []string{"auto.normal", "auto.civil", "auto.military", "auto.self"}
	if int(a) < len(keys) {
		return t(keys[a])
	}
	return "?"
}

// TreasureName 是五件寶物的名稱（說明書 p.24）。
func TreasureName(x Treasure) string {
	keys := []string{"tre.seal", "tre.book", "tre.blade", "tre.beauty", "tre.horse"}
	if int(x) < len(keys) {
		return t(keys[x])
	}
	return "?"
}

// EraName 是一個年號的名稱。英文用羅馬拼音，日文用同樣的漢字。
func EraName(name string) string { return t("era." + name) }

// ErrorText 把一個被擋下來的理由翻成目前的語系。
//
// 具名錯誤是**識別用的**，值固定是繁體中文原文，上層才比得出來
// （`errors.Is`）。翻譯只發生在要顯示的那一刻——把 `fmt.Errorf` 的
// 內容換成譯文的話，`errors.Is` 仍然成立，但錯誤在紀錄裡會隨語系變，
// 對不了帳。
func ErrorText(err error) string {
	if err == nil {
		return ""
	}
	for _, e := range []struct {
		err error
		key string
	}{
		{ErrNotYours, "err.notYours"},
		{ErrAlreadyMoved, "err.commanded"},
		{ErrNoGold, "err.noGold"},
		{ErrNoPeople, "err.noPeople"},
		{ErrNoRoom, "err.troopCap"},
		{ErrUnknownUnit, "err.noUnit"},
		{ErrNoRice, "err.noRice"},
		{ErrNotAdjacent, "err.notAdjacent"},
		{ErrNeedIntel80, "err.needIntel"},
		{ErrTooManyForts, "err.fortCap"},
		{ErrNotLord, "err.lordOnly"},
		{ErrLordCantBe, "err.lordCantBe"},
		{ErrAlreadyPaid, "err.alreadyPaid"},
		{ErrNoTreasure, "err.noTreasure"},
		{ErrCantGift, "err.cantGift"},
		{ErrNoGovernor, "err.noGovernor"},
		{ErrNoOne, "err.noCandidate"},
		{ErrTooManyGens, "err.tooManyGens"},
		{ErrNoChief, "err.noChief"},
	} {
		if errors.Is(err, e.err) {
			return t(e.key)
		}
	}
	return err.Error()
}
