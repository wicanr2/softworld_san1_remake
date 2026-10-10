package game

import (
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/speaker"
)

// Relocalize 只更新顯示文字；姓名來自排入時的原始快照。
// 契約：spec/021 §6.54.3。沒有快照的舊對白沿用既有模板回譯。
func (b *Bubble) Relocalize(from, to i18n.Locale) {
	if b == nil {
		return
	}
	if b.textKey != "" {
		if b.textNamed || b.textName != "" {
			b.Text = i18n.Tf(to, b.textKey, i18n.PersonNameFor(to, b.textName))
		} else {
			b.Text = i18n.T(to, b.textKey)
		}
	} else {
		b.Text = i18n.Relocalize(b.Text, from, to)
	}
}

func (g *State) nameBubbleEvent(x *General, upper, left bool, key string, target *General, salt ...int) Event {
	name, index := "", -1
	if target != nil {
		name, index = target.Name, target.Index
	}
	e := g.bubbleEvent(x, upper, left, tf(key, personName(name)), salt...)
	e.Bubble.textKey, e.Bubble.textName, e.Bubble.textNamed = key, name, true
	e.Bubble.voiceClips, e.Bubble.voiceKnown = speaker.VoiceClipsFor(key, index)
	return e
}

func (g *State) keyBubbleEvent(x *General, upper, left bool, key string, salt ...int) Event {
	e := g.bubbleEvent(x, upper, left, t_(key), salt...)
	e.Bubble.setVoiceKey(key)
	return e
}

func (b *Bubble) setVoiceKey(key string) {
	b.textKey = key
	b.voiceClips, b.voiceKnown = speaker.VoiceClipsFor(key, -1)
}

func (g *State) sayKey(x *General, upper, left bool, key string, salt ...int) {
	if x == nil || x.Name == "" {
		return
	}
	g.pending = append(g.pending, g.keyBubbleEvent(x, upper, left, key, salt...))
}

// VoiceClips 回傳已證實的三段索引。特殊畫面與未知對白不猜語音。
// 映射來自建立時的原始姓名，不受 Relocalize 或後續人物變更影響。
func (b *Bubble) VoiceClips() ([3]int, bool) {
	if b == nil || b.FaceOnly || b.Card || b.Scene != 0 || b.Panel != 0 || b.MapBattle != nil {
		return [3]int{}, false
	}
	return b.voiceClips, b.voiceKnown
}

func (g *State) sayName(x *General, upper, left bool, key string, target *General, salt ...int) {
	if x == nil || x.Name == "" {
		return
	}
	g.pending = append(g.pending, g.nameBubbleEvent(x, upper, left, key, target, salt...))
}

// RelocalizePendingBubbles 更新未移交的顯示佇列，不消耗或重排事件。
func (g *State) RelocalizePendingBubbles(from, to i18n.Locale) {
	for _, e := range g.pending {
		e.Bubble.Relocalize(from, to)
	}
}
