//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestOriginalSaveLoadsIdentically 讓原版載入它自己的進度，remake 走
// `save.ReadOriginal` 讀同一格，**三張表逐位元組比**。
//
// 這是 M6 出口條件裡「存讀檔的同狀態對拍」的載入方向。先前只驗過
// **劇本**進得了記憶體（`TestScenarioTablesLiveInMemory`）與 remake 自己
// 的 round-trip（`internal/save`），沒有驗過「原版讀進去的盤面與 remake
// 讀進去的盤面是同一個」——而存檔比劇本多了 `BASEPRO`／`BASEPRE`／
// 名稱表三個項目（`docs/re/08`）。
//
// 走的是 `bootToMain`：它送的鍵就是「載入舊進度 → 第 1 格」
// （`send` 裡的 `17:"2"`、`23:"1"`），停在主畫面，**還沒走過任何一個月**，
// 所以記憶體裡的三張表就是那一格的內容。
func TestOriginalSaveLoadsIdentically(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))

	// 搜尋樣本用劇本 001 的諸侯表前 48 個位元組；載入舊進度時盤面
	// 與劇本不同，`bootToMain` 搜不到會退回量出來的固定位址。
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

	base := bootToMain(t, o, seedMas)
	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	total := nMas + nSta + nGen
	orig := o.Bytes(addr(base), total)

	g, err := save.ReadOriginal(c, 1, state.EditionBase)
	if err != nil {
		t.Fatalf("remake 讀不了原版的第 1 個進度：%v", err)
	}
	mas, sta, gen := func() ([]byte, []byte, []byte) {
		a, b, d, err := g.Tables()
		if err != nil {
			t.Fatalf("remake 的盤面寫不回三張表：%v", err)
		}
		return a, b, d
	}()
	mine := append(append(append([]byte{}, mas...), sta...), gen...)

	if len(mine) != total {
		t.Fatalf("三張表的長度不一樣：原版 %d，remake %d", total, len(mine))
	}
	t.Logf("原版載入第 1 個進度：三張表 %d 個位元組（諸侯 %d／州郡 %d／人物 %d）",
		total, nMas, nSta, nGen)
	// 兵士（offset 16）與在職將（offset 22）都是原版常式維護的快照。
	// 這裡不再遮掉它們：驗收是三張表 19,220 bytes 原樣全中。
	if n := differs8(orig, mine); n != 0 {
		t.Errorf("原版讀進去的盤面與 remake 讀進去的差 %d 個位元組%s\n%s",
			n, where(orig, mine, nMas, nSta),
			byPrefecture(orig, mine, nMas, nSta))
		// 差的欄位是哪些**值**：保留檔案原值，分辨 remake 解錯與原版
		// 載入時自己改動欄位。
		// 檔案裡的原始位元組也印一份：這樣分得出「remake 解錯」與
		// 「原版載入時自己改了它」。
		var fileSta []byte
		if sc, err := state.LoadScenario(c, state.Slot("SV1")); err == nil {
			_, fs, _ := sc.Tables()
			fileSta = fs
		}
		ob, mb := orig[nMas:nMas+nSta], mine[nMas:nMas+nSta]
		w := func(b []byte, at int) int { return int(b[at]) | int(b[at+1])<<8 }
		for q := 0; q < 43; q++ {
			a, b := ob[q*176:(q+1)*176], mb[q*176:(q+1)*176]
			same := true
			for _, off := range []int{16, 17, 22, 23, 30, 32, 33} {
				if a[off] != b[off] {
					same = false
				}
			}
			if same {
				continue
			}
			f := a
			if len(fileSta) == nSta {
				f = fileSta[q*176 : (q+1)*176]
			}
			t.Logf("    郡 %2d：兵士(百) %4d／%4d（檔 %4d）　"+
				"在職將 %2d／%2d（檔 %2d）　所屬 %3d／%3d（檔 %3d）　"+
				"主事者 %5d／%5d（檔 %5d）",
				q, w(a, 16), w(b, 16), w(f, 16), a[22], b[22], f[22],
				a[30], b[30], f[30], w(a, 32), w(b, 32), w(f, 32))
		}
	}
}
