package main

import (
	"math"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/speaker"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// 大地圖上的戰役動畫的播放（`docs/spec/005`「大地圖上的戰役」）。
//
// 原版每一格前後各叫 `speak(0, 速度)`，喇叭播完才回，所以動畫的快慢就是
// 那幾聲的長度。remake 非同步播聲音，每一格停的幀數照同一段音效在那個
// 速度下的長度算（**remake 差異**：原版的音高與長度本來就隨機器而異，
// `docs/spec/008`）。讀不到音效時一聲算 3 幀。原版不收鍵，這裡也不收。
type mapBattlePlay struct {
	b    *game.Bubble
	m    *ui.March
	left int  // 目前這一格還要停幾幀
	done bool // 最後一格（收尾）已經畫了
}

// sfxSamples 是音效那一段的取樣數，開機時從 `DATA1` 讀；0 表示沒讀到。
var sfxSamples int

// speakDivisor 是 `speak(0, 速度)` 的分頻值：`speaker.SFXDivisor` 量自速度 10。
func speakDivisor(speed, base int) int {
	if speed < 10 {
		speed = 10
	}
	return speed * base / 10
}

// speakFrames 是那一聲播多少幀。
func speakFrames(speed int) int {
	if sfxSamples == 0 {
		return 3
	}
	sec := float64(sfxSamples) / speaker.Rate(speakDivisor(speed, speaker.SFXDivisor))
	return max(1, int(math.Round(sec*60)))
}

// updateMapBattle 每幀叫一次；播完回 true。
func (a *app) updateMapBattle(b *game.Bubble) bool {
	p := a.mapBattle
	if p == nil || p.b != b {
		if a.marchArt == nil {
			return true
		}
		mb := b.MapBattle
		at, to := a.s.G.Prefecture(mb.Attacker), a.s.G.Prefecture(mb.Defender)
		if at == nil || to == nil {
			return true
		}
		l := ui.NewMarchLayout(int(at.MapX), int(at.MapY), int(to.MapX), int(to.MapY))
		sel := a.view.Sel
		p = &mapBattlePlay{b: b, m: ui.NewMarch(l, *a.marchArt, mb.Days, a.art.Compose(a.s.G, sel), nil)}
		a.mapBattle = p
	}
	if p.left > 0 {
		p.left--
		return false
	}
	if p.done {
		a.mapBattle = nil
		return true
	}
	before, after := p.m.Speeds(p.m.Frame())
	for _, s := range before {
		a.speak(s)
		p.left += speakFrames(s)
	}
	if !p.m.Step() {
		p.done = true
	}
	for _, s := range after {
		a.speak(s)
		p.left += speakFrames(s)
	}
	return false
}

// speak 播一聲 `speak(0, 速度)`。
func (a *app) speak(speed int) {
	if a.sfx == nil {
		return
	}
	a.sfx.mx.Play(speaker.SFXSlot, speakDivisor(speed, a.sfx.sfxDiv))
}
