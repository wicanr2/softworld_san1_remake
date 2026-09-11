//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 月內迴圈裡那支帶聲音的動畫：`lcall 03EB:0000`（線性 `0x3eb0`）。
//
// `docs/re/08` §2 把它記成「畫面或音樂，remake 用不到」，只因為當時
// 只看得到它被 `RND(12) < 月份` 選中。現在知道它**每一格都送一次
// PC 喇叭的音效**（`speak(0, 10)`，兩段各 160 次），所以它是動畫不是
// 雜訊——這一支把它畫出來的東西存成一連串 PNG。
//
// 做法：進入時開始計數，之後每 N 次 `speak()` 存一張。**存圖的節拍
// 掛在動畫自己的步進上**，不掛指令數——後者會隨執行器的速度漂移。
func TestZZMonthAnimationFrames(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	const animFn = 0x3eb0 // 03EB:0000
	var enters, steps int
	o.OnCall(addr(animFn), func(*oracle.Oracle) {
		enters++
		steps = 0
	})
	o.OnCall(addr(speechSpeakFn), func(o *oracle.Oracle) {
		if enters == 0 {
			return
		}
		steps++
		if steps%20 != 1 {
			return
		}
		dumpScreen(t, o, fmt.Sprintf("anim-%02d-%03d", enters, steps))
	})

	bootToMain(t, o, seedMas)
	work := o.ES()
	o.SetWord(oracle.Addr{Seg: work, Off: sfxStateOff}, 0)
	dumpScreen(t, o, "anim-00-before")

	for _, k := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(k)
		if err := o.Run(120_000_000); err != nil {
			t.Fatalf("送 %q 時停止：%v", k, err)
		}
	}
	dumpScreen(t, o, "anim-99-after")

	t.Logf("動畫進去 %d 次，最後一次走了 %d 步", enters, steps)
	if enters == 0 {
		t.Skip("這一輪的 RND(12) 沒有選中它——換 SAN1_SEED 再試")
	}
}
