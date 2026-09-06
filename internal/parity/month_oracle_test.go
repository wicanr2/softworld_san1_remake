//go:build oracle

package parity

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 讓原版走完一個月，把三張表的每一次變化記下來。
//
// 盤面由對拍這一方寫進去（`tables_oracle_test.go`），所以比出來的差異
// 只可能來自決策，不會來自亂數擺出來的局面。
//
// 每郡每月一道指令（手冊 p.17）；不輸入數字直接按 Enter 就結束該郡的
// 指令（p.5）。所以**一路送 Enter 就會走完自己的回合**，接下來是電腦
// 諸侯的回合，那正是 `docs/mechanics/70-ai` 要看的東西。

// TestZZAdvanceMonth 一路用「內政 → 休息」把每個郡的指令用掉，
// 記錄盤面每一次變化。
//
// 休息是「不做任何事」而且帶 ＊（使用後即轉移控制權，說明書 p.42）。
// 三段：`4` 內政 → `4` 休息 → `Y` 確認（原版會問「休息（Y/N）：」）。
// 所以它是**把一個郡的回合用掉又不動盤面**最便宜的路。走完自己的郡
// 之後輪到電腦諸侯，那時候盤面的變化就是要看的東西。
//
// 環境變數 `SAN1_TURNKEY` 換掉送的鍵（用 `|` 分段，每段之間留沉澱時間，
// 預設 `4\r|4\r|Y`），`SAN1_TURNS` 換輪數（預設 16）。
func TestZZAdvanceMonth(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, sta, gen := sc.Tables()
	total := len(mas) + len(sta) + len(gen)

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToGame(t, o, mas)
	dumpScreen(t, o, "20-過關後的主畫面")

	seq := strings.Split(envOr("SAN1_TURNKEY", "4\r|4\r|Y"), "|")
	turns := 16
	if v, err := strconv.Atoi(os.Getenv("SAN1_TURNS")); err == nil && v > 0 {
		turns = v
	}

	snap := o.Save()
	const settle = 40_000_000
	prevScr, mask := blinkMask(o, snap, settle, 3)
	prev := o.Bytes(addr(base), total)
	dumpTables(t, prev, "00-起點")

	for i := 1; i <= turns; i++ {
		for j, keys := range seq {
			o.Drain()
			o.PressScan(keys)
			if err := o.Run(settle * 3); err != nil {
				t.Logf("第 %d 輪第 %d 段停止：%v", i, j+1, err)
				return
			}
			cur := o.Bytes(addr(base), total)
			scr := screenOf(o)
			t.Logf("第 %2d 輪 %d/%d 送 %q：畫面差 %6d、盤面差 %4d%s",
				i, j+1, len(seq), keys,
				pixelDiff(prevScr, scr, mask), differs8(prev, cur),
				where(prev, cur, len(mas), len(sta)))
			dumpScreen(t, o, fmt.Sprintf("21-第%02d輪-%d", i, j+1))
			if differs8(prev, cur) > 0 {
				dumpTables(t, cur, fmt.Sprintf("%02d-%d", i, j+1))
				t.Log(byPrefecture(prev, cur, len(mas), len(sta)))
				t.Log(byGeneral(prev, cur, len(mas), len(sta)))
			}
			prev, prevScr = cur, scr
		}
	}
	dumpTables(t, o.Bytes(addr(base), total), "99-終點")
}

// dumpTables 把三張表寫成檔，讓分析不必重跑六分鐘的開機。
//
// 寫進 `workplace/`（gitignore）。那是**遊戲執行期的盤面**，
// 與原版的美術、音樂、字型一樣不散布。
func dumpTables(t *testing.T, b []byte, name string) {
	t.Helper()
	dir := os.Getenv("SAN1_DUMP")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Log(err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, name+".bin"), b, 0o644); err != nil {
		t.Log(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
