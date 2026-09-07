//go:build oracle

package parity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 州郡的所屬：原版載完進度之後記憶體裡的值 vs remake 從同一個存檔解出來的值。
//
// 起因是長沙（31）的填色對不上（`internal/ui` 的 `knownFillMismatch`）：
// 原版畫的是圖樣 0，而存檔裡的所屬是 2。要分清楚是 remake 讀錯欄位還是
// 原版畫錯顏色，只有原版自己的記憶體說得準。
//
// 結論：**長沙的所屬在原版記憶體裡也是 2**，remake 讀對了。
//
// 順手量到另一件事：原版的執行期表與存檔差兩格，而**那兩格的填色畫的是
// 存檔的值不是記憶體的值**——所以地圖是「載完就畫」，載入之後那五道提示
// （人數／君主／難度）改掉的東西不會重畫。拿執行期記憶體去對畫面上的
// 顏色會在這兩格上得到假的不符。

// loadedOwnerExceptions 是原版執行期表與存檔不同的兩個郡。
//
//	13 潁川：存檔 5、記憶體 4
//	41 南海：存檔無主、記憶體 14（玩家自創的君主）
//
// 兩格都在載入之後的提示裡被改掉，而地圖已經畫完了。
var loadedOwnerExceptions = map[int]bool{13: true, 41: true}

// TestLoadedPrefectureOwnersMatchTheSave 比原版載完進度之後記憶體裡的
// 所屬與 remake 從存檔解出來的所屬。
func TestLoadedPrefectureOwnersMatchTheSave(t *testing.T) {
	root := origRoot(t)
	c2 := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c2, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	g, err := save.ReadOriginal(c2, 1, state.EditionBase)
	if err != nil {
		t.Fatalf("remake 讀原版第一個進度：%v", err)
	}

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToMain(t, o, seedMas)
	staBase := base + uint32(state.MasterTableSize)

	// ⚠ **所屬是 `u8` 不是 `u16`**：無主是 `0xFF`。照 word 讀會得到
	// `0x00FF`，把它跟 `0xFFFF` 比就會把六個無主的郡全報成不符。
	const recLen = 176
	bad := 0
	for p := 1; p <= state.PrefectureCount; p++ {
		got := int(int8(o.Byte(addr(staBase + uint32(p*recLen+30)))))
		pr := g.Prefecture(p)
		if pr == nil {
			t.Errorf("郡 %d：remake 沒有這一格", p)
			bad++
			continue
		}
		want := -1
		if pr.Owner != state.NoFaction {
			want = int(pr.Owner)
		}
		if got == want {
			continue
		}
		if loadedOwnerExceptions[p] {
			t.Logf("（已知）郡 %d（%s）：原版記憶體 %d、存檔 %d",
				p, pr.Name, got, want)
			continue
		}
		t.Errorf("郡 %d（%s）：原版記憶體說所屬 %d，remake 從存檔解出 %d",
			p, pr.Name, got, want)
		bad++
	}
	t.Logf("%d 個郡，扣掉已知的兩格之後 %d 個對不上", state.PrefectureCount, bad)

	// 長沙那一格：所屬對得上，所以填色取的不是 offset 30。
	got := int(int8(o.Byte(addr(staBase + uint32(31*recLen+30)))))
	if got != 2 {
		t.Errorf("長沙的所屬在原版記憶體裡是 %d，不是 2——"+
			"`internal/ui` 的 knownFillMismatch 說明要重寫", got)
	}

	// 整份倒出來給離線分析用（`SAN1_DUMP` 指到輸出目錄）。
	if dir := os.Getenv("SAN1_DUMP"); dir != "" {
		all := make([]byte, (state.PrefectureCount+1)*recLen)
		for i := range all {
			all[i] = o.Byte(addr(staBase + uint32(i)))
		}
		_ = os.MkdirAll(dir, 0o755)
		f := filepath.Join(dir, "loaded-prefectures.bin")
		if err := os.WriteFile(f, all, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("州郡表已倒到 %s（%d bytes）", f, len(all))
	}
}
