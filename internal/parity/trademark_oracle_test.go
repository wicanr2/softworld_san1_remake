//go:build oracle

package parity

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// 智冠的商標畫面 `CMARKL`／`CMARKR`（Issue #34）：原版 `AA.EXE` 開機時讀進
// 這兩張（`docs/formats/01` §2 的 seek），這一支照 `bootToMenu` 的步驟開機，
// 途中每一百萬條指令看一次畫面，畫面變了就存一張。
//
// ⚠ **它只讀第 0 頁。** 片頭在兩頁之間切顯示頁（`docs/spec/005`「片頭」），
// 所以存下來的不一定是顯示中的畫面——標題字那一幕畫在第 1 頁、這裡看不到，
// 頭像橫幅那一段看到的是拼圖用的工作區。片頭逐拍的對拍是
// `TestZZOpeningMatchesTheOriginal`。
func TestZZBootScreensCensus(t *testing.T) {
	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	var last [sha256.Size]byte
	var next uint64
	scenes := 0
	sample := func() {
		if o.Steps() < next {
			return
		}
		next = o.Steps() + 1_000_000
		h := sha256.Sum256(screenOf(o))
		if h == last {
			return
		}
		last = h
		dumpScreen(t, o, fmt.Sprintf("boot-%03d-step%010d", scenes, o.Steps()))
		scenes++
	}
	wait := func(name string, budget uint64, ready func() bool) {
		t.Helper()
		var check uint64
		cond := oracle.NewCond(name, func(*oracle.Oracle) bool {
			if o.Steps() < check {
				return false
			}
			check = o.Steps() + 50_000
			sample()
			return ready()
		})
		if err := o.RunUntil(cond, oracle.Budget(budget)); err != nil {
			t.Fatalf("等待%s失敗（step=%d）：%v", name, o.Steps(), err)
		}
	}
	key := func(name, k string) {
		before := o.KeyWaits()
		wait(name, 50_000_000, func() bool { return o.KeyWaits() > before })
		o.Type(k)
	}
	s := observeBoot(o)
	key("音樂裝置輸入", "1")
	key("繪圖裝置輸入", "2")
	key("磁碟裝置輸入", "2")
	wait("開場故事續行畫面", 400_000_000, func() bool { return sha256.Sum256(screenOf(o)) == bootOpeningStoryScreen })
	o.Type("\r")
	wait("標題續行畫面", 1_000_000_000, func() bool { return sha256.Sum256(screenOf(o)) == bootOpeningTitleScreen })
	before := s.titleBIOSAsk
	wait("標題清鍵後的新鍵輪詢", 500_000_000, func() bool { return s.titleBIOSAsk > before })
	if err := o.SendKeys("Return"); err != nil {
		t.Fatal(err)
	}
	wait("主選單輸入", 1_000_000_000, func() bool { return s.menuAsk > 0 })
	t.Logf("開機到主選單出現過 %d 幕（存在 SAN1_SHOTS）", scenes)
	for _, op := range o.FileOps() {
		if op.Name == "DATA1.GRP" && op.Op == "seek" && (op.Pos == 531158 || op.Pos == 577562) {
			t.Logf("讀 CMARK：seek %d", op.Pos)
		}
	}
}

// TestZZTrademarkMatchesTheOriginal 開機後每一百萬條指令看一次畫面，找到
// 商標那一幕（`CMARKL` 貼上去之後），與 `assets.TrademarkScreen` 逐格比整張。
func TestZZTrademarkMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	c1 := openContainer(t, filepath.Join(root, "DATA1"))
	want, err := assets.TrademarkScreen(c1)
	if err != nil {
		t.Fatal(err)
	}
	left, err := assets.DecodeImage(c1.Data(mustEntry(t, c1, "CMARKL.IMG")))
	if err != nil {
		t.Fatal(err)
	}
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	s := observeBoot(o)
	_ = s
	waitBootKey(t, o, "音樂裝置輸入", "1")
	waitBootKey(t, o, "繪圖裝置輸入", "2")
	waitBootKey(t, o, "磁碟裝置輸入", "2")
	// 左半整塊貼好的那一刻：CMARKL 在 (0, y) 逐格相同。y 從畫面上量。
	match := func(scr []uint8, y int) bool {
		for yy := 0; yy < left.H; yy++ {
			for xx := 0; xx < left.W; xx++ {
				if scr[(y+yy)*scrW+xx]&15 != left.Pix[yy*left.W+xx]&15 {
					return false
				}
			}
		}
		return true
	}
	var shot []uint8
	at := -1
	var next uint64
	cond := oracle.NewCond("商標那一幕", func(*oracle.Oracle) bool {
		if o.Steps() < next {
			return false
		}
		next = o.Steps() + 1_000_000
		scr := screenOf(o)
		for y := 0; y+left.H <= scrH; y++ {
			if match(scr, y) {
				// 右半也要貼好才算。
				full := true
				for i := range scr {
					if scr[i]&15 != want.Pix[i]&15 {
						full = false
						break
					}
				}
				if full || at < 0 {
					at = y
				}
				if full {
					shot = scr
					return true
				}
			}
		}
		return false
	})
	if err := o.RunUntil(cond, oracle.Budget(40_000_000)); err != nil {
		if at >= 0 {
			t.Fatalf("CMARKL 在 y %d 逐格相同，但整張沒有與 TrademarkScreen 相同過（TrademarkY ＝ %d）：%v",
				at, assets.TrademarkY, err)
		}
		t.Fatalf("開機四千萬條指令內沒有出現商標那一幕：%v", err)
	}
	dumpScreen(t, o, "trademark")
	t.Logf("第 %d 條指令：整張 %d×%d 與 TrademarkScreen 逐格相同（CMARKL 在 y %d）", o.Steps(), scrW, scrH, at)
	_ = shot
}

func mustEntry(t *testing.T, c *assets.Container, name string) int {
	t.Helper()
	i, ok := c.ByName(name)
	if !ok {
		t.Fatalf("容器裡沒有 %s", name)
	}
	return i
}
