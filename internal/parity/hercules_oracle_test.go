//go:build oracle

package parity

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

// TestZZHerculesBootDrawsB0000 是 Issue #4 的收據：裝置選單選 Hercules
// （繪圖裝置答 `1`），原版把畫面畫在 `B0000`，dosgolem 的 `Hercules()`／
// `HerculesNonZero()` 看得到它。
//
// 這裡跑固定的指令數而不是行為停點：Hercules 那條路徑沒有 EGA 那套畫面
// 雜湊可以等（`docs/spec/015` 的路標是 EGA 畫面的 SHA-256），而 15 億道
// 指令在 EGA 那一側已經走到開場故事，這一側只要「畫了東西」就夠。
//
// 正對照：同一段指令用 EGA（答 `2`）跑，`B0000` 必須是 0——否則
// 「非零」量到的不是 Hercules 的畫面，是別的東西恰好落在那一段。
func TestZZHerculesBootDrawsB0000(t *testing.T) {
	root := origRoot(t)
	const steps = 1_500_000_000
	run := func(keys string) (*oracle.Oracle, int) {
		o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
		if err != nil {
			t.Fatalf("載入原版：%v", err)
		}
		o.Type(keys)
		if err := o.Run(steps); err != nil {
			t.Fatalf("%q：原版停止：%v", keys, err)
		}
		return o, o.HerculesNonZero()
	}

	ega, egaNZ := run("122")
	defer ega.Close()
	dumpScreen(t, ega, "hercules-ega-ref") // 同一段指令 EGA 畫到哪，給對照
	if egaNZ != 0 {
		t.Errorf("EGA 那一側 B0000 應該是空的，卻有 %d 個非零位元組", egaNZ)
	}

	herc, hercNZ := run("112")
	defer herc.Close()
	t.Logf("Hercules：B0000 非零 %d / 32768 個位元組；EGA：%d", hercNZ, egaNZ)
	if hercNZ == 0 {
		t.Fatalf("選了 Hercules 卻 B0000 全零——擷取還是沒涵蓋這一段")
	}

	// 原版把同一張圖寫進兩頁（B0000 與 B8000 逐位元組相同）——這是這一支
	// 程式的行為，記下來讓人知道「B8000 有東西」不代表它在畫 CGA。
	p0 := herc.Bytes(addr(0xB0000), 0x8000)
	p1 := herc.Bytes(addr(0xB8000), 0x8000)
	t.Logf("B0000 與 B8000 兩頁%s", map[bool]string{true: "逐位元組相同", false: "不同"}[bytes.Equal(p0, p1)])

	// HGC 的 CRTC（3B4h／3B5h）與模式埠（3B8h）：程式設了什麼版面，
	// 解碼器就該用什麼版面；標準是 R1＝45（每列 45 個字 ＝ 90 bytes）。
	regs := map[uint16][]uint8{}
	for _, w := range herc.PortWrites() {
		if w.Port >= 0x3B0 && w.Port <= 0x3BF {
			regs[w.Port] = append(regs[w.Port], w.Val)
		}
	}
	for port, vals := range regs {
		if len(vals) > 40 {
			vals = vals[:40]
		}
		t.Logf("埠 %03Xh 寫入 %d 次：% x", port, len(regs[port]), vals)
	}

	w, h, stride := herc.HerculesGeometry()
	t.Logf("CRTC 給的幾何：%d×%d、每列 %d bytes", w, h, stride)
	if w != 640 || h != 408 {
		t.Errorf("原版把 Hercules 開成 640×408（R1＝40、R6＝102、R9＝3），解出來卻是 %d×%d", w, h)
	}
	px := herc.Hercules()
	lit := 0
	for _, p := range px {
		lit += int(p)
	}
	t.Logf("解成 %d×%d 之後亮 %d 個像素", w, h, lit)
	if lit == 0 {
		t.Fatalf("B0000 有非零位元組但解碼後一個亮點都沒有——解碼器接錯了")
	}

	dir := os.Getenv("SAN1_SHOTS")
	if dir == "" {
		return
	}
	_ = os.MkdirAll(dir, 0o755)
	// 原始位元組也留一份：解碼的版面（90 bytes／列、四 bank）是 HGC 的標準，
	// 程式自己的畫法不一定照它，圖看起來不對時要拿原始位元組換版面試。
	if err := os.WriteFile(filepath.Join(dir, "hercules-b0000.bin"), p0, 0o644); err != nil {
		t.Fatal(err)
	}
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i, p := range px {
		if p != 0 {
			img.SetGray(i%w, i/w, color.Gray{Y: 255})
		}
	}
	f, err := os.Create(filepath.Join(dir, "hercules-boot.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
