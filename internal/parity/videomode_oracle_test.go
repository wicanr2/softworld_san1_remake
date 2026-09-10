//go:build oracle

package parity

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 畫面幾列高：CRTC 的暫存器 ＋ 逐列墨水統計。
//
// 答案是 **408**，定案在 `docs/spec/006`。這一支是那份規格的兩條證據
// 常駐在測試裡的版本，順便當回歸——尺寸再被誰改回 350 時，
// 這裡的數字會立刻不對。
//
// 為什麼要這樣問：平面式 VRAM 不記解析度。`IndexedEGASize` 的尺寸由
// 呼叫端給，它的註解自己就寫著「猜錯不會報錯」；而原版一次都沒呼叫
// `int 10h AH=00`，BDA 的模式位元組一路停在開機值 03h，
// 所以**沒有任何一格暫存器說過 350**。只能問兩個地方：原版寫進 CRTC
// 的值，以及畫面資料本身。
//
// ⚠ **不能只看一張畫面。** 一張畫面沒畫到某一列，與那一列不存在，
// 在資料上長得一模一樣。主選單與遊戲主畫面一起看，才分得出
// 「這張沒畫」與「畫面就這麼高」。

const probeW, probeH = 640, 480

// rowInk 回傳每一列的非零像素數。
func rowInk(pix []uint8, w, h int) []int {
	out := make([]int, h)
	for y := 0; y < h; y++ {
		n := 0
		for x := 0; x < w; x++ {
			if pix[y*w+x] != 0 {
				n++
			}
		}
		out[y] = n
	}
	return out
}

// dumpTall 把畫面照 probeH 存成 PNG（`SAN1_SHOTS` 才寫檔）。
func dumpTall(t *testing.T, pix []uint8, name string) {
	t.Helper()
	dir := os.Getenv("SAN1_SHOTS")
	if dir == "" {
		return
	}
	_ = os.MkdirAll(dir, 0o755)
	img := image.NewRGBA(image.Rect(0, 0, probeW, probeH))
	for y := 0; y < probeH; y++ {
		for x := 0; x < probeW; x++ {
			img.Set(x, y, assets.EGAPalette[pix[y*probeW+x]&15])
		}
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Log(err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Log(err)
	}
}

// report 印出「最後一列有墨水的位置」與 340–420 之間的逐列統計。
func report(t *testing.T, pix []uint8, name string) {
	t.Helper()
	ink := rowInk(pix, probeW, probeH)
	last := -1
	for y := probeH - 1; y >= 0; y-- {
		if ink[y] > 0 {
			last = y
			break
		}
	}
	total350, total400 := 0, 0
	for y := 0; y < 350; y++ {
		total350 += ink[y]
	}
	for y := 350; y < 400; y++ {
		total400 += ink[y]
	}
	after400 := 0
	for y := 400; y < probeH; y++ {
		after400 += ink[y]
	}
	t.Logf("%s：最後一列有墨水 = %d；0–349 共 %d 點、350–399 共 %d 點、400–479 共 %d 點",
		name, last, total350, total400, after400)
	for y := 340; y < 420; y += 4 {
		t.Logf("  %s 列 %3d–%3d：%4d %4d %4d %4d", name, y, y+3,
			ink[y], ink[y+1], ink[y+2], ink[y+3])
	}
	dumpTall(t, pix, name)
}

// TestZZScreenHeightProbe 量畫面的實際高度，並讀出原版設進 CRTC 的值。
//
// 逐列墨水統計的形狀（`docs/spec/006` §2.5）：主選單在 350–399 有內容
// （第三列按鈕的下半、直牌最後一個字、小飾框的下半），408 起是離屏資料；
// 開場三英圖貼滿 0–399，400–407 是貼圖前清成的黑。
func TestZZScreenHeightProbe(t *testing.T) {
	root := origRoot(t)

	// 一、主選單（使用者比對的那一張）。
	func() {
		o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
		if err != nil {
			t.Fatal(err)
		}
		defer o.Close()
		// 開機的四題裝置選擇要答完才走得到主選單（`bootToMain`：`122`
		// ＋ 第 4 步的 `\r`）；主選單那一題**不答**，畫面就停在那裡。
		// **不送鍵的話一步都走不動**——它卡在第一題等輸入，
		// 而畫面全黑看起來像「還沒畫」不像「在等人回答」。
		o.TypeBoth("122")
		for i := 0; i < 17; i++ {
			if err := o.Run(50_000_000); err != nil {
				t.Fatalf("主選單：停止：%v", err)
			}
			if i == 4 {
				o.TypeBoth("\r")
			}
		}
		pix := o.IndexedEGASize(probeW, probeH)
		if len(pix) < probeW*probeH {
			t.Fatalf("主選單：畫面只有 %d 個像素", len(pix))
		}
		report(t, pix, "probe-title")

		// CRTC（`3D4`／`3D5`）是「畫面幾列高」唯一的一手來源。
		// 索引 `12` ＝ Vertical Display End 的低八位，索引 `07` 的
		// bit1／bit6 是它的第 8／9 位——`349` 與 `399` 差在這裡。
		// `PortWrites()` 回**全部**埠寫入，範圍過濾由呼叫端做
		// （dosgolem `oracle/ports.go`）。
		var idx uint8
		n := 0
		last := map[uint8]uint8{}
		for _, w := range o.PortWrites() {
			if w.Port < 0x3D4 || w.Port > 0x3D5 {
				continue
			}
			n++
			if w.Port == 0x3D4 {
				idx = w.Val
				continue
			}
			last[idx] = w.Val
		}
		t.Logf("CRTC 寫入 %d 次", n)
		for _, i := range []uint8{0x00, 0x01, 0x06, 0x07, 0x09, 0x10, 0x11, 0x12, 0x13, 0x15, 0x16, 0x17} {
			if v, ok := last[i]; ok {
				t.Logf("  CRTC[%02X] = %02X", i, v)
			}
		}
		// 其他視訊埠也一起看：`3C2`（Miscellaneous Output，bit6–7 選
		// 垂直大小）、`3C4`（序列器）、`3CE`（圖形控制器）。
		for _, port := range []uint16{0x3C2, 0x3C3, 0x3CC} {
			var lastVal uint8
			cnt := 0
			for _, w := range o.PortWrites() {
				if w.Port == port {
					lastVal, cnt = w.Val, cnt+1
				}
			}
			if cnt > 0 {
				t.Logf("  埠 %03X 最後寫入 %02X（共 %d 次）", port, lastVal, cnt)
			}
		}
	}()

	// 二、遊戲主畫面（README 那一張的基準）。
	func() {
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
		bootToMain(t, o, seedMas)
		pix := o.IndexedEGASize(probeW, probeH)
		if len(pix) < probeW*probeH {
			t.Fatalf("主畫面：畫面只有 %d 個像素", len(pix))
		}
		report(t, pix, "probe-main")
	}()
}
