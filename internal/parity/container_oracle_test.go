//go:build oracle

package parity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// TestContainerOffsetsMatchOriginal 是這個專案最硬的一條對拍。
//
// remake 的解碼器用 `.IDX` 的結束位移算每一項的邊界
// （起點 ＝ IDX[i-1]、長度 ＝ IDX[i] − IDX[i-1]，`docs/formats/01`）。
// 那是從檔案結構推出來的，**推得對不對只有原版知道**。
//
// 這裡讓原版自己跑，攔它對 `DATA1.GRP` 的 seek：**它 seek 到哪個位移，
// 就是它自己算出來的項目起點**。兩條路徑算同一件事，逐項比。
//
// 讀取量不能拿來比——原版按 512 byte 磁區塊讀，與項目長度無關
// （8 bytes 的 `MUSV.IDX` 它照樣要求 512）。
//
// 靜態的 round-trip（末值 ＝ 檔長、無縫覆蓋）只證明解碼器自洽；
// 這一條證明它算的**就是原版拿來用的那組數字**。
func TestContainerOffsetsMatchOriginal(t *testing.T) {
	root := origRoot(t)

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatalf("載入原版：%v", err)
	}
	o.TraceFiles()
	o.Type("122") // 無音樂／EGA／硬碟——第三題是走到載資料的分岔點
	// 跑到它讀完資料為止；預算給足，讀不到就讓它自己說。
	if err := o.Run(60_000_000); err != nil {
		t.Logf("原版停止：%v", err) // 停在自己的錯誤是預期的，不影響這一條
	}

	// remake 這一邊：解同一個容器。
	c := openContainer(t, filepath.Join(root, "DATA1"))
	starts := map[int64]assets.Entry{} // 起點 → 項目
	for i := 0; i < c.Len(); i++ {
		e := c.Entry(i)
		starts[int64(e.Start)] = e
	}

	// 原版對 DATA1.GRP 的 (seek, read) 配對。
	var lastSeek int64 = -1
	checked := 0
	for _, op := range o.FileOps() {
		if op.Name != "DATA1.GRP" {
			continue
		}
		switch op.Op {
		case "seek":
			// dosgolem 的 `FileOp.Result` 在 main 上拆成兩欄：`Pos`（seek 後的
			// 位置／read 的起點）與 `Len`（實際的位元組數）。
			lastSeek = op.Pos
		case "read":
			if lastSeek < 0 {
				continue
			}
			e, ok := starts[lastSeek]
			if !ok {
				t.Errorf("原版 seek 到 %d 然後讀 %d bytes，"+
					"但 remake 的解碼器裡沒有從 %d 開始的項目",
					lastSeek, op.Arg, lastSeek)
				lastSeek = -1
				continue
			}
			// ⚠ **不要拿「讀取量 ≤ 項目長度」當判準。** 原版是按
			// 512 byte 磁區塊讀的，與項目長度無關：8 bytes 的
			// MUSV.IDX 它照樣要求 512。第一版這樣寫，把原版正常的
			// 過讀報成「remake 把邊界算小了」。
			//
			// 真正的判準是**起點**：seek 的位移就是原版自己算出來的
			// 項目起點，那個數字對上了才代表兩邊算的是同一件事。
			if op.Arg%512 != 0 {
				t.Errorf("原版讀 %d bytes，不是 512 的倍數——"+
					"「它按磁區塊讀」這個認識要修正", op.Arg)
			}
			t.Logf("✓ 原版 seek %d 讀 %d → remake 說那是 %q（%d..%d，長 %d）",
				lastSeek, op.Arg, e.Name, e.Start, e.End, e.Size())
			checked++
			lastSeek = -1
		}
	}

	// **正對照**：一次都沒對到的話，上面的迴圈全過也證明不了任何事。
	if checked == 0 {
		t.Fatal("原版一次都沒有對 DATA1.GRP 做 seek+read——" +
			"這條對拍沒有實際比對到東西，不能算通過")
	}
	t.Logf("共比對 %d 組 (seek, read)", checked)
}

// TestContainerHeaderReadsAreComplete 驗原版把 .IDX 與 .NAM 整份讀進去。
//
// 如果原版只讀一部分，remake 「整份解析」的做法就與它不同，
// 那會在項數上分歧。
func TestContainerHeaderReadsAreComplete(t *testing.T) {
	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatalf("載入原版：%v", err)
	}
	o.TraceFiles()
	o.Type("122")
	if err := o.Run(60_000_000); err != nil {
		t.Logf("原版停止：%v", err)
	}

	for _, want := range []struct {
		name string
		size int64
	}{
		{"DATA1.IDX", 868},
		{"DATA1.NAM", 3472},
	} {
		var got int64
		seen := false
		for _, op := range o.FileOps() {
			if op.Name != want.name {
				continue
			}
			seen = true
			if op.Op == "read" {
				got += int64(op.Len) // 實際讀到的量（舊欄位 `Result`）
			}
		}
		if !seen {
			t.Errorf("原版沒有讀 %s", want.name)
			continue
		}
		if got != want.size {
			t.Errorf("原版從 %s 讀了 %d bytes，檔案是 %d bytes——"+
				"它不是整份讀進去的，remake 的做法要跟著改", want.name, got, want.size)
		}
	}
}

// openContainer 開一組容器。
func openContainer(t *testing.T, base string) *assets.Container {
	t.Helper()
	rd := func(ext string) []byte {
		b, err := os.ReadFile(base + ext)
		if err != nil {
			t.Fatalf("讀 %s%s：%v", base, ext, err)
		}
		return b
	}
	c, err := assets.OpenContainer(rd(".NAM"), rd(".IDX"), rd(".GRP"))
	if err != nil {
		t.Fatalf("OpenContainer：%v", err)
	}
	return c
}
