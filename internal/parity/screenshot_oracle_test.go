//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestZZOriginalMainScreen 把原版的遊戲主畫面存成 PNG。
//
// 這是**素材合成的基準畫面**：remake 要用原版的 `MAINMAP*` 拼出同一張，
// 判準是逐像素相同（`docs/spec/005`）。設 `SAN1_SHOTS` 才會寫檔。
func TestZZOriginalMainScreen(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	bootToGame(t, o, seedMas)
	dumpScreen(t, o, "orig-main")
	t.Log("原版主畫面已存（要設 SAN1_SHOTS）")
}

// TestZZOriginalLoadedScreen 存**剛載完進度**的主畫面。
//
// 與 `TestZZOriginalMainScreen` 差一個月：那一支走的是 `bootToGame`，
// 而觸發防拷密碼的那道指令會把玩家的第一個月用掉（`bootToMain` 的說明）。
// 要拿畫面上的州郡填色去對 remake 讀出來的第一個進度，就得用這一張——
// 差一個月，郡就可能易主，而**那看起來與「填色的對應表錯了」一模一樣**。
func TestZZOriginalLoadedScreen(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	bootToMain(t, o, seedMas)
	dumpScreen(t, o, "orig-loaded")
	// 等原版自己畫出下一個閃爍相位再存第二張；Budget 只作失敗上限。
	// 兩張會不一樣，而**單看一張分不出「閃爍」與「對應表錯了」**。
	first := screenOf(o)
	var nextSample uint64
	changed := oracle.NewCond("主畫面出現下一個閃爍相位", func(o *oracle.Oracle) bool {
		if o.Steps() < nextSample {
			return false
		}
		nextSample = o.Steps() + 100_000
		return pixelDiff(first, screenOf(o), nil) > 0
	})
	if err := o.RunUntil(changed, oracle.Budget(100_000_000)); err != nil {
		t.Fatalf("等待主畫面閃爍：%v", err)
	}
	dumpScreen(t, o, "orig-loaded2")
	t.Log("原版剛載完第一個進度的主畫面已存兩張（要設 SAN1_SHOTS）")
}

// TestZZOriginalOpeningScreens 把開機到主選單之間的每一步都存成 PNG。
//
// 每張圖以前一張圖完成、原版呼叫下一次載圖為擷取點；最後停在主選單
// 真正等掃描碼的位置。指令 Budget 只作失敗上限。
func TestZZOriginalOpeningScreens(t *testing.T) {
	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	frame := 0
	o.OnCall(addr(imgLoadFn), func(o *oracle.Oracle) {
		name := strings.ToUpper(cstr(o, uint32(o.Arg(1))*16+uint32(o.Arg(0))))
		if !strings.HasPrefix(name, "SANT") && !strings.HasPrefix(name, "TITL") &&
			!strings.HasPrefix(name, "CMARK") && name != "MENU3.IMG" {
			return
		}
		safe := strings.NewReplacer(".", "-", "/", "-", "\\", "-").Replace(name)
		dumpScreen(t, o, fmt.Sprintf("open-%02d-before-%s", frame, safe))
		frame++
	})
	bootToMenu(t, o)
	dumpScreen(t, o, fmt.Sprintf("open-%02d-main-menu", frame))
	if frame == 0 {
		t.Fatal("開場到主選單之間沒有觀測到任何已知載圖行為")
	}
	t.Log("開場逐步畫面已存（要設 SAN1_SHOTS）")
}
