package music

import (
	"encoding/binary"
	"fmt"
)

// 匯出成標準 MIDI 檔。
//
// 原版的事件流已經是 MIDI 的形狀，只差**檔案外殼**（`MThd`／`MTrk`）與
// 速度事件。轉出來的檔案任何 MIDI 播放器都放得出來，所以它同時是
// 「解析對不對」的驗收方式：解錯的資料播出來不會是音樂。
//
// ⚠ **這不是原版的音色。** 原版走 AdLib（OPL2），音色參數在音色庫裡，
// 對應還沒解乾淨（`docs/formats/06` §4）。用一般 MIDI 音源播出來
// 音高與節奏是對的，音色不是。

// MIDI 檔的常數。
const (
	midiFormat0 = 0
	midiTracks  = 1
)

// MIDI 把一首曲子轉成標準 MIDI 檔的位元組。
func (s *Song) MIDI() []byte {
	var track []byte
	put := func(b ...byte) { track = append(track, b...) }
	putVar := func(v int) { track = append(track, varBytes(v)...) }

	// 速度：MIDI 用「每四分音符幾微秒」。
	usPerBeat := 500000
	if s.Tempo > 0 {
		usPerBeat = 60000000 / s.Tempo
	}
	putVar(0)
	put(0xFF, 0x51, 0x03,
		byte(usPerBeat>>16), byte(usPerBeat>>8), byte(usPerBeat))

	prev := 0
	for _, e := range s.Events {
		// 即時訊息（0xF8 時脈、0xFC 停止…）不能放進 MIDI 檔，跳過。
		// 它們在原版是給播放器用的，不影響音符。
		if e.Status >= 0xF8 || e.Status == 0xF6 || e.Status == 0xF7 {
			continue
		}
		putVar(e.At - prev)
		prev = e.At
		if e.Status == SysEx {
			put(0xF0)
			putVar(len(e.Data) + 1)
			put(e.Data...)
			put(0xF7)
			continue
		}
		put(e.Status)
		put(e.Data...)
	}
	putVar(0)
	put(0xFF, 0x2F, 0x00) // end of track

	out := make([]byte, 0, len(track)+22)
	out = append(out, 'M', 'T', 'h', 'd')
	out = appendU32(out, 6)
	out = appendU16(out, midiFormat0)
	out = appendU16(out, midiTracks)
	out = appendU16(out, TicksPerBeat)
	out = append(out, 'M', 'T', 'r', 'k')
	out = appendU32(out, uint32(len(track)))
	return append(out, track...)
}

// varBytes 把一個數值寫成 MIDI 的可變長度格式。
func varBytes(v int) []byte {
	if v < 0 {
		v = 0
	}
	buf := []byte{byte(v & 0x7F)}
	for v >>= 7; v > 0; v >>= 7 {
		buf = append([]byte{byte(v&0x7F | 0x80)}, buf...)
	}
	return buf
}

func appendU16(b []byte, v uint16) []byte {
	var t [2]byte
	binary.BigEndian.PutUint16(t[:], v)
	return append(b, t[:]...)
}

func appendU32(b []byte, v uint32) []byte {
	var t [4]byte
	binary.BigEndian.PutUint32(t[:], v)
	return append(b, t[:]...)
}

// Name 是一首曲子在原版主選單上的名字；超出五首就用編號。
func Name(i int) string {
	names := TrackNames()
	if i >= 0 && i < len(names) {
		return names[i]
	}
	return fmt.Sprintf("第%d首", i+1)
}
