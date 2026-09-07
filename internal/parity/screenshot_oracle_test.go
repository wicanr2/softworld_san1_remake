//go:build oracle

package parity

import (
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
