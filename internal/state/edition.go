package state

import "fmt"

// Edition 是原版與加強版。
//
// **旗標只切已經量到的差異**（`docs/spec/004`）：目前是難度上限與電腦
// 諸侯的出兵係數表，兩項都有 `L0` 出處。量不到的不預先造欄位——造出來
// 的旗標會固定住一個還沒驗證的假設（`CLAUDE.md` §3.4）。
type Edition string

const (
	// EditionBase 是 1991 年的原版（`AA.EXE`）。
	EditionBase Edition = "base"
	// EditionPlus 是加強版（`ASV.EXE`）。
	EditionPlus Edition = "plus"
)

// Editions 是全部版本，順序固定（旗標說明與選單都用它）。
func Editions() []Edition { return []Edition{EditionBase, EditionPlus} }

func (e Edition) String() string {
	switch e {
	case EditionBase:
		return "三國演義（原版）"
	case EditionPlus:
		return "三國演義1加強版"
	}
	return string(e)
}

// Valid 回報這是不是認得的版本。
func (e Edition) Valid() bool { return e == EditionBase || e == EditionPlus }

// MaxDifficulty 是開局時難度的上限（`docs/spec/004` §2–3）。
//
// 原版問「請設定難度(1-10)」，加強版問「請設定難度(1-20)」；
// 係數表也跟著從 11 格長到 21 格。
func (e Edition) MaxDifficulty() int {
	if e == EditionPlus {
		return 20
	}
	return 10
}

// ParseEdition 把旗標字串轉成版本。
func ParseEdition(s string) (Edition, error) {
	e := Edition(s)
	if !e.Valid() {
		return "", fmt.Errorf("state: 不認識的版本 %q（有 %v）", s, Editions())
	}
	return e, nil
}
