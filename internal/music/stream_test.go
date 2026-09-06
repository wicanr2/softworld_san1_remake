package music

import (
	"encoding/binary"
	"io"
	"testing"
)

// TestStreamKeepsPlaying 釘住 Stream 是無盡的、立體聲的、而且真的有聲音。
func TestStreamKeepsPlaying(t *testing.T) {
	d := data1(t)
	tracks, err := ParseAll(d[SongIndex], d[SongData])
	if err != nil {
		t.Fatal(err)
	}
	tr := tracks[0]
	const rate = 48000
	s := NewStream(tr.Song, tr.Bank, rate)
	if s == nil {
		t.Fatal("開不出 Stream")
	}
	// 讀兩秒，長度要精準，左右聲道要相同。
	buf := make([]byte, rate*4*2)
	n, err := io.ReadFull(s, buf)
	if err != nil || n != len(buf) {
		t.Fatalf("讀了 %d 個位元組（要 %d）：%v", n, len(buf), err)
	}
	loud, mono := 0, true
	for i := 0; i+4 <= len(buf); i += 4 {
		l := int16(binary.LittleEndian.Uint16(buf[i:]))
		r := int16(binary.LittleEndian.Uint16(buf[i+2:]))
		if l != r {
			mono = false
		}
		if l > 500 || l < -500 {
			loud++
		}
	}
	if !mono {
		t.Error("左右聲道不一樣；OPL2 是單聲道，兩邊該相同")
	}
	if loud < rate/2 {
		t.Errorf("兩秒裡只有 %d 個取樣有聲音", loud)
	}
	// 一次讀不到四個位元組也要接得上，不能吐半個取樣。
	small := make([]byte, 3)
	if n, err := s.Read(small); n != 3 || err != nil {
		t.Fatalf("小緩衝讀了 %d 個位元組：%v", n, err)
	}
	if n, err := s.Read(small); n != 3 || err != nil {
		t.Fatalf("小緩衝第二次讀了 %d 個位元組：%v", n, err)
	}
}

// TestStreamSilenceKeepsTime 釘住靜音不會讓曲子停下來。
//
// 關掉音樂再打開要接得上原本的位置。停掉曲子的話，回來會從頭開始——
// 那是另一件事，不是「靜音」。
func TestStreamSilenceKeepsTime(t *testing.T) {
	d := data1(t)
	tracks, err := ParseAll(d[SongIndex], d[SongData])
	if err != nil {
		t.Fatal(err)
	}
	const rate = 48000
	a := NewStream(tracks[0].Song, tracks[0].Bank, rate)
	b := NewStream(tracks[0].Song, tracks[0].Bank, rate)
	half := make([]byte, rate*4/2)
	if _, err := io.ReadFull(a, half); err != nil {
		t.Fatal(err)
	}
	b.SetSilent(true)
	if _, err := io.ReadFull(b, half); err != nil {
		t.Fatal(err)
	}
	for i, v := range half {
		if v != 0 {
			t.Fatalf("靜音時第 %d 個位元組是 %d，不是 0", i, v)
		}
	}
	b.SetSilent(false)
	x, y := make([]byte, rate*4/2), make([]byte, rate*4/2)
	if _, err := io.ReadFull(a, x); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(b, y); err != nil {
		t.Fatal(err)
	}
	for i := range x {
		if x[i] != y[i] {
			t.Fatalf("靜音回來之後對不上原本的位置（第 %d 個位元組 %d／%d）",
				i, x[i], y[i])
		}
	}
}
