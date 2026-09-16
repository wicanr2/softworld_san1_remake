//go:build oracle

package parity

import (
	"crypto/sha256"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

const (
	bootNumInputFn        = 0x33d8*16 + 0x115e
	bootKeyInputFn        = 0x1538c
	bootASCInputFn        = 0x1058*16 + 0x0e24
	bootMainMenuCaller    = 0x1058*16 + 0x15f7
	bootLoadSlotCaller    = 0x1058*16 + 0x3c9b
	bootPasswordCaller    = 0x03eb*16 + 0x02c2
	bootPasswordYNCaller  = 0x03eb*16 + 0x02de
	bootPlayerPickCaller  = 0x1538*16 + 0x2eb8
	bootMainCmdCaller     = 0x1538*16 + 0x236b
	bootAffairsCaller     = 0x1538*16 + 0x5191
	bootRestYNCaller      = 0x1538*16 + 0x528c
	bootBIOSInputFn       = 0x0583*16 + 0x3792
	bootTitleNewKeyCaller = 0x0ad0*16 + 0x0104
)

var (
	bootOpeningStoryScreen = [sha256.Size]byte{
		0xc8, 0x05, 0xd6, 0xc8, 0xd2, 0xd4, 0x2f, 0x1b,
		0xab, 0x8a, 0xd2, 0x76, 0xfc, 0x08, 0x6e, 0x48,
		0x10, 0x1c, 0xef, 0x3a, 0x92, 0xd8, 0x76, 0x60,
		0x0f, 0x01, 0xae, 0x94, 0xd2, 0x1a, 0x18, 0x56,
	}
	bootOpeningTitleScreen = [sha256.Size]byte{
		0x8d, 0xc9, 0x28, 0xa0, 0x13, 0xf6, 0xb7, 0x44,
		0x77, 0xbc, 0x12, 0x4c, 0xbd, 0xe7, 0x7e, 0x19,
		0x1a, 0x9a, 0x03, 0x3f, 0xaa, 0x19, 0xc4, 0x03,
		0x34, 0x7d, 0x71, 0x0b, 0x6f, 0x11, 0xe9, 0xaa,
	}
	bootLoadSlotScreen = [sha256.Size]byte{
		0xa1, 0x03, 0x0d, 0x32, 0xd8, 0xb9, 0xe9, 0x3e,
		0x8c, 0xc8, 0x5a, 0x9c, 0x38, 0x03, 0x8f, 0x6c,
		0xc3, 0x08, 0x34, 0x2c, 0xf5, 0x71, 0x65, 0x13,
		0xb4, 0x19, 0x81, 0x35, 0x16, 0xa8, 0xf0, 0x3b,
	}
	bootPasswordScreen = [sha256.Size]byte{
		0x42, 0x5b, 0x45, 0x4e, 0xa9, 0x12, 0x4f, 0xfd,
		0x66, 0x1e, 0xe5, 0x65, 0xb0, 0xe3, 0xbf, 0x56,
		0x8b, 0x83, 0xd8, 0xc2, 0xde, 0x11, 0x34, 0x92,
		0x46, 0x1a, 0xb8, 0x62, 0x19, 0x19, 0x5d, 0xac,
	}
	bootPlayerPickScreen = [sha256.Size]byte{
		0x95, 0xa3, 0x3e, 0x17, 0xeb, 0x80, 0xa1, 0x36,
		0x30, 0xa0, 0x03, 0xce, 0x97, 0xe6, 0xd2, 0x39,
		0xb8, 0x89, 0x11, 0xde, 0xfc, 0x01, 0xe9, 0xba,
		0x8a, 0xc3, 0xcd, 0x31, 0x7b, 0x8d, 0x95, 0x61,
	}
)

type bootSignals struct {
	menuImage, menuAsk, slotAsk                 int
	passwordAsk, passwordYN, playerAsk, mainAsk int
	affairsAsk, restYN                          int
	titleBIOSAsk                                int
}

func observeBoot(o *oracle.Oracle) *bootSignals {
	s := &bootSignals{}
	faceCensus(o)
	o.OnCall(addr(imgLoadFn), func(o *oracle.Oracle) {
		name := cstr(o, uint32(o.Arg(1))*16+uint32(o.Arg(0)))
		if name == "MENU3.IMG" {
			s.menuImage++
		}
	})
	o.OnCall(addr(bootASCInputFn), func(o *oracle.Oracle) {
		switch o.Caller().Linear() {
		case bootMainMenuCaller:
			s.menuAsk++
		case bootLoadSlotCaller:
			s.slotAsk++
		}
	})
	o.OnCall(addr(bootKeyInputFn), func(o *oracle.Oracle) {
		switch o.Caller().Linear() {
		case bootPasswordYNCaller:
			s.passwordYN++
		case bootRestYNCaller:
			s.restYN++
		}
	})
	o.OnCall(addr(bootBIOSInputFn), func(o *oracle.Oracle) {
		if o.Caller().Linear() == bootTitleNewKeyCaller && o.Arg(0) == 1 {
			s.titleBIOSAsk++
		}
	})
	o.OnCall(addr(bootNumInputFn), func(o *oracle.Oracle) {
		switch o.Caller().Linear() {
		case bootPasswordCaller:
			if int16(o.Arg(0)) == 0 && int16(o.Arg(1)) == 9999 {
				s.passwordAsk++
			}
		case bootPlayerPickCaller:
			if int16(o.Arg(0)) == 1 && int16(o.Arg(1)) == 1 {
				s.playerAsk++
			}
		case bootMainCmdCaller:
			if int16(o.Arg(0)) == 0 && int16(o.Arg(1)) == 9 {
				s.mainAsk++
			}
		case bootAffairsCaller:
			if int16(o.Arg(0)) == 1 && int16(o.Arg(1)) == 4 {
				s.affairsAsk++
			}
		}
	})
	return s
}

func waitBoot(t *testing.T, o *oracle.Oracle, name string, budget uint64, ready func() bool) {
	t.Helper()
	cond := oracle.NewCond(name, func(*oracle.Oracle) bool { return ready() })
	if err := o.RunUntil(cond, oracle.Budget(budget)); err != nil {
		t.Fatalf("等待%s失敗（step=%d、畫面 SHA-256=%x）：%v",
			name, o.Steps(), sha256.Sum256(screenOf(o)), err)
	}
}

func waitBootScreen(t *testing.T, o *oracle.Oracle, name string, want [sha256.Size]byte,
	budget uint64) {
	t.Helper()
	var next uint64
	cond := oracle.NewCond(name, func(*oracle.Oracle) bool {
		if o.Steps() < next {
			return false
		}
		next = o.Steps() + 1_000_000
		return sha256.Sum256(screenOf(o)) == want
	})
	if err := o.RunUntil(cond, oracle.Budget(budget)); err != nil {
		t.Fatalf("等待%s失敗（step=%d、畫面 SHA-256=%x）：%v",
			name, o.Steps(), sha256.Sum256(screenOf(o)), err)
	}
}

func waitBootKey(t *testing.T, o *oracle.Oracle, name, key string) {
	t.Helper()
	before := o.KeyWaits()
	waitBoot(t, o, name, 50_000_000, func() bool { return o.KeyWaits() > before })
	o.Type(key)
}

func waitBootScan(t *testing.T, o *oracle.Oracle, name string, budget uint64) {
	t.Helper()
	at := oracle.Addr{Seg: 0x1058, Off: 0x0e57}
	if err := o.RunUntil(oracle.At(at), oracle.Budget(budget)); err != nil {
		t.Fatalf("等待%s的掃描碼讀取迴圈失敗（step=%d）：%v", name, o.Steps(), err)
	}
}

// bootToMenu 只走到原版主選單已畫完、正要讀掃描碼的停點。
func bootToMenu(t *testing.T, o *oracle.Oracle) *bootSignals {
	t.Helper()
	s := observeBoot(o)
	waitBootKey(t, o, "音樂裝置輸入", "1")
	waitBootKey(t, o, "繪圖裝置輸入", "2")
	waitBootKey(t, o, "磁碟裝置輸入", "2")
	waitBootScreen(t, o, "開場故事續行畫面", bootOpeningStoryScreen, 400_000_000)
	o.Type("\r")
	waitBootScreen(t, o, "標題續行畫面", bootOpeningTitleScreen, 1_000_000_000)
	beforeBIOS := s.titleBIOSAsk
	waitBoot(t, o, "標題清鍵後的新鍵輪詢", 500_000_000,
		func() bool { return s.titleBIOSAsk > beforeBIOS })
	if err := o.SendKeys("Return"); err != nil {
		t.Fatalf("送出標題 BIOS Return 鍵失敗：%v", err)
	}
	waitBoot(t, o, "主選單輸入", 1_000_000_000, func() bool { return s.menuAsk > 0 })
	if s.menuImage == 0 {
		t.Fatal("主選單開始輸入前沒有載入 MENU3.IMG")
	}
	waitBootScan(t, o, "主選單", 5_000_000)
	return s
}

// bootToMain 把原版開到第一個遊戲主命令輸入，回傳三張表的基底。
func bootToMain(t *testing.T, o *oracle.Oracle, mas []byte) uint32 {
	t.Helper()
	base, _ := bootToMainState(t, o, mas)
	return base
}

func bootToMainState(t *testing.T, o *oracle.Oracle, mas []byte) (uint32, *bootSignals) {
	t.Helper()
	s := bootToPassword(t, o)
	o.Drain()
	o.PressScan("1\r")
	waitBoot(t, o, "密碼確認輸入", 500_000_000, func() bool { return s.passwordYN > 0 })
	o.Drain()
	o.PressScan("Y")
	waitBoot(t, o, "玩家選擇輸入", 500_000_000, func() bool { return s.playerAsk > 0 })
	waitBootScreen(t, o, "玩家選擇畫面", bootPlayerPickScreen, 500_000_000)
	waitBootScan(t, o, "玩家選擇", 5_000_000)
	o.Drain()
	o.PressScan("1\r")
	waitBoot(t, o, "遊戲主命令輸入", 500_000_000, func() bool { return s.mainAsk > 0 })
	waitBootScan(t, o, "遊戲主命令", 5_000_000)

	base := uint32(0)
	if len(mas) >= 48 {
		if h := o.Search(mas[:48]); len(h) == 1 {
			base = h[0]
		}
	}
	if base == 0 {
		base = 0x399b0
	}
	return base, s
}

// bootToPassword 把原版開到載入存檔後的密碼輸入停點。
func bootToPassword(t *testing.T, o *oracle.Oracle) *bootSignals {
	t.Helper()
	s, _ := bootToPasswordState(t, o)
	return s
}

func bootToPasswordState(t *testing.T, o *oracle.Oracle) (*bootSignals, *oracle.State) {
	t.Helper()
	s := bootToMenu(t, o)
	o.Drain()
	o.PressScan("2")
	waitBoot(t, o, "存檔槽輸入", 100_000_000, func() bool { return s.slotAsk > 0 })
	waitBootScreen(t, o, "存檔槽畫面", bootLoadSlotScreen, 500_000_000)
	waitBootScan(t, o, "存檔槽", 5_000_000)
	atSlot := o.Save()
	o.Drain()
	o.PressScan("1")
	waitBoot(t, o, "畫面所示密碼輸入", 500_000_000, func() bool { return s.passwordAsk > 0 })
	waitBootScreen(t, o, "畫面所示密碼畫面", bootPasswordScreen, 100_000_000)
	waitBootScan(t, o, "畫面所示密碼輸入", 5_000_000)
	return s, atSlot
}

// TestBehaviorTriggeredBootReachesNextMainCommand 是 Issue #5 的開機回歸閘門。
// 成功停點只看原版輸入 callsite 與掃描碼等待迴圈，不看實測指令總數。
func TestBehaviorTriggeredBootReachesNextMainCommand(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, _, _ := sc.Tables()
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	if base := bootToGame(t, o, mas); base != 0x399b0 {
		t.Fatalf("載入盤面基底=%#x，預期 %#x", base, uint32(0x399b0))
	}
	want := (oracle.Addr{Seg: 0x1058, Off: 0x0e57}).Linear()
	if got := o.IP().Linear(); got != want {
		t.Fatalf("開機成功停點=%s，預期原版掃描碼等待 1058:0E57", o.IP())
	}
	if o.KeyQueueLen() != 0 {
		t.Fatalf("開機成功後仍有 %d 個掃描碼殘留", o.KeyQueueLen())
	}
}
