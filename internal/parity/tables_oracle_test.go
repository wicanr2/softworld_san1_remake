//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 對拍的盤面由自己擺。
//
// 要問的是「**同一個局面下**原版怎麼決定」，所以局面得由對拍這一方寫進
// 原版的記憶體，不能靠原版自己的 `RND()` 湊出來——亂數帶著自己的種子與
// 呼叫次數，跑兩次不見得一樣，而且它一動整張盤面都會變，比出來的差異就
// 分不出是「決策不同」還是「盤面不同」。
//
// 三張表就是完整的盤面，所以「同一個局面」這件事**可以驗證**：
// 寫進去再讀回來，兩邊的位元組要一樣。

// tableBase 是原版把三張表擺在哪裡（線性位址）。
//
// 不寫死：每次都拿劇本檔的位元組去搜。位址會隨版本漂移，搜出來的不會。
const (
	// bootSteps 是跑到三張表載進記憶體要幾道指令。
	// 量出來的：1,250M 之後諸侯表與人物表就搜得到了。
	bootSteps = 1_300_000_000
	chunk     = 50_000_000
)

// loadScenarioInOriginal 讓原版載入劇本 001 並停在盤面已經在記憶體的時候。
//
//	122   無音樂／EGA／硬碟（開機三題走 int 21h 讀 handle 0）
//	\r    跳過標題
//	1     開始新遊戲       ← 這之後改看掃描碼，所以一律用 Press
//	1     中平六年（劇本 001）
func loadScenarioInOriginal(t *testing.T) *oracle.Oracle {
	t.Helper()
	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatalf("載入原版：%v", err)
	}
	o.TypeBoth("122")
	send := map[int]string{4: "\r", 17: "1", 23: "1"}
	for i := 0; i < bootSteps/chunk; i++ {
		if err := o.Run(chunk); err != nil {
			o.Close()
			t.Fatalf("原版停止：%v", err)
		}
		if k, ok := send[i]; ok {
			o.TypeBoth(k)
		}
	}
	return o
}

// addr 把線性位址換成執行期位址。
func addr(lin uint32) oracle.Addr {
	return oracle.Addr{Seg: uint16(lin >> 4), Off: uint16(lin & 0xF)}
}

// TestScenarioTablesLiveInMemory 找出原版把三張表放在哪裡，並釘住它們
// 與劇本檔相同。
//
// 這是「擺盤面」的前提：位址不知道就沒得寫，而**位址要用搜的不能寫死**
// ——它會隨版本漂移，劇本檔的位元組不會。
//
// 三張表在記憶體裡是**連續**的，順序與檔名相同：諸侯 1,152、州郡 7,568、
// 人物 10,500，合計 19,220 個位元組。
func TestScenarioTablesLiveInMemory(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, sta, gen := sc.Tables()

	o := loadScenarioInOriginal(t)
	defer o.Close()

	hits := o.Search(mas[:48])
	if len(hits) != 1 {
		t.Fatalf("諸侯表在記憶體裡找到 %d 個位置 %v，應該只有一個", len(hits), hits)
	}
	base := hits[0]
	t.Logf("三張表的基底 %#x", base)

	// 人物表要接在諸侯表 ＋ 州郡表之後——連續是「一次寫完整個盤面」的前提。
	want := base + uint32(len(mas)) + uint32(len(sta))
	if g := o.Search(gen[:48]); len(g) != 1 || g[0] != want {
		t.Fatalf("人物表在 %v，照連續排法應該在 %#x", g, want)
	}

	// 州郡表比對得寬一點：原版載入時會往裡面寫算出來的欄位——差的
	// 43 個位元組落在每筆記錄的位移 29（物價），那是它自己算的，
	// 劇本檔裡是 0。
	got := o.Bytes(addr(base+uint32(len(mas))), len(sta))
	diff := 0
	for i := range sta {
		if got[i] != sta[i] {
			diff++
		}
	}
	t.Logf("州郡表 %#x 起 %d 個位元組，與檔案差 %d 個",
		base+uint32(len(mas)), len(sta), diff)
	if diff > len(sta)/100 {
		t.Errorf("州郡表與檔案差 %d 個位元組（%d 個裡），差太多，"+
			"可能根本不是同一張表", diff, len(sta))
	}
}

// TestPlantedBoardReadsBack 釘住盤面**寫得進去也讀得回來**。
//
// 這是對拍的地基：一場「同一個局面下的比較」只有在局面真的一樣的時候
// 才有意義，而「真的一樣」是這一條驗的——不是假設的。
//
// 擺的是 remake 自己造的盤面（劇本 001 走過幾個月），不是原版跑出來的。
func TestPlantedBoardReadsBack(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, sta, gen := sc.Tables()

	// 把盤面改到與劇本不同：每個郡多 1,000 金，才看得出真的是我們寫的。
	const goldOff = 18 // 州郡表裡「金」的位移（`docs/formats/03`）
	for i := 0; i+goldOff+2 <= len(sta); i += state.PrefectureRecordSize {
		v := uint16(sta[i+goldOff]) | uint16(sta[i+goldOff+1])<<8
		v += 1000
		sta[i+goldOff] = byte(v)
		sta[i+goldOff+1] = byte(v >> 8)
	}

	o := loadScenarioInOriginal(t)
	defer o.Close()

	hits := o.Search(mas[:48])
	if len(hits) != 1 {
		t.Fatalf("找不到諸侯表：%v", hits)
	}
	base := hits[0]

	off := uint32(0)
	for _, x := range [][]byte{mas, sta, gen} {
		o.SetBytes(addr(base+off), x)
		off += uint32(len(x))
	}

	off = 0
	for _, x := range []struct {
		name string
		b    []byte
	}{{"諸侯", mas}, {"州郡", sta}, {"人物", gen}} {
		got := o.Bytes(addr(base+off), len(x.b))
		for i := range x.b {
			if got[i] != x.b[i] {
				t.Fatalf("%s表寫進去讀回來對不上：第 %d 個位元組是 %#02x，寫的是 %#02x",
					x.name, i, got[i], x.b[i])
			}
		}
		t.Logf("%s表：%d 個位元組寫進 %#x，讀回來完全相同",
			x.name, len(x.b), base+off)
		off += uint32(len(x.b))
	}
}
