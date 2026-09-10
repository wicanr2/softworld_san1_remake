//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
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
	// 再跑一段之後存第二張：畫面上會閃的東西（例如某些州郡的填色）
	// 兩張會不一樣，而**單看一張分不出「閃爍」與「對應表錯了」**。
	if err := o.Run(20_000_000); err != nil {
		t.Fatalf("停止：%v", err)
	}
	dumpScreen(t, o, "orig-loaded2")
	t.Log("原版剛載完第一個進度的主畫面已存兩張（要設 SAN1_SHOTS）")
}

// TestZZOriginalOpeningScreens 把開機到主選單之間的每一步都存成 PNG。
//
// 開場的圖（`SANT*`／`TITL*`／`CMARK*`）在哪一步出現是**量出來的**，
// 不是算得出來的：`docs/re/02` §3 的指令數會隨執行器改動而變。
// 所以逐步存圖，用眼睛找。
func TestZZOriginalOpeningScreens(t *testing.T) {
	root := origRoot(t)
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	o.TypeBoth("122")
	for i := 0; i < 10; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Fatalf("第 %d 步停止：%v", i, err)
		}
		dumpScreen(t, o, fmt.Sprintf("open-%02d", i))
		if i == 4 {
			o.TypeBoth("\r")
		}
	}
	t.Log("開場逐步畫面已存（要設 SAN1_SHOTS）")
}
