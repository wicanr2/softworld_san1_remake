//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// 開場那張三英圖是 `TITL0`–`TITL3` 四塊拼出來的（`docs/formats/04`）。
//
// 四塊各 160×400，並排正好 640×400，而畫面是 640×408
//（`docs/spec/006`）——**底下 8 列不是素材畫的**。橫向是四塊並排還是
// 交錯、縱向從第幾列開始取，與其推，讓原版自己說：跑到那一格
// 把畫面倒出來，逐格比。
//
// ⚠ 比對只涵蓋素材有的那 400 列；底下 8 列由 `assets` 那一邊的
// `TestTitleArtMatchesTheOriginal` 釘（它比的是整張 408 列）。
//
// 原版讀這四張的時機量得到（`TestZZOpeningAssetNames`）：約 1.9 億條
// 指令，正好在第三格與第四格之間。

// TestTitleArtLayoutMatchesTheOriginal 找出 TITL0–3 怎麼組成開場的三英圖。
func TestTitleArtLayoutMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA1"))

	var pieces [4]*assets.Image
	for i := range pieces {
		name := fmt.Sprintf("TITL%d.IMG", i)
		j, ok := c.ByName(name)
		if !ok {
			t.Fatalf("DATA1 裡沒有 %s", name)
		}
		im, err := assets.DecodeImage(c.Data(j))
		if err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		if im.W != 160 || im.H != 400 {
			t.Fatalf("%s 是 %d×%d，預期 160×400", name, im.W, im.H)
		}
		pieces[i] = im
	}

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	o.Press("122")
	if err := o.Run(250_000_000); err != nil {
		t.Fatalf("跑到三英圖那一格時停止：%v", err)
	}
	dumpScreen(t, o, "orig-title-art")
	pix := o.IndexedEGA(scrW, scrH)
	if len(pix) < scrW*scrH {
		t.Fatalf("畫面只有 %d 個像素", len(pix))
	}

	// 兩種橫向擺法 × 縱向偏移 0..50，逐格數不同的格數。
	type layout struct {
		name string
		at   func(x, y int) byte
	}
	side := layout{"四塊並排", func(x, y int) byte {
		return pieces[x/160].Pix[y*160+x%160]
	}}
	inter := layout{"逐欄交錯", func(x, y int) byte {
		return pieces[x%4].Pix[y*160+x/4]
	}}

	// 只比素材涵蓋得到的列：四塊各 400 列，往下位移 dy 之後
	// 剩 400−dy 列可比。**拿畫面的 408 列去索引 400 列的素材會爆**，
	// 而那看起來像測試壞了不像前提改了。
	const pieceH = 400
	best := struct {
		name string
		dy   int
		bad  int
	}{bad: scrW*scrH + 1}
	for _, l := range []layout{side, inter} {
		for dy := 0; dy <= 50; dy++ {
			bad := 0
			for y := 0; y < pieceH-dy; y++ {
				for x := 0; x < scrW; x++ {
					if l.at(x, y+dy) != pix[y*scrW+x]&15 {
						bad++
					}
				}
			}
			if bad < best.bad {
				best.name, best.dy, best.bad = l.name, dy, bad
			}
			if dy <= 2 || bad == 0 {
				t.Logf("%s、往下取第 %d 列起：%d 格不同（共 %d）",
					l.name, dy, bad, scrW*(pieceH-dy))
			}
		}
	}
	t.Logf("最好的一組：%s、第 %d 列起，%d 格不同", best.name, best.dy, best.bad)
	if best.bad != 0 {
		t.Errorf("沒有一種擺法逐格相同，最好的還差 %d 格", best.bad)
	}
}
