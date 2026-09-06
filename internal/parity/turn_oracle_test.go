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
	send := map[int]string{4: "\r", 17: "1", 23: "1"}
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
		t.Fatal("沒走到盤面載入")
	}
	dumpScreen(t, o, "00-幾人玩")

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

	if !step("01-零人玩", "0\r") {
		return
	}
	// 送 0 之後畫面出「電腦自動示範模式」，再一個鍵才到「請設定難度(1-10)」。
	for i, keys := range []string{"5\r", "5\r", "\r"} {
		if !step(fmt.Sprintf("%02d-續行", i+2), keys) {
			break
		}
	}
	// 進到遊戲之後就讓它自己跑，看盤面什麼時候動。
	prev := o.Bytes(addr(base), total)
	for i := 0; i < 20; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Logf("停止：%v", err)
			break
		}
		cur := o.Bytes(addr(base), total)
		if d := differs8(prev, cur); d > 0 {
			t.Logf("自走 %d 億道：盤面動了 %d 個位元組%s",
				(i+1)*5, d, where(prev, cur, len(mas), len(sta)))
			prev = cur
		}
	}
	dumpScreen(t, o, "99-最後")
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
