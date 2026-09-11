//go:build oracle

package parity

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 主選單右下角小飾框裡那一格的動畫。
//
// `docs/spec/005` 記著「原版放了一段動畫，同一台原版連拍兩張就會不同；
// 那段動畫是什麼還沒解出來」，`internal/assets` 的逐點比對因此把那一格
// 挖掉（`menuAnim`）。**「連拍兩張不同」只說明它會動**，說不出有幾格、
// 循環多久——這一支把那兩個數字量出來。
//
// 做法：停在主選單，**不送任何鍵**，每隔一段指令抓那一格的雜湊。
// 不同的雜湊就是一格；雜湊重複出現的間隔就是週期。
const (
	ornX0, ornY0 = 590, 328
	ornX1, ornY1 = 602, 345
)

func TestZZMenuOrnamentFrames(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	_ = sc0

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	// 走到主選單就停：`bootToMain` 在第 17 步送「2」（載入舊進度），
	// 所以第 16 步結束時畫面還停在主選單。
	o.TypeBoth("122")
	for i := 0; i < 17; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Fatalf("開機第 %d 步停止：%v", i, err)
		}
		if i == 4 {
			o.TypeBoth("\r")
		}
	}
	dumpScreen(t, o, "ornament-menu")

	cell := func() ([]uint8, string) {
		pix := o.IndexedEGASize(scrW, scrH)
		if len(pix) < scrW*scrH {
			return nil, ""
		}
		var b []uint8
		for y := ornY0; y <= ornY1; y++ {
			b = append(b, pix[y*scrW+ornX0:y*scrW+ornX1+1]...)
		}
		return b, fmt.Sprintf("%x", sha256.Sum256(b))[:12]
	}

	// 取樣：每 2,000,000 道指令看一次，看 200 次。**取樣要比動畫快**，
	// 不然量到的是別名不是週期。
	const (
		tick    = 2_000_000
		samples = 200
	)
	order := []string{}
	seenAt := map[string][]int{}
	for i := 0; i < samples; i++ {
		if err := o.Run(tick); err != nil {
			t.Fatalf("取樣第 %d 次停止：%v", i, err)
		}
		px, h := cell()
		if h == "" {
			t.Fatal("抓不到畫面")
		}
		if _, ok := seenAt[h]; !ok {
			order = append(order, h)
			dumpScreen(t, o, fmt.Sprintf("ornament-%02d-%s", len(order), h))
			_ = px
		}
		seenAt[h] = append(seenAt[h], i)
	}

	t.Logf("取樣 %d 次（每 %d 道指令），看到 %d 種畫格：", samples, tick, len(order))
	for i, h := range order {
		at := seenAt[h]
		gaps := map[int]int{}
		for j := 1; j < len(at); j++ {
			gaps[at[j]-at[j-1]]++
		}
		t.Logf("  第 %d 格 %s：出現 %d 次，取樣間隔分佈 %v", i+1, h, len(at), gaps)
	}
	// **一格也算結果**：那表示這一格不是動畫，而是「連拍兩張不同」另有原因。
	if len(order) == 1 {
		t.Logf("只有一種畫格——在這個取樣窗口裡它沒有動")
	}

	// 序列本身：前 40 次取樣的畫格編號，看得出循環。
	idx := map[string]int{}
	for i, h := range order {
		idx[h] = i + 1
	}
	seq := ""
	for i := 0; i < samples && i < 40; i++ {
		for h, at := range seenAt {
			for _, k := range at {
				if k == i {
					seq += fmt.Sprint(idx[h])
				}
			}
		}
	}
	t.Logf("前 40 次取樣的畫格序列：%s", seq)
}
