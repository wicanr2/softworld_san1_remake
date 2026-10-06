package game

import "github.com/wicanr2/softworld_san1_remake/internal/i18n"

// Relocalize 只更新顯示文字；姓名來自排入時的原始快照。
// 契約：spec/021 §6.54.3。沒有快照的舊對白沿用既有模板回譯。
func (b *Bubble) Relocalize(from, to i18n.Locale) {
	if b == nil {
		return
	}
	if b.textKey != "" {
		b.Text = i18n.Tf(to, b.textKey, i18n.PersonNameFor(to, b.textName))
	} else {
		b.Text = i18n.Relocalize(b.Text, from, to)
	}
}

func (g *State) nameBubbleEvent(x *General, upper, left bool, key, name string, salt ...int) Event {
	e := g.bubbleEvent(x, upper, left, tf(key, personName(name)), salt...)
	e.Bubble.textKey, e.Bubble.textName = key, name
	if x != nil {
		switch {
		case key == "bub.warDeclare" && x.Name == "劉備" && name == "孔融":
			e.Bubble.voiceClips, e.Bubble.voiceKnown = [3]int{32, 456, 499}, true
		case key == "bub.warReply" && x.Name == "孔融" && name == "劉備":
			e.Bubble.voiceClips, e.Bubble.voiceKnown = [3]int{0, 457, 499}, true
		}
	}
	return e
}

// VoiceClips 回傳已證實的三段索引。特殊畫面與未知對白不猜語音。
// 映射來自建立時的原始姓名，不受 Relocalize 或後續人物變更影響。
func (b *Bubble) VoiceClips() ([3]int, bool) {
	if b == nil || b.FaceOnly || b.Card || b.Scene != 0 || b.Panel != 0 || b.MapBattle != nil {
		return [3]int{}, false
	}
	return b.voiceClips, b.voiceKnown
}

func (g *State) sayName(x *General, upper, left bool, key, name string, salt ...int) {
	if x == nil || x.Name == "" {
		return
	}
	g.pending = append(g.pending, g.nameBubbleEvent(x, upper, left, key, name, salt...))
}

// RelocalizePendingBubbles 更新未移交的顯示佇列，不消耗或重排事件。
func (g *State) RelocalizePendingBubbles(from, to i18n.Locale) {
	for _, e := range g.pending {
		e.Bubble.Relocalize(from, to)
	}
}
