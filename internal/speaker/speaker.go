// Package speaker 把原版用 PC 喇叭播的一位元取樣變成聲音。
//
// 規格是 `docs/spec/008`，RE 是 `docs/re/09`。三件事要記得：
//
//   - 波形沒有表頭：**檔案長度乘以 8 就是取樣數**，一個位元一個取樣，
//     最高位先送。
//   - 四個緩衝槽。**槽 0 是音效、槽 1–3 是語音**，共用同一支播放器，
//     所以關掉音效連語音都不出聲。
//   - 取樣率在原版是**跟 CPU 速度成正比**的（軟體延遲迴圈），
//     不是資料的屬性。這裡用一個基準機器的模型換算。
//
// ⚠ **本套件不含任何原版資料**，也不散布解出來的內容。它讀的是玩家
// 自己那一份。
package speaker

import (
	"fmt"
	"math"
)

// 四個緩衝槽（`docs/re/09` §6.1）。
const (
	SlotCount = 4 // 索引只能 0–3，原版超出範圍時印 "Invalid SND"
	SFXSlot   = 0 // 音效，開機從 S000.SND 載一次
	VoiceLo   = 1 // 語音三段
	VoiceHi   = 3
)

// 載入器的長度上限（線性 `0x5a76`，原版自己的訊息字串就是這兩個數字）。
const (
	MaxSFXBytes   = 1000 // "Real#0>1000"
	MaxVoiceBytes = 4200 // "Real#1-3>4200"，＝ 0x1068
)

// 取樣率模型（`docs/spec/008` R6，`L3`）。
//
// 原版走的入口用軟體延遲迴圈計時：
//
//	mov cx,分頻值 ; loop $ ; 送一個位元
//
// 所以一個位元的週期數 ≈ `loopCost × 分頻值 + bitFixed`，取樣率是
// CPU 頻率除以它。基準機器取說明書講的那一台（16 MHz 的 80286）。
//
// **這是估的**，真值取決於等待狀態與 BIOS。玩家的「語音速度」直接
// 餵進分頻值，所以仍然調得動。
const (
	RefCPUHz = 16_000_000
	loopCost = 8
	bitFixed = 45

	// SFXDivisor 是音效那一段實測到的分頻值（`L1`，`speak(0, 10)`）。
	SFXDivisor = 13
	// VoiceDivisor 是語音的暫用值。**還沒量到**——訊息常式傳的速度
	// 參數要先讓原版吐一則帶語音的訊息才看得到（`docs/spec/008` §6 R6）。
	VoiceDivisor = 130
)

// Rate 是分頻值對應的取樣率（Hz）。
func Rate(divisor int) float64 {
	if divisor < 10 {
		divisor = 10 // 原版自己夾的下限（`docs/re/09` §6.1）
	}
	return float64(RefCPUHz) / float64(loopCost*divisor+bitFixed)
}

// Clip 是一段一位元取樣。
//
// 零值是空的一段，播出來沒有聲音——這是刻意的：素材讀不到不該擋著
// 開遊戲。
type Clip struct {
	data []byte
}

// NewClip 收下一段波形。**不複製**，呼叫端不得再寫入那份位元組。
func NewClip(b []byte) Clip { return Clip{data: b} }

// Len 是位元組數。
func (c Clip) Len() int { return len(c.data) }

// Samples 是取樣數，＝ 位元組數 × 8。
func (c Clip) Samples() int { return len(c.data) * 8 }

// Bit 是第 i 個取樣。**最高位先送。**
func (c Clip) Bit(i int) bool {
	if i < 0 || i >= c.Samples() {
		return false
	}
	return c.data[i/8]>>(7-uint(i%8))&1 == 1
}

// Bank 是四個緩衝槽。
type Bank struct {
	slots [SlotCount]Clip
}

// Load 把一段波形放進槽。上限照原版（§2）：**超過就回錯誤，不截斷**
// ——安靜地截斷會讓一段語音少掉尾巴，而聽起來只是「怪怪的」。
func (b *Bank) Load(slot int, data []byte) error {
	if slot < 0 || slot >= SlotCount {
		return fmt.Errorf("speaker: 槽 %d 不在 0–%d", slot, SlotCount-1)
	}
	limit := MaxVoiceBytes
	if slot == SFXSlot {
		limit = MaxSFXBytes
	}
	if len(data) > limit {
		return fmt.Errorf("speaker: 槽 %d 收到 %d 個位元組，上限 %d",
			slot, len(data), limit)
	}
	b.slots[slot] = NewClip(data)
	return nil
}

// Clip 取一個槽。
func (b *Bank) Clip(slot int) Clip {
	if slot < 0 || slot >= SlotCount {
		return Clip{}
	}
	return b.slots[slot]
}

// VoiceName 是第 n 段語音在容器裡的名字。索引夾到 0–499（原版的
// `cmp ax,0x1F3`）。
func VoiceName(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 499 {
		n = 499
	}
	return fmt.Sprintf("R%03d.OKR", n)
}

// SFXName 是音效那一段在 `DATA1` 裡的名字。
const SFXName = "S000.SND"

// 渲染參數。兩個位準不取滿幅：一位元訊號滿幅在多數裝置上會削波，
// 聽起來像壞掉。低通是模擬喇叭紙盆——不過濾的話在 48 kHz 上是刺耳的
// 方波。**兩者都是 remake 差異**，不影響位元序列本身。
const (
	amplitude = 9000
	cutoffHz  = 4000
)

// Render 把一段波形算成 16 位元單聲道取樣。
//
// srcRate 是這一段在原版的取樣率（用 Rate 換算），dstRate 是輸出的。
func Render(c Clip, srcRate, dstRate float64) []int16 {
	n := c.Samples()
	if n == 0 || srcRate <= 0 || dstRate <= 0 {
		return nil
	}
	out := make([]int16, int(float64(n)*dstRate/srcRate))
	alpha := 1 - math.Exp(-2*math.Pi*cutoffHz/dstRate)
	y := 0.0
	stepPerOut := srcRate / dstRate
	for i := range out {
		src := int(float64(i) * stepPerOut)
		x := -float64(amplitude)
		if c.Bit(src) {
			x = amplitude
		}
		y += (x - y) * alpha
		out[i] = int16(y)
	}
	return out
}
