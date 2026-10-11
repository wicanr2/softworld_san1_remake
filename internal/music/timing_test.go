package music

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestOriginalSongTiming(t *testing.T) {
	d := data1(t)
	for _, set := range []struct {
		idx, grp string
		seconds  []float64
	}{
		{SongIndex, SongData, []float64{38.4, 48, 48, 32, 25.6}},
		{LongIndex, LongData, []float64{165.6}},
	} {
		tracks, err := ParseAll(d[set.idx], d[set.grp])
		if err != nil {
			t.Fatal(err)
		}
		for i, tr := range tracks {
			s := tr.Song
			if s.BeatTicks != 240 || math.Abs(s.Duration()-set.seconds[i]) > 1e-8 {
				t.Fatalf("%s %d: division=%d duration=%f", set.grp, i, s.BeatTicks, s.Duration())
			}
			if last := s.Events[len(s.Events)-1].At; last != s.Ticks {
				t.Fatalf("最後刻度=%d，表頭=%d", last, s.Ticks)
			}
			if got := binary.BigEndian.Uint16(s.MIDI()[12:14]); got != 240 {
				t.Fatalf("MIDI division=%d", got)
			}
		}
	}
}

func TestOverflowAdvancesTime(t *testing.T) {
	e, err := parseEvents([]byte{0, 0xC0, 0, 0xF8, 0xF8, 120, 0x90, 60, 100, 0, 0xFC})
	if err != nil {
		t.Fatal(err)
	}
	if len(e) != 5 || e[3].At != 600 || e[4].At != 600 {
		t.Fatalf("overflow 時間軸錯誤：%v", e)
	}
}

func TestTempoChangeAffectsPlayerAndDuration(t *testing.T) {
	s := &Song{Tempo: 120, BeatTicks: 240, Ticks: 480, Events: []Event{
		{At: 240, Status: SysEx, Data: []byte{0x7F, 0, 2, 0}},
		{At: 480, Status: 0xFC},
	}}
	if math.Abs(s.Duration()-.75) > 1e-9 {
		t.Fatalf("速度事件後長度=%f", s.Duration())
	}
	if !bytes.HasSuffix(s.MIDI(), []byte{0x81, 0x70, 0xFF, 0x2F, 0}) {
		t.Fatal("MIDI 匯出遺失最後事件至停止的 240 tick")
	}
	p := NewPlayer(s, &Bank{Instruments: []Instrument{{}}})
	for i := 0; i < int(.75*OPLRate); i++ {
		p.Sample()
	}
	if p.Done() {
		t.Fatal("停止事件提早")
	}
	p.Sample()
	if !p.Done() {
		t.Fatal("停止事件延遲")
	}
	if p.tick != 480 {
		t.Fatalf("最後刻度=%d", p.tick)
	}
}
