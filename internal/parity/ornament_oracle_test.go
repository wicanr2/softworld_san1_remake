//go:build oracle

package parity

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// 主選單右下角小飾框裡那一格的動畫。
//
// 舊證據只有「同一台原版連拍兩張不同」，說不出來源、順序或週期；這支
// 直接把每個完整畫格與 `DATA1/CURA0`～`CURA5` 的遮罩合成結果逐點比較，
// 並以高密度取樣釘住循環。
//
// 做法：停在主選單，**不送任何鍵**，每隔一段指令抓那一格的雜湊。
// 不同的雜湊就是一格；雜湊重複出現的間隔就是週期。
const (
	ornX0, ornY0 = 590, 328
	ornX1, ornY1 = 602, 345
)

func TestZZMenuOrnamentFrames(t *testing.T) {
	root := origRoot(t)
	data1 := openContainer(t, filepath.Join(root, "DATA1"))
	data3 := openContainer(t, filepath.Join(root, "DATA3"))
	composed, err := assets.MenuScreenFrames(data1, data3)
	if err != nil {
		t.Fatal(err)
	}

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	// 用行為路標走到主選單真的開始讀掃描碼；固定跑 17 段會隨工具鏈速度
	// 停在尚未啟動動畫的中間畫面，得到一張永遠不動的假結果。
	bootToMenu(t, o)
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
	frameOf := func(px []uint8) int {
		matches := []int{}
		for i, want := range composed {
			match := true
			for y := ornY0; y <= ornY1 && match; y++ {
				for x := ornX0; x <= ornX1; x++ {
					got := px[(y-ornY0)*(ornX1-ornX0+1)+(x-ornX0)]
					if got != want.At(x, y) {
						match = false
						break
					}
				}
			}
			if match {
				matches = append(matches, i)
			}
		}
		if len(matches) != 1 {
			// 高密度取樣可能正落在遮罩與圖像的兩次搬運之間；這不是一個
			// 完整畫格，保留成 -1 才能量出轉場成本，不能硬歸到最近的一格。
			return -1
		}
		return matches[0]
	}

	// 取樣：每 10,000 道指令看一次，看 1,000 次。**取樣要比動畫快**，
	// 不然量到的是別名不是週期。
	const (
		tick    = 10_000
		samples = 1_000
	)
	order := []string{}
	seenAt := map[string][]int{}
	frames := make([]int, 0, samples)
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
		frames = append(frames, frameOf(px))
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
	type run struct {
		frame, samples, start int
	}
	runs := []run{}
	start := 0
	for i := 1; i <= len(frames); i++ {
		if i < len(frames) && frames[i] == frames[start] {
			continue
		}
		runs = append(runs, run{frames[start], i - start, start})
		start = i
	}
	seenFrame := map[int]bool{}
	transient := 0
	for _, f := range frames {
		if f < 0 {
			transient++
		} else {
			seenFrame[f] = true
		}
	}
	t.Logf("未落在完整 CURA 畫格的轉場取樣：%d／%d", transient, samples)
	if len(seenFrame) != len(composed) {
		t.Errorf("高密度取樣只看到 CURA 畫格 %v，應走完 0–5", seenFrame)
	}
	// 完整畫格只准 0→1→2→3→4→5→0；搬運中的 -1 不冒充畫格，也不
	// 中斷前後兩個完整畫格的順序檢查。
	last := -1
	durations := map[int]map[int]int{}
	zeroStarts := []int{}
	for i, r := range runs {
		if r.frame < 0 {
			continue
		}
		if last >= 0 && r.frame != (last+1)%len(composed) {
			t.Errorf("CURA 播放順序從 %d 跳到 %d，不是 0→1→2→3→4→5", last, r.frame)
		}
		last = r.frame
		// 第一段與最後一段可能被取樣窗口裁掉，不拿來量完整停留時間。
		if i > 0 && i < len(runs)-1 {
			if durations[r.frame] == nil {
				durations[r.frame] = map[int]int{}
			}
			durations[r.frame][r.samples]++
		}
		if r.frame == 0 {
			zeroStarts = append(zeroStarts, r.start)
		}
	}
	cycles := map[int]int{}
	for i := 1; i < len(zeroStarts); i++ {
		cycles[zeroStarts[i]-zeroStarts[i-1]]++
	}
	t.Logf("CURA 0→1→2→3→4→5；各格持續取樣數分佈 %v；整輪分佈 %v（每格 %d 道指令）",
		durations, cycles, tick)
}
