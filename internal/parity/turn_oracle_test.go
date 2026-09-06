//go:build oracle

package parity

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 觀測原版走一個月做了什麼。
//
// 電腦自動示範模式（零人玩）下原版自己把整局打完，所有決策都在那條路徑上。
// 盤面在記憶體裡的位置已知（`tables_oracle_test.go`），所以**每隔一段
// 指令把三張表讀下來，逐欄位比**，就看得到它改了什麼——不必反組譯。
//
// 這一條是探索用的：它不斷言原版怎麼決定，只把「哪些欄位會動、什麼時候動」
// 記錄下來。斷言要等看得懂那些變化再寫。

// dumpScreen 把畫面存成 PNG。**每一步都存**：看得到停在哪個提示，
// 就不必猜下一個鍵要送什麼。
func dumpScreen(t *testing.T, o *oracle.Oracle, name string) {
	t.Helper()
	const w, h = 640, 350
	pix := o.IndexedEGA(w, h)
	if len(pix) < w*h {
		t.Logf("畫面只有 %d 個像素，跳過存圖", len(pix))
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// **用 EGA 的十六色，不要用 DAC 的色盤**：EGA 的顏色在
			// 屬性控制器不在 VGA 的 DAC，`Palette()` 在這個模式下是全黑
			// ——存出來的圖會整張黑，看起來像畫面沒東西而不像色盤取錯。
			img.Set(x, y, assets.EGAPalette[pix[y*w+x]&15])
		}
	}
	dir := os.Getenv("SAN1_SHOTS")
	if dir == "" {
		return
	}
	_ = os.MkdirAll(dir, 0o755)
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

func TestZZWatchTurn(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, sta, gen := sc.Tables()
	total := len(mas) + len(sta) + len(gen)

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	o.Press("122")
	// 主選單第二項是「載入舊進度」。兩份 bundle 的 DATA2 裡都帶著存檔槽，
	// 從存檔進去有沒有防拷密碼是這一輪要問的。
	send := map[int]string{4: "\r", 17: "2", 23: "1"}
	var base uint32
	for i := 0; i < 27; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Fatalf("停止：%v", err)
		}
		if k, ok := send[i]; ok {
			o.Press(k)
		}
		if base == 0 {
			if h := o.Search(mas[:48]); len(h) == 1 {
				base = h[0]
			}
		}
	}
	if base == 0 {
		// 走「載入舊進度」時盤面的內容與劇本檔不同，搜不到——那不代表
		// 沒載入。位址是量出來的固定值，退回去用它。
		base = 0x399b0
		t.Logf("搜不到劇本盤面（走存檔那條就是這樣），改用已知位址 %#x", base)
	}
	dumpScreen(t, o, "00-起點")

	screen := func() []uint8 { return append([]uint8(nil), o.IndexedEGA(640, 350)...) }
	diff := func(a, b []uint8) int {
		n := 0
		for i := range a {
			if a[i] != b[i] {
				n++
			}
		}
		return n
	}

	// **這個階段的提示讀掃描碼**，而且字元佇列的殘留要先清掉
	//（`docs/re/02` §3）。送完等畫面動，動了才走下一步。
	step := func(name, keys string) bool {
		// **等提示畫完再送**：上一個畫面剛動不代表下一個提示準備好了，
		// 送早了那個鍵沒人收——而畫面上看起來是「按了沒反應」。
		if err := o.Run(150_000_000); err != nil {
			t.Logf("沉澱時停止：%v", err)
			return false
		}
		before := screen()
		beforeTab := o.Bytes(addr(base), total)
		o.Drain()
		o.PressScan(keys)
		for k := 0; k < 12; k++ {
			if err := o.Run(50_000_000); err != nil {
				t.Logf("停止：%v", err)
				return false
			}
			// 送完四億道還沒動就再送一次；有時第一次落在重畫的中間。
			if k == 8 {
				o.Drain()
				o.PressScan(keys)
			}
			if d := diff(before, screen()); d > 200 {
				tab := differs8(beforeTab, o.Bytes(addr(base), total))
				t.Logf("✔ %s（送 %q）：%d 億道之後畫面動了 %d 像素、盤面 %d 位元組%s",
					name, keys, (k+1)*5, d, tab,
					where(beforeTab, o.Bytes(addr(base), total), len(mas), len(sta)))
				dumpScreen(t, o, name)
				return true
			}
		}
		t.Logf("✗ %s（送 %q）：六億道之內畫面沒動", name, keys)
		dumpScreen(t, o, name+"-卡住")
		return false
	}

	// 從存檔進去之後的提示還不知道，逐步送、逐步看圖。
	for i, keys := range []string{"1\r", "\r", "5\r", "\r", "\r"} {
		if !step(fmt.Sprintf("%02d-載入", i+1), keys) {
			break
		}
	}
	dumpScreen(t, o, "10-主畫面")

	// **先問「有沒有人在讀鍵盤」再問「該送哪一個鍵」。**
	// KeyWaits 是 0 的話，鍵送到哪一條路都沒用——沒有人在讀。
	t.Logf("到主畫面為止：讀過 %d 個鍵、等鍵盤 %d 次", len(o.KeyReads()), o.KeyWaits())
	if r := o.KeyReads(); len(r) > 0 {
		for _, k := range r[max(0, len(r)-8):] {
			t.Logf("    第 %d 道指令 走 %s 讀到 %#02x", k.Step, k.Via, k.Key)
		}
	}
	if u := o.Unimplemented(); len(u) > 0 {
		t.Logf("還沒實作的服務（%d）：%v", len(u), u[:min(len(u), 12)])
	} else {
		t.Log("沒有用到還沒實作的服務")
	}
	waitsBefore := o.KeyWaits()
	readsBefore := len(o.KeyReads())
	if err := o.Run(200_000_000); err != nil {
		t.Logf("停止：%v", err)
	}
	t.Logf("在主畫面空轉兩億道：多等了 %d 次鍵盤、多讀了 %d 個鍵",
		o.KeyWaits()-waitsBefore, len(o.KeyReads())-readsBefore)
	t.Logf("硬體那條：佇列剩 %d 個掃描碼、IRQ1 送出 %d 次、其中 %d 次進到程式自己的常式",
		o.KeyQueueLen(), o.IRQ1Delivered(), o.IRQ1ToProgram())
	o.PressScan("9")
	t.Logf("送一個 9 之後：佇列 %d", o.KeyQueueLen())
	if err := o.Run(100_000_000); err != nil {
		t.Logf("停止：%v", err)
	}
	t.Logf("跑一億道之後：佇列剩 %d、IRQ1 送出 %d 次、進到程式 %d 次",
		o.KeyQueueLen(), o.IRQ1Delivered(), o.IRQ1ToProgram())

	// 主畫面停在「請下您的命令」。**哪一個鍵會讓月份走下去？**
	// 從同一個快照展開試，一輪就問得完（走到這裡要五分鐘）。
	snap := o.Save()
	for _, c := range []struct {
		name string
		play func(*oracle.Oracle)
	}{
		{"清空後 PressScan 9", func(o *oracle.Oracle) { o.Drain(); o.PressScan("9") }},
		{"Type 9（不清空）", func(o *oracle.Oracle) { o.Type("9") }},
		{"清空後 Type 9", func(o *oracle.Oracle) { o.Drain(); o.Type("9") }},
		{"Press 9（不清空）", func(o *oracle.Oracle) { o.Press("9") }},
		{"什麼都不送（對照組）", func(o *oracle.Oracle) {}},
	} {
		o.Restore(snap)
		before := screen()
		beforeTab := o.Bytes(addr(base), total)
		c.play(o)
		best, bestTab := 0, 0
		for k := 0; k < 8; k++ {
			if err := o.Run(50_000_000); err != nil {
				break
			}
			if d := diff(before, screen()); d > best {
				best = d
			}
			if d := differs8(beforeTab, o.Bytes(addr(base), total)); d > bestTab {
				bestTab = d
			}
		}
		t.Logf("%-22s → 畫面差 %6d、盤面差 %4d、佇列剩 %d%s",
			c.name, best, bestTab, o.Pending(),
			where(beforeTab, o.Bytes(addr(base), total), len(mas), len(sta)))
		dumpScreen(t, o, "11-"+c.name)
	}
}

// where 說變化落在哪一張表、哪些欄位位移。
func where(a, b []byte, nmas, nsta int) string {
	tabs := []struct {
		name string
		lo   int
		n    int
		rec  int
	}{
		{"諸侯", 0, nmas, state.MasterRecordSize},
		{"州郡", nmas, nsta, state.PrefectureRecordSize},
		{"人物", nmas + nsta, len(a) - nmas - nsta, state.GeneralRecordSize},
	}
	out := ""
	for _, x := range tabs {
		offs := map[int]int{}
		recs := map[int]bool{}
		for i := 0; i < x.n; i++ {
			if a[x.lo+i] != b[x.lo+i] {
				offs[i%x.rec]++
				recs[i/x.rec] = true
			}
		}
		if len(offs) == 0 {
			continue
		}
		out += "\n    " + x.name + "："
		for o2, n := range offs {
			out += fmt.Sprintf(" %d(×%d)", o2, n)
		}
		out += fmt.Sprintf("　%d 筆", len(recs))
	}
	return out
}

func differs8(a, b []byte) int {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return n
}
