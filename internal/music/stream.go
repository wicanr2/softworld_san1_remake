package music

import (
	"encoding/binary"
	"io"
	"sync"
)

// 邊播邊合成。
//
// 一首曲子算完要幾秒 CPU，整包五首要一分鐘——開遊戲不能等這個。
// `Stream` 是 `io.Reader`，要多少取樣就合多少，交給音訊裝置直接播。
//
// 輸出是 16 位元小端立體聲，取樣率由呼叫端指定；OPL2 本身固定在
// 49,716 Hz，中間用線性內插換過去。

// Stream 是一首曲子的音訊來源。播完自動從頭開始，永遠不會結束。
//
// 一個 Stream 同時只能給一個播放器讀。
type Stream struct {
	mu     sync.Mutex
	p      *Player
	rate   int
	ratio  float64 // 每個輸出取樣要走幾個 OPL 取樣
	frac   float64
	cur    float64
	nxt    float64
	silent bool

	pending []byte // 上一次沒讀完的位元組
}

// NewStream 起一個音訊來源，rate 是輸出的取樣率。
func NewStream(s *Song, b *Bank, rate int) *Stream {
	p := NewPlayer(s, b)
	if p == nil || rate <= 0 {
		return nil
	}
	st := &Stream{p: p, rate: rate, ratio: float64(OPLRate) / float64(rate)}
	st.cur, st.nxt = p.Sample(), p.Sample()
	return st
}

// SetSilent 暫時靜音。曲子照走，只是不出聲——回來的時候接得上原本的位置。
func (s *Stream) SetSilent(v bool) {
	s.mu.Lock()
	s.silent = v
	s.mu.Unlock()
}

// Read 填出 16 位元小端立體聲。
func (s *Stream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := 0
	if len(s.pending) > 0 {
		n = copy(p, s.pending)
		s.pending = s.pending[n:]
	}
	var frame [4]byte
	for n < len(p) {
		v := int16(0)
		if !s.silent {
			v = clip((s.cur + (s.nxt-s.cur)*s.frac) * renderGain)
		}
		binary.LittleEndian.PutUint16(frame[0:], uint16(v))
		binary.LittleEndian.PutUint16(frame[2:], uint16(v))
		k := copy(p[n:], frame[:])
		n += k
		if k < 4 {
			s.pending = append(s.pending[:0], frame[k:]...)
		}
		s.advance()
	}
	return n, nil
}

// advance 把內插位置往前推一個輸出取樣。
func (s *Stream) advance() {
	s.frac += s.ratio
	for s.frac >= 1 {
		s.frac--
		s.cur = s.nxt
		if s.p.Done() && s.p.Chip().Silent() {
			s.p.Rewind()
		}
		s.nxt = s.p.Sample()
	}
}

// Seek 讓 Stream 也算 io.ReadSeeker。它是無盡的循環，只認「回到開頭」。
func (s *Stream) Seek(offset int64, whence int) (int64, error) {
	if offset == 0 && whence == io.SeekStart {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.p.Rewind()
		s.frac, s.pending = 0, nil
		s.cur, s.nxt = s.p.Sample(), s.p.Sample()
		return 0, nil
	}
	return 0, io.ErrUnexpectedEOF
}
