//go:build oracle

package parity

import (
	"errors"
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
	// ⚠ **路徑是相對於這個套件的目錄**（`go test` 的工作目錄），
	// 不是儲存庫根。給 `workplace/rec7/frames` 會找不到而且只是 skip
	// ——看起來像「沒錄過」而不像路徑寫錯。
	dir := os.Getenv("SAN1_REC")
	if dir == "" {
		dir = "../../workplace/rec7/frames"
	}
	if !filepath.IsAbs(dir) && !strings.HasPrefix(dir, "..") {
		dir = filepath.Join("../..", dir)
	}
	shots, err := filepath.Glob(filepath.Join(dir, "*[0-9a-zA-Z].png"))
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
		file   string
		keys   string
		stable bool
	}
	var steps []step
	for _, f := range shots {
		b := strings.TrimSuffix(filepath.Base(f), ".png")
		if strings.HasSuffix(b, ".b") {
			continue
		}
		i := strings.IndexByte(b, '-')
		if i < 0 {
			continue
		}
		// **同一步存了兩張、隔一秒**：兩張一樣才表示畫面靜止。
		// 還在動的那一步不能當判準——動畫在兩個實作上不會停在同一格，
		// 而那不是誰做錯了。沒有第二張時當成不穩定。
		steps = append(steps, step{
			file:   f,
			keys:   b[i+1:],
			stable: nearlySame(f, strings.TrimSuffix(f, ".png")+".b.png", 400),
		})
	}
	t.Logf("參照畫面 %d 張，來源 %s", len(steps), dir)

	// **時間軸要換算，不能照抄。** DOSBox 跑的是 `cycles=fixed 60000`
	// ＝ 每秒 6,000 萬個週期；dosgolem 的時鐘是指令數。用「每秒約三千萬
	// 道指令」把腳本裡的秒數換成指令上限——寧可給多不要給少，
	// **給少了畫面會停在動畫中間，看起來像 dosgolem 少做了什麼**。
	const stepsPerSec = 30_000_000
	// menuStep 是第一個「按選單」的步驟（前面都是開場動畫）。
	const menuStep = 7
	// idleSteps 是「畫面連續多久沒變」才算停下來。取約 0.7 秒。
	const idleSteps = 20_000_000
	waits := loadWaits(filepath.Join(filepath.Dir(dir), "keys.txt"))
	// 順便驗語音：這一串按鍵會走到宣戰對白（`docs/re/09` 的訊息常式
	// `0x3273e`），那正是原版會講話的地方。**兩個開關要直接寫記憶體**
	// ——「其他」的子選單走另一條輸入路徑，送數字進去畫面不動。
	var speaks, says int
	o.OnCall(addr(speechSpeakFn), func(*oracle.Oracle) { speaks++ })
	o.OnCall(addr(speechSayFn), func(*oracle.Oracle) { says++ })

	var worst, worstAt, stableN int
	var reported int
	// **開場不能用時間對齊。** 它是計時動畫，兩邊的時鐘不同，
	// 照秒數換算走到第十步就在不同畫面上——實測 dosgolem 停在
	// 「建安二年」的密碼畫面，而原版在「中平六年」的主畫面。
	//
	// 路標改成程式行為：**主選單自己裝 `int 09h` 處理常式**
	//（`docs/re/02` §2），向量的段值一變就表示它上來了。
	stub := o.Word(oracle.Addr{Seg: 0, Off: 0x09*4 + 2})
	menuUp := oracle.NewCond("主選單裝好自己的 int 09h", func(o *oracle.Oracle) bool {
		return o.Word(oracle.Addr{Seg: 0, Off: 0x09*4 + 2}) != stub
	})

	for n, s := range steps {
		// **裝置三題走 int 21h，其餘走硬體掃描碼**（`docs/re/02` §3）。
		//
		// 防拷密碼那一關兩條路都要餵（它不吃掃描碼），所以之後每一步
		// 兩條都送——**但每一步先 Drain**：沒清乾淨的字元副本會把下一個
		// `int 21h` 的提示堵死，而那看起來像「按了沒反應」。
		k := strings.ReplaceAll(s.keys, "Return", "\r")
		o.Drain()
		switch {
		case n < 3:
			o.Press(k)
		case isPassword(k):
			// 防拷密碼那一關**不吃掃描碼**（`docs/re/02` §3.3），走字元。
			o.Press(k)
		default:
			o.PressScan(k)
		}
		// **對齊的是「畫面靜止」不是時間。** 兩邊的時鐘不同（DOSBox 用
		// 真實秒數、dosgolem 用指令數），而錄製那一側每一步還多花一秒
		// 拍第二張——照秒數換算的預算會讓 dosgolem 永遠落後一點，
		// 而落後的畫面看起來像「dosgolem 少畫了東西」。
		budget := uint64(10) * stepsPerSec
		if n < len(waits) {
			budget = uint64(waits[n]+2) * stepsPerSec
		}
		// **先跑滿預算，再等靜止。** 只等靜止是不夠的：開場是好幾段動畫
		// 接起來的，第一次停格就回來的話，下一個鍵會送在還沒走完的畫面上
		// ——之後每一步都對不上，而每一步看起來都「靜止」。
		if n == menuStep-1 {
			// 走到主選單為止，不管花多少指令。
			if err := o.RunUntil(menuUp, oracle.Budget(20*stepsPerSec)); err != nil {
				t.Fatalf("等主選單時停止：%v", err)
			}
			t.Logf("主選單在第 %d 道指令上來（int 09h 段 %04X → %04X）",
				o.Steps(), stub, o.Word(oracle.Addr{Seg: 0, Off: 0x09*4 + 2}))
		} else if err := o.Run(budget); err != nil {
			t.Fatalf("第 %d 步（%s）停止：%v", n+1, s.keys, err)
		}
		err := o.RunUntil(oracle.ScreenIdle(idleSteps), oracle.Budget(4*idleSteps))
		var be *oracle.BudgetError
		if err != nil && !errors.As(err, &be) {
			t.Fatalf("第 %d 步（%s）等靜止時停止：%v", n+1, s.keys, err)
		}
		settled := err == nil
		// 進了遊戲之後每一步都把兩個開關寫開（冪等）。
		if n >= 14 {
			work := o.ES()
			o.SetWord(oracle.Addr{Seg: work, Off: sfxStateOff}, 0)
			o.SetWord(oracle.Addr{Seg: work, Off: voiceStateOff}, 0)
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
		// **前三步是文字模式**（裝置三題），兩邊都不是 EGA 平面畫面，
		// 拿 `IndexedEGA` 解出來的東西沒有意義——不列入最大差異。
		if n >= 3 && s.stable && settled && bad > worst {
			worst, worstAt = bad, n+1
			stableN++
		}
		pct := 100 * float64(scrW*scrH-bad) / float64(scrW*scrH)
		mark := "動畫中"
		if s.stable && settled {
			mark = "靜止"
		} else if s.stable {
			mark = "原版靜止／dosgolem 還在動"
		} else if settled {
			mark = "dosgolem 靜止／原版還在動"
		}
		if bad > 0 && reported < 26 {
			reported++
			t.Logf("第 %2d 步 送 %-12s %s 差 %6d 點（%.2f%% 相同）",
				n+1, s.keys, mark, bad, pct)
		}
		dumpScreen(t, o, fmt.Sprintf("seq-%03d", n+1))
	}
	t.Logf("語音：訊息常式進去 %d 次、speak %d 次、喇叭切換 %d 次",
		says, speaks, len(o.Speaker()))
	if stableN == 0 {
		t.Logf("沒有任何一步是靜止的——參照畫面是舊版錄的（一步只存一張），" +
			"重跑 tools/dosboxx-record.sh 才有靜止判斷")
		return
	}
	t.Logf("靜止的步驟裡最大差異在第 %d 步：%d 點（%.2f%%）",
		worstAt, worst, 100*float64(scrW*scrH-worst)/float64(scrW*scrH))
}

// loadWaits 讀按鍵腳本裡每一步的等待秒數。
// isPassword 認出防拷密碼那一步：四個數字加 Enter。
//
// **兩條輸入路徑不能一起餵。** 讀掃描碼的提示同時收到字元副本時，
// 一個「1」會被算成兩次——「有幾人玩」那一格就從 1 個玩家變成 11 個，
// 之後每一步都在問「第 N 位」，畫面一直在動，看起來像 dosgolem 走錯了。
func isPassword(k string) bool {
	if len(k) != 5 || k[4] != '\r' {
		return false
	}
	for i := 0; i < 4; i++ {
		if k[i] < '0' || k[i] > '9' {
			return false
		}
	}
	return true
}

// nearlySame 比兩張圖，差異在 tol 點以內就算「同一格畫面」。
//
// ⚠ **不能要求逐位元組相同。** 提示列尾巴那個游標一直在閃，
// 隔一秒拍兩張永遠不會完全一樣——照那個標準判，**每一步都是「動畫中」**，
// 判準就整個空掉了。容差取得住游標與小飾框的動畫（各百來點）即可。
func nearlySame(a, b string, tol int) bool {
	x, err := decodePNG(a)
	if err != nil {
		return false
	}
	y, err := decodePNG(b)
	if err != nil {
		return false
	}
	n := 0
	for py := 0; py < scrH; py++ {
		for px := 0; px < scrW; px++ {
			if color.RGBAModel.Convert(x.At(px, py)) !=
				color.RGBAModel.Convert(y.At(px, py)) {
				n++
				if n > tol {
					return false
				}
			}
		}
	}
	return true
}

func decodePNG(path string) (interface {
	At(x, y int) color.Color
}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return imgpng.Decode(f)
}

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
