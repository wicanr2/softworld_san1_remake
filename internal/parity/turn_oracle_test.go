//go:build oracle

package parity

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 觀測原版走一個月做了什麼。
//
// 盤面在記憶體裡的位置已知（`tables_oracle_test.go`），所以**每送一次鍵
// 就把三張表讀下來，逐欄位比**，就看得到它改了什麼——不必反組譯。
//
// 這一條是探索用的：它不斷言原版怎麼決定，只把「哪些欄位會動、什麼時候動」
// 記錄下來。斷言要等看得懂那些變化再寫。

const scrW, scrH = 640, 350

// dumpScreen 把畫面存成 PNG。**每一步都存**：看得到停在哪個提示，
// 就不必猜下一個鍵要送什麼。
func dumpScreen(t *testing.T, o *oracle.Oracle, name string) {
	t.Helper()
	pix := o.IndexedEGA(scrW, scrH)
	if len(pix) < scrW*scrH {
		t.Logf("畫面只有 %d 個像素，跳過存圖", len(pix))
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, scrW, scrH))
	for y := 0; y < scrH; y++ {
		for x := 0; x < scrW; x++ {
			// **用 EGA 的十六色，不要用 DAC 的色盤**：EGA 的顏色在
			// 屬性控制器不在 VGA 的 DAC，`Palette()` 在這個模式下是全黑
			// ——存出來的圖會整張黑，看起來像畫面沒東西而不像色盤取錯。
			img.Set(x, y, assets.EGAPalette[pix[y*scrW+x]&15])
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

// bootToMain 把原版開到遊戲主畫面，回傳三張表的基底與長度。
//
// 走的是「載入舊進度」那條（`docs/re/02` §3.3）：開新遊戲會問防拷密碼，
// 而密碼表在說明書掃描裡判讀不出來。
func bootToMain(t *testing.T, o *oracle.Oracle, mas []byte) uint32 {
	t.Helper()
	o.Press("122")
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
	}

	// **這個階段的提示讀掃描碼**，而且字元佇列的殘留要先清掉
	//（`docs/re/02` §3）。送完等畫面動，動了才走下一步。
	for i, keys := range []string{"1\r", "\r", "5\r", "\r", "\r"} {
		if err := o.Run(150_000_000); err != nil {
			t.Fatalf("沉澱時停止：%v", err)
		}
		before := screenOf(o)
		o.Drain()
		o.PressScan(keys)
		moved := false
		for k := 0; k < 12; k++ {
			if err := o.Run(50_000_000); err != nil {
				t.Fatalf("停止：%v", err)
			}
			if pixelDiff(before, screenOf(o), nil) > 200 {
				moved = true
				break
			}
		}
		if !moved {
			dumpScreen(t, o, fmt.Sprintf("卡在第%d步", i+1))
			t.Fatalf("載入序列第 %d 步（送 %q）畫面沒動", i+1, keys)
		}
	}
	return base
}

// bootToGame 一路開到**過了防拷密碼**的主畫面。
//
// `bootToMain` 只走到主畫面，那時密碼關還沒跳出來——第一道帶 ＊ 的
// 指令用掉之後才會（`docs/re/02` §3.3）。所以要真的讓月份走下去，
// 得先把那一關過掉。
//
// 代價是**開頭那一個月已經走過去了**：觸發密碼關的那道「內政 → 休息」
// 就是玩家那一個月的指令。回傳時遊戲停在下一個月的主畫面。
func bootToGame(t *testing.T, o *oracle.Oracle, mas []byte) uint32 {
	t.Helper()
	base := bootToMain(t, o, mas)
	const settle = 40_000_000
	for _, k := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(k)
		if err := o.Run(settle * 3); err != nil {
			t.Fatalf("觸發密碼關時停止：%v", err)
		}
	}
	// 密碼那個提示**不吃掃描碼**（實測畫面差 240，與什麼都不送同級），
	// 所以兩條路一起餵。
	o.Drain()
	o.Press(passwordAnswer + "\r")
	if err := o.Run(settle); err != nil {
		t.Fatalf("作答時停止：%v", err)
	}
	o.Press("Y\r")
	if err := o.Run(settle * 3); err != nil {
		t.Fatalf("確認時停止：%v", err)
	}
	return base
}

func screenOf(o *oracle.Oracle) []uint8 {
	return append([]uint8(nil), o.IndexedEGA(scrW, scrH)...)
}

// pixelDiff 數兩張畫面差幾個像素，`mask` 為真的位置不算。
func pixelDiff(a, b []uint8, mask []bool) int {
	n := 0
	for i := range a {
		if i >= len(b) {
			break
		}
		if mask != nil && i < len(mask) && mask[i] {
			continue
		}
		if a[i] != b[i] {
			n++
		}
	}
	return n
}

// blinkMask 找出「什麼都不按也會變」的像素。
//
// **游標閃爍會蓋掉一個字的回顯。** 主畫面上輸入一個數字只改動兩百多個
// 像素，與游標閃爍同一個量級——不扣掉雜訊的話，「按了有反應」與
// 「按了沒反應」量出來的數字一樣，於是每一種送法都會被判成沒反應。
//
// 執行器是決定性的：從同一個快照跑同樣的指令數，游標的相位一模一樣，
// 所以扣掉這一份之後對照組的差應該是零。
func blinkMask(o *oracle.Oracle, snap *oracle.State, steps uint64, rounds int) ([]uint8, []bool) {
	o.Restore(snap)
	base := screenOf(o)
	mask := make([]bool, len(base))
	for r := 0; r < rounds; r++ {
		o.Restore(snap)
		_ = o.Run(steps * uint64(r+1))
		cur := screenOf(o)
		for i := range base {
			if i < len(cur) && base[i] != cur[i] {
				mask[i] = true
			}
		}
	}
	o.Restore(snap)
	return base, mask
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

	base := bootToMain(t, o, mas)
	dumpScreen(t, o, "10-主畫面")

	polls, pressed := o.MouseActivity()
	t.Logf("到主畫面為止：滑鼠被輪詢 %d 次（其中 %d 次有按鍵）、讀過 %d 個鍵、等鍵盤 %d 次",
		polls, pressed, len(o.KeyReads()), o.KeyWaits())

	// **一個數字要跟著 Enter 才算輸入完。** 說明書 p.5／p.17：
	// 「下令用數字鍵 0–9」「先鍵入數字再按 Enter」「不輸入數字直接按
	// Enter 結束該指令」。先前只送裸數字，回顯的那一個字被游標閃爍的
	// 雜訊蓋過去，量起來就是「沒反應」。
	snap := o.Save()
	const settle = 50_000_000
	beforeScr, mask := blinkMask(o, snap, settle, 3)
	masked := 0
	for _, b := range mask {
		if b {
			masked++
		}
	}
	t.Logf("游標雜訊：%d 個像素會自己變（總共 %d）", masked, len(mask))

	beforeTab := o.Bytes(addr(base), total)
	for _, k := range []struct{ name, keys string }{
		{"對照組（什麼都不送）", ""},
		{"Enter", "\r"},
		{"9 ＋ Enter", "9\r"},
	} {
		o.Restore(snap)
		o.Drain()
		if k.keys != "" {
			o.PressScan(k.keys)
		}
		if err := o.Run(settle * 3); err != nil {
			t.Logf("%s：停止 %v", k.name, err)
			continue
		}
		after := screenOf(o)
		tab := o.Bytes(addr(base), total)
		t.Logf("%-22s → 扣掉雜訊還差 %5d 像素、盤面 %4d 位元組%s",
			k.name, pixelDiff(beforeScr, after, mask), differs8(beforeTab, tab),
			where(beforeTab, tab, len(mas), len(sta)))
		dumpScreen(t, o, "11-"+k.name)
	}
	o.Restore(snap)
}

// 已知的欄位名（`internal/state` 的解碼器，`docs/formats/03`）。
//
// **位移印成名字才讀得下去。** 一輪走完會列出幾十筆變化，
// 「29(×43)」與「物價(×43)」的差別是要不要回去翻文件。
var (
	prefField = map[int]string{
		0: "郡名", 1: "郡名", 2: "郡名", 3: "郡名",
		14: "人口", 15: "人口", 16: "兵士", 17: "兵士",
		18: "金", 19: "金", 20: "米", 21: "米",
		22: "在職將", 23: "在野將", 26: "民忠", 27: "地力",
		28: "水利", 29: "物價", 30: "所屬",
	}
	genField = map[int]string{
		7: "年齡", 8: "體力", 9: "智", 10: "武", 11: "魅",
		12: "職位", 13: "出身郡", 16: "忠誠", 17: "身分", 18: "勢力",
		19: "所在", 21: "兵種", 22: "兵力", 23: "兵力",
		24: "訓練", 25: "武裝",
	}
	masField = map[int]string{2: "君主"}
)

// fieldName 把位移換成名字；還沒解出來的就印位移。
func fieldName(m map[int]string, off int) string {
	if n, ok := m[off]; ok {
		return n
	}
	return fmt.Sprintf("+%d", off)
}

// where 說變化落在哪一張表、哪些欄位。
func where(a, b []byte, nmas, nsta int) string {
	tabs := []struct {
		name  string
		lo    int
		n     int
		rec   int
		field map[int]string
	}{
		{"諸侯", 0, nmas, state.MasterRecordSize, masField},
		{"州郡", nmas, nsta, state.PrefectureRecordSize, prefField},
		{"人物", nmas + nsta, len(a) - nmas - nsta, state.GeneralRecordSize, genField},
	}
	out := ""
	for _, x := range tabs {
		offs := map[int]int{}
		recs := map[int]bool{}
		for i := 0; i < x.n && x.lo+i < len(b); i++ {
			if a[x.lo+i] != b[x.lo+i] {
				offs[i%x.rec]++
				recs[i/x.rec] = true
			}
		}
		if len(offs) == 0 {
			continue
		}
		keys := make([]int, 0, len(offs))
		for k := range offs {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		// 同一個欄位的高低位元組要併成一筆，否則一個 16 位元的值會
		// 看起來像兩個欄位在動。
		seen := map[string]int{}
		order := []string{}
		for _, k := range keys {
			n := fieldName(x.field, k)
			if _, ok := seen[n]; !ok {
				order = append(order, n)
			}
			seen[n] += offs[k]
		}
		out += "\n    " + x.name + "："
		for _, n := range order {
			out += fmt.Sprintf(" %s(×%d)", n, seen[n])
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
