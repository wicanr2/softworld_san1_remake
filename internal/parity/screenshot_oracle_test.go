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

	o.Press("122")
	for i := 0; i < 10; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Fatalf("第 %d 步停止：%v", i, err)
		}
		dumpScreen(t, o, fmt.Sprintf("open-%02d", i))
		if i == 4 {
			o.Press("\r")
		}
	}
	t.Log("開場逐步畫面已存（要設 SAN1_SHOTS）")
}
