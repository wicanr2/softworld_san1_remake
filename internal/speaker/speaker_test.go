package speaker

import (
	"bytes"
	"math"
	"testing"
)

func TestClipReadsBitsHighFirst(t *testing.T) {
	// 0x16 ＝ 0001 0110。最高位先送。
	c := NewClip([]byte{0x16})
	want := []bool{false, false, false, true, false, true, true, false}
	if c.Samples() != len(want) {
		t.Fatalf("取樣數 %d，想要 %d", c.Samples(), len(want))
	}
	for i, w := range want {
		if got := c.Bit(i); got != w {
			t.Errorf("第 %d 個取樣 %v，想要 %v", i, got, w)
		}
	}
	// 界外不該爆，回靜音位準。
	if c.Bit(-1) || c.Bit(8) {
		t.Error("界外的取樣不是 false")
	}
}

func TestBankEnforcesTheOriginalLimits(t *testing.T) {
	var b Bank
	if err := b.Load(SFXSlot, make([]byte, MaxSFXBytes)); err != nil {
		t.Fatalf("槽 0 剛好 %d 個位元組被擋：%v", MaxSFXBytes, err)
	}
	if err := b.Load(SFXSlot, make([]byte, MaxSFXBytes+1)); err == nil {
		t.Errorf("槽 0 超過 %d 個位元組沒被擋——原版會印 Real#0>1000", MaxSFXBytes)
	}
	if err := b.Load(1, make([]byte, MaxVoiceBytes)); err != nil {
		t.Fatalf("槽 1 剛好 %d 個位元組被擋：%v", MaxVoiceBytes, err)
	}
	if err := b.Load(1, make([]byte, MaxVoiceBytes+1)); err == nil {
		t.Errorf("槽 1 超過 %d 個位元組沒被擋", MaxVoiceBytes)
	}
	// 槽 0 的上限比語音嚴：1000 < 4200，不能共用一個數字。
	if err := b.Load(SFXSlot, make([]byte, MaxSFXBytes+1)); err == nil {
		t.Error("槽 0 吃到了語音的上限")
	}
	for _, s := range []int{-1, SlotCount} {
		if err := b.Load(s, []byte{0}); err == nil {
			t.Errorf("槽 %d 沒被擋——原版會印 Invalid SND", s)
		}
	}
}

func TestVoiceNameClampsLikeTheOriginal(t *testing.T) {
	for _, tc := range []struct {
		in   int
		want string
	}{{0, "R000.OKR"}, {7, "R007.OKR"}, {427, "R427.OKR"},
		{499, "R499.OKR"}, {500, "R499.OKR"}, {-3, "R000.OKR"}} {
		if got := VoiceName(tc.in); got != tc.want {
			t.Errorf("VoiceName(%d) ＝ %s，想要 %s", tc.in, got, tc.want)
		}
	}
}

func TestRateFollowsTheDelayLoop(t *testing.T) {
	// 分頻值越大越慢。
	if Rate(13) <= Rate(130) {
		t.Errorf("分頻 13（%.0f Hz）沒有比分頻 130（%.0f Hz）快",
			Rate(13), Rate(130))
	}
	// 原版自己把分頻值夾在 10 以上，所以更小的值不該算出更高的取樣率。
	if Rate(1) != Rate(10) {
		t.Errorf("分頻 1 沒有被夾到 10：%.0f vs %.0f", Rate(1), Rate(10))
	}
	// 模型本身：16 MHz ÷ (8×13+45)。
	if want := float64(RefCPUHz) / float64(8*13+45); math.Abs(Rate(13)-want) > 1 {
		t.Errorf("Rate(13) ＝ %.0f，模型算出來是 %.0f", Rate(13), want)
	}
}

func TestRenderResamplesAndSmooths(t *testing.T) {
	c := NewClip(bytes.Repeat([]byte{0xF0}, 16)) // 四高四低
	src, dst := 10000.0, 40000.0
	got := Render(c, src, dst)
	if want := c.Samples() * 4; len(got) != want {
		t.Fatalf("輸出 %d 個取樣，想要 %d", len(got), want)
	}
	// 兩個位準都要出現，否則等於一條直線。
	var lo, hi int
	for _, v := range got {
		if v > 0 {
			hi++
		} else if v < 0 {
			lo++
		}
	}
	if lo == 0 || hi == 0 {
		t.Errorf("只有一個位準（正 %d 負 %d）", hi, lo)
	}
	// 低通之後不該超過位準本身。
	for i, v := range got {
		if v > amplitude || v < -amplitude {
			t.Fatalf("第 %d 個取樣 %d 超出位準 ±%d", i, v, amplitude)
		}
	}
	if len(Render(Clip{}, src, dst)) != 0 {
		t.Error("空的一段算出了取樣")
	}
}

func TestMixerGates(t *testing.T) {
	var b Bank
	if err := b.Load(SFXSlot, bytes.Repeat([]byte{0xF0}, 64)); err != nil {
		t.Fatal(err)
	}
	if err := b.Load(1, bytes.Repeat([]byte{0x0F}, 64)); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name               string
		soundOff, voiceOff bool
		sfx, voice         bool
	}{
		{"兩個都開", false, false, true, true},
		{"只關語音", false, true, true, false},
		{"關音效：語音跟著不出聲", true, false, false, false},
		{"兩個都關", true, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewMixer(&b, 48000)
			m.SetGates(tc.soundOff, tc.voiceOff)
			m.Play(SFXSlot, SFXDivisor)
			if got := m.Busy(); got != tc.sfx {
				t.Errorf("音效 Busy ＝ %v，想要 %v", got, tc.sfx)
			}
			m2 := NewMixer(&b, 48000)
			m2.SetGates(tc.soundOff, tc.voiceOff)
			m2.Say(VoiceDivisor, 1, 2, 3)
			if got := m2.Busy(); got != tc.voice {
				t.Errorf("語音 Busy ＝ %v，想要 %v", got, tc.voice)
			}
		})
	}
}

func TestMixerReadNeverEnds(t *testing.T) {
	var b Bank
	if err := b.Load(SFXSlot, bytes.Repeat([]byte{0xF0}, 8)); err != nil {
		t.Fatal(err)
	}
	m := NewMixer(&b, 48000)
	buf := make([]byte, 4096)

	// 沒東西播的時候是靜音，而且**不回 EOF**——回了播放器就停掉，
	// 之後再有聲音也播不出來。
	n, err := m.Read(buf)
	if err != nil || n != len(buf) {
		t.Fatalf("靜音時讀到 %d 個位元組、err ＝ %v", n, err)
	}
	if !bytes.Equal(buf, make([]byte, len(buf))) {
		t.Error("沒東西播的時候不是靜音")
	}

	m.Play(SFXSlot, SFXDivisor)
	total := 0
	nonZero := false
	for i := 0; i < 64 && m.Busy(); i++ {
		n, err := m.Read(buf)
		if err != nil {
			t.Fatalf("讀到錯誤：%v", err)
		}
		total += n
		for _, v := range buf[:n] {
			if v != 0 {
				nonZero = true
			}
		}
	}
	if !nonZero {
		t.Error("播了音效卻只讀到靜音")
	}
	if m.Busy() {
		t.Error("讀了 64 輪還沒播完")
	}
	// 左右聲道相同。
	m.Play(SFXSlot, SFXDivisor)
	if _, err := m.Read(buf); err != nil {
		t.Fatal(err)
	}
	for i := 0; i+4 <= len(buf); i += 4 {
		if buf[i] != buf[i+2] || buf[i+1] != buf[i+3] {
			t.Fatalf("第 %d 格左右聲道不同", i/4)
		}
	}
}

// 半格的要求不該把取樣切半。
func TestMixerReadKeepsFrameAlignment(t *testing.T) {
	m := NewMixer(&Bank{}, 48000)
	n, err := m.Read(make([]byte, 6))
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Errorf("要 6 個位元組，給了 %d——一格是 4 個位元組", n)
	}
	if n, _ := m.Read(make([]byte, 3)); n != 0 {
		t.Errorf("不足一格時給了 %d 個位元組", n)
	}
}
