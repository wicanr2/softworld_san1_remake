//go:build oracle

package parity

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// 畫肖像的共用常式（`F%03d.FAC`，十格快取；`docs/spec/005` §9、Issue #50）。
const facePaintFn = 0x0f7b4

var faceCensusOnce sync.Map

// faceCensus 在 `SAN1_FACES=<目錄>` 時掛住畫肖像常式：每一個呼叫端第一次
// 命中時存一張畫面（畫**之前**的那一刻，肖像還沒貼上去）並在 `faces.log`
// 記呼叫端與五個參數 `(x, y, 肖像號, ?, 翻面)`。哪一支測試開機都掛，
// 所以拿玩家命令、戰役、單挑那幾條路各跑一次就能盤點誰畫了肖像。
func faceCensus(o *oracle.Oracle) {
	dir := os.Getenv("SAN1_FACES")
	if dir == "" {
		return
	}
	_ = os.MkdirAll(dir, 0o755)
	o.OnCall(addr(facePaintFn), func(o *oracle.Oracle) {
		caller := o.Caller().Linear()
		line := fmt.Sprintf("caller %05x x %d y %d face %d a4 %d flip %d\n", caller,
			int16(o.Arg(0)), int16(o.Arg(1)), int16(o.Arg(2)), int16(o.Arg(3)), int16(o.Arg(4)))
		if f, err := os.OpenFile(filepath.Join(dir, "faces.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			f.WriteString(line)
			f.Close()
		}
		if _, seen := faceCensusOnce.LoadOrStore(caller, true); seen {
			return
		}
		// 畫完再拍：在返回位址上掛一次性的 hook。
		ret := o.Caller()
		var once sync.Once
		o.OnCall(ret, func(o *oracle.Oracle) {
			once.Do(func() {
				pix := o.IndexedEGASize(scrW, scrH)
				if len(pix) < scrW*scrH {
					return
				}
				img := image.NewRGBA(image.Rect(0, 0, scrW, scrH))
				for y := 0; y < scrH; y++ {
					for x := 0; x < scrW; x++ {
						img.Set(x, y, assets.EGAPalette[pix[y*scrW+x]&15])
					}
				}
				if f, err := os.Create(filepath.Join(dir, fmt.Sprintf("face-%05x.png", caller))); err == nil {
					png.Encode(f, img)
					f.Close()
				}
			})
		})
	})
}
