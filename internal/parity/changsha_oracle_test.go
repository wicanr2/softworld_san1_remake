//go:build oracle

package parity

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 州郡的所屬：原版載完進度之後記憶體裡的值 vs remake 從同一個存檔解出來的值。
//
// 起因是長沙（31）的填色曾對不上：畫面是 `EGAFILL.PAL` 檔案區塊 0
// 的淺紅，而存檔裡的所屬是 2。原版記憶體證實所屬沒錯；真正原因是
// 原版執行期勢力槽 0、2 對應的檔案區塊互換。
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

// TestZZTraceMainMapCallers 從原版實際載入 MAINMAP 圖片的呼叫抓主畫面繪製端。
// 同列執行期線性位址與 IDA EA，避免把 file offset、dosgolem 位址與 IDA
// 位址混成同一套數字。
func TestZZTraceMainMapCallers(t *testing.T) {
	root := origRoot(t)
	c2 := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c2, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	seen := map[string]bool{}
	o.OnCall(addr(imgLoadFn), func(o *oracle.Oracle) {
		name := strings.ToUpper(cstr(o, uint32(o.Arg(1))*16+uint32(o.Arg(0))))
		if !strings.HasPrefix(name, "MAINMAP") {
			return
		}
		caller := o.Caller()
		key := fmt.Sprintf("%s:%x", name, caller.Linear())
		if seen[key] {
			return
		}
		seen[key] = true
		t.Logf("%s：呼叫端 runtime=%04x:%04x（線性 %#07x），IDA EA=%#07x",
			name, caller.Seg, caller.Off, caller.Linear(), o.ToIDA(caller))
	})

	bootToMain(t, o, seedMas)
	if len(seen) == 0 {
		t.Fatal("載入主畫面時沒有觀測到 MAINMAP 圖片")
	}
	// 要靜態回查這一層，必須從**同一次執行**取解壓後的碼；歷史
	// OVL.BIN 只到 0x13ee0，涵蓋不到本次 MAINMAP caller 0x14477 起。
	dumpImage(t, o, 0xb000, 0x20000, "main-screen-code")
}

// TestZZTraceMainMapFillCalls 只在第一張 MAINMAP 載入之後追繪圖入口，
// 用來辨認主地圖的填色呼叫端、種子座標及兩個間接 BGI 入口。位址來自
// TestZZTraceMainMapCallers 同一次執行所倒出的解壓後程式碼，不是 AA.EXE
// 的 file offset，也不是歷史 OVL.BIN。
func TestZZTraceMainMapFillCalls(t *testing.T) {
	root := origRoot(t)
	c2 := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c2, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()
	c1 := openContainer(t, filepath.Join(root, "DATA1"))
	fillIndex, ok := c1.ByName("EGAFILL.PAL")
	if !ok {
		t.Fatal("DATA1 沒有 EGAFILL.PAL")
	}
	fillBytes := c1.Data(fillIndex)
	if len(fillBytes) != 16*64 {
		t.Fatalf("EGAFILL.PAL 是 %d bytes，要 1024", len(fillBytes))
	}
	var filePatterns [16]assets.FillPattern
	for i := range filePatterns {
		copy(filePatterns[i][:], fillBytes[i*64:(i+1)*64])
	}

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	mainStarted := false
	o.OnCall(addr(imgLoadFn), func(o *oracle.Oracle) {
		name := strings.ToUpper(cstr(o, uint32(o.Arg(1))*16+uint32(o.Arg(0))))
		if strings.HasPrefix(name, "MAINMAP1") {
			mainStarted = true
			t.Logf("開始追蹤主地圖繪圖：%s", name)
		}
	})

	var fullMapCalls [][2]uint16
	var perPrefecture []uint16
	longshaSeen := false
	longshaPattern := uint16(0xffff)
	o.OnCall(addr(0x32fb8), func(o *oracle.Oracle) {
		if !mainStarted {
			return
		}
		fullMapCalls = append(fullMapCalls, [2]uint16{o.Arg(0), o.Arg(1)})
	})
	o.OnCall(addr(0x32fe6), func(o *oracle.Oracle) {
		if mainStarted {
			perPrefecture = append(perPrefecture, o.Arg(2))
		}
	})
	o.OnCall(addr(0x3239), func(o *oracle.Oracle) {
		if o.Arg(0) != 31 {
			return
		}
		longshaSeen = true
		longshaPattern = o.Arg(1)
		t.Logf("長沙下一層入口 runtime=0110:2139：args=(郡=%d, 圖樣=%d, %#04x)",
			o.Arg(0), o.Arg(1), o.Arg(2))
	})

	base := bootToMain(t, o, seedMas)
	if len(fullMapCalls) != 1 || fullMapCalls[0] != [2]uint16{0x50, 0x2c} {
		t.Fatalf("全圖入口呼叫=%v，要唯一一次 (0x50,0x2c)", fullMapCalls)
	}
	if len(perPrefecture) != 42 {
		t.Fatalf("逐郡入口呼叫 %d 次，要 42 次：%v", len(perPrefecture), perPrefecture)
	}
	for i, p := range perPrefecture {
		if want := uint16(i + 1); p != want {
			t.Fatalf("逐郡入口第 %d 次收到郡 %d，要 %d", i, p, want)
		}
	}
	if !longshaSeen || longshaPattern != 2 {
		t.Fatalf("長沙逐郡繪圖參數：seen=%v pattern=%d，要 seen=true pattern=2",
			longshaSeen, longshaPattern)
	}
	type pixel struct{ x, y uint16 }
	runs := make(map[uint16]map[pixel]bool, 42)
	finalPrefecture := make(map[pixel]uint16)
	for p := uint16(1); p <= 42; p++ {
		list := o.Word(oracle.Addr{Seg: 0x427e, Off: 0x1095 + 2*p})
		area := make(map[pixel]bool)
		for n := uint16(0); n < 4096; n++ {
			x1 := o.Word(oracle.Addr{Seg: 0x427e, Off: list + 6*n})
			if x1 == 0xffff {
				break
			}
			x2 := o.Word(oracle.Addr{Seg: 0x427e, Off: list + 6*n + 2})
			y := o.Word(oracle.Addr{Seg: 0x427e, Off: list + 6*n + 4})
			if x2 < x1 {
				x1, x2 = x2, x1
			}
			for x := x1; x <= x2; x++ {
				pt := pixel{x: x, y: y}
				area[pt] = true
				finalPrefecture[pt] = p
			}
		}
		runs[p] = area
	}
	overwrittenBy := make(map[uint16]int)
	finalOwner := make(map[int]int)
	staBase := base + uint32(state.MasterTableSize)
	for pt := range runs[31] {
		p := finalPrefecture[pt]
		if p != 31 {
			overwrittenBy[p]++
		}
		owner := int(int8(o.Byte(addr(staBase + uint32(p)*176 + 30))))
		finalOwner[owner]++
	}
	t.Logf("長沙預編碼像素=%d，後畫郡覆蓋=%v，終畫面所屬分布=%v",
		len(runs[31]), overwrittenBy, finalOwner)
	if len(overwrittenBy) != 0 || finalOwner[2] != len(runs[31]) {
		t.Fatalf("長沙線段被覆蓋或終畫面所屬不是 2：覆蓋=%v、所屬=%v",
			overwrittenBy, finalOwner)
	}
	for _, pattern := range []uint16{0, 2} {
		raw := o.Bytes(oracle.Addr{Seg: 0x427e, Off: 0x098e + 32*pattern}, 32)
		colours := make(map[byte]int)
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				var colour byte
				mask := byte(0x80 >> x)
				for packedPlane := 0; packedPlane < 4; packedPlane++ {
					if raw[y*4+packedPlane]&mask != 0 {
						colour |= 1 << (3 - packedPlane)
					}
				}
				colours[colour]++
			}
		}
		t.Logf("原版執行期圖樣 %d：packed=% x，解出的色號=%v", pattern, raw, colours)
	}
	for pattern := uint16(0); pattern < 16; pattern++ {
		raw := o.Bytes(oracle.Addr{Seg: 0x427e, Off: 0x098e + 32*pattern}, 32)
		var decoded assets.FillPattern
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				mask := byte(0x80 >> x)
				for packedPlane := 0; packedPlane < 4; packedPlane++ {
					if raw[y*4+packedPlane]&mask != 0 {
						decoded[y*8+x] |= 1 << (3 - packedPlane)
					}
				}
			}
		}
		var matches []int
		for fileIndex := range filePatterns {
			if decoded == filePatterns[fileIndex] {
				matches = append(matches, fileIndex)
			}
		}
		t.Logf("原版執行期槽 %d 對應 EGAFILL 檔案區塊 %v", pattern, matches)
		want := int(pattern)
		if pattern == 0 {
			want = 2
		} else if pattern == 2 {
			want = 0
		}
		if len(matches) != 1 || matches[0] != want {
			t.Errorf("原版執行期槽 %d 對應檔案區塊 %v，要唯一的 %d",
				pattern, matches, want)
		}
	}
	dumpImage(t, o, 0x32f00, 0x33500, "main-map-fill")
	dumpImage(t, o, 0x3000, 0x4000, "main-map-fill-inner")
	dumpImage(t, o, 0x2e00, 0x3000, "main-map-fill-primitive")
}

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
			"勢力槽與 EGAFILL 檔案區塊的轉置證據要重查", got)
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
