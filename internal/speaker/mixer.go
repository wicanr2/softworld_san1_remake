package speaker

import (
	"encoding/binary"
	"sync"
)

// Mixer 是音效與語音的音訊來源。
//
// 它是 `io.Reader`：要多少取樣就給多少，沒東西播的時候給靜音，
// **永遠不會結束**。音訊裝置那一側因此只要開一個永久的播放器。
//
// 原版是播完才回（`docs/re/09` §5），remake 非同步播——畫面不等它。
// 這是記在 `docs/spec/008` R5 的 remake 差異。
type Mixer struct {
	mu   sync.Mutex
	rate float64
	bank *Bank

	// queue 是已經算好的片段，接著播。一句話三段就是三個元素。
	queue [][]int16
	pos   int

	soundOff, voiceOff bool
}

// NewMixer 起一個混音器，rate 是輸出取樣率。
func NewMixer(bank *Bank, rate int) *Mixer {
	if bank == nil {
		bank = &Bank{}
	}
	return &Mixer{bank: bank, rate: float64(rate)}
}

// SetGates 設兩個開關。**參數是「關掉了嗎」**，與原版的存檔欄位同向
//（0 ＝ 開啟）。
//
// 關掉的時候把還沒播的丟掉：原版的開關是在 `speak()` 入口擋，
// 沒有「播到一半繼續」這回事。
func (m *Mixer) SetGates(soundOff, voiceOff bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.soundOff, m.voiceOff = soundOff, voiceOff
	if soundOff {
		m.queue, m.pos = nil, 0
	}
}

// Play 播一個槽。divisor 是分頻值（速度）。
//
// 門是**音效狀態**，四個槽都一樣——`speak()` 自己就是這樣擋的
//（`docs/re/09` §6.1）。槽 1–3 另外過語音狀態那道門。
func (m *Mixer) Play(slot, divisor int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueue(slot, divisor)
}

// Say 把三段接起來當一句話播（`docs/re/09` §6.2）。
func (m *Mixer) Say(divisor int, slots ...int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range slots {
		m.enqueue(s, divisor)
	}
}

// enqueue 要在持鎖時呼叫。
func (m *Mixer) enqueue(slot, divisor int) {
	if m.soundOff {
		return
	}
	if slot >= VoiceLo && slot <= VoiceHi && m.voiceOff {
		return
	}
	s := Render(m.bank.Clip(slot), Rate(divisor), m.rate)
	if len(s) == 0 {
		return
	}
	m.queue = append(m.queue, s)
}

// Busy 回「還有東西沒播完」。
func (m *Mixer) Busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue) > 0
}

// Read 給 16 位元小端立體聲。
//
// **永遠回 nil 錯誤**：這是一個無盡的來源，回 io.EOF 會讓播放器停掉，
// 之後再有聲音也播不出來。
func (m *Mixer) Read(p []byte) (int, error) {
	// 一格是左右各兩個位元組。半格的要求先切掉，下一次再補。
	n := len(p) / 4 * 4
	if n == 0 {
		return 0, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := 0; i < n; i += 4 {
		v := int16(0)
		for len(m.queue) > 0 {
			cur := m.queue[0]
			if m.pos < len(cur) {
				v = cur[m.pos]
				m.pos++
				break
			}
			m.queue = m.queue[1:]
			m.pos = 0
		}
		u := uint16(v)
		binary.LittleEndian.PutUint16(p[i:], u)
		binary.LittleEndian.PutUint16(p[i+2:], u)
	}
	return n, nil
}
