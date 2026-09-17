//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// 場景圖的特效（`0x32dfa(x, y)`，Issue #58、`docs/spec/010`）：呼叫端先
// `0x36c9:0x2b0(名字, 0)` 把 `SCG##.IMG` 載進來，這一支畫在第二頁再由
// `0x32e40` 的四種拉幕搬到第一頁。四十八個呼叫端的名字與 (x, y) 列在
// `docs/spec/010` §1。
var (
	sceneLoadFn   = oracle.Addr{Seg: 0x36c9, Off: 0x2b0} // 載一張圖（名字 far*, 模式）
	sceneEffectFn = oracle.Addr{Seg: 0x3273, Off: 0x6ca} // ＝ `0x32dfa`：畫第二頁 ＋ 拉幕
)

// TestZZSceneEffectMatchesTheOriginal 對四種拉幕各載一張場景圖、直接呼叫
// `0x32dfa`，每一步（音效那一刻）抓一張畫面，與 remake 的 `NewSceneWipe`
// 逐步比：那一塊逐像素相同，塊外一個像素都沒動。
func TestZZSceneEffectMatchesTheOriginal(t *testing.T) {
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
	art, err := ui.NewArtScreen(openContainer(t, filepath.Join(root, "DATA3")),
		openContainer(t, filepath.Join(root, "DATA1")))
	if err != nil {
		t.Fatal(err)
	}
	face := loadFace(t)

	branch := uint32(0)
	var frames [][]uint8
	o.OnCall(addr(speechSpeakFn), func(o *oracle.Oracle) {
		frames = append(frames, append([]uint8(nil), o.IndexedEGASize(scrW, scrH)...))
	})
	bootToMain(t, o, seedMas)
	base := append([]uint8(nil), o.IndexedEGASize(scrW, scrH)...)
	o.Stub(addr(rndFn), func(*oracle.Oracle) uint32 { return branch })
	saved := o.Save()

	// 名字取原版自己 DS 裡的那幾份（呼叫端 `mov $off,%cx`）；(x, y) 用三種
	// 落點各試一次。
	for _, k := range []struct {
		scene   int
		nameOff uint16
		kind    ui.WipeKind
		x, y    int
	}{
		{17, 0x81b6, ui.WipeDown, 448, 268},
		{24, 0x75eb, ui.WipeUp, 432, 80},
		{19, 0x81ec, ui.WipeRight, 432, 120},
		{3, 0x8210, ui.WipeLeft, 432, 80},
	} {
		t.Run(fmt.Sprintf("SCG%02d-%d", k.scene, k.kind), func(t *testing.T) {
			o.Restore(saved)
			branch = uint32(k.kind)
			frames = nil
			if _, err := o.CallBudget(50_000_000, sceneLoadFn, k.nameOff, 0x427e, 0); err != nil {
				t.Fatalf("載圖：%v", err)
			}
			if _, err := o.CallBudget(200_000_000, sceneEffectFn, uint16(k.x), uint16(k.y)); err != nil {
				t.Fatalf("特效：%v", err)
			}
			end := append([]uint8(nil), o.IndexedEGASize(scrW, scrH)...)
			dumpScreen(t, o, fmt.Sprintf("scene-%02d-%d", k.scene, k.kind))

			cv := ui.NewCanvasPx(scrW, scrH, face)
			for y := 0; y < scrH; y++ {
				for x := 0; x < scrW; x++ {
					cv.Img.SetRGBA(x, y, assets.EGAPalette[base[y*scrW+x]&15])
				}
			}
			w := ui.NewSceneWipe(cv, art.Scene(k.scene), k.kind, k.x, k.y)
			if w == nil {
				t.Fatalf("remake 沒有 SCG%02d", k.scene)
			}
			if len(frames) != w.Steps() {
				t.Fatalf("原版 %d 步、remake %d 步", len(frames), w.Steps())
			}
			r := w.Rect
			for i, f := range frames {
				w.Advance(cv.Img)
				bad, first := 0, ""
				for y := 0; y < scrH; y++ {
					for x := 0; x < scrW; x++ {
						op := int(f[y*scrW+x] & 15)
						mp := inkIndex(cv.Img.RGBAAt(x, y))
						if op != mp {
							if bad == 0 {
								first = fmt.Sprintf("(%d,%d) 原版 %d remake %d", x, y, op, mp)
							}
							bad++
						}
					}
				}
				if bad != 0 {
					t.Fatalf("第 %d 步有 %d 個像素不同，第一個 %s", i+1, bad, first)
				}
			}
			if n := pixDiff(end, frames[len(frames)-1], 0, 0, scrW-1, scrH-1); n != 0 {
				t.Errorf("跑完之後與最後一步差 %d 個像素", n)
			}
			t.Logf("SCG%02d 拉幕 %d 落在 (%d,%d)：%d 步逐像素相同，塊 %v 外沒動", k.scene, k.kind, k.x, k.y, len(frames), r)
		})
	}
}
