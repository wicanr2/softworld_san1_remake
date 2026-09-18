package speaker

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

// TestVoicePlaysInEveryLocale 釘住**語音不看語系**（Issue #33）。
//
// 裁定是英日版沿用原版的中文語音，不靜音也不另配。語音會不會出聲
// 只由「其他 → 音效」與「其他 → 語音」兩個開關決定（`Mixer.SetGates`）——
// 多加一道語系的閘門，英日文玩家聽到的就是一片安靜，而**安靜與
// 「這台機器沒有音效卡」在畫面上長得一模一樣**。
func TestVoicePlaysInEveryLocale(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	var b Bank
	if err := b.Load(VoiceLo, []byte{0x16, 0x5a, 0x33}); err != nil {
		t.Fatal(err)
	}
	for _, l := range i18n.Locales() {
		i18n.Current = l
		m := NewMixer(&b, 44100)
		m.Say(VoiceDivisor, VoiceLo)
		if !m.Busy() {
			t.Errorf("%s：語音沒有排進佇列", l)
		}
	}
	// 反向對照：關掉語音開關才該安靜。
	i18n.Current = i18n.ZhHant
	m := NewMixer(&b, 44100)
	m.SetGates(false, true)
	m.Say(VoiceDivisor, VoiceLo)
	if m.Busy() {
		t.Error("語音開關關著卻照樣排進佇列")
	}
}
