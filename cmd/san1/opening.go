package main

import (
	"iter"
	"math/rand/v2"

	"github.com/wicanr2/softworld_san1_remake/internal/opening"
)

// 開機片頭的播放（`docs/spec/005`「片頭」）。畫面怎麼搬由 `internal/opening`
// 照原版逐拍重做；這一層只決定每一拍停多久、按鍵怎麼處理。
//
// 停多久（**remake 差異**，原版的節拍來自 DOS 時鐘與 CPU 速度）：
//
//   - 等秒數 n 次：n 秒。原版是「秒數那一欄跳 n 次」，第一次跳在 0–1 秒之間。
//   - 等 tick n 次：n ÷ 18.2 秒。
//   - 動畫的一步：原版沒有延遲、照 CPU 速度跑；remake 頭像橫幅一幀走 4 步、
//     三英圖捲入一幀走 2 步。
//   - 「程式載入中」：原版停到主程式載入完，remake 停半秒。
//
// 按鍵照原版：等待中按鍵只結束那一段等待；動畫那一步收到按鍵就放棄片頭
// 剩下的部分，直接到「程式載入中」；三英圖停住之後等一個鍵。
type openingPlayer struct {
	script *opening.Script
	next   func() (opening.Beat, bool)
	stop   func()
	beat   opening.Beat
	left   int // 這一拍還要停幾幀
}

// newOpeningPlayer 從第一拍開始；片頭一拍都沒有時回 nil。
func newOpeningPlayer(art *opening.Art) *openingPlayer {
	s := &opening.Script{Art: art, Rand: rand.IntN}
	next, stop := iter.Pull(s.Beats())
	p := &openingPlayer{script: s, next: next, stop: stop}
	if !p.advance() {
		stop()
		return nil
	}
	return p
}

// advance 換到下一拍；片頭結束回 false。
func (p *openingPlayer) advance() bool {
	b, ok := p.next()
	if !ok {
		return false
	}
	p.beat = b
	switch b.Kind {
	case opening.HoldSeconds:
		p.left = b.N * 60
	case opening.HoldTicks:
		p.left = max(1, (b.N*600+91)/182)
	case opening.End:
		p.left = 30
	default:
		p.left = 1
	}
	return true
}

// stepsPerFrame 是動畫那一拍一幀走幾步。
func stepsPerFrame(site opening.Site) int {
	if site == opening.SiteRibbon {
		return 4
	}
	return 2
}

// update 每幀叫一次。changed 表示畫面換了，done 表示片頭播完。
func (p *openingPlayer) update(key bool) (changed, done bool) {
	switch p.beat.Kind {
	case opening.WaitKey:
		if !key {
			return false, false
		}
	case opening.Step:
		if key {
			p.script.Aborted = true
		}
		n := stepsPerFrame(p.beat.Site)
		for i := 0; i < n; i++ {
			if !p.advance() {
				return true, true
			}
			if p.beat.Kind != opening.Step {
				break
			}
		}
		return true, false
	default:
		if key {
			p.left = 0
		} else {
			p.left--
		}
		if p.left > 0 {
			return false, false
		}
	}
	if !p.advance() {
		return true, true
	}
	return true, false
}
