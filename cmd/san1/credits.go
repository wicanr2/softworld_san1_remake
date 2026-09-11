package main

import (
	"fmt"
	"os"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// 製作群（`docs/spec/012`）。
//
// 統一天下之後跑：先定格在朝堂圖，按鍵之後字幕從山後面升起來。
// **這是保存專案**——作者的名字在原版裡，remake 就要放得出來。
//
// ⚠ 原版什麼時候播這一段**沒解**（那一段跑在 `DATA0.GRP`／`DATA4.GRP`
// 兩支還沒反組譯的 overlay）。接在統一之後是 remake 的推測，記在規格裡。

// creditsSpeed 是字幕一幀往上走幾個像素。
//
// 原版的速度沒量到——它由那支 overlay 的迴圈決定。2 像素／幀在 60 fps
// 下走完約 15 秒，讀得完。
const creditsSpeed = 2

// creditsPlay 是一次播放的狀態。
type creditsPlay struct {
	art *assets.Credits
	// hall 為真表示還停在朝堂圖那一格。
	hall   bool
	scroll int
}

// startCredits 開始播。讀不到素材就回 nil——**沒有製作群不該擋著結局**。
func (a *app) startCredits() {
	if a.credits != nil || a.creditsDone || a.c2 == nil {
		return
	}
	cr, err := assets.LoadCredits(a.c2)
	if err != nil {
		fmt.Fprintln(os.Stderr, "san1: 沒有製作群：", err)
		a.creditsDone = true
		return
	}
	a.credits = &creditsPlay{art: cr, hall: true}
	a.dirty = true
}

// updateCredits 收製作群那一段的輸入。回傳「這一幀被它吃掉了」。
func (a *app) updateCredits() bool {
	cp := a.credits
	if cp == nil {
		return false
	}
	if cp.hall {
		// 朝堂圖停著等按鍵——**不自動跳過**：那是結局的定格。
		if anyKeyPressed() {
			cp.hall = false
			a.dirty = true
		}
		return true
	}
	if anyKeyPressed() {
		a.endCredits()
		return true
	}
	cp.scroll += creditsSpeed
	if cp.scroll >= ui.CreditsLength(cp.art) {
		a.endCredits()
	}
	a.dirty = true
	return true
}

// endCredits 收掉，並記住**這一局不再播第二次**。
func (a *app) endCredits() {
	a.credits, a.creditsDone = nil, true
	a.dirty = true
}
