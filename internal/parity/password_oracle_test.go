//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 防拷密碼關。
//
// **它不在開新遊戲那條路上。** 走載入舊進度進得了主畫面，第一道帶 ＊
// 的指令（使用後轉移控制權）用掉之後才跳出來：畫面換成兩張人物圖 ＋
// 一個地支 ＋ 四個 `?`，提示 `請輸入密碼(-寅-)`。
//
// 密碼表印在說明書 p.44–45，採防影印彩色網點，掃描中判讀不出來；
// `AA.EXE` 裡也沒有以 packed BCD 或 16 位元小端連續存放的 1728 組表
//（`CONTEXT.md` Worklist）。所以答案要從原版自己的記憶體問。
//
// 做法：主畫面拍一張快照，觸發密碼關之後看**哪些位址變了**，
// 從裡面挑「值是四位數」的候選，逐個試。試一個很便宜——
// 從同一個快照展開，不必重開機。

// TestZZPassword 找出這一局的密碼。
func TestZZPassword(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, _, _ := sc.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	bootToMain(t, o, mas)
	atMain := o.Save()

	const settle = 40_000_000
	for _, k := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(k)
		if err := o.Run(settle * 3); err != nil {
			t.Fatalf("觸發密碼關時停止：%v", err)
		}
	}
	dumpScreen(t, o, "30-密碼關")
	atPwd := o.Save()
	pwdScr := screenOf(o)

	// 哪一條輸入路徑？**四個 `?` 有沒有被打掉是判準。**
	// 掃描碼那條在這個提示上沒有反應（第一輪的觀測），所以三條都試一次。
	for _, c := range []struct {
		name string
		play func()
	}{
		{"PressScan", func() { o.Drain(); o.PressScan("2492") }},
		{"Type（字元佇列）", func() { o.Drain(); o.Type("2492") }},
		{"Press（兩條都餵）", func() { o.Drain(); o.Press("2492") }},
	} {
		o.Restore(atPwd)
		c.play()
		if err := o.Run(settle * 2); err != nil {
			t.Logf("%s：停止 %v", c.name, err)
			continue
		}
		t.Logf("輸入路徑 %-16s → 畫面差 %d", c.name, pixelDiff(pwdScr, screenOf(o), nil))
		dumpScreen(t, o, "31-路徑-"+c.name)
	}

	// 候選：從主畫面到密碼畫面之間變動的位址裡，值是四位數的。
	o.Restore(atPwd)
	changed := o.SearchChanged(atMain)
	seen := map[int]bool{}
	var cand []int
	for _, a := range changed {
		w := o.Word(addr(a))
		// 純二進位的四位數
		if w >= 1000 && w <= 9999 && !seen[int(w)] {
			seen[int(w)] = true
			cand = append(cand, int(w))
		}
		// packed BCD：0x2492 就是 2492
		if bcd, ok := fromBCD(w); ok && bcd >= 1000 && !seen[bcd] {
			seen[bcd] = true
			cand = append(cand, bcd)
		}
	}
	t.Logf("主畫面 → 密碼畫面之間變了 %d 個位址，其中湊得出 %d 個相異的四位數",
		len(changed), len(cand))
	if len(cand) > 120 {
		t.Logf("候選太多（%d），只試前 120 個", len(cand))
		cand = cand[:120]
	}

	for i, v := range cand {
		o.Restore(atPwd)
		o.Drain()
		o.Press(fmt.Sprintf("%04d\r", v))
		if err := o.Run(settle); err != nil {
			continue
		}
		o.Press("Y\r")
		if err := o.Run(settle * 2); err != nil {
			continue
		}
		d := pixelDiff(pwdScr, screenOf(o), nil)
		if d > 20000 {
			t.Logf("★ 第 %d 個候選 %04d 讓畫面整個換掉（差 %d 像素）——很可能就是密碼",
				i+1, v, d)
			dumpScreen(t, o, fmt.Sprintf("32-命中-%04d", v))
			return
		}
	}
	t.Logf("%d 個候選都沒讓畫面換掉。下一步是換一種存法找候選，"+
		"或反組譯 %s 的比較點", len(cand), "0x507ec 請輸入密碼")
}

// fromBCD 把 packed BCD 的字換成十進位數；有一個 nibble 大於 9 就不是 BCD。
func fromBCD(w uint16) (int, bool) {
	n := 0
	for s := 12; s >= 0; s -= 4 {
		d := int(w>>uint(s)) & 15
		if d > 9 {
			return 0, false
		}
		n = n*10 + d
	}
	return n, true
}
