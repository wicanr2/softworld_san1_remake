//go:build oracle

package parity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"golang.org/x/text/encoding/traditionalchinese"
)

// decodeBig5 用 cp950 解碼。**不要用 big5**（`CLAUDE.md` §3.2）。
func decodeBig5(b []byte) (string, error) {
	out, err := traditionalchinese.Big5.NewDecoder().Bytes(b)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// plusRoot 是加強版的素材目錄。
func plusRoot(t *testing.T) string {
	t.Helper()
	d := os.Getenv("SAN1_ORIG")
	if d == "" {
		t.Skip("沒設 SAN1_ORIG，跳過對拍（本儲存庫不含原版檔案）")
	}
	return filepath.Join(d, "三國演義1加強版")
}

// TestZZBootPlus 把加強版跑到標題畫面。
//
// **兩版的差異只剩程式碼**（`docs/mechanics/90` §5）——`DATA2` 逐項比過
// 沒有實質差異，`NAME00n.SHA` 也排除了。而兩個 `DATA5.GRP` 都是打包過
// 的，靜態抽字串只會拿到雜訊（量過：原版 3,275 條、加強版 2,356 條，
// 沒有一條可讀）。**所以要比程式碼，得先把加強版跑起來。**
//
// 裝置選單的答案與原版相同（`"122"`），可以用 `SAN1_PLUSKEY` 換。
// 設了 `SAN1_DUMP` 就把碼段倒出來。
//
// ⚠ **一度以為是加強版載不進來**：它在一個檔案都沒開的情況下就
// `run-time error R6005 - not enough memory on exec`。真正的原因是
// `DATA0.GRP` 的 `e_cblp` 有一格垃圾（`0xAA90`），而 dosgolem 沒有
// 遮成九位——**載入器的訊息說「檔案太短」，方向完全相反**。
// 修在 `dosgolem/internal/machine/loader.go`。
func TestZZBootPlus(t *testing.T) {
	root := plusRoot(t)
	o, err := oracle.Load(filepath.Join(root, "ASV.EXE"), root)
	if err != nil {
		t.Fatalf("加強版載不進來：%v", err)
	}
	defer o.Close()

	// **接新程式的第一件事是問它缺什麼服務**（`CLAUDE.md` §4.1）。
	o.TraceFiles()
	// 原版的裝置選單要先答三題（`loadOriginal` 送 "122"）；
	// 加強版如果也問，先把答案排進鍵盤佇列，不然它會停在選單上。
	o.Type(envOr("SAN1_PLUSKEY", "122"))
	const budget = 200_000_000
	err = o.Run(budget)
	t.Logf("跑了 %d 步之後：%v", o.Steps(), err)
	if u := o.Unimplemented(); len(u) > 0 {
		t.Logf("用到而還沒實作的服務：%v", u)
	} else {
		t.Log("沒有用到未實作的服務")
	}
	for i, f := range o.FileOps() {
		if i >= 12 {
			t.Logf("……還有 %d 次檔案操作", len(o.FileOps())-12)
			break
		}
		t.Logf("檔案操作 %d：%+v", i+1, f)
	}
	dumpScreen(t, o, "plus-boot")
	t.Logf("主控台：%q", o.Console())
	// 掃記憶體裡的 Big5 字串：解開之後才看得到選單與提示。
	n, sample := countBig5(o.Bytes(addr(0x00b000), 0x45000))
	t.Logf("記憶體裡可讀的中文字串 %d 條；前幾條：%q", n, sample)
	// 走到標題之後再往前推：原版是 `\r` → `2`（載入舊進度）→ `1`，
	// 加強版的選單如果一樣，這一串就會把它帶到主畫面
	// （`bootToMain`，`docs/re/02` §3.3）。**推不動也不當失敗**——
	// 這一支的判準是開得起來，不是走得完。
	send := map[int]string{4: "\r", 17: "2", 23: "1"}
	for i := 0; i < 27; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Logf("第 %d 段停止：%v", i, err)
			break
		}
		if k, ok := send[i]; ok {
			o.Press(k)
		}
	}
	// 主畫面出來之後會問防拷密碼。原版量過**任何四位數都過得去**
	// （`CONTEXT.md` §1），加強版照送。
	o.Press("1234\r")
	for i := 0; i < 6; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Logf("送密碼之後第 %d 段停止：%v", i, err)
			break
		}
	}
	dumpScreen(t, o, "plus-main")

	ds := o.DSReg()
	t.Logf("DS ＝ %#06x（線性 %#07x）", ds, uint32(ds)<<4)
	dumpImage(t, o, 0x00b000, 0x050000, "plus-code")
	lo := uint32(ds) << 4
	dumpImage(t, o, lo, lo+0x10000, "plus-dgroup")
	// **DS 在哪一刻取到的不一定是主程式的 DGROUP**，所以另外倒一份
	// 寬一點的窗，讓字串掃描自己找。
	dumpImage(t, o, 0x00b000, 0x0a0000, "plus-wide")

	// **判準是它真的畫出了標題**：開檔數與畫面都要對得起來。
	// 只看「沒有錯誤」的話，卡在黑畫面也會綠。
	if n := len(o.FileOps()); n < 50 {
		t.Errorf("只做了 %d 次檔案操作——加強版沒走到載資料那一步", n)
	}
	if len(o.Search([]byte("DATA1.NAM"))) == 0 {
		t.Error("記憶體裡找不到 DATA1.NAM——容器沒載進來")
	}
}

// countBig5 數一段記憶體裡有幾條可讀的中文字串，並取前幾條當樣本。
//
// **逐字掃描不用 regex**：Big5 的第二個位元組含 `0x5C` 與 `0x7C`
// （`CLAUDE.md` §3.2）。
func countBig5(b []byte) (int, []string) {
	var out []string
	n, i := 0, 0
	for i < len(b) {
		j, start := i, i
		var buf []byte
		for j < len(b) {
			c := b[j]
			switch {
			case c >= 0xA1 && c <= 0xF9 && j+1 < len(b) &&
				((b[j+1] >= 0x40 && b[j+1] <= 0x7E) || (b[j+1] >= 0xA1 && b[j+1] <= 0xFE)):
				buf = append(buf, b[j], b[j+1])
				j += 2
			case c >= 0x20 && c < 0x7F:
				buf = append(buf, c)
				j++
			default:
				j = len(b) + 1
			}
			if j > len(b) {
				break
			}
		}
		_ = start
		if len(buf) >= 6 {
			if s, err := decodeBig5(buf); err == nil && hasHan(s) {
				n++
				if len(out) < 8 {
					out = append(out, s)
				}
			}
			i += len(buf)
			continue
		}
		i++
	}
	return n, out
}

func hasHan(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

// bootLikePlus 用**加強版那條序列**開機，回傳停下來時的 oracle。
//
// 這是版本比對的正對照。兩份 dump 若取自不同時刻，執行期被改寫過的
// 區段（旗標、遮罩、暫存盤面）也會進到差異裡，而那不是版本差異。
// 判準要成立，兩邊得走同一串鍵、同一批預算、停在同一個畫面。
func bootLikePlus(t *testing.T, o *oracle.Oracle) {
	t.Helper()
	o.Type(envOr("SAN1_PLUSKEY", "122"))
	if err := o.Run(200_000_000); err != nil {
		t.Logf("開機段停止：%v", err)
	}
	send := map[int]string{4: "\r", 17: "2", 23: "1"}
	for i := 0; i < 27; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Logf("第 %d 段停止：%v", i, err)
			break
		}
		if k, ok := send[i]; ok {
			o.Press(k)
		}
	}
	o.Press("1234\r")
	for i := 0; i < 6; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Logf("送密碼之後第 %d 段停止：%v", i, err)
			break
		}
	}
}

// TestZZDumpBaseAtBoot 把**原版**倒在與 `TestZZBootPlus` 相同的時刻。
//
// 有了這一份，DGROUP 的差異才分得開三種來源：
//
//	原版@開機 vs 原版@戰役中 → 執行期會被改寫的區段
//	原版@開機 vs 加強版@開機 → 版本差異
//
// 少了正對照，第一種會被算進第二種——而它們在 hex dump 裡長得一模一樣。
func TestZZDumpBaseAtBoot(t *testing.T) {
	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatalf("載入原版：%v", err)
	}
	defer o.Close()
	o.TraceFiles()
	bootLikePlus(t, o)
	dumpScreen(t, o, "base-boot-main")

	ds := o.DSReg()
	t.Logf("DS ＝ %#06x（線性 %#07x）", ds, uint32(ds)<<4)
	lo := uint32(ds) << 4
	dumpImage(t, o, lo, lo+0x10000, "baseboot-dgroup")
	dumpImage(t, o, 0x00b000, 0x050000, "baseboot-code")

	if n := len(o.FileOps()); n < 50 {
		t.Errorf("只做了 %d 次檔案操作——沒走到載資料那一步", n)
	}
}
