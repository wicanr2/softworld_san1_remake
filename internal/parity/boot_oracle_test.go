//go:build oracle

package parity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/boot"
)

// origRoot 回傳原版目錄；沒設就 skip。
func origRoot(t *testing.T) string {
	t.Helper()
	d := os.Getenv("SAN1_ORIG")
	if d == "" {
		t.Skip("沒設 SAN1_ORIG，跳過對拍（本儲存庫不含原版檔案）")
	}
	return filepath.Join(d, "三國演義")
}

// loadOriginal 把原版載進 dosgolem 並跑到裝置選單問完為止。
//
// keys 是要送的按鍵。回傳 oracle 讓呼叫端繼續問它問題。
func loadOriginal(t *testing.T, keys string) *oracle.Oracle {
	t.Helper()
	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatalf("載入原版：%v", err)
	}
	o.Type(keys)
	// 跑到它開第一個檔為止。開檔代表選單答完、進入載資料階段。
	if err := o.RunUntil(oracle.Opened("DATA0.GRP"), oracle.Budget(30_000_000)); err != nil {
		t.Fatalf("原版沒跑到開檔：%v\n主控台：%q", err, o.Console())
	}
	return o
}

// TestBootPromptsMatchOriginal 對拍啟動裝置選單的題目與選項。
//
// 判準是**原版自己的記憶體裡有沒有這些字串**，不是主控台輸出。
//
// ⚠ 第一版拿主控台比，結果是假的：原版的輸出是**緩衝**的，開檔那一刻
// 主控台只有三個回顯字元（"122"），提示文字要更後面才被沖出來。
// 拿「還沒出現」當成「不一致」會得到相反的結論。
// 記憶體裡的字串在映像載入的那一刻就在了，不受輸出時機影響。
func TestBootPromptsMatchOriginal(t *testing.T) {
	o := loadOriginal(t, "122")

	seen := func(needle string) bool { return len(o.Search([]byte(needle))) > 0 }

	for _, p := range boot.Prompts() {
		if !seen(p.Label) {
			t.Errorf("原版的記憶體裡沒有題目標籤 %q——remake 多了一題，或標籤寫錯", p.Label)
		}
		for _, ch := range p.Choices {
			if !seen(ch.Name) {
				t.Errorf("原版的記憶體裡沒有 %s 的選項 %q——remake 編了一個原版沒有的選項",
					p.Label, ch.Name)
			}
		}
	}

	// 反向：確認這個判準抓得到假的東西，否則上面全過也證明不了什麼
	// （正對照——落空 ≠ 不存在，先證明搜尋真的會落空）。
	if seen("SoundBlaster") {
		t.Error("原版居然有 SoundBlaster——這個反向對照要換一個字串")
	}
}

// TestBootAcceptsOnlyOneAndTwo 對拍「只收 1 或 2」。
//
// remake 的 Config.Set 拒絕其他按鍵。這裡驗證原版也是——
// 送一堆別的鍵進去，原版應該**不前進**（不會開檔）。
func TestBootAcceptsOnlyOneAndTwo(t *testing.T) {
	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatalf("載入原版：%v", err)
	}
	// 送 '3'..'9'、字母、Enter——原版該一題都不過。
	o.Type("3456789abcXYZ\r\n")
	err = o.RunUntil(oracle.Opened("DATA0.GRP"), oracle.Budget(20_000_000))
	if err == nil {
		t.Fatal("原版收了 1/2 以外的按鍵就往下走了——" +
			"那 remake 的 Config.Set 不該拒絕它們")
	}
	// 對稱地驗 remake 這一邊。
	c := boot.NewConfig()
	for _, k := range []byte("3456789abcXYZ\r\n") {
		if err := c.Set(boot.Music, k); err == nil {
			t.Errorf("remake 收了按鍵 %q，但原版不收", k)
		}
	}
	for _, k := range []byte{'1', '2'} {
		if err := c.Set(boot.Music, k); err != nil {
			t.Errorf("remake 拒絕了按鍵 %q，但原版收：%v", k, err)
		}
	}
}

// TestKeySequenceDrivesOriginal 對拍按鍵序列。
//
// remake 的 Config.KeySequence() 產生的字串，送進原版之後原版應該
// **確實把三題答完並開始載資料**。這一條把 remake 的設定模型與原版的
// 實際流程綁在一起——序列錯了原版就走不到開檔。
func TestKeySequenceDrivesOriginal(t *testing.T) {
	c := boot.NewConfig()
	must := func(d boot.Device, k byte) {
		t.Helper()
		if err := c.Set(d, k); err != nil {
			t.Fatal(err)
		}
	}
	must(boot.Music, '1')   // No
	must(boot.Graphic, '2') // EGA
	must(boot.Disk, '2')    // HardDisk
	if !c.Complete() {
		t.Fatal("三題都選了卻說沒選完")
	}
	seq, err := c.KeySequence()
	if err != nil {
		t.Fatal(err)
	}
	if seq != "122" {
		t.Fatalf("序列 ＝ %q，想要 %q", seq, "122")
	}

	o := loadOriginal(t, seq)
	// 原版真的走到載資料了。
	opened := o.Opened()
	if len(opened) == 0 {
		t.Fatal("原版沒開任何檔")
	}
	// 而且回顯了三個選擇——每題一個字元。
	console := o.Console()
	if !strings.HasPrefix(console, seq) {
		t.Errorf("原版回顯 %q，想要以 %q 開頭", firstLine(console), seq)
	}

	// 顯示卡選 EGA，remake 就該用 EGA 的調色盤。
	pal, err := c.Palette()
	if err != nil {
		t.Fatal(err)
	}
	if pal != "EGAFILL.PAL" {
		t.Errorf("調色盤 ＝ %q，想要 EGAFILL.PAL", pal)
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
