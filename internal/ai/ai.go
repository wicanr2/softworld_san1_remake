// Package ai 是電腦諸侯的決策。
//
// **三個版本並列**，由旗標選：
//
//	base      三國演義（原版）的 AI —— 以還原原版行為為目標
//	plus      三國演義1加強版的 AI —— 同上，加強版另有一份
//	enhanced  remake 強化 AI —— remake 自己的，不宣稱與任何原版相同
//
// ⚠ **前兩個是還原，第三個是創作。** 兩者的驗收標準完全不同：
// `base`／`plus` 要與原版對拍（同一個局面下同一串命令），
// `enhanced` 只要「玩起來好」。把它們混在一起——例如在 `base` 裡
// 「順手改好一點」——會讓還原失去意義，而且**沒有人看得出來**，
// 因為兩者的輸出都是合法的命令。
//
// 還原用的 AI 目前**還沒解**（原版的決策程式碼還沒反組譯到）。
// 它們不會假裝自己會下棋：`Derived()` 回 false，`Plan` 回空，
// 呼叫端必須把這件事顯示出來。
package ai

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// Mode 是 AI 的版本。
type Mode string

const (
	ModeBase     Mode = "base"
	ModePlus     Mode = "plus"
	ModeEnhanced Mode = "enhanced"
)

// Modes 是全部三個版本，順序固定（旗標說明與選單都用它）。
func Modes() []Mode { return []Mode{ModeBase, ModePlus, ModeEnhanced} }

// Brain 是一個 AI。
type Brain interface {
	Mode() Mode

	// Name 是給人看的名字。
	Name() string

	// Derived 回報這個 AI 是不是已經從原版還原出來的。
	//
	// **false 表示它不會下棋**，不是「它比較弱」。呼叫端要把這件事
	// 顯示出來——一個安靜地什麼都不做的電腦諸侯，在畫面上看起來
	// 就只是「這個諸侯這回合沒動作」。
	Derived() bool

	// Plan 回傳某個勢力這個月要下的命令。**不改變局面**。
	Plan(g *game.State, f state.FactionID) []game.Order
}

// New 依版本造一個 AI。
func New(m Mode) (Brain, error) {
	switch m {
	case ModeBase:
		return &faithful{mode: ModeBase, name: "三國演義（原版）"}, nil
	case ModePlus:
		return &faithful{mode: ModePlus, name: "三國演義1加強版"}, nil
	case ModeEnhanced:
		return &enhanced{}, nil
	}
	return nil, fmt.Errorf("ai: 不認識的版本 %q（有 %v）", m, Modes())
}

// faithful 是「以還原原版為目標」的 AI 的共同外殼。
//
// 原版的決策程式碼還沒反組譯到，所以它現在不下任何命令。
// **這是刻意的空實作，不是佔位的假 AI**——填一個「差不多的」策略進去，
// 之後就再也分不出哪些行為是還原的、哪些是我編的。
type faithful struct {
	mode Mode
	name string
}

func (f *faithful) Mode() Mode                                     { return f.mode }
func (f *faithful) Name() string                                   { return f.name }
func (f *faithful) Derived() bool                                  { return false }
func (f *faithful) Plan(*game.State, state.FactionID) []game.Order { return nil }
