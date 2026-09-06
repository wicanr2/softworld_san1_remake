package music

import (
	"encoding/binary"
	"io"
	"math"
)

// 把曲子播成聲音。
//
// 事件流是 MIDI 的形狀，聲部的分配卻是 AdLib 的：**一個 MIDI 頻道就是
// 一個聲部**，程式變更選的是那首曲子自己的音色庫索引，音高由 AdLib 的
// 音符表換成 F-number 與 block。
//
// 聲部怎麼對到晶片，看曲子是不是節奏模式（`Song.Percussive`）：
//
//	旋律模式  聲部 0..8  → 第一到九頻道
//	節奏模式  聲部 0..5  → 第一到六頻道
//	          聲部 6..10 → 低音鼓、小鼓、通鼓、鈸、腳踏鈸
//
// 五首曲子的用法與這張表吻合：旋律模式的兩首用到第七至九個聲部，
// 節奏模式的三首旋律聲部都不超過六個，而多出來的聲部掛的音色正好是
// `snare1`（小鼓）、`tom1`（通鼓）、`hihat3`（腳踏鈸）。

// noteFnum 是原版的音符表：一個八度十二個 F-number。
//
// 這不是等律算出來的，是從原版自己的埠寫入量出來的。`思古` 播放期間
// 沒有滑音的六個聲部一共蓋到七個半音，把它們與 block 對齊之後得到
// 索引 1、3、4、6、8、10、11 的值；剩下五個用等比補進去（註記在後面）。
//
// 量到的七個都比等律略高，而且高得不平均（最多約七音分），所以
// **不能拿公式代替**——照等律算會讓每個音都差一兩個 F-number。
var noteFnum = [12]int{
	0x158, // 補
	0x16C, // 量
	0x181, // 補
	0x198, // 量
	0x1B1, // 量
	0x1CB, // 補
	0x1E6, // 量
	0x203, // 補
	0x222, // 量
	0x243, // 補
	0x266, // 量
	0x28A, // 量
}

// bendCents 是滑音走滿一整格是幾音分。
//
// AdLib 的 `.ROL` 用一個 0..2 的倍率表示滑音，1.0 是不動；`思古` 裡
// 有四個聲部的值是 0.85。原版把那四個聲部彈得比不滑音的低約 11 音分
//（0x222 對 0x21F、0x266 對 0x262、0x1E6 對 0x1E3），倒推得到這個
// 數字：七組量測裡六組完全相符，一組差一個 F-number。
//
// ⚠ **原版的算法還沒解出來。** 它的差值不是嚴格的等比——0x198 差 2
// 而更低的 0x16C 差 3——表示中間有整數運算。差的一個 F-number 是
// 三音分，聽不出來，但要對到位元組就得再挖驅動程式。
const bendCents = 69.5

// voice 是一個聲部的狀態。
type voice struct {
	inst  int // 音色庫索引，−1 是還沒選
	bend  float64
	note  int
	on    bool
	block int
	fnum  int
}

// Render 把一首曲子合成成單聲道取樣，取樣率是 OPLRate。
//
// 沒有音色庫（bank 是 nil）就沒有東西可發聲，回傳空的。
func Render(s *Song, b *Bank) []int16 {
	c := NewOPL2()
	var out []int16
	if !drive(c, s, b, func(n int) { out = render(c, out, n) }) {
		return nil
	}
	// 尾巴：讓還在響的音收乾淨，最多三秒。
	for i := 0; i < 3 && !c.Silent(); i++ {
		out = render(c, out, OPLRate)
	}
	return out
}

// drive 把一首曲子的事件依序送進晶片。兩個事件之間呼叫 gap，
// 交出中間要產生幾個取樣；只想看暫存器寫入的話 gap 什麼都不用做。
//
// 沒有音色庫就沒有東西可發聲，回傳 false。
func drive(c *OPL2, s *Song, b *Bank, gap func(n int)) bool {
	if s == nil || b == nil || len(b.Instruments) == 0 || s.Tempo <= 0 {
		return false
	}
	c.Write(0x01, 0x20) // 打開波形選擇，OPL2 才有四種波形
	c.Write(0x08, 0x00)
	rhythmReg := byte(0)
	if s.Percussive {
		rhythmReg = 0x20
		c.Write(0xBD, rhythmReg)
	}
	var vs [11]voice
	for i := range vs {
		vs[i].inst = -1
	}
	perTick := float64(OPLRate) * 60 / (float64(s.Tempo) * TicksPerBeat)
	pos := 0.0
	for _, e := range s.Events {
		if n := int(float64(e.At)*perTick - pos); n > 0 {
			gap(n)
			pos += float64(n)
		}
		v := e.Channel()
		if v < 0 || v >= len(vs) {
			continue
		}
		apply(c, &vs[v], v, e, b, s.Percussive, &rhythmReg)
	}
	return true
}

// renderGain 是合成值換成 16 位元取樣的倍率。
//
// 五首曲子的峰值落在 ±3.6，這個倍率把最吵的一首推到滿刻度的七成，
// 留給沒量過的曲子（`MUSV` 那一首長的）足夠的餘裕。
const renderGain = 6500

// render 產生 n 個取樣接到後面。
func render(c *OPL2, out []int16, n int) []int16 {
	for i := 0; i < n; i++ {
		v := c.Sample() * renderGain
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		out = append(out, int16(v))
	}
	return out
}

// apply 把一個事件送進晶片。
func apply(c *OPL2, v *voice, idx int, e Event, b *Bank, perc bool, rhythmReg *byte) {
	switch e.Kind() {
	case ProgramChange:
		if len(e.Data) == 0 {
			return
		}
		v.inst = int(e.Data[0])
		program(c, idx, v.inst, b, perc)
	case PitchBend:
		if len(e.Data) < 2 {
			return
		}
		raw := int(e.Data[0]) | int(e.Data[1])<<7
		v.bend = float64(raw-0x2000) / 8192 * bendCents / 100
		if v.on {
			keyOn(c, v, idx, perc, rhythmReg, true)
		}
	case NoteOff:
		if len(e.Data) > 0 && int(e.Data[0]) != v.note && v.on {
			return // 放掉的不是正在響的那個音
		}
		keyOff(c, v, idx, perc, rhythmReg)
	case NoteOn:
		if len(e.Data) < 2 {
			return
		}
		if e.Data[1] == 0 {
			if int(e.Data[0]) != v.note && v.on {
				return
			}
			keyOff(c, v, idx, perc, rhythmReg)
			return
		}
		v.note = int(e.Data[0])
		volume(c, idx, v.inst, b, perc, int(e.Data[1]))
		keyOn(c, v, idx, perc, rhythmReg, false)
	}
}

// oplChannelOf 是一個聲部用哪個頻道；節奏模式的打擊樂器共用第七到九頻道。
func oplChannelOf(idx int, perc bool) int {
	if !perc {
		return idx
	}
	switch {
	case idx < 6:
		return idx
	case idx == 6: // 低音鼓
		return 6
	case idx <= 8: // 小鼓、通鼓
		return 7
	}
	return 8 // 鈸、腳踏鈸
}

// 節奏模式裡，小鼓與腳踏鈸掛在第八頻道、通鼓與鈸掛在第九頻道，
// 而且各自只用一個運算子。
func rhythmSlot(idx int) (op int, ok bool) {
	switch idx {
	case 7:
		return opSD, true
	case 8:
		return opTOM, true
	case 9:
		return opTC, true
	case 10:
		return opHH, true
	}
	return 0, false
}

// program 把一件音色填進聲部對應的運算子。
func program(c *OPL2, idx, inst int, b *Bank, perc bool) {
	if inst < 0 || inst >= len(b.Instruments) {
		return
	}
	in := b.Instruments[inst]
	if op, ok := rhythmSlot(idx); ok && perc {
		writeOp(c, opRegOffset(op), in.Modulator())
		return
	}
	ch := oplChannelOf(idx, perc)
	writeOp(c, chOffset[ch], in.Modulator())
	writeOp(c, chOffset[ch]+3, in.Carrier())
	c.Write(byte(0xC0+ch), in.Connection())
}

// opRegOffset 是運算子編號對應的暫存器位移，與 opIndex 相反。
func opRegOffset(op int) int {
	switch {
	case op < 6:
		return op
	case op < 12:
		return op + 2
	default:
		return op + 4
	}
}

// writeOp 依原版的順序寫一個運算子的五個暫存器。
func writeOp(c *OPL2, off int, o Operator) {
	r20, r40, r60, r80, rE0 := o.Registers()
	c.Write(byte(0x20+off), r20)
	c.Write(byte(0x40+off), r40)
	c.Write(byte(0x60+off), r60)
	c.Write(byte(0x80+off), r80)
	c.Write(byte(0xE0+off), rE0)
}

// volume 把力度換成額外的衰減，加在發聲的那個運算子上。
//
// 力度 127 不加衰減；最輕的一級再多 24 dB。串接的時候只有載波在發聲，
// 改調變器只會改音色不會改音量。
func volume(c *OPL2, idx, inst int, b *Bank, perc bool, vel int) {
	if inst < 0 || inst >= len(b.Instruments) {
		return
	}
	in := b.Instruments[inst]
	add := (127 - vel) * 32 / 127
	if op, ok := rhythmSlot(idx); ok && perc {
		setTL(c, opRegOffset(op), in.Modulator(), add)
		return
	}
	ch := oplChannelOf(idx, perc)
	setTL(c, chOffset[ch]+3, in.Carrier(), add)
	if in.Connection()&1 == 1 {
		setTL(c, chOffset[ch], in.Modulator(), add)
	}
}

func setTL(c *OPL2, off int, o Operator, add int) {
	tl := o.TL + add
	if tl > 63 {
		tl = 63
	}
	c.Write(byte(0x40+off), byte(o.KSL&3<<6|tl&63))
}

// keyOn 送音高並讓聲部發聲。retune 為真時只改音高，不重新觸發。
func keyOn(c *OPL2, v *voice, idx int, perc bool, rhythmReg *byte, retune bool) {
	// 音高：八度選 block，半音查表，滑音再乘上去。**滑音乘在查表的值上**，
	// 不是先併進音高再查表——那樣會跨到隔壁半音，用錯一格的基準。
	oct := v.note/12 - 1
	if oct < 0 {
		oct = 0
	} else if oct > 7 {
		oct = 7
	}
	f := float64(noteFnum[v.note%12]) * math.Exp2(v.bend/12)
	fnum := int(f + 0.5)
	if fnum > 0x3FF {
		fnum = 0x3FF
	}
	v.block, v.fnum = oct, fnum
	ch := oplChannelOf(idx, perc)
	c.Write(byte(0xA0+ch), byte(fnum&0xFF))

	if _, ok := rhythmSlot(idx); ok && perc {
		c.Write(byte(0xB0+ch), byte(fnum>>8&3|oct<<2))
		if !retune {
			bit := rhythmBit[idx-6]
			*rhythmReg &^= bit
			c.Write(0xBD, *rhythmReg)
			*rhythmReg |= bit
			c.Write(0xBD, *rhythmReg)
		}
		v.on = true
		return
	}
	if perc && idx == 6 { // 低音鼓
		c.Write(byte(0xB0+ch), byte(fnum>>8&3|oct<<2))
		if !retune {
			*rhythmReg &^= rhythmBit[0]
			c.Write(0xBD, *rhythmReg)
			*rhythmReg |= rhythmBit[0]
			c.Write(0xBD, *rhythmReg)
		}
		v.on = true
		return
	}
	regB := byte(fnum>>8&3 | oct<<2)
	if retune {
		if v.on {
			regB |= 0x20
		}
		c.Write(byte(0xB0+ch), regB)
		return
	}
	// 重新觸發：先放掉再按下，包絡才會從頭走。
	c.Write(byte(0xB0+ch), regB)
	c.Write(byte(0xB0+ch), regB|0x20)
	v.on = true
}

// keyOff 讓聲部停聲。
func keyOff(c *OPL2, v *voice, idx int, perc bool, rhythmReg *byte) {
	if !v.on {
		return
	}
	v.on = false
	ch := oplChannelOf(idx, perc)
	if perc && idx >= 6 {
		bit := rhythmBit[idx-6]
		*rhythmReg &^= bit
		c.Write(0xBD, *rhythmReg)
		return
	}
	c.Write(byte(0xB0+ch), byte(v.fnum>>8&3|v.block<<2))
}

// WriteWAV 把取樣寫成單聲道 16 位元的 WAV。
func WriteWAV(w io.Writer, pcm []int16, rate int) error {
	n := len(pcm) * 2
	hdr := make([]byte, 44)
	copy(hdr[0:], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(36+n))
	copy(hdr[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(hdr[16:], 16)
	binary.LittleEndian.PutUint16(hdr[20:], 1) // PCM
	binary.LittleEndian.PutUint16(hdr[22:], 1) // 單聲道
	binary.LittleEndian.PutUint32(hdr[24:], uint32(rate))
	binary.LittleEndian.PutUint32(hdr[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(hdr[32:], 2)
	binary.LittleEndian.PutUint16(hdr[34:], 16)
	copy(hdr[36:], "data")
	binary.LittleEndian.PutUint32(hdr[40:], uint32(n))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	buf := make([]byte, n)
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
	}
	_, err := w.Write(buf)
	return err
}
