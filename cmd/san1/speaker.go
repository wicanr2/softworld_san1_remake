package main

import (
	"fmt"
	"os"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/speaker"
)

// PC 喇叭的音效與語音（`docs/spec/008`）。
//
// 與配樂是兩件事：配樂走 OPL2／AdLib（`internal/music`），這裡走的是
// 原版自己用一個位元取樣硬推出來的聲音。兩者共用同一個音訊環境，
// 各開一個播放器。
//
// 三個要記得的（規格 §3）：
//
//   - 音效關掉 → 語音跟著不出聲，因為原版的 `speak()` 自己就擋在音效那一道。
//   - 一句話由三段接起來。
//   - 原版播完才回，remake 非同步播（規格 R5 的 remake 差異）。

// voicebox 管一條 PC 喇叭的音訊。
type voicebox struct {
	mx   *speaker.Mixer
	bank *speaker.Bank
	p    *audio.Player

	// voice 是放 `R%03d.OKR` 的兩個容器，照原版的順序找。
	voice []*assets.Container

	// sfxDiv／voiceDiv 是分頻值，決定播出來的取樣率（`docs/spec/008` R8）。
	//
	// 原版的音高**本來就因機器而異**（軟體延遲迴圈），所以這裡給得出
	// 一個數字就好，給得出「正確的那一個」是做不到的事。
	// 加強版把它開成選項（`docs/reference/02-web-00-overview`），
	// 原版沒有——**那一項還沒量到加強版怎麼存，所以不造旗標**
	//（`CLAUDE.md` §3.4），先用命令列調。
	sfxDiv, voiceDiv int
}

// SetDivisors 換分頻值。0 或負數表示沿用預設。
func (v *voicebox) SetDivisors(sfx, voice int) {
	if v == nil {
		return
	}
	if sfx > 0 {
		v.sfxDiv = sfx
	}
	if voice > 0 {
		v.voiceDiv = voice
	}
}

// newVoicebox 讀原版的音效與語音。讀不到就回 nil——沒有聲音不該擋著
// 開遊戲，與配樂同一個原則。
func newVoicebox(root string) *voicebox {
	bank := &speaker.Bank{}
	if c, err := openContainer(root, "DATA1"); err == nil {
		if i, ok := c.ByName(speaker.SFXName); ok {
			if err := bank.Load(speaker.SFXSlot, c.Data(i)); err != nil {
				fmt.Fprintln(os.Stderr, "san1: 音效載不進去：", err)
			}
		}
	}
	v := &voicebox{
		bank: bank, mx: speaker.NewMixer(bank, audioRate),
		sfxDiv: speaker.SFXDivisor, voiceDiv: speaker.VoiceDivisor,
	}
	// 語音分在 DATA2（`R000`–`R427`）與 DATA3（`R428`–`R499`）兩個容器。
	for _, name := range []string{"DATA2", "DATA3"} {
		if c, err := openContainer(root, name); err == nil {
			v.voice = append(v.voice, c)
		}
	}
	p, err := audioContext().NewPlayer(v.mx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "san1: PC 喇叭放不出來：", err)
		return nil
	}
	v.p = p
	// 混音器是一條無盡的來源，開場就讓它一直跑：沒東西播的時候是靜音。
	p.Play()
	return v
}

// SetGates 把兩個選項接上聲音。參數是「關掉了嗎」，與存檔欄位同向。
func (v *voicebox) SetGates(soundOff, voiceOff bool) {
	if v == nil {
		return
	}
	v.mx.SetGates(soundOff, voiceOff)
}

// Click 播一次音效（槽 0）。
func (v *voicebox) Click() {
	if v == nil {
		return
	}
	v.mx.Play(speaker.SFXSlot, v.sfxDiv)
}

// Say 播一句話：三個索引依序載進槽 1–3 再接起來播
//（`docs/re/09` §6.2）。索引 −1 表示那一段留白。
func (v *voicebox) Say(idx ...int) {
	if v == nil || len(v.voice) == 0 {
		return
	}
	slots := make([]int, 0, len(idx))
	for i, n := range idx {
		slot := speaker.VoiceLo + i
		if slot > speaker.VoiceHi || n < 0 {
			continue
		}
		b := v.clip(n)
		if b == nil {
			continue
		}
		if err := v.bank.Load(slot, b); err != nil {
			fmt.Fprintln(os.Stderr, "san1: 語音載不進去：", err)
			continue
		}
		slots = append(slots, slot)
	}
	if len(slots) > 0 {
		v.mx.Say(v.voiceDiv, slots...)
	}
}

// clip 從容器裡找一段語音。找不到回 nil——**不要回一段別的**，
// 那會讓「索引對照表錯了」聽起來只是「講錯話」。
func (v *voicebox) clip(n int) []byte {
	name := speaker.VoiceName(n)
	for _, c := range v.voice {
		if i, ok := c.ByName(name); ok {
			return c.Data(i)
		}
	}
	return nil
}
