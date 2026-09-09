//go:build oracle

package parity

import (
	"fmt"
	"image/color"
	imgpng "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// 拿 DOSBox-X 錄下來的同一串按鍵驗 dosgolem 的完整性。
//
// **這一支問的不是「remake 對不對」，是「執行器夠不夠」**：同一支原版、
// 同一串按鍵，兩個獨立實作走到的畫面該一樣。不一樣的地方就是 dosgolem
// 還沒做到的東西——而那種缺口在單獨跑 dosgolem 時**看不出來**，
// 畫面照樣有東西、程式照樣往前走。
//
// 參照畫面由 `tools/dosboxx-record.sh` 產：`workplace/rec*/frames/`
// 底下一步一張 640×350 的 PNG，檔名是 `<序號>-<那一步送的鍵>.png`。
//
// ⚠ **時間軸不能照抄。** DOSBox 用真實秒數，dosgolem 用指令數；
// 對齊點要是**畫面靜止**不是「第幾秒」。
func TestZZDosgolemMatchesDosbox(t *testing.T) {
	dir := os.Getenv("SAN1_REC")
	if dir == "" {
		dir = "../../workplace/rec4/frames"
	}
	shots, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil || len(shots) == 0 {
		t.Skipf("沒有 DOSBox 參照畫面 %s（跑 tools/dosboxx-record.sh 產）", dir)
	}
	sort.Strings(shots)

	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	// 每一張參照畫面的檔名帶著那一步送了什麼鍵。
	type step struct {
		file string
		keys string
	}
	var steps []step
	for _, f := range shots {
		b := strings.TrimSuffix(filepath.Base(f), ".png")
		i := strings.IndexByte(b, '-')
		if i < 0 {
			continue
		}
		steps = append(steps, step{file: f, keys: b[i+1:]})
	}
	t.Logf("參照畫面 %d 張，來源 %s", len(steps), dir)

	// **時間軸要換算，不能照抄。** DOSBox 跑的是 `cycles=fixed 60000`
	// ＝ 每秒 6,000 萬個週期；dosgolem 的時鐘是指令數。用「每秒約三千萬
	// 道指令」把腳本裡的秒數換成指令上限——寧可給多不要給少，
	// **給少了畫面會停在動畫中間，看起來像 dosgolem 少做了什麼**。
	const stepsPerSec = 30_000_000
	waits := loadWaits(filepath.Join(filepath.Dir(dir), "keys.txt"))
	var worst, worstAt int
	var reported int
	for n, s := range steps {
		// **裝置三題走 int 21h，其餘走硬體掃描碼**（`docs/re/02` §3）。
		//
		// 防拷密碼那一關兩條路都要餵（它不吃掃描碼），所以之後每一步
		// 兩條都送——**但每一步先 Drain**：沒清乾淨的字元副本會把下一個
		// `int 21h` 的提示堵死，而那看起來像「按了沒反應」。
		k := strings.ReplaceAll(s.keys, "Return", "\r")
		o.Drain()
		if n < 3 {
			o.Press(k)
		} else {
			o.PressScan(k)
			o.Press(k)
		}
		budget := uint64(10) * stepsPerSec
		if n < len(waits) {
			budget = uint64(waits[n]) * stepsPerSec
		}
		if err := o.Run(budget); err != nil {
			t.Fatalf("第 %d 步（%s）停止：%v", n+1, s.keys, err)
		}
		want := loadShot(t, s.file)
		got := o.IndexedEGA(scrW, scrH)
		if len(got) < scrW*scrH {
			t.Fatalf("第 %d 步畫面只有 %d 個像素", n+1, len(got))
		}
		bad := 0
		for y := 0; y < scrH; y++ {
			for x := 0; x < scrW; x++ {
				a := assets.EGAPalette[got[y*scrW+x]&15]
				b := color.RGBAModel.Convert(want.At(x, y)).(color.RGBA)
				if a != b {
					bad++
				}
			}
		}
		if bad > worst {
			worst, worstAt = bad, n+1
		}
		pct := 100 * float64(scrW*scrH-bad) / float64(scrW*scrH)
		if bad > 0 && reported < 20 {
			reported++
			t.Logf("第 %2d 步 送 %-12s 差 %6d 點（%.2f%% 相同）",
				n+1, s.keys, bad, pct)
		}
		dumpScreen(t, o, fmt.Sprintf("seq-%03d", n+1))
	}
	t.Logf("最大差異在第 %d 步：%d 點（%.2f%%）",
		worstAt, worst, 100*float64(scrW*scrH-worst)/float64(scrW*scrH))
}

// loadWaits 讀按鍵腳本裡每一步的等待秒數。
func loadWaits(path string) []int {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []int
	for _, line := range strings.Split(string(b), "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(f[0], "%d", &n); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func loadShot(t *testing.T, path string) interface {
	At(x, y int) color.Color
} {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	im, err := imgpng.Decode(f)
	if err != nil {
		t.Fatalf("解 %s：%v", path, err)
	}
	return im
}
