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

	// **開場既不比畫面，也不照秒數對齊。** 兩邊的時鐘差二十倍：DOSBox 是
	// `cycles=fixed 60000`（每秒六千萬道指令），dosgolem 是每個 IRQ0 之間
	// 165,000 道（每秒約三百萬道）。照秒數換算的預算會停在動畫中間，
	// 而**每一格看起來都靜止**——數字只是低，不像走錯了路。
	//
	// 開場照 `bootToMain` 已經驗過的配方走：**整段開場只送一個 Enter**，
	// 其餘的畫面 dosgolem 自己會走完。腳本裡那四個 Enter 是 DOSBox 那一側
	// 的節奏，照抄會多送三個——多餘的鍵留在佇列裡會堵住後面的讀取
	//（`docs/re/02` §3）。
	//
	// 路標是**主選單自己載的圖**：`MENU0A`／`MENU0B`／`MENU1`／`MENU2`／
	// `MENU3`，最後一張進來就表示選單要畫了（`TestZZOpeningAssetNames`
	// 量到在第 281,782,097 道指令）。
	//
	// ⚠ 兩個不能拿來當路標的東西：
	//   - **`int 09h` 的向量**。換掉它的是載入器，第 366M 道指令就換完又
	//     還原（0080 → 0B01 → 0080），那時畫面還在「程式載入中　請稍待」。
	//   - **跑固定的大預算**。主選單閒置久了會**掉回開場循環**——實測跑滿
	//     1.11G 道指令之後畫面退回三英那張，而那看起來像「開場沒走完」。
	var menuDrawn bool
	o.OnCall(addr(imgLoadFn), func(o *oracle.Oracle) {
		if cstr(o, uint32(o.Arg(1))*16+uint32(o.Arg(0))) == "MENU3.IMG" {
			menuDrawn = true
		}
	})
	menuUp := oracle.NewCond("主選單載進 MENU3.IMG", func(*oracle.Oracle) bool {
		return menuDrawn
	})

	// openSteps：開場佔掉的參照畫面數（裝置三題 ＋ 四個續行 Enter）。
	// 前三張是**文字模式**，拿 `IndexedEGA` 解平面沒有意義；中間三張是
	// 計時動畫。第 openSteps 張是主選單，從它開始比。
	const openSteps = 7
	// scanFrom：從第幾步（含）改送硬體掃描碼。裝置三題、開場續行、主選單
	// 與年代讀的都是**字元**；「請問有幾人玩」開始的提示才讀掃描碼
	//（`docs/re/02` §3.1）。送錯路那一步畫面不會動，而**下一步的鍵會落在
	// 上一個畫面上**——之後每一步都錯，卻每一步都「靜止」。
	const scanFrom = 10
	// idleSteps：畫面連續這麼多道指令沒變就算停下來（約兩個模擬秒）。
	// 提示列的游標會閃，多數步驟等不到真正的靜止而跑滿預算——那是慢不是錯。
	const idleSteps = 6_000_000
	// openBudget：從裝置三題答完到主選單畫出來的上限（實測約 282M 道）。
	const openBudget = 1_000_000_000
	// drawBudget：路標到畫完之間留的餘裕。**不能給大**——主選單閒置久了
	// 會掉回開場循環。
	const drawBudget = 20_000_000
	// openChunk：開場每次補一個 Enter 之間跑多少。
	const openChunk = 60_000_000
	// keyGap：**同一步裡兩個鍵之間要留空隙。** 錄製腳本每個鍵隔 0.15 秒，
	// DOSBox 那一側等於九百萬道指令；連著送的話，畫動畫的那幾步會把
	// 第二個鍵吃掉——實測宣戰那一段 dosgolem 從此少一個 Enter，
	// 落後原版兩張對白，而畫面照樣靜止、照樣往前走。
	const keyGap = 6_000_000

	// 順便驗語音：這一串按鍵會走到宣戰對白（`docs/re/09` 的訊息常式
	// `0x3273e`），那正是原版會講話的地方。**兩個開關要直接寫記憶體**
	// ——「其他」的子選單走另一條輸入路徑，送數字進去畫面不動。
	var speaks, says int
	o.OnCall(addr(speechSpeakFn), func(*oracle.Oracle) { speaks++ })
	o.OnCall(addr(speechSayFn), func(*oracle.Oracle) { says++ })

	// send 照錄製時的節奏一個一個送鍵，中間讓原版跑一段。
	send := func(t *testing.T, k string, scan bool) {
		t.Helper()
		for i, r := range k {
			if i > 0 {
				if err := o.Run(keyGap); err != nil {
					t.Fatalf("送 %q 的第 %d 個鍵時停止：%v", k, i+1, err)
				}
			}
			if scan {
				o.PressScan(string(r))
			} else {
				o.Press(string(r))
			}
		}
	}

	var worst, worstAt, stableN, reported int
	for n, s := range steps {
		k := strings.ReplaceAll(s.keys, "Return", "\r")
		o.Drain()
		switch {
		case n < 3:
			// 裝置三題走 `int 21h AH=08`，讀的是字元。
			send(t, k, false)
		case n+1 < openSteps:
			// 開場的續行不照抄腳本，統一在第 openSteps 步處理。
			continue
		case n+1 == openSteps:
			// 這一張是主選單本身，不送鍵——送了會選走一個項目。
		case n+1 >= scanFrom && !isPassword(k):
			send(t, k, true)
		default:
			// 主選單、年代讀字元；防拷密碼那一關**不吃掃描碼**
			//（`docs/re/02` §3.1、§3.3）。
			send(t, k, false)
		}

		var be *oracle.BudgetError
		settled := false
		switch {
		case n < 3:
			if err := o.Run(20_000_000); err != nil {
				t.Fatalf("裝置第 %d 題停止：%v", n+1, err)
			}
		case n+1 == openSteps:
			// **Enter 要送在它真的在等的時候。** 三英那張定格等一個 Enter，
			// 但早送會被前面的畫面吃掉——送在第 60M 道指令上，之後就
			// 一直卡在那張圖上（跑滿 1G 道指令 MENU3 都沒載進來）。
			// 所以「清乾淨、送一個、跑一段、沒到就再來」，不去記那個
			// 量出來的指令數。清乾淨是必要的：佇列裡的殘鍵會堵住後面的讀取。
			for at := uint64(0); !menuDrawn && at < openBudget; at += openChunk {
				o.Drain()
				o.Press("\r")
				if err := o.RunUntil(menuUp, oracle.Budget(openChunk)); err != nil &&
					!errors.As(err, &be) {
					t.Fatalf("開場停止：%v\n主控台 %q", err, o.Console())
				}
			}
			if !menuDrawn {
				t.Fatalf("跑滿 %d 道指令主選單還沒上來\n主控台 %q",
					openBudget, o.Console())
			}
			t.Logf("主選單的圖在第 %d 道指令載進來", o.Steps())
			// 殘留的 Enter 會被剛畫好的選單收走，選掉一個項目。
			o.Drain()
			if err := o.Run(drawBudget); err != nil {
				t.Fatalf("畫主選單時停止：%v", err)
			}
			settled = true
		default:
			err := o.RunUntil(oracle.ScreenIdle(idleSteps), oracle.Budget(20*idleSteps))
			if err != nil && !errors.As(err, &be) {
				t.Fatalf("第 %d 步（%s）等靜止時停止：%v", n+1, s.keys, err)
			}
			settled = err == nil
		}
		// 進了遊戲之後每一步都把音效與語音兩個開關寫開（冪等）。
		if n >= 14 {
			work := o.ES()
			o.SetWord(oracle.Addr{Seg: work, Off: sfxStateOff}, 0)
			o.SetWord(oracle.Addr{Seg: work, Off: voiceStateOff}, 0)
		}
		dumpScreen(t, o, fmt.Sprintf("seq-%03d", n+1))
		if n+1 < openSteps {
			continue
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
		if s.stable && settled {
			stableN++
			if bad > worst {
				worst, worstAt = bad, n+1
			}
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
		if reported < 40 {
			reported++
			t.Logf("第 %2d 步 送 %-12s %s 差 %6d 點（%.2f%% 相同）",
				n+1, s.keys, mark, bad, pct)
		}
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

// imgLoadFn 是載圖常式的入口 `36C9:02B0`；Arg(0)/Arg(1) 是檔名的
// offset/segment（`TestZZOpeningAssetNames`）。
const imgLoadFn = 0x36c9*16 + 0x2b0

// cstr 讀出線性位址上那個以 0 結尾的檔名。
func cstr(o *oracle.Oracle, lin uint32) string {
	var b strings.Builder
	for i := uint32(0); i < 16; i++ {
		c := o.Byte(addr(lin + i))
		if c == 0 {
			break
		}
		b.WriteByte(c)
	}
	return b.String()
}

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
