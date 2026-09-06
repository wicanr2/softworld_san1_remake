package music

import "math"

// OPL2（YM3812）合成。
//
// 這一份不是週期精確的模擬器，而是把 OPL2 的**暫存器介面**照實做出來：
// `0x20/0x40/0x60/0x80/0xA0/0xB0/0xC0/0xE0` 與節奏模式的 `0xBD`
// 意義與硬體相同，所以把原版擷取到的埠寫入序列直接餵進來就會出聲。
// 內部的包絡與相位用浮點算，日後換成整數模型不必動介面。
//
// 幾個數字照硬體：取樣率是主頻 14.31818 MHz 除以 288；包絡是九位元的
// 衰減量，一階 0.1875 dB；相位表 1024 點，四種波形；調變深度是滿刻度
// 兩個週期。

// OPLRate 是 OPL2 的原生取樣率。
const OPLRate = 49716

// 包絡的階段。
const (
	egOff = iota
	egAttack
	egDecay
	egSustain
	egRelease
)

// egMax 是包絡的最大衰減（九位元）。
const egMax = 511

// mulTable 是倍率暫存器的十六個值。0 代表**半倍**，不是零。
var mulTable = [16]float64{0.5, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 10, 12, 12, 15, 15}

// kslTable 是音階衰減表，索引是 F-number 的高四位元。
var kslTable = [16]int{0, 32, 40, 45, 48, 51, 53, 55, 56, 58, 59, 60, 61, 62, 63, 64}

// kslShift 是四種音階衰減強度：關、1.5、3、6 dB／八度。
var kslShift = [4]uint{8, 4, 3, 2}

// slTable 是持續音量的十六階，一階 3 dB；最後一階是全靜音。
var slTable = [16]float64{0, 16, 32, 48, 64, 80, 96, 112,
	128, 144, 160, 176, 192, 208, 224, egMax}

var (
	// waveTable 是四種波形的 1024 點取樣。
	waveTable [4][1024]float64
	// attTable 把衰減量（單位 0.1875 dB）換成振幅。
	attTable [1024]float64
	// egIncTable 是每個取樣的包絡增量，索引是 0..63 的速率。
	egIncTable [64]float64
)

func init() {
	for i := 0; i < 1024; i++ {
		s := math.Sin(2 * math.Pi * (float64(i) + 0.5) / 1024)
		waveTable[0][i] = s
		if s > 0 {
			waveTable[1][i] = s
		}
		waveTable[2][i] = math.Abs(s)
		if i&0x100 == 0 {
			waveTable[3][i] = math.Abs(s)
		}
		attTable[i] = math.Pow(10, -float64(i)*0.1875/20)
	}
	attTable[1023] = 0
	// 速率每加四就快一倍；同一組裡的四階是硬體用不同的計數樣式做出來的
	// 平均增量。速率 60 以上飽和。
	base := [4]float64{0.5, 0.625, 0.75, 0.875}
	for r := 1; r < 64; r++ {
		if r>>2 >= 15 {
			egIncTable[r] = 4
			continue
		}
		egIncTable[r] = base[r&3] * math.Exp2(float64(r>>2)-12)
	}
}

// oplOp 是一個運算子。
type oplOp struct {
	am, vib, egType, ksr bool
	mult, ksl, tl        int
	ar, dr, sl, rr       int
	wave                 int

	phase      float64 // 相位（週期數）
	env        float64 // 衰減，0（最大聲）..511（靜音）
	state      int
	out, prev  float64 // 回授要前兩次的輸出
	kslAtten   int
	rateOffset int
}

// oplCh 是一個頻道。
type oplCh struct {
	fnum, block int
	fb, cnt     int
	on          bool
}

// OPL2 是一顆 YM3812。
type OPL2 struct {
	reg [256]byte
	op  [18]oplOp
	ch  [9]oplCh

	rhythm            bool
	rhythmOn          [5]bool // BD SD TOM TC HH
	tremDeep, vibDeep bool
	waveSel           bool

	tremPhase, vibPhase float64
	noise               uint32

	// Trace 每寫一個暫存器就被呼叫一次。給對拍用：原版擷取到的寫入
	// 序列與這裡送出去的可以逐項比。
	Trace func(reg, val byte)
}

// NewOPL2 造一顆重設過的晶片。
func NewOPL2() *OPL2 {
	c := &OPL2{noise: 1}
	for i := range c.op {
		c.op[i].state = egOff
		c.op[i].env = egMax
	}
	return c
}

// chOffset 是九個頻道的第一個運算子在暫存器裡的位移。
var chOffset = [9]int{0, 1, 2, 8, 9, 10, 16, 17, 18}

// opIndex 把暫存器位移換成運算子編號；不是運算子就回 −1。
func opIndex(off int) int {
	switch {
	case off < 6:
		return off
	case off >= 8 && off < 14:
		return off - 2
	case off >= 16 && off < 22:
		return off - 4
	}
	return -1
}

// rhythmBit 是節奏模式五件打擊樂器在 `0xBD` 的位元：
// 低音鼓、小鼓、通鼓、鈸、腳踏鈸。
var rhythmBit = [5]byte{0x10, 0x08, 0x04, 0x02, 0x01}

// 節奏模式的五件打擊樂器各用哪個運算子。低音鼓用第七頻道的兩個。
const (
	opHH  = 13 // 第八頻道的調變器
	opSD  = 16 // 第八頻道的載波
	opTOM = 14 // 第九頻道的調變器
	opTC  = 17 // 第九頻道的載波
)

// Write 寫一個暫存器。位址與值就是原版送到 0x388／0x389 的那兩個位元組。
//
// 位址的分組不是齊整的：運算子那五組（`0x20/0x40/0x60/0x80/0xE0`）
// 用低五位元選運算子，頻道那三組（`0xA0/0xB0/0xC0`）用低四位元選頻道，
// 而 `0xA0` 與 `0xB0` **落在同一個 0xE0 遮罩裡**——拿 `reg & 0xE0`
// 分組會把音高與 key-on 混成一組，拿 `reg & 0x1F` 取頻道會把 `0xB0`
// 的第零頻道算成第十六個。
func (c *OPL2) Write(reg, val byte) {
	if c.Trace != nil {
		c.Trace(reg, val)
	}
	c.reg[reg] = val
	switch {
	case reg < 0x20:
		switch reg {
		case 0x01:
			c.waveSel = val&0x20 != 0
		case 0x08:
			// NTS：音階速率取 F-number 的哪一位元。這裡固定用第九位元。
		}
	case reg < 0xA0:
		i := opIndex(int(reg & 0x1F))
		if i < 0 {
			return
		}
		o := &c.op[i]
		switch reg & 0xE0 {
		case 0x20:
			o.am, o.vib = val&0x80 != 0, val&0x40 != 0
			o.egType, o.ksr = val&0x20 != 0, val&0x10 != 0
			o.mult = int(val & 15)
			c.refresh()
		case 0x40:
			o.ksl, o.tl = int(val>>6), int(val&63)
			c.refresh()
		case 0x60:
			o.ar, o.dr = int(val>>4), int(val&15)
		case 0x80:
			o.sl, o.rr = int(val>>4), int(val&15)
		}
	case reg == 0xBD:
		c.tremDeep, c.vibDeep = val&0x80 != 0, val&0x40 != 0
		c.rhythm = val&0x20 != 0
		for i, bit := range rhythmBit {
			on := c.rhythm && val&bit != 0
			if on != c.rhythmOn[i] {
				c.rhythmOn[i] = on
				c.keyRhythm(i, on)
			}
		}
	case reg < 0xC0:
		ch := int(reg & 0x0F)
		if ch >= 9 {
			return
		}
		x := &c.ch[ch]
		if reg < 0xB0 {
			x.fnum = x.fnum&0x300 | int(val)
		} else {
			x.fnum = x.fnum&0xFF | int(val&3)<<8
			x.block = int(val >> 2 & 7)
			if on := val&0x20 != 0; on != x.on {
				x.on = on
				c.keyChannel(ch, on)
			}
		}
		c.refresh()
	case reg < 0xE0:
		ch := int(reg & 0x0F)
		if ch >= 9 {
			return
		}
		c.ch[ch].fb = int(val >> 1 & 7)
		c.ch[ch].cnt = int(val & 1)
	default:
		if i := opIndex(int(reg & 0x1F)); i >= 0 {
			c.op[i].wave = int(val & 3)
			if !c.waveSel {
				c.op[i].wave = 0
			}
		}
	}
}

// keyChannel 對一個旋律頻道的兩個運算子下 key-on／key-off。
func (c *OPL2) keyChannel(ch int, on bool) {
	if c.rhythm && ch >= 6 {
		return // 節奏模式下第七到九頻道由 0xBD 管
	}
	c.key(opIndex(chOffset[ch]), on)
	c.key(opIndex(chOffset[ch]+3), on)
}

// keyRhythm 對一件打擊樂器下 key-on／key-off。
func (c *OPL2) keyRhythm(which int, on bool) {
	switch which {
	case 0: // 低音鼓用整個第七頻道
		c.key(opIndex(chOffset[6]), on)
		c.key(opIndex(chOffset[6]+3), on)
	case 1:
		c.key(opSD, on)
	case 2:
		c.key(opTOM, on)
	case 3:
		c.key(opTC, on)
	case 4:
		c.key(opHH, on)
	}
}

func (c *OPL2) key(i int, on bool) {
	if i < 0 {
		return
	}
	o := &c.op[i]
	if on {
		o.state = egAttack
		o.phase = 0
		o.out, o.prev = 0, 0
		return
	}
	if o.state != egOff {
		o.state = egRelease
	}
}

// refresh 重算與音高有關的兩件事：音階衰減與音階速率。
func (c *OPL2) refresh() {
	for ch := 0; ch < 9; ch++ {
		f, b := c.ch[ch].fnum, c.ch[ch].block
		ksl := kslTable[f>>6&15]<<2 - (8-b)<<5
		if ksl < 0 {
			ksl = 0
		}
		ksn := b<<1 | f>>9&1
		for _, off := range []int{chOffset[ch], chOffset[ch] + 3} {
			i := opIndex(off)
			if i < 0 {
				continue
			}
			o := &c.op[i]
			o.kslAtten = ksl >> kslShift[o.ksl]
			if o.ksr {
				o.rateOffset = ksn
			} else {
				o.rateOffset = ksn >> 2
			}
		}
	}
}

// rate 是某一項速率加上音階速率之後的值。R ＝ 0 表示不動。
func (o *oplOp) rate(r int) int {
	if r == 0 {
		return 0
	}
	v := 4*r + o.rateOffset
	if v > 63 {
		v = 63
	}
	return v
}

// advanceEnv 走一步包絡。
func (o *oplOp) advanceEnv() {
	switch o.state {
	case egAttack:
		r := o.rate(o.ar)
		if r == 0 {
			return
		}
		if r >= 60 {
			o.env, o.state = 0, egDecay
			return
		}
		o.env -= (o.env + 1) * egIncTable[r] / 8
		if o.env <= 0 {
			o.env, o.state = 0, egDecay
		}
	case egDecay:
		o.env += egIncTable[o.rate(o.dr)]
		if sl := slTable[o.sl]; o.env >= sl {
			o.env, o.state = sl, egSustain
		}
	case egSustain:
		if o.egType {
			return // 持續型：按著不放就停在這裡
		}
		fallthrough
	case egRelease:
		o.env += egIncTable[o.rate(o.rr)]
		if o.env >= egMax {
			o.env, o.state = egMax, egOff
		}
	}
}

// atten 是這一刻的總衰減：包絡 ＋ 總音量 ＋ 音階衰減 ＋ 振幅調變。
func (c *OPL2) atten(o *oplOp) int {
	a := int(o.env) + o.tl*4 + o.kslAtten
	if o.am {
		d := 5.6 // 1.0 dB，深度位元打開是 4.8 dB
		if c.tremDeep {
			d = 25.6
		}
		a += int(d * 0.5 * (1 - math.Cos(2*math.Pi*c.tremPhase)))
	}
	if a > 1023 {
		a = 1023
	}
	return a
}

// step 讓一個運算子走一步，回傳它的輸出（−1..1）。mod 是加進相位的
// 週期數（調變或回授）。
func (c *OPL2) step(o *oplOp, ch *oplCh, mod float64) float64 {
	inc := float64(ch.fnum) * math.Exp2(float64(ch.block)) * mulTable[o.mult] / (1 << 20)
	if o.vib {
		cents := 7.0
		if c.vibDeep {
			cents = 14.0
		}
		inc *= math.Exp2(cents * math.Sin(2*math.Pi*c.vibPhase) / 1200)
	}
	o.phase += inc
	o.advanceEnv()
	if o.state == egOff {
		return 0
	}
	i := int((o.phase+mod)*1024) & 1023
	return waveTable[o.wave][i] * attTable[c.atten(o)]
}

// stepFixed 讓一個運算子走一步，但相位由外面給（節奏模式的雜音聲部）。
func (c *OPL2) stepFixed(o *oplOp, ch *oplCh, phase int) float64 {
	inc := float64(ch.fnum) * math.Exp2(float64(ch.block)) * mulTable[o.mult] / (1 << 20)
	o.phase += inc
	o.advanceEnv()
	if o.state == egOff {
		return 0
	}
	return waveTable[o.wave][phase&1023] * attTable[c.atten(o)]
}

// wave 取一個運算子未經衰減的波形值，只給節奏模式算相位位元用。
func (o *oplOp) phaseBits() int { return int(o.phase*1024) & 1023 }

// Sample 產生一個取樣。
//
// 回傳的是九個頻道相加的原始值，一個頻道的滿刻度是 ±1，所以理論上限
// 是 ±9；實際的曲子同時響的聲部有限，五首量到的峰值在 ±3.6 以內。
// 要換成整數取樣的人自己挑倍率並且夾住。
func (c *OPL2) Sample() float64 {
	// 顫音 3.7 Hz、抖音 6.1 Hz，都是硬體的固定速度。
	c.tremPhase += 3.7 / OPLRate
	c.vibPhase += 6.1 / OPLRate
	if c.tremPhase >= 1 {
		c.tremPhase--
	}
	if c.vibPhase >= 1 {
		c.vibPhase--
	}
	if c.noise&1 != 0 {
		c.noise ^= 0x800302
	}
	c.noise >>= 1

	sum := 0.0
	last := 9
	if c.rhythm {
		last = 6
	}
	for i := 0; i < last; i++ {
		sum += c.melodic(i)
	}
	if c.rhythm {
		sum += c.melodic(6) // 低音鼓走一般的兩運算子路徑
		sum += c.percussion()
	}
	return sum
}

// melodic 算一個旋律頻道。
func (c *OPL2) melodic(ch int) float64 {
	x := &c.ch[ch]
	m := &c.op[opIndex(chOffset[ch])]
	car := &c.op[opIndex(chOffset[ch]+3)]

	fb := 0.0
	if x.fb > 0 {
		fb = (m.out + m.prev) * math.Exp2(float64(x.fb)-7)
	}
	mod := c.step(m, x, fb)
	m.prev, m.out = m.out, mod
	if x.cnt == 1 {
		// 相加：兩個運算子各自發聲。
		return mod + c.step(car, x, 0)
	}
	// 串接：調變器改載波的相位，滿刻度是兩個週期。
	return c.step(car, x, mod*2)
}

// percussion 算節奏模式的四件單運算子打擊樂器。
//
// 小鼓與鈸走的相位不是自己累積的，而是由第八、九頻道的相位位元與一個
// 雜音暫存器湊出來——那正是 OPL2 讓它們聽起來像噪音的辦法。
func (c *OPL2) percussion() float64 {
	hh, sd := &c.op[opHH], &c.op[opSD]
	tom, tc := &c.op[opTOM], &c.op[opTC]
	ch7, ch8 := &c.ch[7], &c.ch[8]

	h, t := hh.phaseBits(), tc.phaseBits()
	xor := (h>>2^h>>7)&1 | (h>>3^t>>5)&1 | (h>>7^t>>3)&1
	noise := int(c.noise & 1)

	hp := xor << 9
	if xor^noise != 0 {
		hp |= 0xD0
	} else {
		hp |= 0x34
	}
	sp := (0x100 << (h >> 8 & 1)) ^ (noise << 8)
	tp := 0x100 | xor<<9

	return c.stepFixed(hh, ch7, hp) + c.stepFixed(sd, ch7, sp) +
		c.step(tom, ch8, 0) + c.stepFixed(tc, ch8, tp)
}

// Silent 說所有運算子的包絡是不是都收乾淨了。
func (c *OPL2) Silent() bool {
	for i := range c.op {
		if c.op[i].state != egOff {
			return false
		}
	}
	return true
}
